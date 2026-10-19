-- id: Q-PAY-4
-- severity: warn
-- rule: none
-- action: pembayaran pada order terarsip: ignored(order_archived)
SELECT COUNT(*) n,SUM(p.amount) amt FROM {payments} p JOIN {orders} o ON o.id_order=p.id_order
WHERE p.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND p.archived_at IS NULL AND o.archived_at IS NOT NULL;
