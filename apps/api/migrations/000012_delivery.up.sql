-- Baseline modul delivery (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `delivery_notes` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `number` varchar(100) NOT NULL,
  `branch_id` bigint unsigned NOT NULL,
  `sales_order_id` bigint unsigned NOT NULL,
  `delivery_date` datetime(6) NOT NULL,
  `driver_name` varchar(100) DEFAULT NULL,
  `vehicle_plate` varchar(20) DEFAULT NULL,
  `warehouse_staff_name` varchar(100) DEFAULT NULL,
  `recipient_name` varchar(150) DEFAULT NULL,
  `recipient_signature_status` varchar(30) DEFAULT NULL,
  `recipient_signature_missing_reason` text,
  `drop_location_note` text,
  `dispatched_at` datetime(6) DEFAULT NULL,
  `dispatched_by` bigint unsigned DEFAULT NULL,
  `confirmed_at` datetime(6) DEFAULT NULL,
  `confirmed_by` bigint unsigned DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'draft',
  `document_kind` varchar(20) NOT NULL DEFAULT 'order',
  `sales_return_id` bigint unsigned DEFAULT NULL,
  `notes` text,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  `client_ref` varchar(64) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_delivery_notes_number` (`number`),
  UNIQUE KEY `uq_delivery_notes_client_ref` (`branch_id`,`client_ref`),
  KEY `ix_delivery_notes_so` (`sales_order_id`),
  KEY `ix_delivery_notes_return` (`sales_return_id`),
  KEY `ix_delivery_notes_branch_id` (`branch_id`,`id`),
  KEY `ix_delivery_notes_waiting` (`branch_id`,`status`,`document_kind`,`delivery_date`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `delivery_note_items` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `delivery_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `location_id` bigint unsigned NOT NULL,
  `uom` varchar(30) NOT NULL,
  `uom_factor` decimal(24,12) NOT NULL DEFAULT '1.000000000000',
  `qty` decimal(24,12) NOT NULL,
  `qty_base` decimal(24,12) NOT NULL,
  `reservation_key` varchar(100) DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_delivery_note_items_delivery` (`delivery_id`),
  CONSTRAINT `fk_delivery_note_items_delivery` FOREIGN KEY (`delivery_id`) REFERENCES `delivery_notes` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
