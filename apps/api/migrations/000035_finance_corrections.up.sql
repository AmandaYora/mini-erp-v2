-- Koreksi harga modal lewat web: revaluasi persediaan (nilai satu atau beberapa posisi disetel
-- ke harga modal baru). Satu header per komit (satu jurnal), satu baris per posisi. Baris ledger
-- biaya (qty 0, nilai = selisih) ditulis ke finance_inventory_cost_movements dengan ref_type
-- 'inventory_revaluation' dan ref_id = id header ini.
CREATE TABLE `finance_inventory_revaluations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `revaluation_date` date NOT NULL,
  `reason` varchar(500) NOT NULL,
  `total_delta` bigint NOT NULL,
  `journal_entry_id` bigint unsigned DEFAULT NULL,
  `client_ref` varchar(64) DEFAULT NULL,
  `created_by` bigint unsigned DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_revaluations_client_ref` (`branch_id`,`client_ref`),
  KEY `ix_finance_revaluations_branch` (`branch_id`,`id`),
  CONSTRAINT `fk_finance_revaluations_entry` FOREIGN KEY (`journal_entry_id`) REFERENCES `finance_journal_entries` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `finance_inventory_revaluation_lines` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `revaluation_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `qty_base` decimal(24,12) NOT NULL,
  `unit_cost_before` decimal(18,4) NOT NULL,
  `unit_cost_after` decimal(18,4) NOT NULL,
  `value_before` bigint NOT NULL,
  `value_after` bigint NOT NULL,
  `delta` bigint NOT NULL,
  `cost_movement_id` bigint unsigned NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_finance_revaluation_line` (`revaluation_id`,`product_id`,`variant_id`),
  KEY `ix_finance_revaluation_lines_product` (`product_id`,`variant_id`),
  CONSTRAINT `fk_finance_revaluation_lines_header` FOREIGN KEY (`revaluation_id`) REFERENCES `finance_inventory_revaluations` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
