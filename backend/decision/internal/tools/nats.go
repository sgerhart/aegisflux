package tools

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"

	"aegisflux/backend/decision/internal/model"
)

// NATSConfig contains configuration for the NATS tool
type NATSConfig struct {
	URL     string
	Subject string
	Timeout time.Duration
}

// NATSTool provides interface to NATS messaging for plan publishing
type NATSTool struct {
	config NATSConfig
	conn   *nats.Conn
	logger *slog.Logger
}

// NewNATSTool creates a new NATS tool instance
func NewNATSTool(config NATSConfig, conn *nats.Conn, logger *slog.Logger) *NATSTool {
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}

	return &NATSTool{
		config: config,
		conn:   conn,
		logger: logger,
	}
}

// PlanProposedEvent represents a plan proposed event
type PlanProposedEvent struct {
	EventType string      `json:"event_type"`
	Plan      model.Plan  `json:"plan"`
	Timestamp time.Time   `json:"timestamp"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// PublishPlanProposed publishes a plan proposed event to NATS
func (n *NATSTool) PublishPlanProposed(plan model.Plan) error {
	n.logger.Debug("Publishing plan proposed event", "plan_id", plan.ID, "subject", n.config.Subject)

	// Create the event
	event := PlanProposedEvent{
		EventType: "plan.proposed",
		Plan:      plan,
		Timestamp: time.Now(),
		Metadata: map[string]any{
			"service": "decision",
			"version": "1.0.0",
		},
	}

	// Marshal the event to JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal plan proposed event: %w", err)
	}

	// Publish to NATS
	subject := n.config.Subject
	if subject == "" {
		subject = "plans.proposed"
	}

	err = n.conn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish plan proposed event: %w", err)
	}

	n.logger.Info("Plan proposed event published successfully", 
		"plan_id", plan.ID, 
		"subject", subject,
		"plan_status", plan.Status,
		"plan_strategy", plan.Strategy.Mode)

	return nil
}

// PublishPlanStatusUpdate publishes a plan status update event
func (n *NATSTool) PublishPlanStatusUpdate(plan model.Plan, oldStatus model.PlanStatus) error {
	n.logger.Debug("Publishing plan status update", "plan_id", plan.ID, "old_status", oldStatus, "new_status", plan.Status)

	event := map[string]any{
		"event_type": "plan.status_updated",
		"plan_id":    plan.ID,
		"old_status": oldStatus,
		"new_status": plan.Status,
		"timestamp":  time.Now(),
		"metadata": map[string]any{
			"service": "decision",
			"version": "1.0.0",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal plan status update event: %w", err)
	}

	subject := "plans.status_updated"
	err = n.conn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish plan status update event: %w", err)
	}

	n.logger.Info("Plan status update published successfully", 
		"plan_id", plan.ID, 
		"old_status", oldStatus,
		"new_status", plan.Status)

	return nil
}

// PublishPlanCompleted publishes a plan completion event
func (n *NATSTool) PublishPlanCompleted(plan model.Plan, results map[string]any) error {
	n.logger.Debug("Publishing plan completion event", "plan_id", plan.ID)

	event := map[string]any{
		"event_type": "plan.completed",
		"plan_id":    plan.ID,
		"plan":       plan,
		"results":    results,
		"timestamp":  time.Now(),
		"metadata": map[string]any{
			"service": "decision",
			"version": "1.0.0",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal plan completion event: %w", err)
	}

	subject := "plans.completed"
	err = n.conn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish plan completion event: %w", err)
	}

	n.logger.Info("Plan completion event published successfully", 
		"plan_id", plan.ID,
		"plan_status", plan.Status)

	return nil
}

// PublishPlanFailed publishes a plan failure event
func (n *NATSTool) PublishPlanFailed(plan model.Plan, errorMsg string, errorDetails map[string]any) error {
	n.logger.Debug("Publishing plan failure event", "plan_id", plan.ID, "error", errorMsg)

	event := map[string]any{
		"event_type": "plan.failed",
		"plan_id":    plan.ID,
		"plan":       plan,
		"error":      errorMsg,
		"error_details": errorDetails,
		"timestamp":  time.Now(),
		"metadata": map[string]any{
			"service": "decision",
			"version": "1.0.0",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal plan failure event: %w", err)
	}

	subject := "plans.failed"
	err = n.conn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish plan failure event: %w", err)
	}

	n.logger.Info("Plan failure event published successfully", 
		"plan_id", plan.ID,
		"error", errorMsg)

	return nil
}

// SubscribeToFindings subscribes to findings from the correlator service
func (n *NATSTool) SubscribeToFindings(callback func(model.Finding) error) error {
	n.logger.Debug("Subscribing to findings")

	subject := "findings.*"
	queue := "decision"

	_, err := n.conn.QueueSubscribe(subject, queue, func(msg *nats.Msg) {
		var finding model.Finding
		if err := json.Unmarshal(msg.Data, &finding); err != nil {
			n.logger.Error("Failed to unmarshal finding", "error", err)
			return
		}

		n.logger.Debug("Received finding", "finding_id", finding.ID, "severity", finding.Severity)

		if err := callback(finding); err != nil {
			n.logger.Error("Failed to process finding", "finding_id", finding.ID, "error", err)
			return
		}

		// Acknowledge the message
		if err := msg.Ack(); err != nil {
			n.logger.Warn("Failed to acknowledge finding message", "error", err)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to findings: %w", err)
	}

	n.logger.Info("Subscribed to findings successfully", "subject", subject, "queue", queue)
	return nil
}

// PublishToolResult publishes a tool execution result
func (n *NATSTool) PublishToolResult(toolName string, result map[string]any, planID string) error {
	n.logger.Debug("Publishing tool result", "tool", toolName, "plan_id", planID)

	event := map[string]any{
		"event_type": "tool.result",
		"tool_name":  toolName,
		"plan_id":    planID,
		"result":     result,
		"timestamp":  time.Now(),
		"metadata": map[string]any{
			"service": "decision",
			"version": "1.0.0",
		},
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal tool result event: %w", err)
	}

	subject := fmt.Sprintf("tools.%s.result", toolName)
	err = n.conn.Publish(subject, data)
	if err != nil {
		return fmt.Errorf("failed to publish tool result event: %w", err)
	}

	n.logger.Info("Tool result published successfully", 
		"tool", toolName,
		"plan_id", planID)

	return nil
}

// GetConnectionStatus returns the NATS connection status
func (n *NATSTool) GetConnectionStatus() map[string]any {
	status := map[string]any{
		"connected": n.conn.IsConnected(),
		"url":       n.conn.ConnectedUrl(),
		"server_id": n.conn.ConnectedServerId(),
		"timestamp": time.Now(),
	}

	if !n.conn.IsConnected() {
		status["error"] = "not connected to NATS server"
	}

	return status
}
