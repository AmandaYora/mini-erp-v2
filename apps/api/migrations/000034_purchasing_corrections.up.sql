-- Koreksi kesalahan input order beli: alasan batal, tutup sisa order, dan
-- koreksi harga beli setelah penerimaan.
ALTER TABLE `purchase_orders`
  ADD COLUMN `cancel_reason` varchar(500) DEFAULT NULL AFTER `notes`,
  ADD COLUMN `cancelled_at` datetime(6) DEFAULT NULL AFTER `cancel_reason`,
  ADD COLUMN `cancelled_by` bigint unsigned DEFAULT NULL AFTER `cancelled_at`,
  ADD COLUMN `short_close_reason` varchar(500) DEFAULT NULL AFTER `cancelled_by`,
  ADD COLUMN `short_closed_at` datetime(6) DEFAULT NULL AFTER `short_close_reason`,
  ADD COLUMN `short_closed_by` bigint unsigned DEFAULT NULL AFTER `short_closed_at`;

-- One correction of a confirmed/completed order: a price correction after
-- receipt (kind price, journalled by finance as source purchase_price_correction)
-- or a short close (kind short_close, no journal). The lines keep the line
-- economics before and after; received_qty says how much of the line the
-- posted receipts had brought in, i.e. which receipts stay valued as they were
-- booked (at the old economics).
CREATE TABLE `purchase_order_corrections` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `branch_id` bigint unsigned NOT NULL,
  `order_id` bigint unsigned NOT NULL,
  `kind` varchar(20) NOT NULL,
  `correction_date` datetime(6) NOT NULL,
  `reason` varchar(500) NOT NULL,
  `old_grand_total` bigint NOT NULL,
  `new_grand_total` bigint NOT NULL,
  `value_delta` bigint NOT NULL DEFAULT '0',
  `tax_delta` bigint NOT NULL DEFAULT '0',
  `created_at` datetime(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  `created_by` bigint unsigned DEFAULT NULL,
  PRIMARY KEY (`id`),
  KEY `ix_po_corrections_order` (`order_id`,`id`),
  KEY `ix_po_corrections_branch_kind` (`branch_id`,`kind`,`id`),
  CONSTRAINT `fk_po_corrections_order` FOREIGN KEY (`order_id`) REFERENCES `purchase_orders` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE `purchase_order_correction_lines` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `correction_id` bigint unsigned NOT NULL,
  `order_item_id` bigint unsigned NOT NULL,
  `product_id` bigint unsigned NOT NULL,
  `variant_id` bigint unsigned NOT NULL,
  `product_name` varchar(255) NOT NULL DEFAULT '',
  `uom` varchar(30) NOT NULL,
  `stock_tracked` tinyint(1) NOT NULL DEFAULT '1',
  `received_qty` decimal(24,12) NOT NULL DEFAULT '0.000000000000',
  `received_qty_base` decimal(24,12) NOT NULL DEFAULT '0.000000000000',
  `old_qty` decimal(24,12) NOT NULL,
  `new_qty` decimal(24,12) NOT NULL,
  `old_unit_price` bigint NOT NULL,
  `new_unit_price` bigint NOT NULL,
  `old_discount_pct` decimal(9,4) NOT NULL DEFAULT '0.0000',
  `new_discount_pct` decimal(9,4) NOT NULL DEFAULT '0.0000',
  `old_discount_nominal` bigint NOT NULL DEFAULT '0',
  `new_discount_nominal` bigint NOT NULL DEFAULT '0',
  `old_tax_base` bigint NOT NULL,
  `new_tax_base` bigint NOT NULL,
  `old_tax_amount` bigint NOT NULL,
  `new_tax_amount` bigint NOT NULL,
  `old_line_total` bigint NOT NULL,
  `new_line_total` bigint NOT NULL,
  `value_delta` bigint NOT NULL DEFAULT '0',
  `tax_delta` bigint NOT NULL DEFAULT '0',
  PRIMARY KEY (`id`),
  KEY `fk_po_correction_lines_correction` (`correction_id`),
  CONSTRAINT `fk_po_correction_lines_correction` FOREIGN KEY (`correction_id`) REFERENCES `purchase_order_corrections` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;
