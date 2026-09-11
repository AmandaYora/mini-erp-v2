ALTER TABLE purchase_orders
  DROP COLUMN supplier_invoice_date,
  DROP COLUMN supplier_invoice_number;

ALTER TABLE sales_orders
  DROP COLUMN tax_invoice_date,
  DROP COLUMN tax_invoice_number;
