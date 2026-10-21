-- Sales return header + lines + replacement lines + settlement memos
-- (F8-18). Twin of purchasereturn/infrastructure/queries/purchasereturns.sql:
-- keep both in step. All queries touch only this module's tables, so the
-- per-module schema block in sqlc.yaml keeps enforcing the boundary at
-- generate time. The dynamic list/count filter lives in listfilter.go
-- (raw-sql allow-list).

-- name: GetReturn :one
SELECT id, number, branch_id, sales_order_id, return_date, subtotal,
  discount_total, tax_total, tax_type, tax_rate, total, status, notes,
  return_mode, replacement_delivery_status, reason, replacement_subtotal,
  replacement_tax_total, replacement_total, difference_amount, receivable_reduction
FROM sales_returns WHERE id = ?;

-- name: ListItemsByReturn :many
SELECT id, product_id, variant_id, location_id, uom, uom_factor,
  qty, qty_base, unit_price, discount_pct, discount_nominal,
  tax_base, tax_amount, line_total, return_condition
FROM sales_return_items WHERE return_id = ? ORDER BY id ASC;

-- name: ListReplacementItems :many
SELECT id, product_id, variant_id, location_id, uom, uom_factor,
  qty, qty_base, unit_price, tax_base, tax_amount, line_total
FROM sales_return_replacement_items WHERE return_id = ? ORDER BY id ASC;

-- name: InsertReturn :execresult
INSERT INTO sales_returns
  (number, branch_id, sales_order_id, return_date, subtotal, discount_total,
   tax_total, tax_type, tax_rate, total, status, notes,
   return_mode, replacement_delivery_status, reason, replacement_subtotal,
   replacement_tax_total, replacement_total, difference_amount, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertReturnItem :exec
INSERT INTO sales_return_items
  (return_id, product_id, variant_id, location_id, uom, uom_factor,
   qty, qty_base, unit_price, discount_pct, discount_nominal,
   tax_base, tax_amount, line_total, return_condition)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: InsertReplacementItem :exec
INSERT INTO sales_return_replacement_items
  (return_id, product_id, variant_id, location_id, uom, uom_factor,
   qty, qty_base, unit_price, tax_base, tax_amount, line_total)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ReturnedQty :one
-- Sums confirmed-returned base qty for one SO line position. Only confirmed
-- returns count — drafts reserve nothing.
SELECT CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM sales_return_items i
JOIN sales_returns s ON s.id = i.return_id
WHERE s.sales_order_id = ? AND s.status = 'confirmed'
  AND i.product_id = ? AND i.variant_id = ?;

-- name: CommittedQtyBySO :many
-- Sums base qty per product/variant over the order's draft AND confirmed
-- returns: a draft holds its share of what can still come back, so two drafts
-- can never both claim the same delivered goods.
SELECT i.product_id, i.variant_id, CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM sales_return_items i
JOIN sales_returns s ON s.id = i.return_id
WHERE s.sales_order_id = ? AND s.status IN ('draft', 'confirmed')
GROUP BY i.product_id, i.variant_id;

-- name: TransitionReturn :execresult
-- CAS: moves the status only from one of the allowed origins.
UPDATE sales_returns
SET status = sqlc.arg(to_status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status IN (sqlc.slice('from_statuses'));

-- name: SetReturnStatus :exec
UPDATE sales_returns
SET status = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: SetReplacementStatus :execresult
-- CAS for the exchange replacement pipeline: only moves from the expected status.
UPDATE sales_returns
SET replacement_delivery_status = sqlc.arg(to_status),
  updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND replacement_delivery_status = sqlc.arg(from_status);

-- name: SetReceivableReduction :exec
UPDATE sales_returns SET receivable_reduction = ? WHERE id = ?;

-- name: DeleteAutoSettlements :exec
-- The automatic offset memo goes with the offset it describes (released when
-- the offset pair is cancelled).
DELETE FROM sales_return_settlements WHERE return_id = ? AND auto = 1;

-- name: CancelledAmong :many
SELECT id FROM sales_returns
WHERE branch_id = ? AND status = 'cancelled' AND id IN (sqlc.slice('ids'));

-- name: ListConfirmedAfter :many
SELECT id, number, branch_id, return_date, total, status FROM sales_returns
WHERE branch_id = ? AND status = 'confirmed' AND id > ? ORDER BY id ASC LIMIT ?;

-- name: ActiveReturnCount :one
SELECT COUNT(*) AS total
FROM sales_returns
WHERE sales_order_id = ? AND status IN ('draft', 'confirmed');

-- name: InsertSettlement :execresult
INSERT INTO sales_return_settlements
  (return_id, settlement_type, settlement_date, amount, payment_method,
   reference_number, notes, auto, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListSettlements :many
SELECT id, settlement_type, settlement_date, amount, payment_method,
  reference_number, notes, auto
FROM sales_return_settlements WHERE return_id = ? ORDER BY id ASC;

-- name: OrderIDsByReturns :many
SELECT id, sales_order_id FROM sales_returns
WHERE id IN (sqlc.slice('ids'));

-- name: ConfirmedByOrders :many
SELECT id, number, branch_id, sales_order_id, total, status, return_mode, difference_amount FROM sales_returns
WHERE branch_id = sqlc.arg(branch_id) AND status = 'confirmed'
  AND sales_order_id IN (sqlc.slice('order_ids'))
ORDER BY id ASC;

-- name: ReturnedByProduct :many
-- Sums returned base qty per product over confirmed returns dated within the
-- inclusive day range (bounds are day-start instants computed in Go).
SELECT i.product_id, CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM sales_return_items i JOIN sales_returns s ON s.id = i.return_id
WHERE s.branch_id = sqlc.arg(branch_id) AND s.status = 'confirmed'
  AND s.return_date >= sqlc.arg(from_at) AND s.return_date <= sqlc.arg(to_at)
GROUP BY i.product_id;

-- name: NetAdjustment :one
-- The confirmed returns' effect on the sales figure: a return_only return
-- subtracts its total, an exchange adds replacement_total - total.
SELECT CAST(COALESCE(SUM(CASE WHEN return_mode = 'exchange' THEN replacement_total - total ELSE -total END), 0) AS SIGNED) AS total
FROM sales_returns
WHERE branch_id = sqlc.arg(branch_id) AND status = 'confirmed'
  AND return_date >= sqlc.arg(from_at) AND return_date <= sqlc.arg(to_at);

-- name: GetCreditSettlement :one
-- One memo row with its parent return (same module): the unit finance
-- journals as Dr Piutang / Cr Uang Muka Pelanggan (K-04a).
SELECT s.id, s.return_id, r.number, r.branch_id, r.sales_order_id,
  s.settlement_type, s.settlement_date, s.amount, r.status
FROM sales_return_settlements s JOIN sales_returns r ON r.id = s.return_id
WHERE s.id = ?;

-- name: ListCreditSettlementsAfter :many
-- customer_credit rows of CONFIRMED returns by ascending id after a cursor
-- (the posting queue's candidate list).
SELECT s.id, s.return_id, r.number, r.branch_id, r.sales_order_id,
  s.settlement_type, s.settlement_date, s.amount, r.status
FROM sales_return_settlements s JOIN sales_returns r ON r.id = s.return_id
WHERE r.branch_id = ? AND r.status = 'confirmed'
  AND s.settlement_type = 'customer_credit' AND s.id > ?
ORDER BY s.id ASC LIMIT ?;

-- name: CancelledCreditSettlementsAmong :many
-- customer_credit rows whose parent return was cancelled (journal orphans).
SELECT s.id FROM sales_return_settlements s JOIN sales_returns r ON r.id = s.return_id
WHERE r.branch_id = sqlc.arg(branch_id) AND r.status = 'cancelled'
  AND s.settlement_type = 'customer_credit' AND s.id IN (sqlc.slice('ids'));

-- name: ListCreditSettlementIDsByReturn :many
SELECT id FROM sales_return_settlements
WHERE return_id = ? AND settlement_type = 'customer_credit' ORDER BY id ASC;

-- name: SumCreditSettled :one
-- Value of the return already kept as customer balance (N-40): payments must
-- not refund or offset it a second time.
SELECT CAST(COALESCE(SUM(amount), 0) AS SIGNED) AS total
FROM sales_return_settlements
WHERE return_id = ? AND settlement_type = 'customer_credit';

-- name: SumCreditSettledByReturns :many
-- Credit settled per return for a page of returns (saldo list, N-41).
SELECT return_id, CAST(COALESCE(SUM(amount), 0) AS SIGNED) AS total
FROM sales_return_settlements
WHERE return_id IN (sqlc.slice('return_ids')) AND settlement_type = 'customer_credit'
GROUP BY return_id;
