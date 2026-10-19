-- id: Q-AUD-1
-- severity: info
-- rule: none
-- action: ukuran audit log
SELECT YEAR(happened_at) yr,COUNT(*) n FROM {audit_logs} WHERE {co}=@c GROUP BY 1;
