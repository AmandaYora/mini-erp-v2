package dashboard

import (
	"database/sql"
	"log/slog"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/dashboard/application"
	"mini-erp/internal/modules/dashboard/infrastructure"
	"mini-erp/internal/modules/dashboard/presentation"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Module wires the dashboard bounded context: read-only widgets plus the
// data-health alerts, whose snoozes are the module's only table (ADR-0025).
type Module struct {
	svc           *application.Service
	health        *application.Health
	handler       *presentation.Handler
	healthHandler *presentation.HealthHandler
}

// NewModule builds the dashboard over module contracts (F1): finance for the
// booked snapshot, stock/sales/purchasing for the live operational widgets,
// and the owning modules' read-only checks for the data-health alerts.
// Contracts-only — arch-check Rule A must stay green.
func NewModule(db *sql.DB, finance financecontracts.FinanceClient, stock stockcontracts.StockClient, sales application.Sales, purchasing application.Purchasing,
	returns application.Returns, payments application.Payments, sources application.HealthSources,
	audit auditcontracts.AuditClient, log *slog.Logger) *Module {
	svc := application.NewService(finance, stock, sales, purchasing)
	svc.SetSources(returns, payments)
	health := application.NewHealth(svc, sources, infrastructure.NewRepository(db), audit, log)
	return &Module{svc: svc, health: health, handler: presentation.NewHandler(svc),
		healthHandler: presentation.NewHealthHandler(health)}
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// Health exposes the data-health composer (tests, composition root).
func (m *Module) Health() *application.Health {
	return m.health
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, branchGuard)
	presentation.RegisterHealthRoutes(mux, m.healthHandler, perm, branchGuard)
}

// Verify reports late-bound dependencies that bootstrap.Wire has not set, so a
// forgotten edge fails at startup instead of at the first request.
func (m *Module) Verify() error {
	if err := m.svc.Verify(); err != nil {
		return err
	}
	return m.health.Verify()
}
