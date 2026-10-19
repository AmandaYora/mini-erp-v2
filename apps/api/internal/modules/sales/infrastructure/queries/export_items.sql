-- Full line snapshots of many orders in one read, for the order export's
-- "Detail Item" sheet (SL-04). Keyset-paged by the caller (500 orders a page).

-- name: ListItemsByOrdersFull :many
SELECT order_id, id, product_id, variant_id, product_code, product_name, uom,
       uom_factor, qty, qty_base, unit_price, discount_pct, discount_nominal, line_total,
       stock_tracked, item_type, variant_name, variant_code, base_uom,
       pricing_source, base_price, discount_reason, line_note, taxable, tax_base, tax_amount
FROM sales_order_items
WHERE order_id IN (sqlc.slice('order_ids'))
ORDER BY order_id ASC, id ASC;
