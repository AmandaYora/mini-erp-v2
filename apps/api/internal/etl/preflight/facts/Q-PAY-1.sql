-- id: Q-PAY-1
-- severity: blocker
-- rule: col:orphan>0
-- action: pembayaran tanpa order dan tanpa alokasi
SELECT SUM(p.id_order IS NOT NULL AND NOT EXISTS(SELECT 1 FROM {payment_allocations} a WHERE a.id_payment=p.id_payment AND a.archived_at IS NULL)) need_synth,
       SUM(p.id_order IS NULL AND NOT EXISTS(SELECT 1 FROM {payment_allocations} a WHERE a.id_payment=p.id_payment AND a.archived_at IS NULL)) orphan,
       SUM(p.id_order IS NOT NULL AND EXISTS(SELECT 1 FROM {payment_allocations} a WHERE a.id_payment=p.id_payment AND a.archived_at IS NULL)) both_forms
FROM {payments} p WHERE p.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND p.archived_at IS NULL;
