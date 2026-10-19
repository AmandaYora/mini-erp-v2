package product

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/modules/product/application"
	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/infrastructure"
	"mini-erp/internal/modules/product/presentation"
)

// Module wires the product bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds the product module over its own tables.
func NewModule(db *sql.DB, audit auditcontracts.AuditClient) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Products exposes the public contract to other modules.
func (m *Module) Products() contracts.ProductClient {
	return m.svc
}

// SetStockState wires the stock module's answers (late-bound: stock is built
// after product because it depends on product's contract).
func (m *Module) SetStockState(p contracts.StockStateProvider) { m.svc.SetStockState(p) }

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper
// and the media contract client.
func (m *Module) RegisterRoutes(mux *http.ServeMux, media mediacontracts.MediaClient, perm authcontracts.Permission) {
	presentation.RegisterRoutes(mux, m.handler, media, perm)
}

// Verify reports late-bound dependencies that bootstrap.Wire has not set, so a
// forgotten edge fails at startup instead of at the first request.
func (m *Module) Verify() error {
	return m.svc.Verify()
}
