package api

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type RegisterInitReq struct {
	OrgID        string         `json:"org_id"`
	HostID       string         `json:"host_id"`
	AgentPubKey  string         `json:"agent_pubkey"` // base64
	MachineIDHash string        `json:"machine_id_hash,omitempty"`
	AgentVersion string         `json:"agent_version,omitempty"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
	Platform     map[string]any `json:"platform,omitempty"`
	Network      map[string]any `json:"network,omitempty"`
}

type RegisterInitResp struct {
	RegistrationID string `json:"registration_id"`
	Nonce          string `json:"nonce"`
	ServerTime     string `json:"server_time"`
}

func (s *Server) postRegisterInit(w http.ResponseWriter, r *http.Request){
	var req RegisterInitReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
	pub, err := base64.StdEncoding.DecodeString(req.AgentPubKey)
	if err != nil || len(pub) != ed25519.PublicKeySize { http.Error(w, "bad pubkey", 400); return }
	nonce := make([]byte, 32); io.ReadFull(rand.Reader, nonce)
	pend := &Pending{
		RegistrationID: uuid.NewString(),
		OrgID: req.OrgID, HostID: req.HostID,
		PubKey: pub, Nonce: nonce,
		Created: time.Now().UTC(),
		ServerTime: time.Now().UTC().Format(time.RFC3339),
		// Store richer metadata
		MachineIDHash: req.MachineIDHash,
		AgentVersion:  req.AgentVersion,
		Capabilities:  req.Capabilities,
		Platform:      req.Platform,
		Network:       req.Network,
	}
	s.store.mu.Lock(); s.store.pending[pend.RegistrationID] = pend; s.store.mu.Unlock()
	resp := RegisterInitResp{ RegistrationID: pend.RegistrationID, Nonce: base64.StdEncoding.EncodeToString(nonce), ServerTime: pend.ServerTime }
	w.Header().Set("content-type","application/json"); json.NewEncoder(w).Encode(resp)
}

type RegisterCompleteReq struct {
	RegistrationID string `json:"registration_id"`
	HostID         string `json:"host_id"`
	Signature      string `json:"signature"` // base64 over (nonce||server_time||host_id)
}
type RegisterCompleteResp struct {
	AgentUID       string `json:"agent_uid"`
	BootstrapToken string `json:"bootstrap_token"`
}

func (s *Server) postRegisterComplete(w http.ResponseWriter, r *http.Request){
	var req RegisterCompleteReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { http.Error(w, err.Error(), 400); return }
	s.store.mu.Lock(); pend := s.store.pending[req.RegistrationID]; s.store.mu.Unlock()
	if pend == nil { http.Error(w, "unknown registration", 404); return }
	msg := append(pend.Nonce, []byte(pend.ServerTime+req.HostID)...)
	sig, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil { http.Error(w, "bad signature", 400); return }
	if !ed25519.Verify(ed25519.PublicKey(pend.PubKey), msg, sig) {
		http.Error(w, "signature verify failed", 401); return
	}
	agent := &Agent{ 
		AgentUID: uuid.NewString(), 
		OrgID: pend.OrgID, 
		HostID: pend.HostID, 
		PubKey: pend.PubKey, 
		Created: time.Now().UTC(), 
		LastSeen: time.Now().UTC(),
		// Copy metadata from pending registration
		MachineIDHash: pend.MachineIDHash,
		AgentVersion:  pend.AgentVersion,
		Capabilities:  pend.Capabilities,
		Platform:      pend.Platform,
		Network:       pend.Network,
		Labels:        map[string]bool{}, // Initialize empty labels map
		Note:          "", // Initialize empty note
	}
	s.store.mu.Lock()
	delete(s.store.pending, pend.RegistrationID)
	s.store.agents[agent.AgentUID] = agent; s.store.byHost[agent.HostID] = agent
	s.store.mu.Unlock()
	resp := RegisterCompleteResp{ AgentUID: agent.AgentUID, BootstrapToken: "dev-"+uuid.NewString() }
	w.Header().Set("content-type","application/json"); json.NewEncoder(w).Encode(resp)
}
