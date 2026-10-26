-- Rekening tujuan tagihan (ADR-0026): id akun kas/bank rekening perusahaan
-- (finance) yang dicetak di nota dan menjadi nilai awal pembayaran transfer.
-- ID primitif tanpa FK lintas modul; NULL = belum dipilih (data lama, ETL).
ALTER TABLE `sales_orders`
  ADD COLUMN `receipt_account_id` BIGINT UNSIGNED NULL AFTER `due_date`;
