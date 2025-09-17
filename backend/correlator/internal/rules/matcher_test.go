package rules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatcher_EffectiveRulesFor(t *testing.T) {
	matcher := NewMatcher()
	
	// Create test rules
	rules := []Rule{
		{
			Metadata: RuleMetadata{ID: "rule-1", Name: "Rule 1"},
			Spec: RuleSpec{
				Selectors: Selector{
					HostIDs: []string{"host-001", "host-002"},
				},
			},
		},
		{
			Metadata: RuleMetadata{ID: "rule-2", Name: "Rule 2"},
			Spec: RuleSpec{
				Selectors: Selector{
					HostGlobs: []string{"web-*", "db-*"},
				},
			},
		},
		{
			Metadata: RuleMetadata{ID: "rule-3", Name: "Rule 3"},
			Spec: RuleSpec{
				Selectors: Selector{
					Labels: []string{"role:web", "env:prod"},
				},
			},
		},
		{
			Metadata: RuleMetadata{ID: "rule-4", Name: "Rule 4"},
			Spec: RuleSpec{
				Selectors: Selector{
					ExcludeHostIDs: []string{"host-001"},
				},
			},
		},
		{
			Metadata: RuleMetadata{ID: "rule-5", Name: "Rule 5"},
			Spec: RuleSpec{
				Selectors: Selector{
					HostIDs:        []string{"host-001", "host-002"},
					ExcludeHostIDs: []string{"host-001"},
					Labels:         []string{"role:web"},
				},
			},
		},
	}
	
	tests := []struct {
		name     string
		hostID   string
		labels   []string
		expected []string // Expected rule IDs
	}{
		{
			name:     "exact host match",
			hostID:   "host-001",
			labels:   []string{},
			expected: []string{"rule-1"},
		},
		{
			name:     "exact host match - host-002",
			hostID:   "host-002",
			labels:   []string{},
			expected: []string{"rule-1", "rule-4"},
		},
		{
			name:     "glob match - web host",
			hostID:   "web-001",
			labels:   []string{},
			expected: []string{"rule-2", "rule-4"},
		},
		{
			name:     "glob match - db host",
			hostID:   "db-master",
			labels:   []string{},
			expected: []string{"rule-2", "rule-4"},
		},
		{
			name:     "label match - web and prod",
			hostID:   "any-host",
			labels:   []string{"role:web", "env:prod"},
			expected: []string{"rule-3", "rule-4"},
		},
		{
			name:     "label match - only web",
			hostID:   "any-host",
			labels:   []string{"role:web"},
			expected: []string{"rule-4"},
		},
		{
			name:     "excluded host",
			hostID:   "host-001",
			labels:   []string{"role:web"},
			expected: []string{"rule-1"}, // rule-4 and rule-5 excluded due to exclude_host_ids
		},
		{
			name:     "no matches",
			hostID:   "other-host",
			labels:   []string{},
			expected: []string{"rule-4"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effective := matcher.EffectiveRulesFor(tt.hostID, tt.labels, rules)
			
			// Extract rule IDs for comparison
			var actualIDs []string
			for _, rule := range effective {
				actualIDs = append(actualIDs, rule.Metadata.ID)
			}
			
			assert.ElementsMatch(t, tt.expected, actualIDs, 
				"Expected rule IDs %v, got %v", tt.expected, actualIDs)
		})
	}
}

func TestMatcher_isHostExcluded(t *testing.T) {
	matcher := NewMatcher()
	
	tests := []struct {
		name           string
		hostID         string
		excludeHostIDs []string
		expected       bool
	}{
		{
			name:           "host not excluded",
			hostID:         "host-001",
			excludeHostIDs: []string{"host-002", "host-003"},
			expected:       false,
		},
		{
			name:           "host is excluded",
			hostID:         "host-001",
			excludeHostIDs: []string{"host-001", "host-002"},
			expected:       true,
		},
		{
			name:           "empty exclude list",
			hostID:         "host-001",
			excludeHostIDs: []string{},
			expected:       false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.isHostExcluded(tt.hostID, tt.excludeHostIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_isHostInList(t *testing.T) {
	matcher := NewMatcher()
	
	tests := []struct {
		name     string
		hostID   string
		hostIDs  []string
		expected bool
	}{
		{
			name:     "host in list",
			hostID:   "host-001",
			hostIDs:  []string{"host-001", "host-002"},
			expected: true,
		},
		{
			name:     "host not in list",
			hostID:   "host-003",
			hostIDs:  []string{"host-001", "host-002"},
			expected: false,
		},
		{
			name:     "empty list",
			hostID:   "host-001",
			hostIDs:  []string{},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.isHostInList(tt.hostID, tt.hostIDs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatcher_isHostMatchingGlobs(t *testing.T) {
	matcher := NewMatcher()
	
	tests := []struct {
		name       string
		hostID     string
		hostGlobs  []string
		expected   bool
	}{
		{
			name:       "matches web-* glob",
			hostID:     "web-001",
			hostGlobs:  []string{"web-*"},
			expected:   true,
		},
		{
			name:       "matches db-* glob",
			hostID:     "db-master",
			hostGlobs:  []string{"db-*"},
			expected:   true,
		},
		{
			name:       "matches multiple globs",
			hostID:     "web-001",
			hostGlobs:  []string{"web-*", "db-*"},
			expected:   true,
		},
		{
			name:       "no glob match",
			hostID:     "api-001",
			hostGlobs:  []string{"web-*", "db-*"},
			expected:   false,
		},
		{
			name:       "empty glob list",
			hostID:     "web-001",
			hostGlobs:  []string{},
			expected:   false,
		},
		{
			name:       "complex glob pattern",
			hostID:     "prod-web-01",
			hostGlobs:  []string{"prod-*", "*-web-*"},
			expected:   true,
		},
		{
			name:       "exact match with glob",
			hostID:     "web",
			hostGlobs:  []string{"web"},
			expected:   true,
		},
		{
			name:       "question mark glob",
			hostID:     "web1",
			hostGlobs:  []string{"web?"},
			expected:   true,
		},
		{
			name:       "question mark glob no match",
			hostID:     "web01",
			hostGlobs:  []string{"web?"},
			expected:   false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.isHostMatchingGlobs(tt.hostID, tt.hostGlobs)
			assert.Equal(t, tt.expected, result, 
				"Host %s with globs %v should match: %v", tt.hostID, tt.hostGlobs, tt.expected)
		})
	}
}

func TestMatcher_hasAllLabels(t *testing.T) {
	matcher := NewMatcher()
	
	tests := []struct {
		name           string
		labelMap       map[string]bool
		requiredLabels []string
		expected       bool
	}{
		{
			name: "all labels present",
			labelMap: map[string]bool{
				"role:web": true,
				"env:prod": true,
				"zone:us":  true,
			},
			requiredLabels: []string{"role:web", "env:prod"},
			expected:       true,
		},
		{
			name: "missing one label",
			labelMap: map[string]bool{
				"role:web": true,
				"env:prod": true,
			},
			requiredLabels: []string{"role:web", "env:prod", "zone:us"},
			expected:       false,
		},
		{
			name: "missing all labels",
			labelMap: map[string]bool{
				"role:api": true,
				"env:dev":  true,
			},
			requiredLabels: []string{"role:web", "env:prod"},
			expected:       false,
		},
		{
			name:           "no required labels",
			labelMap:       map[string]bool{"role:web": true},
			requiredLabels: []string{},
			expected:       true,
		},
		{
			name:           "empty label map",
			labelMap:       map[string]bool{},
			requiredLabels: []string{"role:web"},
			expected:       false,
		},
		{
			name: "single label match",
			labelMap: map[string]bool{
				"role:web": true,
			},
			requiredLabels: []string{"role:web"},
			expected:       true,
		},
		{
			name: "single label no match",
			labelMap: map[string]bool{
				"role:api": true,
			},
			requiredLabels: []string{"role:web"},
			expected:       false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.hasAllLabels(tt.labelMap, tt.requiredLabels)
			assert.Equal(t, tt.expected, result,
				"Label map %v with required %v should match: %v", 
				tt.labelMap, tt.requiredLabels, tt.expected)
		})
	}
}

func TestMatcher_ruleMatches(t *testing.T) {
	matcher := NewMatcher()
	
	tests := []struct {
		name     string
		hostID   string
		labelMap map[string]bool
		rule     Rule
		expected bool
	}{
		{
			name:   "matches host_ids only",
			hostID: "host-001",
			labelMap: map[string]bool{},
			rule: Rule{
				Spec: RuleSpec{
					Selectors: Selector{
						HostIDs: []string{"host-001", "host-002"},
					},
				},
			},
			expected: true,
		},
		{
			name:   "matches host_globs only",
			hostID: "web-001",
			labelMap: map[string]bool{},
			rule: Rule{
				Spec: RuleSpec{
					Selectors: Selector{
						HostGlobs: []string{"web-*"},
					},
				},
			},
			expected: true,
		},
		{
			name:   "matches labels only",
			hostID: "any-host",
			labelMap: map[string]bool{
				"role:web": true,
				"env:prod": true,
			},
			rule: Rule{
				Spec: RuleSpec{
					Selectors: Selector{
						Labels: []string{"role:web", "env:prod"},
					},
				},
			},
			expected: true,
		},
		{
			name:   "excluded by exclude_host_ids",
			hostID: "host-001",
			labelMap: map[string]bool{},
			rule: Rule{
				Spec: RuleSpec{
					Selectors: Selector{
						HostIDs:        []string{"host-001"},
						ExcludeHostIDs: []string{"host-001"},
					},
				},
			},
			expected: false,
		},
		{
			name:   "complex rule with all selectors",
			hostID: "web-001",
			labelMap: map[string]bool{
				"role:web": true,
				"env:prod": true,
			},
			rule: Rule{
				Spec: RuleSpec{
					Selectors: Selector{
						HostIDs:        []string{"web-001", "web-002"},
						HostGlobs:      []string{"web-*"},
						Labels:         []string{"role:web"},
						ExcludeHostIDs: []string{"web-999"},
					},
				},
			},
			expected: true,
		},
		{
			name:   "fails host_ids check",
			hostID: "host-003",
			labelMap: map[string]bool{},
			rule: Rule{
				Spec: RuleSpec{
					Selectors: Selector{
						HostIDs: []string{"host-001", "host-002"},
					},
				},
			},
			expected: false,
		},
		{
			name:   "fails host_globs check",
			hostID: "api-001",
			labelMap: map[string]bool{},
			rule: Rule{
				Spec: RuleSpec{
					Selectors: Selector{
						HostGlobs: []string{"web-*", "db-*"},
					},
				},
			},
			expected: false,
		},
		{
			name:   "fails labels check",
			hostID: "web-001",
			labelMap: map[string]bool{
				"role:api": true,
			},
			rule: Rule{
				Spec: RuleSpec{
					Selectors: Selector{
						Labels: []string{"role:web", "env:prod"},
					},
				},
			},
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.ruleMatches(tt.hostID, tt.labelMap, tt.rule)
			assert.Equal(t, tt.expected, result,
				"Host %s with labels %v and rule %+v should match: %v",
				tt.hostID, tt.labelMap, tt.rule.Spec.Selectors, tt.expected)
		})
	}
}

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   []string
		expected map[string]string
	}{
		{
			name:   "key-value labels",
			labels: []string{"role:web", "env:prod", "zone:us"},
			expected: map[string]string{
				"role": "web",
				"env":  "prod",
				"zone": "us",
			},
		},
		{
			name:   "mixed key-value and key-only",
			labels: []string{"role:web", "enabled", "env:prod"},
			expected: map[string]string{
				"role":    "web",
				"enabled": "",
				"env":     "prod",
			},
		},
		{
			name:     "empty labels",
			labels:   []string{},
			expected: map[string]string{},
		},
		{
			name:   "labels with colons in value",
			labels: []string{"url:https://example.com", "config:key=value"},
			expected: map[string]string{
				"url":    "https://example.com",
				"config": "key=value",
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLabels(tt.labels)
			assert.Equal(t, tt.expected, result)
		})
	}
}
