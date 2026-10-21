package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/delivery/application"
	"mini-erp/internal/modules/delivery/contracts"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/listquery"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
	"mini-erp/internal/shared/timeutil"
)

// Handler serves the delivery module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires delivery endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func noteView(n *contracts.DeliveryNote) map[string]any {
	items := make([]any, 0, len(n.Items))
	for _, it := range n.Items {
		items = append(items, map[string]any{
			"id": it.ID, "productId": it.ProductID, "variantId": it.VariantID,
			"locationId": it.LocationID, "uom": it.UOM, "uomFactor": it.UOMFactor,
			"qty": it.Qty, "qtyBase": it.QtyBase,
		})
	}
	return map[string]any{
		"id": n.ID, "number": n.Number, "branchId": n.BranchID,
		"salesOrderId": n.SalesOrderID, "orderNumber": n.OrderNumber,
		"deliveryDate": n.DeliveryDate, "status": n.Status, "notes": n.Notes,
		"driverName": n.DriverName, "vehiclePlate": n.VehiclePlate,
		"warehouseStaffName":              n.WarehouseStaffName,
		"recipientName":                   n.RecipientName,
		"recipientSignatureStatus":        n.RecipientSignatureStatus,
		"recipientSignatureMissingReason": n.RecipientSignatureMissingReason,
		"dropLocationNote":                n.DropLocationNote,
		"dispatchedAt":                    timeutil.FormatWIB(n.DispatchedAt), "dispatchedBy": n.DispatchedBy,
		"confirmedAt": timeutil.FormatWIB(n.ConfirmedAt), "confirmedBy": n.ConfirmedBy,
		"documentKind": n.DocumentKind, "salesReturnId": n.SalesReturnID,
		"cancelReason": n.CancelReason, "cancelledAt": timeutil.FormatWIB(n.CancelledAt), "cancelledBy": n.CancelledBy,
		// SJ Langsung (ADR-0022): its own customer/ship-to while it has no order.
		"origin": n.Origin, "partyId": n.PartyID, "partyName": n.PartyName,
		"shipToLabel": n.ShipToLabel, "shipToRecipient": n.ShipToRecipient,
		"shipToPhone": n.ShipToPhone, "shipToAddress": n.ShipToAddress,
		"linkedAt": timeutil.FormatWIB(n.LinkedAt), "linkedBy": n.LinkedBy,
		"items": items,
	}
}

// lineInputs maps the shared line payload of Create / CreateDirect.
type linePayload struct {
	ProductID      int64   `json:"productId" validate:"required"`
	VariantID      int64   `json:"variantId" validate:"required"`
	LocationID     int64   `json:"locationId"`
	UOM            string  `json:"uom" validate:"required"`
	Qty            float64 `json:"qty" validate:"required"`
	ReservationKey string  `json:"reservationKey"`
}

func lineInputs(items []linePayload) []application.LineInput {
	lines := make([]application.LineInput, 0, len(items))
	for _, l := range items {
		lines = append(lines, application.LineInput{
			ProductID: l.ProductID, VariantID: l.VariantID,
			LocationID: l.LocationID, UOM: l.UOM, Qty: l.Qty, ReservationKey: l.ReservationKey,
		})
	}
	return lines
}

// CreateDirect handles POST /api/v1/deliveries/direct: a surat jalan for a
// customer with no order yet (SJ Langsung, ADR-0022).
func (h *Handler) CreateDirect(w http.ResponseWriter, r *http.Request) {
	var in struct {
		PartyID            int64         `json:"partyId" validate:"required"`
		DeliveryDate       string        `json:"deliveryDate"`
		ShipToAddressID    int64         `json:"shipToAddressId"`
		ShipToLabel        string        `json:"shipToLabel"`
		ShipToRecipient    string        `json:"shipToRecipient"`
		ShipToPhone        string        `json:"shipToPhone"`
		ShipToAddress      string        `json:"shipToAddress"`
		DriverName         string        `json:"driverName"`
		VehiclePlate       string        `json:"vehiclePlate"`
		WarehouseStaffName string        `json:"warehouseStaffName"`
		DropLocationNote   string        `json:"dropLocationNote"`
		Notes              string        `json:"notes"`
		ClientRef          string        `json:"clientRef"`
		Items              []linePayload `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	n, replayed, err := h.svc.CreateDirect(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), application.DirectInput{
		PartyID: in.PartyID, DeliveryDate: in.DeliveryDate, Notes: in.Notes,
		ShipToAddressID: in.ShipToAddressID, ShipToLabel: in.ShipToLabel, ShipToRecipient: in.ShipToRecipient,
		ShipToPhone: in.ShipToPhone, ShipToAddress: in.ShipToAddress,
		Doc: application.DocInput{
			DriverName: in.DriverName, VehiclePlate: in.VehiclePlate,
			WarehouseStaffName: in.WarehouseStaffName, DropLocationNote: in.DropLocationNote,
			ClientRef: in.ClientRef,
		},
		Lines: lineInputs(in.Items),
	})
	if err != nil {
		response.FailErr(w, err)
		return
	}
	view := noteView(n)
	if len(n.Warnings) > 0 {
		view["warnings"] = n.Warnings
	}
	if replayed {
		view["replayed"] = true
		response.OK(w, "Surat jalan langsung sudah dibuat sebelumnya", view)
		return
	}
	response.Created(w, "Surat jalan langsung dibuat", view)
}

// OpenDirect handles GET /api/v1/deliveries/open-direct?partyId&limit: the
// confirmed SJ Langsung still waiting for their order (oldest first) plus the
// count and the oldest delivery date — what the order form and the dashboard
// read. limit=0 returns the summary only.
func (h *Handler) OpenDirect(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	partyID, err := listquery.ParseID(q.Get("partyId"), "partyId")
	if err != nil {
		response.FailErr(w, err)
		return
	}
	limit := pagination.ParseDisplayLimit(q.Get("limit"), 50, 200)
	if q.Get("limit") == "0" {
		limit = 0 // summary only (dashboard counter)
	}
	branchID := authcontracts.BranchID(r)
	sum, serr := h.svc.OpenDirectSummary(r.Context(), branchID, partyID)
	if serr != nil {
		response.FailErr(w, serr)
		return
	}
	items := []any{}
	if limit > 0 && sum.Count > 0 {
		notes, lerr := h.svc.ListOpenDirect(r.Context(), branchID, partyID, limit)
		if lerr != nil {
			response.FailErr(w, lerr)
			return
		}
		for _, n := range notes {
			items = append(items, noteView(n))
		}
	}
	response.OK(w, "Surat jalan langsung menunggu order", map[string]any{
		"items": items, "count": sum.Count, "oldestDate": sum.OldestDate,
	})
}

// List handles GET /api/v1/deliveries.
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
	srID, err := listquery.ParseID(q.Get("salesReturnId"), "salesReturnId")
	if err != nil {
		response.FailErr(w, err)
		return
	}
	if err := listquery.ValidateEnum("origin", q.Get("origin"), contracts.OriginOrder, contracts.OriginDirect); err != nil {
		response.FailErr(w, err)
		return
	}
	res, err := h.svc.ListFiltered(r.Context(), authcontracts.BranchID(r), application.ListFilter{
		Search: c.Search, Statuses: c.Statuses, SalesOrderID: soID, SalesReturnID: srID,
		DocumentKind: q.Get("documentKind"), From: c.From, To: c.To,
		Origin: q.Get("origin"), Unordered: q.Get("unordered") == "1" || q.Get("unordered") == "true",
	}, page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Notes))
	for _, n := range res.Notes {
		items = append(items, noteView(n))
	}
	response.Paginated(w, "Data surat jalan", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// Get handles GET /api/v1/deliveries/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Surat jalan"))
		return
	}
	n, appErr := h.svc.GetByID(r.Context(), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	if n == nil || n.BranchID != authcontracts.BranchID(r) {
		response.Fail(w, apperror.NotFound("Surat jalan"))
		return
	}
	response.OK(w, "Data surat jalan", noteView(n))
}

// Create handles POST /api/v1/deliveries.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SalesOrderID       int64         `json:"salesOrderId" validate:"required"`
		DeliveryDate       string        `json:"deliveryDate"`
		DriverName         string        `json:"driverName"`
		VehiclePlate       string        `json:"vehiclePlate"`
		WarehouseStaffName string        `json:"warehouseStaffName"`
		DropLocationNote   string        `json:"dropLocationNote"`
		Notes              string        `json:"notes"`
		ClientRef          string        `json:"clientRef"`
		Items              []linePayload `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	n, replayed, err := h.svc.CreateIdempotent(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.SalesOrderID, in.DeliveryDate, in.Notes, application.DocInput{
		DriverName: in.DriverName, VehiclePlate: in.VehiclePlate,
		WarehouseStaffName: in.WarehouseStaffName, DropLocationNote: in.DropLocationNote,
		ClientRef: in.ClientRef,
	}, lineInputs(in.Items))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	view := noteView(n)
	if len(n.Warnings) > 0 {
		view["warnings"] = n.Warnings
	}
	if replayed {
		view["replayed"] = true
		response.OK(w, "Surat jalan sudah dibuat sebelumnya", view)
		return
	}
	response.Created(w, "Surat jalan dibuat", view)
}

// Confirm handles POST /api/v1/deliveries/{id}/confirm.
func (h *Handler) Confirm(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Surat jalan"))
		return
	}
	var in struct {
		RecipientName                   string `json:"recipientName"`
		RecipientSignatureStatus        string `json:"recipientSignatureStatus"`
		RecipientSignatureMissingReason string `json:"recipientSignatureMissingReason"`
		Settlement                      *struct {
			Mode           string `json:"mode"`
			Method         string `json:"method"`
			PaymentDate    string `json:"paymentDate"`
			Reference      string `json:"reference"`
			AmountTendered int64  `json:"amountTendered"`
			DueDate        string `json:"dueDate"`
		} `json:"settlement"`
	}
	// Receipt evidence is optional: bare confirms (POS flow, existing
	// callers) post an empty body.
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	ci := application.ConfirmInput{
		RecipientName: in.RecipientName, RecipientSignatureStatus: in.RecipientSignatureStatus,
		RecipientSignatureMissingReason: in.RecipientSignatureMissingReason,
	}
	if in.Settlement != nil {
		// Paying on the spot needs payment.create; converting to net terms
		// needs the credit-approval permission (legacy parity).
		sess := authcontracts.SessionFromContext(r.Context())
		need := "payment.create"
		if in.Settlement.Mode == application.SettleSwitchToNet {
			need = "sales.approve"
		}
		posOnly := false
		if sess != nil && !sess.HasPermission(need) && need == "payment.create" && sess.HasPermission("sales.create") {
			// The cashier of a POS counter takes the money on the spot; the
			// service refuses this for any order that is not a POS order.
			posOnly = true
		} else if sess == nil || !sess.HasPermission(need) {
			response.Fail(w, apperror.Forbidden())
			return
		}
		ci.Settlement = &application.Settlement{PosOnly: posOnly,
			Mode: in.Settlement.Mode, Method: in.Settlement.Method, PaymentDate: in.Settlement.PaymentDate,
			Reference: in.Settlement.Reference, AmountTendered: in.Settlement.AmountTendered, DueDate: in.Settlement.DueDate,
		}
	}
	n, appErr := h.svc.Confirm(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, ci)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Surat jalan dikonfirmasi", noteView(n))
}

// Cancel handles POST /api/v1/deliveries/{id}/cancel {reason} (reason mandatory).
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Surat jalan"))
		return
	}
	reason, err := httpx.DecodeReason(r)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	n, appErr := h.svc.Cancel(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, reason)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Surat jalan dibatalkan", noteView(n))
}

// ProofUpload handles POST /api/v1/deliveries/{id}/proof (multipart image).
func (h *Handler) ProofUpload(w http.ResponseWriter, r *http.Request, media mediacontracts.MediaClient) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Surat jalan"))
		return
	}
	fh, header, appErr := httpx.MultipartFile(r, 6<<20)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	defer func() { _ = fh.Close() }()
	url, svcErr := h.svc.Proof(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, r.FormValue("purpose"), mediacontracts.Upload{
		OriginalName: header.Filename, MIME: header.Header.Get("Content-Type"),
		Content: fh, Accept: []string{"image/"},
	})
	if svcErr != nil {
		response.FailErr(w, svcErr)
		return
	}
	response.Created(w, "Bukti kirim diunggah", map[string]any{"url": url})
}

// Proofs handles GET /api/v1/deliveries/{id}/proofs.
func (h *Handler) Proofs(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Surat jalan"))
		return
	}
	proofs, appErr := h.svc.Proofs(r.Context(), authcontracts.BranchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	items := make([]any, 0, len(proofs))
	for _, p := range proofs {
		items = append(items, p)
	}
	response.OK(w, "Bukti kirim", items)
}

// WorkQueue handles GET /api/v1/deliveries/work-queue: the warehouse
// worklist (shippable orders + waiting drafts). Read-only, no audit.
func (h *Handler) WorkQueue(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 100
	if raw := q.Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			limit = n
		}
	}
	queue, err := h.svc.WorkQueue(r.Context(), authcontracts.BranchID(r),
		q.Get("search"), q.Get("from"), q.Get("to"), q.Get("age"), limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	createSJ := make([]any, 0, len(queue.CreateSJ))
	for _, o := range queue.CreateSJ {
		lines := make([]any, 0, len(o.Lines))
		for _, l := range o.Lines {
			lines = append(lines, map[string]any{
				"productId": l.ProductID, "variantId": l.VariantID,
				"productCode": l.ProductCode, "productName": l.ProductName,
				"uom": l.UOM, "ordered": l.Ordered, "delivered": l.Delivered,
				"remaining": l.Remaining,
			})
		}
		createSJ = append(createSJ, map[string]any{
			"orderId": o.OrderID, "number": o.Number, "partyName": o.PartyName,
			"orderDate": o.OrderDate, "dueDate": o.DueDate,
			"paymentTerms": o.PaymentTerms, "grandTotal": o.GrandTotal,
			"lines": lines, "totalOrdered": o.TotalOrdered,
			"totalDelivered": o.TotalDelivered, "pendingDrafts": o.PendingDrafts,
			"itemCount": o.ItemCount,
		})
	}
	waiting := make([]any, 0, len(queue.WaitingReturn))
	for _, n := range queue.WaitingReturn {
		waiting = append(waiting, map[string]any{
			"id": n.ID, "number": n.Number, "salesOrderId": n.SalesOrderID, "origin": n.Origin,
			"orderNumber": n.OrderNumber, "partyName": n.PartyName,
			"deliveryDate": n.DeliveryDate, "ageDays": n.AgeDays,
			"driverName": n.DriverName, "vehiclePlate": n.VehiclePlate,
			"hasSignedDocument": n.HasSignedDocument,
		})
	}
	response.OK(w, "Antrian kerja pengiriman", map[string]any{
		"createSj": createSJ, "waitingReturn": waiting, "limit": queue.Limit,
	})
}

// RegisterRoutes mounts delivery endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, media mediacontracts.MediaClient, perm authcontracts.Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/deliveries", perm("delivery.view", authcontracts.Guard(h.List, branchGuard)))
	mux.Handle("GET /api/v1/deliveries/work-queue", perm("delivery.view", authcontracts.Guard(h.WorkQueue, branchGuard)))
	mux.Handle("POST /api/v1/deliveries", perm("delivery.create", authcontracts.Guard(h.Create, branchGuard)))
	mux.Handle("POST /api/v1/deliveries/direct", perm("delivery.create_direct", authcontracts.Guard(h.CreateDirect, branchGuard)))
	mux.Handle("GET /api/v1/deliveries/open-direct", perm("delivery.view", authcontracts.Guard(h.OpenDirect, branchGuard)))
	mux.Handle("GET /api/v1/deliveries/{id}", perm("delivery.view", authcontracts.Guard(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/deliveries/{id}/confirm", perm("delivery.create", authcontracts.Guard(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/deliveries/{id}/cancel", perm("delivery.archive", authcontracts.Guard(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/deliveries/{id}/proof", perm("delivery.create",
		authcontracts.Guard(func(w http.ResponseWriter, r *http.Request) { h.ProofUpload(w, r, media) }, branchGuard)))
	mux.Handle("GET /api/v1/deliveries/{id}/proofs", perm("delivery.view", authcontracts.Guard(h.Proofs, branchGuard)))
}
