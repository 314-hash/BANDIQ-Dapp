package main

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/report"
)

type registeredNode struct {
	NodeID          string    `json:"node_id"`
	OperatorWallet  string    `json:"operator_wallet"`
	PublicKeyHex    string    `json:"public_key_hex"`
	SoftwareVersion string    `json:"software_version"`
	RegisteredAt    time.Time `json:"registered_at"`
	LastSeen        time.Time `json:"last_seen"`
	LastSequence    uint64    `json:"last_sequence"`
}

type backendServer struct {
	mu           sync.RWMutex
	nodes        map[string]*registeredNode
	reports      []*report.MeasurementReport
	seenReportIDs map[string]bool
}

func newBackendServer() *backendServer {
	return &backendServer{
		nodes:         make(map[string]*registeredNode),
		reports:       make([]*report.MeasurementReport, 0),
		seenReportIDs: make(map[string]bool),
	}
}

func main() {
	port := flag.Int("port", 8080, "HTTP port to bind mock backend server")
	flag.Parse()

	server := newBackendServer()

	mux := http.NewServeMux()

	// Speedtest measurement endpoints
	mux.HandleFunc("/api/v1/ping", server.handlePing)
	mux.HandleFunc("/api/v1/speedtest/download", server.handleDownload)
	mux.HandleFunc("/api/v1/speedtest/upload", server.handleUpload)

	// Node registration and report ingestion
	mux.HandleFunc("/api/v1/nodes/register", server.handleRegisterNode)
	mux.HandleFunc("/api/v1/nodes", server.handleListNodes)
	mux.HandleFunc("/api/v1/reports/submit", server.handleSubmitReport)
	mux.HandleFunc("/api/v1/reports", server.handleListReports)

	// Simulated Sidra Chain RPC endpoint (Chain ID 97453 = 0x17cad)
	mux.HandleFunc("/rpc", server.handleSimulatedRPC)

	// Dashboard static file serving
	dashboardDir := "dashboard"
	if _, err := os.Stat(dashboardDir); err == nil {
		fs := http.FileServer(http.Dir(dashboardDir))
		mux.Handle("/dashboard/", http.StripPrefix("/dashboard/", fs))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/dashboard/", http.StatusFound)
				return
			}
			http.NotFound(w, r)
		})
	}

	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-BANDIQ-Node-ID, X-BANDIQ-Sequence, X-BANDIQ-Signature")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("[BANDIQ Mock Backend] Starting server on http://localhost:%d", *port)
	log.Printf("[BANDIQ Mock Backend] Web Dashboard available at http://localhost:%d/dashboard/", *port)
	log.Printf("[BANDIQ Mock Backend] Mobile LAN access: http://<LAN-IP>:%d/dashboard/", *port)
	log.Printf("[BANDIQ Mock Backend] Providing mock Sidra RPC on http://localhost:%d/rpc (Chain ID: 97453)", *port)

	if err := http.ListenAndServe(addr, corsHandler); err != nil {
		log.Fatalf("Mock backend failed: %v", err)
	}
}

func (s *backendServer) handlePing(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("pong"))
}

func (s *backendServer) handleDownload(w http.ResponseWriter, r *http.Request) {
	sizeMBStr := r.URL.Query().Get("size")
	sizeMB, _ := strconv.Atoi(sizeMBStr)
	if sizeMB <= 0 {
		sizeMB = 5
	}
	if sizeMB > 50 {
		sizeMB = 50 // cap for safety
	}

	totalBytes := sizeMB * 1024 * 1024
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(totalBytes))
	w.WriteHeader(http.StatusOK)

	chunk := make([]byte, 64*1024) // 64KB zero buffer
	written := 0
	for written < totalBytes {
		remaining := totalBytes - written
		toWrite := len(chunk)
		if remaining < toWrite {
			toWrite = remaining
		}
		n, err := w.Write(chunk[:toWrite])
		if err != nil {
			return
		}
		written += n
	}
}

func (s *backendServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	n, err := io.Copy(io.Discard, r.Body)
	if err != nil {
		http.Error(w, "failed reading body", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"bytes_sink": n,
	})
}

func (s *backendServer) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID          string    `json:"node_id"`
		OperatorWallet  string    `json:"operator_wallet"`
		PublicKeyHex    string    `json:"public_key_hex"`
		SoftwareVersion string    `json:"software_version"`
		RegisteredAt    time.Time `json:"registered_at"`
		Signature       string    `json:"signature"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid registration json", http.StatusBadRequest)
		return
	}

	if err := identity.ValidateWallet(req.OperatorWallet); err != nil {
		http.Error(w, "invalid operator wallet", http.StatusBadRequest)
		return
	}

	pubBytes, err := hex.DecodeString(req.PublicKeyHex)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		http.Error(w, "invalid public key hex", http.StatusBadRequest)
		return
	}

	// Verify registration signature
	expectedMsg := fmt.Sprintf("bandiq-register:%s:%s:%s", req.NodeID, req.OperatorWallet, req.RegisteredAt.Format(time.RFC3339))
	if !identity.VerifySignatureHex(req.PublicKeyHex, []byte(expectedMsg), req.Signature) {
		http.Error(w, "invalid registration signature", http.StatusUnauthorized)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.nodes[req.NodeID] = &registeredNode{
		NodeID:          req.NodeID,
		OperatorWallet:  req.OperatorWallet,
		PublicKeyHex:    req.PublicKeyHex,
		SoftwareVersion: req.SoftwareVersion,
		RegisteredAt:    req.RegisteredAt,
		LastSeen:        time.Now().UTC(),
		LastSequence:    0,
	}

	log.Printf("[Registration] Node registered: %s (Wallet: %s, Ver: %s)",
		req.NodeID, req.OperatorWallet, req.SoftwareVersion)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "registered",
		"node_id": req.NodeID,
	})
}

func (s *backendServer) handleSubmitReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rep report.MeasurementReport
	if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
		http.Error(w, fmt.Sprintf("invalid report payload: %v", err), http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Check if node is registered
	node, exists := s.nodes[rep.NodeID]
	if !exists {
		http.Error(w, "node not registered", http.StatusUnauthorized)
		return
	}

	// 2. Replay protection: check report ID
	if s.seenReportIDs[rep.ReportID] {
		http.Error(w, "replay detected: report ID already ingested", http.StatusConflict)
		return
	}

	// 3. Replay protection: check sequence monotonic ordering
	if rep.SequenceNumber <= node.LastSequence {
		http.Error(w, fmt.Sprintf("replay detected: sequence %d is not greater than last sequence %d",
			rep.SequenceNumber, node.LastSequence), http.StatusConflict)
		return
	}

	// 4. Validate timestamp freshness (max drift 5 minutes)
	if err := rep.Validate(5 * time.Minute); err != nil {
		http.Error(w, fmt.Sprintf("report validation failed: %v", err), http.StatusBadRequest)
		return
	}

	// 5. Verify cryptographic signature
	if err := rep.Verify(node.PublicKeyHex); err != nil {
		http.Error(w, fmt.Sprintf("signature verification failed: %v", err), http.StatusUnauthorized)
		return
	}

	// Ingestion passed all verification checks!
	node.LastSeen = time.Now().UTC()
	node.LastSequence = rep.SequenceNumber
	s.seenReportIDs[rep.ReportID] = true
	s.reports = append(s.reports, &rep)

	log.Printf("[Report Ingested] ID: %s | Node: %s | Seq: %d | Down: %.2f Mbps | Up: %.2f Mbps | Lat: %.2f ms | ActiveBytesIn: %d | ServedBytes: %d",
		rep.ReportID, rep.NodeID, rep.SequenceNumber,
		rep.Metrics.DownloadMbps, rep.Metrics.UploadMbps, rep.Metrics.LatencyMs,
		rep.Metrics.ActiveTestBytesIn, rep.Metrics.ServedBytesIn)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "accepted",
		"report_id":       rep.ReportID,
		"sequence_number": rep.SequenceNumber,
		"verified":        true,
	})
}

func (s *backendServer) handleListNodes(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*registeredNode
	for _, n := range s.nodes {
		list = append(list, n)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(list)
}

func (s *backendServer) handleListReports(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.reports)
}

func (s *backendServer) handleSimulatedRPC(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JSONRPC string        `json:"jsonrpc"`
		Method  string        `json:"method"`
		Params  []interface{} `json:"params"`
		ID      interface{}   `json:"id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json-rpc", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	switch req.Method {
	case "eth_chainId":
		// Return Sidra Chain ID 97453 in hex: 0x17cad
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  "0x17cad",
		})

	case "eth_call":
		// Mock 5,000,000 BNDQ balance (18 decimals)
		// 5000000 * 10^18 in hex = 0x446c3b15f99266800000
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  "0x00000000000000000000000000000000000000000000446c3b15f99266800000",
		})

	default:
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"result":  "0x0",
		})
	}
}
