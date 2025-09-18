package rules

import (
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MapSnapshotSynthesizer synthesizes MapSnapshot drafts from network risk findings
type MapSnapshotSynthesizer struct {
	logger *slog.Logger
	// Configuration
	defaultTTLSeconds int
	maxEdgesPerSnapshot int
	// Network analysis patterns
	suspiciousPorts    map[int]bool
	knownServices      map[int]string
	highRiskProtocols  map[string]bool
}

// NewMapSnapshotSynthesizer creates a new MapSnapshot synthesizer
func NewMapSnapshotSynthesizer(logger *slog.Logger) *MapSnapshotSynthesizer {
	return &MapSnapshotSynthesizer{
		logger: logger,
		defaultTTLSeconds: 3600, // 1 hour
		maxEdgesPerSnapshot: 50,
		suspiciousPorts: map[int]bool{
			22:   true, // SSH
			23:   true, // Telnet
			135:  true, // RPC
			139:  true, // NetBIOS
			445:  true, // SMB
			1433: true, // SQL Server
			3389: true, // RDP
			5432: true, // PostgreSQL
			6379: true, // Redis
			27017: true, // MongoDB
		},
		knownServices: map[int]string{
			80:   "http",
			443:  "https",
			22:   "ssh",
			23:   "telnet",
			25:   "smtp",
			53:   "dns",
			110:  "pop3",
			143:  "imap",
			993:  "imaps",
			995:  "pop3s",
			1433: "mssql",
			3306: "mysql",
			5432: "postgresql",
			6379: "redis",
			27017: "mongodb",
		},
		highRiskProtocols: map[string]bool{
			"tcp": true,
			"udp": true,
		},
	}
}

// SynthesizeMapSnapshot synthesizes a MapSnapshot from a network risk finding
func (ms *MapSnapshotSynthesizer) SynthesizeMapSnapshot(finding *Finding) (*MapSnapshotDraft, error) {
	// Extract network information from finding
	networkInfo, err := ms.extractNetworkInfo(finding)
	if err != nil {
		return nil, fmt.Errorf("failed to extract network info: %w", err)
	}

	// Generate service ID
	serviceID := ms.generateServiceID(finding, networkInfo)

	// Create network edges
	edges, err := ms.createNetworkEdges(finding, networkInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to create network edges: %w", err)
	}

	// Create allow CIDRs (whitelist)
	allowCIDRs := ms.createAllowCIDRs(finding, networkInfo)

	// Calculate TTL
	ttlSeconds := ms.calculateTTL(finding, networkInfo)

	// Create metadata
	meta := ms.createMetadata(finding, networkInfo)

	// Create MapSnapshot draft
	draft := &MapSnapshotDraft{
		ServiceID:  serviceID,
		Edges:      edges,
		AllowCIDRs: allowCIDRs,
		TTLSeconds: ttlSeconds,
		Meta:       meta,
		Reason:     ms.generateReason(finding, networkInfo),
		Confidence: ms.calculateConfidence(finding, networkInfo),
	}

	// Validate the draft
	if err := ms.validateMapSnapshotDraft(draft); err != nil {
		return nil, fmt.Errorf("invalid MapSnapshot draft: %w", err)
	}

	ms.logger.Info("Synthesized MapSnapshot draft",
		"finding_id", finding.ID,
		"service_id", serviceID,
		"edges_count", len(edges),
		"allow_cidrs_count", len(allowCIDRs),
		"ttl_seconds", ttlSeconds)

	return draft, nil
}

// NetworkInfo contains extracted network information from a finding
type NetworkInfo struct {
	SourceIP    string
	DestIP      string
	SourcePort  int
	DestPort    int
	Protocol    string
	HostID      string
	RiskLevel   string
	AttackType  string
	TimeWindow  time.Duration
}

// extractNetworkInfo extracts network information from a finding
func (ms *MapSnapshotSynthesizer) extractNetworkInfo(finding *Finding) (*NetworkInfo, error) {
	info := &NetworkInfo{
		HostID:    finding.HostID,
		RiskLevel: finding.Severity,
		AttackType: finding.Type,
	}

	// Extract from evidence
	if srcIP, ok := finding.Evidence["src_ip"].(string); ok {
		info.SourceIP = srcIP
	}
	if dstIP, ok := finding.Evidence["dst_ip"].(string); ok {
		info.DestIP = dstIP
	}
	if srcPort, ok := finding.Evidence["src_port"].(string); ok {
		if port, err := strconv.Atoi(srcPort); err == nil {
			info.SourcePort = port
		}
	}
	if dstPort, ok := finding.Evidence["dst_port"].(string); ok {
		if port, err := strconv.Atoi(dstPort); err == nil {
			info.DestPort = port
		}
	}
	if proto, ok := finding.Evidence["proto"].(string); ok {
		info.Protocol = proto
	}

	// Extract from args if available
	if args, ok := finding.Evidence["args"].(map[string]interface{}); ok {
		if srcIP, ok := args["src_ip"].(string); ok && info.SourceIP == "" {
			info.SourceIP = srcIP
		}
		if dstIP, ok := args["dst_ip"].(string); ok && info.DestIP == "" {
			info.DestIP = dstIP
		}
		if srcPort, ok := args["src_port"].(string); ok && info.SourcePort == 0 {
			if port, err := strconv.Atoi(srcPort); err == nil {
				info.SourcePort = port
			}
		}
		if dstPort, ok := args["dst_port"].(string); ok && info.DestPort == 0 {
			if port, err := strconv.Atoi(dstPort); err == nil {
				info.DestPort = port
			}
		}
		if proto, ok := args["proto"].(string); ok && info.Protocol == "" {
			info.Protocol = proto
		}
	}

	// Set defaults
	if info.Protocol == "" {
		info.Protocol = "tcp"
	}
	if info.DestPort == 0 {
		info.DestPort = 80 // Default HTTP port
	}

	// Calculate time window from finding metadata
	if windowStart, ok := finding.Metadata["window_start"].(string); ok {
		if windowEnd, ok := finding.Metadata["window_end"].(string); ok {
			if start, err := time.Parse(time.RFC3339, windowStart); err == nil {
				if end, err := time.Parse(time.RFC3339, windowEnd); err == nil {
					info.TimeWindow = end.Sub(start)
				}
			}
		}
	}

	return info, nil
}

// generateServiceID generates a service ID for the MapSnapshot
func (ms *MapSnapshotSynthesizer) generateServiceID(finding *Finding, networkInfo *NetworkInfo) int {
	// Use host ID hash as base
	hash := 0
	for _, char := range finding.HostID {
		hash = hash*31 + int(char)
	}

	// Add network info to hash
	if networkInfo.DestPort > 0 {
		hash = hash*31 + networkInfo.DestPort
	}
	if networkInfo.DestIP != "" {
		for _, char := range networkInfo.DestIP {
			hash = hash*31 + int(char)
		}
	}

	// Ensure positive service ID
	if hash < 0 {
		hash = -hash
	}

	return hash % 10000
}

// createNetworkEdges creates network edges for the MapSnapshot
func (ms *MapSnapshotSynthesizer) createNetworkEdges(finding *Finding, networkInfo *NetworkInfo) ([]NetworkEdge, error) {
	var edges []NetworkEdge

	// If we have specific destination IP and port, create targeted edge
	if networkInfo.DestIP != "" && networkInfo.DestPort > 0 {
		// Validate IP
		if net.ParseIP(networkInfo.DestIP) == nil {
			return nil, fmt.Errorf("invalid destination IP: %s", networkInfo.DestIP)
		}

		// Create CIDR from IP
		cidr := networkInfo.DestIP + "/32"
		if ms.isPrivateIP(networkInfo.DestIP) {
			// For private IPs, use /24 subnet
			parts := strings.Split(networkInfo.DestIP, ".")
			if len(parts) == 4 {
				cidr = fmt.Sprintf("%s.%s.%s.0/24", parts[0], parts[1], parts[2])
			}
		}

		edges = append(edges, NetworkEdge{
			DstCIDR: cidr,
			Proto:   networkInfo.Protocol,
			Port:    networkInfo.DestPort,
		})
	} else {
		// Create restrictive default edges based on attack type
		edges = ms.createDefaultEdges(finding, networkInfo)
	}

	// Limit number of edges
	if len(edges) > ms.maxEdgesPerSnapshot {
		edges = edges[:ms.maxEdgesPerSnapshot]
	}

	return edges, nil
}

// createDefaultEdges creates default restrictive edges based on attack type
func (ms *MapSnapshotSynthesizer) createDefaultEdges(finding *Finding, networkInfo *NetworkInfo) []NetworkEdge {
	var edges []NetworkEdge

	switch finding.Type {
	case "network_scan", "port_scan":
		// Block all external traffic
		edges = append(edges, NetworkEdge{
			DstCIDR: "0.0.0.0/0",
			Proto:   "any",
			Port:    0,
		})
	case "reconnaissance":
		// Block common reconnaissance ports
		reconPorts := []int{22, 23, 25, 53, 80, 443, 135, 139, 445, 1433, 3389}
		for _, port := range reconPorts {
			edges = append(edges, NetworkEdge{
				DstCIDR: "0.0.0.0/0",
				Proto:   "tcp",
				Port:    port,
			})
		}
	case "data_exfiltration":
		// Block common data exfiltration ports
		exfilPorts := []int{21, 22, 23, 25, 53, 80, 443, 993, 995}
		for _, port := range exfilPorts {
			edges = append(edges, NetworkEdge{
				DstCIDR: "0.0.0.0/0",
				Proto:   "tcp",
				Port:    port,
			})
		}
	case "lateral_movement":
		// Block internal network access
		edges = append(edges, NetworkEdge{
			DstCIDR: "10.0.0.0/8",
			Proto:   "any",
			Port:    0,
		})
		edges = append(edges, NetworkEdge{
			DstCIDR: "172.16.0.0/12",
			Proto:   "any",
			Port:    0,
		})
		edges = append(edges, NetworkEdge{
			DstCIDR: "192.168.0.0/16",
			Proto:   "any",
			Port:    0,
		})
	default:
		// Default restrictive policy
		edges = append(edges, NetworkEdge{
			DstCIDR: "0.0.0.0/0",
			Proto:   "any",
			Port:    0,
		})
	}

	return edges
}

// createAllowCIDRs creates allow CIDRs (whitelist) for the MapSnapshot
func (ms *MapSnapshotSynthesizer) createAllowCIDRs(finding *Finding, networkInfo *NetworkInfo) []NetworkEdge {
	var allowCIDRs []NetworkEdge

	// Allow essential services
	essentialPorts := []int{53} // DNS
	for _, port := range essentialPorts {
		allowCIDRs = append(allowCIDRs, NetworkEdge{
			DstCIDR: "0.0.0.0/0",
			Proto:   "udp",
			Port:    port,
		})
	}

	// Allow management access if source is internal
	if networkInfo.SourceIP != "" && ms.isPrivateIP(networkInfo.SourceIP) {
		allowCIDRs = append(allowCIDRs, NetworkEdge{
			DstCIDR: "0.0.0.0/0",
			Proto:   "tcp",
			Port:    22, // SSH
		})
	}

	return allowCIDRs
}

// calculateTTL calculates TTL based on finding severity and network info
func (ms *MapSnapshotSynthesizer) calculateTTL(finding *Finding, networkInfo *NetworkInfo) int {
	baseTTL := ms.defaultTTLSeconds

	// Adjust based on severity
	switch finding.Severity {
	case "critical":
		baseTTL = 7200 // 2 hours
	case "high":
		baseTTL = 3600 // 1 hour
	case "medium":
		baseTTL = 1800 // 30 minutes
	case "low":
		baseTTL = 900 // 15 minutes
	}

	// Adjust based on confidence
	if finding.Confidence > 0.9 {
		baseTTL = int(float64(baseTTL) * 1.5)
	} else if finding.Confidence < 0.8 {
		baseTTL = int(float64(baseTTL) * 0.5)
	}

	// Adjust based on attack type
	switch finding.Type {
	case "network_scan", "port_scan":
		baseTTL = int(float64(baseTTL) * 2.0) // Longer TTL for scans
	case "data_exfiltration":
		baseTTL = int(float64(baseTTL) * 1.5) // Longer TTL for exfiltration
	case "reconnaissance":
		baseTTL = int(float64(baseTTL) * 0.8) // Shorter TTL for reconnaissance
	}

	return baseTTL
}

// createMetadata creates metadata for the MapSnapshot
func (ms *MapSnapshotSynthesizer) createMetadata(finding *Finding, networkInfo *NetworkInfo) map[string]interface{} {
	meta := map[string]interface{}{
		"finding_id":      finding.ID,
		"rule_id":         finding.RuleID,
		"created_by":      "correlator",
		"auto_generated":  true,
		"severity":        finding.Severity,
		"confidence":      finding.Confidence,
		"attack_type":     finding.Type,
		"host_id":         finding.HostID,
		"created_at":      time.Now().UTC().Format(time.RFC3339),
	}

	// Add network-specific metadata
	if networkInfo.DestIP != "" {
		meta["destination_ip"] = networkInfo.DestIP
	}
	if networkInfo.DestPort > 0 {
		meta["destination_port"] = networkInfo.DestPort
		meta["destination_service"] = ms.knownServices[networkInfo.DestPort]
	}
	if networkInfo.Protocol != "" {
		meta["protocol"] = networkInfo.Protocol
	}
	if networkInfo.TimeWindow > 0 {
		meta["time_window_seconds"] = int(networkInfo.TimeWindow.Seconds())
	}

	// Add risk assessment
	meta["risk_assessment"] = ms.assessRisk(finding, networkInfo)

	return meta
}

// generateReason generates a human-readable reason for the MapSnapshot
func (ms *MapSnapshotSynthesizer) generateReason(finding *Finding, networkInfo *NetworkInfo) string {
	reason := fmt.Sprintf("Generated from finding %s: %s", finding.ID, finding.Description)

	if networkInfo.DestIP != "" && networkInfo.DestPort > 0 {
		reason += fmt.Sprintf(" (targeting %s:%d)", networkInfo.DestIP, networkInfo.DestPort)
	}

	if finding.Severity == "critical" {
		reason += " - CRITICAL severity requires immediate network restriction"
	}

	return reason
}

// calculateConfidence calculates confidence for the MapSnapshot
func (ms *MapSnapshotSynthesizer) calculateConfidence(finding *Finding, networkInfo *NetworkInfo) float64 {
	confidence := finding.Confidence

	// Adjust based on network info completeness
	if networkInfo.DestIP != "" && networkInfo.DestPort > 0 {
		confidence += 0.1 // More confident with specific targets
	}

	// Adjust based on attack type specificity
	switch finding.Type {
	case "network_scan", "port_scan":
		confidence += 0.05 // High confidence for scans
	case "data_exfiltration":
		confidence += 0.1 // Very high confidence for exfiltration
	case "reconnaissance":
		confidence -= 0.05 // Lower confidence for reconnaissance
	}

	// Ensure confidence is within bounds
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// assessRisk assesses the risk level of the network activity
func (ms *MapSnapshotSynthesizer) assessRisk(finding *Finding, networkInfo *NetworkInfo) map[string]interface{} {
	risk := map[string]interface{}{
		"level": finding.Severity,
		"score": ms.calculateRiskScore(finding, networkInfo),
		"factors": []string{},
	}

	// Add risk factors
	if networkInfo.DestPort > 0 && ms.suspiciousPorts[networkInfo.DestPort] {
		risk["factors"] = append(risk["factors"].([]string), "suspicious_port")
	}

	if networkInfo.DestIP != "" && !ms.isPrivateIP(networkInfo.DestIP) {
		risk["factors"] = append(risk["factors"].([]string), "external_target")
	}

	if finding.Confidence > 0.9 {
		risk["factors"] = append(risk["factors"].([]string), "high_confidence")
	}

	return risk
}

// calculateRiskScore calculates a numeric risk score
func (ms *MapSnapshotSynthesizer) calculateRiskScore(finding *Finding, networkInfo *NetworkInfo) float64 {
	score := 0.0

	// Base score from severity
	switch finding.Severity {
	case "critical":
		score = 0.9
	case "high":
		score = 0.7
	case "medium":
		score = 0.5
	case "low":
		score = 0.3
	}

	// Adjust based on confidence
	score = score * finding.Confidence

	// Adjust based on attack type
	switch finding.Type {
	case "data_exfiltration":
		score += 0.2
	case "lateral_movement":
		score += 0.15
	case "network_scan":
		score += 0.1
	case "reconnaissance":
		score += 0.05
	}

	// Ensure score is within bounds
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// isPrivateIP checks if an IP is private
func (ms *MapSnapshotSynthesizer) isPrivateIP(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check private IP ranges
	privateRanges := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(parsedIP) {
			return true
		}
	}

	return false
}

// validateMapSnapshotDraft validates a MapSnapshot draft
func (ms *MapSnapshotSynthesizer) validateMapSnapshotDraft(draft *MapSnapshotDraft) error {
	if draft.ServiceID < 0 {
		return fmt.Errorf("invalid service ID: %d", draft.ServiceID)
	}

	if len(draft.Edges) == 0 {
		return fmt.Errorf("no edges specified")
	}

	if draft.TTLSeconds < 60 {
		return fmt.Errorf("TTL too short: %d seconds", draft.TTLSeconds)
	}

	// Validate edges
	for i, edge := range draft.Edges {
		if edge.DstCIDR == "" {
			return fmt.Errorf("edge %d: empty destination CIDR", i)
		}
		if edge.Proto == "" {
			return fmt.Errorf("edge %d: empty protocol", i)
		}
		if edge.Port < 0 || edge.Port > 65535 {
			return fmt.Errorf("edge %d: invalid port: %d", i, edge.Port)
		}
	}

	return nil
}
