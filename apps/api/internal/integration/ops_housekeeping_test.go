package integration

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"mini-erp/internal/database"
	companyapp "mini-erp/internal/modules/company/application"
	"mini-erp/internal/shared/housekeeping"
)

func count(t *testing.T, q string) int {
	t.Helper()
	var n int
	if err := testMods.db.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return n
}

// housekeepingTasksForTest mirrors bootstrap.Modules.HousekeepingTasks over
// the harness modules (the harness keeps modules, not the Wire result).
func housekeepingTasksForTest() []housekeeping.Task {
	ret := housekeeping.Defaults()
	cut := func(days int) time.Time { return housekeeping.Cutoff(time.Now(), days) }
	m := testMods
	return []housekeeping.Task{
		{Name: "stock_reservations", Run: func(c context.Context, n int) (int64, error) {
			return m.stock.Service().PurgeReservations(c, cut(ret.ReservationDays), n)
		}},
		{Name: "user_sessions", Run: func(c context.Context, n int) (int64, error) {
			return m.auth.Service().PurgeSessions(c, cut(ret.SessionDays), n)
		}},
		{Name: "assistant_tool_executions", Run: func(c context.Context, n int) (int64, error) {
			return m.assistant.Service().PurgeToolExecutions(c, cut(ret.AssistantDays), n)
		}},
		{Name: "assistant_messages", Run: func(c context.Context, n int) (int64, error) {
			return m.assistant.Service().PurgeMessages(c, cut(ret.AssistantDays), n)
		}},
		{Name: "assistant_runs", Run: func(c context.Context, n int) (int64, error) {
			return m.assistant.Service().PurgeRuns(c, cut(ret.AssistantDays), n)
		}},
		{Name: "assistant_threads", Run: func(c context.Context, n int) (int64, error) {
			return m.assistant.Service().PurgeIdleThreads(c, cut(ret.IdleThreadDays), n)
		}},
		{Name: "audit_logs", Run: func(c context.Context, n int) (int64, error) {
			return m.audit.Service().Purge(c, cut(ret.AuditDays), n)
		}},
	}
}

// TestHousekeepingRetention (F4-10): old finished rows go, live/recent rows
// stay, a second run is a no-op, and a held advisory lock skips the sweep.
func TestHousekeepingRetention(t *testing.T) {
	freshDB(t)
	db := testMods.db
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	// Reservations: old expired (delete), old active (keep), recent expired (keep).
	for i, r := range []struct {
		key, status, age string
	}{
		{"hk-old-expired", "expired", "30 DAY"},
		{"hk-old-active", "active", "30 DAY"},
		{"hk-new-expired", "expired", "1 DAY"},
	} {
		_ = i
		exec(`INSERT INTO stock_reservations (reservation_key, branch_id, product_id, variant_id, qty_total, qty_remaining, status, expires_at)
		      VALUES (?, 1, 1, 1, 1, 0, ?, UTC_TIMESTAMP(6) - INTERVAL `+r.age+`)`, r.key, r.status)
	}
	// Sessions: expired long ago (delete), revoked long ago (delete), live (keep).
	hash := func(c string) string { return c + strings.Repeat("0", 63) }
	exec(`INSERT INTO user_sessions (user_id, refresh_hash, expires_at, absolute_expires_at, last_activity_at)
	      VALUES (1, ?, UTC_TIMESTAMP(6) - INTERVAL 90 DAY, UTC_TIMESTAMP(6) - INTERVAL 90 DAY, UTC_TIMESTAMP(6) - INTERVAL 90 DAY)`, hash("a"))
	exec(`INSERT INTO user_sessions (user_id, refresh_hash, expires_at, absolute_expires_at, last_activity_at, revoked_at)
	      VALUES (1, ?, UTC_TIMESTAMP(6) + INTERVAL 1 DAY, UTC_TIMESTAMP(6) + INTERVAL 1 DAY, UTC_TIMESTAMP(6), UTC_TIMESTAMP(6) - INTERVAL 60 DAY)`, hash("b"))
	exec(`INSERT INTO user_sessions (user_id, refresh_hash, expires_at, absolute_expires_at, last_activity_at)
	      VALUES (1, ?, UTC_TIMESTAMP(6) + INTERVAL 1 DAY, UTC_TIMESTAMP(6) + INTERVAL 1 DAY, UTC_TIMESTAMP(6))`, hash("c"))
	// Assistant history: an old run with tool execution + message + idle thread.
	exec(`INSERT INTO assistant_threads (phone, last_message_at, created_at) VALUES ('6281', UTC_TIMESTAMP(6) - INTERVAL 400 DAY, UTC_TIMESTAMP(6) - INTERVAL 400 DAY)`)
	exec(`INSERT INTO assistant_runs (intent, created_at) VALUES ('help', UTC_TIMESTAMP(6) - INTERVAL 200 DAY)`)
	exec(`INSERT INTO assistant_runs (intent) VALUES ('help')`)
	exec(`INSERT INTO assistant_tool_executions (run_id, tool, input_json, created_at) VALUES (1, 'x', '{}', UTC_TIMESTAMP(6) - INTERVAL 200 DAY)`)
	exec(`INSERT INTO assistant_messages (thread_id, direction, body, created_at) VALUES (1, 'in', 'hai', UTC_TIMESTAMP(6) - INTERVAL 200 DAY)`)

	r := &housekeeping.Runner{DB: db, Tasks: housekeepingTasksForTest(), Batch: 1}
	got, err := r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("run: %v (%v)", err, got)
	}
	if got["stock_reservations"] != 1 || got["user_sessions"] != 2 || got["assistant_runs"] != 1 ||
		got["assistant_tool_executions"] != 1 || got["assistant_messages"] != 1 || got["assistant_threads"] != 1 {
		t.Fatalf("deleted counts = %v", got)
	}
	if n := count(t, "SELECT COUNT(*) FROM stock_reservations"); n != 2 {
		t.Fatalf("reservations left = %d, want 2 (active + recent)", n)
	}
	if n := count(t, "SELECT COUNT(*) FROM user_sessions"); n != 1 {
		t.Fatalf("sessions left = %d, want 1 (live)", n)
	}
	if n := count(t, "SELECT COUNT(*) FROM assistant_runs"); n != 1 {
		t.Fatalf("runs left = %d, want 1 (recent)", n)
	}

	// Idempotent.
	got, err = r.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	for k, v := range got {
		if v != 0 {
			t.Fatalf("rerun deleted %d from %s", v, k)
		}
	}

	// Another process holding the advisory lock -> this run is skipped.
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	var ok int
	if err := conn.QueryRowContext(context.Background(), "SELECT GET_LOCK('mini_erp_housekeeping', 0)").Scan(&ok); err != nil || ok != 1 {
		t.Fatalf("take lock: %v %d", err, ok)
	}
	if _, err := r.RunOnce(context.Background()); !errors.Is(err, housekeeping.ErrLocked) {
		t.Fatalf("expected ErrLocked, got %v", err)
	}
	_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('mini_erp_housekeeping')")
}

// F4-10: the retention windows come from company settings (registered keys),
// defaults equal the housekeeping defaults, and the env still overrides.
func TestRetentionFromCompanySettings(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	defs := map[string]string{}
	for _, d := range companyapp.SettingDefinitions() {
		defs[d.Key] = d.Default
	}
	d := housekeeping.Defaults()
	for key, want := range map[string]int{
		housekeeping.KeyAudit: d.AuditDays, housekeeping.KeySession: d.SessionDays,
		housekeeping.KeyReservation: d.ReservationDays, housekeeping.KeyAssistant: d.AssistantDays,
		housekeeping.KeyIdleThread: d.IdleThreadDays,
	} {
		if defs[key] != strconv.Itoa(want) {
			t.Fatalf("registered default of %s = %q, want %d", key, defs[key], want)
		}
	}
	if _, err := testMods.db.Exec(`INSERT INTO company_settings (setting_key, setting_value) VALUES ('retention.audit_days','365')
		ON DUPLICATE KEY UPDATE setting_value='365'`); err != nil {
		t.Fatal(err)
	}
	if got := testMods.company.Company(); got == nil {
		t.Fatal("no company client")
	}
	settings, err := testMods.company.Company().GetSettings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if r := housekeeping.Resolve(settings, housekeeping.Retention{}); r.AuditDays != 365 || r.SessionDays != d.SessionDays {
		t.Fatalf("resolved %+v", r)
	}
	t.Setenv("RETENTION_AUDIT_DAYS", "100")
	if r := housekeeping.Resolve(settings, housekeeping.EnvOverrides()); r.AuditDays != 100 {
		t.Fatalf("env must override settings, got %d", r.AuditDays)
	}
}

// F4-12: every pooled connection opened by database.Open runs READ COMMITTED
// on a UTC session clock, with the production pool bounds.
func TestOpenAppliesSessionAttributes(t *testing.T) {
	db, err := database.OpenWithPool(testDBDSN(), 5, 2, time.Minute, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if st := db.Stats(); st.MaxOpenConnections != 5 {
		t.Fatalf("max open = %d, want 5", st.MaxOpenConnections)
	}
	// Several connections at once: the init hook runs on each, not only the first.
	ctx := context.Background()
	var conns []*sql.Conn
	for i := 0; i < 3; i++ {
		c, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
		var iso, tz string
		if err := c.QueryRowContext(ctx, "SELECT @@session.transaction_isolation, @@session.time_zone").Scan(&iso, &tz); err != nil {
			t.Fatal(err)
		}
		if iso != "READ-COMMITTED" || tz != "+00:00" {
			t.Fatalf("conn %d: isolation=%s tz=%s", i, iso, tz)
		}
	}
	for _, c := range conns {
		_ = c.Close()
	}
}
