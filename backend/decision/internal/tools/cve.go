package tools

import (
	"fmt"
	"log/slog"
	"math/rand"
	"time"
)

// CVEConfig contains configuration for the CVE tool
type CVEConfig struct {
	APIEndpoint string
	APIKey      string
	Timeout     time.Duration
	MockMode    bool // For testing without real CVE API
}

// CVETool provides interface to CVE lookup services
type CVETool struct {
	config CVEConfig
	logger *slog.Logger
}

// NewCVETool creates a new CVE tool instance
func NewCVETool(config CVEConfig, logger *slog.Logger) *CVETool {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &CVETool{
		config: config,
		logger: logger,
	}
}

// CVEInfo represents CVE information
type CVEInfo struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	Score       float64  `json:"score"`
	Packages    []string `json:"packages"`
	References  []string `json:"references"`
	Published   string   `json:"published"`
	Modified    string   `json:"modified"`
}

// Lookup performs CVE lookup for a host and list of packages
func (c *CVETool) Lookup(hostID string, pkgs []string) ([]map[string]any, error) {
	c.logger.Debug("Performing CVE lookup", "host_id", hostID, "packages", pkgs)

	if c.config.MockMode {
		return c.mockLookup(hostID, pkgs), nil
	}

	// TODO: Implement real CVE API integration
	// This would typically involve:
	// 1. Querying CVE databases (NVD, MITRE, etc.)
	// 2. Package vulnerability databases
	// 3. Host-specific vulnerability scanners
	
	return c.mockLookup(hostID, pkgs), nil
}

// mockLookup provides mock CVE data for testing
func (c *CVETool) mockLookup(hostID string, pkgs []string) []map[string]any {
	rand.Seed(time.Now().UnixNano())
	
	var results []map[string]any
	
	// Generate mock CVEs for some packages
	for _, pkg := range pkgs {
		// 30% chance of finding a CVE for each package
		if rand.Float32() < 0.3 {
			cve := c.generateMockCVE(pkg)
			cve["host_id"] = hostID
			cve["package"] = pkg
			results = append(results, cve)
		}
	}
	
	c.logger.Info("CVE lookup completed", "host_id", hostID, "packages_checked", len(pkgs), "vulnerabilities_found", len(results))
	return results
}

// generateMockCVE creates a mock CVE entry
func (c *CVETool) generateMockCVE(pkg string) map[string]any {
	severities := []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}
	severity := severities[rand.Intn(len(severities))]
	
	score := rand.Float64() * 10.0
	if severity == "CRITICAL" {
		score = 7.0 + rand.Float64()*3.0 // 7.0-10.0
	} else if severity == "HIGH" {
		score = 4.0 + rand.Float64()*3.0 // 4.0-7.0
	} else if severity == "MEDIUM" {
		score = 2.0 + rand.Float64()*2.0 // 2.0-4.0
	} else {
		score = rand.Float64() * 2.0 // 0.0-2.0
	}
	
	cveID := fmt.Sprintf("CVE-%d-%04d", 2020+rand.Intn(5), rand.Intn(10000))
	
	return map[string]any{
		"cve_id":       cveID,
		"description":  fmt.Sprintf("Vulnerability in %s package affecting security", pkg),
		"severity":     severity,
		"score":        score,
		"package":      pkg,
		"references":   []string{
			fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", cveID),
			fmt.Sprintf("https://cve.mitre.org/cgi-bin/cvename.cgi?name=%s", cveID),
		},
		"published":    time.Now().AddDate(0, -rand.Intn(24), -rand.Intn(30)).Format("2006-01-02"),
		"modified":     time.Now().AddDate(0, 0, -rand.Intn(7)).Format("2006-01-02"),
		"exploitable":  severity == "CRITICAL" || severity == "HIGH",
		"patch_available": rand.Float32() < 0.7, // 70% chance patch is available
	}
}

// GetHostVulnerabilities retrieves all known vulnerabilities for a host
func (c *CVETool) GetHostVulnerabilities(hostID string) ([]map[string]any, error) {
	// This would typically query a vulnerability scanner or asset management system
	// For now, return mock data
	
	c.logger.Debug("Getting host vulnerabilities", "host_id", hostID)
	
	if c.config.MockMode {
		return c.mockHostVulnerabilities(hostID), nil
	}
	
	// TODO: Implement real host vulnerability scanning
	return c.mockHostVulnerabilities(hostID), nil
}

// mockHostVulnerabilities provides mock host vulnerability data
func (c *CVETool) mockHostVulnerabilities(hostID string) []map[string]any {
	rand.Seed(time.Now().UnixNano())
	
	var vulnerabilities []map[string]any
	
	// Generate 0-5 vulnerabilities per host
	numVulns := rand.Intn(6)
	for i := 0; i < numVulns; i++ {
		vuln := map[string]any{
			"host_id":        hostID,
			"vulnerability_id": fmt.Sprintf("HOST-VULN-%d", rand.Intn(10000)),
			"title":          fmt.Sprintf("Host vulnerability %d on %s", i+1, hostID),
			"severity":       []string{"LOW", "MEDIUM", "HIGH", "CRITICAL"}[rand.Intn(4)],
			"description":    fmt.Sprintf("Security vulnerability detected on host %s", hostID),
			"detected_at":    time.Now().AddDate(0, 0, -rand.Intn(30)).Format("2006-01-02T15:04:05Z"),
			"status":         []string{"open", "in_progress", "patched", "false_positive"}[rand.Intn(4)],
			"cvss_score":     rand.Float64() * 10.0,
			"affected_services": []string{"ssh", "http", "mysql", "postgresql"}[rand.Intn(4)],
		}
		vulnerabilities = append(vulnerabilities, vuln)
	}
	
	return vulnerabilities
}

// GetPackageVulnerabilities retrieves vulnerabilities for specific packages
func (c *CVETool) GetPackageVulnerabilities(packages []string) ([]map[string]any, error) {
	c.logger.Debug("Getting package vulnerabilities", "packages", packages)
	
	// Aggregate vulnerabilities for all packages
	var allVulns []map[string]any
	for _, pkg := range packages {
		pkgVulns, err := c.Lookup("", []string{pkg})
		if err != nil {
			c.logger.Warn("Failed to lookup package vulnerabilities", "package", pkg, "error", err)
			continue
		}
		allVulns = append(allVulns, pkgVulns...)
	}
	
	return allVulns, nil
}
