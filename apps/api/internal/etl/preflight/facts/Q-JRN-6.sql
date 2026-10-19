-- id: Q-JRN-6
-- severity: info
-- rule: none
-- action: jurnal sebelum/sesudah opening-date
SELECT id_branch,SUM(DATE(CONVERT_TZ(journal_date,'+00:00','+07:00'))<@open) prior_n,SUM(DATE(CONVERT_TZ(journal_date,'+00:00','+07:00'))>=@open) cur_n FROM {finance_journal_entries} WHERE {co}=@c AND status IN ('posted','reversed') GROUP BY 1;
