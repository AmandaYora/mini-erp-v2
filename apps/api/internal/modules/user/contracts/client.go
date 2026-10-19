package contracts

import (
	"context"
	"time"
)

// User is the account record other modules may rely on. Cross-module
// references to users are always this ID, resolved via UserClient.
type User struct {
	ID       int64
	Username string
	Email    *string
	FullName string
	Status   string // active | inactive
	// Phone and DisplayTitle are optional profile fields (legacy phone_e164 /
	// display_title); empty when unset.
	Phone        string
	DisplayTitle string
	// LastLoginAt is the last successful login (users.last_login_at). It is
	// only populated by the admin list (U-01); other reads leave it nil.
	LastLoginAt *time.Time
}

// Role groups permissions. Roles are global (single is_system flag,
// ADR-0009) — never per-company.
type Role struct {
	ID          int64
	Code        string
	Name        string
	Description string
	IsSystem    bool
	// IsReserved roles (superadmin) are invisible to non-reserved sessions and
	// can never be granted through the API (F3b-2).
	IsReserved bool
}

// PermissionSeed is a catalog code+name pair for seeding. Other modules
// describe their own codes with this shape (contracts-to-contracts only),
// so the seed aggregates every module's catalog without touching internals.
type PermissionSeed struct {
	Code string
	Name string
}

// BranchAccess is one row of user_branch_access.
type BranchAccess struct {
	BranchID  int64
	IsDefault bool
}

// UserClient is the public surface of the user module.
type UserClient interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByUsername(ctx context.Context, username string) (*User, error)
	// GetByLogin resolves a user by username OR email (F3b-1). Username wins
	// when it is one user's username and another's email. Empty never matches.
	GetByLogin(ctx context.Context, identifier string) (*User, error)
	// VerifyCredentials returns the user when the identifier (username or
	// email) exists, the password matches, and the account is active —
	// unauthorized otherwise. Empty emails are never matched (KI-22).
	VerifyCredentials(ctx context.Context, identifier, password string) (*User, error)
	// TouchLogin stamps users.last_login_at (best effort).
	TouchLogin(ctx context.Context, userID int64) error
	GetRoleIDs(ctx context.Context, userID int64) ([]int64, error)
	GetBranchAccess(ctx context.Context, userID int64) ([]BranchAccess, error)
	// NamesByIDs resolves display names (full name, falling back to the
	// username) for many users in one grouped read. Unknown ids are absent —
	// best-effort, never an error for a missing user.
	NamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)
}

// RoleClient resolves roles and permissions.
type RoleClient interface {
	GetRole(ctx context.Context, id int64) (*Role, error)
	List(ctx context.Context) ([]*Role, error)
	// GetPermissionCodes returns the distinct permission codes held by any
	// of the given roles.
	GetPermissionCodes(ctx context.Context, roleIDs []int64) ([]string, error)
}
