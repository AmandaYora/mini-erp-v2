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
		"channel": o.Channel, "memberCode": o.MemberCode,
		"orderDate": o.OrderDate, "dueDate": o.DueDate, "paymentTerms": o.PaymentTerms,
		"taxType": o.TaxType, "taxRate": o.TaxRate,
		"subtotal": o.Subtotal, "discountTotal": o.DiscountTotal,
		"taxTotal": o.TaxTotal, "grandTotal": o.GrandTotal,
		"status": o.Status, "notes": o.Notes,
		"shipToAddressId": o.ShipToAddressID, "shipToLabel": o.ShipToLabel,
		"shipToRecipient": o.ShipToRecipient, "shipToPhone": o.ShipToPhone,
		"shipToAddress":    o.ShipToAddress,
		"taxInvoiceNumber": o.TaxInvoiceNumber, "clientRef": o.ClientRef, "taxInvoiceDate": o.TaxInvoiceDate,
		"createdAt": o.CreatedAt, "createdBy": o.CreatedBy, "createdByName": o.CreatedByName,
		"assignedUserId": o.AssignedUserID, "assignedUserName": o.AssignedUserName,
		"creditApprovedBy": o.CreditApprovedBy, "creditApprovedByName": o.CreditApprovedByName,
		"creditApprovedAt": o.CreditApprovedAt, "cashierName": o.CashierName,
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
	TaxInvoiceNumber string        `json:"taxInvoiceNumber"`
	TaxInvoiceDate   string        `json:"taxInvoiceDate"`
	ClientRef        string        `json:"clientRef"`
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
		TaxInvoiceNumber: in.TaxInvoiceNumber, TaxInvoiceDate: in.TaxInvoiceDate,
		ClientRef: in.ClientRef,
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
	o, appErr := h.svc.CancelOrder(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Sales order dibatalkan", orderView(o))
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
	mux.Handle("GET /api/v1/sales-orders/{id}", perm("sales.view", authcontracts.Guard(h.Get, branchGuard)))
	mux.Handle("PUT /api/v1/sales-orders/{id}", perm("sales.update", authcontracts.Guard(h.Update, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/confirm", perm("sales.update", authcontracts.Guard(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/cancel", perm("sales.archive", authcontracts.Guard(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/approve-credit", perm("sales.approve", authcontracts.Guard(h.ApproveCredit, branchGuard)))
	mux.Handle("GET /api/v1/sales-orders/export", perm("sales.export", authcontracts.Guard(h.Export, branchGuard)))
}
