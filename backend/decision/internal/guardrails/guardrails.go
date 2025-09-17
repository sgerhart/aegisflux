package guardrails

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"aegisflux/backend/decision/internal/config"
	"aegisflux/backend/decision/internal/model"
)

// Guardrails handles strategy decision logic with safety rules
type Guardrails struct {
	logger *slog.Logger
	config *config.ConfigSnapshot
}

// NewGuardrails creates a new guardrails instance
func NewGuardrails(logger *slog.Logger) *Guardrails {
	return &Guardrails{
		logger: logger,
		config: nil, // Will be set via UpdateConfig
	}
}

// UpdateConfig updates the guardrails configuration
func (g *Guardrails) UpdateConfig(config *config.ConfigSnapshot) {
	g.config = config
	g.logger.Info("Guardrails configuration updated", 
		"decision_mode", config.DecisionMode,
		"max_canary_hosts", config.MaxCanaryHosts,
		"default_ttl_seconds", config.DefaultTTLSeconds,
		"never_block_labels", config.NeverBlockLabels,
		"last_updated", config.LastUpdated)
}

// StrategyDecision represents the result of strategy decision logic
type StrategyDecision struct {
	// Final strategy after applying all rules
	Strategy model.StrategyMode `json:"strategy"`
	// Canary size (0 if not applicable)
	CanarySize int `json:"canary_size"`
	// TTL in seconds
	TTLSeconds int `json:"ttl_seconds"`
	// Reasons for the decision
	Reasons []string `json:"reasons"`
	// Applied rules
	AppliedRules []string `json:"applied_rules"`
}

// DecideStrategy determines the appropriate strategy based on guardrails rules
func (g *Guardrails) DecideStrategy(desired string, numTargets int, hostLabels []string) (*StrategyDecision, error) {
	g.logger.Info("Deciding strategy with guardrails", 
		"desired", desired, 
		"num_targets", numTargets, 
		"host_labels", hostLabels)

	// Start with the desired strategy
	strategy := g.parseStrategy(desired)
	reasons := []string{}
	appliedRules := []string{}

	// Rule 1: Maintenance window active -> downgrade enforce→canary→suggest→observe
	if g.isMaintenanceWindowActive() {
		strategy = g.downgradeForMaintenance(strategy)
		reasons = append(reasons, "Maintenance window is active - strategy downgraded for safety")
		appliedRules = append(appliedRules, "maintenance_window")
	}

	// Rule 2: Any target has NEVER_BLOCK_LABELS -> cap at canary (or suggest if canary_size==0)
	neverBlockLabels := g.getNeverBlockLabels()
	if g.hasNeverBlockLabels(hostLabels, neverBlockLabels) {
		originalStrategy := strategy
		strategy = g.capForNeverBlock(strategy)
		reasons = append(reasons, fmt.Sprintf("Target has NEVER_BLOCK_LABELS (%v) - strategy capped from %s to %s", 
			neverBlockLabels, originalStrategy, strategy))
		appliedRules = append(appliedRules, "never_block_labels")
	}

	// Rule 3: Calculate canary_size
	canarySize := g.calculateCanarySize(strategy, numTargets)
	if canarySize == 0 && (strategy == model.StrategyModeEnforce || strategy == model.StrategyModeCanary) {
		strategy = model.StrategyModeSuggest
		reasons = append(reasons, "Canary size is 0 in enforce/canary mode - downgraded to suggest")
		appliedRules = append(appliedRules, "canary_size_zero")
		canarySize = 0 // Suggest mode doesn't use canary size
	}

	// Rule 4: Set TTL from environment
	ttlSeconds := g.getDefaultTTL()

	decision := &StrategyDecision{
		Strategy:     strategy,
		CanarySize:   canarySize,
		TTLSeconds:   ttlSeconds,
		Reasons:      reasons,
		AppliedRules: appliedRules,
	}

	g.logger.Info("Strategy decision completed", 
		"final_strategy", strategy,
		"canary_size", canarySize,
		"ttl_seconds", ttlSeconds,
		"applied_rules", appliedRules)

	return decision, nil
}

// parseStrategy parses a string strategy to StrategyMode
func (g *Guardrails) parseStrategy(strategy string) model.StrategyMode {
	switch strings.ToLower(strategy) {
	case "observe":
		return model.StrategyModeObserve
	case "suggest":
		return model.StrategyModeSuggest
	case "canary":
		return model.StrategyModeCanary
	case "enforce":
		return model.StrategyModeEnforce
	case "conservative":
		return model.StrategyModeConservative
	case "balanced":
		return model.StrategyModeBalanced
	case "aggressive":
		return model.StrategyModeAggressive
	default:
		g.logger.Warn("Unknown strategy, defaulting to suggest", "strategy", strategy)
		return model.StrategyModeSuggest
	}
}

// isMaintenanceWindowActive checks if maintenance window is currently active
func (g *Guardrails) isMaintenanceWindowActive() bool {
	// Check environment variable for maintenance window configuration
	maintenanceWindow := os.Getenv("DECISION_MAINTENANCE_WINDOW")
	if maintenanceWindow == "" {
		return false
	}

	// Parse maintenance window (format: "start_hour,end_hour" in 24h format)
	parts := strings.Split(maintenanceWindow, ",")
	if len(parts) != 2 {
		g.logger.Warn("Invalid maintenance window format", "window", maintenanceWindow)
		return false
	}

	startHour, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		g.logger.Warn("Invalid start hour in maintenance window", "hour", parts[0])
		return false
	}

	endHour, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		g.logger.Warn("Invalid end hour in maintenance window", "hour", parts[1])
		return false
	}

	// Get current hour
	currentHour := time.Now().Hour()

	// Check if current hour is within maintenance window
	if startHour <= endHour {
		// Same day maintenance window
		return currentHour >= startHour && currentHour < endHour
	} else {
		// Overnight maintenance window
		return currentHour >= startHour || currentHour < endHour
	}
}

// downgradeForMaintenance downgrades strategy for maintenance window
func (g *Guardrails) downgradeForMaintenance(strategy model.StrategyMode) model.StrategyMode {
	switch strategy {
	case model.StrategyModeEnforce:
		return model.StrategyModeCanary
	case model.StrategyModeCanary:
		return model.StrategyModeSuggest
	case model.StrategyModeSuggest:
		return model.StrategyModeObserve
	case model.StrategyModeAggressive:
		return model.StrategyModeBalanced
	case model.StrategyModeBalanced:
		return model.StrategyModeConservative
	case model.StrategyModeConservative:
		return model.StrategyModeSuggest
	default:
		return model.StrategyModeObserve
	}
}

// getNeverBlockLabels gets the list of NEVER_BLOCK_LABELS from configuration or environment
func (g *Guardrails) getNeverBlockLabels() []string {
	// Use configuration if available
	if g.config != nil {
		return g.config.NeverBlockLabels
	}
	
	// Fallback to environment variable
	neverBlockStr := os.Getenv("DECISION_NEVER_BLOCK_LABELS")
	if neverBlockStr == "" {
		// Default never block labels
		return []string{"critical", "production", "database", "load-balancer", "monitoring"}
	}

	// Parse comma-separated labels
	labels := strings.Split(neverBlockStr, ",")
	for i, label := range labels {
		labels[i] = strings.TrimSpace(strings.ToLower(label))
	}
	return labels
}

// hasNeverBlockLabels checks if any host labels match NEVER_BLOCK_LABELS
func (g *Guardrails) hasNeverBlockLabels(hostLabels []string, neverBlockLabels []string) bool {
	for _, hostLabel := range hostLabels {
		hostLabelLower := strings.ToLower(strings.TrimSpace(hostLabel))
		for _, neverBlock := range neverBlockLabels {
			if strings.Contains(hostLabelLower, neverBlock) || strings.Contains(neverBlock, hostLabelLower) {
				return true
			}
		}
	}
	return false
}

// capForNeverBlock caps strategy for never block labels
func (g *Guardrails) capForNeverBlock(strategy model.StrategyMode) model.StrategyMode {
	switch strategy {
	case model.StrategyModeEnforce, model.StrategyModeAggressive:
		return model.StrategyModeCanary
	case model.StrategyModeCanary:
		// Already at canary, no change needed
		return model.StrategyModeCanary
	default:
		return strategy
	}
}

// calculateCanarySize calculates the canary size based on strategy and number of targets
func (g *Guardrails) calculateCanarySize(strategy model.StrategyMode, numTargets int) int {
	// Only calculate canary size for canary strategy
	if strategy != model.StrategyModeCanary {
		return 0
	}

	maxCanary := g.getMaxCanaryHosts()
	if maxCanary == 0 {
		return 0
	}
	
	canarySize := numTargets
	if canarySize > maxCanary {
		canarySize = maxCanary
	}

	return canarySize
}

// getMaxCanaryHosts gets the maximum canary hosts from configuration or environment
func (g *Guardrails) getMaxCanaryHosts() int {
	// Use configuration if available
	if g.config != nil {
		return g.config.MaxCanaryHosts
	}
	
	// Fallback to environment variable
	if maxStr := os.Getenv("DECISION_MAX_CANARY_HOSTS"); maxStr != "" {
		if max, err := strconv.Atoi(maxStr); err == nil {
			return max
		}
	}
	// Default maximum canary hosts
	return 5
}

// getDefaultTTL gets the default TTL from configuration or environment
func (g *Guardrails) getDefaultTTL() int {
	// Use configuration if available
	if g.config != nil {
		return g.config.DefaultTTLSeconds
	}
	
	// Fallback to environment variable
	if ttlStr := os.Getenv("DECISION_DEFAULT_TTL_SECONDS"); ttlStr != "" {
		if ttl, err := strconv.Atoi(ttlStr); err == nil {
			return ttl
		}
	}
	// Default TTL: 1 hour
	return 3600
}

// ValidateStrategy validates if a strategy is safe for the given context
func (g *Guardrails) ValidateStrategy(strategy model.StrategyMode, numTargets int, hostLabels []string) (bool, []string) {
	var warnings []string

	// Check if strategy is too aggressive for the context
	if strategy == model.StrategyModeEnforce && numTargets > 10 {
		warnings = append(warnings, "Enforce strategy with many targets (>10) may cause widespread impact")
	}

	if strategy == model.StrategyModeAggressive && numTargets > 5 {
		warnings = append(warnings, "Aggressive strategy with many targets (>5) may cause significant impact")
	}

	// Check never block labels
	neverBlockLabels := g.getNeverBlockLabels()
	if g.hasNeverBlockLabels(hostLabels, neverBlockLabels) {
		if strategy == model.StrategyModeEnforce || strategy == model.StrategyModeAggressive {
			warnings = append(warnings, "Strategy may conflict with NEVER_BLOCK_LABELS")
		}
	}

	// Check maintenance window
	if g.isMaintenanceWindowActive() {
		if strategy == model.StrategyModeEnforce || strategy == model.StrategyModeAggressive {
			warnings = append(warnings, "Strategy is aggressive during maintenance window")
		}
	}

	isValid := len(warnings) == 0
	return isValid, warnings
}

// GetStrategyRecommendation provides a strategy recommendation based on context
func (g *Guardrails) GetStrategyRecommendation(numTargets int, hostLabels []string, findingSeverity string) model.StrategyMode {
	// Start with a conservative recommendation
	recommendation := model.StrategyModeSuggest

	// Adjust based on finding severity
	switch strings.ToLower(findingSeverity) {
	case "critical":
		recommendation = model.StrategyModeCanary
		if numTargets <= 2 {
			recommendation = model.StrategyModeEnforce
		}
	case "high":
		recommendation = model.StrategyModeCanary
	case "medium":
		recommendation = model.StrategyModeSuggest
	case "low":
		recommendation = model.StrategyModeObserve
	}

	// Apply guardrails to the recommendation
	decision, err := g.DecideStrategy(string(recommendation), numTargets, hostLabels)
	if err != nil {
		g.logger.Warn("Failed to apply guardrails to recommendation", "error", err)
		return model.StrategyModeObserve
	}

	return decision.Strategy
}

// GetGuardrailsStatus returns the current status of all guardrails
func (g *Guardrails) GetGuardrailsStatus() map[string]interface{} {
	status := map[string]interface{}{
		"maintenance_window_active": g.isMaintenanceWindowActive(),
		"never_block_labels":        g.getNeverBlockLabels(),
		"max_canary_hosts":          g.getMaxCanaryHosts(),
		"default_ttl_seconds":       g.getDefaultTTL(),
	}

	// Add maintenance window details if active
	if g.isMaintenanceWindowActive() {
		maintenanceWindow := os.Getenv("DECISION_MAINTENANCE_WINDOW")
		status["maintenance_window"] = maintenanceWindow
	}

	return status
}
