package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/xeipuuv/gojsonschema"
)

type MapSnapshot struct {
	Version    int               `json:"version"`
	ServiceID  uint32            `json:"service_id"`
	Edges      []Edge            `json:"edges"`
	AllowCIDRs []CIDRAllow       `json:"allow_cidrs"`
	TTLSeconds int               `json:"ttl_seconds"`
	Meta       map[string]string `json:"meta,omitempty"`
}

type Edge struct {
	DstCIDR string `json:"dst_cidr"`
	Proto   string `json:"proto"` // tcp/udp/any
	Port    int    `json:"port"`
}

type CIDRAllow struct {
	CIDR  string `json:"cidr"`
	Proto string `json:"proto"`
	Port  int    `json:"port"`
}

type SegMapsHandler struct {
	nc     *nats.Conn
	schema *gojsonschema.Schema
}

// NewSegMapsHandler creates a new SegMapsHandler with NATS connection and schema validation
func NewSegMapsHandler(nc *nats.Conn) (*SegMapsHandler, error) {
	// Load the MapSnapshot schema
	var schemaPath string
	
	// Try different possible paths for the schema file
	possiblePaths := []string{
		"schemas/mapsnapshot.json",
		"../../../schemas/mapsnapshot.json",
		"/Users/stevengerhart/workspace/github/sgerhart/aegisflux/schemas/mapsnapshot.json",
	}
	
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			schemaPath = path
			break
		}
	}
	
	if schemaPath == "" {
		return nil, fmt.Errorf("schema file not found in any of the expected locations: %v", possiblePaths)
	}
	
	// Get absolute path for file:// URL
	absPath, err := filepath.Abs(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}
	
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + absPath)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return nil, fmt.Errorf("failed to load schema: %w", err)
	}
	
	return &SegMapsHandler{
		nc:     nc,
		schema: schema,
	}, nil
}

// validateMapSnapshot validates a MapSnapshot against the JSON schema
func (h *SegMapsHandler) validateMapSnapshot(snap MapSnapshot) error {
	// Convert to JSON for validation
	jsonData, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}
	
	documentLoader := gojsonschema.NewBytesLoader(jsonData)
	result, err := h.schema.Validate(documentLoader)
	if err != nil {
		return fmt.Errorf("schema validation error: %w", err)
	}
	
	if !result.Valid() {
		var errors []string
		for _, desc := range result.Errors() {
			errors = append(errors, desc.String())
		}
		return fmt.Errorf("validation failed: %v", errors)
	}
	
	return nil
}

// publishToNATS publishes the MapSnapshot to NATS 'actions.seg.maps' subject
func (h *SegMapsHandler) publishToNATS(snap MapSnapshot, targetHosts []string) error {
	// Create the message payload
	message := map[string]interface{}{
		"snapshot":     snap,
		"target_hosts": targetHosts,
		"timestamp":    fmt.Sprintf("%d", time.Now().Unix()),
	}
	
	jsonData, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	
	// Publish to NATS
	subject := "actions.seg.maps"
	if err := h.nc.Publish(subject, jsonData); err != nil {
		return fmt.Errorf("failed to publish to NATS: %w", err)
	}
	
	return nil
}

// PostSegMapsHandler handles POST /seg/maps requests with schema validation and NATS publishing
func (h *SegMapsHandler) PostSegMapsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	
	// Parse the MapSnapshot
	var snap MapSnapshot
	if err := json.Unmarshal(body, &snap); err != nil {
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	
	// Validate against schema
	if err := h.validateMapSnapshot(snap); err != nil {
		http.Error(w, fmt.Sprintf("Validation failed: %v", err), http.StatusBadRequest)
		return
	}
	
	// Extract target hosts from query parameter or use default
	targetHosts := r.URL.Query()["target_host"]
	if len(targetHosts) == 0 {
		// Default to localhost if no target hosts specified
		targetHosts = []string{"localhost"}
	}
	
	// Publish to NATS
	if err := h.publishToNATS(snap, targetHosts); err != nil {
		http.Error(w, fmt.Sprintf("Failed to publish to NATS: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Return success response
	w.Header().Set("Content-Type", "application/json")
	response := map[string]interface{}{
		"accepted":     true,
		"service_id":   snap.ServiceID,
		"target_hosts": targetHosts,
		"timestamp":    time.Now().Unix(),
	}
	
	json.NewEncoder(w).Encode(response)
}
