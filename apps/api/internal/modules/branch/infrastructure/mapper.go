package infrastructure

import (
	"database/sql"

	"mini-erp/internal/modules/branch/contracts"
	"mini-erp/internal/modules/branch/infrastructure/sqlc"
)

// toBranch maps a generated row to the public contracts type. The generated
// single-row and IN-list projections share one field set, so GetBranchesByIDs
// rows are converted to GetBranchRow first.
func toBranch(r sqlc.GetBranchRow) *contracts.Branch {
	return &contracts.Branch{
		ID:                   r.ID,
		Code:                 r.Code,
		Name:                 r.Name,
		Address:              r.Address.String,
		City:                 r.City.String,
		Phone:                r.Phone.String,
		Status:               r.Status,
		IsHead:               r.IsHead,
		DefaultLocationLabel: r.DefaultLocationLabel,
	}
}

// actor stores the acting user id as-is (0 = system/seed, kept verbatim).
func actor(id int64) sql.NullInt64 {
	return sql.NullInt64{Int64: id, Valid: true}
}
