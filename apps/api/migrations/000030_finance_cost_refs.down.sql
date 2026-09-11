ALTER TABLE finance_inventory_cost_movements
  DROP KEY ix_cost_moves_ref,
  DROP COLUMN ref_id,
  DROP COLUMN ref_type;
