package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/report"
	"bandiq-node-agent/internal/spool"
)

var (
	ErrBackendUnreachable = errors.New("backend service unreachable")
	ErrReplayDetected     = errors.New("backend rejected report: potential replay detected")
	ErrRegistrationFailed = errors.New("failed to register node with backend")
)

// RegistrationRequest holds the payload to register a node with the BANDIQ backend.
type RegistrationRequest struct {
	NodeID          string    `json:"node_id"`
	OperatorWallet  string    `json:"operator_wallet"`
	PublicKeyHex    string    `json:"public_key_hex"`
	SoftwareVersion string    `json:"software_version"`
	RegisteredAt    time.Time `json:"registered_at"`
	Signature       string    `json:"signature"`
}

// Client manages authenticated HTTP interactions with the BANDIQ backend and manages offline spooling.
type Client struct {
	mu         sync.Mutex
	baseURL    string
	identity   *identity.Identity
	spoolQueue *spool.Queue
	httpClient *http.Client
}

// NewClient initializes the authenticated backend client.
func NewClient(baseURL string, id *identity.Identity, spoolQueue *spool.Queue) *Client {
	return &Client{
		baseURL:    baseURL,
		identity:   id,
		spoolQueue: spoolQueue,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// RegisterNode informs the backend about this node's public key and operator association.
func (c *Client) RegisterNode(ctx context.Context, softwareVersion string) error {
	now := time.Now().UTC()
	msg := fmt.Sprintf("bandiq-register:%s:%s:%s", c.identity.NodeID, c.identity.OperatorWallet, now.Format(time.RFC3339))
	sig := c.identity.SignHex([]byte(msg))

	reqPayload := RegistrationRequest{
		NodeID:          c.identity.NodeID,
		OperatorWallet:  c.identity.OperatorWallet,
		PublicKeyHex:    c.identity.PublicKeyHex(),
		SoftwareVersion: softwareVersion,
		RegisteredAt:    now,
		Signature:       sig,
	}

	data, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/v1/nodes/register", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BANDIQ-Node-ID", c.identity.NodeID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBackendUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%w: status %d (%s)", ErrRegistrationFailed, resp.StatusCode, string(body))
	}

	return nil
}

// SubmitReport sends a signed report to the backend. If backend is unavailable, it spools locally.
func (c *Client) SubmitReport(ctx context.Context, rep *report.MeasurementReport) error {
	data, err := json.Marshal(rep)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/reports/submit", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BANDIQ-Node-ID", rep.NodeID)
	req.Header.Set("X-BANDIQ-Sequence", fmt.Sprintf("%d", rep.SequenceNumber))
	req.Header.Set("X-BANDIQ-Signature", rep.Signature)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Network failed: enqueue in persistent offline queue
		if c.spoolQueue != nil {
			_ = c.spoolQueue.Enqueue(rep)
		}
		return fmt.Errorf("%w: spooled offline (%v)", ErrBackendUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		// Replay or duplicate
		return ErrReplayDetected
	}

	if resp.StatusCode >= 500 {
		// Server error: spool for retry
		if c.spoolQueue != nil {
			_ = c.spoolQueue.Enqueue(rep)
		}
		return fmt.Errorf("backend error %d: spooled for retry", resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected backend status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// DrainSpool processes pending offline reports and flushes them to the backend in FIFO order.
func (c *Client) DrainSpool(ctx context.Context, maxBatch int) (int, error) {
	if c.spoolQueue == nil {
		return 0, nil
	}

	flushed := 0
	for i := 0; i < maxBatch; i++ {
		rep, err := c.spoolQueue.Peek()
		if err != nil {
			if errors.Is(err, spool.ErrSpoolEmpty) {
				break
			}
			return flushed, err
		}

		// Attempt submission directly
		data, err := json.Marshal(rep)
		if err != nil {
			// Malformed item, drop it
			_ = c.spoolQueue.Dequeue(rep.ReportID)
			continue
		}

		url := fmt.Sprintf("%s/api/v1/reports/submit", c.baseURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return flushed, err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-BANDIQ-Node-ID", rep.NodeID)
		req.Header.Set("X-BANDIQ-Sequence", fmt.Sprintf("%d", rep.SequenceNumber))
		req.Header.Set("X-BANDIQ-Signature", rep.Signature)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			// Backend still unreachable, break and retry later
			break
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusAccepted || resp.StatusCode == http.StatusConflict {
			// Accepted or already processed by backend: dequeue
			_ = c.spoolQueue.Dequeue(rep.ReportID)
			flushed++
		} else {
			// 5xx error, stop draining for now
			break
		}
	}

	return flushed, nil
}
