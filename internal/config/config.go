package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bandiq-node-agent/internal/identity"
)

const (
	DefaultChainID      = 97453
	DefaultTokenAddress = "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"
	DefaultRPCURL       = "https://rpc.sidrachain.com"
	DefaultBackendURL   = "http://127.0.0.1:8080"
	DefaultAdminAddr    = "127.0.0.1:8725"
	DefaultSoftwareVer  = "0.1.0"
)

var (
	ErrInvalidConfig = errors.New("invalid agent configuration")
)

// Config encapsulates all configuration for the BANDIQ node agent.
type Config struct {
	SoftwareVersion string            `json:"software_version"`
	DataDir         string            `json:"data_dir"`
	LogLevel        string            `json:"log_level"`
	Operator        OperatorConfig    `json:"operator"`
	Network         NetworkConfig     `json:"network"`
	Measurement     MeasurementConfig `json:"measurement"`
	Backend         BackendConfig     `json:"backend"`
	Admin           AdminConfig       `json:"admin"`
}

type OperatorConfig struct {
	WalletAddress string `json:"wallet_address"`
	ConsentGiven  bool   `json:"consent_given"`
}

type NetworkConfig struct {
	TargetChainID        uint64 `json:"target_chain_id"`
	TokenContractAddress string `json:"token_contract_address"`
	RPCURL               string `json:"rpc_url"`
	VerifyChainOnStartup bool   `json:"verify_chain_on_startup"`
}

type MeasurementConfig struct {
	Enabled               bool     `json:"enabled"`
	TestIntervalSec       int      `json:"test_interval_sec"`
	MaxTestDurationSec    int      `json:"max_test_duration_sec"`
	MaxDataPerTestMB      int      `json:"max_data_per_test_mb"`
	DailyDataCapMB        int      `json:"daily_data_cap_mb"`
	MaxBandwidthLimitMbps float64  `json:"max_bandwidth_limit_mbps"`
	TestServers           []string `json:"test_servers"`
}

type BackendConfig struct {
	BaseURL           string `json:"base_url"`
	SubmitReports     bool   `json:"submit_reports"`
	ReportIntervalSec int    `json:"report_interval_sec"`
	SpoolDir          string `json:"spool_dir"`
	MaxSpoolReports   int    `json:"max_spool_reports"`
}

type AdminConfig struct {
	Enabled     bool   `json:"enabled"`
	BindAddress string `json:"bind_address"`
	AuthToken   string `json:"auth_token,omitempty"`
}

// DefaultConfig generates a sane, safety-first configuration.
func DefaultConfig(dataDir string) *Config {
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			dataDir = ".bandiq"
		} else {
			dataDir = filepath.Join(homeDir, ".bandiq")
		}
	}

	return &Config{
		SoftwareVersion: DefaultSoftwareVer,
		DataDir:         dataDir,
		LogLevel:        "info",
		Operator: OperatorConfig{
			WalletAddress: "",
			ConsentGiven:  false,
		},
		Network: NetworkConfig{
			TargetChainID:        DefaultChainID,
			TokenContractAddress: DefaultTokenAddress,
			RPCURL:               DefaultRPCURL,
			VerifyChainOnStartup: true,
		},
		Measurement: MeasurementConfig{
			Enabled:               true,
			TestIntervalSec:       900, // Every 15 minutes
			MaxTestDurationSec:    10,  // 10 seconds max per test
			MaxDataPerTestMB:      25,  // 25 MB max per test
			DailyDataCapMB:        1000, // 1 GB daily cap
			MaxBandwidthLimitMbps: 50.0, // 50 Mbps throttle
			TestServers:           []string{"http://127.0.0.1:8080"},
		},
		Backend: BackendConfig{
			BaseURL:           DefaultBackendURL,
			SubmitReports:     true,
			ReportIntervalSec: 60,
			SpoolDir:          filepath.Join(dataDir, "spool"),
			MaxSpoolReports:   500,
		},
		Admin: AdminConfig{
			Enabled:     true,
			BindAddress: DefaultAdminAddr,
		},
	}
}

// Validate checks configuration sanity and enforces security invariants.
func (c *Config) Validate() error {
	if c.Operator.WalletAddress != "" {
		if err := identity.ValidateWallet(c.Operator.WalletAddress); err != nil {
			return fmt.Errorf("invalid operator wallet: %w", err)
		}
	}

	if c.Network.TargetChainID != DefaultChainID {
		return fmt.Errorf("invalid target chain ID %d (must be %d for Sidra Chain)", c.Network.TargetChainID, DefaultChainID)
	}

	if err := identity.ValidateWallet(c.Network.TokenContractAddress); err != nil {
		return fmt.Errorf("invalid token contract address: %w", err)
	}

	if c.Measurement.MaxTestDurationSec <= 0 || c.Measurement.MaxTestDurationSec > 60 {
		return fmt.Errorf("max test duration must be between 1 and 60 seconds (got %d)", c.Measurement.MaxTestDurationSec)
	}

	if c.Measurement.MaxDataPerTestMB <= 0 || c.Measurement.MaxDataPerTestMB > 500 {
		return fmt.Errorf("max data per test must be between 1 and 500 MB (got %d)", c.Measurement.MaxDataPerTestMB)
	}

	if c.Measurement.DailyDataCapMB <= 0 {
		return fmt.Errorf("daily data cap must be positive (got %d)", c.Measurement.DailyDataCapMB)
	}

	// Security: Admin interface MUST bind to localhost by default
	if c.Admin.Enabled {
		host, _, err := net.SplitHostPort(c.Admin.BindAddress)
		if err != nil {
			return fmt.Errorf("invalid admin bind address: %w", err)
		}
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return fmt.Errorf("security violation: admin bind address %s is not a loopback address (must be 127.0.0.1 or localhost)", c.Admin.BindAddress)
		}
	}

	if c.Backend.BaseURL != "" {
		if _, err := url.ParseRequestURI(c.Backend.BaseURL); err != nil {
			return fmt.Errorf("invalid backend base url: %w", err)
		}
	}

	return nil
}

// LoadFromFile loads configuration from a JSON file.
func LoadFromFile(filePath string) (*Config, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig("")
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Apply environment variable overrides
	cfg.ApplyEnvOverrides()

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// SaveToFile commits configuration to disk.
func (c *Config) SaveToFile(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(filePath, data, 0644)
}

// ApplyEnvOverrides overrides settings using BANDIQ_* environment variables.
func (c *Config) ApplyEnvOverrides() {
	if val := os.Getenv("BANDIQ_OPERATOR_WALLET"); val != "" {
		c.Operator.WalletAddress = strings.TrimSpace(val)
	}
	if val := os.Getenv("BANDIQ_BACKEND_URL"); val != "" {
		c.Backend.BaseURL = strings.TrimSpace(val)
	}
	if val := os.Getenv("BANDIQ_SIDRA_RPC_URL"); val != "" {
		c.Network.RPCURL = strings.TrimSpace(val)
	}
	if val := os.Getenv("BANDIQ_DATA_DIR"); val != "" {
		c.DataDir = strings.TrimSpace(val)
		c.Backend.SpoolDir = filepath.Join(c.DataDir, "spool")
	}
	if val := os.Getenv("BANDIQ_LOG_LEVEL"); val != "" {
		c.LogLevel = strings.TrimSpace(val)
	}
}

// TestInterval returns duration from seconds.
func (c *Config) TestInterval() time.Duration {
	return time.Duration(c.Measurement.TestIntervalSec) * time.Second
}

// MaxTestDuration returns duration from seconds.
func (c *Config) MaxTestDuration() time.Duration {
	return time.Duration(c.Measurement.MaxTestDurationSec) * time.Second
}
