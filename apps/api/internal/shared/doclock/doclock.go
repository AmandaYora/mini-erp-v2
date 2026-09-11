package doclock

import (
	"sync"
)

// Striped mutexes keyed by document identity ("po:12", "so:7", "party:3").
// Serializes concurrent writes against the same document's remaining-qty /
// outstanding math (legacy KI-92 class: two simultaneous receipts overrunning
// a PO because both read the same remaining figure).
//
// Scope: single instance. The deployment is one app container, so in-memory
// serialization is airtight here. A multi-instance topology would need DB
// row locks instead — this package must be replaced then, not extended.
var (
	mu      sync.Mutex
	stripes = map[string]*sync.Mutex{}
)

func keyLock(key string) *sync.Mutex {
	mu.Lock()
	defer mu.Unlock()
	if l, ok := stripes[key]; ok {
		return l
	}
	l := &sync.Mutex{}
	stripes[key] = l
	return l
}

// Lock serializes fn against key. Keep fn short: validation + writes only.
func Lock(key string, fn func() error) error {
	l := keyLock(key)
	l.Lock()
	defer l.Unlock()
	return fn()
}
