package infrastructure

import (
	"context"
	"strings"

	"mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/delivery/infrastructure/sqlc"
	"mini-erp/internal/shared/listquery"
	"mini-erp/internal/shared/timeutil"
)

// ListFilter is the filter set of the delivery note list and its count.
type ListFilter struct {
	Search        string // number, recipient, driver or plate
	Statuses      []string
	SalesOrderID  int64
	From, To      string // delivery_date, YYYY-MM-DD, half-open on the upper bound
	SalesReturnID int64
	DocumentKind  string
	// Origin narrows to "order" or "direct" notes (ADR-0022); Unordered keeps
	// only direct notes that have no order yet.
	Origin    string
	Unordered bool
}

// where is the single source of the filter, shared by ListFiltered and CountFiltered.
func (f ListFilter) where(branchID int64) (string, []any) {
	w := listquery.New().
		Eq("branch_id", branchID).
		StatusCSV("status", strings.Join(f.Statuses, ",")).
		Eq("sales_order_id", f.SalesOrderID).
		Eq("sales_return_id", f.SalesReturnID).
		Eq("document_kind", f.DocumentKind).
		Eq("origin", f.Origin).
		DateRange("delivery_date", f.From, f.To).
		Like(f.Search, "number", "recipient_name", "driver_name", "vehicle_plate")
	if f.Unordered {
		w.Raw("origin = 'direct' AND sales_order_id = 0 AND status <> 'cancelled'")
	}
	return w.SQL()
}

// List keeps the narrow signature internal callers use (status may be a CSV).
func (r *Repository) List(ctx context.Context, branchID, soID int64, status string, limit, offset int) ([]*contracts.DeliveryNote, error) {
	return r.ListFiltered(ctx, branchID, ListFilter{SalesOrderID: soID, Statuses: listquery.SplitCSV(status)}, limit, offset)
}

// Count counts the narrow List filter set.
func (r *Repository) Count(ctx context.Context, branchID, soID int64, status string) (int64, error) {
	return r.CountFiltered(ctx, branchID, ListFilter{SalesOrderID: soID, Statuses: listquery.SplitCSV(status)})
}

// noteColumns is the 31-column projection the raw list/waiting queries share
// with the sqlc note queries (same columns, same order, so rows scan into
// sqlc.GetNoteRow and map through toNote).
const noteColumns = `id, number, branch_id, sales_order_id, delivery_date,
	driver_name, vehicle_plate, warehouse_staff_name, recipient_name,
	recipient_signature_status, recipient_signature_missing_reason,
	drop_location_note, dispatched_at, dispatched_by, confirmed_at,
	confirmed_by, document_kind, sales_return_id, status, notes,
	cancel_reason, cancelled_at, cancelled_by,
	origin, party_id, ship_to_label, ship_to_recipient, ship_to_phone, ship_to_address,
	linked_at, linked_by`

func scanNoteRow(row interface {
	Scan(dest ...any) error
}) (sqlc.GetNoteRow, error) {
	var n sqlc.GetNoteRow
	err := row.Scan(&n.ID, &n.Number, &n.BranchID, &n.SalesOrderID,
		&n.DeliveryDate, &n.DriverName, &n.VehiclePlate, &n.WarehouseStaffName,
		&n.RecipientName, &n.RecipientSignatureStatus, &n.RecipientSignatureMissingReason,
		&n.DropLocationNote, &n.DispatchedAt, &n.DispatchedBy, &n.ConfirmedAt,
		&n.ConfirmedBy, &n.DocumentKind, &n.SalesReturnID, &n.Status, &n.Notes,
		&n.CancelReason, &n.CancelledAt, &n.CancelledBy,
		&n.Origin, &n.PartyID, &n.ShipToLabel, &n.ShipToRecipient, &n.ShipToPhone, &n.ShipToAddress,
		&n.LinkedAt, &n.LinkedBy)
	return n, err
}

// ListFiltered searches notes of a branch, newest first.
func (r *Repository) ListFiltered(ctx context.Context, branchID int64, f ListFilter, limit, offset int) ([]*contracts.DeliveryNote, error) {
	conds, args := f.where(branchID)
	args = append(args, limit, offset)
	// raw-sql: variable-arity per-word LIKE filter (listquery) shared with CountFiltered
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+noteColumns+" FROM delivery_notes WHERE "+conds+" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.DeliveryNote
	for rows.Next() {
		row, err := scanNoteRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, toNote(row))
	}
	return out, rows.Err()
}

// CountFiltered counts the ListFiltered set.
func (r *Repository) CountFiltered(ctx context.Context, branchID int64, f ListFilter) (int64, error) {
	conds, args := f.where(branchID)
	var total int64
	// raw-sql: same variable-arity filter as ListFiltered (one builder so list and count cannot drift)
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM delivery_notes WHERE "+conds, args...).Scan(&total)
	return total, err
}

// WaitingReturn lists draft order-kind notes oldest-first for the work
// queue's "waiting_return" tab — SJ fisik yang belum kembali. One bounded
// query; age bands mirror legacy (today / >2 / >7 days from delivery date).
//
// search matches the SJ number, driver or plate, OR (orderIDs, resolved by the
// caller through the sales contract from the order number / customer) the order
// the note belongs to — applied before the LIMIT so an old note is findable.
func (r *Repository) WaitingReturn(ctx context.Context, branchID int64, search string, orderIDs []int64, from, to, age string, limit int) ([]*contracts.DeliveryNote, error) {
	conds := []string{"branch_id = ?", "status = 'draft'", "document_kind = 'order'"}
	args := []any{branchID}
	if strings.TrimSpace(search) != "" {
		sc, sa := listquery.New().LikeOrIn(search, []string{"number", "driver_name", "vehicle_plate"}, "sales_order_id", orderIDs).SQL()
		conds = append(conds, sc)
		args = append(args, sa...)
	}
	if strings.TrimSpace(from) != "" {
		conds = append(conds, "delivery_date >= ?")
		args = append(args, strings.TrimSpace(from)+" 00:00:00")
	}
	if strings.TrimSpace(to) != "" {
		conds = append(conds, "delivery_date <= ?")
		args = append(args, strings.TrimSpace(to)+" 23:59:59")
	}
	// Age bands are computed from today's WIB calendar day in Go and compared
	// against the bare column (sargable, and independent of the MySQL session
	// time zone, which CURDATE() depended on).
	today := timeutil.NowUTC().In(timeutil.Jakarta)
	day := func(offset int) string { return today.AddDate(0, 0, offset).Format("2006-01-02") + " 00:00:00" }
	switch age {
	case "today":
		conds = append(conds, "delivery_date >= ? AND delivery_date < ?")
		args = append(args, day(0), day(1))
	case "gt_2":
		conds = append(conds, "delivery_date < ?")
		args = append(args, day(-2))
	case "gt_7":
		conds = append(conds, "delivery_date < ?")
		args = append(args, day(-7))
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	args = append(args, limit)
	// raw-sql: optional from/to plus a 4-way age band switch vary the condition
	// arity; sqlc.narg/slice cannot express the switch. Bounded, sargable.
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+noteColumns+" FROM delivery_notes WHERE "+strings.Join(conds, " AND ")+
			" ORDER BY delivery_date ASC, id ASC LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.DeliveryNote
	for rows.Next() {
		row, err := scanNoteRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, toNote(row))
	}
	return out, rows.Err()
}
