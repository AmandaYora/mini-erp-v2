-- Baseline modul salesreturn (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `sales_returns` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `number` varchar(100) NOT NULL,
  `branch_id` bigint unsigned NOT NULL,
  `sales_order_id` bigint unsigned NOT NULL,
  `return_date` datetime(6) NOT NULL,
  `total` bigint NOT NULL DEFAULT '0',
  `subtotal` bigint NOT NULL DEFAULT '0',
  `discount_total` bigint NOT NULL DEFAULT '0',
  `tax_total` bigint NOT NULL DEFAULT '0',
  `tax_type` varchar(20) NOT NULL DEFAULT 'none',
  `tax_rate` decimal(9,4) NOT NULL DEFAULT '0.0000',
  `return_mode` varchar(20) NOT NULL DEFAULT 'return_only',
  `replacement_delivery_status` varchar(20) NOT NULL DEFAULT 'not_required',
  `status` varchar(20) NOT NULL DEFAULT 'draft',
  `notes` text,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  `reason` text NOT NULL DEFAULT (_utf8mb4''),
  `replacement_subtotal` bigint NOT NULL DEFAULT '0',
  `replacement_tax_total` bigint NOT NULL DEFAULT '0',
  `replacement_total` bigint NOT NULL DEFAULT '0',
  `difference_amount` bigint NOT NULL DEFAULT '0',
  `receivable_reduction` bigint NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_sales_returns_number` (`number`),
  KEY `ix_sales_returns_so` (`sales_order_id`),
  KEY `ix_sales_returns_branch_id` (`branch_id`,`id`),
  KEY `ix_sales_returns_date` (`branch_id`,`return_date`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `sales_return_items` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `return_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `location_id` bigint unsigned NOT NULL,
  `uom` varchar(30) NOT NULL,
  `uom_factor` decimal(24,12) NOT NULL DEFAULT '1.000000000000',
  `qty` decimal(24,12) NOT NULL,
  `qty_base` decimal(24,12) NOT NULL,
  `unit_price` bigint NOT NULL,
  `discount_pct` decimal(9,4) NOT NULL DEFAULT '0.0000',
  `discount_nominal` bigint NOT NULL DEFAULT '0',
  `tax_base` bigint NOT NULL DEFAULT '0',
  `tax_amount` bigint NOT NULL DEFAULT '0',
  `line_total` bigint NOT NULL,
  `return_condition` varchar(12) NOT NULL DEFAULT 'normal',
  PRIMARY KEY (`id`),
  KEY `fk_sales_return_items_return` (`return_id`),
  CONSTRAINT `fk_sales_return_items_return` FOREIGN KEY (`return_id`) REFERENCES `sales_returns` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `sales_return_replacement_items` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `return_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `location_id` bigint unsigned NOT NULL,
  `uom` varchar(30) NOT NULL,
  `uom_factor` decimal(24,12) NOT NULL DEFAULT '1.000000000000',
  `qty` decimal(24,12) NOT NULL,
  `qty_base` decimal(24,12) NOT NULL,
  `unit_price` bigint NOT NULL DEFAULT '0',
  `line_total` bigint NOT NULL DEFAULT '0',
  `tax_base` bigint NOT NULL DEFAULT '0',
  `tax_amount` bigint NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `ix_sales_return_replacement_items_return` (`return_id`),
  CONSTRAINT `fk_sales_return_replacement_items_return` FOREIGN KEY (`return_id`) REFERENCES `sales_returns` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `sales_return_settlements` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `return_id` bigint unsigned NOT NULL,
  `settlement_type` varchar(30) NOT NULL,
  `settlement_date` datetime(6) NOT NULL,
  `amount` bigint NOT NULL DEFAULT '0',
  `payment_method` varchar(30) DEFAULT NULL,
  `reference_number` varchar(100) DEFAULT NULL,
  `notes` text,
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `auto` tinyint(1) NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `ix_sales_return_settlements_return` (`return_id`),
  CONSTRAINT `fk_sales_return_settlements_return` FOREIGN KEY (`return_id`) REFERENCES `sales_returns` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
