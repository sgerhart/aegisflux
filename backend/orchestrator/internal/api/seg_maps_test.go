package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nats-io/nats.go"
)

func TestSegMapsHandler(t *testing.T) {
	// Create a test NATS connection (in-memory)
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("Skipping test: NATS not available: %v", err)
	}
	defer nc.Close()

	// Create handler
	handler, err := NewSegMapsHandler(nc)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Test valid MapSnapshot
	validSnapshot := MapSnapshot{
		Version:    1,
		ServiceID:  123,
		TTLSeconds: 300,
		Edges: []Edge{
			{
				DstCIDR: "192.168.1.0/24",
				Proto:   "tcp",
				Port:    80,
			},
		},
		AllowCIDRs: []CIDRAllow{
			{
				CIDR:  "10.0.0.0/8",
				Proto: "tcp",
				Port:  443,
			},
		},
	}

	jsonData, err := json.Marshal(validSnapshot)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// Test POST request
	req := httptest.NewRequest("POST", "/seg/maps?target_host=test-host", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostSegMapsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
		t.Errorf("Response body: %s", w.Body.String())
	}

	// Verify response
	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if accepted, ok := response["accepted"].(bool); !ok || !accepted {
		t.Errorf("Expected accepted=true, got %v", response["accepted"])
	}
}

func TestSegMapsHandlerInvalidData(t *testing.T) {
	// Create a test NATS connection (in-memory)
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("Skipping test: NATS not available: %v", err)
	}
	defer nc.Close()

	// Create handler
	handler, err := NewSegMapsHandler(nc)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Test invalid JSON
	req := httptest.NewRequest("POST", "/seg/maps", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostSegMapsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestSegMapsHandlerInvalidSchema(t *testing.T) {
	// Create a test NATS connection (in-memory)
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		t.Skipf("Skipping test: NATS not available: %v", err)
	}
	defer nc.Close()

	// Create handler
	handler, err := NewSegMapsHandler(nc)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}

	// Test invalid schema (missing required fields)
	invalidSnapshot := map[string]interface{}{
		"version": "invalid", // should be integer
		// missing service_id and ttl_seconds
	}

	jsonData, err := json.Marshal(invalidSnapshot)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	req := httptest.NewRequest("POST", "/seg/maps", bytes.NewReader(jsonData))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.PostSegMapsHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}
