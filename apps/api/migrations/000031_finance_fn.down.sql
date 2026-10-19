ALTER TABLE `business_expenses`
  DROP KEY `ix_business_expenses_party`,
  DROP COLUMN `party_id`;
DROP TABLE IF EXISTS `finance_tax_adjustment_meta`;
DROP TABLE IF EXISTS `finance_posting_ignores`;
