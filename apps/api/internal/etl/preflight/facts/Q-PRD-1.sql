-- id: Q-PRD-1
-- severity: warn
-- rule: none
-- action: service/non_stock -> jasa
SELECT p.product_type,p.stock_tracked,COUNT(*) n,SUM(EXISTS(SELECT 1 FROM {inventory_movements} im WHERE im.id_product=p.id_product)) w_mov
FROM {products} p WHERE p.{co}=@c GROUP BY 1,2;
