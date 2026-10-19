package sales

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/sales/application"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure"
	"mini-erp/internal/modules/sales/presentation"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Module wires the sales bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds sales over its own tables plus provider contracts.
func NewModule(
	db *sql.DB,
	parties partycontracts.PartyClient,
	products productcontracts.ProductClient,
	pricing partycontracts.PricingClient,
	branches branchcontracts.BranchClient,
	audit auditcontracts.AuditClient,
	users usercontracts.UserClient,
) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), parties, products, pricing, branches, audit, users)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// SetDependents wires the modules built on top of an order (see the service).
func (m *Module) SetDependents(deliveries deliverycontracts.DeliveryClient, payments paymentcontracts.PaymentClient, returns salesreturncontracts.SalesReturnClient) {
	m.svc.SetDependents(deliveries, payments, returns)
}

// SalesOrders exposes the public contract to other modules.
func (m *Module) SalesOrders() contracts.SalesOrderClient {
	return m.svc
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
