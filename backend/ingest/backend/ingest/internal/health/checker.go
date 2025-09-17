package health

import (
	"log/slog"
	"sync"
)

// ServiceChecker implements the Checker interface
type ServiceChecker struct {
	grpcReady    bool
	natsReady    bool
	schemaReady  bool
	mu           sync.RWMutex
	logger       *slog.Logger
}

// NewServiceChecker creates a new service checker
func NewServiceChecker(logger *slog.Logger) *ServiceChecker {
	return &ServiceChecker{
		logger: logger,
	}
}

// SetGRPCReady sets the gRPC readiness status
func (c *ServiceChecker) SetGRPCReady(ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.grpcReady = ready
	c.logger.Debug("gRPC readiness updated", "ready", ready)
}

// SetNATSReady sets the NATS readiness status
func (c *ServiceChecker) SetNATSReady(ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.natsReady = ready
	c.logger.Debug("NATS readiness updated", "ready", ready)
}

// SetSchemaReady sets the schema readiness status
func (c *ServiceChecker) SetSchemaReady(ready bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.schemaReady = ready
	c.logger.Debug("Schema readiness updated", "ready", ready)
}

// IsHealthy returns true if both gRPC and NATS are healthy
func (c *ServiceChecker) IsHealthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Health check: both gRPC and NATS should be up
	return c.grpcReady && c.natsReady
}

// IsReady returns true if NATS is connected and schema is compiled
func (c *ServiceChecker) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	// Readiness check: NATS connected and schema compiled
	return c.natsReady && c.schemaReady
}

// GetStatus returns the current status of all components
func (c *ServiceChecker) GetStatus() map[string]bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	return map[string]bool{
		"grpc":   c.grpcReady,
		"nats":   c.natsReady,
		"schema": c.schemaReady,
	}
}
