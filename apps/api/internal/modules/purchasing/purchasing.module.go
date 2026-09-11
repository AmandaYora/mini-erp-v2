package purchasing

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/purchasing/application"
	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/purchasing/infrastructure"
	"mini-erp/internal/modules/purchasing/presentation"
)

// Module wires the purchasing bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds purchasing over its own tables plus provider contracts.
func NewModule(
	db *sql.DB,
	parties partycontracts.PartyClient,
	products productcontracts.ProductClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), parties, products, branches, audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// PurchaseOrders exposes the public contract to other modules.
func (m *Module) PurchaseOrders() contracts.PurchaseOrderClient {
	return m.svc
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
// branchGuard enforces an active selected branch on every business route.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm presentation.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, branchGuard)
}
