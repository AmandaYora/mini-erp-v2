-- id: Q-PAY-3
-- severity: warn
-- rule: none
-- action: metode cek tetap check; bukti bayar jadi baris media
SELECT payment_method,financial_side,COUNT(*) n,SUM(proof_file_path IS NOT NULL) w_proof,SUM(amount_tendered IS NOT NULL) w_tend,SUM(archived_at IS NOT NULL) arch
FROM {payments} WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) GROUP BY 1,2;
