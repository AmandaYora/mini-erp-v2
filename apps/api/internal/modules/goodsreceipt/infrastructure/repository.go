package infrastructure

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

	"mini-erp/internal/modules/goodsreceipt/contracts"
)

// Repository owns the goodsreceipt module tables.
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

// GetReceipt returns nil, nil when missing.
func (r *Repository) GetReceipt(ctx context.Context, id int64) (*contracts.GoodsReceipt, error) {
	rec := &contracts.GoodsReceipt{}
	var received sql.NullTime
	var notes sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, branch_id, purchase_order_id, received_at, notes
		 FROM goods_receipts WHERE id = ?`, id).
		Scan(&rec.ID, &rec.BranchID, &rec.PurchaseOrderID, &received, &notes)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if received.Valid {
		rec.ReceivedAt = received.Time.Format("2006-01-02 15:04:05")
	}
	rec.Notes = notes.String
	return rec, nil
}

// ItemsByReceipt returns the receipt lines in row order.
func (r *Repository) ItemsByReceipt(ctx context.Context, receiptID int64) ([]*contracts.Item, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base
		 FROM goods_receipt_items WHERE receipt_id = ? ORDER BY id ASC`, receiptID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.Item
	for rows.Next() {
		it := &contracts.Item{}
		var factor, qty, qtyBase string
		if err := rows.Scan(&it.ID, &it.ProductID, &it.VariantID, &it.LocationID,
			&it.UOM, &factor, &qty, &qtyBase); err != nil {
			return nil, err
		}
		it.UOMFactor = toFloat(factor)
		it.Qty = toFloat(qty)
		it.QtyBase = toFloat(qtyBase)
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReceivedQty sums received base qty for one PO line position.
func (r *Repository) ReceivedQty(ctx context.Context, poID, productID, variantID int64) (float64, error) {
	var total sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(i.qty_base), 0) FROM goods_receipt_items i
		 JOIN goods_receipts g ON g.id = i.receipt_id
		 WHERE g.purchase_order_id = ? AND i.product_id = ? AND i.variant_id = ?`,
		poID, productID, variantID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return toFloat(total.String), nil
}

// CreateReceipt inserts header + lines atomically, returning the id.
func (r *Repository) CreateReceipt(ctx context.Context, rec *contracts.GoodsReceipt, actorID int64) (int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	res, err := tx.ExecContext(ctx,
		`INSERT INTO goods_receipts (branch_id, purchase_order_id, received_at, notes, created_by)
		 VALUES (?, ?, ?, ?, ?)`,
		rec.BranchID, rec.PurchaseOrderID, rec.ReceivedAt, nullIfEmpty(rec.Notes), actorID)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range rec.Items {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO goods_receipt_items
			 (receipt_id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			id, it.ProductID, it.VariantID, it.LocationID, it.UOM,
			it.UOMFactor, it.Qty, it.QtyBase); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// List searches receipts of a branch by PO, newest first.
func (r *Repository) List(ctx context.Context, branchID, poID int64, limit, offset int) ([]*contracts.GoodsReceipt, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if poID != 0 {
		conds += " AND purchase_order_id = ?"
		args = append(args, poID)
	}
	args = append(args, limit, offset)
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, branch_id, purchase_order_id, received_at, notes FROM goods_receipts
		 WHERE `+conds+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []*contracts.GoodsReceipt
	for rows.Next() {
		rec := &contracts.GoodsReceipt{}
		var received sql.NullTime
		var notes sql.NullString
		if err := rows.Scan(&rec.ID, &rec.BranchID, &rec.PurchaseOrderID, &received, &notes); err != nil {
			return nil, err
		}
		if received.Valid {
			rec.ReceivedAt = received.Time.Format("2006-01-02 15:04:05")
		}
		rec.Notes = notes.String
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, branchID, poID int64) (int64, error) {
	conds := "branch_id = ?"
	args := []any{branchID}
	if poID != 0 {
		conds += " AND purchase_order_id = ?"
		args = append(args, poID)
	}
	var total int64
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM goods_receipts WHERE "+conds, args...).Scan(&total)
	return total, err
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
