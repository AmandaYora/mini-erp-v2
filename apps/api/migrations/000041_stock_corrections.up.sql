-- Koreksi salah input: alasan, waktu, dan pelaku pembatalan transfer stok.
ALTER TABLE `stock_transfers`
  ADD COLUMN `cancel_reason` varchar(255) DEFAULT NULL AFTER `status`,
  ADD COLUMN `cancelled_at` datetime(6) DEFAULT NULL AFTER `cancel_reason`,
  ADD COLUMN `cancelled_by` bigint unsigned DEFAULT NULL AFTER `cancelled_at`;
