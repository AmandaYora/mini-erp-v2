package application

import (
	"strings"
	"testing"

	"mini-erp/internal/modules/product/contracts"
)

func hiddenDefault(id int64, code string) *contracts.Variant {
	return &contracts.Variant{ID: id, ProductID: 1, Code: code, Name: "P", IsDefault: true, IsHidden: true, Status: "active"}
}

func visible(id int64, code string, def bool) *contracts.Variant {
	return &contracts.Variant{ID: id, ProductID: 1, Code: code, Name: code, IsDefault: def, Status: "active"}
}

func TestPlanVariantsFreshProductGetsHiddenDefault(t *testing.T) {
	out, err := planVariants(variantPlanInput{ProductCode: "P1", ProductName: "Produk"})
	if err != nil || len(out) != 1 || out[0].ID != 0 || !out[0].IsHidden || !out[0].IsDefault || out[0].Code != "P1" {
		t.Fatalf("got %+v %v", out, err)
	}
}

func TestPlanVariantsKeepWhenPayloadOmitted(t *testing.T) {
	ex := []*contracts.Variant{visible(5, "S", true), visible(6, "M", false)}
	out, err := planVariants(variantPlanInput{ProductCode: "P1", OldProductCode: "P1", ProductName: "P", Existing: ex})
	if err != nil || len(out) != 0 {
		t.Fatalf("omitted payload must not touch variants: %+v %v", out, err)
	}
}

func TestPlanVariantsCodeFollowsOnlyWithoutHistory(t *testing.T) {
	ex := []*contracts.Variant{hiddenDefault(5, "OLD")}
	out, _ := planVariants(variantPlanInput{ProductCode: "NEW", OldProductCode: "OLD", ProductName: "P", Existing: ex})
	if len(out) != 1 || out[0].Code != "NEW" {
		t.Fatalf("no history: default must follow, got %+v", out)
	}
	out, _ = planVariants(variantPlanInput{ProductCode: "NEW", OldProductCode: "OLD", ProductName: "P", Existing: ex,
		States: map[int64]contracts.VariantStockState{5: {HasHistory: true}}})
	for _, v := range out {
		if v.Code != "OLD" {
			t.Fatalf("history: default code must stay frozen, got %+v", v)
		}
	}
}

func TestPlanVariantsNeverArchivesStockOrHistory(t *testing.T) {
	ex := []*contracts.Variant{visible(5, "S", true), visible(6, "M", false)}
	payload := []VariantInput{{ID: 5, Code: "S", Name: "S", IsDefault: true}}

	// M holds stock: refused with the legacy message.
	_, err := planVariants(variantPlanInput{ProductCode: "P", ProductName: "P", Existing: ex, Payload: payload,
		States: map[int64]contracts.VariantStockState{6: {OnHand: 3, HasHistory: true}}})
	if err == nil || !strings.Contains(errMsg(err), "masih punya stok") {
		t.Fatalf("stock variant removal must be refused, got %v", err)
	}
	// M has history only: not removed silently either.
	_, err = planVariants(variantPlanInput{ProductCode: "P", ProductName: "P", Existing: ex, Payload: payload,
		States: map[int64]contracts.VariantStockState{6: {HasHistory: true}}})
	if err == nil || !strings.Contains(errMsg(err), "riwayat") {
		t.Fatalf("history variant removal must be refused, got %v", err)
	}
	// ... but it can be hidden explicitly.
	out, err := planVariants(variantPlanInput{ProductCode: "P", ProductName: "P", Existing: ex,
		Payload: []VariantInput{{ID: 5, Code: "S", Name: "S", IsDefault: true}, {ID: 6, Code: "M", Name: "M", Status: "inactive"}},
		States:  map[int64]contracts.VariantStockState{6: {HasHistory: true}}})
	if err != nil {
		t.Fatal(err)
	}
	var m *contracts.Variant
	for _, v := range out {
		if v.ID == 6 {
			m = v
		}
	}
	if m == nil || m.Status != "inactive" {
		t.Fatalf("M must be inactive, got %+v", out)
	}
	// Inactive with stock is still refused.
	_, err = planVariants(variantPlanInput{ProductCode: "P", ProductName: "P", Existing: ex,
		Payload: []VariantInput{{ID: 5, Code: "S", Name: "S", IsDefault: true}, {ID: 6, Code: "M", Name: "M", Status: "inactive"}},
		States:  map[int64]contracts.VariantStockState{6: {OnHand: 1}}})
	if err == nil {
		t.Fatal("hiding a variant that holds stock must be refused")
	}
	// A never-used variant absent from the payload is archived.
	out, err = planVariants(variantPlanInput{ProductCode: "P", ProductName: "P", Existing: ex, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	archived := false
	for _, v := range out {
		if v.ID == 6 && v.Status == "archived" {
			archived = true
		}
	}
	if !archived {
		t.Fatalf("unused variant must be archived: %+v", out)
	}
}

func TestPlanVariantsRenameLockedByHistory(t *testing.T) {
	ex := []*contracts.Variant{visible(5, "S", true)}
	_, err := planVariants(variantPlanInput{ProductCode: "P", ProductName: "P", Existing: ex,
		Payload: []VariantInput{{ID: 5, Code: "SMALL", Name: "S"}},
		States:  map[int64]contracts.VariantStockState{5: {HasHistory: true}}})
	if err == nil || !strings.Contains(errMsg(err), "terkunci") {
		t.Fatalf("rename of a variant with history must be refused: %v", err)
	}
	out, err := planVariants(variantPlanInput{ProductCode: "P", ProductName: "P", Existing: ex,
		Payload: []VariantInput{{ID: 5, Code: "SMALL", Name: "S"}}})
	if err != nil || len(out) != 1 || out[0].Code != "SMALL" || out[0].ID != 5 {
		t.Fatalf("rename without history keeps the id: %+v %v", out, err)
	}
}

func TestPlanVariantsHiddenDefaultStaysHiddenWhenEchoed(t *testing.T) {
	ex := []*contracts.Variant{hiddenDefault(5, "P1")}
	out, err := planVariants(variantPlanInput{ProductCode: "P1", OldProductCode: "P1", ProductName: "P", Existing: ex,
		Payload: []VariantInput{{ID: 5, Code: "P1", Name: "P", IsDefault: true}}})
	if err != nil || len(out) != 1 || !out[0].IsHidden || !out[0].IsDefault {
		t.Fatalf("echoed default must stay hidden: %+v %v", out, err)
	}
}

func TestPlanVariantsExplicitVariantsRetireDefault(t *testing.T) {
	ex := []*contracts.Variant{hiddenDefault(5, "P1")}
	payload := []VariantInput{{Code: "A", Name: "A", IsDefault: true}, {Code: "B", Name: "B"}}
	// No stock/history on the default: it is archived.
	out, err := planVariants(variantPlanInput{ProductCode: "P1", OldProductCode: "P1", ProductName: "P", Existing: ex, Payload: payload})
	if err != nil {
		t.Fatal(err)
	}
	var def, inserts int
	for _, v := range out {
		if v.ID == 5 && v.Status == "archived" {
			def++
		}
		if v.ID == 0 {
			inserts++
		}
	}
	if def != 1 || inserts != 2 {
		t.Fatalf("default archived=%d inserts=%d in %+v", def, inserts, out)
	}
	// Stock on the default: refused (it would be orphaned).
	_, err = planVariants(variantPlanInput{ProductCode: "P1", OldProductCode: "P1", ProductName: "P", Existing: ex, Payload: payload,
		States: map[int64]contracts.VariantStockState{5: {OnHand: 2, HasHistory: true}}})
	if err == nil {
		t.Fatal("activating variants while the default holds stock must be refused")
	}
	// History only: the default stays active and hidden (scannable by product code).
	out, err = planVariants(variantPlanInput{ProductCode: "P1", OldProductCode: "P1", ProductName: "P", Existing: ex, Payload: payload,
		States: map[int64]contracts.VariantStockState{5: {HasHistory: true}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range out {
		if v.ID == 5 && (v.Status != "active" || !v.IsHidden) {
			t.Fatalf("history default must stay active+hidden: %+v", v)
		}
	}
	defaults := 0
	for _, v := range out {
		if v.IsDefault && v.Status == "active" {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("exactly one default expected, got %d in %+v", defaults, out)
	}
}

func TestNormalizeInputTypeRules(t *testing.T) {
	in := normalizeInput(ProductInput{Code: " ab ", Name: " X ", Type: "jasa", Tracked: true,
		BaseUOM: "jam", PurchaseUOM: "dus", SalesUOM: "dus", PurchaseFactor: 12, SalesFactor: 12})
	if in.Code != "AB" || in.Tracked || in.PurchaseUOM != "jam" || in.SalesUOM != "jam" || in.PurchaseFactor != 1 || in.SalesFactor != 1 {
		t.Fatalf("jasa must be untracked, one unit, factor 1: %+v", in)
	}
	in = normalizeInput(ProductInput{Type: "barang", Tracked: true, BaseUOM: "pcs", PurchaseUOM: "dus", PurchaseFactor: 12})
	if !in.Tracked || in.PurchaseFactor != 12 {
		t.Fatalf("barang keeps its configuration: %+v", in)
	}
	for _, typ := range []string{"paket", "non_stok"} {
		if !contracts.ValidType(typ) {
			t.Errorf("%s must be a valid type", typ)
		}
	}
	if contracts.ValidType("physical") {
		t.Error("legacy names are mapped by import, not accepted by the API")
	}
}

func TestCheckUpdateRules(t *testing.T) {
	ex := &contracts.Product{Type: "barang", Tracked: true, BaseUOM: "pcs", PurchaseUOM: "dus", SalesUOM: "pcs",
		PurchaseFactor: 12, SalesFactor: 1, Status: "active"}
	same := ProductInput{Type: "barang", Tracked: true, BaseUOM: "pcs", PurchaseUOM: "dus", SalesUOM: "pcs", PurchaseFactor: 12, SalesFactor: 1}
	if ae := checkUpdateRules(ex, same, contracts.StockState{HasHistory: true}); ae != nil {
		t.Fatalf("unchanged config with history is fine: %v", ae)
	}
	changed := same
	changed.PurchaseFactor = 6
	if ae := checkUpdateRules(ex, changed, contracts.StockState{HasHistory: true}); ae == nil {
		t.Fatal("factor change with history must be refused")
	}
	untrack := same
	untrack.Tracked = false
	if ae := checkUpdateRules(ex, untrack, contracts.StockState{Available: 3}); ae == nil {
		t.Fatal("untracking with stock must be refused")
	}
	if ae := checkUpdateRules(ex, untrack, contracts.StockState{}); ae != nil {
		t.Fatalf("untracking a product without stock or history is allowed: %v", ae)
	}
	inactive := same
	inactive.Status = "inactive"
	if ae := checkUpdateRules(ex, inactive, contracts.StockState{HasHolds: true}); ae == nil {
		t.Fatal("deactivating with live holds must be refused")
	}
}
