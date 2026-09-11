package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/purchasereturn/application"
	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the purchasereturn module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires purchasereturn endpoints.
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

func returnView(ret *contracts.PurchaseReturn) map[string]any {
	items := make([]any, 0, len(ret.Items))
	for _, it := range ret.Items {
		items = append(items, map[string]any{
			"id": it.ID, "productId": it.ProductID, "variantId": it.VariantID,
			"locationId": it.LocationID, "uom": it.UOM, "uomFactor": it.UOMFactor,
			"qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "discountPct": it.DiscountPct,
			"discountNominal": it.DiscountNominal, "taxBase": it.TaxBase,
			"taxAmount": it.TaxAmount, "lineTotal": it.LineTotal,
		})
	}
	settlements := make([]any, 0, len(ret.Settlements))
	var settled int64
	for _, st := range ret.Settlements {
		settlements = append(settlements, map[string]any{
			"id": st.ID, "type": st.Type, "date": st.Date, "amount": st.Amount,
			"paymentMethod": st.PaymentMethod, "referenceNumber": st.ReferenceNumber,
			"notes": st.Notes,
		})
		settled += st.Amount
	}
	return map[string]any{
		"id": ret.ID, "number": ret.Number, "branchId": ret.BranchID,
		"purchaseOrderId": ret.PurchaseOrderID, "orderNumber": ret.OrderNumber,
		"returnDate": ret.ReturnDate, "subtotal": ret.Subtotal,
		"discountTotal": ret.DiscountTotal, "taxTotal": ret.TaxTotal,
		"taxType": ret.TaxType, "taxRate": ret.TaxRate, "total": ret.Total,
		"status": ret.Status, "notes": ret.Notes, "items": items,
		"settlements": settlements, "settledTotal": settled,
		"unsettledTotal": ret.Total - settled,
	}
}

type linePayload struct {
	ProductID  int64   `json:"productId" validate:"required"`
	VariantID  int64   `json:"variantId" validate:"required"`
	LocationID int64   `json:"locationId" validate:"required"`
	UOM        string  `json:"uom" validate:"required"`
	Qty        float64 `json:"qty" validate:"required"`
}

func toLines(in []linePayload) []application.LineInput {
	out := make([]application.LineInput, 0, len(in))
	for _, l := range in {
		out = append(out, application.LineInput{
			ProductID: l.ProductID, VariantID: l.VariantID,
			LocationID: l.LocationID, UOM: l.UOM, Qty: l.Qty,
		})
	}
	return out
}

func previewView(p *application.ReturnPreview) map[string]any {
	items := make([]any, 0, len(p.Items))
	for _, it := range p.Items {
		items = append(items, map[string]any{
			"productId": it.ProductID, "variantId": it.VariantID,
			"locationId": it.LocationID, "uom": it.UOM, "uomFactor": it.UOMFactor,
			"qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "discountPct": it.DiscountPct,
			"discountNominal": it.DiscountNominal, "taxBase": it.TaxBase,
			"taxAmount": it.TaxAmount, "lineTotal": it.LineTotal,
		})
	}
	return map[string]any{
		"subtotal": p.Subtotal, "discountTotal": p.DiscountTotal,
		"taxTotal": p.TaxTotal, "total": p.Total, "items": items,
	}
}

// ReturnContext handles GET /api/v1/purchase-returns/context?purchaseOrderId=.
func (h *Handler) ReturnContext(w http.ResponseWriter, r *http.Request) {
	var poID int64
	if raw := r.URL.Query().Get("purchaseOrderId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			poID = id
		}
	}
	if poID == 0 {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "purchaseOrderId", Message: "wajib diisi"}}))
		return
	}
	ctx, err := h.svc.ReturnContext(r.Context(), branchID(r), poID)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Konteks retur", ctx)
}

// Preview handles POST /api/v1/purchase-returns/preview (dry-run, no writes).
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PurchaseOrderID int64         `json:"purchaseOrderId" validate:"required"`
		Items           []linePayload `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	p, err := h.svc.PreviewReturn(r.Context(), branchID(r), in.PurchaseOrderID, toLines(in.Items))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pratinjau retur", previewView(p))
}

// AddSettlement handles POST /api/v1/purchase-returns/{id}/settlements.
func (h *Handler) AddSettlement(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur pembelian"))
		return
	}
	var in struct {
		Type            string `json:"type" validate:"required"`
		Date            string `json:"date"`
		Amount          int64  `json:"amount" validate:"required"`
		PaymentMethod   string `json:"paymentMethod"`
		ReferenceNumber string `json:"referenceNumber"`
		Notes           string `json:"notes"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	ret, appErr := h.svc.AddSettlement(r.Context(), actorID(r), branchID(r), id, application.SettlementInput{
		Type: in.Type, Date: in.Date, Amount: in.Amount,
		PaymentMethod: in.PaymentMethod, ReferenceNumber: in.ReferenceNumber, Notes: in.Notes,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.Created(w, "Penyelesaian dicatat", returnView(ret))
}

// List handles GET /api/v1/purchase-returns.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	var poID int64
	if raw := q.Get("purchaseOrderId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			poID = id
		}
	}
	res, err := h.svc.List(r.Context(), branchID(r), poID, q.Get("status"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Returns))
	for _, ret := range res.Returns {
		items = append(items, returnView(ret))
	}
	response.Paginated(w, "Data retur pembelian", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// Get handles GET /api/v1/purchase-returns/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur pembelian"))
		return
	}
	ret, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if ret == nil || ret.BranchID != branchID(r) {
		response.Fail(w, apperror.NotFound("Retur pembelian"))
		return
	}
	response.OK(w, "Data retur pembelian", returnView(ret))
}

// Create handles POST /api/v1/purchase-returns.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PurchaseOrderID int64         `json:"purchaseOrderId" validate:"required"`
		ReturnDate      string        `json:"returnDate"`
		Notes           string        `json:"notes"`
		Items           []linePayload `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	ret, err := h.svc.Create(r.Context(), actorID(r), branchID(r), in.PurchaseOrderID, in.ReturnDate, in.Notes, toLines(in.Items))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Retur pembelian dibuat", returnView(ret))
}

// Confirm handles POST /api/v1/purchase-returns/{id}/confirm.
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur pembelian"))
		return
	}
	ret, appErr := h.svc.Confirm(r.Context(), actorID(r), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Retur pembelian dikonfirmasi", returnView(ret))
}

// Cancel handles POST /api/v1/purchase-returns/{id}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur pembelian"))
		return
	}
	ret, appErr := h.svc.Cancel(r.Context(), actorID(r), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Retur pembelian dibatalkan", returnView(ret))
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// guarded adapts the branch guard into func-style middleware.
func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts purchasereturn endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/purchase-returns", perm("purchasereturn.view", guarded(h.List, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns", perm("purchasereturn.create", guarded(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/purchase-returns/context", perm("purchasereturn.view", guarded(h.ReturnContext, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns/preview", perm("purchasereturn.create", guarded(h.Preview, branchGuard)))
	mux.Handle("GET /api/v1/purchase-returns/{id}", perm("purchasereturn.view", guarded(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns/{id}/confirm", perm("purchasereturn.create", guarded(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns/{id}/cancel", perm("purchasereturn.archive", guarded(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns/{id}/settlements", perm("purchasereturn.create", guarded(h.AddSettlement, branchGuard)))
}
