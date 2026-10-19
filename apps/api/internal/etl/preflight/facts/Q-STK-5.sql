-- id: Q-STK-5
-- severity: blocker
-- rule: first>0
-- action: reservasi aktif saat freeze
SELECT COUNT(*) FROM {stock_reservations} WHERE status='active' AND id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c);
