-- Modul `goodsreceipt` (L5): penerimaan barang atas PO.
-- Tanpa nomor dokumen (kontrak §7.7 tak mendaftar GR). Relasi ke PO/produk =
-- primitive ID tanpa FK lintas modul. Uang tak ada; qty desimal.

CREATE TABLE goods_receipts (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id         BIGINT UNSIGNED NOT NULL,
  purchase_order_id BIGINT UNSIGNED NOT NULL,
  received_at       DATETIME(6) NOT NULL,
  notes             VARCHAR(255) NULL,
  created_at        DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by        BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  KEY ix_goods_receipts_po (purchase_order_id)
);

CREATE TABLE goods_receipt_items (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  receipt_id  BIGINT UNSIGNED NOT NULL,
  product_id  BIGINT UNSIGNED NOT NULL,
  variant_id  BIGINT UNSIGNED NOT NULL,
  location_id BIGINT UNSIGNED NOT NULL,
  uom         VARCHAR(30) NOT NULL,
  uom_factor  DECIMAL(24,12) NOT NULL DEFAULT 1,
  qty         DECIMAL(24,12) NOT NULL,
  qty_base    DECIMAL(24,12) NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_goods_receipt_items_receipt FOREIGN KEY (receipt_id) REFERENCES goods_receipts (id)
);
