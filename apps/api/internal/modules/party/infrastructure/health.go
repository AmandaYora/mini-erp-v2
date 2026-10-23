package infrastructure

import (
	"context"

	"mini-erp/internal/modules/party/infrastructure/sqlc"
)

// DuplicateNameGroups lists the names (lower-cased, trimmed) shared by several
// active parties of one type, largest group first (data-health A10, ADR-0025).
func (r *Repository) DuplicateNameGroups(ctx context.Context, partyType string) ([]sqlc.ListDuplicatePartyNameGroupsRow, error) {
	return r.q.ListDuplicatePartyNameGroups(ctx, partyType)
}
