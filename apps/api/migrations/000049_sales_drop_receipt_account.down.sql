ALTER TABLE `sales_orders`
  ADD COLUMN `receipt_account_id` BIGINT UNSIGNED NULL AFTER `due_date`;
