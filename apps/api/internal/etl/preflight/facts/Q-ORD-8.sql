-- id: Q-ORD-8
-- severity: warn
-- rule: none
-- action: baris campuran pajak / selisih total
SELECT o.id_order,o.order_number,COUNT(DISTINCT oi.taxable) mixed,
       ROUND(o.total_amount-SUM(oi.line_total_after_tax),2) diff_total,
       SUM(oi.quantity_in_base_uom<>oi.quantity*oi.uom_to_base_factor) bad_base
FROM {orders} o JOIN {order_items} oi ON oi.id_order=o.id_order
WHERE o.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c)
GROUP BY o.id_order HAVING mixed>1 OR ABS(diff_total)>0.009 OR bad_base>0;
