ALTER TABLE `sales_orders`
  DROP COLUMN `short_closed_by`,
  DROP COLUMN `short_closed_at`,
  DROP COLUMN `short_close_reason`,
  DROP COLUMN `cancelled_by`,
  DROP COLUMN `cancelled_at`,
  DROP COLUMN `cancel_reason`;
