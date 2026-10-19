package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/modules/purchasing/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// Repository owns the purchasing module tables. Data access goes through the
// sqlc-generated Queries (ADR-0012); only the dynamic list queries in
// list_raw.go are hand-written.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// GetOrder returns nil, nil when missing.
func (r *Repository) GetOrder(ctx context.Context, id int64) (*contracts.PurchaseOrder, error) {
	row, err := r.q.GetOrder(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toOrder(row), nil
}

// ItemsByOrder returns the document lines in row order.
func (r *Repository) ItemsByOrder(ctx context.Context, orderID int64) ([]*contracts.Item, error) {
	rows, err := r.q.ListItemsByOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Item
	for _, row := range rows {
		out = append(out, toItem(row))
	}
	return out, nil
}

// CreateOrder inserts header + lines atomically.
func (r *Repository) CreateOrder(ctx context.Context, o *contracts.PurchaseOrder, actorID int64) (int64, error) {
	h, err := toHeaderParams(o, true)
	if err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	res, err := q.InsertOrder(ctx, sqlc.InsertOrderParams{
		Number: o.Number, BranchID: o.BranchID, PartyID: o.PartyID, OrderDate: h.orderDate, DueDate: h.dueDate,
		PaymentTerms: o.PaymentTerms, TaxType: o.TaxType, TaxRate: o.TaxRate, Subtotal: o.Subtotal,
		DiscountTotal: o.DiscountTotal, TaxTotal: o.TaxTotal, GrandTotal: o.GrandTotal,
		Notes:                 dbutil.NullString(o.Notes),
		SupplierInvoiceNumber: dbutil.NullString(o.SupplierInvoiceNumber), SupplierInvoiceDate: h.invDate,
		TaxInvoiceNumber: dbutil.NullString(o.TaxInvoiceNumber), TaxInvoiceDate: h.taxInvDate,
		AssignedUserID:  dbutil.NullInt64(o.AssignedUserID),
		PaymentTermDays: h.termDays, CreatedBy: actor(actorID), UpdatedBy: actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := replaceItems(ctx, q, id, o.Items); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateOrder rewrites a draft header + lines atomically.
func (r *Repository) UpdateOrder(ctx context.Context, o *contracts.PurchaseOrder, actorID int64) error {
	h, err := toHeaderParams(o, true)
	if err != nil {
		return err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	if err := q.UpdateOrder(ctx, sqlc.UpdateOrderParams{
		PartyID: o.PartyID, OrderDate: h.orderDate, DueDate: h.dueDate, PaymentTerms: o.PaymentTerms,
		TaxType: o.TaxType, TaxRate: o.TaxRate, Subtotal: o.Subtotal, DiscountTotal: o.DiscountTotal,
		TaxTotal: o.TaxTotal, GrandTotal: o.GrandTotal, Notes: dbutil.NullString(o.Notes),
		SupplierInvoiceNumber: dbutil.NullString(o.SupplierInvoiceNumber), SupplierInvoiceDate: h.invDate,
		TaxInvoiceNumber: dbutil.NullString(o.TaxInvoiceNumber), TaxInvoiceDate: h.taxInvDate,
		AssignedUserID:  dbutil.NullInt64(o.AssignedUserID),
		PaymentTermDays: h.termDays, Now: now(), UpdatedBy: actor(actorID), ID: o.ID,
	}); err != nil {
		return err
	}
	if err := replaceItems(ctx, q, o.ID, o.Items); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceItems(ctx context.Context, q *sqlc.Queries, orderID int64, items []*contracts.Item) error {
	if err := q.DeleteItemsByOrder(ctx, orderID); err != nil {
		return err
	}
	for _, it := range items {
		if err := q.InsertItem(ctx, toInsertItem(orderID, it)); err != nil {
			return err
		}
	}
	return nil
}

// Transition moves the order between statuses only from an allowed origin
// (CAS). Returns false when another writer already moved it.
func (r *Repository) Transition(ctx context.Context, id int64, from []string, to string, actorID int64) (bool, error) {
	if len(from) == 0 {
		return false, nil
	}
	res, err := r.q.TransitionOrder(ctx, sqlc.TransitionOrderParams{
		ToStatus: to, Now: now(), ActorID: actor(actorID), ID: id, FromStatuses: from,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	return affected == 1, err
}

// UpdateHeaderLight rewrites only the header fields that stay editable once
// receipts, payments or returns hang off the order.
func (r *Repository) UpdateHeaderLight(ctx context.Context, o *contracts.PurchaseOrder, actorID int64) error {
	h, err := toHeaderParams(o, false)
	if err != nil {
		return err
	}
	return r.q.UpdateHeaderLight(ctx, sqlc.UpdateHeaderLightParams{
		DueDate: h.dueDate, Notes: dbutil.NullString(o.Notes),
		SupplierInvoiceNumber: dbutil.NullString(o.SupplierInvoiceNumber), SupplierInvoiceDate: h.invDate,
		TaxInvoiceNumber: dbutil.NullString(o.TaxInvoiceNumber), TaxInvoiceDate: h.taxInvDate,
		AssignedUserID:  dbutil.NullInt64(o.AssignedUserID),
		PaymentTermDays: h.termDays, Now: now(), UpdatedBy: actor(actorID), ID: o.ID,
	})
}

// SwitchToNet moves a confirmed cash-terms order to net with a due date (CAS).
func (r *Repository) SwitchToNet(ctx context.Context, id int64, dueDate string, actorID int64) (bool, error) {
	due, err := dbutil.ParseDateTime(dueDate)
	if err != nil {
		return false, err
	}
	res, err := r.q.SwitchToNet(ctx, sqlc.SwitchToNetParams{
		DueDate: sql.NullTime{Time: due, Valid: true}, Now: now(), UpdatedBy: actor(actorID), ID: id,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	return affected == 1, err
}

// SetStatus moves the document lifecycle.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	return r.q.SetStatus(ctx, sqlc.SetStatusParams{Status: status, Now: now(), UpdatedBy: actor(actorID), ID: id})
}

// ListByParty lists a party's documents in a branch, newest first.
func (r *Repository) ListByParty(ctx context.Context, branchID, partyID int64) ([]*contracts.OrderSummary, error) {
	rows, err := r.q.ListOrdersByParty(ctx, sqlc.ListOrdersByPartyParams{BranchID: branchID, PartyID: partyID})
	if err != nil {
		return nil, err
	}
	var out []*contracts.OrderSummary
	for _, row := range rows {
		out = append(out, toSummary(row))
	}
	return out, nil
}

// GetOrderByNumber returns nil, nil when missing (branch-scoped).
func (r *Repository) GetOrderByNumber(ctx context.Context, branchID int64, number string) (*contracts.PurchaseOrder, error) {
	row, err := r.q.GetOrderByNumber(ctx, sqlc.GetOrderByNumberParams{BranchID: branchID, Number: number})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toOrder(sqlc.GetOrderRow(row)), nil
}

// ListSummaries lists lightweight rows for operational readers (assistant
// bot). Empty statuses = all non-cancelled; empty from/to = unbounded.
func (r *Repository) ListSummaries(ctx context.Context, branchID int64, statuses []string, from, to string, limit int) ([]*contracts.OrderSummary, error) {
	fromAt, err := boundFrom(from)
	if err != nil {
		return nil, err
	}
	toAt, err := boundTo(to)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	out := []*contracts.OrderSummary{}
	if len(statuses) == 0 {
		rows, err := r.q.ListOrderSummariesLive(ctx, sqlc.ListOrderSummariesLiveParams{
			BranchID: branchID, FromAt: fromAt, ToAt: toAt, Limit: int32(limit)})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, toSummary(sqlc.ListOrdersByPartyRow(row)))
		}
		return out, nil
	}
	rows, err := r.q.ListOrderSummariesByStatus(ctx, sqlc.ListOrderSummariesByStatusParams{
		BranchID: branchID, Statuses: statuses, FromAt: fromAt, ToAt: toAt, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, toSummary(sqlc.ListOrdersByPartyRow(row)))
	}
	return out, nil
}

// ItemsByOrders returns the lines of many orders in one grouped read, keyed by
// order id, in row order (export page).
func (r *Repository) ItemsByOrders(ctx context.Context, orderIDs []int64) (map[int64][]*contracts.Item, error) {
	out := map[int64][]*contracts.Item{}
	if len(orderIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.ListItemsByOrders(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.OrderID] = append(out[row.OrderID], toItem(sqlc.ListItemsByOrderRow{
			ID: row.ID, ProductID: row.ProductID, VariantID: row.VariantID, ProductCode: row.ProductCode,
			ProductName: row.ProductName, Uom: row.Uom, UomFactor: row.UomFactor, Qty: row.Qty, QtyBase: row.QtyBase,
			UnitPrice: row.UnitPrice, DiscountPct: row.DiscountPct, DiscountNominal: row.DiscountNominal,
			LineTotal: row.LineTotal, StockTracked: row.StockTracked, ItemType: row.ItemType,
			VariantName: row.VariantName, VariantCode: row.VariantCode, BaseUom: row.BaseUom,
			DiscountReason: row.DiscountReason, LineNote: row.LineNote, Taxable: row.Taxable,
			TaxRate: row.TaxRate, TaxBase: row.TaxBase, TaxAmount: row.TaxAmount,
		}))
	}
	return out, nil
}
