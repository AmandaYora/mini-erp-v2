-- id: Q-MED-3
-- severity: info
-- rule: none
-- action: bukti bayar tanpa baris media
SELECT COUNT(*) n,SUM(CHAR_LENGTH(proof_file_path)>255) long_key FROM {payments} WHERE proof_file_path IS NOT NULL AND id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c);
