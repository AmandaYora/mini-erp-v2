package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"mini-erp/internal/modules/sales/contracts"
)

// Repository owns the sales module tables.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func toFloat(s string) float64 {
	f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return f
}

const orderColumns = `id, number, branch_id, party_id, channel, member_code,
	order_date, due_date, payment_terms, tax_type, tax_rate, subtotal,
	discount_total, tax_total, grand_total, status, notes,
	ship_to_address_id, ship_to_label, ship_to_recipient, ship_to_phone,
	ship_to_address, tax_invoice_number, tax_invoice_date`

func scanOrder(row interface {
	Scan(dest ...any) error
}, o *contracts.SalesOrder) error {
	var orderDate sql.NullTime
	var due, taxInvDate sql.NullTime
	var member, notes sql.NullString
	var rate string
	var shipID sql.NullInt64
	var shipLabel, shipRecipient, shipPhone, shipAddr, taxInvNo sql.NullString
	err := row.Scan(&o.ID, &o.Number, &o.BranchID, &o.PartyID, &o.Channel, &member,
		&orderDate, &due, &o.PaymentTerms, &o.TaxType, &rate, &o.Subtotal,
		&o.DiscountTotal, &o.TaxTotal, &o.GrandTotal, &o.Status, &notes,
		&shipID, &shipLabel, &shipRecipient, &shipPhone, &shipAddr,
		&taxInvNo, &taxInvDate)
	if orderDate.Valid {
		o.OrderDate = orderDate.Time.Format("2006-01-02 15:04:05")
	}
	o.DueDate, o.MemberCode, o.Notes = formatNullTime(due), member.String, notes.String
	o.TaxRate = toFloat(rate)
	o.ShipToAddressID = shipID.Int64
	o.ShipToLabel, o.ShipToRecipient = shipLabel.String, shipRecipient.String
	o.ShipToPhone, o.ShipToAddress = shipPhone.String, shipAddr.String
	o.TaxInvoiceNumber = taxInvNo.String
	o.TaxInvoiceDate = formatNullTime(taxInvDate)
	return err
}

func formatNullTime(t sql.NullTime) string {
	if !t.Valid {
		return ""
	}
	return t.Time.Format("2006-01-02 15:04:05")
}

// GetOrder returns nil, nil when missing.
func (r *Repository) GetOrder(ctx context.Context, id int64) (*contracts.SalesOrder, error) {
	o := &contracts.SalesOrder{}
	err := scanOrder(r.db.QueryRowContext(ctx,
		"SELECT "+orderColumns+" FROM sales_orders WHERE id = ?", id), o)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return o, err
}

// ItemsByOrder returns the document lines in row order.
func (r *Repository) ItemsByOrder(ctx context.Context, orderID int64) ([]*contracts.Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, variant_id, product_code, product_name, uom,
			uom_factor, qty, qty_base, unit_price, discount_pct, discount_nominal, line_total
		 FROM sales_order_items WHERE order_id = ? ORDER BY id ASC`, orderID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Item
	for rows.Next() {
		it := &contracts.Item{}
		var factor, qty, qtyBase, pct string
		if err := rows.Scan(&it.ID, &it.ProductID, &it.VariantID, &it.ProductCode,
			&it.ProductName, &it.UOM, &factor, &qty, &qtyBase, &it.UnitPrice,
			&pct, &it.DiscountNominal, &it.LineTotal); err != nil {
			return nil, err
		}
		it.UOMFactor = toFloat(factor)
		it.Qty = toFloat(qty)
		it.QtyBase = toFloat(qtyBase)
		it.DiscountPct = toFloat(pct)
		out = append(out, it)
	}
	return out, rows.Err()
}

// CreateOrder inserts header + lines atomically.
func (r *Repository) CreateOrder(ctx context.Context, o *contracts.SalesOrder, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO sales_orders
		 (number, branch_id, party_id, channel, member_code, order_date, due_date,
		  payment_terms, tax_type, tax_rate, subtotal, discount_total, tax_total,
		  grand_total, status, notes, ship_to_address_id, ship_to_label,
		  ship_to_recipient, ship_to_phone, ship_to_address,
		  tax_invoice_number, tax_invoice_date, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.Number, o.BranchID, o.PartyID, o.Channel, nullIfEmpty(o.MemberCode),
		o.OrderDate, nullIfEmpty(o.DueDate), o.PaymentTerms, o.TaxType, o.TaxRate,
		o.Subtotal, o.DiscountTotal, o.TaxTotal, o.GrandTotal,
		nullIfEmpty(o.Notes),
		nullIfZero(o.ShipToAddressID), nullIfEmpty(o.ShipToLabel),
		nullIfEmpty(o.ShipToRecipient), nullIfEmpty(o.ShipToPhone),
		nullIfEmpty(o.ShipToAddress), nullIfEmpty(o.TaxInvoiceNumber),
		nullIfEmpty(o.TaxInvoiceDate), actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := replaceItems(ctx, tx, id, o.Items); err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

// UpdateOrder rewrites a draft header + lines atomically.
func (r *Repository) UpdateOrder(ctx context.Context, o *contracts.SalesOrder, actorID int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx,
		`UPDATE sales_orders SET party_id = ?, channel = ?, member_code = ?,
		 order_date = ?, due_date = ?, payment_terms = ?, tax_type = ?, tax_rate = ?,
		 subtotal = ?, discount_total = ?, tax_total = ?, grand_total = ?, notes = ?,
		 ship_to_address_id = ?, ship_to_label = ?, ship_to_recipient = ?,
		 ship_to_phone = ?, ship_to_address = ?,
		 tax_invoice_number = ?, tax_invoice_date = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		o.PartyID, o.Channel, nullIfEmpty(o.MemberCode), o.OrderDate, nullIfEmpty(o.DueDate),
		o.PaymentTerms, o.TaxType, o.TaxRate, o.Subtotal, o.DiscountTotal,
		o.TaxTotal, o.GrandTotal, nullIfEmpty(o.Notes),
		nullIfZero(o.ShipToAddressID), nullIfEmpty(o.ShipToLabel),
		nullIfEmpty(o.ShipToRecipient), nullIfEmpty(o.ShipToPhone),
		nullIfEmpty(o.ShipToAddress), nullIfEmpty(o.TaxInvoiceNumber),
		nullIfEmpty(o.TaxInvoiceDate), actorID, o.ID); err != nil {
		return err
	}
	if err := replaceItems(ctx, tx, o.ID, o.Items); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceItems(ctx context.Context, tx *sql.Tx, orderID int64, items []*contracts.Item) error {
	if _, err := tx.ExecContext(ctx, "DELETE FROM sales_order_items WHERE order_id = ?", orderID); err != nil {
		return err
	}
	for _, it := range items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sales_order_items
			 (order_id, product_id, variant_id, product_code, product_name, uom,
			  uom_factor, qty, qty_base, unit_price, discount_pct, discount_nominal, line_total)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			orderID, it.ProductID, it.VariantID, it.ProductCode, it.ProductName,
			it.UOM, it.UOMFactor, it.Qty, it.QtyBase, it.UnitPrice,
			it.DiscountPct, it.DiscountNominal, it.LineTotal); err != nil {
			return err
		}
	}
	return nil
}

// SetStatus moves the document lifecycle.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sales_orders SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, id)
	return err
}

// SetTerms switches payment terms + due date (credit approval path).
func (r *Repository) SetTerms(ctx context.Context, id int64, terms, dueDate string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sales_orders SET payment_terms = ?, due_date = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		terms, dueDate, actorID, id)
	return err
}

// List searches numbers with optional status/channel filters, newest first.
func (r *Repository) List(ctx context.Context, branchID int64, search, status, channel string, limit, offset int) ([]*contracts.SalesOrder, error) {
	conds := []string{"branch_id = ?"}
	args := []any{branchID}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	if channel != "" {
		conds = append(conds, "channel = ?")
		args = append(args, channel)
	}
	for _, word := range strings.Fields(search) {
		conds = append(conds, "number LIKE ?")
		args = append(args, "%"+word+"%")
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+orderColumns+" FROM sales_orders WHERE "+strings.Join(conds, " AND ")+
			" ORDER BY id DESC LIMIT ? OFFSET ?", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.SalesOrder
	for rows.Next() {
		o := &contracts.SalesOrder{}
		if err := scanOrder(rows, o); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, branchID int64, search, status, channel string) (int64, error) {
	conds := []string{"branch_id = ?"}
	args := []any{branchID}
	if status != "" {
		conds = append(conds, "status = ?")
		args = append(args, status)
	}
	if channel != "" {
		conds = append(conds, "channel = ?")
		args = append(args, channel)
	}
	for _, word := range strings.Fields(search) {
		conds = append(conds, "number LIKE ?")
		args = append(args, "%"+word+"%")
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sales_orders WHERE "+strings.Join(conds, " AND "), args...).Scan(&total)
	return total, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// ListByParty lists a party's documents in a branch, newest first.
func (r *Repository) ListByParty(ctx context.Context, branchID, partyID int64) ([]*contracts.OrderSummary, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, party_id, order_date, due_date, grand_total, status FROM sales_orders
		 WHERE branch_id = ? AND party_id = ? ORDER BY id DESC`, branchID, partyID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.OrderSummary
	for rows.Next() {
		var s contracts.OrderSummary
		var orderDate, due sql.NullTime
		if err := rows.Scan(&s.ID, &s.Number, &s.PartyID, &orderDate, &due, &s.GrandTotal, &s.Status); err != nil {
			return nil, err
		}
		s.OrderDate, s.DueDate = formatNullTime(orderDate), formatNullTime(due)
		out = append(out, &s)
	}
	return out, rows.Err()
}

// GetOrderByNumber returns nil, nil when missing (branch-scoped).
func (r *Repository) GetOrderByNumber(ctx context.Context, branchID int64, number string) (*contracts.SalesOrder, error) {
	o := &contracts.SalesOrder{}
	err := scanOrder(r.db.QueryRowContext(ctx,
		"SELECT "+orderColumns+" FROM sales_orders WHERE branch_id = ? AND number = ?", branchID, number), o)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return o, err
}

// ListSummaries lists lightweight rows for operational readers (assistant
// bot). Empty statuses = all non-cancelled; empty from/to = unbounded
// (order_date range, datetimes normalized here like ListEntries).
func (r *Repository) ListSummaries(ctx context.Context, branchID int64, statuses []string, from, to string, limit int) ([]*contracts.OrderSummary, error) {
	conds := []string{"branch_id = ?"}
	args := []any{branchID}
	if len(statuses) > 0 {
		placeholders := strings.Repeat("?,", len(statuses)-1) + "?"
		conds = append(conds, "status IN ("+placeholders+")")
		for _, s := range statuses {
			args = append(args, s)
		}
	} else {
		conds = append(conds, "status <> 'cancelled'")
	}
	if strings.TrimSpace(from) != "" {
		conds = append(conds, "order_date >= ?")
		args = append(args, strings.TrimSpace(from)+" 00:00:00")
	}
	if strings.TrimSpace(to) != "" {
		conds = append(conds, "order_date <= ?")
		args = append(args, strings.TrimSpace(to)+" 23:59:59")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	args = append(args, limit)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, party_id, order_date, due_date, grand_total, status
		 FROM sales_orders WHERE `+strings.Join(conds, " AND ")+` ORDER BY id DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*contracts.OrderSummary{}
	for rows.Next() {
		var s contracts.OrderSummary
		var orderDate, due sql.NullTime
		if err := rows.Scan(&s.ID, &s.Number, &s.PartyID, &orderDate, &due, &s.GrandTotal, &s.Status); err != nil {
			return nil, err
		}
		s.OrderDate, s.DueDate = formatNullTime(orderDate), formatNullTime(due)
		out = append(out, &s)
	}
	return out, rows.Err()
}
