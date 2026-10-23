package infrastructure

import (
	"context"

	"mini-erp/internal/shared/dbutil"
)

// NegativePosition is one stock position below zero on hand.
type NegativePosition struct {
	ProductID  int64
	VariantID  int64
	LocationID int64
	OnHand     float64
}

// NegativePositions lists the branch's positions with on_hand < 0, most
// negative first (data-health A6, ADR-0025). One read, no write.
func (r *Repository) NegativePositions(ctx context.Context, branchID int64) ([]NegativePosition, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, variant_id, location_id, on_hand FROM stock_balances
		 WHERE branch_id = ? AND on_hand < 0 ORDER BY on_hand, product_id`, branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []NegativePosition
	for rows.Next() {
		var p NegativePosition
		var qty string
		if err := rows.Scan(&p.ProductID, &p.VariantID, &p.LocationID, &qty); err != nil {
			return nil, err
		}
		p.OnHand = dbutil.ParseDecimal(qty)
		out = append(out, p)
	}
	return out, rows.Err()
}
