-- Modul `finance` (L7): COA, jurnal, HPP, periode, pajak, biaya usaha.
-- Jurnal adalah satu-satunya sumber laporan keuangan. Uang jurnal integer
-- rupiah; biaya satuan HPP menyimpan 4 desimal (rata-rata berjalan pasti
-- pecahan) dan dibulatkan saat dijurnal. Tanpa company_id (ADR-0009).

CREATE TABLE finance_accounts (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  code       VARCHAR(20)  NOT NULL,
  name       VARCHAR(150) NOT NULL,
  type       VARCHAR(20)  NOT NULL,
  is_cash    TINYINT(1)   NOT NULL DEFAULT 0,
  status     VARCHAR(20)  NOT NULL DEFAULT 'active',
  created_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_at DATETIME(6)  NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_finance_accounts_code (code)
);

CREATE TABLE finance_account_mappings (
  mapping_key VARCHAR(50) NOT NULL,
  account_id  BIGINT UNSIGNED NOT NULL,
  updated_at  DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  updated_by  BIGINT UNSIGNED NULL,
  PRIMARY KEY (mapping_key),
  CONSTRAINT fk_finance_mappings_account FOREIGN KEY (account_id) REFERENCES finance_accounts (id)
);

CREATE TABLE finance_periods (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id  BIGINT UNSIGNED NOT NULL,
  year       INT NOT NULL,
  month      INT NOT NULL,
  status     VARCHAR(20) NOT NULL DEFAULT 'open',
  closed_at  DATETIME(6) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_finance_periods (branch_id, year, month)
);

CREATE TABLE finance_document_sequences (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id     BIGINT UNSIGNED NOT NULL,
  seq_key       VARCHAR(40) NOT NULL,
  current_value BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uq_finance_sequences (branch_id, seq_key)
);

CREATE TABLE finance_journal_entries (
  id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  number          VARCHAR(60) NOT NULL,
  branch_id       BIGINT UNSIGNED NOT NULL,
  entry_date      DATETIME(6) NOT NULL,
  memo            VARCHAR(255) NOT NULL,
  source_doc_type VARCHAR(50) NULL,
  source_doc_id   BIGINT UNSIGNED NULL,
  status          VARCHAR(20) NOT NULL DEFAULT 'posted',
  reversed_by     BIGINT UNSIGNED NULL,
  created_at      DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by      BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_finance_journal_number (number),
  UNIQUE KEY uq_finance_journal_source (source_doc_type, source_doc_id),
  KEY ix_finance_journal_branch_date (branch_id, entry_date)
);

CREATE TABLE finance_journal_lines (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  entry_id   BIGINT UNSIGNED NOT NULL,
  account_id BIGINT UNSIGNED NOT NULL,
  debit      BIGINT NOT NULL DEFAULT 0,
  credit     BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  CONSTRAINT fk_finance_journal_lines_entry FOREIGN KEY (entry_id) REFERENCES finance_journal_entries (id),
  KEY ix_finance_journal_lines_account (account_id)
);

CREATE TABLE finance_tax_periods (
  id         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id  BIGINT UNSIGNED NOT NULL,
  year       INT NOT NULL,
  month      INT NOT NULL,
  status     VARCHAR(20) NOT NULL DEFAULT 'open',
  closed_at  DATETIME(6) NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_finance_tax_periods (branch_id, year, month)
);

CREATE TABLE finance_tax_report_snapshots (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  tax_period_id BIGINT UNSIGNED NOT NULL,
  snapshot_json JSON NOT NULL,
  created_at    DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by    BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  CONSTRAINT fk_finance_tax_snapshots_period FOREIGN KEY (tax_period_id) REFERENCES finance_tax_periods (id)
);

CREATE TABLE business_expenses (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  number            VARCHAR(60) NOT NULL,
  branch_id         BIGINT UNSIGNED NOT NULL,
  expense_account_id BIGINT UNSIGNED NOT NULL,
  pay_account_id    BIGINT UNSIGNED NOT NULL,
  amount            BIGINT NOT NULL,
  expense_date      DATETIME(6) NOT NULL,
  notes             VARCHAR(255) NULL,
  journal_entry_id  BIGINT UNSIGNED NULL,
  status            VARCHAR(20) NOT NULL DEFAULT 'posted',
  created_at        DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by        BIGINT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_business_expenses_number (number),
  CONSTRAINT fk_business_expenses_expense FOREIGN KEY (expense_account_id) REFERENCES finance_accounts (id),
  CONSTRAINT fk_business_expenses_pay FOREIGN KEY (pay_account_id) REFERENCES finance_accounts (id)
);

CREATE TABLE finance_inventory_cost_movements (
  id                BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  branch_id         BIGINT UNSIGNED NOT NULL,
  product_id        BIGINT UNSIGNED NOT NULL,
  variant_id        BIGINT UNSIGNED NOT NULL,
  stock_movement_id BIGINT UNSIGNED NOT NULL,
  direction         VARCHAR(10) NOT NULL,
  qty_base          DECIMAL(24,12) NOT NULL,
  unit_cost         DECIMAL(18,4) NULL,
  total_cost        DECIMAL(18,2) NULL,
  is_estimated      TINYINT(1) NOT NULL DEFAULT 0,
  created_at        DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  UNIQUE KEY uq_finance_cost_movement (stock_movement_id),
  KEY ix_finance_cost_position (branch_id, product_id, variant_id)
);
