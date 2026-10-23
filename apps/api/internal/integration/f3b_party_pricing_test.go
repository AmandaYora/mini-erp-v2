package integration

import (
	"strings"
	"testing"

	partyapp "mini-erp/internal/modules/party/application"
	productapp "mini-erp/internal/modules/product/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// F3b-5.1: Informasi Tambahan round trip and bounds.
func TestF3bPartyAttributes(t *testing.T) {
	freshDB(t)
	svc := testMods.party.Service()
	c, err := svc.CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: "Pak Budi", Phone: "081234567890",
		Attributes: map[string]string{" NPWP ": " 01.234 ", "Bank": "BCA 123"}})
	if err != nil {
		t.Fatal(err)
	}
	if c.Attributes["NPWP"] != "01.234" || c.Attributes["Bank"] != "BCA 123" || len(c.Attributes) != 2 {
		t.Fatalf("attributes: %+v", c.Attributes)
	}
	// Omitted on update = unchanged; replaced when sent; {} clears.
	up, err := svc.UpdateCustomer(ctx(), 0, c.ID, partyapp.PartyInput{Name: "Pak Budi 2"})
	if err != nil || len(up.Attributes) != 2 {
		t.Fatalf("omitted attributes must be kept: %+v %v", up, err)
	}
	up, err = svc.UpdateCustomer(ctx(), 0, c.ID, partyapp.PartyInput{Name: "Pak Budi 2", Attributes: map[string]string{"PIC": "Sari"}})
	if err != nil || len(up.Attributes) != 1 || up.Attributes["PIC"] != "Sari" {
		t.Fatalf("replace: %+v %v", up, err)
	}
	up, err = svc.UpdateCustomer(ctx(), 0, c.ID, partyapp.PartyInput{Name: "Pak Budi 2", Attributes: map[string]string{}})
	if err != nil || len(up.Attributes) != 0 || up.Attributes == nil {
		t.Fatalf("clear: %+v %v", up, err)
	}
	big := map[string]string{}
	for i := 0; i < 21; i++ {
		big[strings.Repeat("k", i+1)] = "v"
	}
	if _, err := svc.CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "PT X", Attributes: big}); err == nil {
		t.Error("more than 20 attributes must be rejected")
	}
	if _, err := svc.CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "PT X", Attributes: map[string]string{"k": ""}}); err == nil {
		t.Error("empty attribute value must be rejected")
	}
	if _, err := svc.CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "PT X", Attributes: map[string]string{strings.Repeat("k", 61): "v"}}); err == nil {
		t.Error("attribute name over 60 characters must be rejected")
	}
	got, err := svc.GetByID(ctx(), c.ID)
	if err != nil || got.Attributes == nil {
		t.Fatalf("GetByID attributes: %+v %v", got, err)
	}
}

// F3b-5.3: legacy sequential codes, archived rows included.
func TestF3bPartyAutoCode(t *testing.T) {
	freshDB(t)
	svc := testMods.party.Service()
	mk := func(typ, name string) string {
		var code string
		if typ == "c" {
			p, err := svc.CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: name})
			if err != nil {
				t.Fatal(err)
			}
			code = p.Code
		} else {
			p, err := svc.CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: name})
			if err != nil {
				t.Fatal(err)
			}
			code = p.Code
		}
		return code
	}
	if a, b := mk("c", "A"), mk("c", "B"); a != "CUS-001" || b != "CUS-002" {
		t.Fatalf("customer codes: %s %s", a, b)
	}
	if s := mk("s", "S"); s != "SUP-001" {
		t.Fatalf("supplier code: %s", s)
	}
	// A manual code sets the high-water mark; archived rows still count.
	m, err := svc.CreateCustomer(ctx(), 0, partyapp.PartyInput{Code: "CUS-010", Name: "Manual"})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.Archive(ctx(), 0, m.ID, "customer"); err != nil {
		t.Fatal(err)
	}
	if next := mk("c", "C"); next != "CUS-011" {
		t.Fatalf("next code after archived CUS-010 = %s", next)
	}
	// Concurrent creators never share a code.
	done := make(chan string, 8)
	for i := 0; i < 8; i++ {
		go func() {
			p, err := svc.CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: "Paralel"})
			if err != nil {
				done <- "ERR " + err.Error()
				return
			}
			done <- p.Code
		}()
	}
	seen := map[string]bool{}
	for i := 0; i < 8; i++ {
		c := <-done
		if strings.HasPrefix(c, "ERR") || seen[c] {
			t.Fatalf("concurrent auto code: %q (dup=%v)", c, seen[c])
		}
		seen[c] = true
	}
	// Free-text search reaches email and address.
	if _, err := svc.CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: "Cari", Email: "cari@contoh.id", Address: "Jl. Mawar 7"}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.List(ctx(), partyapp.ListFilter{Type: "customer", Search: "mawar"}, 1, 10)
	if err != nil || res.Total != 1 {
		t.Fatalf("search by address: %+v %v", res, err)
	}
}

func f3bMemberCustomer(t *testing.T, in partyapp.MemberTypeInput) (customerID int64) {
	t.Helper()
	m, err := testMods.party.Service().CreateMemberType(ctx(), 0, in)
	if err != nil {
		t.Fatalf("member type: %v", err)
	}
	c, err := testMods.party.Service().CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: "Member " + in.Code, MemberTypeID: &m.ID})
	if err != nil {
		t.Fatal(err)
	}
	return c.ID
}

// F3b-5.2: purchase basis converts the purchase unit, errors are per line,
// product lookups are batched.
func TestF3bMemberPricingQuote(t *testing.T) {
	freshDB(t)
	// 100.000 per dus of 12 pcs; sold per pcs.
	dus := f3bProduct(t, productapp.ProductInput{Code: "PR-DUS", Name: "Dus Mie", PurchaseUOM: "dus",
		PurchasePrice: 100000, SellingPrice: 15000, MinSellingPrice: 12000})
	// Re-create with a real factor (helper pins factors to 1).
	in := f3bUpdateInput(dus)
	in.PurchaseFactor = 12
	if _, err := testMods.product.Service().UpdateProduct(ctx(), 0, dus.ID, in); err != nil {
		t.Fatal(err)
	}
	noSell := f3bProduct(t, productapp.ProductInput{Code: "PR-NOSELL", Name: "Tanpa Harga", PurchasePrice: 5000, SellingPrice: 1})
	zero := f3bUpdateInput(noSell)
	zero.SellingPrice, zero.MinSellingPrice = 0, 0
	if _, err := testMods.product.Service().UpdateProduct(ctx(), 0, noSell.ID, zero); err != nil {
		t.Fatal(err)
	}
	pricing := testMods.party.Pricing()

	cust := f3bMemberCustomer(t, partyapp.MemberTypeInput{Code: "BUY", Name: "Beli+10", Basis: "purchase_price",
		Direction: "plus", Type: "percent", Value: 10})
	q, err := pricing.Quote(ctx(), cust, []int64{dus.ID, noSell.ID, 987654})
	if err != nil {
		t.Fatalf("a bad line must not fail the quote: %v", err)
	}
	if len(q.Lines) != 3 {
		t.Fatalf("lines: %+v", q.Lines)
	}
	l := q.Lines[0]
	if !l.Applied || l.MemberPrice != 9167 || l.StandardPrice != 15000 || l.Error != "" {
		t.Fatalf("100000/12 +10%% = 9167 per pcs, got %+v", l)
	}
	// noSell: the purchase basis exists, so it prices fine; the missing selling
	// price only matters for the selling basis (next customer).
	if !q.Lines[1].Applied || q.Lines[1].Error != "" {
		t.Fatalf("purchase basis does not need a selling price: %+v", q.Lines[1])
	}
	if q.Lines[2].Error == "" || q.Lines[2].Applied {
		t.Fatalf("unknown product must be a line error: %+v", q.Lines[2])
	}

	sell := f3bMemberCustomer(t, partyapp.MemberTypeInput{Code: "SELL", Name: "Jual-10", Basis: "selling_price",
		Direction: "minus", Type: "percent", Value: 10})
	q, err = pricing.Quote(ctx(), sell, []int64{dus.ID, noSell.ID})
	if err != nil {
		t.Fatal(err)
	}
	if q.Lines[0].MemberPrice != 13500 || !q.Lines[0].Applied {
		t.Fatalf("selling basis: %+v", q.Lines[0])
	}
	if q.Lines[1].Applied || !strings.Contains(q.Lines[1].Error, "harga jual normal") || !strings.Contains(q.Lines[1].Error, "PR-NOSELL") {
		t.Fatalf("empty basis price must be a named line error: %+v", q.Lines[1])
	}
	minc := f3bMemberCustomer(t, partyapp.MemberTypeInput{Code: "MINB", Name: "Min", Basis: "min_selling_price",
		Direction: "minus", Type: "nominal", Value: 100})
	q, _ = pricing.Quote(ctx(), minc, []int64{dus.ID, noSell.ID})
	if q.Lines[0].MemberPrice != 11900 || !strings.Contains(q.Lines[1].Error, "jual minimum") {
		t.Fatalf("min basis: %+v", q.Lines)
	}

	// A sales order surfaces the line error instead of a generic failure.
	fx := newShop(t, ctx())
	_, err = testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: sell, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: noSell.ID, VariantID: noSell.Variants[0].ID, UOM: "pcs", Qty: 1}},
	})
	ae := mustAppErr(t, err)
	if len(ae.Fields) == 0 || !strings.Contains(ae.Fields[0].Message, "harga jual normal") {
		t.Fatalf("sales must report the quote line error: %+v", ae)
	}

	// A large basket answers in one batch (no per-line product reads) and
	// still reports each line.
	ids := make([]int64, 0, 60)
	for i := 0; i < 60; i++ {
		ids = append(ids, dus.ID)
	}
	q, err = pricing.Quote(ctx(), sell, ids)
	if err != nil || len(q.Lines) != 60 || q.Lines[59].MemberPrice != 13500 {
		t.Fatalf("batch quote: %v %d", err, len(q.Lines))
	}
}

// F3b-5.2: percent bounds and rounding vocabulary.
func TestF3bMemberTypeRules(t *testing.T) {
	freshDB(t)
	svc := testMods.party.Service()
	mk := func(in partyapp.MemberTypeInput) error {
		_, err := svc.CreateMemberType(ctx(), 0, in)
		return err
	}
	if err := mk(partyapp.MemberTypeInput{Code: "UP150", Name: "Markup", Direction: "plus", Type: "percent", Value: 150}); err != nil {
		t.Errorf("markup over 100%% must be legal: %v", err)
	}
	if err := mk(partyapp.MemberTypeInput{Code: "DOWN150", Name: "Diskon", Direction: "minus", Type: "percent", Value: 150}); err == nil {
		t.Error("a discount over 100% must be rejected")
	}
	if err := mk(partyapp.MemberTypeInput{Code: "HUGE", Name: "Huge", Direction: "plus", Type: "percent", Value: 200000}); err == nil {
		t.Error("percent above the technical ceiling must be rejected")
	}
	if err := mk(partyapp.MemberTypeInput{Code: "BADMODE", Name: "x", Mode: "banana", Step: 100}); err == nil {
		t.Error("unknown rounding mode must be rejected")
	}
	if err := mk(partyapp.MemberTypeInput{Code: "NOSTEP", Name: "x", Mode: "nearest"}); err == nil {
		t.Error("rounding mode without a step must be rejected")
	}
	// Legacy spellings fold into the legacy vocabulary with their default step.
	for code, want := range map[string][2]any{
		"R500": {"round_500", "nearest"}, "FLR": {"floor", "down"}, "CEI": {"ceil", "up"},
	} {
		m, err := svc.CreateMemberType(ctx(), 0, partyapp.MemberTypeInput{Code: code, Name: code, Mode: want[0].(string)})
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if m.RoundingMode != want[1].(string) || m.RoundingStep <= 0 {
			t.Errorf("%s stored as %q step %v", code, m.RoundingMode, m.RoundingStep)
		}
	}
	m, err := svc.CreateMemberType(ctx(), 0, partyapp.MemberTypeInput{Code: "N250", Name: "N", Mode: "nearest", Step: 250,
		Direction: "minus", Type: "percent", Value: 4})
	if err != nil || m.RoundingMode != "nearest" || m.RoundingStep != 250 {
		t.Fatalf("nearest/250: %+v %v", m, err)
	}
	p := f3bProduct(t, productapp.ProductInput{Code: "RND-1", Name: "Bulat", SellingPrice: 10000})
	c, _ := svc.CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: "Rnd", MemberTypeID: &m.ID})
	q, err := testMods.party.Pricing().Quote(ctx(), c.ID, []int64{p.ID})
	if err != nil || q.Lines[0].MemberPrice != 9500 { // 9600 -> nearest 250 = 9500
		t.Fatalf("rounding: %+v %v", q, err)
	}
}
