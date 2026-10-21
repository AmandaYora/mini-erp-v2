-- Koreksi pembayaran & alasan batal: pembayaran pengganti menunjuk pembayaran
-- yang dikoreksinya; pembatalan mencatat siapa, kapan, dan alasannya.
ALTER TABLE `payments`
  ADD COLUMN `corrects_payment_id` bigint unsigned DEFAULT NULL AFTER `credit_source_id`,
  ADD COLUMN `cancel_reason` varchar(255) DEFAULT NULL AFTER `corrects_payment_id`,
  ADD COLUMN `cancelled_at` datetime(6) DEFAULT NULL AFTER `cancel_reason`,
  ADD COLUMN `cancelled_by` bigint unsigned DEFAULT NULL AFTER `cancelled_at`,
  ADD KEY `ix_payments_corrects` (`corrects_payment_id`);
