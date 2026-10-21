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
	"mini-erp/internal/shared/timeutil"
)

// Handler serves the stock module HTTP surface.
type Handler struct {
	svc *application.Service
}

// NewHandler wires stock endpoints.
func NewHandler(svc *application.Service) *Handler {
	return &Handler{svc: svc}
}

func locationView(l *contracts.Location) map[string]any {
	var parent any
	if l.ParentID != nil {
		parent = *l.ParentID
	}
	return map[string]any{
		"id": l.ID, "branchId": l.BranchID, "code": l.Code, "name": l.Name,
		"parentId": parent, "isSystem": l.IsSystem, "kind": l.Kind, "status": l.Status,
		"isPickingArea": l.IsPickingArea, "isPrimary": l.IsPrimary, "sortOrder": l.SortOrder,
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
	branchID := authcontracts.BranchID(r)
	if raw := r.URL.Query().Get("branchId"); raw != "" {
		target, perr := strconv.ParseInt(raw, 10, 64)
		if perr != nil || target <= 0 {
			response.Fail(w, apperror.Validation("", []apperror.FieldError{{Field: "branchId", Message: "tidak valid"}}))
			return
		}
		// Another branch's locations (transfer destination) need access to it.
		if err := h.svc.AuthorizeBranch(r.Context(), authcontracts.ActorID(r), branchID, target); err != nil {
			response.FailErr(w, err)
			return
		}
		branchID = target
	}
	locations, err := h.svc.ListLocations(r.Context(), branchID, r.URL.Query().Get("status"))
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
		Code          string `json:"code" validate:"required"`
		Name          string `json:"name" validate:"required"`
		ParentID      *int64 `json:"parentId"`
		ForceTransfer bool   `json:"forceTransfer"`
		IsPickingArea *bool  `json:"isPickingArea"`
		IsPrimary     *bool  `json:"isPrimary"`
		SortOrder     *int   `json:"sortOrder"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	l, err := h.svc.CreateLocation(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), application.LocationInput{
		Code: in.Code, Name: in.Name, ParentID: in.ParentID, ForceTransfer: in.ForceTransfer,
		IsPickingArea: in.IsPickingArea, IsPrimary: in.IsPrimary, SortOrder: in.SortOrder,
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
		Code          string `json:"code"`
		Name          string `json:"name" validate:"required"`
		ParentID      *int64 `json:"parentId"`
		ForceTransfer bool   `json:"forceTransfer"`
		IsPickingArea *bool  `json:"isPickingArea"`
		IsPrimary     *bool  `json:"isPrimary"`
		SortOrder     *int   `json:"sortOrder"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		response.Fail(w, apperror.Validation("", fields))
		return
	}
	l, appErr := h.svc.UpdateLocation(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id, application.LocationInput{
		Code: in.Code, Name: in.Name, ParentID: in.ParentID, ForceTransfer: in.ForceTransfer,
		IsPickingArea: in.IsPickingArea, IsPrimary: in.IsPrimary, SortOrder: in.SortOrder,
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
	if appErr := h.svc.ArchiveLocation(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id); appErr != nil {
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
		balances, err := h.svc.BalancesByProduct(r.Context(), authcontracts.BranchID(r), productID)
		if err != nil {
			response.FailErr(w, err)
			return
		}
		out = make([]any, 0, len(balances))
		for _, b := range balances {
			out = append(out, contractBalanceView(b))
		}
	} else {
		page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
		list, total, err := h.svc.ListBalances(r.Context(), authcontracts.BranchID(r), application.BalanceQuery{
			LocationID: queryInt(q.Get("locationId")), Search: q.Get("search"),
			CriticalOnly: q.Get("criticalOnly") == "true", IncludeDamaged: q.Get("includeDamaged") == "true",
			Negative: q.Get("negative") == "true", Page: page, Limit: limit})
		if err != nil {
			response.FailErr(w, err)
			return
		}
		items := make([]any, 0, len(list))
		for _, b := range list {
			items = append(items, b)
		}
		response.Paginated(w, "Saldo stok", items, response.Meta{
			Page: page, Limit: limit, Total: total,
			TotalPages: pagination.TotalPages(total, limit),
		})
		return
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
	balances, appErr := h.svc.BalancesByProduct(r.Context(), authcontracts.BranchID(r), productID)
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
	res, err := h.svc.ListMovements(r.Context(), authcontracts.BranchID(r),
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
			"createdBy": m.CreatedBy, "createdAt": timeutil.FormatWIB(m.CreatedAt),
			"balanceBefore": m.BalanceBefore, "balanceAfter": m.BalanceAfter, "movedAt": timeutil.FormatWIB(m.MovedAt),
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
	res, err := h.svc.Scan(r.Context(), authcontracts.BranchID(r), in.Barcode)
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
	suggestions, shortfall, err := h.svc.Suggest(r.Context(), authcontracts.BranchID(r), in.ProductID, in.VariantID, in.Qty)
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
	id, err := h.svc.Hold(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.ProductID, in.VariantID, in.LocationID, in.Qty, in.Key, in.TTLMinutes)
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
	if err := h.svc.Release(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.Key); err != nil {
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
	if err := h.svc.Adjust(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), application.AdjustInput{
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
		DriverName   string `json:"driverName"`
		VehiclePlate string `json:"vehiclePlate"`
		TransferDate string `json:"transferDate"`
		Items        []struct {
			ProductID  int64   `json:"productId" validate:"required"`
			VariantID  int64   `json:"variantId" validate:"required"`
			Qty        float64 `json:"qty" validate:"required"`
			Notes      string  `json:"notes"`
			ToLocation int64   `json:"toLocationId"`
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
			ToLocationID: it.ToLocation,
		})
	}
	if err := h.svc.AuthorizeBranch(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.ToBranchID); err != nil {
		response.FailErr(w, err)
		return
	}
	t, err := h.svc.CreateTransferWith(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in.ToBranchID,
		in.FromLocation, in.ToLocation, in.Notes, items, application.TransferMeta{
			DriverName: in.DriverName, VehiclePlate: in.VehiclePlate, TransferDate: in.TransferDate, Enforced: true,
		})
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
			"toLocationId": it.ToLocation,
		})
	}
	return map[string]any{
		"id": t.ID, "number": t.Number, "fromBranchId": t.FromBranchID, "toBranchId": t.ToBranchID,
		"fromLocationId": t.FromLocation, "toLocationId": t.ToLocation,
		"status": t.Status, "items": items,
		"notes": t.Notes, "driverName": t.DriverName, "vehiclePlate": t.VehiclePlate, "transferDate": t.TransferDate,
	}
}

// GetTransfer handles GET /api/v1/stock/transfers/{id}.
func (h *Handler) GetTransfer(w http.ResponseWriter, r *http.Request) {
	id, err := httpx.PathInt(r, "id")
	if err != nil {
		response.Fail(w, apperror.NotFound("Transfer"))
		return
	}
	t, appErr := h.svc.GetTransfer(r.Context(), authcontracts.BranchID(r), id)
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
			ProductID  int64   `json:"productId" validate:"required"`
			VariantID  int64   `json:"variantId" validate:"required"`
			Qty        float64 `json:"qty" validate:"required"`
			Notes      string  `json:"notes"`
			ToLocation int64   `json:"toLocationId"`
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
			ToLocationID: it.ToLocation,
		})
	}
	t, err := h.svc.MoveLocation(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r),
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
	res, err := h.svc.ListTransfersDir(r.Context(), authcontracts.BranchID(r), q.Get("status"), q.Get("direction"), page, limit)
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
		t, appErr = h.svc.DispatchTransfer(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
	case "receive":
		t, appErr = h.svc.ReceiveTransfer(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
	default:
		t, appErr = h.svc.CancelTransfer(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), id)
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
	q := r.URL.Query()
	page, limit := pagination.Parse(q.Get("page"), q.Get("limit"))
	res, err := h.svc.DamagedListPage(r.Context(), authcontracts.BranchID(r), page, limit, q.Get("empty") == "true")
	if err != nil {
		response.FailErr(w, err)
		return
	}
	out := make([]any, 0, len(res.Items))
	for _, b := range res.Items {
		v := balanceView(b.BalanceView)
		v["estimatedUnitCost"], v["estimatedLoss"] = b.EstimatedUnitCost, b.EstimatedLoss
		out = append(out, v)
	}
	// data carries the whole-set figures next to the page (header total + count).
	totalPages := pagination.TotalPages(res.Total, limit)
	response.Paginated(w, "Stok rusak", map[string]any{
		"items": out, "total": res.Total, "withStock": res.WithStock, "empty": res.Empty,
		"totalEstimatedLoss": res.TotalEstimatedLoss,
		"page":               page, "limit": limit, "totalPages": totalPages,
	}, response.Meta{Page: page, Limit: limit, Total: res.Total, TotalPages: totalPages})
}

func damagedInput(r *http.Request, locationOptional bool) (application.DamagedInput, *apperror.AppError) {
	var in struct {
		ProductID  int64   `json:"productId" validate:"required"`
		VariantID  int64   `json:"variantId" validate:"required"`
		LocationID int64   `json:"locationId"`
		Qty        float64 `json:"qty" validate:"required"`
		Reason     string  `json:"reason"`
	}
	if fields := httpx.DecodeJSON(r, &in); fields != nil {
		return application.DamagedInput{}, apperror.Validation("", fields)
	}
	if in.LocationID <= 0 && !locationOptional {
		return application.DamagedInput{}, apperror.Validation("", []apperror.FieldError{{Field: "locationId", Message: "wajib diisi"}})
	}
	return application.DamagedInput{
		ProductID: in.ProductID, VariantID: in.VariantID,
		LocationID: in.LocationID, Qty: in.Qty, Reason: in.Reason,
	}, nil
}

// DamagedMoveIn handles POST /api/v1/stock/damaged/move-in.
func (h *Handler) DamagedMoveIn(w http.ResponseWriter, r *http.Request) {
	in, appErr := damagedInput(r, true)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if err := h.svc.MoveToDamaged(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in); err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Stok dicatat rusak", nil)
}

// DamagedRestore handles POST /api/v1/stock/damaged/restore.
func (h *Handler) DamagedRestore(w http.ResponseWriter, r *http.Request) {
	in, appErr := damagedInput(r, false)
	if appErr != nil {
		response.Fail(w, appErr)
		return
	}
	if err := h.svc.RestoreDamaged(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), in); err != nil {
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
	if err := h.svc.WriteOff(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), application.DamagedInput{
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
	if err := h.svc.OpeningTemplate(r.Context(), authcontracts.BranchID(r), w); err != nil {
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
	prev, err := h.svc.PreviewOpening(r.Context(), authcontracts.BranchID(r), fh)
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
	res, err := h.svc.CommitOpening(r.Context(), authcontracts.ActorID(r), authcontracts.BranchID(r), fh)
	if err != nil {
		response.FailErr(w, err)
		return
	}
	response.OK(w, "Saldo awal dibukukan", res)
}
