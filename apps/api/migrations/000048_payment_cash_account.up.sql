-- Rekening pembayaran (ADR-0026): id akun kas/bank (finance) tempat uang
-- transfer/cek benar-benar masuk atau keluar. Jurnal pembayaran memakai akun
-- ini; NULL = jalur lama (pemetaan `cash`/`bank` menurut metode) — pembayaran
-- historis dan hasil ETL tidak berubah. ID primitif tanpa FK lintas modul.
ALTER TABLE `payments`
  ADD COLUMN `cash_account_id` BIGINT UNSIGNED NULL AFTER `reference_number`,
  ADD KEY `idx_payments_cash_account` (`cash_account_id`);
