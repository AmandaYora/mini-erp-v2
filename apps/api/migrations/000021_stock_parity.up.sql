-- ST-01: atribut lokasi gudang (area picking, lokasi utama, urutan) dan
-- ST-02: sopir, kendaraan, dan tanggal transfer pada dokumen transfer stok.
-- Semua kolom aditif dan idempoten terhadap ETL (spec load membawa kolom
-- legacy is_picking_area/is_primary/sort_order dan driver_name/vehicle_number/
-- transfer_date). Nilai lama tidak diubah kecuali transfer_date, yang diisi
-- dari hari (WIB) kirim/dibuat agar dokumen lama punya tanggal.
ALTER TABLE `stock_locations`
  ADD COLUMN `is_picking_area` tinyint(1) NOT NULL DEFAULT '0' AFTER `kind`,
  ADD COLUMN `is_primary` tinyint(1) NOT NULL DEFAULT '0' AFTER `is_picking_area`,
  ADD COLUMN `sort_order` int NOT NULL DEFAULT '0' AFTER `is_primary`;

ALTER TABLE `stock_transfers`
  ADD COLUMN `driver_name` varchar(150) DEFAULT NULL AFTER `notes`,
  ADD COLUMN `vehicle_plate` varchar(30) DEFAULT NULL AFTER `driver_name`,
  ADD COLUMN `transfer_date` datetime(6) DEFAULT NULL AFTER `vehicle_plate`;

-- transfer_date holds the Jakarta calendar day (midnight), like order_date.
UPDATE `stock_transfers`
  SET `transfer_date` = DATE(CONVERT_TZ(COALESCE(`dispatched_at`, `created_at`), '+00:00', '+07:00'))
  WHERE `transfer_date` IS NULL;
