package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/salesreturn/application"
	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

type linePayload struct {
	ProductID  int64   `json:"productId" validate:"required"`
	VariantID  int64   `json:"variantId" validate:"required"`
	LocationID int64   `json:"locationId" validate:"required"`
	UOM        string  `json:"uom" validate:"required"`
	Qty        float64 `json:"qty" validate:"required"`
}

// Handler serves the salesreturn module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires salesreturn endpoints.
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

func returnView(ret *contracts.SalesReturn) map[string]any {
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
	repl := make([]any, 0, len(ret.ReplacementItems))
	for _, it := range ret.ReplacementItems {
		repl = append(repl, map[string]any{
			"id": it.ID, "productId": it.ProductID, "variantId": it.VariantID,
			"locationId": it.LocationID, "uom": it.UOM, "uomFactor": it.UOMFactor,
			"qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "lineTotal": it.LineTotal,
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
		"salesOrderId": ret.SalesOrderID, "orderNumber": ret.OrderNumber,
		"returnDate": ret.ReturnDate, "subtotal": ret.Subtotal,
		"discountTotal": ret.DiscountTotal, "taxTotal": ret.TaxTotal,
		"taxType": ret.TaxType, "taxRate": ret.TaxRate, "total": ret.Total,
		"status": ret.Status, "notes": ret.Notes,
		"returnMode":                ret.ReturnMode,
		"replacementDeliveryStatus": ret.ReplacementDeliveryStatus,
		"items":                     items, "replacementItems": repl,
		"settlements": settlements, "settledTotal": settled,
		"unsettledTotal": ret.Total - settled,
	}
}

// List handles GET /api/v1/sales-returns.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	var soID int64
	if raw := q.Get("salesOrderId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			soID = id
		}
	}
	res, err := h.svc.List(r.Context(), branchID(r), soID, q.Get("status"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Returns))
	for _, ret := range res.Returns {
		items = append(items, returnView(ret))
	}
	response.Paginated(w, "Data retur penjualan", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// Get handles GET /api/v1/sales-returns/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur penjualan"))
		return
	}
	ret, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if ret == nil || ret.BranchID != branchID(r) {
		response.Fail(w, apperror.NotFound("Retur penjualan"))
		return
	}
	response.OK(w, "Data retur penjualan", returnView(ret))
}

// Create handles POST /api/v1/sales-returns.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SalesOrderID     int64                `json:"salesOrderId" validate:"required"`
		ReturnDate       string               `json:"returnDate"`
		Notes            string               `json:"notes"`
		ReturnMode       string               `json:"returnMode"`
		Items            []linePayload        `json:"items" validate:"required"`
		ReplacementItems []replacementPayload `json:"replacementItems"`
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
	repl := make([]application.ReplacementInput, 0, len(in.ReplacementItems))
	for _, l := range in.ReplacementItems {
		repl = append(repl, application.ReplacementInput{
			ProductID: l.ProductID, VariantID: l.VariantID,
			LocationID: l.LocationID, UOM: l.UOM, Qty: l.Qty,
		})
	}
	ret, err := h.svc.Create(r.Context(), actorID(r), branchID(r), in.SalesOrderID, in.ReturnDate, in.Notes, in.ReturnMode, lines, repl)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Retur penjualan dibuat", returnView(ret))
}

// Confirm handles POST /api/v1/sales-returns/{id}/confirm.
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur penjualan"))
		return
	}
	ret, appErr := h.svc.Confirm(r.Context(), actorID(r), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Retur penjualan dikonfirmasi", returnView(ret))
}

// Cancel handles POST /api/v1/sales-returns/{id}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur penjualan"))
		return
	}
	ret, appErr := h.svc.Cancel(r.Context(), actorID(r), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Retur penjualan dibatalkan", returnView(ret))
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// guarded adapts the branch guard into func-style middleware.
func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// replacementPayload mirrors application.ReplacementInput.
type replacementPayload struct {
	ProductID  int64   `json:"productId" validate:"required"`
	VariantID  int64   `json:"variantId" validate:"required"`
	LocationID int64   `json:"locationId" validate:"required"`
	UOM        string  `json:"uom" validate:"required"`
	Qty        float64 `json:"qty" validate:"required"`
}

func toReplacements(in []replacementPayload) []application.ReplacementInput {
	out := make([]application.ReplacementInput, 0, len(in))
	for _, l := range in {
		out = append(out, application.ReplacementInput{
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
	repl := make([]any, 0, len(p.ReplacementItems))
	var replTotal int64
	for _, it := range p.ReplacementItems {
		repl = append(repl, map[string]any{
			"productId": it.ProductID, "variantId": it.VariantID,
			"locationId": it.LocationID, "uom": it.UOM, "uomFactor": it.UOMFactor,
			"qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "lineTotal": it.LineTotal,
		})
		replTotal += it.LineTotal
	}
	return map[string]any{
		"mode": p.Mode, "subtotal": p.Subtotal, "discountTotal": p.DiscountTotal,
		"taxTotal": p.TaxTotal, "total": p.Total,
		"items": items, "replacementItems": repl, "replacementTotal": replTotal,
	}
}

// ReturnContext handles GET /api/v1/sales-returns/context?salesOrderId=.
func (h *Handler) ReturnContext(w http.ResponseWriter, r *http.Request) {
	var soID int64
	if raw := r.URL.Query().Get("salesOrderId"); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil && id > 0 {
			soID = id
		}
	}
	if soID == 0 {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "salesOrderId", Message: "wajib diisi"}}))
		return
	}
	ctx, err := h.svc.ReturnContext(r.Context(), branchID(r), soID)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Konteks retur", ctx)
}

// Preview handles POST /api/v1/sales-returns/preview (dry-run, no writes).
func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SalesOrderID     int64                `json:"salesOrderId" validate:"required"`
		ReturnMode       string               `json:"returnMode"`
		Items            []linePayload        `json:"items" validate:"required"`
		ReplacementItems []replacementPayload `json:"replacementItems"`
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
	p, err := h.svc.PreviewReturn(r.Context(), branchID(r), in.SalesOrderID, in.ReturnMode, lines, toReplacements(in.ReplacementItems))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pratinjau retur", previewView(p))
}

// DispatchReplacement handles POST /api/v1/sales-returns/{id}/replacement-deliveries.
func (h *Handler) DispatchReplacement(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur penjualan"))
		return
	}
	var in struct {
		DeliveryDate       string `json:"deliveryDate"`
		Notes              string `json:"notes"`
		DriverName         string `json:"driverName"`
		VehiclePlate       string `json:"vehiclePlate"`
		WarehouseStaffName string `json:"warehouseStaffName"`
		DropLocationNote   string `json:"dropLocationNote"`
	}
	// All fields optional: a bare POST dispatches with defaults.
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	dn, appErr := h.svc.CreateReplacementDelivery(r.Context(), actorID(r), branchID(r), id, application.ReplacementDispatchInput{
		DeliveryDate: in.DeliveryDate, Notes: in.Notes,
		DriverName: in.DriverName, VehiclePlate: in.VehiclePlate,
		WarehouseStaffName: in.WarehouseStaffName, DropLocationNote: in.DropLocationNote,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.Created(w, "Pengiriman pengganti dibuat", map[string]any{
		"id": dn.ID, "number": dn.Number, "status": dn.Status,
	})
}

// ConfirmReplacement handles POST /api/v1/sales-returns/{id}/replacement-deliveries/{deliveryId}/confirm.
func (h *Handler) ConfirmReplacement(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur penjualan"))
		return
	}
	deliveryID, err := httpx.PathInt(r, "deliveryId")
	if err != nil {
		response.Fail(w, apperror.NotFound("Surat jalan pengganti"))
		return
	}
	var in struct {
		RecipientName                   string `json:"recipientName"`
		RecipientSignatureStatus        string `json:"recipientSignatureStatus"`
		RecipientSignatureMissingReason string `json:"recipientSignatureMissingReason"`
	}
	// Receipt evidence optional: bare confirms post an empty body.
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	dn, appErr := h.svc.ConfirmReplacementDelivery(r.Context(), actorID(r), branchID(r), id, deliveryID,
		deliverycontracts.ReplacementConfirm{
			RecipientName:            in.RecipientName,
			RecipientSignatureStatus: in.RecipientSignatureStatus,
			MissingReason:            in.RecipientSignatureMissingReason,
		})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Pengiriman pengganti dikonfirmasi", map[string]any{
		"id": dn.ID, "number": dn.Number, "status": dn.Status,
	})
}

// AddSettlement handles POST /api/v1/sales-returns/{id}/settlements.
func (h *Handler) AddSettlement(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Retur penjualan"))
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

// RegisterRoutes mounts salesreturn endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/sales-returns", perm("salesreturn.view", guarded(h.List, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns", perm("salesreturn.create", guarded(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/sales-returns/context", perm("salesreturn.view", guarded(h.ReturnContext, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/preview", perm("salesreturn.create", guarded(h.Preview, branchGuard)))
	mux.Handle("GET /api/v1/sales-returns/{id}", perm("salesreturn.view", guarded(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/confirm", perm("salesreturn.create", guarded(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/cancel", perm("salesreturn.archive", guarded(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/replacement-deliveries", perm("salesreturn.create", guarded(h.DispatchReplacement, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/replacement-deliveries/{deliveryId}/confirm", perm("salesreturn.create", guarded(h.ConfirmReplacement, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/settlements", perm("salesreturn.create", guarded(h.AddSettlement, branchGuard)))
}
