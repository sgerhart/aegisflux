package agents

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// SegmenterAgent handles target segmentation and related target inference
type SegmenterAgent struct {
	runtime *Runtime
	logger  *slog.Logger
}

// NewSegmenterAgent creates a new segmenter agent
func NewSegmenterAgent(runtime *Runtime, logger *slog.Logger) *SegmenterAgent {
	return &SegmenterAgent{
		runtime: runtime,
		logger:  logger,
	}
}

// SegmentationResult represents the result of target segmentation
type SegmentationResult struct {
	// Primary target (the original target)
	PrimaryTarget string `json:"primary_target"`
	// Related targets discovered from graph analysis
	RelatedTargets []RelatedTarget `json:"related_targets"`
	// Total number of targets (primary + related)
	TotalTargets int `json:"total_targets"`
	// Canary size limit applied
	CanarySize int `json:"canary_size"`
	// Segmentation reasoning
	Reasoning string `json:"reasoning"`
	// Creation timestamp
	CreatedAt time.Time `json:"created_at"`
}

// RelatedTarget represents a related target with risk context
type RelatedTarget struct {
	// Target identifier (host ID, service name, etc.)
	TargetID string `json:"target_id"`
	// Target type (host, service, pod, etc.)
	TargetType string `json:"target_type"`
	// Relationship type (peer, dependency, upstream, downstream)
	Relationship string `json:"relationship"`
	// Risk score from risk.context
	RiskScore float64 `json:"risk_score"`
	// Risk level (low, medium, high, critical)
	RiskLevel string `json:"risk_level"`
	// Connection details
	ConnectionDetails map[string]any `json:"connection_details,omitempty"`
	// Confidence in relationship
	Confidence float64 `json:"confidence"`
}

// InferRelatedTargets infers related targets from graph analysis
func (s *SegmenterAgent) InferRelatedTargets(ctx context.Context, primaryTarget string, canarySize int) (*SegmentationResult, error) {
	s.logger.Info("Inferring related targets", "primary_target", primaryTarget, "canary_size", canarySize)

	// Get default canary size if not provided
	if canarySize <= 0 {
		canarySize = s.getDefaultCanarySize()
	}

	// For now, use fallback targets to avoid agent runtime issues
	// TODO: Implement proper graph analysis when agent runtime is stable
	relatedTargets := s.generateFallbackTargets(primaryTarget)
	reasoning := "Using fallback targets for testing"

	// Apply canary size limit to avoid overreach
	if len(relatedTargets) > canarySize {
		relatedTargets = s.applyCanaryLimit(relatedTargets, canarySize)
		reasoning += fmt.Sprintf(" (limited to %d targets)", canarySize)
	}

	// Assess risk context for each related target
	for i := range relatedTargets {
		riskScore, riskLevel := s.assessTargetRisk(ctx, relatedTargets[i].TargetID)
		relatedTargets[i].RiskScore = riskScore
		relatedTargets[i].RiskLevel = riskLevel
	}

	result := &SegmentationResult{
		PrimaryTarget:   primaryTarget,
		RelatedTargets:  relatedTargets,
		TotalTargets:    len(relatedTargets) + 1, // +1 for primary target
		CanarySize:      canarySize,
		Reasoning:       reasoning,
		CreatedAt:       time.Now(),
	}

	s.logger.Info("Target segmentation completed", 
		"primary_target", primaryTarget,
		"related_targets", len(relatedTargets),
		"total_targets", result.TotalTargets,
		"canary_size", canarySize)

	return result, nil
}

// analyzeGraphRelationships analyzes the graph to find related targets
func (s *SegmenterAgent) analyzeGraphRelationships(ctx context.Context, primaryTarget string) ([]RelatedTarget, string, error) {
	s.logger.Debug("Analyzing graph relationships", "primary_target", primaryTarget)

	// Prepare graph query to find connected hosts/services
	graphQuery := `
		MATCH (primary:Host {id: $target})
		MATCH (primary)-[:COMMUNICATES]->(ne:NetworkEndpoint)<-[:COMMUNICATES]-(connected:Host)
		WHERE primary.id <> connected.id
		RETURN DISTINCT connected.id as target_id, 
		       'host' as target_type,
		       'peer' as relationship,
		       ne.ip as connection_ip,
		       ne.port as connection_port,
		       ne.protocol as protocol
		ORDER BY connected.id
		LIMIT 10
	`

	// Execute graph query using agent runtime
	result, err := s.runtime.ExecuteAgent(ctx, "segmenter", 
		fmt.Sprintf("Find related targets for %s using graph query", primaryTarget), 
		map[string]any{
			"target": primaryTarget,
			"query": graphQuery,
		})
	if err != nil {
		return nil, "", fmt.Errorf("failed to execute graph analysis: %w", err)
	}

	// Extract results from tool execution
	var relatedTargets []RelatedTarget
	reasoning := "Graph analysis found connected hosts"

	for _, toolResult := range result.ToolResults {
		if toolResult.Function == "graph.query" && toolResult.Error == nil {
			if results, ok := toolResult.Result["results"].([]interface{}); ok {
				for _, result := range results {
					if resultMap, ok := result.(map[string]interface{}); ok {
						target := RelatedTarget{
							TargetID: resultMap["target_id"].(string),
							TargetType: resultMap["target_type"].(string),
							Relationship: resultMap["relationship"].(string),
							ConnectionDetails: map[string]any{
								"ip": resultMap["connection_ip"],
								"port": resultMap["connection_port"],
								"protocol": resultMap["protocol"],
							},
							Confidence: 0.8, // High confidence for direct connections
						}
						relatedTargets = append(relatedTargets, target)
					}
				}
			}
		}
	}

	if len(relatedTargets) == 0 {
		reasoning = "No direct connections found in graph"
	}

	return relatedTargets, reasoning, nil
}

// assessTargetRisk assesses risk context for a target
func (s *SegmenterAgent) assessTargetRisk(ctx context.Context, targetID string) (float64, string) {
	s.logger.Debug("Assessing target risk", "target_id", targetID)

	// For now, use fallback risk assessment to avoid agent runtime issues
	// TODO: Implement proper risk assessment when agent runtime is stable
	return s.generateFallbackRisk(targetID)
}

// calculateRiskLevel converts risk score to risk level
func (s *SegmenterAgent) calculateRiskLevel(riskScore float64) string {
	switch {
	case riskScore >= 8.0:
		return "critical"
	case riskScore >= 6.0:
		return "high"
	case riskScore >= 4.0:
		return "medium"
	default:
		return "low"
	}
}

// generateFallbackRisk generates fallback risk assessment
func (s *SegmenterAgent) generateFallbackRisk(targetID string) (float64, string) {
	// Generate realistic risk scores based on target patterns
	var riskScore float64
	
	if len(targetID) > 0 {
		// Higher risk for web servers
		if targetID[:3] == "web" {
			riskScore = 6.5
		} else if targetID[:2] == "db" {
			riskScore = 7.2 // Database servers are critical
		} else if targetID[:3] == "api" {
			riskScore = 5.8 // API servers moderate risk
		} else {
			riskScore = 4.2 // Default moderate risk
		}
	} else {
		riskScore = 5.0 // Default risk
	}

	riskLevel := s.calculateRiskLevel(riskScore)
	return riskScore, riskLevel
}

// generateFallbackTargets generates fallback related targets
func (s *SegmenterAgent) generateFallbackTargets(primaryTarget string) []RelatedTarget {
	// Generate realistic fallback targets based on primary target
	var targets []RelatedTarget

	if len(primaryTarget) > 0 {
		// Generate targets based on primary target type
		if primaryTarget[:3] == "web" {
			// Web servers typically connect to databases and load balancers
			targets = append(targets, RelatedTarget{
				TargetID: "db-01",
				TargetType: "host",
				Relationship: "dependency",
				RiskScore: 7.2,
				RiskLevel: "high",
				ConnectionDetails: map[string]any{"service": "mysql", "port": 3306},
				Confidence: 0.6,
			})
			targets = append(targets, RelatedTarget{
				TargetID: "lb-01",
				TargetType: "host",
				Relationship: "upstream",
				RiskScore: 5.5,
				RiskLevel: "medium",
				ConnectionDetails: map[string]any{"service": "nginx", "port": 80},
				Confidence: 0.7,
			})
		} else if primaryTarget[:2] == "db" {
			// Database servers connect to application servers
			targets = append(targets, RelatedTarget{
				TargetID: "app-01",
				TargetType: "host",
				Relationship: "downstream",
				RiskScore: 6.0,
				RiskLevel: "medium",
				ConnectionDetails: map[string]any{"service": "app", "port": 8080},
				Confidence: 0.8,
			})
		} else {
			// Generic fallback
			targets = append(targets, RelatedTarget{
				TargetID: "peer-01",
				TargetType: "host",
				Relationship: "peer",
				RiskScore: 5.0,
				RiskLevel: "medium",
				ConnectionDetails: map[string]any{"service": "generic", "port": 443},
				Confidence: 0.5,
			})
		}
	}

	return targets
}

// applyCanaryLimit applies canary size limit to targets
func (s *SegmenterAgent) applyCanaryLimit(targets []RelatedTarget, canarySize int) []RelatedTarget {
	if len(targets) <= canarySize {
		return targets
	}

	// Sort by risk score (highest first) and confidence (highest first)
	// This ensures we keep the most important and reliable targets
	sortedTargets := make([]RelatedTarget, len(targets))
	copy(sortedTargets, targets)

	// Simple sorting by risk score * confidence
	for i := 0; i < len(sortedTargets)-1; i++ {
		for j := i + 1; j < len(sortedTargets); j++ {
			scoreI := sortedTargets[i].RiskScore * sortedTargets[i].Confidence
			scoreJ := sortedTargets[j].RiskScore * sortedTargets[j].Confidence
			if scoreI < scoreJ {
				sortedTargets[i], sortedTargets[j] = sortedTargets[j], sortedTargets[i]
			}
		}
	}

	return sortedTargets[:canarySize]
}

// getDefaultCanarySize gets the default canary size from environment
func (s *SegmenterAgent) getDefaultCanarySize() int {
	if sizeStr := os.Getenv("DECISION_CANARY_SIZE"); sizeStr != "" {
		if size, err := strconv.Atoi(sizeStr); err == nil {
			return size
		}
	}
	// Default to 5 targets if not specified
	return 5
}

// GetSegmentationSummary returns a summary of segmentation results
func (s *SegmenterAgent) GetSegmentationSummary(result *SegmentationResult) map[string]any {
	// Count targets by risk level
	riskCounts := map[string]int{
		"low": 0,
		"medium": 0,
		"high": 0,
		"critical": 0,
	}

	// Count targets by relationship type
	relationshipCounts := map[string]int{
		"peer": 0,
		"dependency": 0,
		"upstream": 0,
		"downstream": 0,
	}

	for _, target := range result.RelatedTargets {
		riskCounts[target.RiskLevel]++
		relationshipCounts[target.Relationship]++
	}

	// Calculate average risk score
	var totalRisk float64
	for _, target := range result.RelatedTargets {
		totalRisk += target.RiskScore
	}
	avgRisk := float64(0)
	if len(result.RelatedTargets) > 0 {
		avgRisk = totalRisk / float64(len(result.RelatedTargets))
	}

	return map[string]any{
		"primary_target": result.PrimaryTarget,
		"total_related_targets": len(result.RelatedTargets),
		"canary_size": result.CanarySize,
		"risk_distribution": riskCounts,
		"relationship_distribution": relationshipCounts,
		"average_risk_score": avgRisk,
		"reasoning": result.Reasoning,
		"created_at": result.CreatedAt,
	}
}
