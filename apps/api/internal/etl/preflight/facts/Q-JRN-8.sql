-- id: Q-JRN-8
-- severity: info
-- rule: none
-- action: antrean posting legacy
SELECT event_key,posting_status,YEAR(source_date) yr,COUNT(*) n FROM {finance_posting_sources} WHERE {co}=@c GROUP BY 1,2,3 ORDER BY yr,event_key;
