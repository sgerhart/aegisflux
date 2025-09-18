package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/sgerhart/aegisflux/backend/correlator/internal/rules"
)

func main() {
	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fmt.Println("🧪 Testing Enhanced Correlator Rules\n")

	// Test 1: Rule validation
	fmt.Println("1. Testing rule validation...")
	testRuleValidation(logger)

	// Test 2: Finding generation
	fmt.Println("\n2. Testing finding generation...")
	testFindingGeneration(logger)

	// Test 3: Temporal window validation
	fmt.Println("\n3. Testing temporal window validation...")
	testTemporalWindowValidation(logger)

	// Test 4: Host selector validation
	fmt.Println("\n4. Testing host selector validation...")
	testHostSelectorValidation(logger)

	// Test 5: Action validation
	fmt.Println("\n5. Testing action validation...")
	testActionValidation(logger)

	fmt.Println("\n🎉 All tests completed!")
}

func testRuleValidation(logger *slog.Logger) {
	// Test valid rule
	rule := &rules.Rule{
		APIVersion: "v1",
		Kind:       "Rule",
		Metadata: rules.RuleMetadata{
			ID:      "test-rule",
			Name:    "Test Rule",
			Version: "1.0.0",
		},
		Spec: rules.RuleSpec{
			Enabled: true,
			Selectors: rules.Selector{
				HostIDs:        []string{"host-1", "host-2"},
				HostPatterns:   []string{"web-.*", "api-.*"},
				Environment:    []string{"production"},
				ServiceTypes:   []string{"web", "api"},
				Regions:        []string{"us-east-1"},
			},
			Condition: rules.Condition{
				WindowSeconds: 300,
				TemporalWindow: &rules.TemporalWindow{
					DurationSeconds: 600,
					StepSeconds:     60,
					OverlapAllowed:  true,
					WindowType:      "sliding",
				},
				TimeRange: &rules.TimeRange{
					StartHour: 9,
					EndHour:   17,
					Days:      []string{"monday", "tuesday", "wednesday", "thursday", "friday"},
					Timezone:  "UTC",
				},
				Frequency: &rules.Frequency{
					MinIntervalSeconds: 30,
					MaxIntervalSeconds: 300,
					BackoffMultiplier:  1.5,
				},
				When: map[string]interface{}{
					"event_type": "connect",
					"severity":   "high",
				},
			},
			Outcome: rules.Outcome{
				Severity:   "high",
				Confidence: 0.85,
				Title:      "Test Finding",
				Description: "This is a test finding",
				Tags:       []string{"test", "security"},
				Metadata: map[string]interface{}{
					"category": "test",
				},
				Actions: []rules.Action{
					{
						Type: "alert",
						Parameters: map[string]interface{}{
							"channel": "#test",
						},
						Priority: 1,
						Enabled:  true,
					},
				},
			},
			TTLSeconds: 3600,
		},
	}

	if err := rule.Validate(); err != nil {
		fmt.Printf("❌ Rule validation failed: %v\n", err)
	} else {
		fmt.Println("✅ Rule validation passed")
	}

	// Test invalid rule
	invalidRule := &rules.Rule{
		APIVersion: "v1",
		Kind:       "Rule",
		Metadata: rules.RuleMetadata{
			ID:      "", // Invalid: empty ID
			Name:    "Invalid Rule",
			Version: "1.0.0",
		},
		Spec: rules.RuleSpec{
			Enabled: true,
			Outcome: rules.Outcome{
				Severity:   "invalid", // Invalid severity
				Confidence: 1.5,       // Invalid confidence
			},
			TTLSeconds: -1, // Invalid TTL
		},
	}

	if err := invalidRule.Validate(); err != nil {
		fmt.Printf("✅ Invalid rule correctly rejected: %v\n", err)
	} else {
		fmt.Println("❌ Invalid rule should have been rejected")
	}
}

func testFindingGeneration(logger *slog.Logger) {
	// Create finding generator
	generator := rules.NewFindingGenerator(logger)

	// Create test rule
	rule := &rules.Rule{
		APIVersion: "v1",
		Kind:       "Rule",
		Metadata: rules.RuleMetadata{
			ID:      "test-finding-rule",
			Name:    "Test Finding Rule",
			Version: "1.0.0",
		},
		Spec: rules.RuleSpec{
			Enabled: true,
			Outcome: rules.Outcome{
				Severity:   "high",
				Confidence: 0.9,
				Title:      "Test Finding Title",
				Description: "Test finding description",
				Tags:       []string{"test", "security"},
				Actions: []rules.Action{
					{
						Type: "alert",
						Parameters: map[string]interface{}{
							"channel": "#test",
						},
						Priority: 1,
						Enabled:  true,
					},
				},
			},
		},
	}

	// Test evidence
	evidence := map[string]interface{}{
		"event_type": "connect",
		"dst_ip":     "192.168.1.100",
		"dst_port":   "443",
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	// Generate finding
	finding, err := generator.GenerateFinding(
		rule,
		"test-host-01",
		evidence,
		[]string{"event-1", "event-2"},
		time.Now().Add(-5*time.Minute),
		time.Now(),
	)

	if err != nil {
		fmt.Printf("❌ Finding generation failed: %v\n", err)
		return
	}

	// Validate finding
	if err := finding.Validate(); err != nil {
		fmt.Printf("❌ Generated finding is invalid: %v\n", err)
		return
	}

	// Print finding details
	findingJSON, _ := json.MarshalIndent(finding, "", "  ")
	fmt.Printf("✅ Finding generated successfully:\n%s\n", findingJSON)
}

func testTemporalWindowValidation(logger *slog.Logger) {
	// Test valid temporal window
	validWindow := &rules.TemporalWindow{
		DurationSeconds: 600,
		StepSeconds:     60,
		OverlapAllowed:  true,
		WindowType:      "sliding",
	}

	if err := validWindow.Validate(); err != nil {
		fmt.Printf("❌ Valid temporal window rejected: %v\n", err)
	} else {
		fmt.Println("✅ Valid temporal window accepted")
	}

	// Test invalid temporal window
	invalidWindow := &rules.TemporalWindow{
		DurationSeconds: 300,
		StepSeconds:     600, // Invalid: step > duration
		OverlapAllowed:  true,
		WindowType:      "invalid", // Invalid window type
	}

	if err := invalidWindow.Validate(); err != nil {
		fmt.Printf("✅ Invalid temporal window correctly rejected: %v\n", err)
	} else {
		fmt.Println("❌ Invalid temporal window should have been rejected")
	}
}

func testHostSelectorValidation(logger *slog.Logger) {
	// Test valid host selector
	validSelector := rules.Selector{
		HostIDs:        []string{"host-1", "host-2"},
		HostPatterns:   []string{"web-.*", "api-.*"},
		Environment:    []string{"production", "staging"},
		ServiceTypes:   []string{"web", "api", "database"},
		Regions:        []string{"us-east-1", "us-west-2"},
		Labels:         []string{"security-critical"},
		ExcludeHostIDs: []string{"test-*", "dev-*"},
	}

	// Basic validation (no specific validation method yet)
	if len(validSelector.HostIDs) > 0 || len(validSelector.HostPatterns) > 0 {
		fmt.Println("✅ Host selector structure valid")
	} else {
		fmt.Println("❌ Host selector structure invalid")
	}
}

func testActionValidation(logger *slog.Logger) {
	// Test valid action
	validAction := rules.Action{
		Type: "alert",
		Parameters: map[string]interface{}{
			"channel": "#security",
			"priority": "high",
		},
		Priority: 1,
		Enabled:  true,
	}

	if err := validAction.Validate(); err != nil {
		fmt.Printf("❌ Valid action rejected: %v\n", err)
	} else {
		fmt.Println("✅ Valid action accepted")
	}

	// Test invalid action
	invalidAction := rules.Action{
		Type: "invalid", // Invalid action type
		Parameters: map[string]interface{}{
			"channel": "#security",
		},
		Priority: -1, // Invalid priority
		Enabled:  true,
	}

	if err := invalidAction.Validate(); err != nil {
		fmt.Printf("✅ Invalid action correctly rejected: %v\n", err)
	} else {
		fmt.Println("❌ Invalid action should have been rejected")
	}
}
