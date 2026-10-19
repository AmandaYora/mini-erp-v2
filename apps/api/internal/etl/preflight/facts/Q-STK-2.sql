-- id: Q-STK-2
-- severity: blocker
-- rule: notin:reference_type=delivery_note,goods_receipt,sales_return,sales_return_exchange,purchase_return,opening_stock,manual_adjustment,stock_transfer,transfer,damaged_stock,damaged_restore,damaged_write_off,order
-- action: reference_type NULL/tak dikenal
SELECT reference_type,movement_type,COUNT(*) n FROM {inventory_movements} WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) GROUP BY 1,2 ORDER BY 3 DESC;
