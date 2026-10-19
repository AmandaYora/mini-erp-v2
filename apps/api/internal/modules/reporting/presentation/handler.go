package presentation

import (
	"net/http"
	"strconv"

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

// GetSalesTrend handles GET /api/v1/reporting/sales-trend?from&to.
func (h *Handler) GetSalesTrend(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	points, err := h.svc.SalesTrend(r.Context(), authcontracts.BranchID(r), q.Get("from"), q.Get("to"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Tren penjualan", points)
}

// GetInventory handles GET /api/v1/reporting/inventory. Same data as
// GET /api/v1/finance/reports/inventory-value (finance owns the valuation),
// exposed under reporting.view for that audience.
func (h *Handler) GetInventory(w http.ResponseWriter, r *http.Request) {
	rep, err := h.svc.Inventory(r.Context(), authcontracts.BranchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Laporan persediaan", rep)
}

// GetOrders handles GET /api/v1/reporting/orders?type=sales|purchasing&status=&from=&to=&page=&limit=.
func (h *Handler) GetOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	rep, err := h.svc.Orders(r.Context(), authcontracts.BranchID(r), q.Get("type"), q.Get("status"), q.Get("from"), q.Get("to"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Laporan order", rep)
}

// GetStock handles GET /api/v1/reporting/stock?criticalOnly=&page=&limit=.
func (h *Handler) GetStock(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	rep, err := h.svc.Stock(r.Context(), authcontracts.BranchID(r), q.Get("criticalOnly") == "true", page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Laporan stok", rep)
}

// RegisterRoutes mounts reporting endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/reporting/sales-trend", perm("reporting.view", authcontracts.Guard(h.GetSalesTrend, branchGuard)))
	mux.Handle("GET /api/v1/reporting/inventory", perm("reporting.view", authcontracts.Guard(h.GetInventory, branchGuard)))
	mux.Handle("GET /api/v1/reporting/orders", perm("reporting.view", authcontracts.Guard(h.GetOrders, branchGuard)))
	mux.Handle("GET /api/v1/reporting/stock", perm("reporting.view", authcontracts.Guard(h.GetStock, branchGuard)))
}
