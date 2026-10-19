package contracts

import (
	"context"
	"database/sql"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// AuditTx is the caller's open transaction, borrowed so the trail row
// co-commits with the mutation it describes (ADR-0011). *sql.Tx satisfies
// it; the audit module never commits or rolls back the borrowed handle.
type AuditTx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// AuditClient is the public surface of the audit module.
type AuditClient interface {
	// Log appends one trail record (best-effort).
	Log(ctx context.Context, e Entry) error
	// LogTx appends one trail record inside the caller's transaction
	// (ADR-0011): for money/stock operations the trail must never be lost
	// to a crash between commit and a best-effort Log. The caller owns the
	// error — money paths roll back on it instead of swallowing it.
	LogTx(ctx context.Context, tx AuditTx, e Entry) error
	// List returns newest-first records for one branch (plus global,
	// branch-less events) with optional action/entity filters.
	List(ctx context.Context, branchID int64, action, entity string, page, limit int) ([]Record, int64, error)
}

// Entry is one audit trail record. Writes are best-effort: Log happens after
// the mutation commits and never fails the operation.
type Entry struct {
	Action    string // e.g. "sales.confirm"
	Entity    string // e.g. "sales_order"
	EntityID  int64  // 0 when not applicable
	BranchID  int64  // 0 for global (non-branch) entities
	ActorID   int64  // 0 for system/seed
	ActorType string // "user" (default) or "system" (F4-9)
	Note      string // short context: document number, code, ...
	// Before/After/Metadata carry structured diffs, marshaled by the audit
	// module (F4-9/F3b-8). Nil/empty means no detail row. Backward
	// compatible: existing call sites leave them unset.
	Before   any
	After    any
	Metadata any
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
	// ActorType is "user" or "system"; Before/After/Metadata are the raw JSON
	// diff stored in audit_log_details (nil when the entry had none).
	ActorType string
	Before    []byte
	After     []byte
	Metadata  []byte
}

// ListFilter selects audit records for the console view (F3b-8).
// BranchID > 0 shows that branch plus global (branch-less) events;
// AllBranches ignores branch scoping (cross-branch audit view).
type ListFilter struct {
	BranchID    int64
	AllBranches bool
	Action      string // prefix match: "product." finds product.create/update
	Entity      string
	EntityID    int64
	ActorID     int64
	From, To    string // WIB calendar days, YYYY-MM-DD (inclusive)
	Page, Limit int
}

// PermissionCatalog lists this module's permission codes for the seed.
func PermissionCatalog() []usercontracts.PermissionSeed {
	return []usercontracts.PermissionSeed{
		{Code: "audit.view", Name: "Audit: Lihat"},
	}
}
