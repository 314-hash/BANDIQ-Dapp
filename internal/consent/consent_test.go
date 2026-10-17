package consent

import (
	"os"
	"testing"
)

func TestConsentLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bandiq-consent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mgr, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to init consent manager: %v", err)
	}

	// Initially should not have consent
	if mgr.HasConsent() {
		t.Fatalf("expected no consent initially")
	}
	if mgr.CanRunActiveTests() {
		t.Fatalf("expected active tests disallowed initially")
	}

	// Grant consent with active tests allowed, relay disallowed
	wallet := "0x6d1537bee4f1ee3f55f4bc9319aa40252cd39133"
	nodeID := "bandiq-node-test-12345"
	if err := mgr.Grant(wallet, nodeID, true, false); err != nil {
		t.Fatalf("failed to grant consent: %v", err)
	}

	if !mgr.HasConsent() {
		t.Fatalf("expected consent granted")
	}
	if !mgr.CanRunActiveTests() {
		t.Fatalf("expected active tests allowed")
	}
	if mgr.CanRelayTraffic() {
		t.Fatalf("expected traffic relay disallowed")
	}

	// Re-load manager from disk to verify persistence
	mgr2, err := NewManager(tmpDir)
	if err != nil {
		t.Fatalf("failed to reload consent manager: %v", err)
	}
	if !mgr2.HasConsent() {
		t.Fatalf("expected reloaded consent granted")
	}
	if mgr2.Status().OperatorWallet != wallet {
		t.Fatalf("expected wallet %s, got %s", wallet, mgr2.Status().OperatorWallet)
	}

	// Revoke consent
	if err := mgr2.Revoke(); err != nil {
		t.Fatalf("failed to revoke consent: %v", err)
	}
	if mgr2.HasConsent() {
		t.Fatalf("expected consent revoked")
	}
	if mgr2.CanRunActiveTests() {
		t.Fatalf("expected active tests disabled after revocation")
	}
}
