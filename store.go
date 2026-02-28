package main

import (
	"fmt"
	"math/rand"
	"strings"
)

// ============================================================
// Τοπικές πράξεις — αγγίζουν απευθείας το DataTable
// Καλούνται από την  HandleConnection (server handler)
// ============================================================

func (n *Node) StorePut(key, value string) {
	n.TableLock.Lock()
	if existing, exists := n.DataTable[key]; exists {
		n.DataTable[key] = existing + "," + value //concatenate values αν το κλειδι υπαρχει
	} else {
		n.DataTable[key] = value
	}
	n.TableLock.Unlock()
}

//Map Lookup για το key

func (n *Node) StoreGet(key string) (string, bool) {
	n.TableLock.RLock()
	val, exists := n.DataTable[key]
	n.TableLock.RUnlock()
	return val, exists
}

func (n *Node) StoreDelete(key string) {
	n.TableLock.Lock()
	delete(n.DataTable, key)
	n.TableLock.Unlock()
}

func (n *Node) StoreGetAll() map[string]string {
	n.TableLock.RLock()
	snapshot := make(map[string]string, len(n.DataTable)) // snapshot — αντιγράφει όλο το DataTable σε νέο map και το επιστρέφει
	for k, v := range n.DataTable {
		snapshot[k] = v
	}
	n.TableLock.RUnlock()
	return snapshot
}

func (n *Node) PrintLocalTable() {
	snapshot := n.StoreGetAll()
	if len(snapshot) == 0 {
		fmt.Println("  (Άδειο)")
		return
	}
	for key, value := range snapshot {
		fmt.Printf("  - %s: %s\n", key, value)
	}
}

// ============================================================
// Κατανεμημένες πράξεις — κάνουν routing στο δίκτυο
// Καλούνται από το CLI
// ============================================================

func (n *Node) Insert(key, value string) (string, error) {
	hashedKey := hashString(key)
	succ, err := n.FindSuccessor(hashedKey)
	if err != nil {
		return "", fmt.Errorf("σφάλμα εύρεσης υπευθύνου: %v", err)
	}
	_, err = n.call(succ.Address, fmt.Sprintf("INSERT %s %s", key, value))
	if err != nil {
		return "", fmt.Errorf("σφάλμα αποθήκευσης στον %s: %v", succ.Address, err)
	}
	return succ.Address, nil
}

func (n *Node) Query(key string) (string, string, error) {

	if n.Consistency == "eventual" {
		// 1. Βρίσκουμε τον Primary
		hashedKey := hashString(key)
		primaryNode, err := n.FindSuccessor(hashedKey)
		if err != nil {
			return "", "", fmt.Errorf("σφάλμα εύρεσης υπευθύνου: %v", err)
		}

		// 2. Παίρνουμε Primary + Replicas
		replicas, err := n.GetSuccessors(primaryNode.Address, n.K)
		if err != nil || len(replicas) == 0 {
			replicas = []string{primaryNode.Address}
		}

		// 3. Διαλέγουμε τυχαία έναν
		randomIndex := rand.Intn(len(replicas))
		targetReplicaAddr := replicas[randomIndex]

		// 4. Αν έτυχε να διαλέξει εμάς, ψάχνουμε τοπικά
		if targetReplicaAddr == n.Address {
			val, exists := n.StoreGet(key)
			if exists {
				return val, n.Address, nil
			}
			return "NOT_FOUND", n.Address, nil
		}

		// 5. Αλλιώς στέλνουμε το query στον τυχαίο replica
		resp, err := n.call(targetReplicaAddr, fmt.Sprintf("QUERY %s", key))
		if err == nil {
			return resp, targetReplicaAddr, nil
		}
		return "NOT_FOUND", targetReplicaAddr, err
	}

	// === LINEAR CONSISTENCY ===

	hashedKey := hashString(key)
	succ, err := n.FindSuccessor(hashedKey)
	if err != nil {
		return "", "", fmt.Errorf("σφάλμα εύρεσης υπευθύνου: %v", err)
	}
	res, err := n.call(succ.Address, fmt.Sprintf("QUERY %s", key))
	if err != nil {
		return "", "", fmt.Errorf("σφάλμα δικτύου: %v", err)
	}
	return res, succ.Address, nil
}

func (n *Node) QueryAll() map[string]map[string]string {
	results := make(map[string]map[string]string)

	// Τα δικά μας
	results[n.Address] = n.StoreGetAll()

	// Περιήγηση στον δακτύλιο με visited για αποφυγή infinite loop
	visited := map[string]bool{n.Address: true}
	n.mu.RLock()
	currentAddr := n.Successor.Address
	n.mu.RUnlock()

	for !visited[currentAddr] {
		visited[currentAddr] = true

		res, err := n.call(currentAddr, "QUERY_ALL")
		if err != nil {
			break
		}

		nodeData := make(map[string]string)
		if res != "" {
			for _, p := range strings.Split(res, ";") {
				parts := strings.SplitN(p, ":", 2)
				if len(parts) == 2 {
					nodeData[parts[0]] = parts[1]
				}
			}
		}
		results[currentAddr] = nodeData

		next, err := n.call(currentAddr, "GET_SUCCESSOR")
		if err != nil {
			break
		}
		currentAddr = strings.TrimSpace(next)
	}

	return results
}

func (n *Node) Delete(key string) (string, error) {
	hashedKey := hashString(key)
	succ, err := n.FindSuccessor(hashedKey)
	if err != nil {
		return "", fmt.Errorf("σφάλμα εύρεσης υπευθύνου: %v", err)
	}
	_, err = n.call(succ.Address, "DELETE "+key)
	if err != nil {
		return "", fmt.Errorf("σφάλμα δικτύου: %v", err)
	}
	return succ.Address, nil
}
func (n *Node) ReplicateEventual(key, value string, isDelete bool) {
	// Βρίσκουμε τους k κόμβους
	successors, err := n.GetSuccessors(n.Address, n.K)
	if err != nil || len(successors) <= 1 {
		return
	}

	// 2. Προσπερνάμε τον εαυτό μας (i=0) και στέλνουμε στους υπόλοιπους
	for i := 1; i < len(successors); i++ {
		succAddr := successors[i]

		// 3. Asynchronous call
		go func(addr string) {
			if isDelete {
				n.call(addr, fmt.Sprintf("REPLICA_DELETE %s", key))
			} else {
				n.call(addr, fmt.Sprintf("REPLICA_INSERT %s %s", key, value))
			}
		}(succAddr)
	}
}
func (n *Node) StorePutOverwrite(key, value string) {
	n.TableLock.Lock()
	n.DataTable[key] = value
	n.TableLock.Unlock()
}
