package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
)

// actorID resolves the acting user from the session context populated by the
// auth middleware. Importing auth/contracts (never auth internals) is the
// sanctioned cross-module direction.
func actorID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.UserID
	}
	return 0
}
