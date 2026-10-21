-- id: Q-HPP-2
-- severity: warn
-- rule: none
-- action: cost_open harus = mov_open per cabang x produk: nilai dibagi ke mutasi opening_stock (tanpa mutasi sintetis); selisih = blocker COST_OPENING_UNMATCHED di transform
SELECT c.id_branch,c.id_product,SUM(c.quantity_in_base_uom) cost_open,
  COALESCE((SELECT SUM(im.quantity) FROM {inventory_movements} im WHERE im.id_branch=c.id_branch AND im.id_product=c.id_product AND im.reference_type='opening_stock'),0) mov_open
FROM {finance_inventory_cost_movements} c WHERE c.{co}=@c AND c.movement_type='opening' GROUP BY 1,2;
