-- Tahap C3: nomor faktur pajak di dokumen (dibutuhkan kertas kerja SPT dan
-- cek kelengkapan saat tutup periode — meniru finance-reporting.service.ts:388
-- dan finance-close.service.ts:112 legacy).

ALTER TABLE sales_orders
  ADD COLUMN tax_invoice_number VARCHAR(50) NULL AFTER tax_rate,
  ADD COLUMN tax_invoice_date   DATETIME(6) NULL AFTER tax_invoice_number;

ALTER TABLE purchase_orders
  ADD COLUMN supplier_invoice_number VARCHAR(50) NULL AFTER tax_rate,
  ADD COLUMN supplier_invoice_date   DATETIME(6) NULL AFTER supplier_invoice_number;
