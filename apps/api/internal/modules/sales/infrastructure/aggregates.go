package infrastructure

import (
	"context"
	"database/sql"
	"math"
	"strings"

	"mini-erp/internal/modules/sales/contracts"
)

// StatusCounts counts orders per status for one branch in one grouped query.
// The fixed state machine (draft/confirmed/completed/cancelled) is read
// straight off the status column — no config tables exist by design (D4).
func (r *Repository) StatusCounts(ctx context.Context, branchID int64) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM sales_orders WHERE branch_id = ? GROUP BY status`,
		branchID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := map[string]int64{}
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// TopProducts ranks products by base qty sold in a range, biggest first. One
// grouped query over the module's own items join: cancelled orders are out
// (they did not sell), everything else counts — operational sales include
// drafts that have not posted yet. Product names come from the frozen line
// snapshots, so no product lookup per row.
func (r *Repository) TopProducts(ctx context.Context, branchID int64, from, to string, limit int) ([]*contracts.TopProduct, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT i.product_id, i.variant_id, i.product_code, i.product_name,
			COALESCE(SUM(i.qty_base), 0), COALESCE(SUM(i.line_total), 0),
			COUNT(DISTINCT o.id)
		 FROM sales_order_items i
		 JOIN sales_orders o ON o.id = i.order_id
		 WHERE o.branch_id = ? AND o.status <> 'cancelled'
		   AND o.order_date >= ? AND o.order_date <= ?
		 GROUP BY i.product_id, i.variant_id, i.product_code, i.product_name
		 ORDER BY SUM(i.qty_base) DESC LIMIT ?`,
		branchID, strings.TrimSpace(from)+" 00:00:00", strings.TrimSpace(to)+" 23:59:59", limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.TopProduct
	for rows.Next() {
		p := &contracts.TopProduct{}
		var qty, revenue string
		if err := rows.Scan(&p.ProductID, &p.VariantID, &p.ProductCode,
			&p.ProductName, &qty, &revenue, &p.Orders); err != nil {
			return nil, err
		}
		// SUM() over integer columns still comes back DECIMAL ("150000.00"),
		// so both aggregates parse through float. Money stays integer
		// rupiah: line_total is BIGINT, the fraction is always .00 (P1).
		p.QtyBase = toFloat(qty)
		p.Revenue = int64(math.Round(toFloat(revenue)))
		out = append(out, p)
	}
	return out, rows.Err()
}

// OpenHeaders lists confirmed non-POS order headers oldest-first for the
// delivery work queue (J3). POS orders never wait in the warehouse queue.
func (r *Repository) OpenHeaders(ctx context.Context, branchID int64, limit int) ([]*contracts.OpenShipment, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, party_id, order_date, due_date, payment_terms, grand_total
		 FROM sales_orders
		 WHERE branch_id = ? AND status = 'confirmed' AND channel <> 'pos'
		 ORDER BY id ASC LIMIT ?`, branchID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.OpenShipment
	for rows.Next() {
		o := &contracts.OpenShipment{}
		var orderDate, due sql.NullTime
		var terms sql.NullString
		if err := rows.Scan(&o.OrderID, &o.Number, &o.PartyID, &orderDate, &due,
			&terms, &o.GrandTotal); err != nil {
			return nil, err
		}
		o.OrderDate = formatNullTime(orderDate)
		o.DueDate = formatNullTime(due)
		o.PaymentTerms = terms.String
		out = append(out, o)
	}
	return out, rows.Err()
}

// ItemsByOrders returns lines for many orders in row order — one grouped
// read for the whole work queue, never one query per order (P3).
func (r *Repository) ItemsByOrders(ctx context.Context, orderIDs []int64) (map[int64][]*contracts.Item, error) {
	out := map[int64][]*contracts.Item{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(orderIDs)-1) + "?"
	args := make([]any, 0, len(orderIDs))
	for _, id := range orderIDs {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT order_id, id, product_id, variant_id, product_code, product_name,
			uom, uom_factor, qty, qty_base, unit_price, discount_pct,
			discount_nominal, line_total
		 FROM sales_order_items WHERE order_id IN (`+placeholders+`)
		 ORDER BY order_id ASC, id ASC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var orderID int64
		it := &contracts.Item{}
		var factor, qty, qtyBase, pct string
		if err := rows.Scan(&orderID, &it.ID, &it.ProductID, &it.VariantID,
			&it.ProductCode, &it.ProductName, &it.UOM, &factor, &qty, &qtyBase,
			&it.UnitPrice, &pct, &it.DiscountNominal, &it.LineTotal); err != nil {
			return nil, err
		}
		it.UOMFactor = toFloat(factor)
		it.Qty = toFloat(qty)
		it.QtyBase = toFloat(qtyBase)
		it.DiscountPct = toFloat(pct)
		out[orderID] = append(out[orderID], it)
	}
	return out, rows.Err()
}

// OrderParties maps order ids to their party ids in one grouped read — the
// batch behind balance computation over many returns (A3). Unknown ids are
// absent from the map.
func (r *Repository) OrderParties(ctx context.Context, orderIDs []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(orderIDs)-1) + "?"
	args := make([]any, 0, len(orderIDs))
	for _, id := range orderIDs {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, party_id FROM sales_orders WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, partyID int64
		if err := rows.Scan(&id, &partyID); err != nil {
			return nil, err
		}
		out[id] = partyID
	}
	return out, rows.Err()
}

// CountMissingTaxInvoice counts taxed orders past draft without a tax
// invoice number — one COUNT query for the readiness gate.
func (r *Repository) CountMissingTaxInvoice(ctx context.Context, branchID int64) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sales_orders
		 WHERE branch_id = ? AND status IN ('confirmed','completed')
		   AND tax_type <> 'none'
		   AND (tax_invoice_number IS NULL OR tax_invoice_number = '')`,
		branchID).Scan(&n)
	return n, err
}

// SummariesByIDs resolves many orders to lightweight rows in one read.
// Unknown ids are absent from the result.
func (r *Repository) SummariesByIDs(ctx context.Context, orderIDs []int64) (map[int64]*contracts.OrderSummary, error) {
	out := map[int64]*contracts.OrderSummary{}
	seen := make(map[int64]bool, len(orderIDs))
	args := make([]any, 0, len(orderIDs))
	for _, id := range orderIDs {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		args = append(args, id)
	}
	if len(args) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(args)-1) + "?"
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, party_id, order_date, due_date, grand_total, status
		 FROM sales_orders WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var s contracts.OrderSummary
		var orderDate, due sql.NullTime
		if err := rows.Scan(&s.ID, &s.Number, &s.PartyID, &orderDate, &due, &s.GrandTotal, &s.Status); err != nil {
			return nil, err
		}
		s.OrderDate, s.DueDate = formatNullTime(orderDate), formatNullTime(due)
		row := s
		out[s.ID] = &row
	}
	return out, rows.Err()
}
