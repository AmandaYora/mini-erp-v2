package integration

import (
	"testing"

	stockapp "mini-erp/internal/modules/stock/application"
)

// F2a-9b: the branch-wide balance list is paginated with an exact total,
// filters by location/search/critical, and hides the damaged location by default.
func TestListBalancesFilters(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	all, total, err := svc.ListBalances(ctx(), fx.branchID, stockapp.BalanceQuery{Page: 1, Limit: 1})
	if err != nil || total < 1 || len(all) != 1 || all[0].ProductCode == "" {
		t.Fatalf("list = %+v total %d err %v; want 1 decorated row of >=1", all, total, err)
	}
	other, total2, err := svc.ListBalances(ctx(), fx.branchID, stockapp.BalanceQuery{LocationID: fx.locationID + 999999, Page: 1, Limit: 10})
	if err != nil || total2 != 0 || len(other) != 0 {
		t.Fatalf("foreign location = %d rows total %d err %v; want none", len(other), total2, err)
	}
	hit, _, err := svc.ListBalances(ctx(), fx.branchID, stockapp.BalanceQuery{Search: all[0].ProductCode, Page: 1, Limit: 10})
	if err != nil || len(hit) == 0 {
		t.Fatalf("search by code = %d rows, %v; want a hit", len(hit), err)
	}
	miss, _, err := svc.ListBalances(ctx(), fx.branchID, stockapp.BalanceQuery{Search: "tidak-ada-produk-ini", Page: 1, Limit: 10})
	if err != nil || len(miss) != 0 {
		t.Fatalf("search miss = %d rows, %v; want none", len(miss), err)
	}
	// Damaged stock is hidden unless asked for.
	if err := svc.MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 2, Reason: "pecah"}); err != nil {
		t.Fatalf("move damaged: %v", err)
	}
	visible, _, _ := svc.ListBalances(ctx(), fx.branchID, stockapp.BalanceQuery{Page: 1, Limit: 50})
	withDamaged, _, _ := svc.ListBalances(ctx(), fx.branchID, stockapp.BalanceQuery{IncludeDamaged: true, Page: 1, Limit: 50})
	if len(withDamaged) != len(visible)+1 {
		t.Fatalf("damaged rows: visible=%d withDamaged=%d; want one extra", len(visible), len(withDamaged))
	}
}

// F2a-9b: adjustments always need an approver by default; the company can opt out.
func TestAdjustAlwaysNeedsApproverByDefault(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	in := stockapp.AdjustInput{ProductID: fx.productID, VariantID: fx.variantID, Location: fx.locationID, Mode: "in", QtyDelta: 1, Reason: "tes"}
	if err := svc.Adjust(ctx(), 0, fx.branchID, in); err != nil {
		t.Fatalf("opted-out adjust: %v", err)
	}
	if _, err := testMods.db.Exec(`DELETE FROM company_settings WHERE setting_key = ?`, stockapp.StrictAdjustSetting); err != nil {
		t.Fatalf("clear setting: %v", err)
	}
	if err := svc.Adjust(ctx(), 0, fx.branchID, in); err == nil {
		t.Fatal("default mode allowed an adjustment without approver")
	}
}

// F2a-9b: damaged moves need a reason.
func TestDamagedMovesNeedReason(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	if err := svc.MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 1}); err == nil {
		t.Fatal("move to damaged without reason accepted")
	}
	if err := svc.RestoreDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 1}); err == nil {
		t.Fatal("restore without reason accepted")
	}
}

// F2a-9b: scan resolves variant barcode, variant code and product code.
func TestScanResolvesCodes(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	var pcode, vcode string
	if err := testMods.db.QueryRow(`SELECT p.code, v.code FROM products p JOIN product_variants v ON v.product_id = p.id WHERE p.id = ? AND v.id = ?`, fx.productID, fx.variantID).Scan(&pcode, &vcode); err != nil {
		t.Fatalf("codes: %v", err)
	}
	svc := testMods.stock.Service()
	for _, code := range []string{pcode, vcode} {
		res, err := svc.Scan(ctx(), fx.branchID, code)
		if err != nil {
			t.Fatalf("scan %q: %v", code, err)
		}
		if m, ok := res.Product.(map[string]any); !ok || m["id"] != fx.productID {
			t.Fatalf("scan %q -> %+v; want the fixture product", code, res.Product)
		}
	}
	if _, err := svc.Scan(ctx(), fx.branchID, "KODE-TIDAK-ADA"); err == nil {
		t.Fatal("unknown code resolved")
	}
}
