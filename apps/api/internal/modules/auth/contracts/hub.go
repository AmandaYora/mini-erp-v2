package contracts

import (
	"context"
	"errors"
	"sync"
)

// ErrHubNotAttached is returned when a session revocation is requested before
// auth attached itself to the hub: a wiring bug, surfaced instead of ignored.
var ErrHubNotAttached = errors.New("authz hub: session revoker not attached")

// AuthzHub breaks the construction-order cycle between auth and the modules
// whose writes auth must hear about (user, branch). The composition root
// creates it FIRST and hands it to user and branch as their
// AuthzInvalidator / SessionRevoker; auth, built last, subscribes itself.
// That replaces the former late-bound Set* calls with plain constructor
// arguments.
//
// An invalidation with no subscriber is a harmless no-op (staleness falls
// back to auth's short TTL); Verify reports whether auth ever attached.
type AuthzHub struct {
	mu      sync.RWMutex
	subs    []AuthzInvalidator
	revoker SessionRevoker
}

// NewAuthzHub creates an empty hub.
func NewAuthzHub() *AuthzHub { return &AuthzHub{} }

// Subscribe registers an invalidator that receives every announcement.
// A nil subscriber is ignored.
func (h *AuthzHub) Subscribe(i AuthzInvalidator) {
	if i == nil {
		return
	}
	h.mu.Lock()
	h.subs = append(h.subs, i)
	h.mu.Unlock()
}

// AttachRevoker registers the session revoker (auth).
func (h *AuthzHub) AttachRevoker(r SessionRevoker) {
	h.mu.Lock()
	h.revoker = r
	h.mu.Unlock()
}

// InvalidateAll implements AuthzInvalidator by fan-out.
func (h *AuthzHub) InvalidateAll() {
	for _, s := range h.snapshot() {
		s.InvalidateAll()
	}
}

// InvalidateUser implements AuthzInvalidator by fan-out.
func (h *AuthzHub) InvalidateUser(userID int64) {
	for _, s := range h.snapshot() {
		s.InvalidateUser(userID)
	}
}

// RevokeUserSessions implements SessionRevoker by delegating to the attached
// revoker.
func (h *AuthzHub) RevokeUserSessions(ctx context.Context, userID, exceptSessionID int64) (int64, error) {
	h.mu.RLock()
	r := h.revoker
	h.mu.RUnlock()
	if r == nil {
		return 0, ErrHubNotAttached
	}
	return r.RevokeUserSessions(ctx, userID, exceptSessionID)
}

// Verify fails unless auth subscribed and attached its revoker.
func (h *AuthzHub) Verify() error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.subs) == 0 {
		return errors.New("authz hub: no invalidator subscribed (auth not wired)")
	}
	if h.revoker == nil {
		return ErrHubNotAttached
	}
	return nil
}

func (h *AuthzHub) snapshot() []AuthzInvalidator {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return append([]AuthzInvalidator(nil), h.subs...)
}
