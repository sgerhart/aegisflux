package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/sgerhart/aegisflux/backend/correlator/internal/metrics"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/model"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/rules"
	"github.com/sgerhart/aegisflux/backend/correlator/internal/store"
	"github.com/nats-io/nats.go"
)

// Subscriber handles NATS subscription for both enriched and raw events
type Subscriber struct {
	nc          *nats.Conn
	store       *store.MemoryStore
	ruleLoader  *rules.Loader
	matcher     *rules.Matcher
	windowBuffer *rules.WindowBuffer
	evaluator   *rules.Evaluator
	logger      *slog.Logger
	queue       string
	metrics     *metrics.Metrics
	
	// Metrics
	invalidEventsTotal int64
	
	// Subscriptions
	enrichedSub *nats.Subscription
	rawSub      *nats.Subscription
}

// NewSubscriber creates a new NATS subscriber
func NewSubscriber(nc *nats.Conn, store *store.MemoryStore, queue string, ruleLoader *rules.Loader, overrideManager *rules.OverrideManager, metrics *metrics.Metrics, logger *slog.Logger) *Subscriber {
	windowBuffer := rules.NewWindowBuffer(5 * time.Minute) // 5 minute window
	matcher := rules.NewMatcher()
	evaluator := rules.NewEvaluator(windowBuffer, matcher, ruleLoader, overrideManager, 300, metrics, logger) // 5 minute label TTL
	
	return &Subscriber{
		nc:           nc,
		store:        store,
		ruleLoader:   ruleLoader,
		matcher:      matcher,
		windowBuffer: windowBuffer,
		evaluator:    evaluator,
		logger:       logger,
		queue:        queue,
		metrics:      metrics,
	}
}

// Subscribe starts listening for both enriched and raw events
func (s *Subscriber) Subscribe(ctx context.Context) error {
	s.logger.Info("Subscribing to events", "queue", s.queue)

	// Subscribe to enriched events (preferred)
	enrichedSub, err := s.nc.QueueSubscribe("events.enriched", s.queue, s.handleEnrichedMessage)
	if err != nil {
		s.logger.Error("Failed to subscribe to enriched events", "error", err)
		return err
	}
	s.enrichedSub = enrichedSub
	s.logger.Info("Subscribed to enriched events", "subject", "events.enriched", "queue", s.queue)

	// Subscribe to raw events (fallback)
	rawSub, err := s.nc.QueueSubscribe("events.raw", s.queue, s.handleRawMessage)
	if err != nil {
		s.logger.Error("Failed to subscribe to raw events", "error", err)
		// Clean up enriched subscription
		enrichedSub.Unsubscribe()
		return err
	}
	s.rawSub = rawSub
	s.logger.Info("Subscribed to raw events", "subject", "events.raw", "queue", s.queue)

	// Wait for context cancellation
	<-ctx.Done()

	// Graceful shutdown with drain
	s.logger.Info("Starting graceful shutdown")
	if err := s.gracefulShutdown(); err != nil {
		s.logger.Error("Error during graceful shutdown", "error", err)
		return err
	}

	s.logger.Info("Graceful shutdown completed")
	return nil
}

// handleEnrichedMessage processes incoming enriched events
func (s *Subscriber) handleEnrichedMessage(msg *nats.Msg) {
	startTime := time.Now()
	s.logger.Debug("Received enriched event", "subject", msg.Subject, "data_length", len(msg.Data))

	// Parse the enriched event
	event, err := s.parseEvent(msg.Data)
	if err != nil {
		s.logger.Error("Failed to parse enriched event", "error", err)
		s.metrics.IncEventsInvalid()
		s.incrementInvalidEvents()
		msg.Ack() // Acknowledge to prevent redelivery
		return
	}

	// Process the event for correlations
	s.processEvent(event)

	// Update metrics
	s.metrics.IncEventsProcessed()
	s.metrics.ObserveEventProcessingDuration(time.Since(startTime).Seconds())

	// Acknowledge the message
	if err := msg.Ack(); err != nil {
		s.logger.Error("Failed to acknowledge message", "error", err)
	}
}

// handleRawMessage processes incoming raw events
func (s *Subscriber) handleRawMessage(msg *nats.Msg) {
	startTime := time.Now()
	s.logger.Debug("Received raw event", "subject", msg.Subject, "data_length", len(msg.Data))

	// Parse the raw event
	event, err := s.parseEvent(msg.Data)
	if err != nil {
		s.logger.Error("Failed to parse raw event", "error", err)
		s.metrics.IncEventsInvalid()
		s.incrementInvalidEvents()
		msg.Ack() // Acknowledge to prevent redelivery
		return
	}

	// Process the event for correlations
	s.processEvent(event)

	// Update metrics
	s.metrics.IncEventsProcessed()
	s.metrics.ObserveEventProcessingDuration(time.Since(startTime).Seconds())

	// Acknowledge the message
	if err := msg.Ack(); err != nil {
		s.logger.Error("Failed to acknowledge message", "error", err)
	}
}

// parseEvent converts event data to Event model
func (s *Subscriber) parseEvent(data []byte) (*model.Event, error) {
	var eventData map[string]interface{}
	if err := json.Unmarshal(data, &eventData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event data: %w", err)
	}

	event := &model.Event{}

	// Parse timestamp
	if ts, ok := eventData["timestamp"].(float64); ok {
		event.Timestamp = time.Unix(int64(ts)/1000, 0)
	} else if ts, ok := eventData["timestamp"].(string); ok {
		if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
			event.Timestamp = parsed
		} else {
			event.Timestamp = time.Now()
		}
	} else {
		event.Timestamp = time.Now()
	}

	// Parse host ID
	if hostID, ok := eventData["host_id"].(string); ok {
		event.HostID = hostID
	}

	// Parse event type
	if eventType, ok := eventData["event_type"].(string); ok {
		event.EventType = eventType
	} else if eventType, ok := eventData["type"].(string); ok {
		event.EventType = eventType
	}

	// Parse binary path
	if binaryPath, ok := eventData["binary_path"].(string); ok {
		event.BinaryPath = binaryPath
	} else if source, ok := eventData["source"].(string); ok {
		event.BinaryPath = source
	}

	// Parse args
	if args, ok := eventData["args"].(map[string]interface{}); ok {
		event.Args = args
	}

	// Parse context
	if context, ok := eventData["context"].(map[string]interface{}); ok {
		event.Context = context
	}

	return event, nil
}

// processEvent applies correlation rules to generate findings
func (s *Subscriber) processEvent(event *model.Event) {
	startTime := time.Now()
	s.logger.Debug("Processing event for correlations", "host_id", event.HostID, "event_type", event.EventType)

	// Use the evaluator to process the event
	findings := s.evaluator.OnEvent(event)

	// Store findings and update metrics
	for _, finding := range findings {
		if added := s.store.AddFinding(finding); added {
			s.metrics.IncFindingsGenerated()
			s.logger.Info("New finding created",
				"finding_id", finding.ID,
				"severity", finding.Severity,
				"host_id", finding.HostID,
				"rule_id", finding.RuleID)
		} else {
			s.metrics.IncFindingsDeduplicated()
		}
	}

	// Update rule evaluation metrics
	s.metrics.ObserveRuleEvaluationDuration(time.Since(startTime).Seconds())
}

// applyCorrelationRulesWithWindow applies loaded correlation rules with window buffer context
func (s *Subscriber) applyCorrelationRulesWithWindow(event *model.Event, ruleList []rules.Rule) []*model.Finding {
	var findings []*model.Finding

	// Apply each loaded rule
	for _, rule := range ruleList {
		finding := s.evaluateRuleWithWindow(event, rule)
		if finding != nil {
			findings = append(findings, finding)
		}
	}

	return findings
}

// applyCorrelationRules applies loaded correlation rules to generate findings (legacy method)
func (s *Subscriber) applyCorrelationRules(event *model.Event, ruleList []rules.Rule) []*model.Finding {
	var findings []*model.Finding

	// Apply each loaded rule
	for _, rule := range ruleList {
		// Check if rule applies to this host/event
		if s.ruleMatches(event, rule) {
			finding := s.evaluateRule(event, rule)
			if finding != nil {
				findings = append(findings, finding)
			}
		}
	}

	return findings
}

// ruleMatches checks if a rule applies to the given event
func (s *Subscriber) ruleMatches(event *model.Event, rule rules.Rule) bool {
	// Check host selectors
	selectors := rule.Spec.Selectors
	
	// Check host_ids
	if len(selectors.HostIDs) > 0 {
		matches := false
		for _, hostID := range selectors.HostIDs {
			if event.HostID == hostID {
				matches = true
				break
			}
		}
		if !matches {
			return false
		}
	}
	
	// Check exclude_host_ids
	for _, excludeID := range selectors.ExcludeHostIDs {
		if event.HostID == excludeID {
			return false
		}
	}
	
	// TODO: Implement host_globs and labels matching
	
	return true
}

// evaluateRuleWithWindow evaluates a rule against an event using window buffer context
func (s *Subscriber) evaluateRuleWithWindow(event *model.Event, rule rules.Rule) *model.Finding {
	condition := rule.Spec.Condition
	outcome := rule.Spec.Outcome

	// Get time window for this rule
	windowDuration := time.Duration(condition.WindowSeconds) * time.Second

	// Check if this event matches the rule's conditions
	if s.eventMatchesCondition(event, condition, windowDuration) {
		// Create finding with evidence from window buffer
		evidence := s.buildEvidenceFromWindow(event, rule, windowDuration)
		
		return &model.Finding{
			ID:         generateFindingID(event.HostID, rule.Metadata.ID),
			Severity:   outcome.Severity,
			Confidence: outcome.Confidence,
			Status:     "open",
			HostID:     event.HostID,
			Evidence:   evidence,
			Timestamp:  time.Now(),
			RuleID:     rule.Metadata.ID,
			TTLSeconds: rule.Spec.TTLSeconds,
		}
	}

	return nil
}

// eventMatchesCondition checks if an event matches the rule condition using window buffer
func (s *Subscriber) eventMatchesCondition(event *model.Event, condition rules.Condition, windowDuration time.Duration) bool {
	// Check "any_of" conditions
	if whenAny, exists := condition.When["any_of"]; exists {
		if patterns, ok := whenAny.([]rules.EventPattern); ok {
			for _, pattern := range patterns {
				if s.eventMatchesPattern(event, pattern, windowDuration) {
					return true
				}
			}
		}
	}

	// Check "all_of" conditions (if any)
	if whenAll, exists := condition.When["all_of"]; exists {
		if patterns, ok := whenAll.([]rules.EventPattern); ok {
			for _, pattern := range patterns {
				if !s.eventMatchesPattern(event, pattern, windowDuration) {
					return false
				}
			}
			return true
		}
	}

	return false
}

// eventMatchesPattern checks if an event matches a specific pattern using window buffer
func (s *Subscriber) eventMatchesPattern(event *model.Event, pattern rules.EventPattern, windowDuration time.Duration) bool {
	// Check event type
	if pattern.EventType != "" && pattern.EventType != event.EventType {
		return false
	}

	// Check binary path regex if specified
	if pattern.BinaryPathRegex != "" {
		matched, err := regexp.MatchString(pattern.BinaryPathRegex, event.BinaryPath)
		if err != nil || !matched {
			return false
		}
	}

	// For window-based matching, check if we have enough events of this type in the window
	if pattern.EventType != "" {
		recentEvents := s.windowBuffer.RecentByType(event.HostID, pattern.EventType, windowDuration)
		return len(recentEvents) > 0
	}

	return true
}

// buildEvidenceFromWindow creates evidence from events in the window buffer
func (s *Subscriber) buildEvidenceFromWindow(event *model.Event, rule rules.Rule, windowDuration time.Duration) []model.Evidence {
	var evidence []model.Evidence

	// Add current event as evidence
	evidence = append(evidence, model.Evidence{
		Type:        "event",
		Description: "Current triggering event",
		Data: map[string]interface{}{
			"event_type":  event.EventType,
			"binary_path": event.BinaryPath,
			"args":        event.Args,
			"context":     event.Context,
		},
		Timestamp: event.Timestamp,
	})

	// Add related events from window buffer
	recentEvents := s.windowBuffer.RecentEvents(event.HostID, windowDuration)
	
	if len(recentEvents) > 1 {
		evidence = append(evidence, model.Evidence{
			Type:        "context",
			Description: fmt.Sprintf("Related events in %d second window", rule.Spec.Condition.WindowSeconds),
			Data: map[string]interface{}{
				"event_count": len(recentEvents),
				"window_seconds": rule.Spec.Condition.WindowSeconds,
				"related_events": recentEvents,
			},
			Timestamp: time.Now(),
		})
	}

	return evidence
}

// evaluateRule evaluates a rule against an event and returns a finding if triggered (legacy method)
func (s *Subscriber) evaluateRule(event *model.Event, rule rules.Rule) *model.Finding {
	// For now, implement simple rule evaluation
	// In production, this would be much more sophisticated
	
	condition := rule.Spec.Condition
	outcome := rule.Spec.Outcome
	
	// Simple event type matching
	if whenAny, exists := condition.When["any_of"]; exists {
		if patterns, ok := whenAny.([]interface{}); ok {
			for _, pattern := range patterns {
				if patternMap, ok := pattern.(map[string]interface{}); ok {
					if eventType, exists := patternMap["event_type"]; exists {
						if eventType == event.EventType {
							// Rule triggered
							return &model.Finding{
								ID:         generateFindingID(event.HostID, rule.Metadata.ID),
								Severity:   outcome.Severity,
								Confidence: outcome.Confidence,
								Status:     "open",
								HostID:     event.HostID,
								Evidence: []model.Evidence{
									{
										Type:        "event",
										Description: "Rule triggered by event",
										Data: map[string]interface{}{
											"event_type": event.EventType,
											"binary_path": event.BinaryPath,
											"args": event.Args,
										},
										Timestamp: event.Timestamp,
									},
								},
								Timestamp:  time.Now(),
								RuleID:     rule.Metadata.ID,
								TTLSeconds: rule.Spec.TTLSeconds,
							}
						}
					}
				}
			}
		}
	}
	
	return nil
}

// extractLabelsFromEvent extracts labels from the event context
func (s *Subscriber) extractLabelsFromEvent(event *model.Event) []string {
	var labels []string
	
	// Extract labels from context
	if event.Context != nil {
		// Look for common label patterns
		if tags, ok := event.Context["tags"].(string); ok {
			// Parse comma-separated or space-separated tags
			if tags != "" {
				// Split by space (common format)
				tagParts := strings.Fields(tags)
				labels = append(labels, tagParts...)
			}
		}
		
		// Look for individual label fields
		for key, value := range event.Context {
			if valueStr, ok := value.(string); ok && valueStr != "" {
				// Add as key:value label
				labels = append(labels, key+":"+valueStr)
			}
		}
	}
	
	return labels
}

// isSuspiciousBinary checks if a binary path is suspicious
func (s *Subscriber) isSuspiciousBinary(binaryPath string) bool {
	suspiciousPaths := []string{
		"/tmp/", "/var/tmp/", "/dev/shm/",
		"/home/", "/root/",
	}
	
	for _, path := range suspiciousPaths {
		if len(binaryPath) >= len(path) && binaryPath[:len(path)] == path {
			return true
		}
	}
	
	return false
}

// isUnusualConnection checks if a network connection is unusual
func (s *Subscriber) isUnusualConnection(args map[string]interface{}) bool {
	// Check for connections to unusual ports
	if dstPort, ok := args["dst_port"].(float64); ok {
		port := int(dstPort)
		// Flag connections to unusual ports (not common services)
		unusualPorts := []int{4444, 5555, 6666, 7777, 8888, 9999}
		for _, unusualPort := range unusualPorts {
			if port == unusualPort {
				return true
			}
		}
	}
	
	return false
}

// generateFindingID creates a unique finding ID
func generateFindingID(hostID, ruleType string) string {
	return hostID + "-" + ruleType + "-" + time.Now().Format("20060102-150405")
}

// StartGC starts the garbage collection routine for the window buffer
func (s *Subscriber) StartGC(gcInterval time.Duration) {
	s.windowBuffer.StartGC(gcInterval)
}

// StopGC stops the garbage collection routine for the window buffer
func (s *Subscriber) StopGC() {
	s.windowBuffer.StopGC()
}

// gracefulShutdown performs graceful shutdown with drain
func (s *Subscriber) gracefulShutdown() error {
	s.logger.Info("Starting graceful shutdown with drain")

	// Drain enriched subscription
	if s.enrichedSub != nil {
		s.logger.Info("Draining enriched subscription")
		if err := s.enrichedSub.Drain(); err != nil {
			s.logger.Error("Failed to drain enriched subscription", "error", err)
		} else {
			s.logger.Info("Enriched subscription drained successfully")
		}
	}

	// Drain raw subscription
	if s.rawSub != nil {
		s.logger.Info("Draining raw subscription")
		if err := s.rawSub.Drain(); err != nil {
			s.logger.Error("Failed to drain raw subscription", "error", err)
		} else {
			s.logger.Info("Raw subscription drained successfully")
		}
	}

	// Stop window buffer GC
	s.windowBuffer.StopGC()

	s.logger.Info("Graceful shutdown completed")
	return nil
}

// incrementInvalidEvents increments the invalid events counter
func (s *Subscriber) incrementInvalidEvents() {
	// In a real implementation, this would use atomic operations or metrics library
	// For now, we'll just log it
	s.logger.Debug("Invalid event received", "total_invalid", s.invalidEventsTotal+1)
}

// GetMetrics returns subscriber metrics
func (s *Subscriber) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"correlator_events_invalid_total": s.invalidEventsTotal,
		"evaluator_metrics":               s.evaluator.GetMetrics(),
		"window_buffer_stats":             s.windowBuffer.GetStats(),
	}
}
