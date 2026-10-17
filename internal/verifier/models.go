package verifier

import (
	"time"

	"bandiq-node-agent/internal/report"
)

// DecisionStatus defines the outcome of the verification evaluation.
type DecisionStatus string

const (
	StatusAccepted      DecisionStatus = "ACCEPTED"
	StatusRejected      DecisionStatus = "REJECTED"
	StatusPendingReview DecisionStatus = "PENDING_REVIEW"
)

// RejectionCode provides machine-readable reasons for report rejection.
type RejectionCode string

const (
	CodeNone                    RejectionCode = "NONE"
	CodeMalformedSchema         RejectionCode = "ERR_MALFORMED_SCHEMA"
	CodeUnregisteredNode        RejectionCode = "ERR_UNREGISTERED_NODE"
	CodeInvalidSignature        RejectionCode = "ERR_INVALID_SIGNATURE"
	CodeDuplicateReportID       RejectionCode = "ERR_DUPLICATE_REPORT_ID"
	CodeSequenceOutOfOrder      RejectionCode = "ERR_SEQUENCE_OUT_OF_ORDER"
	CodeTimestampStale          RejectionCode = "ERR_TIMESTAMP_STALE"
	CodeTimestampFuture         RejectionCode = "ERR_TIMESTAMP_FUTURE"
	CodeImpossibleThroughput    RejectionCode = "ERR_IMPOSSIBLE_THROUGHPUT"
	CodeCounterInconsistency    RejectionCode = "ERR_COUNTER_INCONSISTENCY"
	CodeIdenticalSampleSpam     RejectionCode = "ERR_IDENTICAL_SAMPLE_SPAM"
	CodeRateLimitExceeded       RejectionCode = "ERR_RATE_LIMIT_EXCEEDED"
	CodeDailyCapExceeded        RejectionCode = "ERR_DAILY_CAP_EXCEEDED"
	CodeTrafficMisclassification RejectionCode = "ERR_TRAFFIC_MISCLASSIFICATION"
	CodeLowConfidence           RejectionCode = "ERR_LOW_CONFIDENCE"
)

// VerificationResult contains the comprehensive outcome of a report evaluation.
type VerificationResult struct {
	ReportID               string             `json:"report_id"`
	NodeID                 string             `json:"node_id"`
	OperatorWallet         string             `json:"operator_wallet"`
	Status                 DecisionStatus     `json:"status"`
	RejectionCode          RejectionCode      `json:"rejection_code"`
	ConfidenceScore        float64            `json:"confidence_score"`
	VerifiedThroughputMbps float64            `json:"verified_throughput_mbps"`
	VerifiedBytes          uint64             `json:"verified_bytes"`
	IsRewardEligible       bool               `json:"is_reward_eligible"`
	SettlementVoucher      *SettlementVoucher `json:"settlement_voucher,omitempty"`
	EvaluatedAt            time.Time          `json:"evaluated_at"`
	Details                string             `json:"details"`
}

// SettlementVoucher represents an auditable cryptographic attestation produced by the verifier for accepted usage.
type SettlementVoucher struct {
	VoucherID        string    `json:"voucher_id"`
	EpochID          uint64    `json:"epoch_id"`
	NodeID           string    `json:"node_id"`
	OperatorWallet   string    `json:"operator_wallet"`
	VerifiedBytes    uint64    `json:"verified_bytes"`
	QoSScore         uint32    `json:"qos_score"` // 0 - 10000 basis points (e.g. 9850 = 98.5%)
	Nonce            uint64    `json:"nonce"`
	IssuedAt         time.Time `json:"issued_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	VerifierIdentity string    `json:"verifier_identity"`
	Signature        string    `json:"signature"`
}

// Dispute encapsulates an operator's formal challenge to a rejected or pending report.
type Dispute struct {
	DisputeID      string     `json:"dispute_id"`
	ReportID       string     `json:"report_id"`
	NodeID         string     `json:"node_id"`
	OperatorWallet string     `json:"operator_wallet"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"` // "OPEN", "RESOLVED", "DENIED"
	SubmittedAt    time.Time  `json:"submitted_at"`
	ResolvedAt     *time.Time `json:"resolved_at,omitempty"`
	ResolutionNote string     `json:"resolution_note,omitempty"`
}

// NodeState tracks the stateful telemetry history required for sequence checking, counter deltas, and fraud detection.
type NodeState struct {
	NodeID            string
	OperatorWallet    string
	PublicKeyHex      string
	SoftwareVersion   string
	RegisteredAt      time.Time
	LastSeen          time.Time
	LastSequence      uint64
	LastReportTime    time.Time
	LastReportID      string
	LastInterfaceSent uint64
	LastInterfaceRecv uint64
	LastMetrics       *report.Metrics
	ConsecutiveSpams  int
	DailyBytesToday   uint64
	LastResetDay      int
	ReputationScore   float64 // 0.0 - 1.0
	Suspended         bool
}
