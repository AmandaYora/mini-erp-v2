package integration

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	partyapp "mini-erp/internal/modules/party/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productapp "mini-erp/internal/modules/product/application"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// F4-7 / F4-17: list and report paths issue a CONSTANT number of statements
// whatever the number of rows. Each case measures the same request over a
// small and a large data set (after one warm-up call, so one-time caches do
// not count) and requires identical statement counts. A reintroduced
// per-row lookup (the N+1 this task removed) turns the test red.

var productTables = regexp.MustCompile(`(?i)\b(from|join)\s+(products|product_variants|product_categories)\b`)

// measure runs fn once to warm up, then once under the counter.
func measure(t *testing.T, ctr *stmtCounter, name string, fn func() error) int {
	t.Helper()
	if err := fn(); err != nil {
		t.Fatalf("%s (warm-up): %v", name, err)
	}
	ctr.Reset()
	if err := fn(); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return ctr.Count()
}

func requireConstant(t *testing.T, name string, small, large int) {
	t.Helper()
	if small == 0 {
		t.Fatalf("%s: counter saw no statements (harness broken)", name)
	}
	if small != large {
		t.Errorf("%s: %d statements with few rows, %d with many (must be constant)", name, small, large)
		return
	}
	t.Logf("%s: %d statements (constant)", name, small)
}

// TestF4ListAndReportQueryCountsAreConstant creates delivered, taxed sales
// orders (half of them paid) and measures sales list, payment list/ledger,
// aging and the tax detail report at 5 and at 40 documents.
func TestF4ListAndReportQueryCountsAreConstant(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	fx := newShop(t, ctx)
	m := testMods
	cm, ctr := countedMods(t)

	// Enough stock for every order below (fixture adds 50).
	var created int
	create := func(n int) {
		for i := 0; i < n; i++ {
			created++
			so, err := m.sales.Service().CreateOrder(ctx, 0, fx.branchID, salesapp.OrderInput{
				PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod",
				TaxType: "exclude", TaxRate: 11,
				Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
			})
			if err != nil {
				t.Fatalf("create SO %d: %v", created, err)
			}
			if _, err := m.sales.Service().ConfirmOrder(ctx, 0, fx.branchID, so.ID); err != nil {
				t.Fatalf("confirm SO %d: %v", created, err)
			}
			deliverAll(t, ctx, fx, so.ID, 1)
			var noteID int64
			if err := m.db.QueryRowContext(ctx, "SELECT id FROM delivery_notes WHERE sales_order_id = ?", so.ID).Scan(&noteID); err != nil {
				t.Fatalf("note of SO %d: %v", so.ID, err)
			}
			if _, err := m.finance.Service().Post(ctx, 0, fx.branchID, "delivery", noteID); err != nil {
				t.Fatalf("post delivery %d: %v", noteID, err)
			}
			if created%2 == 0 {
				if _, err := m.payment.Service().Create(ctx, 0, fx.branchID, fx.customerID, "cash", "2026-09-11", "lunas",
					so.GrandTotal, []paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: so.ID, Amount: so.GrandTotal}}); err != nil {
					t.Fatalf("pay SO %d: %v", so.ID, err)
				}
			}
		}
	}

	type probe struct {
		name string
		fn   func() error
	}
	probes := []probe{
		{"sales list", func() error {
			r, err := cm.sales.Service().List(ctx, fx.branchID, salesapp.ListFilter{}, 1, 100)
			if err == nil && r == nil {
				err = fmt.Errorf("nil result")
			}
			return err
		}},
		{"payment list (filtered)", func() error {
			_, err := cm.payment.Service().ListFiltered(ctx, fx.branchID, paymentapp.ListFilter{}, 1, 100)
			return err
		}},
		{"payment ledger by party", func() error {
			_, _, err := cm.payment.Service().Ledger(ctx, fx.branchID, fx.customerID, 1, 100)
			return err
		}},
		{"receivable aging documents", func() error {
			_, err := cm.finance.Service().AgingDocuments(ctx, fx.branchID, "receivable", "2026-12-01")
			return err
		}},
		{"dashboard summary", func() error {
			start := time.Now()
			_, err := cm.dashboard.Service().Summary(ctx, fx.branchID)
			t.Logf("dashboard summary wall time: %v", time.Since(start))
			return err
		}},
		{"tax detail report", func() error {
			rows, _, err := cm.finance.Service().TaxDetailReport(ctx, fx.branchID, 2026, 9, 1, 500)
			if err == nil && len(rows) == 0 {
				err = fmt.Errorf("no tax rows: fixture did not post")
			}
			return err
		}},
	}

	create(5)
	small := map[string]int{}
	for _, p := range probes {
		small[p.name] = measure(t, ctr, p.name, p.fn)
	}
	create(35)
	for _, p := range probes {
		requireConstant(t, p.name, small[p.name], measure(t, ctr, p.name, p.fn))
	}
}

// TestF4PricingReadsProductsInOneBatch: creating a document with 3 or 30
// lines reads the product tables the same number of times (priceLines uses
// ProductClient.GetByIDs once, not GetByID + ResolveUOM per line).
func TestF4PricingReadsProductsInOneBatch(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	fx := newShop(t, ctx)
	m := testMods
	cm, ctr := countedMods(t)

	var items []salesapp.LineInput
	var pitems []purchasingapp.LineInput
	items = append(items, salesapp.LineInput{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1})
	pitems = append(pitems, purchasingapp.LineInput{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1, UnitPrice: 1000})
	for i := 0; i < 29; i++ {
		p, err := m.product.Service().CreateProduct(ctx, 0, productapp.ProductInput{
			Code: fmt.Sprintf("QC-%03d", i), Name: fmt.Sprintf("Barang %d", i), Type: "barang", Tracked: true,
			BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs", PurchaseFactor: 1, SalesFactor: 1,
			PurchasePrice: 10000, SellingPrice: 15000,
		})
		if err != nil {
			t.Fatalf("create product: %v", err)
		}
		items = append(items, salesapp.LineInput{ProductID: p.ID, VariantID: p.Variants[0].ID, UOM: "pcs", Qty: 1})
		pitems = append(pitems, purchasingapp.LineInput{ProductID: p.ID, VariantID: p.Variants[0].ID, UOM: "pcs", Qty: 1, UnitPrice: 1000})
	}
	sup, err := m.party.Service().CreateSupplier(ctx, 0, partyapp.PartyInput{Name: "Supplier Uji"})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}

	salesCount := func(n int) int {
		ctr.Reset()
		if _, err := cm.sales.Service().CreateOrder(ctx, 0, fx.branchID, salesapp.OrderInput{
			PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none", Items: items[:n],
		}); err != nil {
			t.Fatalf("sales create %d lines: %v", n, err)
		}
		return ctr.CountMatching(productTables)
	}
	purchCount := func(n int) int {
		ctr.Reset()
		if _, err := cm.purchasing.Service().CreateOrder(ctx, 0, fx.branchID, purchasingapp.OrderInput{
			PartyID: sup.ID, PaymentTerms: "cod", TaxType: "none", Items: pitems[:n],
		}); err != nil {
			t.Fatalf("purchase create %d lines: %v", n, err)
		}
		return ctr.CountMatching(productTables)
	}
	salesCount(1) // warm-up
	requireConstant(t, "sales CreateOrder product reads (3 vs 30 lines)", salesCount(3), salesCount(30))
	purchCount(1)
	requireConstant(t, "purchasing CreateOrder product reads (3 vs 30 lines)", purchCount(3), purchCount(30))
	if n := salesCount(30); n > 6 {
		t.Errorf("sales CreateOrder reads product tables %d times for 30 lines, want a handful (<= 6)", n)
	}
}
