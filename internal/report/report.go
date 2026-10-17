package report

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/system"
)

var (
	ErrInvalidReportID      = errors.New("invalid or empty report ID")
	ErrInvalidSequence      = errors.New("sequence number must be greater than zero")
	ErrTimestampFuture      = errors.New("report timestamp is too far in the future")
	ErrTimestampStale       = errors.New("report timestamp is stale")
	ErrMissingSignature     = errors.New("report is missing cryptographic signature")
	ErrInvalidSignature     = errors.New("cryptographic signature verification failed")
	ErrNegativeBytes        = errors.New("byte counters cannot be negative")
	ErrNegativeMetrics      = errors.New("throughput or latency cannot be negative")
)

// Metrics encapsulates performance measurements, strictly distinguishing synthetic test traffic from served traffic.
type Metrics struct {
	DownloadMbps       float64 `json:"download_mbps"`
	UploadMbps         float64 `json:"upload_mbps"`
	LatencyMs          float64 `json:"latency_ms"`
	MinLatencyMs       float64 `json:"min_latency_ms"`
	MaxLatencyMs       float64 `json:"max_latency_ms"`
	JitterMs           float64 `json:"jitter_ms"`
	PacketLossPct      float64 `json:"packet_loss_pct"`
	ActiveTestBytesIn  uint64  `json:"active_test_bytes_in"`  // Synthetic speedtest traffic downloaded
	ActiveTestBytesOut uint64  `json:"active_test_bytes_out"` // Synthetic speedtest traffic uploaded
	ServedBytesIn      uint64  `json:"served_bytes_in"`       // Actual useful network traffic received for clients
	ServedBytesOut     uint64  `json:"served_bytes_out"`      // Actual useful network traffic served to clients
	TestDurationMs     int64   `json:"test_duration_ms"`
}

// MeasurementReport represents a verifiable proof-of-bandwidth submission.
type MeasurementReport struct {
	ReportID          string                   `json:"report_id"`
	NodeID            string                   `json:"node_id"`
	OperatorWallet    string                   `json:"operator_wallet"`
	SequenceNumber    uint64                   `json:"sequence_number"`
	Timestamp         time.Time                `json:"timestamp"`
	SoftwareVersion   string                   `json:"software_version"`
	TestServerID      string                   `json:"test_server_id"`
	MeasurementMethod string                   `json:"measurement_method"`
	Metrics           Metrics                  `json:"metrics"`
	InterfaceCounters system.InterfaceCounters `json:"interface_counters"`
	SystemMetrics     system.SystemMetrics     `json:"system_metrics"`
	Signature         string                   `json:"signature,omitempty"`
}

// CanonicalPayload defines the exact deterministic JSON structure that gets hashed and signed.
type CanonicalPayload struct {
	ReportID          string                   `json:"report_id"`
	NodeID            string                   `json:"node_id"`
	OperatorWallet    string                   `json:"operator_wallet"`
	SequenceNumber    uint64                   `json:"sequence_number"`
	TimestampRFC3339  string                   `json:"timestamp_rfc3339"`
	SoftwareVersion   string                   `json:"software_version"`
	TestServerID      string                   `json:"test_server_id"`
	MeasurementMethod string                   `json:"measurement_method"`
	Metrics           Metrics                  `json:"metrics"`
	InterfaceCounters system.InterfaceCounters `json:"interface_counters"`
	SystemMetrics     system.SystemMetrics     `json:"system_metrics"`
}

// GenerateReportID generates a cryptographically secure UUIDv4 string.
func GenerateReportID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant RFC 4122
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}

// CanonicalDigest computes the deterministic SHA-256 digest of the report payload.
func (r *MeasurementReport) CanonicalDigest() ([]byte, error) {
	payload := CanonicalPayload{
		ReportID:          r.ReportID,
		NodeID:            r.NodeID,
		OperatorWallet:    r.OperatorWallet,
		SequenceNumber:    r.SequenceNumber,
		TimestampRFC3339:  r.Timestamp.UTC().Format(time.RFC3339Nano),
		SoftwareVersion:   r.SoftwareVersion,
		TestServerID:      r.TestServerID,
		MeasurementMethod: r.MeasurementMethod,
		Metrics:           r.Metrics,
		InterfaceCounters: r.InterfaceCounters,
		SystemMetrics:     r.SystemMetrics,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal canonical report payload: %w", err)
	}

	h := sha256.Sum256(data)
	return h[:], nil
}

// Sign signs the report with the node's private identity key.
func (r *MeasurementReport) Sign(id *identity.Identity) error {
	if r.ReportID == "" {
		r.ReportID = GenerateReportID()
	}
	r.NodeID = id.NodeID
	r.OperatorWallet = id.OperatorWallet

	digest, err := r.CanonicalDigest()
	if err != nil {
		return err
	}

	r.Signature = id.SignHex(digest)
	return nil
}

// Verify checks the cryptographic signature against a public key hex.
func (r *MeasurementReport) Verify(pubKeyHex string) error {
	if r.Signature == "" {
		return ErrMissingSignature
	}

	digest, err := r.CanonicalDigest()
	if err != nil {
		return fmt.Errorf("failed to compute digest: %w", err)
	}

	if !identity.VerifySignatureHex(pubKeyHex, digest, r.Signature) {
		return ErrInvalidSignature
	}

	return nil
}

// Validate checks sanity and boundaries of all report values.
func (r *MeasurementReport) Validate(maxDrift time.Duration) error {
	if r.ReportID == "" {
		return ErrInvalidReportID
	}
	if r.SequenceNumber == 0 {
		return ErrInvalidSequence
	}
	if err := identity.ValidateWallet(r.OperatorWallet); err != nil {
		return err
	}

	now := time.Now().UTC()
	if r.Timestamp.After(now.Add(maxDrift)) {
		return ErrTimestampFuture
	}
	if r.Timestamp.Before(now.Add(-maxDrift)) {
		return ErrTimestampStale
	}

	if r.Metrics.DownloadMbps < 0 || r.Metrics.UploadMbps < 0 || r.Metrics.LatencyMs < 0 {
		return ErrNegativeMetrics
	}
	if math.IsNaN(r.Metrics.DownloadMbps) || math.IsInf(r.Metrics.DownloadMbps, 0) {
		return ErrNegativeMetrics
	}

	return nil
}
