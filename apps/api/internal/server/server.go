package server

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"mini-erp/internal/config"
	"mini-erp/internal/shared/response"
)

// Server wires routes and middleware. Module routes mount here as modules
// are implemented; cross-cutting concerns live in middleware.go.
type Server struct {
	mux *http.ServeMux
	log *slog.Logger
	cfg *config.Config
}

// New builds the server with middleware applied.
func New(cfg *config.Config, log *slog.Logger) *Server {
	s := &Server{mux: http.NewServeMux(), log: log, cfg: cfg}
	s.mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		response.OK(w, "ok", nil)
	})
	return s
}

// MountStatic serves the built SPA from dir, falling back to index.html for
// client-side routes. Skipped silently when PUBLIC_DIR is empty or missing —
// the normal dev case, where Vite serves the frontend instead.
// http.ServeMux matches the longest pattern, so /api/v1/... still wins over /.
func (s *Server) MountStatic(dir string) {
	if dir == "" {
		return
	}
	if _, err := os.Stat(dir); err != nil {
		return
	}
	files := http.FileServer(http.Dir(dir))
	s.mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if info, err := os.Stat(filepath.Join(dir, filepath.Clean(r.URL.Path))); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	}))
}

// Mux exposes the router for module registration by the composition root.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// MountUploads serves stored files (product photos, transfer proofs,
// delivery proofs) under /uploads/*. Keys are generated server-side (uuid),
// never taken from request paths, so traversal is impossible by construction.
func (s *Server) MountUploads(dir string) {
	if dir == "" {
		return
	}
	if _, err := os.Stat(dir); err != nil {
		return
	}
	s.mux.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(dir))))
}

// Handler returns the fully wrapped mux.
func (s *Server) Handler() http.Handler {
	return chain(s.log, s.cfg.CORSOrigins, s.mux)
}
