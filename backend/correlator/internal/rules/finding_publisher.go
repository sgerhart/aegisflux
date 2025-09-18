package rules

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// FindingPublisher handles publishing findings to NATS
type FindingPublisher struct {
	natsConn *nats.Conn
	logger   *slog.Logger
}

// NewFindingPublisher creates a new finding publisher
func NewFindingPublisher(natsConn *nats.Conn, logger *slog.Logger) *FindingPublisher {
	return &FindingPublisher{
		natsConn: natsConn,
		logger:   logger,
	}
}

// PublishFinding publishes a finding to the correlator.findings subject
func (fp *FindingPublisher) PublishFinding(finding *Finding) error {
	if fp.natsConn == nil || !fp.natsConn.IsConnected() {
		return fmt.Errorf("NATS connection not available")
	}

	// Serialize finding to JSON
	findingJSON, err := json.Marshal(finding)
	if err != nil {
		return fmt.Errorf("failed to marshal finding: %w", err)
	}

	// Create NATS headers
	headers := nats.Header{}
	headers.Set("x-finding-id", finding.ID)
	headers.Set("x-host-id", finding.HostID)
	headers.Set("x-rule-id", finding.RuleID)
	headers.Set("x-severity", finding.Severity)
	headers.Set("x-timestamp", finding.TS)
	headers.Set("x-correlation-id", finding.CorrelationID)

	// Publish finding
	msg := &nats.Msg{
		Subject: "correlator.findings",
		Data:    findingJSON,
		Header:  headers,
	}

	if err := fp.natsConn.PublishMsg(msg); err != nil {
		return fmt.Errorf("failed to publish finding: %w", err)
	}

	fp.logger.Info("Published finding",
		"finding_id", finding.ID,
		"rule_id", finding.RuleID,
		"host_id", finding.HostID,
		"severity", finding.Severity,
		"subject", "correlator.findings")

	return nil
}

// PublishFindings publishes multiple findings
func (fp *FindingPublisher) PublishFindings(findings []*Finding) error {
	var errors []error
	successCount := 0

	for _, finding := range findings {
		if err := fp.PublishFinding(finding); err != nil {
			errors = append(errors, fmt.Errorf("finding %s: %w", finding.ID, err))
		} else {
			successCount++
		}
	}

	fp.logger.Info("Published findings batch",
		"total", len(findings),
		"successful", successCount,
		"failed", len(errors))

	if len(errors) > 0 {
		return fmt.Errorf("failed to publish %d findings: %v", len(errors), errors)
	}

	return nil
}

// PublishFindingWithRetry publishes a finding with retry logic
func (fp *FindingPublisher) PublishFindingWithRetry(finding *Finding, maxRetries int, retryDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := fp.PublishFinding(finding); err != nil {
			lastErr = err
			if attempt < maxRetries {
				fp.logger.Warn("Failed to publish finding, retrying",
					"finding_id", finding.ID,
					"attempt", attempt+1,
					"max_retries", maxRetries,
					"error", err)
				time.Sleep(retryDelay)
				continue
			}
		} else {
			return nil
		}
	}

	return fmt.Errorf("failed to publish finding after %d attempts: %w", maxRetries+1, lastErr)
}

// PublishFindingAsync publishes a finding asynchronously
func (fp *FindingPublisher) PublishFindingAsync(finding *Finding) <-chan error {
	result := make(chan error, 1)

	go func() {
		defer close(result)
		result <- fp.PublishFinding(finding)
	}()

	return result
}

// PublishFindingsAsync publishes multiple findings asynchronously
func (fp *FindingPublisher) PublishFindingsAsync(findings []*Finding) <-chan error {
	result := make(chan error, 1)

	go func() {
		defer close(result)
		result <- fp.PublishFindings(findings)
	}()

	return result
}

// PublishFindingWithCallback publishes a finding with a callback
func (fp *FindingPublisher) PublishFindingWithCallback(finding *Finding, callback func(*Finding, error)) {
	go func() {
		err := fp.PublishFinding(finding)
		callback(finding, err)
	}()
}

// PublishFindingsWithCallback publishes multiple findings with a callback
func (fp *FindingPublisher) PublishFindingsWithCallback(findings []*Finding, callback func([]*Finding, error)) {
	go func() {
		err := fp.PublishFindings(findings)
		callback(findings, err)
	}()
}

// PublishFindingWithTimeout publishes a finding with a timeout
func (fp *FindingPublisher) PublishFindingWithTimeout(finding *Finding, timeout time.Duration) error {
	if fp.natsConn == nil || !fp.natsConn.IsConnected() {
		return fmt.Errorf("NATS connection not available")
	}

	// Serialize finding to JSON
	findingJSON, err := json.Marshal(finding)
	if err != nil {
		return fmt.Errorf("failed to marshal finding: %w", err)
	}

	// Create NATS headers
	headers := nats.Header{}
	headers.Set("x-finding-id", finding.ID)
	headers.Set("x-host-id", finding.HostID)
	headers.Set("x-rule-id", finding.RuleID)
	headers.Set("x-severity", finding.Severity)
	headers.Set("x-timestamp", finding.TS)
	headers.Set("x-correlation-id", finding.CorrelationID)

	// Publish finding with timeout
	msg := &nats.Msg{
		Subject: "correlator.findings",
		Data:    findingJSON,
		Header:  headers,
	}

	// Use NATS request with timeout
	reply, err := fp.natsConn.RequestMsg(msg, timeout)
	if err != nil {
		return fmt.Errorf("failed to publish finding with timeout: %w", err)
	}

	if reply != nil {
		fp.logger.Debug("Received reply for finding",
			"finding_id", finding.ID,
			"reply_subject", reply.Subject,
			"reply_data", string(reply.Data))
	}

	fp.logger.Info("Published finding with timeout",
		"finding_id", finding.ID,
		"rule_id", finding.RuleID,
		"host_id", finding.HostID,
		"severity", finding.Severity,
		"timeout", timeout)

	return nil
}

// PublishFindingWithAck publishes a finding and waits for acknowledgment
func (fp *FindingPublisher) PublishFindingWithAck(finding *Finding, ackTimeout time.Duration) error {
	if fp.natsConn == nil || !fp.natsConn.IsConnected() {
		return fmt.Errorf("NATS connection not available")
	}

	// Serialize finding to JSON
	findingJSON, err := json.Marshal(finding)
	if err != nil {
		return fmt.Errorf("failed to marshal finding: %w", err)
	}

	// Create NATS headers
	headers := nats.Header{}
	headers.Set("x-finding-id", finding.ID)
	headers.Set("x-host-id", finding.HostID)
	headers.Set("x-rule-id", finding.RuleID)
	headers.Set("x-severity", finding.Severity)
	headers.Set("x-timestamp", finding.TS)
	headers.Set("x-correlation-id", finding.CorrelationID)
	headers.Set("x-require-ack", "true")

	// Publish finding with acknowledgment
	msg := &nats.Msg{
		Subject: "correlator.findings",
		Data:    findingJSON,
		Header:  headers,
	}

	// Use NATS request to get acknowledgment
	reply, err := fp.natsConn.RequestMsg(msg, ackTimeout)
	if err != nil {
		return fmt.Errorf("failed to publish finding with ack: %w", err)
	}

	if reply == nil {
		return fmt.Errorf("no acknowledgment received for finding %s", finding.ID)
	}

	// Parse acknowledgment
	var ack struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(reply.Data, &ack); err != nil {
		return fmt.Errorf("failed to parse acknowledgment: %w", err)
	}

	if ack.Status != "ok" {
		return fmt.Errorf("finding acknowledgment failed: %s", ack.Message)
	}

	fp.logger.Info("Published finding with acknowledgment",
		"finding_id", finding.ID,
		"rule_id", finding.RuleID,
		"host_id", finding.HostID,
		"severity", finding.Severity,
		"ack_status", ack.Status)

	return nil
}

// PublishFindingWithDeduplication publishes a finding with deduplication
func (fp *FindingPublisher) PublishFindingWithDeduplication(finding *Finding, dedupeKey string) error {
	if fp.natsConn == nil || !fp.natsConn.IsConnected() {
		return fmt.Errorf("NATS connection not available")
	}

	// Serialize finding to JSON
	findingJSON, err := json.Marshal(finding)
	if err != nil {
		return fmt.Errorf("failed to marshal finding: %w", err)
	}

	// Create NATS headers
	headers := nats.Header{}
	headers.Set("x-finding-id", finding.ID)
	headers.Set("x-host-id", finding.HostID)
	headers.Set("x-rule-id", finding.RuleID)
	headers.Set("x-severity", finding.Severity)
	headers.Set("x-timestamp", finding.TS)
	headers.Set("x-correlation-id", finding.CorrelationID)
	headers.Set("x-dedupe-key", dedupeKey)

	// Publish finding with deduplication
	msg := &nats.Msg{
		Subject: "correlator.findings",
		Data:    findingJSON,
		Header:  headers,
	}

	if err := fp.natsConn.PublishMsg(msg); err != nil {
		return fmt.Errorf("failed to publish finding with deduplication: %w", err)
	}

	fp.logger.Info("Published finding with deduplication",
		"finding_id", finding.ID,
		"rule_id", finding.RuleID,
		"host_id", finding.HostID,
		"severity", finding.Severity,
		"dedupe_key", dedupeKey)

	return nil
}
