package bootstrap

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// repoRoot anchors file walks at the monorepo root regardless of where the
// test binary runs.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller gagal")
	}
	// file = <root>/apps/api/internal/bootstrap/catalog_test.go:
	// bootstrap -> internal -> api -> apps -> root = 4 levels up.
	dir := filepath.Dir(file)
	for i := 0; i < 4; i++ {
		dir = filepath.Dir(dir)
	}
	return dir
}

var permRe = regexp.MustCompile(`perm\("([^"]+)"\)`)

// TestCatalogCoversRoutes (F1-6): every permission code guarding a route
// must exist in the aggregated catalog — a new route code without a catalog
// entry fails closed forever on fresh databases.
func TestCatalogCoversRoutes(t *testing.T) {
	root := repoRoot(t)
	known := map[string]bool{}
	for _, s := range AllPermissionSeeds() {
		known[s.Code] = true
	}
	var missing []string
	seen := map[string]bool{}
	err := filepath.Walk(filepath.Join(root, "apps", "api", "internal", "modules"),
		func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
				return err
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, m := range permRe.FindAllSubmatch(raw, -1) {
				code := string(m[1])
				if !seen[code] {
					seen[code] = true
					if !known[code] {
						rel, _ := filepath.Rel(root, path)
						missing = append(missing, code+" ("+rel+")")
					}
				}
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("kode rute tanpa katalog: %v (tambah ke contracts modul + bootstrap.catalogGroups)", missing)
	}
}

// TestStockApproveCatalogued pins the known non-route code: stock.approve is
// consumed by the adjustment approver check, not by a route guard, so the
// route scan above cannot see it — it must still be catalogued.
func TestStockApproveCatalogued(t *testing.T) {
	known := map[string]bool{}
	for _, s := range AllPermissionSeeds() {
		known[s.Code] = true
	}
	if !known["stock.approve"] {
		t.Fatal("stock.approve dipakai checkApprover tetapi tak terkatalog")
	}
}

// TestDuplicateCodesEmpty enforces one code, one owner across catalogs.
func TestDuplicateCodesEmpty(t *testing.T) {
	if dups := DuplicateCodes(); len(dups) > 0 {
		t.Fatalf("kode ganda antar katalog: %v", dups)
	}
}

// TestCatalogNamesNonEmpty guards the seed UI: every code needs a display name.
func TestCatalogNamesNonEmpty(t *testing.T) {
	for _, s := range AllPermissionSeeds() {
		if strings.TrimSpace(s.Name) == "" {
			t.Fatalf("kode %q tanpa nama tampil", s.Code)
		}
	}
}
