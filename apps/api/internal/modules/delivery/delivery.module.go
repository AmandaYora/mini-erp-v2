package delivery

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/delivery/application"
	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure"
	"mini-erp/internal/modules/delivery/presentation"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Module wires the delivery bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds delivery over its own tables plus provider contracts.
func NewModule(
	db *sql.DB,
	sales application.Sales,
	products productcontracts.ProductClient,
	stock stockcontracts.StockClient,
	branches branchcontracts.BranchClient,
	media mediacontracts.MediaClient,
	audit auditcontracts.AuditClient,
) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), sales, products, stock, branches, media, audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Deliveries exposes the public contract to other modules.
func (m *Module) Deliveries() contracts.DeliveryClient {
	return m.svc
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, media mediacontracts.MediaClient, perm presentation.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, media, perm, branchGuard)
}
