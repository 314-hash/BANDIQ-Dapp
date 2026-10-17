package verifier

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/report"
)

// Service coordinates Proof-of-Bandwidth verification, audit decisions, and settlement voucher issuance.
type Service struct {
	mu         sync.RWMutex
	store      *Store
	config     PipelineConfig
	verifierID *identity.Identity
	httpServer *http.Server
}

// NewService creates a new verification service instance with an Ed25519 verifier identity.
func NewService(verifierID *identity.Identity, cfg PipelineConfig) *Service {
	return &Service{
		store:      NewStore(),
		config:     cfg,
		verifierID: verifierID,
	}
}

// Store returns the underlying store for queries.
func (s *Service) Store() *Store {
	return s.store
}

// VerifyReport evaluates a signed report through all verification gates and issues a voucher if accepted.
func (s *Service) VerifyReport(rep *report.MeasurementReport) VerificationResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.store.GetNode(rep.NodeID)
	if !exists {
		res := VerificationResult{
			ReportID:       rep.ReportID,
			NodeID:         rep.NodeID,
			OperatorWallet: rep.OperatorWallet,
			Status:         StatusRejected,
			RejectionCode:  CodeUnregisteredNode,
			Details:        fmt.Sprintf("node %s is not registered", rep.NodeID),
			EvaluatedAt:    time.Now().UTC(),
		}
		s.store.seenIDs[rep.ReportID] = true
		s.store.auditLog = append(s.store.auditLog, res)
		return res
	}

	// Run through the 10 gates
	res := EvaluateReport(rep, node, s.store.seenIDs, s.config)

	// If Accepted and eligible, generate cryptographic settlement voucher
	if res.Status == StatusAccepted {
		epochID, nonce := s.store.NextEpochNonce()
		qosBasisPoints := uint32(res.ConfidenceScore * 10000)

		voucher, err := GenerateVoucher(
			s.verifierID,
			epochID,
			rep.NodeID,
			rep.OperatorWallet,
			res.VerifiedBytes,
			qosBasisPoints,
			nonce,
			24*time.Hour, // Voucher valid for 24 hours
		)
		if err == nil {
			res.SettlementVoucher = voucher
		}
	}

	// Update node state and audit log in store
	s.store.UpdateNodeOnDecision(rep.NodeID, rep, res)

	return res
}

// StartHTTP starts the REST API server for the verification service.
func (s *Service) StartHTTP(addr string) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/nodes/register", s.handleRegisterNode)
	mux.HandleFunc("/api/v1/nodes", s.handleListNodes)
	mux.HandleFunc("/api/v1/reports/verify", s.handleVerifyReport)
	mux.HandleFunc("/api/v1/reports/submit", s.handleVerifyReport) // backward-compatible alias
	mux.HandleFunc("/api/v1/audit/decisions", s.handleListDecisions)
	mux.HandleFunc("/api/v1/disputes", s.handleDisputes)

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

	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      corsHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

// Stop gracefully terminates the HTTP server.
func (s *Service) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

func (s *Service) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "HEALTHY",
		"service":           "BANDIQ Proof-of-Bandwidth Verifier",
		"verifier_identity": s.verifierID.NodeID,
		"timestamp":         time.Now().UTC(),
	})
}

func (s *Service) handleRegisterNode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID          string `json:"node_id"`
		OperatorWallet  string `json:"operator_wallet"`
		PublicKeyHex    string `json:"public_key_hex"`
		SoftwareVersion string `json:"software_version"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request json", http.StatusBadRequest)
		return
	}

	if err := s.store.RegisterNode(req.NodeID, req.OperatorWallet, req.PublicKeyHex, req.SoftwareVersion); err != nil {
		if err == ErrNodeAlreadyRegistered {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"status": "already_registered"})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "registered",
		"node_id": req.NodeID,
	})
}

func (s *Service) handleVerifyReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rep report.MeasurementReport
	if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
		http.Error(w, fmt.Sprintf("malformed json: %v", err), http.StatusBadRequest)
		return
	}

	result := s.VerifyReport(&rep)

	w.Header().Set("Content-Type", "application/json")
	if result.Status == StatusAccepted {
		w.WriteHeader(http.StatusOK)
	} else if result.Status == StatusPendingReview {
		w.WriteHeader(http.StatusAccepted)
	} else {
		// Distinguish client errors (e.g. replay/duplicate) from rejected tests
		if result.RejectionCode == CodeDuplicateReportID || result.RejectionCode == CodeSequenceOutOfOrder {
			w.WriteHeader(http.StatusConflict)
		} else {
			w.WriteHeader(http.StatusUnprocessableEntity)
		}
	}

	_ = json.NewEncoder(w).Encode(result)
}

func (s *Service) handleListNodes(w http.ResponseWriter, r *http.Request) {
	nodes := s.store.ListNodes()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(nodes)
}

func (s *Service) handleListDecisions(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit, _ := strconv.Atoi(limitStr)
	if limit <= 0 {
		limit = 50
	}

	decisions := s.store.ListAuditLog(limit)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(decisions)
}

func (s *Service) handleDisputes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		disputes := s.store.ListDisputes()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(disputes)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			ReportID       string `json:"report_id"`
			NodeID         string `json:"node_id"`
			OperatorWallet string `json:"operator_wallet"`
			Reason         string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid dispute json", http.StatusBadRequest)
			return
		}

		disp, err := s.store.FileDispute(req.ReportID, req.NodeID, req.OperatorWallet, req.Reason)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(disp)
		return
	}

	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}
