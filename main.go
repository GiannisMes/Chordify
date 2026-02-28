package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	port := flag.String("port", "8000", "Η πόρτα του κόμβου")
	addr := flag.String("addr", "127.0.0.1", "Η IP/Όνομα του κόμβου")
	bootstrapAddr := flag.String("bootstrap", "", "Η διεύθυνση του bootstrap κόμβου")
	k := flag.Int("k", 1, "Replication factor")
	consistency := flag.String("consistency", "eventual", "Μοντέλο συνέπειας (eventual / linear)")
	interactive := flag.Bool("interactive", false, "Ενεργοποίηση CLI (μόνο για local χρήση)")

	flag.Parse()

	myAddress := *addr + ":" + *port

	me := NewNode(myAddress, *k, *consistency)

	fmt.Printf("--- Chordify Node Started ---\n")
	fmt.Printf("Address: %s\n", me.Address)
	fmt.Printf("ID:      %s\n", me.ID.String())

	go me.StartServer()
	time.Sleep(500 * time.Millisecond)

	if *bootstrapAddr == "" {
		fmt.Println("Status:  Running as Bootstrap node.")
		me.Successor = &NodeInfo{ID: me.ID, Address: me.Address}
		me.Predecessor = &NodeInfo{ID: me.ID, Address: me.Address}
	} else {
		var err error
		for i := 0; i < 10; i++ {
			err = me.Join(*bootstrapAddr)
			if err == nil {
				break
			}
			fmt.Printf("Join αποτυχία (απόπειρα %d/10): %v — επανάληψη σε 1s...\n", i+1, err)
			time.Sleep(1 * time.Second)
		}
		if err != nil {
			fmt.Printf("Σφάλμα Join μετά από 10 απόπειρες: %v\n", err)
			os.Exit(1)
		}
	}

	go me.Stabilize()
	go me.FixFingers()

	fmt.Println("Ο κόμβος είναι έτοιμος.")

	if *interactive {
		me.RunCLI()
	} else {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		fmt.Println("Λαμβάνω σήμα τερματισμού, αποχωρώ gracefully...")
		me.Depart()
	}
}
