package contracts

import "context"

// User is the account record other modules may rely on. Cross-module
// references to users are always this ID, resolved via UserClient.
type User struct {
	ID       int64
	Username string
	Email    *string
	FullName string
	Status   string // active | inactive
}

// Role groups permissions. Roles are global (single is_system flag,
// ADR-0009) — never per-company.
type Role struct {
	ID          int64
	Code        string
	Name        string
	Description string
	IsSystem    bool
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
	// VerifyCredentials returns the user when the username exists, the
	// password matches, and the account is active — unauthorized otherwise.
	// Empty emails are never matched: they are stored NULL (KI-22).
	VerifyCredentials(ctx context.Context, username, password string) (*User, error)
	GetRoleIDs(ctx context.Context, userID int64) ([]int64, error)
	GetBranchAccess(ctx context.Context, userID int64) ([]BranchAccess, error)
}

// RoleClient resolves roles and permissions.
type RoleClient interface {
	GetRole(ctx context.Context, id int64) (*Role, error)
	List(ctx context.Context) ([]*Role, error)
	// GetPermissionCodes returns the distinct permission codes held by any
	// of the given roles.
	GetPermissionCodes(ctx context.Context, roleIDs []int64) ([]string, error)
}
