-- Baseline modul stock (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `stock_locations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `code` varchar(50) NOT NULL,
  `name` varchar(255) NOT NULL,
  `parent_id` bigint unsigned DEFAULT NULL,
  `is_system` tinyint(1) NOT NULL DEFAULT '0',
  `kind` varchar(12) NOT NULL DEFAULT 'normal',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_stock_locations` (`branch_id`,`code`),
  KEY `fk_stock_locations_parent` (`parent_id`),
  CONSTRAINT `fk_stock_locations_parent` FOREIGN KEY (`parent_id`) REFERENCES `stock_locations` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `stock_balances` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `location_id` bigint unsigned NOT NULL,
  `on_hand` decimal(24,12) NOT NULL DEFAULT '0.000000000000',
  `reserved` decimal(24,12) NOT NULL DEFAULT '0.000000000000',
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_stock_balances` (`branch_id`,`product_id`,`variant_id`,`location_id`),
  KEY `fk_stock_balances_location` (`location_id`),
  KEY `ix_stock_balances_product` (`product_id`,`variant_id`),
  CONSTRAINT `fk_stock_balances_location` FOREIGN KEY (`location_id`) REFERENCES `stock_locations` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `stock_movements` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `location_id` bigint unsigned NOT NULL,
  `direction` varchar(10) NOT NULL,
  `movement_type` varchar(20) NOT NULL,
  `qty_base` decimal(24,12) NOT NULL,
  `balance_before` decimal(24,12) DEFAULT NULL,
  `balance_after` decimal(24,12) DEFAULT NULL,
  `unit_cost` decimal(18,4) DEFAULT NULL,
  `ref_type` varchar(50) NOT NULL DEFAULT '',
  `ref_id` bigint unsigned DEFAULT NULL,
  `notes` varchar(500) DEFAULT NULL,
  `created_by` bigint unsigned DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `moved_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_stock_movements_ref` (`ref_type`,`ref_id`,`product_id`,`variant_id`,`location_id`,`direction`),
  KEY `ix_stock_movements_lookup` (`branch_id`,`product_id`,`variant_id`,`location_id`),
  KEY `fk_stock_movements_location` (`location_id`),
  KEY `ix_stock_movements_branch_time` (`branch_id`,`moved_at`),
  KEY `ix_stock_movements_product` (`product_id`,`variant_id`),
  CONSTRAINT `fk_stock_movements_location` FOREIGN KEY (`location_id`) REFERENCES `stock_locations` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `stock_reservations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `reservation_key` varchar(100) NOT NULL,
  `branch_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `location_id` bigint unsigned DEFAULT NULL,
  `qty_total` decimal(24,12) NOT NULL,
  `qty_remaining` decimal(24,12) NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `expires_at` datetime(6) NOT NULL,
  `created_by` bigint unsigned DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_stock_reservations_key` (`reservation_key`),
  KEY `ix_stock_reservations_lookup` (`branch_id`,`product_id`,`variant_id`,`status`),
  KEY `ix_stock_reservations_exp` (`status`,`expires_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `stock_transfers` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `number` varchar(100) NOT NULL,
  `from_branch_id` bigint unsigned NOT NULL,
  `to_branch_id` bigint unsigned NOT NULL,
  `from_location_id` bigint unsigned NOT NULL,
  `to_location_id` bigint unsigned NOT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'draft',
  `notes` text,
  `dispatched_at` datetime(6) DEFAULT NULL,
  `received_at` datetime(6) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_stock_transfers_number` (`number`),
  KEY `fk_stock_transfers_from_loc` (`from_location_id`),
  KEY `fk_stock_transfers_to_loc` (`to_location_id`),
  KEY `ix_stock_transfers_from` (`from_branch_id`,`id`),
  KEY `ix_stock_transfers_to` (`to_branch_id`,`id`),
  CONSTRAINT `fk_stock_transfers_from_loc` FOREIGN KEY (`from_location_id`) REFERENCES `stock_locations` (`id`),
  CONSTRAINT `fk_stock_transfers_to_loc` FOREIGN KEY (`to_location_id`) REFERENCES `stock_locations` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `stock_transfer_items` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `transfer_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `qty_base` decimal(24,12) NOT NULL,
  `notes` text,
  `to_location_id` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_stock_transfer_items_transfer` (`transfer_id`),
  CONSTRAINT `fk_stock_transfer_items_transfer` FOREIGN KEY (`transfer_id`) REFERENCES `stock_transfers` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
