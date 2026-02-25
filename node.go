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
	//γενικα το join tous enwnei pros ta mprosta
	resp, err := n.call(bootstrapAddr, "FIND_SUCCESSOR "+n.ID.String())
	if err != nil {
		return fmt.Errorf("σφάλμα επικοινωνίας με τον bootstrap: %v", err)
	}

	// 2. Μετατρέπουμε το string που λάβαμε (π.χ. "127.0.0.1:8000,ID") σε NodeInfo
	newSuccessor := parseNodeInfo(resp)
	if newSuccessor == nil {
		return fmt.Errorf("λάθος μορφή απάντησης από τον bootstrap")
	}

	// 3. Ορίζουμε  τον Successor μας
	n.Successor = newSuccessor
	// Ζητάμε από τον successor τα keys που μας ανήκουν
	n.call(newSuccessor.Address, fmt.Sprintf("TRANSFER_KEYS %s,%s", n.Address, n.ID.String()))

	fmt.Printf(" Ο Successor  είναι: %s\n", n.Successor.Address)
	return nil
}
func (n *Node) Depart() {

	if n.Successor != nil && n.Predecessor != nil && n.Successor.Address != n.Address {
		n.TableLock.RLock()
		fmt.Printf("Μεταφορά %d εγγραφών στον Successor (%s)...\n", len(n.DataTable), n.Successor.Address)

		// Συλλέγουμε τα replica keys πριν ενημερώσουμε pointers
		replicaKeys := []string{}
		replicaPrimaries := map[string]string{}

		for key, value := range n.DataTable {
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
		n.TableLock.RUnlock()

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
