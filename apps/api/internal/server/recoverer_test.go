package server

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"mini-erp/internal/shared/reqctx"
)

func recovererFor(h http.HandlerFunc, got *[]UnexpectedError) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/boom/{id}", h)
	mux.HandleFunc("GET /readyz", h)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return recoverer(log, func(_ context.Context, e UnexpectedError) { *got = append(*got, e) }, mux)
}

// OP-06: a panic is answered with the 500 envelope and reported once, with the
// registered route (no ids) and nothing from the request.
func TestRecovererReportsPanic(t *testing.T) {
	var got []UnexpectedError
	h := recovererFor(func(w http.ResponseWriter, r *http.Request) {
		if info := reqctx.From(r.Context()); info != nil {
			info.UserID, info.BranchID = 7, 3
		}
		panic("kaboom")
	}, &got)
	req := httptest.NewRequest("GET", "/api/v1/boom/42?token=secret", nil)
	req.Header.Set("Authorization", "Bearer very-secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 500 || !strings.Contains(rec.Body.String(), `"success":false`) {
		t.Fatalf("response = %d %s", rec.Code, rec.Body.String())
	}
	if len(got) != 1 {
		t.Fatalf("reports = %d, want 1", len(got))
	}
	e := got[0]
	if !e.Panic || e.Status != 500 || e.Message != "kaboom" || e.UserID != 7 || e.BranchID != 3 {
		t.Fatalf("report = %+v", e)
	}
	if e.Route != "GET /api/v1/boom/{id}" {
		t.Fatalf("route = %q, want the registered pattern", e.Route)
	}
	if strings.Contains(e.Route+e.Message, "secret") {
		t.Fatalf("report leaked request data: %+v", e)
	}
}

// A handler-returned 500 is reported too; 4xx, 503 and the probes are not.
func TestRecovererReportsOnly5xx(t *testing.T) {
	cases := []struct {
		path   string
		status int
		want   int
	}{
		{"/api/v1/boom/1", 500, 1},
		{"/api/v1/boom/1", 502, 1},
		{"/api/v1/boom/1", 503, 0},
		{"/api/v1/boom/1", 404, 0},
		{"/api/v1/boom/1", 200, 0},
		{"/readyz", 500, 0},
	}
	for _, c := range cases {
		var got []UnexpectedError
		h := recovererFor(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(c.status) }, &got)
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", c.path, nil))
		if len(got) != c.want {
			t.Errorf("%s -> %d: reports = %d, want %d", c.path, c.status, len(got), c.want)
		}
		if c.want == 1 && got[0].Panic {
			t.Errorf("plain %d flagged as panic", c.status)
		}
	}
}
