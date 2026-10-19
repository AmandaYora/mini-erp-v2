-- id: Q-INC-1
-- severity: warn
-- rule: none
-- action: order inklusif pajak: jurnal GR inklusif kandidat koreksi (P2)
SELECT o.order_kind,COUNT(*) n,SUM(o.tax_amount) tax,
  SUM(EXISTS(SELECT 1 FROM {finance_posting_sources} s WHERE s.{co}=@c AND s.event_key IN ('sales_completed','purchase_received')
            AND ((s.source_type='order' AND s.id_source=o.id_order) OR (s.source_type='goods_receipt' AND s.id_source IN (SELECT id_goods_receipt FROM {goods_receipts} WHERE id_order=o.id_order)))
            AND s.posting_status='posted')) posted_n
FROM {orders} o WHERE o.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND o.taxable=1 AND o.price_includes_tax=1 GROUP BY 1;
