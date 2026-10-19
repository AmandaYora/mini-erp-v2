package integration

import (
	"sync"
	"testing"

	branchcontracts "mini-erp/internal/modules/branch/contracts"
)

// TestDocumentNumbersUniqueUnderConcurrency (F2a-11): 200 concurrent
// allocations yield 200 distinct numbers (LAST_INSERT_ID atomicity).
func TestDocumentNumbersUniqueUnderConcurrency(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.branch.Service()

	const n = 200
	got := make([]string, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			num, err := svc.NextDocumentNumber(ctx(), fx.branchID, branchcontracts.DocSalesOrder)
			if err != nil {
				t.Errorf("alloc %d: %v", i, err)
				return
			}
			got[i] = num
		}(i)
	}
	wg.Wait()
	seen := map[string]bool{}
	for i, num := range got {
		if num == "" {
			t.Fatalf("alloc %d kosong (lihat error di atas)", i)
		}
		if seen[num] {
			t.Fatalf("nomor ganda: %q", num)
		}
		seen[num] = true
	}
}
