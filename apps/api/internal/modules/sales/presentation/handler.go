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

func orderView(o *contracts.SalesOrder) map[string]any {
	items := make([]any, 0, len(o.Items))
	for _, it := range o.Items {
		items = append(items, map[string]any{
			"id": it.ID, "productId": it.ProductID, "variantId": it.VariantID,
			"productCode": it.ProductCode, "productName": it.ProductName,
			"uom": it.UOM, "uomFactor": it.UOMFactor, "qty": it.Qty, "qtyBase": it.QtyBase,
			"unitPrice": it.UnitPrice, "discountPct": it.DiscountPct,
			"discountNominal": it.DiscountNominal, "lineTotal": it.LineTotal,
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
		"taxInvoiceNumber": o.TaxInvoiceNumber, "taxInvoiceDate": o.TaxInvoiceDate,
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
	TaxInvoiceNumber string        `json:"taxInvoiceNumber"`
	TaxInvoiceDate   string        `json:"taxInvoiceDate"`
	Items            []linePayload `json:"items" validate:"required"`
}

func toInput(in orderPayload) application.OrderInput {
	items := make([]application.LineInput, 0, len(in.Items))
	for _, l := range in.Items {
		items = append(items, application.LineInput{
			ProductID: l.ProductID, VariantID: l.VariantID, UOM: l.UOM, Qty: l.Qty,
			DiscountPct: l.DiscountPct, DiscountNominal: l.DiscountNominal,
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
		ShipToAddressID:  in.ShipToAddressID,
		TaxInvoiceNumber: in.TaxInvoiceNumber, TaxInvoiceDate: in.TaxInvoiceDate,
	}
}

// List handles GET /api/v1/sales-orders.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.List(r.Context(), branchID(r), q.Get("search"), q.Get("status"), q.Get("channel"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Orders))
	for _, o := range res.Orders {
		items = append(items, orderView(o))
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
	if o == nil || o.BranchID != branchID(r) {
		response.Fail(w, apperror.NotFound("Sales order"))
		return
	}
	response.OK(w, "Data sales order", orderView(o))
}

// Create handles POST /api/v1/sales-orders.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in orderPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	o, err := h.svc.CreateOrder(r.Context(), actorID(r), branchID(r), toInput(in))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Sales order dibuat", orderView(o))
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
	o, appErr := h.svc.UpdateOrder(r.Context(), actorID(r), branchID(r), id, toInput(in))
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
	o, appErr := h.svc.ConfirmOrder(r.Context(), actorID(r), branchID(r), id)
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
	o, appErr := h.svc.CancelOrder(r.Context(), actorID(r), branchID(r), id)
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
	o, appErr := h.svc.ApproveCredit(r.Context(), actorID(r), branchID(r), id, in.DueDate)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Kredit disetujui", orderView(o))
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// guarded adapts the branch guard into func-style middleware (auth →
// permission → branch → handler).
func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts sales endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/sales-orders", perm("sales.view", guarded(h.List, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders", perm("sales.create", guarded(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/sales-orders/{id}", perm("sales.view", guarded(h.Get, branchGuard)))
	mux.Handle("PUT /api/v1/sales-orders/{id}", perm("sales.update", guarded(h.Update, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/confirm", perm("sales.update", guarded(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/cancel", perm("sales.archive", guarded(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/sales-orders/{id}/approve-credit", perm("sales.approve", guarded(h.ApproveCredit, branchGuard)))
	mux.Handle("GET /api/v1/sales-orders/export", perm("sales.view", guarded(h.Export, branchGuard)))
}
