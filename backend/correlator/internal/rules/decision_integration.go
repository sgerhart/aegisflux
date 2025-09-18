package rules

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// DecisionIntegration handles integration with the Decision Engine
type DecisionIntegration struct {
	natsConn *nats.Conn
	logger   *slog.Logger
	// Services
	forwarder    *FindingForwarder
	synthesizer  *MapSnapshotSynthesizer
	// Configuration
	decisionEngineURL string
	timeoutSeconds    int
	// Statistics
	plansCreated      int64
	safeguardsSent    int64
	mapsnapshotsSent  int64
	errors            int64
}

// NewDecisionIntegration creates a new decision integration service
func NewDecisionIntegration(natsConn *nats.Conn, logger *slog.Logger) *DecisionIntegration {
	return &DecisionIntegration{
		natsConn: natsConn,
		logger:   logger,
		forwarder:   NewFindingForwarder(natsConn, logger),
		synthesizer: NewMapSnapshotSynthesizer(logger),
		decisionEngineURL: "decision.adaptive_safeguards",
		timeoutSeconds:    30,
	}
}

// ProcessFinding processes a finding and creates appropriate responses
func (di *DecisionIntegration) ProcessFinding(finding *Finding) error {
	di.logger.Info("Processing finding for decision integration",
		"finding_id", finding.ID,
		"severity", finding.Severity,
		"type", finding.Type,
		"host_id", finding.HostID)

	// Check if finding should be processed
	if !di.shouldProcess(finding) {
		di.logger.Debug("Finding does not meet processing criteria",
			"finding_id", finding.ID,
			"severity", finding.Severity)
		return nil
	}

	// Forward as adaptive safeguard
	if err := di.forwarder.ForwardFinding(finding); err != nil {
		di.logger.Error("Failed to forward finding as adaptive safeguard",
			"finding_id", finding.ID,
			"error", err)
		di.errors++
		return fmt.Errorf("failed to forward finding: %w", err)
	}

	di.safeguardsSent++

	// Create MapSnapshot for network risks
	if di.isNetworkRisk(finding) {
		if err := di.createMapSnapshotForFinding(finding); err != nil {
			di.logger.Error("Failed to create MapSnapshot for finding",
				"finding_id", finding.ID,
				"error", err)
			di.errors++
			// Don't return error here as safeguard was already sent
		} else {
			di.mapsnapshotsSent++
		}
	}

	// Create decision plan if needed
	if di.shouldCreatePlan(finding) {
		if err := di.createDecisionPlan(finding); err != nil {
			di.logger.Error("Failed to create decision plan",
				"finding_id", finding.ID,
				"error", err)
			di.errors++
			// Don't return error here as other actions succeeded
		} else {
			di.plansCreated++
		}
	}

	di.logger.Info("Successfully processed finding",
		"finding_id", finding.ID,
		"safeguards_sent", di.safeguardsSent,
		"mapsnapshots_sent", di.mapsnapshotsSent,
		"plans_created", di.plansCreated)

	return nil
}

// shouldProcess determines if a finding should be processed
func (di *DecisionIntegration) shouldProcess(finding *Finding) bool {
	// Check severity threshold
	highSeverity := map[string]bool{
		"high":    true,
		"critical": true,
	}
	if !highSeverity[finding.Severity] {
		return false
	}

	// Check confidence threshold
	if finding.Confidence < 0.7 {
		return false
	}

	// Check if it's a network risk or has network-related tags
	if di.isNetworkRisk(finding) {
		return true
	}

	// Check for specific rule types that should be processed
	processableTypes := []string{
		"unauthorized_access",
		"privilege_escalation",
		"data_exfiltration",
		"lateral_movement",
		"network_scan",
		"port_scan",
		"reconnaissance",
	}

	for _, ruleType := range processableTypes {
		if finding.Type == ruleType {
			return true
		}
	}

	return false
}

// isNetworkRisk determines if a finding represents a network risk
func (di *DecisionIntegration) isNetworkRisk(finding *Finding) bool {
	// Check finding type
	networkTypes := map[string]bool{
		"network":           true,
		"connect":           true,
		"network_scan":      true,
		"port_scan":         true,
		"reconnaissance":    true,
		"data_exfiltration": true,
		"lateral_movement":  true,
	}

	if networkTypes[finding.Type] {
		return true
	}

	// Check tags for network-related indicators
	for _, tag := range finding.Tags {
		if networkTypes[tag] {
			return true
		}
	}

	// Check evidence for network indicators
	if evidence, ok := finding.Evidence["event_type"].(string); ok {
		if evidence == "connect" || evidence == "network" {
			return true
		}
	}

	return false
}

// shouldCreatePlan determines if a decision plan should be created
func (di *DecisionIntegration) shouldCreatePlan(finding *Finding) bool {
	// Create plans for critical findings or high-confidence findings
	return finding.Severity == "critical" || finding.Confidence > 0.9
}

// createMapSnapshotForFinding creates a MapSnapshot for a network risk finding
func (di *DecisionIntegration) createMapSnapshotForFinding(finding *Finding) error {
	// Synthesize MapSnapshot
	mapSnapshot, err := di.synthesizer.SynthesizeMapSnapshot(finding)
	if err != nil {
		return fmt.Errorf("failed to synthesize MapSnapshot: %w", err)
	}

	// Create MapSnapshot message
	mapSnapshotMsg := &MapSnapshotMessage{
		FindingID:    finding.ID,
		HostID:       finding.HostID,
		MapSnapshot:  mapSnapshot,
		Reason:       mapSnapshot.Reason,
		Confidence:   mapSnapshot.Confidence,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:    time.Now().Add(time.Duration(mapSnapshot.TTLSeconds) * time.Second).UTC().Format(time.RFC3339),
	}

	// Publish to orchestrator
	if err := di.publishMapSnapshot(mapSnapshotMsg); err != nil {
		return fmt.Errorf("failed to publish MapSnapshot: %w", err)
	}

	di.logger.Info("Created MapSnapshot for finding",
		"finding_id", finding.ID,
		"host_id", finding.HostID,
		"service_id", mapSnapshot.ServiceID,
		"edges_count", len(mapSnapshot.Edges))

	return nil
}

// createDecisionPlan creates a decision plan for a finding
func (di *DecisionIntegration) createDecisionPlan(finding *Finding) error {
	// Create decision plan request
	planRequest := &DecisionPlanRequest{
		FindingID:     finding.ID,
		HostID:        finding.HostID,
		Severity:      finding.Severity,
		Confidence:    finding.Confidence,
		RuleID:        finding.RuleID,
		StrategyMode:  di.determineStrategyMode(finding),
		Priority:      di.calculatePriority(finding),
		AutoApprove:   di.shouldAutoApprove(finding),
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:     time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
	}

	// Publish to decision engine
	if err := di.publishDecisionPlan(planRequest); err != nil {
		return fmt.Errorf("failed to publish decision plan: %w", err)
	}

	di.logger.Info("Created decision plan for finding",
		"finding_id", finding.ID,
		"host_id", finding.HostID,
		"strategy_mode", planRequest.StrategyMode,
		"priority", planRequest.Priority)

	return nil
}

// determineStrategyMode determines the strategy mode for a finding
func (di *DecisionIntegration) determineStrategyMode(finding *Finding) string {
	// Conservative for high confidence, critical findings
	if finding.Severity == "critical" && finding.Confidence > 0.9 {
		return "conservative"
	}

	// Aggressive for high confidence, high severity
	if finding.Severity == "high" && finding.Confidence > 0.8 {
		return "aggressive"
	}

	// Balanced for everything else
	return "balanced"
}

// calculatePriority calculates the priority for a finding
func (di *DecisionIntegration) calculatePriority(finding *Finding) int {
	priority := 5 // Default priority

	// Adjust based on severity
	switch finding.Severity {
	case "critical":
		priority = 1
	case "high":
		priority = 2
	case "medium":
		priority = 3
	case "low":
		priority = 4
	}

	// Adjust based on confidence
	if finding.Confidence > 0.9 {
		priority -= 1
	} else if finding.Confidence < 0.8 {
		priority += 1
	}

	// Adjust based on type
	if di.isNetworkRisk(finding) {
		priority -= 1 // Network risks are higher priority
	}

	return priority
}

// shouldAutoApprove determines if a finding should be auto-approved
func (di *DecisionIntegration) shouldAutoApprove(finding *Finding) bool {
	// Auto-approve only for very high confidence, critical findings
	return finding.Severity == "critical" && finding.Confidence > 0.95
}

// publishMapSnapshot publishes a MapSnapshot to the orchestrator
func (di *DecisionIntegration) publishMapSnapshot(msg *MapSnapshotMessage) error {
	if di.natsConn == nil || !di.natsConn.IsConnected() {
		return fmt.Errorf("NATS connection not available")
	}

	// Serialize message
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal MapSnapshot message: %w", err)
	}

	// Create NATS headers
	headers := nats.Header{}
	headers.Set("x-finding-id", msg.FindingID)
	headers.Set("x-host-id", msg.HostID)
	headers.Set("x-service-id", fmt.Sprintf("%d", msg.MapSnapshot.ServiceID))
	headers.Set("x-confidence", fmt.Sprintf("%.2f", msg.Confidence))
	headers.Set("x-created-at", msg.CreatedAt)

	// Publish to orchestrator
	natsMsg := &nats.Msg{
		Subject: "orchestrator.mapsnapshots",
		Data:    msgJSON,
		Header:  headers,
	}

	if err := di.natsConn.PublishMsg(natsMsg); err != nil {
		return fmt.Errorf("failed to publish MapSnapshot: %w", err)
	}

	di.logger.Info("Published MapSnapshot to orchestrator",
		"finding_id", msg.FindingID,
		"host_id", msg.HostID,
		"service_id", msg.MapSnapshot.ServiceID,
		"subject", "orchestrator.mapsnapshots")

	return nil
}

// publishDecisionPlan publishes a decision plan to the decision engine
func (di *DecisionIntegration) publishDecisionPlan(plan *DecisionPlanRequest) error {
	if di.natsConn == nil || !di.natsConn.IsConnected() {
		return fmt.Errorf("NATS connection not available")
	}

	// Serialize plan
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("failed to marshal decision plan: %w", err)
	}

	// Create NATS headers
	headers := nats.Header{}
	headers.Set("x-finding-id", plan.FindingID)
	headers.Set("x-host-id", plan.HostID)
	headers.Set("x-severity", plan.Severity)
	headers.Set("x-priority", fmt.Sprintf("%d", plan.Priority))
	headers.Set("x-strategy-mode", plan.StrategyMode)
	headers.Set("x-auto-approve", fmt.Sprintf("%t", plan.AutoApprove))

	// Publish to decision engine
	natsMsg := &nats.Msg{
		Subject: "decision.plans",
		Data:    planJSON,
		Header:  headers,
	}

	if err := di.natsConn.PublishMsg(natsMsg); err != nil {
		return fmt.Errorf("failed to publish decision plan: %w", err)
	}

	di.logger.Info("Published decision plan to decision engine",
		"finding_id", plan.FindingID,
		"host_id", plan.HostID,
		"strategy_mode", plan.StrategyMode,
		"subject", "decision.plans")

	return nil
}

// GetStatistics returns integration statistics
func (di *DecisionIntegration) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"plans_created":      di.plansCreated,
		"safeguards_sent":    di.safeguardsSent,
		"mapsnapshots_sent":  di.mapsnapshotsSent,
		"errors":             di.errors,
		"success_rate":       di.calculateSuccessRate(),
	}
}

// calculateSuccessRate calculates the success rate
func (di *DecisionIntegration) calculateSuccessRate() float64 {
	total := di.plansCreated + di.safeguardsSent + di.mapsnapshotsSent
	if total == 0 {
		return 1.0
	}
	return float64(total-di.errors) / float64(total)
}

// MapSnapshotMessage represents a MapSnapshot message to the orchestrator
type MapSnapshotMessage struct {
	FindingID   string            `json:"finding_id"`
	HostID      string            `json:"host_id"`
	MapSnapshot *MapSnapshotDraft `json:"map_snapshot"`
	Reason      string            `json:"reason"`
	Confidence  float64           `json:"confidence"`
	CreatedAt   string            `json:"created_at"`
	ExpiresAt   string            `json:"expires_at"`
}

// DecisionPlanRequest represents a decision plan request to the decision engine
type DecisionPlanRequest struct {
	FindingID    string `json:"finding_id"`
	HostID       string `json:"host_id"`
	Severity     string `json:"severity"`
	Confidence   float64 `json:"confidence"`
	RuleID       string `json:"rule_id"`
	StrategyMode string `json:"strategy_mode"`
	Priority     int    `json:"priority"`
	AutoApprove  bool   `json:"auto_approve"`
	CreatedAt    string `json:"created_at"`
	ExpiresAt    string `json:"expires_at"`
}
