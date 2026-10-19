-- id: Q-MNY-2
-- severity: warn
-- rule: none
-- action: sisa Rp<1 setelah pembulatan -> keputusan P3
SELECT o.id_order,o.order_number,o.total_amount,
  COALESCE((SELECT SUM(p.amount) FROM {payments} p WHERE p.id_order=o.id_order AND p.archived_at IS NULL),0)
 +COALESCE((SELECT SUM(pa.allocated_amount) FROM {payment_allocations} pa WHERE pa.id_order=o.id_order AND pa.archived_at IS NULL),0) paid
FROM {orders} o WHERE o.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND o.archived_at IS NULL
HAVING ABS(o.total_amount-paid)>0 AND ABS(o.total_amount-paid)<1.5;
