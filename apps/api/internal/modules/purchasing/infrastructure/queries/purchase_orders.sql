-- Purchase order header + lines (F8-18). The dynamic list/count/export queries
-- (variable-arity search words, whitelisted ORDER BY) live in list_raw.go.

-- name: GetOrder :one
SELECT id, number, branch_id, party_id, order_date, due_date,
       payment_terms, tax_type, tax_rate, subtotal, discount_total, tax_total,
       grand_total, status, notes, supplier_invoice_number, supplier_invoice_date, payment_term_days,
       tax_invoice_number, tax_invoice_date, assigned_user_id, created_by,
       cancel_reason, cancelled_at, cancelled_by, short_close_reason, short_closed_at, short_closed_by
FROM purchase_orders
WHERE id = ?;

-- name: GetOrderByNumber :one
SELECT id, number, branch_id, party_id, order_date, due_date,
       payment_terms, tax_type, tax_rate, subtotal, discount_total, tax_total,
       grand_total, status, notes, supplier_invoice_number, supplier_invoice_date, payment_term_days,
       tax_invoice_number, tax_invoice_date, assigned_user_id, created_by,
       cancel_reason, cancelled_at, cancelled_by, short_close_reason, short_closed_at, short_closed_by
FROM purchase_orders
WHERE branch_id = ? AND number = ?;

-- name: ListItemsByOrder :many
SELECT id, product_id, variant_id, product_code, product_name, uom,
       uom_factor, qty, qty_base, unit_price, discount_pct, discount_nominal, line_total,
       stock_tracked, item_type, variant_name, variant_code, base_uom,
       discount_reason, line_note, taxable, tax_rate, tax_base, tax_amount
FROM purchase_order_items
WHERE order_id = ?
ORDER BY id ASC;

-- name: InsertOrder :execresult
INSERT INTO purchase_orders
  (number, branch_id, party_id, order_date, due_date, payment_terms,
   tax_type, tax_rate, subtotal, discount_total, tax_total, grand_total,
   status, notes, supplier_invoice_number, supplier_invoice_date,
   tax_invoice_number, tax_invoice_date, assigned_user_id,
   payment_term_days, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateOrder :exec
UPDATE purchase_orders
SET party_id = ?, order_date = ?, due_date = ?,
    payment_terms = ?, tax_type = ?, tax_rate = ?, subtotal = ?,
    discount_total = ?, tax_total = ?, grand_total = ?, notes = ?,
    supplier_invoice_number = ?, supplier_invoice_date = ?, payment_term_days = ?,
    tax_invoice_number = ?, tax_invoice_date = ?, assigned_user_id = ?,
    updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ?;

-- name: DeleteItemsByOrder :exec
DELETE FROM purchase_order_items WHERE order_id = ?;

-- name: InsertItem :exec
INSERT INTO purchase_order_items
  (order_id, product_id, variant_id, product_code, product_name, uom,
   uom_factor, qty, qty_base, unit_price, discount_pct, discount_nominal, line_total,
   stock_tracked, item_type, variant_name, variant_code, base_uom,
   discount_reason, line_note, taxable, tax_rate, tax_base, tax_amount)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: TransitionOrder :execresult
-- CAS: moves the status only from one of the allowed origins.
UPDATE purchase_orders
SET status = sqlc.arg(to_status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status IN (sqlc.slice('from_statuses'));

-- name: UpdateHeaderLight :exec
UPDATE purchase_orders
SET due_date = ?, notes = ?,
    supplier_invoice_number = ?, supplier_invoice_date = ?, payment_term_days = ?,
    tax_invoice_number = ?, tax_invoice_date = ?, assigned_user_id = ?,
    updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ?;

-- name: SwitchToNet :execresult
-- CAS: only a confirmed cash-terms order may switch.
UPDATE purchase_orders
SET payment_terms = 'net', due_date = ?,
    updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ? AND status = 'confirmed' AND payment_terms IN ('cod', 'prepaid');

-- name: SetStatus :exec
UPDATE purchase_orders
SET status = ?, updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ?;

-- name: ListOrdersByParty :many
SELECT id, number, party_id, order_date, due_date, grand_total, status
FROM purchase_orders
WHERE branch_id = ? AND party_id = ?
ORDER BY id DESC;

-- name: ListOrderSummariesLive :many
-- No status filter = every non-cancelled order; NULL bounds = unbounded.
SELECT id, number, party_id, order_date, due_date, grand_total, status
FROM purchase_orders
WHERE branch_id = sqlc.arg(branch_id) AND status <> 'cancelled'
  AND (sqlc.narg('from_at') IS NULL OR order_date >= sqlc.narg('from_at'))
  AND (sqlc.narg('to_at') IS NULL OR order_date <= sqlc.narg('to_at'))
ORDER BY id DESC
LIMIT ?;

-- name: ListOrderSummariesByStatus :many
SELECT id, number, party_id, order_date, due_date, grand_total, status
FROM purchase_orders
WHERE branch_id = sqlc.arg(branch_id) AND status IN (sqlc.slice('statuses'))
  AND (sqlc.narg('from_at') IS NULL OR order_date >= sqlc.narg('from_at'))
  AND (sqlc.narg('to_at') IS NULL OR order_date <= sqlc.narg('to_at'))
ORDER BY id DESC
LIMIT ?;

-- name: SummariesByIDs :many
SELECT id, number, party_id, order_date, due_date, grand_total, status,
       branch_id, supplier_invoice_number, supplier_invoice_date
FROM purchase_orders
WHERE id IN (sqlc.slice('ids'));

-- name: ListItemsByOrders :many
-- Lines of a whole export page in one read (PO-02 "Detail Item" sheet).
SELECT order_id, id, product_id, variant_id, product_code, product_name, uom,
       uom_factor, qty, qty_base, unit_price, discount_pct, discount_nominal, line_total,
       stock_tracked, item_type, variant_name, variant_code, base_uom,
       discount_reason, line_note, taxable, tax_rate, tax_base, tax_amount
FROM purchase_order_items
WHERE order_id IN (sqlc.slice('order_ids'))
ORDER BY order_id ASC, id ASC;
