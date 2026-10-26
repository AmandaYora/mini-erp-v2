package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/sales/application"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
	"mini-erp/internal/shared/timeutil"
)

// Handler serves the sales module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires sales endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func orderView(o *contracts.SalesOrder) map[string]any {
	items := make([]any, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, map[string]any{
			"id": it.ID, "productId": it.ProductID, "variantId": it.VariantID,
			"productCode": it.ProductCode, "productName": it.ProductName,
			"uom": it.UOM, "uomFactor": it.UOMFactor, "qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "discountPct": it.DiscountPct,
			"discountNominal": it.DiscountNominal, "lineTotal": it.LineTotal,
			"stockTracked": it.StockTracked, "itemType": it.ItemType,
			"variantName": it.VariantName, "variantCode": it.VariantCode, "baseUom": it.BaseUOM,
			"pricingSource": it.PricingSource, "basePrice": it.BasePrice,
			"discountReason": it.DiscountReason, "note": it.Note,
			"taxable": it.Taxable, "taxBase": it.TaxBase, "taxAmount": it.TaxAmount,
		})
	}
	return map[string]any{
		"id": o.ID, "number": o.Number, "branchId": o.BranchID,
		"partyId": o.PartyID, "partyName": o.PartyName,
		"partyPhone": o.PartyPhone, "partyAddress": o.PartyAddress,
		"channel": o.Channel, "origin": o.Origin, "memberCode": o.MemberCode,
		"orderDate": o.OrderDate, "dueDate": o.DueDate, "paymentTerms": o.PaymentTerms,
		"receiptAccountId": o.ReceiptAccountID,
		"taxType":          o.TaxType, "taxRate": o.TaxRate,
		"subtotal": o.Subtotal, "discountTotal": o.DiscountTotal,
		"taxTotal": o.TaxTotal, "grandTotal": o.GrandTotal,
		"status": o.Status, "notes": o.Notes,
		"shipToAddressId": o.ShipToAddressID, "shipToLabel": o.ShipToLabel,
		"shipToRecipient": o.ShipToRecipient, "shipToPhone": o.ShipToPhone,
		"shipToAddress":    o.ShipToAddress,
		"taxInvoiceNumber": o.TaxInvoiceNumber, "clientRef": o.ClientRef, "taxInvoiceDate": o.TaxInvoiceDate,
		"createdAt": timeutil.FormatWIB(o.CreatedAt), "createdBy": o.CreatedBy, "createdByName": o.CreatedByName,
		"assignedUserId": o.AssignedUserID, "assignedUserName": o.AssignedUserName,
		"creditApprovedBy": o.CreditApprovedBy, "creditApprovedByName": o.CreditApprovedByName,
		"creditApprovedAt": timeutil.FormatWIB(o.CreditApprovedAt), "cashierName": o.CashierName,
		"cancelReason": o.CancelReason, "cancelledAt": timeutil.FormatWIB(o.CancelledAt),
		"cancelledBy": o.CancelledBy, "cancelledByName": o.CancelledByName,
		"shortCloseReason": o.ShortCloseReason, "shortClosedAt": timeutil.FormatWIB(o.ShortClosedAt),
		"shortClosedBy": o.ShortClosedBy, "shortClosedByName": o.ShortClosedByName,
		"items": items,
	}
}

type linePayload struct {
	ProductID       int64   `json:"productId" validate:"required"`
	VariantID       int64   `json:"variantId" validate:"required"`
	UOM             string  `json:"uom" validate:"required"`
	Qty             float64 `json:"qty" validate:"required"`
	DiscountPct     float64 `json:"discountPct"`
	DiscountNominal int64   `json:"discountNominal"`
	UnitPrice       *int64  `json:"unitPrice"`
	DiscountReason  string  `json:"discountReason"`
	Note            string  `json:"note"`
	Taxable         *bool   `json:"taxable"`
}

type orderPayload struct {
	PartyID          int64         `json:"partyId"`
	Channel          string        `json:"channel"`
	OrderDate        string        `json:"orderDate"`
	DueDate          string        `json:"dueDate"`
	PaymentTerms     string        `json:"paymentTerms"`
	TaxType          string        `json:"taxType"`
	TaxRate          float64       `json:"taxRate"`
	Notes            string        `json:"notes"`
	ShipToAddressID  int64         `json:"shipToAddressId"`
	ShipToLabel      string        `json:"shipToLabel"`
	ShipToRecipient  string        `json:"shipToRecipient"`
	ShipToPhone      string        `json:"shipToPhone"`
	ShipToAddress    string        `json:"shipToAddress"`
	AssignedUserID   int64         `json:"assignedUserId"`
	ReceiptAccountID int64         `json:"receiptAccountId"`
	TaxInvoiceNumber string        `json:"taxInvoiceNumber"`
	TaxInvoiceDate   string        `json:"taxInvoiceDate"`
	ClientRef        string        `json:"clientRef"`
	Reason           string        `json:"reason"`     // optional why of a correction (audit trail only)
	HeaderOnly       bool          `json:"headerOnly"` // confirmed order: header-light fields only
	Items            []linePayload `json:"items" validate:"required"`
}

func toInput(in orderPayload) application.OrderInput {
	items := make([]application.LineInput, 0, len(in.Items))
	for _, l := range in.Items {
		items = append(items, application.LineInput{
			ProductID: l.ProductID, VariantID: l.VariantID, UOM: l.UOM, Qty: l.Qty,
			DiscountPct: l.DiscountPct, DiscountNominal: l.DiscountNominal,
			UnitPrice: l.UnitPrice, DiscountReason: l.DiscountReason, Note: l.Note, Taxable: l.Taxable,
		})
	}
	terms := in.PaymentTerms
	if terms == "" {
		terms = "cod"
	}
	taxType := in.TaxType
	if taxType == "" {
		taxType = "none"
	}
	return application.OrderInput{
		PartyID: in.PartyID, Channel: in.Channel, OrderDate: in.OrderDate,
		DueDate: in.DueDate, PaymentTerms: terms, TaxType: taxType,
		TaxRate: in.TaxRate, Notes: in.Notes, Items: items,
		ShipToAddressID: in.ShipToAddressID,
		ShipToLabel:     in.ShipToLabel, ShipToRecipient: in.ShipToRecipient,
		ShipToPhone: in.ShipToPhone, ShipToAddress: in.ShipToAddress,
		AssignedUserID:   in.AssignedUserID,
		ReceiptAccountID: in.ReceiptAccountID,
		TaxInvoiceNumber: in.TaxInvoiceNumber, TaxInvoiceDate: in.TaxInvoiceDate,
		ClientRef: in.ClientRef, Reason: in.Reason, HeaderOnly: in.HeaderOnly,
	}
}

// List handles GET /api/v1/sales-orders.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	f, c, err := parseListFilter(r)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	page, limit := c.Page, c.Limit
	res, err := h.svc.List(r.Context(), authcontracts.BranchID(r), f, page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	sums, perr := h.svc.PaymentSummaries(r.Context(), res.Orders)
	if perr == nil {
		perr = h.svc.DecorateList(r.Context(), res.Orders)
	}
	if perr != nil {
		response.FailErr(w, perr)
		return
	}
	items := make([]any, 0, len(res.Orders))
	for _, o := range res.Orders {
		v := orderView(o)
		withPayment(v, sums[o.ID])
		items = append(items, v)
	}
	response.Paginated(w, "Data sales order", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// Get handles GET /api/v1/sales-orders/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	o, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if o == nil || o.BranchID != authcontracts.BranchID(r) {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	if derr := h.svc.DecorateDetail(r.Context(), []*contracts.SalesOrder{o}); derr != nil {
		response.FailErr(w, derr)
		return
	}
	v := orderView(o)
	if sums, perr := h.svc.PaymentSummaries(r.Context(), []*contracts.SalesOrder{o}); perr == nil {
		withPayment(v, sums[o.ID])
	}
	corr, cerr := h.svc.Correction(r.Context(), o)
	if cerr != nil {
		response.FailErr(w, cerr)
		return
	}
	v["editMode"], v["editLockReason"], v["canCloseShort"] = corr.EditMode, corr.EditLockReason, corr.CanCloseShort
	response.OK(w, "Data sales order", v)
}

func withPayment(v map[string]any, p application.PaymentSummary) {
	v["paidAmount"], v["outstandingAmount"], v["paymentStatus"] = p.PaidAmount, p.OutstandingAmount, p.PaymentStatus
	v["effectiveDueDate"], v["isOverdue"] = p.EffectiveDueDate, p.IsOverdue
	v["originalTotal"], v["returnAdjustment"], v["netTotal"] = p.OriginalTotal, p.ReturnAdjustment, p.NetTotal
	v["returnOffset"], v["returnRefunded"], v["returnCredit"] = p.ReturnOffset, p.ReturnRefunded, p.ReturnCredit
	v["refundDue"], v["exchangeChargeDue"], v["financialStatus"] = p.RefundDue, p.ExchangeChargeDue, p.FinancialStatus
}

// Create handles POST /api/v1/sales-orders.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in orderPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	o, replayed, err := h.svc.CreateOrderIdempotent(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), toInput(in))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	view := orderView(o)
	if replayed {
		view["replayed"] = true
		response.OK(w, "Sales order sudah dibuat sebelumnya", view)
		return
	}
	response.Created(w, "Sales order dibuat", view)
}

// State handles GET /api/v1/sales-orders/by-client-ref/{ref}/state: where a
// client cart (POS) stands, so it can resume from nextStep.
func (h *Handler) State(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.StateByClientRef(r.Context(), authcontracts.BranchID(r), r.PathValue("ref"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Status keranjang", map[string]any{
		"order": orderView(st.Order), "deliveries": st.Deliveries,
		"paidAmount": st.PaidAmount, "outstanding": st.Outstanding, "nextStep": st.NextStep,
	})
}

// Update handles PUT /api/v1/sales-orders/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	var in orderPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	o, appErr := h.svc.UpdateOrder(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, toInput(in))
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Sales order disimpan", orderView(o))
}

// SetReceiptAccount handles PUT /api/v1/sales-orders/{id}/receipt-account:
// the bill's rekening tujuan only (ADR-0026), completed orders included.
func (h *Handler) SetReceiptAccount(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	var in struct {
		ReceiptAccountID int64 `json:"receiptAccountId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	o, appErr := h.svc.SetReceiptAccount(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, in.ReceiptAccountID)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Rekening tujuan disimpan", orderView(o))
}

// Confirm handles POST /api/v1/sales-orders/{id}/confirm.
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	o, appErr := h.svc.ConfirmOrder(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Sales order dikonfirmasi", orderView(o))
}

// Cancel handles POST /api/v1/sales-orders/{id}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	// The body is optional (older clients send none); the web UI always sends
	// a reason.
	var in reasonPayload
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	o, appErr := h.svc.CancelOrderWithReason(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, in.Reason)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Sales order dibatalkan", orderView(o))
}

type reasonPayload struct {
	Reason string `json:"reason"`
}

// ShortClosePreview handles GET /api/v1/sales-orders/{id}/close-short/preview:
// what "tutup sisa order" would do (lines cut to delivered qty, new total,
// refund due), without writing.
func (h *Handler) ShortClosePreview(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	plan, appErr := h.svc.ShortClosePreview(r.Context(), authcontracts.BranchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Pratinjau tutup sisa order", plan)
}

// ShortClose handles POST /api/v1/sales-orders/{id}/close-short {reason}.
func (h *Handler) ShortClose(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	var in reasonPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	o, appErr := h.svc.ShortClose(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, in.Reason)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Sisa order ditutup", orderView(o))
}

// fromDeliveriesPayload is the body of preview and create "order dari surat
// jalan" (ADR-0022): the notes, the header, prices per note line, extra
// non-stock lines and the cash settlement.
type fromDeliveriesPayload struct {
	DeliveryIDs      []int64 `json:"deliveryIds" validate:"required"`
	OrderDate        string  `json:"orderDate"`
	PaymentTerms     string  `json:"paymentTerms"`
	DueDate          string  `json:"dueDate"`
	TaxType          string  `json:"taxType"`
	TaxRate          float64 `json:"taxRate"`
	Notes            string  `json:"notes"`
	AssignedUserID   int64   `json:"assignedUserId"`
	ReceiptAccountID int64   `json:"receiptAccountId"`
	TaxInvoiceNumber string  `json:"taxInvoiceNumber"`
	TaxInvoiceDate   string  `json:"taxInvoiceDate"`
	Lines            []struct {
		ProductID       int64   `json:"productId" validate:"required"`
		VariantID       int64   `json:"variantId" validate:"required"`
		UOM             string  `json:"uom" validate:"required"`
		UnitPrice       *int64  `json:"unitPrice"`
		DiscountPct     float64 `json:"discountPct"`
		DiscountNominal int64   `json:"discountNominal"`
		DiscountReason  string  `json:"discountReason"`
		Note            string  `json:"note"`
		Taxable         *bool   `json:"taxable"`
	} `json:"lines"`
	ExtraItems []linePayload `json:"extraItems"`
	Settlement *struct {
		Mode           string `json:"mode"`
		Method         string `json:"method"`
		PaymentDate    string `json:"paymentDate"`
		Reference      string `json:"reference"`
		AmountTendered int64  `json:"amountTendered"`
		DueDate        string `json:"dueDate"`
		CashAccountID  int64  `json:"cashAccountId"`
	} `json:"settlement"`
	ClientRef string `json:"clientRef"`
}

func toFromDeliveriesInput(in fromDeliveriesPayload) application.FromDeliveriesInput {
	out := application.FromDeliveriesInput{
		DeliveryIDs: in.DeliveryIDs, OrderDate: in.OrderDate, PaymentTerms: in.PaymentTerms, DueDate: in.DueDate,
		TaxType: in.TaxType, TaxRate: in.TaxRate, Notes: in.Notes, AssignedUserID: in.AssignedUserID,
		ReceiptAccountID: in.ReceiptAccountID,
		TaxInvoiceNumber: in.TaxInvoiceNumber, TaxInvoiceDate: in.TaxInvoiceDate, ClientRef: in.ClientRef,
	}
	for _, l := range in.Lines {
		out.Lines = append(out.Lines, application.FromDeliveriesLine{
			ProductID: l.ProductID, VariantID: l.VariantID, UOM: l.UOM, UnitPrice: l.UnitPrice,
			DiscountPct: l.DiscountPct, DiscountNominal: l.DiscountNominal,
			DiscountReason: l.DiscountReason, Note: l.Note, Taxable: l.Taxable,
		})
	}
	for _, e := range in.ExtraItems {
		out.ExtraItems = append(out.ExtraItems, application.LineInput{
			ProductID: e.ProductID, VariantID: e.VariantID, UOM: e.UOM, Qty: e.Qty,
			DiscountPct: e.DiscountPct, DiscountNominal: e.DiscountNominal,
			UnitPrice: e.UnitPrice, DiscountReason: e.DiscountReason, Note: e.Note, Taxable: e.Taxable,
		})
	}
	if in.Settlement != nil {
		out.Settlement = &application.FromDeliveriesSettlement{
			Mode: in.Settlement.Mode, Method: in.Settlement.Method, PaymentDate: in.Settlement.PaymentDate,
			Reference: in.Settlement.Reference, AmountTendered: in.Settlement.AmountTendered, DueDate: in.Settlement.DueDate,
			CashAccountID: in.Settlement.CashAccountID,
		}
	}
	return out
}

// FromDeliveriesPreview handles POST /api/v1/sales-orders/from-deliveries/preview:
// the priced order the notes would produce, without writing.
func (h *Handler) FromDeliveriesPreview(w http.ResponseWriter, r *http.Request) {
	var in fromDeliveriesPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	pv, err := h.svc.PreviewFromDeliveries(r.Context(), authcontracts.BranchID(r), toFromDeliveriesInput(in))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pratinjau order dari surat jalan", fromDeliveriesPreviewView(pv))
}

func fromDeliveriesPreviewView(pv *application.FromDeliveriesPreview) map[string]any {
	items := make([]any, 0, len(pv.Items))
	for _, it := range pv.Items {
		items = append(items, map[string]any{
			"productId": it.ProductID, "variantId": it.VariantID,
			"productCode": it.ProductCode, "productName": it.ProductName,
			"variantName": it.VariantName, "variantCode": it.VariantCode,
			"uom": it.UOM, "uomFactor": it.UOMFactor, "qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "discountPct": it.DiscountPct, "discountNominal": it.DiscountNominal,
			"lineTotal": it.LineTotal, "stockTracked": it.StockTracked, "itemType": it.ItemType,
			"pricingSource": it.PricingSource, "basePrice": it.BasePrice,
			"discountReason": it.DiscountReason, "note": it.Note,
			"taxable": it.Taxable, "taxBase": it.TaxBase, "taxAmount": it.TaxAmount,
		})
	}
	return map[string]any{
		"partyId": pv.PartyID, "partyName": pv.PartyName,
		"shipToLabel": pv.ShipToLabel, "shipToRecipient": pv.ShipToRecipient,
		"shipToPhone": pv.ShipToPhone, "shipToAddress": pv.ShipToAddress,
		"deliveryDate": pv.DeliveryDate, "deliveries": pv.Notes, "items": items,
		"subtotal": pv.Subtotal, "discountTotal": pv.DiscountTotal, "taxTotal": pv.TaxTotal, "grandTotal": pv.GrandTotal,
		"warnings": pv.Warnings,
	}
}

// FromDeliveries handles POST /api/v1/sales-orders/from-deliveries: builds,
// confirms, links, settles and completes the order in one idempotent call.
// Paying on the spot needs payment.create; converting to net terms needs
// sales.approve (the same rules as the delivery COD settlement).
func (h *Handler) FromDeliveries(w http.ResponseWriter, r *http.Request) {
	var in fromDeliveriesPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	sess := authcontracts.SessionFromContext(r.Context())
	allowed := func(mode string) bool {
		if sess == nil {
			return false
		}
		if mode == application.SettleSwitchToNet {
			return sess.HasPermission("sales.approve")
		}
		return sess.HasPermission("payment.create")
	}
	res, err := h.svc.CreateFromDeliveries(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), toFromDeliveriesInput(in), allowed)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	view := orderView(res.Order)
	if len(res.Warnings) > 0 {
		view["warnings"] = res.Warnings
	}
	if res.Replayed {
		view["replayed"] = true
		response.OK(w, "Order dari surat jalan sudah dibuat sebelumnya", view)
		return
	}
	response.Created(w, "Order dibuat dari surat jalan dan selesai", view)
}

// ApproveCredit handles POST /api/v1/sales-orders/{id}/approve-credit.
func (h *Handler) ApproveCredit(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	var in struct {
		DueDate string `json:"dueDate" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	o, appErr := h.svc.ApproveCredit(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, in.DueDate)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Kredit disetujui", orderView(o))
}

// RegisterRoutes mounts sales endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/sales-orders", perm("sales.view", authcontracts.Guard(h.List, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders", perm("sales.create", authcontracts.Guard(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/sales-orders/by-client-ref/{ref}/state", perm("sales.view", authcontracts.Guard(h.State, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/from-deliveries/preview", perm("sales.create", authcontracts.Guard(h.FromDeliveriesPreview, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/from-deliveries", perm("sales.create", authcontracts.Guard(h.FromDeliveries, branchGuard)))
	mux.Handle("GET /api/v1/sales-orders/{id}", perm("sales.view", authcontracts.Guard(h.Get, branchGuard)))
	mux.Handle("PUT /api/v1/sales-orders/{id}", perm("sales.update", authcontracts.Guard(h.Update, branchGuard)))
	mux.Handle("PUT /api/v1/sales-orders/{id}/receipt-account", perm("sales.update", authcontracts.Guard(h.SetReceiptAccount, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/confirm", perm("sales.update", authcontracts.Guard(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/cancel", perm("sales.archive", authcontracts.Guard(h.Cancel, branchGuard)))
	mux.Handle("GET /api/v1/sales-orders/{id}/close-short/preview", perm("sales.archive", authcontracts.Guard(h.ShortClosePreview, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/close-short", perm("sales.archive", authcontracts.Guard(h.ShortClose, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/approve-credit", perm("sales.approve", authcontracts.Guard(h.ApproveCredit, branchGuard)))
	mux.Handle("GET /api/v1/sales-orders/export", perm("sales.export", authcontracts.Guard(h.Export, branchGuard)))
}
