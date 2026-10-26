package infrastructure

import (
	"context"

	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/payment/infrastructure/sqlc"
	"mini-erp/internal/shared/listquery"
)

// ListFilter is the filter set of the payment list and its count.
// SearchPartyIDs are party ids the caller resolved from Search through the
// party contract: a row matches by number/notes/reference OR by those parties.
type ListFilter struct {
	Search         string
	SearchPartyIDs []int64
	PartyID        int64
	Status         string
	Method         string
	Direction      string
	From, To       string // paid_at, YYYY-MM-DD, half-open on the upper bound
	// OrderType+OrderID keep only payments allocated to that order (the
	// allocation table belongs to this module, so a sub-select is allowed).
	OrderType string
	OrderID   int64
	// CashAccountID keeps the payments of one company rekening (ADR-0026).
	CashAccountID int64
}

func (f ListFilter) where(branchID int64) (string, []any) {
	w := listquery.New().
		Eq("branch_id", branchID).
		Eq("party_id", f.PartyID).
		Eq("status", f.Status).
		Eq("method", f.Method).
		Eq("direction", f.Direction).
		Eq("cash_account_id", f.CashAccountID).
		DateRange("paid_at", f.From, f.To).
		LikeOrIn(f.Search, []string{"number", "notes", "reference_number"}, "party_id", f.SearchPartyIDs)
	if f.OrderID != 0 {
		w.Raw("id IN (SELECT payment_id FROM payment_allocations WHERE order_type = ? AND order_id = ?)", f.OrderType, f.OrderID)
	}
	return w.SQL()
}

const paymentColumns = `id, number, branch_id, party_id, party_type, direction,
	amount, method, paid_at, notes, status, client_ref, amount_tendered, reference_number, pair_id, cash_account_id`

// ListFiltered returns one page of payments, newest first, via a deferred join.
func (r *Repository) ListFiltered(ctx context.Context, branchID int64, f ListFilter, limit, offset int) ([]*contracts.Payment, error) {
	where, args := f.where(branchID)
	args = append(args, limit, offset)
	// raw-sql: filter set is dynamic (search words x columns OR party-id IN-list, optional date range/order sub-select), shared with CountFiltered
	rows, err := r.db.QueryContext(ctx,
		`SELECT `+paymentColumns+` FROM payments
		 WHERE id IN (SELECT id FROM (SELECT id FROM payments WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?) x)
		 ORDER BY id DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Payment
	for rows.Next() {
		var row sqlc.GetPaymentRow
		if err := rows.Scan(&row.ID, &row.Number, &row.BranchID, &row.PartyID, &row.PartyType, &row.Direction,
			&row.Amount, &row.Method, &row.PaidAt, &row.Notes, &row.Status, &row.ClientRef,
			&row.AmountTendered, &row.ReferenceNumber, &row.PairID, &row.CashAccountID); err != nil {
			return nil, err
		}
		out = append(out, toPayment(row))
	}
	return out, rows.Err()
}

// CountFiltered counts the ListFiltered set.
func (r *Repository) CountFiltered(ctx context.Context, branchID int64, f ListFilter) (int64, error) {
	where, args := f.where(branchID)
	var total int64
	// raw-sql: same dynamic filter as ListFiltered (one builder so list and count cannot drift)
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payments WHERE "+where, args...).Scan(&total)
	return total, err
}
