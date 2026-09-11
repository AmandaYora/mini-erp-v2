package goodsreceipt

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/goodsreceipt/application"
	"mini-erp/internal/modules/goodsreceipt/contracts"
	"mini-erp/internal/modules/goodsreceipt/infrastructure"
	"mini-erp/internal/modules/goodsreceipt/presentation"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Module wires the goodsreceipt bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds goodsreceipt over its own tables plus provider contracts.
func NewModule(
	db *sql.DB,
	purchasing purchasingcontracts.PurchaseOrderClient,
	products productcontracts.ProductClient,
	stock stockcontracts.StockClient,
	audit auditcontracts.AuditClient,
) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), purchasing, products, stock, audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// GoodsReceipts exposes the public contract to other modules.
func (m *Module) GoodsReceipts() contracts.GoodsReceiptClient {
	return m.svc
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm presentation.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, branchGuard)
}
