package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nats-io/nats.go"
)

type Server struct{ 
	mux *http.ServeMux 
	segMapsHandler *SegMapsHandler
}

func New(nc *nats.Conn) (*Server, error) {
	segMapsHandler, err := NewSegMapsHandler(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create seg maps handler: %w", err)
	}
	
	s := &Server{
		mux: http.NewServeMux(),
		segMapsHandler: segMapsHandler,
	}
	s.routes()
	return s, nil
}

func (s *Server) routes(){
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.Write([]byte("ok")) })
	s.mux.HandleFunc("/seg/maps", s.segMapsHandler.PostSegMapsHandler)
	s.mux.HandleFunc("/seg/maps/promote", s.promoteHandler)
	s.mux.HandleFunc("/seg/maps/rollback", s.rollbackHandler)
}

func (s *Server) Handler() http.Handler { return s.mux }

// promoteHandler handles promote requests for canary/enforce transitions
func (s *Server) promoteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// TODO: Implement promote logic
	// This is a stub as requested in the prompt
	response := map[string]interface{}{
		"status": "promote_stub",
		"message": "Promote endpoint - implementation pending",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// rollbackHandler handles rollback requests for canary/enforce transitions
func (s *Server) rollbackHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	// TODO: Implement rollback logic
	// This is a stub as requested in the prompt
	response := map[string]interface{}{
		"status": "rollback_stub",
		"message": "Rollback endpoint - implementation pending",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
