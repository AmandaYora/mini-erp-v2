package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/goodsreceipt/application"
	"mini-erp/internal/modules/goodsreceipt/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the goodsreceipt module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires goodsreceipt endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func actorID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.UserID
	}
	return 0
}

func branchID(r *http.Request) int64 {
	if s := authcontracts.SessionFromContext(r.Context()); s != nil {
		return s.BranchID
	}
	return 0
}

func receiptView(rec *contracts.GoodsReceipt) map[string]any {
	items := make([]any, 0, len(rec.Items))
	for _, it := range rec.Items {
		items = append(items, map[string]any{
			"id": it.ID, "productId": it.ProductID, "variantId": it.VariantID,
			"locationId": it.LocationID, "uom": it.UOM, "uomFactor": it.UOMFactor,
			"qty": it.Qty, "qtyBase": it.QtyBase,
		})
	}
	return map[string]any{
		"id": rec.ID, "branchId": rec.BranchID, "purchaseOrderId": rec.PurchaseOrderID,
		"orderNumber": rec.OrderNumber, "receivedAt": rec.ReceivedAt,
		"notes": rec.Notes, "items": items,
	}
}

// List handles GET /api/v1/goods-receipts.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	var poID int64
	if raw := q.Get("purchaseOrderId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			poID = id
		}
	}
	res, err := h.svc.List(r.Context(), branchID(r), poID, page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Receipts))
	for _, rec := range res.Receipts {
		items = append(items, receiptView(rec))
	}
	response.Paginated(w, "Data penerimaan barang", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// Get handles GET /api/v1/goods-receipts/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Penerimaan"))
		return
	}
	rec, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if rec == nil || rec.BranchID != branchID(r) {
		response.Fail(w, apperror.NotFound("Penerimaan"))
		return
	}
	response.OK(w, "Data penerimaan", receiptView(rec))
}

// Create handles POST /api/v1/goods-receipts.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PurchaseOrderID int64  `json:"purchaseOrderId" validate:"required"`
		ReceivedAt      string `json:"receivedAt"`
		Notes           string `json:"notes"`
		Items           []struct {
			ProductID  int64   `json:"productId" validate:"required"`
			VariantID  int64   `json:"variantId" validate:"required"`
			LocationID int64   `json:"locationId" validate:"required"`
			UOM        string  `json:"uom" validate:"required"`
			Qty        float64 `json:"qty" validate:"required"`
		} `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	lines := make([]application.LineInput, 0, len(in.Items))
	for _, l := range in.Items {
		lines = append(lines, application.LineInput{
			ProductID: l.ProductID, VariantID: l.VariantID,
			LocationID: l.LocationID, UOM: l.UOM, Qty: l.Qty,
		})
	}
	rec, err := h.svc.Create(r.Context(), actorID(r), branchID(r), in.PurchaseOrderID, in.ReceivedAt, in.Notes, lines)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Penerimaan dicatat", receiptView(rec))
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// guarded adapts the branch guard into func-style middleware.
func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts goodsreceipt endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/goods-receipts", perm("goodsreceipt.view", guarded(h.List, branchGuard)))
	mux.Handle("POST /api/v1/goods-receipts", perm("goodsreceipt.create", guarded(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/goods-receipts/{id}", perm("goodsreceipt.view", guarded(h.Get, branchGuard)))
}
