package rules


// Selector defines which hosts/events this rule applies to
type Selector struct {
	HostIDs        []string `yaml:"host_ids" json:"host_ids"`
	HostGlobs      []string `yaml:"host_globs" json:"host_globs"`
	Labels         []string `yaml:"labels" json:"labels"`
	ExcludeHostIDs []string `yaml:"exclude_host_ids" json:"exclude_host_ids"`
}

// EventPattern defines a pattern to match events
type EventPattern struct {
	EventType        string            `yaml:"event_type" json:"event_type"`
	BinaryPathRegex  string            `yaml:"binary_path_regex" json:"binary_path_regex"`
	Args             map[string]string `yaml:"args" json:"args"`
	Context          map[string]string `yaml:"context" json:"context"`
}

// Condition defines when a rule should trigger
type Condition struct {
	WindowSeconds int                     `yaml:"window_seconds" json:"window_seconds"`
	When          map[string]interface{}  `yaml:"when" json:"when"`
	RequiresPrior map[string]interface{}  `yaml:"requires_prior" json:"requires_prior"`
}

// Outcome defines what happens when a rule triggers
type Outcome struct {
	Severity   string   `yaml:"severity" json:"severity"`
	Confidence float64  `yaml:"confidence" json:"confidence"`
	Evidence   []string `yaml:"evidence" json:"evidence"`
}

// Dedupe defines deduplication settings for findings
type Dedupe struct {
	KeyTemplate     string `yaml:"key_template" json:"key_template"`
	CooldownSeconds int    `yaml:"cooldown_seconds" json:"cooldown_seconds"`
}

// RuleMetadata contains metadata about a rule
type RuleMetadata struct {
	ID      string `yaml:"id" json:"id"`
	Name    string `yaml:"name" json:"name"`
	Version string `yaml:"version" json:"version"`
}

// RuleSpec contains the rule specification
type RuleSpec struct {
	Enabled    bool      `yaml:"enabled" json:"enabled"`
	Selectors  Selector  `yaml:"selectors" json:"selectors"`
	Condition  Condition `yaml:"condition" json:"condition"`
	Outcome    Outcome   `yaml:"outcome" json:"outcome"`
	Dedupe     Dedupe    `yaml:"dedupe" json:"dedupe"`
	TTLSeconds int       `yaml:"ttl_seconds" json:"ttl_seconds"`
}

// Rule represents a complete correlation rule
type Rule struct {
	APIVersion string       `yaml:"apiVersion" json:"apiVersion"`
	Kind       string       `yaml:"kind" json:"kind"`
	Metadata   RuleMetadata `yaml:"metadata" json:"metadata"`
	Spec       RuleSpec     `yaml:"spec" json:"spec"`
	SourceFile string       `json:"source_file"` // Internal field for tracking source
}

// RuleSnapshot represents a collection of loaded rules
type RuleSnapshot struct {
	Rules   []Rule
	Version int64 // Timestamp when snapshot was created
}

// Validate checks if a rule is valid
func (r *Rule) Validate() error {
	// Check required fields
	if r.Metadata.ID == "" {
		return &ValidationError{Field: "metadata.id", Message: "rule ID is required"}
	}
	
	if r.Metadata.Name == "" {
		return &ValidationError{Field: "metadata.name", Message: "rule name is required"}
	}
	
	if r.Spec.Outcome.Severity == "" {
		return &ValidationError{Field: "spec.outcome.severity", Message: "severity is required"}
	}
	
	// Validate severity values
	validSeverities := map[string]bool{
		"low": true, "medium": true, "high": true, "critical": true,
	}
	if !validSeverities[r.Spec.Outcome.Severity] {
		return &ValidationError{Field: "spec.outcome.severity", Message: "invalid severity, must be low/medium/high/critical"}
	}
	
	// Validate confidence range
	if r.Spec.Outcome.Confidence < 0.0 || r.Spec.Outcome.Confidence > 1.0 {
		return &ValidationError{Field: "spec.outcome.confidence", Message: "confidence must be between 0.0 and 1.0"}
	}
	
	// Validate TTL
	if r.Spec.TTLSeconds <= 0 {
		return &ValidationError{Field: "spec.ttl_seconds", Message: "TTL must be positive"}
	}
	
	return nil
}

// IsEnabled checks if the rule is enabled
func (r *Rule) IsEnabled() bool {
	return r.Spec.Enabled
}

// ValidationError represents a rule validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
