-- Modul `party` (L2): customer/supplier + alamat kirim + tipe member & harga.
-- Satu tabel parties dibedakan party_type. Kode unik per (code, party_type) dan
-- dicadangkan lintas arsip (tak ada hapus permanen — restore selalu bisa, KI-71).
-- Tanpa company_id (ADR-0009).

CREATE TABLE member_types (
  id                        BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code                      VARCHAR(50)  NOT NULL,
  name                      VARCHAR(120) NOT NULL,
  description_text          TEXT NULL,
  price_basis               VARCHAR(30)  NOT NULL DEFAULT 'selling_price',
  price_adjustment_direction VARCHAR(20) NOT NULL DEFAULT 'minus',
  price_adjustment_type     VARCHAR(20)  NOT NULL DEFAULT 'percent',
  price_adjustment_value    DECIMAL(18,4) NOT NULL DEFAULT 0,
  rounding_mode             VARCHAR(20)  NOT NULL DEFAULT 'none',
  rounding_increment        DECIMAL(18,2) NULL,
  status                    VARCHAR(20)  NOT NULL DEFAULT 'active',
  archived_at               DATETIME(6) NULL,
  created_at                DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at                DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by                BIGINT UNSIGNED NULL,
  updated_by                BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_member_types_code (code)
);

CREATE TABLE parties (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code           VARCHAR(50)  NOT NULL,
  party_type     VARCHAR(20)  NOT NULL,
  name           VARCHAR(255) NOT NULL,
  phone          VARCHAR(30)  NULL,
  email          VARCHAR(255) NULL,
  address_text   TEXT NULL,
  notes          TEXT NULL,
  member_type_id BIGINT UNSIGNED NULL,
  status         VARCHAR(20)  NOT NULL DEFAULT 'active',
  archived_at    DATETIME(6) NULL,
  created_at     DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at     DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by     BIGINT UNSIGNED NULL,
  updated_by     BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_parties_code_type (code, party_type),
  CONSTRAINT fk_parties_member_type FOREIGN KEY (member_type_id) REFERENCES member_types (id)
);

CREATE TABLE party_delivery_addresses (
  id             BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  party_id       BIGINT UNSIGNED NOT NULL,
  label          VARCHAR(120) NULL,
  recipient_name VARCHAR(150) NULL,
  phone          VARCHAR(30)  NULL,
  address_text   TEXT NOT NULL,
  is_primary     TINYINT(1)   NOT NULL DEFAULT 0,
  sort_order     INT NOT NULL DEFAULT 0,
  status         VARCHAR(20)  NOT NULL DEFAULT 'active',
  archived_at    DATETIME(6) NULL,
  created_at     DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at     DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  CONSTRAINT fk_party_addresses_party FOREIGN KEY (party_id) REFERENCES parties (id)
);
