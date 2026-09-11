package contracts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
)

// ErrSessionInvalid marks every expected authentication failure: bad token,
// unknown/revoked/expired session, inactive user, lost roles. Middleware maps
// exactly this to 401 — any other error is infrastructure trouble and becomes
// a logged 500, never a silent logout.
var ErrSessionInvalid = errors.New("session invalid")

// Session is the authenticated request scope. BranchID 0 means no branch
// selected yet (multi-branch user pending selection). There is deliberately
// no company/tenant field (ADR-0009).
type Session struct {
	ID       int64
	UserID   int64
	BranchID int64
	// BranchActive is false when the branch was closed after login.
	// Business routes deny it; switch-branch stays open as the way out.
	BranchActive bool
	RoleID       int64
	RoleCode     string
	Permissions  []string
}

// HasBranch reports whether a branch is selected.
func (s *Session) HasBranch() bool {
	return s != nil && s.BranchID != 0
}

// HasPermission checks one permission code.
func (s *Session) HasPermission(perm string) bool {
	if s == nil {
		return false
	}
	for _, p := range s.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

type ctxKey struct{}

// WithSession stores the session in the request context.
func WithSession(ctx context.Context, s *Session) context.Context {
	return context.WithValue(ctx, ctxKey{}, s)
}

// SessionFromContext returns the session or nil outside authenticated routes.
func SessionFromContext(ctx context.Context) *Session {
	if s, ok := ctx.Value(ctxKey{}).(*Session); ok {
		return s
	}
	return nil
}

// SessionProvider verifies access tokens. Implemented by the auth module,
// consumed by middleware.
type SessionProvider interface {
	Verify(ctx context.Context, accessToken string) (*Session, error)
}

// PermissionChecker answers one permission for the session in context.
// Permissions resolve from the database on every call, so revocations apply
// without forcing logout.
type PermissionChecker interface {
	HasPermission(ctx context.Context, perm string) bool
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"success":false,"message":"Sesi berakhir, silakan login kembali"}`))
}

func forbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"success":false,"message":"Anda tidak memiliki akses"}`))
}

func serverError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte(`{"success":false,"message":"Terjadi kesalahan internal"}`))
}

// Authenticate rejects requests without a valid Bearer session. It is the
// outer wrapper: every guarded route authenticates first.
func Authenticate(log *slog.Logger, provider SessionProvider, next http.Handler) http.Handler {
	if provider == nil {
		panic("auth: nil SessionProvider")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(token) <= len(prefix) || token[:len(prefix)] != prefix {
			unauthorized(w)
			return
		}
		s, err := provider.Verify(r.Context(), token[len(prefix):])
		if err != nil {
			if errors.Is(err, ErrSessionInvalid) {
				unauthorized(w)
				return
			}
			log.Error("session verify", "error", err, "path", r.URL.Path)
			serverError(w)
			return
		}
		if s == nil {
			unauthorized(w)
			return
		}
		next.ServeHTTP(w, r.WithContext(WithSession(r.Context(), s)))
	})
}

// RequirePermission denies requests lacking perm (fail-closed). It must sit
// inside Authenticate — without a session every request is denied.
func RequirePermission(checker PermissionChecker, perm string, next http.Handler) http.Handler {
	if perm == "" {
		panic("auth: empty permission — undeclared routes are denied by default")
	}
	if checker == nil {
		panic("auth: nil PermissionChecker")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !checker.HasPermission(r.Context(), perm) {
			forbidden(w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireBranch denies requests with no branch selected (the branch guard:
// most business endpoints need an active branch, login/selection do not).
// A branch closed after login is denied too — switch-branch remains the way out.
func RequireBranch(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := SessionFromContext(r.Context())
		if s == nil || !s.HasBranch() {
			deny("Pilih cabang aktif terlebih dahulu", w)
			return
		}
		if !s.BranchActive {
			deny("Cabang sudah tidak aktif", w)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func deny(message string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_, _ = w.Write([]byte(`{"success":false,"message":"` + message + `"}`))
}
