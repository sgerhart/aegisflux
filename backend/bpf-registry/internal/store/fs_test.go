package store

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"aegisflux/backend/bpf-registry/internal/model"
)

func TestFSStore_PutGet(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "bpf-registry-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create FS store
	store, err := NewFSStore(tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create FS store: %v", err)
	}

	// Test data
	testID := "test-artifact-123"
	testData := []byte("test tar.zst data")
	testMetadata := map[string]interface{}{
		"name":         "test-artifact",
		"version":      "1.0.0",
		"description":  "Test artifact for unit testing",
		"type":         "program",
		"architecture": "x86_64",
		"kernel_version": "5.4.0",
		"size":         float64(len(testData)),
		"checksum":     "abc123def456",
		"created_at":   time.Now().Format(time.RFC3339Nano),
		"updated_at":   time.Now().Format(time.RFC3339Nano),
		"tags":         []interface{}{"test", "unit"},
	}

	// Test Put
	err = store.Put(testID, testData, testMetadata)
	if err != nil {
		t.Fatalf("Failed to put artifact: %v", err)
	}

	// Verify files were created
	binaryPath := filepath.Join(tempDir, testID, "artifact.tar.zst")
	metadataPath := filepath.Join(tempDir, testID, "metadata.json")

	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Error("Binary file was not created")
	}

	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Error("Metadata file was not created")
	}

	// Test Get
	retrievedMetadata, retrievedData, err := store.Get(testID)
	if err != nil {
		t.Fatalf("Failed to get artifact: %v", err)
	}

	// Verify retrieved data
	if string(retrievedData) != string(testData) {
		t.Errorf("Retrieved data doesn't match. Expected: %s, Got: %s", 
			string(testData), string(retrievedData))
	}

	// Verify retrieved metadata
	if retrievedMetadata["name"] != testMetadata["name"] {
		t.Errorf("Retrieved name doesn't match. Expected: %v, Got: %v",
			testMetadata["name"], retrievedMetadata["name"])
	}

	if retrievedMetadata["version"] != testMetadata["version"] {
		t.Errorf("Retrieved version doesn't match. Expected: %v, Got: %v",
			testMetadata["version"], retrievedMetadata["version"])
	}
}

func TestFSStore_List(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "bpf-registry-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create FS store
	store, err := NewFSStore(tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create FS store: %v", err)
	}

	// Test with empty store
	summaries, err := store.List()
	if err != nil {
		t.Fatalf("Failed to list artifacts: %v", err)
	}

	if len(summaries) != 0 {
		t.Errorf("Expected empty list, got %d items", len(summaries))
	}

	// Add test artifacts
	testArtifacts := []struct {
		id       string
		data     []byte
		metadata map[string]interface{}
	}{
		{
			id:   "artifact-1",
			data: []byte("test data 1"),
			metadata: map[string]interface{}{
				"name":        "test-1",
				"version":     "1.0.0",
				"description": "First test artifact",
				"type":        "program",
				"size":        float64(11),
			},
		},
		{
			id:   "artifact-2",
			data: []byte("test data 2"),
			metadata: map[string]interface{}{
				"name":        "test-2",
				"version":     "2.0.0",
				"description": "Second test artifact",
				"type":        "map",
				"size":        float64(11),
			},
		},
	}

	for _, artifact := range testArtifacts {
		err = store.Put(artifact.id, artifact.data, artifact.metadata)
		if err != nil {
			t.Fatalf("Failed to put artifact %s: %v", artifact.id, err)
		}
	}

	// Test List
	summaries, err = store.List()
	if err != nil {
		t.Fatalf("Failed to list artifacts: %v", err)
	}

	if len(summaries) != 2 {
		t.Errorf("Expected 2 artifacts, got %d", len(summaries))
	}

	// Verify summaries contain correct data
	summaryMap := make(map[string]model.ArtifactSummary)
	for _, summary := range summaries {
		summaryMap[summary.ID] = summary
	}

	if summary1, ok := summaryMap["artifact-1"]; ok {
		if summary1.Name != "test-1" {
			t.Errorf("Expected name 'test-1', got '%s'", summary1.Name)
		}
		if summary1.Type != "program" {
			t.Errorf("Expected type 'program', got '%s'", summary1.Type)
		}
	} else {
		t.Error("artifact-1 not found in summaries")
	}

	if summary2, ok := summaryMap["artifact-2"]; ok {
		if summary2.Name != "test-2" {
			t.Errorf("Expected name 'test-2', got '%s'", summary2.Name)
		}
		if summary2.Type != "map" {
			t.Errorf("Expected type 'map', got '%s'", summary2.Type)
		}
	} else {
		t.Error("artifact-2 not found in summaries")
	}
}

func TestFSStore_GetNotFound(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "bpf-registry-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create FS store
	store, err := NewFSStore(tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create FS store: %v", err)
	}

	// Test Get with non-existent artifact
	_, _, err = store.Get("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent artifact")
	}

	if !contains(err.Error(), "artifact not found") {
		t.Errorf("Expected 'artifact not found' error, got: %v", err)
	}
}

func TestFSStore_MetadataToSummary(t *testing.T) {
	// Create temporary directory for testing
	tempDir, err := os.MkdirTemp("", "bpf-registry-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError,
	}))

	// Create FS store
	store, err := NewFSStore(tempDir, logger)
	if err != nil {
		t.Fatalf("Failed to create FS store: %v", err)
	}

	// Test metadata conversion
	testMetadata := map[string]interface{}{
		"name":           "test-artifact",
		"version":        "1.0.0",
		"description":    "Test artifact",
		"type":           "program",
		"architecture":   "x86_64",
		"kernel_version": "5.4.0",
		"size":           float64(1024),
		"checksum":       "abc123",
		"created_at":     "2023-01-01T00:00:00Z",
		"updated_at":     "2023-01-01T00:00:00Z",
		"tags":           []interface{}{"test", "unit"},
	}

	summary, err := store.metadataToSummary("test-id", testMetadata)
	if err != nil {
		t.Fatalf("Failed to convert metadata to summary: %v", err)
	}

	// Verify conversion
	if summary.ID != "test-id" {
		t.Errorf("Expected ID 'test-id', got '%s'", summary.ID)
	}

	if summary.Name != "test-artifact" {
		t.Errorf("Expected name 'test-artifact', got '%s'", summary.Name)
	}

	if summary.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", summary.Version)
	}

	if summary.Size != 1024 {
		t.Errorf("Expected size 1024, got %d", summary.Size)
	}

	if len(summary.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(summary.Tags))
	}

	if summary.Tags[0] != "test" || summary.Tags[1] != "unit" {
		t.Errorf("Expected tags ['test', 'unit'], got %v", summary.Tags)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && (s[:len(substr)] == substr || 
		s[len(s)-len(substr):] == substr || 
		containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
