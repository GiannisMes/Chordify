package main

import (
	"flag"
	"fmt"
)

func main() {
	port := flag.String("port", "8000", "Η πόρτα του κόμβου")
	// Το -bootstrap ορίζει την IP:Port του σταθερού κόμβου για την είσοδο στο δίκτυο
	bootstrapAddr := flag.String("bootstrap", "", "Η διεύθυνση του bootstrap κόμβου")
	k := flag.Int("k", 1, "Replication factor") ///default replication factor 1
	consistency := flag.String("consistency", "eventual", "Μοντέλο συνέπειας (eventual / linear)")

	flag.Parse()

	//διευθυνση κομβου (local host)
	myAddress := "127.0.0.1:" + *port

	//δημιουργια κομβου με κληση της συναρτησης απο  node.go
	// Η NewNode καλεί αυτόματα τη hashString από το hash.go
	me := NewNode(myAddress, *k, *consistency)

	fmt.Printf("--- Chordify Node Started ---\n")
	fmt.Printf("Address: %s\n", me.Address)
	fmt.Printf("ID:      %s\n", me.ID.String())

	// 4. Έλεγχος αν ο κόμβος είναι ο Bootstrap ή αν πρέπει να συνδεθεί σε άλλον
	if *bootstrapAddr == "" {
		fmt.Println("Status:  Running as Bootstrap node.")
		// Εδώ ο Successor και ο Predecessor είναι ο εαυτός του στην αρχή
		me.Successor = &NodeInfo{ID: me.ID, Address: me.Address}
		me.Predecessor = &NodeInfo{ID: me.ID, Address: me.Address}
	} else {
		err := me.Join(*bootstrapAddr)
		if err != nil {
			fmt.Printf("Σφάλμα Join: %v\n", err)
		}

	}
	go me.Stabilize()
	go me.StartServer()
	go me.FixFingers()

	fmt.Println("Ο κόμβος είναι έτοιμος. Πάτα Ctrl+C για τερματισμό.")

	me.RunCLI()

}
