-- Penundaan alert kesehatan data (ADR-0025). Alert sendiri tidak disimpan:
-- dihitung langsung dari data. Hanya penundaannya yang disimpan, satu baris per
-- (cabang, alert); menunda ulang menimpa baris itu, membatalkan menghapusnya
-- (riwayat lengkap ada di audit_logs). Tanpa FK lintas modul.
CREATE TABLE `dashboard_alert_snoozes` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `branch_id` BIGINT UNSIGNED NOT NULL,
  `alert_id` VARCHAR(80) NOT NULL,
  `reason` VARCHAR(255) NOT NULL,
  `duration_days` TINYINT UNSIGNED NOT NULL,
  `snoozed_until` DATETIME(6) NOT NULL,
  `count_at_snooze` BIGINT NOT NULL,
  `amount_at_snooze` BIGINT NULL,
  `created_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` BIGINT UNSIGNED NULL,
  `updated_by` BIGINT UNSIGNED NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_dashboard_alert_snooze` (`branch_id`, `alert_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
