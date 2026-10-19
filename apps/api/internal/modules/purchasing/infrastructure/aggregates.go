package infrastructure

import (
	"context"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/purchasing/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// StatusCounts counts orders per status for one branch in one grouped query.
// The fixed state machine (draft/confirmed/completed/cancelled) is read
// straight off the status column — no config tables exist by design (D4).
func (r *Repository) StatusCounts(ctx context.Context, branchID int64) (map[string]int64, error) {
	rows, err := r.q.CountOrdersByStatus(ctx, branchID)
	if err != nil {
		return nil, err
	}
	out := map[string]int64{}
	for _, row := range rows {
		out[row.Status] = row.Total
	}
	return out, nil
}

// OrderParties maps order ids to their party ids in one grouped read — the
// batch behind balance computation over many returns (A3). Unknown ids are
// absent from the map.
func (r *Repository) OrderParties(ctx context.Context, orderIDs []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.OrderPartiesByIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.PartyID
	}
	return out, nil
}

// CountMissingSupplierInvoiceBetween counts the same orders within an order-date range.
func (r *Repository) CountMissingSupplierInvoiceBetween(ctx context.Context, branchID int64, from, to string) (int64, error) {
	a, err := dbutil.ParseDateTime(from)
	if err != nil {
		return 0, err
	}
	b, err := dbutil.ParseDateTime(to)
	if err != nil {
		return 0, err
	}
	return r.q.CountMissingSupplierInvoiceBetween(ctx, sqlc.CountMissingSupplierInvoiceBetweenParams{
		BranchID: branchID, FromAt: a, ToAt: b})
}

// CountMissingSupplierInvoice counts taxed orders past draft without a
// supplier invoice number — one COUNT query for the readiness gate.
func (r *Repository) CountMissingSupplierInvoice(ctx context.Context, branchID int64) (int64, error) {
	return r.q.CountMissingSupplierInvoice(ctx, branchID)
}

// CountOrders counts the non-cancelled orders dated within a range.
func (r *Repository) CountOrders(ctx context.Context, branchID int64, from, to string) (int64, error) {
	a, b, err := dayRange(from, to)
	if err != nil {
		return 0, err
	}
	return r.q.CountOrdersBetween(ctx, sqlc.CountOrdersBetweenParams{BranchID: branchID, FromAt: a, ToAt: b})
}

// OpenOrderTotals pages confirmed/completed orders by ascending id.
func (r *Repository) OpenOrderTotals(ctx context.Context, branchID, afterID int64, limit int) ([]contracts.OrderTotal, error) {
	rows, err := r.q.ListOpenOrderTotals(ctx, sqlc.ListOpenOrderTotalsParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	var out []contracts.OrderTotal
	for _, row := range rows {
		out = append(out, toOrderTotal(row))
	}
	return out, nil
}

// ReportOrders pages a date-range order list with the exact total and the sum
// of grand totals over the whole filter (not just the page).
func (r *Repository) ReportOrders(ctx context.Context, branchID int64, status, from, to string, limit, offset int) ([]*contracts.OrderSummary, int64, int64, error) {
	fromAt, err := boundFrom(from)
	if err != nil {
		return nil, 0, 0, err
	}
	toAt, err := boundTo(to)
	if err != nil {
		return nil, 0, 0, err
	}
	st := dbutil.NullString(status)
	totals, err := r.q.ReportOrderTotals(ctx, sqlc.ReportOrderTotalsParams{
		BranchID: branchID, Status: st, FromAt: fromAt, ToAt: toAt})
	if err != nil {
		return nil, 0, 0, err
	}
	rows, err := r.q.ReportOrdersPage(ctx, sqlc.ReportOrdersPageParams{
		BranchID: branchID, Status: st, FromAt: fromAt, ToAt: toAt, Limit: int32(limit), Offset: int32(offset)})
	if err != nil {
		return nil, 0, 0, err
	}
	var out []*contracts.OrderSummary
	for _, row := range rows {
		out = append(out, toSummary(sqlc.ListOrdersByPartyRow(row)))
	}
	return out, totals.Total, totals.GrandSum, nil
}
