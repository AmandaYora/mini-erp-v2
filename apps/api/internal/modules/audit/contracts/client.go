package contracts

import (
	"context"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// Entry is one audit trail record. Writes are best-effort: Log happens after
// the mutation commits and never fails the operation.
type Entry struct {
	Action   string // e.g. "sales.confirm"
	Entity   string // e.g. "sales_order"
	EntityID int64  // 0 when not applicable
	BranchID int64  // 0 for global (non-branch) entities
	ActorID  int64  // 0 for system/seed
	Note     string // short context: document number, code, ...
}

// Record is a stored entry.
type Record struct {
	ID        int64
	Action    string
	Entity    string
	EntityID  int64
	BranchID  int64
	ActorID   int64
	Note      string
	CreatedAt string
}

// AuditClient is the public surface of the audit module.
type AuditClient interface {
	// Log appends one trail record (best-effort).
	Log(ctx context.Context, e Entry) error
	// List returns newest-first records for one branch (plus global,
	// branch-less events) with optional action/entity filters.
	List(ctx context.Context, branchID int64, action, entity string, page, limit int) ([]Record, int64, error)
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "audit.view", Name: "Audit: Lihat"},
	}
}
