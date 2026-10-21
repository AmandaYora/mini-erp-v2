package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"time"

	"mini-erp/internal/modules/salesreturn/contracts"
	"mini-erp/internal/modules/salesreturn/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// Repository owns the salesreturn module tables. Data access goes through the
// sqlc-generated Queries (ADR-0012); only the dynamic list filter in
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
func (r *Repository) GetReturn(ctx context.Context, id int64) (*contracts.SalesReturn, error) {
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

// ReturnedQty sums confirmed-returned base qty for one SO line position.
// Only confirmed returns count — drafts reserve nothing.
func (r *Repository) ReturnedQty(ctx context.Context, soID, productID, variantID int64) (float64, error) {
	return r.q.ReturnedQty(ctx, sqlc.ReturnedQtyParams{
		SalesOrderID: soID, ProductID: productID, VariantID: variantID,
	})
}

// CommittedQtyBySO sums base qty per "product/variant" over the order's draft
// AND confirmed returns (P6): a draft holds its share of what can still come
// back, so two drafts can never both claim the same delivered goods.
func (r *Repository) CommittedQtyBySO(ctx context.Context, soID int64) (map[string]float64, error) {
	rows, err := r.q.CommittedQtyBySO(ctx, soID)
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
func (r *Repository) ConfirmedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.SalesReturn, error) {
	rows, err := r.q.ListConfirmedAfter(ctx, sqlc.ListConfirmedAfterParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var out []*contracts.SalesReturn
	for _, row := range rows {
		out = append(out, toConfirmed(row))
	}
	return out, nil
}

// ActiveReturnCount counts the order's live (draft + confirmed) returns.
func (r *Repository) ActiveReturnCount(ctx context.Context, soID int64) (int64, error) {
	return r.q.ActiveReturnCount(ctx, soID)
}

// CreateReturn inserts a draft return with lines + replacement lines,
// returning the id. All inserts share one tx.
func (r *Repository) CreateReturn(ctx context.Context, ret *contracts.SalesReturn, replacements []*contracts.ReplacementItem, actorID int64) (int64, error) {
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
		Number:                    ret.Number,
		BranchID:                  ret.BranchID,
		SalesOrderID:              ret.SalesOrderID,
		ReturnDate:                returnDate,
		Subtotal:                  ret.Subtotal,
		DiscountTotal:             ret.DiscountTotal,
		TaxTotal:                  ret.TaxTotal,
		TaxType:                   ret.TaxType,
		TaxRate:                   ret.TaxRate,
		Total:                     ret.Total,
		Notes:                     dbutil.NullString(ret.Notes),
		ReturnMode:                returnModeOrDefault(ret.ReturnMode),
		ReplacementDeliveryStatus: replacementStatusOrDefault(ret.ReplacementDeliveryStatus),
		Reason:                    ret.Reason,
		ReplacementSubtotal:       ret.ReplacementSubtotal,
		ReplacementTaxTotal:       ret.ReplacementTaxTotal,
		ReplacementTotal:          ret.ReplacementTotal,
		DifferenceAmount:          ret.DifferenceAmount,
		CreatedBy:                 actor(actorID),
		UpdatedBy:                 actor(actorID),
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
			ReturnCondition: conditionOrDefault(it.Condition),
		}); err != nil {
			return 0, err
		}
	}
	for _, it := range replacements {
		if err := q.InsertReplacementItem(ctx, sqlc.InsertReplacementItemParams{
			ReturnID: id, ProductID: it.ProductID, VariantID: it.VariantID,
			LocationID: it.LocationID, Uom: it.UOM,
			UomFactor: it.UOMFactor, Qty: it.Qty, QtyBase: it.QtyBase,
			UnitPrice: it.UnitPrice, TaxBase: it.TaxBase, TaxAmount: it.TaxAmount,
			LineTotal: it.LineTotal,
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

// SetReplacementStatus moves the exchange replacement pipeline as a CAS: it
// only moves from the expected status, and reports whether it did.
func (r *Repository) SetReplacementStatus(ctx context.Context, id int64, from, to string, actorID int64) (bool, error) {
	res, err := r.q.SetReplacementStatus(ctx, sqlc.SetReplacementStatusParams{
		ToStatus: to, Now: now(), ActorID: actor(actorID), ID: id, FromStatus: from,
	})
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n == 1, err
}

// InsertReplacementItems stores exchange replacement lines (priced once at
// creation from the live catalog — a new sale in effect).
func (r *Repository) InsertReplacementItems(ctx context.Context, returnID int64, items []*contracts.ReplacementItem) error {
	for _, it := range items {
		if err := r.q.InsertReplacementItem(ctx, sqlc.InsertReplacementItemParams{
			ReturnID: returnID, ProductID: it.ProductID, VariantID: it.VariantID,
			LocationID: it.LocationID, Uom: it.UOM,
			UomFactor: it.UOMFactor, Qty: it.Qty, QtyBase: it.QtyBase,
			UnitPrice: it.UnitPrice, TaxBase: it.TaxBase, TaxAmount: it.TaxAmount,
			LineTotal: it.LineTotal,
		}); err != nil {
			return err
		}
	}
	return nil
}

// ReplacementItems lists an exchange return's replacement lines.
func (r *Repository) ReplacementItems(ctx context.Context, returnID int64) ([]*contracts.ReplacementItem, error) {
	rows, err := r.q.ListReplacementItems(ctx, returnID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.ReplacementItem
	for _, row := range rows {
		out = append(out, toReplacement(row))
	}
	return out, nil
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
		Auto:            s.Auto,
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

// SetReduction records the amount automatically offset against the order bill.
func (r *Repository) SetReduction(ctx context.Context, id, amount int64) error {
	return r.q.SetReceivableReduction(ctx, sqlc.SetReceivableReductionParams{
		ReceivableReduction: amount, ID: id,
	})
}

// ReleaseAutoOffset zeroes the automatic offset and drops its memo in one
// transaction.
func (r *Repository) ReleaseAutoOffset(ctx context.Context, id int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	if err := q.SetReceivableReduction(ctx, sqlc.SetReceivableReductionParams{ReceivableReduction: 0, ID: id}); err != nil {
		return err
	}
	if err := q.DeleteAutoSettlements(ctx, id); err != nil {
		return err
	}
	return tx.Commit()
}

// OrderIDsByReturns maps return id -> originating sales order id in one
// grouped read. Unknown ids are simply absent from the result.
func (r *Repository) OrderIDsByReturns(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	seen := make(map[int64]bool, len(ids))
	args := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		args = append(args, id)
	}
	if len(args) == 0 {
		return out, nil
	}
	rows, err := r.q.OrderIDsByReturns(ctx, args)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.SalesOrderID
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

// ReturnedByProduct sums returned base qty per product over confirmed returns
// dated within [from, to].
func (r *Repository) ReturnedByProduct(ctx context.Context, branchID int64, from, to string) (map[int64]float64, error) {
	a, b, err := dayBounds(from, to)
	if err != nil {
		return nil, err
	}
	rows, err := r.q.ReturnedByProduct(ctx, sqlc.ReturnedByProductParams{
		BranchID: branchID, FromAt: a, ToAt: b,
	})
	if err != nil {
		return nil, err
	}
	out := map[int64]float64{}
	for _, row := range rows {
		out[row.ProductID] = row.Total
	}
	return out, nil
}

// NetAdjustment is the confirmed returns' effect on the sales figure.
func (r *Repository) NetAdjustment(ctx context.Context, branchID int64, from, to string) (int64, error) {
	a, b, err := dayBounds(from, to)
	if err != nil {
		return 0, err
	}
	return r.q.NetAdjustment(ctx, sqlc.NetAdjustmentParams{
		BranchID: branchID, FromAt: a, ToAt: b,
	})
}

func creditRef(id, returnID int64, number string, branchID, orderID int64, date time.Time, amount int64, status string) *contracts.CreditSettlement {
	return &contracts.CreditSettlement{
		ID: id, ReturnID: returnID, ReturnNumber: number, BranchID: branchID, OrderID: orderID,
		Date: date.Format(wireDateTime), Amount: amount, ReturnStatus: status,
	}
}

// GetCreditSettlement resolves one settlement row of type customer_credit with
// its parent return, or nil when missing / of another type.
func (r *Repository) GetCreditSettlement(ctx context.Context, id int64) (*contracts.CreditSettlement, error) {
	row, err := r.q.GetCreditSettlement(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.SettlementType != contracts.SettleCustomerCredit {
		return nil, nil
	}
	return creditRef(row.ID, row.ReturnID, row.Number, row.BranchID, row.SalesOrderID, row.SettlementDate, row.Amount, row.Status), nil
}

// CreditSettlementsAfter pages customer_credit rows of confirmed returns.
func (r *Repository) CreditSettlementsAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.CreditSettlement, error) {
	rows, err := r.q.ListCreditSettlementsAfter(ctx, sqlc.ListCreditSettlementsAfterParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*contracts.CreditSettlement, 0, len(rows))
	for _, row := range rows {
		out = append(out, creditRef(row.ID, row.ReturnID, row.Number, row.BranchID, row.SalesOrderID, row.SettlementDate, row.Amount, row.Status))
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

// CreditSettlementIDs lists a return's customer_credit settlement ids.
func (r *Repository) CreditSettlementIDs(ctx context.Context, returnID int64) ([]int64, error) {
	return r.q.ListCreditSettlementIDsByReturn(ctx, returnID)
}

// CreditSettled sums the return's credit settlements (one scalar read).
func (r *Repository) CreditSettled(ctx context.Context, returnID int64) (int64, error) {
	return r.q.SumCreditSettled(ctx, returnID)
}

// OrderEffects sums the confirmed returns' money effect per order: two grouped
// reads, chunked by 500 order ids.
func (r *Repository) OrderEffects(ctx context.Context, branchID int64, orderIDs []int64) (map[int64]contracts.OrderEffect, error) {
	out := map[int64]contracts.OrderEffect{}
	for start := 0; start < len(orderIDs); start += 500 {
		end := start + 500
		if end > len(orderIDs) {
			end = len(orderIDs)
		}
		chunk := orderIDs[start:end]
		totals, err := r.q.OrderReturnTotals(ctx, sqlc.OrderReturnTotalsParams{BranchID: branchID, OrderIds: chunk})
		if err != nil {
			return nil, err
		}
		for _, row := range totals {
			e := out[row.SalesOrderID]
			e.Adjustment, e.Offset = row.Adjustment, row.OffsetTotal
			out[row.SalesOrderID] = e
		}
		memos, err := r.q.OrderReturnSettlements(ctx, sqlc.OrderReturnSettlementsParams{BranchID: branchID, OrderIds: chunk})
		if err != nil {
			return nil, err
		}
		for _, row := range memos {
			e := out[row.SalesOrderID]
			switch row.SettlementType {
			case contracts.SettleRefund:
				e.Refunded += row.Total
			case contracts.SettleCustomerCredit:
				e.Credit += row.Total
			case contracts.SettleCollectPayment:
				e.Collected += row.Total
			}
			out[row.SalesOrderID] = e
		}
	}
	return out, nil
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
