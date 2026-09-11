-- Tahap C1: kelengkapan dokumen surat jalan (kunci penentu sah-tidaknya SJ di
-- lapangan) + stempel waktu/aktor dispatch & konfirmasi. Alur 2-status tetap:
-- dispatched_* diisi saat konfirmasi (barang berangkat), confirmed_* + data
-- penerima diisi di momen yang sama (serah terima langsung). Kolom terpisah
-- disiapkan untuk alur 3-status di masa depan dan untuk mendaratkan kolom
-- legacy apa adanya saat ETL.

ALTER TABLE delivery_notes
  ADD COLUMN driver_name                      VARCHAR(100) NULL AFTER delivery_date,
  ADD COLUMN vehicle_plate                    VARCHAR(20)  NULL AFTER driver_name,
  ADD COLUMN warehouse_staff_name             VARCHAR(100) NULL AFTER vehicle_plate,
  ADD COLUMN recipient_name                   VARCHAR(150) NULL AFTER warehouse_staff_name,
  ADD COLUMN recipient_signature_status       VARCHAR(30)  NULL AFTER recipient_name,
  ADD COLUMN recipient_signature_missing_reason TEXT       NULL AFTER recipient_signature_status,
  ADD COLUMN drop_location_note               TEXT         NULL AFTER recipient_signature_missing_reason,
  ADD COLUMN dispatched_at                    DATETIME(6)  NULL AFTER drop_location_note,
  ADD COLUMN dispatched_by                    BIGINT UNSIGNED NULL AFTER dispatched_at,
  ADD COLUMN confirmed_at                     DATETIME(6)  NULL AFTER dispatched_by,
  ADD COLUMN confirmed_by                     BIGINT UNSIGNED NULL AFTER confirmed_at;
