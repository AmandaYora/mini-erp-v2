package finance

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/finance/application"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/modules/finance/presentation"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
)

// Module wires the finance bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds finance over its own tables plus provider contracts.
// Finance is the top consumer: it reads every transactional module through
// contracts and owns the only books.
func NewModule(
	db *sql.DB,
	branches branchcontracts.BranchClient,
	products productcontracts.ProductClient,
	parties partycontracts.PartyClient,
	purchasing purchasingcontracts.PurchaseOrderClient,
	sales salescontracts.SalesOrderClient,
	salesReturns salesreturncontracts.SalesReturnClient,
	purchaseReturns purchasereturncontracts.PurchaseReturnClient,
	goodsreceipt goodsreceiptcontracts.GoodsReceiptClient,
	delivery deliverycontracts.DeliveryClient,
	payment paymentcontracts.PaymentClient,
	stock stockcontracts.StockClient,
	audit auditcontracts.AuditClient,
) *Module {
	svc := application.NewService(infrastructure.NewRepository(db),
		branches, products, parties, purchasing, sales, salesReturns, purchaseReturns,
		goodsreceipt, delivery, payment, stock, audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Finance exposes the public contract to other modules.
func (m *Module) Finance() contracts.FinanceClient {
	return m.svc
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// SetOps injects the operational order surfaces for the readiness gate
// (composition-root privilege).
func (m *Module) SetOps(sales salescontracts.SalesOpsClient, purch purchasingcontracts.PurchasingOpsClient) {
	m.svc.SetOps(sales, purch)
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm presentation.Permission, branchGuard func(http.Handler) http.Handler) {
	presentation.RegisterRoutes(mux, m.handler, perm, branchGuard)
}
