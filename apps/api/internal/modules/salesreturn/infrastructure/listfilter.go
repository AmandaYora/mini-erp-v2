package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/shared/listquery"
)

// ListFilter is the filter set of the sales return list and its count.
type ListFilter struct {
	Search string // number, notes, reason, or the order the caller resolved
	// SearchOrderIDs are source sales order ids resolved by the caller from
	// Search through the sales contract (order number / customer, RT-01): a
	// row matches by its own text OR by one of those orders.
	SearchOrderIDs []int64
	Statuses       []string
	SalesOrderID   int64
	From, To       string // return_date, YYYY-MM-DD, half-open on the upper bound
}

// where is the single source of the filter, shared by ListFiltered and CountFiltered.
func (f ListFilter) where(branchID int64) (string, []any) {
	return listquery.New().
		Eq("branch_id", branchID).
		StatusCSV("status", strings.Join(f.Statuses, ",")).
		Eq("sales_order_id", f.SalesOrderID).
		DateRange("return_date", f.From, f.To).
		LikeOrIn(f.Search, []string{"number", "notes", "reason"}, "sales_order_id", f.SearchOrderIDs).
		SQL()
}

// List keeps the narrow signature internal callers use (status may be a CSV).
func (r *Repository) List(ctx context.Context, branchID, soID int64, status string, limit, offset int) ([]*contracts.SalesReturn, error) {
	return r.ListFiltered(ctx, branchID, ListFilter{SalesOrderID: soID, Statuses: listquery.SplitCSV(status)}, limit, offset)
}

// Count counts the narrow List filter set.
func (r *Repository) Count(ctx context.Context, branchID, soID int64, status string) (int64, error) {
	return r.CountFiltered(ctx, branchID, ListFilter{SalesOrderID: soID, Statuses: listquery.SplitCSV(status)})
}

// ListFiltered searches returns of a branch, newest first.
func (r *Repository) ListFiltered(ctx context.Context, branchID int64, f ListFilter, limit, offset int) ([]*contracts.SalesReturn, error) {
	conds, args := f.where(branchID)
	args = append(args, limit, offset)
	// raw-sql: variable-arity per-word LIKE filter (listquery) shared with CountFiltered
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, branch_id, sales_order_id, return_date, subtotal,
			discount_total, tax_total, tax_type, tax_rate, total, status, notes,
			return_mode, replacement_delivery_status, reason
		 FROM sales_returns WHERE `+conds+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.SalesReturn
	for rows.Next() {
		ret := &contracts.SalesReturn{}
		var returned sql.NullTime
		var notes sql.NullString
		var rate float64
		if err := rows.Scan(&ret.ID, &ret.Number, &ret.BranchID, &ret.SalesOrderID,
			&returned, &ret.Subtotal, &ret.DiscountTotal, &ret.TaxTotal,
			&ret.TaxType, &rate, &ret.Total, &ret.Status, &notes,
			&ret.ReturnMode, &ret.ReplacementDeliveryStatus, &ret.Reason); err != nil {
			return nil, err
		}
		if returned.Valid {
			ret.ReturnDate = returned.Time.Format(wireDateTime)
		}
		ret.TaxRate = rate
		ret.Notes = notes.String
		out = append(out, ret)
	}
	return out, rows.Err()
}

// CountFiltered counts the ListFiltered set.
func (r *Repository) CountFiltered(ctx context.Context, branchID int64, f ListFilter) (int64, error) {
	conds, args := f.where(branchID)
	var total int64
	// raw-sql: same variable-arity filter as ListFiltered (one builder so list and count cannot drift)
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sales_returns WHERE "+conds, args...).Scan(&total)
	return total, err
}
