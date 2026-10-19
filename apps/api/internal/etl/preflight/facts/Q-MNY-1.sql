-- id: Q-MNY-1
-- severity: warn
-- rule: none
-- action: setiap nilai berpecahan masuk adjustments.csv
SELECT 'orders' t,SUM(total_amount<>FLOOR(total_amount)) frac,SUM(total_amount<>FLOOR(total_amount) AND archived_at IS NOT NULL) frac_archived,
       SUM(total_amount<>FLOOR(total_amount) AND (discount_amount<>0 OR subtotal_before_tax<>subtotal_after_tax)) frac_tax_or_disc
FROM {orders} WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c)
UNION ALL SELECT 'order_items',SUM(unit_price<>FLOOR(unit_price) OR unit_price_before_discount<>FLOOR(unit_price_before_discount)),SUM(line_total<>FLOOR(line_total)),NULL
 FROM {order_items} WHERE id_order IN (SELECT id_order FROM {orders} WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c))
UNION ALL SELECT 'payments',SUM(amount<>FLOOR(amount)),NULL,NULL FROM {payments} WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c)
UNION ALL SELECT 'journal_lines',SUM(debit_amount<>FLOOR(debit_amount) OR credit_amount<>FLOOR(credit_amount)),NULL,NULL FROM {finance_journal_lines} WHERE {co}=@c;
