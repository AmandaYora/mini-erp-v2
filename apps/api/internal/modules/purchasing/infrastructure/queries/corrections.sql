-- Corrections of confirmed orders (000034): cancel with a reason, short close
-- ("tutup sisa order") and the purchase price correction after receipt.

-- name: CancelOrder :execresult
-- CAS: cancels only from draft/confirmed, stamping who, when and why.
UPDATE purchase_orders
SET status = 'cancelled', cancel_reason = sqlc.arg(reason), cancelled_at = sqlc.arg(cancelled_at),
    cancelled_by = sqlc.arg(actor_id), updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status IN ('draft', 'confirmed');

-- name: ShortCloseOrder :execresult
-- CAS: a confirmed order becomes completed with the re-priced totals.
UPDATE purchase_orders
SET status = 'completed', subtotal = ?, discount_total = ?, tax_total = ?, grand_total = ?,
    short_close_reason = sqlc.arg(reason), short_closed_at = sqlc.arg(closed_at), short_closed_by = sqlc.arg(actor_id),
    updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = 'confirmed';

-- name: RepriceOrder :execresult
-- CAS: header totals of a price correction; the status must not have moved.
UPDATE purchase_orders
SET subtotal = ?, discount_total = ?, tax_total = ?, grand_total = ?,
    updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = sqlc.arg(status) AND grand_total = sqlc.arg(old_grand_total);

-- name: UpdateItemEconomics :exec
-- One line re-priced in place (its id stays: receipts and returns key on it).
UPDATE purchase_order_items
SET qty = ?, qty_base = ?, unit_price = ?, discount_pct = ?, discount_nominal = ?,
    line_total = ?, tax_base = ?, tax_amount = ?
WHERE id = ? AND order_id = ?;

-- name: InsertCorrection :execresult
INSERT INTO purchase_order_corrections
  (branch_id, order_id, kind, correction_date, reason, old_grand_total, new_grand_total,
   value_delta, tax_delta, created_at, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertCorrectionLine :exec
INSERT INTO purchase_order_correction_lines
  (correction_id, order_item_id, product_id, variant_id, product_name, uom, stock_tracked,
   received_qty, received_qty_base, old_qty, new_qty, old_unit_price, new_unit_price,
   old_discount_pct, new_discount_pct, old_discount_nominal, new_discount_nominal,
   old_tax_base, new_tax_base, old_tax_amount, new_tax_amount, old_line_total, new_line_total,
   value_delta, tax_delta)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetCorrection :one
SELECT id, branch_id, order_id, kind, correction_date, reason, old_grand_total, new_grand_total,
       value_delta, tax_delta, created_at, created_by
FROM purchase_order_corrections
WHERE id = ?;

-- name: ListCorrectionsByOrder :many
SELECT id, branch_id, order_id, kind, correction_date, reason, old_grand_total, new_grand_total,
       value_delta, tax_delta, created_at, created_by
FROM purchase_order_corrections
WHERE order_id = ?
ORDER BY id ASC;

-- name: ListPriceCorrectionsAfter :many
-- The finance posting queue: price corrections that move the books (a
-- correction of nothing received yet has no value to journal).
SELECT id, branch_id, order_id, kind, correction_date, reason, old_grand_total, new_grand_total,
       value_delta, tax_delta, created_at, created_by
FROM purchase_order_corrections
WHERE branch_id = ? AND kind = 'price' AND id > ? AND (value_delta <> 0 OR tax_delta <> 0)
ORDER BY id ASC
LIMIT ?;

-- name: ListCorrectionLines :many
SELECT correction_id, id, order_item_id, product_id, variant_id, product_name, uom, stock_tracked,
       received_qty, received_qty_base, old_qty, new_qty, old_unit_price, new_unit_price,
       old_discount_pct, new_discount_pct, old_discount_nominal, new_discount_nominal,
       old_tax_base, new_tax_base, old_tax_amount, new_tax_amount, old_line_total, new_line_total,
       value_delta, tax_delta
FROM purchase_order_correction_lines
WHERE correction_id IN (sqlc.slice('correction_ids'))
ORDER BY correction_id ASC, id ASC;
