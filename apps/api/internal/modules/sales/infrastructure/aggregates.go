package infrastructure

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/listquery"
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

// TopProducts ranks products by base qty sold in a range, biggest first. One
// grouped query over the module's own items join: cancelled orders are out
// (they did not sell), everything else counts — operational sales include
// drafts that have not posted yet. Product names come from the frozen line
// snapshots, so no product lookup per row.
func (r *Repository) TopProducts(ctx context.Context, branchID int64, from, to string, limit int) ([]*contracts.TopProduct, error) {
	a, b, err := dayRange(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.TopProducts(ctx, sqlc.TopProductsParams{
		BranchID: branchID, FromAt: sql.NullTime{Time: a, Valid: true}, ToAt: sql.NullTime{Time: b, Valid: true},
		Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var out []*contracts.TopProduct
	for _, row := range rows {
		// Money stays integer rupiah: the revenue SUM is cast to BIGINT in SQL (P1).
		out = append(out, &contracts.TopProduct{
			ProductID: row.ProductID, VariantID: row.VariantID, ProductCode: row.ProductCode,
			ProductName: row.ProductName, QtyBase: row.QtyBase, Revenue: row.Revenue, Orders: row.Orders,
		})
	}
	return out, nil
}

// orphanPosAfter is how long a confirmed POS order may sit before the delivery
// work queue treats it as an abandoned checkout.
const orphanPosAfter = 10 * time.Minute

// OpenHeaders lists confirmed order headers oldest-first for the delivery work
// queue (J3). A POS order joins only once it has sat confirmed for 10 minutes
// without being handed over (an abandoned checkout) and is flagged orphanPos.
func (r *Repository) OpenHeaders(ctx context.Context, branchID int64, limit int) ([]*contracts.OpenShipment, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.q.ListOpenHeaders(ctx, sqlc.ListOpenHeadersParams{
		BranchID: branchID, OrphanBefore: now().Add(-orphanPosAfter), Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	var out []*contracts.OpenShipment
	for _, row := range rows {
		out = append(out, &contracts.OpenShipment{
			OrderID: row.ID, Number: row.Number, PartyID: row.PartyID,
			OrderDate: row.OrderDate.Format(wireDateTime), DueDate: dbutil.FormatNullTime(row.DueDate),
			PaymentTerms: row.PaymentTerms, GrandTotal: row.GrandTotal, OrphanPos: row.OrphanPos != 0,
		})
	}
	return out, nil
}

// OpenHeadersFiltered is OpenHeaders with the order-date range and the
// number-or-customer search applied before LIMIT/OFFSET (DL-02). partyIDs are the
// customers the caller resolved from the search term.
func (r *Repository) OpenHeadersFiltered(ctx context.Context, branchID int64, search, from, to string, partyIDs []int64, limit, offset int) ([]*contracts.OpenShipment, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	p := sqlc.ListOpenHeadersFilteredParams{
		BranchID: branchID, OrphanBefore: now().Add(-orphanPosAfter),
		PartyIds: partyIDs, Limit: int32(limit), Offset: int32(offset),
	}
	if t, ok := listquery.ParseDate(from); ok {
		p.FromAt = sql.NullTime{Time: t, Valid: true}
	}
	if t, ok := listquery.ParseDate(to); ok {
		p.ToBefore = sql.NullTime{Time: t.AddDate(0, 0, 1), Valid: true}
	}
	if term := strings.TrimSpace(search); term != "" {
		p.NumberLike = sql.NullString{String: "%" + listquery.EscapeLike(term) + "%", Valid: true}
	}
	rows, err := r.q.ListOpenHeadersFiltered(ctx, p)
	if err != nil {
		return nil, err
	}
	var out []*contracts.OpenShipment
	for _, row := range rows {
		out = append(out, &contracts.OpenShipment{
			OrderID: row.ID, Number: row.Number, PartyID: row.PartyID,
			OrderDate: row.OrderDate.Format(wireDateTime), DueDate: dbutil.FormatNullTime(row.DueDate),
			PaymentTerms: row.PaymentTerms, GrandTotal: row.GrandTotal, OrphanPos: row.OrphanPos != 0,
		})
	}
	return out, nil
}

// SearchOrderIDs lists order ids matching a number substring or caller-resolved
// customer ids (DL-02).
func (r *Repository) SearchOrderIDs(ctx context.Context, branchID int64, term string, partyIDs []int64, limit int) ([]int64, error) {
	return r.q.SearchOrderIDs(ctx, sqlc.SearchOrderIDsParams{
		BranchID: branchID, NumberLike: "%" + listquery.EscapeLike(strings.TrimSpace(term)) + "%",
		PartyIds: partyIDs, Limit: int32(limit),
	})
}

// ItemsByOrders returns lines for many orders in row order — one grouped
// read for the whole work queue, never one query per order (P3).
func (r *Repository) ItemsByOrders(ctx context.Context, orderIDs []int64) (map[int64][]*contracts.Item, error) {
	out := map[int64][]*contracts.Item{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.ListItemsByOrders(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.OrderID] = append(out[row.OrderID], &contracts.Item{
			ID: row.ID, ProductID: row.ProductID, VariantID: row.VariantID,
			ProductCode: row.ProductCode, ProductName: row.ProductName, UOM: row.Uom,
			UOMFactor: row.UomFactor, Qty: row.Qty, QtyBase: row.QtyBase, UnitPrice: row.UnitPrice,
			DiscountPct: row.DiscountPct, DiscountNominal: row.DiscountNominal, LineTotal: row.LineTotal,
		})
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

// CountMissingTaxInvoice counts taxed orders past draft without a tax
// invoice number — one COUNT query for the readiness gate.
func (r *Repository) CountMissingTaxInvoice(ctx context.Context, branchID int64) (int64, error) {
	return r.q.CountMissingTaxInvoice(ctx, branchID)
}

// CountMissingTaxInvoiceBetween counts the same orders within an order-date range.
func (r *Repository) CountMissingTaxInvoiceBetween(ctx context.Context, branchID int64, from, to string) (int64, error) {
	a, err := dbutil.ParseDateTime(from)
	if err != nil {
		return 0, err
	}
	b, err := dbutil.ParseDateTime(to)
	if err != nil {
		return 0, err
	}
	return r.q.CountMissingTaxInvoiceBetween(ctx, sqlc.CountMissingTaxInvoiceBetweenParams{
		BranchID: branchID, FromAt: a, ToAt: b})
}

// SummariesByIDs resolves many orders to lightweight rows in one read
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
				TaxInvoiceNumber: row.TaxInvoiceNumber.String, TaxInvoiceDate: dbutil.FormatNullTime(row.TaxInvoiceDate),
			}
		}
	}
	return out, nil
}

// CompletedSales sums the grand total (tax included) and counts the orders
// completed within a Jakarta-day range — the "penjualan" figure, which needs
// no journal posting.
func (r *Repository) CompletedSales(ctx context.Context, branchID int64, from, to string) (int64, int64, error) {
	start, end := dayBounds(from, to)
	row, err := r.q.SumCompletedSales(ctx, sqlc.SumCompletedSalesParams{
		BranchID: branchID, FromAt: sql.NullTime{Time: start, Valid: true}, ToAt: sql.NullTime{Time: end, Valid: true}})
	if err != nil {
		return 0, 0, err
	}
	return row.GrandSum, row.Total, nil
}

// CountOrders counts the non-cancelled orders dated within a range.
func (r *Repository) CountOrders(ctx context.Context, branchID int64, from, to string) (int64, error) {
	a, b, err := dayRange(from, to)
	if err != nil {
		return 0, err
	}
	return r.q.CountOrdersBetween(ctx, sqlc.CountOrdersBetweenParams{BranchID: branchID, FromAt: a, ToAt: b})
}

// PriorityOrders ranks the live (draft/confirmed) orders with the legacy score:
// past due 100, due within 2 days 80, pending (draft) 60, otherwise 40, plus the
// order's age in days (max 30). One query, scored and ordered in SQL.
func (r *Repository) PriorityOrders(ctx context.Context, branchID int64, today string, limit int) ([]*contracts.PriorityOrder, error) {
	day, err := dbutil.ParseDateTime(today)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ListPriorityOrders(ctx, sqlc.ListPriorityOrdersParams{Today: day, BranchID: branchID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	var out []*contracts.PriorityOrder
	for _, row := range rows {
		out = append(out, &contracts.PriorityOrder{
			ID: row.ID, Number: row.Number, PartyID: row.PartyID, GrandTotal: row.GrandTotal, Status: row.Status,
			OrderDate: row.OrderDate.Format(wireDateTime), DueDate: dbutil.FormatNullTime(row.DueDate), Score: int(row.Score),
		})
	}
	return out, nil
}

// OpenOrderTotals pages the confirmed/completed orders by ascending id after a
// cursor with their grand total — the receivable side of "outstanding per
// order, never below zero". The (branch_id, id) index is forced in the query
// (F4-17, see queries/aggregates.sql).
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

// ItemsByOrdersFull returns the complete line snapshots of many orders (the
// export's detail sheet): one grouped read per call, rows in order/line order.
func (r *Repository) ItemsByOrdersFull(ctx context.Context, orderIDs []int64) (map[int64][]*contracts.Item, error) {
	out := map[int64][]*contracts.Item{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.ListItemsByOrdersFull(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.OrderID] = append(out[row.OrderID], toItem(sqlc.ListItemsByOrderRow{
			ID: row.ID, ProductID: row.ProductID, VariantID: row.VariantID,
			ProductCode: row.ProductCode, ProductName: row.ProductName, Uom: row.Uom,
			UomFactor: row.UomFactor, Qty: row.Qty, QtyBase: row.QtyBase, UnitPrice: row.UnitPrice,
			DiscountPct: row.DiscountPct, DiscountNominal: row.DiscountNominal, LineTotal: row.LineTotal,
			StockTracked: row.StockTracked, ItemType: row.ItemType, VariantName: row.VariantName,
			VariantCode: row.VariantCode, BaseUom: row.BaseUom, PricingSource: row.PricingSource,
			BasePrice: row.BasePrice, DiscountReason: row.DiscountReason, LineNote: row.LineNote,
			Taxable: row.Taxable, TaxBase: row.TaxBase, TaxAmount: row.TaxAmount,
		}))
	}
	return out, nil
}
