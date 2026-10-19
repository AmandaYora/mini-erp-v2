package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
)

// RegisterRoutes mounts user endpoints. Every business route declares its
// permission explicitly.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission) {
	mux.Handle("GET /api/v1/users", perm("users.view", h.ListUsers))
	mux.Handle("POST /api/v1/users", perm("users.create", h.CreateUser))
	mux.Handle("GET /api/v1/users/{id}", perm("users.view", h.GetUser))
	mux.Handle("PUT /api/v1/users/{id}", perm("users.update", h.UpdateUser))
	mux.Handle("PATCH /api/v1/users/{id}/status", perm("users.archive", h.SetStatus))
	mux.Handle("POST /api/v1/users/{id}/password", perm("users.update", h.ChangePassword))

	mux.Handle("GET /api/v1/roles", perm("roles.view", h.ListRoles))
	mux.Handle("GET /api/v1/roles/{id}", perm("roles.view", h.GetRole))
	mux.Handle("POST /api/v1/roles", perm("roles.manage", h.CreateRole))
	mux.Handle("PUT /api/v1/roles/{id}", perm("roles.manage", h.UpdateRole))
	mux.Handle("DELETE /api/v1/roles/{id}", perm("roles.manage", h.DeleteRole))
	mux.Handle("PUT /api/v1/roles/{id}/permissions", perm("roles.manage", h.SetRolePermissions))
	mux.Handle("GET /api/v1/permissions", perm("roles.manage", h.ListPermissions))
}
