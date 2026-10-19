-- id: Q-STK-6
-- severity: info
-- rule: none
-- action: mutasi bertabrakan digabung
SELECT reference_type,id_reference,id_product,id_product_variant,id_stock_location,movement_type,COUNT(*) n FROM {inventory_movements}
WHERE id_reference IS NOT NULL AND id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) GROUP BY 1,2,3,4,5,6 HAVING n>1;
