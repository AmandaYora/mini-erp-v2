DROP INDEX ix_finance_lines_account_party ON finance_journal_lines;
DROP INDEX ix_finance_lines_account_product ON finance_journal_lines;

ALTER TABLE finance_journal_lines
  DROP COLUMN description,
  DROP COLUMN party_id,
  DROP COLUMN variant_id,
  DROP COLUMN product_id;
