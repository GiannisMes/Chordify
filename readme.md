# Chordify — Distributed Hash Table in Go

A fully functional implementation of the **Chord DHT protocol** with finger tables, replication, and two consistency models: **Eventual Consistency** and **Linearizability** (Chain Replication).

## Why Go

Go's native concurrency primitives (goroutines, channels) map naturally to a distributed node model where each node handles multiple simultaneous TCP connections. Its low overhead for I/O-bound operations makes it well-suited for a system where every operation involves inter-node network communication.

## File Structure

| File | Description |
|---|---|
| `main.go` | Entry point — flag parsing (`-port`, `-bootstrap`, `-k`, `-consistency`, `-interactive`) |
| `node.go` | `Node` struct, `NewNode`, `Join`, `Depart`, `RedistributeMyReplicas` |
| `chord.go` | Chord routing: `FindSuccessor`, `ClosestPrecedingNode`, `Stabilize`, `FixFingers`, `Notify` |
| `store.go` | `StorePut`, `StoreGet`, `StoreDelete`, `Insert`, `Query`, `Delete`, `ReplicateEventual` |
| `server.go` | TCP server (`StartServer`, `HandleConnection`) and all TCP command handlers |
| `network.go` | Inter-node communication: `call`, `parseNodeInfo`, `GetSuccessors` |
| `cli.go` | Interactive CLI: info, insert, query, delete, overlay, exit |
| `hash.go` | SHA-1 hashing (`hashString`) |
| `throughput_test.py` | Concurrent stress-test across 10 nodes — measures requests/sec |
| `consistency_test.py` | Validates stale reads under eventual vs. linearizable consistency |
| `dockerfile` | Single-node image (Go build + minimal runtime) |
| `docker-compose.yml` | Orchestrates a 10-node cluster via env vars (`K_VAL`, `CONSISTENCY`) |

### TCP Protocol Commands (`server.go`)

- `PING`, `ID`, `FIND_SUCCESSOR`, `GET_PREDECESSOR`, `GET_SUCCESSOR`
- `NOTIFY`, `SET_SUCCESSOR`, `SET_PREDECESSOR`
- `INSERT`, `DELETE`, `QUERY`, `QUERY_ALL`
- `REPLICA_INSERT`, `REPLICA_DELETE`
- `REBALANCE_REPLICAS`
- `CHAIN_WRITE`, `CHAIN_READ`, `CHAIN_DELETE`
- `TRANSFER`, `TRANSFER_KEYS`
- `CLIENT_INSERT`, `CLIENT_QUERY`, `CLIENT_DELETE` — DHT-routed client operations
- `CLIENT_DEPART` — graceful node departure
- `OVERLAY_INFO`, `SYSTEM_INFO`

### Proxy HTTP API (`proxy/main.go`)

| Endpoint | Description |
|---|---|
| `GET /ping` | PING the bootstrap node |
| `GET /insert?key=&value=` | CLIENT_INSERT with Chord routing |
| `GET /query?key=` | CLIENT_QUERY with Chord routing |
| `GET /delete?key=` | CLIENT_DELETE with Chord routing |
| `GET /overlay` | Traverse the ring and return all node IDs |
| `GET /queryall` | QUERY_ALL from every node in the ring |
| `GET /sysinfo` | Returns K and active consistency model |
| `GET /depart?addr=` | Graceful node departure |
| `GET /run-throughput-test` | Execute `throughput_test.py` and stream output |

## Getting Started

### 1. Local (single nodes)

```bash
# Bootstrap node (no replication)
go run . -port 8000 -interactive

# Join with replication factor k=3 and eventual consistency
go run . -port 8001 -bootstrap 127.0.0.1:8000 -k 3 -consistency eventual -interactive
```

### 2. Full 10-node cluster (Docker Compose)

```powershell
# Example: K=3, Linearizability
$env:K_VAL="3"; $env:CONSISTENCY="linear"; docker-compose up --build
```

### 3. Web UI (Proxy Dashboard)

```powershell
cd proxy
go run .
```

Open **http://localhost:8080** in your browser.

| UI Element | Action |
|---|---|
| PING | Connect and detect the network |
| REFRESH | Update the DHT ring and stats |
| INSERT / QUERY / DELETE | Interact with the DHT |
| OVERLAY | Visualize the ring topology |
| KEYS | View all keys on a node (by address or NODE0/NODE1…) |
| THROUGHPUT | Run `throughput_test.py` directly from the UI |
| DEPART | Graceful node removal |

## CLI Commands

```
info                   — Node info (address, ID, successor, predecessor)
insert <key> <value>   — Insert key/value (DHT-routed to responsible node)
query <key>            — Lookup a key
query *                — Dump all keys across all nodes
delete <key>           — Remove a key
overlay                — Print the ring (predecessor/successor per node)
exit                   — Graceful departure
```

## Features

### Core Chord
- SHA-1 hashing for node IDs and keys
- 160-entry finger tables with periodic maintenance via `FixFingers`
- `Stabilize` and `Notify` for automatic ring maintenance
- `O(log N)` routing via `FindSuccessor`
- Smooth node join (`Join`) and departure (`Depart`) with automatic key transfer (`TRANSFER_KEYS`)

### Dynamic Membership (Join & Graceful Depart)

Maintaining correct replication under node churn is the core challenge. This system handles it fully:

- **Node Join:** The joining node takes over as primary for a key range from its successor. Keys are transferred automatically, and replicas are redistributed to maintain the configured replication factor K across the new successor list.
- **Graceful Depart:** On departure, a node actively transfers all its primary keys to its successor (`TRANSFER`), then triggers `REBALANCE_REPLICAS` so the successor rebuilds replicas at the K-th successor — keeping fault tolerance intact.

### Eventual Consistency
- Configurable replication factor via `-k` (default: 1)
- Each key is stored on the primary node and K-1 successors
- **Write:** Primary stores → async `REPLICA_INSERT` to K-1 successors
- **Read:** Random node chosen from the K responsible nodes (may return stale data)

### Linearizability (Chain Replication)
- **Write path:** HEAD (primary) → replica₁ → … → TAIL (sequential, synchronous)
- **Read path:** Always from the TAIL — guarantees fresh reads
- **Write:** Primary stores and propagates via `CHAIN_WRITE`
- **Read:** Forwarded via `CHAIN_READ` to the tail, which returns the authoritative value

## Experiments

### Throughput Test

```powershell
$env:K_VAL="3"; $env:CONSISTENCY="eventual"; docker-compose up --build
python throughput_test.py
```

Runs concurrent requests across all 10 nodes and reports aggregate requests/sec.

### Consistency Test

```bash
python consistency_test.py <num_nodes> <k> <mode>

# Example
python consistency_test.py 5 3 eventual
```

> **Note on stale reads with eventual consistency:** On localhost, network latency is near-zero, so stale reads may be hard to observe. To simulate realistic conditions, add a `time.Sleep(2 * time.Second)` inside `ReplicateEventual` in `node.go` — this creates a window where a QUERY can arrive before replication completes.

## Benchmark Results

Each value below is the average of three independent runs on a local Docker setup (10 nodes).

| K | Consistency | Avg Write Throughput (inserts/sec) | Avg Read Throughput (queries/sec) |
|:---:|:---:|:---:|:---:|
| **K = 1** | — | **491.98** | **480.15** |
| **K = 3** | Eventual | 433.82 | 426.32 |
| **K = 3** | Linear | 413.74 | 412.55 |
| **K = 5** | Eventual | 359.67 | 378.14 |
| **K = 5** | Linear | 383.96 | 379.35 |

**Key observations:**

1. **Replication penalty:** Baseline (K=1) achieves ~490 req/sec with zero replication overhead. Throughput decreases gradually as K increases (~360–380 req/sec at K=5), reflecting the expected cost of fault tolerance.

2. **Eventual vs. Linear at K=3:** Eventual outperforms Linear (433 vs. 413 writes/sec) because it returns immediately to the client and offloads replication to background goroutines, while Linear must wait for the full chain to acknowledge.

3. **Saturation at K=5:** Linear slightly outperforms Eventual (383 vs. 359 writes/sec) at K=5. On localhost, synchronous chain writes are fast. Eventual consistency spawns K-1=4 background goroutines per INSERT; under high load these accumulate, increasing goroutine scheduling overhead. The sequential nature of chain replication acts as natural backpressure, protecting throughput at high replication factors.
