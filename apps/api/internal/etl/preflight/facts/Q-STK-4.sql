-- id: Q-STK-4
-- severity: blocker
-- rule: rows>0
-- action: item SJ order tanpa alokasi lengkap (SJ pengganti tidak punya alokasi: lokasinya dari baris retur)
SELECT dni.id_delivery_note_item,dni.quantity_in_base_uom,COALESCE(SUM(a.quantity_in_base_uom),0) alloc
FROM {delivery_note_items} dni JOIN {delivery_notes} dn ON dn.id_delivery_note=dni.id_delivery_note
LEFT JOIN {stock_issue_allocations} a ON a.id_delivery_note_item=dni.id_delivery_note_item
WHERE dn.id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c) AND dn.archived_at IS NULL AND dn.document_kind = 'order'
GROUP BY dni.id_delivery_note_item HAVING ABS(dni.quantity_in_base_uom-alloc)>0.000000000001;
