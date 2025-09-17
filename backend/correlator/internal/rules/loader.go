package rules

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Loader handles loading and managing correlation rules
type Loader struct {
	rulesDir    string
	hotReload   bool
	logger      *slog.Logger
	mu          sync.RWMutex
	snapshot    *RuleSnapshot
	watchers    []chan struct{}
	debounceMs  int
}

// NewLoader creates a new rule loader
func NewLoader(rulesDir string, hotReload bool, debounceMs int, logger *slog.Logger) *Loader {
	return &Loader{
		rulesDir:   rulesDir,
		hotReload:  hotReload,
		logger:     logger,
		debounceMs: debounceMs,
	}
}

// LoadSnapshot loads all rules from the rules directory
func (l *Loader) LoadSnapshot() (*RuleSnapshot, error) {
	l.logger.Info("Loading rules snapshot", "rules_dir", l.rulesDir)
	
	// Read all rule files
	ruleFiles, err := l.readRuleFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to read rule files: %w", err)
	}
	
	// Load and parse rules
	var allRules []Rule
	ruleMap := make(map[string]Rule) // For deduplication by ID
	
	for _, file := range ruleFiles {
		rules, err := l.loadRulesFromFile(file)
		if err != nil {
			l.logger.Warn("Failed to load rules from file", "file", file, "error", err)
			continue
		}
		
		for _, rule := range rules {
			// Skip disabled rules
			if !rule.IsEnabled() {
				l.logger.Debug("Skipping disabled rule", "rule_id", rule.Metadata.ID, "file", file)
				continue
			}
			
			// Validate rule
			if err := rule.Validate(); err != nil {
				l.logger.Warn("Invalid rule skipped", "rule_id", rule.Metadata.ID, "file", file, "error", err)
				continue
			}
			
			// Handle rule ID conflicts (filename override wins)
			if existingRule, exists := ruleMap[rule.Metadata.ID]; exists {
				l.logger.Info("Rule ID conflict resolved by filename override", 
					"rule_id", rule.Metadata.ID, 
					"new_file", file, 
					"old_file", existingRule.SourceFile)
			}
			
			rule.SourceFile = file
			ruleMap[rule.Metadata.ID] = rule
		}
	}
	
	// Convert map to slice and sort by ID for consistent ordering
	for _, rule := range ruleMap {
		allRules = append(allRules, rule)
	}
	
	sort.Slice(allRules, func(i, j int) bool {
		return allRules[i].Metadata.ID < allRules[j].Metadata.ID
	})
	
	snapshot := &RuleSnapshot{
		Rules:   allRules,
		Version: time.Now().UnixNano(),
	}
	
	l.logger.Info("Rules snapshot loaded", 
		"total_rules", len(allRules), 
		"enabled_rules", len(ruleMap),
		"version", snapshot.Version)
	
	// Update internal snapshot
	l.mu.Lock()
	l.snapshot = snapshot
	l.mu.Unlock()
	
	// Notify watchers
	l.notifyWatchers()
	
	return snapshot, nil
}

// GetSnapshot returns the current rules snapshot
func (l *Loader) GetSnapshot() *RuleSnapshot {
	l.mu.RLock()
	defer l.mu.RUnlock()
	
	if l.snapshot == nil {
		return &RuleSnapshot{Rules: []Rule{}, Version: 0}
	}
	
	// Return a copy to prevent external modifications
	rules := make([]Rule, len(l.snapshot.Rules))
	copy(rules, l.snapshot.Rules)
	
	return &RuleSnapshot{
		Rules:   rules,
		Version: l.snapshot.Version,
	}
}

// WatchForChanges starts watching for rule file changes (if hot reload is enabled)
func (l *Loader) WatchForChanges() error {
	if !l.hotReload {
		l.logger.Info("Hot reload disabled")
		return nil
	}
	
	l.logger.Info("Starting rule file watcher", "rules_dir", l.rulesDir)
	
	// Create debounced reload channel
	reloadChan := make(chan struct{}, 1)
	
	// Start file watcher
	go l.watchFiles(reloadChan)
	
	// Start debounced reloader
	go l.debouncedReload(reloadChan)
	
	return nil
}

// Subscribe returns a channel that receives notifications when rules change
func (l *Loader) Subscribe() <-chan struct{} {
	ch := make(chan struct{}, 1)
	
	l.mu.Lock()
	l.watchers = append(l.watchers, ch)
	l.mu.Unlock()
	
	// Send current snapshot immediately
	go func() {
		ch <- struct{}{}
	}()
	
	return ch
}

// readRuleFiles reads all rule files from the rules directory, sorted by filename
func (l *Loader) readRuleFiles() ([]string, error) {
	var files []string
	
	err := filepath.WalkDir(l.rulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		// Skip directories
		if d.IsDir() {
			return nil
		}
		
		// Only process YAML and JSON files
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			files = append(files, path)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, err
	}
	
	// Sort files by filename for consistent loading order
	sort.Strings(files)
	
	return files, nil
}

// loadRulesFromFile loads rules from a single file
func (l *Loader) loadRulesFromFile(filename string) ([]Rule, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	
	// Determine file type and parse accordingly
	ext := strings.ToLower(filepath.Ext(filename))
	
	var rules []Rule
	
	if ext == ".yaml" || ext == ".yml" {
		// Try to parse as single rule first
		var rule Rule
		if err := yaml.Unmarshal(data, &rule); err == nil && rule.Metadata.ID != "" {
			// Single rule
			rules = append(rules, rule)
		} else {
			// Try parsing as array of rules
			if err := yaml.Unmarshal(data, &rules); err != nil {
				return nil, fmt.Errorf("failed to parse YAML: %w", err)
			}
		}
	} else if ext == ".json" {
		// JSON parsing would go here if needed
		return nil, fmt.Errorf("JSON parsing not yet implemented")
	} else {
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}
	
	l.logger.Debug("Loaded rules from file", "file", filename, "count", len(rules))
	return rules, nil
}

// watchFiles watches for file system changes
func (l *Loader) watchFiles(reloadChan chan struct{}) {
	// Simple polling-based watcher (in production, you might use fsnotify)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	var lastModTime time.Time
	
	for range ticker.C {
		hasChanges := false
		
		err := filepath.WalkDir(l.rulesDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			
			if d.IsDir() {
				return nil
			}
			
			ext := strings.ToLower(filepath.Ext(path))
			if ext == ".yaml" || ext == ".yml" || ext == ".json" {
				info, err := d.Info()
				if err != nil {
					return err
				}
				
				if info.ModTime().After(lastModTime) {
					lastModTime = info.ModTime()
					hasChanges = true
				}
			}
			
			return nil
		})
		
		if err != nil {
			l.logger.Error("Error watching files", "error", err)
			continue
		}
		
		if hasChanges {
			l.logger.Info("Rule files changed, triggering reload")
			select {
			case reloadChan <- struct{}{}:
			default:
				// Channel is full, skip this notification
			}
		}
	}
}

// debouncedReload handles debounced rule reloading
func (l *Loader) debouncedReload(reloadChan chan struct{}) {
	var timer *time.Timer
	
	for range reloadChan {
		if timer != nil {
			timer.Stop()
		}
		
		timer = time.AfterFunc(time.Duration(l.debounceMs)*time.Millisecond, func() {
			l.logger.Info("Debounced reload triggered")
			if _, err := l.LoadSnapshot(); err != nil {
				l.logger.Error("Failed to reload rules", "error", err)
			}
		})
	}
}

// notifyWatchers notifies all subscribed watchers
func (l *Loader) notifyWatchers() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	for _, ch := range l.watchers {
		select {
		case ch <- struct{}{}:
		default:
			// Channel is full, skip this notification
		}
	}
}
