package consent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	CurrentTermsVersion = "1.0.0"
	ConsentFileName     = "consent.json"
)

var (
	ErrConsentNotGranted = errors.New("explicit operator consent has not been granted: network measurement and traffic relay are disabled")
)

// Record holds the explicit consent granted by the node operator.
type Record struct {
	Granted           bool      `json:"granted"`
	GrantedAt         time.Time `json:"granted_at"`
	TermsVersion      string    `json:"terms_version"`
	OperatorWallet    string    `json:"operator_wallet"`
	NodeID            string    `json:"node_id"`
	AllowActiveTests  bool      `json:"allow_active_tests"`
	AllowTrafficRelay bool      `json:"allow_traffic_relay"`
	Notes             string    `json:"notes,omitempty"`
}

// Manager oversees consent status and persistence.
type Manager struct {
	mu       sync.RWMutex
	filePath string
	record   Record
}

// NewManager initializes a consent manager tied to a data directory.
func NewManager(dataDir string) (*Manager, error) {
	filePath := filepath.Join(dataDir, ConsentFileName)
	m := &Manager{
		filePath: filePath,
	}

	if err := m.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load consent record: %w", err)
	}

	return m, nil
}

func (m *Manager) load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return err
	}

	var rec Record
	if err := json.Unmarshal(data, &rec); err != nil {
		return fmt.Errorf("corrupt consent file: %w", err)
	}

	m.record = rec
	return nil
}

// Save commits the consent record to disk atomically.
func (m *Manager) saveLocked() error {
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory for consent: %w", err)
	}

	data, err := json.MarshalIndent(m.record, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal consent record: %w", err)
	}

	tmpFile := m.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write tmp consent file: %w", err)
	}

	if err := os.Rename(tmpFile, m.filePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to commit consent file: %w", err)
	}

	return nil
}

// Grant records explicit consent by the operator.
func (m *Manager) Grant(operatorWallet, nodeID string, allowActiveTests, allowTrafficRelay bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.record = Record{
		Granted:           true,
		GrantedAt:         time.Now().UTC(),
		TermsVersion:      CurrentTermsVersion,
		OperatorWallet:    operatorWallet,
		NodeID:            nodeID,
		AllowActiveTests:  allowActiveTests,
		AllowTrafficRelay: allowTrafficRelay,
		Notes:             "Explicit consent given by operator for BANDIQ network telemetry.",
	}

	return m.saveLocked()
}

// Revoke revokes operator consent immediately and disables all network tasks.
func (m *Manager) Revoke() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.record = Record{
		Granted:           false,
		GrantedAt:         time.Now().UTC(),
		TermsVersion:      CurrentTermsVersion,
		AllowActiveTests:  false,
		AllowTrafficRelay: false,
		Notes:             "Consent revoked by operator.",
	}

	return m.saveLocked()
}

// Status returns a copy of the current consent record.
func (m *Manager) Status() Record {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.record
}

// HasConsent returns true if valid consent has been granted.
func (m *Manager) HasConsent() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.record.Granted
}

// CanRunActiveTests checks if synthetic bandwidth benchmarks are permitted.
func (m *Manager) CanRunActiveTests() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.record.Granted && m.record.AllowActiveTests
}

// CanRelayTraffic checks if traffic forwarding / serving is explicitly permitted.
func (m *Manager) CanRelayTraffic() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.record.Granted && m.record.AllowTrafficRelay
}
