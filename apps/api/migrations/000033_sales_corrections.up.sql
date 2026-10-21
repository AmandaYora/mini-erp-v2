-- Koreksi salah input order jual: alasan + siapa/kapan untuk pembatalan dan
-- "tutup sisa order" (short-close). Kolom nullable: order lama dan order yang
-- tidak pernah dibatalkan/ditutup sisa tetap NULL.
ALTER TABLE `sales_orders`
  ADD COLUMN `cancel_reason` varchar(255) DEFAULT NULL,
  ADD COLUMN `cancelled_at` datetime(6) DEFAULT NULL,
  ADD COLUMN `cancelled_by` bigint unsigned DEFAULT NULL,
  ADD COLUMN `short_close_reason` varchar(255) DEFAULT NULL,
  ADD COLUMN `short_closed_at` datetime(6) DEFAULT NULL,
  ADD COLUMN `short_closed_by` bigint unsigned DEFAULT NULL;
