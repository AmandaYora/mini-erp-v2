-- SJ Langsung (ADR-0022): surat jalan yang dibuat sebelum ada order jual.
-- origin membedakan asalnya; selama belum diorder sales_order_id = 0 dan
-- pelanggan + alamat kirim disimpan sebagai snapshot di SJ. Semua kolom
-- berdefault/nullable sehingga data lama dan hasil ETL tetap origin='order'.
ALTER TABLE `delivery_notes`
  ADD COLUMN `origin` varchar(20) NOT NULL DEFAULT 'order' AFTER `document_kind`,
  ADD COLUMN `party_id` bigint unsigned DEFAULT NULL AFTER `origin`,
  ADD COLUMN `ship_to_label` varchar(120) DEFAULT NULL AFTER `party_id`,
  ADD COLUMN `ship_to_recipient` varchar(150) DEFAULT NULL AFTER `ship_to_label`,
  ADD COLUMN `ship_to_phone` varchar(30) DEFAULT NULL AFTER `ship_to_recipient`,
  ADD COLUMN `ship_to_address` text AFTER `ship_to_phone`,
  ADD COLUMN `linked_at` datetime(6) DEFAULT NULL AFTER `ship_to_address`,
  ADD COLUMN `linked_by` bigint unsigned DEFAULT NULL AFTER `linked_at`,
  ADD KEY `ix_delivery_notes_direct_open` (`branch_id`, `origin`, `sales_order_id`, `status`, `delivery_date`);
