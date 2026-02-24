# Chordify - Distributed Hash Table

Υλοποίηση DHT βασισμένη στο πρωτόκολλο Chord με finger tables.

## Δομή αρχείων
- `main.go`    — Startup, παράμετροι (-port, -bootstrap, -k, -consistency)
- `node.go`    — Struct, NewNode, Join, Depart
- `chord.go`   — FindSuccessor, ClosestPrecedingNode, Stabilize, FixFingers, Notify, GetOverlay, GetSuccessors
- `store.go`   — Τοπικές και κατανεμημένες πράξεις δεδομένων, ReplicateEventual
- `server.go`  — TCP Server, HandleConnection
- `network.go` — call, parseNodeInfo
- `cli.go`     — CLI interface
- `hash.go`    — SHA-1 hash function

## Εκκίνηση
```bash
# Bootstrap κόμβος (χωρίς replication)
go run . -port 8000

# Bootstrap κόμβος με replication k=3 και eventual consistency
go run . -port 8000 -k 3 -consistency eventual

# Νέος κόμβος
go run . -port 8001 -bootstrap 127.0.0.1:8000 -k 3 -consistency eventual
go run . -port 8002 -bootstrap 127.0.0.1:8000 -k 3 -consistency eventual
# κ.ο.κ.
```

## Εντολές CLI
- `info`                    — Πληροφορίες κόμβου (address, ID, successor, predecessor)
- `insert <key> <value>`    — Εισαγωγή δεδομένων (routing στον υπεύθυνο κόμβο)
- `query <key>`             — Αναζήτηση key
- `query *`                 — Όλα τα δεδομένα ανά κόμβο
- `delete <key>`            — Διαγραφή key
- `overlay`                 — Εμφάνιση δακτυλίου (pred/succ κάθε κόμβου)
- `exit`                    — Graceful αποχώρηση κόμβου

## Υλοποιημένα

### Core Chord
- SHA-1 hashing για node IDs και keys
- Finger tables (160 entries) με FixFingers
- Stabilize / Notify για αυτόματη συντήρηση δακτυλίου
- FindSuccessor με δρομολόγηση μέσω finger tables
- Join / Depart (graceful)

### Replication (eventual consistency)
- Παράμετρος `-k` για replication factor (default k=1)
- Κάθε key αποθηκεύεται στον primary + k-1 επόμενους successors
- `INSERT` → αποθήκευση στον primary + async `REPLICA_INSERT` στους k-1 successors
- `DELETE` → διαγραφή από primary + async `REPLICA_DELETE` στους k-1 successors
- `QUERY` → διαβάζει τοπικά αν ο κόμβος έχει το key (primary ή replica), αλλιώς routing στον primary

### Depart με replication
- **Primary exit**: καθαρίζει τα παλιά replicas (REPLICA_DELETE στους k-1 successors) → INSERT στον νέο primary → αυτόματο replication σε k-1 νέους κόμβους
- **Replica exit**: ενημερώνει πρώτα τους pointers → στέλνει REBALANCE_REPLICAS στον primary → primary στέλνει νέο replica στον k-οστό successor

### TCP Commands (server)
- `PING`, `ID`, `FIND_SUCCESSOR`, `GET_PREDECESSOR`, `GET_SUCCESSOR`
- `NOTIFY`, `SET_SUCCESSOR`, `SET_PREDECESSOR`
- `INSERT`, `DELETE`, `QUERY`, `QUERY_ALL`
- `REPLICA_INSERT`, `REPLICA_DELETE`
- `REBALANCE_REPLICAS`
- `OVERLAY_INFO`

## Μένει
- Linearizability consistency model (chain replication ή quorum)
- Throughput measurement (write/read throughput για k=1, k=3, k=5)
