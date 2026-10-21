package assistant

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"mini-erp/internal/modules/assistant/application"
	"mini-erp/internal/modules/assistant/infrastructure"
	"mini-erp/internal/modules/assistant/presentation"
	auditcontracts "mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Module wires the assistant bounded context (WhatsApp bot + tooling).
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds the assistant over its own tables plus provider
// contracts. The WA session file lives under sessionDir, a folder the API
// never serves (it is the credential of the linked number).
func NewModule(
	db *sql.DB,
	sessionDir string,
	branches branchcontracts.BranchClient,
	products productcontracts.ProductClient,
	purchasing purchasingcontracts.PurchaseOrderClient,
	sales salescontracts.SalesOrderClient,
	stock stockcontracts.StockClient,
	finance financecontracts.FinanceClient,
	log *slog.Logger,
	audit auditcontracts.AuditClient,
) *Module {
	svc := application.NewService(
		infrastructure.NewRepository(db),
		infrastructure.NewGateway(sessionDir, log),
		branches, products, purchasing, sales, stock, finance, log, audit,
	)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// Boot attempts a silent WhatsApp reconnect when a session was paired
// before. Best-effort: failures only log.
func (m *Module) Boot(ctx context.Context) {
	m.svc.Boot(ctx)
}

// SetContext binds the bot's long-lived work to the application lifecycle.
func (m *Module) SetContext(ctx context.Context) { m.svc.SetContext(ctx) }

// Close stops the assistant (waits for in-flight replies, closes the gateway).
func (m *Module) Close(ctx context.Context) { m.svc.Close(ctx) }

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, branchGuard)
}
