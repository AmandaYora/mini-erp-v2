package doclock

import (
	"sync"
	"sync/atomic"
)

// Striped mutexes keyed by document identity ("po:12", "so:7", "party:3").
// Serializes concurrent writes against the same document's remaining-qty /
// outstanding math (legacy KI-92 class: two simultaneous receipts overrunning
// a PO because both read the same remaining figure).
//
// Scope: single instance. The deployment is one app container, so in-memory
// serialization is airtight here. A multi-instance topology would need DB
// row locks instead — this package must be replaced then, not extended.
//
// Entries are reference-counted and removed once the last holder releases.
// Keys are document identities, so a map that only ever grew would keep one
// mutex per document the process has ever touched — a leak that scales with
// business volume rather than with concurrency, which is what it should
// actually track.
var (
	mu      sync.Mutex
	stripes = map[string]*stripe{}
)

type stripe struct {
	lock sync.Mutex
	// waiters counts holders plus goroutines queued on lock, guarded by mu —
	// never by lock itself, which is held across the caller's work.
	waiters int
}

// acquire returns the stripe for key, registering this caller as a waiter so
// release cannot delete it out from under a queued goroutine.
func acquire(key string) *stripe {
	mu.Lock()
	defer mu.Unlock()
	s, ok := stripes[key]
	if !ok {
		s = &stripe{}
		stripes[key] = s
	}
	s.waiters++
	return s
}

// release drops this caller's claim and removes the stripe when it was the
// last one. A later caller for the same key simply creates a fresh stripe:
// with no holders left there is nothing to serialize against.
func release(key string) {
	mu.Lock()
	defer mu.Unlock()
	s, ok := stripes[key]
	if !ok {
		return
	}
	s.waiters--
	if s.waiters <= 0 {
		delete(stripes, key)
	}
}

// bypass turns Lock into a plain call. Tests that must prove the DB-level
// guards (CAS status flips, unique keys) set it so the process mutex does not
// mask a race; production never does.
var bypass atomic.Bool

// SetBypass disables serialization. TEST ONLY: call it from _test.go files and
// restore it with defer SetBypass(false).
func SetBypass(on bool) { bypass.Store(on) }

// Lock serializes fn against key. Keep fn short: validation + writes only.
func Lock(key string, fn func() error) error {
	if bypass.Load() {
		return fn()
	}
	s := acquire(key)
	defer release(key)
	s.lock.Lock()
	defer s.lock.Unlock()
	return fn()
}
