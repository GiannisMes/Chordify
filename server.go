package main

import (
	"bufio"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

// StartServer ξεκινάει τον TCP server για να δέχεται συνδέσεις από άλλους κόμβους
func (n *Node) StartServer() {

	_, port, err := net.SplitHostPort(n.Address) //χωριζει το ip απο το port
	if err != nil {
		fmt.Println("Σφάλμα στην ανάλυση της διεύθυνσης:", err)
		return
	}

	ln, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Println("Σφάλμα στον server:", err)
		return
	}

	fmt.Printf("Ο Server του κόμβου ακούει στην πόρτα: %s\n", port)

	for {
		conn, err := ln.Accept() //ο κομβος περιμενει να αποδεχτει μια συνδεση απο αλλο κομβο
		if err != nil {
			fmt.Println("Σφάλμα σύνδεσης:", err)
			continue
		}
		go n.HandleConnection(conn) //δημιουργει μια νεα go routine για να χειριστει την συνδεση , ετσι μπορει να δεχεται πολλες συνδεσεις ταυτοχρονα(δεν της εξυπηρετει ο ιδιος ο κομβος)
	}

}
func (n *Node) HandleConnection(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second)) //αν ενας client δεν κλεισει σωστα την συνδεση go routine καθαριζει
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		request := scanner.Text()
		args := strings.Fields(request)
		if len(args) == 0 {
			continue
		}

		command := strings.ToUpper(args[0])
		var response string

		switch command {
		case "PING":
			response = "PONG"
		case "ID":
			response = n.ID.String()

		case "FIND_SUCCESSOR":
			if len(args) < 2 {
				response = "ERROR_BAD_FORMAT"
				break
			}
			targetID, _ := new(big.Int).SetString(args[1], 10)
			succ, err := n.FindSuccessor(targetID)
			if err != nil || succ == nil {
				response = "ERROR_NOT_FOUND"
				break
			}
			response = fmt.Sprintf("%s,%s", succ.Address, succ.ID.String())

		case "GET_PREDECESSOR":
			n.mu.RLock()
			pred := n.Predecessor
			n.mu.RUnlock()

			if pred != nil {
				response = fmt.Sprintf("%s,%s", pred.Address, pred.ID.String())
			} else {
				response = "NONE"
			}
		case "NOTIFY":

			if len(args) < 2 {
				continue
			}
			info := parseNodeInfo(args[1])
			if info != nil {
				n.Notify(info)
			} else {
				response = "ERROR PARSING NOTIFY"
			}
		case "SET_SUCCESSOR": //χερισμος depart
			info := parseNodeInfo(args[1])
			n.mu.Lock()
			n.Successor = info
			n.mu.Unlock()
			response = "OK"

		case "SET_PREDECESSOR": //χειρισμος depart
			if args[1] == "NONE" {
				n.mu.Lock()
				n.Predecessor = nil
				n.mu.Unlock()

			} else {
				info := parseNodeInfo(args[1])
				n.mu.Lock()
				n.Predecessor = info
				n.mu.Unlock()
			}
			response = "OK"

		case "INSERT":
			if len(args) >= 3 {
				key := args[1]
				val := strings.Join(args[2:], " ")
				n.StorePut(key, val) // Αποθήκευση στον primary κόμβο
				fullVal, _ := n.StoreGet(key)

				// replication trigger
				if n.K > 1 && n.Consistency == "eventual" {
					n.ReplicateEventual(key, fullVal, false)
				} else if n.K > 1 && n.Consistency == "linear" {
					n.mu.RLock()
					next := n.Successor
					n.mu.RUnlock()
					n.call(next.Address, fmt.Sprintf("CHAIN_WRITE %d %s %s", n.K-1, key, fullVal))
				}
				response = "OK"
			} else {
				response = "ERROR_BAD_INSERT"
			}
		case "REPLICA_INSERT": // Το δέχεται ένας replica κόμβος
			if len(args) >= 3 {
				key := args[1]
				val := strings.Join(args[2:], " ")
				n.StorePutOverwrite(key, val)
				response = "OK"
			} else {
				response = "ERROR_BAD_REPLICA_INSERT"
			}
		case "DELETE":
			if len(args) >= 2 {
				key := strings.Join(args[1:], " ")
				n.StoreDelete(key)

				if n.K > 1 && n.Consistency == "eventual" {
					n.ReplicateEventual(key, "", true)
				} else if n.K > 1 && n.Consistency == "linear" {
					n.mu.RLock()
					next := n.Successor
					n.mu.RUnlock()
					n.call(next.Address, fmt.Sprintf("CHAIN_DELETE %d %s %s", n.K-1, key, ""))
				}
				response = "OK"
			} else {
				response = "ERROR_BAD_DELETE"
			}
		case "REPLICA_DELETE": // Το δέχεται ένας replica κόμβος
			if len(args) >= 2 {
				key := strings.Join(args[1:], " ")
				n.StoreDelete(key)
				response = "OK"
			} else {
				response = "ERROR_BAD_REPLICA_DELETE"
			}
		case "CHAIN_WRITE":
			if len(args) >= 4 {
				var remaining int
				fmt.Sscanf(args[1], "%d", &remaining)
				key := args[2]
				val := strings.Join(args[3:], " ")
				n.StorePutOverwrite(key, val)
				if remaining > 1 {
					n.mu.RLock()
					next := n.Successor
					n.mu.RUnlock()
					resp, err := n.call(next.Address, fmt.Sprintf("CHAIN_WRITE %d %s %s", remaining-1, key, val))
					if err != nil || resp != "OK" {
						response = "ERROR_CHAIN_WRITE"
						break
					}
				}
				response = "OK"
			}

		case "CHAIN_READ":
			if len(args) >= 3 {
				var remaining int
				fmt.Sscanf(args[1], "%d", &remaining)
				key := strings.Join(args[2:], " ")
				if remaining <= 1 || n.Successor.Address == n.Address {
					// Είμαι ο tail, επιστρέφω τοπικά
					val, exists := n.StoreGet(key)
					if exists {
						response = val
					} else {
						response = "NOT_FOUND"
					}
				} else {
					n.mu.RLock()
					next := n.Successor
					n.mu.RUnlock()
					resp, err := n.call(next.Address, fmt.Sprintf("CHAIN_READ %d %s", remaining-1, key))
					if err != nil {
						response = "NOT_FOUND"
					} else {
						response = resp
					}
				}
			}
		case "CHAIN_DELETE":
			if len(args) >= 3 {
				var remaining int
				fmt.Sscanf(args[1], "%d", &remaining)
				key := strings.Join(args[2:], " ")

				// 1. Διαγράφουμε τοπικά το κλειδί
				n.StoreDelete(key)

				// 2. Αν υπάρχουν κι άλλοι κόμβοι στην αλυσίδα, προωθούμε το delete
				if remaining > 1 {
					n.mu.RLock()
					next := n.Successor
					n.mu.RUnlock()
					resp, err := n.call(next.Address, fmt.Sprintf("CHAIN_DELETE %d %s", remaining-1, key))
					if err != nil || resp != "OK" {
						response = "ERROR_CHAIN_DELETE"
						break
					}
				}
				response = "OK"
			} else {
				response = "ERROR_BAD_CHAIN_DELETE"
			}
		case "REBALANCE_REPLICAS":
			if len(args) >= 2 {
				key := strings.Join(args[1:], " ")
				val, exists := n.StoreGet(key)
				if exists && n.K > 1 {
					if n.Consistency == "eventual" {
						successors, err := n.GetSuccessors(n.Address, n.K)
						if err == nil && len(successors) == n.K {
							lastSucc := successors[n.K-1]
							n.call(lastSucc, fmt.Sprintf("REPLICA_INSERT %s %s", key, val))
						}
					} else if n.Consistency == "linear" {
						n.mu.RLock()
						next := n.Successor
						n.mu.RUnlock()
						n.call(next.Address, fmt.Sprintf("CHAIN_WRITE %d %s %s", n.K-1, key, val))
					}
				}
				response = "OK"
			}

		case "QUERY":
			if len(args) >= 2 {
				key := strings.Join(args[1:], " ")
				if n.Consistency == "linear" && n.K > 1 {
					resp, err := n.call(n.Successor.Address, fmt.Sprintf("CHAIN_READ %d %s", n.K-1, key))
					if err != nil {
						response = "NOT_FOUND"
					} else {
						response = resp
					}
				} else {
					val, exists := n.StoreGet(key)
					if exists {
						response = val
					} else {
						response = "NOT_FOUND"
					}
				}
			}
		case "QUERY_ALL":
			data := n.StoreGetAll()
			var pairs []string
			for k, v := range data {
				pairs = append(pairs, fmt.Sprintf("%s:%s", k, v))
			}
			response = strings.Join(pairs, ";")
		case "GET_SUCCESSOR":
			n.mu.RLock()
			response = n.Successor.Address
			n.mu.RUnlock()
		case "OVERLAY_INFO":
			n.mu.RLock()
			predAddr := "NONE"
			if n.Predecessor != nil {
				predAddr = n.Predecessor.Address
			}
			succAddr := n.Successor.Address
			n.mu.RUnlock()
			response = fmt.Sprintf("Pred: %s | Succ: %s", predAddr, succAddr)
		case "TRANSFER_KEYS":
			parts := strings.Split(args[1], ",")
			if len(parts) < 3 {
				response = "ERROR"
				break
			}
			newNode := &NodeInfo{Address: parts[0]}
			newNode.ID, _ = new(big.Int).SetString(parts[1], 10)
			predID, _ := new(big.Int).SetString(parts[2], 10)

			snapshot := n.StoreGetAll()
			for key, val := range snapshot {
				hashedKey := hashString(key)
				inRange := checkRange(hashedKey, predID, newNode.ID)

				if inRange {
					resp, err := n.call(newNode.Address, fmt.Sprintf("TRANSFER %s %s", key, val))

					if err == nil && resp == "OK" {
						n.StoreDelete(key)
						if n.K > 1 {
							successors, err := n.GetSuccessors(n.Address, n.K)
							if err == nil {
								for i := 1; i < len(successors); i++ {
									n.call(successors[i], fmt.Sprintf("REPLICA_DELETE %s", key))
								}
							}
						}
					}
				}
			}
			response = "OK"
		case "TRANSFER":
			// Ο κόμβος λαμβάνει key από departing/joining κόμβο
			if len(args) >= 3 {
				key := args[1]
				val := strings.Join(args[2:], " ")
				n.StorePutOverwrite(key, val) // overwrite, όχι concat
				// Trigger replication
				if n.K > 1 && n.Consistency == "eventual" {
					n.ReplicateEventual(key, val, false)
				} else if n.K > 1 && n.Consistency == "linear" {
					n.mu.RLock()
					next := n.Successor
					n.mu.RUnlock()
					n.call(next.Address, fmt.Sprintf("CHAIN_WRITE %d %s %s", n.K-1, key, val))
				}
				response = "OK"
			}
		case "CLIENT_INSERT": //χρηση για scripts__
			if len(args) >= 3 {
				key := args[1]
				val := strings.Join(args[2:], " ")
				// Εδώ καλούμε τη μέθοδο του store.go που κάνει το σωστό DHT routing
				_, err := n.Insert(key, val)
				if err != nil {
					response = "ERROR_ROUTING"
				} else {
					response = "OK"
				}
			}

		case "CLIENT_QUERY":
			if len(args) >= 2 {
				key := strings.Join(args[1:], " ")
				res, _, err := n.Query(key)
				if err != nil {
					response = "NOT_FOUND"
				} else {
					response = res
				}
			}
		default:
			response = "UNKNOWN_COMMAND"

		}

		// Στέλνουμε την απάντηση πίσω στον αποστολέα
		fmt.Fprintln(conn, response)
	}
}
