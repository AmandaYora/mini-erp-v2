-- id: Q-JRN-7
-- severity: warn
-- rule: none
-- action: jurnal GR inklusif terposting (P2)
SELECT COUNT(*) n,SUM(s.source_amount) amt FROM {finance_posting_sources} s JOIN {goods_receipts} gr ON gr.id_goods_receipt=s.id_source JOIN {orders} o ON o.id_order=gr.id_order
WHERE s.{co}=@c AND s.event_key='purchase_received' AND s.posting_status='posted' AND o.taxable=1 AND o.price_includes_tax=1;
