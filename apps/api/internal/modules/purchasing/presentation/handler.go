package presentation

import (
	"net/http"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/purchasing/application"
	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the purchasing module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires purchasing endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func withPayment(v map[string]any, p application.PaymentSummary) {
	v["paidAmount"], v["outstandingAmount"], v["paymentStatus"] = p.PaidAmount, p.OutstandingAmount, p.PaymentStatus
	v["returnOffset"] = p.ReturnOffset
	v["effectiveDueDate"], v["isOverdue"] = p.EffectiveDueDate, p.IsOverdue
}

func orderView(o *contracts.PurchaseOrder) map[string]any {
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
			"discountReason": it.DiscountReason, "note": it.Note,
			"taxable": it.Taxable, "taxRate": it.TaxRate, "taxBase": it.TaxBase, "taxAmount": it.TaxAmount,
		})
	}
	return map[string]any{
		"id": o.ID, "number": o.Number, "branchId": o.BranchID,
		"partyId": o.PartyID, "partyName": o.PartyName,
		"orderDate": o.OrderDate, "dueDate": o.DueDate, "paymentTerms": o.PaymentTerms,
		"taxType": o.TaxType, "taxRate": o.TaxRate,
		"subtotal": o.Subtotal, "discountTotal": o.DiscountTotal,
		"taxTotal": o.TaxTotal, "grandTotal": o.GrandTotal,
		"status": o.Status, "notes": o.Notes,
		"supplierInvoiceNumber": o.SupplierInvoiceNumber,
		"supplierInvoiceDate":   o.SupplierInvoiceDate, "paymentTermDays": o.PaymentTermDays,
		"taxInvoiceNumber": o.TaxInvoiceNumber, "taxInvoiceDate": o.TaxInvoiceDate,
		"assignedUserId": o.AssignedUserID, "assignedUserName": o.AssignedUserName,
		"createdBy": o.CreatedBy, "createdByName": o.CreatedByName,
		"items": items,
	}
}

type linePayload struct {
	ProductID       int64   `json:"productId" validate:"required"`
	VariantID       int64   `json:"variantId" validate:"required"`
	UOM             string  `json:"uom" validate:"required"`
	Qty             float64 `json:"qty" validate:"required"`
	UnitPrice       int64   `json:"unitPrice"`
	DiscountPct     float64 `json:"discountPct"`
	DiscountNominal int64   `json:"discountNominal"`
	DiscountReason  string  `json:"discountReason"`
	Note            string  `json:"note"`
	Taxable         *bool   `json:"taxable"`
	// TaxRate overrides the order rate for this line (PO-03); omitted = order rate.
	TaxRate *float64 `json:"taxRate"`
}

type orderPayload struct {
	PartyID               int64         `json:"partyId" validate:"required"`
	OrderDate             string        `json:"orderDate"`
	DueDate               string        `json:"dueDate"`
	PaymentTerms          string        `json:"paymentTerms"`
	TaxType               string        `json:"taxType"`
	TaxRate               float64       `json:"taxRate"`
	Notes                 string        `json:"notes"`
	SupplierInvoiceNumber string        `json:"supplierInvoiceNumber"`
	SupplierInvoiceDate   string        `json:"supplierInvoiceDate"`
	TaxInvoiceNumber      string        `json:"taxInvoiceNumber"`
	TaxInvoiceDate        string        `json:"taxInvoiceDate"`
	AssignedUserID        int64         `json:"assignedUserId"`
	PaymentTermDays       int           `json:"paymentTermDays"`
	Items                 []linePayload `json:"items" validate:"required"`
}

func toInput(in orderPayload) application.OrderInput {
	items := make([]application.LineInput, 0, len(in.Items))
	for _, l := range in.Items {
		items = append(items, application.LineInput{
			ProductID: l.ProductID, VariantID: l.VariantID, UOM: l.UOM, Qty: l.Qty,
			UnitPrice: l.UnitPrice, DiscountPct: l.DiscountPct, DiscountNominal: l.DiscountNominal,
			DiscountReason: l.DiscountReason, Note: l.Note, Taxable: l.Taxable, TaxRate: l.TaxRate,
		})
	}
	terms := in.PaymentTerms
	if terms == "" {
		terms = "net"
	}
	taxType := in.TaxType
	if taxType == "" {
		taxType = "none"
	}
	return application.OrderInput{
		PartyID: in.PartyID, OrderDate: in.OrderDate, DueDate: in.DueDate,
		PaymentTerms: terms, TaxType: taxType, TaxRate: in.TaxRate,
		Notes: in.Notes, Items: items,
		SupplierInvoiceNumber: in.SupplierInvoiceNumber,
		SupplierInvoiceDate:   in.SupplierInvoiceDate,
		TaxInvoiceNumber:      in.TaxInvoiceNumber,
		TaxInvoiceDate:        in.TaxInvoiceDate,
		AssignedUserID:        in.AssignedUserID,
		PaymentTermDays:       in.PaymentTermDays,
	}
}

// List handles GET /api/v1/purchase-orders.
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
	if perr != nil {
		response.FailErr(w, perr)
		return
	}
	returns, rerr := h.svc.ReturnSummaries(r.Context(), res.Orders)
	if rerr != nil {
		response.FailErr(w, rerr)
		return
	}
	items := make([]any, 0, len(res.Orders))
	for _, o := range res.Orders {
		v := orderView(o)
		withPayment(v, sums[o.ID])
		v["returnSummary"] = returns[o.ID]
		items = append(items, v)
	}
	response.Paginated(w, "Data purchase order", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// Get handles GET /api/v1/purchase-orders/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Purchase order"))
		return
	}
	o, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if o == nil || o.BranchID != authcontracts.BranchID(r) {
		response.Fail(w, apperror.NotFound("Purchase order"))
		return
	}
	v := orderView(o)
	if sums, perr := h.svc.PaymentSummaries(r.Context(), []*contracts.PurchaseOrder{o}); perr == nil {
		withPayment(v, sums[o.ID])
	}
	if rs, rerr := h.svc.ReturnSummaries(r.Context(), []*contracts.PurchaseOrder{o}); rerr == nil {
		v["returnSummary"] = rs[o.ID]
	}
	response.OK(w, "Data purchase order", v)
}

// Create handles POST /api/v1/purchase-orders.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in orderPayload
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	o, err := h.svc.CreateOrder(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), toInput(in))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Purchase order dibuat", orderView(o))
}

// Update handles PUT /api/v1/purchase-orders/{id}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Purchase order"))
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
	response.OK(w, "Purchase order disimpan", orderView(o))
}

// Confirm handles POST /api/v1/purchase-orders/{id}/confirm.
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Purchase order"))
		return
	}
	o, appErr := h.svc.ConfirmOrder(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Purchase order dikonfirmasi", orderView(o))
}

// Cancel handles POST /api/v1/purchase-orders/{id}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Purchase order"))
		return
	}
	o, appErr := h.svc.CancelOrder(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Purchase order dibatalkan", orderView(o))
}

// RegisterRoutes mounts purchasing endpoints with explicit permissions.
// Every route is branch-guarded: cross-branch document access reads as missing.
func RegisterRoutes(mux *http.ServeMux, h *Handler, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/purchase-orders", perm("purchasing.view", authcontracts.Guard(h.List, branchGuard)))
	mux.Handle("POST /api/v1/purchase-orders", perm("purchasing.create", authcontracts.Guard(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/purchase-orders/{id}", perm("purchasing.view", authcontracts.Guard(h.Get, branchGuard)))
	mux.Handle("PUT /api/v1/purchase-orders/{id}", perm("purchasing.update", authcontracts.Guard(h.Update, branchGuard)))
	mux.Handle("POST /api/v1/purchase-orders/{id}/confirm", perm("purchasing.update", authcontracts.Guard(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/purchase-orders/{id}/cancel", perm("purchasing.archive", authcontracts.Guard(h.Cancel, branchGuard)))
	mux.Handle("GET /api/v1/purchase-orders/export", perm("purchasing.export", authcontracts.Guard(h.Export, branchGuard)))
}
