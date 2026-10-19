-- Baseline modul payment (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `payments` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `number` varchar(100) NOT NULL,
  `branch_id` bigint unsigned NOT NULL,
  `party_id` bigint unsigned NOT NULL,
  `party_type` varchar(20) NOT NULL,
  `direction` varchar(10) NOT NULL,
  `amount` bigint NOT NULL,
  `method` varchar(30) NOT NULL DEFAULT 'cash',
  `paid_at` datetime(6) NOT NULL,
  `notes` text,
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  `client_ref` varchar(64) DEFAULT NULL,
  `amount_tendered` bigint DEFAULT NULL,
  `reference_number` varchar(100) DEFAULT NULL,
  `pair_id` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_payments_number` (`number`),
  UNIQUE KEY `uq_payments_client_ref` (`branch_id`,`client_ref`),
  KEY `ix_payments_party` (`party_id`,`status`),
  KEY `ix_payments_branch_id` (`branch_id`,`id`),
  KEY `ix_payments_pair` (`pair_id`),
  KEY `ix_payments_paid_at` (`branch_id`,`paid_at`),
  KEY `ix_payments_branch_party` (`branch_id`,`party_id`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `payment_allocations` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `payment_id` bigint unsigned NOT NULL,
  `order_type` varchar(20) NOT NULL,
  `order_id` bigint unsigned NOT NULL,
  `amount` bigint NOT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_payment_allocations_payment` (`payment_id`),
  KEY `ix_payment_allocations_cover` (`order_type`,`order_id`,`payment_id`,`amount`),
  CONSTRAINT `fk_payment_allocations_payment` FOREIGN KEY (`payment_id`) REFERENCES `payments` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
