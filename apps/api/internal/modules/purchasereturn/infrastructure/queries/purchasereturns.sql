-- Purchase return header + lines + settlement memos (F8-18). Twin of
-- salesreturn/infrastructure/queries/sales_returns.sql: keep both in step.
-- All queries touch only this module's tables, so the per-module schema
-- block in sqlc.yaml keeps enforcing the boundary at generate time.
-- The dynamic list/count filter lives in listfilter.go (raw-sql allow-list).

-- name: GetReturn :one
SELECT id, number, branch_id, purchase_order_id, return_date, subtotal,
  discount_total, tax_total, tax_type, tax_rate, total, status, notes,
  reason, payable_reduction, cancel_reason, cancelled_at, cancelled_by
FROM purchase_returns WHERE id = ?;

-- name: ListItemsByReturn :many
SELECT id, product_id, variant_id, location_id, uom, uom_factor,
  qty, qty_base, unit_price, discount_pct, discount_nominal,
  tax_base, tax_amount, line_total
FROM purchase_return_items WHERE return_id = ? ORDER BY id ASC;

-- name: InsertReturn :execresult
INSERT INTO purchase_returns
  (number, branch_id, purchase_order_id, return_date, subtotal, discount_total,
   tax_total, tax_type, tax_rate, total, status, notes, reason, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?);

-- name: InsertReturnItem :exec
INSERT INTO purchase_return_items
  (return_id, product_id, variant_id, location_id, uom, uom_factor,
   qty, qty_base, unit_price, discount_pct, discount_nominal,
   tax_base, tax_amount, line_total)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ReturnedQty :one
-- Sums confirmed-returned base qty for one PO line position.
SELECT CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM purchase_return_items i
JOIN purchase_returns p ON p.id = i.return_id
WHERE p.purchase_order_id = ? AND p.status = 'confirmed'
  AND i.product_id = ? AND i.variant_id = ?;

-- name: CommittedQtyByPO :many
-- Sums base qty per product/variant over the order's draft AND confirmed
-- returns: a draft holds its share of what can still go back, so two drafts
-- can never both claim the same received goods.
SELECT i.product_id, i.variant_id, CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM purchase_return_items i
JOIN purchase_returns p ON p.id = i.return_id
WHERE p.purchase_order_id = ? AND p.status IN ('draft', 'confirmed')
GROUP BY i.product_id, i.variant_id;

-- name: TransitionReturn :execresult
-- CAS: moves the status only from one of the allowed origins.
UPDATE purchase_returns
SET status = sqlc.arg(to_status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status IN (sqlc.slice('from_statuses'));

-- name: CancelReturn :execresult
-- CAS: voids a draft or confirmed return and records why, when and by whom
-- in ONE statement.
UPDATE purchase_returns
SET status = 'cancelled',
  cancel_reason = sqlc.arg(reason),
  cancelled_at = sqlc.arg(stamp), cancelled_by = sqlc.arg(actor_id),
  updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status IN ('draft', 'confirmed');

-- name: SetReturnStatus :exec
UPDATE purchase_returns
SET status = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: SetPayableReduction :exec
UPDATE purchase_returns SET payable_reduction = ? WHERE id = ?;

-- name: CancelledAmong :many
SELECT id FROM purchase_returns
WHERE branch_id = ? AND status = 'cancelled' AND id IN (sqlc.slice('ids'));

-- name: ListConfirmedAfter :many
SELECT id, number, branch_id, return_date, total, status FROM purchase_returns
WHERE branch_id = ? AND status = 'confirmed' AND id > ? ORDER BY id ASC LIMIT ?;

-- name: ActiveReturnCount :one
SELECT COUNT(*) AS total
FROM purchase_returns
WHERE purchase_order_id = ? AND status IN ('draft', 'confirmed');

-- name: InsertSettlement :execresult
INSERT INTO purchase_return_settlements
  (return_id, settlement_type, settlement_date, amount, payment_method,
   reference_number, notes, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListSettlements :many
SELECT id, settlement_type, settlement_date, amount, payment_method,
  reference_number, notes
FROM purchase_return_settlements WHERE return_id = ? ORDER BY id ASC;

-- name: ConfirmedByOrders :many
SELECT id, number, branch_id, purchase_order_id, total, status FROM purchase_returns
WHERE branch_id = sqlc.arg(branch_id) AND status = 'confirmed'
  AND purchase_order_id IN (sqlc.slice('order_ids'))
ORDER BY id ASC;

-- name: GetCreditSettlement :one
-- One memo row with its parent return (same module): the unit finance
-- journals as Dr Uang Muka Supplier / Cr Hutang (K-04a). Rows written before
-- the K-04b vocabulary fix carry the sales name 'customer_credit'.
SELECT s.id, s.return_id, r.number, r.branch_id, r.purchase_order_id,
  s.settlement_type, s.settlement_date, s.amount, r.status
FROM purchase_return_settlements s JOIN purchase_returns r ON r.id = s.return_id
WHERE s.id = ?;

-- name: ListCreditSettlementsAfter :many
-- supplier_credit rows of CONFIRMED returns by ascending id after a cursor
-- (the posting queue's candidate list).
SELECT s.id, s.return_id, r.number, r.branch_id, r.purchase_order_id,
  s.settlement_type, s.settlement_date, s.amount, r.status
FROM purchase_return_settlements s JOIN purchase_returns r ON r.id = s.return_id
WHERE r.branch_id = ? AND r.status = 'confirmed'
  AND s.settlement_type IN ('supplier_credit', 'customer_credit') AND s.id > ?
ORDER BY s.id ASC LIMIT ?;

-- name: CancelledCreditSettlementsAmong :many
-- supplier_credit rows whose parent return was cancelled (journal orphans).
SELECT s.id FROM purchase_return_settlements s JOIN purchase_returns r ON r.id = s.return_id
WHERE r.branch_id = sqlc.arg(branch_id) AND r.status = 'cancelled'
  AND s.settlement_type IN ('supplier_credit', 'customer_credit') AND s.id IN (sqlc.slice('ids'));

-- name: ListCreditSettlementIDsByReturn :many
SELECT id FROM purchase_return_settlements
WHERE return_id = ? AND settlement_type IN ('supplier_credit', 'customer_credit') ORDER BY id ASC;

-- name: SumCreditSettled :one
-- Value of the return already kept as supplier balance (N-40): payments must
-- not refund or offset it a second time. Old rows may carry the sales name.
SELECT CAST(COALESCE(SUM(amount), 0) AS SIGNED) AS total
FROM purchase_return_settlements
WHERE return_id = ? AND settlement_type IN ('supplier_credit', 'customer_credit');


-- name: ConfirmedQtyByOrders :many
-- Confirmed-returned base qty per order and product/variant for a page of
-- orders (PO list "Diretur" badge, PO-01).
SELECT p.purchase_order_id, i.product_id, i.variant_id,
  CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM purchase_return_items i
JOIN purchase_returns p ON p.id = i.return_id
WHERE p.status = 'confirmed' AND p.purchase_order_id IN (sqlc.slice('po_ids'))
GROUP BY p.purchase_order_id, i.product_id, i.variant_id;

-- name: SumCreditSettledByReturns :many
-- Credit settled per return for a page of returns (saldo list, N-41). Old rows
-- may carry the sales name 'customer_credit'.
SELECT return_id, CAST(COALESCE(SUM(amount), 0) AS SIGNED) AS total
FROM purchase_return_settlements
WHERE return_id IN (sqlc.slice('return_ids')) AND settlement_type IN ('supplier_credit', 'customer_credit')
GROUP BY return_id;
