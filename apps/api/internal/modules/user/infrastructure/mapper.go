package infrastructure

import (
	"database/sql"
	"strings"
	"time"

	"mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/modules/user/infrastructure/sqlc"
	"mini-erp/internal/shared/timeutil"
)

// now is the single clock the repository stamps updated_at/last_login_at
// with; the queries take it as a parameter (ADR-0012: no UTC_TIMESTAMP in
// sqlc).
func now() time.Time { return timeutil.NowUTC() }

// toUser maps the account projection shared by GetUser, GetUserByUsername,
// GetUserByLogin and the raw list path (identical field sets, so the
// generated row types convert to sqlc.GetUserRow).
func toUser(r sqlc.GetUserRow) *contracts.User {
	u := &contracts.User{
		ID:           r.ID,
		Username:     r.Username,
		FullName:     r.FullName,
		Status:       r.Status,
		Phone:        r.Phone.String,
		DisplayTitle: r.DisplayTitle.String,
	}
	if r.Email.Valid {
		email := r.Email.String
		u.Email = &email
	}
	return u
}

func toRole(r sqlc.GetRoleRow) *contracts.Role {
	return &contracts.Role{
		ID:          r.ID,
		Code:        r.Code,
		Name:        r.Name,
		Description: r.Description.String,
		IsSystem:    r.IsSystem,
		IsReserved:  r.IsReserved,
	}
}

// actor stores the acting user id as-is (0 = system/seed, kept verbatim).
func actor(id int64) sql.NullInt64 { return sql.NullInt64{Int64: id, Valid: true} }

// nullStr maps the empty string to SQL NULL.
func nullStr(v string) sql.NullString {
	if v == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: v, Valid: true}
}

// nullEmail maps a nil or empty email pointer to SQL NULL (KI-22: any number
// of users may have no email because NULL is never equal to NULL).
func nullEmail(email *string) sql.NullString {
	if email == nil || *email == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *email, Valid: true}
}

// displayName resolves the list name (full_name, else username).
func displayName(full, username string) string {
	if strings.TrimSpace(full) == "" {
		return username
	}
	return full
}
