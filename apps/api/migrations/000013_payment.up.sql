-- Modul `payment` (L5): pembayaran + alokasi ke PO/SO.
-- Satu pembayaran milik satu pihak; alokasi menunjuk dokumen lintas modul
-- sebagai (order_type, order_id) primitif. Uang integer rupiah.

CREATE TABLE payments (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  number      VARCHAR(60) NOT NULL,
  branch_id   BIGINT UNSIGNED NOT NULL,
  party_id    BIGINT UNSIGNED NOT NULL,
  party_type  VARCHAR(20) NOT NULL,
  direction   VARCHAR(10) NOT NULL,
  amount      BIGINT NOT NULL,
  method      VARCHAR(30) NOT NULL DEFAULT 'cash',
  paid_at     DATETIME(6) NOT NULL,
  notes       VARCHAR(255) NULL,
  status      VARCHAR(20) NOT NULL DEFAULT 'active',
  created_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by  BIGINT UNSIGNED NULL,
  updated_by  BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_payments_number (number),
  KEY ix_payments_party (party_id, status)
);

CREATE TABLE payment_allocations (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  payment_id  BIGINT UNSIGNED NOT NULL,
  order_type  VARCHAR(20) NOT NULL,
  order_id    BIGINT UNSIGNED NOT NULL,
  amount      BIGINT NOT NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_payment_allocations_payment FOREIGN KEY (payment_id) REFERENCES payments (id),
  KEY ix_payment_allocations_order (order_type, order_id)
);
