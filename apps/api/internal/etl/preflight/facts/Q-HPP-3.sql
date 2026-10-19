-- id: Q-HPP-3
-- severity: warn
-- rule: none
-- action: baris biaya estimasi dibuat bila ID < kursor
SELECT im.id_branch,im.reference_type,COUNT(*) n FROM {inventory_movements} im
WHERE im.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND im.movement_type IN ('in','out')
  AND NOT EXISTS(SELECT 1 FROM {finance_inventory_cost_movements} c WHERE c.id_inventory_movement=im.id_inventory_movement) GROUP BY 1,2;
