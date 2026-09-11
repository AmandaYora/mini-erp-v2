-- Modul `stock` (L3): lokasi, saldo, mutasi, transfer, reservasi.
-- Tanpa company_id (ADR-0009). Relasi ke product/variant/branch = primitive ID
-- tanpa FK lintas modul (hanya FK di dalam modul). Uang tak ada di sini;
-- kuantitas DECIMAL(24,12) (DB_SCHEMA §5).

CREATE TABLE stock_locations (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id  BIGINT UNSIGNED NOT NULL,
  code       VARCHAR(30)  NOT NULL,
  name       VARCHAR(150) NOT NULL,
  parent_id  BIGINT UNSIGNED NULL,
  is_system  TINYINT(1)   NOT NULL DEFAULT 0,
  status     VARCHAR(20)  NOT NULL DEFAULT 'active',
  created_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by BIGINT UNSIGNED NULL,
  updated_by BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_stock_locations (branch_id, code),
  CONSTRAINT fk_stock_locations_parent FOREIGN KEY (parent_id) REFERENCES stock_locations (id)
);

CREATE TABLE stock_balances (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id   BIGINT UNSIGNED NOT NULL,
  product_id  BIGINT UNSIGNED NOT NULL,
  variant_id  BIGINT UNSIGNED NOT NULL,
  location_id BIGINT UNSIGNED NOT NULL,
  on_hand     DECIMAL(24,12) NOT NULL DEFAULT 0,
  reserved    DECIMAL(24,12) NOT NULL DEFAULT 0,
  updated_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_stock_balances (branch_id, product_id, variant_id, location_id),
  CONSTRAINT fk_stock_balances_location FOREIGN KEY (location_id) REFERENCES stock_locations (id)
);

CREATE TABLE stock_movements (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id    BIGINT UNSIGNED NOT NULL,
  product_id   BIGINT UNSIGNED NOT NULL,
  variant_id   BIGINT UNSIGNED NOT NULL,
  location_id  BIGINT UNSIGNED NOT NULL,
  direction    VARCHAR(10) NOT NULL,
  movement_type VARCHAR(20) NOT NULL,
  qty_base     DECIMAL(24,12) NOT NULL,
  ref_type     VARCHAR(50) NOT NULL DEFAULT '',
  ref_id       BIGINT UNSIGNED NULL,
  notes        VARCHAR(255) NULL,
  created_by   BIGINT UNSIGNED NULL,
  created_at   DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY ix_stock_movements_lookup (branch_id, product_id, variant_id, location_id),
  CONSTRAINT fk_stock_movements_location FOREIGN KEY (location_id) REFERENCES stock_locations (id)
);

CREATE TABLE stock_transfers (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  number           VARCHAR(60) NOT NULL,
  from_branch_id   BIGINT UNSIGNED NOT NULL,
  to_branch_id     BIGINT UNSIGNED NOT NULL,
  from_location_id BIGINT UNSIGNED NOT NULL,
  to_location_id   BIGINT UNSIGNED NOT NULL,
  status           VARCHAR(20) NOT NULL DEFAULT 'draft',
  notes            VARCHAR(255) NULL,
  dispatched_at    DATETIME(6) NULL,
  received_at      DATETIME(6) NULL,
  created_at       DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at       DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by       BIGINT UNSIGNED NULL,
  updated_by       BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_stock_transfers_number (number),
  CONSTRAINT fk_stock_transfers_from_loc FOREIGN KEY (from_location_id) REFERENCES stock_locations (id),
  CONSTRAINT fk_stock_transfers_to_loc FOREIGN KEY (to_location_id) REFERENCES stock_locations (id)
);

CREATE TABLE stock_transfer_items (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  transfer_id BIGINT UNSIGNED NOT NULL,
  product_id  BIGINT UNSIGNED NOT NULL,
  variant_id  BIGINT UNSIGNED NOT NULL,
  qty_base    DECIMAL(24,12) NOT NULL,
  notes       VARCHAR(255) NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_stock_transfer_items_transfer FOREIGN KEY (transfer_id) REFERENCES stock_transfers (id)
);

CREATE TABLE stock_reservations (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  reservation_key  VARCHAR(100) NOT NULL,
  branch_id        BIGINT UNSIGNED NOT NULL,
  product_id       BIGINT UNSIGNED NOT NULL,
  variant_id       BIGINT UNSIGNED NOT NULL,
  location_id      BIGINT UNSIGNED NULL,
  qty_total        DECIMAL(24,12) NOT NULL,
  qty_remaining    DECIMAL(24,12) NOT NULL,
  status           VARCHAR(20) NOT NULL DEFAULT 'active',
  expires_at       DATETIME(6) NOT NULL,
  created_by       BIGINT UNSIGNED NULL,
  created_at       DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_stock_reservations_key (reservation_key),
  KEY ix_stock_reservations_lookup (branch_id, product_id, variant_id, status)
);
