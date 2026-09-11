package presentation

import (
	"net/http"

	"mini-erp/internal/modules/audit/application"
	"mini-erp/internal/modules/audit/contracts"
	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the audit module HTTP surface (read-only; writes arrive via
// the AuditClient from every mutating module).
type Handler struct {
	svc *application.Service
}

// NewHandler wires audit endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func branchID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.BranchID
	}
	return 0
}

func recordView(rec contracts.Record) map[string]any {
	return map[string]any{
		"id": rec.ID, "action": rec.Action, "entity": rec.Entity,
		"entityId": rec.EntityID, "branchId": rec.BranchID, "actorId": rec.ActorID,
		"note": rec.Note, "createdAt": rec.CreatedAt,
	}
}

// ListLogs handles GET /api/v1/audit-logs.
func (h *Handler) ListLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, total, err := h.svc.List(r.Context(), branchID(r), q.Get("action"), q.Get("entity"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res))
	for _, rec := range res {
		items = append(items, recordView(rec))
	}
	response.Paginated(w, "Data audit", items, response.Meta{
		Page: page, Limit: limit, Total: total,
		TotalPages: pagination.TotalPages(total, limit),
	})
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts audit endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/audit-logs", perm("audit.view", guarded(h.ListLogs, branchGuard)))
}
