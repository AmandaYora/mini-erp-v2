-- id: Q-JRN-9
-- severity: warn
-- rule: none
-- action: dokumen terposting sebagian
SELECT o.id_order,o.order_number FROM {orders} o JOIN {finance_posting_sources} s ON s.source_type='order' AND s.id_source=o.id_order AND s.event_key='sales_completed' AND s.posting_status='posted'
WHERE o.order_kind='sales' AND EXISTS(SELECT 1 FROM {finance_posting_sources} c WHERE c.{co}=@c AND c.event_key='stock_issue_cogs' AND c.posting_status<>'posted'
  AND c.id_source IN (SELECT id_inventory_movement FROM {inventory_movements} WHERE (reference_type='order' AND id_reference=o.id_order) OR (reference_type='delivery_note' AND id_reference IN (SELECT id_delivery_note FROM {delivery_notes} WHERE id_order=o.id_order))));
