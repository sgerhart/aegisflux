package rules

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// FindingForwarder handles forwarding high-severity findings to the Decision Engine
type FindingForwarder struct {
	natsConn *nats.Conn
	logger   *slog.Logger
	// Configuration
	highSeverityThresholds map[string]bool
	networkRiskTypes       map[string]bool
	// Statistics
	forwardedCount    int64
	networkRiskCount  int64
	adaptiveSafeguardCount int64
}

// NewFindingForwarder creates a new finding forwarder
func NewFindingForwarder(natsConn *nats.Conn, logger *slog.Logger) *FindingForwarder {
	return &FindingForwarder{
		natsConn: natsConn,
		logger:   logger,
		highSeverityThresholds: map[string]bool{
			"high":    true,
			"critical": true,
		},
		networkRiskTypes: map[string]bool{
			"network":           true,
			"connect":           true,
			"network_scan":      true,
			"port_scan":         true,
			"reconnaissance":    true,
			"data_exfiltration": true,
			"lateral_movement":  true,
		},
	}
}

// AdaptiveSafeguard represents a proposed adaptive safeguard
type AdaptiveSafeguard struct {
	ID              string                 `json:"id"`
	FindingID       string                 `json:"finding_id"`
	Type            string                 `json:"type"`            // "network_restriction", "service_isolation", "traffic_blocking"
	Severity        string                 `json:"severity"`
	Confidence      float64                `json:"confidence"`
	HostID          string                 `json:"host_id"`
	RuleID          string                 `json:"rule_id"`
	Description     string                 `json:"description"`
	ProposedAction  string                 `json:"proposed_action"`
	MapSnapshot     *MapSnapshotDraft      `json:"map_snapshot,omitempty"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       string                 `json:"created_at"`
	ExpiresAt       string                 `json:"expires_at"`
	Priority        int                    `json:"priority"`
	AutoApprove     bool                   `json:"auto_approve"`
}

// MapSnapshotDraft represents a proposed MapSnapshot for network risks
type MapSnapshotDraft struct {
	ServiceID   int                    `json:"service_id"`
	Edges       []NetworkEdge          `json:"edges"`
	AllowCIDRs  []NetworkEdge          `json:"allow_cidrs,omitempty"`
	TTLSeconds  int                    `json:"ttl_seconds"`
	Meta        map[string]interface{} `json:"meta"`
	Reason      string                 `json:"reason"`
	Confidence  float64                `json:"confidence"`
}

// NetworkEdge represents a network edge in a MapSnapshot
type NetworkEdge struct {
	DstCIDR string `json:"dst_cidr"`
	Proto   string `json:"proto"`
	Port    int    `json:"port"`
}

// ForwardFinding forwards a finding to the Decision Engine if it meets criteria
func (ff *FindingForwarder) ForwardFinding(finding *Finding) error {
	if ff.natsConn == nil || !ff.natsConn.IsConnected() {
		return fmt.Errorf("NATS connection not available")
	}

	// Check if finding should be forwarded
	if !ff.shouldForward(finding) {
		ff.logger.Debug("Finding does not meet forwarding criteria",
			"finding_id", finding.ID,
			"severity", finding.Severity,
			"type", finding.Type)
		return nil
	}

	// Create adaptive safeguard
	safeguard, err := ff.createAdaptiveSafeguard(finding)
	if err != nil {
		return fmt.Errorf("failed to create adaptive safeguard: %w", err)
	}

	// Forward to Decision Engine
	if err := ff.forwardToDecisionEngine(safeguard); err != nil {
		return fmt.Errorf("failed to forward to decision engine: %w", err)
	}

	// Update statistics
	ff.forwardedCount++
	if ff.isNetworkRisk(finding) {
		ff.networkRiskCount++
	}
	ff.adaptiveSafeguardCount++

	ff.logger.Info("Forwarded finding as adaptive safeguard",
		"finding_id", finding.ID,
		"safeguard_id", safeguard.ID,
		"type", safeguard.Type,
		"severity", safeguard.Severity,
		"host_id", safeguard.HostID)

	return nil
}

// shouldForward determines if a finding should be forwarded
func (ff *FindingForwarder) shouldForward(finding *Finding) bool {
	// Check severity threshold
	if !ff.highSeverityThresholds[finding.Severity] {
		return false
	}

	// Check confidence threshold
	if finding.Confidence < 0.7 {
		return false
	}

	// Check if it's a network risk or has network-related tags
	if ff.isNetworkRisk(finding) {
		return true
	}

	// Check for specific rule types that should be forwarded
	networkRuleTypes := []string{
		"network_scan",
		"port_scan",
		"reconnaissance",
		"data_exfiltration",
		"lateral_movement",
		"unauthorized_access",
		"privilege_escalation",
	}

	for _, ruleType := range networkRuleTypes {
		if finding.Type == ruleType {
			return true
		}
	}

	return false
}

// isNetworkRisk determines if a finding represents a network risk
func (ff *FindingForwarder) isNetworkRisk(finding *Finding) bool {
	// Check finding type
	if ff.networkRiskTypes[finding.Type] {
		return true
	}

	// Check tags for network-related indicators
	for _, tag := range finding.Tags {
		if ff.networkRiskTypes[tag] {
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

// createAdaptiveSafeguard creates an adaptive safeguard from a finding
func (ff *FindingForwarder) createAdaptiveSafeguard(finding *Finding) (*AdaptiveSafeguard, error) {
	safeguard := &AdaptiveSafeguard{
		ID:          fmt.Sprintf("safeguard-%s-%d", finding.ID, time.Now().UnixNano()),
		FindingID:   finding.ID,
		Type:        ff.determineSafeguardType(finding),
		Severity:    finding.Severity,
		Confidence:  finding.Confidence,
		HostID:      finding.HostID,
		RuleID:      finding.RuleID,
		Description: finding.Description,
		Metadata:    finding.Metadata,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
		ExpiresAt:   time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339),
		Priority:    ff.calculatePriority(finding),
		AutoApprove: ff.shouldAutoApprove(finding),
	}

	// Generate proposed action
	safeguard.ProposedAction = ff.generateProposedAction(finding, safeguard.Type)

	// Create MapSnapshot draft for network risks
	if ff.isNetworkRisk(finding) {
		mapSnapshot, err := ff.createMapSnapshotDraft(finding)
		if err != nil {
			ff.logger.Warn("Failed to create MapSnapshot draft", "error", err)
		} else {
			safeguard.MapSnapshot = mapSnapshot
		}
	}

	return safeguard, nil
}

// determineSafeguardType determines the type of safeguard to propose
func (ff *FindingForwarder) determineSafeguardType(finding *Finding) string {
	if ff.isNetworkRisk(finding) {
		return "network_restriction"
	}

	// Check for specific finding types
	switch finding.Type {
	case "unauthorized_access", "privilege_escalation":
		return "service_isolation"
	case "data_exfiltration", "lateral_movement":
		return "traffic_blocking"
	case "reconnaissance", "network_scan":
		return "network_restriction"
	default:
		return "service_isolation"
	}
}

// calculatePriority calculates the priority of the safeguard
func (ff *FindingForwarder) calculatePriority(finding *Finding) int {
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
	if ff.isNetworkRisk(finding) {
		priority -= 1 // Network risks are higher priority
	}

	return priority
}

// shouldAutoApprove determines if the safeguard should be auto-approved
func (ff *FindingForwarder) shouldAutoApprove(finding *Finding) bool {
	// Auto-approve only for very high confidence, critical findings
	return finding.Severity == "critical" && finding.Confidence > 0.95
}

// generateProposedAction generates a human-readable proposed action
func (ff *FindingForwarder) generateProposedAction(finding *Finding, safeguardType string) string {
	switch safeguardType {
	case "network_restriction":
		return fmt.Sprintf("Restrict network access for host %s to prevent %s", finding.HostID, finding.Type)
	case "service_isolation":
		return fmt.Sprintf("Isolate service on host %s due to %s", finding.HostID, finding.Type)
	case "traffic_blocking":
		return fmt.Sprintf("Block suspicious traffic from host %s", finding.HostID)
	default:
		return fmt.Sprintf("Apply security measures to host %s", finding.HostID)
	}
}

// createMapSnapshotDraft creates a MapSnapshot draft for network risks
func (ff *FindingForwarder) createMapSnapshotDraft(finding *Finding) (*MapSnapshotDraft, error) {
	// Extract network information from finding evidence
	edges, err := ff.extractNetworkEdges(finding)
	if err != nil {
		return nil, fmt.Errorf("failed to extract network edges: %w", err)
	}

	// Generate service ID (in real implementation, this would be more sophisticated)
	serviceID := ff.generateServiceID(finding)

	// Calculate TTL based on severity and confidence
	ttlSeconds := ff.calculateTTL(finding)

	// Create MapSnapshot draft
	draft := &MapSnapshotDraft{
		ServiceID:  serviceID,
		Edges:      edges,
		TTLSeconds: ttlSeconds,
		Meta: map[string]interface{}{
			"finding_id":    finding.ID,
			"rule_id":       finding.RuleID,
			"created_by":    "correlator",
			"reason":        finding.Description,
			"severity":      finding.Severity,
			"confidence":    finding.Confidence,
			"auto_generated": true,
		},
		Reason:     fmt.Sprintf("Generated from finding %s: %s", finding.ID, finding.Description),
		Confidence: finding.Confidence,
	}

	return draft, nil
}

// extractNetworkEdges extracts network edges from finding evidence
func (ff *FindingForwarder) extractNetworkEdges(finding *Finding) ([]NetworkEdge, error) {
	var edges []NetworkEdge

	// Extract from evidence
	if dstIP, ok := finding.Evidence["dst_ip"].(string); ok {
		if dstPort, ok := finding.Evidence["dst_port"].(string); ok {
			port := 80 // Default port
			if portStr, err := fmt.Sscanf(dstPort, "%d", &port); err == nil && portStr == 1 {
				// Port parsed successfully
			}

			proto := "tcp" // Default protocol
			if protoStr, ok := finding.Evidence["proto"].(string); ok {
				proto = protoStr
			}

			edges = append(edges, NetworkEdge{
				DstCIDR: dstIP + "/32",
				Proto:   proto,
				Port:    port,
			})
		}
	}

	// If no specific edges found, create a restrictive default
	if len(edges) == 0 {
		edges = append(edges, NetworkEdge{
			DstCIDR: "0.0.0.0/0",
			Proto:   "any",
			Port:    0,
		})
	}

	return edges, nil
}

// generateServiceID generates a service ID for the MapSnapshot
func (ff *FindingForwarder) generateServiceID(finding *Finding) int {
	// Simple hash-based service ID generation
	// In real implementation, this would be more sophisticated
	hash := 0
	for _, char := range finding.HostID {
		hash = hash*31 + int(char)
	}
	return hash % 10000
}

// calculateTTL calculates TTL based on finding severity and confidence
func (ff *FindingForwarder) calculateTTL(finding *Finding) int {
	baseTTL := 3600 // 1 hour base

	// Adjust based on severity
	switch finding.Severity {
	case "critical":
		baseTTL = 7200 // 2 hours
	case "high":
		baseTTL = 3600 // 1 hour
	case "medium":
		baseTTL = 1800 // 30 minutes
	case "low":
		baseTTL = 900 // 15 minutes
	}

	// Adjust based on confidence
	if finding.Confidence > 0.9 {
		baseTTL = int(float64(baseTTL) * 1.5)
	} else if finding.Confidence < 0.8 {
		baseTTL = int(float64(baseTTL) * 0.5)
	}

	return baseTTL
}

// forwardToDecisionEngine forwards the adaptive safeguard to the Decision Engine
func (ff *FindingForwarder) forwardToDecisionEngine(safeguard *AdaptiveSafeguard) error {
	// Serialize safeguard to JSON
	safeguardJSON, err := json.Marshal(safeguard)
	if err != nil {
		return fmt.Errorf("failed to marshal safeguard: %w", err)
	}

	// Create NATS headers
	headers := nats.Header{}
	headers.Set("x-safeguard-id", safeguard.ID)
	headers.Set("x-finding-id", safeguard.FindingID)
	headers.Set("x-severity", safeguard.Severity)
	headers.Set("x-host-id", safeguard.HostID)
	headers.Set("x-priority", fmt.Sprintf("%d", safeguard.Priority))
	headers.Set("x-auto-approve", fmt.Sprintf("%t", safeguard.AutoApprove))

	// Publish to Decision Engine
	msg := &nats.Msg{
		Subject: "decision.adaptive_safeguards",
		Data:    safeguardJSON,
		Header:  headers,
	}

	if err := ff.natsConn.PublishMsg(msg); err != nil {
		return fmt.Errorf("failed to publish safeguard: %w", err)
	}

	ff.logger.Info("Forwarded adaptive safeguard to Decision Engine",
		"safeguard_id", safeguard.ID,
		"finding_id", safeguard.FindingID,
		"type", safeguard.Type,
		"severity", safeguard.Severity,
		"subject", "decision.adaptive_safeguards")

	return nil
}

// GetStatistics returns forwarding statistics
func (ff *FindingForwarder) GetStatistics() map[string]interface{} {
	return map[string]interface{}{
		"forwarded_count":         ff.forwardedCount,
		"network_risk_count":      ff.networkRiskCount,
		"adaptive_safeguard_count": ff.adaptiveSafeguardCount,
	}
}

// ForwardFindingsBatch forwards multiple findings
func (ff *FindingForwarder) ForwardFindingsBatch(findings []*Finding) error {
	var errors []error
	successCount := 0

	for _, finding := range findings {
		if err := ff.ForwardFinding(finding); err != nil {
			errors = append(errors, fmt.Errorf("finding %s: %w", finding.ID, err))
		} else {
			successCount++
		}
	}

	ff.logger.Info("Forwarded findings batch",
		"total", len(findings),
		"successful", successCount,
		"failed", len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("failed to forward %d findings: %v", len(errors), errors)
	}

	return nil
}
