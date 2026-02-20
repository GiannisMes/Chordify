package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func (n *Node) RunCLI() {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Πληκτρολογήστε εντολή (π.χ. help, info, exit):")
	fmt.Print("> ")

	for scanner.Scan() {
		input := scanner.Text()
		args := strings.Fields(input)
		if len(args) == 0 {
			fmt.Print("> ")
			continue
		}

		command := strings.ToLower(args[0])

		switch command {
		case "help":
			fmt.Println("Διαθέσιμες εντολές: info, help, exit, ping <addr>, insert <key> <val>, query <key|*>, delete <key>, overlay")

		case "info":
			n.mu.RLock()
			succ := n.Successor
			pred := n.Predecessor
			n.mu.RUnlock()
			fmt.Printf("Address: %s\nID:      %s\n", n.Address, n.ID.String())
			if succ != nil {
				fmt.Printf("Successor:   %s\n", succ.Address)
			}
			if pred != nil {
				fmt.Printf("Predecessor: %s\n", pred.Address)
			}

		case "ping":
			if len(args) < 2 {
				fmt.Println("Χρήση: ping <address>")
				break
			}
			res, err := n.call(args[1], "PING")
			if err != nil {
				fmt.Printf("Σφάλμα: %v\n", err)
			} else {
				fmt.Printf("Απάντηση από %s: %s\n", args[1], res)
			}

		case "insert":
			if len(args) < 3 {
				fmt.Println("Χρήση: insert <key> <value>")
				break
			}
			key := args[1]
			value := strings.Join(args[2:], " ")
			addr, err := n.Insert(key, value)
			if err != nil {
				fmt.Printf("Σφάλμα: %v\n", err)
			} else {
				fmt.Printf("Επιτυχία! Το '%s' αποθηκεύτηκε στον κόμβο %s\n", key, addr)
			}

		case "query":
			if len(args) < 2 {
				fmt.Println("Χρήση: query <key|*>")
				break
			}
			key := args[1]
			if key == "*" {
				results := n.QueryAll()
				fmt.Printf("\n--- Κατανεμημένα Δεδομένα ---\n")
				for addr, data := range results {
					fmt.Printf("Κόμβος %s:\n", addr)
					if len(data) == 0 {
						fmt.Println("  (Άδειο)")
					} else {
						for k, v := range data {
							fmt.Printf("  - %s: %s\n", k, v)
						}
					}
				}
				fmt.Println("-----------------------------")
			} else {
				val, addr, err := n.Query(key)
				if err != nil {
					fmt.Printf("Σφάλμα: %v\n", err)
				} else if val == "NOT_FOUND" {
					fmt.Printf("Το '%s' δεν βρέθηκε.\n", key)
				} else {
					fmt.Printf("ΒΡΕΘΗΚΕ! Κόμβος: %s | Value: %s\n", addr, val)
				}
			}

		case "delete":
			if len(args) < 2 {
				fmt.Println("Χρήση: delete <key>")
				break
			}
			addr, err := n.Delete(args[1])
			if err != nil {
				fmt.Printf("Σφάλμα: %v\n", err)
			} else {
				fmt.Printf("Διαγράφηκε από τον κόμβο %s\n", addr)
			}

		case "overlay":
			fmt.Println("\n--- Δακτύλιος Chord ---")
			for _, line := range n.GetOverlay() {
				fmt.Println(line)
			}
			fmt.Println("-----------------------")

		case "exit":
			n.Depart()
			fmt.Println("Τερματισμός κόμβου...")
			os.Exit(0)

		default:
			fmt.Println("Άγνωστη εντολή. Γράψτε 'help' για βοήθεια.")
		}

		fmt.Print("> ")
	}
}
