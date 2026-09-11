-- Tahap C2: alamat kirim menempel ke SO sebagai snapshot (kontrak §7 no. 3
-- MODULE_MAP: nota tak bergeser bila master alamat diubah). ship_to_address_id
-- menunjuk party_delivery_addresses tanpa FK; 4 kolom snapshot dibekukan saat
-- order dibuat/diubah (draf).

ALTER TABLE sales_orders
  ADD COLUMN ship_to_address_id BIGINT       UNSIGNED NULL AFTER party_id,
  ADD COLUMN ship_to_label      VARCHAR(100) NULL AFTER ship_to_address_id,
  ADD COLUMN ship_to_recipient  VARCHAR(150) NULL AFTER ship_to_label,
  ADD COLUMN ship_to_phone      VARCHAR(30)  NULL AFTER ship_to_recipient,
  ADD COLUMN ship_to_address    VARCHAR(255) NULL AFTER ship_to_phone;
