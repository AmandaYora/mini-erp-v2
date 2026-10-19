package infrastructure

import (
	"context"
	"sync"
	"time"

	"mini-erp/internal/modules/finance/contracts"
)

// The chart of accounts is small, fixed, and read on every posting.
//
// Every journal line resolves its account by code before it can be written,
// so posting one delivery asked the database for the same handful of rows
// four to eight times, and a POS checkout paid that on the critical path.
// Reports do the same at their head. The chart itself is a few dozen rows
// that change only when an administrator edits them — in THIS module, so
// invalidation needs no cross-module contract, just a reset on every write
// below.
//
// The cache holds the whole chart or nothing: a partial cache would have to
// distinguish "absent because unknown" from "absent because not loaded", and
// a missing account is a real answer here (nil, nil) that callers act on.
type chartCache struct {
	mu       sync.RWMutex
	loaded   bool
	loadedAt time.Time
	// gen counts invalidations. A loader notes it before reading and refuses
	// to store when it moved: otherwise a write landing between the read and
	// the store would leave the pre-write chart cached (F4-13).
	gen uint64
	// afterRead is a test hook run between the database read and the store.
	afterRead func()
	// loader replaces the database read in tests.
	loader   func(context.Context) ([]*contracts.Account, map[string]*contracts.Account, error)
	byCode   map[string]contracts.Account
	byID     map[int64]contracts.Account
	byMapKey map[string]contracts.Account
}

// chartTTL is the safety net for writes nobody routed through invalidate.
const chartTTL = 10 * time.Minute

func newChartCache() *chartCache {
	return &chartCache{}
}

// invalidate drops the chart after any account or mapping write.
func (c *chartCache) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gen++
	c.loaded = false
	c.byCode, c.byID, c.byMapKey = nil, nil, nil
}

// ensureChart returns a loaded chart. When the shared cache is fresh it is
// returned as is. On a cold/expired cache the chart is read; it is stored for
// everyone only if no invalidation happened during the read, otherwise the
// freshly read (possibly already stale) snapshot serves THIS call alone.
// Concurrent callers may both load on a cold cache; that costs one duplicate
// pair of reads and avoids holding a write lock across I/O.
func (r *Repository) ensureChart(ctx context.Context) (*chartCache, error) {
	c := r.chart
	c.mu.RLock()
	fresh := c.loaded && time.Since(c.loadedAt) < chartTTL
	gen := c.gen
	c.mu.RUnlock()
	if fresh {
		return c, nil
	}
	var accounts []*contracts.Account
	var mappings map[string]*contracts.Account
	var err error
	if c.loader != nil {
		accounts, mappings, err = c.loader(ctx)
	} else if accounts, err = r.ListAccounts(ctx, ""); err == nil {
		mappings, err = r.ListMappings(ctx)
	}
	if err != nil {
		return nil, err
	}
	if c.afterRead != nil {
		c.afterRead()
	}
	snap := &chartCache{
		byCode:   make(map[string]contracts.Account, len(accounts)),
		byID:     make(map[int64]contracts.Account, len(accounts)),
		byMapKey: make(map[string]contracts.Account, len(mappings)),
	}
	for _, a := range accounts {
		snap.byCode[a.Code] = *a
		snap.byID[a.ID] = *a
	}
	for k, a := range mappings {
		if a != nil {
			snap.byMapKey[k] = *a
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.gen != gen {
		return snap, nil // invalidated mid-read: use, do not store
	}
	c.byCode, c.byID, c.byMapKey = snap.byCode, snap.byID, snap.byMapKey
	c.loaded, c.loadedAt = true, time.Now()
	return c, nil
}

// chartLookup runs fn against the loaded chart and returns a copy of the hit.
// Copies, never pointers: a caller mutating a returned account must not edit
// the cache every other caller reads.
func (r *Repository) chartLookup(ctx context.Context, fn func(*chartCache) (contracts.Account, bool)) (*contracts.Account, error) {
	c, err := r.ensureChart(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	a, ok := fn(c)
	if !ok {
		return nil, nil
	}
	out := a
	return &out, nil
}
