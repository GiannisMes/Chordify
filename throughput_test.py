import time
import sys
import socket
import threading
import os

def send_command(host, port, command):
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(10)
        s.connect((host, port))
        s.sendall((command + "\n").encode())
        response = s.makefile().readline().strip()
        s.close()
        return response
    except Exception as e:
        return f"ERROR: {e}"

def worker(node_id, action_type, results_dict):
    port = 8000 + node_id
    filename = f"{action_type.lower()}_{node_id:02d}.txt"
    req_count = 0

    if not os.path.exists(filename):
        print(f"  [Προειδοποίηση] Το αρχείο {filename} δεν βρέθηκε!")
        results_dict[node_id] = 0
        return

    with open(filename, 'r', encoding='utf-8') as f:
        lines = f.readlines()

    for line in lines:
        line = line.strip()
        if not line:
            continue

        if action_type == "INSERT":
            parts = line.split(',') if ',' in line else line.split()
            key = parts[0].replace(" ", "_")
            value = parts[1] if len(parts) > 1 else "default_val"
            send_command("127.0.0.1", port, f"CLIENT_INSERT {key} {value}")
            req_count += 1

        elif action_type == "QUERY":
            key = line.split(',')[0].strip() if ',' in line else line.split()[0].strip()
            key = key.replace(" ", "_")
            send_command("127.0.0.1", port, f"CLIENT_QUERY {key}")
            req_count += 1

    results_dict[node_id] = req_count

def run_concurrent_test(num_nodes, action_type):
    threads = []
    results_dict = {}

    print(f"\n--- Έναρξη ταυτόχρονων {action_type}s σε {num_nodes} κόμβους ---")
    start_time = time.time()

    for i in range(num_nodes):
        t = threading.Thread(target=worker, args=(i, action_type, results_dict))
        threads.append(t)
        t.start()

    for t in threads:
        t.join()

    end_time = time.time()
    total_time = end_time - start_time
    total_requests = sum(results_dict.values())
    throughput = (total_requests / total_time) if total_time > 0 else 0

    print(f"Συνολικά Requests: {total_requests}")
    print(f"Συνολικός Χρόνος:  {total_time:.4f} δευτερόλεπτα")
    print(f"Throughput:        {throughput:.2f} requests/sec")

    return throughput

if __name__ == "__main__":
    num_nodes = 10
    k = int(sys.argv[1]) if len(sys.argv) > 1 else 3
    consistency = sys.argv[2] if len(sys.argv) > 2 else "eventual"

    print(f"=== ΠΕΙΡΑΜΑ: K={k} | CONSISTENCY={consistency.upper()} ===")
    print(f"(Οι κόμβοι τρέχουν ήδη σε Docker στα ports 8000-8009)")

    # 1. Write Throughput
    run_concurrent_test(num_nodes, "INSERT")

    print("\nΠεριμένουμε 3s για σταθεροποίηση replicas...")
    time.sleep(3)

    # 2. Read Throughput
    run_concurrent_test(num_nodes, "QUERY")