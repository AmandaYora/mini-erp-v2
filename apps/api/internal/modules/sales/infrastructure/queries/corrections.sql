-- Corrections of an order entered wrong (cancel with reason, short-close).
-- Sales-only: the purchasing twin has no counterpart (yet), so these stay out
-- of sales_orders.sql and TestSqlcSalesPurchasingParity.

-- name: GetOrderCorrections :one
SELECT cancel_reason, cancelled_at, cancelled_by,
       short_close_reason, short_closed_at, short_closed_by
FROM sales_orders
WHERE id = ?;

-- name: CancelOrderWithReason :execresult
-- CAS: only a draft or confirmed order is cancelled; the reason and who/when
-- are stamped in the same statement as the status flip.
UPDATE sales_orders
SET status = 'cancelled', cancel_reason = sqlc.narg(reason),
    cancelled_at = sqlc.arg(stamp), cancelled_by = sqlc.arg(actor_id),
    updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status IN ('draft', 'confirmed');

-- name: ShortCloseOrder :execresult
-- CAS: only a confirmed order is short-closed. The header money is rewritten
-- to the delivered lines, the order completes and the reason is stamped.
UPDATE sales_orders
SET subtotal = sqlc.arg(subtotal), discount_total = sqlc.arg(discount_total),
    tax_total = sqlc.arg(tax_total), grand_total = sqlc.arg(grand_total),
    status = 'completed', completed_at = sqlc.arg(stamp),
    short_close_reason = sqlc.arg(reason), short_closed_at = sqlc.arg(stamp),
    short_closed_by = sqlc.arg(actor_id),
    updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = 'confirmed';

-- name: ShortCloseItem :exec
-- Rewrites one line to its delivered quantity, in place (the line id stays).
UPDATE sales_order_items
SET qty = sqlc.arg(qty), qty_base = sqlc.arg(qty_base),
    discount_nominal = sqlc.arg(discount_nominal), line_total = sqlc.arg(line_total),
    tax_base = sqlc.arg(tax_base), tax_amount = sqlc.arg(tax_amount)
WHERE id = sqlc.arg(id) AND order_id = sqlc.arg(order_id);

-- name: DeleteOrderItem :exec
-- Drops a line nothing was delivered of (short-close).
DELETE FROM sales_order_items WHERE id = sqlc.arg(id) AND order_id = sqlc.arg(order_id);
