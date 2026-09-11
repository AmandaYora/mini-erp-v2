package infrastructure

import (
	"context"
	"strings"
)

// StatusCounts counts orders per status for one branch in one grouped query.
// The fixed state machine (draft/confirmed/completed/cancelled) is read
// straight off the status column — no config tables exist by design (D4).
func (r *Repository) StatusCounts(ctx context.Context, branchID int64) (map[string]int64, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT status, COUNT(*) FROM purchase_orders WHERE branch_id = ? GROUP BY status`,
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
		"SELECT id, party_id FROM purchase_orders WHERE id IN ("+placeholders+")", args...)
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

// CountMissingSupplierInvoice counts taxed orders past draft without a
// supplier invoice number — one COUNT query for the readiness gate.
func (r *Repository) CountMissingSupplierInvoice(ctx context.Context, branchID int64) (int64, error) {
	var n int64
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM purchase_orders
		 WHERE branch_id = ? AND status IN ('confirmed','completed')
		   AND tax_type <> 'none'
		   AND (supplier_invoice_number IS NULL OR supplier_invoice_number = '')`,
		branchID).Scan(&n)
	return n, err
}
