-- Delivery note header + lines (F8-18). All queries touch only this
-- module's tables (delivery_notes, delivery_note_items), so the per-module
-- schema block in sqlc.yaml keeps enforcing the boundary at generate time.
-- The dynamic list/count/waiting filters live in listfilter.go (raw-sql
-- allow-list). The client_ref column is never projected (as before: reads
-- resolve it via NoteIDByClientRef only).

-- name: GetNote :one
SELECT id, number, branch_id, sales_order_id, delivery_date,
  driver_name, vehicle_plate, warehouse_staff_name, recipient_name,
  recipient_signature_status, recipient_signature_missing_reason,
  drop_location_note, dispatched_at, dispatched_by, confirmed_at,
  confirmed_by, document_kind, sales_return_id, status, notes,
  cancel_reason, cancelled_at, cancelled_by
FROM delivery_notes WHERE id = ?;

-- name: ListItemsByDelivery :many
SELECT id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base, reservation_key
FROM delivery_note_items WHERE delivery_id = ? ORDER BY id ASC;

-- name: InsertNote :execresult
INSERT INTO delivery_notes
  (number, branch_id, sales_order_id, delivery_date, driver_name,
   vehicle_plate, warehouse_staff_name, drop_location_note,
   document_kind, sales_return_id,
   status, notes, client_ref, created_by, updated_by)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?, ?, ?);

-- name: InsertNoteItem :exec
INSERT INTO delivery_note_items
  (delivery_id, product_id, variant_id, location_id, uom, uom_factor, qty, qty_base, reservation_key)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: DeliveredQty :one
-- Sums confirmed-delivered base qty for one SO line position. Replacement
-- shipments (document_kind <> 'order') are excluded: they ship new goods,
-- they don't fulfill the order.
SELECT CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM delivery_note_items i
JOIN delivery_notes d ON d.id = i.delivery_id
WHERE d.sales_order_id = ? AND d.status = 'confirmed'
  AND d.document_kind = 'order'
  AND i.product_id = ? AND i.variant_id = ?;

-- name: CommittedQtyBySO :many
-- Sums base qty per product/variant over the order's draft AND confirmed
-- order-kind notes: a draft holds its share of the order's remaining quantity
-- so two drafts can never both claim it.
SELECT i.product_id, i.variant_id, CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM delivery_note_items i
JOIN delivery_notes d ON d.id = i.delivery_id
WHERE d.sales_order_id = ? AND d.status IN ('draft', 'confirmed')
  AND d.document_kind = 'order'
GROUP BY i.product_id, i.variant_id;

-- name: ConfirmedQtyBefore :many
-- Sums line-UOM qty per product/variant/uom over the order's confirmed
-- order-kind notes with an id below the cursor.
SELECT i.product_id, i.variant_id, i.uom, CAST(COALESCE(SUM(i.qty), 0) AS DECIMAL(36,12)) AS total
FROM delivery_note_items i
JOIN delivery_notes d ON d.id = i.delivery_id
WHERE d.sales_order_id = ? AND d.status = 'confirmed'
  AND d.document_kind = 'order' AND d.id < ?
GROUP BY i.product_id, i.variant_id, i.uom;

-- name: ActiveNoteCount :one
SELECT COUNT(*) AS total
FROM delivery_notes
WHERE sales_order_id = ? AND document_kind = 'order' AND status IN ('draft', 'confirmed');

-- name: ListConfirmedAfter :many
SELECT id, number, branch_id, sales_order_id, delivery_date,
  driver_name, vehicle_plate, warehouse_staff_name, recipient_name,
  recipient_signature_status, recipient_signature_missing_reason,
  drop_location_note, dispatched_at, dispatched_by, confirmed_at,
  confirmed_by, document_kind, sales_return_id, status, notes,
  cancel_reason, cancelled_at, cancelled_by
FROM delivery_notes
WHERE branch_id = ? AND status = 'confirmed' AND id > ? ORDER BY id ASC LIMIT ?;

-- name: DeliveredQtyBulk :many
-- Sums confirmed order-kind deliveries per (order, product, variant) — one
-- GROUP BY for the whole work queue, never one query per line.
SELECT d.sales_order_id, i.product_id, i.variant_id, CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS total
FROM delivery_note_items i
JOIN delivery_notes d ON d.id = i.delivery_id
WHERE d.sales_order_id IN (sqlc.slice('order_ids'))
  AND d.status = 'confirmed' AND d.document_kind = 'order'
GROUP BY d.sales_order_id, i.product_id, i.variant_id;

-- name: DraftCounts :many
-- Counts draft order-kind notes per order — one GROUP BY for the whole queue.
SELECT sales_order_id, COUNT(*) AS total FROM delivery_notes
WHERE sales_order_id IN (sqlc.slice('order_ids'))
  AND status = 'draft' AND document_kind = 'order'
GROUP BY sales_order_id;

-- name: NoteIDByClientRef :one
SELECT id FROM delivery_notes WHERE branch_id = ? AND client_ref = ?;

-- name: SetNoteStatus :exec
UPDATE delivery_notes
SET status = ?, updated_at = sqlc.arg(now), updated_by = ? WHERE id = ?;

-- name: TransitionNote :execresult
-- CAS: moves the status only from one of the allowed origins.
UPDATE delivery_notes
SET status = sqlc.arg(to_status), updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status IN (sqlc.slice('from_statuses'));

-- name: CancelNote :execresult
-- CAS: voids a draft and records why, when and by whom in ONE statement.
UPDATE delivery_notes
SET status = 'cancelled',
    cancel_reason = sqlc.arg(reason),
    cancelled_at = sqlc.arg(stamp), cancelled_by = sqlc.arg(actor_id),
    updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = 'draft';

-- name: ConfirmNote :execresult
-- Flips draft -> confirmed and records dispatch + receipt evidence in ONE
-- statement guarded by the status (CAS).
UPDATE delivery_notes
SET status = 'confirmed',
    recipient_name = sqlc.arg(recipient_name),
    recipient_signature_status = sqlc.arg(sig_status),
    recipient_signature_missing_reason = sqlc.arg(sig_reason),
    dispatched_at = sqlc.arg(stamp), dispatched_by = sqlc.arg(actor_id),
    confirmed_at = sqlc.arg(stamp), confirmed_by = sqlc.arg(actor_id),
    updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id) AND status = 'draft';

-- name: SetDispatch :exec
-- Stamps the dispatch + confirm event without flipping the status.
UPDATE delivery_notes
SET recipient_name = sqlc.arg(recipient_name),
    recipient_signature_status = sqlc.arg(sig_status),
    recipient_signature_missing_reason = sqlc.arg(sig_reason),
    dispatched_at = sqlc.arg(stamp), dispatched_by = sqlc.arg(actor_id),
    confirmed_at = sqlc.arg(stamp), confirmed_by = sqlc.arg(actor_id),
    updated_at = sqlc.arg(now), updated_by = sqlc.arg(actor_id)
WHERE id = sqlc.arg(id);

-- name: OrderIDsByNotes :many
SELECT id, sales_order_id FROM delivery_notes
WHERE id IN (sqlc.slice('ids'));
