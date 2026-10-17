package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigValidation(t *testing.T) {
	cfg := DefaultConfig("/tmp/bandiq-test")
	cfg.Operator.WalletAddress = "0x6d1537bEE4F1EE3f55F4BC9319Aa40252cd39133"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected default config to be valid: %v", err)
	}

	// Test invalid chain ID
	cfg.Network.TargetChainID = 1
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for non-Sidra chain ID, got nil")
	}
	cfg.Network.TargetChainID = DefaultChainID

	// Test admin security violation: binding to non-loopback
	cfg.Admin.BindAddress = "0.0.0.0:8725"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for binding admin to 0.0.0.0, got nil")
	}
	cfg.Admin.BindAddress = "192.168.1.50:8725"
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected error for binding admin to non-loopback IP, got nil")
	}
	cfg.Admin.BindAddress = DefaultAdminAddr
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid admin loopback: %v", err)
	}
}

func TestConfigPersistenceAndEnv(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bandiq-cfg-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgFile := filepath.Join(tmpDir, "config.json")
	cfg := DefaultConfig(tmpDir)
	cfg.Operator.WalletAddress = "0x6d1537bee4f1ee3f55f4bc9319aa40252cd39133"
	cfg.Measurement.MaxDataPerTestMB = 15

	if err := cfg.SaveToFile(cfgFile); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	loaded, err := LoadFromFile(cfgFile)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	if loaded.Measurement.MaxDataPerTestMB != 15 {
		t.Fatalf("expected MaxDataPerTestMB 15, got %d", loaded.Measurement.MaxDataPerTestMB)
	}

	// Test env override
	os.Setenv("BANDIQ_OPERATOR_WALLET", "0x0000000000000000000000000000000000000001")
	defer os.Unsetenv("BANDIQ_OPERATOR_WALLET")

	loaded.ApplyEnvOverrides()
	if loaded.Operator.WalletAddress != "0x0000000000000000000000000000000000000001" {
		t.Fatalf("env override failed")
	}
}
