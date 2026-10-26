-- Rekening perusahaan (ADR-0026): identitas rekening bank (bank, nomor, atas
-- nama) yang tertaut 1:1 ke satu akun kas/bank (`finance_accounts.is_cash`).
-- Saldo tetap milik akun buku besar; tabel ini tidak menyimpan saldo (itu yang
-- membuat `finance_cash_accounts` legacy tidak dibawa). Pembayaran transfer/cek
-- yang memilih rekening dijurnal ke akun rekening itu.
CREATE TABLE `finance_bank_accounts` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `account_id` BIGINT UNSIGNED NOT NULL,
  `bank_name` VARCHAR(50) NOT NULL,
  `account_number` VARCHAR(50) NOT NULL,
  `account_holder` VARCHAR(150) NOT NULL,
  `show_on_documents` TINYINT(1) NOT NULL DEFAULT '1',
  `is_default` TINYINT(1) NOT NULL DEFAULT '0',
  `sort_order` INT NOT NULL DEFAULT '0',
  `status` VARCHAR(20) NOT NULL DEFAULT 'active',
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` BIGINT UNSIGNED NULL,
  `updated_by` BIGINT UNSIGNED NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_bank_accounts_account` (`account_id`),
  UNIQUE KEY `uq_finance_bank_accounts_number` (`bank_name`, `account_number`),
  CONSTRAINT `fk_finance_bank_accounts_account` FOREIGN KEY (`account_id`) REFERENCES `finance_accounts` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
