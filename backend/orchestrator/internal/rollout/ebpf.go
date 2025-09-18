package rollout

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"log/slog"
)

// ApplyRequest represents a request to apply a BPF program
type ApplyRequest struct {
	PlanID    string   `json:"plan_id"`
	Targets   []string `json:"targets"`
	ArtifactID string  `json:"artifact_id"`
	TTL       int      `json:"ttl"` // Time-to-live in seconds
	Canary    bool     `json:"canary,omitempty"`
}

// ApplyResponse represents the response from an apply operation
type ApplyResponse struct {
	RequestID   string            `json:"request_id"`
	PlanID      string            `json:"plan_id"`
	Status      string            `json:"status"` // "pending", "success", "failed", "rollback"
	Targets     []TargetStatus    `json:"targets"`
	AppliedAt   time.Time         `json:"applied_at"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Error       string            `json:"error,omitempty"`
	Metrics     *DeploymentMetrics `json:"metrics,omitempty"`
}

// TargetStatus represents the status of a target deployment
type TargetStatus struct {
	TargetID   string    `json:"target_id"`
	Status     string    `json:"status"` // "pending", "applied", "failed", "rollback"
	AppliedAt  time.Time `json:"applied_at"`
	Error      string    `json:"error,omitempty"`
	Telemetry  *TelemetryData `json:"telemetry,omitempty"`
}

// DeploymentMetrics contains deployment metrics
type DeploymentMetrics struct {
	TotalTargets     int     `json:"total_targets"`
	SuccessfulTargets int    `json:"successful_targets"`
	FailedTargets    int     `json:"failed_targets"`
	RollbackTargets  int     `json:"rollback_targets"`
	AverageLatency   float64 `json:"average_latency_ms"`
	ViolationCount   int     `json:"violation_count"`
}

// TelemetryData contains telemetry information from agents
type TelemetryData struct {
	Violations     int       `json:"violations"`
	Errors         int       `json:"errors"`
	Latency        float64   `json:"latency_ms"`
	LastUpdate     time.Time `json:"last_update"`
	AgentVersion   string    `json:"agent_version"`
	BPFLoadTime    float64   `json:"bpf_load_time_ms"`
	PacketCount    int64     `json:"packet_count"`
	BlockCount     int64     `json:"block_count"`
}

// ViolationThreshold represents thresholds for rollback decisions
type ViolationThreshold struct {
	MaxViolations    int     `json:"max_violations"`
	MaxErrorRate     float64 `json:"max_error_rate"`
	MaxLatency       float64 `json:"max_latency_ms"`
	MinSuccessRate   float64 `json:"min_success_rate"`
}

// BPFRolloutManager manages BPF program rollouts with canary deployments
type BPFRolloutManager struct {
	logger          *slog.Logger
	nc              *nats.Conn
	registryURL     string
	activeRollouts  map[string]*ApplyResponse
	rolloutMutex    sync.RWMutex
	telemetrySub    *nats.Subscription
	actionsSub      *nats.Subscription
	thresholds      ViolationThreshold
	observationWindow time.Duration
}

// NewBPFRolloutManager creates a new BPF rollout manager
func NewBPFRolloutManager(logger *slog.Logger, nc *nats.Conn, registryURL string) *BPFRolloutManager {
	return &BPFRolloutManager{
		logger:  logger,
		nc:      nc,
		registryURL: registryURL,
		activeRollouts: make(map[string]*ApplyResponse),
		thresholds: ViolationThreshold{
			MaxViolations:  5,
			MaxErrorRate:   0.1, // 10%
			MaxLatency:     1000, // 1 second
			MinSuccessRate: 0.95, // 95%
		},
		observationWindow: 5 * time.Minute,
	}
}

// ApplyCanary applies a BPF program using canary deployment
func (rm *BPFRolloutManager) ApplyCanary(ctx context.Context, req ApplyRequest) (*ApplyResponse, error) {
	rm.logger.Info("Starting canary BPF deployment",
		"plan_id", req.PlanID,
		"artifact_id", req.ArtifactID,
		"targets_count", len(req.Targets))

	// Generate request ID
	requestID := fmt.Sprintf("%s-%d", req.PlanID, time.Now().Unix())

	// Create response
	response := &ApplyResponse{
		RequestID: requestID,
		PlanID:    req.PlanID,
		Status:    "pending",
		Targets:   make([]TargetStatus, len(req.Targets)),
		AppliedAt: time.Now(),
	}

	// Initialize target statuses
	for i, target := range req.Targets {
		response.Targets[i] = TargetStatus{
			TargetID:  target,
			Status:    "pending",
			AppliedAt: time.Now(),
		}
	}

	// Store active rollout
	rm.rolloutMutex.Lock()
	rm.activeRollouts[requestID] = response
	rm.rolloutMutex.Unlock()

	// Start canary deployment process
	go rm.executeCanaryDeployment(ctx, requestID, req)

	return response, nil
}

// executeCanaryDeployment executes the canary deployment process
func (rm *BPFRolloutManager) executeCanaryDeployment(ctx context.Context, requestID string, req ApplyRequest) {
	defer func() {
		rm.rolloutMutex.Lock()
		if rollout, exists := rm.activeRollouts[requestID]; exists {
			completedAt := time.Now()
			rollout.CompletedAt = &completedAt
		}
		rm.rolloutMutex.Unlock()
	}()

	rm.logger.Info("Executing canary deployment", "request_id", requestID)

	// Phase 1: Apply to canary targets (first 10% or minimum 1 target)
	canaryCount := len(req.Targets) / 10
	if canaryCount < 1 {
		canaryCount = 1
	}
	if canaryCount > len(req.Targets) {
		canaryCount = len(req.Targets)
	}

	canaryTargets := req.Targets[:canaryCount]
	remainingTargets := req.Targets[canaryCount:]

	rm.logger.Info("Phase 1: Applying to canary targets",
		"request_id", requestID,
		"canary_count", canaryCount)

	// Apply to canary targets
	if err := rm.applyToTargets(ctx, requestID, canaryTargets, req.ArtifactID, req.TTL); err != nil {
		rm.logger.Error("Canary deployment failed", "request_id", requestID, "error", err)
		rm.updateRolloutStatus(requestID, "failed", err.Error())
		return
	}

	// Phase 2: Observe canary targets
	rm.logger.Info("Phase 2: Observing canary targets",
		"request_id", requestID,
		"observation_window", rm.observationWindow)

	success := rm.observeCanaryTargets(ctx, requestID, canaryTargets)

	if !success {
		rm.logger.Warn("Canary deployment failed validation, rolling back",
			"request_id", requestID)
		rm.rollbackTargets(ctx, requestID, canaryTargets)
		rm.updateRolloutStatus(requestID, "rollback", "Canary validation failed")
		return
	}

	// Phase 3: Apply to remaining targets
	if len(remainingTargets) > 0 {
		rm.logger.Info("Phase 3: Applying to remaining targets",
			"request_id", requestID,
			"remaining_count", len(remainingTargets))

		if err := rm.applyToTargets(ctx, requestID, remainingTargets, req.ArtifactID, req.TTL); err != nil {
			rm.logger.Error("Remaining targets deployment failed", "request_id", requestID, "error", err)
			rm.updateRolloutStatus(requestID, "failed", err.Error())
			return
		}
	}

	// Phase 4: Final observation
	rm.logger.Info("Phase 4: Final observation of all targets", "request_id", requestID)
	
	finalSuccess := rm.observeCanaryTargets(ctx, requestID, req.Targets)
	
	if finalSuccess {
		rm.updateRolloutStatus(requestID, "success", "")
		rm.logger.Info("Canary deployment completed successfully", "request_id", requestID)
	} else {
		rm.logger.Warn("Final validation failed, rolling back all targets", "request_id", requestID)
		rm.rollbackTargets(ctx, requestID, req.Targets)
		rm.updateRolloutStatus(requestID, "rollback", "Final validation failed")
	}
}

// applyToTargets applies BPF program to specified targets
func (rm *BPFRolloutManager) applyToTargets(ctx context.Context, requestID string, targets []string, artifactID string, ttl int) error {
	// Create apply action message
	action := map[string]interface{}{
		"request_id":  requestID,
		"action":      "apply_ebpf",
		"artifact_id": artifactID,
		"ttl":         ttl,
		"targets":     targets,
		"timestamp":   time.Now(),
	}

	actionJSON, err := json.Marshal(action)
	if err != nil {
		return fmt.Errorf("failed to marshal action: %w", err)
	}

	// Publish to NATS
	subject := "actions.apply.ebpf"
	if err := rm.nc.Publish(subject, actionJSON); err != nil {
		return fmt.Errorf("failed to publish action: %w", err)
	}

	rm.logger.Info("Published BPF apply action",
		"request_id", requestID,
		"subject", subject,
		"targets", targets)

	return nil
}

// observeCanaryTargets observes canary targets for violations
func (rm *BPFRolloutManager) observeCanaryTargets(ctx context.Context, requestID string, targets []string) bool {
	// Create context with timeout for observation window
	observeCtx, cancel := context.WithTimeout(ctx, rm.observationWindow)
	defer cancel()

	// Subscribe to telemetry for these targets
	telemetryChan := make(chan *TelemetryData, 100)
	
	subject := "agent.telemetry"
	sub, err := rm.nc.Subscribe(subject, func(msg *nats.Msg) {
		var telemetry TelemetryData
		if err := json.Unmarshal(msg.Data, &telemetry); err != nil {
			rm.logger.Error("Failed to unmarshal telemetry", "error", err)
			return
		}

		// Check if this telemetry is for our targets
		// In a real implementation, you'd match target IDs
		telemetryChan <- &telemetry
	})
	
	if err != nil {
		rm.logger.Error("Failed to subscribe to telemetry", "error", err)
		return false
	}
	defer sub.Unsubscribe()

	// Collect telemetry data during observation window
	var totalViolations int
	var totalErrors int
	var totalLatency float64
	var messageCount int

	observeTicker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer observeTicker.Stop()

	for {
		select {
		case <-observeCtx.Done():
			rm.logger.Info("Observation window completed",
				"request_id", requestID,
				"total_violations", totalViolations,
				"total_errors", totalErrors,
				"message_count", messageCount)
			
			// Evaluate success criteria
			success := rm.evaluateSuccess(totalViolations, totalErrors, totalLatency, messageCount)
			return success

		case telemetry := <-telemetryChan:
			totalViolations += telemetry.Violations
			totalErrors += telemetry.Errors
			totalLatency += telemetry.Latency
			messageCount++

			// Update target status with telemetry
			rm.updateTargetTelemetry(requestID, telemetry)

		case <-observeTicker.C:
			rm.logger.Debug("Observation checkpoint",
				"request_id", requestID,
				"violations", totalViolations,
				"errors", totalErrors,
				"messages", messageCount)
		}
	}
}

// evaluateSuccess evaluates if the deployment meets success criteria
func (rm *BPFRolloutManager) evaluateSuccess(violations, errors int, latency float64, messageCount int) bool {
	if messageCount == 0 {
		rm.logger.Warn("No telemetry messages received during observation")
		return false
	}

	avgLatency := latency / float64(messageCount)
	errorRate := float64(errors) / float64(messageCount)
	successRate := 1.0 - errorRate

	rm.logger.Info("Evaluating deployment success",
		"violations", violations,
		"error_rate", errorRate,
		"avg_latency", avgLatency,
		"success_rate", successRate)

	// Check thresholds
	if violations > rm.thresholds.MaxViolations {
		rm.logger.Warn("Too many violations", "violations", violations, "threshold", rm.thresholds.MaxViolations)
		return false
	}

	if errorRate > rm.thresholds.MaxErrorRate {
		rm.logger.Warn("Error rate too high", "error_rate", errorRate, "threshold", rm.thresholds.MaxErrorRate)
		return false
	}

	if avgLatency > rm.thresholds.MaxLatency {
		rm.logger.Warn("Latency too high", "avg_latency", avgLatency, "threshold", rm.thresholds.MaxLatency)
		return false
	}

	if successRate < rm.thresholds.MinSuccessRate {
		rm.logger.Warn("Success rate too low", "success_rate", successRate, "threshold", rm.thresholds.MinSuccessRate)
		return false
	}

	rm.logger.Info("Deployment meets success criteria")
	return true
}

// rollbackTargets rolls back BPF program from specified targets
func (rm *BPFRolloutManager) rollbackTargets(ctx context.Context, requestID string, targets []string) {
	rm.logger.Info("Rolling back targets", "request_id", requestID, "targets", targets)

	// Create rollback action message
	action := map[string]interface{}{
		"request_id": requestID,
		"action":     "rollback_ebpf",
		"targets":    targets,
		"timestamp":  time.Now(),
	}

	actionJSON, err := json.Marshal(action)
	if err != nil {
		rm.logger.Error("Failed to marshal rollback action", "error", err)
		return
	}

	// Publish rollback action
	subject := "actions.rollback.ebpf"
	if err := rm.nc.Publish(subject, actionJSON); err != nil {
		rm.logger.Error("Failed to publish rollback action", "error", err)
		return
	}

	rm.logger.Info("Published BPF rollback action", "request_id", requestID, "subject", subject)
}

// updateRolloutStatus updates the status of a rollout
func (rm *BPFRolloutManager) updateRolloutStatus(requestID, status, errorMsg string) {
	rm.rolloutMutex.Lock()
	defer rm.rolloutMutex.Unlock()

	if rollout, exists := rm.activeRollouts[requestID]; exists {
		rollout.Status = status
		if errorMsg != "" {
			rollout.Error = errorMsg
		}
	}
}

// updateTargetTelemetry updates telemetry data for a target
func (rm *BPFRolloutManager) updateTargetTelemetry(requestID string, telemetry *TelemetryData) {
	rm.rolloutMutex.Lock()
	defer rm.rolloutMutex.Unlock()

	if rollout, exists := rm.activeRollouts[requestID]; exists {
		// Update telemetry for matching targets
		// In a real implementation, you'd match by target ID
		for i := range rollout.Targets {
			rollout.Targets[i].Telemetry = telemetry
		}
	}
}

// GetRolloutStatus retrieves the status of a rollout
func (rm *BPFRolloutManager) GetRolloutStatus(requestID string) (*ApplyResponse, error) {
	rm.rolloutMutex.RLock()
	defer rm.rolloutMutex.RUnlock()

	rollout, exists := rm.activeRollouts[requestID]
	if !exists {
		return nil, fmt.Errorf("rollout not found: %s", requestID)
	}

	return rollout, nil
}

// ListActiveRollouts returns all active rollouts
func (rm *BPFRolloutManager) ListActiveRollouts() []*ApplyResponse {
	rm.rolloutMutex.RLock()
	defer rm.rolloutMutex.RUnlock()

	rollouts := make([]*ApplyResponse, 0, len(rm.activeRollouts))
	for _, rollout := range rm.activeRollouts {
		rollouts = append(rollouts, rollout)
	}

	return rollouts
}

// SetViolationThresholds updates violation thresholds
func (rm *BPFRolloutManager) SetViolationThresholds(thresholds ViolationThreshold) {
	rm.thresholds = thresholds
	rm.logger.Info("Updated violation thresholds", "thresholds", thresholds)
}

// SetObservationWindow sets the observation window for canary deployments
func (rm *BPFRolloutManager) SetObservationWindow(window time.Duration) {
	rm.observationWindow = window
	rm.logger.Info("Updated observation window", "window", window)
}
