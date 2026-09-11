package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"mini-erp/internal/modules/purchasereturn/contracts"
)

// Repository owns the purchasereturn module tables.
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
func (r *Repository) GetReturn(ctx context.Context, id int64) (*contracts.PurchaseReturn, error) {
	ret := &contracts.PurchaseReturn{}
	var returned sql.NullTime
	var notes sql.NullString
	var rate string
	err := r.db.QueryRowContext(ctx,
		`SELECT id, number, branch_id, purchase_order_id, return_date, subtotal,
			discount_total, tax_total, tax_type, tax_rate, total, status, notes
		 FROM purchase_returns WHERE id = ?`, id).
		Scan(&ret.ID, &ret.Number, &ret.BranchID, &ret.PurchaseOrderID,
			&returned, &ret.Subtotal, &ret.DiscountTotal, &ret.TaxTotal,
			&ret.TaxType, &rate, &ret.Total, &ret.Status, &notes)
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
		 FROM purchase_return_items WHERE return_id = ? ORDER BY id ASC`, returnID)
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

// ReturnedQty sums confirmed-returned base qty for one PO line position.
func (r *Repository) ReturnedQty(ctx context.Context, poID, productID, variantID int64) (float64, error) {
	var total sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(i.qty_base), 0) FROM purchase_return_items i
		 JOIN purchase_returns p ON p.id = i.return_id
		 WHERE p.purchase_order_id = ? AND p.status = 'confirmed'
		   AND i.product_id = ? AND i.variant_id = ?`,
		poID, productID, variantID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return toFloat(total.String), nil
}

// CreateReturn inserts a draft return with lines, returning the id.
func (r *Repository) CreateReturn(ctx context.Context, ret *contracts.PurchaseReturn, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO purchase_returns
		 (number, branch_id, purchase_order_id, return_date, subtotal, discount_total,
		  tax_total, tax_type, tax_rate, total, status, notes, created_by, updated_by)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?)`,
		ret.Number, ret.BranchID, ret.PurchaseOrderID, ret.ReturnDate,
		ret.Subtotal, ret.DiscountTotal, ret.TaxTotal, ret.TaxType, ret.TaxRate,
		ret.Total, nullIfEmpty(ret.Notes), actorID, actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range ret.Items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO purchase_return_items
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
	return id, tx.Commit()
}

// SetStatus moves the return lifecycle.
func (r *Repository) SetStatus(ctx context.Context, id int64, status string, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE purchase_returns SET status = ?, updated_at = UTC_TIMESTAMP(6), updated_by = ? WHERE id = ?",
		status, actorID, id)
	return err
}

// List searches returns of a branch, newest first.
func (r *Repository) List(ctx context.Context, branchID, poID int64, status string, limit, offset int) ([]*contracts.PurchaseReturn, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if poID != 0 {
		conds += " AND purchase_order_id = ?"
		args = append(args, poID)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, number, branch_id, purchase_order_id, return_date, subtotal,
			discount_total, tax_total, tax_type, tax_rate, total, status, notes
		 FROM purchase_returns WHERE `+conds+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.PurchaseReturn
	for rows.Next() {
		ret := &contracts.PurchaseReturn{}
		var returned sql.NullTime
		var notes sql.NullString
		var rate string
		if err := rows.Scan(&ret.ID, &ret.Number, &ret.BranchID, &ret.PurchaseOrderID,
			&returned, &ret.Subtotal, &ret.DiscountTotal, &ret.TaxTotal,
			&ret.TaxType, &rate, &ret.Total, &ret.Status, &notes); err != nil {
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

// InsertSettlement records how return value was settled (memo-level).
func (r *Repository) InsertSettlement(ctx context.Context, returnID int64, s *contracts.Settlement, actorID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO purchase_return_settlements
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
		 FROM purchase_return_settlements WHERE return_id = ? ORDER BY id ASC`, returnID)
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

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, branchID, poID int64, status string) (int64, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if poID != 0 {
		conds += " AND purchase_order_id = ?"
		args = append(args, poID)
	}
	if status != "" {
		conds += " AND status = ?"
		args = append(args, status)
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM purchase_returns WHERE "+conds, args...).Scan(&total)
	return total, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
