package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"bandiq-node-agent/internal/config"
	"bandiq-node-agent/internal/consent"
	"bandiq-node-agent/internal/report"
	"bandiq-node-agent/internal/spool"
	"bandiq-node-agent/internal/system"
)

func TestAdminServerEndpoints(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bandiq-admin-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := config.DefaultConfig(tmpDir)
	cfg.Admin.BindAddress = "127.0.0.1:28725" // Unique port for test

	consentMgr, _ := consent.NewManager(tmpDir)
	spoolQueue, _ := spool.NewQueue(tmpDir, 50)
	collector := system.NewCollector()

	wallet := "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"
	nodeID := "bandiq-node-test-admin"

	triggerCalled := false
	triggerFn := func() (*report.MeasurementReport, error) {
		triggerCalled = true
		return &report.MeasurementReport{
			ReportID:       report.GenerateReportID(),
			SequenceNumber: 99,
			Timestamp:      time.Now().UTC(),
		}, nil
	}

	server, err := NewServer(cfg, consentMgr, spoolQueue, collector, nodeID, wallet, triggerFn)
	if err != nil {
		t.Fatalf("failed to create admin server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("failed to start admin server: %v", err)
	}
	defer func() { _ = server.Stop(context.Background()) }()

	// Give server a split second to listen
	time.Sleep(50 * time.Millisecond)
	baseURL := fmt.Sprintf("http://%s", cfg.Admin.BindAddress)

	// 1. GET /health
	resp, err := http.Get(baseURL + "/health")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("health check failed: %v", err)
	}
	resp.Body.Close()

	// 2. POST /pause
	resp, err = http.Post(baseURL+"/pause", "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("pause request failed: %v", err)
	}
	resp.Body.Close()
	if !server.IsPaused() {
		t.Fatalf("expected server state to be paused")
	}

	// 3. POST /resume
	resp, err = http.Post(baseURL+"/resume", "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("resume request failed: %v", err)
	}
	resp.Body.Close()
	if server.IsPaused() {
		t.Fatalf("expected server state to be unpaused")
	}

	// 4. POST /caps
	newDailyCap := 5000
	newLimitMbps := 100.0
	capsPayload, _ := json.Marshal(capsRequest{
		DailyDataCapMB:        &newDailyCap,
		MaxBandwidthLimitMbps: &newLimitMbps,
	})
	resp, err = http.Post(baseURL+"/caps", "application/json", bytes.NewReader(capsPayload))
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("caps update failed: %v", err)
	}
	resp.Body.Close()

	if cfg.Measurement.DailyDataCapMB != 5000 {
		t.Fatalf("expected updated daily cap 5000, got %d", cfg.Measurement.DailyDataCapMB)
	}

	// 5. POST /trigger
	resp, err = http.Post(baseURL+"/trigger", "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("trigger failed: %v", err)
	}
	resp.Body.Close()
	if !triggerCalled {
		t.Fatalf("expected trigger function to be executed")
	}

	// 6. GET /status
	resp, err = http.Get(baseURL + "/status")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("status query failed: %v", err)
	}
	resp.Body.Close()
}
