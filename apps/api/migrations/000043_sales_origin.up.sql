-- Order dari surat jalan (ADR-0022): origin='delivery' menandai order yang
-- lahir dari SJ Langsung (barangnya sudah terkirim; order dibuat belakangan
-- dari SJ). Berdefault 'standard' sehingga data lama dan hasil ETL tidak berubah.
ALTER TABLE `sales_orders`
  ADD COLUMN `origin` varchar(20) NOT NULL DEFAULT 'standard' AFTER `channel`;
