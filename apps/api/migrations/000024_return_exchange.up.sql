-- Tahap D1+D3: retur tukar + surat jalan pengganti. Mode tukar membawa baris
-- pengganti dalam dokumen retur yang sama (legacy return_mode = exchange);
-- penyelesaiannya dicatat di sales_return_settlements (D2 memakai tabelnya
-- sendiri). SJ pengganti = delivery_notes biasa ber-kind replacement yang
-- menunjuk retur asalnya; stok keluar saat SJ pengganti dikonfirmasi, bukan
-- saat retur dibuat.

ALTER TABLE sales_returns
  ADD COLUMN return_mode VARCHAR(20) NOT NULL DEFAULT 'return_only' AFTER tax_rate,
  ADD COLUMN replacement_delivery_status VARCHAR(20) NOT NULL DEFAULT 'not_required' AFTER return_mode;

CREATE TABLE sales_return_replacement_items (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  return_id   BIGINT UNSIGNED NOT NULL,
  product_id  BIGINT UNSIGNED NOT NULL,
  variant_id  BIGINT UNSIGNED NOT NULL,
  location_id BIGINT UNSIGNED NOT NULL,
  uom         VARCHAR(30) NOT NULL,
  uom_factor  DECIMAL(24,12) NOT NULL DEFAULT 1,
  qty         DECIMAL(24,12) NOT NULL,
  qty_base    DECIMAL(24,12) NOT NULL,
  unit_price  BIGINT NOT NULL DEFAULT 0,
  line_total  BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY ix_sales_return_replacement_items_return (return_id),
  CONSTRAINT fk_sales_return_replacement_items_return FOREIGN KEY (return_id) REFERENCES sales_returns (id)
);

ALTER TABLE delivery_notes
  ADD COLUMN document_kind VARCHAR(20) NOT NULL DEFAULT 'order' AFTER status,
  ADD COLUMN sales_return_id BIGINT UNSIGNED NULL AFTER document_kind,
  ADD KEY ix_delivery_notes_return (sales_return_id);
