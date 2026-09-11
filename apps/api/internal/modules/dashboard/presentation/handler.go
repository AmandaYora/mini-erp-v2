package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/dashboard/application"
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

func branchID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.BranchID
	}
	return 0
}

// GetSummary handles GET /api/v1/dashboard/summary.
func (h *Handler) GetSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := h.svc.Summary(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Ringkasan dasbor", sum)
}

// limitParam reads an optional ?limit= display cap (0 = service default).
func limitParam(r *http.Request) int {
	n, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	return n
}

// GetCriticalStock handles GET /api/v1/dashboard/critical-stock?limit=.
func (h *Handler) GetCriticalStock(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.CriticalStock(r.Context(), branchID(r), limitParam(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Stok kritis", items)
}

// GetPriorityOrders handles GET /api/v1/dashboard/priority-orders?limit=.
func (h *Handler) GetPriorityOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.svc.PriorityOrders(r.Context(), branchID(r), limitParam(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pesanan prioritas", orders)
}

// GetOrderStatus handles GET /api/v1/dashboard/order-status.
func (h *Handler) GetOrderStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.svc.OrderStatus(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Status order", status)
}

// GetTopSellers handles GET /api/v1/dashboard/top-sellers?limit=.
func (h *Handler) GetTopSellers(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.TopSellers(r.Context(), branchID(r), limitParam(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Barang terlaris", rows)
}

// GetTopMargin handles GET /api/v1/dashboard/top-margin?limit=.
func (h *Handler) GetTopMargin(w http.ResponseWriter, r *http.Request) {
	rows, err := h.svc.TopMargin(r.Context(), branchID(r), limitParam(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Margin tertinggi", rows)
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts dashboard endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/dashboard/summary", perm("dashboard.view", guarded(h.GetSummary, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/critical-stock", perm("dashboard.view", guarded(h.GetCriticalStock, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/priority-orders", perm("dashboard.view", guarded(h.GetPriorityOrders, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/order-status", perm("dashboard.view", guarded(h.GetOrderStatus, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/top-sellers", perm("dashboard.view", guarded(h.GetTopSellers, branchGuard)))
	mux.Handle("GET /api/v1/dashboard/top-margin", perm("dashboard.view", guarded(h.GetTopMargin, branchGuard)))
}
