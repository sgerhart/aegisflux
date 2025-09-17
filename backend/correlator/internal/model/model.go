package model

import (
	"time"
)

// Event represents a security event from the ETL pipeline
type Event struct {
	Timestamp   time.Time              `json:"timestamp"`
	HostID      string                 `json:"host_id"`
	EventType   string                 `json:"event_type"`
	BinaryPath  string                 `json:"binary_path"`
	Args        map[string]interface{} `json:"args"`
	Context     map[string]interface{} `json:"context"`
}

// Finding represents a security finding/correlation result
type Finding struct {
	ID           string                 `json:"id"`
	Severity     string                 `json:"severity"`     // low, medium, high, critical
	Confidence   float64                `json:"confidence"`   // 0.0 to 1.0
	Status       string                 `json:"status"`       // open, investigating, resolved, false_positive
	HostID       string                 `json:"host_id"`
	CVE          string                 `json:"cve,omitempty"`
	Evidence     []Evidence             `json:"evidence"`
	Timestamp    time.Time              `json:"timestamp"`
	RuleID       string                 `json:"rule_id"`
	TTLSeconds   int                    `json:"ttl_seconds"`
}

// Evidence represents supporting evidence for a finding
type Evidence struct {
	Type        string                 `json:"type"`        // event, correlation, external
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data"`
	Timestamp   time.Time              `json:"timestamp"`
}
