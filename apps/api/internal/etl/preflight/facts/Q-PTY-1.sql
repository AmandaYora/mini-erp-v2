-- id: Q-PTY-1
-- severity: blocker
-- rule: cell:party_type=partner
-- action: tipe partner tidak didukung revamp (P7)
SELECT party_type,COUNT(*) n,SUM(archived_at IS NOT NULL) arch FROM {business_parties} WHERE {co}=@c GROUP BY 1;
