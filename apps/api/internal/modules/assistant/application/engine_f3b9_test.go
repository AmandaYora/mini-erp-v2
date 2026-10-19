package application

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
)

func TestDetectF3b9(t *testing.T) {
	cases := []struct {
		in       string
		intent   string
		arg      string
		statuses string
	}{
		// sales before performance (legacy order)
		{in: "Penjualan hari ini", intent: IntentSalesSummary},
		{in: "omzet hari ini", intent: IntentSalesSummary},
		{in: "hari ini bagaimana", intent: IntentTodayPerformance},
		{in: "performa", intent: IntentTodayPerformance},
		// help only for a help-like whole message or a feature phrase
		{in: "menu", intent: IntentHelp},
		{in: "apa saja fitur kamu", intent: IntentHelp},
		{in: "menu makan siang", intent: IntentHelp}, // falls through to the default help
		{in: "summary", intent: IntentOperationalSummary},
		{in: "kondisi bisnis", intent: IntentOperationalSummary},
		{in: "kurang bayar", intent: IntentHelp}, // "kurang" no longer means critical stock
		{in: "stok tipis", intent: IntentCriticalStock},
		// order list per status
		{in: "order selesai", intent: IntentOrderStatus, statuses: "completed"},
		{in: "pesanan dibatalkan", intent: IntentOrderStatus, statuses: "cancelled"},
		{in: "transaksi draft", intent: IntentOrderStatus, statuses: "draft"},
		{in: "order diproses", intent: IntentOrderStatus, statuses: "confirmed"},
		{in: "order pending", intent: IntentPendingOrders},
		{in: "cek order", intent: IntentOrderStatus},
		// product info robustness
		{in: "info produk beras", intent: IntentProductInfo, arg: "beras"},
		{in: `info "Beras Premium 5kg"`, intent: IntentProductInfo, arg: "beras premium 5kg"},
		{in: "ada barang gula pasir?", intent: IntentProductInfo, arg: "ada gula pasir?"},
		{in: "stok H-001", intent: IntentProductInfo, arg: "h-001"},
	}
	for _, c := range cases {
		d := Detect(c.in)
		if d.Intent != c.intent {
			t.Errorf("Detect(%q).Intent = %q, want %q", c.in, d.Intent, c.intent)
			continue
		}
		if c.arg != "" && d.Arg != c.arg {
			t.Errorf("Detect(%q).Arg = %q, want %q", c.in, d.Arg, c.arg)
		}
		if c.statuses != "" && strings.Join(d.Statuses, ",") != c.statuses {
			t.Errorf("Detect(%q).Statuses = %v, want %s", c.in, d.Statuses, c.statuses)
		}
	}
}

// --- fakes: only the methods the engine calls; the embedded nil interface
// panics loudly if a test reaches anything else. -----------------------------

type fakeBranches struct{ branchcontracts.BranchClient }

func (fakeBranches) ListAll(context.Context) ([]*branchcontracts.Branch, error) {
	return []*branchcontracts.Branch{{ID: 1, Code: "UTM", Name: "Utama", Status: "active"}}, nil
}

type fakeSales struct {
	salescontracts.SalesOrderClient
	gotStatuses []string
	gotFrom     string
}

func (f *fakeSales) ListOrders(_ context.Context, _ int64, st []string, from, _ string, _ int) ([]*salescontracts.OrderSummary, error) {
	f.gotStatuses, f.gotFrom = st, from
	return []*salescontracts.OrderSummary{{Number: "ORD-UTM/PJ/1", Status: "completed", GrandTotal: 15500, PartyName: "Budi"}}, nil
}

type fakePurchasing struct {
	purchasingcontracts.PurchaseOrderClient
}

func (fakePurchasing) ListOrders(context.Context, int64, []string, string, string, int) ([]*purchasingcontracts.OrderSummary, error) {
	return nil, nil
}

type fakeFinance struct{ financecontracts.FinanceClient }

func (fakeFinance) InventoryValue(context.Context, int64) ([]financecontracts.InventoryPosition, int64, error) {
	return []financecontracts.InventoryPosition{{ProductID: 7, Qty: 12}}, 0, nil
}

type fakeProducts struct {
	productcontracts.ProductClient
	items []*productcontracts.Product
}

func (f fakeProducts) GetByCode(_ context.Context, code string) (*productcontracts.Product, error) {
	for _, p := range f.items {
		if strings.EqualFold(p.Code, code) {
			return p, nil
		}
	}
	return nil, nil
}

func (f fakeProducts) SearchIDs(_ context.Context, q string, _ int) ([]int64, error) {
	var ids []int64
	for _, p := range f.items {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(q)) {
			ids = append(ids, p.ID)
		}
	}
	return ids, nil
}

func (f fakeProducts) GetByID(_ context.Context, id int64) (*productcontracts.Product, error) {
	for _, p := range f.items {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, nil
}

func newTestEngine(sales *fakeSales, products fakeProducts) *Engine {
	return NewEngine(fakeBranches{}, products, sales, fakePurchasing{}, nil, fakeFinance{})
}

func TestEngineOrderListByStatus(t *testing.T) {
	sales := &fakeSales{}
	e := newTestEngine(sales, fakeProducts{})
	intent, ans, _, err := e.Execute(context.Background(), 0, "order selesai")
	if err != nil {
		t.Fatal(err)
	}
	if intent != IntentOrderStatus {
		t.Fatalf("intent = %s", intent)
	}
	if len(sales.gotStatuses) != 1 || sales.gotStatuses[0] != "completed" || sales.gotFrom != "" {
		t.Fatalf("ListOrders statuses=%v from=%q", sales.gotStatuses, sales.gotFrom)
	}
	if !strings.Contains(ans, "ORD-UTM/PJ/1") || !strings.Contains(ans, "Rp 15.500") {
		t.Fatalf("answer = %q", ans)
	}
}

func TestEngineProductInfoSearchesNonTrackedByName(t *testing.T) {
	// A service (non-tracked) is found by name; the old ListStocked path could
	// never see it.
	products := fakeProducts{items: []*productcontracts.Product{
		{ID: 7, Code: "JS-001", Name: "Jasa Servis AC", Type: "jasa", Status: "active", SellingPrice: 150000},
		{ID: 8, Code: "B-1", Name: "Beras Putih", Status: "active"},
		{ID: 9, Code: "B-2", Name: "Beras Merah", Status: "active"},
		{ID: 10, Code: "OLD", Name: "Beras Lama", Status: "archived"},
	}}
	e := newTestEngine(&fakeSales{}, products)

	_, ans, _, err := e.Execute(context.Background(), 1, "info produk servis ac")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans, "JS-001") || !strings.Contains(ans, "Rp 150.000") || !strings.Contains(ans, "stok 12") {
		t.Fatalf("answer = %q", ans)
	}

	// Ambiguous name asks for the code, ignoring archived products.
	_, ans, _, err = e.Execute(context.Background(), 1, "info produk beras")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans, "Ada 2 produk") || strings.Contains(ans, "OLD") {
		t.Fatalf("ambiguous answer = %q", ans)
	}

	_, ans, _, _ = e.Execute(context.Background(), 1, `info "tidak ada"`)
	if !strings.Contains(ans, "tidak ditemukan") {
		t.Fatalf("missing answer = %q", ans)
	}
}

func TestAllowRateCapsMap(t *testing.T) {
	s := &Service{rates: map[string][]time.Time{}}
	for i := 0; i < 10002; i++ {
		s.allowRate("p"+strconv.Itoa(i), 10)
	}
	if len(s.rates) > 10000 {
		t.Fatalf("rates map grew to %d", len(s.rates))
	}
}
