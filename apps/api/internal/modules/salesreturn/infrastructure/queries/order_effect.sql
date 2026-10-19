-- Money effect of the CONFIRMED returns on their source orders (SL-03): one
-- grouped read each over the module's own tables, so an order list page costs
-- two statements however many orders it has.

-- name: OrderReturnTotals :many
-- adjustment: signed effect on the order total (a return-only return subtracts
-- its total, an exchange adds replacement - returned). offset_total is the part
-- already offset against the order's open bill (receivable_reduction).
SELECT sales_order_id,
       CAST(COALESCE(SUM(CASE WHEN return_mode = 'exchange' THEN difference_amount ELSE -total END), 0) AS SIGNED) AS adjustment,
       CAST(COALESCE(SUM(receivable_reduction), 0) AS SIGNED) AS offset_total
FROM sales_returns
WHERE branch_id = sqlc.arg(branch_id) AND status = 'confirmed'
  AND sales_order_id IN (sqlc.slice('order_ids'))
GROUP BY sales_order_id;

-- name: OrderReturnSettlements :many
-- Operator-entered settlement memos (auto offset rows excluded) per order and type.
SELECT r.sales_order_id, s.settlement_type,
       CAST(COALESCE(SUM(s.amount), 0) AS SIGNED) AS total
FROM sales_return_settlements s
JOIN sales_returns r ON r.id = s.return_id
WHERE r.branch_id = sqlc.arg(branch_id) AND r.status = 'confirmed' AND s.auto = 0
  AND r.sales_order_id IN (sqlc.slice('order_ids'))
GROUP BY r.sales_order_id, s.settlement_type;
