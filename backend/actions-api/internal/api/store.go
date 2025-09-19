package api

import (
	"sync"
	"time"
)
type Pending struct {
	RegistrationID string
	OrgID, HostID  string
	PubKey, Nonce  []byte
	Created        time.Time
	ServerTime     string
	// Richer metadata from registration
	MachineIDHash string
	AgentVersion  string
	Capabilities  map[string]any
	Platform      map[string]any
	Network       map[string]any
}
type Agent struct {
	AgentUID       string
	OrgID, HostID  string
	PubKey         []byte
	Created, LastSeen time.Time
	// Richer metadata
	Hostname       string
	MachineIDHash  string
	AgentVersion   string
	Capabilities   map[string]any
	Platform       map[string]any
	Network        map[string]any
	Labels         map[string]bool
	Note           string
}
type Store struct {
	mu sync.Mutex
	pending map[string]*Pending
	agents  map[string]*Agent
	byHost  map[string]*Agent
}
func NewStore()*Store{
	return &Store{pending: map[string]*Pending{}, agents: map[string]*Agent{}, byHost: map[string]*Agent{}}
}
