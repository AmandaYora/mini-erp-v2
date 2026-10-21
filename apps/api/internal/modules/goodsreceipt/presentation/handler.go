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
	"mini-erp/internal/shared/timeutil"
)

// Handler serves the goodsreceipt module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires goodsreceipt endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
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
	v := map[string]any{
		"id": rec.ID, "branchId": rec.BranchID, "purchaseOrderId": rec.PurchaseOrderID,
		"number": rec.Number, "status": rec.Status,
		"supplierDeliveryNumber": rec.SupplierDeliveryNumber,
		"orderNumber":            rec.OrderNumber, "receivedAt": rec.ReceivedAt,
		"notes": rec.Notes, "items": items,
		"lineCount": rec.LineCount, "receivedBy": rec.CreatedBy, "receivedByName": rec.ReceivedByName,
		"cancelReason": rec.CancelReason, "cancelledAt": timeutil.FormatWIB(rec.CancelledAt), "cancelledBy": rec.CancelledBy,
	}
	if rec.Payment != nil {
		v["payment"] = map[string]any{"id": rec.Payment.ID, "number": rec.Payment.Number}
	}
	return v
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
	res, err := h.svc.List(r.Context(), authcontracts.BranchID(r), poID, page, limit)
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
	if rec == nil || rec.BranchID != authcontracts.BranchID(r) {
		response.Fail(w, apperror.NotFound("Penerimaan"))
		return
	}
	response.OK(w, "Data penerimaan", receiptView(rec))
}

// Create handles POST /api/v1/goods-receipts.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PurchaseOrderID        int64              `json:"purchaseOrderId" validate:"required"`
		ReceivedAt             string             `json:"receivedAt"`
		Notes                  string             `json:"notes"`
		SupplierDeliveryNumber string             `json:"supplierDeliveryNumber"`
		ClientRef              string             `json:"clientRef"`
		Settlement             *settlementPayload `json:"settlement"`
		Items                  []struct {
			ProductID  int64   `json:"productId" validate:"required"`
			VariantID  int64   `json:"variantId" validate:"required"`
			LocationID int64   `json:"locationId"`
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
	st, ok := settlementFrom(w, r, in.Settlement)
	if !ok {
		return
	}
	rec, err := h.svc.Create(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.PurchaseOrderID, in.ReceivedAt, in.Notes, lines,
		application.WithSupplierDeliveryNumber(in.SupplierDeliveryNumber),
		application.WithClientRef(in.ClientRef),
		application.WithSettlement(st))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Penerimaan dicatat", receiptView(rec))
}

// Complete handles POST /api/v1/goods-receipts/{id}/complete: re-run the
// stock posting of a pending receipt (idempotent).
func (h *Handler) Complete(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Penerimaan"))
		return
	}
	var in struct {
		Settlement *settlementPayload `json:"settlement"`
	}
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	st, ok := settlementFrom(w, r, in.Settlement)
	if !ok {
		return
	}
	rec, appErr := h.svc.Complete(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, st)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Penerimaan diposting", receiptView(rec))
}

// Discard handles POST /api/v1/goods-receipts/{id}/discard: abandon a
// pending receipt, undoing any stock it already moved. Body {reason}
// (mandatory).
func (h *Handler) Discard(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Penerimaan"))
		return
	}
	reason, err := httpx.DecodeReason(r)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	rec, appErr := h.svc.Discard(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, reason)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Penerimaan dibatalkan", receiptView(rec))
}

type settlementPayload struct {
	Mode        string `json:"mode"`
	Method      string `json:"method"`
	PaymentDate string `json:"paymentDate"`
	Reference   string `json:"reference"`
	DueDate     string `json:"dueDate"`
	Notes       string `json:"notes"`
}

// settlementFrom converts the optional settlement body, checking the caller may
// do what it asks: paying needs payment.create, converting to net terms needs
// purchasing.update.
func settlementFrom(w http.ResponseWriter, r *http.Request, in *settlementPayload) (*application.Settlement, bool) {
	if in == nil {
		return nil, true
	}
	need := "payment.create"
	if in.Mode == application.SettleSwitchToNet {
		need = "purchasing.update"
	}
	if sess := authcontracts.SessionFromContext(r.Context()); sess == nil || !sess.HasPermission(need) {
		response.Fail(w, apperror.Forbidden())
		return nil, false
	}
	return &application.Settlement{Mode: in.Mode, Method: in.Method, PaymentDate: in.PaymentDate,
		Reference: in.Reference, DueDate: in.DueDate, Notes: in.Notes}, true
}

// RegisterRoutes mounts goodsreceipt endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/goods-receipts", perm("goodsreceipt.view", authcontracts.Guard(h.List, branchGuard)))
	mux.Handle("POST /api/v1/goods-receipts", perm("goodsreceipt.create", authcontracts.Guard(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/goods-receipts/{id}", perm("goodsreceipt.view", authcontracts.Guard(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/goods-receipts/{id}/complete", perm("goodsreceipt.create", authcontracts.Guard(h.Complete, branchGuard)))
	mux.Handle("POST /api/v1/goods-receipts/{id}/discard", perm("goodsreceipt.create", authcontracts.Guard(h.Discard, branchGuard)))
}
