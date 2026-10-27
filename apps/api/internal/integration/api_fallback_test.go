package integration

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Production serves the SPA from the same mux as the API. An API path no
// module registers — a typo or a removed route like the old receipt-account
// endpoint — must still fail in the JSON envelope, not return 200 HTML.
func TestUnknownAPIRouteBesideSPA(t *testing.T) {
	srv := routeServer(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><title>shell</title>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv.MountStatic(dir)
	h := srv.Handler()

	type failBody struct {
		Success *bool  `json:"success"`
		Message string `json:"message"`
		Code    string `json:"code"`
	}
	expectFail := func(method, path string, status int, code string) {
		t.Helper()
		rec := doReq(h, method, path, "", "")
		if rec.Code != status {
			t.Fatalf("%s %s = %d, want %d; body %q", method, path, rec.Code, status, rec.Body.String())
		}
		if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
			t.Fatalf("%s %s Content-Type = %q", method, path, ct)
		}
		var b failBody
		if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil || b.Success == nil || *b.Success || b.Code != code || b.Message == "" {
			t.Fatalf("%s %s body = %q", method, path, rec.Body.String())
		}
	}

	expectFail(http.MethodPut, "/api/v1/tidak-ada", http.StatusNotFound, "not_found")
	expectFail(http.MethodPut, "/api/v1/sales-orders/1/receipt-account", http.StatusNotFound, "not_found")
	expectFail(http.MethodDelete, "/api/v1/sales-orders", http.StatusMethodNotAllowed, "method_not_allowed")
	// Registered routes keep their own guard.
	expectFail(http.MethodGet, "/api/v1/sales-orders", http.StatusUnauthorized, "")
	expectFail(http.MethodPut, "/api/v1/sales-orders/1", http.StatusUnauthorized, "")

	for _, p := range []string{"/", "/sales-orders/1"} {
		rec := doReq(h, http.MethodGet, p, "", "")
		if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("GET %s = %d %q", p, rec.Code, rec.Header().Get("Content-Type"))
		}
	}
}
