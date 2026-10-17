package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	walletRegex = regexp.MustCompile(`^0x[0-9a-fA-F]{40}$`)

	ErrInvalidWallet    = errors.New("invalid EVM operator wallet address format (must be 0x followed by 40 hex characters)")
	ErrKeyNotFound      = errors.New("node identity key not found")
	ErrSignatureInvalid = errors.New("cryptographic signature verification failed")
)

// KeyStore represents the serialized node key file.
type KeyStore struct {
	NodeID         string    `json:"node_id"`
	PublicKeyHex   string    `json:"public_key_hex"`
	PrivateKeyHex  string    `json:"private_key_hex"`
	OperatorWallet string    `json:"operator_wallet"`
	CreatedAt      time.Time `json:"created_at"`
}

// Identity wraps the node's Ed25519 keypair and operator association.
type Identity struct {
	NodeID         string
	PublicKey      ed25519.PublicKey
	PrivateKey     ed25519.PrivateKey
	OperatorWallet string
	CreatedAt      time.Time
}

// ValidateWallet checks if an EVM address is syntactically valid.
func ValidateWallet(wallet string) error {
	wallet = strings.TrimSpace(wallet)
	if !walletRegex.MatchString(wallet) {
		return ErrInvalidWallet
	}
	return nil
}

// DeriveNodeID creates a human-readable deterministic node identifier from a public key.
func DeriveNodeID(pubKey ed25519.PublicKey) string {
	hash := sha256.Sum256(pubKey)
	return fmt.Sprintf("bandiq-node-%s", hex.EncodeToString(hash[:16]))
}

// GenerateNewIdentity creates a new Ed25519 identity associated with an operator wallet.
func GenerateNewIdentity(operatorWallet string) (*Identity, error) {
	if err := ValidateWallet(operatorWallet); err != nil {
		return nil, err
	}

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ed25519 keypair: %w", err)
	}

	nodeID := DeriveNodeID(pubKey)

	return &Identity{
		NodeID:         nodeID,
		PublicKey:      pubKey,
		PrivateKey:     privKey,
		OperatorWallet: strings.ToLower(operatorWallet),
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// SaveToFile securely persists the keypair to disk.
func (id *Identity) SaveToFile(filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create directory for key: %w", err)
	}

	ks := KeyStore{
		NodeID:         id.NodeID,
		PublicKeyHex:   hex.EncodeToString(id.PublicKey),
		PrivateKeyHex:  hex.EncodeToString(id.PrivateKey),
		OperatorWallet: id.OperatorWallet,
		CreatedAt:      id.CreatedAt,
	}

	data, err := json.MarshalIndent(ks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal keystore: %w", err)
	}

	// Write to temporary file first then atomic rename
	tmpPath := filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write temporary key file: %w", err)
	}

	if err := os.Rename(tmpPath, filePath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit key file: %w", err)
	}

	return nil
}

// LoadFromFile loads an existing keypair from disk.
func LoadFromFile(filePath string) (*Identity, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrKeyNotFound
		}
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var ks KeyStore
	if err := json.Unmarshal(data, &ks); err != nil {
		return nil, fmt.Errorf("failed to parse key file: %w", err)
	}

	pubBytes, err := hex.DecodeString(ks.PublicKeyHex)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key hex in keystore")
	}

	privBytes, err := hex.DecodeString(ks.PrivateKeyHex)
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key hex in keystore")
	}

	if err := ValidateWallet(ks.OperatorWallet); err != nil {
		return nil, fmt.Errorf("keystore contains invalid operator wallet: %w", err)
	}

	return &Identity{
		NodeID:         ks.NodeID,
		PublicKey:      ed25519.PublicKey(pubBytes),
		PrivateKey:     ed25519.PrivateKey(privBytes),
		OperatorWallet: strings.ToLower(ks.OperatorWallet),
		CreatedAt:      ks.CreatedAt,
	}, nil
}

// Sign creates an Ed25519 signature over arbitrary message bytes.
func (id *Identity) Sign(message []byte) []byte {
	return ed25519.Sign(id.PrivateKey, message)
}

// SignHex creates an Ed25519 signature and returns it as a lowercase hex string.
func (id *Identity) SignHex(message []byte) string {
	sig := id.Sign(message)
	return hex.EncodeToString(sig)
}

// PublicKeyHex returns the hex-encoded Ed25519 public key.
func (id *Identity) PublicKeyHex() string {
	return hex.EncodeToString(id.PublicKey)
}

// PrivateKeyHex returns the hex-encoded Ed25519 private key.
func (id *Identity) PrivateKeyHex() string {
	return hex.EncodeToString(id.PrivateKey)
}

// VerifySignature verifies an Ed25519 signature against a public key.
func VerifySignature(pubKey ed25519.PublicKey, message, signature []byte) bool {
	if len(signature) != ed25519.SignatureSize || len(pubKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(pubKey, message, signature)
}

// VerifySignatureHex verifies an Ed25519 hex signature against a hex public key.
func VerifySignatureHex(pubKeyHex string, message []byte, sigHex string) bool {
	pubBytes, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return false
	}
	sigBytes, err := hex.DecodeString(sigHex)
	if err != nil {
		return false
	}
	return VerifySignature(pubBytes, message, sigBytes)
}
