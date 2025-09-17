package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// GraphConfig contains configuration for the graph tool
type GraphConfig struct {
	Neo4jURL      string
	Username      string
	Password      string
	Database      string
	Timeout       time.Duration
	MaxResults    int
}

// GraphTool provides interface to Neo4j graph database
type GraphTool struct {
	config GraphConfig
	client *http.Client
	logger *slog.Logger
}

// NewGraphTool creates a new graph tool instance
func NewGraphTool(config GraphConfig, logger *slog.Logger) *GraphTool {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxResults == 0 {
		config.MaxResults = 1000
	}

	return &GraphTool{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: logger,
	}
}

// GraphQuery executes a Cypher query against the Neo4j database
func (g *GraphTool) GraphQuery(cypher string, params map[string]any, limit int) ([]map[string]any, error) {
	// Set default limit if not provided
	if limit <= 0 {
		limit = g.config.MaxResults
	}

	// Prepare the request payload
	payload := map[string]any{
		"query":      cypher,
		"parameters": params,
		"resultDataContents": []string{"row", "graph"},
		"includeStats": false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal query payload: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/db/%s/tx/commit", g.config.Neo4jURL, g.config.Database)
	req, err := http.NewRequestWithContext(context.Background(), "POST", url, 
		&bytes.Buffer{})
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	
	// Set basic auth if credentials provided
	if g.config.Username != "" && g.config.Password != "" {
		req.SetBasicAuth(g.config.Username, g.config.Password)
	}

	// Set the JSON body
	req.Body = io.NopCloser(bytes.NewReader(jsonData))

	g.logger.Debug("Executing Cypher query", 
		"query", cypher, 
		"params", params, 
		"limit", limit)

	// Execute the request
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Neo4j query failed with status %d: %s", 
			resp.StatusCode, string(body))
	}

	// Parse the response
	var result Neo4jResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract results
	results := make([]map[string]any, 0, limit)
	for _, resultItem := range result.Results {
		for i, row := range resultItem.Data {
			if i >= limit {
				break
			}
			
			rowData := make(map[string]any)
			for j, column := range resultItem.Columns {
				if j < len(row) {
					rowData[column] = row[j]
				}
			}
			results = append(results, rowData)
		}
	}

	g.logger.Info("Cypher query executed successfully", 
		"result_count", len(results),
		"query_time_ms", result.Results[0].Meta.ResultAvailableAfter+result.Results[0].Meta.ResultConsumedAfter)

	return results, nil
}

// Neo4jResponse represents the response structure from Neo4j HTTP API
type Neo4jResponse struct {
	Results []Neo4jResult `json:"results"`
	Errors  []Neo4jError  `json:"errors"`
}

type Neo4jResult struct {
	Columns []string       `json:"columns"`
	Data    []Neo4jRow     `json:"data"`
	Meta    Neo4jMeta      `json:"meta"`
}

type Neo4jRow []any

type Neo4jMeta struct {
	ResultAvailableAfter int `json:"result_available_after"`
	ResultConsumedAfter  int `json:"result_consumed_after"`
}

type Neo4jError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// GetHostConnections retrieves network connections for a specific host
func (g *GraphTool) GetHostConnections(hostID string, limit int) ([]map[string]any, error) {
	cypher := `
		MATCH (h:Host {id: $hostID})-[:COMMUNICATES]->(ne:NetworkEndpoint)
		RETURN h.id as host_id, ne.ip as dst_ip, ne.port as dst_port, ne.protocol as protocol
		ORDER BY ne.port
		LIMIT $limit
	`
	
	params := map[string]any{
		"hostID": hostID,
		"limit":  limit,
	}
	
	return g.GraphQuery(cypher, params, limit)
}

// GetConnectedHosts retrieves hosts that have communicated with the given host
func (g *GraphTool) GetConnectedHosts(hostID string, limit int) ([]map[string]any, error) {
	cypher := `
		MATCH (h1:Host {id: $hostID})-[:COMMUNICATES]->(ne:NetworkEndpoint)<-[:COMMUNICATES]-(h2:Host)
		WHERE h1.id <> h2.id
		RETURN DISTINCT h2.id as connected_host, ne.ip as endpoint_ip, ne.port as endpoint_port
		ORDER BY h2.id
		LIMIT $limit
	`
	
	params := map[string]any{
		"hostID": hostID,
		"limit":  limit,
	}
	
	return g.GraphQuery(cypher, params, limit)
}

// GetHostLabels retrieves labels for a specific host
func (g *GraphTool) GetHostLabels(hostID string) ([]string, error) {
	cypher := `
		MATCH (h:Host {id: $hostID})
		RETURN h.labels as labels
	`
	
	params := map[string]any{
		"hostID": hostID,
	}
	
	results, err := g.GraphQuery(cypher, params, 1)
	if err != nil {
		return nil, err
	}
	
	if len(results) == 0 {
		return []string{}, nil
	}
	
	labels, ok := results[0]["labels"].([]string)
	if !ok {
		// Try to parse from JSON string if stored as string
		if labelsStr, ok := results[0]["labels"].(string); ok {
			var parsedLabels []string
			if err := json.Unmarshal([]byte(labelsStr), &parsedLabels); err == nil {
				return parsedLabels, nil
			}
		}
		return []string{}, nil
	}
	
	return labels, nil
}
