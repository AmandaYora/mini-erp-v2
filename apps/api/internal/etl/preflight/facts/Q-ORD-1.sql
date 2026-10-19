-- id: Q-ORD-1
-- severity: blocker
-- rule: notin:status_group=pending,active,completed,cancelled
-- action: peta lewat status_group; grup tak dikenal -> hentikan, minta keputusan
SELECT d.code,d.label,d.status_group,COUNT(o.id_order) n
FROM {order_status_definitions} d LEFT JOIN {orders} o ON o.id_current_status=d.id_order_status_definition
WHERE d.{co}=@c GROUP BY d.id_order_status_definition;
