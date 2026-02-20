package main

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

func (n *Node) FindSuccessor(id *big.Int) (*NodeInfo, error) {
	n.mu.RLock()
	succ := n.Successor
	n.mu.RUnlock()

	// Αν είμαστε μόνοι στο δίκτυο
	if succ.Address == n.Address {
		return succ, nil
	}

	// 1. Ελέγχουμε αν το id είναι ανάμεσα σε εμάς και τον άμεσο Successor μας.
	if checkRange(id, n.ID, succ.ID) {
		return succ, nil
	}

	// 2. Βρίσκουμε τον πιο κοντινό κόμβο πριν το id
	cpn := n.ClosestPrecedingNode(id)

	// 3. Προστασία από άπειρη λούπα
	if cpn.Address == n.Address {
		resp, err := n.call(succ.Address, "FIND_SUCCESSOR "+id.String())
		if err != nil {
			return nil, err
		}
		return parseNodeInfo(resp), nil
	}
	// 4. Εδώ γίνεται το "άλμα"!
	// Προωθούμε την αναζήτηση στον κόμβο που βρήκαμε από το Finger Table.
	resp, err := n.call(cpn.Address, "FIND_SUCCESSOR "+id.String())
	if err != nil {
		return nil, err
	}

	// Μετατρέπουμε την απάντηση ξανά σε NodeInfo
	return parseNodeInfo(resp), nil
}

// ClosestPrecedingNode: Ψάχνει στο Finger Table τον πιο κοντινό κόμβο πριν το id
func (n *Node) ClosestPrecedingNode(id *big.Int) *NodeInfo {
	n.mu.RLock()
	defer n.mu.RUnlock() // για να μην βαζω 160 RLock, ενα για καθε θεση του finger table

	for i := 159; i >= 0; i-- {
		finger := n.FingerTable[i]
		if finger != nil {
			fingerID := finger.ID
			if checkRange(fingerID, n.ID, id) && fingerID.Cmp(id) != 0 {
				return finger
			}
		}
	}
	return &NodeInfo{Address: n.Address, ID: n.ID}
}
func (n *Node) FixFingers() {
	next := 0
	// Υπολογίζουμε το μέγιστο μέγεθος του δακτυλίου (2^160) για το SHA-1
	m := new(big.Int).Exp(big.NewInt(2), big.NewInt(160), nil)

	for {
		time.Sleep(500 * time.Millisecond) // Ανανεώνει μία θέση κάθε μισό δευτερόλεπτο

		// 1. Υπολογίζουμε το "άλμα" που πρέπει να κάνουμε: 2^next
		offset := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(next)), nil)

		// 2. Το target είναι (Το_ID_μου + 2^next) modulo 2^160
		target := new(big.Int).Add(n.ID, offset)
		target.Mod(target, m)

		// 3. Βρίσκουμε τον Successor για αυτό το target ID
		succ, err := n.FindSuccessor(target)
		if err == nil && succ != nil {
			n.mu.Lock()
			n.FingerTable[next] = succ
			n.mu.Unlock()
		}

		// 4. Προχωράμε στην επόμενη θέση για τον επόμενο κύκλο
		next++
		if next >= 160 {
			next = 0 // Αν φτάσουμε στο τέλος (160), ξεκινάμε πάλι από την αρχή
		}
	}
}
func (n *Node) Notify(potential *NodeInfo) {
	if potential.Address == n.Address {
		return
	}

	n.mu.RLock()
	pred := n.Predecessor
	n.mu.RUnlock()

	if pred == nil || pred.Address == n.Address || checkRange(potential.ID, pred.ID, n.ID) {
		n.mu.Lock()
		n.Predecessor = potential
		n.mu.Unlock()
	}
}

func (n *Node) Stabilize() {
	for {
		time.Sleep(1 * time.Second)

		n.mu.RLock()
		succ := n.Successor
		n.mu.RUnlock()

		if succ == nil {
			continue
		}

		// Ρωτάμε τον επόμενο: "Ποιος είναι ο δικός σου predecessor;"
		resp, err := n.call(succ.Address, "GET_PREDECESSOR") // ← succ.Address
		if err == nil && resp != "NONE" && resp != "UNKNOWN_COMMAND" {
			x := parseNodeInfo(resp)
			if checkRange(x.ID, n.ID, succ.ID) { // ← succ.ID, δεν χρειάζεται 2ο RLock
				n.mu.Lock() // ← Lock γιατί γράφουμε
				n.Successor = x
				n.mu.Unlock()

				succ = x // ← ενημερώνουμε και την τοπική μεταβλητή
			}
		}

		// Ειδοποιούμε τον επόμενο ότι υπάρχουμε
		n.call(succ.Address, fmt.Sprintf("NOTIFY %s,%s", n.Address, n.ID.String())) // ← succ.Address
	}
}

// checkRange: Ελέγχει αν το id ανήκει στο διάστημα (start, end] κυκλικά
func checkRange(id, start, end *big.Int) bool {
	if start.Cmp(end) < 0 {
		// Κανονική σειρά (π.χ. από 10 έως 20)
		return id.Cmp(start) > 0 && id.Cmp(end) <= 0
	}
	// Ο κύκλος κάνει "γύρο" (π.χ. από 90 έως 10)
	return id.Cmp(start) > 0 || id.Cmp(end) <= 0
}

func (n *Node) GetOverlay() []string {
	// Μαζεύουμε όλους τους κόμβους με τα IDs τους
	type nodeEntry struct {
		addr string
		id   *big.Int
	}

	nodes := []nodeEntry{{addr: n.Address, id: n.ID}}
	visited := map[string]bool{n.Address: true}

	n.mu.RLock()
	curr := n.Successor.Address
	n.mu.RUnlock()

	for !visited[curr] && curr != "" {
		visited[curr] = true
		idStr, err := n.call(curr, "ID")
		if err != nil {
			break
		}
		id, _ := new(big.Int).SetString(idStr, 10)
		nodes = append(nodes, nodeEntry{addr: curr, id: id})

		next, err := n.call(curr, "GET_SUCCESSOR")
		if err != nil {
			break
		}
		curr = strings.TrimSpace(next)
	}

	// Βρίσκουμε τον κόμβο με το μικρότερο ID
	startIdx := 0
	for i, node := range nodes {
		if node.id.Cmp(nodes[startIdx].id) < 0 {
			startIdx = i
		}
	}

	// Τυπώνουμε με σειρά από εκεί
	var lines []string
	for i := 0; i < len(nodes); i++ {
		node := nodes[(startIdx+i)%len(nodes)]
		res, err := n.call(node.addr, "OVERLAY_INFO")
		if err != nil {
			break
		}
		lines = append(lines, fmt.Sprintf("%s -> %s", node.addr, res))
	}

	return lines
}

// αυτη ειναι για το replication factor, για να βρουμε τους k διαδοχικους successor
func (n *Node) GetSuccessors(startAddr string, k int) ([]string, error) {
	successors := []string{startAddr}

	currentAddr := startAddr
	for i := 1; i < k; i++ {
		next, err := n.call(currentAddr, "GET_SUCCESSOR")
		if err != nil {
			return nil, fmt.Errorf("σφάλμα εύρεσης successor: %v", err)
		}
		next = strings.TrimSpace(next)

		// Αν γυρίσαμε στον εαυτό μας, σταματάμε — λιγότεροι κόμβοι από k , εχω δηλαδη οσο ειναι το δικτυο
		if next == startAddr || next == n.Address {
			break
		}

		successors = append(successors, next)
		currentAddr = next
	}

	return successors, nil
}
