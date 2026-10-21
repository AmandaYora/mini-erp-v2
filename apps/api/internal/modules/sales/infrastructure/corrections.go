package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// FillCorrections loads the cancel / short-close stamps of one order into o
// (detail reads only). A missing row leaves o untouched.
func (r *Repository) FillCorrections(ctx context.Context, o *contracts.SalesOrder) error {
	row, err := r.q.GetOrderCorrections(ctx, o.ID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	o.CancelReason = row.CancelReason.String
	o.CancelledAt = dbutil.FormatNullTime(row.CancelledAt)
	o.CancelledBy = row.CancelledBy.Int64
	o.ShortCloseReason = row.ShortCloseReason.String
	o.ShortClosedAt = dbutil.FormatNullTime(row.ShortClosedAt)
	o.ShortClosedBy = row.ShortClosedBy.Int64
	return nil
}

// CancelWithReason moves a draft/confirmed order to cancelled (CAS) and stamps
// the reason and who/when in the same statement. False = another writer
// already moved it.
func (r *Repository) CancelWithReason(ctx context.Context, id int64, reason string, actorID int64) (bool, error) {
	t := now()
	res, err := r.q.CancelOrderWithReason(ctx, sqlc.CancelOrderWithReasonParams{
		Reason: dbutil.NullString(reason), Stamp: sql.NullTime{Time: t, Valid: true},
		ActorID: actor(actorID), Now: t, ID: id,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	return affected == 1, err
}

// ExecTx is the open transaction an audit hook writes through (ADR-0011): the
// trail row co-commits with the correction. *sql.Tx satisfies it, and so does
// the audit contract's AuditTx, so the use case never names database/sql.
type ExecTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ShortClose rewrites a confirmed order to its delivered lines and completes
// it, atomically: the header CAS (status confirmed → completed) goes first, so
// a lost race changes nothing; then every kept line is rewritten in place
// (its id stays), the removed lines are deleted, and the audit hook joins the
// same transaction (ADR-0011). o carries the new header money and the kept
// lines (with their ids). False = the order was no longer confirmed.
func (r *Repository) ShortClose(ctx context.Context, o *contracts.SalesOrder, removed []int64, reason string, actorID int64,
	audit func(ctx context.Context, tx ExecTx) error) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	t := now()
	res, err := q.ShortCloseOrder(ctx, sqlc.ShortCloseOrderParams{
		Subtotal: o.Subtotal, DiscountTotal: o.DiscountTotal, TaxTotal: o.TaxTotal, GrandTotal: o.GrandTotal,
		Stamp: sql.NullTime{Time: t, Valid: true}, Reason: dbutil.NullString(reason),
		ActorID: actor(actorID), Now: t, ID: o.ID,
	})
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return false, err
	}
	for _, it := range o.Items {
		if err := q.ShortCloseItem(ctx, sqlc.ShortCloseItemParams{
			Qty: it.Qty, QtyBase: it.QtyBase, DiscountNominal: it.DiscountNominal, LineTotal: it.LineTotal,
			TaxBase: it.TaxBase, TaxAmount: it.TaxAmount, ID: it.ID, OrderID: o.ID,
		}); err != nil {
			return false, err
		}
	}
	for _, id := range removed {
		if err := q.DeleteOrderItem(ctx, sqlc.DeleteOrderItemParams{ID: id, OrderID: o.ID}); err != nil {
			return false, err
		}
	}
	if audit != nil {
		if err := audit(ctx, tx); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}
