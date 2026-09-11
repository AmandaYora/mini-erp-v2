package product

import (
	"database/sql"
	"net/http"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/modules/product/application"
	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/infrastructure"
	"mini-erp/internal/modules/product/presentation"
)

// Module wires the product bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds the product module over its own tables.
func NewModule(db *sql.DB, audit auditcontracts.AuditClient) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), audit)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Products exposes the public contract to other modules.
func (m *Module) Products() contracts.ProductClient {
	return m.svc
}

// Service exposes use cases to the dev seed (composition root privilege).
func (m *Module) Service() *application.Service {
	return m.svc
}

// RegisterRoutes mounts endpoints with the auth-built permission wrapper
// and the media contract client.
func (m *Module) RegisterRoutes(mux *http.ServeMux, media mediacontracts.MediaClient, perm presentation.Permission) {
	presentation.RegisterRoutes(mux, m.handler, media, perm)
}
