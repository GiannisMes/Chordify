package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const bootstrapAddr = "127.0.0.1:8000"

// toLocal μετατρέπει τις διευθύνσεις του Docker (nodeX:800X) σε localhost
func toLocal(addr string) string {
	if addr == "" || addr == "NONE" {
		return addr
	}
	parts := strings.Split(addr, ":")
	if len(parts) == 2 {
		return "127.0.0.1:" + parts[1]
	}
	return addr
}
func tcpCall(addr, cmd string) string {
	localAddr := toLocal(addr)
	conn, err := net.DialTimeout("tcp", localAddr, 2*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	fmt.Fprintln(conn, cmd)
	resp, _ := bufio.NewReader(conn).ReadString('\n')
	return strings.TrimSpace(resp)
}

func corsHeader(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "application/json")
}

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "chordify-ui.html")
	})

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		res := tcpCall(bootstrapAddr, "PING")
		if res == "PONG" {
			json.NewEncoder(w).Encode(map[string]string{"node": bootstrapAddr, "status": "OK"})
		} else {
			http.Error(w, `{"error": "Offline"}`, 500)
		}
	})

	http.HandleFunc("/insert", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		key := r.URL.Query().Get("key")
		val := r.URL.Query().Get("value")
		res := tcpCall(bootstrapAddr, fmt.Sprintf("CLIENT_INSERT %s %s", key, val))
		json.NewEncoder(w).Encode(map[string]string{"result": res})
	})

	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		key := r.URL.Query().Get("key")
		res := tcpCall(bootstrapAddr, fmt.Sprintf("CLIENT_QUERY %s", key))
		json.NewEncoder(w).Encode(map[string]string{"result": res})
	})

	http.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		key := r.URL.Query().Get("key")
		res := tcpCall(bootstrapAddr, fmt.Sprintf("CLIENT_DELETE %s", key))
		json.NewEncoder(w).Encode(map[string]string{"result": res})
	})

	http.HandleFunc("/overlay", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		curr := bootstrapAddr
		visited := make(map[string]bool)
		var overlay []map[string]string

		for !visited[curr] && curr != "" {
			visited[curr] = true

			// 1. Ρωτάει τον κόμβο (αν δεν απαντήσει, η αλυσίδα σπάει, όπως στο CLI)
			id := tcpCall(curr, "ID")
			if id == "" {
				break
			}

			// 2. Παίρνει πληροφορίες
			info := tcpCall(curr, "OVERLAY_INFO")
			pred, succ := "NONE", "NONE"
			parts := strings.Split(info, "|")
			if len(parts) == 2 {
				pRaw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[0]), "Pred:"))
				sRaw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[1]), "Succ:"))
				pred = toLocal(pRaw)
				succ = toLocal(sRaw)
			}

			overlay = append(overlay, map[string]string{
				"address":     curr,
				"id":          id,
				"predecessor": pred,
				"successor":   succ,
			})

			// 3. Πάει στον επόμενο ακριβώς όπως κάνει το CLI
			nextSucc := tcpCall(curr, "GET_SUCCESSOR")
			curr = toLocal(strings.TrimSpace(nextSucc))
		}
		json.NewEncoder(w).Encode(overlay)
	})

	http.HandleFunc("/queryall", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		curr := bootstrapAddr
		visited := make(map[string]bool)
		allData := make(map[string]map[string]string)

		for !visited[curr] && curr != "" {
			visited[curr] = true

			// 1. ΕΛΕΓΧΟΣ ΖΩΗΣ: Ρωτάμε το ID. Αν δεν απαντήσει, είναι νεκρός!
			if tcpCall(curr, "ID") == "" {
				break
			}

			// 2. Παίρνουμε τα κλειδιά. Αν είναι κενό, απλά δεν έχει κλειδιά!
			keysStr := tcpCall(curr, "QUERY_ALL")
			nodeKeys := make(map[string]string)

			if keysStr != "" {
				pairs := strings.Split(keysStr, ";")
				for _, p := range pairs {
					kvs := strings.SplitN(p, ":", 2)
					if len(kvs) == 2 {
						nodeKeys[kvs[0]] = kvs[1]
					}
				}
			}
			allData[curr] = nodeKeys

			// 3. Πάμε στον επόμενο
			nextSucc := tcpCall(curr, "GET_SUCCESSOR")
			curr = toLocal(strings.TrimSpace(nextSucc))
		}
		json.NewEncoder(w).Encode(allData)
	})

	http.HandleFunc("/sysinfo", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		res := tcpCall(bootstrapAddr, "SYSTEM_INFO")
		parts := strings.Split(res, "|")
		if len(parts) == 2 {
			json.NewEncoder(w).Encode(map[string]string{"k": parts[0], "consistency": parts[1]})
		} else {
			http.Error(w, `{"error": "Failed"}`, 500)
		}
	})

	http.HandleFunc("/depart", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		nodeAddr := toLocal(r.URL.Query().Get("addr"))
		res := tcpCall(nodeAddr, "CLIENT_DEPART")
		json.NewEncoder(w).Encode(map[string]string{"result": res})
	})

	http.HandleFunc("/run-throughput-test", func(w http.ResponseWriter, r *http.Request) {
		corsHeader(w)
		scriptDir, _ := filepath.Abs("..")
		cmd := exec.Command("py", filepath.Join(scriptDir, "throughput_test.py"))
		cmd.Dir = scriptDir
		cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
		out, err := cmd.CombinedOutput()
		if err != nil {
			json.NewEncoder(w).Encode(map[string]string{"status": "error", "output": string(out) + "\nError: " + err.Error()})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "output": string(out)})
	})

	fmt.Println(" Ο Proxy Server ξεκίνησε στο http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
