package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"mini-erp/internal/modules/delivery/contracts"
)

// Repository owns the delivery module tables.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func toFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

const noteColumns = `id, number, branch_id, sales_order_id, delivery_date,
	driver_name, vehicle_plate, warehouse_staff_name, recipient_name,
	recipient_signature_status, recipient_signature_missing_reason,
	drop_location_note, dispatched_at, dispatched_by, confirmed_at,
	confirmed_by, document_kind, sales_return_id, status, notes`

func scanRow(row interface {
	Scan(dest ...any) error
}, n *contracts.DeliveryNote) error {
	var delivered, dispatched, confirmed sql.NullTime
	var driver, plate, staff, recipient, sigStatus, sigReason, drop, notes sql.NullString
	var dispatchedBy, confirmedBy, returnID sql.NullInt64
	var kind sql.NullString
	err := row.Scan(&n.ID, &n.Number, &n.BranchID, &n.SalesOrderID,
		&delivered, &driver, &plate, &staff, &recipient, &sigStatus, &sigReason,
		&drop, &dispatched, &dispatchedBy, &confirmed, &confirmedBy,
		&kind, &returnID, &n.Status, &notes)
	if err != nil {
		return err
	}
	if delivered.Valid {
		n.DeliveryDate = delivered.Time.Format("2006-01-02 15:04:05")
	}
	n.DriverName, n.VehiclePlate, n.WarehouseStaffName = driver.String, plate.String, staff.String
	n.RecipientName, n.RecipientSignatureStatus = recipient.String, sigStatus.String
	n.RecipientSignatureMissingReason, n.DropLocationNote = sigReason.String, drop.String
	if dispatched.Valid {
		n.DispatchedAt = dispatched.Time.Format("2006-01-02 15:04:05")
	}
	n.DispatchedBy = dispatchedBy.Int64
	if confirmed.Valid {
		n.ConfirmedAt = confirmed.Time.Format("2006-01-02 15:04:05")
	}
	n.ConfirmedBy = confirmedBy.Int64
	n.DocumentKind = kind.String
	if n.DocumentKind == "" {
		n.DocumentKind = contracts.DocumentKindOrder
	}
	n.SalesReturnID = returnID.Int64
	n.Notes = notes.String
	return nil
}

// GetNote returns nil, nil when missing.
func (r *Repository) GetNote(ctx context.Context, id int64) (*contracts.DeliveryNote, error) {
	n := &contracts.DeliveryNote{}
	err := scanRow(r.db.QueryRowContext(ctx,
		"SELECT "+noteColumns+" FROM delivery_notes WHERE id = ?", id), n)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return n, err
}

// ItemsByDelivery returns the note lines in row order.
func (r *Repository) ItemsByDelivery(ctx context.Context, deliveryID int64) ([]*contracts.Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base
		 FROM delivery_note_items WHERE delivery_id = ? ORDER BY id ASC`, deliveryID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Item
	for rows.Next() {
		it := &contracts.Item{}
		var factor, qty, qtyBase string
		if err := rows.Scan(&it.ID, &it.ProductID, &it.VariantID, &it.LocationID,
			&it.UOM, &factor, &qty, &qtyBase); err != nil {
			return nil, err
		}
		it.UOMFactor = toFloat(factor)
		it.Qty = toFloat(qty)
		it.QtyBase = toFloat(qtyBase)
		out = append(out, it)
	}
	return out, rows.Err()
}

// DeliveredQty sums confirmed-delivered base qty for one SO line position.
// Replacement shipments (document_kind <> 'order') are excluded: they ship
// new goods, they don't fulfill the order.
func (r *Repository) DeliveredQty(ctx context.Context, soID, productID, variantID int64) (float64, error) {
	var total sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(i.qty_base), 0) FROM delivery_note_items i
		 JOIN delivery_notes d ON d.id = i.delivery_id
		 WHERE d.sales_order_id = ? AND d.status = 'confirmed'
		   AND d.document_kind = 'order'
		   AND i.product_id = ? AND i.variant_id = ?`,
		soID, productID, variantID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return toFloat(total.String), nil
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
	placeholders := strings.Repeat("?,", len(soIDs)-1) + "?"
	args := make([]any, 0, len(soIDs))
	for _, id := range soIDs {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT d.sales_order_id, i.product_id, i.variant_id, COALESCE(SUM(i.qty_base), 0)
		 FROM delivery_note_items i
		 JOIN delivery_notes d ON d.id = i.delivery_id
		 WHERE d.sales_order_id IN (`+placeholders+`)
		   AND d.status = 'confirmed' AND d.document_kind = 'order'
		 GROUP BY d.sales_order_id, i.product_id, i.variant_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var soID, productID, variantID int64
		var total sql.NullString
		if err := rows.Scan(&soID, &productID, &variantID, &total); err != nil {
			return nil, err
		}
		out[BulkKey(soID, productID, variantID)] = toFloat(total.String)
	}
	return out, rows.Err()
}

// DraftCounts counts draft order-kind notes per order — one GROUP BY for the
// whole work queue. A waiting draft is why an order can show remaining qty
// and still need no new SJ.
func (r *Repository) DraftCounts(ctx context.Context, soIDs []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(soIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(soIDs)-1) + "?"
	args := make([]any, 0, len(soIDs))
	for _, id := range soIDs {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT sales_order_id, COUNT(*) FROM delivery_notes
		 WHERE sales_order_id IN (`+placeholders+`)
		   AND status = 'draft' AND document_kind = 'order'
		 GROUP BY sales_order_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var soID, n int64
		if err := rows.Scan(&soID, &n); err != nil {
			return nil, err
		}
		out[soID] = n
	}
	return out, rows.Err()
}

// WaitingReturn lists draft order-kind notes oldest-first for the work
// queue's "waiting_return" tab — SJ fisik yang belum kembali. One bounded
// query; age bands mirror legacy (today / >2 / >7 days from delivery date).
func (r *Repository) WaitingReturn(ctx context.Context, branchID int64, from, to, age string, limit int) ([]*contracts.DeliveryNote, error) {
	conds := []string{"branch_id = ?", "status = 'draft'", "document_kind = 'order'"}
	args := []any{branchID}
	if strings.TrimSpace(from) != "" {
		conds = append(conds, "delivery_date >= ?")
		args = append(args, strings.TrimSpace(from)+" 00:00:00")
	}
	if strings.TrimSpace(to) != "" {
		conds = append(conds, "delivery_date <= ?")
		args = append(args, strings.TrimSpace(to)+" 23:59:59")
	}
	switch age {
	case "today":
		conds = append(conds, "DATEDIFF(CURDATE(), delivery_date) = 0")
	case "gt_2":
		conds = append(conds, "DATEDIFF(CURDATE(), delivery_date) > 2")
	case "gt_7":
		conds = append(conds, "DATEDIFF(CURDATE(), delivery_date) > 7")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+noteColumns+" FROM delivery_notes WHERE "+strings.Join(conds, " AND ")+
			" ORDER BY delivery_date ASC, id ASC LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.DeliveryNote
	for rows.Next() {
		n := &contracts.DeliveryNote{}
		if err := scanRow(rows, n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// CreateNote inserts a draft note with lines, returning the id.
func (r *Repository) CreateNote(ctx context.Context, n *contracts.DeliveryNote, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	kind := n.DocumentKind
	if kind == "" {
		kind = contracts.DocumentKindOrder
	}
	res, err := tx.ExecContext(ctx,
		`INSERT INTO delivery_notes
		 (number, branch_id, sales_order_id, delivery_date, driver_name,
		  vehicle_plate, warehouse_staff_name, drop_location_note,
		  document_kind, sales_return_id,
		  status, notes, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?)`,
		n.Number, n.BranchID, n.SalesOrderID, n.DeliveryDate,
		nullIfEmpty(n.DriverName), nullIfEmpty(n.VehiclePlate),
		nullIfEmpty(n.WarehouseStaffName), nullIfEmpty(n.DropLocationNote),
		kind, nullIfZero(n.SalesReturnID),
		nullIfEmpty(n.Notes), actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range n.Items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO delivery_note_items
			 (delivery_id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, it.ProductID, it.VariantID, it.LocationID, it.UOM,
			it.UOMFactor, it.Qty, it.QtyBase); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// SetStatus moves the note lifecycle.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE delivery_notes SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, id)
	return err
}

// SetDispatch stamps the dispatch + confirm event. Our draft→confirmed flow
// merges dispatch and receipt: both stamp the confirmer and time, while the
// recipient columns carry the receipt evidence.
func (r *Repository) SetDispatch(ctx context.Context, id int64, n *contracts.DeliveryNote, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE delivery_notes
		 SET recipient_name = ?, recipient_signature_status = ?,
		     recipient_signature_missing_reason = ?,
		     dispatched_at = UTC_TIMESTAMP(6), dispatched_by = ?,
		     confirmed_at = UTC_TIMESTAMP(6), confirmed_by = ?,
		     updated_by = ?
		 WHERE id = ?`,
		nullIfEmpty(n.RecipientName), nullIfEmpty(n.RecipientSignatureStatus),
		nullIfEmpty(n.RecipientSignatureMissingReason),
		actorID, actorID, actorID, id)
	return err
}

// List searches notes of a branch, newest first.
func (r *Repository) List(ctx context.Context, branchID, soID int64, status string, limit, offset int) ([]*contracts.DeliveryNote, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if soID != 0 {
		conds += " AND sales_order_id = ?"
		args = append(args, soID)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+noteColumns+" FROM delivery_notes WHERE "+conds+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.DeliveryNote
	for rows.Next() {
		n := &contracts.DeliveryNote{}
		if err := scanRow(rows, n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, branchID, soID int64, status string) (int64, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if soID != 0 {
		conds += " AND sales_order_id = ?"
		args = append(args, soID)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM delivery_notes WHERE "+conds, args...).Scan(&total)
	return total, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// OrderIDsByNotes maps note id -> sales order id in one grouped read.
// Unknown ids are simply absent from the result.
func (r *Repository) OrderIDsByNotes(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	args, ok := dedupeIDs(ids)
	if !ok {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(args)-1) + "?"
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, sales_order_id FROM delivery_notes WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, orderID int64
		if err := rows.Scan(&id, &orderID); err != nil {
			return nil, err
		}
		out[id] = orderID
	}
	return out, rows.Err()
}

// dedupeIDs drops zero and duplicate ids, returning them as query args.
// ok is false when nothing is left to ask for — callers skip the query.
func dedupeIDs(ids []int64) ([]any, bool) {
	seen := make(map[int64]bool, len(ids))
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		args = append(args, id)
	}
	return args, len(args) > 0
}
