package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/dashboard/application"
	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// HealthHandler serves the data-health alerts (ADR-0025): the dashboard card
// and the /data-health page with its snoozes.
type HealthHandler struct {
	h *application.Health
}

// NewHealthHandler wires the data-health endpoints.
func NewHealthHandler(h *application.Health) *HealthHandler {
	return &HealthHandler{h: h}
}

// viewerOf reads the session the rules need. Branch, user, role and
// permissions come from the auth session only, never from the request.
func viewerOf(r *http.Request) application.Viewer {
	sess := authcontracts.SessionFromContext(r.Context())
	if sess == nil {
		return application.Viewer{}
	}
	return application.Viewer{UserID: sess.UserID, RoleCode: sess.RoleCode,
		Security: sess.HasPermission(dashboardcontracts.PermAlertsSecurity)}
}

// maxItemsLimit bounds GET /data-health/{alertId}/items?limit=.
const maxItemsLimit = 100

// Alerts handles GET /api/v1/dashboard/alerts?limit=.
func (hh *HealthHandler) Alerts(w http.ResponseWriter, r *http.Request) {
	limit := pagination.ParseDisplayLimit(r.URL.Query().Get("limit"), 0, maxDisplayLimit)
	sum, err := hh.h.Alerts(r.Context(), authcontracts.BranchID(r), viewerOf(r), limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Alert kesehatan data", sum)
}

// DataHealth handles GET /api/v1/data-health?refresh=0|1.
func (hh *HealthHandler) DataHealth(w http.ResponseWriter, r *http.Request) {
	refresh := r.URL.Query().Get("refresh") == "1"
	out, err := hh.h.DataHealth(r.Context(), authcontracts.BranchID(r), viewerOf(r), refresh)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Kesehatan data", out)
}

// Items handles GET /api/v1/data-health/{alertId}/items?page=&limit=.
func (hh *HealthHandler) Items(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	if limit > maxItemsLimit {
		limit = maxItemsLimit
	}
	items, total, err := hh.h.Items(r.Context(), authcontracts.BranchID(r), viewerOf(r), r.PathValue("alertId"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Paginated(w, "Daftar alert", items, response.Meta{
		Page: page, Limit: limit, Total: total, TotalPages: pagination.TotalPages(total, limit),
	})
}

type snoozePayload struct {
	Days   int    `json:"days"`
	Reason string `json:"reason"`
}

// Snooze handles POST /api/v1/data-health/{alertId}/snooze.
func (hh *HealthHandler) Snooze(w http.ResponseWriter, r *http.Request) {
	var in snoozePayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	alert, err := hh.h.Snooze(r.Context(), authcontracts.BranchID(r), viewerOf(r), r.PathValue("alertId"),
		application.SnoozeInput{Days: in.Days, Reason: in.Reason})
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Alert ditunda", alert)
}

// Unsnooze handles DELETE /api/v1/data-health/{alertId}/snooze (idempotent).
func (hh *HealthHandler) Unsnooze(w http.ResponseWriter, r *http.Request) {
	alertID := r.PathValue("alertId")
	existed, err := hh.h.Unsnooze(r.Context(), authcontracts.BranchID(r), viewerOf(r), alertID)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	msg := "Alert diaktifkan lagi"
	if !existed {
		msg = "Alert tidak sedang ditunda"
	}
	response.OK(w, msg, map[string]any{"alertId": alertID, "snoozed": false})
}

// RegisterHealthRoutes mounts the data-health endpoints. The keamanan area is
// filtered per request by dashboard.alerts.security inside the service.
func RegisterHealthRoutes(mux *http.ServeMux, hh *HealthHandler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/dashboard/alerts", perm("dashboard.alerts.view_all", authcontracts.Guard(hh.Alerts, branchGuard)))
	mux.Handle("GET /api/v1/data-health", perm("dashboard.alerts.view_all", authcontracts.Guard(hh.DataHealth, branchGuard)))
	mux.Handle("GET /api/v1/data-health/{alertId}/items", perm("dashboard.alerts.view_all", authcontracts.Guard(hh.Items, branchGuard)))
	mux.Handle("POST /api/v1/data-health/{alertId}/snooze", perm("dashboard.alerts.view_all", authcontracts.Guard(hh.Snooze, branchGuard)))
	mux.Handle("DELETE /api/v1/data-health/{alertId}/snooze", perm("dashboard.alerts.view_all", authcontracts.Guard(hh.Unsnooze, branchGuard)))
}
