-- id: Q-JRN-1
-- severity: warn
-- rule: none
-- action: jurnal tanpa cabang -> cabang pusat
SELECT event_key,status,COUNT(*) n,SUM(YEAR(journal_date)>=YEAR(@open)) cur FROM {finance_journal_entries} WHERE {co}=@c AND id_branch IS NULL GROUP BY 1,2;
