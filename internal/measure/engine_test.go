package measure

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"

	"bandiq-node-agent/internal/config"
	"bandiq-node-agent/internal/consent"
)

func TestMeasurementEngineFullFlow(t *testing.T) {
	// Setup mock test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/ping":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("pong"))

		case "/api/v1/speedtest/download":
			sizeMBStr := r.URL.Query().Get("size")
			sizeMB, _ := strconv.Atoi(sizeMBStr)
			if sizeMB <= 0 {
				sizeMB = 1
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			chunk := make([]byte, 32*1024) // 32KB
			total := sizeMB * 1024 * 1024
			written := 0
			for written < total {
				toWrite := len(chunk)
				if total-written < toWrite {
					toWrite = total - written
				}
				_, _ = w.Write(chunk[:toWrite])
				written += toWrite
			}

		case "/api/v1/speedtest/upload":
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "bandiq-measure-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create consent manager with consent granted
	consentMgr, err := consent.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to create consent manager: %v", err)
	}
	_ = consentMgr.Grant("0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133", "node-1", true, false)

	cfg := &config.MeasurementConfig{
		Enabled:            true,
		MaxTestDurationSec: 5,
		MaxDataPerTestMB:   2,
		DailyDataCapMB:     50,
	}

	engine := NewEngine()
	metrics, err := engine.RunBenchmark(context.Background(), server.URL, cfg, consentMgr)
	if err != nil {
		t.Fatalf("benchmark failed: %v", err)
	}

	if metrics.DownloadMbps <= 0 {
		t.Fatalf("expected positive download speed, got %.2f", metrics.DownloadMbps)
	}
	if metrics.UploadMbps <= 0 {
		t.Fatalf("expected positive upload speed, got %.2f", metrics.UploadMbps)
	}
	if metrics.ActiveTestBytesIn == 0 || metrics.ActiveTestBytesOut == 0 {
		t.Fatalf("expected active test bytes recorded")
	}
	// Verify strict distinction: served bytes MUST be 0 in test mode
	if metrics.ServedBytesIn != 0 || metrics.ServedBytesOut != 0 {
		t.Fatalf("served bytes should be 0 during synthetic benchmark")
	}

	t.Logf("Benchmark Result: Down=%.2f Mbps, Up=%.2f Mbps, Latency=%.2f ms, Jitter=%.2f ms, Loss=%.1f%%, ActiveIn=%d, ActiveOut=%d",
		metrics.DownloadMbps, metrics.UploadMbps, metrics.LatencyMs, metrics.JitterMs, metrics.PacketLossPct,
		metrics.ActiveTestBytesIn, metrics.ActiveTestBytesOut)

	// Test consent check: if consent revoked, benchmark must fail
	_ = consentMgr.Revoke()
	_, err = engine.RunBenchmark(context.Background(), server.URL, cfg, consentMgr)
	if err == nil {
		t.Fatalf("expected error when consent is revoked, got nil")
	}
}
