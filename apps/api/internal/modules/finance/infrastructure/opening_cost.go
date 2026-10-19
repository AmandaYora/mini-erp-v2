package infrastructure

import (
	"context"
	"database/sql"

	"mini-erp/internal/shared/dbutil"
)

// SyntheticCostBase is the first stock_movement_id used by cost-only ledger
// rows (K-05 "lengkapi harga modal"). Real stock movements count up from 1, so
// the two ranges never meet; the costing cursor ignores this range (see
// MaxCostMovementID) so a synthetic row can never hide a real movement.
const SyntheticCostBase int64 = 1 << 62

// RefOpeningCost is the ledger ref_type of a cost-only row: it adds value to a
// position that already holds quantity at no recorded cost, and moves no stock.
const RefOpeningCost = "opening_cost"

// NextSyntheticCostIDs reserves n consecutive ids above every synthetic id in
// use. Callers serialise on the per-branch costing lock; the unique movement
// key at the database is the backstop for two branches racing.
func (r *Repository) NextSyntheticCostIDs(ctx context.Context, n int) ([]int64, error) {
	var max sql.NullInt64
	if err := r.db.QueryRowContext(ctx,
		"SELECT MAX(stock_movement_id) FROM finance_inventory_cost_movements WHERE stock_movement_id >= ?",
		SyntheticCostBase).Scan(&max); err != nil {
		return nil, err
	}
	next := SyntheticCostBase
	if max.Valid {
		next = max.Int64 + 1
	}
	ids := make([]int64, n)
	for i := range ids {
		ids[i] = next + int64(i)
	}
	return ids, nil
}

// InsertCostValueTx writes one cost-only row inside the caller's transaction:
// qty 0, total = value, unit = the cost it prices the position at. It is
// never estimated — an operator set it explicitly.
func (r *Repository) InsertCostValueTx(ctx context.Context, tx *sql.Tx, branchID, productID, variantID, id int64, unit float64, value int64, movedAt string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO finance_inventory_cost_movements
		 (branch_id, product_id, variant_id, stock_movement_id, ref_type, ref_id, direction,
		  qty_base, unit_cost, total_cost, is_estimated, moved_at)
		 VALUES (?, ?, ?, ?, ?, NULL, 'in', 0, ?, ?, 0, ?)`,
		branchID, productID, variantID, id, RefOpeningCost, unit, value, dbutil.NullIfEmpty(normalizeStamp(movedAt)))
	return err
}

// UnjournaledCostValue sums the ledger value of movements that move stock value
// without any journal by design: cross-branch transfers (ref_type transfer /
// transfer_cancel — each branch's ledger takes the other's cost, nothing is
// booked) and, until the finance opening is posted, opening stock (the cutover
// journal is what will absorb it).
func (r *Repository) UnjournaledCostValue(ctx context.Context, branchID int64) (transfer, opening float64, err error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT CASE WHEN ref_type LIKE 'transfer%' THEN 'transfer' ELSE 'opening' END, COALESCE(SUM(total_cost), 0)
		 FROM finance_inventory_cost_movements
		 WHERE branch_id = ? AND (ref_type LIKE 'transfer%' OR ref_type = 'opening')
		 GROUP BY 1`, branchID)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var kind, v string
		if err := rows.Scan(&kind, &v); err != nil {
			return 0, 0, err
		}
		if kind == "transfer" {
			transfer = dbutil.ParseDecimal(v)
		} else {
			opening = dbutil.ParseDecimal(v)
		}
	}
	return transfer, opening, rows.Err()
}
