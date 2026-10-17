package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/report"
	"bandiq-node-agent/internal/spool"
)

func TestClientSubmissionAndSpooling(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bandiq-client-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	wallet := "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"
	id, err := identity.GenerateNewIdentity(wallet)
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	queue, err := spool.NewQueue(tmpDir, 100)
	if err != nil {
		t.Fatalf("failed to init spool queue: %v", err)
	}

	var submittedReports int32

	// Mock backend server
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/nodes/register":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"registered"}`))

		case "/api/v1/reports/submit":
			var rep report.MeasurementReport
			if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			// Verify node ID header
			if r.Header.Get("X-BANDIQ-Node-ID") != id.NodeID {
				http.Error(w, "invalid node ID header", http.StatusUnauthorized)
				return
			}
			atomic.AddInt32(&submittedReports, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"accepted"}`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer backend.Close()

	cli := NewClient(backend.URL, id, queue)

	// Test registration
	if err := cli.RegisterNode(context.Background(), "0.1.0"); err != nil {
		t.Fatalf("node registration failed: %v", err)
	}

	// Create test report
	rep1 := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		SequenceNumber: 1,
		Timestamp:      time.Now().UTC(),
		Metrics: report.Metrics{
			DownloadMbps: 120.5,
		},
	}
	_ = rep1.Sign(id)

	// Submit report online
	if err := cli.SubmitReport(context.Background(), rep1); err != nil {
		t.Fatalf("online report submission failed: %v", err)
	}
	if atomic.LoadInt32(&submittedReports) != 1 {
		t.Fatalf("expected 1 submitted report")
	}

	// Test offline spooling: point client to unreachable port
	offlineCli := NewClient("http://127.0.0.1:59999", id, queue)
	rep2 := &report.MeasurementReport{
		ReportID:       report.GenerateReportID(),
		SequenceNumber: 2,
		Timestamp:      time.Now().UTC(),
		Metrics: report.Metrics{
			DownloadMbps: 95.0,
		},
	}
	_ = rep2.Sign(id)

	err = offlineCli.SubmitReport(context.Background(), rep2)
	if err == nil {
		t.Fatalf("expected error from unreachable backend")
	}

	// Verify report 2 was queued on disk
	count, _ := queue.Count()
	if count != 1 {
		t.Fatalf("expected 1 report in spool, got %d", count)
	}

	// Now drain spool against the online backend
	flushed, err := cli.DrainSpool(context.Background(), 10)
	if err != nil {
		t.Fatalf("failed to drain spool: %v", err)
	}
	if flushed != 1 {
		t.Fatalf("expected 1 flushed report, got %d", flushed)
	}
	if atomic.LoadInt32(&submittedReports) != 2 {
		t.Fatalf("expected total 2 reports received by backend")
	}

	// Spool should now be empty
	remaining, _ := queue.Count()
	if remaining != 0 {
		t.Fatalf("expected empty spool after drain, got %d", remaining)
	}
}
