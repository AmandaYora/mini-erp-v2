-- id: Q-ORD-6
-- severity: warn
-- rule: none
-- action: cancelled+bayar: pembayaran dimigrasi cancelled; pending+SJ: naikkan ke confirmed
SELECT os.status_group,COUNT(DISTINCT o.id_order) n,
  SUM(EXISTS(SELECT 1 FROM {payments} p WHERE p.id_order=o.id_order AND p.archived_at IS NULL)) w_pay,
  SUM(EXISTS(SELECT 1 FROM {delivery_notes} dn WHERE dn.id_order=o.id_order AND dn.archived_at IS NULL)) w_sj
FROM {orders} o JOIN {order_status_definitions} os ON os.id_order_status_definition=o.id_current_status
WHERE o.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND os.status_group IN ('cancelled','pending') GROUP BY 1;
