# Chordify - Distributed Hash Table (Τελική Αναφορά)

Υλοποίηση κατανεμημένου συστήματος DHT βασισμένου στο πρωτόκολλο Chord με χρήση finger tables. Η εργασία υλοποιήθηκε στα πλαίσια του μαθήματος "Κατανεμημένα Συστήματα".

## Δομή Αρχείων
- `main.go`    — Startup, διαχείριση παραμέτρων (-port, -bootstrap, -k, -consistency)
- `node.go`    — Βασική δομή Node, NewNode, Join, Depart
- `chord.go`   — Συναρτήσεις δρομολόγησης: FindSuccessor, ClosestPrecedingNode, Stabilize, FixFingers, Notify
- `store.go`   — Διαχείριση τοπικών και κατανεμημένων δεδομένων, ReplicateEventual
- `server.go`  — TCP Server, HandleConnection
- `network.go` — Επικοινωνία δικτύου (call, parseNodeInfo)
- `cli.go`     — Διεπαφή γραμμής εντολών (CLI interface)
- `hash.go`    — Συνάρτηση κατακερματισμού SHA-1
- `throughput_test.py`  — Python script για αυτόματη εκτέλεση ταυτόχρονων requests στα πειράματα
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

## Εκκίνηση και Εκτέλεση

Το σύστημα υποστηρίζει τοπική εκτέλεση μέσω Go ή προσομοίωση δικτύου 10 κόμβων μέσω Docker.

### 1. Τοπική Εκτέλεση (Μεμονωμένοι Κόμβοι)
`bash
# Bootstrap κόμβος (χωρίς replication)
go run . -port 8000 -interactive

# Νέος κόμβος με replication k=3 και eventual consistency
go run . -port 8001 -bootstrap 127.0.0.1:8000 -k 3 -consistency eventual -interactive
`

### 2. Αυτοματοποιημένη Εκτέλεση 10 Κόμβων (Docker Compose)
Για την εκτέλεση των πειραμάτων, το σύστημα στήνεται γρήγορα μέσω Docker:
`powershell
# Παράδειγμα εκκίνησης με K=3 και Linearizability
$env:K_VAL="3"; $env:CONSISTENCY="linear"; docker-compose up --build
`

## Εντολές CLI
- `info`                    — Πληροφορίες κόμβου (address, ID, successor, predecessor)
- `insert <key> <value>`    — Εισαγωγή δεδομένων (routing στον υπεύθυνο κόμβο)
- `query <key>`             — Αναζήτηση key
- `query *`                 — Εμφάνιση όλων των δεδομένων ανά κόμβο
- `delete <key>`            — Διαγραφή key
- `overlay`                 — Εμφάνιση του δακτυλίου (predecessor/successor κάθε κόμβου)
- `exit`                    — Graceful αποχώρηση κόμβου

## Υλοποιημένα Χαρακτηριστικά

### Core Chord
- SHA-1 hashing για την παραγωγή node IDs και keys.
- Finger tables (160 entries) με περιοδική συντήρηση μέσω της `FixFingers`.
- Συναρτήσεις `Stabilize` και `Notify` για την αυτόματη συντήρηση του δακτυλίου.
- `FindSuccessor` με δρομολόγηση `O(log N)` μέσω finger tables.
- Ομαλή εισαγωγή (`Join`) και αποχώρηση (`Depart`) κόμβων με αυτόματη μεταφορά κλειδιών (`TRANSFER_KEYS`).

### Διαχείριση Δυναμικού Δικτύου (Node Join & Graceful Depart)
Η δυσκολία σε ένα σύστημα με replication είναι η διατήρηση των αντιγράφων όταν αλλάζει η τοπολογία (Node Churn). Το σύστημα το αντιμετωπίζει πλήρως:
- **Node Join:** Όταν ένας νέος κόμβος εισέρχεται στο δίκτυο, αναλαμβάνει τον ρόλο του primary για ένα συγκεκριμένο εύρος κλειδιών του successor του. Τα κλειδιά αυτά μεταφέρονται αυτόματα στον νέο κόμβο (`TRANSFER_KEYS`). Ταυτόχρονα, το σύστημα φροντίζει να ανανεώσει τα replicas ώστε τα μεταφερθέντα κλειδιά να διατηρήσουν το σωστό replication factor ($K$) στους νέους successors.
- **Graceful Depart:** Κατά την ομαλή αποχώρηση ενός κόμβου, ο κόμβος δεν τερματίζει απλά τη λειτουργία του χάνοντας δεδομένα. Αντίθετα, μεταφέρει ενεργά όλα τα primary κλειδιά του στον successor του (`TRANSFER`). Επιπλέον, ειδοποιεί το δίκτυο να κάνει ανακατανομή (`REBALANCE_REPLICAS`), εξασφαλίζοντας ότι ο διάδοχος θα δημιουργήσει νέα αντίγραφα στον $k$-οστό successor, διατηρώντας το επίπεδο Fault Tolerance ανέπαφο.

### Replication - Eventual Consistency
- Παράμετρος `-k` για τον καθορισμό του replication factor (default k=1).
- Κάθε key αποθηκεύεται στον primary κόμβο και στους k-1 επόμενους successors.
- `INSERT` → αποθήκευση στον primary + ασύγχρονη κλήση (`REPLICA_INSERT`) στους k-1 successors.
- `QUERY` → επιλογή **τυχαίου** κόμβου από τους k υπεύθυνους (primary + replicas). Ενδέχεται να επιστρέψει stale data.

### Replication - Linearizability (Chain Replication)
- **Write path**: HEAD (primary) → replica1 → ... → TAIL (σειριακά και σύγχρονα).
- **Read path**: Η ανάγνωση γίνεται πάντα από τον TAIL, εγγυώντας απόλυτα fresh τιμές.
- `INSERT` → Ο primary αποθηκεύει και προωθεί το αίτημα μέσω `CHAIN_WRITE` στους successors.
- `QUERY` → Το αίτημα προωθείται μέσω `CHAIN_READ` μέχρι τον tail, ο οποίος επιστρέφει την τελική τιμή.

---

###  Εκτέλεση Πειραμάτων

Το σύστημα συνοδεύεται από δύο σενάρια (scripts) δοκιμών, το καθένα βελτιστοποιημένο για διαφορετικό σκοπό:

#### 1. Έλεγχος Throughput (Επιδόσεις)
Χρησιμοποιείται για τη μέτρηση της μέγιστης απόδοσης του συστήματος σε σταθερό περιβάλλον.
* **Προετοιμασία:** Ξεκινήστε το δίκτυο με την εντολή: `$env:K_VAL="3"; $env:CONSISTENCY="eventual"; docker-compose up --build` (PowerShell) ή τις αντίστοιχες εντολές για το OS σας.
* **Εκτέλεση:** Τρέξτε την εντολή: `python throughput_test.py`
* **Λεπτομέρειες:** Το script συνδέεται ταυτόχρονα σε 10 containers (ports 8000-8009) και υπολογίζει τα συνολικά Requests ανά δευτερόλεπτο.

#### 2. Έλεγχος Consistency (Θεωρία & Ορθότητα)
Χρησιμοποιείται για την επαλήθευση των μοντέλων συνέπειας (Linearizability vs Eventual Consistency).
* **Προετοιμασία:** Βεβαιωθείτε ότι το Docker είναι απενεργοποιημένο (`docker-compose down`).
* **Εκτέλεση:** `python consistency_test.py [πλήθος_κόμβων] [k] [mode]`
    * Παράδειγμα: `python consistency_test.py 5 3 eventual`
* **Λεπτομέρειες:** Το script διαχειρίζεται τοπικά τη λειτουργία των κόμβων. 

> **Σημαντική Σημείωση για το Eventual Consistency:** > Λόγω της πολύ υψηλής ταχύτητας επικοινωνίας σε τοπικό επίπεδο (localhost), τα "stale reads" (περιπτώσεις όπου το Query επιστρέφει "Not Found" επειδή ο replica δεν έχει ενημερωθεί ακόμα) μπορεί να μην είναι εμφανή. Για να προσομοιώσετε ρεαλιστικές συνθήκες δικτυακής καθυστέρησης και να παρατηρήσετε το φαινόμενο, προτείνεται η προσθήκη μιας εντολής καθυστέρησης (π.χ. `time.Sleep(2 * time.Second)`) συγκεκριμένα στη συνάρτηση `ReplicateEventual` στο αρχείο `node.go`. Αυτό θα δώσει το απαραίτητο "παράθυρο" χρόνου στο script να εκτελέσει το Query πριν ολοκληρωθεί η αναπαραγωγή των δεδομένων.

## Αποτελέσματα Πειραμάτων

### Μέρος 1: Ορθότητα Δεδομένων (Stale vs Fresh Reads)
Σε πειράματα λειτουργικότητας, παρατηρήθηκαν οι εξής συμπεριφορές:

* **Eventual Consistency:** Το `QUERY` επιλέγει τυχαία έναν από τους k υπεύθυνους. Υπάρχει κίνδυνος **stale read** αν το ασύγχρονο replication δεν έχει προλάβει να ολοκληρωθεί.
  * *Παράδειγμα:* Μετά από `INSERT Hey_Jude 1001`, ένα άμεσο `QUERY Hey_Jude` μπορεί να επιστρέψει `1001` (αν ρωτηθεί ο primary) ή `NOT_FOUND` (αν ρωτηθεί καθυστερημένο replica).
* **Linearizability:** Κάθε `QUERY` διαβάζει υποχρεωτικά από τον tail. Ποτέ δεν εμφανίζεται stale read ή `NOT_FOUND` μετά από επιτυχές INSERT. Το μοντέλο εγγυάται ισχυρή συνέπεια με κόστος στο latency λόγω του chain traversal.

### Μέρος 2: Μετρήσεις Απόδοσης (Throughput Analysis)
Πραγματοποιήθηκε stress-test στο σύστημα με τη χρήση Python script που εκτελούσε ταυτόχρονα requests σε 10 κόμβους (βάσει της εκφώνησης). Οι μετρήσεις λήφθηκαν τοπικά μέσω Docker . Κάθε τιμή στον παρακάτω πίνακα αποτελεί τον μέσο όρο τριών ανεξάρτητων μετρήσεων.

| K (Replication) | Consistency Model | Μέσο Write Throughput (Inserts/sec) | Μέσο Read Throughput (Queries/sec) |
| :---: | :---: | :---: | :---: |
| **K = 1** | - | **491.98** | **480.15** |
| **K = 3** | Eventual | 433.82 | 426.32 |
| **K = 3** | Linear | 413.74 | 412.55 |
| **K = 5** | Eventual | 359.67 | 378.14 |
| **K = 5** | Linear | 383.96 | 379.35 |

**Συμπεράσματα Πειράματος:**

1. **Το Κόστος του Replication (Replication Penalty):** Η μέγιστη απόδοση επιτυγχάνεται στο baseline ($K=1$) με ~490 req/sec, καθώς απουσιάζει το overhead της αντιγραφής. Όσο αυξάνεται το replication factor (σε $K=3$ και $K=5$), το throughput μειώνεται σταδιακά (~360-380 req/sec), αναδεικνύοντας το αναμενόμενο κόστος του Fault Tolerance.

2. **Eventual vs Linearizability ($K=3$):** Σε λογικό φόρτο ($K=3$), το Eventual Consistency (433.82 writes/sec) υπερτερεί έναντι του Linear (413.74 writes/sec). Αυτό οφείλεται στην ικανότητα του Eventual να επιστρέφει το αποτέλεσμα στον client άμεσα, αναθέτοντας την ενημέρωση των αντιγράφων σε background goroutines, εν αντιθέσει με το Linear που αναμένει την ολοκλήρωση της chain replication αλυσίδας.

3. **Το Σημείο Κορεσμού  ($K=5$):** Στο  σενάριο του $K=5$, παρατηρήθηκε ότι το Linear (383.96 writes/sec) διατήρησε καλύτερη απόδοση από το Eventual (359.67 writes/sec). Επειδή το πείραμα διεξήχθη σε τοπικό περιβάλλον (Localhost), το network latency ήταν πρακτικά μηδενικό, επιτρέποντας στο Linearizability να εκτελεστεί σειριακά με εξαιρετική ταχύτητα. Αντίθετα, στο Eventual Consistency, η ταυτόχρονη εκκίνηση χιλιάδων background goroutines (λόγω των παράλληλων ταυτόχρονων requests σε 10 κόμβους) οδήγησε τον επεξεργαστή σε σημείο κορεσμού (CPU Saturation / Thrashing). Σε αυτή την περίπτωση, η σειριακή φύση του Linear λειτούργησε ως φυσικός ρυθμιστής φόρτου (backpressure), προστατεύοντας το σύστημα από την υπερφόρτωση.

## Μελλοντικές Προσθήκες (To-Do)
- Δημιουργία Γραφικού Περιβάλλοντος (Web UI) για την οπτικοποίηση του δακτυλίου και την εύκολη αλληλεπίδραση των χρηστών με τους κόμβους.