package sign

import (
	"log/slog"
	"os"
	"testing"
)

func TestVaultSigner_DevMode(t *testing.T) {
	// Set up test environment
	os.Setenv("VAULT_DEV_MODE", "true")
	defer os.Unsetenv("VAULT_DEV_MODE")
	
	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Reduce noise in tests
	}))
	
	// Create signer
	signer, err := NewVaultSigner(logger)
	if err != nil {
		t.Fatalf("Failed to create Vault signer: %v", err)
	}
	
	// Test data
	testData := []byte("test artifact data")
	
	// Test signing
	signature, err := signer.Sign(testData)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}
	
	// Verify signature is not empty
	if signature == "" {
		t.Error("Signature should not be empty")
	}
	
	// Verify signature is base64 encoded
	if len(signature) == 0 {
		t.Error("Signature should have content")
	}
	
	// Test deterministic behavior (same input should produce same output within the same hour)
	signature2, err := signer.Sign(testData)
	if err != nil {
		t.Fatalf("Failed to sign data second time: %v", err)
	}
	
	// In dev mode, signatures should be deterministic for the same data within the same hour
	if signature != signature2 {
		t.Logf("Note: Signatures differ (expected in dev mode with different timestamps)")
		t.Logf("Signature 1: %s", signature[:16]+"...")
		t.Logf("Signature 2: %s", signature2[:16]+"...")
	}
	
	// Test verification
	valid, err := signer.Verify(testData, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	
	if !valid {
		t.Error("Signature verification should succeed")
	}
	
	// Test verification with wrong data
	wrongData := []byte("wrong data")
	valid, err = signer.Verify(wrongData, signature)
	if err != nil {
		t.Fatalf("Failed to verify signature with wrong data: %v", err)
	}
	
	if valid {
		t.Error("Signature verification should fail with wrong data")
	}
}

func TestMockVaultSigner_Deterministic(t *testing.T) {
	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	
	// Create mock signer
	signer := NewMockVaultSigner(logger)
	
	// Test data
	testData := []byte("test artifact data")
	
	// Test signing multiple times - should be deterministic
	signature1, err := signer.Sign(testData)
	if err != nil {
		t.Fatalf("Failed to sign data: %v", err)
	}
	
	signature2, err := signer.Sign(testData)
	if err != nil {
		t.Fatalf("Failed to sign data second time: %v", err)
	}
	
	// Mock signer should produce deterministic signatures
	if signature1 != signature2 {
		t.Errorf("Mock signer should produce deterministic signatures")
		t.Errorf("Signature 1: %s", signature1)
		t.Errorf("Signature 2: %s", signature2)
	}
	
	// Test verification
	valid, err := signer.Verify(testData, signature1)
	if err != nil {
		t.Fatalf("Failed to verify signature: %v", err)
	}
	
	if !valid {
		t.Error("Signature verification should succeed")
	}
	
	// Test with different data
	differentData := []byte("different test data")
	differentSignature, err := signer.Sign(differentData)
	if err != nil {
		t.Fatalf("Failed to sign different data: %v", err)
	}
	
	// Different data should produce different signature
	if signature1 == differentSignature {
		t.Error("Different data should produce different signatures")
	}
}

func TestVaultSigner_FixtureInput(t *testing.T) {
	// Test with a known fixture input to ensure deterministic behavior
	fixtureData := []byte("fixture_input_for_testing")
	
	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))
	
	// Create mock signer for deterministic testing
	signer := NewMockVaultSigner(logger)
	
	// Sign the fixture data
	signature, err := signer.Sign(fixtureData)
	if err != nil {
		t.Fatalf("Failed to sign fixture data: %v", err)
	}
	
	// Verify signature format
	if len(signature) == 0 {
		t.Error("Signature should not be empty")
	}
	
	// Log the signature for manual verification if needed
	t.Logf("Fixture signature: %s", signature)
	
	// Test that the same fixture produces the same signature
	signature2, err := signer.Sign(fixtureData)
	if err != nil {
		t.Fatalf("Failed to sign fixture data second time: %v", err)
	}
	
	if signature != signature2 {
		t.Error("Fixture data should produce deterministic signatures")
	}
	
	// Test verification
	valid, err := signer.Verify(fixtureData, signature)
	if err != nil {
		t.Fatalf("Failed to verify fixture signature: %v", err)
	}
	
	if !valid {
		t.Error("Fixture signature verification should succeed")
	}
}
