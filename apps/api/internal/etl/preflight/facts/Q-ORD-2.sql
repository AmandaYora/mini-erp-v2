-- id: Q-ORD-2
-- severity: blocker
-- rule: rows>0
-- action: akhiri nomor ganda dgn sufiks kode cabang; keputusan pemilik bila di dokumen tercetak
SELECT 'orders' t,order_number num,COUNT(*) n,COUNT(DISTINCT id_branch) nb FROM {orders}
 WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) GROUP BY order_number HAVING n>1
UNION ALL SELECT 'payments',payment_number,COUNT(*),COUNT(DISTINCT id_branch) FROM {payments}
 WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) GROUP BY payment_number HAVING COUNT(*)>1
UNION ALL SELECT 'delivery_notes',sj_number,COUNT(*),COUNT(DISTINCT id_branch) FROM {delivery_notes}
 WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) GROUP BY sj_number HAVING COUNT(*)>1
UNION ALL SELECT 'sales_returns',return_number,COUNT(*),COUNT(DISTINCT id_branch) FROM {sales_returns} WHERE {co}=@c GROUP BY return_number HAVING COUNT(*)>1
UNION ALL SELECT 'purchase_returns',return_number,COUNT(*),COUNT(DISTINCT id_branch) FROM {purchase_returns} WHERE {co}=@c GROUP BY return_number HAVING COUNT(*)>1
UNION ALL SELECT 'stock_transfers',transfer_number,COUNT(*),COUNT(DISTINCT id_from_branch) FROM {stock_transfers} WHERE {co}=@c GROUP BY transfer_number HAVING COUNT(*)>1
UNION ALL SELECT 'journals',journal_number,COUNT(*),COUNT(DISTINCT id_branch) FROM {finance_journal_entries} WHERE {co}=@c GROUP BY journal_number HAVING COUNT(*)>1;
