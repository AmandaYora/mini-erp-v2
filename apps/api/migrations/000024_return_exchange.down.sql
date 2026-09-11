ALTER TABLE delivery_notes
  DROP KEY ix_delivery_notes_return,
  DROP COLUMN sales_return_id,
  DROP COLUMN document_kind;

DROP TABLE IF EXISTS sales_return_replacement_items;

ALTER TABLE sales_returns
  DROP COLUMN replacement_delivery_status,
  DROP COLUMN return_mode;
