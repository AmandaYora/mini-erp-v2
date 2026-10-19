package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/shared/listquery"
)

// ListFilter is the filter set of the purchase return list and its count.
type ListFilter struct {
	Search string // number, notes, reason, or the order the caller resolved
	// SearchPurchaseOrderIDs are source PO ids resolved by the caller from
	// Search through the purchasing contract (order number / supplier, RT-01).
	SearchPurchaseOrderIDs []int64
	Statuses               []string
	PurchaseOrderID int64
	From, To        string // return_date, YYYY-MM-DD, half-open on the upper bound
}

// where is the single source of the filter, shared by ListFiltered and CountFiltered.
func (f ListFilter) where(branchID int64) (string, []any) {
	return listquery.New().
		Eq("branch_id", branchID).
		StatusCSV("status", strings.Join(f.Statuses, ",")).
		Eq("purchase_order_id", f.PurchaseOrderID).
		DateRange("return_date", f.From, f.To).
		LikeOrIn(f.Search, []string{"number", "notes", "reason"}, "purchase_order_id", f.SearchPurchaseOrderIDs).
		SQL()
}

// List keeps the narrow signature internal callers use (status may be a CSV).
func (r *Repository) List(ctx context.Context, branchID, poID int64, status string, limit, offset int) ([]*contracts.PurchaseReturn, error) {
	return r.ListFiltered(ctx, branchID, ListFilter{PurchaseOrderID: poID, Statuses: listquery.SplitCSV(status)}, limit, offset)
}

// Count counts the narrow List filter set.
func (r *Repository) Count(ctx context.Context, branchID, poID int64, status string) (int64, error) {
	return r.CountFiltered(ctx, branchID, ListFilter{PurchaseOrderID: poID, Statuses: listquery.SplitCSV(status)})
}

// ListFiltered searches returns of a branch, newest first.
func (r *Repository) ListFiltered(ctx context.Context, branchID int64, f ListFilter, limit, offset int) ([]*contracts.PurchaseReturn, error) {
	conds, args := f.where(branchID)
	args = append(args, limit, offset)
	// raw-sql: variable-arity per-word LIKE filter (listquery) shared with CountFiltered
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, branch_id, purchase_order_id, return_date, subtotal,
			discount_total, tax_total, tax_type, tax_rate, total, status, notes, reason
		 FROM purchase_returns WHERE `+conds+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.PurchaseReturn
	for rows.Next() {
		ret := &contracts.PurchaseReturn{}
		var returned sql.NullTime
		var notes sql.NullString
		var rate float64
		if err := rows.Scan(&ret.ID, &ret.Number, &ret.BranchID, &ret.PurchaseOrderID,
			&returned, &ret.Subtotal, &ret.DiscountTotal, &ret.TaxTotal,
			&ret.TaxType, &rate, &ret.Total, &ret.Status, &notes, &ret.Reason); err != nil {
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
		"SELECT COUNT(*) FROM purchase_returns WHERE "+conds, args...).Scan(&total)
	return total, err
}
