package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	productapp "mini-erp/internal/modules/product/application"
	partyapp "mini-erp/internal/modules/party/application"
)

// Catalog parity (PR-03 … PR-10): search ranking, stock/cost list columns,
// category order, photo upload fields, import warnings, member-type impact
// and explicit-member quotes.

type apiBody struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

func decodeData(t *testing.T, rec *httptest.ResponseRecorder, into any) {
	t.Helper()
	var b apiBody
	if err := json.Unmarshal(rec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, rec.Body.String())
	}
	if err := json.Unmarshal(b.Data, into); err != nil {
		t.Fatalf("decode data: %v (%s)", err, b.Data)
	}
}

func codesOf(ps []*productCodeRow) string {
	var out []string
	for _, p := range ps {
		out = append(out, p.Code)
	}
	return strings.Join(out, ",")
}

type productCodeRow struct{ Code string }

func catalogProduct(t *testing.T, code, name string) int64 {
	t.Helper()
	return f3bProduct(t, productapp.ProductInput{Code: code, Name: name}).ID
}

// PR-03: a typed code puts that product first (exact code, code prefix, name
// prefix, rest); productId hydrates an option even if it is no longer active;
// preferStock floats sellable stock above an equal relevance rank.
func TestProductSearchRankingHydrationAndStockPreference(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.product.Service()
	catalogProduct(t, "AB-1", "Zeta Satu")
	catalogProduct(t, "AB", "Alpha")
	catalogProduct(t, "XAB", "AB Beta")
	q1 := catalogProduct(t, "Q1", "Gamma ab")

	opts, err := svc.SearchOptions(ctx(), productapp.ListFilter{Search: "ab"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []*productCodeRow
	for _, p := range opts {
		got = append(got, &productCodeRow{p.Code})
	}
	if codesOf(got) != "AB,AB-1,XAB,Q1" {
		t.Fatalf("ranked order = %s, want AB,AB-1,XAB,Q1", codesOf(got))
	}

	// Hydration by id works for an archived product; a plain search hides it.
	if err := svc.ArchiveProduct(ctx(), 0, q1); err != nil {
		t.Fatal(err)
	}
	id := q1
	opts, err = svc.SearchOptions(ctx(), productapp.ListFilter{ID: &id}, 5)
	if err != nil || len(opts) != 1 || opts[0].ID != q1 || opts[0].Status != "archived" {
		t.Fatalf("hydrate by productId: %+v %v", opts, err)
	}
	opts, _ = svc.SearchOptions(ctx(), productapp.ListFilter{Search: "Gamma"}, 5)
	if len(opts) != 0 {
		t.Fatalf("archived must stay hidden from a plain search: %+v", opts)
	}

	// preferStock: PS-A (alphabetically first) has no stock, PS-B has.
	psA := f3bProduct(t, productapp.ProductInput{Code: "PS-A", Name: "Aaa stok"})
	psB := f3bProduct(t, productapp.ProductInput{Code: "PS-B", Name: "Bbb stok"})
	f3bStock(t, fx, psB.ID, psB.Variants[0].ID, 5)
	plain, _ := svc.SearchOptionsFor(ctx(), productapp.ListFilter{Search: "stok"}, 5, productapp.SearchOptionsOpts{})
	if len(plain) != 2 || plain[0].ID != psA.ID {
		t.Fatalf("without preference name order wins: %+v", plain)
	}
	pref, _ := svc.SearchOptionsFor(ctx(), productapp.ListFilter{Search: "stok"}, 5,
		productapp.SearchOptionsOpts{PreferStock: true, BranchID: fx.branchID})
	if len(pref) != 2 || pref[0].ID != psB.ID {
		t.Fatalf("preferStock must float the stocked product: %+v", pref)
	}
	// Relevance still beats stock: an exact code stays first although unstocked.
	pref, _ = svc.SearchOptionsFor(ctx(), productapp.ListFilter{Search: "PS-A"}, 5,
		productapp.SearchOptionsOpts{PreferStock: true, BranchID: fx.branchID})
	if len(pref) == 0 || pref[0].ID != psA.ID {
		t.Fatalf("exact code outranks stock: %+v", pref)
	}
}

// PR-05: one grouped stock read for any page size; HTTP list shows stock only
// with stock.view and the purchase price only to cost roles.
func TestProductListStockAndCostColumns(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	cm, ctr := countedMods(t)

	// min stock above the 50 on hand => below minimum.
	p, _ := testMods.product.Service().GetByID(ctx(), fx.productID)
	in := f3bUpdateInput(p)
	in.MinStock = 60
	if _, err := testMods.product.Service().UpdateProduct(ctx(), 0, p.ID, in); err != nil {
		t.Fatal(err)
	}
	ids := []int64{fx.productID}
	for i := 0; i < 9; i++ {
		ids = append(ids, catalogProduct(t, fmt.Sprintf("LC-%02d", i), fmt.Sprintf("Barang %d", i)))
	}
	statements := func(n int) int {
		ctr.Reset()
		m, err := cm.product.Service().StockAvailability(ctx(), fx.branchID, ids[:n])
		if err != nil {
			t.Fatal(err)
		}
		if n >= 1 && m[fx.productID] != 50 {
			t.Fatalf("available = %v, want 50", m[fx.productID])
		}
		return ctr.CountMatching(regexp.MustCompile(`(?i)stock_balances`))
	}
	statements(1)
	if a, b := statements(1), statements(10); a != 1 || b != 1 {
		t.Fatalf("stock availability must be ONE read regardless of page size: 1 id=%d, 10 ids=%d", a, b)
	}

	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatal(err)
	}
	h := routeServer(t).Handler()
	tokStock := guardUser(t, fx.branchID, "pr5-stock", []string{"products.view", "stock.view"})
	tokCost := guardUser(t, fx.branchID, "pr5-cost", []string{"products.view", "products.update"})
	tokBuy := guardUser(t, fx.branchID, "pr5-buy", []string{"products.view", "purchasing.view", "stock.view"})
	tokNone := guardUser(t, fx.branchID, "pr5-none", []string{"products.view"})

	row := func(tok string) map[string]any {
		rec := doReq(h, "GET", "/api/v1/products?search=TST-001", tok, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("list = %d %s", rec.Code, rec.Body.String())
		}
		var items []map[string]any
		decodeData(t, rec, &items)
		if len(items) != 1 {
			t.Fatalf("items = %d", len(items))
		}
		return items[0]
	}
	r := row(tokStock)
	if r["stockAvailable"] != float64(50) || r["belowMinStock"] != true || r["stockUom"] != "pcs" {
		t.Fatalf("stock role must see stock + below-min flag: %v", r)
	}
	if _, ok := r["purchasePrice"]; ok {
		t.Fatalf("stock-only role must NOT see the purchase price: %v", r)
	}
	r = row(tokCost)
	if _, ok := r["purchasePrice"]; !ok {
		t.Fatalf("products.update role sees the purchase price: %v", r)
	}
	if _, ok := r["stockAvailable"]; ok {
		t.Fatalf("no stock.view => no stock column: %v", r)
	}
	r = row(tokBuy)
	if _, ok := r["purchasePrice"]; !ok || r["stockAvailable"] != float64(50) {
		t.Fatalf("purchasing.view + stock.view sees both: %v", r)
	}
	r = row(tokNone)
	if _, ok := r["purchasePrice"]; ok {
		t.Fatalf("plain viewer must not see cost: %v", r)
	}
	if _, ok := r["stockAvailable"]; ok {
		t.Fatalf("plain viewer must not see stock: %v", r)
	}
}

// PR-04/PM-02 backend half: archived rows carry the archive date.
func TestArchivedRowsCarryArchiveDate(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatal(err)
	}
	pid := catalogProduct(t, "ARC-1", "Arsip Satu")
	if err := testMods.product.Service().ArchiveProduct(ctx(), 0, pid); err != nil {
		t.Fatal(err)
	}
	cust, err := testMods.party.Service().CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: "Pelanggan Arsip"})
	if err != nil {
		t.Fatal(err)
	}
	if err := testMods.party.Service().Archive(ctx(), 0, cust.ID, "customer"); err != nil {
		t.Fatal(err)
	}
	h := routeServer(t).Handler()
	tok := guardUser(t, fx.branchID, "arc-all", catalogCodes())

	var items []map[string]any
	decodeData(t, doReq(h, "GET", "/api/v1/products?status=archived", tok, ""), &items)
	if len(items) != 1 || items[0]["archivedAt"] == nil {
		t.Fatalf("archived product must carry archivedAt: %v", items)
	}
	items = nil
	decodeData(t, doReq(h, "GET", "/api/v1/products?status=active", tok, ""), &items)
	for _, it := range items {
		if it["archivedAt"] != nil {
			t.Fatalf("active product must not carry archivedAt: %v", it)
		}
	}
	items = nil
	decodeData(t, doReq(h, "GET", "/api/v1/customers?status=archived", tok, ""), &items)
	if len(items) != 1 || items[0]["archivedAt"] == nil {
		t.Fatalf("archived customer must carry archivedAt: %v", items)
	}
	// Restoring clears it.
	if err := testMods.party.Service().Restore(ctx(), 0, cust.ID, "customer"); err != nil {
		t.Fatal(err)
	}
	items = nil
	decodeData(t, doReq(h, "GET", "/api/v1/customers?search=Arsip", tok, ""), &items)
	if len(items) != 1 || items[0]["archivedAt"] != nil {
		t.Fatalf("restored customer has no archive date: %v", items)
	}
}

// PR-06: sortOrder is stored, validated, kept on update when omitted, and
// drives the category order (then name).
func TestCategorySortOrder(t *testing.T) {
	freshDB(t)
	svc := testMods.product.Service()
	ord := func(n int) *int { return &n }
	a, err := svc.CreateCategory(ctx(), 0, "CA", "Alfa", nil, ord(3))
	if err != nil || a.SortOrder != 3 {
		t.Fatalf("create with sort: %+v %v", a, err)
	}
	b, _ := svc.CreateCategory(ctx(), 0, "CB", "Bravo", nil, ord(1))
	c, _ := svc.CreateCategory(ctx(), 0, "CC", "Charlie", nil, nil) // default 0
	d, _ := svc.CreateCategory(ctx(), 0, "CD", "Delta", nil, ord(1))
	names := func() string {
		cats, err := svc.ListCategories(ctx())
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, x := range cats {
			out = append(out, x.Name)
		}
		return strings.Join(out, ",")
	}
	if got := names(); got != "Charlie,Bravo,Delta,Alfa" {
		t.Fatalf("order = %s, want Charlie(0),Bravo(1),Delta(1),Alfa(3)", got)
	}
	// Omitted sortOrder on update keeps it; an explicit one moves the row.
	if u, err := svc.UpdateCategory(ctx(), 0, a.ID, "Alfa 2", nil, nil); err != nil || u.SortOrder != 3 {
		t.Fatalf("update w/o sort keeps it: %+v %v", u, err)
	}
	if _, err := svc.UpdateCategory(ctx(), 0, a.ID, "Alfa 2", nil, ord(0)); err != nil {
		t.Fatal(err)
	}
	if got := names(); got != "Alfa 2,Charlie,Bravo,Delta" {
		t.Fatalf("after move: %s", got)
	}
	if _, err := svc.CreateCategory(ctx(), 0, "CE", "Echo", nil, ord(10000)); err == nil {
		t.Fatal("sortOrder above 9999 must be refused")
	}
	if _, err := svc.UpdateCategory(ctx(), 0, d.ID, "Delta", nil, ord(-1)); err == nil {
		t.Fatal("negative sortOrder must be refused")
	}
	_, _ = b, c
}

// PR-07: upload accepts sort_order and is_primary; bad values are 400 before
// any byte is stored.
func TestProductMediaUploadFields(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatal(err)
	}
	h := routeServer(t).Handler()
	tok := guardUser(t, fx.branchID, "pr7-all", catalogCodes())
	raw := f3bJPEG(t, 40, 30)

	upload := func(fields map[string]string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		for k, v := range fields {
			_ = mw.WriteField(k, v)
		}
		fw, _ := mw.CreateFormFile("file", "foto.jpg")
		_, _ = fw.Write(raw)
		_ = mw.Close()
		req := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/products/%d/media", fx.productID), &buf)
		req.RemoteAddr = "203.0.113.9:5000"
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	if rec := upload(nil); rec.Code != http.StatusCreated {
		t.Fatalf("first upload = %d %s", rec.Code, rec.Body.String())
	}
	for _, bad := range []map[string]string{{"sort_order": "abc"}, {"sort_order": "-1"}, {"sort_order": "10000"}, {"is_primary": "maybe"}} {
		if rec := upload(bad); rec.Code != http.StatusBadRequest {
			t.Fatalf("%v = %d, want 400", bad, rec.Code)
		}
	}
	if n := f3bCount(t, `SELECT COUNT(*) FROM media_files WHERE owner_type='product' AND owner_id=`+fmt.Sprint(fx.productID)); n != 1 {
		t.Fatalf("a refused upload must store nothing, have %d files", n)
	}
	rec := upload(map[string]string{"sort_order": "7", "is_primary": "true"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("second upload = %d %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	decodeData(t, rec, &created)
	if created["isPrimary"] != true {
		t.Fatalf("is_primary=true must make it primary: %v", created)
	}
	var sort, primaries int
	if err := testMods.db.QueryRow(`SELECT sort_order FROM media_files WHERE id = ?`, int64(created["id"].(float64))).Scan(&sort); err != nil || sort != 7 {
		t.Fatalf("sort_order stored = %d (%v)", sort, err)
	}
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM media_files WHERE owner_type='product' AND owner_id=? AND is_primary=1`, fx.productID).Scan(&primaries); err != nil || primaries != 1 {
		t.Fatalf("exactly one primary expected, have %d (%v)", primaries, err)
	}
	// Plain upload without fields keeps the old behavior: not primary.
	rec = upload(nil)
	decodeData(t, rec, &created)
	if rec.Code != http.StatusCreated || created["isPrimary"] != false {
		t.Fatalf("plain extra upload must not be primary: %d %v", rec.Code, created)
	}
}

// PR-08: inline legacy attribute columns are an error per non-blank cell that
// points at the right sheet; blank cells and a clean file are untouched.
func TestImportFlagsInlineAttributeColumns(t *testing.T) {
	freshDB(t)
	svc := testMods.product.Service()
	header := append(append([]any{}, f3bProductHeader...), "Info tambahan: Warna", "Atribut: Merek")
	row := func(code string, warna, merek any) []any {
		return []any{code, "Produk " + code, "", "Barang", "Ya", "pcs", "pcs", "pcs", 1, 1, "1000", "2000", 0, "Aktif", warna, merek}
	}
	raw := f3bWorkbook(t, []string{"Daftar Produk"}, f3bSheets{"Daftar Produk": {header,
		row("INL-1", "Merah", ""),
		row("INL-2", "", ""),
	}})
	prev, err := svc.PreviewImport(ctx(), bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if prev.OKToCommit || prev.Summary.Errors != 1 {
		t.Fatalf("one non-blank inline cell => one error and no commit: ok=%v %+v %+v", prev.OKToCommit, prev.Summary, prev.Issues)
	}
	var found bool
	for _, is := range prev.Issues {
		if is.Column == "Info tambahan: Warna" && is.Row == 2 && is.Severity == "error" &&
			strings.Contains(is.Message, "Info Tambahan Produk") {
			found = true
		}
	}
	if !found {
		t.Fatalf("issue must name the column, row and the right sheet: %+v", prev.Issues)
	}
	// Commit refuses the same file instead of silently dropping the value.
	if res, err := svc.CommitImport(ctx(), 0, bytes.NewReader(raw)); err == nil && res != nil && res.Imported.ProductsCreated > 0 {
		t.Fatalf("commit must be blocked: %+v", res)
	}
	if n := f3bCount(t, `SELECT COUNT(*) FROM products`); n != 0 {
		t.Fatalf("nothing may be written, have %d products", n)
	}
	// Same file with empty inline cells is clean.
	clean := f3bWorkbook(t, []string{"Daftar Produk"}, f3bSheets{"Daftar Produk": {header, row("INL-3", "", "")}})
	prev, err = svc.PreviewImport(ctx(), bytes.NewReader(clean))
	if err != nil || !prev.OKToCommit || prev.Summary.Errors != 0 {
		t.Fatalf("blank inline cells are fine: %v %+v %+v", err, prev.Summary, prev.Issues)
	}
}

// PR-09 (D3): the archive dialog's impact figure counts ACTIVE customers on
// the type only; archiving stays allowed.
func TestMemberTypeCustomerCount(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatal(err)
	}
	ps := testMods.party.Service()
	m, err := ps.CreateMemberType(ctx(), 0, partyapp.MemberTypeInput{Code: "GOLD", Name: "Gold", Basis: "selling_price",
		Direction: "minus", Type: "percent", Value: 5})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := ps.CreateMemberType(ctx(), 0, partyapp.MemberTypeInput{Code: "SILV", Name: "Silver", Basis: "selling_price",
		Direction: "minus", Type: "percent", Value: 2})
	mk := func(name string, mt int64) int64 {
		c, err := ps.CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: name, MemberTypeID: &mt})
		if err != nil {
			t.Fatal(err)
		}
		return c.ID
	}
	mk("G1", m.ID)
	mk("G2", m.ID)
	gone := mk("G3", m.ID)
	mk("S1", other.ID)
	if err := ps.Archive(ctx(), 0, gone, "customer"); err != nil {
		t.Fatal(err)
	}
	// A supplier row never counts (only customers hold member types anyway).
	h := routeServer(t).Handler()
	tok := guardUser(t, fx.branchID, "pr9-view", []string{"member_types.view"})
	get := func(id int64) (int, map[string]any) {
		rec := doReq(h, "GET", fmt.Sprintf("/api/v1/member-types/%d/customer-count", id), tok, "")
		var d map[string]any
		if rec.Code == http.StatusOK {
			decodeData(t, rec, &d)
		}
		return rec.Code, d
	}
	if code, d := get(m.ID); code != 200 || d["activeCustomers"] != float64(2) || d["memberTypeId"] != float64(m.ID) {
		t.Fatalf("gold = %d %v, want 2 active", code, d)
	}
	if code, d := get(other.ID); code != 200 || d["activeCustomers"] != float64(1) {
		t.Fatalf("silver = %d %v", code, d)
	}
	if code, _ := get(999999); code != http.StatusNotFound {
		t.Fatalf("unknown type = %d, want 404", code)
	}
	tokNo := guardUser(t, fx.branchID, "pr9-no", []string{"products.view"})
	if rec := doReq(h, "GET", fmt.Sprintf("/api/v1/member-types/%d/customer-count", m.ID), tokNo, ""); rec.Code != http.StatusForbidden {
		t.Fatalf("without member_types.view = %d, want 403", rec.Code)
	}
	// Archiving is still allowed and the figure stays readable.
	if err := ps.ArchiveMemberType(ctx(), 0, m.ID); err != nil {
		t.Fatalf("archive must stay allowed (D3 default): %v", err)
	}
	if code, d := get(m.ID); code != 200 || d["activeCustomers"] != float64(2) {
		t.Fatalf("after archive = %d %v", code, d)
	}
}

// PR-10: quote prices an explicit member type without a customer; it cannot
// be combined with customerId; unknown 404; archived degrades to standard.
func TestQuoteExplicitMemberType(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatal(err)
	}
	ps := testMods.party.Service()
	m, err := ps.CreateMemberType(ctx(), 0, partyapp.MemberTypeInput{Code: "TEN", Name: "Min 10", Basis: "selling_price",
		Direction: "minus", Type: "percent", Value: 10})
	if err != nil {
		t.Fatal(err)
	}
	h := routeServer(t).Handler()
	tok := guardUser(t, fx.branchID, "pr10-q", []string{"pricing.quote"})
	post := func(body string) *httptest.ResponseRecorder {
		return doReq(h, "POST", "/api/v1/pricing/quote", tok, body)
	}
	type quote struct {
		MemberType     string `json:"memberType"`
		MemberInactive bool   `json:"memberInactive"`
		Lines          []struct {
			StandardPrice int64 `json:"standardPrice"`
			MemberPrice   int64 `json:"memberPrice"`
			Applied       bool  `json:"applied"`
		} `json:"lines"`
	}
	rec := post(fmt.Sprintf(`{"memberTypeId":%d,"productIds":[%d]}`, m.ID, fx.productID))
	if rec.Code != 200 {
		t.Fatalf("explicit quote = %d %s", rec.Code, rec.Body.String())
	}
	var q quote
	decodeData(t, rec, &q)
	if q.MemberType != "TEN" || len(q.Lines) != 1 || !q.Lines[0].Applied || q.Lines[0].StandardPrice != 15000 || q.Lines[0].MemberPrice != 13500 {
		t.Fatalf("explicit member quote: %+v", q)
	}
	if rec := post(fmt.Sprintf(`{"memberTypeId":%d,"customerId":%d,"productIds":[%d]}`, m.ID, fx.customerID, fx.productID)); rec.Code != http.StatusBadRequest {
		t.Fatalf("memberTypeId + customerId = %d, want 400", rec.Code)
	}
	if rec := post(fmt.Sprintf(`{"memberTypeId":999999,"productIds":[%d]}`, fx.productID)); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown member type = %d, want 404", rec.Code)
	}
	if rec := post(`{"memberTypeId":` + fmt.Sprint(m.ID) + `,"productIds":[]}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("no lines = %d, want 400", rec.Code)
	}
	// Archived type: standard prices + inactive marker; walk-in call unchanged.
	if err := ps.ArchiveMemberType(ctx(), 0, m.ID); err != nil {
		t.Fatal(err)
	}
	rec = post(fmt.Sprintf(`{"memberTypeId":%d,"productIds":[%d]}`, m.ID, fx.productID))
	q = quote{}
	decodeData(t, rec, &q)
	if rec.Code != 200 || !q.MemberInactive || q.Lines[0].Applied || q.Lines[0].MemberPrice != 15000 {
		t.Fatalf("archived type degrades to standard: %d %+v", rec.Code, q)
	}
	rec = post(fmt.Sprintf(`{"productIds":[%d]}`, fx.productID))
	q = quote{}
	decodeData(t, rec, &q)
	if rec.Code != 200 || q.MemberType != "" || q.Lines[0].MemberPrice != 15000 {
		t.Fatalf("walk-in quote unchanged: %d %+v", rec.Code, q)
	}
	_ = bootstrap.AllPermissionCodes
}
