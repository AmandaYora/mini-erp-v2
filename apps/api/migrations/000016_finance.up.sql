-- Baseline modul finance (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `finance_accounts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(50) NOT NULL,
  `name` varchar(255) NOT NULL,
  `type` varchar(20) NOT NULL,
  `is_cash` tinyint(1) NOT NULL DEFAULT '0',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_accounts_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `business_expenses` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `number` varchar(100) NOT NULL,
  `branch_id` bigint unsigned NOT NULL,
  `expense_account_id` bigint unsigned NOT NULL,
  `pay_account_id` bigint unsigned NOT NULL,
  `amount` bigint NOT NULL,
  `expense_date` datetime(6) NOT NULL,
  `notes` text,
  `journal_entry_id` bigint unsigned DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'posted',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `cancel_reason` varchar(255) DEFAULT NULL,
  `cancelled_at` datetime(6) DEFAULT NULL,
  `cancelled_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_business_expenses_number` (`number`),
  KEY `fk_business_expenses_expense` (`expense_account_id`),
  KEY `fk_business_expenses_pay` (`pay_account_id`),
  KEY `ix_business_expenses_branch_id` (`branch_id`,`id`),
  KEY `ix_business_expenses_date` (`branch_id`,`expense_date`),
  CONSTRAINT `fk_business_expenses_expense` FOREIGN KEY (`expense_account_id`) REFERENCES `finance_accounts` (`id`),
  CONSTRAINT `fk_business_expenses_pay` FOREIGN KEY (`pay_account_id`) REFERENCES `finance_accounts` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_account_daily` (
  `branch_id` bigint unsigned NOT NULL,
  `account_id` bigint unsigned NOT NULL,
  `entry_date` date NOT NULL,
  `scope` tinyint NOT NULL DEFAULT '0',
  `debit` bigint NOT NULL DEFAULT '0',
  `credit` bigint NOT NULL DEFAULT '0',
  PRIMARY KEY (`branch_id`,`account_id`,`entry_date`,`scope`),
  KEY `ix_finance_daily_date` (`branch_id`,`entry_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_account_mappings` (
  `mapping_key` varchar(50) NOT NULL,
  `account_id` bigint unsigned NOT NULL,
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`mapping_key`),
  KEY `fk_finance_mappings_account` (`account_id`),
  CONSTRAINT `fk_finance_mappings_account` FOREIGN KEY (`account_id`) REFERENCES `finance_accounts` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_cutover` (
  `branch_id` bigint unsigned NOT NULL,
  `doc_type` varchar(50) NOT NULL,
  `covered_through_id` bigint unsigned NOT NULL DEFAULT '0',
  `set_by` bigint unsigned DEFAULT NULL,
  `set_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`branch_id`,`doc_type`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_document_sequences` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `seq_key` varchar(40) NOT NULL,
  `current_value` bigint NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_sequences` (`branch_id`,`seq_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_inventory_cost_movements` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `stock_movement_id` bigint unsigned NOT NULL,
  `ref_type` varchar(50) NOT NULL DEFAULT '',
  `ref_id` bigint unsigned DEFAULT NULL,
  `direction` varchar(10) NOT NULL,
  `qty_base` decimal(24,12) NOT NULL,
  `unit_cost` decimal(18,4) DEFAULT NULL,
  `total_cost` decimal(18,2) DEFAULT NULL,
  `is_estimated` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `moved_at` datetime(6) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_cost_movement` (`stock_movement_id`),
  KEY `ix_finance_cost_position` (`branch_id`,`product_id`,`variant_id`),
  KEY `ix_cost_moves_ref` (`ref_type`,`ref_id`),
  KEY `ix_cost_ref_type` (`branch_id`,`ref_type`,`stock_movement_id`),
  KEY `ix_finance_cost_cursor` (`branch_id`,`stock_movement_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_journal_entries` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `number` varchar(120) NOT NULL,
  `branch_id` bigint unsigned NOT NULL,
  `entry_date` datetime(6) NOT NULL,
  `memo` varchar(500) NOT NULL,
  `source_doc_type` varchar(50) DEFAULT NULL,
  `source_doc_id` bigint unsigned DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'posted',
  `reversed_by` bigint unsigned DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `scope` tinyint NOT NULL DEFAULT '0',
  `requeue` tinyint NOT NULL DEFAULT '1',
  `source_active` tinyint GENERATED ALWAYS AS ((case when ((`status` = _utf8mb4'posted') and (`source_doc_type` is not null)) then 1 end)) VIRTUAL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_journal_number` (`number`),
  UNIQUE KEY `uq_finance_journal_source` (`branch_id`,`source_doc_type`,`source_doc_id`,`source_active`),
  KEY `ix_finance_journal_branch_date` (`branch_id`,`entry_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_journal_lines` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `entry_id` bigint unsigned NOT NULL,
  `account_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned DEFAULT NULL,
  `variant_id` bigint unsigned DEFAULT NULL,
  `party_id` bigint unsigned DEFAULT NULL,
  `debit` bigint NOT NULL DEFAULT '0',
  `credit` bigint NOT NULL DEFAULT '0',
  `description` varchar(500) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_finance_journal_lines_entry` (`entry_id`),
  KEY `ix_finance_journal_lines_account` (`account_id`),
  KEY `ix_finance_lines_account_product` (`account_id`,`product_id`),
  KEY `ix_finance_lines_account_party` (`account_id`,`party_id`),
  CONSTRAINT `fk_finance_journal_lines_entry` FOREIGN KEY (`entry_id`) REFERENCES `finance_journal_entries` (`id`),
  CONSTRAINT `chk_finance_lines_one_side` CHECK ((((`debit` > 0) and (`credit` = 0)) or ((`credit` > 0) and (`debit` = 0))))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_periods` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `year` int NOT NULL,
  `month` int NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'open',
  `closed_at` datetime(6) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_periods` (`branch_id`,`year`,`month`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_tax_periods` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `year` int NOT NULL,
  `month` int NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'open',
  `closed_at` datetime(6) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_tax_periods` (`branch_id`,`year`,`month`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_tax_report_snapshots` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `tax_period_id` bigint unsigned NOT NULL,
  `snapshot_json` json NOT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_finance_tax_snapshots_period` (`tax_period_id`),
  CONSTRAINT `fk_finance_tax_snapshots_period` FOREIGN KEY (`tax_period_id`) REFERENCES `finance_tax_periods` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
