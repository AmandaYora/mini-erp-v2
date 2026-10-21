package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"mini-erp/internal/modules/purchasereturn/contracts"
	"mini-erp/internal/modules/purchasereturn/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// Repository owns the purchasereturn module tables. Data access goes through
// the sqlc-generated Queries (ADR-0012); only the dynamic list filter in
// listfilter.go is hand-written (raw-sql allow-list).
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// GetReturn returns nil, nil when missing.
func (r *Repository) GetReturn(ctx context.Context, id int64) (*contracts.PurchaseReturn, error) {
	row, err := r.q.GetReturn(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toReturn(row), nil
}

// ItemsByReturn returns the return lines in row order.
func (r *Repository) ItemsByReturn(ctx context.Context, returnID int64) ([]*contracts.Item, error) {
	rows, err := r.q.ListItemsByReturn(ctx, returnID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Item
	for _, row := range rows {
		out = append(out, toItem(row))
	}
	return out, nil
}

// ReturnedQty sums confirmed-returned base qty for one PO line position.
func (r *Repository) ReturnedQty(ctx context.Context, poID, productID, variantID int64) (float64, error) {
	return r.q.ReturnedQty(ctx, sqlc.ReturnedQtyParams{
		PurchaseOrderID: poID, ProductID: productID, VariantID: variantID,
	})
}

// ConfirmedQtyByOrders sums confirmed-returned base qty per order and
// "product/variant" for a page of orders in one grouped read.
func (r *Repository) ConfirmedQtyByOrders(ctx context.Context, poIDs []int64) (map[int64]map[string]float64, error) {
	out := map[int64]map[string]float64{}
	if len(poIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.ConfirmedQtyByOrders(ctx, poIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		m := out[row.PurchaseOrderID]
		if m == nil {
			m = map[string]float64{}
			out[row.PurchaseOrderID] = m
		}
		m[strconv.FormatInt(row.ProductID, 10)+"/"+strconv.FormatInt(row.VariantID, 10)] = row.Total
	}
	return out, nil
}

// CommittedQtyByPO sums base qty per "product/variant" over the order's draft
// AND confirmed returns (P6): a draft holds its share of what can still go
// back, so two drafts can never both claim the same received goods.
func (r *Repository) CommittedQtyByPO(ctx context.Context, poID int64) (map[string]float64, error) {
	rows, err := r.q.CommittedQtyByPO(ctx, poID)
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, row := range rows {
		out[strconv.FormatInt(row.ProductID, 10)+"/"+strconv.FormatInt(row.VariantID, 10)] = row.Total
	}
	return out, nil
}

// Transition moves a return between statuses only from an allowed origin
// (CAS). Returns false when another writer already moved it.
func (r *Repository) Transition(ctx context.Context, id int64, from []string, to string, actorID int64) (bool, error) {
	return r.TransitionAudited(ctx, id, from, to, actorID, nil)
}

// Cancel flips draft/confirmed -> cancelled and records the reason, time and
// actor in ONE statement guarded by the status (CAS). The optional hook runs
// inside the CAS transaction only when this call won, so the audit row
// co-commits with the flip (ADR-0011) and a replay that lost the CAS writes
// none (ADR-0016).
func (r *Repository) Cancel(ctx context.Context, id int64, reason string, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	t := now()
	res, err := r.q.WithTx(tx).CancelReturn(ctx, sqlc.CancelReturnParams{
		Reason: dbutil.NullString(reason), Stamp: sql.NullTime{Time: t, Valid: true},
		ActorID: actor(actorID), Now: t, ID: id,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected != 1 {
		return false, err
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// TransitionAudited is the CAS status flip with an optional hook that runs in
// the same transaction only when this call won (ADR-0011/0016): the audit row
// co-commits with the flip and a replay that lost the CAS writes none.
func (r *Repository) TransitionAudited(ctx context.Context, id int64, from []string, to string, actorID int64, hook func(ctx context.Context, tx *sql.Tx) error) (bool, error) {
	if len(from) == 0 {
		return false, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := r.q.WithTx(tx).TransitionReturn(ctx, sqlc.TransitionReturnParams{
		ToStatus: to, Now: now(), ActorID: actor(actorID), ID: id, FromStatuses: from,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected != 1 {
		return false, err
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// CancelledAmong returns which of ids are cancelled (one IN query).
func (r *Repository) CancelledAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return r.q.CancelledAmong(ctx, sqlc.CancelledAmongParams{BranchID: branchID, Ids: ids})
}

// ConfirmedAfter pages confirmed returns by ascending id after a cursor.
func (r *Repository) ConfirmedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.PurchaseReturn, error) {
	rows, err := r.q.ListConfirmedAfter(ctx, sqlc.ListConfirmedAfterParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var out []*contracts.PurchaseReturn
	for _, row := range rows {
		out = append(out, toConfirmed(row))
	}
	return out, nil
}

// ActiveReturnCount counts the order's live (draft + confirmed) returns.
func (r *Repository) ActiveReturnCount(ctx context.Context, poID int64) (int64, error) {
	return r.q.ActiveReturnCount(ctx, poID)
}

// SetReduction records the amount automatically offset against the order bill.
func (r *Repository) SetReduction(ctx context.Context, id, amount int64) error {
	return r.q.SetPayableReduction(ctx, sqlc.SetPayableReductionParams{
		PayableReduction: amount, ID: id,
	})
}

// CreateReturn inserts a draft return with lines, returning the id.
func (r *Repository) CreateReturn(ctx context.Context, ret *contracts.PurchaseReturn, actorID int64) (int64, error) {
	returnDate, err := parseReturnDate(ret.ReturnDate)
	if err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	res, err := q.InsertReturn(ctx, sqlc.InsertReturnParams{
		Number:          ret.Number,
		BranchID:        ret.BranchID,
		PurchaseOrderID: ret.PurchaseOrderID,
		ReturnDate:      returnDate,
		Subtotal:        ret.Subtotal,
		DiscountTotal:   ret.DiscountTotal,
		TaxTotal:        ret.TaxTotal,
		TaxType:         ret.TaxType,
		TaxRate:         ret.TaxRate,
		Total:           ret.Total,
		Notes:           dbutil.NullString(ret.Notes),
		Reason:          ret.Reason,
		CreatedBy:       actor(actorID),
		UpdatedBy:       actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range ret.Items {
		if err := q.InsertReturnItem(ctx, sqlc.InsertReturnItemParams{
			ReturnID: id, ProductID: it.ProductID, VariantID: it.VariantID,
			LocationID: it.LocationID, Uom: it.UOM,
			UomFactor: it.UOMFactor, Qty: it.Qty, QtyBase: it.QtyBase,
			UnitPrice: it.UnitPrice, DiscountPct: it.DiscountPct,
			DiscountNominal: it.DiscountNominal,
			TaxBase:         it.TaxBase, TaxAmount: it.TaxAmount, LineTotal: it.LineTotal,
		}); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// SetStatus moves the return lifecycle.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	return r.q.SetReturnStatus(ctx, sqlc.SetReturnStatusParams{
		Status: status, Now: now(), UpdatedBy: actor(actorID), ID: id,
	})
}

// InsertSettlement records how return value was settled (memo-level).
func (r *Repository) InsertSettlement(ctx context.Context, returnID int64, s *contracts.Settlement, actorID int64) (int64, error) {
	return r.InsertSettlementAudited(ctx, returnID, s, actorID, nil)
}

// InsertSettlementAudited inserts the memo row and runs the audit hook in the
// same transaction (ADR-0011): a failing trail rolls the row back.
func (r *Repository) InsertSettlementAudited(ctx context.Context, returnID int64, s *contracts.Settlement, actorID int64, hook func(ctx context.Context, tx *sql.Tx, id int64) error) (int64, error) {
	settleDate, err := parseReturnDate(s.Date)
	if err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := r.q.WithTx(tx).InsertSettlement(ctx, sqlc.InsertSettlementParams{
		ReturnID:        returnID,
		SettlementType:  s.Type,
		SettlementDate:  settleDate,
		Amount:          s.Amount,
		PaymentMethod:   dbutil.NullString(s.PaymentMethod),
		ReferenceNumber: dbutil.NullString(s.ReferenceNumber),
		Notes:           dbutil.NullString(s.Notes),
		CreatedBy:       actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
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

// Settlements lists a return's settlement memos, oldest first.
func (r *Repository) Settlements(ctx context.Context, returnID int64) ([]*contracts.Settlement, error) {
	rows, err := r.q.ListSettlements(ctx, returnID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Settlement
	for _, row := range rows {
		out = append(out, toSettlement(row))
	}
	return out, nil
}

// ConfirmedByOrders lists the confirmed returns of the given orders, in chunks
// of 500 ids (one IN query each).
func (r *Repository) ConfirmedByOrders(ctx context.Context, branchID int64, orderIDs []int64) ([]*contracts.Return, error) {
	var out []*contracts.Return
	for start := 0; start < len(orderIDs); start += 500 {
		end := start + 500
		if end > len(orderIDs) {
			end = len(orderIDs)
		}
		rows, err := r.q.ConfirmedByOrders(ctx, sqlc.ConfirmedByOrdersParams{
			BranchID: branchID, OrderIds: orderIDs[start:end],
		})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, toReturnProjection(row))
		}
	}
	return out, nil
}

func creditRef(id, returnID int64, number string, branchID, orderID int64, date time.Time, amount int64, status string) *contracts.CreditSettlement {
	return &contracts.CreditSettlement{
		ID: id, ReturnID: returnID, ReturnNumber: number, BranchID: branchID, OrderID: orderID,
		Date: date.Format(wireDateTime), Amount: amount, ReturnStatus: status,
	}
}

// GetCreditSettlement resolves one supplier_credit settlement row with its
// parent return, or nil when missing / of another type.
func (r *Repository) GetCreditSettlement(ctx context.Context, id int64) (*contracts.CreditSettlement, error) {
	row, err := r.q.GetCreditSettlement(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if normalizeSettlementType(row.SettlementType) != contracts.SettleSupplierCredit {
		return nil, nil
	}
	return creditRef(row.ID, row.ReturnID, row.Number, row.BranchID, row.PurchaseOrderID, row.SettlementDate, row.Amount, row.Status), nil
}

// CreditSettlementsAfter pages supplier_credit rows of confirmed returns.
func (r *Repository) CreditSettlementsAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.CreditSettlement, error) {
	rows, err := r.q.ListCreditSettlementsAfter(ctx, sqlc.ListCreditSettlementsAfterParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*contracts.CreditSettlement, 0, len(rows))
	for _, row := range rows {
		out = append(out, creditRef(row.ID, row.ReturnID, row.Number, row.BranchID, row.PurchaseOrderID, row.SettlementDate, row.Amount, row.Status))
	}
	return out, nil
}

// CancelledCreditSettlementsAmong returns which settlement ids belong to a
// cancelled return (one IN query).
func (r *Repository) CancelledCreditSettlementsAmong(ctx context.Context, branchID int64, ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return r.q.CancelledCreditSettlementsAmong(ctx, sqlc.CancelledCreditSettlementsAmongParams{BranchID: branchID, Ids: ids})
}

// CreditSettlementIDs lists a return's supplier_credit settlement ids.
func (r *Repository) CreditSettlementIDs(ctx context.Context, returnID int64) ([]int64, error) {
	return r.q.ListCreditSettlementIDsByReturn(ctx, returnID)
}

// CreditSettled sums the return's credit settlements (one scalar read).
func (r *Repository) CreditSettled(ctx context.Context, returnID int64) (int64, error) {
	return r.q.SumCreditSettled(ctx, returnID)
}

// CreditSettledByReturns sums the credit settlements per return in one grouped
// read (saldo list, N-41).
func (r *Repository) CreditSettledByReturns(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.q.SumCreditSettledByReturns(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ReturnID] = row.Total
	}
	return out, nil
}
