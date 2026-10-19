-- id: Q-ORD-9
-- severity: info
-- rule: none
-- action: sebaran per tahun
SELECT order_kind,YEAR(order_date) yr,COUNT(*) n,SUM(total_amount) total FROM {orders}
WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) GROUP BY 1,2 ORDER BY 1,2;
