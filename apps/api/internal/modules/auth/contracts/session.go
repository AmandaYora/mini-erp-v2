package contracts

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"mini-erp/internal/shared/response"
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
// The session was resolved once by Authenticate; revocations apply without
// forcing logout because every write that changes authorization invalidates
// the resolved session through AuthzInvalidator.
type PermissionChecker interface {
	HasPermission(ctx context.Context, perm string) bool
}

// AuthzInvalidator retires authorization state that auth resolved and cached.
//
// Auth resolves a session once and reuses it across requests instead of
// paying six reads per request. That reuse is only safe while the modules
// that OWN the underlying rows announce their writes — a role's permissions,
// a user's status or roles, a branch closing. Those modules receive this
// contract from the composition root and call it after such a write, so the
// next request rebuilds from the database.
//
// Implemented by the auth module. A nil invalidator is a valid no-op: a
// module wired without one simply never announces, and staleness falls back
// to auth's short TTL.
type AuthzInvalidator interface {
	InvalidateAll()
	InvalidateUser(userID int64)
}

// SessionRevoker revokes sessions from the user module (F1-4).
type SessionRevoker interface {
	RevokeUserSessions(ctx context.Context, userID, exceptSessionID int64) (int64, error)
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
			response.SessionExpired(w)
			return
		}
		s, err := provider.Verify(r.Context(), token[len(prefix):])
		if err != nil {
			if errors.Is(err, ErrSessionInvalid) {
				response.SessionExpired(w)
				return
			}
			log.Error("session verify", "error", err, "path", r.URL.Path)
			response.Internal(w)
			return
		}
		if s == nil {
			response.SessionExpired(w)
			return
		}
		ctx := WithSession(r.Context(), s)
		next.ServeHTTP(w, r.WithContext(ctx))
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
			response.Forbidden(w)
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
	response.ForbiddenMsg(w, message)
}

// ActorID is the authenticated user id of the request, 0 when unauthenticated.
// Presentation layers use it instead of reading the body (ADR-0009 scope rule).
func ActorID(r *http.Request) int64 {
	if s := SessionFromContext(r.Context()); s != nil {
		return s.UserID
	}
	return 0
}

// BranchID is the session's active branch id, 0 when none is selected.
func BranchID(r *http.Request) int64 {
	if s := SessionFromContext(r.Context()); s != nil {
		return s.BranchID
	}
	return 0
}

// Permission is the route-permission wrapper injected by the composition
// root; every presentation package takes this one type.
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// Guard adapts a branch guard into func-style middleware, preserving the
// order auth -> permission -> branch -> handler.
func Guard(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}
