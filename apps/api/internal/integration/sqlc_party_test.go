package integration

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
	"time"

	"mini-erp/internal/modules/party/contracts"
	partyinfra "mini-erp/internal/modules/party/infrastructure"
)

// Characterization tests for the party Repository (F8-18).

func sqlcPartyRepo() *partyinfra.Repository { return partyinfra.NewRepository(testMods.db) }

func sqlcMkParty(t *testing.T, r *partyinfra.Repository, typ, code, name string, mod func(*contracts.Party)) int64 {
	t.Helper()
	p := &contracts.Party{Code: code, Type: typ, Name: name}
	if mod != nil {
		mod(p)
	}
	id, err := r.CreateParty(context.Background(), p, 5)
	if err != nil {
		t.Fatalf("create party %s: %v", code, err)
	}
	return id
}

func sqlcMkMember(t *testing.T, r *partyinfra.Repository, code, name string, step float64) int64 {
	t.Helper()
	id, err := r.CreateMemberType(context.Background(), &contracts.MemberType{
		Code: code, Name: name, PriceBasis: "selling_price", Direction: "minus", AdjustmentType: "percent",
		Adjustment: 12.5, RoundingMode: "nearest", RoundingStep: step,
	}, 2)
	if err != nil {
		t.Fatalf("create member %s: %v", code, err)
	}
	return id
}

func TestSqlcPartyCodesAndGet(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPartyRepo()

	if n, err := r.MaxCodeNumber(ctx, "customer", "CUST-"); err != nil || n != 0 {
		t.Fatalf("empty max: %d %v", n, err)
	}
	mt := sqlcMkMember(t, r, "GOLD", "Gold", 100)
	full := sqlcMkParty(t, r, "customer", "CUST-0007", "Budi", func(p *contracts.Party) {
		p.Phone, p.Email, p.Address, p.Notes = "0811", "b@x.id", "Jl. Mawar", "vip"
		p.MemberTypeID = &mt
		p.Attributes = map[string]string{"kota": "Bandung", "rt": "3"}
	})
	bare := sqlcMkParty(t, r, "customer", "CUST-0012", "Ani", nil)
	sqlcMkParty(t, r, "customer", "CUST-ABC", "Non numeric", nil)
	sqlcMkParty(t, r, "customer", "CUST-12x", "Mixed suffix", nil)
	sqlcMkParty(t, r, "supplier", "CUST-0099", "Supplier with same shape", nil)
	arch := sqlcMkParty(t, r, "customer", "CUST-0030", "Archived", nil)
	if err := r.SetPartyStatus(ctx, arch, "archived", 1); err != nil {
		t.Fatal(err)
	}
	sqlcMkParty(t, r, "customer", "OTHER-0500", "Other prefix", nil)

	if n, err := r.MaxCodeNumber(ctx, "customer", "CUST-"); err != nil || n != 30 {
		t.Fatalf("max customer (archived counts): %d %v", n, err)
	}
	if n, err := r.MaxCodeNumber(ctx, "supplier", "CUST-"); err != nil || n != 99 {
		t.Fatalf("max supplier: %d %v", n, err)
	}
	if n, err := r.MaxCodeNumber(ctx, "customer", "ZZ-"); err != nil || n != 0 {
		t.Fatalf("max unknown prefix: %d %v", n, err)
	}
	if n, err := r.MaxCodeNumber(ctx, "customer", ""); err != nil {
		t.Fatalf("max empty prefix: %d %v", n, err)
	}

	p, err := r.GetParty(ctx, full)
	if err != nil || p == nil {
		t.Fatalf("get: %v %v", p, err)
	}
	if p.Code != "CUST-0007" || p.Type != "customer" || p.Name != "Budi" || p.Phone != "0811" || p.Email != "b@x.id" ||
		p.Address != "Jl. Mawar" || p.Notes != "vip" || p.MemberTypeID == nil || *p.MemberTypeID != mt || p.Status != "active" ||
		!reflect.DeepEqual(p.Attributes, map[string]string{"kota": "Bandung", "rt": "3"}) {
		t.Fatalf("full mismatch: %+v", p)
	}
	p, _ = r.GetParty(ctx, bare)
	if p.Phone != "" || p.Email != "" || p.MemberTypeID != nil || p.Attributes == nil || len(p.Attributes) != 0 {
		t.Fatalf("bare mismatch: %+v", p)
	}
	if p, err := r.GetParty(ctx, 987654); err != nil || p != nil {
		t.Fatalf("missing: %v %v", p, err)
	}
	// legacy attribute values: numbers/bools stringified, nulls dropped
	if _, err := testMods.db.Exec(`UPDATE parties SET attributes = '{"a":1,"b":true,"c":null,"d":"x"}' WHERE id = ?`, bare); err != nil {
		t.Fatal(err)
	}
	p, _ = r.GetParty(ctx, bare)
	if !reflect.DeepEqual(p.Attributes, map[string]string{"a": "1", "b": "true", "d": "x"}) {
		t.Fatalf("legacy attrs: %v", p.Attributes)
	}
	if p, err := r.PartyByCode(ctx, "CUST-0007", "customer"); err != nil || p == nil || p.ID != full {
		t.Fatalf("by code: %v %v", p, err)
	}
	if p, err := r.PartyByCode(ctx, "CUST-0007", "supplier"); err != nil || p != nil {
		t.Fatalf("by code other type: %v %v", p, err)
	}
	// duplicate (code,type) rejected
	if _, err := r.CreateParty(ctx, &contracts.Party{Code: "CUST-0007", Type: "customer", Name: "dup"}, 1); err == nil {
		t.Fatal("duplicate code must fail")
	}
}

func TestSqlcPartyUpdateStatusAudit(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPartyRepo()
	mt := sqlcMkMember(t, r, "M1", "M1", 0)
	id := sqlcMkParty(t, r, "customer", "C-1", "Asli", func(p *contracts.Party) {
		p.Phone = "1"
		p.Attributes = map[string]string{"k": "v"}
	})
	if err := r.UpdateParty(ctx, &contracts.Party{ID: id, Name: "Baru", MemberTypeID: &mt}, 9); err != nil {
		t.Fatal(err)
	}
	p, _ := r.GetParty(ctx, id)
	if p.Name != "Baru" || p.Phone != "" || p.MemberTypeID == nil || *p.MemberTypeID != mt || len(p.Attributes) != 0 || p.Code != "C-1" {
		t.Fatalf("update: %+v", p)
	}
	var ub int64
	var upd, cr time.Time
	if err := testMods.db.QueryRow("SELECT updated_by, updated_at, created_at FROM parties WHERE id=?", id).Scan(&ub, &upd, &cr); err != nil || ub != 9 || !upd.After(cr) {
		t.Fatalf("audit: %d %v %v %v", ub, upd, cr, err)
	}
	if err := r.UpdateParty(ctx, &contracts.Party{ID: id, Name: "Baru2", Attributes: map[string]string{"a": "b"}}, 9); err != nil {
		t.Fatal(err)
	}
	p, _ = r.GetParty(ctx, id)
	if p.MemberTypeID != nil || !reflect.DeepEqual(p.Attributes, map[string]string{"a": "b"}) {
		t.Fatalf("update2: %+v", p)
	}

	var arc sql.NullTime
	if err := r.SetPartyStatus(ctx, id, "archived", 3); err != nil {
		t.Fatal(err)
	}
	_ = testMods.db.QueryRow("SELECT archived_at FROM parties WHERE id=?", id).Scan(&arc)
	if !arc.Valid || time.Since(arc.Time) > time.Minute || time.Since(arc.Time) < -time.Minute {
		t.Fatalf("archived_at: %+v", arc)
	}
	if p, _ := r.GetParty(ctx, id); p.Status != "archived" {
		t.Fatalf("status %s", p.Status)
	}
	if err := r.SetPartyStatus(ctx, id, "active", 3); err != nil {
		t.Fatal(err)
	}
	_ = testMods.db.QueryRow("SELECT archived_at FROM parties WHERE id=?", id).Scan(&arc)
	if arc.Valid {
		t.Fatalf("archived_at must clear: %+v", arc)
	}
}

func TestSqlcPartyBatchLookups(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPartyRepo()
	a := sqlcMkParty(t, r, "customer", "A", "Alpha", nil)
	b := sqlcMkParty(t, r, "supplier", "B", "Beta", nil)

	names, err := r.NamesByIDs(ctx, []int64{a, b, a, 0, 999})
	if err != nil || !reflect.DeepEqual(names, map[int64]string{a: "Alpha", b: "Beta"}) {
		t.Fatalf("names: %v %v", names, err)
	}
	if names, err := r.NamesByIDs(ctx, nil); err != nil || names == nil || len(names) != 0 {
		t.Fatalf("names empty: %v %v", names, err)
	}
	if names, err := r.NamesByIDs(ctx, []int64{0, 0}); err != nil || len(names) != 0 {
		t.Fatalf("names zeros: %v %v", names, err)
	}
	labels, err := r.LabelsByIDs(ctx, []int64{a, b, b, 0, 999})
	if err != nil || !reflect.DeepEqual(labels, map[int64]contracts.Label{a: {Code: "A", Name: "Alpha"}, b: {Code: "B", Name: "Beta"}}) {
		t.Fatalf("labels: %v %v", labels, err)
	}
	if labels, err := r.LabelsByIDs(ctx, nil); err != nil || labels == nil || len(labels) != 0 {
		t.Fatalf("labels empty: %v %v", labels, err)
	}
	big := make([]int64, 0, 3000)
	for i := int64(1); i <= 3000; i++ {
		big = append(big, i)
	}
	if names, err := r.NamesByIDs(ctx, big); err != nil || len(names) != 2 {
		t.Fatalf("names big: %v %v", len(names), err)
	}
	if labels, err := r.LabelsByIDs(ctx, big); err != nil || len(labels) != 2 {
		t.Fatalf("labels big: %v %v", len(labels), err)
	}
}

func TestSqlcPartyListSearch(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPartyRepo()
	gold := sqlcMkMember(t, r, "GOLD", "Gold", 0)
	c1 := sqlcMkParty(t, r, "customer", "C-1", "Charlie Toko", func(p *contracts.Party) { p.Phone = "0811"; p.MemberTypeID = &gold })
	c2 := sqlcMkParty(t, r, "customer", "C-2", "Alice Store", func(p *contracts.Party) { p.Email = "alice@shop.id"; p.Address = "Jl. Melati 5" })
	c3 := sqlcMkParty(t, r, "customer", "C-3", "Bob 100% Jaya", nil)
	s1 := sqlcMkParty(t, r, "supplier", "S-1", "Zeta Supplier", nil)
	if err := r.SetPartyStatus(ctx, c3, "archived", 1); err != nil {
		t.Fatal(err)
	}
	ids := func(ps []*contracts.Party) []int64 {
		var o []int64
		for _, p := range ps {
			o = append(o, p.ID)
		}
		return o
	}
	cases := []struct {
		name                string
		typ, search, status string
		limit, offset       int
		want                []int64
	}{
		{"all by name", "", "", "", 50, 0, []int64{c2, c3, c1, s1}},
		{"customers", "customer", "", "", 50, 0, []int64{c2, c3, c1}},
		{"suppliers", "supplier", "", "", 50, 0, []int64{s1}},
		{"active only", "", "", "active", 50, 0, []int64{c2, c1, s1}},
		{"archived only", "", "", "archived", 50, 0, []int64{c3}},
		{"search code", "", "c-1", "", 50, 0, []int64{c1}},
		{"search name", "", "alice", "", 50, 0, []int64{c2}},
		{"search phone", "", "0811", "", 50, 0, []int64{c1}},
		{"search email", "", "shop.id", "", 50, 0, []int64{c2}},
		{"search address", "", "melati", "", 50, 0, []int64{c2}},
		{"two words AND across cols", "", "alice melati", "", 50, 0, []int64{c2}},
		{"two words no match", "", "alice charlie", "", 50, 0, nil},
		{"literal percent", "", "100%", "", 50, 0, []int64{c3}},
		{"literal underscore", "", "C_1", "", 50, 0, nil},
		{"type+search", "supplier", "a", "", 50, 0, []int64{s1}},
		{"page 1", "", "", "", 2, 0, []int64{c2, c3}},
		{"page 2", "", "", "", 2, 2, []int64{c1, s1}},
		{"beyond", "", "", "", 2, 9, nil},
	}
	for _, c := range cases {
		got, err := r.ListParties(ctx, c.typ, c.search, c.status, c.limit, c.offset)
		if err != nil || !reflect.DeepEqual(ids(got), c.want) {
			t.Fatalf("%s: list %v err %v want %v", c.name, ids(got), err, c.want)
		}
	}
	// counts ignore paging
	for _, c := range cases {
		if c.limit != 50 {
			continue
		}
		n, err := r.CountParties(ctx, c.typ, c.search, c.status)
		if err != nil || int(n) != len(c.want) {
			t.Fatalf("%s: count %d err %v want %d", c.name, n, err, len(c.want))
		}
	}
	// member code rides along; member id mapped
	got, _ := r.ListParties(ctx, "customer", "charlie", "", 5, 0)
	if len(got) != 1 || got[0].MemberCode != "GOLD" || got[0].MemberTypeID == nil || *got[0].MemberTypeID != gold || got[0].Phone != "0811" {
		t.Fatalf("member code join: %+v", got)
	}
	got, _ = r.ListParties(ctx, "customer", "alice", "", 5, 0)
	if got[0].MemberCode != "" || got[0].MemberTypeID != nil || got[0].Email != "alice@shop.id" || got[0].Attributes == nil {
		t.Fatalf("no member: %+v", got[0])
	}

	// SearchIDs: all statuses, id order, limit
	sids, err := r.SearchIDs(ctx, "", "", 10)
	if err != nil || !reflect.DeepEqual(sids, []int64{c1, c2, c3, s1}) {
		t.Fatalf("searchids all: %v %v", sids, err)
	}
	sids, _ = r.SearchIDs(ctx, "customer", "", 2)
	if !reflect.DeepEqual(sids, []int64{c1, c2}) {
		t.Fatalf("searchids limit: %v", sids)
	}
	sids, _ = r.SearchIDs(ctx, "", "bob", 10)
	if !reflect.DeepEqual(sids, []int64{c3}) {
		t.Fatalf("searchids term (archived included): %v", sids)
	}
	if sids, err := r.SearchIDs(ctx, "supplier", "alice", 10); err != nil || len(sids) != 0 {
		t.Fatalf("searchids none: %v %v", sids, err)
	}
}

func TestSqlcPartyAddresses(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPartyRepo()
	p1 := sqlcMkParty(t, r, "customer", "C-1", "Satu", nil)
	p2 := sqlcMkParty(t, r, "customer", "C-2", "Dua", nil)

	mk := func(party int64, label string, primary bool, order int, text string) int64 {
		id, err := r.CreateAddress(ctx, &contracts.Address{PartyID: party, Label: label, Recipient: "Rec " + label, Phone: "08" + label, Text: text, IsPrimary: primary, SortOrder: order})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	a1 := mk(p1, "1", false, 2, "Alamat 1")
	a2 := mk(p1, "2", false, 1, "Alamat 2")
	a3 := mk(p1, "3", true, 9, "Alamat 3")
	a4 := mk(p1, "4", false, 1, "Alamat 4")
	other := mk(p2, "5", true, 0, "Alamat 5")
	bare, err := r.CreateAddress(ctx, &contracts.Address{PartyID: p1, Text: "Tanpa label"})
	if err != nil {
		t.Fatal(err)
	}

	ids := func(as []*contracts.Address) []int64 {
		var o []int64
		for _, a := range as {
			o = append(o, a.ID)
		}
		return o
	}
	got, err := r.AddressesByParty(ctx, p1)
	// primary first, then sort_order asc, then id asc
	if err != nil || !reflect.DeepEqual(ids(got), []int64{a3, bare, a2, a4, a1}) {
		t.Fatalf("addresses order: %v %v", ids(got), err)
	}
	if got[0].Label != "3" || got[0].Recipient != "Rec 3" || got[0].Phone != "083" || got[0].Text != "Alamat 3" || !got[0].IsPrimary || got[0].SortOrder != 9 || got[0].Status != "active" || got[0].PartyID != p1 {
		t.Fatalf("mapping: %+v", got[0])
	}
	if b := got[1]; b.Label != "" || b.Recipient != "" || b.Phone != "" || b.IsPrimary {
		t.Fatalf("null mapping: %+v", b)
	}
	if got, err := r.AddressesByParty(ctx, 424242); err != nil || len(got) != 0 {
		t.Fatalf("no addresses: %v %v", got, err)
	}

	if a, err := r.AddressByID(ctx, p1, a2); err != nil || a == nil || a.Text != "Alamat 2" {
		t.Fatalf("by id: %v %v", a, err)
	}
	if a, err := r.AddressByID(ctx, p1, other); err != nil || a != nil {
		t.Fatalf("foreign-owned: %v %v", a, err)
	}
	if a, err := r.AddressByID(ctx, p1, 99999); err != nil || a != nil {
		t.Fatalf("missing: %v %v", a, err)
	}

	// update
	if err := r.UpdateAddress(ctx, &contracts.Address{ID: a1, Text: "Baru", IsPrimary: true, SortOrder: 0}); err != nil {
		t.Fatal(err)
	}
	a, _ := r.AddressByID(ctx, p1, a1)
	if a.Text != "Baru" || a.Label != "" || a.Recipient != "" || a.Phone != "" || !a.IsPrimary || a.SortOrder != 0 {
		t.Fatalf("update: %+v", a)
	}

	// exclusive primary: clear then archived primary → promote oldest active
	if err := r.ClearPrimary(ctx, p1); err != nil {
		t.Fatal(err)
	}
	got, _ = r.AddressesByParty(ctx, p1)
	for _, g := range got {
		if g.IsPrimary {
			t.Fatalf("primary must be cleared: %+v", g)
		}
	}
	if o, _ := r.AddressByID(ctx, p2, other); !o.IsPrimary {
		t.Fatal("ClearPrimary must not touch another party")
	}
	if err := r.SetAddressStatus(ctx, a1, "archived"); err != nil { // oldest id
		t.Fatal(err)
	}
	var arc sql.NullTime
	_ = testMods.db.QueryRow("SELECT archived_at FROM party_delivery_addresses WHERE id=?", a1).Scan(&arc)
	if !arc.Valid {
		t.Fatal("archived_at must be set")
	}
	if got, _ := r.AddressesByParty(ctx, p1); len(got) != 4 {
		t.Fatalf("archived address must disappear: %d", len(got))
	}
	if err := r.PromoteOldestActive(ctx, p1); err != nil {
		t.Fatal(err)
	}
	if a, _ := r.AddressByID(ctx, p1, a2); !a.IsPrimary {
		t.Fatalf("oldest active (a2) must be primary: %+v", a)
	}
	if a, _ := r.AddressByID(ctx, p1, a3); a.IsPrimary {
		t.Fatal("only one primary")
	}
	// party without addresses: promote is a no-op
	if err := r.PromoteOldestActive(ctx, 424242); err != nil {
		t.Fatalf("promote none: %v", err)
	}
	// un-archive clears archived_at
	if err := r.SetAddressStatus(ctx, a1, "active"); err != nil {
		t.Fatal(err)
	}
	_ = testMods.db.QueryRow("SELECT archived_at FROM party_delivery_addresses WHERE id=?", a1).Scan(&arc)
	if arc.Valid {
		t.Fatal("archived_at must clear")
	}
	_ = a4
}

func TestSqlcPartyMemberTypes(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcPartyRepo()
	g := sqlcMkMember(t, r, "GOLD", "Gold Member", 500)
	s := sqlcMkMember(t, r, "SILVER", "Silver Member", 0) // NULL step
	z := sqlcMkMember(t, r, "ZINC", "Anggota Zinc", 0)
	if err := r.SetMemberTypeStatus(ctx, z, "archived", 4); err != nil {
		t.Fatal(err)
	}

	m, err := r.MemberTypeByID(ctx, g)
	if err != nil || m == nil {
		t.Fatalf("by id: %v %v", m, err)
	}
	want := &contracts.MemberType{ID: g, Code: "GOLD", Name: "Gold Member", PriceBasis: "selling_price", Direction: "minus",
		AdjustmentType: "percent", Adjustment: 12.5, RoundingMode: "nearest", RoundingStep: 500, Status: "active"}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("mismatch:\n got %+v\nwant %+v", m, want)
	}
	if m, _ := r.MemberTypeByID(ctx, s); m.RoundingStep != 0 || m.Description != "" {
		t.Fatalf("null step: %+v", m)
	}
	if m, err := r.MemberTypeByID(ctx, 9999); err != nil || m != nil {
		t.Fatalf("missing: %v %v", m, err)
	}
	if m, err := r.MemberTypeByCode(ctx, "SILVER"); err != nil || m == nil || m.ID != s {
		t.Fatalf("by code: %v %v", m, err)
	}
	if m, err := r.MemberTypeByCode(ctx, "NOPE"); err != nil || m != nil {
		t.Fatalf("code free: %v %v", m, err)
	}

	names := func(ms []*contracts.MemberType) []string {
		var o []string
		for _, m := range ms {
			o = append(o, m.Code)
		}
		return o
	}
	cases := []struct {
		name           string
		search, status string
		want           []string
	}{
		{"all by name", "", "", []string{"ZINC", "GOLD", "SILVER"}},
		{"active", "", "active", []string{"GOLD", "SILVER"}},
		{"archived", "", "archived", []string{"ZINC"}},
		{"code search", "silv", "", []string{"SILVER"}},
		{"name search", "anggota", "", []string{"ZINC"}},
		{"two words", "gold member", "", []string{"GOLD"}},
		{"two words miss", "gold silver", "", nil},
		{"search+status", "member", "active", []string{"GOLD", "SILVER"}},
		{"no match", "qqq", "", nil},
	}
	for _, c := range cases {
		got, err := r.ListMemberTypes(ctx, c.search, c.status)
		if err != nil || !reflect.DeepEqual(names(got), c.want) {
			t.Fatalf("%s: %v err %v want %v", c.name, names(got), err, c.want)
		}
	}

	if err := r.UpdateMemberType(ctx, &contracts.MemberType{ID: g, Name: "Gold 2", Description: "desc", PriceBasis: "purchase_price", Direction: "plus",
		AdjustmentType: "nominal", Adjustment: 1500.5, RoundingMode: "up", RoundingStep: 0}, 6); err != nil {
		t.Fatal(err)
	}
	m, _ = r.MemberTypeByID(ctx, g)
	if m.Name != "Gold 2" || m.Description != "desc" || m.PriceBasis != "purchase_price" || m.Direction != "plus" || m.AdjustmentType != "nominal" ||
		m.Adjustment != 1500.5 || m.RoundingMode != "up" || m.RoundingStep != 0 || m.Code != "GOLD" {
		t.Fatalf("update: %+v", m)
	}
	var ub int64
	var arc sql.NullTime
	_ = testMods.db.QueryRow("SELECT updated_by FROM member_types WHERE id=?", g).Scan(&ub)
	if ub != 6 {
		t.Fatalf("updated_by %d", ub)
	}
	_ = testMods.db.QueryRow("SELECT archived_at FROM member_types WHERE id=?", z).Scan(&arc)
	if !arc.Valid {
		t.Fatal("archived_at set")
	}
	if err := r.SetMemberTypeStatus(ctx, z, "active", 4); err != nil {
		t.Fatal(err)
	}
	_ = testMods.db.QueryRow("SELECT archived_at FROM member_types WHERE id=?", z).Scan(&arc)
	if arc.Valid {
		t.Fatal("archived_at cleared")
	}
	if _, err := r.CreateMemberType(ctx, &contracts.MemberType{Code: "GOLD", Name: "dup", PriceBasis: "selling_price", Direction: "minus", AdjustmentType: "percent", RoundingMode: "none"}, 1); err == nil {
		t.Fatal("duplicate member code must fail")
	}
}
