package stock

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	companycontracts "mini-erp/internal/modules/company/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/stock/application"
	"mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/modules/stock/infrastructure"
	"mini-erp/internal/modules/stock/presentation"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Module wires the stock bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds stock over its own tables plus provider contracts.
func NewModule(
	db *sql.DB,
	products productcontracts.ProductClient,
	branches branchcontracts.BranchClient,
	users usercontracts.UserClient,
	roles usercontracts.RoleClient,
	company companycontracts.CompanyClient,
	audit auditcontracts.AuditClient,
) *Module {
	svc := application.NewService(
		infrastructure.NewRepository(db), products, branches, users, roles, company, audit,
	)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Stock exposes the public contract to other modules.
func (m *Module) Stock() contracts.StockClient {
	return m.svc
}

// ProductStockState exposes the stock facts product guards need, through
// product's own contract (stock depends on product, never the reverse).
func (m *Module) ProductStockState() productcontracts.StockStateProvider {
	return m.svc
}

// Health exposes the read-only data-health check (ADR-0025).
func (m *Module) Health() contracts.HealthClient { return m.svc }

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm authcontracts.Permission) {
	presentation.RegisterRoutes(mux, m.handler, perm)
}

// Verify reports late-bound dependencies that bootstrap.Wire has not set, so a
// forgotten edge fails at startup instead of at the first request.
func (m *Module) Verify() error {
	return m.svc.Verify()
}
