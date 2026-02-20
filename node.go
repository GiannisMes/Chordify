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
	TableLock   sync.RWMutex // οταν καποιος παει να κανει insert delete κλειδωνει το map , επιτρεπει ομως σε πολλους να διαβαζουν ταυτοχρονα
}

// κραταει ουσιαστικα το id και adress mazi me to port του successor και predecessor
type NodeInfo struct {
	ID      *big.Int
	Address string
}

// Συνάρτηση για να φτιάχνουμε έναν νέο κόμβο
func NewNode(address string) *Node {
	return &Node{
		ID:          hashString(address), //συναρτηση απο  hash.go
		Address:     address,
		DataTable:   make(map[string]string),
		FingerTable: make([]*NodeInfo, 160), // θα μπορουσαμε και λιγοτερες θεσεις
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

	fmt.Printf(" Ο Successor  είναι: %s\n", n.Successor.Address)
	return nil
}

func (n *Node) Depart() {

	if n.Successor != nil && n.Predecessor != nil && n.Successor.Address != n.Address {
		// Ενημερώνουμε τον Predecessor μας να δείχνει πλέον ως Successor τον δικό μας Successor
		n.call(n.Predecessor.Address, fmt.Sprintf("SET_SUCCESSOR %s,%s", n.Successor.Address, n.Successor.ID.String()))

		// Ενημερώνουμε τον Successor μας να δείχνει πλέον ως Predecessor τον δικό μας Predecessor
		n.call(n.Successor.Address, fmt.Sprintf("SET_PREDECESSOR %s,%s", n.Predecessor.Address, n.Predecessor.ID.String()))
		//μεταφορα των κλειδιων του κομβου που φευγει στον successor του
		n.TableLock.RLock()
		fmt.Printf("Μεταφορά %d εγγραφών στον Successor (%s)...\n", len(n.DataTable), n.Successor.Address)

		for key, value := range n.DataTable {
			_, err := n.call(n.Successor.Address, fmt.Sprintf("INSERT %s %s", key, value))
			if err != nil {
				fmt.Printf("Σφάλμα μεταφοράς του '%s'\n", key)
			} else {
				fmt.Printf("Το '%s' μεταφέρθηκε με επιτυχία!\n", key)
			}
		}
		n.TableLock.RUnlock()
	}
	fmt.Println("Οι δείκτες ενημερώθηκαν. Ο κόμβος αποχωρεί.")
}
