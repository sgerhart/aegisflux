package store

import (
	"container/ring"
	"sync"

	"github.com/aegisflux/correlator/internal/model"
	"github.com/hashicorp/golang-lru/v2"
)

// MemoryStore provides thread-safe storage for findings with ring buffer and LRU deduplication
type MemoryStore struct {
	mu         sync.RWMutex
	findings   *ring.Ring
	dedupe     *lru.Cache[string, bool]
	maxFindings int
	dedupeCap   int
}

// NewMemoryStore creates a new memory store with specified capacities
func NewMemoryStore(maxFindings, dedupeCap int) *MemoryStore {
	dedupeCache, _ := lru.New[string, bool](dedupeCap)
	
	return &MemoryStore{
		findings:     ring.New(maxFindings),
		dedupe:       dedupeCache,
		maxFindings:  maxFindings,
		dedupeCap:    dedupeCap,
	}
}

// AddFinding adds a finding to the ring buffer and checks for duplicates
func (s *MemoryStore) AddFinding(finding *model.Finding) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Create deduplication key based on finding characteristics
	dedupeKey := s.createDedupeKey(finding)
	
	// Check if we've seen this finding before
	if _, exists := s.dedupe.Get(dedupeKey); exists {
		return false // Duplicate, not added
	}
	
	// Add to dedupe cache
	s.dedupe.Add(dedupeKey, true)
	
	// Add to ring buffer
	s.findings.Value = finding
	s.findings = s.findings.Next()
	
	return true // Added successfully
}

// GetFindings returns all findings in chronological order (oldest first)
func (s *MemoryStore) GetFindings() []*model.Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var findings []*model.Finding
	
	// Iterate through ring buffer to collect all findings
	s.findings.Do(func(value interface{}) {
		if value != nil {
			if finding, ok := value.(*model.Finding); ok {
				findings = append(findings, finding)
			}
		}
	})
	
	return findings
}

// GetFindingsByHost returns findings for a specific host
func (s *MemoryStore) GetFindingsByHost(hostID string) []*model.Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var findings []*model.Finding
	
	s.findings.Do(func(value interface{}) {
		if value != nil {
			if finding, ok := value.(*model.Finding); ok && finding.HostID == hostID {
				findings = append(findings, finding)
			}
		}
	})
	
	return findings
}

// GetFindingsBySeverity returns findings with specified severity or higher
func (s *MemoryStore) GetFindingsBySeverity(minSeverity string) []*model.Finding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var findings []*model.Finding
	severityLevels := map[string]int{
		"low":      1,
		"medium":   2,
		"high":     3,
		"critical": 4,
	}
	
	minLevel := severityLevels[minSeverity]
	
	s.findings.Do(func(value interface{}) {
		if value != nil {
			if finding, ok := value.(*model.Finding); ok {
				if level, exists := severityLevels[finding.Severity]; exists && level >= minLevel {
					findings = append(findings, finding)
				}
			}
		}
	})
	
	return findings
}

// ClearFindings removes all findings and clears the dedupe cache
func (s *MemoryStore) ClearFindings() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	// Clear ring buffer
	for i := 0; i < s.findings.Len(); i++ {
		s.findings.Value = nil
		s.findings = s.findings.Next()
	}
	
	// Clear dedupe cache
	s.dedupe.Purge()
}

// GetStats returns store statistics
func (s *MemoryStore) GetStats() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	count := 0
	s.findings.Do(func(value interface{}) {
		if value != nil {
			count++
		}
	})
	
	return map[string]interface{}{
		"total_findings": count,
		"max_findings":   s.maxFindings,
		"dedupe_cap":     s.dedupeCap,
		"dedupe_size":    s.dedupe.Len(),
	}
}

// createDedupeKey creates a deduplication key for a finding
func (s *MemoryStore) createDedupeKey(finding *model.Finding) string {
	// Create a key based on finding characteristics that should be unique
	// This is a simple implementation - in production, you might want more sophisticated deduplication
	return finding.HostID + ":" + finding.RuleID + ":" + finding.Severity
}
