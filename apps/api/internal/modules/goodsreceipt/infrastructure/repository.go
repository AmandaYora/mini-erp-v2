package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"mini-erp/internal/modules/goodsreceipt/contracts"
	"mini-erp/internal/modules/goodsreceipt/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
)

// Repository owns the goodsreceipt module tables. Data access goes through
// the sqlc-generated Queries (ADR-0012).
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// PurchaseOrderIDsByReceipts maps receipt id -> purchase order id in ONE read.
// Unknown ids are absent.
func (r *Repository) PurchaseOrderIDsByReceipts(ctx context.Context, ids []int64) (map[int64]int64, error) {
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
	rows, err := r.q.PurchaseOrderIDsByReceipts(ctx, args)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.PurchaseOrderID
	}
	return out, nil
}

// GetReceipt returns nil, nil when missing.
func (r *Repository) GetReceipt(ctx context.Context, id int64) (*contracts.GoodsReceipt, error) {
	row, err := r.q.GetReceipt(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toReceipt(row), nil
}

// ReceiptByClientRef returns the branch's receipt created with that client
// reference, or nil.
func (r *Repository) ReceiptByClientRef(ctx context.Context, branchID int64, clientRef string) (*contracts.GoodsReceipt, error) {
	row, err := r.q.GetReceiptByClientRef(ctx, sqlc.GetReceiptByClientRefParams{
		BranchID: branchID, ClientRef: dbutil.NullString(clientRef),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toReceipt(sqlc.GetReceiptRow(row)), nil
}

// ItemsByReceipt returns the receipt lines in row order.
func (r *Repository) ItemsByReceipt(ctx context.Context, receiptID int64) ([]*contracts.Item, error) {
	rows, err := r.q.ListItemsByReceipt(ctx, receiptID)
	if err != nil {
		return nil, err
	}
	var out []*contracts.Item
	for _, row := range rows {
		out = append(out, toItem(row))
	}
	return out, nil
}

// ReceivedQty sums POSTED received base qty for one PO line position. A
// pending receipt has not moved stock yet, so it is not "received".
func (r *Repository) ReceivedQty(ctx context.Context, poID, productID, variantID int64) (float64, error) {
	return r.q.ReceivedQty(ctx, sqlc.ReceivedQtyParams{
		PurchaseOrderID: poID, ProductID: productID, VariantID: variantID,
	})
}

// PostedAfter pages posted receipts by ascending id after a cursor.
func (r *Repository) PostedAfter(ctx context.Context, branchID, afterID int64, limit int) ([]*contracts.GoodsReceipt, error) {
	rows, err := r.q.ListPostedAfter(ctx, sqlc.ListPostedAfterParams{
		BranchID: branchID, ID: afterID, Limit: int32(limit),
	})
	if err != nil {
		return nil, err
	}
	var out []*contracts.GoodsReceipt
	for _, row := range rows {
		out = append(out, toReceipt(sqlc.GetReceiptRow(row)))
	}
	return out, nil
}

// ActiveReceiptCount counts the order's live (pending + posted) receipts.
func (r *Repository) ActiveReceiptCount(ctx context.Context, poID int64) (int64, error) {
	return r.q.ActiveReceiptCount(ctx, poID)
}

// QtyByPO sums base qty per "product/variant" for the PO's receipts in the
// given statuses — one GROUP BY, never one query per line.
func (r *Repository) QtyByPO(ctx context.Context, poID int64, statuses ...string) (map[string]float64, error) {
	if len(statuses) == 0 {
		return map[string]float64{}, nil
	}
	rows, err := r.q.QtyByPO(ctx, sqlc.QtyByPOParams{PoID: poID, Statuses: statuses})
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, row := range rows {
		out[strconv.FormatInt(row.ProductID, 10)+"/"+strconv.FormatInt(row.VariantID, 10)] = row.Total
	}
	return out, nil
}

// CreateReceipt inserts a PENDING header + lines atomically, returning the id.
func (r *Repository) CreateReceipt(ctx context.Context, rec *contracts.GoodsReceipt, actorID int64) (int64, error) {
	receivedAt, err := parseReceivedAt(rec.ReceivedAt)
	if err != nil {
		return 0, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.q.WithTx(tx)
	res, err := q.InsertReceipt(ctx, sqlc.InsertReceiptParams{
		Number:                 dbutil.NullString(rec.Number),
		BranchID:               rec.BranchID,
		PurchaseOrderID:        rec.PurchaseOrderID,
		ReceivedAt:             receivedAt,
		Notes:                  dbutil.NullString(rec.Notes),
		SupplierDeliveryNumber: dbutil.NullString(rec.SupplierDeliveryNumber),
		ClientRef:              dbutil.NullString(rec.ClientRef),
		CreatedBy:              actor(actorID),
	})
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	for _, it := range rec.Items {
		if err := q.InsertReceiptItem(ctx, sqlc.InsertReceiptItemParams{
			ReceiptID: id, ProductID: it.ProductID, VariantID: it.VariantID,
			LocationID: it.LocationID, Uom: it.UOM,
			UomFactor: it.UOMFactor, Qty: it.Qty, QtyBase: it.QtyBase,
		}); err != nil {
			return 0, err
		}
	}
	return id, tx.Commit()
}

// Transition moves a receipt between statuses only from an allowed origin
// (CAS). Returns false when another writer already moved it.
func (r *Repository) Transition(ctx context.Context, id int64, from []string, to string) (bool, error) {
	if len(from) == 0 {
		return false, nil
	}
	res, err := r.q.TransitionReceipt(ctx, sqlc.TransitionReceiptParams{
		ToStatus: to, ID: id, FromStatuses: from,
	})
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	return affected == 1, err
}

// ListFilter selects receipts for List.
type ListFilter struct {
	BranchID   int64
	POID       int64
	PostedOnly bool
}

func (f ListFilter) status() sql.NullString {
	if f.PostedOnly {
		return dbutil.NullString(contracts.StatusPosted)
	}
	return sql.NullString{}
}

// List searches receipts of a branch by PO, newest first.
func (r *Repository) List(ctx context.Context, f ListFilter, limit, offset int) ([]*contracts.GoodsReceipt, error) {
	rows, err := r.q.ListReceipts(ctx, sqlc.ListReceiptsParams{
		BranchID: f.BranchID,
		PoID:     dbutil.NullInt64(f.POID),
		Status:   f.status(),
		Limit:    int32(limit), Offset: int32(offset),
	})
	if err != nil {
		return nil, err
	}
	var out []*contracts.GoodsReceipt
	for _, row := range rows {
		out = append(out, toReceipt(sqlc.GetReceiptRow(row)))
	}
	return out, nil
}

// Count counts the List filter set.
func (r *Repository) Count(ctx context.Context, f ListFilter) (int64, error) {
	return r.q.CountReceipts(ctx, sqlc.CountReceiptsParams{
		BranchID: f.BranchID,
		PoID:     dbutil.NullInt64(f.POID),
		Status:   f.status(),
	})
}

// PostedQtyBefore sums line-UOM qty per "product/variant/uom" over the order's
// posted receipts with an id below receiptID.
func (r *Repository) PostedQtyBefore(ctx context.Context, poID, receiptID int64) (map[string]float64, error) {
	rows, err := r.q.PostedQtyBefore(ctx, sqlc.PostedQtyBeforeParams{
		PurchaseOrderID: poID, ID: receiptID,
	})
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, row := range rows {
		// Rows that differ only by uom padding share one trimmed key, so
		// their quantities accumulate (never overwrite each other: the
		// GROUP BY order is unspecified and trimming must not drop units).
		key := strconv.FormatInt(row.ProductID, 10) + "/" + strconv.FormatInt(row.VariantID, 10) + "/" + strings.TrimSpace(row.Uom)
		out[key] += row.Total
	}
	return out, nil
}

// LineCounts maps receipt id -> number of lines in one grouped read (GR-01).
func (r *Repository) LineCounts(ctx context.Context, ids []int64) (map[int64]int, error) {
	out := map[int64]int{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.q.CountItemsByReceipts(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ReceiptID] = int(row.LineCount)
	}
	return out, nil
}

// ReceivedQtyByOrders sums POSTED base qty per order and "product/variant".
func (r *Repository) ReceivedQtyByOrders(ctx context.Context, poIDs []int64) (map[int64]map[string]float64, error) {
	out := map[int64]map[string]float64{}
	if len(poIDs) == 0 {
		return out, nil
	}
	rows, err := r.q.ReceivedQtyByOrders(ctx, poIDs)
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

// PostedHints returns the latest posted receipt location per position and the
// earliest posted receive date of the order.
func (r *Repository) PostedHints(ctx context.Context, poID int64) (*contracts.ReceiptHints, error) {
	rows, err := r.q.ListPostedItemLocations(ctx, poID)
	if err != nil {
		return nil, err
	}
	h := &contracts.ReceiptHints{Locations: map[string]int64{}}
	for _, row := range rows { // newest line first: the first hit per position wins
		key := strconv.FormatInt(row.ProductID, 10) + "/" + strconv.FormatInt(row.VariantID, 10)
		if _, ok := h.Locations[key]; !ok && row.LocationID > 0 {
			h.Locations[key] = row.LocationID
		}
	}
	first, err := r.q.FirstPostedReceivedAt(ctx, poID)
	if errors.Is(err, sql.ErrNoRows) {
		return h, nil
	}
	if err != nil {
		return nil, err
	}
	h.FirstReceivedAt = first.Format(wireDateTime)
	return h, nil
}
