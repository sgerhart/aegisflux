package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct{
	r *chi.Mux
	store *Store
}

func NewServer()*Server{
	s := &Server{ r: chi.NewRouter(), store: NewStore() }
	
	// Add logging middleware
	s.r.Use(middleware.Logger)
	s.r.Use(middleware.RequestID)
	s.r.Use(middleware.Recoverer)
	
	s.routes()
	return s
}
func (s *Server) routes() {
	s.r.Get("/healthz", func(w http.ResponseWriter, r *http.Request){ w.Write([]byte("ok")) })

	// Artifacts assignments
	s.r.Get("/artifacts/for-host/{host_id}", s.getAssignments)

	// Bundles
	s.r.Get("/bundles/{artifact_id}", s.getBundle)
	s.r.Post("/bundles/{artifact_id}", s.putBundle)

	// Admin (dev-only)
	s.r.Post("/admin/assign", s.postAssign)
}
func (s *Server) Handler() http.Handler { return s.r }
