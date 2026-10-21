-- Goods receipt header + lines (F8-18). All queries touch only this
-- module's tables (goods_receipts, goods_receipt_items), so the per-module
-- schema block in sqlc.yaml keeps enforcing the boundary at generate time.

-- name: GetReceipt :one
SELECT id, number, branch_id, purchase_order_id, received_at, notes,
  supplier_delivery_number, status, client_ref, created_by,
  cancel_reason, cancelled_at, cancelled_by
FROM goods_receipts
WHERE id = ?;

-- name: GetReceiptByClientRef :one
SELECT id, number, branch_id, purchase_order_id, received_at, notes,
  supplier_delivery_number, status, client_ref, created_by,
  cancel_reason, cancelled_at, cancelled_by
FROM goods_receipts
WHERE branch_id = ? AND client_ref = ?;

-- name: ListItemsByReceipt :many
SELECT id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base
FROM goods_receipt_items
WHERE receipt_id = ?
ORDER BY id ASC;

-- name: InsertReceipt :execresult
INSERT INTO goods_receipts
  (number, branch_id, purchase_order_id, received_at, notes,
   supplier_delivery_number, status, client_ref, created_by)
VALUES (?, ?, ?, ?, ?, ?, 'pending', ?, ?);

-- name: InsertReceiptItem :exec
INSERT INTO goods_receipt_items
  (receipt_id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base)
VALUES (?, ?, ?, ?, ?, ?, ?, ?);

-- name: TransitionReceipt :execresult
-- CAS: moves the status only from one of the allowed origins.
UPDATE goods_receipts
SET status = sqlc.arg(to_status)
WHERE id = sqlc.arg(id) AND status IN (sqlc.slice('from_statuses'));

-- name: DiscardReceipt :execresult
-- CAS: voids a PENDING receipt and records why, when and by whom in ONE
-- statement.
UPDATE goods_receipts
SET status = 'void',
    cancel_reason = sqlc.arg(reason),
    cancelled_at = sqlc.arg(stamp), cancelled_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: ReceivedQty :one
-- Sums POSTED received base qty for one PO line position. A pending receipt
-- has not moved stock yet, so it is not "received".
SELECT CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM goods_receipt_items i
JOIN goods_receipts g ON g.id = i.receipt_id
WHERE g.purchase_order_id = ? AND g.status = 'posted'
  AND i.product_id = ? AND i.variant_id = ?;

-- name: QtyByPO :many
-- Sums base qty per product/variant over the PO's receipts in the given
-- statuses — one GROUP BY, never one query per line.
SELECT i.product_id, i.variant_id, CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM goods_receipt_items i
JOIN goods_receipts g ON g.id = i.receipt_id
WHERE g.purchase_order_id = sqlc.arg(po_id) AND g.status IN (sqlc.slice('statuses'))
GROUP BY i.product_id, i.variant_id;

-- name: PostedQtyBefore :many
-- Sums line-UOM qty per product/variant/uom over the order's POSTED receipts
-- with an id below the cursor (rounding-remainder split in finance).
SELECT i.product_id, i.variant_id, i.uom, CAST(COALESCE(SUM(i.qty), 0) AS DECIMAL(36,12)) AS total
FROM goods_receipt_items i
JOIN goods_receipts g ON g.id = i.receipt_id
WHERE g.purchase_order_id = ? AND g.status = 'posted' AND g.id < ?
GROUP BY i.product_id, i.variant_id, i.uom;

-- name: ActiveReceiptCount :one
SELECT COUNT(*) AS total
FROM goods_receipts
WHERE purchase_order_id = ? AND status IN ('pending', 'posted');

-- name: PendingReceiptCount :one
-- Receipts still pending on an order: a correction of the order waits for them.
SELECT COUNT(*) AS total
FROM goods_receipts
WHERE purchase_order_id = ? AND status = 'pending';

-- name: ListPostedAfter :many
SELECT id, number, branch_id, purchase_order_id, received_at, notes,
  supplier_delivery_number, status, client_ref, created_by,
  cancel_reason, cancelled_at, cancelled_by
FROM goods_receipts
WHERE branch_id = ? AND status = 'posted' AND id > ?
ORDER BY id ASC LIMIT ?;

-- name: PurchaseOrderIDsByReceipts :many
SELECT id, purchase_order_id
FROM goods_receipts
WHERE id IN (sqlc.slice('ids'));

-- name: ListReceipts :many
SELECT id, number, branch_id, purchase_order_id, received_at, notes,
  supplier_delivery_number, status, client_ref, created_by,
  cancel_reason, cancelled_at, cancelled_by
FROM goods_receipts
WHERE branch_id = sqlc.arg(branch_id)
  AND (sqlc.narg('po_id') IS NULL OR purchase_order_id = sqlc.narg('po_id'))
  AND (sqlc.narg('status') IS NULL OR status = sqlc.narg('status'))
ORDER BY id DESC LIMIT ? OFFSET ?;

-- name: CountReceipts :one
SELECT COUNT(*) AS total
FROM goods_receipts
WHERE branch_id = sqlc.arg(branch_id)
  AND (sqlc.narg('po_id') IS NULL OR purchase_order_id = sqlc.narg('po_id'))
  AND (sqlc.narg('status') IS NULL OR status = sqlc.narg('status'));


-- name: CountItemsByReceipts :many
-- Line count per receipt for a page of the list (one grouped read, GR-01).
SELECT receipt_id, COUNT(*) AS line_count
FROM goods_receipt_items
WHERE receipt_id IN (sqlc.slice('ids'))
GROUP BY receipt_id;

-- name: ReceivedQtyByOrders :many
-- POSTED received base qty per order and product/variant for a page of
-- orders (the PO list's "Diretur" badge compares it with returned qty, PO-01).
SELECT g.purchase_order_id, i.product_id, i.variant_id,
  CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM goods_receipt_items i
JOIN goods_receipts g ON g.id = i.receipt_id
WHERE g.status = 'posted' AND g.purchase_order_id IN (sqlc.slice('po_ids'))
GROUP BY g.purchase_order_id, i.product_id, i.variant_id;

-- name: ListPostedItemLocations :many
-- Location of every line of the order's POSTED receipts, newest line first:
-- the first row per product/variant is the latest receipt location, the
-- default source location of a purchase return (RT-04).
SELECT i.product_id, i.variant_id, i.location_id
FROM goods_receipt_items i
JOIN goods_receipts g ON g.id = i.receipt_id
WHERE g.purchase_order_id = ? AND g.status = 'posted'
ORDER BY i.id DESC;

-- name: FirstPostedReceivedAt :one
-- Earliest receive date of the order's POSTED receipts (no row = none yet).
SELECT received_at
FROM goods_receipts
WHERE purchase_order_id = ? AND status = 'posted'
ORDER BY received_at ASC LIMIT 1;
