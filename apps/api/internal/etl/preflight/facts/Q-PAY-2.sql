-- id: Q-PAY-2
-- severity: blocker
-- rule: rows>0
-- action: jumlah alokasi tidak sama dengan pembayaran
SELECT p.id_payment,p.payment_number,p.amount,SUM(a.allocated_amount) alloc FROM {payments} p JOIN {payment_allocations} a ON a.id_payment=p.id_payment AND a.archived_at IS NULL
WHERE p.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND p.archived_at IS NULL GROUP BY p.id_payment HAVING ABS(p.amount-alloc)>0.009;
