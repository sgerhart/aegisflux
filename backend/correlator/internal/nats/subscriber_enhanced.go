package nats

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aegisflux/correlator/internal/rules"
	"github.com/aegisflux/correlator/internal/store"
	"github.com/nats-io/nats.go"
)

// EnhancedSubscriber handles NATS subscriptions with decision integration
type EnhancedSubscriber struct {
	nc                  *nats.Conn
	logger              *slog.Logger
	store               *store.MemoryStore
	ruleLoader          *rules.Loader
	findingGenerator    *rules.FindingGenerator
	findingPublisher    *rules.FindingPublisher
	decisionIntegration *rules.DecisionIntegration
	running             bool
}

// NewEnhancedSubscriber creates a new enhanced NATS subscriber
func NewEnhancedSubscriber(nc *nats.Conn, logger *slog.Logger) *EnhancedSubscriber {
	return &EnhancedSubscriber{
		nc:     nc,
		logger: logger,
	}
}

// SetupFindingProcessing sets up finding processing with decision integration
func (s *EnhancedSubscriber) SetupFindingProcessing(
	store *store.MemoryStore,
	ruleLoader *rules.Loader,
	findingGenerator *rules.FindingGenerator,
	findingPublisher *rules.FindingPublisher,
	decisionIntegration *rules.DecisionIntegration,
) {
	s.store = store
	s.ruleLoader = ruleLoader
	s.findingGenerator = findingGenerator
	s.findingPublisher = findingPublisher
	s.decisionIntegration = decisionIntegration
}

// Start starts the enhanced NATS subscriber
func (s *EnhancedSubscriber) Start() error {
	s.running = true

	// Subscribe to enriched events
	if err := s.subscribeToEnrichedEvents(); err != nil {
		return err
	}

	// Subscribe to package CVE enriched events
	if err := s.subscribeToPackageCVEEnriched(); err != nil {
		return err
	}

	// Subscribe to CVE updates
	if err := s.subscribeToCVEUpdates(); err != nil {
		return err
	}

	// Subscribe to package CVE mappings
	if err := s.subscribeToPackageCVEMappings(); err != nil {
		return err
	}

	s.logger.Info("Enhanced NATS subscriber started")
	return nil
}

// Stop stops the enhanced NATS subscriber
func (s *EnhancedSubscriber) Stop() error {
	s.running = false
	s.logger.Info("Enhanced NATS subscriber stopped")
	return nil
}

// subscribeToEnrichedEvents subscribes to etl.enriched events
func (s *EnhancedSubscriber) subscribeToEnrichedEvents() error {
	_, err := s.nc.Subscribe("etl.enriched", s.handleEnrichedEvent)
	if err != nil {
		return err
	}
	s.logger.Info("Subscribed to etl.enriched")
	return nil
}

// subscribeToPackageCVEEnriched subscribes to package CVE enriched events
func (s *EnhancedSubscriber) subscribeToPackageCVEEnriched() error {
	_, err := s.nc.Subscribe("etl.enriched", s.handlePackageCVEEnriched)
	if err != nil {
		return err
	}
	s.logger.Info("Subscribed to package CVE enriched events")
	return nil
}

// subscribeToCVEUpdates subscribes to CVE updates
func (s *EnhancedSubscriber) subscribeToCVEUpdates() error {
	_, err := s.nc.Subscribe("feeds.cve.updates", s.handleCVEUpdate)
	if err != nil {
		return err
	}
	s.logger.Info("Subscribed to feeds.cve.updates")
	return nil
}

// subscribeToPackageCVEMappings subscribes to package CVE mappings
func (s *EnhancedSubscriber) subscribeToPackageCVEMappings() error {
	_, err := s.nc.Subscribe("feeds.pkg.cve", s.handlePackageCVEMapping)
	if err != nil {
		return err
	}
	s.logger.Info("Subscribed to feeds.pkg.cve")
	return nil
}

// handleEnrichedEvent handles enriched events
func (s *EnhancedSubscriber) handleEnrichedEvent(msg *nats.Msg) {
	var enrichedEvent map[string]interface{}
	if err := json.Unmarshal(msg.Data, &enrichedEvent); err != nil {
		s.logger.Error("Failed to unmarshal enriched event", "error", err)
		return
	}

	// Check if this is a package CVE enriched event
	if recordType, ok := enrichedEvent["record_type"].(string); ok && recordType == "pkg_cve_enriched" {
		s.handlePackageCVEEnriched(msg)
		return
	}

	// Process regular enriched event
	s.processEnrichedEvent(enrichedEvent)
}

// handlePackageCVEEnriched handles package CVE enriched events
func (s *EnhancedSubscriber) handlePackageCVEEnriched(msg *nats.Msg) {
	var pkgCVEEnriched map[string]interface{}
	if err := json.Unmarshal(msg.Data, &pkgCVEEnriched); err != nil {
		s.logger.Error("Failed to unmarshal package CVE enriched event", "error", err)
		return
	}

	// Process package CVE enriched event
	s.processPackageCVEEnriched(pkgCVEEnriched)
}

// handleCVEUpdate handles CVE updates
func (s *EnhancedSubscriber) handleCVEUpdate(msg *nats.Msg) {
	var cveUpdate map[string]interface{}
	if err := json.Unmarshal(msg.Data, &cveUpdate); err != nil {
		s.logger.Error("Failed to unmarshal CVE update", "error", err)
		return
	}

	s.logger.Debug("Received CVE update", "cve_id", cveUpdate["cve_id"])
	// CVE updates are handled by the ETL enrich service
}

// handlePackageCVEMapping handles package CVE mappings
func (s *EnhancedSubscriber) handlePackageCVEMapping(msg *nats.Msg) {
	var pkgCVEMapping map[string]interface{}
	if err := json.Unmarshal(msg.Data, &pkgCVEMapping); err != nil {
		s.logger.Error("Failed to unmarshal package CVE mapping", "error", err)
		return
	}

	s.logger.Debug("Received package CVE mapping", "host_id", pkgCVEMapping["host_id"])
	// Package CVE mappings are handled by the ETL enrich service
}

// processEnrichedEvent processes a regular enriched event
func (s *EnhancedSubscriber) processEnrichedEvent(enrichedEvent map[string]interface{}) {
	// Extract host ID
	hostID, ok := enrichedEvent["host_id"].(string)
	if !ok {
		s.logger.Warn("Enriched event missing host_id")
		return
	}

	// Get current rules
	snapshot := s.ruleLoader.GetSnapshot()
	if len(snapshot.Rules) == 0 {
		s.logger.Debug("No rules loaded")
		return
	}

	// Process each rule
	for _, rule := range snapshot.Rules {
		if !rule.IsEnabled() {
			continue
		}

		// Check if rule matches
		if s.ruleMatches(rule, enrichedEvent) {
			// Generate finding
			finding, err := s.findingGenerator.GenerateFindingFromEnrichedEvent(
				&rule,
				enrichedEvent,
				[]string{"enriched-event"},
				time.Now().Add(-5*time.Minute),
				time.Now(),
			)
			if err != nil {
				s.logger.Error("Failed to generate finding", "error", err)
				continue
			}

			// Store finding
			s.store.AddFinding(finding)

			// Publish finding
			if err := s.findingPublisher.PublishFinding(finding); err != nil {
				s.logger.Error("Failed to publish finding", "error", err)
				continue
			}

			// Process with decision integration
			if err := s.decisionIntegration.ProcessFinding(finding); err != nil {
				s.logger.Error("Failed to process finding with decision integration", "error", err)
				continue
			}

			s.logger.Info("Processed enriched event with rule",
				"rule_id", rule.Metadata.ID,
				"finding_id", finding.ID,
				"host_id", hostID)
		}
	}
}

// processPackageCVEEnriched processes a package CVE enriched event
func (s *EnhancedSubscriber) processPackageCVEEnriched(pkgCVEEnriched map[string]interface{}) {
	// Extract host ID
	hostID, ok := pkgCVEEnriched["host_id"].(string)
	if !ok {
		s.logger.Warn("Package CVE enriched event missing host_id")
		return
	}

	// Get current rules
	snapshot := s.ruleLoader.GetSnapshot()
	if len(snapshot.Rules) == 0 {
		s.logger.Debug("No rules loaded")
		return
	}

	// Process each rule
	for _, rule := range snapshot.Rules {
		if !rule.IsEnabled() {
			continue
		}

		// Check if rule matches package CVE enriched event
		if s.ruleMatchesPackageCVE(rule, pkgCVEEnriched) {
			// Generate finding
			finding, err := s.findingGenerator.GenerateFindingFromPackageCVE(
				&rule,
				pkgCVEEnriched,
				[]string{"package-cve-enriched"},
				time.Now().Add(-5*time.Minute),
				time.Now(),
			)
			if err != nil {
				s.logger.Error("Failed to generate finding from package CVE", "error", err)
				continue
			}

			// Store finding
			s.store.AddFinding(finding)

			// Publish finding
			if err := s.findingPublisher.PublishFinding(finding); err != nil {
				s.logger.Error("Failed to publish finding", "error", err)
				continue
			}

			// Process with decision integration
			if err := s.decisionIntegration.ProcessFinding(finding); err != nil {
				s.logger.Error("Failed to process finding with decision integration", "error", err)
				continue
			}

			s.logger.Info("Processed package CVE enriched event with rule",
				"rule_id", rule.Metadata.ID,
				"finding_id", finding.ID,
				"host_id", hostID)
		}
	}
}

// ruleMatches checks if a rule matches an enriched event
func (s *EnhancedSubscriber) ruleMatches(rule *rules.Rule, enrichedEvent map[string]interface{}) bool {
	// Check host selectors
	if !s.hostMatches(rule.Spec.Selectors, enrichedEvent) {
		return false
	}

	// Check event type
	if rule.Spec.Condition.When != nil {
		if eventType, ok := rule.Spec.Condition.When["event_type"].(string); ok {
			if enrichedEventType, ok := enrichedEvent["type"].(string); ok {
				if eventType != enrichedEventType {
					return false
				}
			}
		}
	}

	// Check other conditions
	return s.checkConditions(rule.Spec.Condition.When, enrichedEvent)
}

// ruleMatchesPackageCVE checks if a rule matches a package CVE enriched event
func (s *EnhancedSubscriber) ruleMatchesPackageCVE(rule *rules.Rule, pkgCVEEnriched map[string]interface{}) bool {
	// Check host selectors
	if !s.hostMatches(rule.Spec.Selectors, pkgCVEEnriched) {
		return false
	}

	// Check record type
	if rule.Spec.Condition.When != nil {
		if recordType, ok := rule.Spec.Condition.When["record_type"].(string); ok {
			if pkgCVERecordType, ok := pkgCVEEnriched["record_type"].(string); ok {
				if recordType != pkgCVERecordType {
					return false
				}
			}
		}
	}

	// Check enrichment conditions
	if enrichment, ok := pkgCVEEnriched["enrichment"].(map[string]interface{}); ok {
		if !s.checkConditions(rule.Spec.Condition.When, enrichment) {
			return false
		}
	}

	return true
}

// hostMatches checks if a host matches the selectors
func (s *EnhancedSubscriber) hostMatches(selectors rules.Selector, event map[string]interface{}) bool {
	hostID, ok := event["host_id"].(string)
	if !ok {
		return false
	}

	// Check host IDs
	if len(selectors.HostIDs) > 0 {
		found := false
		for _, id := range selectors.HostIDs {
			if id == hostID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check exclude host IDs
	for _, id := range selectors.ExcludeHostIDs {
		if id == hostID {
			return false
		}
	}

	// TODO: Implement other selector checks (patterns, environment, etc.)

	return true
}

// checkConditions checks if conditions match the event
func (s *EnhancedSubscriber) checkConditions(conditions map[string]interface{}, event map[string]interface{}) bool {
	if conditions == nil {
		return true
	}

	for key, expectedValue := range conditions {
		actualValue, ok := event[key]
		if !ok {
			return false
		}

		// Simple equality check for now
		if actualValue != expectedValue {
			return false
		}
	}

	return true
}
