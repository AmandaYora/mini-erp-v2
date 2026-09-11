package presentation

import (
	"net/http"
	"strconv"

	authcontracts "mini-erp/internal/modules/auth/contracts"
	"mini-erp/internal/modules/stock/application"
	"mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/modules/stock/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/httpx"
	"mini-erp/internal/shared/pagination"
	"mini-erp/internal/shared/response"
)

// Handler serves the stock module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires stock endpoints.
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

func locationView(l *contracts.Location) map[string]any {
	var parent any
	if l.ParentID != nil {
		parent = *l.ParentID
	}
	return map[string]any{
		"id": l.ID, "branchId": l.BranchID, "code": l.Code, "name": l.Name,
		"parentId": parent, "isSystem": l.IsSystem, "status": l.Status,
	}
}

func balanceView(b application.BalanceView) map[string]any {
	return map[string]any{
		"branchId": b.BranchID, "productId": b.ProductID, "variantId": b.VariantID,
		"locationId": b.LocationID, "onHand": b.OnHand, "reserved": b.Reserved,
		"available": b.Available,
	}
}

func contractBalanceView(b *contracts.Balance) map[string]any {
	return map[string]any{
		"branchId": b.BranchID, "productId": b.ProductID, "variantId": b.VariantID,
		"locationId": b.LocationID, "onHand": b.OnHand, "reserved": b.Reserved,
		"available": b.Available(),
	}
}

// ListLocations handles GET /api/v1/stock/locations.
func (h *Handler) ListLocations(w http.ResponseWriter, r *http.Request) {
	locations, err := h.svc.ListLocations(r.Context(), branchID(r), r.URL.Query().Get("status"))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(locations))
	for _, l := range locations {
		items = append(items, locationView(l))
	}
	response.OK(w, "Data lokasi", items)
}

// CreateLocation handles POST /api/v1/stock/locations.
func (h *Handler) CreateLocation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Code     string `json:"code" validate:"required"`
		Name     string `json:"name" validate:"required"`
		ParentID *int64 `json:"parentId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	l, err := h.svc.CreateLocation(r.Context(), actorID(r), branchID(r), application.LocationInput{
		Code: in.Code, Name: in.Name, ParentID: in.ParentID,
	})
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Lokasi dibuat", locationView(l))
}

// UpdateLocation handles PUT /api/v1/stock/locations/{id}.
func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Lokasi"))
		return
	}
	var in struct {
		Name     string `json:"name" validate:"required"`
		ParentID *int64 `json:"parentId"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	l, appErr := h.svc.UpdateLocation(r.Context(), actorID(r), branchID(r), id, application.LocationInput{
		Name: in.Name, ParentID: in.ParentID,
	})
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Lokasi disimpan", locationView(l))
}

// ArchiveLocation handles POST /api/v1/stock/locations/{id}/archive.
func (h *Handler) ArchiveLocation(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Lokasi"))
		return
	}
	if appErr := h.svc.ArchiveLocation(r.Context(), actorID(r), branchID(r), id); appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Lokasi diarsipkan", nil)
}

// Balances handles GET /api/v1/stock/balances.
func (h *Handler) Balances(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	productID := queryInt(q.Get("productId"))
	var out []any
	if productID != 0 {
		balances, err := h.svc.BalancesByProduct(r.Context(), branchID(r), productID)
		if err != nil {
			response.FailErr(w, err)
			return
		}
		out = make([]any, 0, len(balances))
		for _, b := range balances {
			out = append(out, contractBalanceView(b))
		}
	} else {
		out = []any{}
	}
	response.OK(w, "Saldo stok", out)
}

// BalanceDetail handles GET /api/v1/stock/balances/{productId} (J5): posisi
// satu item per lokasi di cabang aktif. Alias path-style di atas query yang
// sama — satu service call, tanpa logika ganda.
func (h *Handler) BalanceDetail(w http.ResponseWriter, r *http.Request) {
	productID, err := httpx.PathInt(r, "productId")
	if err != nil || productID <= 0 {
		response.Fail(w, apperror.NotFound("Produk"))
		return
	}
	balances, appErr := h.svc.BalancesByProduct(r.Context(), branchID(r), productID)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	out := make([]any, 0, len(balances))
	for _, b := range balances {
		out = append(out, contractBalanceView(b))
	}
	response.OK(w, "Saldo item stok", out)
}

// Movements handles GET /api/v1/stock/movements.
func (h *Handler) Movements(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.ListMovements(r.Context(), branchID(r),
		queryInt(q.Get("productId")), queryInt(q.Get("variantId")), queryInt(q.Get("locationId")),
		q.Get("type"), q.Get("dateFrom"), q.Get("dateTo"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Movements))
	for _, m := range res.Movements {
		items = append(items, map[string]any{
			"id": m.ID, "productId": m.ProductID, "variantId": m.VariantID,
			"locationId": m.LocationID, "direction": m.Direction, "type": m.Type,
			"qty": m.Qty, "refType": m.RefType, "refId": m.RefID, "notes": m.Notes,
			"createdBy": m.CreatedBy, "createdAt": m.CreatedAt,
		})
	}
	response.Paginated(w, "Mutasi stok", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

func queryInt(raw string) int64 {
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// Scan handles POST /api/v1/stock/scan-product.
func (h *Handler) Scan(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Barcode string `json:"barcode" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	res, err := h.svc.Scan(r.Context(), branchID(r), in.Barcode)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Hasil pindai", res)
}

// Suggest handles POST /api/v1/stock/allocations/suggest.
func (h *Handler) Suggest(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID int64   `json:"productId" validate:"required"`
		VariantID int64   `json:"variantId"`
		Qty       float64 `json:"qty" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	suggestions, shortfall, err := h.svc.Suggest(r.Context(), branchID(r), in.ProductID, in.VariantID, in.Qty)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Saran alokasi", map[string]any{"lines": suggestions, "shortfall": shortfall})
}

// Hold handles POST /api/v1/stock/reservations/hold.
func (h *Handler) Hold(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID  int64   `json:"productId" validate:"required"`
		VariantID  int64   `json:"variantId" validate:"required"`
		LocationID int64   `json:"locationId" validate:"required"`
		Qty        float64 `json:"qty" validate:"required"`
		Key        string  `json:"key" validate:"required"`
		TTLMinutes int     `json:"ttlMinutes"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	id, err := h.svc.Hold(r.Context(), actorID(r), branchID(r), in.ProductID, in.VariantID, in.LocationID, in.Qty, in.Key, in.TTLMinutes)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Stok ditahan", map[string]any{"id": id, "key": in.Key})
}

// Release handles POST /api/v1/stock/reservations/release.
func (h *Handler) Release(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Key string `json:"key" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if err := h.svc.Release(r.Context(), actorID(r), in.Key); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Tahanan dilepas", nil)
}

// Adjust handles POST /api/v1/stock/adjustments.
func (h *Handler) Adjust(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID int64   `json:"productId" validate:"required"`
		VariantID int64   `json:"variantId" validate:"required"`
		Location  int64   `json:"locationId" validate:"required"`
		Mode      string  `json:"mode" validate:"required"`
		QtyAfter  float64 `json:"qtyAfter"`
		QtyDelta  float64 `json:"qtyDelta"`
		Reason    string  `json:"reason" validate:"required"`
		Approver  string  `json:"approver"`
		ApproverP string  `json:"approverPassword"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if err := h.svc.Adjust(r.Context(), actorID(r), branchID(r), application.AdjustInput{
		ProductID: in.ProductID, VariantID: in.VariantID, Location: in.Location,
		Mode: in.Mode, QtyAfter: in.QtyAfter, QtyDelta: in.QtyDelta,
		Reason: in.Reason, Approver: in.Approver, ApproverP: in.ApproverP,
	}); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Koreksi tersimpan", nil)
}

// CreateTransfer handles POST /api/v1/stock/transfers.
func (h *Handler) CreateTransfer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ToBranchID   int64  `json:"toBranchId" validate:"required"`
		FromLocation int64  `json:"fromLocationId" validate:"required"`
		ToLocation   int64  `json:"toLocationId" validate:"required"`
		Notes        string `json:"notes"`
		Items        []struct {
			ProductID int64   `json:"productId" validate:"required"`
			VariantID int64   `json:"variantId" validate:"required"`
			Qty       float64 `json:"qty" validate:"required"`
			Notes     string  `json:"notes"`
		} `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	items := make([]application.TransferItemInput, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, application.TransferItemInput{
			ProductID: it.ProductID, VariantID: it.VariantID, Qty: it.Qty, Notes: it.Notes,
		})
	}
	t, err := h.svc.CreateTransfer(r.Context(), actorID(r), branchID(r), in.ToBranchID,
		in.FromLocation, in.ToLocation, in.Notes, items)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Transfer dibuat", transferView(t))
}

func transferView(t *infrastructure.Transfer) map[string]any {
	items := make([]any, 0, len(t.Items))
	for _, it := range t.Items {
		items = append(items, map[string]any{
			"productId": it.ProductID, "variantId": it.VariantID, "qty": it.Qty, "notes": it.Notes,
		})
	}
	return map[string]any{
		"id": t.ID, "number": t.Number, "fromBranchId": t.FromBranchID, "toBranchId": t.ToBranchID,
		"fromLocationId": t.FromLocation, "toLocationId": t.ToLocation,
		"status": t.Status, "items": items,
	}
}

// GetTransfer handles GET /api/v1/stock/transfers/{id}.
func (h *Handler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Transfer"))
		return
	}
	t, appErr := h.svc.GetTransfer(r.Context(), branchID(r), id)
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	response.OK(w, "Data transfer", transferView(t))
}

// ListTransfers handles GET /api/v1/stock/transfers.
// MoveLocation handles POST /api/v1/stock/transfers/move-location: same-branch
// relocation in one call (J4). The returned transfer is already received.
func (h *Handler) MoveLocation(w http.ResponseWriter, r *http.Request) {
	var in struct {
		FromLocation int64  `json:"fromLocationId" validate:"required"`
		ToLocation   int64  `json:"toLocationId" validate:"required"`
		Notes        string `json:"notes"`
		Items        []struct {
			ProductID int64   `json:"productId" validate:"required"`
			VariantID int64   `json:"variantId" validate:"required"`
			Qty       float64 `json:"qty" validate:"required"`
			Notes     string  `json:"notes"`
		} `json:"items" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	items := make([]application.TransferItemInput, 0, len(in.Items))
	for _, it := range in.Items {
		items = append(items, application.TransferItemInput{
			ProductID: it.ProductID, VariantID: it.VariantID, Qty: it.Qty, Notes: it.Notes,
		})
	}
	t, err := h.svc.MoveLocation(r.Context(), actorID(r), branchID(r),
		in.FromLocation, in.ToLocation, in.Notes, items)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.Created(w, "Stok dipindahkan", transferView(t))
}

func (h *Handler) ListTransfers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.ListTransfers(r.Context(), branchID(r), q.Get("status"), page, limit)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	items := make([]any, 0, len(res.Transfers))
	for _, t := range res.Transfers {
		items = append(items, transferView(t))
	}
	response.Paginated(w, "Data transfer", items, response.Meta{
		Page: page, Limit: limit, Total: res.Total,
		TotalPages: pagination.TotalPages(res.Total, limit),
	})
}

// DispatchTransfer handles POST /api/v1/stock/transfers/{id}/dispatch.
func (h *Handler) DispatchTransfer(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "dispatch")
}

// ReceiveTransfer handles POST /api/v1/stock/transfers/{id}/receive.
func (h *Handler) ReceiveTransfer(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "receive")
}

// CancelTransfer handles POST /api/v1/stock/transfers/{id}/cancel.
func (h *Handler) CancelTransfer(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "cancel")
}

func (h *Handler) transition(w http.ResponseWriter, r *http.Request, action string) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Transfer"))
		return
	}
	var t *infrastructure.Transfer
	var appErr error
	switch action {
	case "dispatch":
		t, appErr = h.svc.DispatchTransfer(r.Context(), actorID(r), branchID(r), id)
	case "receive":
		t, appErr = h.svc.ReceiveTransfer(r.Context(), actorID(r), branchID(r), id)
	default:
		t, appErr = h.svc.CancelTransfer(r.Context(), actorID(r), branchID(r), id)
	}
	if appErr != nil {
		response.FailErr(w, appErr)
		return
	}
	msgs := map[string]string{
		"dispatch": "Transfer dikirim", "receive": "Transfer diterima", "cancel": "Transfer dibatalkan",
	}
	response.OK(w, msgs[action], transferView(t))
}

// DamagedList handles GET /api/v1/stock/damaged.
func (h *Handler) DamagedList(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.DamagedList(r.Context(), branchID(r))
	if err != nil {
		response.FailErr(w, err)
		return
	}
	out := make([]any, 0, len(items))
	for _, b := range items {
		out = append(out, balanceView(b))
	}
	response.OK(w, "Stok rusak", out)
}

func damagedInput(r *http.Request) (application.DamagedInput, *apperror.AppError) {
	var in struct {
		ProductID  int64   `json:"productId" validate:"required"`
		VariantID  int64   `json:"variantId" validate:"required"`
		LocationID int64   `json:"locationId" validate:"required"`
		Qty        float64 `json:"qty" validate:"required"`
		Reason     string  `json:"reason"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		return application.DamagedInput{}, apperror.Validation("", fields)
	}
	return application.DamagedInput{
		ProductID: in.ProductID, VariantID: in.VariantID,
		LocationID: in.LocationID, Qty: in.Qty, Reason: in.Reason,
	}, nil
}

// DamagedMoveIn handles POST /api/v1/stock/damaged/move-in.
func (h *Handler) DamagedMoveIn(w http.ResponseWriter, r *http.Request) {
	in, appErr := damagedInput(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if err := h.svc.MoveToDamaged(r.Context(), actorID(r), branchID(r), in); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Stok dicatat rusak", nil)
}

// DamagedRestore handles POST /api/v1/stock/damaged/restore.
func (h *Handler) DamagedRestore(w http.ResponseWriter, r *http.Request) {
	in, appErr := damagedInput(r)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if err := h.svc.RestoreDamaged(r.Context(), actorID(r), branchID(r), in); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Stok dipulihkan", nil)
}

// DamagedWriteOff handles POST /api/v1/stock/damaged/write-off.
// No location: write-off always consumes the damaged location itself.
func (h *Handler) DamagedWriteOff(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ProductID int64   `json:"productId" validate:"required"`
		VariantID int64   `json:"variantId" validate:"required"`
		Qty       float64 `json:"qty" validate:"required"`
		Reason    string  `json:"reason" validate:"required"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	if err := h.svc.WriteOff(r.Context(), actorID(r), branchID(r), application.DamagedInput{
		ProductID: in.ProductID, VariantID: in.VariantID, Qty: in.Qty, Reason: in.Reason,
	}); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Stok dihapusbukukan", nil)
}

// OpeningTemplate handles GET /api/v1/stock/opening/template.
func (h *Handler) OpeningTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="template-saldo-awal.xlsx"`)
	if err := h.svc.OpeningTemplate(w); err != nil {
		return
	}
}

// OpeningPreview handles POST /api/v1/stock/opening/preview.
func (h *Handler) OpeningPreview(w http.ResponseWriter, r *http.Request) {
	fh, _, appErr := httpx.MultipartFile(r, 10<<20)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	defer func() { _ = fh.Close() }()
	prev, err := h.svc.PreviewOpening(r.Context(), branchID(r), fh)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Pratinjau saldo awal", prev)
}

// OpeningCommit handles POST /api/v1/stock/opening/commit.
func (h *Handler) OpeningCommit(w http.ResponseWriter, r *http.Request) {
	fh, _, appErr := httpx.MultipartFile(r, 10<<20)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	defer func() { _ = fh.Close() }()
	res, err := h.svc.CommitOpening(r.Context(), actorID(r), branchID(r), fh)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Saldo awal dibukukan", res)
}
