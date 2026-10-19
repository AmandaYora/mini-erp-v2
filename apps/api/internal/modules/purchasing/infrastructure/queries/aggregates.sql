-- Grouped / counting reads over purchase_orders (F8-18).

-- name: CountOrdersByStatus :many
SELECT status, COUNT(*) AS total
FROM purchase_orders
WHERE branch_id = ?
GROUP BY status;

-- name: OrderPartiesByIDs :many
SELECT id, party_id
FROM purchase_orders
WHERE id IN (sqlc.slice('ids'));

-- name: CountMissingSupplierInvoiceBetween :one
SELECT COUNT(*) AS total
FROM purchase_orders
WHERE branch_id = ? AND status IN ('confirmed','completed')
  AND tax_type <> 'none'
  AND (supplier_invoice_number IS NULL OR supplier_invoice_number = '')
  AND order_date >= sqlc.arg(from_at) AND order_date <= sqlc.arg(to_at);

-- name: CountMissingSupplierInvoice :one
SELECT COUNT(*) AS total
FROM purchase_orders
WHERE branch_id = ? AND status IN ('confirmed','completed')
  AND tax_type <> 'none'
  AND (supplier_invoice_number IS NULL OR supplier_invoice_number = '');

-- name: CountOrdersBetween :one
SELECT COUNT(*) AS total
FROM purchase_orders
WHERE branch_id = ? AND status <> 'cancelled'
  AND order_date >= sqlc.arg(from_at) AND order_date <= sqlc.arg(to_at);

-- name: ListOpenOrderTotals :many
SELECT id, number, party_id, grand_total, order_date, due_date
FROM purchase_orders
WHERE branch_id = ? AND status IN ('confirmed', 'completed') AND id > ?
ORDER BY id ASC
LIMIT ?;

-- name: ReportOrderTotals :one
-- status NULL = every non-cancelled order; NULL bounds = unbounded.
SELECT COUNT(*) AS total, CAST(COALESCE(SUM(grand_total), 0) AS SIGNED) AS grand_sum
FROM purchase_orders
WHERE branch_id = sqlc.arg(branch_id)
  AND ((sqlc.narg('status') IS NULL AND status <> 'cancelled') OR status = sqlc.narg('status'))
  AND (sqlc.narg('from_at') IS NULL OR order_date >= sqlc.narg('from_at'))
  AND (sqlc.narg('to_at') IS NULL OR order_date <= sqlc.narg('to_at'));

-- name: ReportOrdersPage :many
SELECT id, number, party_id, order_date, due_date, grand_total, status
FROM purchase_orders
WHERE branch_id = sqlc.arg(branch_id)
  AND ((sqlc.narg('status') IS NULL AND status <> 'cancelled') OR status = sqlc.narg('status'))
  AND (sqlc.narg('from_at') IS NULL OR order_date >= sqlc.narg('from_at'))
  AND (sqlc.narg('to_at') IS NULL OR order_date <= sqlc.narg('to_at'))
ORDER BY id DESC
LIMIT ? OFFSET ?;
