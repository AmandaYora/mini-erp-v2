package application

import (
	"context"
	"sync"

	"mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/modules/user/domain"
	"mini-erp/internal/shared/apperror"
)

// seedCheck remembers, per user, the hash last compared against the legacy
// seed passwords and the verdict. A changed hash (password change, weak-hash
// upgrade) no longer equals the stored one, so the entry invalidates itself.
type seedCheck struct {
	hash    string
	matched bool
}

// seedPasswordCache runs the slow bcrypt comparisons off the request path:
// a miss starts one background pass; callers meanwhile read "not ready".
// The zero value is ready to use.
type seedPasswordCache struct {
	mu      sync.Mutex
	checked map[int64]seedCheck
	running bool
}

// pendingHash is one account whose current hash has no verdict yet.
type pendingHash struct {
	userID int64
	hash   string
}

// SeedPasswordAccounts is data-health alert A1 (security.seed_password,
// ADR-0025): active superadmin holders still on a legacy seed password. The
// owner decided the check covers the developer superadmin account only.
//
// Query count: 1 read of the role's holders. bcrypt never runs on the request
// path: unknown hashes are compared by one background goroutine and the
// answer is ready=false until it finishes.
func (s *Service) SeedPasswordAccounts(ctx context.Context) ([]contracts.SeedPasswordAccount, bool, error) {
	rows, err := s.repo.RoleHolderHashes(ctx, domain.ReservedSuperadminRole)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	c := &s.seedPasswords
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []contracts.SeedPasswordAccount
	var pending []pendingHash
	for _, r := range rows {
		got, ok := c.checked[r.ID]
		if !ok || got.hash != r.PasswordHash {
			pending = append(pending, pendingHash{userID: r.ID, hash: r.PasswordHash})
			continue
		}
		if got.matched {
			out = append(out, contracts.SeedPasswordAccount{UserID: r.ID, Username: r.Username})
		}
	}
	if len(pending) == 0 {
		return out, true, nil
	}
	if !c.running {
		c.running = true
		go c.compare(pending)
	}
	return nil, false, nil
}

// compare runs the bcrypt comparisons of one pass and stores the verdicts.
func (c *seedPasswordCache) compare(pending []pendingHash) {
	verdicts := make([]seedCheck, len(pending))
	for i, p := range pending {
		verdicts[i] = seedCheck{hash: p.hash, matched: contracts.MatchesLegacySeedPassword(p.hash)}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.checked == nil {
		c.checked = map[int64]seedCheck{}
	}
	for i, p := range pending {
		c.checked[p.userID] = verdicts[i]
	}
	c.running = false
}
