package integration

import (
	"testing"
	"time"

	stockapp "mini-erp/internal/modules/stock/application"
)

// Temuan 6 (2026-10-23): a correction changed on_hand but left
// stock_balances.updated_at at the row's first write, so the row looked
// untouched when the incident was traced. Every balance shift now stamps it.
func TestStockBalanceShiftStampsUpdatedAt(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	stamp := func() time.Time {
		t.Helper()
		var at time.Time
		if err := testMods.db.QueryRow(`SELECT updated_at FROM stock_balances
			WHERE product_id = ? AND variant_id = ? AND location_id = ?`, fx.productID, fx.variantID, fx.locationID).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	adjust := func(delta float64) {
		t.Helper()
		if err := testMods.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
			ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID, Mode: "in", QtyDelta: delta, Reason: "uji cap waktu saldo",
		}); err != nil {
			t.Fatal(err)
		}
	}
	adjust(5)
	old := time.Date(2026, 10, 19, 7, 6, 51, 0, time.UTC)
	if _, err := testMods.db.Exec(`UPDATE stock_balances SET updated_at = ? WHERE product_id = ? AND variant_id = ? AND location_id = ?`,
		old, fx.productID, fx.variantID, fx.locationID); err != nil {
		t.Fatal(err)
	}
	adjust(3)
	if at := stamp(); !at.After(old) {
		t.Fatalf("updated_at = %v after a correction, want later than %v", at, old)
	}
}
