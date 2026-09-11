-- Modul `purchasing` (L4): purchase order ke supplier.
-- Baris menyimpan snapshot nama/harga/satuan (kontrak §7.4) — nota tak bergeser.
-- Uang integer rupiah (ADR-0005); qty desimal (DB_SCHEMA §5). Tanpa company_id.

CREATE TABLE purchase_orders (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  number         VARCHAR(60) NOT NULL,
  branch_id      BIGINT UNSIGNED NOT NULL,
  party_id       BIGINT UNSIGNED NOT NULL,
  order_date     DATETIME(6) NOT NULL,
  due_date       DATETIME(6) NULL,
  payment_terms  VARCHAR(20) NOT NULL DEFAULT 'net',
  tax_type       VARCHAR(20) NOT NULL DEFAULT 'none',
  tax_rate       DECIMAL(5,2) NOT NULL DEFAULT 0,
  subtotal       BIGINT NOT NULL DEFAULT 0,
  discount_total BIGINT NOT NULL DEFAULT 0,
  tax_total      BIGINT NOT NULL DEFAULT 0,
  grand_total    BIGINT NOT NULL DEFAULT 0,
  status         VARCHAR(20) NOT NULL DEFAULT 'draft',
  notes          VARCHAR(255) NULL,
  created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by     BIGINT UNSIGNED NULL,
  updated_by     BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_purchase_orders_number (number),
  KEY ix_purchase_orders_branch (branch_id, status)
);

CREATE TABLE purchase_order_items (
  id                 BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  order_id           BIGINT UNSIGNED NOT NULL,
  product_id         BIGINT UNSIGNED NOT NULL,
  variant_id         BIGINT UNSIGNED NOT NULL,
  product_code       VARCHAR(50)  NOT NULL,
  product_name       VARCHAR(255) NOT NULL,
  uom                VARCHAR(30)  NOT NULL,
  uom_factor         DECIMAL(24,12) NOT NULL DEFAULT 1,
  qty                DECIMAL(24,12) NOT NULL,
  qty_base           DECIMAL(24,12) NOT NULL,
  unit_price         BIGINT NOT NULL,
  discount_pct       DECIMAL(5,2) NOT NULL DEFAULT 0,
  discount_nominal   BIGINT NOT NULL DEFAULT 0,
  line_total         BIGINT NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_purchase_order_items_order FOREIGN KEY (order_id) REFERENCES purchase_orders (id)
);
