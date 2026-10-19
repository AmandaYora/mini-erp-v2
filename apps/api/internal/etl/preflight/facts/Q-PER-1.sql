-- id: Q-PER-1
-- severity: info
-- rule: none
-- action: periode tertutup
SELECT status,MIN(date_from),MAX(date_to),COUNT(*) FROM {finance_periods} WHERE {co}=@c GROUP BY 1;
