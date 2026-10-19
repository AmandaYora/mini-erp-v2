package application

import (
	"sync"
	"time"

	"mini-erp/internal/modules/auth/contracts"
)

// sessionCacheTTL bounds how long a resolved session may be reused without
// touching the database. It is the backstop, not the invalidation mechanism:
// every write that changes what a session may do invalidates explicitly
// (see AuthzInvalidator), so this only covers paths nobody hooked up.
const sessionCacheTTL = 30 * time.Second

// maxSessionCacheEntries caps memory. Past the cap the map is dropped whole
// rather than evicted one by one — the same bounded-reset strategy the login
// rate limiter uses, and a rebuild costs six reads per live session.
const maxSessionCacheEntries = 10000

// sessionCache memoizes resolved sessions across requests.
//
// Verify used to cost six sequential reads on EVERY authenticated request:
// session row, user, role ids, permission codes, role, branch. Those six
// rows change orders of magnitude more rarely than requests arrive, so the
// resolved contracts.Session is cached per session id and rebuilt only when
// something that feeds it changes.
//
// Correctness rests on two invalidation paths, both deliberate:
//
//   - explicit — auth's own writes (logout, branch/role switch, revoke) drop
//     the affected entries; writes in other modules that change authorization
//     (role permissions, user status, user roles, branch closure) call
//     contracts.AuthzInvalidator, which bumps a generation counter that
//     retires every entry at once.
//   - TTL — sessionCacheTTL expires entries regardless.
//
// The explicit path is what keeps "a revoked permission applies without
// forcing logout" true, exactly as before. The TTL only bounds staleness for
// a write nobody thought to hook up.
//
// Entries are never handed out by pointer: Verify returns a copy so a handler
// mutating its own session can never corrupt another request's view.
type sessionCache struct {
	mu  sync.RWMutex
	ttl time.Duration
	gen uint64
	// epoch counts EVERY invalidation (drop, dropUser, bump). A resolver
	// takes mark() before it reads the database and putIf refuses to store
	// when an invalidation happened in between (F4-13): otherwise a session
	// built from rows read before a permission change would be cached as
	// valid for the whole TTL.
	epoch uint64
	items map[int64]sessionEntry
}

type sessionEntry struct {
	sess    contracts.Session
	expires time.Time
	gen     uint64
}

func newSessionCache(ttl time.Duration) *sessionCache {
	return &sessionCache{ttl: ttl, items: map[int64]sessionEntry{}}
}

// get returns a copy of the cached session for id, or false when absent,
// expired, or retired by a generation bump.
func (c *sessionCache) get(sessionID int64, now time.Time) (contracts.Session, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.items[sessionID]
	if !ok || e.gen != c.gen || now.After(e.expires) {
		return contracts.Session{}, false
	}
	return e.sess, true
}

// mark returns the invalidation epoch; take it BEFORE reading the database.
func (c *sessionCache) mark() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.epoch
}

// putIf is put guarded by a mark taken before the rows were read; it reports
// whether the session was stored.
func (c *sessionCache) putIf(sessionID int64, sess contracts.Session, now time.Time, mark uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.epoch != mark {
		return false
	}
	if len(c.items) >= maxSessionCacheEntries {
		c.items = map[int64]sessionEntry{}
	}
	c.items[sessionID] = sessionEntry{sess: sess, expires: now.Add(c.ttl), gen: c.gen}
	return true
}

// put stores a resolved session. The caller must not mutate sess afterwards;
// Verify builds a fresh value for every miss, so it never does.
func (c *sessionCache) put(sessionID int64, sess contracts.Session, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) >= maxSessionCacheEntries {
		c.items = map[int64]sessionEntry{}
	}
	c.items[sessionID] = sessionEntry{sess: sess, expires: now.Add(c.ttl), gen: c.gen}
}

// drop retires one session (logout, branch/role switch).
func (c *sessionCache) drop(sessionID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	delete(c.items, sessionID)
}

// dropUser retires every cached session belonging to one user (status change,
// role reassignment, replay revoke). The scan is over live sessions only.
func (c *sessionCache) dropUser(userID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	for id, e := range c.items {
		if e.sess.UserID == userID {
			delete(c.items, id)
		}
	}
}

// bump retires every entry at once, for writes whose blast radius is every
// session (role permissions, branch status).
func (c *sessionCache) bump() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.epoch++
	c.gen++
}
