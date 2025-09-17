package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"aegisflux/backend/ingest/protos"
)

const (
	// Default subject for publishing events
	DefaultSubject = "events.raw"
	// Connection timeout
	ConnectTimeout = 10 * time.Second
	// Reconnect interval
	ReconnectInterval = 5 * time.Second
	// Max reconnect attempts
	MaxReconnectAttempts = 10
)

// Publisher implements event publishing to NATS
type Publisher struct {
	conn      *nats.Conn
	subject   string
	logger    *slog.Logger
	mu        sync.RWMutex
	ready     bool
	reconnect chan struct{}
}

// NewPublisher creates a new NATS publisher
func NewPublisher(natsURL, subject string, logger *slog.Logger) (*Publisher, error) {
	if subject == "" {
		subject = DefaultSubject
	}

	p := &Publisher{
		subject:   subject,
		logger:    logger,
		reconnect: make(chan struct{}, 1),
	}

	// Connect to NATS with fail-fast behavior
	if err := p.connect(natsURL); err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Start reconnect goroutine
	go p.reconnectLoop(natsURL)

	logger.Info("NATS publisher initialized", "url", natsURL, "subject", subject)
	return p, nil
}

// connect establishes connection to NATS
func (p *Publisher) connect(natsURL string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close existing connection if any
	if p.conn != nil {
		p.conn.Close()
	}

	// Connect with timeout

	conn, err := nats.Connect(natsURL, nats.Timeout(ConnectTimeout))
	if err != nil {
		p.ready = false
		return fmt.Errorf("failed to connect to NATS at %s: %w", natsURL, err)
	}

	// Note: Event handlers can be added here if needed

	p.conn = conn
	p.ready = true

	p.logger.Info("Connected to NATS", "url", natsURL)
	return nil
}

// reconnectLoop handles automatic reconnection
func (p *Publisher) reconnectLoop(natsURL string) {
	for range p.reconnect {
		p.mu.Lock()
		ready := p.ready
		p.mu.Unlock()

		if ready {
			continue
		}

		p.logger.Warn("Attempting to reconnect to NATS", "url", natsURL)
		
		if err := p.connect(natsURL); err != nil {
			p.logger.Error("Reconnection failed", "error", err)
			time.Sleep(ReconnectInterval)
			continue
		}

		p.logger.Info("Successfully reconnected to NATS")
	}
}

// PublishEvent publishes an event to NATS
func (p *Publisher) PublishEvent(ctx context.Context, e *protos.Event) error {
	p.mu.RLock()
	conn := p.conn
	ready := p.ready
	p.mu.RUnlock()

	if !ready || conn == nil {
		return fmt.Errorf("NATS publisher not ready")
	}

	// Convert event to JSON
	eventData, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("failed to marshal event to JSON: %w", err)
	}

	// Create message with headers
	msg := nats.NewMsg(p.subject)
	msg.Data = eventData

	// Add host-id header if present in metadata
	if hostID, exists := e.Metadata["host_id"]; exists {
		msg.Header.Set("x-host-id", hostID)
	}

	// Add additional headers
	msg.Header.Set("x-event-id", e.Id)
	msg.Header.Set("x-event-type", e.Type)
	msg.Header.Set("x-event-source", e.Source)
	msg.Header.Set("x-timestamp", fmt.Sprintf("%d", e.Timestamp))

	// Publish with timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	select {
	case <-ctx.Done():
		return fmt.Errorf("publish timeout: %w", ctx.Err())
	default:
		if err := conn.PublishMsg(msg); err != nil {
			p.logger.Error("Failed to publish event", "event_id", e.Id, "error", err)
			return fmt.Errorf("failed to publish event: %w", err)
		}
	}

	p.logger.Debug("Event published successfully", "event_id", e.Id, "subject", p.subject)
	return nil
}

// IsReady returns the readiness status of the publisher
func (p *Publisher) IsReady() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.ready && p.conn != nil && p.conn.IsConnected()
}

// Close closes the NATS connection
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
	p.ready = false

	close(p.reconnect)
	p.logger.Info("NATS publisher closed")
	return nil
}

