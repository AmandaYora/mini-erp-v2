-- Grouped / counting / operational reads over the sales tables (F8-18).

-- name: CountOrdersByStatus :many
SELECT status, COUNT(*) AS total
FROM sales_orders
WHERE branch_id = ?
GROUP BY status;

-- name: OrderPartiesByIDs :many
SELECT id, party_id
FROM sales_orders
WHERE id IN (sqlc.slice('ids'));

-- name: TopProducts :many
-- Completed orders only, ranked by base qty sold; names come from the frozen
-- line snapshots, so no product lookup per row.
SELECT i.product_id, i.variant_id, i.product_code, i.product_name,
       CAST(COALESCE(SUM(i.qty_base), 0) AS DECIMAL(36,12)) AS qty_base,
       CAST(COALESCE(SUM(i.line_total), 0) AS SIGNED) AS revenue,
       COUNT(DISTINCT o.id) AS orders
FROM sales_order_items i
JOIN sales_orders o ON o.id = i.order_id
WHERE o.branch_id = sqlc.arg(branch_id) AND o.status = 'completed'
  AND o.completed_at >= sqlc.arg(from_at) AND o.completed_at <= sqlc.arg(to_at)
GROUP BY i.product_id, i.variant_id, i.product_code, i.product_name
ORDER BY SUM(i.qty_base) DESC
LIMIT ?;

-- name: ListOpenHeaders :many
-- Confirmed order headers oldest-first for the delivery work queue. A POS order
-- joins only once it has sat confirmed since before orphan_before.
SELECT id, number, party_id, order_date, due_date, payment_terms, grand_total,
       CAST(channel = 'pos' AS SIGNED) AS orphan_pos
FROM sales_orders
WHERE branch_id = sqlc.arg(branch_id) AND status = 'confirmed' AND origin <> 'delivery'
  AND (channel <> 'pos' OR updated_at < sqlc.arg(orphan_before))
ORDER BY id ASC
LIMIT ?;

-- name: ListOpenHeadersFiltered :many
-- ListOpenHeaders plus the work queue's filters (DL-02), all applied BEFORE the
-- LIMIT/OFFSET so an order beyond the first page is still findable: an order-date
-- range [from_at, to_before) and a search that matches the number OR one of the
-- customer ids the caller resolved through the party contract.
SELECT id, number, party_id, order_date, due_date, payment_terms, grand_total,
       CAST(channel = 'pos' AS SIGNED) AS orphan_pos
FROM sales_orders
WHERE branch_id = sqlc.arg(branch_id) AND status = 'confirmed' AND origin <> 'delivery'
  AND (channel <> 'pos' OR updated_at < sqlc.arg(orphan_before))
  AND (sqlc.narg(from_at) IS NULL OR order_date >= sqlc.narg(from_at))
  AND (sqlc.narg(to_before) IS NULL OR order_date < sqlc.narg(to_before))
  AND (sqlc.narg(number_like) IS NULL
       OR number LIKE sqlc.narg(number_like)
       OR party_id IN (sqlc.slice('party_ids')))
ORDER BY id ASC
LIMIT ? OFFSET ?;

-- name: SearchOrderIDs :many
-- Order ids of a branch whose number matches or whose customer is one of the ids
-- the caller resolved; the delivery work queue's "waiting for the signed SJ" tab
-- searches its notes by order through this (DL-02).
SELECT id FROM sales_orders
WHERE branch_id = sqlc.arg(branch_id)
  AND (number LIKE sqlc.arg(number_like) OR party_id IN (sqlc.slice('party_ids')))
ORDER BY id DESC
LIMIT ?;

-- name: ListItemsByOrders :many
SELECT order_id, id, product_id, variant_id, product_code, product_name,
       uom, uom_factor, qty, qty_base, unit_price, discount_pct,
       discount_nominal, line_total
FROM sales_order_items
WHERE order_id IN (sqlc.slice('order_ids'))
ORDER BY order_id ASC, id ASC;

-- name: CountMissingTaxInvoice :one
SELECT COUNT(*) AS total
FROM sales_orders
WHERE branch_id = ? AND status IN ('confirmed','completed')
  AND tax_type <> 'none'
  AND (tax_invoice_number IS NULL OR tax_invoice_number = '');

-- name: CountMissingTaxInvoiceBetween :one
SELECT COUNT(*) AS total
FROM sales_orders
WHERE branch_id = ? AND status IN ('confirmed','completed')
  AND tax_type <> 'none'
  AND (tax_invoice_number IS NULL OR tax_invoice_number = '')
  AND order_date >= sqlc.arg(from_at) AND order_date <= sqlc.arg(to_at);

-- name: SumCompletedSales :one
SELECT CAST(COALESCE(SUM(grand_total), 0) AS SIGNED) AS grand_sum, COUNT(*) AS total
FROM sales_orders
WHERE branch_id = ? AND status = 'completed'
  AND completed_at >= sqlc.arg(from_at) AND completed_at <= sqlc.arg(to_at);

-- name: CountOrdersBetween :one
SELECT COUNT(*) AS total
FROM sales_orders
WHERE branch_id = ? AND status <> 'cancelled'
  AND order_date >= sqlc.arg(from_at) AND order_date <= sqlc.arg(to_at);

-- name: ListPriorityOrders :many
-- Live (draft/confirmed) orders scored like the legacy system: past due 100, due
-- within 2 days 80, pending (draft) 60, otherwise 40, plus the order's age in
-- days (max 30). today is the caller's Jakarta calendar day.
SELECT id, number, party_id, grand_total, status, order_date, due_date, CAST(score AS SIGNED) AS score FROM (
  SELECT id, number, party_id, grand_total, status, order_date, due_date,
    (CASE
       WHEN due_date IS NOT NULL AND DATE(due_date) < CAST(sqlc.arg(today) AS DATE) THEN 100
       WHEN due_date IS NOT NULL AND DATE(due_date) <= DATE_ADD(CAST(sqlc.arg(today) AS DATE), INTERVAL 2 DAY) THEN 80
       WHEN status = 'draft' THEN 60
       ELSE 40 END)
    + LEAST(GREATEST(DATEDIFF(CAST(sqlc.arg(today) AS DATE), DATE(order_date)), 0), 30) AS score
  FROM sales_orders WHERE branch_id = sqlc.arg(branch_id) AND status IN ('draft', 'confirmed')
) t
ORDER BY score DESC, id ASC
LIMIT ?;

-- name: ListOpenOrderTotals :many
-- The index is forced (F4-17): left to its statistics the optimizer picked
-- uq_sales_orders_client_ref (a ref on the all-NULL second column) plus a
-- filesort of the whole branch for every 1.000-row page. (branch_id, id)
-- reads exactly one page in id order.
SELECT id, number, party_id, grand_total, order_date, due_date
FROM sales_orders FORCE INDEX (ix_sales_orders_branch_id)
WHERE branch_id = ? AND status IN ('confirmed', 'completed') AND id > ?
ORDER BY id ASC
LIMIT ?;

-- name: ReportOrderTotals :one
-- status NULL = every non-cancelled order; NULL bounds = unbounded.
SELECT COUNT(*) AS total, CAST(COALESCE(SUM(grand_total), 0) AS SIGNED) AS grand_sum
FROM sales_orders
WHERE branch_id = sqlc.arg(branch_id)
  AND ((sqlc.narg('status') IS NULL AND status <> 'cancelled') OR status = sqlc.narg('status'))
  AND (sqlc.narg('from_at') IS NULL OR order_date >= sqlc.narg('from_at'))
  AND (sqlc.narg('to_at') IS NULL OR order_date <= sqlc.narg('to_at'));

-- name: ReportOrdersPage :many
SELECT id, number, party_id, order_date, due_date, grand_total, status
FROM sales_orders
WHERE branch_id = sqlc.arg(branch_id)
  AND ((sqlc.narg('status') IS NULL AND status <> 'cancelled') OR status = sqlc.narg('status'))
  AND (sqlc.narg('from_at') IS NULL OR order_date >= sqlc.narg('from_at'))
  AND (sqlc.narg('to_at') IS NULL OR order_date <= sqlc.narg('to_at'))
ORDER BY id DESC
LIMIT ? OFFSET ?;

-- name: ListUnshippedOrderTotals :many
-- Data-health A12 (ADR-0025): the draft/confirmed (not yet fully shipped)
-- orders by ascending id after a cursor; same forced index as above.
SELECT id, number, party_id, grand_total, order_date, due_date
FROM sales_orders FORCE INDEX (ix_sales_orders_branch_id)
WHERE branch_id = ? AND status IN ('draft', 'confirmed') AND id > ?
ORDER BY id ASC
LIMIT ?;
