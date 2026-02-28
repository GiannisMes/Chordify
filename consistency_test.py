import subprocess
import time
import sys
import socket

def start_nodes(num_nodes, k, consistency):
    processes = []
    
    # Bootstrap κόμβος
    p = subprocess.Popen(
        ["go", "run", ".", "-port", "8000", "-k", str(k), "-consistency", consistency],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE
    )
    processes.append(p)
    print(f"Εκκίνηση bootstrap κόμβου 8000...")
    time.sleep(2)

    # Υπόλοιποι κόμβοι
    for i in range(1, num_nodes):
        port = 8000 + i
        p = subprocess.Popen(
            ["go", "run", ".", "-port", str(port), "-bootstrap", "127.0.0.1:8000", "-k", str(k), "-consistency", consistency],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )
        processes.append(p)
        print(f"Εκκίνηση κόμβου {port}...")
        time.sleep(1)

    print(f"\nΌλοι οι κόμβοι ξεκίνησαν. Περιμένουμε σταθεροποίηση δακτυλίου...")
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
    except Exception as e:
        return f"ERROR: {e}"

def run_requests(host, port, input_file, output_file):
    with open(input_file, "r", encoding="utf-8") as f:
        lines = f.readlines()

    results = []
    for line in lines:
        line = line.strip()
        if not line:
            continue

        parts = [p.strip() for p in line.split(",")]
        command = parts[0].lower()

        if command == "insert" and len(parts) >= 3:
            key = parts[1].replace(" ", "_")
            value = parts[2]
            resp = send_command(host, port, f"CLIENT_INSERT {key} {value}")
            results.append(f"INSERT {key} {value} → {resp}")
            print(f"INSERT {key} {value} → {resp}")

        elif command == "query" and len(parts) >= 2:
            key = parts[1].replace(" ", "_")
            resp = send_command(host, port, f"CLIENT_QUERY {key}")
            results.append(f"QUERY {key} → {resp}")
            print(f"QUERY {key} → {resp}")

    with open(output_file, "w", encoding="utf-8") as f:
        f.write("\n".join(results))

    print(f"\nΑποτελέσματα αποθηκεύτηκαν στο {output_file}")

def stop_nodes(processes):
    print("\nΤερματισμός κόμβων...")
    for p in processes:
        p.terminate()

if __name__ == "__main__":
    num_nodes = int(sys.argv[1]) if len(sys.argv) > 1 else 5
    k = int(sys.argv[2]) if len(sys.argv) > 2 else 3
    consistency = sys.argv[3] if len(sys.argv) > 3 else "linear"
    input_file = sys.argv[4] if len(sys.argv) > 4 else "requests.txt"
    output_file = sys.argv[5] if len(sys.argv) > 5 else f"results_{consistency}.txt"

    processes = start_nodes(num_nodes, k, consistency)

    try:
        run_requests("127.0.0.1", 8000, input_file, output_file)
    finally:
        stop_nodes(processes)