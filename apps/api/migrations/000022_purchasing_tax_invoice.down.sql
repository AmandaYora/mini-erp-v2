ALTER TABLE `purchase_order_items` DROP COLUMN `tax_rate`;

ALTER TABLE `purchase_orders`
  DROP COLUMN `tax_invoice_date`,
  DROP COLUMN `tax_invoice_number`;
