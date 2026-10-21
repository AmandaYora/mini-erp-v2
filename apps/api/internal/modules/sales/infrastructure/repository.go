package infrastructure

import (
	"context"
	"database/sql"
	"errors"

	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/modules/sales/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// Repository owns the sales module tables. Data access goes through the
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

// OrderByClientRef finds a branch's order by client reference, nil when none.
func (r *Repository) OrderByClientRef(ctx context.Context, branchID int64, ref string) (*contracts.SalesOrder, error) {
	row, err := r.q.GetOrderByClientRef(ctx, sqlc.GetOrderByClientRefParams{BranchID: branchID, ClientRef: dbutil.NullString(ref)})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toOrder(sqlc.GetOrderRow(row)), nil
}

// GetOrder returns nil, nil when missing.
func (r *Repository) GetOrder(ctx context.Context, id int64) (*contracts.SalesOrder, error) {
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
func (r *Repository) CreateOrder(ctx context.Context, o *contracts.SalesOrder, actorID int64) (int64, error) {
	h, err := toHeaderParams(o, true)
	if err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	origin := o.Origin
	if origin == "" {
		origin = contracts.OriginStandard
	}
	q := r.q.WithTx(tx)
	res, err := q.InsertOrder(ctx, sqlc.InsertOrderParams{
		Number: o.Number, BranchID: o.BranchID, PartyID: o.PartyID, Channel: o.Channel, Origin: origin,
		MemberCode: dbutil.NullString(o.MemberCode), OrderDate: h.orderDate, DueDate: h.dueDate,
		PaymentTerms: o.PaymentTerms, TaxType: o.TaxType, TaxRate: o.TaxRate, Subtotal: o.Subtotal,
		DiscountTotal: o.DiscountTotal, TaxTotal: o.TaxTotal, GrandTotal: o.GrandTotal,
		Notes:           dbutil.NullString(o.Notes),
		ShipToAddressID: dbutil.NullInt64(o.ShipToAddressID), ShipToLabel: dbutil.NullString(o.ShipToLabel),
		ShipToRecipient: dbutil.NullString(o.ShipToRecipient), ShipToPhone: dbutil.NullString(o.ShipToPhone),
		ShipToAddress:    dbutil.NullString(o.ShipToAddress),
		TaxInvoiceNumber: dbutil.NullString(o.TaxInvoiceNumber), TaxInvoiceDate: h.invDate,
		ClientRef: dbutil.NullString(o.ClientRef), CreatedBy: actor(actorID), UpdatedBy: actor(actorID),
		AssignedUserID: dbutil.NullInt64(o.AssignedUserID),
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
func (r *Repository) UpdateOrder(ctx context.Context, o *contracts.SalesOrder, actorID int64) error {
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
		PartyID: o.PartyID, Channel: o.Channel, MemberCode: dbutil.NullString(o.MemberCode),
		OrderDate: h.orderDate, DueDate: h.dueDate, PaymentTerms: o.PaymentTerms, TaxType: o.TaxType,
		TaxRate: o.TaxRate, Subtotal: o.Subtotal, DiscountTotal: o.DiscountTotal, TaxTotal: o.TaxTotal,
		GrandTotal: o.GrandTotal, Notes: dbutil.NullString(o.Notes),
		ShipToAddressID: dbutil.NullInt64(o.ShipToAddressID), ShipToLabel: dbutil.NullString(o.ShipToLabel),
		ShipToRecipient: dbutil.NullString(o.ShipToRecipient), ShipToPhone: dbutil.NullString(o.ShipToPhone),
		ShipToAddress:    dbutil.NullString(o.ShipToAddress),
		TaxInvoiceNumber: dbutil.NullString(o.TaxInvoiceNumber), TaxInvoiceDate: h.invDate,
		AssignedUserID: dbutil.NullInt64(o.AssignedUserID),
		Now:            now(), UpdatedBy: actor(actorID), ID: o.ID,
	}); err != nil {
		return err
	}
	if err := replaceItems(ctx, q, o.ID, o.Items); err != nil {
		return err
	}
	return tx.Commit()
}

// UpdateHeaderLight rewrites only the header fields that stay editable once
// documents hang off the order (notes, ship-to snapshot, due date, tax invoice).
func (r *Repository) UpdateHeaderLight(ctx context.Context, o *contracts.SalesOrder, actorID int64) error {
	h, err := toHeaderParams(o, false)
	if err != nil {
		return err
	}
	return r.q.UpdateHeaderLight(ctx, sqlc.UpdateHeaderLightParams{
		DueDate: h.dueDate, Notes: dbutil.NullString(o.Notes),
		ShipToAddressID: dbutil.NullInt64(o.ShipToAddressID), ShipToLabel: dbutil.NullString(o.ShipToLabel),
		ShipToRecipient: dbutil.NullString(o.ShipToRecipient), ShipToPhone: dbutil.NullString(o.ShipToPhone),
		ShipToAddress:    dbutil.NullString(o.ShipToAddress),
		TaxInvoiceNumber: dbutil.NullString(o.TaxInvoiceNumber), TaxInvoiceDate: h.invDate,
		AssignedUserID: dbutil.NullInt64(o.AssignedUserID),
		Now:            now(), UpdatedBy: actor(actorID), ID: o.ID,
	})
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

// SetStatus moves the document lifecycle.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	t := now()
	return r.q.SetStatus(ctx, sqlc.SetStatusParams{
		Status: status, Now: t, ActorID: actor(actorID), Stamp: sql.NullTime{Time: t, Valid: true}, ID: id,
	})
}

// Transition moves the order between statuses only from an allowed origin
// (CAS). Returns false when another writer already moved it.
func (r *Repository) Transition(ctx context.Context, id int64, from []string, to string, actorID int64) (bool, error) {
	if len(from) == 0 {
		return false, nil
	}
	t := now()
	res, err := r.q.TransitionOrder(ctx, sqlc.TransitionOrderParams{
		ToStatus: to, Now: t, ActorID: actor(actorID), Stamp: sql.NullTime{Time: t, Valid: true},
		ID: id, FromStatuses: from,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	return affected == 1, err
}

// SwitchToNet moves a confirmed cash-terms order (cod or prepaid) to net with
// a due date (CAS). Returns false when the order is no longer eligible.
func (r *Repository) SwitchToNet(ctx context.Context, id int64, dueDate string, actorID int64) (bool, error) {
	due, err := dbutil.ParseDateTime(dueDate)
	if err != nil {
		return false, err
	}
	t := now()
	res, err := r.q.SwitchToNet(ctx, sqlc.SwitchToNetParams{
		DueDate: sql.NullTime{Time: due, Valid: true}, ActorID: actor(actorID),
		ApprovedAt: sql.NullTime{Time: t, Valid: true}, Now: t, ID: id,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	return affected == 1, err
}

// SetTerms switches payment terms + due date (credit approval path).
func (r *Repository) SetTerms(ctx context.Context, id int64, terms, dueDate string, actorID int64) error {
	due, err := dbutil.ParseDateTime(dueDate)
	if err != nil {
		return err
	}
	return r.q.SetTerms(ctx, sqlc.SetTermsParams{
		PaymentTerms: terms, DueDate: sql.NullTime{Time: due, Valid: true}, Now: now(), UpdatedBy: actor(actorID), ID: id,
	})
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
func (r *Repository) GetOrderByNumber(ctx context.Context, branchID int64, number string) (*contracts.SalesOrder, error) {
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
// bot). Empty statuses = all non-cancelled; empty from/to = unbounded
// (order_date range, datetimes normalized here like ListEntries).
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
