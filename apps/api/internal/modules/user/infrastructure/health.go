package infrastructure

import (
	"context"

	"mini-erp/internal/modules/user/infrastructure/sqlc"
)

// RoleHolderHashes lists the active holders of a role with their password
// hash (data-health A1). The rows never leave the user module.
func (r *Repository) RoleHolderHashes(ctx context.Context, roleCode string) ([]sqlc.ListActiveRoleHolderHashesRow, error) {
	return r.q.ListActiveRoleHolderHashes(ctx, roleCode)
}
