package contracts

import (
	"context"

	"golang.org/x/crypto/bcrypt"
)

// legacySeedPasswords are the default passwords the legacy seed created
// (design V13). Kept unexported: the only question callers may ask is whether
// a hash matches one of them, never what they are.
var legacySeedPasswords = []string{"superadmin123", "owner123", "admin123", "staff123", "kasir 123"}

// MatchesLegacySeedPassword reports whether a bcrypt hash still verifies one
// of the legacy seed passwords. bcrypt makes this deliberately slow (one
// comparison per seed password): never call it per request. The ETL locks
// such accounts; the data-health check A1 flags the ones kept active.
func MatchesLegacySeedPassword(hash string) bool {
	for _, p := range legacySeedPasswords {
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(p)) == nil {
			return true
		}
	}
	return false
}

// SeedPasswordAccount is an active privileged account whose password is still
// a legacy seed password. Only the username leaves the user module.
type SeedPasswordAccount struct {
	UserID   int64
	Username string
}

// HealthClient is the user module's read-only data-health surface (ADR-0025).
type HealthClient interface {
	// SeedPasswordAccounts lists the active superadmin holders whose password
	// is still a legacy seed password (A1). The bcrypt comparisons run in the
	// background and are cached per stored hash: ready=false means the answer
	// is still being computed (the caller shows the check as pending).
	SeedPasswordAccounts(ctx context.Context) (accounts []SeedPasswordAccount, ready bool, err error)
}
