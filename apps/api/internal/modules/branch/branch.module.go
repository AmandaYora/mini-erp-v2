package branch

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/branch/application"
	"mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/branch/infrastructure"
	"mini-erp/internal/modules/branch/presentation"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Module wires the branch bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds the branch module over its own tables.
func NewModule(db *sql.DB, users usercontracts.UserClient, audit auditcontracts.AuditClient) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), users, audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Branches exposes the public contract to other modules.
func (m *Module) Branches() contracts.BranchClient {
	return m.svc
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
// my-access is login-only (no branches.view needed, no branch guard — it is
// what lets a branchless session pick a branch), so it takes the bare
// authn wrapper instead of perm.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm presentation.Permission, authn func(func(http.ResponseWriter, *http.Request)) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, authn)
}
