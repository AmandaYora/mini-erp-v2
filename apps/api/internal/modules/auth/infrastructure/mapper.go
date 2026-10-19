package infrastructure

import "mini-erp/internal/modules/auth/infrastructure/sqlc"

// toSession maps the generated session projection (all three Get* queries
// share one column list, so they convert to one row type).
func toSession(r sqlc.GetUserSessionByRefreshHashRow) *SessionRecord {
	return &SessionRecord{
		ID: r.ID, UserID: r.UserID, RefreshHash: r.RefreshHash, PrevRefreshHash: r.PrevRefreshHash,
		RotatedAt: r.RotatedAt, ActiveBranchID: r.IDActiveBranch, ActiveRoleID: r.IDActiveRole,
		ExpiresAt: r.ExpiresAt, AbsoluteExpiresAt: r.AbsoluteExpiresAt, RevokedAt: r.RevokedAt,
	}
}
