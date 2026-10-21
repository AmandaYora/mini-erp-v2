-- name: InsertAllocation :exec
INSERT INTO payment_allocations (payment_id, order_type, order_id, amount) VALUES (?, ?, ?, ?);

-- name: ListAllocationsByPayment :many
SELECT order_type, order_id, amount FROM payment_allocations
WHERE payment_id = ? ORDER BY id ASC;

-- name: ListAllocationsByPayments :many
SELECT payment_id, order_type, order_id, amount FROM payment_allocations
WHERE payment_id IN (sqlc.slice('payment_ids')) ORDER BY id ASC;

-- name: SumPaidForOrder :one
SELECT CAST(COALESCE(SUM(a.amount), 0) AS SIGNED) AS total FROM payment_allocations a
JOIN payments p ON p.id = a.payment_id
WHERE a.order_type = ? AND a.order_id = ? AND p.status = 'active';

-- name: SumPaidForOrders :many
SELECT a.order_id, CAST(COALESCE(SUM(a.amount), 0) AS SIGNED) AS total FROM payment_allocations a
JOIN payments p ON p.id = a.payment_id
WHERE a.order_type = sqlc.arg(order_type) AND a.order_id IN (sqlc.slice('order_ids')) AND p.status = 'active'
GROUP BY a.order_id;

-- name: SumOffsetForOrders :many
-- The part of SumPaidForOrders that moved no money: offset legs (a return
-- set against the bill). An order's real cash payment is the difference.
SELECT a.order_id, CAST(COALESCE(SUM(a.amount), 0) AS SIGNED) AS total FROM payment_allocations a
JOIN payments p ON p.id = a.payment_id
WHERE a.order_type = sqlc.arg(order_type) AND a.order_id IN (sqlc.slice('order_ids'))
  AND p.status = 'active' AND p.method = 'offset'
GROUP BY a.order_id;

-- name: SumCreditUsedForReturn :one
-- Saldo (uang muka) of one return already used on bills (N-41): active
-- allocations ONTO the return made by legs that name this very return as their
-- credit source. Ordinary refunds/offsets of the return never carry a source.
SELECT CAST(COALESCE(SUM(a.amount), 0) AS SIGNED) AS total FROM payment_allocations a
JOIN payments p ON p.id = a.payment_id
WHERE a.order_type = sqlc.arg(order_type) AND a.order_id = sqlc.arg(order_id) AND p.status = 'active'
  AND p.credit_source_type = a.order_type AND p.credit_source_id = a.order_id;

-- name: SumCreditUsedForReturns :many
SELECT a.order_id, CAST(COALESCE(SUM(a.amount), 0) AS SIGNED) AS total FROM payment_allocations a
JOIN payments p ON p.id = a.payment_id
WHERE a.order_type = sqlc.arg(order_type) AND a.order_id IN (sqlc.slice('order_ids')) AND p.status = 'active'
  AND p.credit_source_type = a.order_type AND p.credit_source_id = a.order_id
GROUP BY a.order_id;

-- name: ListAllocationOrdersByPayments :many
SELECT payment_id, order_id FROM payment_allocations
WHERE order_type = sqlc.arg(order_type) AND payment_id IN (sqlc.slice('payment_ids'));
