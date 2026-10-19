-- Baseline modul product (F4-1): skema akhir hasil rantai migrasi 000001-000072.
-- Dibangkitkan scripts/squash-baseline.mjs dari SHOW CREATE TABLE; diverifikasi scripts/schema-compare.mjs.

CREATE TABLE `product_categories` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(50) NOT NULL,
  `name` varchar(255) NOT NULL,
  `parent_id` bigint unsigned DEFAULT NULL,
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  `sort_order` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_product_categories_code` (`code`),
  KEY `fk_product_categories_parent` (`parent_id`),
  CONSTRAINT `fk_product_categories_parent` FOREIGN KEY (`parent_id`) REFERENCES `product_categories` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `products` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `code` varchar(50) NOT NULL,
  `name` varchar(255) NOT NULL,
  `summary` text,
  `category_id` bigint unsigned DEFAULT NULL,
  `type` varchar(20) NOT NULL DEFAULT 'barang',
  `stock_tracked` tinyint(1) NOT NULL DEFAULT '1',
  `base_uom` varchar(30) NOT NULL,
  `purchase_uom` varchar(30) NOT NULL,
  `sales_uom` varchar(30) NOT NULL,
  `purchase_to_base_factor` decimal(24,12) NOT NULL DEFAULT '1.000000000000',
  `sales_to_base_factor` decimal(24,12) NOT NULL DEFAULT '1.000000000000',
  `purchase_price` bigint NOT NULL DEFAULT '0',
  `selling_price` bigint NOT NULL DEFAULT '0',
  `min_selling_price` bigint NOT NULL DEFAULT '0',
  `min_stock_qty` decimal(24,12) NOT NULL DEFAULT '0.000000000000',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  `updated_by` bigint unsigned DEFAULT NULL,
  `attributes` json DEFAULT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_products_code` (`code`),
  KEY `fk_products_category` (`category_id`),
  KEY `ix_products_name` (`name`),
  CONSTRAINT `fk_products_category` FOREIGN KEY (`category_id`) REFERENCES `product_categories` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `product_variants` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `product_id` bigint unsigned NOT NULL,
  `code` varchar(80) NOT NULL,
  `name` varchar(255) NOT NULL,
  `barcode` varchar(100) DEFAULT NULL,
  `is_default` tinyint(1) NOT NULL DEFAULT '0',
  `status` varchar(20) NOT NULL DEFAULT 'active',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `updated_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `attributes` json DEFAULT NULL,
  `is_hidden` tinyint(1) NOT NULL DEFAULT '0',
  `sort_order` int NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_product_variants` (`product_id`,`code`),
  UNIQUE KEY `uq_product_variants_barcode` (`barcode`),
  KEY `ix_product_variants_code` (`code`),
  CONSTRAINT `fk_product_variants_product` FOREIGN KEY (`product_id`) REFERENCES `products` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
