package api

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/go-chi/chi/v5"
)

type Assignment struct {
	ArtifactID string `json:"artifact_id"`
}
type Store struct {
	mu sync.Mutex
	assign map[string][]Assignment   // host_id -> assignments
	bundles map[string][]byte        // artifact_id -> bytes
}
func NewStore()*Store{
	return &Store{
		assign:  make(map[string][]Assignment),
		bundles: make(map[string][]byte),
	}
}

// GET /artifacts/for-host/{host_id}
func (s *Server) getAssignments(w http.ResponseWriter, r *http.Request){
	host := chi.URLParam(r, "host_id")
	log.Printf("[REGISTRY] GET /artifacts/for-host/%s from %s (User-Agent: %s)", host, r.RemoteAddr, r.UserAgent())
	s.store.mu.Lock(); list := s.store.assign[host]; s.store.mu.Unlock()
	writeJSON(w, list, 200)
}

// GET /bundles/{artifact_id}
func (s *Server) getBundle(w http.ResponseWriter, r *http.Request){
	id := chi.URLParam(r, "artifact_id")
	log.Printf("[REGISTRY] GET /bundles/%s from %s (User-Agent: %s)", id, r.RemoteAddr, r.UserAgent())
	s.store.mu.Lock(); b, ok := s.store.bundles[id]; s.store.mu.Unlock()
	if !ok { http.Error(w, "not found", 404); return }
	w.Header().Set("content-type", "application/octet-stream")
	w.WriteHeader(200); w.Write(b)
}

// POST /bundles/{artifact_id}  body: {"bytes_b64":"..."}
func (s *Server) putBundle(w http.ResponseWriter, r *http.Request){
	id := chi.URLParam(r, "artifact_id")
	var body struct{ BytesB64 string `json:"bytes_b64"` }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil { http.Error(w, err.Error(), 400); return }
	b, err := base64.StdEncoding.DecodeString(body.BytesB64)
	if err != nil { http.Error(w, "bad base64", 400); return }
	s.store.mu.Lock(); s.store.bundles[id] = b; s.store.mu.Unlock()
	writeJSON(w, map[string]any{"ok":true,"artifact_id":id,"size":len(b)}, 200)
}

// POST /admin/assign  body: {"host_id":"...","artifact_id":"..."}
func (s *Server) postAssign(w http.ResponseWriter, r *http.Request){
	var body struct{ 
		HostID string `json:"host_id"`
		ArtifactID string `json:"artifact_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil { http.Error(w, err.Error(), 400); return }
	if body.HostID == "" || body.ArtifactID == "" { http.Error(w, "host_id and artifact_id required", 400); return }
	s.store.mu.Lock()
	s.store.assign[body.HostID] = append(s.store.assign[body.HostID], Assignment{ArtifactID: body.ArtifactID})
	s.store.mu.Unlock()
	writeJSON(w, map[string]any{"ok":true,"host_id":body.HostID,"artifacts":s.store.assign[body.HostID]}, 200)
}

func writeJSON(w http.ResponseWriter, v any, code int){
	w.Header().Set("content-type","application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
