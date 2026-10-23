package infrastructure

// raw-sql: the sales order list, its COUNT and the keyset export take a
// variable-arity search (one LIKE per word, OR-ed with an IN of party ids
// resolved by the caller) and a whitelisted dynamic ORDER BY. sqlc cannot
// express either (ADR-0012 §Hand-written SQL); the filter is built by
// shared/listquery so List, Count and ListAfter can never drift apart. Covered
// by internal/integration/sqlc_sales_test.go and f3a_orders_list_test.go.

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure/sqlc"
	"mini-erp/internal/shared/listquery"
)

// ListFilter is the filter set of the sales order list, count and export.
// SearchPartyIDs are party ids resolved by the caller from Search through
// the party contract: a row matches by number OR by one of those parties.
type ListFilter struct {
	Search         string
	SearchPartyIDs []int64
	Statuses       []string
	Channel        string
	PaymentTerms   string
	// PaymentStatus (unpaid|partial|paid|overdue|received) is resolved by the service
	// against the payment contract — the SQL never sees it; the service turns it
	// into Statuses/OverdueBefore below and filters the paid amounts in memory.
	PaymentStatus string
	// OverdueBefore (YYYY-MM-DD) keeps net orders due before that day (overdue
	// pre-filter; the balance check happens in the service).
	OverdueBefore string
	PartyID       int64
	// Origin narrows to "standard" or "delivery" (orders built from SJ Langsung).
	Origin   string
	From, To string // order_date, YYYY-MM-DD, half-open on the upper bound
	// OrderBy is a whitelisted "expr ASC|DESC"; empty = newest id first.
	OrderBy string
}

// where is the single source of the filter, shared by List, Count and ListAfter.
func (f ListFilter) where(branchID int64) (string, []any) {
	w := listquery.New().
		Eq("branch_id", branchID).
		StatusCSV("status", strings.Join(f.Statuses, ",")).
		Eq("channel", f.Channel).
		Eq("payment_terms", f.PaymentTerms).
		Eq("party_id", f.PartyID).
		Eq("origin", f.Origin).
		DateRange("order_date", f.From, f.To).
		LikeOrIn(f.Search, []string{"number"}, "party_id", f.SearchPartyIDs)
	if f.OverdueBefore != "" {
		w.Raw("payment_terms = 'net' AND due_date IS NOT NULL AND due_date < ?", f.OverdueBefore+" 00:00:00")
	}
	return w.SQL()
}

func (f ListFilter) orderBy() string {
	if f.OrderBy == "" {
		return "id DESC"
	}
	return f.OrderBy + ", id DESC"
}

// rawOrderColumns is the GetOrder projection, in the order scanHeader reads it.
const rawOrderColumns = `id, number, branch_id, party_id, channel, member_code,
	order_date, due_date, payment_terms, tax_type, tax_rate, subtotal,
	discount_total, tax_total, grand_total, status, notes,
	ship_to_address_id, ship_to_label, ship_to_recipient, ship_to_phone,
	ship_to_address, tax_invoice_number, tax_invoice_date, client_ref,
	created_by, assigned_user_id, credit_approved_by, credit_approved_at, created_at, origin`

// List returns one page via a deferred join: the page of ids is picked on the
// narrow index, only those rows are read wide.
func (r *Repository) List(ctx context.Context, branchID int64, f ListFilter, limit, offset int) ([]*contracts.SalesOrder, error) {
	where, args := f.where(branchID)
	args = append(args, limit, offset)
	// raw-sql: variable-arity search + dynamic ORDER BY (see file header)
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+prefixed("o.", rawOrderColumns)+` FROM sales_orders o
		 JOIN (SELECT id FROM sales_orders WHERE `+where+` ORDER BY `+f.orderBy()+` LIMIT ? OFFSET ?) x ON x.id = o.id
		 ORDER BY `+prefixed("o.", f.orderBy()), args...)
	if err != nil {
		return nil, err
	}
	return scanOrders(rows)
}

// ListAfter is the keyset page of the export: id strictly below afterID
// (0 = from the top), newest first.
func (r *Repository) ListAfter(ctx context.Context, branchID int64, f ListFilter, afterID int64, limit int) ([]*contracts.SalesOrder, error) {
	where, args := f.where(branchID)
	if afterID > 0 {
		where += " AND id < ?"
		args = append(args, afterID)
	}
	args = append(args, limit)
	// raw-sql: variable-arity search (see file header)
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+rawOrderColumns+" FROM sales_orders WHERE "+where+" ORDER BY id DESC LIMIT ?", args...)
	if err != nil {
		return nil, err
	}
	return scanOrders(rows)
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, branchID int64, f ListFilter) (int64, error) {
	where, args := f.where(branchID)
	var total int64
	// raw-sql: variable-arity search (see file header)
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sales_orders WHERE "+where, args...).Scan(&total)
	return total, err
}

// scanOrders reads the rows into the generated header row type so the same
// mapper serves the sqlc and the raw path; rows are fully read and closed here.
func scanOrders(rows *sql.Rows) ([]*contracts.SalesOrder, error) {
	defer func() { _ = rows.Close() }()
	var out []*contracts.SalesOrder
	for rows.Next() {
		var h sqlc.GetOrderRow
		if err := rows.Scan(&h.ID, &h.Number, &h.BranchID, &h.PartyID, &h.Channel, &h.MemberCode,
			&h.OrderDate, &h.DueDate, &h.PaymentTerms, &h.TaxType, &h.TaxRate, &h.Subtotal,
			&h.DiscountTotal, &h.TaxTotal, &h.GrandTotal, &h.Status, &h.Notes,
			&h.ShipToAddressID, &h.ShipToLabel, &h.ShipToRecipient, &h.ShipToPhone,
			&h.ShipToAddress, &h.TaxInvoiceNumber, &h.TaxInvoiceDate, &h.ClientRef,
			&h.CreatedBy, &h.AssignedUserID, &h.CreditApprovedBy, &h.CreditApprovedAt, &h.CreatedAt, &h.Origin); err != nil {
			return nil, err
		}
		out = append(out, toOrder(h))
	}
	return out, rows.Err()
}

// prefixed qualifies a comma-separated list (columns or ORDER BY terms) with
// a table alias.
func prefixed(alias, list string) string {
	parts := strings.Split(list, ",")
	for i, p := range parts {
		parts[i] = alias + strings.TrimSpace(p)
	}
	return strings.Join(parts, ", ")
}
