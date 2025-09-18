package systemd

import (
	"fmt"
	"net"
	"os"
	"time"
)

// Notifier provides systemd integration
type Notifier struct {
	socket string
	conn   net.Conn
}

// NewNotifier creates a new systemd notifier
func NewNotifier() *Notifier {
	return &Notifier{
		socket: os.Getenv("NOTIFY_SOCKET"),
	}
}

// IsAvailable checks if systemd notification is available
func (n *Notifier) IsAvailable() bool {
	return n.socket != "" && n.socket[0] == '@'
}

// Notify sends a notification to systemd
func (n *Notifier) Notify(state string) error {
	if !n.IsAvailable() {
		return fmt.Errorf("systemd notification not available")
	}
	
	// Connect to systemd socket
	if n.conn == nil {
		conn, err := net.Dial("unixgram", n.socket)
		if err != nil {
			return fmt.Errorf("failed to connect to systemd socket: %w", err)
		}
		n.conn = conn
	}
	
	// Send notification
	message := fmt.Sprintf("STATUS=%s\n", state)
	_, err := n.conn.Write([]byte(message))
	return err
}

// NotifyReady notifies systemd that the service is ready
func (n *Notifier) NotifyReady() error {
	if !n.IsAvailable() {
		return nil
	}
	
	message := "READY=1\n"
	if n.conn == nil {
		conn, err := net.Dial("unixgram", n.socket)
		if err != nil {
			return fmt.Errorf("failed to connect to systemd socket: %w", err)
		}
		n.conn = conn
	}
	
	_, err := n.conn.Write([]byte(message))
	return err
}

// NotifyStopping notifies systemd that the service is stopping
func (n *Notifier) NotifyStopping() error {
	if !n.IsAvailable() {
		return nil
	}
	
	message := "STOPPING=1\n"
	if n.conn == nil {
		conn, err := net.Dial("unixgram", n.socket)
		if err != nil {
			return fmt.Errorf("failed to connect to systemd socket: %w", err)
		}
		n.conn = conn
	}
	
	_, err := n.conn.Write([]byte(message))
	return err
}

// NotifyReloading notifies systemd that the service is reloading
func (n *Notifier) NotifyReloading() error {
	if !n.IsAvailable() {
		return nil
	}
	
	message := "RELOADING=1\n"
	if n.conn == nil {
		conn, err := net.Dial("unixgram", n.socket)
		if err != nil {
			return fmt.Errorf("failed to connect to systemd socket: %w", err)
		}
		n.conn = conn
	}
	
	_, err := n.conn.Write([]byte(message))
	return err
}

// NotifyWatchdog notifies systemd watchdog
func (n *Notifier) NotifyWatchdog() error {
	if !n.IsAvailable() {
		return nil
	}
	
	message := "WATCHDOG=1\n"
	if n.conn == nil {
		conn, err := net.Dial("unixgram", n.socket)
		if err != nil {
			return fmt.Errorf("failed to connect to systemd socket: %w", err)
		}
		n.conn = conn
	}
	
	_, err := n.conn.Write([]byte(message))
	return err
}

// NotifyStatus updates systemd status
func (n *Notifier) NotifyStatus(status string) error {
	return n.Notify(status)
}

// Close closes the systemd notification connection
func (n *Notifier) Close() error {
	if n.conn != nil {
		return n.conn.Close()
	}
	return nil
}

// StartWatchdog starts the systemd watchdog
func (n *Notifier) StartWatchdog(interval time.Duration) {
	if !n.IsAvailable() {
		return
	}
	
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			if err := n.NotifyWatchdog(); err != nil {
				// Log error but continue
				fmt.Printf("Failed to notify systemd watchdog: %v\n", err)
			}
		}
	}()
}
