package verifier

import (
	"fmt"
	"math"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/report"
)

// PipelineConfig configures thresholds and parameters for the verification gates.
type PipelineConfig struct {
	MaxClockDrift        time.Duration
	MinReportInterval    time.Duration
	MaxPlausibleThroughputMbps float64
	DailyDataCapBytes    uint64
	ConfidenceThreshold  float64 // Minimum confidence to accept (e.g. 0.70)
	ReviewThreshold      float64 // Threshold below which report is rejected rather than pending (e.g. 0.40)
	RequireSessionReceipt bool
}

// DefaultPipelineConfig returns strict, production-grade defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		MaxClockDrift:              5 * time.Minute,
		MinReportInterval:          30 * time.Second,
		MaxPlausibleThroughputMbps: 10000.0, // 10 Gbps cap for residential/standard nodes
		DailyDataCapBytes:          10 * 1024 * 1024 * 1024, // 10 GB daily cap
		ConfidenceThreshold:        0.70,
		ReviewThreshold:            0.40,
		RequireSessionReceipt:      false, // In measurement-only v0.1.0, active benchmarks are evaluated
	}
}

// EvaluateReport runs an incoming measurement report through the 10 sequential verification gates.
func EvaluateReport(
	rep *report.MeasurementReport,
	node *NodeState,
	seenIDs map[string]bool,
	cfg PipelineConfig,
) VerificationResult {
	res := VerificationResult{
		ReportID:        rep.ReportID,
		NodeID:          rep.NodeID,
		OperatorWallet:  rep.OperatorWallet,
		Status:          StatusRejected,
		RejectionCode:   CodeNone,
		ConfidenceScore: 0.0,
		EvaluatedAt:     time.Now().UTC(),
	}

	// -------------------------------------------------------------
	// Gate 1: Schema & Format Integrity
	// -------------------------------------------------------------
	if rep.ReportID == "" || len(rep.ReportID) < 16 {
		res.RejectionCode = CodeMalformedSchema
		res.Details = "report_id missing or invalid UUID format"
		return res
	}
	if rep.NodeID == "" {
		res.RejectionCode = CodeMalformedSchema
		res.Details = "node_id missing"
		return res
	}
	if err := identity.ValidateWallet(rep.OperatorWallet); err != nil {
		res.RejectionCode = CodeMalformedSchema
		res.Details = fmt.Sprintf("invalid operator wallet format: %v", err)
		return res
	}
	if rep.SequenceNumber == 0 {
		res.RejectionCode = CodeMalformedSchema
		res.Details = "sequence_number must be strictly greater than 0"
		return res
	}

	// -------------------------------------------------------------
	// Gate 2: Registered Node Identity & Signature
	// -------------------------------------------------------------
	if node == nil {
		res.RejectionCode = CodeUnregisteredNode
		res.Details = fmt.Sprintf("node %s is not registered in the network registry", rep.NodeID)
		return res
	}
	if node.Suspended {
		res.RejectionCode = CodeUnregisteredNode
		res.Details = fmt.Sprintf("node %s is currently suspended due to policy violation", rep.NodeID)
		return res
	}
	if err := rep.Verify(node.PublicKeyHex); err != nil {
		res.RejectionCode = CodeInvalidSignature
		res.Details = fmt.Sprintf("cryptographic signature verification failed: %v", err)
		return res
	}

	// -------------------------------------------------------------
	// Gate 3: Replay Defense & Sequence Monotonicity
	// -------------------------------------------------------------
	if seenIDs[rep.ReportID] {
		res.RejectionCode = CodeDuplicateReportID
		res.Details = fmt.Sprintf("replay attack detected: report_id %s has already been evaluated", rep.ReportID)
		return res
	}
	if rep.SequenceNumber <= node.LastSequence {
		res.RejectionCode = CodeSequenceOutOfOrder
		res.Details = fmt.Sprintf("out-of-order sequence: received %d, but node is already at %d",
			rep.SequenceNumber, node.LastSequence)
		return res
	}

	// -------------------------------------------------------------
	// Gate 4: Temporal Freshness & Clock Drift
	// -------------------------------------------------------------
	now := time.Now().UTC()
	if rep.Timestamp.After(now.Add(cfg.MaxClockDrift)) {
		res.RejectionCode = CodeTimestampFuture
		res.Details = fmt.Sprintf("timestamp is in the future (+%v drift)", rep.Timestamp.Sub(now))
		return res
	}
	if rep.Timestamp.Before(now.Add(-cfg.MaxClockDrift)) {
		res.RejectionCode = CodeTimestampStale
		res.Details = fmt.Sprintf("timestamp is stale (-%v drift)", now.Sub(rep.Timestamp))
		return res
	}
	if !node.LastReportTime.IsZero() && rep.Timestamp.Before(node.LastReportTime) {
		res.RejectionCode = CodeTimestampStale
		res.Details = "timestamp is earlier than previous accepted report from this node"
		return res
	}

	// -------------------------------------------------------------
	// Gate 5: Physical Plausibility & Impossible Bounds
	// -------------------------------------------------------------
	m := rep.Metrics
	if m.DownloadMbps < 0 || m.UploadMbps < 0 || m.LatencyMs < 0 || m.JitterMs < 0 || m.PacketLossPct < 0 {
		res.RejectionCode = CodeImpossibleThroughput
		res.Details = "negative metric values are physically impossible"
		return res
	}
	if math.IsNaN(m.DownloadMbps) || math.IsInf(m.DownloadMbps, 0) ||
		math.IsNaN(m.UploadMbps) || math.IsInf(m.UploadMbps, 0) {
		res.RejectionCode = CodeImpossibleThroughput
		res.Details = "throughput contains invalid NaN or Inf floating point values"
		return res
	}
	if m.DownloadMbps > cfg.MaxPlausibleThroughputMbps || m.UploadMbps > cfg.MaxPlausibleThroughputMbps {
		res.RejectionCode = CodeImpossibleThroughput
		res.Details = fmt.Sprintf("throughput %.2f Mbps exceeds maximum physical plausibility ceiling (%.2f Mbps)",
			m.DownloadMbps, cfg.MaxPlausibleThroughputMbps)
		return res
	}
	if m.PacketLossPct > 100.0 {
		res.RejectionCode = CodeImpossibleThroughput
		res.Details = "packet loss percentage cannot exceed 100%"
		return res
	}

	// -------------------------------------------------------------
	// Gate 6: Interface Counter Consistency
	// -------------------------------------------------------------
	if node.LastInterfaceRecv > 0 && rep.InterfaceCounters.BytesRecv > 0 {
		// Only check if interface counter didn't wrap or reset (e.g. reboot)
		if rep.InterfaceCounters.BytesRecv >= node.LastInterfaceRecv {
			deltaHardwareIn := rep.InterfaceCounters.BytesRecv - node.LastInterfaceRecv
			// If reported active test download bytes exceed the total bytes that actually
			// traversed the physical OS network adapter, the test is mathematically fabricated!
			if m.ActiveTestBytesIn > deltaHardwareIn && deltaHardwareIn > 0 {
				res.RejectionCode = CodeCounterInconsistency
				res.Details = fmt.Sprintf("hardware interface delta (%d bytes) is less than reported test traffic (%d bytes)",
					deltaHardwareIn, m.ActiveTestBytesIn)
				return res
			}
		}
	}

	// -------------------------------------------------------------
	// Gate 7: Duplicate / Identical Sample Spam
	// -------------------------------------------------------------
	if node.LastMetrics != nil {
		// If float metrics are identical to 4 decimal places across consecutive tests,
		// natural network jitter is missing (indicates scripted replay or mock loop)
		if math.Abs(m.DownloadMbps-node.LastMetrics.DownloadMbps) < 0.0001 &&
			math.Abs(m.UploadMbps-node.LastMetrics.UploadMbps) < 0.0001 &&
			math.Abs(m.LatencyMs-node.LastMetrics.LatencyMs) < 0.0001 {
			res.RejectionCode = CodeIdenticalSampleSpam
			res.Details = "detected suspicious duplicate sample: consecutive metrics are identically matching"
			return res
		}
	}

	// -------------------------------------------------------------
	// Gate 8: Rate Limiting & Daily Quota
	// -------------------------------------------------------------
	if !node.LastReportTime.IsZero() && now.Sub(node.LastReportTime) < cfg.MinReportInterval {
		res.RejectionCode = CodeRateLimitExceeded
		res.Details = fmt.Sprintf("rate limit violated: interval %v is shorter than minimum %v",
			now.Sub(node.LastReportTime), cfg.MinReportInterval)
		return res
	}

	currentDay := now.Day()
	if currentDay != node.LastResetDay {
		node.DailyBytesToday = 0
		node.LastResetDay = currentDay
	}
	totalReportBytes := m.ActiveTestBytesIn + m.ActiveTestBytesOut + m.ServedBytesIn + m.ServedBytesOut
	if (node.DailyBytesToday + totalReportBytes) > cfg.DailyDataCapBytes {
		res.RejectionCode = CodeDailyCapExceeded
		res.Details = fmt.Sprintf("daily data cap exceeded: %d bytes would surpass %d byte quota",
			node.DailyBytesToday+totalReportBytes, cfg.DailyDataCapBytes)
		return res
	}

	// -------------------------------------------------------------
	// Gate 9: Traffic Segregation & Eligibility Classification
	// -------------------------------------------------------------
	// Critical DePIN Rule: Synthetic benchmarks are NEVER billable customer traffic.
	// Only served bytes with verified mutual receipts count towards direct reward settlement.
	isRewardEligible := false
	var billableBytes uint64 = 0

	if m.ServedBytesOut > 0 {
		// Real workload traffic delivered
		billableBytes = m.ServedBytesOut
		isRewardEligible = true
	} else {
		// Synthetic speedtest capacity benchmark only
		billableBytes = 0
		isRewardEligible = false
	}

	// -------------------------------------------------------------
	// Gate 10: Statistical Confidence Rating
	// -------------------------------------------------------------
	confidence := 1.0

	// Deduct for packet loss
	if m.PacketLossPct > 0 {
		confidence -= (m.PacketLossPct / 100.0) * 0.4
	}

	// Deduct for high jitter
	if m.JitterMs > 25.0 {
		confidence -= 0.15
	} else if m.JitterMs > 10.0 {
		confidence -= 0.05
	}

	// Deduct for CPU overload on host
	if rep.SystemMetrics.CPUPercent > 90.0 {
		confidence -= 0.1
	}

	// Bound confidence between 0.0 and 1.0
	if confidence < 0.0 {
		confidence = 0.0
	}
	confidence = math.Round(confidence*100) / 100

	res.ConfidenceScore = confidence
	res.VerifiedThroughputMbps = m.DownloadMbps
	res.VerifiedBytes = billableBytes
	res.IsRewardEligible = isRewardEligible

	if confidence >= cfg.ConfidenceThreshold {
		res.Status = StatusAccepted
		res.RejectionCode = CodeNone
		res.Details = fmt.Sprintf("verification passed (Confidence: %.2f)", confidence)
	} else if confidence >= cfg.ReviewThreshold {
		res.Status = StatusPendingReview
		res.RejectionCode = CodeLowConfidence
		res.Details = fmt.Sprintf("flagged for witness challenge / secondary probe (Confidence: %.2f)", confidence)
	} else {
		res.Status = StatusRejected
		res.RejectionCode = CodeLowConfidence
		res.Details = fmt.Sprintf("confidence score %.2f is below minimum threshold %.2f", confidence, cfg.ReviewThreshold)
	}

	return res
}
