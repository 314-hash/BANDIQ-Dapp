package measure

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sync"
	"time"

	"bandiq-node-agent/internal/config"
	"bandiq-node-agent/internal/consent"
	"bandiq-node-agent/internal/report"
)

var (
	ErrDailyCapExceeded    = errors.New("daily data cap exceeded: benchmark skipped to protect operator bandwidth")
	ErrMeasurementDisabled = errors.New("active measurement is disabled in configuration")
	ErrNoServerAvailable   = errors.New("no test server available or reachable")
)

// Engine conducts bandwidth, latency, jitter, and packet loss measurements.
type Engine struct {
	mu            sync.Mutex
	httpClient    *http.Client
	dailyBytesIn  uint64
	dailyBytesOut uint64
	lastResetDay  int
}

// NewEngine creates a new measurement engine.
func NewEngine() *Engine {
	return &Engine{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		lastResetDay: time.Now().UTC().Day(),
	}
}

// checkDailyCapAndAdd verifies and updates daily data consumption.
func (e *Engine) checkDailyCapAndAdd(estBytes uint64, capMB int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	currentDay := time.Now().UTC().Day()
	if currentDay != e.lastResetDay {
		e.dailyBytesIn = 0
		e.dailyBytesOut = 0
		e.lastResetDay = currentDay
	}

	capBytes := uint64(capMB) * 1024 * 1024
	if (e.dailyBytesIn + e.dailyBytesOut + estBytes) > capBytes {
		return ErrDailyCapExceeded
	}

	return nil
}

func (e *Engine) recordUsedBytes(bytesIn, bytesOut uint64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.dailyBytesIn += bytesIn
	e.dailyBytesOut += bytesOut
}

// RunBenchmark conducts a full synthetic benchmark test against a designated test server.
func (e *Engine) RunBenchmark(
	ctx context.Context,
	serverURL string,
	cfg *config.MeasurementConfig,
	consentMgr *consent.Manager,
) (*report.Metrics, error) {
	// Security: Explicit operator consent is mandatory
	if consentMgr == nil || !consentMgr.CanRunActiveTests() {
		return nil, consent.ErrConsentNotGranted
	}

	if !cfg.Enabled {
		return nil, ErrMeasurementDisabled
	}

	// Protect operator bandwidth with daily cap check
	estBytes := uint64(cfg.MaxDataPerTestMB) * 1024 * 1024
	if err := e.checkDailyCapAndAdd(estBytes, cfg.DailyDataCapMB); err != nil {
		return nil, err
	}

	startTime := time.Now()

	// 1. Latency, Jitter, and Packet Loss
	pingURL := fmt.Sprintf("%s/api/v1/ping", serverURL)
	latencyStats, err := e.measureLatency(ctx, pingURL, 5)
	if err != nil {
		return nil, fmt.Errorf("latency probe failed: %w", err)
	}

	// 2. Download Throughput
	dlTargetMB := cfg.MaxDataPerTestMB / 2
	if dlTargetMB < 1 {
		dlTargetMB = 1
	}
	dlURL := fmt.Sprintf("%s/api/v1/speedtest/download?size=%d", serverURL, dlTargetMB)
	dlMbps, dlBytes, err := e.measureDownload(ctx, dlURL, time.Duration(cfg.MaxTestDurationSec)*time.Second, uint64(dlTargetMB)*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("download test failed: %w", err)
	}

	// 3. Upload Throughput
	ulTargetMB := cfg.MaxDataPerTestMB / 2
	if ulTargetMB < 1 {
		ulTargetMB = 1
	}
	ulURL := fmt.Sprintf("%s/api/v1/speedtest/upload", serverURL)
	ulMbps, ulBytes, err := e.measureUpload(ctx, ulURL, time.Duration(cfg.MaxTestDurationSec)*time.Second, uint64(ulTargetMB)*1024*1024)
	if err != nil {
		return nil, fmt.Errorf("upload test failed: %w", err)
	}

	// Record used bytes against daily cap
	e.recordUsedBytes(dlBytes, ulBytes)

	totalDuration := time.Since(startTime).Milliseconds()

	return &report.Metrics{
		DownloadMbps:       roundTo2(dlMbps),
		UploadMbps:         roundTo2(ulMbps),
		LatencyMs:          roundTo2(latencyStats.avgMs),
		MinLatencyMs:       roundTo2(latencyStats.minMs),
		MaxLatencyMs:       roundTo2(latencyStats.maxMs),
		JitterMs:           roundTo2(latencyStats.jitterMs),
		PacketLossPct:      roundTo2(latencyStats.lossPct),
		ActiveTestBytesIn:  dlBytes,
		ActiveTestBytesOut: ulBytes,
		ServedBytesIn:      0, // In local measurement-only mode, relay is 0
		ServedBytesOut:     0,
		TestDurationMs:     totalDuration,
	}, nil
}

type latencyResult struct {
	minMs    float64
	avgMs    float64
	maxMs    float64
	jitterMs float64
	lossPct  float64
}

// measureLatency runs multiple probes to calculate RTT, RFC 3550 jitter, and packet loss.
func (e *Engine) measureLatency(ctx context.Context, pingURL string, count int) (latencyResult, error) {
	var samples []float64
	failed := 0

	for i := 0; i < count; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pingURL, nil)
		if err != nil {
			failed++
			continue
		}

		t0 := time.Now()
		resp, err := e.httpClient.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			failed++
			if resp != nil {
				resp.Body.Close()
			}
			continue
		}
		resp.Body.Close()

		rtt := float64(time.Since(t0).Microseconds()) / 1000.0 // ms
		samples = append(samples, rtt)
		time.Sleep(30 * time.Millisecond) // Short spacing
	}

	lossPct := (float64(failed) / float64(count)) * 100.0
	if len(samples) == 0 {
		return latencyResult{lossPct: lossPct}, fmt.Errorf("all %d latency probes failed", count)
	}

	minMs := samples[0]
	maxMs := samples[0]
	sum := 0.0
	for _, s := range samples {
		if s < minMs {
			minMs = s
		}
		if s > maxMs {
			maxMs = s
		}
		sum += s
	}
	avgMs := sum / float64(len(samples))

	// RFC 3550 statistical jitter calculation
	jitter := 0.0
	for i := 1; i < len(samples); i++ {
		d := math.Abs(samples[i] - samples[i-1])
		jitter += (d - jitter) / 16.0
	}

	return latencyResult{
		minMs:    minMs,
		avgMs:    avgMs,
		maxMs:    maxMs,
		jitterMs: jitter,
		lossPct:  lossPct,
	}, nil
}

// measureDownload downloads payload from test server and measures speed.
func (e *Engine) measureDownload(ctx context.Context, dlURL string, maxDuration time.Duration, maxBytes uint64) (float64, uint64, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, maxDuration)
	defer cancel()

	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodGet, dlURL, nil)
	if err != nil {
		return 0, 0, err
	}

	t0 := time.Now()
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("download endpoint returned status %d", resp.StatusCode)
	}

	buf := make([]byte, 64*1024) // 64KB buffer
	var totalRead uint64

	for {
		n, rErr := resp.Body.Read(buf)
		if n > 0 {
			totalRead += uint64(n)
			if maxBytes > 0 && totalRead >= maxBytes {
				break
			}
		}
		if rErr != nil {
			if rErr == io.EOF || errors.Is(rErr, context.DeadlineExceeded) || errors.Is(rErr, context.Canceled) {
				break
			}
			return 0, totalRead, rErr
		}
	}

	elapsedSec := time.Since(t0).Seconds()
	if elapsedSec <= 0.001 {
		elapsedSec = 0.001
	}

	mbps := (float64(totalRead) * 8.0) / (elapsedSec * 1000000.0)
	return mbps, totalRead, nil
}

// measureUpload uploads dummy data to the test server and measures speed.
func (e *Engine) measureUpload(ctx context.Context, ulURL string, maxDuration time.Duration, targetBytes uint64) (float64, uint64, error) {
	ctxTimeout, cancel := context.WithTimeout(ctx, maxDuration)
	defer cancel()

	reader := io.LimitReader(rand.Reader, int64(targetBytes))

	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodPost, ulURL, reader)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(targetBytes)

	t0 := time.Now()
	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		return 0, 0, fmt.Errorf("upload endpoint returned status %d", resp.StatusCode)
	}

	elapsedSec := time.Since(t0).Seconds()
	if elapsedSec <= 0.001 {
		elapsedSec = 0.001
	}

	mbps := (float64(targetBytes) * 8.0) / (elapsedSec * 1000000.0)
	return mbps, targetBytes, nil
}

func roundTo2(val float64) float64 {
	return math.Round(val*100) / 100
}
