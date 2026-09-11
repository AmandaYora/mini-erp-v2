package infrastructure

import (
	"context"
	"database/sql"

	"mini-erp/internal/modules/assistant/contracts"
)

// Repository owns the assistant_* tables (channel, config, authorizations,
// threads, messages, runs, tool executions). No company_id (ADR-0009):
// single-tenant, one WhatsApp session for the whole installation.
type Repository struct {
	db *sql.DB
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// --- channel (singleton id=1) -----------------------------------------------

// ChannelState is the persisted gateway state row.
type ChannelState struct {
	State     string
	Phone     string
	LastError string
	UpdatedAt string
}

// ts renders DATETIME(6) without microseconds for JSON readers.
func ts(col string) string {
	return "DATE_FORMAT(" + col + ", '%Y-%m-%d %H:%i:%s')"
}

// GetChannel returns the singleton row, creating the default when missing.
func (r *Repository) GetChannel(ctx context.Context) (*ChannelState, error) {
	st := &ChannelState{}
	var phone, lastErr sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT state, phone, last_error, `+ts("updated_at")+` FROM assistant_channel WHERE id = 1`).
		Scan(&st.State, &phone, &lastErr, &st.UpdatedAt)
	if err == sql.ErrNoRows {
		if _, err := r.db.ExecContext(ctx,
			`INSERT INTO assistant_channel (id, state) VALUES (1, 'disconnected')`); err != nil {
			return nil, err
		}
		st.State = "disconnected"
		return st, nil
	}
	if err != nil {
		return nil, err
	}
	st.Phone, st.LastError = phone.String, lastErr.String
	return st, nil
}

// SetChannel upserts the singleton state row.
func (r *Repository) SetChannel(ctx context.Context, state, phone, lastErr string) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO assistant_channel (id, state, phone, last_error) VALUES (1, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE state = VALUES(state), phone = VALUES(phone), last_error = VALUES(last_error)`,
		state, nullIfEmpty(phone), nullIfEmpty(lastErr))
	return err
}

// --- config (singleton id=1) --------------------------------------------------

// BotConfig is the persisted tuning row.
type BotConfig struct {
	Mode            string
	RateLimitPerMin int
	UpdatedAt       string
}

// GetConfig returns the singleton row, creating the default when missing.
func (r *Repository) GetConfig(ctx context.Context) (*BotConfig, error) {
	cfg := &BotConfig{}
	err := r.db.QueryRowContext(ctx,
		`SELECT mode, rate_limit_per_minute, `+ts("updated_at")+` FROM assistant_config WHERE id = 1`).
		Scan(&cfg.Mode, &cfg.RateLimitPerMin, &cfg.UpdatedAt)
	if err == sql.ErrNoRows {
		if _, err := r.db.ExecContext(ctx,
			`INSERT INTO assistant_config (id, mode, rate_limit_per_minute) VALUES (1, 'rule_based', 10)`); err != nil {
			return nil, err
		}
		cfg.Mode, cfg.RateLimitPerMin = "rule_based", 10
		return cfg, nil
	}
	return cfg, err
}

// UpdateConfig rewrites the singleton tuning row.
func (r *Repository) UpdateConfig(ctx context.Context, mode string, rateLimit int) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO assistant_config (id, mode, rate_limit_per_minute) VALUES (1, ?, ?)
		 ON DUPLICATE KEY UPDATE mode = VALUES(mode), rate_limit_per_minute = VALUES(rate_limit_per_minute)`,
		mode, rateLimit)
	return err
}

// --- authorizations -------------------------------------------------------------

// ListAuthorizations returns active rows (or all with revoked).
func (r *Repository) ListAuthorizations(ctx context.Context, includeRevoked bool) ([]*contracts.Authorization, error) {
	where := "WHERE status = 'active'"
	if includeRevoked {
		where = ""
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, phone, name, access_level, status, is_primary_owner, `+ts("last_seen_at")+`
		 FROM assistant_authorizations `+where+` ORDER BY is_primary_owner DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*contracts.Authorization{}
	for rows.Next() {
		var a contracts.Authorization
		var primary int
		var seen sql.NullString
		if err := rows.Scan(&a.ID, &a.Phone, &a.Name, &a.AccessLevel, &a.Status, &primary, &seen); err != nil {
			return nil, err
		}
		a.IsPrimaryOwner = primary == 1
		a.LastSeenAt = seen.String
		out = append(out, &a)
	}
	return out, rows.Err()
}

// FindActiveByPhone resolves one active authorization by exact digits.
func (r *Repository) FindActiveByPhone(ctx context.Context, phone string) (*contracts.Authorization, error) {
	var a contracts.Authorization
	var primary int
	var seen sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, phone, name, access_level, status, is_primary_owner, `+ts("last_seen_at")+`
		 FROM assistant_authorizations WHERE phone = ? AND status = 'active'`, phone).
		Scan(&a.ID, &a.Phone, &a.Name, &a.AccessLevel, &a.Status, &primary, &seen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.IsPrimaryOwner = primary == 1
	a.LastSeenAt = seen.String
	return &a, nil
}

// PhoneTaken reports whether any row (any status) holds the digits.
func (r *Repository) PhoneTaken(ctx context.Context, phone string, excludeID int64) (bool, string, error) {
	var status string
	err := r.db.QueryRowContext(ctx,
		`SELECT status FROM assistant_authorizations WHERE phone = ? AND id <> ?`, phone, excludeID).
		Scan(&status)
	if err == sql.ErrNoRows {
		return false, "", nil
	}
	return err == nil, status, err
}

// CreateAuthorization inserts one whitelist row.
func (r *Repository) CreateAuthorization(ctx context.Context, a *contracts.Authorization, actorID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO assistant_authorizations
		 (phone, name, access_level, status, is_primary_owner, created_by, updated_by)
		 VALUES (?, ?, ?, 'active', ?, ?, ?)`,
		a.Phone, a.Name, a.AccessLevel, boolToInt(a.IsPrimaryOwner), nullIfZero(actorID), nullIfZero(actorID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAuthorization rewrites an active row (revoked rows read as missing:
// re-adding a revoked number goes through create-which-reactivates).
func (r *Repository) UpdateAuthorization(ctx context.Context, a *contracts.Authorization, actorID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE assistant_authorizations
		 SET phone = ?, name = ?, access_level = ?, is_primary_owner = ?, updated_by = ?
		 WHERE id = ? AND status = 'active'`,
		a.Phone, a.Name, a.AccessLevel, boolToInt(a.IsPrimaryOwner), nullIfZero(actorID), a.ID)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Reactivate flips a revoked row back to active with fresh attributes.
func (r *Repository) Reactivate(ctx context.Context, id int64, a *contracts.Authorization, actorID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assistant_authorizations
		 SET name = ?, access_level = ?, is_primary_owner = ?, status = 'active',
		     last_seen_at = NULL, updated_by = ?
		 WHERE id = ?`,
		a.Name, a.AccessLevel, boolToInt(a.IsPrimaryOwner), nullIfZero(actorID), id)
	return err
}

// Revoke flips a row to revoked and drops its primary flag.
func (r *Repository) Revoke(ctx context.Context, id int64, actorID int64) (bool, error) {
	res, err := r.db.ExecContext(ctx,
		`UPDATE assistant_authorizations
		 SET status = 'revoked', is_primary_owner = 0, updated_by = ?
		 WHERE id = ? AND status = 'active'`,
		nullIfZero(actorID), id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// DemoteOthers clears every other primary flag (single primary invariant).
func (r *Repository) DemoteOthers(ctx context.Context, keepID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assistant_authorizations SET is_primary_owner = 0 WHERE id <> ?`, keepID)
	return err
}

// TouchSeen stamps last_seen_at for an active sender.
func (r *Repository) TouchSeen(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assistant_authorizations SET last_seen_at = CURRENT_TIMESTAMP(6) WHERE id = ?`, id)
	return err
}

// --- threads & messages ---------------------------------------------------------

// FindOpenThread resolves the sender's open thread, or nil.
func (r *Repository) FindOpenThread(ctx context.Context, phone string) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM assistant_threads
		 WHERE phone = ? AND status = 'open' ORDER BY id DESC LIMIT 1`, phone).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}

// CreateThread opens one thread for the sender.
func (r *Repository) CreateThread(ctx context.Context, phone string, branchID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO assistant_threads (phone, branch_id) VALUES (?, ?)`,
		phone, nullIfZero(branchID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// TouchThread bumps the thread clock.
func (r *Repository) TouchThread(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assistant_threads SET last_message_at = CURRENT_TIMESTAMP(6) WHERE id = ?`, id)
	return err
}

// InsertMessage stores one inbound/outbound row.
func (r *Repository) InsertMessage(ctx context.Context, threadID int64, direction, body, status string, runID int64) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO assistant_messages (thread_id, direction, body, status, run_id)
		 VALUES (?, ?, ?, ?, ?)`,
		threadID, direction, body, status, nullIfZero(runID))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetMessageStatus flips a message row (processed with its run, or failed).
func (r *Repository) SetMessageStatus(ctx context.Context, id int64, status string, runID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assistant_messages SET status = ?, run_id = ? WHERE id = ?`,
		status, nullIfZero(runID), id)
	return err
}

// --- runs & tool executions -------------------------------------------------------

// CreateRun opens one running row.
func (r *Repository) CreateRun(ctx context.Context, threadID, branchID, actorID int64, phone, intent, mode string) (int64, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO assistant_runs (thread_id, phone, branch_id, actor_id, intent, mode, status)
		 VALUES (?, ?, ?, ?, ?, ?, 'running')`,
		nullIfZero(threadID), nullIfEmpty(phone), nullIfZero(branchID), actorID, intent, mode)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishRun closes a run with its outcome.
func (r *Repository) FinishRun(ctx context.Context, id int64, status, answer string, durationMs int64, failure string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE assistant_runs SET status = ?, answer = ?, duration_ms = ?, failure_reason = ? WHERE id = ?`,
		status, nullIfEmpty(answer), durationMs, nullIfEmpty(failure), id)
	return err
}

// InsertToolExecution records one tool call of a run.
func (r *Repository) InsertToolExecution(ctx context.Context, runID int64, seq int, tool, input, output string, durationMs int64) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO assistant_tool_executions (run_id, seq, tool, input_json, output_json, duration_ms)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		runID, seq, tool, input, nullIfEmpty(output), durationMs)
	return err
}

// RunStats aggregates the last 7 calendar days per day×intent×mode.
func (r *Repository) RunStats(ctx context.Context) ([]*contracts.RunStat, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DATE(created_at) AS day, intent, mode, COUNT(*),
		        SUM(status = 'completed'), COALESCE(AVG(duration_ms), 0)
		 FROM assistant_runs
		 WHERE created_at >= DATE_SUB(CURDATE(), INTERVAL 7 DAY)
		 GROUP BY day, intent, mode
		 ORDER BY day DESC, COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []*contracts.RunStat{}
	for rows.Next() {
		var s contracts.RunStat
		var day string
		var avg float64
		if err := rows.Scan(&day, &s.Intent, &s.Mode, &s.Total, &s.Success, &avg); err != nil {
			return nil, err
		}
		s.Day = day
		s.AvgDurationMs = int64(avg + 0.5)
		out = append(out, &s)
	}
	return out, rows.Err()
}

func nullIfZero(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
