package verifier

import (
	"testing"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/report"
	"bandiq-node-agent/internal/system"
)

func setupTestVerifierAndNode(t *testing.T) (*Service, *identity.Identity, PipelineConfig) {
	verifierID, err := identity.GenerateNewIdentity("0x0000000000000000000000000000000000000001")
	if err != nil {
		t.Fatalf("failed to create verifier id: %v", err)
	}

	cfg := DefaultPipelineConfig()
	svc := NewService(verifierID, cfg)

	nodeWallet := "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"
	nodeID, err := identity.GenerateNewIdentity(nodeWallet)
	if err != nil {
		t.Fatalf("failed to create node id: %v", err)
	}

	_ = svc.Store().RegisterNode(nodeID.NodeID, nodeWallet, nodeID.PublicKeyHex(), "0.1.0")

	return svc, nodeID, cfg
}

func TestAdversarial_UnregisteredNode(t *testing.T) {
	verifierID, _ := identity.GenerateNewIdentity("0x0000000000000000000000000000000000000001")
	svc := NewService(verifierID, DefaultPipelineConfig())

	unregisteredNode, _ := identity.GenerateNewIdentity("0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133")

	rep := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         unregisteredNode.NodeID,
		OperatorWallet: unregisteredNode.OperatorWallet,
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC(),
	}
	_ = rep.Sign(unregisteredNode)

	res := svc.VerifyReport(rep)
	if res.Status != StatusRejected || res.RejectionCode != CodeUnregisteredNode {
		t.Fatalf("expected ERR_UNREGISTERED_NODE, got %s (code: %s)", res.Status, res.RejectionCode)
	}
}

func TestAdversarial_SignatureForgery(t *testing.T) {
	svc, nodeID, _ := setupTestVerifierAndNode(t)

	rep := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC(),
		Metrics: report.Metrics{
			DownloadMbps: 100.0,
		},
	}
	_ = rep.Sign(nodeID)

	// Malicious actor tampers with metrics after signature was created
	rep.Metrics.DownloadMbps = 999.0

	res := svc.VerifyReport(rep)
	if res.Status != StatusRejected || res.RejectionCode != CodeInvalidSignature {
		t.Fatalf("expected ERR_INVALID_SIGNATURE on tampered report, got %s (code: %s)", res.Status, res.RejectionCode)
	}
}

func TestAdversarial_ReplayAttack(t *testing.T) {
	svc, nodeID, _ := setupTestVerifierAndNode(t)

	rep := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC(),
		Metrics: report.Metrics{
			DownloadMbps: 50.0,
		},
	}
	_ = rep.Sign(nodeID)

	// First submission passes
	res1 := svc.VerifyReport(rep)
	if res1.Status != StatusAccepted {
		t.Fatalf("first report failed: %s", res1.Details)
	}

	// Replay exact same report
	res2 := svc.VerifyReport(rep)
	if res2.Status != StatusRejected || res2.RejectionCode != CodeDuplicateReportID {
		t.Fatalf("expected ERR_DUPLICATE_REPORT_ID on replay, got %s (code: %s)", res2.Status, res2.RejectionCode)
	}
}

func TestAdversarial_SequenceOutOfOrder(t *testing.T) {
	svc, nodeID, _ := setupTestVerifierAndNode(t)

	// Submit sequence 5
	rep5 := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 5,
		Timestamp:      time.Now().UTC(),
	}
	_ = rep5.Sign(nodeID)
	res5 := svc.VerifyReport(rep5)
	if res5.Status != StatusAccepted {
		t.Fatalf("seq 5 failed: %s", res5.Details)
	}

	// Try submitting sequence 3 (stale sequence attempt)
	rep3 := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 3,
		Timestamp:      time.Now().UTC(),
	}
	_ = rep3.Sign(nodeID)
	res3 := svc.VerifyReport(rep3)
	if res3.Status != StatusRejected || res3.RejectionCode != CodeSequenceOutOfOrder {
		t.Fatalf("expected ERR_SEQUENCE_OUT_OF_ORDER, got %s (code: %s)", res3.Status, res3.RejectionCode)
	}
}

func TestAdversarial_ClockDrift(t *testing.T) {
	svc, nodeID, _ := setupTestVerifierAndNode(t)

	// Future timestamp (+1 hour)
	futureRep := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Add(1 * time.Hour),
	}
	_ = futureRep.Sign(nodeID)
	resFuture := svc.VerifyReport(futureRep)
	if resFuture.Status != StatusRejected || resFuture.RejectionCode != CodeTimestampFuture {
		t.Fatalf("expected ERR_TIMESTAMP_FUTURE, got %s (code: %s)", resFuture.Status, resFuture.RejectionCode)
	}

	// Stale timestamp (-2 hours)
	staleRep := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC().Add(-2 * time.Hour),
	}
	_ = staleRep.Sign(nodeID)
	resStale := svc.VerifyReport(staleRep)
	if resStale.Status != StatusRejected || resStale.RejectionCode != CodeTimestampStale {
		t.Fatalf("expected ERR_TIMESTAMP_STALE, got %s (code: %s)", resStale.Status, resStale.RejectionCode)
	}
}

func TestAdversarial_ImpossibleThroughput(t *testing.T) {
	svc, nodeID, _ := setupTestVerifierAndNode(t)

	// Claiming 50,000 Mbps (50 Gbps) on a standard residential node
	rep := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC(),
		Metrics: report.Metrics{
			DownloadMbps: 50000.0,
		},
	}
	_ = rep.Sign(nodeID)

	res := svc.VerifyReport(rep)
	if res.Status != StatusRejected || res.RejectionCode != CodeImpossibleThroughput {
		t.Fatalf("expected ERR_IMPOSSIBLE_THROUGHPUT, got %s (code: %s)", res.Status, res.RejectionCode)
	}
}

func TestAdversarial_InterfaceCounterFraud(t *testing.T) {
	svc, nodeID, _ := setupTestVerifierAndNode(t)

	// Report 1: Baseline interface counters
	rep1 := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC(),
		Metrics: report.Metrics{
			DownloadMbps:      100.0,
			ActiveTestBytesIn: 1000000,
		},
		InterfaceCounters: system.InterfaceCounters{
			BytesRecv: 10000000,
		},
	}
	_ = rep1.Sign(nodeID)
	res1 := svc.VerifyReport(rep1)
	if res1.Status != StatusAccepted {
		t.Fatalf("baseline report failed: %s", res1.Details)
	}

	// Report 2: Node claims 50 MB (52,428,800 bytes) active test download,
	// but the OS physical hardware interface only incremented by 1 MB (1,000,000 bytes)!
	rep2 := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 2,
		Timestamp:      time.Now().UTC().Add(1 * time.Minute),
		Metrics: report.Metrics{
			DownloadMbps:      250.0,
			ActiveTestBytesIn: 52428800, // 50 MB
		},
		InterfaceCounters: system.InterfaceCounters{
			BytesRecv: 11000000, // only +1 MB incremented!
		},
	}
	_ = rep2.Sign(nodeID)
	res2 := svc.VerifyReport(rep2)
	if res2.Status != StatusRejected || res2.RejectionCode != CodeCounterInconsistency {
		t.Fatalf("expected ERR_COUNTER_INCONSISTENCY, got %s (code: %s, details: %s)",
			res2.Status, res2.RejectionCode, res2.Details)
	}
}

func TestAdversarial_IdenticalSampleSpam(t *testing.T) {
	svc, nodeID, _ := setupTestVerifierAndNode(t)

	// Report 1
	rep1 := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC(),
		Metrics: report.Metrics{
			DownloadMbps: 94.2541,
			UploadMbps:   38.6012,
			LatencyMs:    14.8219,
		},
		InterfaceCounters: system.InterfaceCounters{
			BytesRecv: 10000000,
		},
	}
	_ = rep1.Sign(nodeID)
	res1 := svc.VerifyReport(rep1)
	if res1.Status != StatusAccepted {
		t.Fatalf("report 1 failed: %s", res1.Details)
	}

	// Report 2: Scripted bot sends exactly identical floating-point measurements to 4 decimals
	rep2 := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		NodeID:         nodeID.NodeID,
		OperatorWallet: nodeID.OperatorWallet,
		SequenceNumber: 2,
		Timestamp:      time.Now().UTC().Add(1 * time.Minute),
		Metrics: report.Metrics{
			DownloadMbps: 94.2541, // EXACT match
			UploadMbps:   38.6012, // EXACT match
			LatencyMs:    14.8219, // EXACT match
		},
		InterfaceCounters: system.InterfaceCounters{
			BytesRecv: 20000000,
		},
	}
	_ = rep2.Sign(nodeID)
	res2 := svc.VerifyReport(rep2)
	if res2.Status != StatusRejected || res2.RejectionCode != CodeIdenticalSampleSpam {
		t.Fatalf("expected ERR_IDENTICAL_SAMPLE_SPAM, got %s (code: %s)", res2.Status, res2.RejectionCode)
	}
}
