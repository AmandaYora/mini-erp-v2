package purchasereturn

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/purchasereturn/application"
	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/modules/purchasereturn/infrastructure"
	"mini-erp/internal/modules/purchasereturn/presentation"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Module wires the purchasereturn bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds purchasereturn over its own tables plus provider contracts.
func NewModule(
	db *sql.DB,
	purchasing purchasingcontracts.PurchaseOrderClient,
	goodsreceipt goodsreceiptcontracts.GoodsReceiptClient,
	products productcontracts.ProductClient,
	stock stockcontracts.StockClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
) *Module {
	svc := application.NewService(infrastructure.NewRepository(db),
		purchasing, goodsreceipt, products, stock, branches, audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// PurchaseReturns exposes the public contract to other modules.
func (m *Module) PurchaseReturns() contracts.PurchaseReturnClient {
	return m.svc
}

// Returns exposes the payment-facing projection to the payment module.
func (m *Module) Returns() contracts.ReturnsClient {
	return m.svc
}

// SetPayments injects the payment client (composition-root privilege).
func (m *Module) SetPayments(p paymentcontracts.PaymentClient) {
	m.svc.SetPayments(p)
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm presentation.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, branchGuard)
}
