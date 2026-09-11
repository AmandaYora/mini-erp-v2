-- Modul `delivery` (L5): surat jalan atas SO. Stok berkurang di sini
-- (kontrak §7.12), bukan saat SO dikonfirmasi. Nomor SJ via BranchClient.

CREATE TABLE delivery_notes (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  number         VARCHAR(60) NOT NULL,
  branch_id      BIGINT UNSIGNED NOT NULL,
  sales_order_id BIGINT UNSIGNED NOT NULL,
  delivery_date  DATETIME(6) NOT NULL,
  status         VARCHAR(20) NOT NULL DEFAULT 'draft',
  notes          VARCHAR(255) NULL,
  created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by     BIGINT UNSIGNED NULL,
  updated_by     BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_delivery_notes_number (number),
  KEY ix_delivery_notes_so (sales_order_id)
);

CREATE TABLE delivery_note_items (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  delivery_id BIGINT UNSIGNED NOT NULL,
  product_id  BIGINT UNSIGNED NOT NULL,
  variant_id  BIGINT UNSIGNED NOT NULL,
  location_id BIGINT UNSIGNED NOT NULL,
  uom         VARCHAR(30) NOT NULL,
  uom_factor  DECIMAL(24,12) NOT NULL DEFAULT 1,
  qty         DECIMAL(24,12) NOT NULL,
  qty_base    DECIMAL(24,12) NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_delivery_note_items_delivery FOREIGN KEY (delivery_id) REFERENCES delivery_notes (id)
);
