package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStaticServer(t *testing.T) http.Handler {
	t.Helper()
	dir := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<!doctype html><title>shell</title>")
	write("sw.js", "self.addEventListener('install', () => self.skipWaiting());")
	write("assets/index-abc123.js", "console.log(1)")
	write("favicon.svg", "<svg xmlns='http://www.w3.org/2000/svg'/>")

	var buf bytes.Buffer
	srv := newOpsServer(t, &buf)
	srv.MountStatic(dir)
	return srv.Handler()
}

// The legacy app registered /sw.js; browsers re-fetch it to update that
// registration. It must arrive as JavaScript (an HTML fallback is rejected and
// the legacy worker lives on) and must never be stored by a cache.
func TestStaticServiceWorkerScriptIsJavaScriptAndNotStored(t *testing.T) {
	h := newStaticServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/sw.js", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/sw.js = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Fatalf("/sw.js Content-Type = %q, want text/javascript", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("/sw.js Cache-Control = %q, want no-store", cc)
	}
	if !strings.Contains(rec.Body.String(), "skipWaiting") {
		t.Fatalf("/sw.js body is not the script: %q", rec.Body.String())
	}
}

func TestStaticCacheClasses(t *testing.T) {
	h := newStaticServer(t)
	cases := []struct {
		path, cache, typ string
	}{
		{"/assets/index-abc123.js", "public, max-age=31536000, immutable", "text/javascript"},
		{"/favicon.svg", "public, max-age=0, must-revalidate", "image/svg+xml"},
		// Client-side routes and legacy PWA files v2 does not ship fall back to the shell.
		{"/pos", "no-cache", "text/html"},
		{"/offline.html", "no-cache", "text/html"},
		{"/manifest.webmanifest", "no-cache", "text/html"},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", c.path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d", c.path, rec.Code)
		}
		if cc := rec.Header().Get("Cache-Control"); cc != c.cache {
			t.Errorf("%s Cache-Control = %q, want %q", c.path, cc, c.cache)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, c.typ) {
			t.Errorf("%s Content-Type = %q, want %s", c.path, ct, c.typ)
		}
	}
}
