package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"mini-erp/internal/modules/auth/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

// SessionRecord is the user_sessions row. User/branch/role links are
// primitive IDs without cross-module foreign keys.
type SessionRecord struct {
	ID                int64
	UserID            int64
	RefreshHash       string
	PrevRefreshHash   sql.NullString
	RotatedAt         sql.NullTime
	ActiveBranchID    sql.NullInt64
	ActiveRoleID      sql.NullInt64
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         sql.NullTime
	// UserAgent / IPAddress are the login trail (F3b-1); write-only here.
	UserAgent string
	IPAddress string
}

// Repository owns the auth module tables.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

func nowArg() sql.NullTime { return sql.NullTime{Time: timeutil.NowUTC(), Valid: true} }

// CreateSession inserts a fresh session row.
func (r *Repository) CreateSession(ctx context.Context, rec *SessionRecord) (int64, error) {
	res, err := r.q.InsertUserSession(ctx, sqlc.InsertUserSessionParams{
		UserID: rec.UserID, RefreshHash: rec.RefreshHash,
		IDActiveBranch: rec.ActiveBranchID, IDActiveRole: rec.ActiveRoleID,
		UserAgent: dbutil.NullString(rec.UserAgent), IpAddress: dbutil.NullString(rec.IPAddress),
		ExpiresAt: rec.ExpiresAt, AbsoluteExpiresAt: rec.AbsoluteExpiresAt, LastActivityAt: rec.ExpiresAt,
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetSessionByID loads a session row, or nil when missing.
func (r *Repository) GetSessionByID(ctx context.Context, id int64) (*SessionRecord, error) {
	row, err := r.q.GetUserSessionByID(ctx, id)
	return sessionOrNil(sqlc.GetUserSessionByRefreshHashRow(row), err)
}

// GetSessionByRefreshHash loads a session by its current refresh hash.
func (r *Repository) GetSessionByRefreshHash(ctx context.Context, hash string) (*SessionRecord, error) {
	row, err := r.q.GetUserSessionByRefreshHash(ctx, hash)
	return sessionOrNil(row, err)
}

// GetSessionByPrevHash finds the session whose previous hash matches —
// proof that a rotated-out token is being replayed.
func (r *Repository) GetSessionByPrevHash(ctx context.Context, hash string) (*SessionRecord, error) {
	row, err := r.q.GetUserSessionByPrevHash(ctx, sql.NullString{String: hash, Valid: true})
	return sessionOrNil(sqlc.GetUserSessionByRefreshHashRow(row), err)
}

// Rotate swaps in a new refresh hash with CAS (F1-10): only the holder of
// the current hash wins; concurrent tabs lose and fall into the grace path.
// Returns rows affected.
func (r *Repository) Rotate(ctx context.Context, id int64, oldHash, newHash string, newExpiry time.Time) (int64, error) {
	now := timeutil.NowUTC()
	res, err := r.q.RotateUserSession(ctx, sqlc.RotateUserSessionParams{
		NewHash: newHash, RotatedAt: sql.NullTime{Time: now, Valid: true}, NewExpiry: newExpiry,
		Now: now, ID: id, OldHash: oldHash,
	})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// RevokeSession marks one session revoked.
func (r *Repository) RevokeSession(ctx context.Context, id int64) error {
	return r.q.RevokeUserSession(ctx, sqlc.RevokeUserSessionParams{Now: nowArg(), ID: id})
}

// RevokeUserSessions marks all of a user's sessions revoked (used on replay).
func (r *Repository) RevokeUserSessions(ctx context.Context, userID int64) error {
	return r.q.RevokeAllUserSessions(ctx, sqlc.RevokeAllUserSessionsParams{Now: nowArg(), UserID: userID})
}

// RevokeUserSessionsExcept revokes all sessions except one (F1-4). exceptID 0 revokes all.
// Returns rows affected.
func (r *Repository) RevokeUserSessionsExcept(ctx context.Context, userID, exceptID int64) (int64, error) {
	res, err := r.q.RevokeUserSessionsExcept(ctx, sqlc.RevokeUserSessionsExceptParams{Now: nowArg(), UserID: userID, ExceptID: exceptID})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateBranchRole changes the session's active branch and role.
func (r *Repository) UpdateBranchRole(ctx context.Context, id, branchID, roleID int64) error {
	return r.q.UpdateUserSessionBranchRole(ctx, sqlc.UpdateUserSessionBranchRoleParams{
		BranchID: dbutil.NullInt64(branchID), RoleID: sql.NullInt64{Int64: roleID, Valid: true},
		Now: timeutil.NowUTC(), ID: id,
	})
}

func sessionOrNil(row sqlc.GetUserSessionByRefreshHashRow, err error) (*SessionRecord, error) {
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toSession(row), nil
}
