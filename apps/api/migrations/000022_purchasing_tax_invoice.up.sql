-- PO-03: faktur pajak supplier pada PO dan tarif pajak per baris.
ALTER TABLE `purchase_orders`
  ADD COLUMN `tax_invoice_number` varchar(100) DEFAULT NULL AFTER `supplier_invoice_date`,
  ADD COLUMN `tax_invoice_date` datetime(6) DEFAULT NULL AFTER `tax_invoice_number`;

ALTER TABLE `purchase_order_items`
  ADD COLUMN `tax_rate` decimal(9,4) NOT NULL DEFAULT '0.0000' AFTER `taxable`;
