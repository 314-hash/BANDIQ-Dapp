package report

import (
	"testing"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/system"
)

func TestReportSigningAndVerification(t *testing.T) {
	wallet := "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"
	id, err := identity.GenerateNewIdentity(wallet)
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	rep := &MeasurementReport{
		ReportID:          GenerateReportID(),
		SequenceNumber:    1,
		Timestamp:         time.Now().UTC(),
		SoftwareVersion:   "0.1.0",
		TestServerID:      "srv-alpha-1",
		MeasurementMethod: "HTTP_SPEEDTEST_V1",
		Metrics: Metrics{
			DownloadMbps:       85.4,
			UploadMbps:         32.1,
			LatencyMs:          18.5,
			MinLatencyMs:       16.2,
			MaxLatencyMs:       22.1,
			JitterMs:           2.4,
			PacketLossPct:      0.0,
			ActiveTestBytesIn:  12582912, // ~12MB
			ActiveTestBytesOut: 4194304,  // ~4MB
			ServedBytesIn:      0,
			ServedBytesOut:     0,
			TestDurationMs:     8500,
		},
		InterfaceCounters: system.InterfaceCounters{
			InterfaceName: "Ethernet",
			BytesRecv:     50000000,
			BytesSent:     10000000,
		},
		SystemMetrics: system.SystemMetrics{
			UptimeSec:     12000,
			CPUPercent:    5.2,
			MemoryTotalMB: 8192,
			MemoryUsedMB:  4096,
		},
	}

	// Sign the report
	if err := rep.Sign(id); err != nil {
		t.Fatalf("failed to sign report: %v", err)
	}

	if rep.Signature == "" {
		t.Fatalf("expected signature to be populated")
	}

	// Verify with valid public key
	if err := rep.Verify(id.PublicKeyHex()); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}

	// Validate report constraints
	if err := rep.Validate(5 * time.Minute); err != nil {
		t.Fatalf("report validation failed: %v", err)
	}

	// Verify tamper resistance: modify throughput metric
	rep.Metrics.DownloadMbps = 9999.9
	if err := rep.Verify(id.PublicKeyHex()); err == nil {
		t.Fatalf("tampered report should have failed verification")
	}
}
