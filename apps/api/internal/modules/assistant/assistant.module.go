package assistant

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"mini-erp/internal/modules/assistant/application"
	"mini-erp/internal/modules/assistant/contracts"
	"mini-erp/internal/modules/assistant/infrastructure"
	"mini-erp/internal/modules/assistant/presentation"
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
// contracts. The WA session file lives under storageDir.
func NewModule(
	db *sql.DB,
	storageDir string,
	branches branchcontracts.BranchClient,
	products productcontracts.ProductClient,
	purchasing purchasingcontracts.PurchaseOrderClient,
	sales salescontracts.SalesOrderClient,
	stock stockcontracts.StockClient,
	finance financecontracts.FinanceClient,
	log *slog.Logger,
) *Module {
	svc := application.NewService(
		infrastructure.NewRepository(db),
		infrastructure.NewGateway(storageDir, log),
		branches, products, purchasing, sales, stock, finance, log,
	)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Assistant exposes the public contract to other modules.
func (m *Module) Assistant() contracts.AssistantClient {
	return m.svc
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

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm presentation.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, branchGuard)
}
