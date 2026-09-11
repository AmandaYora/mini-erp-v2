package audit

import (
	"database/sql"
	"log/slog"
	"net/http"

	"mini-erp/internal/modules/audit/application"
	"mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/audit/infrastructure"
	"mini-erp/internal/modules/audit/presentation"
)

// Module wires the audit bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds the audit module over its own table.
func NewModule(db *sql.DB, log *slog.Logger) *Module {
	svc := application.NewService(infrastructure.NewRepository(db), log)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Audit exposes the public contract to other modules.
func (m *Module) Audit() contracts.AuditClient {
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
