-- Baseline modul audit (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `audit_logs` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `action_key` varchar(100) NOT NULL,
  `entity_type` varchar(50) NOT NULL,
  `entity_id` bigint unsigned DEFAULT NULL,
  `branch_id` bigint unsigned DEFAULT NULL,
  `actor_type` varchar(20) NOT NULL DEFAULT 'user',
  `actor_id` bigint unsigned DEFAULT NULL,
  `note` varchar(500) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  KEY `ix_audit_logs_branch_time` (`branch_id`,`created_at`),
  KEY `ix_audit_logs_action` (`action_key`),
  KEY `ix_audit_logs_branch` (`branch_id`,`id`),
  KEY `ix_audit_logs_branch_action` (`branch_id`,`action_key`,`id`),
  KEY `ix_audit_logs_entity` (`entity_type`,`entity_id`,`id`),
  KEY `ix_audit_logs_created` (`created_at`),
  KEY `ix_audit_logs_actor` (`actor_id`,`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `audit_log_details` (
  `log_id` bigint unsigned NOT NULL,
  `before_json` json DEFAULT NULL,
  `after_json` json DEFAULT NULL,
  `metadata_json` json DEFAULT NULL,
  PRIMARY KEY (`log_id`),
  CONSTRAINT `fk_audit_log_details_log` FOREIGN KEY (`log_id`) REFERENCES `audit_logs` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
