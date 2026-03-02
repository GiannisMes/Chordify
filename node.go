package main

import (
	"fmt"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

type Node struct {
	mu          sync.RWMutex
	ID          *big.Int // Node ID
	Address     string   // Το IP:Port (π.χ. "127.0.0.1:8000")
	Successor   *NodeInfo
	Predecessor *NodeInfo
	DataTable   map[string]string // Τα δεδομενα του κομβου δηλαδη key,value του τραγουδιου
	FingerTable []*NodeInfo
	K           int    // Replication factor
	Consistency string // Μοντέλο συνέπειας (eventual / linear)
	TableLock   sync.RWMutex
	Departing   int32
	// οταν καποιος παει να κανει insert delete κλειδωνει το map , επιτρεπει ομως σε πολλους να διαβαζουν ταυτοχρονα
}

// κραταει ουσιαστικα το id και adress mazi me to port του successor και predecessor
type NodeInfo struct {
	ID      *big.Int
	Address string
}

// Συνάρτηση για να φτιάχνουμε έναν νέο κόμβο
func NewNode(address string, k int, consistency string) *Node {
	return &Node{
		ID:          hashString(address), //συναρτηση απο  hash.go
		Address:     address,
		DataTable:   make(map[string]string),
		FingerTable: make([]*NodeInfo, 160), // θα μπορουσαμε και λιγοτερες θεσεις
		K:           k,
		Consistency: consistency,
	}
}

func (n *Node) Join(bootstrapAddr string) error {
	// 1. Στέλνουμε το μήνυμα FIND_SUCCESSOR στον bootstrap για το δικό μας ID
	resp, err := n.call(bootstrapAddr, "FIND_SUCCESSOR "+n.ID.String())
	if err != nil {
		return fmt.Errorf("σφάλμα επικοινωνίας με τον bootstrap: %v", err)
	}

	// 2. Μετατρέπουμε το string που λάβαμε σε NodeInfo
	newSuccessor := parseNodeInfo(resp)
	if newSuccessor == nil {
		return fmt.Errorf("λάθος μορφή απάντησης από τον bootstrap")
	}

	// 3. Ορίζουμε τον Successor μας
	n.Successor = newSuccessor

	// 4. Βρίσκουμε τον predecessor του successor (= ο δικός μας predecessor)
	predResp, err := n.call(newSuccessor.Address, "GET_PREDECESSOR")
	var predID *big.Int
	if err == nil && predResp != "NONE" {
		predNode := parseNodeInfo(predResp)
		if predNode != nil {
			predID = predNode.ID
		}
	}
	// Fallback: αν δεν βρούμε predecessor χρησιμοποιούμε τον successor
	if predID == nil {
		predID = newSuccessor.ID
	}

	// 5. Ζητάμε από τον successor τα keys που μας ανήκουν
	n.call(newSuccessor.Address, fmt.Sprintf("TRANSFER_KEYS %s,%s,%s", n.Address, n.ID.String(), predID.String()))

	fmt.Printf("Ο Successor είναι: %s\n", n.Successor.Address)
	return nil
}

func (n *Node) Depart() {
	atomic.StoreInt32(&n.Departing, 1)
	time.Sleep(1500 * time.Millisecond)
	n.mu.RLock()
	succ := n.Successor
	pred := n.Predecessor
	n.mu.RUnlock()

	if succ == nil || pred == nil || succ.Address == n.Address {
		fmt.Println("Ο κόμβος αποχωρεί (μόνος στο δίκτυο).")
		return
	}

	n.TableLock.RLock()
	transferData := make(map[string]string)
	for key, value := range n.DataTable {
		transferData[key] = value
	}
	n.TableLock.RUnlock()

	fmt.Printf("Μεταφορά %d εγγραφών στον Successor (%s)...\n", len(transferData), succ.Address)

	primaryKeys := []string{}
	primaryVals := map[string]string{}
	replicaKeys := []string{}
	replicaPrimaries := map[string]string{}

	for key, value := range transferData {
		hashedKey := hashString(key)

		// checkRange αντί FindSuccessor — δεν αποτυγχάνει ποτέ
		isPrimary := pred == nil || pred.Address == n.Address || checkRange(hashedKey, pred.ID, n.ID)

		if isPrimary {
			primaryKeys = append(primaryKeys, key)
			primaryVals[key] = value
		} else if n.K > 1 {
			primary, err := n.FindSuccessor(hashedKey)
			if err == nil && primary != nil && primary.Address != n.Address {
				replicaKeys = append(replicaKeys, key)
				replicaPrimaries[key] = primary.Address
			}
		}
	}

	// Βήμα 1: TRANSFER primary keys → succ
	for _, key := range primaryKeys {
		_, err := n.call(succ.Address, fmt.Sprintf("TRANSFER %s %s", key, primaryVals[key]))
		if err != nil {
			fmt.Printf("Σφάλμα μεταφοράς του '%s'\n", key)
		} else {
			fmt.Printf("Το '%s' (primary) μεταφέρθηκε με επιτυχία!\n", key)
		}
	}

	// Βήμα 2: Ενημέρωση ring pointers
	n.call(succ.Address, fmt.Sprintf("SET_PREDECESSOR %s,%s", pred.Address, pred.ID.String()))
	n.call(pred.Address, fmt.Sprintf("SET_SUCCESSOR %s,%s", succ.Address, succ.ID.String()))

	// Βήμα 3: REBALANCE για replica keys
	for _, key := range replicaKeys {
		primaryAddr := replicaPrimaries[key]
		_, err := n.call(primaryAddr, fmt.Sprintf("REBALANCE_REPLICAS %s", key))
		if err != nil {
			fmt.Printf("Σφάλμα ειδοποίησης primary για '%s'\n", key)
		} else {
			fmt.Printf("Το '%s' (replica) → ειδοποιήθηκε ο primary %s\n", key, primaryAddr)
		}
	}

	// Βήμα 4: Ο succ γίνεται νέος primary για τα δικά μας primary keys.
	// Πρέπει να συμπληρώσει τη νέα K-θέση — το κάνει μόνος αφού του πούμε.
	for _, key := range primaryKeys {
		n.call(succ.Address, fmt.Sprintf("REBALANCE_REPLICAS %s", key))
	}

	fmt.Println("Οι δείκτες ενημερώθηκαν. Ο κόμβος αποχωρεί.")
}

func (n *Node) RedistributeMyReplicas() {
	snapshot := n.StoreGetAll()

	n.mu.RLock()
	pred := n.Predecessor
	selfID := n.ID
	next := n.Successor
	n.mu.RUnlock()

	nodeToDelete := ""
	successors, err := n.GetSuccessors(n.Address, n.K+1)
	if err == nil && len(successors) > n.K {
		nodeToDelete = successors[n.K]
	}

	for key, val := range snapshot {
		hashedKey := hashString(key)

		isPrimary := false
		if pred == nil || pred.Address == n.Address {
			isPrimary = true
		} else {
			isPrimary = checkRange(hashedKey, pred.ID, selfID)
		}
		if !isPrimary {
			continue
		}

		if n.Consistency == "eventual" {
			n.ReplicateEventual(key, val, false)
		} else if n.Consistency == "linear" {
			if next != nil {
				n.call(next.Address, fmt.Sprintf("CHAIN_WRITE %d %s %s", n.K-1, key, val))
			}
		}

		if nodeToDelete != n.Address && nodeToDelete != "" {
			n.call(nodeToDelete, fmt.Sprintf("REPLICA_DELETE %s", key))
		}
	}
}
