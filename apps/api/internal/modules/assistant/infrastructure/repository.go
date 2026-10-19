package infrastructure

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"mini-erp/internal/modules/assistant/contracts"
	"mini-erp/internal/modules/assistant/infrastructure/sqlc"
	"mini-erp/internal/shared/dbutil"
	"mini-erp/internal/shared/timeutil"
)

// Repository owns the assistant_* tables (channel, config, authorizations,
// threads, messages, runs, tool executions). No company_id (ADR-0009):
// single-tenant, one WhatsApp session for the whole installation.
type Repository struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewRepository wires the module's storage.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db, q: sqlc.New(db)}
}

// --- channel (singleton id=1) -----------------------------------------------

// ChannelState is the persisted gateway state row.
type ChannelState struct {
	State     string
	Phone     string
	LastError string
	UpdatedAt string
}

// GetChannel returns the singleton row, creating the default when missing.
func (r *Repository) GetChannel(ctx context.Context) (*ChannelState, error) {
	row, err := r.q.GetChannel(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		if err := r.q.InsertDefaultChannel(ctx); err != nil {
			return nil, err
		}
		return &ChannelState{State: "disconnected"}, nil
	}
	if err != nil {
		return nil, err
	}
	return toChannelState(row), nil
}

// SetChannel upserts the singleton state row.
func (r *Repository) SetChannel(ctx context.Context, state, phone, lastErr string) error {
	return r.q.UpsertChannel(ctx, sqlc.UpsertChannelParams{
		State: state, Phone: dbutil.NullString(phone), LastError: dbutil.NullString(lastErr),
	})
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
	row, err := r.q.GetConfig(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		if err := r.q.InsertDefaultConfig(ctx); err != nil {
			return nil, err
		}
		return &BotConfig{Mode: "rule_based", RateLimitPerMin: 10}, nil
	}
	if err != nil {
		return &BotConfig{}, err
	}
	return toBotConfig(row), nil
}

// UpdateConfig rewrites the singleton tuning row.
func (r *Repository) UpdateConfig(ctx context.Context, mode string, rateLimit int) error {
	return r.q.UpsertConfig(ctx, sqlc.UpsertConfigParams{Mode: mode, RateLimitPerMinute: uint32(rateLimit)})
}

// --- authorizations -------------------------------------------------------------

// ListAuthorizations returns active rows (or all with revoked).
func (r *Repository) ListAuthorizations(ctx context.Context, includeRevoked bool) ([]*contracts.Authorization, error) {
	out := []*contracts.Authorization{}
	if includeRevoked {
		rows, err := r.q.ListAllAuthorizations(ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			out = append(out, toAuthorization(row.ID, row.Phone, row.Name, row.AccessLevel, row.Status, row.IsPrimaryOwner, row.LastSeenAt))
		}
		return out, nil
	}
	rows, err := r.q.ListActiveAuthorizations(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, toAuthorization(row.ID, row.Phone, row.Name, row.AccessLevel, row.Status, row.IsPrimaryOwner, row.LastSeenAt))
	}
	return out, nil
}

// FindActiveByPhone resolves one active authorization by exact digits.
func (r *Repository) FindActiveByPhone(ctx context.Context, phone string) (*contracts.Authorization, error) {
	row, err := r.q.FindActiveAuthorizationByPhone(ctx, phone)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return toAuthorization(row.ID, row.Phone, row.Name, row.AccessLevel, row.Status, row.IsPrimaryOwner, row.LastSeenAt), nil
}

// PhoneTaken reports whether any row (any status) holds the digits.
func (r *Repository) PhoneTaken(ctx context.Context, phone string, excludeID int64) (bool, string, error) {
	status, err := r.q.GetAuthorizationStatusByPhone(ctx, sqlc.GetAuthorizationStatusByPhoneParams{Phone: phone, ID: excludeID})
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	return err == nil, status, err
}

// CreateAuthorization inserts one whitelist row.
func (r *Repository) CreateAuthorization(ctx context.Context, a *contracts.Authorization, actorID int64) (int64, error) {
	res, err := r.q.CreateAuthorization(ctx, sqlc.CreateAuthorizationParams{
		Phone: a.Phone, Name: a.Name, AccessLevel: a.AccessLevel, IsPrimaryOwner: a.IsPrimaryOwner,
		CreatedBy: dbutil.NullInt64(actorID), UpdatedBy: dbutil.NullInt64(actorID),
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateAuthorization rewrites an active row (revoked rows read as missing:
// re-adding a revoked number goes through create-which-reactivates).
func (r *Repository) UpdateAuthorization(ctx context.Context, a *contracts.Authorization, actorID int64) (bool, error) {
	res, err := r.q.UpdateActiveAuthorization(ctx, sqlc.UpdateActiveAuthorizationParams{
		Phone: a.Phone, Name: a.Name, AccessLevel: a.AccessLevel, IsPrimaryOwner: a.IsPrimaryOwner,
		UpdatedBy: dbutil.NullInt64(actorID), ID: a.ID,
	})
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Reactivate flips a revoked row back to active with fresh attributes.
func (r *Repository) Reactivate(ctx context.Context, id int64, a *contracts.Authorization, actorID int64) error {
	return r.q.ReactivateAuthorization(ctx, sqlc.ReactivateAuthorizationParams{
		Name: a.Name, AccessLevel: a.AccessLevel, IsPrimaryOwner: a.IsPrimaryOwner,
		UpdatedBy: dbutil.NullInt64(actorID), ID: id,
	})
}

// Revoke flips a row to revoked and drops its primary flag.
func (r *Repository) Revoke(ctx context.Context, id int64, actorID int64) (bool, error) {
	res, err := r.q.RevokeAuthorization(ctx, sqlc.RevokeAuthorizationParams{UpdatedBy: dbutil.NullInt64(actorID), ID: id})
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// DemoteOthers clears every other primary flag (single primary invariant).
func (r *Repository) DemoteOthers(ctx context.Context, keepID int64) error {
	return r.q.DemoteOtherAuthorizations(ctx, keepID)
}

// TouchSeen stamps last_seen_at for an active sender.
func (r *Repository) TouchSeen(ctx context.Context, id int64) error {
	return r.q.TouchAuthorizationSeen(ctx, sqlc.TouchAuthorizationSeenParams{
		Now: sql.NullTime{Time: timeutil.NowUTC(), Valid: true}, ID: id,
	})
}

// --- threads & messages ---------------------------------------------------------

// FindOpenThread resolves the sender's open thread, or 0.
func (r *Repository) FindOpenThread(ctx context.Context, phone string) (int64, error) {
	id, err := r.q.FindOpenThread(ctx, phone)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

// CreateThread opens one thread for the sender.
func (r *Repository) CreateThread(ctx context.Context, phone string, branchID int64) (int64, error) {
	res, err := r.q.CreateThread(ctx, sqlc.CreateThreadParams{Phone: phone, BranchID: dbutil.NullInt64(branchID)})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// TouchThread bumps the thread clock.
func (r *Repository) TouchThread(ctx context.Context, id int64) error {
	return r.q.TouchThread(ctx, sqlc.TouchThreadParams{Now: timeutil.NowUTC(), ID: id})
}

// InsertMessage stores one inbound/outbound row.
func (r *Repository) InsertMessage(ctx context.Context, threadID int64, direction, body, status string, runID int64) (int64, error) {
	res, err := r.q.InsertMessage(ctx, sqlc.InsertMessageParams{
		ThreadID: threadID, Direction: direction, Body: body, Status: status, RunID: dbutil.NullInt64(runID),
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SetMessageStatus flips a message row (processed with its run, or failed).
func (r *Repository) SetMessageStatus(ctx context.Context, id int64, status string, runID int64) error {
	return r.q.SetMessageStatus(ctx, sqlc.SetMessageStatusParams{Status: status, RunID: dbutil.NullInt64(runID), ID: id})
}

// --- runs & tool executions -------------------------------------------------------

// CreateRun opens one running row.
func (r *Repository) CreateRun(ctx context.Context, threadID, branchID, actorID int64, phone, intent, mode string) (int64, error) {
	res, err := r.q.CreateRun(ctx, sqlc.CreateRunParams{
		ThreadID: dbutil.NullInt64(threadID), Phone: dbutil.NullString(phone), BranchID: dbutil.NullInt64(branchID),
		ActorID: actorID, Intent: intent, Mode: mode,
	})
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// FinishRun closes a run with its outcome.
func (r *Repository) FinishRun(ctx context.Context, id int64, status, answer string, durationMs int64, failure string) error {
	return r.q.FinishRun(ctx, sqlc.FinishRunParams{
		Status: status, Answer: dbutil.NullString(answer), DurationMs: uint32(durationMs),
		FailureReason: dbutil.NullString(failure), ID: id,
	})
}

// InsertToolExecution records one tool call of a run.
func (r *Repository) InsertToolExecution(ctx context.Context, runID int64, seq int, tool, input, output string, durationMs int64, status, errorText string) error {
	return r.q.InsertToolExecution(ctx, sqlc.InsertToolExecutionParams{
		RunID: runID, Seq: uint32(seq), Tool: tool, InputJson: input,
		OutputJson: dbutil.NullString(output), DurationMs: uint32(durationMs),
		Status: status, ErrorText: dbutil.NullString(errorText),
	})
}

// RunStats aggregates the last 7 calendar days per day×intent×mode.
func (r *Repository) RunStats(ctx context.Context) ([]*contracts.RunStat, error) {
	now := timeutil.NowUTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	rows, err := r.q.RunStats(ctx, today.AddDate(0, 0, -7))
	if err != nil {
		return nil, err
	}
	out := make([]*contracts.RunStat, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRunStat(row))
	}
	return out, nil
}
