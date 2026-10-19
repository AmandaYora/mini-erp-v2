-- id: Q-ORD-5
-- severity: warn
-- rule: none
-- action: prepaid->cod; due_date = order_date (+payment_term_days)
SELECT payment_terms,order_kind,COUNT(*) n,SUM(payment_terms='net' AND due_date IS NULL) net_no_due,
       SUM(id_related_party IS NULL) no_party
FROM {orders} WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) GROUP BY 1,2;
