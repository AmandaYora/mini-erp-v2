package auth

import (
	"database/sql"
	"log/slog"
	"net/http"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/auth/application"
	"mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/auth/infrastructure"
	"mini-erp/internal/modules/auth/presentation"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	companycontracts "mini-erp/internal/modules/company/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Module wires the auth bounded context.
type Module struct {
	svc     *application.Service
	handler *presentation.Handler
	log     *slog.Logger
}

// NewModule builds auth over its own tables plus provider contracts.
func NewModule(
	db *sql.DB,
	users usercontracts.UserClient,
	roles usercontracts.RoleClient,
	branches branchcontracts.BranchClient,
	company companycontracts.CompanyClient,
	jwtSecret string,
	accessTTL, refreshTTL time.Duration,
	log *slog.Logger,
	audit auditcontracts.AuditClient,
	hub *contracts.AuthzHub,
) *Module {
	svc := application.NewService(
		infrastructure.NewRepository(db), users, roles, branches, company,
		jwtSecret, accessTTL, refreshTTL, log, audit,
	)
	// user and branch announce their writes through the hub, which was handed
	// to them before auth existed; auth subscribes itself here.
	if hub != nil {
		hub.Subscribe(svc)
		hub.AttachRevoker(svc)
	}
	return &Module{svc: svc, handler: presentation.NewHandler(svc), log: log}
}

// Sessions exposes token verification to middleware composition.
func (m *Module) Sessions() contracts.SessionProvider {
	return m.svc
}

// Service exposes use cases to integration tests (composition root
// privilege, same as user.Module.Service).
func (m *Module) Service() *application.Service {
	return m.svc
}

// Invalidator exposes authorization-cache invalidation directly (tests). The
// running app reaches it through the AuthzHub that user and branch hold.
func (m *Module) Invalidator() contracts.AuthzInvalidator {
	return m.svc
}

// Permission builds the fail-closed guard: authentication is always applied
// inside, so guarded routes can never forget it.
func (m *Module) Permission(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler {
	return contracts.NewGuardedHandler(perm,
		contracts.Authenticate(m.log, m.svc, contracts.RequirePermission(m.svc, perm, http.HandlerFunc(next))))
}

// RegisterRoutes mounts auth endpoints.
func (m *Module) RegisterRoutes(mux *http.ServeMux) {
	presentation.RegisterRoutes(mux, m.handler, m.log)
}
