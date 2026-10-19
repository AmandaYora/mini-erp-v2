package reporting

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/reporting/application"
	"mini-erp/internal/modules/reporting/presentation"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Module wires the reporting bounded context (read-only, no tables).
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds reporting over the finance contract plus the operational
// readers the order and stock reports use.
func NewModule(finance financecontracts.FinanceClient, sales salescontracts.SalesOpsClient, purchasing purchasingcontracts.PurchasingOpsClient, stock stockcontracts.StockClient) *Module {
	svc := application.NewService(finance)
	svc.SetSources(sales, purchasing, stock)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, branchGuard)
}

// Verify reports late-bound dependencies that bootstrap.Wire has not set, so a
// forgotten edge fails at startup instead of at the first request.
func (m *Module) Verify() error {
	return m.svc.Verify()
}
