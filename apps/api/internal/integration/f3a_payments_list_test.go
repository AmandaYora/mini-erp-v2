package integration

import (
	"context"
	"testing"

	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
)

// F3a-5: payment list filters run on the server (party-name search, method,
// paid-date range, order allocation), rows carry partyName, Count matches.
func TestF3aPaymentListFilters(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	fx := newShop(t, ctx)
	m := testMods

	so1 := sellFixture(t, ctx, fx, 1)
	so2 := sellFixture(t, ctx, fx, 2)
	pay := func(method, date, notes string, soID int64) int64 {
		so, err := m.sales.Service().GetByID(ctx, soID)
		if err != nil {
			t.Fatal(err)
		}
		p, err := m.payment.Service().Create(ctx, 0, fx.branchID, fx.customerID, method, date, notes, so.GrandTotal,
			[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: soID, Amount: so.GrandTotal}})
		if err != nil {
			t.Fatalf("create payment: %v", err)
		}
		return p.ID
	}
	p1 := pay("cash", "2026-01-10", "lunas pertama", so1)
	p2 := pay("transfer", "2026-02-20", "lunas kedua", so2)

	list := func(f paymentapp.ListFilter) ([]int64, int64, []string) {
		t.Helper()
		res, err := m.payment.Service().ListFiltered(ctx, fx.branchID, f, 1, 50)
		if err != nil {
			t.Fatalf("list %+v: %v", f, err)
		}
		var ids []int64
		var names []string
		for _, p := range res.Payments {
			ids = append(ids, p.ID)
			names = append(names, p.PartyName)
		}
		return ids, res.Total, names
	}
	expect := func(name string, ids []int64, total int64, want ...int64) {
		t.Helper()
		if len(ids) != len(want) || total != int64(len(want)) {
			t.Fatalf("%s: got %v total %d, want %v", name, ids, total, want)
		}
		for i := range want {
			found := false
			for _, id := range ids {
				if id == want[i] {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s: got %v, want %v", name, ids, want)
			}
		}
	}

	ids, tot, names := list(paymentapp.ListFilter{})
	expect("all", ids, tot, p1, p2)
	for _, n := range names {
		if n != "Customer Uji" {
			t.Fatalf("partyName not decorated: %v", names)
		}
	}
	ids, tot, _ = list(paymentapp.ListFilter{Search: "customer uji"})
	expect("search by party name", ids, tot, p1, p2)
	ids, tot, _ = list(paymentapp.ListFilter{Search: "kedua"})
	expect("search by notes", ids, tot, p2)
	ids, tot, _ = list(paymentapp.ListFilter{Method: "transfer"})
	expect("method", ids, tot, p2)
	ids, tot, _ = list(paymentapp.ListFilter{From: "2026-01-01", To: "2026-01-10"})
	expect("date range includes the last day", ids, tot, p1)
	ids, tot, _ = list(paymentapp.ListFilter{OrderType: paymentcontracts.OrderSales, OrderID: so2})
	expect("order allocation", ids, tot, p2)
	ids, tot, _ = list(paymentapp.ListFilter{Direction: "in", PartyID: fx.customerID, Status: "active"})
	expect("direction+party+status", ids, tot, p1, p2)

	if _, err := m.payment.Service().ListFiltered(ctx, fx.branchID, paymentapp.ListFilter{Direction: "sideways"}, 1, 10); err == nil {
		t.Fatal("bad direction must be rejected")
	}
	if _, err := m.payment.Service().ListFiltered(ctx, fx.branchID, paymentapp.ListFilter{OrderID: 5}, 1, 10); err == nil {
		t.Fatal("orderId without orderType must be rejected")
	}

	// Ledger (batched enrichment) still carries allocation order numbers.
	led, total, err := m.payment.Service().Ledger(ctx, fx.branchID, fx.customerID, 1, 20)
	if err != nil || total != 2 || len(led) != 2 {
		t.Fatalf("ledger: %v total=%d len=%d", err, total, len(led))
	}
	for _, p := range led {
		if len(p.Allocations) != 1 || p.Allocations[0].OrderNumber == "" || p.PartyName == "" {
			t.Fatalf("ledger row not enriched: %+v", p)
		}
	}
}
