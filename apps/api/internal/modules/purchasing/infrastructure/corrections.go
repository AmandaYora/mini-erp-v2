package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/purchasing/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// ErrOrderMoved reports a lost CAS: the order's status (or, for a price
// correction, its total) changed between the read and the write.
var ErrOrderMoved = errors.New("purchase order moved")

// Execer is the open transaction a hook writes through (the audit trail,
// ADR-0011); *sql.Tx satisfies it, and so does audit's AuditTx.
type Execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// TxHook runs inside a correction's transaction.
type TxHook func(ctx context.Context, tx Execer) error

// CancelOrder moves a draft/confirmed order to cancelled with its reason (CAS).
// It returns false when another writer already moved it; hook then never ran.
func (r *Repository) CancelOrder(ctx context.Context, id int64, reason string, actorID int64, hook TxHook) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	at := now()
	res, err := r.q.WithTx(tx).CancelOrder(ctx, sqlc.CancelOrderParams{
		Reason: dbutil.NullString(reason), CancelledAt: sql.NullTime{Time: at, Valid: true},
		ActorID: actor(actorID), Now: at, ID: id,
	})
	if err != nil {
		return false, err
	}
	if n, err := res.RowsAffected(); err != nil || n != 1 {
		return false, err
	}
	if hook != nil {
		if err := hook(ctx, tx); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

// CorrectionWrite is one correction to store: the re-priced lines (written in
// place, ids kept), the new header totals and the history record.
type CorrectionWrite struct {
	Order      *contracts.PurchaseOrder // the order as read (status + old total for the CAS)
	Items      []*contracts.Item        // lines whose economics changed, by ID
	Subtotal   int64
	Discount   int64
	Tax        int64
	Grand      int64
	Correction *contracts.Correction
	ActorID    int64
}

// ApplyCorrection writes one correction atomically: lines, header (a CAS on
// status and the old total, so two corrections can never interleave), the
// history record and the hook. Returns the correction id; ErrOrderMoved on a
// lost CAS.
func (r *Repository) ApplyCorrection(ctx context.Context, w CorrectionWrite, hook func(ctx context.Context, tx Execer, correctionID int64) error) (int64, error) {
	date, err := dbutil.ParseDateTime(w.Correction.Date)
	if err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	at := now()
	var res sql.Result
	if w.Correction.Kind == contracts.CorrectionShortClose {
		res, err = q.ShortCloseOrder(ctx, sqlc.ShortCloseOrderParams{
			Subtotal: w.Subtotal, DiscountTotal: w.Discount, TaxTotal: w.Tax, GrandTotal: w.Grand,
			Reason: dbutil.NullString(w.Correction.Reason), ClosedAt: sql.NullTime{Time: at, Valid: true},
			ActorID: actor(w.ActorID), Now: at, ID: w.Order.ID,
		})
	} else {
		res, err = q.RepriceOrder(ctx, sqlc.RepriceOrderParams{
			Subtotal: w.Subtotal, DiscountTotal: w.Discount, TaxTotal: w.Tax, GrandTotal: w.Grand,
			Now: at, ActorID: actor(w.ActorID), ID: w.Order.ID, Status: w.Order.Status, OldGrandTotal: w.Order.GrandTotal,
		})
	}
	if err != nil {
		return 0, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return 0, err
	} else if n != 1 {
		return 0, ErrOrderMoved
	}
	for _, it := range w.Items {
		if err := q.UpdateItemEconomics(ctx, sqlc.UpdateItemEconomicsParams{
			Qty: it.Qty, QtyBase: it.QtyBase, UnitPrice: it.UnitPrice, DiscountPct: it.DiscountPct,
			DiscountNominal: it.DiscountNominal, LineTotal: it.LineTotal, TaxBase: it.TaxBase, TaxAmount: it.TaxAmount,
			ID: it.ID, OrderID: w.Order.ID,
		}); err != nil {
			return 0, err
		}
	}
	c := w.Correction
	ins, err := q.InsertCorrection(ctx, sqlc.InsertCorrectionParams{
		BranchID: w.Order.BranchID, OrderID: w.Order.ID, Kind: c.Kind, CorrectionDate: date, Reason: c.Reason,
		OldGrandTotal: c.OldGrandTotal, NewGrandTotal: c.NewGrandTotal, ValueDelta: c.ValueDelta, TaxDelta: c.TaxDelta,
		CreatedAt: at, CreatedBy: actor(w.ActorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := ins.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, l := range c.Lines {
		if err := q.InsertCorrectionLine(ctx, sqlc.InsertCorrectionLineParams{
			CorrectionID: id, OrderItemID: l.OrderItemID, ProductID: l.ProductID, VariantID: l.VariantID,
			ProductName: l.ProductName, Uom: l.UOM, StockTracked: l.StockTracked,
			ReceivedQty: l.ReceivedQty, ReceivedQtyBase: l.ReceivedQtyBase, OldQty: l.OldQty, NewQty: l.NewQty,
			OldUnitPrice: l.OldUnitPrice, NewUnitPrice: l.NewUnitPrice,
			OldDiscountPct: l.OldDiscountPct, NewDiscountPct: l.NewDiscountPct,
			OldDiscountNominal: l.OldDiscountNominal, NewDiscountNominal: l.NewDiscountNominal,
			OldTaxBase: l.OldTaxBase, NewTaxBase: l.NewTaxBase, OldTaxAmount: l.OldTaxAmount, NewTaxAmount: l.NewTaxAmount,
			OldLineTotal: l.OldLineTotal, NewLineTotal: l.NewLineTotal, ValueDelta: l.ValueDelta, TaxDelta: l.TaxDelta,
		}); err != nil {
			return 0, err
		}
	}
	if hook != nil {
		if err := hook(ctx, tx, id); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

func toCorrection(r sqlc.PurchaseOrderCorrection) *contracts.Correction {
	return &contracts.Correction{
		ID: r.ID, BranchID: r.BranchID, OrderID: r.OrderID, Kind: r.Kind,
		Date: r.CorrectionDate.Format(wireDate), Reason: r.Reason,
		OldGrandTotal: r.OldGrandTotal, NewGrandTotal: r.NewGrandTotal,
		ValueDelta: r.ValueDelta, TaxDelta: r.TaxDelta,
		CreatedAt: wibStamp(sql.NullTime{Time: r.CreatedAt, Valid: true}), CreatedBy: r.CreatedBy.Int64,
	}
}

// withLines attaches every correction's lines in one grouped read.
func (r *Repository) withLines(ctx context.Context, cs []*contracts.Correction) error {
	if len(cs) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(cs))
	byID := make(map[int64]*contracts.Correction, len(cs))
	for _, c := range cs {
		ids = append(ids, c.ID)
		byID[c.ID] = c
		c.Lines = []*contracts.CorrectionLine{}
	}
	rows, err := r.q.ListCorrectionLines(ctx, ids)
	if err != nil {
		return err
	}
	for _, l := range rows {
		c := byID[l.CorrectionID]
		if c == nil {
			continue
		}
		c.Lines = append(c.Lines, &contracts.CorrectionLine{
			ID: l.ID, OrderItemID: l.OrderItemID, ProductID: l.ProductID, VariantID: l.VariantID,
			ProductName: l.ProductName, UOM: l.Uom, StockTracked: l.StockTracked,
			ReceivedQty: l.ReceivedQty, ReceivedQtyBase: l.ReceivedQtyBase, OldQty: l.OldQty, NewQty: l.NewQty,
			OldUnitPrice: l.OldUnitPrice, NewUnitPrice: l.NewUnitPrice,
			OldDiscountPct: l.OldDiscountPct, NewDiscountPct: l.NewDiscountPct,
			OldDiscountNominal: l.OldDiscountNominal, NewDiscountNominal: l.NewDiscountNominal,
			OldTaxBase: l.OldTaxBase, NewTaxBase: l.NewTaxBase, OldTaxAmount: l.OldTaxAmount, NewTaxAmount: l.NewTaxAmount,
			OldLineTotal: l.OldLineTotal, NewLineTotal: l.NewLineTotal, ValueDelta: l.ValueDelta, TaxDelta: l.TaxDelta,
		})
	}
	return nil
}

// GetCorrection returns one correction with lines, or nil when missing.
func (r *Repository) GetCorrection(ctx context.Context, id int64) (*contracts.Correction, error) {
	row, err := r.q.GetCorrection(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	c := toCorrection(row)
	if err := r.withLines(ctx, []*contracts.Correction{c}); err != nil {
		return nil, err
	}
	return c, nil
}

// CorrectionsByOrder lists an order's corrections with lines, oldest first.
func (r *Repository) CorrectionsByOrder(ctx context.Context, orderID int64) ([]*contracts.Correction, error) {
	rows, err := r.q.ListCorrectionsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	out := make([]*contracts.Correction, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCorrection(row))
	}
	if err := r.withLines(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListPriceCorrectionsAfter pages the branch's journal-bearing price
// corrections by ascending id (headers only).
func (r *Repository) ListPriceCorrectionsAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.Correction, error) {
	rows, err := r.q.ListPriceCorrectionsAfter(ctx, sqlc.ListPriceCorrectionsAfterParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*contracts.Correction, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCorrection(row))
	}
	return out, nil
}
