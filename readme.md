# Chordify - Distributed Hash Table

Υλοποίηση DHT βασισμένη στο πρωτόκολλο Chord με finger tables.

## Δομή αρχείων
- `main.go`    — Startup, παράμετροι
- `node.go`    — Struct, NewNode, Join, Depart
- `chord.go`   — FindSuccessor, ClosestPrecedingNode, Stabilize, FixFingers, Notify, GetOverlay
- `store.go`   — Τοπικές και κατανεμημένες πράξεις δεδομένων
- `server.go`  — TCP Server, HandleConnection
- `network.go` — call, parseNodeInfo
- `cli.go`     — CLI interface
- `hash.go`    — SHA-1 hash function

## Εκκίνηση
# Bootstrap κόμβος
go run . -port 8000

# Νέος κόμβος
go run . -port 8001 -bootstrap 127.0.0.1:8000 etc 8002 ,8003 αν θες να βαλεις κ αλλους

## Εντολές CLI
- `info`                    — Πληροφορίες κόμβου
- `insert <key> <value>`    — Εισαγωγή δεδομένων
- `query <key>`             — Αναζήτηση
- `query *`                 — Όλα τα δεδομένα
- `delete <key>`            — Διαγραφή
- `overlay`                 — Εμφάνιση δακτυλίου
- `exit`                    — Αποχώρηση κόμβου  , τα δεδομενα του μεταφερονται στον successor

## Μένει
- Replication (k=1, k=3, k=5) ,εχω γραψει την συναρτηση GetSuccessors που βρισκει τους επομενους k successors θα χρησιμοποιηθει δλδ για το replication 
- Consistency models (eventual / linearizability)
- Throughput measurement