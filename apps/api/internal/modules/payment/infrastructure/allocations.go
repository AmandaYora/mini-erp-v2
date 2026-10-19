package infrastructure

import (
	"context"

	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/shared/listquery"
)

// AllocationsByPayments returns the allocations of many payments in row order,
// grouped by payment id, in one IN query per chunk (F4-7).
func (r *Repository) AllocationsByPayments(ctx context.Context, paymentIDs []int64) (map[int64][]*contracts.Allocation, error) {
	out := map[int64][]*contracts.Allocation{}
	for start := 0; start < len(paymentIDs); start += listquery.MaxIn {
		end := start + listquery.MaxIn
		if end > len(paymentIDs) {
			end = len(paymentIDs)
		}
		rows, err := r.q.ListAllocationsByPayments(ctx, paymentIDs[start:end])
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.PaymentID] = append(out[row.PaymentID], toAllocation(row.OrderType, row.OrderID, row.Amount))
		}
	}
	return out, nil
}
