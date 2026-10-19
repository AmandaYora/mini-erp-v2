-- Baseline modul media (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `media_files` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `owner_type` varchar(50) NOT NULL,
  `owner_id` bigint unsigned NOT NULL,
  `file_key` varchar(700) NOT NULL,
  `original_name` varchar(255) NOT NULL,
  `mime` varchar(120) NOT NULL,
  `size_bytes` bigint NOT NULL DEFAULT '0',
  `is_primary` tinyint(1) NOT NULL DEFAULT '0',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `origin` enum('app','legacy') NOT NULL DEFAULT 'app',
  `width` int DEFAULT NULL,
  `height` int DEFAULT NULL,
  `original_size_bytes` bigint DEFAULT NULL,
  `sort_order` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_media_files_key` (`file_key`),
  KEY `ix_media_files_owner` (`owner_type`,`owner_id`),
  KEY `ix_media_files_origin` (`origin`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
