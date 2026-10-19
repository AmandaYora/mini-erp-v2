package dashboard

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/dashboard/application"
	"mini-erp/internal/modules/dashboard/presentation"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Module wires the dashboard bounded context (read-only, no tables).
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds the dashboard over module contracts (F1): finance for the
// booked snapshot, stock/sales/purchasing for the live operational widgets.
// Contracts-only — arch-check Rule A must stay green.
func NewModule(finance financecontracts.FinanceClient, stock stockcontracts.StockClient, sales application.Sales, purchasing application.Purchasing,
	returns application.Returns, payments application.Payments) *Module {
	svc := application.NewService(finance, stock, sales, purchasing)
	svc.SetSources(returns, payments)
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
