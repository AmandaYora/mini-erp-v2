package presentation

import (
	"net/http"
	"strconv"
	"strings"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/purchasereturn/application"
	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/listquery"
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

// allowedTypes keeps the JSON an array ([] never null) for lists, where the
// position is not computed.
func allowedTypes(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
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
		"status": ret.Status, "notes": ret.Notes, "reason": ret.Reason, "payableReduction": ret.PayableReduction, "items": items,
		"settlements": settlements, "settledTotal": settled,
		"unsettledTotal":    ret.Total - settled,
		"externalRemaining": ret.ExternalRemaining, "allowedSettlementTypes": allowedTypes(ret.AllowedSettlementTypes),
	}
}

type linePayload struct {
	ProductID  int64   `json:"productId" validate:"required"`
	VariantID  int64   `json:"variantId" validate:"required"`
	LocationID int64   `json:"locationId"`
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
		"orderOutstanding": p.OrderOutstanding, "payableReduction": p.PayableReduction,
		"externalRemaining": p.ExternalRemaining, "allowedSettlementTypes": allowedTypes(p.AllowedSettlementTypes),
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
	ctx, err := h.svc.ReturnContext(r.Context(), authcontracts.BranchID(r), poID)
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
	p, err := h.svc.PreviewReturn(r.Context(), authcontracts.BranchID(r), in.PurchaseOrderID, toLines(in.Items))
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
	// A refund moves money out: legacy demands its own permission on top of
	// create (F3b-2). The service never sees the session, so the handler checks.
	if in.Type == "refund" {
		if sess := authcontracts.SessionFromContext(r.Context()); sess == nil || !sess.HasPermission("purchasereturn.refund") {
			response.Fail(w, &apperror.AppError{Code: apperror.CodeForbidden, Message: "Anda tidak memiliki izin memproses refund"})
			return
		}
	}
	ret, appErr := h.svc.AddSettlement(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, application.SettlementInput{
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
	c, err := listquery.ParseCommon(q, nil)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	page, limit := c.Page, c.Limit
	poID, err := listquery.ParseID(q.Get("purchaseOrderId"), "purchaseOrderId")
	if err != nil {
		response.FailErr(w, err)
		return
	}
	res, err := h.svc.ListFiltered(r.Context(), authcontracts.BranchID(r), application.ListFilter{
		Search: c.Search, Statuses: c.Statuses, PurchaseOrderID: poID, From: c.From, To: c.To,
	}, page, limit)
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
	if ret == nil || ret.BranchID != authcontracts.BranchID(r) {
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
		Reason          string        `json:"reason"`
		Items           []linePayload `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if strings.TrimSpace(in.Reason) == "" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan retur wajib diisi"}}))
		return
	}
	ret, err := h.svc.Create(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.PurchaseOrderID, in.ReturnDate, in.Notes, toLines(in.Items),
		application.WithReason(in.Reason))
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
	ret, appErr := h.svc.Confirm(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
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
	ret, appErr := h.svc.Cancel(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Retur pembelian dibatalkan", returnView(ret))
}

// RegisterRoutes mounts purchasereturn endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/purchase-returns", perm("purchasereturn.view", authcontracts.Guard(h.List, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns", perm("purchasereturn.create", authcontracts.Guard(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/purchase-returns/context", perm("purchasereturn.view", authcontracts.Guard(h.ReturnContext, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns/preview", perm("purchasereturn.create", authcontracts.Guard(h.Preview, branchGuard)))
	mux.Handle("GET /api/v1/purchase-returns/{id}", perm("purchasereturn.view", authcontracts.Guard(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns/{id}/confirm", perm("purchasereturn.create", authcontracts.Guard(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns/{id}/cancel", perm("purchasereturn.archive", authcontracts.Guard(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/purchase-returns/{id}/settlements", perm("purchasereturn.create", authcontracts.Guard(h.AddSettlement, branchGuard)))
}
