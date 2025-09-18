package sign

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

// VaultSigner handles artifact signing using HashiCorp Vault
type VaultSigner struct {
	vaultAddr    string
	signerPath   string
	client       *http.Client
	logger       *slog.Logger
	devMode      bool
}

// NewVaultSigner creates a new Vault signer
func NewVaultSigner(logger *slog.Logger) (*VaultSigner, error) {
	vaultAddr := getEnv("VAULT_ADDR", "http://localhost:8200")
	signerPath := getEnv("BPF_REGISTRY_SIGNER_PATH", "secret/data/bpf-registry/signer")
	
	// Check if we're in development mode (no real Vault)
	devMode := getEnv("VAULT_DEV_MODE", "true") == "true"
	
	signer := &VaultSigner{
		vaultAddr:  vaultAddr,
		signerPath: signerPath,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger:  logger,
		devMode: devMode,
	}
	
	logger.Info("Vault signer initialized", 
		"vault_addr", vaultAddr,
		"signer_path", signerPath,
		"dev_mode", devMode)
	
	return signer, nil
}

// Sign signs the given data and returns a base64-encoded signature
func (v *VaultSigner) Sign(data []byte) (string, error) {
	if v.devMode {
		return v.signDevMode(data)
	}
	
	return v.signProduction(data)
}

// signDevMode creates a deterministic signature for development/testing
func (v *VaultSigner) signDevMode(data []byte) (string, error) {
	// Create a deterministic signature using SHA256 hash
	hash := sha256.Sum256(data)
	
	// Add timestamp for uniqueness but make it deterministic for testing
	timestamp := time.Now().Unix() / 3600 // Round to hour for deterministic testing
	
	// Create a fake signature that's deterministic for the same data
	signatureData := fmt.Sprintf("dev_signature_%x_%d", hash, timestamp)
	signatureHash := sha256.Sum256([]byte(signatureData))
	
	signature := base64.StdEncoding.EncodeToString(signatureHash[:])
	
	v.logger.Debug("Generated dev mode signature", 
		"data_size", len(data),
		"signature", signature[:16]+"...") // Log first 16 chars only
	
	return signature, nil
}

// signProduction signs data using real Vault (future implementation)
func (v *VaultSigner) signProduction(data []byte) (string, error) {
	// TODO: Implement real Vault signing
	// This would involve:
	// 1. Authenticating with Vault (using token or other auth method)
	// 2. Reading the signing key from the specified path
	// 3. Using the key to sign the data
	// 4. Returning the base64-encoded signature
	
	v.logger.Warn("Production Vault signing not yet implemented, using dev mode fallback")
	return v.signDevMode(data)
}

// Verify verifies a signature against the given data
func (v *VaultSigner) Verify(data []byte, signature string) (bool, error) {
	// For now, just verify that we can generate the same signature
	expectedSignature, err := v.Sign(data)
	if err != nil {
		return false, err
	}
	
	return expectedSignature == signature, nil
}

// getEnv gets an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// MockVaultSigner creates a mock signer for testing with deterministic signatures
type MockVaultSigner struct {
	logger *slog.Logger
}

// NewMockVaultSigner creates a new mock Vault signer for testing
func NewMockVaultSigner(logger *slog.Logger) *MockVaultSigner {
	return &MockVaultSigner{
		logger: logger,
	}
}

// Sign creates a deterministic signature for testing
func (m *MockVaultSigner) Sign(data []byte) (string, error) {
	// Create a deterministic signature based on data content
	hash := sha256.Sum256(data)
	
	// Use a fixed timestamp for deterministic testing
	testTimestamp := int64(1234567890)
	
	// Create deterministic signature
	signatureData := fmt.Sprintf("test_signature_%x_%d", hash, testTimestamp)
	signatureHash := sha256.Sum256([]byte(signatureData))
	
	signature := base64.StdEncoding.EncodeToString(signatureHash[:])
	
	m.logger.Debug("Generated mock signature", 
		"data_size", len(data),
		"signature", signature[:16]+"...")
	
	return signature, nil
}

// Verify verifies a signature (always returns true for mock)
func (m *MockVaultSigner) Verify(data []byte, signature string) (bool, error) {
	expectedSignature, err := m.Sign(data)
	if err != nil {
		return false, err
	}
	
	return expectedSignature == signature, nil
}
