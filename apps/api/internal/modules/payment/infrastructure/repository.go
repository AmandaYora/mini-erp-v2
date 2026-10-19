package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/modules/payment/infrastructure/sqlc"
	"mini-erp/internal/shared/timeutil"
)

// Repository owns the payment module tables.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// DB exposes the pool for module-scoped transactions.
func (r *Repository) DB() *sql.DB { return r.db }

// GetPayment returns nil, nil when missing.
func (r *Repository) GetPayment(ctx context.Context, id int64) (*contracts.Payment, error) {
	row, err := r.q.GetPayment(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toPayment(row), nil
}

// PaymentByClientRef returns the branch's payment created with that client
// reference, or nil.
func (r *Repository) PaymentByClientRef(ctx context.Context, branchID int64, clientRef string) (*contracts.Payment, error) {
	row, err := r.q.GetPaymentByClientRef(ctx, sqlc.GetPaymentByClientRefParams{
		BranchID: branchID, ClientRef: nullStr(clientRef),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toPayment(sqlc.GetPaymentRow(row)), nil
}

// AllocationsByPayment returns a payment's allocations in row order.
func (r *Repository) AllocationsByPayment(ctx context.Context, paymentID int64) ([]*contracts.Allocation, error) {
	rows, err := r.q.ListAllocationsByPayment(ctx, paymentID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Allocation
	for _, row := range rows {
		out = append(out, toAllocation(row.OrderType, row.OrderID, row.Amount))
	}
	return out, nil
}

// PaidForOrder sums active allocations against one document.
func (r *Repository) PaidForOrder(ctx context.Context, orderType string, orderID int64) (int64, error) {
	return r.q.SumPaidForOrder(ctx, sqlc.SumPaidForOrderParams{OrderType: orderType, OrderID: orderID})
}

// CreditUsedForReturn sums the part of a return's saldo already used on bills.
func (r *Repository) CreditUsedForReturn(ctx context.Context, returnType string, returnID int64) (int64, error) {
	return r.q.SumCreditUsedForReturn(ctx, sqlc.SumCreditUsedForReturnParams{OrderType: returnType, OrderID: returnID})
}

// CreditUsedForReturns batches CreditUsedForReturn over one return type.
func (r *Repository) CreditUsedForReturns(ctx context.Context, returnType string, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.q.SumCreditUsedForReturns(ctx, sqlc.SumCreditUsedForReturnsParams{OrderType: returnType, OrderIds: ids})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.OrderID] = row.Total
	}
	return out, nil
}

// PaidForOrders batches PaidForOrder over one document type: a single
// grouped query instead of one per row (balance/ledger paths).
func (r *Repository) PaidForOrders(ctx context.Context, orderType string, orderIDs []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.SumPaidForOrders(ctx, sqlc.SumPaidForOrdersParams{OrderType: orderType, OrderIds: orderIDs})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.OrderID] = row.Total
	}
	return out, nil
}

func insertPaymentTx(ctx context.Context, q *sqlc.Queries, p *contracts.Payment, actorID int64) (int64, error) {
	paidAt, err := parsePaidAt(p.PaidAt)
	if err != nil {
		return 0, err
	}
	res, err := q.InsertPayment(ctx, sqlc.InsertPaymentParams{
		Number:           p.Number,
		BranchID:         p.BranchID,
		PartyID:          p.PartyID,
		PartyType:        p.PartyType,
		Direction:        p.Direction,
		Amount:           p.Amount,
		Method:           p.Method,
		PaidAt:           paidAt,
		Notes:            nullStr(p.Notes),
		ClientRef:        nullStr(p.ClientRef),
		AmountTendered:   sql.NullInt64{Int64: p.AmountTendered, Valid: p.AmountTendered > 0},
		ReferenceNumber:  nullStr(p.Reference),
		CreditSourceType: nullStr(p.CreditSourceType),
		CreditSourceID:   nullID(p.CreditSourceID),
		CreatedBy:        actor(actorID),
		UpdatedBy:        actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, a := range p.Allocations {
		if err := q.InsertAllocation(ctx, sqlc.InsertAllocationParams{
			PaymentID: id, OrderType: a.OrderType, OrderID: a.OrderID, Amount: a.Amount,
		}); err != nil {
			return 0, err
		}
	}
	return id, nil
}

// TxHook runs inside the repository's open transaction just before commit;
// an error rolls the whole transaction back (ADR-0011: the audit trail of a
// money mutation co-commits with it).
type TxHook func(ctx context.Context, tx *sql.Tx) error

// CreatePayment inserts header + allocations atomically. hook (optional)
// runs in the same transaction with the new id available through the
// returned closure argument.
func (r *Repository) CreatePayment(ctx context.Context, p *contracts.Payment, actorID int64, hook func(ctx context.Context, tx *sql.Tx, id int64) error) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	id, err := insertPaymentTx(ctx, r.q.WithTx(tx), p, actorID)
	if err != nil {
		return 0, err
	}
	if hook != nil {
		if err := hook(ctx, tx, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// CreatePaymentPair inserts both legs of a credit offset in ONE transaction
// and links them through pair_id, so a failure can never leave one leg alone.
// Returns the two ids in argument order.
func (r *Repository) CreatePaymentPair(ctx context.Context, first, second *contracts.Payment, actorID int64) (int64, int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	id1, err := insertPaymentTx(ctx, q, first, actorID)
	if err != nil {
		return 0, 0, err
	}
	id2, err := insertPaymentTx(ctx, q, second, actorID)
	if err != nil {
		return 0, 0, err
	}
	if err := q.SetPairID(ctx, sqlc.SetPairIDParams{PairID: nullID(id2), ID: id1}); err != nil {
		return 0, 0, err
	}
	if err := q.SetPairID(ctx, sqlc.SetPairIDParams{PairID: nullID(id1), ID: id2}); err != nil {
		return 0, 0, err
	}
	return id1, id2, tx.Commit()
}

// SetStatus flips active/cancelled. Cancelled payments stop counting toward
// every outstanding figure (computed live, never stored).
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	return r.q.SetPaymentStatus(ctx, sqlc.SetPaymentStatusParams{
		Status: status, Now: timeutil.NowUTC(), UpdatedBy: actor(actorID), ID: id,
	})
}

// CancelWithPair cancels an ACTIVE payment together with its paired leg (CAS:
// only rows still active change). Returns how many rows were cancelled; 0 means
// another writer already cancelled it.
//
// hook (optional) runs inside the same transaction, only when a row changed;
// an error from it rolls the cancellation back (ADR-0011).
func (r *Repository) CancelWithPair(ctx context.Context, id, actorID int64, hook TxHook) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := r.q.WithTx(tx).CancelActiveWithPair(ctx, sqlc.CancelActiveWithPairParams{
		Now: timeutil.NowUTC(), Actor: actor(actorID), ID: id, PairID: nullID(id),
	})
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n > 0 && hook != nil {
		if err := hook(ctx, tx); err != nil {
			return 0, err
		}
	}
	return n, tx.Commit()
}

// PairedPayment returns the opposite leg of a paired payment, or nil.
func (r *Repository) PairedPayment(ctx context.Context, pairID int64) (*contracts.Payment, error) {
	if pairID == 0 {
		return nil, nil
	}
	return r.GetPayment(ctx, pairID)
}

// List searches payments of a branch, newest first.
func (r *Repository) List(ctx context.Context, branchID, partyID int64, status string, limit, offset int) ([]*contracts.Payment, error) {
	rows, err := r.q.ListPayments(ctx, sqlc.ListPaymentsParams{
		BranchID: branchID, PartyID: nullID(partyID), Status: nullStr(status),
		Limit: int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	var out []*contracts.Payment
	for _, row := range rows {
		out = append(out, toPayment(sqlc.GetPaymentRow(row)))
	}
	return out, nil
}

// CancelledAmong returns which of ids are cancelled (one IN query).
func (r *Repository) CancelledAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := r.q.ListCancelledAmong(ctx, sqlc.ListCancelledAmongParams{BranchID: branchID, Ids: ids})
	if err != nil {
		return nil, err
	}
	var out []int64
	out = append(out, rows...)
	return out, nil
}

// ActiveAfter pages active payments by ascending id after a cursor.
func (r *Repository) ActiveAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.Payment, error) {
	rows, err := r.q.ListActivePaymentsAfter(ctx, sqlc.ListActivePaymentsAfterParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var out []*contracts.Payment
	for _, row := range rows {
		out = append(out, toPayment(sqlc.GetPaymentRow(row)))
	}
	return out, nil
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, branchID, partyID int64, status string) (int64, error) {
	return r.q.CountPayments(ctx, sqlc.CountPaymentsParams{
		BranchID: branchID, PartyID: nullID(partyID), Status: nullStr(status),
	})
}

// LedgerByParty lists a party's active payments with allocations, newest first.
func (r *Repository) LedgerByParty(ctx context.Context, branchID, partyID int64, limit, offset int) ([]*contracts.Payment, error) {
	return r.List(ctx, branchID, partyID, "active", limit, offset)
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
	uniq := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		uniq = append(uniq, id)
	}
	if len(uniq) == 0 {
		return out, nil
	}
	rows, err := r.q.ListAllocationOrdersByPayments(ctx, sqlc.ListAllocationOrdersByPaymentsParams{
		OrderType: contracts.OrderSales, PaymentIds: uniq,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.PaymentID] = append(out[row.PaymentID], row.OrderID)
	}
	return out, nil
}
