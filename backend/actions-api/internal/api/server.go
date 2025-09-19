package api

import "net/http"

type Server struct{ mux *http.ServeMux; store *Store }

func NewServer()*Server{
	s := &Server{mux: http.NewServeMux(), store: NewStore()}
	s.routes(); return s
}
func (s *Server) routes(){
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request){ w.Write([]byte("ok")) })
	
	// Agent registration endpoints
	s.mux.HandleFunc("/agents/register/init", s.postRegisterInit)
	s.mux.HandleFunc("/agents/register/complete", s.postRegisterComplete)
	
	// Agents API endpoints
	s.mux.HandleFunc("/agents", s.getAgents)
	s.mux.HandleFunc("/agents/", s.agentDispatch) // Subrouter emulation for /agents/{uid}/* paths
}
func (s *Server) Handler() http.Handler { return s.mux }
