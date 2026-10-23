package infrastructure

import (
	"context"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure/sqlc"
)

// UnshippedOrderTotals pages the draft/confirmed orders (not fully shipped)
// by ascending id after a cursor (data-health A12, ADR-0025).
func (r *Repository) UnshippedOrderTotals(ctx context.Context, branchID, afterID int64, limit int) ([]contracts.OrderTotal, error) {
	rows, err := r.q.ListUnshippedOrderTotals(ctx, sqlc.ListUnshippedOrderTotalsParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]contracts.OrderTotal, 0, len(rows))
	for _, row := range rows {
		out = append(out, toOrderTotal(sqlc.ListOpenOrderTotalsRow(row)))
	}
	return out, nil
}
