package rules

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FindingGenerator handles the generation of findings from rule matches
type FindingGenerator struct {
	logger *slog.Logger
}

// NewFindingGenerator creates a new finding generator
func NewFindingGenerator(logger *slog.Logger) *FindingGenerator {
	return &FindingGenerator{
		logger: logger,
	}
}

// GenerateFinding creates a finding from a rule match
func (fg *FindingGenerator) GenerateFinding(
	rule *Rule,
	hostID string,
	evidence map[string]interface{},
	sourceEvents []string,
	windowStart, windowEnd time.Time,
) (*Finding, error) {
	// Generate unique finding ID
	findingID, err := fg.generateFindingID(rule.Metadata.ID, hostID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate finding ID: %w", err)
	}

	// Generate correlation ID
	correlationID := fg.generateCorrelationID(rule.Metadata.ID, hostID)

	// Create finding
	finding := &Finding{
		ID:            findingID,
		HostID:        hostID,
		Severity:      rule.Spec.Outcome.Severity,
		Type:          fg.generateFindingType(rule),
		Evidence:      evidence,
		TS:            time.Now().UTC().Format(time.RFC3339),
		RuleID:        rule.Metadata.ID,
		RuleName:      rule.Metadata.Name,
		Confidence:    rule.Spec.Outcome.Confidence,
		Title:         fg.generateTitle(rule, hostID),
		Description:   fg.generateDescription(rule, evidence),
		Tags:          fg.generateTags(rule, evidence),
		Metadata:      fg.generateMetadata(rule, evidence),
		Actions:       rule.Spec.Outcome.Actions,
		CorrelationID: correlationID,
		SourceEvents:  sourceEvents,
		WindowStart:   windowStart.UTC().Format(time.RFC3339),
		WindowEnd:     windowEnd.UTC().Format(time.RFC3339),
	}

	// Validate finding
	if err := finding.Validate(); err != nil {
		return nil, fmt.Errorf("invalid finding generated: %w", err)
	}

	fg.logger.Info("Generated finding",
		"finding_id", finding.ID,
		"rule_id", rule.Metadata.ID,
		"host_id", hostID,
		"severity", finding.Severity,
		"confidence", finding.Confidence)

	return finding, nil
}

// generateFindingID creates a unique finding ID
func (fg *FindingGenerator) generateFindingID(ruleID, hostID string) (string, error) {
	// Create a deterministic but unique ID based on rule, host, and timestamp
	timestamp := time.Now().UnixNano()
	base := fmt.Sprintf("%s-%s-%d", ruleID, hostID, timestamp)
	
	// Add some randomness to ensure uniqueness
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	randomStr := hex.EncodeToString(randomBytes)
	
	return fmt.Sprintf("finding-%s-%s", base, randomStr), nil
}

// generateCorrelationID creates a correlation ID for related findings
func (fg *FindingGenerator) generateCorrelationID(ruleID, hostID string) string {
	// Use UUID for correlation ID
	return uuid.New().String()
}

// generateFindingType creates a finding type based on the rule
func (fg *FindingGenerator) generateFindingType(rule *Rule) string {
	// Extract type from rule name or use default
	if rule.Metadata.Name != "" {
		// Convert rule name to type format
		ruleType := strings.ToLower(rule.Metadata.Name)
		ruleType = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(ruleType, "_")
		ruleType = strings.Trim(ruleType, "_")
		return ruleType
	}
	
	// Default type based on severity
	switch rule.Spec.Outcome.Severity {
	case "critical":
		return "critical_security_incident"
	case "high":
		return "high_security_alert"
	case "medium":
		return "medium_security_warning"
	case "low":
		return "low_security_notice"
	default:
		return "security_finding"
	}
}

// generateTitle creates a finding title
func (fg *FindingGenerator) generateTitle(rule *Rule, hostID string) string {
	if rule.Spec.Outcome.Title != "" {
		return rule.Spec.Outcome.Title
	}
	
	// Generate title from rule name and host
	return fmt.Sprintf("%s detected on %s", rule.Metadata.Name, hostID)
}

// generateDescription creates a finding description
func (fg *FindingGenerator) generateDescription(rule *Rule, evidence map[string]interface{}) string {
	if rule.Spec.Outcome.Description != "" {
		return rule.Spec.Outcome.Description
	}
	
	// Generate description from evidence
	var descParts []string
	descParts = append(descParts, fmt.Sprintf("Rule '%s' triggered", rule.Metadata.Name))
	
	if len(evidence) > 0 {
		descParts = append(descParts, "Evidence:")
		for key, value := range evidence {
			descParts = append(descParts, fmt.Sprintf("- %s: %v", key, value))
		}
	}
	
	return strings.Join(descParts, "\n")
}

// generateTags creates tags for the finding
func (fg *FindingGenerator) generateTags(rule *Rule, evidence map[string]interface{}) []string {
	tags := make([]string, 0)
	
	// Add rule tags
	tags = append(tags, rule.Spec.Outcome.Tags...)
	
	// Add severity tag
	tags = append(tags, "severity:"+rule.Spec.Outcome.Severity)
	
	// Add rule ID tag
	tags = append(tags, "rule:"+rule.Metadata.ID)
	
	// Add evidence-based tags
	for key, value := range evidence {
		if str, ok := value.(string); ok && len(str) < 50 {
			tags = append(tags, fmt.Sprintf("%s:%s", key, str))
		}
	}
	
	return tags
}

// generateMetadata creates metadata for the finding
func (fg *FindingGenerator) generateMetadata(rule *Rule, evidence map[string]interface{}) map[string]interface{} {
	metadata := make(map[string]interface{})
	
	// Copy rule metadata
	metadata["rule_version"] = rule.Metadata.Version
	metadata["rule_source_file"] = rule.SourceFile
	metadata["rule_enabled"] = rule.Spec.Enabled
	
	// Add temporal information
	metadata["window_seconds"] = rule.Spec.Condition.WindowSeconds
	if rule.Spec.Condition.TemporalWindow != nil {
		metadata["temporal_window_duration"] = rule.Spec.Condition.TemporalWindow.DurationSeconds
		metadata["temporal_window_type"] = rule.Spec.Condition.TemporalWindow.WindowType
	}
	
	// Add evidence summary
	metadata["evidence_count"] = len(evidence)
	metadata["evidence_keys"] = make([]string, 0, len(evidence))
	for key := range evidence {
		metadata["evidence_keys"] = append(metadata["evidence_keys"].([]string), key)
	}
	
	// Add rule outcome metadata
	metadata["outcome_confidence"] = rule.Spec.Outcome.Confidence
	metadata["outcome_evidence_count"] = len(rule.Spec.Outcome.Evidence)
	
	return metadata
}

// GenerateFindingFromEnrichedEvent creates a finding from an enriched event
func (fg *FindingGenerator) GenerateFindingFromEnrichedEvent(
	rule *Rule,
	enrichedEvent map[string]interface{},
	sourceEvents []string,
	windowStart, windowEnd time.Time,
) (*Finding, error) {
	// Extract host ID from enriched event
	hostID, ok := enrichedEvent["host_id"].(string)
	if !ok {
		return nil, fmt.Errorf("enriched event missing host_id")
	}
	
	// Create evidence from enriched event
	evidence := make(map[string]interface{})
	evidence["enriched_event"] = enrichedEvent
	evidence["event_type"] = enrichedEvent["type"]
	evidence["event_source"] = enrichedEvent["source"]
	
	// Add context if available
	if context, ok := enrichedEvent["context"].(map[string]interface{}); ok {
		evidence["context"] = context
	}
	
	// Add payload if available
	if payload, ok := enrichedEvent["payload"].(string); ok {
		evidence["payload"] = payload
	}
	
	// Add args if available
	if args, ok := enrichedEvent["args"].(map[string]interface{}); ok {
		evidence["args"] = args
	}
	
	return fg.GenerateFinding(rule, hostID, evidence, sourceEvents, windowStart, windowEnd)
}

// GenerateFindingFromPackageCVE creates a finding from a package CVE enriched event
func (fg *FindingGenerator) GenerateFindingFromPackageCVE(
	rule *Rule,
	pkgCVEEnriched map[string]interface{},
	sourceEvents []string,
	windowStart, windowEnd time.Time,
) (*Finding, error) {
	// Extract host ID from package CVE enriched event
	hostID, ok := pkgCVEEnriched["host_id"].(string)
	if !ok {
		return nil, fmt.Errorf("package CVE enriched event missing host_id")
	}
	
	// Create evidence from package CVE data
	evidence := make(map[string]interface{})
	evidence["package_cve_enriched"] = pkgCVEEnriched
	
	// Add package information
	if packageInfo, ok := pkgCVEEnriched["package"].(map[string]interface{}); ok {
		evidence["package_name"] = packageInfo["name"]
		evidence["package_version"] = packageInfo["version"]
		evidence["package_distro"] = packageInfo["distro"]
	}
	
	// Add CVE candidate information
	if cveCandidate, ok := pkgCVEEnriched["cve_candidate"].(map[string]interface{}); ok {
		evidence["cve_id"] = cveCandidate["cve_id"]
		evidence["cve_score"] = cveCandidate["score"]
		evidence["cve_severity"] = cveCandidate["severity"]
		evidence["cve_cvss_score"] = cveCandidate["cvss_score"]
	}
	
	// Add enrichment information
	if enrichment, ok := pkgCVEEnriched["enrichment"].(map[string]interface{}); ok {
		evidence["exploitability_score"] = enrichment["exploitability_score"]
		evidence["risk_level"] = enrichment["risk_level"]
	}
	
	// Add CVE data if available
	if cveData, ok := pkgCVEEnriched["cve_data"].(map[string]interface{}); ok {
		evidence["cve_data"] = cveData
	}
	
	return fg.GenerateFinding(rule, hostID, evidence, sourceEvents, windowStart, windowEnd)
}
