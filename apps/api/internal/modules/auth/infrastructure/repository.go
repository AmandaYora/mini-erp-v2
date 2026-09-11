package infrastructure

import (
	"context"
	"database/sql"
	"time"
)

// SessionRecord is the user_sessions row. User/branch/role links are
// primitive IDs without cross-module foreign keys.
type SessionRecord struct {
	ID                int64
	UserID            int64
	RefreshHash       string
	PrevRefreshHash   sql.NullString
	ActiveBranchID    sql.NullInt64
	ActiveRoleID      sql.NullInt64
	ExpiresAt         time.Time
	AbsoluteExpiresAt time.Time
	RevokedAt         sql.NullTime
}

// Repository owns the auth module tables.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateSession inserts a fresh session row.
func (r *Repository) CreateSession(ctx context.Context, rec *SessionRecord) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO user_sessions
		 (user_id, refresh_hash, id_active_branch, id_active_role,
		  expires_at, absolute_expires_at, last_activity_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.UserID, rec.RefreshHash, nullInt(rec.ActiveBranchID), nullInt(rec.ActiveRoleID),
		rec.ExpiresAt, rec.AbsoluteExpiresAt, rec.ExpiresAt)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func nullInt(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}

// GetSessionByID loads a session row, or nil when missing.
func (r *Repository) GetSessionByID(ctx context.Context, id int64) (*SessionRecord, error) {
	rec := &SessionRecord{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, refresh_hash, prev_refresh_hash, id_active_branch,
			id_active_role, expires_at, absolute_expires_at, revoked_at
		 FROM user_sessions WHERE id = ?`, id).
		Scan(&rec.ID, &rec.UserID, &rec.RefreshHash, &rec.PrevRefreshHash,
			&rec.ActiveBranchID, &rec.ActiveRoleID, &rec.ExpiresAt,
			&rec.AbsoluteExpiresAt, &rec.RevokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rec, err
}

// GetSessionByRefreshHash loads a session by its current refresh hash.
func (r *Repository) GetSessionByRefreshHash(ctx context.Context, hash string) (*SessionRecord, error) {
	rec := &SessionRecord{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, refresh_hash, prev_refresh_hash, id_active_branch,
			id_active_role, expires_at, absolute_expires_at, revoked_at
		 FROM user_sessions WHERE refresh_hash = ?`, hash).
		Scan(&rec.ID, &rec.UserID, &rec.RefreshHash, &rec.PrevRefreshHash,
			&rec.ActiveBranchID, &rec.ActiveRoleID, &rec.ExpiresAt,
			&rec.AbsoluteExpiresAt, &rec.RevokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rec, err
}

// GetSessionByPrevHash finds the session whose previous hash matches —
// proof that a rotated-out token is being replayed.
func (r *Repository) GetSessionByPrevHash(ctx context.Context, hash string) (*SessionRecord, error) {
	rec := &SessionRecord{}
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, refresh_hash, prev_refresh_hash, id_active_branch,
			id_active_role, expires_at, absolute_expires_at, revoked_at
		 FROM user_sessions WHERE prev_refresh_hash = ?`, hash).
		Scan(&rec.ID, &rec.UserID, &rec.RefreshHash, &rec.PrevRefreshHash,
			&rec.ActiveBranchID, &rec.ActiveRoleID, &rec.ExpiresAt,
			&rec.AbsoluteExpiresAt, &rec.RevokedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rec, err
}

// Rotate swaps in a new refresh hash, keeping the old one as replay evidence.
func (r *Repository) Rotate(ctx context.Context, id int64, newHash string, newExpiry time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE user_sessions SET prev_refresh_hash = refresh_hash, refresh_hash = ?,
		 expires_at = ?, last_activity_at = UTC_TIMESTAMP(6) WHERE id = ?`,
		newHash, newExpiry, id)
	return err
}

// RevokeSession marks one session revoked.
func (r *Repository) RevokeSession(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE user_sessions SET revoked_at = UTC_TIMESTAMP(6) WHERE id = ?", id)
	return err
}

// RevokeUserSessions marks all of a user's sessions revoked (used on replay).
func (r *Repository) RevokeUserSessions(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE user_sessions SET revoked_at = UTC_TIMESTAMP(6) WHERE user_id = ? AND revoked_at IS NULL",
		userID)
	return err
}

// UpdateBranchRole changes the session's active branch and role.
func (r *Repository) UpdateBranchRole(ctx context.Context, id, branchID, roleID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE user_sessions SET id_active_branch = ?, id_active_role = ?,
		 last_activity_at = UTC_TIMESTAMP(6) WHERE id = ?`,
		nullBranch(branchID), roleID, id)
	return err
}

func nullBranch(branchID int64) any {
	if branchID == 0 {
		return nil
	}
	return branchID
}
