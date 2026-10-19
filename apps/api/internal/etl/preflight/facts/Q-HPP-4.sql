-- id: Q-HPP-4
-- severity: info
-- rule: none
-- action: nilai ledger vs state
SELECT s.id_branch,s.id_product,s.inventory_value,
  (SELECT c.inventory_value_after FROM {finance_inventory_cost_movements} c WHERE c.id_branch=s.id_branch AND c.id_product=s.id_product ORDER BY c.id_finance_inventory_cost_movement DESC LIMIT 1) last_after
FROM {finance_inventory_cost_states} s WHERE s.{co}=@c HAVING ABS(inventory_value-COALESCE(last_after,0))>0.009;
