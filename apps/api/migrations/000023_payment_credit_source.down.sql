ALTER TABLE `payments`
  DROP KEY `ix_payments_credit_source`,
  DROP COLUMN `credit_source_id`,
  DROP COLUMN `credit_source_type`;
