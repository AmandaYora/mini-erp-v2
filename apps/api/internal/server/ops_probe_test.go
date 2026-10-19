package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mini-erp/internal/config"
)

func newOpsServer(t *testing.T, buf *bytes.Buffer) *Server {
	t.Helper()
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return New(&config.Config{MetricsToken: "tok"}, log)
}

func TestHealthzAndReadyz(t *testing.T) {
	var buf bytes.Buffer
	srv := newOpsServer(t, &buf)
	h := srv.Handler()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz = %d", rec.Code)
	}

	// Readiness follows the probe: failing database -> 503, no detail leaked.
	srv.SetReady(func(context.Context) error { return errors.New("dial tcp 10.0.0.5: refused") })
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("/readyz failing = %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Fatalf("readiness leaked detail: %s", rec.Body.String())
	}

	srv.SetReady(func(context.Context) error { return nil })
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/readyz ok = %d", rec.Code)
	}

	// Shutdown flips readiness off even though the last probe succeeded.
	srv.MarkNotReady()
	srv.SetReady(func(context.Context) error { return errors.New("closing") })
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/health/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready after MarkNotReady = %d", rec.Code)
	}
}

func TestRequestIDAndPanicStack(t *testing.T) {
	var buf bytes.Buffer
	srv := newOpsServer(t, &buf)
	srv.Mux().HandleFunc("/boom", func(http.ResponseWriter, *http.Request) { panic("kaboom") })
	h := srv.Handler()

	req := httptest.NewRequest("GET", "/boom", nil)
	req.Header.Set("X-Request-ID", "client-req-12345")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("panic status = %d", rec.Code)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "client-req-12345" {
		t.Fatalf("request id = %q", got)
	}
	logs := buf.String()
	if !strings.Contains(logs, "kaboom") || !strings.Contains(logs, "stack=") || !strings.Contains(logs, "client-req-12345") {
		t.Fatalf("panic log lacks stack/request id: %s", logs)
	}
	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Fatal("panic text leaked to the client")
	}

	// A malformed id is replaced, never echoed.
	req = httptest.NewRequest("GET", "/healthz", nil)
	req.Header.Set("X-Request-ID", "bad id\twith spaces")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Request-ID"); got == "" || strings.Contains(got, " ") {
		t.Fatalf("malformed id handling = %q", got)
	}
}

func TestMetricsEndpoint(t *testing.T) {
	var buf bytes.Buffer
	srv := newOpsServer(t, &buf)
	h := srv.Handler()

	// Generate one counted request, then scrape.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/health", nil))
	req := httptest.NewRequest("GET", "/api/v1/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("metrics without token = %d, want 404", rec.Code)
	}
	req.Header.Set("Authorization", "Bearer tok")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK || !strings.Contains(body, "go_goroutines") || !strings.Contains(body, "http_in_flight") {
		t.Fatalf("metrics = %d %s", rec.Code, body)
	}
}
