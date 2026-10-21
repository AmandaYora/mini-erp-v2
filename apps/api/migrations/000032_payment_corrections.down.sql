ALTER TABLE `payments`
  DROP KEY `ix_payments_corrects`,
  DROP COLUMN `cancelled_by`,
  DROP COLUMN `cancelled_at`,
  DROP COLUMN `cancel_reason`,
  DROP COLUMN `corrects_payment_id`;
