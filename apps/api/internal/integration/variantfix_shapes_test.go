package integration

import (
	"errors"
	"strings"
	"testing"

	productapp "mini-erp/internal/modules/product/application"
	salesapp "mini-erp/internal/modules/sales/application"
	"mini-erp/internal/shared/apperror"
)

// variantfix (temuan 2, RUNBOOK §6d) flips flags on rows that already carry
// stock: the legacy "Standar" of a single-variant product becomes hidden, and
// variants the legacy had switched off become inactive. These tests make the
// same flips in SQL after stock exists and prove nothing a user relies on
// moves: the product still sells, scans and is valued; an inactive variant's
// stock stays in balances and in the inventory value.

func standarProduct(t *testing.T, code string) (productID, variantID int64) {
	t.Helper()
	p := f3bProduct(t, productapp.ProductInput{Code: code, Name: "Kabel " + code, Variants: []productapp.VariantInput{
		{Code: code, Name: "Standar", Barcode: code, IsDefault: true},
	}})
	if len(p.Variants) != 1 || p.Variants[0].IsHidden {
		t.Fatalf("variants = %+v, want one visible Standar (the ETL shape before variantfix)", p.Variants)
	}
	return p.ID, p.Variants[0].ID
}

// productValue is one product's share of the inventory value report.
func productValue(t *testing.T, branchID, productID int64) (value int64, variants map[int64]bool) {
	t.Helper()
	rows, _, err := testMods.finance.Service().InventoryValue(ctx(), branchID)
	if err != nil {
		t.Fatal(err)
	}
	variants = map[int64]bool{}
	for _, r := range rows {
		if r.ProductID == productID {
			value += r.Value
			variants[r.VariantID] = true
		}
	}
	return value, variants
}

func TestVariantfixHiddenStandarStillSells(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	pid, vid := standarProduct(t, "KBL-MK-ROL-001")
	seedInCost(t, fx, pid, vid, fx.locationID, 100, 7000)
	if _, err := fin.Sync(ctx(), fx.branchID); err != nil {
		t.Fatal(err)
	}
	if v, _ := productValue(t, fx.branchID, pid); v != 700000 {
		t.Fatalf("inventory value = %d, want 700000", v)
	}
	if _, err := testMods.db.Exec(`UPDATE product_variants SET is_hidden = 1 WHERE id = ?`, vid); err != nil {
		t.Fatal(err)
	}

	for _, code := range []string{"KBL-MK-ROL-001", "kbl-mk-rol-001"} {
		m, err := testMods.product.Service().ResolveScanCode(ctx(), code)
		if err != nil || m == nil || m.Variant == nil || m.Variant.ID != vid || m.VariantRequired {
			t.Fatalf("scan %q = %+v err=%v, want the hidden Standar without asking", code, m, err)
		}
	}
	if v, _ := productValue(t, fx.branchID, pid); v != 700000 {
		t.Fatalf("inventory value = %d after hiding Standar, want 700000", v)
	}
	pos, err := fin.RevaluationPositions(ctx(), fx.branchID, pid)
	if err != nil || len(pos) != 1 || pos[0].VariantID != vid || pos[0].Qty != 100 || pos[0].VariantName != "Varian bawaan" {
		t.Fatalf("revaluation positions = %+v err=%v", pos, err)
	}
	sellAndDeliver(t, fx, pid, vid, 3)
	bal, err := testMods.stock.Service().BalancesByProduct(ctx(), fx.branchID, pid)
	if err != nil || len(bal) != 1 || bal[0].OnHand != 97 {
		t.Fatalf("balances after selling = %+v err=%v, want 97 on the Standar", bal, err)
	}
}

func TestVariantfixInactiveVariantKeepsStockAndValue(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	p := f3bProduct(t, productapp.ProductInput{Code: "GRT-PRS", Name: "Granit Paris", Variants: []productapp.VariantInput{
		{Code: "GRT-PRS-A", Name: "Krem", IsDefault: true},
		{Code: "GRT-PRS-B", Name: "Putih"},
	}})
	var keep, off int64
	for _, v := range p.Variants {
		if v.Code == "GRT-PRS-A" {
			keep = v.ID
		} else {
			off = v.ID
		}
	}
	seedInCost(t, fx, p.ID, keep, fx.locationID, 10, 5000)
	seedInCost(t, fx, p.ID, off, fx.locationID, 4, 5000)
	if _, err := fin.Sync(ctx(), fx.branchID); err != nil {
		t.Fatal(err)
	}
	if v, _ := productValue(t, fx.branchID, p.ID); v != 70000 {
		t.Fatalf("inventory value = %d, want 70000", v)
	}
	if _, err := testMods.db.Exec(`UPDATE product_variants SET status = 'inactive', is_hidden = 1 WHERE id = ?`, off); err != nil {
		t.Fatal(err)
	}

	bal, err := testMods.stock.Service().BalancesByProduct(ctx(), fx.branchID, p.ID)
	var offQty float64
	for _, b := range bal {
		if b.VariantID == off {
			offQty += b.OnHand
		}
	}
	if err != nil || offQty != 4 {
		t.Fatalf("balances = %+v err=%v, want the inactive variant's 4 still listed", bal, err)
	}
	if v, vs := productValue(t, fx.branchID, p.ID); v != 70000 || !vs[off] {
		t.Fatalf("inventory value = %d (variants %v), want 70000 incl. the inactive variant", v, vs)
	}
	pos, err := fin.RevaluationPositions(ctx(), fx.branchID, p.ID)
	if err != nil || len(pos) != 2 {
		t.Fatalf("revaluation positions = %+v err=%v, want both variants", pos, err)
	}
	// Like an archived variant (and like legacy), an inactive one is no longer
	// offered for sale; the active sibling still sells.
	_, err = testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: p.ID, VariantID: off, UOM: "pcs", Qty: 1}},
	})
	var ae *apperror.AppError
	if !errors.As(err, &ae) || len(ae.Fields) == 0 || !strings.Contains(ae.Fields[0].Message, "varian tidak dikenal") {
		t.Fatalf("sale of the inactive variant = %v, want refused", err)
	}
	sellAndDeliver(t, fx, p.ID, keep, 2)
}
