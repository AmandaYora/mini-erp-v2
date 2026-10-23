package application

import (
	"context"
	"sync"
	"time"

	"mini-erp/internal/shared/health"
)

// Data-health cache (ADR-0025, spec §2.2). Per (branch, tier) one computed
// set of check outcomes, shared by the dashboard card and the data-health
// page. At most one computation runs per key; a stale entry (≤ 3× TTL) is
// served at once while one goroutine refreshes it. Snoozes are never cached.

// Tiers of checks: cheap ones every minute, expensive ones every ten minutes
// and only ever in the background.
const (
	TierFast  = "fast"
	TierHeavy = "heavy"
)

const (
	fastTTL  = 60 * time.Second
	heavyTTL = 10 * time.Minute
	// pendingTTL re-runs a tier soon when one of its checks was not ready
	// yet (A1's bcrypt verdicts compute in the background).
	pendingTTL = 5 * time.Second
	// staleFactor bounds how old an entry may be and still be served while
	// it refreshes.
	staleFactor = 3
	// coldWait bounds how long a request waits for a missing fast tier.
	coldWait = 5 * time.Second
	// computeTimeout bounds one background computation.
	computeTimeout = 2 * time.Minute
	// refreshEvery rate-limits "Periksa ulang" per branch.
	refreshEvery = 30 * time.Second
	// maxCacheEntries bounds the map (branches × tiers); the oldest goes.
	maxCacheEntries = 128
)

// checkOutcome is one check's result at computation time.
type checkOutcome struct {
	status     string // ok | alert | pending | error
	result     health.Result
	err        error
	computedAt time.Time
}

// tierResult is one tier's outcomes, keyed by check id.
type tierResult struct {
	outcomes   map[string]checkOutcome
	computedAt time.Time
	pending    bool
}

type cacheKey struct {
	branchID int64
	tier     string
}

type cacheEntry struct {
	value tierResult
}

// healthCache memoizes tier results per branch.
type healthCache struct {
	mu          sync.Mutex
	entries     map[cacheKey]*cacheEntry
	inflight    map[cacheKey]chan struct{}
	lastRefresh map[int64]time.Time
	now         func() time.Time
}

func newHealthCache(now func() time.Time) *healthCache {
	return &healthCache{
		entries:     map[cacheKey]*cacheEntry{},
		inflight:    map[cacheKey]chan struct{}{},
		lastRefresh: map[int64]time.Time{},
		now:         now,
	}
}

func ttlOf(tier string) time.Duration {
	if tier == TierHeavy {
		return heavyTTL
	}
	return fastTTL
}

// get returns the tier result for a branch. compute runs at most once per key
// at a time, detached from the request (a cancelled request must not abort the
// computation others wait on). When wait is false (heavy tier) a missing entry
// returns ok=false immediately; when true the caller waits up to coldWait.
func (c *healthCache) get(ctx context.Context, branchID int64, tier string, wait bool,
	compute func(ctx context.Context) tierResult) (tierResult, bool) {
	key := cacheKey{branchID, tier}
	c.mu.Lock()
	e := c.entries[key]
	now := c.now()
	if e != nil {
		age := now.Sub(e.value.computedAt)
		fresh := ttlOf(tier)
		if e.value.pending {
			fresh = pendingTTL
		}
		if age < fresh {
			v := e.value
			c.mu.Unlock()
			return v, true
		}
		if age <= staleFactor*ttlOf(tier) {
			c.startLocked(key, compute)
			v := e.value
			c.mu.Unlock()
			return v, true
		}
	}
	done := c.startLocked(key, compute)
	c.mu.Unlock()
	if !wait {
		return tierResult{}, false
	}
	timer := time.NewTimer(coldWait)
	defer timer.Stop()
	select {
	case <-done:
	case <-timer.C:
	case <-ctx.Done():
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.entries[key]; e != nil {
		return e.value, true
	}
	return tierResult{}, false
}

// startLocked starts compute for key unless one already runs; it returns the
// channel closed when that computation finishes. c.mu must be held.
func (c *healthCache) startLocked(key cacheKey, compute func(ctx context.Context) tierResult) chan struct{} {
	if ch, ok := c.inflight[key]; ok {
		return ch
	}
	ch := make(chan struct{})
	c.inflight[key] = ch
	go func() {
		defer close(ch)
		ctx, cancel := context.WithTimeout(context.Background(), computeTimeout)
		defer cancel()
		v := compute(ctx)
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.inflight, key)
		c.entries[key] = &cacheEntry{value: v}
		c.evictLocked()
	}()
	return ch
}

// evictLocked drops the oldest entries above maxCacheEntries.
func (c *healthCache) evictLocked() {
	for len(c.entries) > maxCacheEntries {
		var oldest cacheKey
		var at time.Time
		first := true
		for k, e := range c.entries {
			if first || e.value.computedAt.Before(at) {
				oldest, at, first = k, e.value.computedAt, false
			}
		}
		delete(c.entries, oldest)
	}
}

// refresh drops both tiers of a branch so the next read recomputes them. It
// is rate-limited per branch; a refused refresh returns false silently.
func (c *healthCache) refresh(branchID int64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if last, ok := c.lastRefresh[branchID]; ok && now.Sub(last) < refreshEvery {
		return false
	}
	c.lastRefresh[branchID] = now
	delete(c.entries, cacheKey{branchID, TierFast})
	delete(c.entries, cacheKey{branchID, TierHeavy})
	return true
}
