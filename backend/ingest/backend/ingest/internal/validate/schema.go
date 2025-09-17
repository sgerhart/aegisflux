package validate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"aegisflux/backend/ingest/protos"
)

// SchemaValidator implements JSON schema validation for events
type SchemaValidator struct {
	schema *jsonschema.Schema
	logger *slog.Logger
	mu     sync.RWMutex
}

// NewSchemaValidator creates a new schema validator
func NewSchemaValidator(logger *slog.Logger) (*SchemaValidator, error) {
	// Load the schema file
	schemaPath := filepath.Join("schemas", "Event.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	// Compile the schema
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	err = compiler.AddResource("event.json", strings.NewReader(string(schemaData)))
	if err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}
	schema, err := compiler.Compile("event.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	logger.Info("Schema validator initialized", "schema_path", schemaPath)

	return &SchemaValidator{
		schema: schema,
		logger: logger,
	}, nil
}

// ValidateEvent validates an event against the JSON schema
func (v *SchemaValidator) ValidateEvent(ctx context.Context, e *protos.Event) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	// Convert protobuf Event to map for validation
	eventMap := v.eventToMap(e)

	// Validate against schema
	if err := v.schema.Validate(eventMap); err != nil {
		v.logger.Warn("Event validation failed", 
			"event_id", e.Id, 
			"error", err.Error())
		return fmt.Errorf("validation failed: %w", err)
	}

	v.logger.Debug("Event validation successful", "event_id", e.Id)
	return nil
}

// eventToMap converts a protobuf Event to a map for JSON schema validation
func (v *SchemaValidator) eventToMap(e *protos.Event) map[string]any {
	eventMap := map[string]any{
		"id":        e.Id,
		"type":      e.Type,
		"source":    e.Source,
		"timestamp": e.Timestamp,
	}

	// Add metadata if present - convert map[string]string to map[string]any
	if len(e.Metadata) > 0 {
		metadata := make(map[string]any)
		for k, v := range e.Metadata {
			metadata[k] = v
		}
		eventMap["metadata"] = metadata
	}

	// Add payload if present (convert bytes to base64 string)
	if len(e.Payload) > 0 {
		// For validation purposes, we'll treat the payload as base64
		// In a real implementation, you might want to validate the actual payload content
		eventMap["payload"] = string(e.Payload)
	}

	return eventMap
}

// ReloadSchema reloads the schema from disk (useful for development)
func (v *SchemaValidator) ReloadSchema() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	schemaPath := filepath.Join("schemas", "Event.json")
	schemaData, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	err = compiler.AddResource("event.json", strings.NewReader(string(schemaData)))
	if err != nil {
		return fmt.Errorf("failed to add schema resource: %w", err)
	}
	schema, err := compiler.Compile("event.json")
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	v.schema = schema
	v.logger.Info("Schema reloaded successfully", "schema_path", schemaPath)
	return nil
}
