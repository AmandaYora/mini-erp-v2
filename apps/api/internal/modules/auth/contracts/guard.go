package contracts

import "net/http"

// GuardedHandler is the handler returned by the auth module's Permission
// wrapper: authentication + RequirePermission around the route handler. It
// remembers the permission it enforces so TestRouteGuards can assert, for
// every registered route, that the guard exists and which code it demands
// (F3b-2 route registry). It adds no behaviour of its own.
type GuardedHandler struct {
	perm string
	next http.Handler
}

// NewGuardedHandler wraps next, recording perm as the required permission.
func NewGuardedHandler(perm string, next http.Handler) *GuardedHandler {
	return &GuardedHandler{perm: perm, next: next}
}

// RequiredPermission is the permission code the route demands.
func (g *GuardedHandler) RequiredPermission() string { return g.perm }

// ServeHTTP implements http.Handler.
func (g *GuardedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.next.ServeHTTP(w, r)
}

// PermissionGuarded is implemented by every handler built by Permission.
type PermissionGuarded interface {
	RequiredPermission() string
}
