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
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the delivery module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires delivery endpoints.
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
		"dispatchedAt":                    n.DispatchedAt, "dispatchedBy": n.DispatchedBy,
		"confirmedAt": n.ConfirmedAt, "confirmedBy": n.ConfirmedBy,
		"documentKind": n.DocumentKind, "salesReturnId": n.SalesReturnID,
		"items": items,
	}
}

// List handles GET /api/v1/deliveries.
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
	if n == nil || n.BranchID != branchID(r) {
		response.Fail(w, apperror.NotFound("Surat jalan"))
		return
	}
	response.OK(w, "Data surat jalan", noteView(n))
}

// Create handles POST /api/v1/deliveries.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		SalesOrderID       int64  `json:"salesOrderId" validate:"required"`
		DeliveryDate       string `json:"deliveryDate"`
		DriverName         string `json:"driverName"`
		VehiclePlate       string `json:"vehiclePlate"`
		WarehouseStaffName string `json:"warehouseStaffName"`
		DropLocationNote   string `json:"dropLocationNote"`
		Notes              string `json:"notes"`
		Items              []struct {
			ProductID  int64   `json:"productId" validate:"required"`
			VariantID  int64   `json:"variantId" validate:"required"`
			LocationID int64   `json:"locationId" validate:"required"`
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
	n, err := h.svc.Create(r.Context(), actorID(r), branchID(r), in.SalesOrderID, in.DeliveryDate, in.Notes, application.DocInput{
		DriverName: in.DriverName, VehiclePlate: in.VehiclePlate,
		WarehouseStaffName: in.WarehouseStaffName, DropLocationNote: in.DropLocationNote,
	}, lines)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Surat jalan dibuat", noteView(n))
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
	}
	// Receipt evidence is optional: bare confirms (POS flow, existing
	// callers) post an empty body.
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if fields := httpx.DecodeJSON(r, &in); fields != nil {
			response.Fail(w, apperror.Validation("", fields))
			return
		}
	}
	n, appErr := h.svc.Confirm(r.Context(), actorID(r), branchID(r), id, application.ConfirmInput{
		RecipientName: in.RecipientName, RecipientSignatureStatus: in.RecipientSignatureStatus,
		RecipientSignatureMissingReason: in.RecipientSignatureMissingReason,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Surat jalan dikonfirmasi", noteView(n))
}

// Cancel handles POST /api/v1/deliveries/{id}/cancel.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Surat jalan"))
		return
	}
	n, appErr := h.svc.Cancel(r.Context(), actorID(r), branchID(r), id)
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
	url, svcErr := h.svc.Proof(r.Context(), actorID(r), branchID(r), id, mediacontracts.Upload{
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
	proofs, appErr := h.svc.Proofs(r.Context(), branchID(r), id)
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
	queue, err := h.svc.WorkQueue(r.Context(), branchID(r),
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
		})
	}
	waiting := make([]any, 0, len(queue.WaitingReturn))
	for _, n := range queue.WaitingReturn {
		waiting = append(waiting, map[string]any{
			"id": n.ID, "number": n.Number, "salesOrderId": n.SalesOrderID,
			"orderNumber": n.OrderNumber, "partyName": n.PartyName,
			"deliveryDate": n.DeliveryDate, "ageDays": n.AgeDays,
			"driverName": n.DriverName, "vehiclePlate": n.VehiclePlate,
		})
	}
	response.OK(w, "Antrian kerja pengiriman", map[string]any{
		"createSj": createSJ, "waitingReturn": waiting, "limit": queue.Limit,
	})
}

// Permission is injected by the composition root (built from the auth module).
type Permission func(perm string, next func(http.ResponseWriter, *http.Request)) http.Handler

// guarded adapts the branch guard into func-style middleware.
func guarded(next func(http.ResponseWriter, *http.Request), branchGuard func(http.Handler) http.Handler) func(http.ResponseWriter, *http.Request) {
	return branchGuard(http.HandlerFunc(next)).ServeHTTP
}

// RegisterRoutes mounts delivery endpoints with explicit permissions.
func RegisterRoutes(mux *http.ServeMux, h *Handler, media mediacontracts.MediaClient, perm Permission, branchGuard func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/deliveries", perm("delivery.view", guarded(h.List, branchGuard)))
	mux.Handle("GET /api/v1/deliveries/work-queue", perm("delivery.view", guarded(h.WorkQueue, branchGuard)))
	mux.Handle("POST /api/v1/deliveries", perm("delivery.create", guarded(h.Create, branchGuard)))
	mux.Handle("GET /api/v1/deliveries/{id}", perm("delivery.view", guarded(h.Get, branchGuard)))
	mux.Handle("POST /api/v1/deliveries/{id}/confirm", perm("delivery.create", guarded(h.Confirm, branchGuard)))
	mux.Handle("POST /api/v1/deliveries/{id}/cancel", perm("delivery.archive", guarded(h.Cancel, branchGuard)))
	mux.Handle("POST /api/v1/deliveries/{id}/proof", perm("delivery.create",
		func(w http.ResponseWriter, r *http.Request) { h.ProofUpload(w, r, media) }))
	mux.Handle("GET /api/v1/deliveries/{id}/proofs", perm("delivery.view", guarded(h.Proofs, branchGuard)))
}
