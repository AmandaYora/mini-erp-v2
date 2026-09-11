-- Tahap B (D1): nilai retur mengikuti nilai asli — diskon proporsional ikut,
-- PPN ikut dibalik. Kolom `total` tetap berarti nilai yang dikembalikan ke
-- pihak lawan. Invarian: subtotal = jumlah tax base SETELAH diskon sehingga
-- total = subtotal + tax_total untuk ketiga tipe pajak. discount_total hanya
-- informasional (ditampilkan), jangan dikurangkan lagi di jurnal.

ALTER TABLE sales_returns
  ADD COLUMN subtotal       BIGINT       NOT NULL DEFAULT 0 AFTER total,
  ADD COLUMN discount_total BIGINT       NOT NULL DEFAULT 0 AFTER subtotal,
  ADD COLUMN tax_total      BIGINT       NOT NULL DEFAULT 0 AFTER discount_total,
  ADD COLUMN tax_type       VARCHAR(20)  NOT NULL DEFAULT 'none' AFTER tax_total,
  ADD COLUMN tax_rate       DECIMAL(5,2) NOT NULL DEFAULT 0 AFTER tax_type;

ALTER TABLE sales_return_items
  ADD COLUMN discount_pct     DECIMAL(5,2) NOT NULL DEFAULT 0 AFTER unit_price,
  ADD COLUMN discount_nominal BIGINT       NOT NULL DEFAULT 0 AFTER discount_pct,
  ADD COLUMN tax_base         BIGINT       NOT NULL DEFAULT 0 AFTER discount_nominal,
  ADD COLUMN tax_amount       BIGINT       NOT NULL DEFAULT 0 AFTER tax_base;

ALTER TABLE purchase_returns
  ADD COLUMN subtotal       BIGINT       NOT NULL DEFAULT 0 AFTER total,
  ADD COLUMN discount_total BIGINT       NOT NULL DEFAULT 0 AFTER subtotal,
  ADD COLUMN tax_total      BIGINT       NOT NULL DEFAULT 0 AFTER discount_total,
  ADD COLUMN tax_type       VARCHAR(20)  NOT NULL DEFAULT 'none' AFTER tax_total,
  ADD COLUMN tax_rate       DECIMAL(5,2) NOT NULL DEFAULT 0 AFTER tax_type;

ALTER TABLE purchase_return_items
  ADD COLUMN discount_pct     DECIMAL(5,2) NOT NULL DEFAULT 0 AFTER unit_price,
  ADD COLUMN discount_nominal BIGINT       NOT NULL DEFAULT 0 AFTER discount_pct,
  ADD COLUMN tax_base         BIGINT       NOT NULL DEFAULT 0 AFTER discount_nominal,
  ADD COLUMN tax_amount       BIGINT       NOT NULL DEFAULT 0 AFTER tax_base;
