package ratelimit

import (
	"sync"
	"time"
)

// Limiter bounds event rates per key with sweep-expired eviction (never a
// full-map reset, so flooding with fresh keys cannot wipe active counters).
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	limit    int
	interval time.Duration
	now      func() time.Time
}

type bucket struct {
	count int
	reset time.Time
}

// New builds a limiter allowing limit events per interval per key.
func New(limit int, interval time.Duration) *Limiter {
	return &Limiter{buckets: map[string]*bucket{}, limit: limit, interval: interval, now: time.Now}
}

// Allow reports whether key may proceed.
func (l *Limiter) Allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	// Opportunistic sweep: drop expired buckets when the map grows.
	if len(l.buckets) > 4096 {
		for k, b := range l.buckets {
			if now.After(b.reset) {
				delete(l.buckets, k)
			}
		}
		// Hard cap: refuse brand-new keys instead of evicting live ones.
		if len(l.buckets) > 20000 {
			if _, ok := l.buckets[key]; !ok {
				return false
			}
		}
	}
	b, ok := l.buckets[key]
	if !ok || now.After(b.reset) {
		l.buckets[key] = &bucket{count: 1, reset: now.Add(l.interval)}
		return true
	}
	b.count++
	return b.count <= l.limit
}
