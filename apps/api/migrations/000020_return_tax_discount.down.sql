ALTER TABLE purchase_return_items
  DROP COLUMN tax_amount,
  DROP COLUMN tax_base,
  DROP COLUMN discount_nominal,
  DROP COLUMN discount_pct;

ALTER TABLE purchase_returns
  DROP COLUMN tax_rate,
  DROP COLUMN tax_type,
  DROP COLUMN tax_total,
  DROP COLUMN discount_total,
  DROP COLUMN subtotal;

ALTER TABLE sales_return_items
  DROP COLUMN tax_amount,
  DROP COLUMN tax_base,
  DROP COLUMN discount_nominal,
  DROP COLUMN discount_pct;

ALTER TABLE sales_returns
  DROP COLUMN tax_rate,
  DROP COLUMN tax_type,
  DROP COLUMN tax_total,
  DROP COLUMN discount_total,
  DROP COLUMN subtotal;
