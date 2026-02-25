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
- `script.py`  — Python script για αυτόματη εκτέλεση requests από αρχείο

## Εκκίνηση
```bash
# Bootstrap κόμβος (χωρίς replication)
go run . -port 8000

# Bootstrap κόμβος με replication k=3 και eventual consistency
go run . -port 8000 -k 3 -consistency eventual

# Bootstrap κόμβος με replication k=3 και linearizability
go run . -port 8000 -k 3 -consistency linear

# Νέος κόμβος
go run . -port 8001 -bootstrap 127.0.0.1:8000 -k 3 -consistency eventual
go run . -port 8002 -bootstrap 127.0.0.1:8000 -k 3 -consistency eventual
# κ.ο.κ.
```

## Εκτέλεση script πειραμάτων
```bash
# Eventual consistency
py script.py 10 3 eventual requests.txt results_eventual.txt

# Linearizability
py script.py 10 3 linear requests.txt results_linear.txt
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

### Replication - Eventual Consistency
- Παράμετρος `-k` για replication factor (default k=1)
- Κάθε key αποθηκεύεται στον primary + k-1 επόμενους successors
- `INSERT` → αποθήκευση στον primary (με concat αν υπάρχει ήδη) + async `REPLICA_INSERT` στους k-1 successors
- `DELETE` → διαγραφή από primary + async `REPLICA_DELETE` στους k-1 successors
- `QUERY` → επιλογή **τυχαίου** κόμβου από τους k υπεύθυνους (primary + replicas) → πιθανό stale read

### Replication - Linearizability (Chain Replication)
- **Write path**: HEAD (primary) → replica1 → ... → TAIL (σειριακά, σύγχρονα)
- **Read path**: πάντα από τον TAIL → εγγυημένα fresh τιμές
- `INSERT` → primary αποθηκεύει + `CHAIN_WRITE` διαδίδει σειριακά στους k-1 successors
- `QUERY` → primary προωθεί `CHAIN_READ` μέχρι τον tail → tail επιστρέφει τιμή
- `DELETE` → ίδιο με write, διαγραφή διαδίδεται μέσω chain

### Depart με replication
- **Primary exit**: μεταφέρει keys στον νέο primary με `TRANSFER` → αυτόματο replication σε k-1 νέους κόμβους
- **Replica exit**: ενημερώνει pointers → στέλνει `REBALANCE_REPLICAS` στον primary → primary στέλνει νέο replica στον k-οστό successor

### TCP Commands (server)
- `PING`, `ID`, `FIND_SUCCESSOR`, `GET_PREDECESSOR`, `GET_SUCCESSOR`
- `NOTIFY`, `SET_SUCCESSOR`, `SET_PREDECESSOR`
- `INSERT`, `DELETE`, `QUERY`, `QUERY_ALL`
- `REPLICA_INSERT`, `REPLICA_DELETE`
- `REBALANCE_REPLICAS`
- `CHAIN_WRITE`, `CHAIN_READ`
- `TRANSFER`, `TRANSFER_KEYS`
- `CLIENT_INSERT`, `CLIENT_QUERY` — για χρήση από scripts (σωστό DHT routing)
- `OVERLAY_INFO`

## Αποτελέσματα πειράματος (requests.txt, k=3, 10 κόμβοι)

### Eventual Consistency
- Το `QUERY` επιλέγει **τυχαία** έναν από τους k υπεύθυνους κόμβους
- Πιθανό **stale read** αν το async replication δεν έχει ολοκληρωθεί
- Παράδειγμα ασυνέπειας:
```
INSERT Hey_Jude 1001 → OK
QUERY Hey_Jude → 1001       ← έπεσε στον primary
QUERY Hey_Jude → NOT_FOUND  ← έπεσε σε stale replica
```

### Linearizability
- Κάθε `QUERY` διαβάζει **πάντα** από τον tail → εγγυημένα fresh τιμές
- Ποτέ stale read, ποτέ NOT_FOUND μετά από επιτυχές INSERT
- Παράδειγμα:
```
INSERT Hey_Jude 1001 → OK
QUERY Hey_Jude → 1001  ← πάντα fresh
QUERY Hey_Jude → 1001  ← πάντα fresh
```

**Συμπέρασμα**: Το linearizability δίνει πάντα fresh τιμές σε βάρος latency (chain traversal), ενώ το eventual consistency είναι ταχύτερο αλλά με κίνδυνο stale reads.

## Μένει
- Throughput measurement (write/read throughput για k=1, k=3, k=5)
- UI (γραφικό περιβάλλον)
- Διόρθωση TRANSFER_KEYS κατά το Join νέου κόμβου (τα keys δεν μεταφέρονται σωστά στον νέο κόμβο)
- Επαλήθευση και διόρθωση replication κατά το Join νέου κόμβου (k-1 replicas για τα μεταφερθέντα keys)
