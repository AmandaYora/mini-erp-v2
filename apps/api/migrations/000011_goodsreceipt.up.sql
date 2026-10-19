-- Baseline modul goodsreceipt (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `goods_receipts` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `number` varchar(100) DEFAULT NULL,
  `branch_id` bigint unsigned NOT NULL,
  `purchase_order_id` bigint unsigned NOT NULL,
  `received_at` datetime(6) NOT NULL,
  `notes` text,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `supplier_delivery_number` varchar(100) DEFAULT NULL,
  `status` varchar(12) NOT NULL DEFAULT 'posted',
  `client_ref` varchar(64) DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_goods_receipts_number` (`number`),
  UNIQUE KEY `uq_goods_receipts_client_ref` (`branch_id`,`client_ref`),
  KEY `ix_goods_receipts_po` (`purchase_order_id`),
  KEY `ix_goods_receipts_branch_id` (`branch_id`,`id`),
  KEY `ix_goods_receipts_po_status` (`purchase_order_id`,`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `goods_receipt_items` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `receipt_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `location_id` bigint unsigned NOT NULL,
  `uom` varchar(30) NOT NULL,
  `uom_factor` decimal(24,12) NOT NULL DEFAULT '1.000000000000',
  `qty` decimal(24,12) NOT NULL,
  `qty_base` decimal(24,12) NOT NULL,
  PRIMARY KEY (`id`),
  KEY `fk_goods_receipt_items_receipt` (`receipt_id`),
  CONSTRAINT `fk_goods_receipt_items_receipt` FOREIGN KEY (`receipt_id`) REFERENCES `goods_receipts` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
