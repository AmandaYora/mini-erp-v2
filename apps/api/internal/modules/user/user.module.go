package user

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
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
func NewModule(db *sql.DB, audit auditcontracts.AuditClient) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), audit)
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
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm presentation.Permission) {
	presentation.RegisterRoutes(mux, m.handler, perm)
}
