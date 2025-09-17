package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"aegisflux/backend/decision/internal/model"
)

// NATSPublisher defines the interface for publishing NATS messages
type NATSPublisher interface {
	Publish(subject string, data []byte) error
}

// PlanStore defines the interface for storing and retrieving plans
type PlanStore interface {
	// Store a plan
	Store(ctx context.Context, plan *model.Plan) error
	// Get a plan by ID
	Get(ctx context.Context, id string) (*model.Plan, error)
	// List all plans
	List(ctx context.Context) ([]*model.Plan, error)
	// Update a plan
	Update(ctx context.Context, plan *model.Plan) error
	// Delete a plan
	Delete(ctx context.Context, id string) error
	// Get plans by status
	GetByStatus(ctx context.Context, status model.PlanStatus) ([]*model.Plan, error)
	// Cleanup expired plans
	Cleanup(ctx context.Context) error
}

// MemoryPlanStore implements PlanStore using in-memory storage
type MemoryPlanStore struct {
	mu       sync.RWMutex
	plans    map[string]*model.Plan
	capacity int
	logger   *slog.Logger
	publisher NATSPublisher
}

// NewMemoryPlanStore creates a new in-memory plan store
func NewMemoryPlanStore(capacity int, logger *slog.Logger, publisher NATSPublisher) *MemoryPlanStore {
	return &MemoryPlanStore{
		plans:     make(map[string]*model.Plan),
		capacity:  capacity,
		logger:    logger,
		publisher: publisher,
	}
}

// Store stores a plan in memory
func (s *MemoryPlanStore) Store(ctx context.Context, plan *model.Plan) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check capacity
	if len(s.plans) >= s.capacity {
		// Remove oldest plan
		var oldestID string
		var oldestTime time.Time
		for id, p := range s.plans {
			if oldestID == "" || p.CreatedAt.Before(oldestTime) {
				oldestID = id
				oldestTime = p.CreatedAt
			}
		}
		if oldestID != "" {
			delete(s.plans, oldestID)
			s.logger.Info("Removed oldest plan due to capacity limit", "plan_id", oldestID)
		}
	}

	// Store the plan
	s.plans[plan.ID] = plan
	s.logger.Info("Plan stored", "plan_id", plan.ID, "status", plan.Status)

	// Publish plan creation event
	if s.publisher != nil {
		go func() {
			if err := s.publishPlanEvent("plan.created", plan); err != nil {
				s.logger.Error("Failed to publish plan creation event", "error", err)
			}
		}()
	}

	return nil
}

// Get retrieves a plan by ID
func (s *MemoryPlanStore) Get(ctx context.Context, id string) (*model.Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plan, exists := s.plans[id]
	if !exists {
		return nil, fmt.Errorf("plan not found: %s", id)
	}

	return plan, nil
}

// List retrieves all plans
func (s *MemoryPlanStore) List(ctx context.Context) ([]*model.Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plans := make([]*model.Plan, 0, len(s.plans))
	for _, plan := range s.plans {
		plans = append(plans, plan)
	}

	return plans, nil
}

// Update updates an existing plan
func (s *MemoryPlanStore) Update(ctx context.Context, plan *model.Plan) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.plans[plan.ID]; !exists {
		return fmt.Errorf("plan not found: %s", plan.ID)
	}

	plan.UpdatedAt = time.Now()
	s.plans[plan.ID] = plan
	s.logger.Info("Plan updated", "plan_id", plan.ID, "status", plan.Status)

	// Publish plan update event
	if s.publisher != nil {
		go func() {
			if err := s.publishPlanEvent("plan.updated", plan); err != nil {
				s.logger.Error("Failed to publish plan update event", "error", err)
			}
		}()
	}

	return nil
}

// Delete removes a plan by ID
func (s *MemoryPlanStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	plan, exists := s.plans[id]
	if !exists {
		return fmt.Errorf("plan not found: %s", id)
	}

	delete(s.plans, id)
	s.logger.Info("Plan deleted", "plan_id", id)

	// Publish plan deletion event
	if s.publisher != nil {
		go func() {
			if err := s.publishPlanEvent("plan.deleted", plan); err != nil {
				s.logger.Error("Failed to publish plan deletion event", "error", err)
			}
		}()
	}

	return nil
}

// GetByStatus retrieves plans by status
func (s *MemoryPlanStore) GetByStatus(ctx context.Context, status model.PlanStatus) ([]*model.Plan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var plans []*model.Plan
	for _, plan := range s.plans {
		if plan.Status == status {
			plans = append(plans, plan)
		}
	}

	return plans, nil
}

// Cleanup removes expired plans
func (s *MemoryPlanStore) Cleanup(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var expiredIDs []string

	for id, plan := range s.plans {
		if plan.ExpiresAt != nil && plan.ExpiresAt.Before(now) {
			expiredIDs = append(expiredIDs, id)
		}
	}

	for _, id := range expiredIDs {
		delete(s.plans, id)
		s.logger.Info("Removed expired plan", "plan_id", id)
	}

	if len(expiredIDs) > 0 {
		s.logger.Info("Cleanup completed", "expired_count", len(expiredIDs))
	}

	return nil
}

// publishPlanEvent publishes a plan event to NATS
func (s *MemoryPlanStore) publishPlanEvent(eventType string, plan *model.Plan) error {
	event := map[string]interface{}{
		"type":      eventType,
		"plan_id":   plan.ID,
		"timestamp": time.Now(),
		"plan":      plan,
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal plan event: %w", err)
	}

	subject := fmt.Sprintf("plans.%s", eventType)
	return s.publisher.Publish(subject, data)
}

// GetStats returns store statistics
func (s *MemoryPlanStore) GetStats(ctx context.Context) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := map[string]interface{}{
		"total_plans":    len(s.plans),
		"capacity":       s.capacity,
		"utilization":    float64(len(s.plans)) / float64(s.capacity),
		"status_counts":  make(map[string]int),
	}

	// Count plans by status
	for _, plan := range s.plans {
		statusStr := string(plan.Status)
		stats["status_counts"].(map[string]int)[statusStr]++
	}

	return stats
}
