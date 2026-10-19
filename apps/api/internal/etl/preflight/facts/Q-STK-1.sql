-- id: Q-STK-1
-- severity: warn
-- rule: none
-- action: mutasi koreksi 'Koreksi migrasi' ditambahkan
SELECT ib.id_branch,ib.id_product,ib.id_product_variant,ib.id_stock_location,ib.on_hand_qty,COALESCE(m.net,0) mov_net
FROM {inventory_balances} ib LEFT JOIN (SELECT id_branch,id_product,id_product_variant,id_stock_location,SUM(balance_after-balance_before) net
   FROM {inventory_movements} GROUP BY 1,2,3,4) m USING(id_branch,id_product,id_product_variant,id_stock_location)
WHERE ib.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND ABS(ib.on_hand_qty-COALESCE(m.net,0))>0.000000000001;
