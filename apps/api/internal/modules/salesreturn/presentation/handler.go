package presentation

import (
	"net/http"
	"strconv"
	"strings"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/salesreturn/application"
	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/listquery"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

type linePayload struct {
	ProductID  int64   `json:"productId" validate:"required"`
	VariantID  int64   `json:"variantId" validate:"required"`
	LocationID int64   `json:"locationId"`
	UOM        string  `json:"uom" validate:"required"`
	Qty        float64 `json:"qty" validate:"required"`
	Condition  string  `json:"condition"`
}

// Handler serves the salesreturn module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires salesreturn endpoints.
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

func returnView(ret *contracts.SalesReturn) map[string]any {
	items := make([]any, 0, len(ret.Items))
	for _, it := range ret.Items {
		items = append(items, map[string]any{
			"id": it.ID, "productId": it.ProductID, "variantId": it.VariantID,
			"locationId": it.LocationID, "uom": it.UOM, "uomFactor": it.UOMFactor,
			"qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "discountPct": it.DiscountPct,
			"discountNominal": it.DiscountNominal, "taxBase": it.TaxBase,
			"taxAmount": it.TaxAmount, "lineTotal": it.LineTotal, "condition": it.Condition,
		})
	}
	repl := make([]any, 0, len(ret.ReplacementItems))
	for _, it := range ret.ReplacementItems {
		repl = append(repl, map[string]any{
			"id": it.ID, "productId": it.ProductID, "variantId": it.VariantID,
			"locationId": it.LocationID, "uom": it.UOM, "uomFactor": it.UOMFactor,
			"qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "taxBase": it.TaxBase, "taxAmount": it.TaxAmount, "lineTotal": it.LineTotal,
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
		"reason": ret.Reason, "replacementSubtotal": ret.ReplacementSubtotal,
		"replacementTaxTotal": ret.ReplacementTaxTotal, "replacementTotal": ret.ReplacementTotal,
		"differenceAmount": ret.DifferenceAmount, "receivableReduction": ret.ReceivableReduction,
		"returnMode":                ret.ReturnMode,
		"replacementDeliveryStatus": ret.ReplacementDeliveryStatus,
		"items":                     items, "replacementItems": repl,
		"settlements": settlements, "settledTotal": settled,
		"unsettledTotal": ret.Total - settled,
		"externalRemaining": ret.ExternalRemaining, "chargeRemaining": ret.ChargeRemaining,
		"allowedSettlementTypes": allowedTypes(ret.AllowedSettlementTypes),
	}
}

// List handles GET /api/v1/sales-returns.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c, err := listquery.ParseCommon(q, nil)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	page, limit := c.Page, c.Limit
	soID, err := listquery.ParseID(q.Get("salesOrderId"), "salesOrderId")
	if err != nil {
		response.FailErr(w, err)
		return
	}
	res, err := h.svc.ListFiltered(r.Context(), authcontracts.BranchID(r), application.ListFilter{
		Search: c.Search, Statuses: c.Statuses, SalesOrderID: soID, From: c.From, To: c.To,
	}, page, limit)
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
	if ret == nil || ret.BranchID != authcontracts.BranchID(r) {
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
		Reason           string               `json:"reason"`
		Items            []linePayload        `json:"items" validate:"required"`
		ReplacementItems []replacementPayload `json:"replacementItems"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if strings.TrimSpace(in.Reason) == "" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan retur wajib diisi"}}))
		return
	}
	lines := make([]application.LineInput, 0, len(in.Items))
	for _, l := range in.Items {
		lines = append(lines, application.LineInput{
			ProductID: l.ProductID, VariantID: l.VariantID,
			LocationID: l.LocationID, UOM: l.UOM, Qty: l.Qty, Condition: l.Condition,
		})
	}
	repl := make([]application.ReplacementInput, 0, len(in.ReplacementItems))
	for _, l := range in.ReplacementItems {
		repl = append(repl, application.ReplacementInput{
			ProductID: l.ProductID, VariantID: l.VariantID,
			LocationID: l.LocationID, UOM: l.UOM, Qty: l.Qty, UnitPrice: l.UnitPrice,
		})
	}
	ret, err := h.svc.Create(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.SalesOrderID, in.ReturnDate, in.Notes, in.ReturnMode, lines, repl,
		application.WithReason(in.Reason))
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
	// Optional body: the exchange surcharge collected with the counter hand-over
	// of a POS exchange (RT-02). A bare POST confirms as before.
	var in struct {
		Settlement *settlementPayload `json:"settlement"`
	}
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	var opts []application.ConfirmOption
	if st, ok := toReplacementSettlement(w, r, in.Settlement); !ok {
		return
	} else if st != nil {
		opts = append(opts, application.WithCounterSettlement(st))
	}
	ret, appErr := h.svc.Confirm(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, opts...)
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
	ret, appErr := h.svc.Cancel(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Retur penjualan dibatalkan", returnView(ret))
}

// replacementPayload mirrors application.ReplacementInput.
type replacementPayload struct {
	ProductID  int64   `json:"productId" validate:"required"`
	VariantID  int64   `json:"variantId" validate:"required"`
	LocationID int64   `json:"locationId"`
	UOM        string  `json:"uom" validate:"required"`
	Qty        float64 `json:"qty" validate:"required"`
	UnitPrice  *int64  `json:"unitPrice"`
}

func toReplacements(in []replacementPayload) []application.ReplacementInput {
	out := make([]application.ReplacementInput, 0, len(in))
	for _, l := range in {
		out = append(out, application.ReplacementInput{
			ProductID: l.ProductID, VariantID: l.VariantID,
			LocationID: l.LocationID, UOM: l.UOM, Qty: l.Qty, UnitPrice: l.UnitPrice,
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
		"difference": p.Difference, "owedToCustomer": p.OwedToCustomer, "chargeToCustomer": p.ChargeToCustomer,
		"orderOutstanding": p.OrderOutstanding, "receivableReduction": p.ReceivableReduction,
		"externalRemaining": p.ExternalRemaining, "allowedSettlementTypes": p.AllowedSettlementTypes,
		"hasParty": p.HasParty,
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
	ctx, err := h.svc.ReturnContext(r.Context(), authcontracts.BranchID(r), soID)
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
	p, err := h.svc.PreviewReturn(r.Context(), authcontracts.BranchID(r), in.SalesOrderID, in.ReturnMode, lines, toReplacements(in.ReplacementItems))
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
	// Driver and warehouse staff are required (RT-03) unless the company turned
	// the delivery evidence rule off; the delivery module enforces it.
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	dn, appErr := h.svc.CreateReplacementDelivery(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, application.ReplacementDispatchInput{
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
		// Settlement collects the exchange surcharge with the hand-over (K-04e).
		Settlement *settlementPayload `json:"settlement"`
	}
	// Receipt evidence optional: bare confirms post an empty body.
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	confirm := deliverycontracts.ReplacementConfirm{
		RecipientName:            in.RecipientName,
		RecipientSignatureStatus: in.RecipientSignatureStatus,
		MissingReason:            in.RecipientSignatureMissingReason,
	}
	st, ok := toReplacementSettlement(w, r, in.Settlement)
	if !ok {
		return
	}
	confirm.Settlement = st
	dn, appErr := h.svc.ConfirmReplacementDelivery(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, deliveryID, confirm)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Pengiriman pengganti dikonfirmasi", map[string]any{
		"id": dn.ID, "number": dn.Number, "status": dn.Status,
	})
}

// settlementPayload is the pay_now surcharge collected with a hand-over.
type settlementPayload struct {
	Mode           string `json:"mode"`
	Amount         int64  `json:"amount"`
	Method         string `json:"method"`
	PaymentDate    string `json:"paymentDate"`
	Reference      string `json:"reference"`
	AmountTendered int64  `json:"amountTendered"`
}

// toReplacementSettlement validates the optional pay_now body and the caller's
// right to take money: the confirm routes only demand salesreturn.create, so
// payment.create is checked here. nil, true = nothing collected now.
func toReplacementSettlement(w http.ResponseWriter, r *http.Request, st *settlementPayload) (*deliverycontracts.ReplacementSettlement, bool) {
	if st == nil {
		return nil, true
	}
	if st.Mode != "pay_now" {
		response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "settlement.mode", Message: "harus pay_now"}}))
		return nil, false
	}
	if sess := authcontracts.SessionFromContext(r.Context()); sess == nil || !sess.HasPermission("payment.create") {
		response.Fail(w, &apperror.AppError{Code: apperror.CodeForbidden, Message: "Anda tidak memiliki izin mencatat pembayaran"})
		return nil, false
	}
	return &deliverycontracts.ReplacementSettlement{
		Amount: st.Amount, Method: st.Method, PaymentDate: st.PaymentDate,
		Reference: st.Reference, AmountTendered: st.AmountTendered,
	}, true
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
	// A refund moves money out: legacy demands its own permission on top of
	// create (F3b-2). The service never sees the session, so the handler checks.
	if in.Type == "refund" {
		if sess := authcontracts.SessionFromContext(r.Context()); sess == nil || !sess.HasPermission("salesreturn.refund") {
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

// RegisterRoutes mounts salesreturn endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/sales-returns", perm("salesreturn.view", authcontracts.Guard(h.List, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns", perm("salesreturn.create", authcontracts.Guard(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/sales-returns/context", perm("salesreturn.view", authcontracts.Guard(h.ReturnContext, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/preview", perm("salesreturn.create", authcontracts.Guard(h.Preview, branchGuard)))
	mux.Handle("GET /api/v1/sales-returns/{id}", perm("salesreturn.view", authcontracts.Guard(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/confirm", perm("salesreturn.create", authcontracts.Guard(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/cancel", perm("salesreturn.archive", authcontracts.Guard(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/replacement-deliveries", perm("salesreturn.create", authcontracts.Guard(h.DispatchReplacement, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/replacement-deliveries/{deliveryId}/confirm", perm("salesreturn.create", authcontracts.Guard(h.ConfirmReplacement, branchGuard)))
	mux.Handle("POST /api/v1/sales-returns/{id}/settlements", perm("salesreturn.create", authcontracts.Guard(h.AddSettlement, branchGuard)))
}
