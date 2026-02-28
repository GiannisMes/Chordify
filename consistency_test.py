import subprocess
import time
import sys
import socket
import os

def start_nodes(num_nodes, k, consistency):
    processes = []
    
    # Bootstrap κόμβος
    p = subprocess.Popen(
        ["go", "run", ".", "-port", "8000", "-k", str(k), "-consistency", consistency],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE
    )
    processes.append(p)
    print(f"Εκκίνηση bootstrap κόμβου 8000...")
    time.sleep(2)

    # Υπόλοιποι κόμβοι
    for i in range(1, num_nodes):
        port = 8000 + i
        p = subprocess.Popen(
            ["go", "run", ".", "-port", str(port), "-bootstrap", "127.0.0.1:8000", "-k", str(k), "-consistency", consistency],
            stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE
        )
        processes.append(p)
        print(f"Εκκίνηση κόμβου {port}...")
        time.sleep(1)

    print(f"\nΌλοι οι κόμβοι ξεκίνησαν. Περιμένουμε σταθεροποίηση...")
    time.sleep(5)
    return processes

def send_command(host, port, command):
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        s.settimeout(5)
        s.connect((host, port))
        s.sendall((command + "\n").encode())
        response = s.makefile().readline().strip()
        s.close()
        return response
    except Exception:
        return "ERROR"

def run_requests(host, port, input_file, output_file):
    if not os.path.exists(input_file):
        print(f"Σφάλμα: Το αρχείο {input_file} δεν βρέθηκε!")
        return

    with open(input_file, "r", encoding="utf-8") as f:
        lines = f.readlines()

    results = []
    for line in lines:
        line = line.strip()
        if not line: continue

        parts = [p.strip() for p in line.split(",")]
        command = parts[0].lower()

        if command == "insert" and len(parts) >= 3:
            key, value = parts[1].replace(" ", "_"), parts[2]
            resp = send_command(host, port, f"CLIENT_INSERT {key} {value}")
            results.append(f"INSERT {key} → {resp}")
            print(f"INSERT {key} → {resp}")

        elif command == "query" and len(parts) >= 2:
            key = parts[1].replace(" ", "_")
            resp = send_command(host, port, f"CLIENT_QUERY {key}")
            results.append(f"QUERY {key} → {resp}")
            print(f"QUERY {key} → {resp}")

    with open(output_file, "w", encoding="utf-8") as f:
        f.write("\n".join(results))

def stop_nodes(processes):
    print("\nΤερματισμός κόμβων...")
    for p in processes:
        p.terminate()

if __name__ == "__main__":
    num_nodes = int(sys.argv[1]) if len(sys.argv) > 1 else 5
    k = int(sys.argv[2]) if len(sys.argv) > 2 else 3
    consistency = sys.argv[3] if len(sys.argv) > 3 else "linear"
    
    # Paths διορθωμένα για τον φάκελο dataset
    input_file = os.path.join("dataset", "requests.txt")
    output_file = os.path.join("dataset", f"results_{consistency}.txt")

    procs = start_nodes(num_nodes, k, consistency)
    try:
        run_requests("127.0.0.1", 8000, input_file, output_file)
    finally:
        stop_nodes(procs)