package verifier

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"bandiq-node-agent/internal/report"
)

var (
	ErrNodeAlreadyRegistered = errors.New("node is already registered")
	ErrNodeNotFound          = errors.New("node not found in registry")
	ErrDisputeNotFound       = errors.New("dispute not found")
)

// Store provides thread-safe access to node registry states, audit logs, and dispute records.
type Store struct {
	mu           sync.RWMutex
	nodes        map[string]*NodeState
	seenIDs      map[string]bool
	auditLog     []VerificationResult
	disputes     map[string]*Dispute
	epochCounter uint64
	nonceCounter uint64
}

// NewStore initializes a new verifier store.
func NewStore() *Store {
	return &Store{
		nodes:        make(map[string]*NodeState),
		seenIDs:      make(map[string]bool),
		auditLog:     make([]VerificationResult, 0),
		disputes:     make(map[string]*Dispute),
		epochCounter: 1,
	}
}

// RegisterNode adds a newly registered node into the verifier registry.
func (s *Store) RegisterNode(nodeID, wallet, pubKeyHex, softwareVersion string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.nodes[nodeID]; exists {
		return ErrNodeAlreadyRegistered
	}

	s.nodes[nodeID] = &NodeState{
		NodeID:          nodeID,
		OperatorWallet:  wallet,
		PublicKeyHex:    pubKeyHex,
		SoftwareVersion: softwareVersion,
		RegisteredAt:    time.Now().UTC(),
		LastSeen:        time.Now().UTC(),
		LastSequence:    0,
		ReputationScore: 1.0,
		LastResetDay:    time.Now().UTC().Day(),
	}

	return nil
}

// GetNode retrieves a node's state.
func (s *Store) GetNode(nodeID string) (*NodeState, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, exists := s.nodes[nodeID]
	return n, exists
}

// UpdateNodeOnDecision updates node sequence and counter telemetry when a report is processed.
func (s *Store) UpdateNodeOnDecision(nodeID string, rep *report.MeasurementReport, res VerificationResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		return
	}

	s.seenIDs[rep.ReportID] = true
	s.auditLog = append(s.auditLog, res)

	node.LastSeen = time.Now().UTC()

	if res.Status == StatusAccepted {
		node.LastSequence = rep.SequenceNumber
		node.LastReportTime = rep.Timestamp
		node.LastReportID = rep.ReportID
		node.LastInterfaceRecv = rep.InterfaceCounters.BytesRecv
		node.LastInterfaceSent = rep.InterfaceCounters.BytesSent
		node.LastMetrics = &rep.Metrics
		node.DailyBytesToday += (rep.Metrics.ActiveTestBytesIn + rep.Metrics.ActiveTestBytesOut + rep.Metrics.ServedBytesIn + rep.Metrics.ServedBytesOut)
		node.ConsecutiveSpams = 0
	} else if res.RejectionCode == CodeIdenticalSampleSpam {
		node.ConsecutiveSpams++
		if node.ConsecutiveSpams >= 3 {
			node.ReputationScore = 0.2
		}
	}
}

// NextEpochNonce increments and returns a unique epoch nonce for voucher generation.
func (s *Store) NextEpochNonce() (uint64, uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nonceCounter++
	return s.epochCounter, s.nonceCounter
}

// AdvanceEpoch increments the verification epoch.
func (s *Store) AdvanceEpoch() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.epochCounter++
	return s.epochCounter
}

// ListNodes returns all registered nodes.
func (s *Store) ListNodes() []*NodeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*NodeState
	for _, n := range s.nodes {
		list = append(list, n)
	}
	return list
}

// ListAuditLog returns the latest verification decisions.
func (s *Store) ListAuditLog(limit int) []VerificationResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	total := len(s.auditLog)
	if limit <= 0 || limit > total {
		limit = total
	}

	start := total - limit
	if start < 0 {
		start = 0
	}

	res := make([]VerificationResult, limit)
	copy(res, s.auditLog[start:])
	return res
}

// FileDispute logs an operator's formal challenge to a rejected report.
func (s *Store) FileDispute(reportID, nodeID, wallet, reason string) (*Dispute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	disputeID := fmt.Sprintf("disp-%d", time.Now().UnixNano())
	d := &Dispute{
		DisputeID:      disputeID,
		ReportID:       reportID,
		NodeID:         nodeID,
		OperatorWallet: wallet,
		Reason:         reason,
		Status:         "OPEN",
		SubmittedAt:    time.Now().UTC(),
	}

	s.disputes[disputeID] = d
	return d, nil
}

// ListDisputes returns all disputes.
func (s *Store) ListDisputes() []*Dispute {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var list []*Dispute
	for _, d := range s.disputes {
		list = append(list, d)
	}
	return list
}

// ResolveDispute resolves or denies an appeal.
func (s *Store) ResolveDispute(disputeID, status, resolutionNote string) (*Dispute, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, exists := s.disputes[disputeID]
	if !exists {
		return nil, ErrDisputeNotFound
	}

	now := time.Now().UTC()
	d.Status = status
	d.ResolvedAt = &now
	d.ResolutionNote = resolutionNote

	return d, nil
}
