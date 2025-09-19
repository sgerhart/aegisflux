package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// AgentListResponse represents the response for listing agents
type AgentListResponse struct {
	Agents []AgentInfo `json:"agents"`
	Total  int         `json:"total"`
}

// AgentInfo represents agent information for list responses (without sensitive data)
type AgentInfo struct {
	AgentUID      string                 `json:"agent_uid"`
	OrgID         string                 `json:"org_id"`
	HostID        string                 `json:"host_id"`
	Hostname      string                 `json:"hostname,omitempty"`
	MachineIDHash string                 `json:"machine_id_hash,omitempty"`
	AgentVersion  string                 `json:"agent_version,omitempty"`
	Platform      map[string]any         `json:"platform,omitempty"`
	Network       map[string]any         `json:"network,omitempty"`
	Labels        []string               `json:"labels"`
	Note          string                 `json:"note,omitempty"`
	Created       time.Time              `json:"created"`
	LastSeen      time.Time              `json:"last_seen"`
}

// AgentDetailResponse represents the full agent response
type AgentDetailResponse struct {
	AgentUID      string                 `json:"agent_uid"`
	OrgID         string                 `json:"org_id"`
	HostID        string                 `json:"host_id"`
	Hostname      string                 `json:"hostname,omitempty"`
	MachineIDHash string                 `json:"machine_id_hash,omitempty"`
	AgentVersion  string                 `json:"agent_version,omitempty"`
	Capabilities  map[string]any         `json:"capabilities,omitempty"`
	Platform      map[string]any         `json:"platform,omitempty"`
	Network       map[string]any         `json:"network,omitempty"`
	Labels        []string               `json:"labels"`
	Note          string                 `json:"note,omitempty"`
	Created       time.Time              `json:"created"`
	LastSeen      time.Time              `json:"last_seen"`
}

// LabelsUpdateRequest represents a request to update agent labels
type LabelsUpdateRequest struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

// NoteUpdateRequest represents a request to update agent note
type NoteUpdateRequest struct {
	Note string `json:"note"`
}

// getAgents handles GET /agents with filtering support
func (s *Server) getAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse query parameters for filtering
	labelFilter := r.URL.Query().Get("label")
	hostnameFilter := r.URL.Query().Get("hostname")
	hostIDFilter := r.URL.Query().Get("host_id")
	ipFilter := r.URL.Query().Get("ip")

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	var filteredAgents []AgentInfo
	for _, agent := range s.store.agents {
		// Apply filters
		if labelFilter != "" {
			if !agent.Labels[labelFilter] {
				continue
			}
		}
		if hostnameFilter != "" {
			if agent.Hostname != hostnameFilter {
				continue
			}
		}
		if hostIDFilter != "" {
			if agent.HostID != hostIDFilter {
				continue
			}
		}
		if ipFilter != "" {
			if !s.agentHasIP(agent, ipFilter) {
				continue
			}
		}

		// Convert labels map to slice
		labels := make([]string, 0, len(agent.Labels))
		for label := range agent.Labels {
			labels = append(labels, label)
		}

		agentInfo := AgentInfo{
			AgentUID:      agent.AgentUID,
			OrgID:         agent.OrgID,
			HostID:        agent.HostID,
			Hostname:      agent.Hostname,
			MachineIDHash: agent.MachineIDHash,
			AgentVersion:  agent.AgentVersion,
			Platform:      agent.Platform,
			Network:       agent.Network,
			Labels:        labels,
			Note:          agent.Note,
			Created:       agent.Created,
			LastSeen:      agent.LastSeen,
		}
		filteredAgents = append(filteredAgents, agentInfo)
	}

	response := AgentListResponse{
		Agents: filteredAgents,
		Total:  len(filteredAgents),
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// agentDispatch handles sub-routes for individual agents
func (s *Server) agentDispatch(w http.ResponseWriter, r *http.Request) {
	// Extract agent UID from path: /agents/{uid} or /agents/{uid}/labels or /agents/{uid}/note
	path := strings.TrimPrefix(r.URL.Path, "/agents/")
	parts := strings.Split(path, "/")
	
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Agent UID required", http.StatusBadRequest)
		return
	}
	
	agentUID := parts[0]
	
	// Route to appropriate handler based on path
	if len(parts) == 1 {
		// /agents/{uid}
		s.getAgent(w, r, agentUID)
	} else if len(parts) == 2 {
		switch parts[1] {
		case "labels":
			// /agents/{uid}/labels
			s.updateAgentLabels(w, r, agentUID)
		case "note":
			// /agents/{uid}/note
			s.updateAgentNote(w, r, agentUID)
		default:
			http.Error(w, "Invalid endpoint", http.StatusNotFound)
		}
	} else {
		http.Error(w, "Invalid path", http.StatusNotFound)
	}
}

// getAgent handles GET /agents/{agent_uid}
func (s *Server) getAgent(w http.ResponseWriter, r *http.Request, agentUID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.store.mu.Lock()
	agent, exists := s.store.agents[agentUID]
	s.store.mu.Unlock()

	if !exists {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Convert labels map to slice
	labels := make([]string, 0, len(agent.Labels))
	for label := range agent.Labels {
		labels = append(labels, label)
	}

	response := AgentDetailResponse{
		AgentUID:      agent.AgentUID,
		OrgID:         agent.OrgID,
		HostID:        agent.HostID,
		Hostname:      agent.Hostname,
		MachineIDHash: agent.MachineIDHash,
		AgentVersion:  agent.AgentVersion,
		Capabilities:  agent.Capabilities,
		Platform:      agent.Platform,
		Network:       agent.Network,
		Labels:        labels,
		Note:          agent.Note,
		Created:       agent.Created,
		LastSeen:      agent.LastSeen,
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updateAgentLabels handles PUT /agents/{agent_uid}/labels
func (s *Server) updateAgentLabels(w http.ResponseWriter, r *http.Request, agentUID string) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LabelsUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	agent, exists := s.store.agents[agentUID]
	if !exists {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	// Add labels
	for _, label := range req.Add {
		if label != "" {
			agent.Labels[label] = true
		}
	}

	// Remove labels
	for _, label := range req.Remove {
		if label != "" {
			delete(agent.Labels, label)
		}
	}

	// Convert labels map to slice for response
	labels := make([]string, 0, len(agent.Labels))
	for label := range agent.Labels {
		labels = append(labels, label)
	}

	response := map[string]interface{}{
		"agent_uid": agent.AgentUID,
		"labels":    labels,
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// updateAgentNote handles PUT /agents/{agent_uid}/note
func (s *Server) updateAgentNote(w http.ResponseWriter, r *http.Request, agentUID string) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req NoteUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	s.store.mu.Lock()
	defer s.store.mu.Unlock()

	agent, exists := s.store.agents[agentUID]
	if !exists {
		http.Error(w, "Agent not found", http.StatusNotFound)
		return
	}

	agent.Note = req.Note

	response := map[string]interface{}{
		"agent_uid": agent.AgentUID,
		"note":      agent.Note,
	}

	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// agentHasIP checks if an agent has the specified IP in its network configuration
func (s *Server) agentHasIP(agent *Agent, ip string) bool {
	if agent.Network == nil {
		return false
	}

	// Check various network fields that might contain IP addresses
	// This is a simple implementation - could be enhanced based on actual network structure
	for _, value := range agent.Network {
		switch v := value.(type) {
		case string:
			if v == ip {
				return true
			}
		case []interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok && str == ip {
					return true
				}
			}
		case map[string]interface{}:
			for _, item := range v {
				if str, ok := item.(string); ok && str == ip {
					return true
				}
			}
		}
	}

	return false
}

