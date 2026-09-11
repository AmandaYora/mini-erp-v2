package payment

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/payment/application"
	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/payment/infrastructure"
	"mini-erp/internal/modules/payment/presentation"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
)

// Module wires the payment bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds payment over its own tables plus provider contracts.
func NewModule(
	db *sql.DB,
	parties partycontracts.PartyClient,
	purchasing purchasingcontracts.PurchaseOrderClient,
	sales salescontracts.SalesOrderClient,
	salesReturns salesreturncontracts.ReturnsClient,
	purchaseReturns purchasereturncontracts.ReturnsClient,
	branches branchcontracts.BranchClient,
	media mediacontracts.MediaClient,
	audit auditcontracts.AuditClient,
) *Module {
	svc := application.NewService(infrastructure.NewRepository(db),
		parties, purchasing, sales, salesReturns, purchaseReturns, branches, media, audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Payments exposes the public contract to other modules.
func (m *Module) Payments() contracts.PaymentClient {
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
