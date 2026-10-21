-- Koreksi salah input: alasan, waktu, dan pelaku pembatalan retur pembelian.
ALTER TABLE `purchase_returns`
  ADD COLUMN `cancel_reason` varchar(255) DEFAULT NULL AFTER `status`,
  ADD COLUMN `cancelled_at` datetime(6) DEFAULT NULL AFTER `cancel_reason`,
  ADD COLUMN `cancelled_by` bigint unsigned DEFAULT NULL AFTER `cancelled_at`;
