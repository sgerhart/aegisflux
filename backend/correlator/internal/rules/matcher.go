package rules

import (
	"path/filepath"
	"strings"
)

// Matcher handles rule matching logic for hosts and labels
type Matcher struct{}

// NewMatcher creates a new rule matcher
func NewMatcher() *Matcher {
	return &Matcher{}
}

// EffectiveRulesFor returns the rules that apply to the given host and labels
func (m *Matcher) EffectiveRulesFor(hostID string, labels []string, rules []Rule) []Rule {
	var effectiveRules []Rule
	
	// Create a map for quick label lookup
	labelMap := make(map[string]bool)
	for _, label := range labels {
		labelMap[label] = true
	}
	
	for _, rule := range rules {
		if m.ruleMatches(hostID, labelMap, rule) {
			effectiveRules = append(effectiveRules, rule)
		}
	}
	
	return effectiveRules
}

// ruleMatches checks if a rule matches the given host and labels
func (m *Matcher) ruleMatches(hostID string, labelMap map[string]bool, rule Rule) bool {
	selectors := rule.Spec.Selectors
	
	// Check exclude_host_ids first (highest priority)
	if m.isHostExcluded(hostID, selectors.ExcludeHostIDs) {
		return false
	}
	
	// Check if any positive selectors are specified
	hasHostSelectors := len(selectors.HostIDs) > 0 || len(selectors.HostGlobs) > 0
	hasLabelSelectors := len(selectors.Labels) > 0
	
	// If no selectors are specified, the rule matches everything (except excluded hosts)
	if !hasHostSelectors && !hasLabelSelectors {
		return true
	}
	
	// Check host matching (if host selectors are specified)
	hostMatches := true
	if hasHostSelectors {
		hostMatches = false
		
		// Check host_ids (exact match)
		if len(selectors.HostIDs) > 0 && m.isHostInList(hostID, selectors.HostIDs) {
			hostMatches = true
		}
		
		// Check host_globs (pattern match)
		if len(selectors.HostGlobs) > 0 && m.isHostMatchingGlobs(hostID, selectors.HostGlobs) {
			hostMatches = true
		}
	}
	
	// Check label matching (if label selectors are specified)
	labelMatches := true
	if hasLabelSelectors {
		labelMatches = m.hasAllLabels(labelMap, selectors.Labels)
	}
	
	// Both host and label conditions must be satisfied (if specified)
	return hostMatches && labelMatches
}

// isHostExcluded checks if the host is in the exclude list
func (m *Matcher) isHostExcluded(hostID string, excludeHostIDs []string) bool {
	for _, excludeID := range excludeHostIDs {
		if hostID == excludeID {
			return true
		}
	}
	return false
}

// isHostInList checks if the host ID exactly matches any in the list
func (m *Matcher) isHostInList(hostID string, hostIDs []string) bool {
	for _, id := range hostIDs {
		if hostID == id {
			return true
		}
	}
	return false
}

// isHostMatchingGlobs checks if the host ID matches any of the glob patterns
func (m *Matcher) isHostMatchingGlobs(hostID string, hostGlobs []string) bool {
	for _, glob := range hostGlobs {
		// Use filepath.Match for glob pattern matching
		matched, err := filepath.Match(glob, hostID)
		if err == nil && matched {
			return true
		}
	}
	return false
}

// hasAllLabels checks if ALL required labels are present in the label map
func (m *Matcher) hasAllLabels(labelMap map[string]bool, requiredLabels []string) bool {
	for _, requiredLabel := range requiredLabels {
		if !labelMap[requiredLabel] {
			return false
		}
	}
	return true
}

// Helper function to parse label strings (key:value format)
// This can be used to normalize labels if needed
func parseLabels(labels []string) map[string]string {
	result := make(map[string]string)
	for _, label := range labels {
		parts := strings.SplitN(label, ":", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		} else {
			// Handle labels without values (just key)
			result[label] = ""
		}
	}
	return result
}
