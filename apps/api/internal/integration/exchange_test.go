package integration

import (
	"testing"

	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
)

// TestExchangeReplacementFlow locks the D4 happy path: replacement stock
// does NOT move at return creation, moves at replacement dispatch (confirm),
// and the replacement SJ is excluded from auto-journaling.
func TestExchangeReplacementFlow(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	// Available now 48 (50 opening − 2 delivered).

	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}},
		[]salesreturnapp.ReplacementInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 1,
		}})
	if err != nil {
		t.Fatalf("create exchange: %v", err)
	}
	if ret.ReturnMode != "exchange" || ret.ReplacementDeliveryStatus != "pending" {
		t.Fatalf("mode=%q replStatus=%q, want exchange/pending", ret.ReturnMode, ret.ReplacementDeliveryStatus)
	}
	if len(ret.ReplacementItems) != 1 || ret.ReplacementItems[0].UnitPrice != 15000 {
		t.Fatalf("replacement not priced from catalog: %+v", ret.ReplacementItems)
	}
	avail := available(t, fx)
	if avail != 48 {
		t.Fatalf("stock moved at return creation: available=%v, want 48", avail)
	}

	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	if avail := available(t, fx); avail != 50 {
		t.Fatalf("restock missing: available=%v, want 50", avail)
	}

	dn, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.ReplacementDispatchInput{})
	if err != nil {
		t.Fatalf("dispatch replacement: %v", err)
	}
	if dn.DocumentKind != deliverycontracts.DocumentKindReplacement || dn.SalesReturnID != ret.ID {
		t.Fatalf("replacement link broken: %+v", dn)
	}
	if dn.Status != "draft" {
		t.Fatalf("replacement status=%q, want draft", dn.Status)
	}
	if avail := available(t, fx); avail != 50 {
		t.Fatalf("draft replacement moved stock: available=%v, want 50", avail)
	}
	afterDispatch, err := m.salesreturn.Service().GetByID(ctx(), ret.ID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if afterDispatch.ReplacementDeliveryStatus != "dispatched" {
		t.Fatalf("replStatus=%q, want dispatched", afterDispatch.ReplacementDeliveryStatus)
	}

	// Second dispatch must fail on status, not on unique key.
	if _, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.ReplacementDispatchInput{}); err == nil {
		t.Fatal("expected rejection for double dispatch, got nil")
	}

	confirmed, err := m.salesreturn.Service().ConfirmReplacementDelivery(ctx(), 0, fx.branchID, ret.ID, dn.ID,
		deliverycontracts.ReplacementConfirm{})
	if err != nil {
		t.Fatalf("confirm replacement: %v", err)
	}
	if confirmed.Status != "confirmed" {
		t.Fatalf("delivery status=%q, want confirmed", confirmed.Status)
	}
	if avail := available(t, fx); avail != 49 {
		t.Fatalf("replacement stock out missing: available=%v, want 49", avail)
	}
	done, err := m.salesreturn.Service().GetByID(ctx(), ret.ID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if done.ReplacementDeliveryStatus != "confirmed" {
		t.Fatalf("replStatus=%q, want confirmed", done.ReplacementDeliveryStatus)
	}

	if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "delivery", dn.ID); err == nil {
		t.Fatal("expected rejection posting replacement delivery, got nil")
	}
}

// TestPreviewMatchesCreate locks D5's no-drift rule for both return modules.
func TestPreviewMatchesCreate(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 4)
	deliverAll(t, ctx(), fx, soID, 4)
	lines := []salesreturnapp.LineInput{{
		ProductID: fx.productID, VariantID: fx.variantID,
		LocationID: fx.locationID, UOM: "pcs", Qty: 3,
	}}
	prev, err := m.salesreturn.Service().PreviewReturn(ctx(), fx.branchID, soID, "return_only", lines, nil)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	created := createSalesReturn(t, fx, soID, 3)
	if prev.Subtotal != created.Subtotal || prev.DiscountTotal != created.DiscountTotal ||
		prev.TaxTotal != created.TaxTotal || prev.Total != created.Total {
		t.Fatalf("preview %+v != created %+v", prev, created)
	}
	if len(prev.Items) != len(created.Items) || prev.Items[0].LineTotal != created.Items[0].LineTotal {
		t.Fatalf("preview lines drifted from created lines")
	}

	poID := purchaseFixture(t, ctx(), fx, 10000, 4)
	plines := []purchasereturnapp.LineInput{{
		ProductID: fx.productID, VariantID: fx.variantID,
		LocationID: fx.locationID, UOM: "pcs", Qty: 3,
	}}
	pprev, err := m.purchasereturn.Service().PreviewReturn(ctx(), fx.branchID, poID, plines)
	if err != nil {
		t.Fatalf("purchase preview: %v", err)
	}
	// Fixture PO: 10% discount, exclude-tax 11%.
	// 3 × 10000 = 30000 gross − 3000 = 27000 net + 2970 tax → 29970.
	if pprev.Subtotal != 27000 || pprev.TaxTotal != 2970 || pprev.Total != 29970 {
		t.Fatalf("purchase preview = %d/%d/%d, want 27000/2970/29970",
			pprev.Subtotal, pprev.TaxTotal, pprev.Total)
	}
	pcreated, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "", plines)
	if err != nil {
		t.Fatalf("create purchase return: %v", err)
	}
	if pprev.Total != pcreated.Total || pprev.Subtotal != pcreated.Subtotal {
		t.Fatalf("purchase preview %+v != created %+v", pprev, pcreated)
	}
}

// TestReplacementDispatchShortStock locks the D4 failure path: a short line
// aborts the WHOLE dispatch and the return stays pending.
func TestReplacementDispatchShortStock(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	empty := costedProduct(t, fx, "TST-EMPTY", "Barang Kosong")
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)

	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}},
		[]salesreturnapp.ReplacementInput{{
			ProductID: empty.productID, VariantID: empty.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 1,
		}})
	if err != nil {
		t.Fatalf("create exchange: %v", err)
	}
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	if _, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.ReplacementDispatchInput{}); err == nil {
		t.Fatal("expected rejection for short stock, got nil")
	}
	after, err := m.salesreturn.Service().GetByID(ctx(), ret.ID)
	if err != nil {
		t.Fatalf("get return: %v", err)
	}
	if after.ReplacementDeliveryStatus != "pending" {
		t.Fatalf("replStatus=%q, want pending after failed dispatch", after.ReplacementDeliveryStatus)
	}
}

// TestSettlementCap locks D2's bound: settlements can never exceed the
// return total, on either module.
func TestSettlementCap(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	ret := createSalesReturn(t, fx, soID, 2)
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: ret.Total + 1}); err == nil {
		t.Fatal("expected rejection for over-settlement, got nil")
	}
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "bogus", Amount: 1000}); err == nil {
		t.Fatal("expected rejection for unknown type, got nil")
	}
	done, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: ret.Total})
	if err != nil {
		t.Fatalf("add settlement: %v", err)
	}
	if len(done.Settlements) != 1 || done.Settlements[0].Amount != ret.Total {
		t.Fatalf("settlement not persisted: %+v", done.Settlements)
	}

	poID := purchaseFixture(t, ctx(), fx, 10000, 2)
	pret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}})
	if err != nil {
		t.Fatalf("create purchase return: %v", err)
	}
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, pret.ID); err != nil {
		t.Fatalf("confirm purchase return: %v", err)
	}
	if _, err := m.purchasereturn.Service().AddSettlement(ctx(), 0, fx.branchID, pret.ID,
		purchasereturnapp.SettlementInput{Type: "refund", Amount: pret.Total + 1}); err == nil {
		t.Fatal("expected rejection for purchase over-settlement, got nil")
	}
	pdone, err := m.purchasereturn.Service().AddSettlement(ctx(), 0, fx.branchID, pret.ID,
		purchasereturnapp.SettlementInput{Type: "refund", Amount: pret.Total})
	if err != nil {
		t.Fatalf("add purchase settlement: %v", err)
	}
	if len(pdone.Settlements) != 1 {
		t.Fatalf("purchase settlement not persisted: %+v", pdone.Settlements)
	}
}

func available(t *testing.T, fx shopFixture) float64 {
	t.Helper()
	bal, err := testMods.stock.Stock().GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err != nil {
		t.Fatalf("balance: %v", err)
	}
	return bal.Available()
}
