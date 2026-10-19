-- FN-01/FN-07/FN-10: penanda dokumen diabaikan dari antrian posting, jenis+catatan koreksi fiskal,
-- pihak lawan opsional pada biaya usaha.
CREATE TABLE `finance_posting_ignores` (
  `branch_id` bigint unsigned NOT NULL,
  `doc_type` varchar(50) NOT NULL,
  `doc_id` bigint unsigned NOT NULL,
  `reason` varchar(255) NOT NULL,
  `ignored_by` bigint unsigned DEFAULT NULL,
  `ignored_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`branch_id`,`doc_type`,`doc_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_tax_adjustment_meta` (
  `entry_id` bigint unsigned NOT NULL,
  `classification` varchar(30) NOT NULL,
  `note` varchar(1000) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`entry_id`),
  KEY `ix_tax_adjust_meta_class` (`classification`),
  CONSTRAINT `fk_tax_adjust_meta_entry` FOREIGN KEY (`entry_id`) REFERENCES `finance_journal_entries` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

ALTER TABLE `business_expenses`
  ADD COLUMN `party_id` bigint unsigned DEFAULT NULL AFTER `cancelled_by`,
  ADD KEY `ix_business_expenses_party` (`party_id`);
