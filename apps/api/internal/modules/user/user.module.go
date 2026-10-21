package user

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/user/application"
	"mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/modules/user/infrastructure"
	"mini-erp/internal/modules/user/presentation"
)

// Module wires the user bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds the user module over its own tables.
// authz and revoker are the auth hub (created first by the composition root);
// nil means "no auth wired" (dev seed): invalidation is a no-op.
func NewModule(db *sql.DB, audit auditcontracts.AuditClient, authz authcontracts.AuthzInvalidator, revoker authcontracts.SessionRevoker) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), audit)
	svc.SetAuthzInvalidator(authz)
	svc.SetSessionRevoker(revoker)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Users exposes the public contract to other modules.
func (m *Module) Users() contracts.UserClient {
	return m.svc
}

// Roles exposes role/permission resolution to other modules.
func (m *Module) Roles() contracts.RoleClient {
	return m.svc
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints. The permission wrapper comes from the
// auth module via the composition root (already including authentication),
// so no cross-module internals leak.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm authcontracts.Permission) {
	presentation.RegisterRoutes(mux, m.handler, perm)
}
