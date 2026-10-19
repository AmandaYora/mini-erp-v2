-- id: Q-DLV-0
-- severity: blocker
-- rule: col:n>0
-- action: SJ dengan document_kind tak dikenal, atau SJ pengganti tanpa retur penjualan (id_sales_return kosong/tidak ada): perbaiki data
SELECT COUNT(*) n FROM {delivery_notes} d WHERE d.id_branch IN (SELECT id_branch FROM {branches} WHERE {co} = @c)
  AND (d.document_kind NOT IN ('order','replacement')
    OR (d.document_kind = 'replacement' AND (d.id_sales_return IS NULL OR d.id_sales_return NOT IN (SELECT id_sales_return FROM {sales_returns} WHERE {co} = @c)))
    OR (d.document_kind = 'order' AND d.id_order IS NULL));
