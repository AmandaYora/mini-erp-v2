package infrastructure

import (
	"context"
)

// ProductHasHistory reports whether any movement ever touched the product, in
// any branch or location (EXISTS: it stops at the first row).
func (r *Repository) ProductHasHistory(ctx context.Context, productID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT EXISTS(SELECT 1 FROM stock_movements WHERE product_id = ? LIMIT 1)", productID).Scan(&n)
	return n == 1, err
}

// ProductOnHand sums the product's on-hand quantity over every branch and
// position (legacy guard: stock anywhere in the company blocks archiving).
func (r *Repository) ProductOnHand(ctx context.Context, productID int64) (float64, error) {
	var sum float64
	err := r.db.QueryRowContext(ctx,
		"SELECT COALESCE(SUM(on_hand), 0) FROM stock_balances WHERE product_id = ?", productID).Scan(&sum)
	return sum, err
}

// ProductHasHolds reports an active reservation or an in-flight transfer
// (draft/dispatched) that still references the product.
func (r *Repository) ProductHasHolds(ctx context.Context, productID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM stock_reservations
		                WHERE product_id = ? AND status = 'active' AND qty_remaining > 0
		                  AND expires_at > UTC_TIMESTAMP(6) LIMIT 1)
		     OR EXISTS(SELECT 1 FROM stock_transfer_items i
		                 JOIN stock_transfers t ON t.id = i.transfer_id
		                WHERE i.product_id = ? AND t.status IN ('draft', 'dispatched') LIMIT 1)`,
		productID, productID).Scan(&n)
	return n == 1, err
}

// VariantStockRow is one variant's stock facts.
type VariantStockRow struct {
	VariantID  int64
	OnHand     float64
	HasHistory bool
}

// VariantStockStates answers for every variant of the product that has any
// balance row or movement, in one grouped read.
func (r *Repository) VariantStockStates(ctx context.Context, productID int64) ([]VariantStockRow, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT variant_id, SUM(on_hand), MAX(has_history) FROM (
		   SELECT variant_id, on_hand, 0 AS has_history FROM stock_balances WHERE product_id = ?
		   UNION ALL
		   SELECT DISTINCT variant_id, 0, 1 FROM stock_movements WHERE product_id = ?
		 ) u GROUP BY variant_id`, productID, productID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []VariantStockRow
	for rows.Next() {
		var v VariantStockRow
		var hist int
		if err := rows.Scan(&v.VariantID, &v.OnHand, &hist); err != nil {
			return nil, err
		}
		v.HasHistory = hist == 1
		out = append(out, v)
	}
	return out, rows.Err()
}

// AvailableByProducts sums on_hand - reserved per product for one branch over
// active, non-damaged locations (and every variant) in one grouped read.
func (r *Repository) AvailableByProducts(ctx context.Context, branchID int64, productIDs []int64) (map[int64]float64, error) {
	out := make(map[int64]float64, len(productIDs))
	if len(productIDs) == 0 {
		return out, nil
	}
	args := make([]any, 0, len(productIDs)+1)
	args = append(args, branchID)
	marks := make([]byte, 0, len(productIDs)*2)
	for i, id := range productIDs {
		if i > 0 {
			marks = append(marks, ',')
		}
		marks = append(marks, '?')
		args = append(args, id)
	}
	// raw-sql: stock is a pending (not yet sqlc) module; one grouped read over a caller-sized IN list
	rows, err := r.db.QueryContext(ctx,
		`SELECT b.product_id, SUM(b.on_hand - b.reserved)
		   FROM stock_balances b
		   JOIN stock_locations l ON l.id = b.location_id AND l.status = 'active' AND l.kind <> 'damaged'
		  WHERE b.branch_id = ? AND b.product_id IN (`+string(marks)+`)
		  GROUP BY b.product_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var sum float64
		if err := rows.Scan(&id, &sum); err != nil {
			return nil, err
		}
		out[id] = sum
	}
	return out, rows.Err()
}
