package infrastructure

import (
	"context"
	"testing"
	"time"

	"mini-erp/internal/modules/finance/contracts"
)

// F4-13: an invalidation that lands between the database read and the cache
// store must not leave the pre-write chart cached.
func TestChartCacheInvalidateDuringLoadIsNotStored(t *testing.T) {
	r := &Repository{chart: newChartCache()}
	name := "lama"
	r.chart.loader = func(context.Context) ([]*contracts.Account, map[string]*contracts.Account, error) {
		return []*contracts.Account{{ID: 1, Code: "1100", Name: name}}, map[string]*contracts.Account{}, nil
	}
	// The write happens after the loader read the old name, before the store.
	r.chart.afterRead = func() { name = "baru"; r.chart.invalidate(); r.chart.afterRead = nil }
	first, err := r.chartLookup(context.Background(), func(c *chartCache) (contracts.Account, bool) {
		a, ok := c.byCode["1100"]
		return a, ok
	})
	if err != nil || first == nil || first.Name != "lama" {
		t.Fatalf("this call may see the snapshot it read: %+v %v", first, err)
	}
	if r.chart.loaded {
		t.Fatal("stale chart was stored in the shared cache")
	}
	second, _ := r.chartLookup(context.Background(), func(c *chartCache) (contracts.Account, bool) {
		a, ok := c.byCode["1100"]
		return a, ok
	})
	if second == nil || second.Name != "baru" {
		t.Fatalf("next call must read the new chart, got %+v", second)
	}
	if !r.chart.loaded {
		t.Fatal("clean load should be cached")
	}
}

func TestChartCacheTTLExpires(t *testing.T) {
	r := &Repository{chart: newChartCache()}
	loads := 0
	r.chart.loader = func(context.Context) ([]*contracts.Account, map[string]*contracts.Account, error) {
		loads++
		return nil, map[string]*contracts.Account{}, nil
	}
	look := func() {
		_, _ = r.chartLookup(context.Background(), func(c *chartCache) (contracts.Account, bool) { return contracts.Account{}, false })
	}
	look()
	look()
	if loads != 1 {
		t.Fatalf("loads = %d, want 1 while fresh", loads)
	}
	r.chart.loadedAt = time.Now().Add(-chartTTL - time.Second)
	look()
	if loads != 2 {
		t.Fatalf("loads = %d, want 2 after TTL", loads)
	}
}
