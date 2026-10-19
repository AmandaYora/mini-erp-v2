-- id: Q-JRN-3
-- severity: info
-- rule: none
-- action: sebaran jurnal
SELECT status,posting_type,event_key,YEAR(journal_date) yr,COUNT(*) n FROM {finance_journal_entries} WHERE {co}=@c GROUP BY 1,2,3,4 ORDER BY yr,event_key;
