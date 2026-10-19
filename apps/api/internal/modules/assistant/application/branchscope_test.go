package application

import (
	"context"
	"errors"
	"strings"
	"testing"

	"mini-erp/internal/modules/assistant/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/shared/apperror"
)

// --- fakes ------------------------------------------------------------------

type multiBranches struct {
	branchcontracts.BranchClient
	list []*branchcontracts.Branch
}

func (m multiBranches) ListAll(context.Context) ([]*branchcontracts.Branch, error) { return m.list, nil }

// recordingFinance records which branches NetProfit was asked about (the
// isolation tests assert nothing outside the scope is ever read).
type recordingFinance struct {
	financecontracts.FinanceClient
	seen    []int64
	failErr error
}

func (f *recordingFinance) NetProfit(_ context.Context, branchID int64, _, _ string) (int64, int64, int64, error) {
	f.seen = append(f.seen, branchID)
	if f.failErr != nil {
		return 0, 0, 0, f.failErr
	}
	return 1000, 400, 600, nil
}

func (f *recordingFinance) InventoryValue(context.Context, int64) ([]financecontracts.InventoryPosition, int64, error) {
	if f.failErr != nil {
		return nil, 0, f.failErr
	}
	return []financecontracts.InventoryPosition{{ProductID: 7, Qty: 4}}, 0, nil
}

func scopeBranches() []*branchcontracts.Branch {
	return []*branchcontracts.Branch{
		{ID: 1, Code: "JKT", Name: "Cabang Pusat", City: "Jakarta", Status: "active"},
		{ID: 2, Code: "BDG", Name: "Cabang Bandung", City: "Bandung", Status: "active"},
		{ID: 3, Code: "SBY", Name: "Toko Surabaya Timur", City: "Surabaya", Status: "active"},
		{ID: 4, Code: "JKS", Name: "Cabang Jakarta Selatan", City: "Jakarta", Status: "active"},
		{ID: 5, Code: "MLG", Name: "Cabang Malang", City: "Malang", Status: "inactive"},
	}
}

func newScopeEngine(fin *recordingFinance) *Engine {
	return NewEngine(multiBranches{list: scopeBranches()}, fakeProducts{}, &fakeSales{}, fakePurchasing{}, nil, fin)
}

func names(bs []*branchcontracts.Branch) string {
	var out []string
	for _, b := range bs {
		out = append(out, b.Code)
	}
	return strings.Join(out, ",")
}

// --- resolver (AS-01) ---------------------------------------------------------

func TestMatchBranchesByNameCodeCity(t *testing.T) {
	active := []*branchcontracts.Branch{scopeBranches()[1], scopeBranches()[2], scopeBranches()[0]}
	cases := []struct {
		text string
		want string
	}{
		{"penjualan Bandung hari ini", "BDG"},             // city / name without the generic lead word
		{"omzet cabang bandung", "BDG"},                   // full name
		{"omzet bdg kemarin", "BDG"},                      // code, any case
		{"stok kritis surabaya timur", "SBY"},             // multi-word name part
		{"omzet SBY", "SBY"},                              // code
		{"omzet hari ini", ""},                            // no name -> none
		{"omzet bandungan", ""},                           // whole word only, never a substring
		{"omzet jakarta", "JKT"},                          // city of the only accessible Jakarta branch
		{"bandung dan surabaya", "BDG,SBY"},               // two distinct branches -> ambiguous upstream
		{"omzet Cabang Bandung, hari ini!", "BDG"},        // punctuation
		{"info produk 'bandung kopi'", "BDG"},             // raw matcher sees the quote; Run strips it (below)
	}
	for _, c := range cases {
		got := names(matchBranches(c.text, active))
		if got != c.want {
			// result order is by name: Cabang Bandung < Toko Surabaya Timur.
			t.Errorf("matchBranches(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

func TestMatchBranchesPrefersTheMoreSpecificName(t *testing.T) {
	all := scopeBranches()[:4]
	// "jakarta selatan" matches JKS fully and JKT/JKS by city "jakarta": the
	// longer phrase wins instead of calling it ambiguous.
	if got := names(matchBranches("omzet jakarta selatan", all)); got != "JKS" {
		t.Errorf("jakarta selatan -> %q, want JKS", got)
	}
	// Plain "jakarta" is the city of two branches: genuinely ambiguous.
	if got := names(matchBranches("omzet jakarta", all)); got != "JKS,JKT" && got != "JKT,JKS" {
		t.Errorf("jakarta -> %q, want both", got)
	}
}

func TestOrderNumberAndQuotedProductNeverPickABranch(t *testing.T) {
	all := scopeBranches()
	if got := names(matchBranches(stripNonBranchTokens("status ORD-BDG/PJ/2026/09/00001"), all)); got != "" {
		t.Errorf("order number picked %q", got)
	}
	if got := names(matchBranches(stripNonBranchTokens(`info "Kopi Bandung"`), all)); got != "" {
		t.Errorf("quoted product picked %q", got)
	}
}

func TestRunNarrowsToTheNamedBranch(t *testing.T) {
	fin := &recordingFinance{}
	e := newScopeEngine(fin)
	out, err := e.Run(context.Background(), Scope{}, "penjualan Bandung hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if len(fin.seen) != 1 || fin.seen[0] != 2 {
		t.Fatalf("finance read branches %v, want only Bandung (2)", fin.seen)
	}
	if out.BranchID != 2 || out.BranchName != "Cabang Bandung" {
		t.Errorf("outcome branch = %d %q", out.BranchID, out.BranchName)
	}
	if !strings.HasPrefix(out.Answer, "Cabang: Cabang Bandung\n") {
		t.Errorf("answer lacks the branch label: %q", out.Answer)
	}
}

func TestRunAmbiguousBranchAsksForClarification(t *testing.T) {
	fin := &recordingFinance{}
	e := newScopeEngine(fin)
	out, err := e.Run(context.Background(), Scope{}, "omzet jakarta hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if len(fin.seen) != 0 || len(out.Calls) != 0 {
		t.Fatalf("ambiguous branch must not read data: finance=%v calls=%d", fin.seen, len(out.Calls))
	}
	for _, want := range []string{"Cabang mana", "Cabang Pusat", "Cabang Jakarta Selatan"} {
		if !strings.Contains(out.Answer, want) {
			t.Errorf("clarification missing %q: %q", want, out.Answer)
		}
	}
}

func TestRunWithoutBranchNameCoversAllActiveBranches(t *testing.T) {
	fin := &recordingFinance{}
	e := newScopeEngine(fin)
	out, err := e.Run(context.Background(), Scope{}, "omzet hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if len(fin.seen) != 4 { // the inactive Malang branch is skipped
		t.Fatalf("finance read branches %v, want the 4 active ones", fin.seen)
	}
	if out.BranchID != 0 || out.BranchName != "" || strings.HasPrefix(out.Answer, "Cabang:") {
		t.Errorf("multi-branch answer must not claim one branch: %+v", out)
	}
}

func TestRunExplicitBranchWinsOverTextAndInactiveStaysReachable(t *testing.T) {
	fin := &recordingFinance{}
	e := newScopeEngine(fin)
	out, err := e.Run(context.Background(), Scope{BranchID: 3}, "omzet bandung")
	if err != nil {
		t.Fatal(err)
	}
	if len(fin.seen) != 1 || fin.seen[0] != 3 || out.BranchID != 3 {
		t.Fatalf("explicit pick must win: finance=%v out=%d", fin.seen, out.BranchID)
	}
	if _, err := e.Run(context.Background(), Scope{BranchID: 99}, "omzet"); err == nil {
		t.Error("unknown explicit branch must be rejected")
	}
}

func TestRunHelpMentionsBranches(t *testing.T) {
	e := newScopeEngine(&recordingFinance{})
	out, err := e.Run(context.Background(), Scope{}, "bantuan")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Answer, "nama, kode, atau kota cabang") {
		t.Errorf("help text must explain branch naming: %q", out.Answer)
	}
}

// --- branch isolation (AS-01) -----------------------------------------------

func TestRestrictedScopeNeverReadsOtherBranches(t *testing.T) {
	fin := &recordingFinance{}
	e := newScopeEngine(fin)
	mine := Scope{Restricted: true, Allowed: []int64{1}} // only Cabang Pusat

	// Naming a branch outside the scope: refusal, nothing read, no leak.
	out, err := e.Run(context.Background(), mine, "penjualan Bandung hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if len(fin.seen) != 0 || len(out.Calls) != 0 {
		t.Fatalf("read a forbidden branch: finance=%v calls=%d", fin.seen, len(out.Calls))
	}
	if !strings.Contains(out.Answer, "tidak punya akses ke cabang Cabang Bandung") {
		t.Errorf("answer = %q", out.Answer)
	}

	// No name: "all" means all of MY branches.
	fin.seen = nil
	if _, err := e.Run(context.Background(), mine, "omzet hari ini"); err != nil {
		t.Fatal(err)
	}
	if len(fin.seen) != 1 || fin.seen[0] != 1 {
		t.Fatalf("restricted 'all' read %v, want only [1]", fin.seen)
	}

	// Explicit pick outside the scope: 403, before anything runs.
	if _, err := e.Run(context.Background(), Scope{BranchID: 2, Restricted: true, Allowed: []int64{1}}, "omzet"); err == nil {
		t.Error("explicit branch outside access must fail")
	} else if ae, ok := err.(*apperror.AppError); !ok || ae.Code != apperror.CodeForbidden {
		t.Errorf("err = %v, want forbidden", err)
	}
	if err := e.CheckScope(context.Background(), Scope{BranchID: 2, Restricted: true, Allowed: []int64{1}}); err == nil {
		t.Error("CheckScope must reject a branch outside access")
	}

	// Ambiguity is judged among accessible branches only: Jakarta is ambiguous
	// for someone with both Jakarta branches, plain for someone with one.
	fin.seen = nil
	if _, err := e.Run(context.Background(), Scope{Restricted: true, Allowed: []int64{1}}, "omzet jakarta"); err != nil {
		t.Fatal(err)
	}
	if len(fin.seen) != 1 || fin.seen[0] != 1 {
		t.Errorf("jakarta for a one-branch user read %v, want [1]", fin.seen)
	}

	// No accessible branch at all.
	out, err = e.Run(context.Background(), Scope{Restricted: true}, "omzet hari ini")
	if err != nil || !strings.Contains(out.Answer, "belum punya akses") {
		t.Errorf("no access -> %v %q", err, out.Answer)
	}
}

// --- product_info completeness & failed tool rows (AS-02) --------------------

func TestProductInfoAnswerIsComplete(t *testing.T) {
	products := fakeProducts{items: []*productcontracts.Product{{
		ID: 7, Code: "B-1", Name: "Beras Putih", Status: "active", CategoryName: "Sembako",
		Tracked: true, BaseUOM: "kg", PurchaseUOM: "karung", SalesUOM: "kg", MinStock: 20, SellingPrice: 15000,
	}}}
	e := NewEngine(multiBranches{list: scopeBranches()[:1]}, products, &fakeSales{}, fakePurchasing{}, nil, &recordingFinance{})
	_, ans, _, err := e.Execute(context.Background(), 0, "info B-1")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"B-1 — Beras Putih", "Status: aktif", "Kategori: Sembako", "Harga jual Rp 15.000",
		"Satuan beli/jual: karung/kg", "Minimum stok: 20 kg", "stok 4",
	} {
		if !strings.Contains(ans, want) {
			t.Errorf("answer missing %q:\n%s", want, ans)
		}
	}
}

func TestToolFailureIsRecordedWithErrorText(t *testing.T) {
	fin := &recordingFinance{failErr: errors.New("db down")}
	e := newScopeEngine(fin)
	out, err := e.Run(context.Background(), Scope{BranchID: 2}, "omzet hari ini")
	if err == nil {
		t.Fatal("tool failure must surface as an error")
	}
	if out == nil || len(out.Calls) != 1 {
		t.Fatalf("calls = %+v", out)
	}
	c := out.Calls[0]
	if c.Status != contracts.ToolFailed || c.Error != "db down" || c.Tool != "sales_summary" {
		t.Errorf("failed call = %+v", c)
	}
	in, _ := c.Input.(map[string]any)
	if in["branch"] != "BDG" {
		t.Errorf("failed call input = %v, want the branch it ran for", c.Input)
	}

	// A good run's calls are marked success.
	ok := &recordingFinance{}
	out, err = newScopeEngine(ok).Run(context.Background(), Scope{BranchID: 2}, "omzet hari ini")
	if err != nil || out.Calls[0].Status != contracts.ToolSuccess || out.Calls[0].Error != "" {
		t.Errorf("success call = %+v err=%v", out.Calls, err)
	}
}
