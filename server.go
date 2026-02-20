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
				n.StorePut(args[1], strings.Join(args[2:], " "))
				response = "OK"
			} else {
				response = "ERROR_BAD_INSERT"
			}

		case "QUERY":
			if len(args) == 2 {
				val, exists := n.StoreGet(args[1])
				if exists {
					response = val
				} else {
					response = "NOT_FOUND"
				}
			} else {
				response = "ERROR_BAD_QUERY"
			}
		case "DELETE":
			if len(args) == 2 {
				n.StoreDelete(args[1])
				response = "OK"
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
		default:
			response = "UNKNOWN_COMMAND"

		}

		// Στέλνουμε την απάντηση πίσω στον αποστολέα
		fmt.Fprintln(conn, response)
	}
}
