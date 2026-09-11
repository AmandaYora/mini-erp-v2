package party

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/party/application"
	"mini-erp/internal/modules/party/contracts"
	"mini-erp/internal/modules/party/infrastructure"
	"mini-erp/internal/modules/party/presentation"
	productcontracts "mini-erp/internal/modules/product/contracts"
)

// Module wires the party bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds the party module over its own tables plus the product
// contract (member pricing quotes product prices through it).
func NewModule(db *sql.DB, products productcontracts.ProductClient, audit auditcontracts.AuditClient) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), products, audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Parties exposes the public contract to other modules.
func (m *Module) Parties() contracts.PartyClient {
	return m.svc
}

// Pricing exposes member-price quotes to other modules.
func (m *Module) Pricing() contracts.PricingClient {
	return m.svc
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper.
func (m *Module) RegisterRoutes(mux *http.ServeMux, perm presentation.Permission) {
	presentation.RegisterRoutes(mux, m.handler, perm)
}
