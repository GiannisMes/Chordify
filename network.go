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

func (n *Node) call(address string, message string) (string, error) {
	// 1. Άνοιγμα σύνδεσης (Socket) με τον άλλον κόμβο
	conn, err := net.DialTimeout("tcp", address, 10*time.Second) //αν ο κόμβος που καλούμε είναι κάτω ή μη προσβάσιμος, η σύνδεση αποτυγχάνει σε max 2s
	if err != nil {
		return "", err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second)) // αν η επικοινωνια αποτυχει

	// 2. Αποστολή του μηνύματος
	fmt.Fprintln(conn, message)

	// 3. Ανάγνωση της απάντησης
	response, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(response), nil
}

// parseNodeInfo: Μετατρέπει το string "IP:Port,ID" σε αντικείμενο NodeInfo
func parseNodeInfo(input string) *NodeInfo {
	input = strings.TrimSpace(input) // Καθαρίζει τυχόν κενά ή αλλαγές γραμμής
	parts := strings.Split(input, ",")

	if len(parts) < 2 {
		fmt.Printf("DEBUG: Έλαβα λάθος string: '%s'\n", input)
		return nil
	}

	id := new(big.Int)
	id, ok := id.SetString(parts[1], 10)
	if !ok {
		fmt.Printf("DEBUG: Το ID '%s' δεν είναι έγκυρος αριθμός\n", parts[1])
		return nil
	}

	return &NodeInfo{
		Address: parts[0],
		ID:      id,
	}
}
