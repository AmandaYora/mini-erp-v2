package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// Repository owns the delivery module tables. Data access goes through the
// sqlc-generated Queries (ADR-0012); only the dynamic list/waiting filters in
// listfilter.go are hand-written (raw-sql allow-list).
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// GetNote returns nil, nil when missing.
func (r *Repository) GetNote(ctx context.Context, id int64) (*contracts.DeliveryNote, error) {
	row, err := r.q.GetNote(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toNote(row), nil
}

// ItemsByDelivery returns the note lines in row order.
func (r *Repository) ItemsByDelivery(ctx context.Context, deliveryID int64) ([]*contracts.Item, error) {
	rows, err := r.q.ListItemsByDelivery(ctx, deliveryID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Item
	for _, row := range rows {
		out = append(out, toItem(row))
	}
	return out, nil
}

// DeliveredQty sums confirmed-delivered base qty for one SO line position.
// Replacement shipments (document_kind <> 'order') are excluded: they ship
// new goods, they don't fulfill the order.
func (r *Repository) DeliveredQty(ctx context.Context, soID, productID, variantID int64) (float64, error) {
	return r.q.DeliveredQty(ctx, sqlc.DeliveredQtyParams{
		SalesOrderID: soID, ProductID: productID, VariantID: variantID,
	})
}

// CommittedQtyBySO sums base qty per "product/variant" over the order's
// draft AND confirmed order-kind notes (P6): a draft holds its share of the
// order's remaining quantity so two drafts can never both claim it.
func (r *Repository) CommittedQtyBySO(ctx context.Context, soID int64) (map[string]float64, error) {
	rows, err := r.q.CommittedQtyBySO(ctx, soID)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, row := range rows {
		out[strconv.FormatInt(row.ProductID, 10)+"/"+strconv.FormatInt(row.VariantID, 10)] = row.Total
	}
	return out, nil
}

// ConfirmedQtyBefore sums line-UOM qty per "product/variant/uom" over the
// order's confirmed order-kind notes with an id below noteID. Rows that
// differ only by uom padding share one trimmed key, so their quantities
// accumulate (the GROUP BY order is unspecified and trimming must not drop
// units).
func (r *Repository) ConfirmedQtyBefore(ctx context.Context, soID, noteID int64) (map[string]float64, error) {
	rows, err := r.q.ConfirmedQtyBefore(ctx, sqlc.ConfirmedQtyBeforeParams{
		SalesOrderID: soID, ID: noteID,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, row := range rows {
		key := strconv.FormatInt(row.ProductID, 10) + "/" + strconv.FormatInt(row.VariantID, 10) + "/" + strings.TrimSpace(row.Uom)
		out[key] += row.Total
	}
	return out, nil
}

// ActiveNoteCount counts the order's live (draft + confirmed) order-kind
// shipments — the guard that stops an order from being cancelled under them.
func (r *Repository) ActiveNoteCount(ctx context.Context, soID int64) (int64, error) {
	return r.q.ActiveNoteCount(ctx, soID)
}

// ConfirmedAfter pages confirmed notes by ascending id after a cursor.
func (r *Repository) ConfirmedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.DeliveryNote, error) {
	rows, err := r.q.ListConfirmedAfter(ctx, sqlc.ListConfirmedAfterParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var out []*contracts.DeliveryNote
	for _, row := range rows {
		out = append(out, toNote(sqlc.GetNoteRow(row)))
	}
	return out, nil
}

// BulkKey keys delivered sums per (order, product, variant). Exported for
// the application layer's queue math so the format lives in exactly one
// place within the module.
func BulkKey(soID, productID, variantID int64) string {
	return strconv.FormatInt(soID, 10) + "/" +
		strconv.FormatInt(productID, 10) + "/" + strconv.FormatInt(variantID, 10)
}

// DeliveredQtyBulk sums confirmed order-kind deliveries per (order, product,
// variant) — one GROUP BY for the whole work queue, never one query per
// line (P3). Replacement shipments are excluded like DeliveredQty.
func (r *Repository) DeliveredQtyBulk(ctx context.Context, soIDs []int64) (map[string]float64, error) {
	out := map[string]float64{}
	if len(soIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.DeliveredQtyBulk(ctx, soIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[BulkKey(row.SalesOrderID, row.ProductID, row.VariantID)] = row.Total
	}
	return out, nil
}

// DraftCounts counts draft order-kind notes per order — one GROUP BY for the
// whole work queue. A waiting draft is why an order can show remaining qty
// and still need no new SJ.
func (r *Repository) DraftCounts(ctx context.Context, soIDs []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(soIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.DraftCounts(ctx, soIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.SalesOrderID] = row.Total
	}
	return out, nil
}

// NoteIDByClientRef finds a branch's note by its client reference, or 0.
func (r *Repository) NoteIDByClientRef(ctx context.Context, branchID int64, ref string) (int64, error) {
	id, err := r.q.NoteIDByClientRef(ctx, sqlc.NoteIDByClientRefParams{
		BranchID: branchID, ClientRef: dbutil.NullString(ref),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// CreateNote inserts a draft note with lines, returning the id.
func (r *Repository) CreateNote(ctx context.Context, n *contracts.DeliveryNote, actorID int64) (int64, error) {
	deliverAt, err := parseDeliveryDate(n.DeliveryDate)
	if err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	kind := n.DocumentKind
	if kind == "" {
		kind = contracts.DocumentKindOrder
	}
	q := r.q.WithTx(tx)
	res, err := q.InsertNote(ctx, sqlc.InsertNoteParams{
		Number:             n.Number,
		BranchID:           n.BranchID,
		SalesOrderID:       n.SalesOrderID,
		DeliveryDate:       deliverAt,
		DriverName:         dbutil.NullString(n.DriverName),
		VehiclePlate:       dbutil.NullString(n.VehiclePlate),
		WarehouseStaffName: dbutil.NullString(n.WarehouseStaffName),
		DropLocationNote:   dbutil.NullString(n.DropLocationNote),
		DocumentKind:       kind,
		SalesReturnID:      dbutil.NullInt64(n.SalesReturnID),
		Notes:              dbutil.NullString(n.Notes),
		ClientRef:          dbutil.NullString(n.ClientRef),
		CreatedBy:          actor(actorID),
		UpdatedBy:          actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range n.Items {
		if err := q.InsertNoteItem(ctx, sqlc.InsertNoteItemParams{
			DeliveryID: id, ProductID: it.ProductID, VariantID: it.VariantID,
			LocationID: it.LocationID, Uom: it.UOM,
			UomFactor: it.UOMFactor, Qty: it.Qty, QtyBase: it.QtyBase,
			ReservationKey: dbutil.NullString(it.ReservationKey),
		}); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// SetStatus moves the note lifecycle.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	return r.q.SetNoteStatus(ctx, sqlc.SetNoteStatusParams{
		Status: status, Now: now(), UpdatedBy: actor(actorID), ID: id,
	})
}

// Confirm flips draft -> confirmed and records dispatch + receipt evidence in
// ONE statement guarded by the status (CAS). Returns false when the note was
// no longer a draft (a concurrent confirm or cancel won).
// The optional hook runs inside the CAS transaction only when this call won
// the flip, so the audit row co-commits with the status change (ADR-0011) and
// a replay that lost the CAS writes none (ADR-0016).
func (r *Repository) Confirm(ctx context.Context, id int64, n *contracts.DeliveryNote, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	t := now()
	res, err := r.q.WithTx(tx).ConfirmNote(ctx, sqlc.ConfirmNoteParams{
		RecipientName: dbutil.NullString(n.RecipientName),
		SigStatus:     dbutil.NullString(n.RecipientSignatureStatus),
		SigReason:     dbutil.NullString(n.RecipientSignatureMissingReason),
		Stamp:         sql.NullTime{Time: t, Valid: true},
		ActorID:       actor(actorID),
		Now:           t,
		ID:            id,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected != 1 {
		return false, err
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// Cancel flips draft -> cancelled and records the reason, time and actor in
// ONE statement guarded by the status (CAS). Returns false when the note was
// no longer a draft. The optional hook runs inside the CAS transaction only
// when this call won, so the audit row co-commits with the flip (ADR-0011)
// and a replay that lost the CAS writes none (ADR-0016).
func (r *Repository) Cancel(ctx context.Context, id int64, reason string, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	t := now()
	res, err := r.q.WithTx(tx).CancelNote(ctx, sqlc.CancelNoteParams{
		Reason:  dbutil.NullString(reason),
		Stamp:   sql.NullTime{Time: t, Valid: true},
		ActorID: actor(actorID),
		Now:     t,
		ID:      id,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected != 1 {
		return false, err
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// Transition moves a note between statuses only from an allowed origin (CAS).
func (r *Repository) Transition(ctx context.Context, id int64, from []string, to string, actorID int64) (bool, error) {
	if len(from) == 0 {
		return false, nil
	}
	res, err := r.q.TransitionNote(ctx, sqlc.TransitionNoteParams{
		ToStatus: to, Now: now(), ActorID: actor(actorID), ID: id, FromStatuses: from,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	return affected == 1, err
}

// SetDispatch stamps the dispatch + confirm event. Our draft→confirmed flow
// merges dispatch and receipt: both stamp the confirmer and time, while the
// recipient columns carry the receipt evidence.
func (r *Repository) SetDispatch(ctx context.Context, id int64, n *contracts.DeliveryNote, actorID int64) error {
	t := now()
	return r.q.SetDispatch(ctx, sqlc.SetDispatchParams{
		RecipientName: dbutil.NullString(n.RecipientName),
		SigStatus:     dbutil.NullString(n.RecipientSignatureStatus),
		SigReason:     dbutil.NullString(n.RecipientSignatureMissingReason),
		Stamp:         sql.NullTime{Time: t, Valid: true},
		ActorID:       actor(actorID),
		Now:           t,
		ID:            id,
	})
}

// OrderIDsByNotes maps note id -> sales order id in one grouped read.
// Unknown ids are simply absent from the result.
func (r *Repository) OrderIDsByNotes(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	seen := make(map[int64]bool, len(ids))
	args := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		args = append(args, id)
	}
	if len(args) == 0 {
		return out, nil
	}
	rows, err := r.q.OrderIDsByNotes(ctx, args)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.SalesOrderID
	}
	return out, nil
}
