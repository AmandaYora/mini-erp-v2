package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mini-erp/internal/shared/response"
)

// apiFallbackServer mounts one guarded route the way modules do, and the SPA
// when withStatic is set (production) — dev runs without it.
func apiFallbackServer(t *testing.T, withStatic bool) http.Handler {
	t.Helper()
	var buf bytes.Buffer
	srv := newOpsServer(t, &buf)
	srv.Mux().HandleFunc("GET /api/v1/things/{id}", func(w http.ResponseWriter, r *http.Request) {
		response.SessionExpired(w)
	})
	srv.Mux().HandleFunc("PUT /api/v1/things/{id}", func(w http.ResponseWriter, r *http.Request) {
		response.SessionExpired(w)
	})
	if withStatic {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>shell</title>"), 0o644); err != nil {
			t.Fatal(err)
		}
		srv.MountStatic(dir)
	}
	return srv.Handler()
}

type failBody struct {
	Success *bool  `json:"success"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func decodeFail(t *testing.T, rec *httptest.ResponseRecorder) failBody {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json; body %q", ct, rec.Body.String())
	}
	var b failBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	if b.Success == nil || *b.Success {
		t.Fatalf("success = %v, want false", b.Success)
	}
	return b
}

// A removed or mistyped API route must fail in the envelope, never answer 200
// with the SPA shell: the client would read it as success and choke on HTML.
func TestUnknownAPIPathIsJSON404(t *testing.T) {
	for _, withStatic := range []bool{false, true} {
		h := apiFallbackServer(t, withStatic)
		for _, c := range []struct{ method, path string }{
			{"PUT", "/api/v1/tidak-ada"},
			{"GET", "/api/v1/tidak-ada"},
			{"PUT", "/api/v1/things/1/receipt-account"},
			{"POST", "/api/v2/things"},
			{"GET", "/api/"},
			{"GET", "/api"},
		} {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(c.method, c.path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("static=%v %s %s = %d, want 404; body %q", withStatic, c.method, c.path, rec.Code, rec.Body.String())
			}
			b := decodeFail(t, rec)
			if b.Message != "Endpoint tidak ditemukan" || b.Code != "not_found" {
				t.Fatalf("static=%v %s %s body = %+v", withStatic, c.method, c.path, b)
			}
		}
	}
}

// The catch-all matches every method, so it must still tell "wrong method"
// apart from "no such path" and name the methods that do work.
func TestKnownAPIPathWrongMethodIsJSON405(t *testing.T) {
	for _, withStatic := range []bool{false, true} {
		h := apiFallbackServer(t, withStatic)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/v1/things/7", nil))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("static=%v DELETE = %d, want 405; body %q", withStatic, rec.Code, rec.Body.String())
		}
		if got := rec.Header().Get("Allow"); got != "GET, HEAD, PUT" {
			t.Fatalf("static=%v Allow = %q", withStatic, got)
		}
		if b := decodeFail(t, rec); b.Code != "method_not_allowed" || b.Message == "" {
			t.Fatalf("static=%v body = %+v", withStatic, b)
		}
	}
}

func TestKnownAPIRouteStillAnswers(t *testing.T) {
	h := apiFallbackServer(t, true)
	for _, method := range []string{"GET", "PUT"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/api/v1/things/7", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s /api/v1/things/7 = %d, want 401", method, rec.Code)
		}
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/v1/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/v1/health = %d", rec.Code)
	}
}

// Client-side routes keep the shell, including ones that only look API-ish.
func TestSPARoutesStillServeShell(t *testing.T) {
	h := apiFallbackServer(t, true)
	for _, p := range []string{"/", "/sales-orders/12/edit", "/finance/reports/profit-loss", "/apidocs"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", p, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("%s Content-Type = %q", p, ct)
		}
		if !strings.Contains(rec.Body.String(), "<title>shell</title>") {
			t.Fatalf("%s body = %q", p, rec.Body.String())
		}
	}
}
