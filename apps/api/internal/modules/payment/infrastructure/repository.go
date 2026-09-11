package infrastructure

import (
	"context"
	"database/sql"
	"strings"

	"mini-erp/internal/modules/payment/contracts"
)

// Repository owns the payment module tables.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// GetPayment returns nil, nil when missing.
func (r *Repository) GetPayment(ctx context.Context, id int64) (*contracts.Payment, error) {
	p := &contracts.Payment{}
	var paid sql.NullTime
	var notes sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, branch_id, party_id, party_type, direction,
			amount, method, paid_at, notes, status
		 FROM payments WHERE id = ?`, id).
		Scan(&p.ID, &p.Number, &p.BranchID, &p.PartyID, &p.PartyType, &p.Direction,
			&p.Amount, &p.Method, &paid, &notes, &p.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if paid.Valid {
		p.PaidAt = paid.Time.Format("2006-01-02 15:04:05")
	}
	p.Notes = notes.String
	return p, nil
}

// AllocationsByPayment returns a payment's allocations in row order.
func (r *Repository) AllocationsByPayment(ctx context.Context, paymentID int64) ([]*contracts.Allocation, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT order_type, order_id, amount FROM payment_allocations
		 WHERE payment_id = ? ORDER BY id ASC`, paymentID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Allocation
	for rows.Next() {
		var a contracts.Allocation
		if err := rows.Scan(&a.OrderType, &a.OrderID, &a.Amount); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}

// PaidForOrder sums active allocations against one document.
func (r *Repository) PaidForOrder(ctx context.Context, orderType string, orderID int64) (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(a.amount), 0) FROM payment_allocations a
		 JOIN payments p ON p.id = a.payment_id
		 WHERE a.order_type = ? AND a.order_id = ? AND p.status = 'active'`,
		orderType, orderID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Int64, nil
}

// PaidForOrders batches PaidForOrder over one document type: a single
// grouped query instead of one per row (balance/ledger paths).
func (r *Repository) PaidForOrders(ctx context.Context, orderType string, orderIDs []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(orderIDs)-1) + "?"
	args := make([]any, 0, len(orderIDs)+1)
	args = append(args, orderType)
	for _, id := range orderIDs {
		args = append(args, id)
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT a.order_id, COALESCE(SUM(a.amount), 0) FROM payment_allocations a
		 JOIN payments p ON p.id = a.payment_id
		 WHERE a.order_type = ? AND a.order_id IN (`+placeholders+`) AND p.status = 'active'
		 GROUP BY a.order_id`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, total int64
		if err := rows.Scan(&id, &total); err != nil {
			return nil, err
		}
		out[id] = total
	}
	return out, rows.Err()
}

// CreatePayment inserts header + allocations atomically.
func (r *Repository) CreatePayment(ctx context.Context, p *contracts.Payment, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO payments
		 (number, branch_id, party_id, party_type, direction, amount, method,
		  paid_at, notes, status, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		p.Number, p.BranchID, p.PartyID, p.PartyType, p.Direction, p.Amount,
		p.Method, p.PaidAt, nullIfEmpty(p.Notes), actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, a := range p.Allocations {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO payment_allocations (payment_id, order_type, order_id, amount)
			 VALUES (?, ?, ?, ?)`,
			id, a.OrderType, a.OrderID, a.Amount); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// SetStatus flips active/cancelled. Cancelled payments stop counting toward
// every outstanding figure (computed live, never stored).
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE payments SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, id)
	return err
}

// List searches payments of a branch, newest first.
func (r *Repository) List(ctx context.Context, branchID, partyID int64, status string, limit, offset int) ([]*contracts.Payment, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if partyID != 0 {
		conds += " AND party_id = ?"
		args = append(args, partyID)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, branch_id, party_id, party_type, direction,
			amount, method, paid_at, notes, status FROM payments
		 WHERE `+conds+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Payment
	for rows.Next() {
		p := &contracts.Payment{}
		var paid sql.NullTime
		var notes sql.NullString
		if err := rows.Scan(&p.ID, &p.Number, &p.BranchID, &p.PartyID, &p.PartyType,
			&p.Direction, &p.Amount, &p.Method, &paid, &notes, &p.Status); err != nil {
			return nil, err
		}
		if paid.Valid {
			p.PaidAt = paid.Time.Format("2006-01-02 15:04:05")
		}
		p.Notes = notes.String
		out = append(out, p)
	}
	return out, rows.Err()
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, branchID, partyID int64, status string) (int64, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if partyID != 0 {
		conds += " AND party_id = ?"
		args = append(args, partyID)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM payments WHERE "+conds, args...).Scan(&total)
	return total, err
}

// LedgerByParty lists a party's active payments with allocations, newest first.
func (r *Repository) LedgerByParty(ctx context.Context, branchID, partyID int64, limit, offset int) ([]*contracts.Payment, error) {
	return r.List(ctx, branchID, partyID, "active", limit, offset)
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// OrderIDsByPayments maps payment id -> the SALES order ids it settles.
//
// Only `sales` allocations are returned. Sales-return allocations point at a
// return document, and that return is already attributed to its own order by
// the salesreturn module — following both would attribute one economic
// transaction twice.
func (r *Repository) OrderIDsByPayments(ctx context.Context, ids []int64) (map[int64][]int64, error) {
	out := map[int64][]int64{}
	seen := make(map[int64]bool, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, contracts.OrderSales)
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		args = append(args, id)
	}
	if len(args) == 1 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(args)-2) + "?"
	rows, err := r.db.QueryContext(ctx,
		`SELECT payment_id, order_id FROM payment_allocations
		 WHERE order_type = ? AND payment_id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var paymentID, orderID int64
		if err := rows.Scan(&paymentID, &orderID); err != nil {
			return nil, err
		}
		out[paymentID] = append(out[paymentID], orderID)
	}
	return out, rows.Err()
}
