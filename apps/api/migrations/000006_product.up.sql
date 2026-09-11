-- Modul `product` (L2): katalog, kategori, varian.
-- UOM sebagai kolom tetap (base/purchase/sales + 2 faktor) — cukup untuk
-- kebutuhan terbukti (DB_SCHEMA §6). Tanpa kolom `uom` warisan (KI-60).
-- Harga sebagai integer rupiah (ADR-0005).

CREATE TABLE product_categories (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code       VARCHAR(50)  NOT NULL,
  name       VARCHAR(150) NOT NULL,
  parent_id  BIGINT UNSIGNED NULL,
  status     VARCHAR(20)  NOT NULL DEFAULT 'active',
  created_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by BIGINT UNSIGNED NULL,
  updated_by BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_product_categories_code (code),
  CONSTRAINT fk_product_categories_parent FOREIGN KEY (parent_id) REFERENCES product_categories (id)
);

CREATE TABLE products (
  id                       BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code                     VARCHAR(50)  NOT NULL,
  name                     VARCHAR(255) NOT NULL,
  category_id              BIGINT UNSIGNED NULL,
  type                     VARCHAR(20)  NOT NULL DEFAULT 'barang',
  stock_tracked            TINYINT(1)   NOT NULL DEFAULT 1,
  base_uom                 VARCHAR(30)  NOT NULL,
  purchase_uom             VARCHAR(30)  NOT NULL,
  sales_uom                VARCHAR(30)  NOT NULL,
  purchase_to_base_factor  DECIMAL(24,12) NOT NULL DEFAULT 1,
  sales_to_base_factor     DECIMAL(24,12) NOT NULL DEFAULT 1,
  purchase_price           BIGINT NOT NULL DEFAULT 0,
  selling_price            BIGINT NOT NULL DEFAULT 0,
  min_selling_price        BIGINT NOT NULL DEFAULT 0,
  min_stock_qty            DECIMAL(24,12) NOT NULL DEFAULT 0,
  status                   VARCHAR(20)  NOT NULL DEFAULT 'active',
  created_at               DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at               DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by               BIGINT UNSIGNED NULL,
  updated_by               BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_products_code (code),
  CONSTRAINT fk_products_category FOREIGN KEY (category_id) REFERENCES product_categories (id)
);

CREATE TABLE product_variants (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  product_id BIGINT UNSIGNED NOT NULL,
  code       VARCHAR(50)  NOT NULL,
  name       VARCHAR(150) NOT NULL,
  barcode    VARCHAR(100) NULL,
  is_default TINYINT(1)   NOT NULL DEFAULT 0,
  status     VARCHAR(20)  NOT NULL DEFAULT 'active',
  created_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_product_variants (product_id, code),
  UNIQUE KEY uq_product_variants_barcode (barcode),
  CONSTRAINT fk_product_variants_product FOREIGN KEY (product_id) REFERENCES products (id)
);
