package application

import (
	"context"

	"mini-erp/internal/modules/finance/infrastructure"
)

// WithBranchScope makes the reports read several branches at once (the caller
// has already restricted ids to the user's branch access).
func WithBranchScope(ctx context.Context, ids []int64) context.Context {
	return infrastructure.WithBranchSet(ctx, ids)
}
