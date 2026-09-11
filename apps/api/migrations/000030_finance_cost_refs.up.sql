-- Cost ledger carries its source ref so journals value a document's own
-- movements at their recorded historical cost instead of the live average.
-- Without this, any document that empties a position (full delivery, full
-- return) relieves zero: the average is read after the out-movement that
-- just zeroed it. Queryable columns, no blobs (P1/P2).

ALTER TABLE finance_inventory_cost_movements
  ADD COLUMN ref_type VARCHAR(50) NOT NULL DEFAULT '' AFTER stock_movement_id,
  ADD COLUMN ref_id   BIGINT UNSIGNED NULL AFTER ref_type,
  ADD KEY ix_cost_moves_ref (ref_type, ref_id);
