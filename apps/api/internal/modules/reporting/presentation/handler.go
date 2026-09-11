package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/reporting/application"
	"mini-erp/internal/shared/response"
)

// Handler serves the reporting module HTTP surface (read-only).
type Handler struct {
	svc *application.Service
}

// NewHandler wires reporting endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func branchID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.BranchID
	}
	return 0
}

// GetSalesTrend handles GET /api/v1/reporting/sales-trend?from&to.
func (h *Handler) GetSalesTrend(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	points, err := h.svc.SalesTrend(r.Context(), branchID(r), q.Get("from"), q.Get("to"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Tren penjualan", points)
}

// GetInventory handles GET /api/v1/reporting/inventory.
func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
	rep, err := h.svc.Inventory(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Laporan persediaan", rep)
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts reporting endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/reporting/sales-trend", perm("reporting.view", guarded(h.GetSalesTrend, branchGuard)))
	mux.Handle("GET /api/v1/reporting/inventory", perm("reporting.view", guarded(h.GetInventory, branchGuard)))
}
