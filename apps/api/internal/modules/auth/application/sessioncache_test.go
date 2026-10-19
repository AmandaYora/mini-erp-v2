package application

import (
	"testing"
	"time"

	"mini-erp/internal/modules/auth/contracts"
)

func sample(userID int64, perms ...string) contracts.Session {
	return contracts.Session{ID: 7, UserID: userID, BranchID: 3, BranchActive: true,
		RoleID: 1, RoleCode: "admin", Permissions: perms}
}

func TestSessionCacheHitAndMiss(t *testing.T) {
	c := newSessionCache(time.Minute)
	now := time.Now()
	if _, ok := c.get(7, now); ok {
		t.Fatal("empty cache reported a hit")
	}
	c.put(7, sample(1, "sales.view"), now)
	got, ok := c.get(7, now)
	if !ok {
		t.Fatal("stored session missed")
	}
	if !got.HasPermission("sales.view") || got.UserID != 1 {
		t.Fatalf("session = %+v, want user 1 with sales.view", got)
	}
}

// A hit must hand back a copy: one request mutating its session must not
// rewrite what every other request sees.
func TestSessionCacheReturnsCopy(t *testing.T) {
	c := newSessionCache(time.Minute)
	now := time.Now()
	c.put(7, sample(1, "sales.view"), now)
	first, _ := c.get(7, now)
	first.BranchID = 999
	second, _ := c.get(7, now)
	if second.BranchID != 3 {
		t.Fatalf("branch = %d, want 3 — the cache handed out its own value", second.BranchID)
	}
}

func TestSessionCacheExpires(t *testing.T) {
	c := newSessionCache(time.Second)
	now := time.Now()
	c.put(7, sample(1), now)
	if _, ok := c.get(7, now.Add(2*time.Second)); ok {
		t.Fatal("expired entry still served")
	}
}

// Logout and branch/role switch drop exactly one session.
func TestSessionCacheDrop(t *testing.T) {
	c := newSessionCache(time.Minute)
	now := time.Now()
	c.put(7, sample(1), now)
	c.put(8, sample(2), now)
	c.drop(7)
	if _, ok := c.get(7, now); ok {
		t.Fatal("dropped session still served")
	}
	if _, ok := c.get(8, now); !ok {
		t.Fatal("drop removed an unrelated session")
	}
}

// Deactivating a user or changing their roles must retire that user's
// sessions and only theirs.
func TestSessionCacheDropUser(t *testing.T) {
	c := newSessionCache(time.Minute)
	now := time.Now()
	c.put(7, sample(1), now)
	c.put(9, sample(1), now)
	c.put(8, sample(2), now)
	c.dropUser(1)
	if _, ok := c.get(7, now); ok {
		t.Fatal("user 1 session 7 survived")
	}
	if _, ok := c.get(9, now); ok {
		t.Fatal("user 1 session 9 survived")
	}
	if _, ok := c.get(8, now); !ok {
		t.Fatal("user 2 session was dropped too")
	}
}

// A permission grant or a branch closing must apply to every live session
// on the next request, not after the TTL.
func TestSessionCacheBumpRetiresEverything(t *testing.T) {
	c := newSessionCache(time.Minute)
	now := time.Now()
	c.put(7, sample(1), now)
	c.put(8, sample(2), now)
	c.bump()
	if _, ok := c.get(7, now); ok {
		t.Fatal("session 7 survived a generation bump")
	}
	if _, ok := c.get(8, now); ok {
		t.Fatal("session 8 survived a generation bump")
	}
	// The cache must stay usable afterwards.
	c.put(7, sample(1, "finance.view"), now)
	if got, ok := c.get(7, now); !ok || !got.HasPermission("finance.view") {
		t.Fatal("cache unusable after bump")
	}
}

// Memory is bounded: a flood of distinct sessions must not grow forever.
func TestSessionCacheBounded(t *testing.T) {
	c := newSessionCache(time.Minute)
	now := time.Now()
	for i := 0; i < maxSessionCacheEntries+10; i++ {
		c.put(int64(i), sample(int64(i)), now)
	}
	c.mu.RLock()
	size := len(c.items)
	c.mu.RUnlock()
	if size > maxSessionCacheEntries {
		t.Fatalf("cache holds %d entries, cap is %d", size, maxSessionCacheEntries)
	}
}

// F4-13: a session resolved from rows read BEFORE an invalidation must not be
// cached as valid afterwards.
func TestSessionCachePutIfRefusesAfterInvalidation(t *testing.T) {
	now := time.Now()
	for name, inval := range map[string]func(*sessionCache){
		"bump": func(c *sessionCache) { c.bump() }, "dropUser": func(c *sessionCache) { c.dropUser(1) },
		"drop": func(c *sessionCache) { c.drop(7) },
	} {
		c := newSessionCache(time.Minute)
		mark := c.mark()
		inval(c) // permission change lands while Verify reads
		if c.putIf(7, sample(1, "sales.view"), now, mark) {
			t.Fatalf("%s: stale session stored", name)
		}
		if _, ok := c.get(7, now); ok {
			t.Fatalf("%s: stale session served", name)
		}
	}
	c := newSessionCache(time.Minute)
	if !c.putIf(7, sample(1), now, c.mark()) {
		t.Fatal("no invalidation: must store")
	}
}
