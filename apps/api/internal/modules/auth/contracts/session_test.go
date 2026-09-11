package contracts

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireBranch(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	h := RequireBranch(next)

	cases := []struct {
		name   string
		sess   *Session
		status int
		body   string
	}{
		{"no session", nil, 403, "Pilih cabang aktif terlebih dahulu"},
		{"no branch", &Session{BranchID: 0, BranchActive: true}, 403, "Pilih cabang aktif terlebih dahulu"},
		{"inactive branch", &Session{BranchID: 7, BranchActive: false}, 403, "Cabang sudah tidak aktif"},
		{"active branch", &Session{BranchID: 7, BranchActive: true}, 418, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tc.sess != nil {
				req = req.WithContext(WithSession(req.Context(), tc.sess))
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d", rec.Code, tc.status)
			}
			if tc.body != "" && !contains(rec.Body.String(), tc.body) {
				t.Fatalf("body = %q, want substring %q", rec.Body.String(), tc.body)
			}
		})
	}
}

func TestSessionHasPermission(t *testing.T) {
	s := &Session{Permissions: []string{"users.view"}}
	if !s.HasPermission("users.view") {
		t.Fatal("expected permission to match")
	}
	if s.HasPermission("users.create") {
		t.Fatal("unexpected permission match")
	}
	var nilSess *Session
	if nilSess.HasPermission("x") {
		t.Fatal("nil session must deny")
	}
	if SessionFromContext(context.Background()) != nil {
		t.Fatal("empty context must yield nil session")
	}
}

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}

type stubProvider struct {
	sess *Session
	err  error
}

func (p stubProvider) Verify(_ context.Context, _ string) (*Session, error) {
	return p.sess, p.err
}

func TestAuthenticateMapsErrors(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	// Expected auth failure → 401 with the session message.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer bad")
	Authenticate(log, stubProvider{err: ErrSessionInvalid}, next).ServeHTTP(rec, req)
	if rec.Code != 401 || !contains(rec.Body.String(), "Sesi berakhir") {
		t.Fatalf("invalid session: got %d %q", rec.Code, rec.Body.String())
	}

	// Infrastructure trouble → logged 500, never a silent logout.
	rec = httptest.NewRecorder()
	Authenticate(log, stubProvider{err: errors.New("db gone")}, next).ServeHTTP(rec, req)
	if rec.Code != 500 || !contains(rec.Body.String(), "internal") {
		t.Fatalf("infra error: got %d %q", rec.Code, rec.Body.String())
	}

	// Valid session passes through with context populated.
	rec = httptest.NewRecorder()
	Authenticate(log, stubProvider{sess: &Session{UserID: 9}}, next).ServeHTTP(rec, req)
	if rec.Code != 204 {
		t.Fatalf("valid session: got %d", rec.Code)
	}
}
