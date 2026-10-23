package infrastructure

import (
	"context"
	"database/sql"

	"mini-erp/internal/shared/dbutil"
)

// Data-health reads (ADR-0025). Each is one grouped, read-only statement over
// this module's own tables: no costing drain, no advisory lock, no write.

// ProductCost is one product's running ledger position (all variants).
type ProductCost struct {
	ProductID int64
	Qty       float64
	Value     float64
}

// StockedProductCosts returns the per-product ledger position of every product
// still holding stock (qty > 0) — one GROUP BY on ix_finance_cost_position.
func (r *Repository) StockedProductCosts(ctx context.Context, branchID int64) ([]ProductCost, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id,
			COALESCE(SUM(CASE WHEN direction = 'in' THEN qty_base ELSE -qty_base END), 0) AS qty,
			COALESCE(SUM(total_cost), 0)
		 FROM finance_inventory_cost_movements WHERE branch_id = ?
		 GROUP BY product_id HAVING qty > 0`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []ProductCost
	for rows.Next() {
		var p ProductCost
		var qty, val string
		if err := rows.Scan(&p.ProductID, &qty, &val); err != nil {
			return nil, err
		}
		p.Qty, p.Value = dbutil.ParseDecimal(qty), dbutil.ParseDecimal(val)
		out = append(out, p)
	}
	return out, rows.Err()
}

// OpeningOnlyPositions returns the (product, variant) positions whose only
// cost-ledger rows are the opening balance (and value-only opening_cost
// corrections) and that still hold a quantity: stock never counted, sold or
// received since the books opened.
func (r *Repository) OpeningOnlyPositions(ctx context.Context, branchID int64) ([]CostPosition, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, variant_id,
			COALESCE(SUM(CASE WHEN direction = 'in' THEN qty_base ELSE -qty_base END), 0) AS q,
			COALESCE(SUM(total_cost), 0)
		 FROM finance_inventory_cost_movements WHERE branch_id = ?
		 GROUP BY product_id, variant_id
		 HAVING SUM(ref_type NOT IN ('opening', 'opening_cost')) = 0 AND q <> 0`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []CostPosition
	for rows.Next() {
		var p CostPosition
		var qty, val string
		if err := rows.Scan(&p.ProductID, &p.VariantID, &qty, &val); err != nil {
			return nil, err
		}
		p.Qty, p.Total = dbutil.ParseDecimal(qty), dbutil.ParseDecimal(val)
		out = append(out, p)
	}
	return out, rows.Err()
}

// ActiveMonths returns every YYYYMM the branch has booked activity in, plus
// every month with an explicit period row, each with whether it is closed.
// Months without a period row are open (GetPeriod nil = never touched).
func (r *Repository) ActiveMonths(ctx context.Context, branchID int64) (map[int]bool, error) {
	out := map[int]bool{}
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT YEAR(entry_date) * 100 + MONTH(entry_date) FROM finance_account_daily WHERE branch_id = ?`, branchID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ym int
		if err := rows.Scan(&ym); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[ym] = false
	}
	if err := closeRows(rows); err != nil {
		return nil, err
	}
	rows, err = r.db.QueryContext(ctx,
		`SELECT year * 100 + month, status = 'closed' FROM finance_periods WHERE branch_id = ?`, branchID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ym int
		var closed bool
		if err := rows.Scan(&ym, &closed); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out[ym] = closed
	}
	return out, closeRows(rows)
}

func closeRows(rows *sql.Rows) error {
	err := rows.Err()
	if cerr := rows.Close(); err == nil {
		err = cerr
	}
	return err
}

// UnbalancedNumbers lists up to limit journal numbers dated on/after from
// whose legs do not net to zero (same rule as CountUnbalanced).
func (r *Repository) UnbalancedNumbers(ctx context.Context, branchID int64, from string, limit int) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT e.number
		 FROM finance_journal_lines l
		 JOIN finance_journal_entries e ON e.id = l.entry_id
		 WHERE e.branch_id = ? AND e.entry_date >= ?
		 GROUP BY l.entry_id, e.number
		 HAVING COALESCE(SUM(l.debit), 0) <> COALESCE(SUM(l.credit), 0)
		    OR COALESCE(SUM(l.debit), 0) <= 0
		 ORDER BY l.entry_id LIMIT ?`, branchID, from, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}
