-- id: Q-JRN-2
-- severity: blocker
-- rule: absdiff:d,c>0.009
-- action: jurnal tidak seimbang
SELECT e.id_finance_journal_entry,e.journal_number,e.event_key,SUM(l.debit_amount) d,SUM(l.credit_amount) c
FROM {finance_journal_entries} e JOIN {finance_journal_lines} l ON l.id_finance_journal_entry=e.id_finance_journal_entry
WHERE e.{co}=@c AND e.status IN ('posted','reversed') GROUP BY e.id_finance_journal_entry HAVING ABS(d-c)>0.0001;
