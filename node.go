package main

import (
	"fmt"
	"math/big"
	"sync"
)

type Node struct {
	mu          sync.RWMutex
	ID          *big.Int // Node ID
	Address     string   // Το IP:Port (π.χ. "127.0.0.1:8000")
	Successor   *NodeInfo
	Predecessor *NodeInfo
	DataTable   map[string]string // Τα δεδομενα του κομβου δηλαδη key,value του τραγουδιου
	FingerTable []*NodeInfo
	K           int          // Replication factor
	Consistency string       // Μοντέλο συνέπειας (eventual / linear)
	TableLock   sync.RWMutex // οταν καποιος παει να κανει insert delete κλειδωνει το map , επιτρεπει ομως σε πολλους να διαβαζουν ταυτοχρονα
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
	fmt.Printf("DEBUG JOIN: newSuccessor=%s predID=%s myID=%s\n", newSuccessor.Address, predID.String(), n.ID.String())

	// 5. Ζητάμε από τον successor τα keys που μας ανήκουν
	n.call(newSuccessor.Address, fmt.Sprintf("TRANSFER_KEYS %s,%s,%s", n.Address, n.ID.String(), predID.String()))

	fmt.Printf("Ο Successor είναι: %s\n", n.Successor.Address)
	return nil
}
func (n *Node) Depart() {
	if n.Successor != nil && n.Predecessor != nil && n.Successor.Address != n.Address {

		n.TableLock.RLock()
		transferData := make(map[string]string)
		for key, value := range n.DataTable {
			transferData[key] = value
		}
		n.TableLock.RUnlock() // Απελευθερώνουμε το lock ΠΡΙΝ τα network calls

		fmt.Printf("Μεταφορά %d εγγραφών στον Successor (%s)...\n", len(transferData), n.Successor.Address)

		// Συλλέγουμε τα replica keys πριν ενημερώσουμε pointers
		replicaKeys := []string{}
		replicaPrimaries := map[string]string{}

		for key, value := range transferData {
			hashedKey := hashString(key)
			primary, err := n.FindSuccessor(hashedKey)
			if err != nil {
				continue
			}

			if primary.Address == n.Address {
				// Είμαι PRIMARY → στέλνω TRANSFER στον successor (overwrite, όχι concat)
				_, err := n.call(n.Successor.Address, fmt.Sprintf("TRANSFER %s %s", key, value))
				if err != nil {
					fmt.Printf("Σφάλμα μεταφοράς του '%s'\n", key)
				} else {
					fmt.Printf("Το '%s' (primary) μεταφέρθηκε με επιτυχία!\n", key)
				}
			} else {
				// Είμαι REPLICA → θα ειδοποιήσουμε τον primary ΜΕΤΑ την ενημέρωση pointers
				if n.K > 1 {
					replicaKeys = append(replicaKeys, key)
					replicaPrimaries[key] = primary.Address
				}
			}
		}

		// Ενημέρωσε pointers ΠΡΙΝ το rebalance ώστε ο primary να βλέπει σωστό δακτύλιο
		n.call(n.Predecessor.Address, fmt.Sprintf("SET_SUCCESSOR %s,%s", n.Successor.Address, n.Successor.ID.String()))
		n.call(n.Successor.Address, fmt.Sprintf("SET_PREDECESSOR %s,%s", n.Predecessor.Address, n.Predecessor.ID.String()))

		// Τώρα ειδοποίησε τους primaries για rebalance
		for _, key := range replicaKeys {
			primaryAddr := replicaPrimaries[key]
			_, err := n.call(primaryAddr, fmt.Sprintf("REBALANCE_REPLICAS %s", key))
			if err != nil {
				fmt.Printf("Σφάλμα ειδοποίησης primary για '%s'\n", key)
			} else {
				fmt.Printf("Το '%s' (replica) → ειδοποιήθηκε ο primary %s\n", key, primaryAddr)
			}
		}
	}
	fmt.Println("Οι δείκτες ενημερώθηκαν. Ο κόμβος αποχωρεί.")
}
func (n *Node) RedistributeMyReplicas() {
	snapshot := n.StoreGetAll()

	for key, val := range snapshot {
		hashedKey := hashString(key)
		primary, err := n.FindSuccessor(hashedKey)
		if err != nil {
			continue
		}

		// Ελέγχουμε αν εμείς είμαστε ο Primary αυτού του κλειδιού
		if primary.Address == n.Address {
			// Στέλνουμε τα αντίγραφα στο νέο σετ κόμβων (που πλέον περιλαμβάνει τον νέο κόμβο)
			if n.Consistency == "eventual" {
				n.ReplicateEventual(key, val, false)
			} else if n.Consistency == "linear" {
				n.mu.RLock()
				next := n.Successor
				n.mu.RUnlock()
				n.call(next.Address, fmt.Sprintf("CHAIN_WRITE %d %s %s", n.K-1, key, val))
			}

			// Καθαρίζουμε τον παλιό replica που βγήκε εκτός του νέου K-set
			successors, err := n.GetSuccessors(n.Address, n.K+1)
			if err == nil && len(successors) > n.K {
				n.call(successors[n.K], fmt.Sprintf("REPLICA_DELETE %s", key))
			}
		}
	}
}
