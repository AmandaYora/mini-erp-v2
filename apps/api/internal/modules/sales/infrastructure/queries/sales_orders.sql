-- Sales order header + lines (F8-18). Twin of purchasing/infrastructure/queries/
-- purchase_orders.sql: keep both in step (TestSqlcSalesPurchasingParity). The
-- dynamic list/count/export queries live in list_raw.go.

-- name: GetOrder :one
SELECT id, number, branch_id, party_id, channel, member_code,
       order_date, due_date, payment_terms, tax_type, tax_rate, subtotal,
       discount_total, tax_total, grand_total, status, notes,
       ship_to_address_id, ship_to_label, ship_to_recipient, ship_to_phone,
       ship_to_address, tax_invoice_number, tax_invoice_date, client_ref,
       created_by, assigned_user_id, credit_approved_by, credit_approved_at, created_at, origin,
       receipt_account_id
FROM sales_orders
WHERE id = ?;

-- name: GetOrderByNumber :one
SELECT id, number, branch_id, party_id, channel, member_code,
       order_date, due_date, payment_terms, tax_type, tax_rate, subtotal,
       discount_total, tax_total, grand_total, status, notes,
       ship_to_address_id, ship_to_label, ship_to_recipient, ship_to_phone,
       ship_to_address, tax_invoice_number, tax_invoice_date, client_ref,
       created_by, assigned_user_id, credit_approved_by, credit_approved_at, created_at, origin,
       receipt_account_id
FROM sales_orders
WHERE branch_id = ? AND number = ?;

-- name: GetOrderByClientRef :one
SELECT id, number, branch_id, party_id, channel, member_code,
       order_date, due_date, payment_terms, tax_type, tax_rate, subtotal,
       discount_total, tax_total, grand_total, status, notes,
       ship_to_address_id, ship_to_label, ship_to_recipient, ship_to_phone,
       ship_to_address, tax_invoice_number, tax_invoice_date, client_ref,
       created_by, assigned_user_id, credit_approved_by, credit_approved_at, created_at, origin,
       receipt_account_id
FROM sales_orders
WHERE branch_id = ? AND client_ref = ?;

-- name: ListItemsByOrder :many
SELECT id, product_id, variant_id, product_code, product_name, uom,
       uom_factor, qty, qty_base, unit_price, discount_pct, discount_nominal, line_total,
       stock_tracked, item_type, variant_name, variant_code, base_uom,
       pricing_source, base_price, discount_reason, line_note, taxable, tax_base, tax_amount
FROM sales_order_items
WHERE order_id = ?
ORDER BY id ASC;

-- name: InsertOrder :execresult
INSERT INTO sales_orders
  (number, branch_id, party_id, channel, member_code, order_date, due_date,
   payment_terms, tax_type, tax_rate, subtotal, discount_total, tax_total,
   grand_total, status, notes, ship_to_address_id, ship_to_label,
   ship_to_recipient, ship_to_phone, ship_to_address,
   tax_invoice_number, tax_invoice_date, client_ref, created_by, updated_by, assigned_user_id, origin,
   receipt_account_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateOrder :exec
UPDATE sales_orders
SET party_id = ?, channel = ?, member_code = ?,
    order_date = ?, due_date = ?, payment_terms = ?, tax_type = ?, tax_rate = ?,
    subtotal = ?, discount_total = ?, tax_total = ?, grand_total = ?, notes = ?,
    ship_to_address_id = ?, ship_to_label = ?, ship_to_recipient = ?,
    ship_to_phone = ?, ship_to_address = ?,
    tax_invoice_number = ?, tax_invoice_date = ?,
    assigned_user_id = ?, receipt_account_id = ?,
    updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ?;

-- name: DeleteItemsByOrder :exec
DELETE FROM sales_order_items WHERE order_id = ?;

-- name: InsertItem :exec
INSERT INTO sales_order_items
  (order_id, product_id, variant_id, product_code, product_name, uom,
   uom_factor, qty, qty_base, unit_price, discount_pct, discount_nominal, line_total,
   stock_tracked, item_type, variant_name, variant_code, base_uom,
   pricing_source, base_price, discount_reason, line_note, taxable, tax_base, tax_amount)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: UpdateHeaderLight :exec
UPDATE sales_orders
SET due_date = ?, notes = ?,
    ship_to_address_id = ?, ship_to_label = ?, ship_to_recipient = ?,
    ship_to_phone = ?, ship_to_address = ?,
    tax_invoice_number = ?, tax_invoice_date = ?,
    assigned_user_id = ?, receipt_account_id = ?,
    updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ?;

-- name: SetReceiptAccount :exec
-- Rekening tujuan of a bill that may no longer take header edits (completed,
-- still unpaid) — ADR-0026.
UPDATE sales_orders
SET receipt_account_id = sqlc.narg(receipt_account_id), updated_at = sqlc.arg(now), updated_by = sqlc.arg(updated_by)
WHERE id = sqlc.arg(id);

-- name: SetStatus :exec
-- completed_at is stamped (stamp = now) the moment the order becomes completed.
UPDATE sales_orders
SET status = sqlc.arg(status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id),
    completed_at = CASE WHEN sqlc.arg(status) = 'completed' THEN sqlc.arg(stamp) ELSE completed_at END
WHERE id = sqlc.arg(id);

-- name: TransitionOrder :execresult
-- CAS: moves the status only from one of the allowed origins.
UPDATE sales_orders
SET status = sqlc.arg(to_status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id),
    completed_at = CASE WHEN sqlc.arg(to_status) = 'completed' THEN sqlc.arg(stamp) ELSE completed_at END
WHERE id = sqlc.arg(id) AND status IN (sqlc.slice('from_statuses'));

-- name: SwitchToNet :execresult
-- CAS: only a confirmed cash-terms order (cod or prepaid) may switch.
-- The approval (who/when, SL-09) is stamped in the same statement: the credit
-- switch is the approval, whichever path reaches it (approve-credit endpoint or
-- the delivery settlement switch_to_net).
UPDATE sales_orders
SET payment_terms = 'net', due_date = sqlc.arg(due_date),
    credit_approved_by = sqlc.arg(actor_id), credit_approved_at = sqlc.arg(approved_at),
    updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = 'confirmed' AND payment_terms IN ('cod', 'prepaid');

-- name: SetTerms :exec
UPDATE sales_orders
SET payment_terms = ?, due_date = ?,
    updated_at = sqlc.arg(now), updated_by = ?
WHERE id = ?;

-- name: ListOrdersByParty :many
SELECT id, number, party_id, order_date, due_date, grand_total, status
FROM sales_orders
WHERE branch_id = ? AND party_id = ?
ORDER BY id DESC;

-- name: ListOrderSummariesLive :many
-- No status filter = every non-cancelled order; NULL bounds = unbounded.
SELECT id, number, party_id, order_date, due_date, grand_total, status
FROM sales_orders
WHERE branch_id = sqlc.arg(branch_id) AND status <> 'cancelled'
  AND (sqlc.narg('from_at') IS NULL OR order_date >= sqlc.narg('from_at'))
  AND (sqlc.narg('to_at') IS NULL OR order_date <= sqlc.narg('to_at'))
ORDER BY id DESC
LIMIT ?;

-- name: ListOrderSummariesByStatus :many
SELECT id, number, party_id, order_date, due_date, grand_total, status
FROM sales_orders
WHERE branch_id = sqlc.arg(branch_id) AND status IN (sqlc.slice('statuses'))
  AND (sqlc.narg('from_at') IS NULL OR order_date >= sqlc.narg('from_at'))
  AND (sqlc.narg('to_at') IS NULL OR order_date <= sqlc.narg('to_at'))
ORDER BY id DESC
LIMIT ?;

-- name: SummariesByIDs :many
SELECT id, number, party_id, order_date, due_date, grand_total, status,
       branch_id, tax_invoice_number, tax_invoice_date
FROM sales_orders
WHERE id IN (sqlc.slice('ids'));
