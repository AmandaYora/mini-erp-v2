package integration

import (
	"context"
	"testing"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	goodsreceiptapp "mini-erp/internal/modules/goodsreceipt/application"
	partyapp "mini-erp/internal/modules/party/application"
	productapp "mini-erp/internal/modules/product/application"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
)

// purchaseProduct creates a second product with NO opening stock, so the
// moving average is exactly the receipts below (the shared fixture product
// carries 50 units that would dilute it).
func purchaseProduct(t *testing.T, fx shopFixture) shopFixture {
	t.Helper()
	return costedProduct(t, fx, "TST-P2", "Barang Uji Beli")
}

// costedProduct creates a tracked product with no stock on hand.
func costedProduct(t *testing.T, fx shopFixture, code, name string) shopFixture {
	t.Helper()
	p, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: code, Name: name, Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs",
		PurchaseFactor: 1, SalesFactor: 1, PurchasePrice: 10000, SellingPrice: 15000,
	})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	if len(p.Variants) == 0 {
		t.Fatal("product has no default variant")
	}
	fx.productID = p.ID
	fx.variantID = p.Variants[0].ID
	return fx
}

// receivePO builds supplier + confirmed PO + full GR receipt (tax-none,
// no discount: unit cost == unit price exactly).
func receivePO(t *testing.T, fx shopFixture, unitPrice int64, qty float64) int64 {
	t.Helper()
	m := testMods
	sup, err := m.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{
		Name: "Supplier Uji",
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	po, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", DueDate: "2026-10-10",
		TaxType: "none",
		Items: []purchasingapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs",
			Qty: qty, UnitPrice: unitPrice,
		}},
	})
	if err != nil {
		t.Fatalf("create PO: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po.ID); err != nil {
		t.Fatalf("confirm PO: %v", err)
	}
	if _, err := m.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, po.ID, "2026-09-10", "",
		[]goodsreceiptapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: qty,
		}}); err != nil {
		t.Fatalf("receive PO: %v", err)
	}
	return po.ID
}

// purchaseFixture builds supplier + confirmed PO + full GR receipt.
func purchaseFixture(t *testing.T, ctx context.Context, fx shopFixture, unitPrice int64, qty float64) int64 {
	t.Helper()
	m := testMods
	sup, err := m.party.Service().CreateSupplier(ctx, 0, partyapp.PartyInput{
		Name: "Supplier Uji",
	})
	if err != nil {
		t.Fatalf("create supplier: %v", err)
	}
	po, err := m.purchasing.Service().CreateOrder(ctx, 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", DueDate: "2026-10-10",
		TaxType: "exclude", TaxRate: 11,
		Items: []purchasingapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs",
			Qty: qty, UnitPrice: unitPrice, DiscountPct: 10,
		}},
	})
	if err != nil {
		t.Fatalf("create PO: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx, 0, fx.branchID, po.ID); err != nil {
		t.Fatalf("confirm PO: %v", err)
	}
	if _, err := m.goodsreceipt.Service().Create(ctx, 0, fx.branchID, po.ID, "2026-09-10", "",
		[]goodsreceiptapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: qty,
		}}); err != nil {
		t.Fatalf("receive PO: %v", err)
	}
	return po.ID
}

func journalSums(entry *financecontracts.JournalEntry) map[string][2]int64 {
	out := map[string][2]int64{}
	for _, l := range entry.Lines {
		s := out[l.AccountCode]
		s[0] += l.Debit
		s[1] += l.Credit
		out[l.AccountCode] = s
	}
	return out
}

// TestPurchaseReturnBothDiffSigns covers the B4 formula change
// (diff = subtotal − relieved, never total − relieved) on both sides:
// returning the expensive PO credits 6200, returning the cheap one debits it.
func TestPurchaseReturnBothDiffSigns(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods

	cheapPO := purchaseFixture(t, ctx(), fx, 10000, 2)
	dearPO := purchaseFixture(t, ctx(), fx, 20000, 2)
	// Cheap: gross 20000 − 2000 = 18000 +1980 = 19980.
	// Dear:  gross 40000 − 4000 = 36000 +3960 = 39960.
	// Moving average after both receipts: (18000+36000)/4 = 13500/unit.

	dear, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, dearPO, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}})
	if err != nil {
		t.Fatalf("create dear return: %v", err)
	}
	if dear.Subtotal != 36000 || dear.TaxTotal != 3960 || dear.Total != 39960 {
		t.Fatalf("dear header = %d/%d/%d, want 36000/3960/39960",
			dear.Subtotal, dear.TaxTotal, dear.Total)
	}
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, dear.ID); err != nil {
		t.Fatalf("confirm dear return: %v", err)
	}
	dearEntry, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "purchase_return", dear.ID)
	if err != nil {
		t.Fatalf("post dear return: %v", err)
	}
	legs := journalSums(dearEntry)
	// relieved = 2 × 13500 = 27000; diff = 36000 − 27000 = +9000 → Cr 6200.
	if legs["2100"] != [2]int64{39960, 0} {
		t.Fatalf("2100 legs = %v, want debit 39960", legs["2100"])
	}
	if legs["1400"] != [2]int64{0, 3960} {
		t.Fatalf("1400 legs = %v, want credit 3960", legs["1400"])
	}
	if legs["1300"] != [2]int64{0, 27000} {
		t.Fatalf("1300 legs = %v, want credit 27000", legs["1300"])
	}
	if legs["6200"] != [2]int64{0, 9000} {
		t.Fatalf("6200 legs = %v, want credit 9000 (diff>0)", legs["6200"])
	}

	cheap, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, cheapPO, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}})
	if err != nil {
		t.Fatalf("create cheap return: %v", err)
	}
	if _, err := m.purchasereturn.Service().Confirm(ctx(), 0, fx.branchID, cheap.ID); err != nil {
		t.Fatalf("confirm cheap return: %v", err)
	}
	cheapEntry, err := m.finance.Service().Post(ctx(), 0, fx.branchID, "purchase_return", cheap.ID)
	if err != nil {
		t.Fatalf("post cheap return: %v", err)
	}
	legs = journalSums(cheapEntry)
	// relieved still 2 × 13500 = 27000 (average unchanged by returns);
	// diff = 18000 − 27000 = −9000 → Dr 6200.
	if legs["2100"] != [2]int64{19980, 0} {
		t.Fatalf("2100 legs = %v, want debit 19980", legs["2100"])
	}
	if legs["1400"] != [2]int64{0, 1980} {
		t.Fatalf("1400 legs = %v, want credit 1980", legs["1400"])
	}
	if legs["1300"] != [2]int64{0, 27000} {
		t.Fatalf("1300 legs = %v, want credit 27000", legs["1300"])
	}
	if legs["6200"] != [2]int64{9000, 0} {
		t.Fatalf("6200 legs = %v, want debit 9000 (diff<0)", legs["6200"])
	}
}
