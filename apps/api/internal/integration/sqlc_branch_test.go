package integration

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"mini-erp/internal/modules/branch/contracts"
	branchinfra "mini-erp/internal/modules/branch/infrastructure"
)

// Characterization tests for the branch Repository (F8-18): they pin the
// behaviour of every public method so the sqlc rewrite is provably neutral.

func sqlcBranchRepo() *branchinfra.Repository { return branchinfra.NewRepository(testMods.db) }

func sqlcMkBranch(t *testing.T, r *branchinfra.Repository, code, name, city, phone, addr string, head bool, label string) int64 {
	t.Helper()
	id, err := r.Create(context.Background(), &contracts.Branch{
		Code: code, Name: name, City: city, Phone: phone, Address: addr, IsHead: head, DefaultLocationLabel: label,
	}, 7, nil, time.Now().UTC())
	if err != nil {
		t.Fatalf("create %s: %v", code, err)
	}
	return id
}

func TestSqlcBranchGetAndNulls(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcBranchRepo()
	full := sqlcMkBranch(t, r, "B1", "Cabang Satu", "Bandung", "0812", "Jl. A", true, "Gudang X")
	bare := sqlcMkBranch(t, r, "B2", "Cabang Dua", "", "", "", false, "")

	b, err := r.GetByID(ctx, full)
	if err != nil || b == nil {
		t.Fatalf("get full: %v %v", b, err)
	}
	if b.Code != "B1" || b.Name != "Cabang Satu" || b.City != "Bandung" || b.Phone != "0812" || b.Address != "Jl. A" ||
		b.Status != "active" || !b.IsHead || b.DefaultLocationLabel != "Gudang X" {
		t.Fatalf("full mismatch: %+v", b)
	}
	b, err = r.GetByID(ctx, bare)
	if err != nil || b == nil {
		t.Fatalf("get bare: %v %v", b, err)
	}
	if b.City != "" || b.Phone != "" || b.Address != "" || b.IsHead || b.DefaultLocationLabel != "Gudang Utama" {
		t.Fatalf("bare (NULL columns) mismatch: %+v", b)
	}
	if b, err := r.GetByID(ctx, 999999); err != nil || b != nil {
		t.Fatalf("missing must be nil,nil: %v %v", b, err)
	}
}

func TestSqlcBranchGetByIDs(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcBranchRepo()
	c := sqlcMkBranch(t, r, "C", "Charlie", "", "", "", false, "")
	a := sqlcMkBranch(t, r, "A", "Alpha", "", "", "", false, "")
	bb := sqlcMkBranch(t, r, "B", "Bravo", "", "", "", false, "")

	if got, err := r.GetByIDs(ctx, nil); err != nil || len(got) != 0 {
		t.Fatalf("nil ids: %v %v", got, err)
	}
	if got, err := r.GetByIDs(ctx, []int64{}); err != nil || len(got) != 0 {
		t.Fatalf("empty ids: %v %v", got, err)
	}
	got, err := r.GetByIDs(ctx, []int64{c, a, bb, 424242})
	if err != nil || len(got) != 3 {
		t.Fatalf("ids: %v %v", got, err)
	}
	if got[0].Name != "Alpha" || got[1].Name != "Bravo" || got[2].Name != "Charlie" {
		t.Fatalf("must be ordered by name: %v %v %v", got[0].Name, got[1].Name, got[2].Name)
	}
	if got, err := r.GetByIDs(ctx, []int64{424242}); err != nil || len(got) != 0 {
		t.Fatalf("only unknown: %v %v", got, err)
	}
	// large IN list, duplicates tolerated
	many := make([]int64, 0, 2000)
	for i := int64(1000); i < 3000; i++ {
		many = append(many, i)
	}
	many = append(many, a, a, c)
	got, err = r.GetByIDs(ctx, many)
	if err != nil || len(got) != 2 {
		t.Fatalf("large list: %d %v", len(got), err)
	}
}

func TestSqlcBranchExistsAndMutations(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcBranchRepo()
	id1 := sqlcMkBranch(t, r, "X1", "Satu", "Kota", "1", "Alamat", false, "")
	id2 := sqlcMkBranch(t, r, "X2", "Dua", "", "", "", false, "")

	for _, c := range []struct {
		code string
		excl int64
		want bool
	}{{"X1", 0, true}, {"X1", id1, false}, {"X1", id2, true}, {"NOPE", 0, false}} {
		got, err := r.ExistsByCode(ctx, c.code, c.excl)
		if err != nil || got != c.want {
			t.Fatalf("exists %+v: got %v err %v", c, got, err)
		}
	}

	// Update: sets fields, empties become NULL, empty label falls back to default.
	if err := r.Update(ctx, &contracts.Branch{ID: id1, Name: "Satu Baru", Status: "inactive", IsHead: true}, 9); err != nil {
		t.Fatal(err)
	}
	b, _ := r.GetByID(ctx, id1)
	if b.Name != "Satu Baru" || b.Status != "inactive" || !b.IsHead || b.City != "" || b.Phone != "" || b.Address != "" ||
		b.DefaultLocationLabel != "Gudang Utama" || b.Code != "X1" {
		t.Fatalf("update mismatch: %+v", b)
	}
	var updBy int64
	var updatedAt, createdAt time.Time
	if err := testMods.db.QueryRow("SELECT updated_by, updated_at, created_at FROM branches WHERE id = ?", id1).Scan(&updBy, &updatedAt, &createdAt); err != nil {
		t.Fatal(err)
	}
	if updBy != 9 || time.Since(updatedAt) > time.Minute || time.Since(updatedAt) < -time.Minute || !updatedAt.After(createdAt) {
		t.Fatalf("audit columns: by=%d upd=%v created=%v", updBy, updatedAt, createdAt)
	}
	// Update of a missing id is a no-op without error.
	if err := r.Update(ctx, &contracts.Branch{ID: 99999, Name: "x", Status: "active"}, 1); err != nil {
		t.Fatalf("update missing: %v", err)
	}

	// Create seeds the sequences in the same tx.
	kinds := branchinfra.AllDocKinds()
	id, err := r.Create(ctx, &contracts.Branch{Code: "SEQ", Name: "Seq"}, 1, kinds, time.Date(2026, 10, 5, 3, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := testMods.db.QueryRow("SELECT COUNT(*) FROM branch_document_sequences WHERE branch_id = ?", id).Scan(&n); err != nil || n != len(kinds) {
		t.Fatalf("sequences seeded: %d err %v", n, err)
	}
	// Duplicate code fails and leaves no orphan sequences.
	if _, err := r.Create(ctx, &contracts.Branch{Code: "SEQ", Name: "Dup"}, 1, kinds, time.Now().UTC()); err == nil {
		t.Fatal("duplicate code must fail")
	}
	if err := testMods.db.QueryRow("SELECT COUNT(*) FROM branch_document_sequences").Scan(&n); err != nil || n != len(kinds) {
		t.Fatalf("no orphan sequences after failed create: %d err %v", n, err)
	}
}

func TestSqlcBranchChangeCodeAndHasDocuments(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcBranchRepo()
	id := sqlcMkBranch(t, r, "OLD", "Cabang", "", "", "", false, "")

	if has, err := r.HasDocuments(ctx, id); err != nil || has {
		t.Fatalf("no sequences: %v %v", has, err)
	}
	used, err := r.TryChangeCode(ctx, id, "NEW", 5)
	if err != nil || used {
		t.Fatalf("unused change: %v %v", used, err)
	}
	if b, _ := r.GetByID(ctx, id); b.Code != "NEW" {
		t.Fatalf("code not changed: %s", b.Code)
	}
	// sequences at 0 still count as unused
	if _, err := r.NextNumber(ctx, id, "k0"); err != nil {
		t.Fatal(err)
	}
	if has, _ := r.HasDocuments(ctx, id); !has {
		t.Fatal("after a number was issued HasDocuments must be true")
	}
	used, err = r.TryChangeCode(ctx, id, "NEW2", 5)
	if err != nil || !used {
		t.Fatalf("used change: %v %v", used, err)
	}
	if b, _ := r.GetByID(ctx, id); b.Code != "NEW" {
		t.Fatalf("code must stay: %s", b.Code)
	}
}

func TestSqlcBranchListCount(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcBranchRepo()
	ids := map[string]int64{}
	ids["JKT"] = sqlcMkBranch(t, r, "JKT", "Jakarta Pusat", "Jakarta", "", "", true, "")
	ids["BDG"] = sqlcMkBranch(t, r, "BDG", "Bandung Kota", "Bandung", "", "", false, "")
	ids["SBY"] = sqlcMkBranch(t, r, "SBY", "Surabaya Timur", "Surabaya", "", "", false, "")
	ids["MLG"] = sqlcMkBranch(t, r, "MLG", "Malang", "", "", "", false, "") // NULL city
	// deactivate one
	if err := r.Update(ctx, &contracts.Branch{ID: ids["SBY"], Name: "Surabaya Timur", City: "Surabaya", Status: "inactive"}, 1); err != nil {
		t.Fatal(err)
	}

	names := func(bs []*contracts.Branch) []string {
		var out []string
		for _, b := range bs {
			out = append(out, b.Code)
		}
		return out
	}
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	cases := []struct {
		name           string
		search, status string
		limit, offset  int
		want           []string
		total          int64
	}{
		{"all ordered by name", "", "", 50, 0, []string{"BDG", "JKT", "MLG", "SBY"}, 4},
		{"status active", "", "active", 50, 0, []string{"BDG", "JKT", "MLG"}, 3},
		{"status inactive", "", "inactive", 50, 0, []string{"SBY"}, 1},
		{"status none", "", "ghost", 50, 0, nil, 0},
		{"search by code", "jkt", "", 50, 0, []string{"JKT"}, 1},
		{"search by name", "kota", "", 50, 0, []string{"BDG"}, 1},
		{"search by city", "surabaya", "", 50, 0, []string{"SBY"}, 1},
		{"search two words (AND)", "jakarta pusat", "", 50, 0, []string{"JKT"}, 1},
		{"search two words no match", "jakarta malang", "", 50, 0, nil, 0},
		{"search words across columns", "bdg bandung", "", 50, 0, []string{"BDG"}, 1},
		{"search + status", "a", "active", 50, 0, []string{"BDG", "JKT", "MLG"}, 3},
		{"search whitespace only", "   ", "", 50, 0, []string{"BDG", "JKT", "MLG", "SBY"}, 4},
		{"search NULL city only by name", "malang", "", 50, 0, []string{"MLG"}, 1},
		{"page 1", "", "", 2, 0, []string{"BDG", "JKT"}, 4},
		{"page 2", "", "", 2, 2, []string{"MLG", "SBY"}, 4},
		{"offset past end", "", "", 2, 10, nil, 4},
		{"limit 1 offset 3", "", "", 1, 3, []string{"SBY"}, 4},
		{"percent literal", "100%", "", 50, 0, nil, 0},
	}
	for _, c := range cases {
		got, err := r.List(ctx, c.search, c.status, c.limit, c.offset)
		if err != nil {
			t.Fatalf("%s: list: %v", c.name, err)
		}
		if !eq(names(got), c.want) {
			t.Fatalf("%s: got %v want %v", c.name, names(got), c.want)
		}
		total, err := r.Count(ctx, c.search, c.status)
		if err != nil || total != c.total {
			t.Fatalf("%s: count %d err %v want %d", c.name, total, err, c.total)
		}
	}
}

func TestSqlcBranchNextNumber(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	r := sqlcBranchRepo()
	id := sqlcMkBranch(t, r, "SEQ", "Seq", "", "", "", false, "")
	other := sqlcMkBranch(t, r, "OTH", "Oth", "", "", "", false, "")

	// first call creates the missing row (INSERT IGNORE path) and returns 1
	for want := int64(1); want <= 3; want++ {
		n, err := r.NextNumber(ctx, id, "sales_order:2026-10")
		if err != nil || n != want {
			t.Fatalf("seq: got %d err %v want %d", n, err, want)
		}
	}
	// independent keys / branches
	if n, _ := r.NextNumber(ctx, id, "payment"); n != 1 {
		t.Fatalf("other key: %d", n)
	}
	if n, _ := r.NextNumber(ctx, other, "sales_order:2026-10"); n != 1 {
		t.Fatalf("other branch: %d", n)
	}
	// pre-seeded row (Create with kinds) starts at 0
	pre, err := r.Create(ctx, &contracts.Branch{Code: "PRE", Name: "Pre"}, 1, []contracts.DocKind{contracts.DocPayment}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if n, err := r.NextNumber(ctx, pre, "payment"); err != nil || n != 1 {
		t.Fatalf("pre-seeded: %d %v", n, err)
	}

	// concurrency: unique, gap-free numbers
	const workers, per = 8, 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	var all []int64
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < per; i++ {
				n, err := r.NextNumber(ctx, id, "conc")
				if err != nil {
					t.Errorf("concurrent: %v", err)
					return
				}
				mu.Lock()
				all = append(all, n)
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	sort.Slice(all, func(i, j int) bool { return all[i] < all[j] })
	if len(all) != workers*per {
		t.Fatalf("lost numbers: %d", len(all))
	}
	for i, n := range all {
		if n != int64(i+1) {
			t.Fatalf("not gap-free at %d: %v", i, all)
		}
	}
}
