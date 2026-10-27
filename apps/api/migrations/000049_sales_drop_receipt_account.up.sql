-- Rekening tujuan tagihan dihapus (ADR-0026 revisi 2026-10-27, analysis/005):
-- rekening dipilih saat pembayaran dicatat (payments.cash_account_id), bukan
-- di order. Kolom 000047 tidak dibaca atau ditulis lagi.
ALTER TABLE `sales_orders` DROP COLUMN `receipt_account_id`;
