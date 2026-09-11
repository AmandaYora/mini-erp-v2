package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"mini-erp/internal/modules/salesreturn/contracts"
)

// Repository owns the salesreturn module tables.
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

// GetReturn returns nil, nil when missing.
func (r *Repository) GetReturn(ctx context.Context, id int64) (*contracts.SalesReturn, error) {
	ret := &contracts.SalesReturn{}
	var returned sql.NullTime
	var notes sql.NullString
	var rate string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, branch_id, sales_order_id, return_date, subtotal,
			discount_total, tax_total, tax_type, tax_rate, total, status, notes,
			return_mode, replacement_delivery_status
		 FROM sales_returns WHERE id = ?`, id).
		Scan(&ret.ID, &ret.Number, &ret.BranchID, &ret.SalesOrderID,
			&returned, &ret.Subtotal, &ret.DiscountTotal, &ret.TaxTotal,
			&ret.TaxType, &rate, &ret.Total, &ret.Status, &notes,
			&ret.ReturnMode, &ret.ReplacementDeliveryStatus)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if returned.Valid {
		ret.ReturnDate = returned.Time.Format("2006-01-02 15:04:05")
	}
	ret.TaxRate = toFloat(rate)
	ret.Notes = notes.String
	return ret, nil
}

// ItemsByReturn returns the return lines in row order.
func (r *Repository) ItemsByReturn(ctx context.Context, returnID int64) ([]*contracts.Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, variant_id, location_id, uom, uom_factor,
			qty, qty_base, unit_price, discount_pct, discount_nominal,
			tax_base, tax_amount, line_total
		 FROM sales_return_items WHERE return_id = ? ORDER BY id ASC`, returnID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Item
	for rows.Next() {
		it := &contracts.Item{}
		var factor, qty, qtyBase, pct string
		if err := rows.Scan(&it.ID, &it.ProductID, &it.VariantID, &it.LocationID,
			&it.UOM, &factor, &qty, &qtyBase, &it.UnitPrice, &pct,
			&it.DiscountNominal, &it.TaxBase, &it.TaxAmount, &it.LineTotal); err != nil {
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

// ReturnedQty sums confirmed-returned base qty for one SO line position.
// Only confirmed returns count — drafts reserve nothing.
func (r *Repository) ReturnedQty(ctx context.Context, soID, productID, variantID int64) (float64, error) {
	var total sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(i.qty_base), 0) FROM sales_return_items i
		 JOIN sales_returns s ON s.id = i.return_id
		 WHERE s.sales_order_id = ? AND s.status = 'confirmed'
		   AND i.product_id = ? AND i.variant_id = ?`,
		soID, productID, variantID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return toFloat(total.String), nil
}

// CreateReturn inserts a draft return with lines + replacement lines,
// returning the id. All inserts share one tx.
func (r *Repository) CreateReturn(ctx context.Context, ret *contracts.SalesReturn, replacements []*contracts.ReplacementItem, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO sales_returns
		 (number, branch_id, sales_order_id, return_date, subtotal, discount_total,
		  tax_total, tax_type, tax_rate, total, status, notes,
		  return_mode, replacement_delivery_status, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?)`,
		ret.Number, ret.BranchID, ret.SalesOrderID, ret.ReturnDate,
		ret.Subtotal, ret.DiscountTotal, ret.TaxTotal, ret.TaxType, ret.TaxRate,
		ret.Total, nullIfEmpty(ret.Notes),
		returnModeOrDefault(ret.ReturnMode), replacementStatusOrDefault(ret.ReplacementDeliveryStatus),
		actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range ret.Items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sales_return_items
			 (return_id, product_id, variant_id, location_id, uom, uom_factor,
			  qty, qty_base, unit_price, discount_pct, discount_nominal,
			  tax_base, tax_amount, line_total)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, it.ProductID, it.VariantID, it.LocationID, it.UOM,
			it.UOMFactor, it.Qty, it.QtyBase, it.UnitPrice, it.DiscountPct,
			it.DiscountNominal, it.TaxBase, it.TaxAmount, it.LineTotal); err != nil {
			return 0, err
		}
	}
	for _, it := range replacements {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO sales_return_replacement_items
			 (return_id, product_id, variant_id, location_id, uom, uom_factor,
			  qty, qty_base, unit_price, line_total)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			id, it.ProductID, it.VariantID, it.LocationID, it.UOM,
			it.UOMFactor, it.Qty, it.QtyBase, it.UnitPrice, it.LineTotal); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// SetStatus moves the return lifecycle.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE sales_returns SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, id)
	return err
}

// SetReplacementStatus moves the exchange replacement pipeline.
func (r *Repository) SetReplacementStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE sales_returns SET replacement_delivery_status = ?,
		 updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?`,
		status, actorID, id)
	return err
}

// InsertReplacementItems stores exchange replacement lines (priced once at
// creation from the live catalog — a new sale in effect).
func (r *Repository) InsertReplacementItems(ctx context.Context, returnID int64, items []*contracts.ReplacementItem) error {
	for _, it := range items {
		if _, err := r.db.ExecContext(ctx,
			`INSERT INTO sales_return_replacement_items
			 (return_id, product_id, variant_id, location_id, uom, uom_factor,
			  qty, qty_base, unit_price, line_total)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			returnID, it.ProductID, it.VariantID, it.LocationID, it.UOM,
			it.UOMFactor, it.Qty, it.QtyBase, it.UnitPrice, it.LineTotal); err != nil {
			return err
		}
	}
	return nil
}

// ReplacementItems lists an exchange return's replacement lines.
func (r *Repository) ReplacementItems(ctx context.Context, returnID int64) ([]*contracts.ReplacementItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, variant_id, location_id, uom, uom_factor,
			qty, qty_base, unit_price, line_total
		 FROM sales_return_replacement_items WHERE return_id = ? ORDER BY id ASC`, returnID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.ReplacementItem
	for rows.Next() {
		it := &contracts.ReplacementItem{}
		var factor, qty, qtyBase string
		if err := rows.Scan(&it.ID, &it.ProductID, &it.VariantID, &it.LocationID,
			&it.UOM, &factor, &qty, &qtyBase, &it.UnitPrice, &it.LineTotal); err != nil {
			return nil, err
		}
		it.UOMFactor = toFloat(factor)
		it.Qty = toFloat(qty)
		it.QtyBase = toFloat(qtyBase)
		out = append(out, it)
	}
	return out, rows.Err()
}

// InsertSettlement records how return value was settled (memo-level).
func (r *Repository) InsertSettlement(ctx context.Context, returnID int64, s *contracts.Settlement, actorID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO sales_return_settlements
		 (return_id, settlement_type, settlement_date, amount, payment_method,
		  reference_number, notes, created_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		returnID, s.Type, s.Date, s.Amount,
		nullIfEmpty(s.PaymentMethod), nullIfEmpty(s.ReferenceNumber),
		nullIfEmpty(s.Notes), actorID)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Settlements lists a return's settlement memos, oldest first.
func (r *Repository) Settlements(ctx context.Context, returnID int64) ([]*contracts.Settlement, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, settlement_type, settlement_date, amount, payment_method,
			reference_number, notes
		 FROM sales_return_settlements WHERE return_id = ? ORDER BY id ASC`, returnID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Settlement
	for rows.Next() {
		var s contracts.Settlement
		var date sql.NullTime
		var method, ref, notes sql.NullString
		if err := rows.Scan(&s.ID, &s.Type, &date, &s.Amount, &method, &ref, &notes); err != nil {
			return nil, err
		}
		if date.Valid {
			s.Date = date.Time.Format("2006-01-02 15:04:05")
		}
		s.PaymentMethod, s.ReferenceNumber, s.Notes = method.String, ref.String, notes.String
		out = append(out, &s)
	}
	return out, rows.Err()
}

func returnModeOrDefault(m string) string {
	if m == contracts.ReturnModeExchange {
		return contracts.ReturnModeExchange
	}
	return contracts.ReturnModeReturnOnly
}

func replacementStatusOrDefault(s string) string {
	switch s {
	case contracts.ReplacementPending, contracts.ReplacementDispatched, contracts.ReplacementConfirmed:
		return s
	default:
		return contracts.ReplacementNotRequired
	}
}

// List searches returns of a branch, newest first.
func (r *Repository) List(ctx context.Context, branchID, soID int64, status string, limit, offset int) ([]*contracts.SalesReturn, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if soID != 0 {
		conds += " AND sales_order_id = ?"
		args = append(args, soID)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, branch_id, sales_order_id, return_date, subtotal,
			discount_total, tax_total, tax_type, tax_rate, total, status, notes,
			return_mode, replacement_delivery_status
		 FROM sales_returns WHERE `+conds+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.SalesReturn
	for rows.Next() {
		ret := &contracts.SalesReturn{}
		var returned sql.NullTime
		var notes sql.NullString
		var rate string
		if err := rows.Scan(&ret.ID, &ret.Number, &ret.BranchID, &ret.SalesOrderID,
			&returned, &ret.Subtotal, &ret.DiscountTotal, &ret.TaxTotal,
			&ret.TaxType, &rate, &ret.Total, &ret.Status, &notes,
			&ret.ReturnMode, &ret.ReplacementDeliveryStatus); err != nil {
			return nil, err
		}
		if returned.Valid {
			ret.ReturnDate = returned.Time.Format("2006-01-02 15:04:05")
		}
		ret.TaxRate = toFloat(rate)
		ret.Notes = notes.String
		out = append(out, ret)
	}
	return out, rows.Err()
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, branchID, soID int64, status string) (int64, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if soID != 0 {
		conds += " AND sales_order_id = ?"
		args = append(args, soID)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sales_returns WHERE "+conds, args...).Scan(&total)
	return total, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// OrderIDsByReturns maps return id -> originating sales order id in one
// grouped read. Unknown ids are simply absent from the result.
func (r *Repository) OrderIDsByReturns(ctx context.Context, ids []int64) (map[int64]int64, error) {
	out := map[int64]int64{}
	seen := make(map[int64]bool, len(ids))
	args := make([]any, 0, len(ids))
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
	placeholders := strings.Repeat("?,", len(args)-1) + "?"
	rows, err := r.db.QueryContext(ctx,
		"SELECT id, sales_order_id FROM sales_returns WHERE id IN ("+placeholders+")", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id, orderID int64
		if err := rows.Scan(&id, &orderID); err != nil {
			return nil, err
		}
		out[id] = orderID
	}
	return out, rows.Err()
}
