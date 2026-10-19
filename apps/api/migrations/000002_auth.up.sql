-- Baseline modul auth (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `user_sessions` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `user_id` bigint unsigned NOT NULL,
  `refresh_hash` char(64) NOT NULL,
  `prev_refresh_hash` char(64) DEFAULT NULL,
  `rotated_at` datetime(6) DEFAULT NULL,
  `id_active_branch` bigint unsigned DEFAULT NULL,
  `id_active_role` bigint unsigned DEFAULT NULL,
  `user_agent` varchar(255) DEFAULT NULL,
  `ip_address` varchar(64) DEFAULT NULL,
  `expires_at` datetime(6) NOT NULL,
  `absolute_expires_at` datetime(6) NOT NULL,
  `last_activity_at` datetime(6) NOT NULL,
  `revoked_at` datetime(6) DEFAULT NULL,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_user_sessions_refresh` (`refresh_hash`),
  KEY `ix_user_sessions_user` (`user_id`),
  KEY `ix_user_sessions_expiry` (`absolute_expires_at`),
  KEY `ix_user_sessions_revoked` (`revoked_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
