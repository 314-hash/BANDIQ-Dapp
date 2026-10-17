package verifier

import (
	"testing"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/report"
	"bandiq-node-agent/internal/system"
)

func TestVerificationHappyPath(t *testing.T) {
	// 1. Setup Verifier
	verifierID, err := identity.GenerateNewIdentity("0x0000000000000000000000000000000000000001")
	if err != nil {
		t.Fatalf("failed to create verifier identity: %v", err)
	}

	cfg := DefaultPipelineConfig()
	svc := NewService(verifierID, cfg)

	// 2. Setup Node
	nodeWallet := "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"
	nodeID, err := identity.GenerateNewIdentity(nodeWallet)
	if err != nil {
		t.Fatalf("failed to create node identity: %v", err)
	}

	// Register node
	if err := svc.Store().RegisterNode(nodeID.NodeID, nodeWallet, nodeID.PublicKeyHex(), "0.1.0"); err != nil {
		t.Fatalf("failed to register node: %v", err)
	}

	// 3. Create Valid Report
	rep := &report.MeasurementReport{
		ReportID:          report.GenerateReportID(),
		NodeID:            nodeID.NodeID,
		OperatorWallet:    nodeWallet,
		SequenceNumber:    1,
		Timestamp:         time.Now().UTC(),
		SoftwareVersion:   "0.1.0",
		TestServerID:      "http://127.0.0.1:8080",
		MeasurementMethod: "HTTP_SPEEDTEST_V1",
		Metrics: report.Metrics{
			DownloadMbps:       124.5,
			UploadMbps:         45.2,
			LatencyMs:          14.2,
			MinLatencyMs:       12.1,
			MaxLatencyMs:       16.8,
			JitterMs:           1.4,
			PacketLossPct:      0.0,
			ActiveTestBytesIn:  10485760, // 10 MB
			ActiveTestBytesOut: 5242880,  // 5 MB
			ServedBytesIn:      0,
			ServedBytesOut:     0,
			TestDurationMs:     4200,
		},
		InterfaceCounters: system.InterfaceCounters{
			InterfaceName: "Ethernet",
			BytesRecv:     50000000,
			BytesSent:     20000000,
		},
		SystemMetrics: system.SystemMetrics{
			CPUPercent: 12.0,
		},
	}
	if err := rep.Sign(nodeID); err != nil {
		t.Fatalf("failed to sign report: %v", err)
	}

	// 4. Process Report
	res := svc.VerifyReport(rep)
	if res.Status != StatusAccepted {
		t.Fatalf("expected report to be ACCEPTED, got %s (code: %s, details: %s)",
			res.Status, res.RejectionCode, res.Details)
	}
	if res.ConfidenceScore < 0.90 {
		t.Fatalf("expected high confidence score, got %.2f", res.ConfidenceScore)
	}
	if res.SettlementVoucher == nil {
		t.Fatalf("expected settlement voucher to be issued")
	}

	// 5. Verify Cryptographic Settlement Voucher
	if err := VerifyVoucher(res.SettlementVoucher, verifierID.PublicKeyHex()); err != nil {
		t.Fatalf("settlement voucher verification failed: %v", err)
	}
	if res.SettlementVoucher.NodeID != nodeID.NodeID {
		t.Fatalf("voucher node ID mismatch: %s != %s", res.SettlementVoucher.NodeID, nodeID.NodeID)
	}
}

func TestDisputeLifecycle(t *testing.T) {
	verifierID, _ := identity.GenerateNewIdentity("0x0000000000000000000000000000000000000001")
	svc := NewService(verifierID, DefaultPipelineConfig())

	nodeWallet := "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"
	reportID := report.GenerateReportID()

	disp, err := svc.Store().FileDispute(reportID, "node-1", nodeWallet, "ISP maintenance window caused latency spike")
	if err != nil {
		t.Fatalf("failed to file dispute: %v", err)
	}
	if disp.Status != "OPEN" {
		t.Fatalf("expected OPEN status, got %s", disp.Status)
	}

	// Resolve dispute
	resolved, err := svc.Store().ResolveDispute(disp.DisputeID, "RESOLVED", "ISP maintenance logs verified; score restored")
	if err != nil {
		t.Fatalf("failed to resolve dispute: %v", err)
	}
	if resolved.Status != "RESOLVED" || resolved.ResolvedAt == nil {
		t.Fatalf("dispute resolution state invalid")
	}
}
