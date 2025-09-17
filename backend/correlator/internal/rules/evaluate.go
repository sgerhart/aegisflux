package rules

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aegisflux/correlator/internal/metrics"
	"github.com/aegisflux/correlator/internal/model"
)

// Evaluator handles rule evaluation with caching and template rendering
type Evaluator struct {
	windowBuffer    *WindowBuffer
	matcher         *Matcher
	ruleLoader      *Loader
	overrideManager *OverrideManager
	hostLabelsCache *HostLabelsCache
	dedupeCache     *DedupeCache
	logger          *slog.Logger
	metrics         *EvaluationMetrics
	prometheusMetrics *metrics.Metrics
}

// HostLabelsCache manages host labels with TTL
type HostLabelsCache struct {
	mu     sync.RWMutex
	cache  map[string]*CachedLabels
	ttlSec int
}

// CachedLabels holds labels with timestamp
type CachedLabels struct {
	Labels    []string
	Timestamp time.Time
}

// DedupeCache manages deduplication with cooldown
type DedupeCache struct {
	mu    sync.RWMutex
	cache map[string]time.Time
}

// EvaluationMetrics tracks evaluation statistics
type EvaluationMetrics struct {
	mu                   sync.RWMutex
	eventsProcessed      int64
	rulesEvaluated       int64
	findingsGenerated    int64
	findingsDeduplicated int64
	cacheHits            int64
	cacheMisses          int64
}

// NewEvaluator creates a new rule evaluator
func NewEvaluator(windowBuffer *WindowBuffer, matcher *Matcher, ruleLoader *Loader, overrideManager *OverrideManager, labelTTLSec int, prometheusMetrics *metrics.Metrics, logger *slog.Logger) *Evaluator {
	return &Evaluator{
		windowBuffer:      windowBuffer,
		matcher:           matcher,
		ruleLoader:        ruleLoader,
		overrideManager:   overrideManager,
		hostLabelsCache:   NewHostLabelsCache(labelTTLSec),
		dedupeCache:       NewDedupeCache(),
		logger:            logger,
		metrics:           &EvaluationMetrics{},
		prometheusMetrics: prometheusMetrics,
	}
}

// NewHostLabelsCache creates a new host labels cache
func NewHostLabelsCache(ttlSec int) *HostLabelsCache {
	return &HostLabelsCache{
		cache:  make(map[string]*CachedLabels),
		ttlSec: ttlSec,
	}
}

// NewDedupeCache creates a new deduplication cache
func NewDedupeCache() *DedupeCache {
	return &DedupeCache{
		cache: make(map[string]time.Time),
	}
}

// OnEvent processes an event and evaluates rules
func (e *Evaluator) OnEvent(event *model.Event) []*model.Finding {
	if event == nil {
		return nil
	}

	e.metrics.incrementEventsProcessed()

	// Update window buffer
	e.windowBuffer.Add(event)

	// Update labels cache
	e.updateHostLabelsCache(event)

	// Get effective rules for this host
	labels := e.getHostLabels(event.HostID)
	snapshot := e.ruleLoader.GetSnapshot()
	effectiveRules := e.matcher.EffectiveRulesFor(event.HostID, labels, snapshot.Rules)

	e.logger.Debug("Processing event for rule evaluation",
		"host_id", event.HostID,
		"event_type", event.EventType,
		"effective_rules", len(effectiveRules))

	var findings []*model.Finding

	// Evaluate each effective rule
	for _, rule := range effectiveRules {
		e.metrics.incrementRulesEvaluated()
		if e.prometheusMetrics != nil {
			e.prometheusMetrics.IncRulesEvaluated()
		}
		
		// Apply overrides to the rule
		modifiedRule := e.overrideManager.ApplyOverrides(&rule)
		
		finding := e.evaluateRule(event, *modifiedRule)
		if finding != nil {
			// Check deduplication
			if e.shouldDedupe(finding, rule) {
				e.metrics.incrementFindingsDeduplicated()
				e.logger.Debug("Finding deduplicated",
					"finding_id", finding.ID,
					"rule_id", rule.Metadata.ID)
				continue
			}

			// Record dedupe key
			e.recordDedupeKey(finding, rule)

			findings = append(findings, finding)
			e.metrics.incrementFindingsGenerated()

			e.logger.Info("New finding generated",
				"finding_id", finding.ID,
				"rule_id", rule.Metadata.ID,
				"severity", finding.Severity,
				"host_id", finding.HostID)
		}
	}

	return findings
}

// evaluateRule evaluates a single rule against an event
func (e *Evaluator) evaluateRule(event *model.Event, rule Rule) *model.Finding {
	condition := rule.Spec.Condition
	outcome := rule.Spec.Outcome

	// Check WHEN conditions
	if !e.evaluateWhenConditions(event, condition) {
		return nil
	}

	// Check REQUIRES_PRIOR conditions if specified
	if !e.evaluateRequiresPriorConditions(event, condition) {
		return nil
	}

	// Create finding with rendered evidence
	evidence := e.renderEvidence(event, rule, condition)
	
	return &model.Finding{
		ID:         e.generateFindingID(event.HostID, rule.Metadata.ID, event.Timestamp),
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

// evaluateWhenConditions checks WHEN conditions
func (e *Evaluator) evaluateWhenConditions(event *model.Event, condition Condition) bool {
	// Check any_of conditions
	if whenAny, exists := condition.When["any_of"]; exists {
		if patterns, ok := whenAny.([]EventPattern); ok {
			for _, pattern := range patterns {
				if e.eventMatchesPattern(event, pattern, time.Duration(condition.WindowSeconds)*time.Second) {
					return true
				}
			}
		}
	}

	// Check all_of conditions
	if whenAll, exists := condition.When["all_of"]; exists {
		if patterns, ok := whenAll.([]EventPattern); ok {
			for _, pattern := range patterns {
				if !e.eventMatchesPattern(event, pattern, time.Duration(condition.WindowSeconds)*time.Second) {
					return false
				}
			}
			return true
		}
	}

	return false
}

// evaluateRequiresPriorConditions checks REQUIRES_PRIOR conditions
func (e *Evaluator) evaluateRequiresPriorConditions(event *model.Event, condition Condition) bool {
	if condition.RequiresPrior == nil {
		return true // No prior requirements
	}

	windowDuration := time.Duration(condition.WindowSeconds) * time.Second

	// Check any_of prior conditions
	if priorAny, exists := condition.RequiresPrior["any_of"]; exists {
		if patterns, ok := priorAny.([]EventPattern); ok {
			for _, pattern := range patterns {
				if e.hasPriorEventMatchingPattern(event, pattern, windowDuration) {
					return true
				}
			}
		}
	}

	// Check all_of prior conditions
	if priorAll, exists := condition.RequiresPrior["all_of"]; exists {
		if patterns, ok := priorAll.([]EventPattern); ok {
			for _, pattern := range patterns {
				if !e.hasPriorEventMatchingPattern(event, pattern, windowDuration) {
					return false
				}
			}
			return true
		}
	}

	return true // No prior requirements or all met
}

// eventMatchesPattern checks if an event matches a pattern
func (e *Evaluator) eventMatchesPattern(event *model.Event, pattern EventPattern, windowDuration time.Duration) bool {
	// Check event type
	if pattern.EventType != "" && pattern.EventType != event.EventType {
		return false
	}

	// Check binary path regex
	if pattern.BinaryPathRegex != "" {
		matched, err := regexp.MatchString(pattern.BinaryPathRegex, event.BinaryPath)
		if err != nil || !matched {
			return false
		}
	}

	// Check args patterns
	if pattern.Args != nil {
		if !e.matchArgsPattern(event.Args, pattern.Args) {
			return false
		}
	}

	// Check context patterns
	if pattern.Context != nil {
		if !e.matchContextPattern(event.Context, pattern.Context) {
			return false
		}
	}

	return true
}

// hasPriorEventMatchingPattern checks if there's a prior event matching the pattern
func (e *Evaluator) hasPriorEventMatchingPattern(event *model.Event, pattern EventPattern, windowDuration time.Duration) bool {
	// Get recent events from window buffer
	recentEvents := e.windowBuffer.RecentEvents(event.HostID, windowDuration)
	
	for _, priorEvent := range recentEvents {
		// Skip the current event (compare by timestamp and event type as proxy for ID)
		if priorEvent.Timestamp.Equal(event.Timestamp) && priorEvent.EventType == event.EventType {
			continue
		}
		
		if e.eventMatchesPattern(priorEvent, pattern, windowDuration) {
			return true
		}
	}
	
	return false
}

// matchArgsPattern checks if event args match the pattern
func (e *Evaluator) matchArgsPattern(eventArgs map[string]interface{}, patternArgs map[string]string) bool {
	for key, expectedValue := range patternArgs {
		actualValue, exists := eventArgs[key]
		if !exists {
			return false
		}
		// Convert actual value to string for comparison
		actualStr := fmt.Sprintf("%v", actualValue)
		if actualStr != expectedValue {
			return false
		}
	}
	return true
}

// matchContextPattern checks if event context matches the pattern
func (e *Evaluator) matchContextPattern(eventContext map[string]interface{}, patternContext map[string]string) bool {
	for key, expectedValue := range patternContext {
		actualValue, exists := eventContext[key]
		if !exists {
			return false
		}
		// Convert actual value to string for comparison
		actualStr := fmt.Sprintf("%v", actualValue)
		if actualStr != expectedValue {
			return false
		}
	}
	return true
}

// renderEvidence renders evidence templates with field substitution
func (e *Evaluator) renderEvidence(event *model.Event, rule Rule, condition Condition) []model.Evidence {
	var evidence []model.Evidence

	// Add current event evidence
	evidence = append(evidence, model.Evidence{
		Type:        "event",
		Description: e.renderTemplate("Rule triggered by {event_type} event", event, rule),
		Data: map[string]interface{}{
			"event_type":  event.EventType,
			"binary_path": event.BinaryPath,
			"args":        event.Args,
			"context":     event.Context,
			"timestamp":   event.Timestamp,
		},
		Timestamp: event.Timestamp,
	})

	// Render evidence templates from rule
	for _, evidenceTemplate := range rule.Spec.Outcome.Evidence {
		renderedDescription := e.renderTemplate(evidenceTemplate, event, rule)
		evidence = append(evidence, model.Evidence{
			Type:        "rule",
			Description: renderedDescription,
			Data: map[string]interface{}{
				"template": evidenceTemplate,
				"rule_id":  rule.Metadata.ID,
			},
			Timestamp: time.Now(),
		})
	}

	// Add window context evidence
	windowDuration := time.Duration(condition.WindowSeconds) * time.Second
	recentEvents := e.windowBuffer.RecentEvents(event.HostID, windowDuration)
	
	if len(recentEvents) > 1 {
		evidence = append(evidence, model.Evidence{
			Type:        "context",
			Description: fmt.Sprintf("Found %d related events in %d second window", len(recentEvents), condition.WindowSeconds),
			Data: map[string]interface{}{
				"event_count":    len(recentEvents),
				"window_seconds": condition.WindowSeconds,
				"related_events": recentEvents,
			},
			Timestamp: time.Now(),
		})
	}

	return evidence
}

// renderTemplate renders a template with field substitution
func (e *Evaluator) renderTemplate(template string, event *model.Event, rule Rule) string {
	result := template

	// Replace basic event fields
	result = strings.ReplaceAll(result, "{event_type}", event.EventType)
	result = strings.ReplaceAll(result, "{binary_path}", event.BinaryPath)
	result = strings.ReplaceAll(result, "{host_id}", event.HostID)
	result = strings.ReplaceAll(result, "{timestamp}", event.Timestamp.Format(time.RFC3339))

	// Replace args fields
	if event.Args != nil {
		for key, value := range event.Args {
			placeholder := fmt.Sprintf("{args.%s}", key)
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		}
	}

	// Replace context fields
	if event.Context != nil {
		for key, value := range event.Context {
			placeholder := fmt.Sprintf("{context.%s}", key)
			result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
		}
	}

	// Replace rule fields
	result = strings.ReplaceAll(result, "{rule.id}", rule.Metadata.ID)
	result = strings.ReplaceAll(result, "{rule.name}", rule.Metadata.Name)
	result = strings.ReplaceAll(result, "{rule.version}", rule.Metadata.Version)

	return result
}

// updateHostLabelsCache updates the host labels cache
func (e *Evaluator) updateHostLabelsCache(event *model.Event) {
	if event.Context == nil {
		return
	}

	// Extract labels from context
	labels := e.extractLabelsFromContext(event.Context)
	
	if len(labels) > 0 {
		e.hostLabelsCache.set(event.HostID, labels)
	}
}

// extractLabelsFromContext extracts labels from event context
func (e *Evaluator) extractLabelsFromContext(context map[string]interface{}) []string {
	var labels []string

	// Look for labels field
	if labelsField, exists := context["labels"]; exists {
		if labelsStr, ok := labelsField.(string); ok && labelsStr != "" {
			// Parse comma-separated or space-separated labels
			parts := strings.Fields(strings.ReplaceAll(labelsStr, ",", " "))
			labels = append(labels, parts...)
		}
	}

	// Look for tags field
	if tagsField, exists := context["tags"]; exists {
		if tagsStr, ok := tagsField.(string); ok && tagsStr != "" {
			parts := strings.Fields(strings.ReplaceAll(tagsStr, ",", " "))
			labels = append(labels, parts...)
		}
	}

	// Add key-value pairs from context as labels (excluding labels and tags fields)
	for key, value := range context {
		if key == "labels" || key == "tags" {
			continue // Skip these as they're handled above
		}
		if valueStr, ok := value.(string); ok && valueStr != "" {
			labels = append(labels, fmt.Sprintf("%s:%s", key, valueStr))
		}
	}

	return labels
}

// getHostLabels gets labels for a host from cache
func (e *Evaluator) getHostLabels(hostID string) []string {
	labels, hit := e.hostLabelsCache.get(hostID)
	if hit {
		e.metrics.incrementCacheHits()
	} else {
		e.metrics.incrementCacheMisses()
	}
	return labels
}

// shouldDedupe checks if a finding should be deduplicated
func (e *Evaluator) shouldDedupe(finding *model.Finding, rule Rule) bool {
	if rule.Spec.Dedupe.KeyTemplate == "" {
		return false // No deduplication configured
	}

	// Render the dedupe key template
	key := e.renderDedupeKey(finding, rule)
	
	// Check if key exists and is within cooldown
	return e.dedupeCache.isWithinCooldown(key, time.Duration(rule.Spec.Dedupe.CooldownSeconds)*time.Second)
}

// renderDedupeKey renders the deduplication key template
func (e *Evaluator) renderDedupeKey(finding *model.Finding, rule Rule) string {
	// Create a dummy event for template rendering
	event := &model.Event{
		HostID: finding.HostID,
		// Add other fields as needed for template rendering
	}
	
	return e.renderTemplate(rule.Spec.Dedupe.KeyTemplate, event, rule)
}

// recordDedupeKey records a deduplication key
func (e *Evaluator) recordDedupeKey(finding *model.Finding, rule Rule) {
	if rule.Spec.Dedupe.KeyTemplate == "" {
		return
	}

	key := e.renderDedupeKey(finding, rule)
	e.dedupeCache.set(key, time.Now())
}

// generateFindingID generates a unique finding ID
func (e *Evaluator) generateFindingID(hostID, ruleID string, eventTimestamp time.Time) string {
	return fmt.Sprintf("%s-%s-%d", hostID, ruleID, eventTimestamp.Unix())
}

// GetMetrics returns evaluation metrics
func (e *Evaluator) GetMetrics() map[string]interface{} {
	return e.metrics.getMetrics()
}

// Cache methods

// get retrieves labels for a host
func (c *HostLabelsCache) get(hostID string) ([]string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cached, exists := c.cache[hostID]
	if !exists {
		return nil, false
	}

	// Check TTL
	if time.Since(cached.Timestamp) > time.Duration(c.ttlSec)*time.Second {
		delete(c.cache, hostID)
		return nil, false
	}

	return cached.Labels, true
}

// set stores labels for a host
func (c *HostLabelsCache) set(hostID string, labels []string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[hostID] = &CachedLabels{
		Labels:    labels,
		Timestamp: time.Now(),
	}
}

// clear removes all cached labels
func (c *HostLabelsCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]*CachedLabels)
}

// isWithinCooldown checks if a key is within cooldown period
func (c *DedupeCache) isWithinCooldown(key string, cooldown time.Duration) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	timestamp, exists := c.cache[key]
	if !exists {
		return false
	}

	return time.Since(timestamp) < cooldown
}

// set records a deduplication key
func (c *DedupeCache) set(key string, timestamp time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[key] = timestamp
}

// clear removes all deduplication keys
func (c *DedupeCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache = make(map[string]time.Time)
}

// Metrics methods

func (m *EvaluationMetrics) incrementEventsProcessed() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.eventsProcessed++
}

func (m *EvaluationMetrics) incrementRulesEvaluated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rulesEvaluated++
}

func (m *EvaluationMetrics) incrementFindingsGenerated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.findingsGenerated++
}

func (m *EvaluationMetrics) incrementFindingsDeduplicated() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.findingsDeduplicated++
}

func (m *EvaluationMetrics) incrementCacheHits() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheHits++
}

func (m *EvaluationMetrics) incrementCacheMisses() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheMisses++
}

func (m *EvaluationMetrics) getMetrics() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"events_processed":       m.eventsProcessed,
		"rules_evaluated":        m.rulesEvaluated,
		"findings_generated":     m.findingsGenerated,
		"findings_deduplicated":  m.findingsDeduplicated,
		"cache_hits":             m.cacheHits,
		"cache_misses":           m.cacheMisses,
		"cache_hit_ratio":        float64(m.cacheHits) / float64(m.cacheHits+m.cacheMisses+1),
	}
}
