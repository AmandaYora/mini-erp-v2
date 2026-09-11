-- Tahap D2: penyelesaian retur sebagai catatan (memo-level, seperti legacy) —
-- bagaimana nilai retur diselesaikan di dunia nyata: tagih tunai, potong
-- piutang/hutang, refund, atau kredit pelanggan. Aliran uangnya sendiri tetap
-- lewat modul payment yang sudah ada; tabel ini mencatat keputusannya supaya
-- sisa yang belum diselesaikan terlihat (total − Σ settlement).
-- purchase side simetris (supplier): collect_payment = bayar ke supplier,
-- refund = terima kembali, dsb. — maknanya mengikuti arah retur.

CREATE TABLE sales_return_settlements (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  return_id        BIGINT UNSIGNED NOT NULL,
  settlement_type  VARCHAR(30) NOT NULL,
  settlement_date  DATETIME(6) NOT NULL,
  amount           BIGINT NOT NULL DEFAULT 0,
  payment_method   VARCHAR(30) NULL,
  reference_number VARCHAR(100) NULL,
  notes            VARCHAR(255) NULL,
  created_at       DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by       BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  KEY ix_sales_return_settlements_return (return_id),
  CONSTRAINT fk_sales_return_settlements_return FOREIGN KEY (return_id) REFERENCES sales_returns (id)
);

CREATE TABLE purchase_return_settlements (
  id               BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  return_id        BIGINT UNSIGNED NOT NULL,
  settlement_type  VARCHAR(30) NOT NULL,
  settlement_date  DATETIME(6) NOT NULL,
  amount           BIGINT NOT NULL DEFAULT 0,
  payment_method   VARCHAR(30) NULL,
  reference_number VARCHAR(100) NULL,
  notes            VARCHAR(255) NULL,
  created_at       DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by       BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  KEY ix_purchase_return_settlements_return (return_id),
  CONSTRAINT fk_purchase_return_settlements_return FOREIGN KEY (return_id) REFERENCES purchase_returns (id)
);
