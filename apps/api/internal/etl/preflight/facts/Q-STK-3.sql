-- id: Q-STK-3
-- severity: info
-- rule: none
-- action: SJ sintetis untuk serah langsung
SELECT COUNT(DISTINCT id_reference) orders_direct,COUNT(*) mov FROM {inventory_movements} WHERE reference_type='order' AND id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c);
