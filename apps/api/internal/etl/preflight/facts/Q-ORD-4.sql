-- id: Q-ORD-4
-- severity: blocker
-- rule: rows>0
-- action: tentukan status per kasus; default cancelled + temuan
SELECT o.id_order,o.order_number FROM {orders} o
WHERE o.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND o.archived_at IS NOT NULL
 AND (EXISTS(SELECT 1 FROM {inventory_movements} im WHERE im.reference_type='order' AND im.id_reference=o.id_order)
   OR EXISTS(SELECT 1 FROM {delivery_notes} dn WHERE dn.id_order=o.id_order)
   OR EXISTS(SELECT 1 FROM {payments} p WHERE p.id_order=o.id_order)
   OR EXISTS(SELECT 1 FROM {payment_allocations} pa WHERE pa.id_order=o.id_order));
