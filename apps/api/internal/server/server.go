package server

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"mini-erp/internal/config"
	"mini-erp/internal/shared/clientip"
	"mini-erp/internal/shared/response"
	"mini-erp/internal/shared/signedurl"
)

// Server wires routes and middleware. Module routes mount here as modules
// are implemented; cross-cutting concerns live in middleware.go.
type Server struct {
	mux     *http.ServeMux
	log     *slog.Logger
	cfg     *config.Config
	trusted []*net.IPNet
	ready   func(ctx context.Context) error
	readyMu sync.Mutex
	readyOK bool
	readyAt time.Time
	report  ErrorReporter
}

// New builds the server with middleware applied.
func New(cfg *config.Config, log *slog.Logger) *Server {
	trusted, _ := clientip.ParseTrusted(cfg.TrustedProxies)
	s := &Server{mux: http.NewServeMux(), log: log, cfg: cfg, trusted: trusted}
	s.mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		response.OK(w, "ok", nil)
	})
	s.mux.HandleFunc("/api/v1/health/ready", s.handleReady)
	// Unversioned probe aliases for proxies/orchestrators (F6-4): /healthz is
	// pure liveness, /readyz checks the database through the readiness probe.
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		response.OK(w, "ok", nil)
	})
	s.mux.HandleFunc("/readyz", s.handleReady)
	s.mux.Handle("/api/v1/metrics", MetricsHandler(cfg.MetricsToken))
	return s
}

// SetErrorReporter installs the sink for panics and unexpected 5xx responses
// (OP-06); the composition root points it at the audit trail.
func (s *Server) SetErrorReporter(fn ErrorReporter) { s.report = fn }

// SetReady installs the readiness probe (DB ping + writable dirs + migration
// version). Result caches 30s; failure flips to 503 (F6-4).
func (s *Server) SetReady(fn func(ctx context.Context) error) { s.ready = fn }

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	s.readyMu.Lock()
	cached := s.readyOK && time.Since(s.readyAt) < 30*time.Second
	s.readyMu.Unlock()
	if cached {
		response.OK(w, "ready", nil)
		return
	}
	if s.ready == nil {
		response.OK(w, "ready", nil)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.ready(ctx); err != nil {
		s.log.Warn("readiness failed", "error", err)
		response.Unavailable(w, "Layanan belum siap")
		return
	}
	s.readyMu.Lock()
	s.readyOK = true
	s.readyAt = time.Now()
	s.readyMu.Unlock()
	response.OK(w, "ready", nil)
}

// MarkNotReady flips readiness to failing (called on shutdown, F6-6).
func (s *Server) MarkNotReady() {
	s.readyMu.Lock()
	s.readyOK = false
	s.readyAt = time.Time{}
	s.readyMu.Unlock()
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
			setStaticCache(w, r.URL.Path)
			files.ServeHTTP(w, r)
			return
		}
		// The SPA shell: never cached, because it is what points at the
		// current asset hashes. Caching it would pin a browser to an old
		// deploy's script tags.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	}))
}

// buildAssetPath is the directory Vite emits content-hashed files into.
const buildAssetPath = "/assets/"

// setStaticCache marks build output immutable and everything else revalidated.
//
// Vite puts a content hash in every filename under /assets/, so those URLs
// can never change meaning: a new build produces new names. They are the one
// class of response a browser may keep for a year without asking. Anything
// else served from the public directory (favicon, manifest, images dropped
// in by hand) keeps its name across deploys, so it gets revalidation instead
// — correctness first, and a 304 is still nearly free.
func setStaticCache(w http.ResponseWriter, urlPath string) {
	if strings.HasPrefix(urlPath, buildAssetPath) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=0, must-revalidate")
}

// Mux exposes the router for module registration by the composition root.
func (s *Server) Mux() *http.ServeMux {
	return s.mux
}

// MountMedia serves stored files (product photos, transfer proofs, delivery
// proofs) under /media/*, and only from dir — the media folder, never the
// storage root, so secrets kept beside it (the WhatsApp session) stay private.
//
// Access needs a valid, unexpired signature in the query (exp, sig): an <img>
// cannot send an Authorization header, so the URL carries the proof. There is
// no directory listing and no way to name anything outside dir.
func (s *Server) MountMedia(dir string, signer *signedurl.Signer) {
	if dir == "" || signer == nil {
		return
	}
	// The old public prefix is gone; answer it explicitly so it never falls
	// through to the SPA catch-all and look like a working URL.
	s.mux.HandleFunc("/uploads/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	s.mux.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		serveMedia(w, r, dir, signer)
	})
}

// mediaContentTypes is the only set of types ever served; everything else is
// refused, so an uploaded file cannot be rendered as HTML or script.
var mediaContentTypes = map[string]string{
	".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
	".webp": "image/webp", ".pdf": "application/pdf",
}

func serveMedia(w http.ResponseWriter, r *http.Request, dir string, signer *signedurl.Signer) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/media/")
	// Listing and traversal are closed before anything touches the disk or the
	// signature: an empty path, a directory-style path, a dot segment, a
	// backslash or a NUL are all simply "not found".
	if rel == "" || strings.HasSuffix(rel, "/") || strings.ContainsAny(rel, "\\\x00") || path.Clean(rel) != rel || strings.HasPrefix(rel, "../") || rel == ".." {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	exp, err := strconv.ParseInt(q.Get("exp"), 10, 64)
	if err != nil {
		denyMedia(w)
		return
	}
	if err := signer.Verify(rel, exp, q.Get("sig")); err != nil {
		denyMedia(w)
		return
	}
	ctype, ok := mediaContentTypes[strings.ToLower(path.Ext(rel))]
	if !ok {
		http.NotFound(w, r)
		return
	}
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if within, err := filepath.Rel(dir, full); err != nil || strings.HasPrefix(within, "..") {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(full)
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer func() { _ = f.Close() }()
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Content-Disposition", "inline")
	// Cache only for as long as the URL itself remains valid (at most an hour):
	// a cached copy must not outlive the permission that produced it.
	maxAge := int(signer.Remaining(exp) / time.Second)
	if maxAge > 3600 {
		maxAge = 3600
	}
	w.Header().Set("Cache-Control", "private, max-age="+strconv.Itoa(maxAge))
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
}

func denyMedia(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	http.Error(w, "forbidden", http.StatusForbidden)
}

// Handler returns the fully wrapped mux.
func (s *Server) Handler() http.Handler {
	return TrackInFlight(chain(s.log, s.report, s.cfg.CORSOrigins, s.trusted, s.cfg.ClientIPHeader, s.mux))
}
