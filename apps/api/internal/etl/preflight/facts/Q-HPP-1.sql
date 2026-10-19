-- id: Q-HPP-1
-- severity: info
-- rule: none
-- action: sebaran baris biaya
SELECT movement_type,source_type,COUNT(*) n,SUM(id_inventory_movement IS NULL) no_mov,SUM(id_product_variant IS NULL) no_var FROM {finance_inventory_cost_movements} WHERE {co}=@c GROUP BY 1,2;
