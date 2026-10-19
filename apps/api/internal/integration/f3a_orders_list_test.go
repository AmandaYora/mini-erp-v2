package integration

import (
	"bytes"
	"context"
	"testing"

	"github.com/xuri/excelize/v2"

	partyapp "mini-erp/internal/modules/party/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// F3a-1: order list filters (party search, status CSV, terms, date range,
// party id), partyName in rows, Count = filtered rows, export honouring the
// same filters.
func TestF3aSalesOrderListFilters(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	fx := newShop(t, ctx)
	m := testMods

	budi, err := m.party.Service().CreateCustomer(ctx, 0, partyapp.PartyInput{
		Name: "Budi Santoso", Phone: "0899111222", Email: "budi@contoh.id", Address: "Jl. Kenanga 7",
	})
	if err != nil {
		t.Fatal(err)
	}
	mk := func(party int64, date, terms string) int64 {
		in := salesapp.OrderInput{
			PartyID: party, Channel: "regular", PaymentTerms: terms, TaxType: "none", OrderDate: date,
			Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
		}
		if terms == "net" {
			in.DueDate = "2030-01-01"
		}
		so, err := m.sales.Service().CreateOrder(ctx, 0, fx.branchID, in)
		if err != nil {
			t.Fatalf("create SO: %v", err)
		}
		return so.ID
	}
	a := mk(budi.ID, "2026-01-05", "cod")       // budi, cod, draft
	b := mk(budi.ID, "2026-01-31", "net")       // budi, net, draft (last day of range)
	c := mk(fx.customerID, "2026-02-10", "cod") // other customer
	d := mk(0, "2026-02-11", "cod")             // walk-in
	if _, err := m.sales.Service().ConfirmOrder(ctx, 0, fx.branchID, c); err != nil {
		t.Fatal(err)
	}
	_ = d

	list := func(f salesapp.ListFilter) ([]int64, int64) {
		t.Helper()
		res, err := m.sales.Service().List(ctx, fx.branchID, f, 1, 50)
		if err != nil {
			t.Fatalf("list %+v: %v", f, err)
		}
		var ids []int64
		for _, o := range res.Orders {
			ids = append(ids, o.ID)
		}
		return ids, res.Total
	}
	same := func(name string, got []int64, total int64, want ...int64) {
		t.Helper()
		if len(got) != len(want) || total != int64(len(want)) {
			t.Fatalf("%s: got %v (total %d), want %v", name, got, total, want)
		}
		seen := map[int64]bool{}
		for _, g := range got {
			seen[g] = true
		}
		for _, w := range want {
			if !seen[w] {
				t.Fatalf("%s: got %v, want %v", name, got, want)
			}
		}
	}

	ids, tot := list(salesapp.ListFilter{Search: "santoso"})
	same("search by party name", ids, tot, a, b)
	ids, tot = list(salesapp.ListFilter{Search: "0899111"})
	same("search by phone", ids, tot, a, b)
	ids, tot = list(salesapp.ListFilter{Search: "budi@contoh"})
	same("search by email", ids, tot, a, b)
	ids, tot = list(salesapp.ListFilter{Search: "kenanga"})
	same("search by address", ids, tot, a, b)
	ids, tot = list(salesapp.ListFilter{From: "2026-01-01", To: "2026-01-31"})
	same("date range includes the last day", ids, tot, a, b)
	ids, tot = list(salesapp.ListFilter{Statuses: []string{"confirmed", "completed"}})
	same("status csv", ids, tot, c)
	ids, tot = list(salesapp.ListFilter{PaymentTerms: "net"})
	same("payment terms", ids, tot, b)
	ids, tot = list(salesapp.ListFilter{PartyID: budi.ID})
	same("party id", ids, tot, a, b)

	// 1-char search never searches parties (A-15) and does not match numbers here.
	if ids, _ = list(salesapp.ListFilter{Search: "b"}); len(ids) != 0 {
		t.Fatalf("one-char search must not match party names: %v", ids)
	}

	if _, err := m.sales.Service().List(ctx, fx.branchID, salesapp.ListFilter{PaymentTerms: "bitcoin"}, 1, 10); err == nil {
		t.Fatal("unknown payment terms must be rejected")
	}

	// partyName decoration and walk-in "Tunai".
	res, err := m.sales.Service().List(ctx, fx.branchID, salesapp.ListFilter{}, 1, 50)
	if err != nil {
		t.Fatal(err)
	}
	names := map[int64]string{}
	for _, o := range res.Orders {
		names[o.ID] = o.PartyName
	}
	if names[a] != "Budi Santoso" || names[d] != "Tunai" || res.Total != 4 {
		t.Fatalf("partyName/total wrong: %v total=%d", names, res.Total)
	}

	// Pagination: page 2 of limit 3 holds the oldest remaining row; count unaffected.
	p2, err := m.sales.Service().List(ctx, fx.branchID, salesapp.ListFilter{}, 2, 3)
	if err != nil || len(p2.Orders) != 1 || p2.Total != 4 {
		t.Fatalf("page 2: %v %+v", err, p2)
	}

	// Export honours the same filters and carries the customer name.
	file, err := m.sales.Service().Export(ctx, fx.branchID, salesapp.ListFilter{Search: "santoso", PaymentTerms: "net"}, "xlsx")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	var buf bytes.Buffer
	if err := file.Send(&buf); err != nil {
		t.Fatal(err)
	}
	x, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatal(err)
	}
	// SL-04: the workbook has three sheets; the order list is "Daftar Order" (named columns).
	rows, err := x.GetRows("Daftar Order")
	if err != nil {
		t.Fatal(err)
	}
	col := map[string]int{}
	if len(rows) > 0 {
		for i, h := range rows[0] {
			col[h] = i
		}
	}
	if len(rows) != 2 || rows[1][col["Customer"]] != "Budi Santoso" || rows[1][col["Termin"]] != "Net / Tempo" {
		t.Fatalf("export rows: %v", rows)
	}
}
