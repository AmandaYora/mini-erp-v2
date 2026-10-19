-- Payment facts per order for readers that label an order (SL-04 export
-- columns "Metode Bayar"/"Kasir", SL-10 receipt cashier): one grouped read over
-- the module's own tables, active payments only, internal offset legs excluded
-- (they moved no cash and have no cashier).

-- name: OrderPaymentInfo :many
SELECT a.order_id, p.method, p.created_by
FROM payment_allocations a
JOIN payments p ON p.id = a.payment_id
WHERE a.order_type = sqlc.arg(order_type) AND a.order_id IN (sqlc.slice('order_ids'))
  AND p.status = 'active' AND p.method <> 'offset'
ORDER BY a.order_id ASC, p.id ASC;
