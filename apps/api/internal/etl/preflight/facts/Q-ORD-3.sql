-- id: Q-ORD-3
-- severity: blocker
-- rule: colmax:mx_no>60;colmax:mx_sinv>50;colmax:mx_tinv>50
-- action: panjang melebihi kolom target; catatan >255 dipotong dan dicatat
SELECT MAX(CHAR_LENGTH(order_number)) mx_no, MAX(CHAR_LENGTH(notes)) mx_notes,
       SUM(CHAR_LENGTH(notes)>255) notes_gt255, MAX(CHAR_LENGTH(supplier_invoice_number)) mx_sinv,
       MAX(CHAR_LENGTH(tax_invoice_number)) mx_tinv, SUM(CHAR_LENGTH(ship_to_address_snapshot)>255) ship_gt255
FROM {orders} WHERE id_branch IN (SELECT id_branch FROM {branches} WHERE {co}=@c);
