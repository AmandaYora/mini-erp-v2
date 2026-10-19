// Package housekeeping runs the daily retention sweep (F4-10): bounded,
// idempotent deletes of old operational rows, one task per owning module.
// It is technical plumbing only: the SQL lives in each module's own
// repository and arrives here as a plain function.
package housekeeping

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"os"
	"strconv"
	"time"

	"mini-erp/internal/shared/timeutil"
)

// lockName guards against two server processes sweeping at once.
const lockName = "mini_erp_housekeeping"

// Task is one table's retention pass. Run deletes at most batch rows older
// than the task's own cutoff and returns how many it deleted; the runner
// repeats it until a pass deletes fewer than batch (bounded by MaxBatches).
type Task struct {
	Name string
	Run  func(ctx context.Context, batch int) (int64, error)
}

// Retention holds the retention windows in days. Zero or negative values
// fall back to the defaults.
type Retention struct {
	AuditDays       int
	SessionDays     int
	ReservationDays int
	AssistantDays   int
	IdleThreadDays  int
}

// Defaults are the windows from the F4-10 design.
func Defaults() Retention {
	return Retention{AuditDays: 730, SessionDays: 30, ReservationDays: 7, AssistantDays: 90, IdleThreadDays: 180}
}

func (r Retention) withDefaults() Retention {
	d := Defaults()
	if r.AuditDays > 0 {
		d.AuditDays = r.AuditDays
	}
	if r.SessionDays > 0 {
		d.SessionDays = r.SessionDays
	}
	if r.ReservationDays > 0 {
		d.ReservationDays = r.ReservationDays
	}
	if r.AssistantDays > 0 {
		d.AssistantDays = r.AssistantDays
	}
	if r.IdleThreadDays > 0 {
		d.IdleThreadDays = r.IdleThreadDays
	}
	return d
}

// Normalized returns the retention with defaults filled in.
func (r Retention) Normalized() Retention { return r.withDefaults() }

// FromEnv reads RETENTION_*_DAYS overrides; unset/invalid keeps defaults.
func FromEnv() Retention {
	get := func(k string) int {
		n, err := strconv.Atoi(os.Getenv(k))
		if err != nil {
			return 0
		}
		return n
	}
	return Retention{
		AuditDays:       get("RETENTION_AUDIT_DAYS"),
		SessionDays:     get("RETENTION_SESSION_DAYS"),
		ReservationDays: get("RETENTION_RESERVATION_DAYS"),
		AssistantDays:   get("RETENTION_ASSISTANT_DAYS"),
		IdleThreadDays:  get("RETENTION_IDLE_THREAD_DAYS"),
	}.withDefaults()
}

// Setting keys in company_settings (F4-10); they match the company module's
// registered keys (pinned by an integration test).
const (
	KeyAudit       = "retention.audit_days"
	KeySession     = "retention.session_days"
	KeyReservation = "retention.reservation_days"
	KeyAssistant   = "retention.assistant_days"
	KeyIdleThread  = "retention.idle_thread_days"
)

// EnvOverrides reads RETENTION_*_DAYS WITHOUT filling defaults: a zero field
// means "not overridden".
func EnvOverrides() Retention {
	get := func(k string) int {
		n, err := strconv.Atoi(os.Getenv(k))
		if err != nil || n <= 0 {
			return 0
		}
		return n
	}
	return Retention{
		AuditDays: get("RETENTION_AUDIT_DAYS"), SessionDays: get("RETENTION_SESSION_DAYS"),
		ReservationDays: get("RETENTION_RESERVATION_DAYS"), AssistantDays: get("RETENTION_ASSISTANT_DAYS"),
		IdleThreadDays: get("RETENTION_IDLE_THREAD_DAYS"),
	}
}

// Resolve merges the windows: a non-zero env override wins, else a valid
// company setting, else the default. Invalid or missing settings fall back
// silently (retention must never fail a sweep).
func Resolve(settings map[string]string, env Retention) Retention {
	pick := func(envV int, key string) int {
		if envV > 0 {
			return envV
		}
		if n, err := strconv.Atoi(settings[key]); err == nil && n > 0 {
			return n
		}
		return 0
	}
	return Retention{
		AuditDays: pick(env.AuditDays, KeyAudit), SessionDays: pick(env.SessionDays, KeySession),
		ReservationDays: pick(env.ReservationDays, KeyReservation), AssistantDays: pick(env.AssistantDays, KeyAssistant),
		IdleThreadDays: pick(env.IdleThreadDays, KeyIdleThread),
	}.withDefaults()
}

// Cutoff returns now minus days (UTC).
func Cutoff(now time.Time, days int) time.Time {
	return now.UTC().AddDate(0, 0, -days)
}

// Runner executes tasks under a database advisory lock.
type Runner struct {
	DB         *sql.DB
	Log        *slog.Logger
	Tasks      []Task
	Batch      int           // rows per delete (default 5000)
	MaxBatches int           // passes per task per run (default 200)
	Pause      time.Duration // breather between passes (default 50ms)
}

// ErrLocked reports that another process holds the housekeeping lock.
var ErrLocked = errors.New("housekeeping: lock held by another process")

func (r *Runner) defaults() (batch, maxBatches int, pause time.Duration) {
	batch, maxBatches, pause = r.Batch, r.MaxBatches, r.Pause
	if batch <= 0 {
		batch = 5000
	}
	if maxBatches <= 0 {
		maxBatches = 200
	}
	if pause <= 0 {
		pause = 50 * time.Millisecond
	}
	return
}

// RunOnce sweeps every task once. It returns per-task deleted counts. A
// failing task is logged and the sweep continues; the first error is
// returned after the others ran. ErrLocked when another process sweeps.
func (r *Runner) RunOnce(ctx context.Context) (map[string]int64, error) {
	conn, err := r.DB.Conn(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	var got sql.NullInt64
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK(?, 0)", lockName).Scan(&got); err != nil {
		return nil, err
	}
	if !got.Valid || got.Int64 != 1 {
		return nil, ErrLocked
	}
	defer func() {
		// The caller's ctx may already be cancelled during shutdown.
		rel, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, _ = conn.ExecContext(rel, "SELECT RELEASE_LOCK(?)", lockName)
	}()

	batch, maxBatches, pause := r.defaults()
	out := map[string]int64{}
	var first error
	for _, t := range r.Tasks {
		var total int64
		for i := 0; i < maxBatches; i++ {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			n, err := t.Run(ctx, batch)
			total += n
			if err != nil {
				r.log().Error("housekeeping task failed", "task", t.Name, "error", err)
				if first == nil {
					first = err
				}
				break
			}
			if n < int64(batch) {
				break
			}
			select {
			case <-ctx.Done():
				return out, ctx.Err()
			case <-time.After(pause):
			}
		}
		out[t.Name] = total
		if total > 0 {
			r.log().Info("housekeeping", "task", t.Name, "deleted", total)
		}
	}
	return out, first
}

func (r *Runner) log() *slog.Logger {
	if r.Log != nil {
		return r.Log
	}
	return slog.Default()
}

// NextRun returns the next occurrence of hour:00 WIB strictly after now.
func NextRun(now time.Time, hour int) time.Time {
	local := now.In(timeutil.Jakarta)
	next := time.Date(local.Year(), local.Month(), local.Day(), hour, 0, 0, 0, timeutil.Jakarta)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next.UTC()
}

// Loop sweeps once a day at hour:00 WIB until ctx is cancelled.
func (r *Runner) Loop(ctx context.Context, hour int) {
	for {
		wait := time.Until(NextRun(time.Now(), hour))
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if _, err := r.RunOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			if errors.Is(err, ErrLocked) {
				r.log().Info("housekeeping skipped: another process holds the lock")
			} else {
				r.log().Error("housekeeping run", "error", err)
			}
		}
	}
}
