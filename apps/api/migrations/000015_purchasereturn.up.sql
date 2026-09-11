-- Modul `purchasereturn` (L6): retur ke supplier atas PO/penerimaan.
-- Nilai baris = qty × harga satuan snapshot PO. Penyelesaian via payments.

CREATE TABLE purchase_returns (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  number            VARCHAR(60) NOT NULL,
  branch_id         BIGINT UNSIGNED NOT NULL,
  purchase_order_id BIGINT UNSIGNED NOT NULL,
  return_date       DATETIME(6) NOT NULL,
  total             BIGINT NOT NULL DEFAULT 0,
  status            VARCHAR(20) NOT NULL DEFAULT 'draft',
  notes             VARCHAR(255) NULL,
  created_at        DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at        DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by        BIGINT UNSIGNED NULL,
  updated_by        BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_purchase_returns_number (number),
  KEY ix_purchase_returns_po (purchase_order_id)
);

CREATE TABLE purchase_return_items (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  return_id   BIGINT UNSIGNED NOT NULL,
  product_id  BIGINT UNSIGNED NOT NULL,
  variant_id  BIGINT UNSIGNED NOT NULL,
  location_id BIGINT UNSIGNED NOT NULL,
  uom         VARCHAR(30) NOT NULL,
  uom_factor  DECIMAL(24,12) NOT NULL DEFAULT 1,
  qty         DECIMAL(24,12) NOT NULL,
  qty_base    DECIMAL(24,12) NOT NULL,
  unit_price  BIGINT NOT NULL,
  line_total  BIGINT NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_purchase_return_items_return FOREIGN KEY (return_id) REFERENCES purchase_returns (id)
);
