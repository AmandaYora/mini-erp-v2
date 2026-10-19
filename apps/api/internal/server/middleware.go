package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"

	"mini-erp/internal/shared/clientip"
	"mini-erp/internal/shared/reqctx"
	"mini-erp/internal/shared/response"
)

// statusWriter captures the response status for request logs.
type statusWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

// Unwrap exposes the underlying writer so http.ResponseController works
// through the chain (F6-4: per-route deadline extension).
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// requestID accepts a client-supplied X-Request-ID when well-formed, else
// mints one. The ID is echoed back and stored in reqctx for logs.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		info := reqctx.From(r.Context())
		if info == nil {
			info = &reqctx.Info{}
			r = r.WithContext(reqctx.WithInfo(r.Context(), info))
		}
		info.ID = id
		next.ServeHTTP(w, r)
	})
}

// clientIP resolves the client address once per request (F1-3): the
// forwarding header is honoured only when the direct peer is trusted.
func clientIP(trusted []*net.IPNet, header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientip.Resolve(r, trusted, header)
			info := reqctx.From(r.Context())
			if info == nil {
				info = &reqctx.Info{}
				r = r.WithContext(reqctx.WithInfo(r.Context(), info))
			}
			info.IP = ip
			next.ServeHTTP(w, r)
		})
	}
}

// UnexpectedError describes one panic or 5xx response for the audit trail
// (OP-06). It carries no request body, header, query string or token — only
// what is needed to find the failing endpoint again.
type UnexpectedError struct {
	Route     string // registered mux pattern, or the path when none matched
	Status    int
	Panic     bool
	Message   string // panic value; empty for a plain 5xx
	RequestID string
	UserID    int64
	BranchID  int64
}

// ErrorReporter receives unexpected errors after the response is written.
// It must be cheap, bounded and never panic (the audit module dedupes and
// rate-limits behind it).
type ErrorReporter func(ctx context.Context, e UnexpectedError)

// reportable says whether a status is an unexpected server error: any 5xx
// except 503 (an intentional "not ready"/draining answer) and the probes.
func reportable(status int, path string) bool {
	if status < 500 || status == http.StatusServiceUnavailable {
		return false
	}
	return !(strings.HasPrefix(path, "/api/v1/health") || path == "/healthz" || path == "/readyz")
}

// recoverer converts panics into 500 JSON instead of dropping connections and
// reports panics and 5xx responses to report (nil = log only).
func recoverer(log *slog.Logger, report ErrorReporter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Seed the mutable request info here, outermost: inner middleware fill
		// the same pointer, so a panic log still sees the request id.
		if reqctx.From(r.Context()) == nil {
			r = r.WithContext(reqctx.WithInfo(r.Context(), &reqctx.Info{}))
		}
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			var panicMsg string
			panicked := false
			if rec := recover(); rec != nil {
				panicked = true
				panicMsg = fmt.Sprint(rec)
				id := ""
				if info := reqctx.From(r.Context()); info != nil {
					id = info.ID
				}
				log.Error("panic", "request_id", id, "path", r.URL.Path, "panic", rec, "stack", string(debug.Stack()))
				if id != "" {
					w.Header().Set("X-Request-ID", id)
				}
				response.Internal(sw)
			}
			if report == nil || !reportable(sw.status, r.URL.Path) {
				return
			}
			route := r.URL.Path
			if r.Pattern != "" {
				route = r.Pattern
			}
			e := UnexpectedError{Route: route, Status: sw.status, Panic: panicked, Message: panicMsg}
			if info := reqctx.From(r.Context()); info != nil {
				e.RequestID, e.UserID, e.BranchID = info.ID, info.UserID, info.BranchID
			}
			report(r.Context(), e)
		}()
		next.ServeHTTP(sw, r)
	})
}

// requestLog records one JSON line per request.
func requestLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/health") || r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
			next.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		route := r.URL.Path
		if r.Pattern != "" {
			route = r.Pattern
		}
		info := reqctx.From(r.Context())
		var reqID, ip string
		var uid, bid int64
		if info != nil {
			reqID, ip, uid, bid = info.ID, info.IP, info.UserID, info.BranchID
		}
		attrs := []any{
			"request_id", reqID,
			"ip", ip,
			"method", r.Method,
			"route", route,
			"path", redactPath(r.URL.Path),
			"status", sw.status,
			"bytes", sw.bytes,
			"duration_ms", time.Since(start).Milliseconds(),
			"user_id", uid,
			"branch_id", bid,
			"ua", r.UserAgent(),
		}
		switch {
		case sw.status >= 500:
			log.Error("request", attrs...)
		case sw.status == 401 || sw.status == 404:
			log.Info("request", attrs...)
		case sw.status >= 400:
			log.Warn("request", attrs...)
		default:
			log.Info("request", attrs...)
		}
		Observe(route, r.Method, sw.status, time.Since(start))
	})
}

func redactPath(p string) string {
	// Never log media signatures.
	if i := strings.Index(p, "?"); i >= 0 {
		p = p[:i]
	}
	return p
}

// cors allows only explicitly configured origins (same-origin by default,
// which needs no CORS at all). A wildcard origin is never accepted.
func cors(origins []string, next http.Handler) http.Handler {
	if len(origins) == 0 {
		return next
	}
	allowed := make(map[string]struct{}, len(origins))
	for _, o := range origins {
		if o != "*" {
			allowed[o] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if origin := r.Header.Get("Origin"); origin != "" {
			if _, ok := allowed[origin]; !ok {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Extend lengthens read/write deadlines for the current request (heavy
// upload/export routes). No-op when the underlying writer does not support it.
func Extend(w http.ResponseWriter, r *http.Request, read, write time.Duration) {
	rc := http.NewResponseController(w)
	if read > 0 {
		_ = rc.SetReadDeadline(time.Now().Add(read))
	}
	if write > 0 {
		_ = rc.SetWriteDeadline(time.Now().Add(write))
	}
	_ = r
}

// chain applies middleware outside-in: recoverer outermost, handler innermost.
//
// compress sits inside cors and outside the handler: it must see the real
// Content-Type the handler sets, and must not wrap the CORS preflight short
// circuit, which has no body to compress.
func chain(log *slog.Logger, report ErrorReporter, origins []string, trusted []*net.IPNet, ipHeader string, h http.Handler) http.Handler {
	inner := cors(origins, compress(h))
	withLog := requestLog(log, inner)
	withIP := clientIP(trusted, ipHeader)(withLog)
	withID := requestID(withIP)
	return recoverer(log, report, withID)
}
