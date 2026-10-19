-- id: Q-RET-0
-- severity: blocker
-- rule: col:n>0
-- action: retur yang TIDAK dapat dipetakan transform (status/mode/jenis penyelesaian/jenis baris tak dikenal, order asal bukan order termigrasi, item tanpa produk): perbaiki data atau putuskan padanannya; retur biasa dimigrasi
SELECT
 (SELECT COUNT(*) FROM {sales_returns} WHERE {co} = @c AND (status NOT IN ('completed','cancelled') OR return_mode NOT IN ('return_only','exchange')
    OR replacement_delivery_status NOT IN ('not_required','pending','dispatched','delivered')
    OR id_original_order NOT IN (SELECT id_order FROM {orders} WHERE order_kind = 'sales')))
 + (SELECT COUNT(*) FROM {purchase_returns} WHERE {co} = @c AND (status NOT IN ('completed','cancelled')
    OR id_original_order NOT IN (SELECT id_order FROM {orders} WHERE order_kind = 'purchase')))
 + (SELECT COUNT(*) FROM {sales_return_items} i JOIN {sales_returns} r ON r.id_sales_return = i.id_sales_return
    WHERE r.{co} = @c AND (i.line_type NOT IN ('returned','replacement') OR (i.return_condition IS NOT NULL AND i.return_condition NOT IN ('normal','damaged','not_returned'))
    OR i.id_product_variant NOT IN (SELECT id_product_variant FROM {product_variants})))
 + (SELECT COUNT(*) FROM {purchase_return_items} i JOIN {purchase_returns} r ON r.id_purchase_return = i.id_purchase_return
    WHERE r.{co} = @c AND i.id_product_variant NOT IN (SELECT id_product_variant FROM {product_variants}))
 + (SELECT COUNT(*) FROM {sales_return_settlements} s JOIN {sales_returns} r ON r.id_sales_return = s.id_sales_return
    WHERE r.{co} = @c AND s.archived_at IS NULL AND s.settlement_type NOT IN ('refund','customer_credit','collect_payment'))
 + (SELECT COUNT(*) FROM {purchase_return_settlements} s JOIN {purchase_returns} r ON r.id_purchase_return = s.id_purchase_return
    WHERE r.{co} = @c AND s.archived_at IS NULL AND s.settlement_type NOT IN ('refund','supplier_credit')) n;
