-- Who carried an order (SL-04 export columns "Sopir"/"No Plat"): driver and
-- vehicle of the order's live shipments, one grouped read over this module's table.

-- name: OrderTransportByOrders :many
SELECT sales_order_id, driver_name, vehicle_plate
FROM delivery_notes
WHERE sales_order_id IN (sqlc.slice('order_ids'))
  AND document_kind = 'order' AND status IN ('draft', 'confirmed')
ORDER BY sales_order_id ASC, id ASC;
