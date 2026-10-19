-- Baseline modul party (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `member_types` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(50) NOT NULL,
  `name` varchar(120) NOT NULL,
  `description_text` text,
  `price_basis` varchar(30) NOT NULL DEFAULT 'selling_price',
  `price_adjustment_direction` varchar(20) NOT NULL DEFAULT 'minus',
  `price_adjustment_type` varchar(20) NOT NULL DEFAULT 'percent',
  `price_adjustment_value` decimal(18,4) NOT NULL DEFAULT '0.0000',
  `rounding_mode` varchar(20) NOT NULL DEFAULT 'none',
  `rounding_increment` decimal(18,2) DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `archived_at` datetime(6) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_member_types_code` (`code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `parties` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(50) NOT NULL,
  `party_type` varchar(20) NOT NULL,
  `name` varchar(255) NOT NULL,
  `phone` varchar(30) DEFAULT NULL,
  `email` varchar(255) DEFAULT NULL,
  `address_text` text,
  `notes` text,
  `attributes` json DEFAULT NULL,
  `member_type_id` bigint unsigned DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `archived_at` datetime(6) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_parties_code_type` (`code`,`party_type`),
  KEY `fk_parties_member_type` (`member_type_id`),
  KEY `ix_parties_type_name` (`party_type`,`name`),
  CONSTRAINT `fk_parties_member_type` FOREIGN KEY (`member_type_id`) REFERENCES `member_types` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `party_delivery_addresses` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `party_id` bigint unsigned NOT NULL,
  `label` varchar(120) DEFAULT NULL,
  `recipient_name` varchar(150) DEFAULT NULL,
  `phone` varchar(30) DEFAULT NULL,
  `address_text` text NOT NULL,
  `is_primary` tinyint(1) NOT NULL DEFAULT '0',
  `sort_order` int NOT NULL DEFAULT '0',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `archived_at` datetime(6) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `fk_party_addresses_party` (`party_id`),
  CONSTRAINT `fk_party_addresses_party` FOREIGN KEY (`party_id`) REFERENCES `parties` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
