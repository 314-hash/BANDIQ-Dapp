package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"bandiq-node-agent/internal/config"
	"bandiq-node-agent/internal/consent"
	"bandiq-node-agent/internal/report"
	"bandiq-node-agent/internal/spool"
	"bandiq-node-agent/internal/system"
)

// State provides the current operational status of the BANDIQ agent.
type State struct {
	mu           sync.RWMutex
	Paused       bool                      `json:"paused"`
	NodeID       string                    `json:"node_id"`
	Wallet       string                    `json:"operator_wallet"`
	Health       string                    `json:"health"`
	ChainStatus  string                    `json:"chain_status"`
	LatestReport *report.MeasurementReport `json:"latest_report,omitempty"`
	StartTime    time.Time                 `json:"start_time"`
	TotalTests   uint64                    `json:"total_tests"`
}

// Server exposes a secure, localhost-only HTTP management API.
type Server struct {
	httpServer *http.Server
	cfg        *config.Config
	consentMgr *consent.Manager
	spoolQueue *spool.Queue
	collector  system.Collector
	state      *State
	triggerFn  func() (*report.MeasurementReport, error)
}

// NewServer creates a local administrative server.
func NewServer(
	cfg *config.Config,
	consentMgr *consent.Manager,
	spoolQueue *spool.Queue,
	collector system.Collector,
	nodeID, wallet string,
	triggerFn func() (*report.MeasurementReport, error),
) (*Server, error) {
	// Security: verify bind address is loopback
	host, _, err := net.SplitHostPort(cfg.Admin.BindAddress)
	if err != nil {
		return nil, fmt.Errorf("invalid admin bind address: %w", err)
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return nil, fmt.Errorf("security violation: admin interface cannot bind to non-loopback address '%s'", cfg.Admin.BindAddress)
	}

	state := &State{
		Paused:      false,
		NodeID:      nodeID,
		Wallet:      wallet,
		Health:      "HEALTHY",
		ChainStatus: "CONNECTED",
		StartTime:   time.Now().UTC(),
	}

	s := &Server{
		cfg:        cfg,
		consentMgr: consentMgr,
		spoolQueue: spoolQueue,
		collector:  collector,
		state:      state,
		triggerFn:  triggerFn,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/pause", s.handlePause)
	mux.HandleFunc("/resume", s.handleResume)
	mux.HandleFunc("/caps", s.handleCaps)
	mux.HandleFunc("/trigger", s.handleTrigger)
	mux.HandleFunc("/reports/latest", s.handleLatestReport)

	corsHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-BANDIQ-Node-ID, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		mux.ServeHTTP(w, r)
	})

	s.httpServer = &http.Server{
		Addr:         cfg.Admin.BindAddress,
		Handler:      corsHandler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s, nil
}

// Start runs the HTTP server.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return err
	}
	go func() {
		_ = s.httpServer.Serve(ln)
	}()
	return nil
}

// Stop shuts down the admin HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// SetHealth updates health state string.
func (s *Server) SetHealth(status string) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	s.state.Health = status
}

// SetChainStatus updates chain status string.
func (s *Server) SetChainStatus(status string) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	s.state.ChainStatus = status
}

// RecordCompletedTest updates state with the latest report.
func (s *Server) RecordCompletedTest(rep *report.MeasurementReport) {
	s.state.mu.Lock()
	defer s.state.mu.Unlock()
	s.state.LatestReport = rep
	s.state.TotalTests++
}

// IsPaused returns pause state.
func (s *Server) IsPaused() bool {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()
	return s.state.Paused
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	health := s.state.Health
	paused := s.state.Paused
	s.state.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    health,
		"paused":    paused,
		"timestamp": time.Now().UTC(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	defer s.state.mu.RUnlock()

	sysMetrics, _ := s.collector.GetSystemMetrics()
	spoolCount, _ := s.spoolQueue.Count()
	consentStatus := s.consentMgr.Status()

	resp := map[string]interface{}{
		"node_id":          s.state.NodeID,
		"operator_wallet":  s.state.Wallet,
		"health":           s.state.Health,
		"chain_status":     s.state.ChainStatus,
		"paused":           s.state.Paused,
		"uptime_sec":       uint64(time.Since(s.state.StartTime).Seconds()),
		"total_tests":      s.state.TotalTests,
		"spool_count":      spoolCount,
		"consent":          consentStatus,
		"system":           sysMetrics,
		"bandwidth_limits": s.cfg.Measurement,
		"latest_report":    s.state.LatestReport,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.state.mu.Lock()
	s.state.Paused = true
	s.state.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "paused",
		"message": "BANDIQ measurement loop is now paused",
	})
}

func (s *Server) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s.state.mu.Lock()
	s.state.Paused = false
	s.state.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "active",
		"message": "BANDIQ measurement loop resumed",
	})
}

type capsRequest struct {
	DailyDataCapMB        *int     `json:"daily_data_cap_mb,omitempty"`
	MaxBandwidthLimitMbps *float64 `json:"max_bandwidth_limit_mbps,omitempty"`
}

func (s *Server) handleCaps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req capsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.DailyDataCapMB != nil {
		if *req.DailyDataCapMB <= 0 {
			http.Error(w, "daily data cap must be positive", http.StatusBadRequest)
			return
		}
		s.cfg.Measurement.DailyDataCapMB = *req.DailyDataCapMB
	}

	if req.MaxBandwidthLimitMbps != nil {
		if *req.MaxBandwidthLimitMbps <= 0 {
			http.Error(w, "bandwidth limit must be positive", http.StatusBadRequest)
			return
		}
		s.cfg.Measurement.MaxBandwidthLimitMbps = *req.MaxBandwidthLimitMbps
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "updated",
		"limits": s.cfg.Measurement,
	})
}

func (s *Server) handleTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.triggerFn == nil {
		http.Error(w, "trigger function not configured", http.StatusInternalServerError)
		return
	}

	rep, err := s.triggerFn()
	if err != nil {
		http.Error(w, fmt.Sprintf("benchmark trigger failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rep)
}

func (s *Server) handleLatestReport(w http.ResponseWriter, r *http.Request) {
	s.state.mu.RLock()
	rep := s.state.LatestReport
	s.state.mu.RUnlock()

	if rep == nil {
		http.Error(w, "no reports completed yet", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(rep)
}
