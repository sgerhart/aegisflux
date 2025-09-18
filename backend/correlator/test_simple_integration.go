package main

import (
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

	fmt.Println("🧪 Testing Simple Integration\n")

	// Test 1: Finding Forwarder Creation
	fmt.Println("1. Testing Finding Forwarder Creation...")
	testFindingForwarderCreation(logger)

	// Test 2: MapSnapshot Synthesizer Creation
	fmt.Println("\n2. Testing MapSnapshot Synthesizer Creation...")
	testMapSnapshotSynthesizerCreation(logger)

	// Test 3: Decision Integration Creation
	fmt.Println("\n3. Testing Decision Integration Creation...")
	testDecisionIntegrationCreation(logger)

	// Test 4: Finding Generation
	fmt.Println("\n4. Testing Finding Generation...")
	testFindingGeneration(logger)

	fmt.Println("\n🎉 All tests completed!")
}

func testFindingForwarderCreation(logger *slog.Logger) {
	// Create finding forwarder
	forwarder := rules.NewFindingForwarder(nil, logger)
	
	if forwarder != nil {
		fmt.Printf("✅ Finding forwarder created successfully\n")
		
		// Test statistics
		stats := forwarder.GetStatistics()
		fmt.Printf("  Statistics: %+v\n", stats)
	} else {
		fmt.Printf("❌ Failed to create finding forwarder\n")
	}
}

func testMapSnapshotSynthesizerCreation(logger *slog.Logger) {
	// Create MapSnapshot synthesizer
	synthesizer := rules.NewMapSnapshotSynthesizer(logger)
	
	if synthesizer != nil {
		fmt.Printf("✅ MapSnapshot synthesizer created successfully\n")
	} else {
		fmt.Printf("❌ Failed to create MapSnapshot synthesizer\n")
	}
}

func testDecisionIntegrationCreation(logger *slog.Logger) {
	// Create decision integration
	integration := rules.NewDecisionIntegration(nil, logger)
	
	if integration != nil {
		fmt.Printf("✅ Decision integration created successfully\n")
		
		// Test statistics
		stats := integration.GetStatistics()
		fmt.Printf("  Statistics: %+v\n", stats)
	} else {
		fmt.Printf("❌ Failed to create decision integration\n")
	}
}

func testFindingGeneration(logger *slog.Logger) {
	// Create finding generator
	_ = rules.NewFindingGenerator(logger)
	
	// Create test finding
	finding := &rules.Finding{
		ID:       "test-finding-01",
		HostID:   "web-01",
		Severity: "high",
		Type:     "network_scan",
		Evidence: map[string]interface{}{
			"event_type": "connect",
			"dst_ip":     "192.168.1.100",
			"dst_port":   "80",
			"proto":      "tcp",
		},
		TS:         time.Now().UTC().Format(time.RFC3339),
		RuleID:     "test-rule",
		Confidence: 0.85,
		Tags:       []string{"network", "scan"},
	}

	// Test finding validation
	if err := finding.Validate(); err != nil {
		fmt.Printf("❌ Finding validation failed: %v\n", err)
		return
	}

	fmt.Printf("✅ Finding validation passed\n")
	fmt.Printf("  Finding ID: %s\n", finding.ID)
	fmt.Printf("  Host ID: %s\n", finding.HostID)
	fmt.Printf("  Severity: %s\n", finding.Severity)
	fmt.Printf("  Type: %s\n", finding.Type)
	fmt.Printf("  Confidence: %.2f\n", finding.Confidence)
}
