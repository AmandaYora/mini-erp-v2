-- id: Q-JRN-4
-- severity: blocker
-- rule: colmax:mx_no>60
-- action: nomor jurnal >60 karakter
SELECT MAX(CHAR_LENGTH(journal_number)) mx_no,SUM(CHAR_LENGTH(description)>255) desc_gt255 FROM {finance_journal_entries} WHERE {co}=@c;
