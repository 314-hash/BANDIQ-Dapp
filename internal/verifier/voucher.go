package verifier

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"bandiq-node-agent/internal/identity"
	"bandiq-node-agent/internal/report"
)

// CanonicalVoucherPayload defines the deterministic fields hashed to generate the voucher signature.
type CanonicalVoucherPayload struct {
	VoucherID        string `json:"voucher_id"`
	EpochID          uint64 `json:"epoch_id"`
	NodeID           string `json:"node_id"`
	OperatorWallet   string `json:"operator_wallet"`
	VerifiedBytes    uint64 `json:"verified_bytes"`
	QoSScore         uint32 `json:"qos_score"`
	Nonce            uint64 `json:"nonce"`
	IssuedAtRFC3339  string `json:"issued_at_rfc3339"`
	ExpiresAtRFC3339 string `json:"expires_at_rfc3339"`
	VerifierIdentity string `json:"verifier_identity"`
}

// GenerateVoucher creates and cryptographically signs a settlement voucher for an accepted report.
func GenerateVoucher(
	verifierID *identity.Identity,
	epochID uint64,
	nodeID, wallet string,
	verifiedBytes uint64,
	qosScore uint32,
	nonce uint64,
	validityDuration time.Duration,
) (*SettlementVoucher, error) {
	now := time.Now().UTC()
	voucherID := report.GenerateReportID()

	v := &SettlementVoucher{
		VoucherID:        voucherID,
		EpochID:          epochID,
		NodeID:           nodeID,
		OperatorWallet:   wallet,
		VerifiedBytes:    verifiedBytes,
		QoSScore:         qosScore,
		Nonce:            nonce,
		IssuedAt:         now,
		ExpiresAt:        now.Add(validityDuration),
		VerifierIdentity: verifierID.NodeID,
	}

	payload := CanonicalVoucherPayload{
		VoucherID:        v.VoucherID,
		EpochID:          v.EpochID,
		NodeID:           v.NodeID,
		OperatorWallet:   v.OperatorWallet,
		VerifiedBytes:    v.VerifiedBytes,
		QoSScore:         v.QoSScore,
		Nonce:            v.Nonce,
		IssuedAtRFC3339:  v.IssuedAt.Format(time.RFC3339),
		ExpiresAtRFC3339: v.ExpiresAt.Format(time.RFC3339),
		VerifierIdentity: v.VerifierIdentity,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal canonical voucher payload: %w", err)
	}

	hash := sha256.Sum256(data)
	v.Signature = verifierID.SignHex(hash[:])

	return v, nil
}

// VerifyVoucher validates the cryptographic signature of a settlement voucher against the verifier's public key.
func VerifyVoucher(v *SettlementVoucher, verifierPubKeyHex string) error {
	payload := CanonicalVoucherPayload{
		VoucherID:        v.VoucherID,
		EpochID:          v.EpochID,
		NodeID:           v.NodeID,
		OperatorWallet:   v.OperatorWallet,
		VerifiedBytes:    v.VerifiedBytes,
		QoSScore:         v.QoSScore,
		Nonce:            v.Nonce,
		IssuedAtRFC3339:  v.IssuedAt.UTC().Format(time.RFC3339),
		ExpiresAtRFC3339: v.ExpiresAt.UTC().Format(time.RFC3339),
		VerifierIdentity: v.VerifierIdentity,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal voucher for verification: %w", err)
	}

	hash := sha256.Sum256(data)
	if !identity.VerifySignatureHex(verifierPubKeyHex, hash[:], v.Signature) {
		return fmt.Errorf("invalid verifier cryptographic signature on settlement voucher")
	}

	if time.Now().UTC().After(v.ExpiresAt) {
		return fmt.Errorf("settlement voucher has expired")
	}

	return nil
}
