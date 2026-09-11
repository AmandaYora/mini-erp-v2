package presentation

import "net/http"

// Auth wraps a handler with session authentication. Permission wraps a
// handler with an explicit permission check. Both are injected by the
// composition root (built from the auth module) so this package never
// imports another module's internals.
//
// By construction perm() already includes auth(): authentication can never
// be forgotten on a guarded route, and undeclared routes do not exist.
// Permission guards a handler with an explicit permission code. It takes a
// plain handler func to keep call sites clean; the auth module adapts it to
// http.Handler inside its fail-closed wrapper (auth first, then permission).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// RegisterRoutes mounts user endpoints. Every business route declares its
// permission explicitly.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission) {
	mux.Handle("GET /api/v1/users", perm("users.view", h.ListUsers))
	mux.Handle("POST /api/v1/users", perm("users.create", h.CreateUser))
	mux.Handle("GET /api/v1/users/{id}", perm("users.view", h.GetUser))
	mux.Handle("PUT /api/v1/users/{id}", perm("users.update", h.UpdateUser))
	mux.Handle("PATCH /api/v1/users/{id}/status", perm("users.archive", h.SetStatus))
	mux.Handle("POST /api/v1/users/{id}/password", perm("users.update", h.ChangePassword))

	mux.Handle("GET /api/v1/roles", perm("roles.view", h.ListRoles))
	mux.Handle("POST /api/v1/roles", perm("roles.manage", h.CreateRole))
	mux.Handle("PUT /api/v1/roles/{id}", perm("roles.manage", h.UpdateRole))
	mux.Handle("DELETE /api/v1/roles/{id}", perm("roles.manage", h.DeleteRole))
	mux.Handle("PUT /api/v1/roles/{id}/permissions", perm("roles.manage", h.SetRolePermissions))
	mux.Handle("GET /api/v1/permissions", perm("roles.manage", h.ListPermissions))
}
