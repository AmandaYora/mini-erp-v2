package infrastructure

import (
	"context"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/listquery"
)

// SummariesByIDs resolves many purchase orders to lightweight rows in one read
// (chunks of listquery.MaxIn ids). Unknown ids are absent from the result.
func (r *Repository) SummariesByIDs(ctx context.Context, orderIDs []int64) (map[int64]*contracts.OrderSummary, error) {
	out := map[int64]*contracts.OrderSummary{}
	seen := make(map[int64]bool, len(orderIDs))
	var ids []int64
	for _, id := range orderIDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for start := 0; start < len(ids); start += listquery.MaxIn {
		end := start + listquery.MaxIn
		if end > len(ids) {
			end = len(ids)
		}
		rows, err := r.q.SummariesByIDs(ctx, ids[start:end])
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out[row.ID] = &contracts.OrderSummary{
				ID: row.ID, Number: row.Number, PartyID: row.PartyID,
				OrderDate: row.OrderDate.Format(wireDateTime), DueDate: dbutil.FormatNullTime(row.DueDate),
				GrandTotal: row.GrandTotal, Status: row.Status, BranchID: row.BranchID,
				SupplierInvoiceNumber: row.SupplierInvoiceNumber.String,
				SupplierInvoiceDate:   dbutil.FormatNullTime(row.SupplierInvoiceDate),
			}
		}
	}
	return out, nil
}
