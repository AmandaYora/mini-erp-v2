package infrastructure

import (
	"context"
	"database/sql"

	"mini-erp/internal/shared/dbutil"
)

// RefPurchasePriceCorrection is the ledger ref_type of the value-only rows a
// purchase price correction writes (ref_id = the correction id): the received
// goods still on hand change value with no quantity moving. Written in the same
// transaction as the correction's journal, so the Persediaan legs and these
// rows are always equal (ADR-0020, ADR-0022).
const RefPurchasePriceCorrection = "purchase_price_correction"

// InsertValueRowTx writes one value-only cost row inside the caller's
// transaction: qty 0, total = value (signed), unit 0 (it prices no unit, so it
// never becomes a position's "last inflow cost"). id comes from
// NextSyntheticCostIDs.
func (r *Repository) InsertValueRowTx(ctx context.Context, tx *sql.Tx, branchID, productID, variantID, id int64, refType string, refID, value int64, movedAt string) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO finance_inventory_cost_movements
		 (branch_id, product_id, variant_id, stock_movement_id, ref_type, ref_id, direction,
		  qty_base, unit_cost, total_cost, is_estimated, moved_at)
		 VALUES (?, ?, ?, ?, ?, ?, 'in', 0, 0, ?, 0, ?)`,
		branchID, productID, variantID, id, refType, refID, value, dbutil.NullIfEmpty(normalizeStamp(movedAt)))
	return err
}
