package reporting

import (
	"net/http"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/reporting/application"
	"mini-erp/internal/modules/reporting/contracts"
	"mini-erp/internal/modules/reporting/presentation"
)

// Module wires the reporting bounded context (read-only, no tables).
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
}

// NewModule builds reporting over the finance contract.
func NewModule(finance financecontracts.FinanceClient) *Module {
	svc := application.NewService(finance)
	return &Module{svc: svc, handler: presentation.NewHandler(svc)}
}

// Reporting exposes the public contract to other modules.
func (m *Module) Reporting() contracts.ReportingClient {
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
