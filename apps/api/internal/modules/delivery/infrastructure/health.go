package infrastructure

import (
	"context"

	"mini-erp/internal/modules/delivery/infrastructure/sqlc"
)

// UnsignedConfirmed lists the branch's confirmed regular order notes that carry
// no recipient-signature status, oldest confirmation first (data-health A11).
func (r *Repository) UnsignedConfirmed(ctx context.Context, branchID int64) ([]sqlc.ListUnsignedConfirmedRow, error) {
	return r.q.ListUnsignedConfirmed(ctx, branchID)
}
