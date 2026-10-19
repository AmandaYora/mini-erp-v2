package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/dashboard/application"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the dashboard module HTTP surface (read-only).
type Handler struct {
	svc *application.Service
}

// NewHandler wires dashboard endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

// GetSummary handles GET /api/v1/dashboard/summary.
func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := h.svc.Summary(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Ringkasan dasbor", sum)
}

// maxDisplayLimit bounds ?limit= before the service applies its own widget caps.
const maxDisplayLimit = 100

// limitParam reads an optional ?limit= display cap (0 = service default).
func limitParam(r *http.Request) int {
	return pagination.ParseDisplayLimit(r.URL.Query().Get("limit"), 0, maxDisplayLimit)
}

// GetCriticalStock handles GET /api/v1/dashboard/critical-stock?limit=.
func (h *Handler) GetCriticalStock(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.CriticalStock(r.Context(), authcontracts.BranchID(r), limitParam(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Stok kritis", items)
}

// GetPriorityOrders handles GET /api/v1/dashboard/priority-orders?limit=.
func (h *Handler) GetPriorityOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.svc.PriorityOrders(r.Context(), authcontracts.BranchID(r), limitParam(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pesanan prioritas", orders)
}

// GetOrderStatus handles GET /api/v1/dashboard/order-status.
func (h *Handler) GetOrderStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.svc.OrderStatus(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Status order", status)
}

// GetTopSellers handles GET /api/v1/dashboard/top-sellers?limit=.
func (h *Handler) GetTopSellers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.TopSellers(r.Context(), authcontracts.BranchID(r), limitParam(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Barang terlaris", rows)
}

// GetTopMargin handles GET /api/v1/dashboard/top-margin?limit=.
func (h *Handler) GetTopMargin(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.TopMargin(r.Context(), authcontracts.BranchID(r), limitParam(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Margin tertinggi", rows)
}

// RegisterRoutes mounts dashboard endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/dashboard/summary", perm("dashboard.view", authcontracts.Guard(h.GetSummary, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/critical-stock", perm("dashboard.view", authcontracts.Guard(h.GetCriticalStock, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/priority-orders", perm("dashboard.view", authcontracts.Guard(h.GetPriorityOrders, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/order-status", perm("dashboard.view", authcontracts.Guard(h.GetOrderStatus, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/top-sellers", perm("dashboard.view", authcontracts.Guard(h.GetTopSellers, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/top-margin", perm("dashboard.view", authcontracts.Guard(h.GetTopMargin, branchGuard)))
}
