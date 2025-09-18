package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/aegisflux/correlator/internal/rules"
)

func main() {
	// Set up logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	fmt.Println("🧪 Testing Decision Integration\n")

	// Test 1: Finding Forwarder
	fmt.Println("1. Testing Finding Forwarder...")
	testFindingForwarder(logger)

	// Test 2: MapSnapshot Synthesizer
	fmt.Println("\n2. Testing MapSnapshot Synthesizer...")
	testMapSnapshotSynthesizer(logger)

	// Test 3: Decision Integration
	fmt.Println("\n3. Testing Decision Integration...")
	testDecisionIntegration(logger)

	// Test 4: Adaptive Safeguard Creation
	fmt.Println("\n4. Testing Adaptive Safeguard Creation...")
	testAdaptiveSafeguardCreation(logger)

	// Test 5: MapSnapshot Draft Creation
	fmt.Println("\n5. Testing MapSnapshot Draft Creation...")
	testMapSnapshotDraftCreation(logger)

	fmt.Println("\n🎉 All tests completed!")
}

func testFindingForwarder(logger *slog.Logger) {
	// Create finding forwarder (without NATS for testing)
	forwarder := rules.NewFindingForwarder(nil, logger)

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

	// Test shouldForward
	shouldForward := forwarder.ShouldForward(finding)
	fmt.Printf("  Should forward finding: %t\n", shouldForward)

	// Test isNetworkRisk
	isNetworkRisk := forwarder.IsNetworkRisk(finding)
	fmt.Printf("  Is network risk: %t\n", isNetworkRisk)

	// Test createAdaptiveSafeguard
	safeguard, err := forwarder.CreateAdaptiveSafeguard(finding)
	if err != nil {
		fmt.Printf("❌ Failed to create adaptive safeguard: %v\n", err)
		return
	}

	fmt.Printf("✅ Created adaptive safeguard: %s\n", safeguard.ID)
	fmt.Printf("  Type: %s\n", safeguard.Type)
	fmt.Printf("  Severity: %s\n", safeguard.Severity)
	fmt.Printf("  Priority: %d\n", safeguard.Priority)
	fmt.Printf("  Auto Approve: %t\n", safeguard.AutoApprove)
}

func testMapSnapshotSynthesizer(logger *slog.Logger) {
	// Create MapSnapshot synthesizer
	synthesizer := rules.NewMapSnapshotSynthesizer(logger)

	// Create test finding
	finding := &rules.Finding{
		ID:       "test-finding-02",
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

	// Test synthesizeMapSnapshot
	mapSnapshot, err := synthesizer.SynthesizeMapSnapshot(finding)
	if err != nil {
		fmt.Printf("❌ Failed to synthesize MapSnapshot: %v\n", err)
		return
	}

	fmt.Printf("✅ Synthesized MapSnapshot:\n")
	fmt.Printf("  Service ID: %d\n", mapSnapshot.ServiceID)
	fmt.Printf("  Edges: %d\n", len(mapSnapshot.Edges))
	fmt.Printf("  Allow CIDRs: %d\n", len(mapSnapshot.AllowCIDRs))
	fmt.Printf("  TTL Seconds: %d\n", mapSnapshot.TTLSeconds)
	fmt.Printf("  Confidence: %.2f\n", mapSnapshot.Confidence)
	fmt.Printf("  Reason: %s\n", mapSnapshot.Reason)

	// Print edges
	for i, edge := range mapSnapshot.Edges {
		fmt.Printf("  Edge %d: %s %s:%d\n", i+1, edge.Proto, edge.DstCIDR, edge.Port)
	}
}

func testDecisionIntegration(logger *slog.Logger) {
	// Create decision integration (without NATS for testing)
	integration := rules.NewDecisionIntegration(nil, logger)

	// Create test finding
	finding := &rules.Finding{
		ID:       "test-finding-03",
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

	// Test shouldProcess
	shouldProcess := integration.ShouldProcess(finding)
	fmt.Printf("  Should process finding: %t\n", shouldProcess)

	// Test isNetworkRisk
	isNetworkRisk := integration.IsNetworkRisk(finding)
	fmt.Printf("  Is network risk: %t\n", isNetworkRisk)

	// Test shouldCreatePlan
	shouldCreatePlan := integration.ShouldCreatePlan(finding)
	fmt.Printf("  Should create plan: %t\n", shouldCreatePlan)

	// Test determineStrategyMode
	strategyMode := integration.DetermineStrategyMode(finding)
	fmt.Printf("  Strategy mode: %s\n", strategyMode)

	// Test calculatePriority
	priority := integration.CalculatePriority(finding)
	fmt.Printf("  Priority: %d\n", priority)

	// Test shouldAutoApprove
	autoApprove := integration.ShouldAutoApprove(finding)
	fmt.Printf("  Auto approve: %t\n", autoApprove)
}

func testAdaptiveSafeguardCreation(logger *slog.Logger) {
	// Create finding forwarder
	forwarder := rules.NewFindingForwarder(nil, logger)

	// Create test finding
	finding := &rules.Finding{
		ID:       "test-finding-04",
		HostID:   "web-01",
		Severity: "critical",
		Type:     "data_exfiltration",
		Evidence: map[string]interface{}{
			"event_type": "connect",
			"dst_ip":     "203.0.113.1",
			"dst_port":   "443",
			"proto":      "tcp",
		},
		TS:         time.Now().UTC().Format(time.RFC3339),
		RuleID:     "test-rule",
		Confidence: 0.95,
		Tags:       []string{"network", "exfiltration"},
	}

	// Create adaptive safeguard
	safeguard, err := forwarder.CreateAdaptiveSafeguard(finding)
	if err != nil {
		fmt.Printf("❌ Failed to create adaptive safeguard: %v\n", err)
		return
	}

	// Print safeguard details
	fmt.Printf("✅ Created adaptive safeguard:\n")
	safeguardJSON, _ := json.MarshalIndent(safeguard, "", "  ")
	fmt.Printf("%s\n", safeguardJSON)
}

func testMapSnapshotDraftCreation(logger *slog.Logger) {
	// Create MapSnapshot synthesizer
	synthesizer := rules.NewMapSnapshotSynthesizer(logger)

	// Create test finding
	finding := &rules.Finding{
		ID:       "test-finding-05",
		HostID:   "web-01",
		Severity: "high",
		Type:     "port_scan",
		Evidence: map[string]interface{}{
			"event_type": "connect",
			"dst_ip":     "192.168.1.0/24",
			"dst_port":   "22",
			"proto":      "tcp",
		},
		TS:         time.Now().UTC().Format(time.RFC3339),
		RuleID:     "test-rule",
		Confidence: 0.9,
		Tags:       []string{"network", "scan", "ssh"},
	}

	// Create MapSnapshot draft
	mapSnapshot, err := synthesizer.SynthesizeMapSnapshot(finding)
	if err != nil {
		fmt.Printf("❌ Failed to create MapSnapshot draft: %v\n", err)
		return
	}

	// Print MapSnapshot details
	fmt.Printf("✅ Created MapSnapshot draft:\n")
	mapSnapshotJSON, _ := json.MarshalIndent(mapSnapshot, "", "  ")
	fmt.Printf("%s\n", mapSnapshotJSON)
}
