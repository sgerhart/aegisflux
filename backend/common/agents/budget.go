package agents

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"
)

// BudgetError represents a budget-related error
type BudgetError struct {
	Message string
	Type    string
}

func (e BudgetError) Error() string {
	return e.Message
}

// BudgetExceeded represents when budget limits are exceeded
var BudgetExceeded = BudgetError{
	Message: "budget exceeded",
	Type:    "budget_exceeded",
}

// RateLimitExceeded represents when rate limits are exceeded
var RateLimitExceeded = BudgetError{
	Message: "rate limit exceeded",
	Type:    "rate_limit_exceeded",
}

// TokenCost represents the cost per 1K tokens for different providers/models
type TokenCost struct {
	InputTokens  float64 `json:"input_tokens"`  // Cost per 1K input tokens
	OutputTokens float64 `json:"output_tokens"` // Cost per 1K output tokens
}

// BudgetManager manages cost budgets and rate limiting
type BudgetManager struct {
	mu                    sync.RWMutex
	maxCostPerHour        float64
	rateLimitRPM          int
	tokenCosts            map[string]TokenCost
	hourlyCosts           map[string]float64 // hour timestamp -> total cost
	requestCounts         map[string]int     // minute timestamp -> request count
	logger                *slog.Logger
	currentHour           string
	currentMinute         string
}

// NewBudgetManager creates a new budget manager
func NewBudgetManager(logger *slog.Logger) (*BudgetManager, error) {
	bm := &BudgetManager{
		logger:         logger,
		tokenCosts:     make(map[string]TokenCost),
		hourlyCosts:    make(map[string]float64),
		requestCounts:  make(map[string]int),
	}

	// Load configuration from environment
	if err := bm.loadConfig(); err != nil {
		return nil, fmt.Errorf("failed to load budget config: %w", err)
	}

	// Initialize default token costs
	bm.initializeDefaultCosts()

	// Set initial time windows
	bm.updateTimeWindows()

	// Start cleanup goroutine
	go bm.startCleanup()

	return bm, nil
}

// loadConfig loads budget configuration from environment variables
func (bm *BudgetManager) loadConfig() error {
	// Load max cost per hour
	if maxCostStr := os.Getenv("AGENT_MAX_COST_USD_PER_HOUR"); maxCostStr != "" {
		if maxCost, err := strconv.ParseFloat(maxCostStr, 64); err == nil {
			bm.maxCostPerHour = maxCost
		} else {
			return fmt.Errorf("invalid AGENT_MAX_COST_USD_PER_HOUR: %s", maxCostStr)
		}
	} else {
		bm.maxCostPerHour = 10.0 // Default $10/hour
	}

	// Load rate limit RPM
	if rpmStr := os.Getenv("AGENT_RATE_RPM"); rpmStr != "" {
		if rpm, err := strconv.Atoi(rpmStr); err == nil {
			bm.rateLimitRPM = rpm
		} else {
			return fmt.Errorf("invalid AGENT_RATE_RPM: %s", rpmStr)
		}
	} else {
		bm.rateLimitRPM = 100 // Default 100 requests per minute
	}

	bm.logger.Info("Budget configuration loaded",
		"max_cost_per_hour", bm.maxCostPerHour,
		"rate_limit_rpm", bm.rateLimitRPM)

	return nil
}

// initializeDefaultCosts sets up default token costs for common providers/models
func (bm *BudgetManager) initializeDefaultCosts() {
	// OpenAI GPT-4 costs (as of 2024)
	bm.tokenCosts["openai:gpt-4"] = TokenCost{
		InputTokens:  0.03,  // $0.03 per 1K input tokens
		OutputTokens: 0.06,  // $0.06 per 1K output tokens
	}

	bm.tokenCosts["openai:gpt-4-turbo"] = TokenCost{
		InputTokens:  0.01,  // $0.01 per 1K input tokens
		OutputTokens: 0.03,  // $0.03 per 1K output tokens
	}

	bm.tokenCosts["openai:gpt-3.5-turbo"] = TokenCost{
		InputTokens:  0.0015, // $0.0015 per 1K input tokens
		OutputTokens: 0.002,  // $0.002 per 1K output tokens
	}

	// Local models (assume free or minimal cost)
	bm.tokenCosts["local:llama2"] = TokenCost{
		InputTokens:  0.0,   // Free
		OutputTokens: 0.0,   // Free
	}

	bm.tokenCosts["local:codellama"] = TokenCost{
		InputTokens:  0.0,   // Free
		OutputTokens: 0.0,   // Free
	}

	bm.tokenCosts["local:mistral"] = TokenCost{
		InputTokens:  0.0,   // Free
		OutputTokens: 0.0,   // Free
	}

	bm.logger.Debug("Initialized default token costs", "cost_count", len(bm.tokenCosts))
}

// CheckBudget checks if a request can be made within budget and rate limits
func (bm *BudgetManager) CheckBudget(provider, model string, inputTokens, outputTokens int) error {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	// Update time windows
	bm.updateTimeWindows()

	// Check rate limit
	if err := bm.checkRateLimit(); err != nil {
		return err
	}

	// Check cost budget
	if err := bm.checkCostBudget(provider, model, inputTokens, outputTokens); err != nil {
		return err
	}

	// Record the request
	bm.recordRequest(provider, model, inputTokens, outputTokens)

	return nil
}

// checkRateLimit checks if the request rate is within limits
func (bm *BudgetManager) checkRateLimit() error {
	currentMinuteRequests := bm.requestCounts[bm.currentMinute]
	if currentMinuteRequests >= bm.rateLimitRPM {
		bm.logger.Warn("Rate limit exceeded",
			"current_requests", currentMinuteRequests,
			"limit", bm.rateLimitRPM,
			"minute", bm.currentMinute)
		return RateLimitExceeded
	}
	return nil
}

// checkCostBudget checks if the request cost is within budget
func (bm *BudgetManager) checkCostBudget(provider, model string, inputTokens, outputTokens int) error {
	// Calculate estimated cost
	cost := bm.estimateCost(provider, model, inputTokens, outputTokens)
	if cost == 0 {
		return nil // Free request
	}

	// Check if adding this cost would exceed the hourly budget
	currentHourCost := bm.hourlyCosts[bm.currentHour]
	if currentHourCost+cost > bm.maxCostPerHour {
		bm.logger.Warn("Budget exceeded",
			"current_cost", currentHourCost,
			"request_cost", cost,
			"max_cost", bm.maxCostPerHour,
			"hour", bm.currentHour)
		return BudgetExceeded
	}

	return nil
}

// estimateCost estimates the cost of a request
func (bm *BudgetManager) estimateCost(provider, model string, inputTokens, outputTokens int) float64 {
	key := fmt.Sprintf("%s:%s", provider, model)
	cost, exists := bm.tokenCosts[key]
	if !exists {
		// Use a default cost if not found
		bm.logger.Debug("Unknown provider/model, using default cost", "key", key)
		cost = TokenCost{
			InputTokens:  0.01,  // Conservative default
			OutputTokens: 0.02,  // Conservative default
		}
	}

	inputCost := (float64(inputTokens) / 1000.0) * cost.InputTokens
	outputCost := (float64(outputTokens) / 1000.0) * cost.OutputTokens

	return inputCost + outputCost
}

// recordRequest records a request for tracking
func (bm *BudgetManager) recordRequest(provider, model string, inputTokens, outputTokens int) {
	// Update request count
	bm.requestCounts[bm.currentMinute]++

	// Update cost tracking
	cost := bm.estimateCost(provider, model, inputTokens, outputTokens)
	if cost > 0 {
		bm.hourlyCosts[bm.currentHour] += cost
	}

	bm.logger.Debug("Request recorded",
		"provider", provider,
		"model", model,
		"input_tokens", inputTokens,
		"output_tokens", outputTokens,
		"cost", cost,
		"minute_requests", bm.requestCounts[bm.currentMinute],
		"hour_cost", bm.hourlyCosts[bm.currentHour])
}

// updateTimeWindows updates the current hour and minute windows
func (bm *BudgetManager) updateTimeWindows() {
	now := time.Now()
	bm.currentHour = now.Format("2006-01-02-15")
	bm.currentMinute = now.Format("2006-01-02-15-04")
}

// startCleanup starts a goroutine to clean up old tracking data
func (bm *BudgetManager) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		bm.cleanup()
	}
}

// cleanup removes old tracking data to prevent memory leaks
func (bm *BudgetManager) cleanup() {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	now := time.Now()
	cutoffHour := now.Add(-24 * time.Hour).Format("2006-01-02-15")
	cutoffMinute := now.Add(-1 * time.Hour).Format("2006-01-02-15-04")

	// Clean up old hourly costs
	for hour := range bm.hourlyCosts {
		if hour < cutoffHour {
			delete(bm.hourlyCosts, hour)
		}
	}

	// Clean up old request counts
	for minute := range bm.requestCounts {
		if minute < cutoffMinute {
			delete(bm.requestCounts, minute)
		}
	}

	bm.logger.Debug("Budget cleanup completed",
		"hourly_costs", len(bm.hourlyCosts),
		"request_counts", len(bm.requestCounts))
}

// GetStats returns current budget statistics
func (bm *BudgetManager) GetStats() map[string]interface{} {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	bm.updateTimeWindows()

	return map[string]interface{}{
		"max_cost_per_hour":    bm.maxCostPerHour,
		"rate_limit_rpm":       bm.rateLimitRPM,
		"current_hour_cost":    bm.hourlyCosts[bm.currentHour],
		"current_minute_requests": bm.requestCounts[bm.currentMinute],
		"budget_utilization":   bm.hourlyCosts[bm.currentHour] / bm.maxCostPerHour,
		"rate_utilization":     float64(bm.requestCounts[bm.currentMinute]) / float64(bm.rateLimitRPM),
	}
}

// SetTokenCost sets the cost for a specific provider/model combination
func (bm *BudgetManager) SetTokenCost(provider, model string, cost TokenCost) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	key := fmt.Sprintf("%s:%s", provider, model)
	bm.tokenCosts[key] = cost

	bm.logger.Info("Updated token cost", "key", key, "input_cost", cost.InputTokens, "output_cost", cost.OutputTokens)
}
