-- SL-09: siapa/kapan menyetujui kredit sebuah order jual (sebelumnya hanya ada di
-- audit_logs). Kolom nullable: order lama / tanpa persetujuan kredit tetap NULL.
-- Idempoten terhadap ETL: kolom opsional di spec load (credit_approved_*).
ALTER TABLE `sales_orders`
  ADD COLUMN `credit_approved_by` bigint unsigned DEFAULT NULL,
  ADD COLUMN `credit_approved_at` datetime(6) DEFAULT NULL;
