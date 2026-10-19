package integration

import (
	"regexp"
	"testing"
	"time"

	"mini-erp/internal/modules/assistant/contracts"
	assistantinfra "mini-erp/internal/modules/assistant/infrastructure"
)

// Characterization of the assistant Repository (F8-18): locks the observable
// behaviour of every public method before and after the sqlc migration.

var sqlcAsstTsRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}$`)

func TestSqlcAssistantChannelConfig(t *testing.T) {
	freshDB(t)
	repo := assistantinfra.NewRepository(testMods.db)

	// Missing rows are created with defaults.
	ch, err := repo.GetChannel(ctx())
	if err != nil || ch.State != "disconnected" || ch.Phone != "" || ch.LastError != "" {
		t.Fatalf("default channel = %+v, %v", ch, err)
	}
	ch, err = repo.GetChannel(ctx())
	if err != nil || ch.State != "disconnected" || !sqlcAsstTsRe.MatchString(ch.UpdatedAt) {
		t.Fatalf("channel after create = %+v, %v", ch, err)
	}
	if err := repo.SetChannel(ctx(), "connected", "6281", "boom"); err != nil {
		t.Fatal(err)
	}
	ch, _ = repo.GetChannel(ctx())
	if ch.State != "connected" || ch.Phone != "6281" || ch.LastError != "boom" {
		t.Fatalf("channel = %+v", ch)
	}
	// Empty phone/lastErr map to NULL and read back as "".
	if err := repo.SetChannel(ctx(), "disconnected", "", ""); err != nil {
		t.Fatal(err)
	}
	ch, _ = repo.GetChannel(ctx())
	if ch.State != "disconnected" || ch.Phone != "" || ch.LastError != "" {
		t.Fatalf("channel cleared = %+v", ch)
	}
	var nulls int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM assistant_channel WHERE phone IS NULL AND last_error IS NULL`).Scan(&nulls); err != nil || nulls != 1 {
		t.Fatalf("expected NULL columns, got %d (%v)", nulls, err)
	}

	cfg, err := repo.GetConfig(ctx())
	if err != nil || cfg.Mode != "rule_based" || cfg.RateLimitPerMin != 10 {
		t.Fatalf("default config = %+v, %v", cfg, err)
	}
	if err := repo.UpdateConfig(ctx(), "llm", 33); err != nil {
		t.Fatal(err)
	}
	cfg, _ = repo.GetConfig(ctx())
	if cfg.Mode != "llm" || cfg.RateLimitPerMin != 33 || !sqlcAsstTsRe.MatchString(cfg.UpdatedAt) {
		t.Fatalf("config = %+v", cfg)
	}
	// UpdateConfig on a fresh table upserts.
	freshDB(t)
	if err := repo.UpdateConfig(ctx(), "rule_based", 5); err != nil {
		t.Fatal(err)
	}
	cfg, _ = repo.GetConfig(ctx())
	if cfg.RateLimitPerMin != 5 {
		t.Fatalf("upserted config = %+v", cfg)
	}
}

func TestSqlcAssistantAuthorizations(t *testing.T) {
	freshDB(t)
	repo := assistantinfra.NewRepository(testMods.db)

	if got, err := repo.ListAuthorizations(ctx(), true); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty list = %v, %v (must be non-nil empty)", got, err)
	}
	id1, err := repo.CreateAuthorization(ctx(), &contracts.Authorization{Phone: "6281", Name: "A", AccessLevel: "authorized_party"}, 0)
	if err != nil || id1 == 0 {
		t.Fatalf("create: %d %v", id1, err)
	}
	id2, _ := repo.CreateAuthorization(ctx(), &contracts.Authorization{Phone: "6282", Name: "B", AccessLevel: "owner", IsPrimaryOwner: true}, 7)
	id3, _ := repo.CreateAuthorization(ctx(), &contracts.Authorization{Phone: "6283", Name: "C", AccessLevel: "authorized_party"}, 7)

	var createdBy, updatedBy *int64
	if err := testMods.db.QueryRow(`SELECT created_by, updated_by FROM assistant_authorizations WHERE id = ?`, id1).Scan(&createdBy, &updatedBy); err != nil || createdBy != nil || updatedBy != nil {
		t.Fatalf("actor 0 must store NULL: %v %v %v", createdBy, updatedBy, err)
	}
	if err := testMods.db.QueryRow(`SELECT created_by, updated_by FROM assistant_authorizations WHERE id = ?`, id2).Scan(&createdBy, &updatedBy); err != nil || createdBy == nil || *createdBy != 7 {
		t.Fatalf("actor 7 stored wrong: %v %v", createdBy, err)
	}

	// Order: primary first, then id DESC.
	list, err := repo.ListAuthorizations(ctx(), false)
	if err != nil || len(list) != 3 || list[0].ID != id2 || list[1].ID != id3 || list[2].ID != id1 {
		t.Fatalf("order = %+v %v", list, err)
	}
	if !list[0].IsPrimaryOwner || list[1].IsPrimaryOwner || list[0].LastSeenAt != "" || list[0].Status != "active" {
		t.Fatalf("fields = %+v", list[0])
	}

	// FindActiveByPhone.
	a, err := repo.FindActiveByPhone(ctx(), "6281")
	if err != nil || a == nil || a.ID != id1 || a.Name != "A" || a.AccessLevel != "authorized_party" {
		t.Fatalf("find = %+v %v", a, err)
	}
	if a, err := repo.FindActiveByPhone(ctx(), "nope"); a != nil || err != nil {
		t.Fatalf("find missing = %v %v", a, err)
	}

	// PhoneTaken: excludes the id itself; reports any status.
	taken, status, err := repo.PhoneTaken(ctx(), "6281", 0)
	if err != nil || !taken || status != "active" {
		t.Fatalf("taken = %v %q %v", taken, status, err)
	}
	if taken, _, _ := repo.PhoneTaken(ctx(), "6281", id1); taken {
		t.Fatal("excluded id must not count as taken")
	}
	if taken, status, err := repo.PhoneTaken(ctx(), "6999", 0); taken || status != "" || err != nil {
		t.Fatalf("free phone = %v %q %v", taken, status, err)
	}

	// UpdateAuthorization: ok for active, false for missing.
	ok, err := repo.UpdateAuthorization(ctx(), &contracts.Authorization{ID: id1, Phone: "6281", Name: "A2", AccessLevel: "owner", IsPrimaryOwner: true}, 9)
	if err != nil || !ok {
		t.Fatalf("update = %v %v", ok, err)
	}
	if ok, _ := repo.UpdateAuthorization(ctx(), &contracts.Authorization{ID: 9999, Phone: "x", Name: "x", AccessLevel: "x"}, 9); ok {
		t.Fatal("update of missing row must report false")
	}
	if a, _ := repo.FindActiveByPhone(ctx(), "6281"); a.Name != "A2" || !a.IsPrimaryOwner {
		t.Fatalf("after update = %+v", a)
	}

	// DemoteOthers keeps exactly the kept id primary.
	if err := repo.DemoteOthers(ctx(), id1); err != nil {
		t.Fatal(err)
	}
	if a, _ := repo.FindActiveByPhone(ctx(), "6282"); a.IsPrimaryOwner {
		t.Fatal("id2 must be demoted")
	}
	if a, _ := repo.FindActiveByPhone(ctx(), "6281"); !a.IsPrimaryOwner {
		t.Fatal("kept id must stay primary")
	}

	// TouchSeen stamps last_seen_at (formatted without microseconds).
	if err := repo.TouchSeen(ctx(), id3); err != nil {
		t.Fatal(err)
	}
	a, _ = repo.FindActiveByPhone(ctx(), "6283")
	if !sqlcAsstTsRe.MatchString(a.LastSeenAt) {
		t.Fatalf("last seen = %q", a.LastSeenAt)
	}
	var drift float64
	if err := testMods.db.QueryRow(`SELECT ABS(TIMESTAMPDIFF(SECOND, last_seen_at, UTC_TIMESTAMP(6))) FROM assistant_authorizations WHERE id = ?`, id3).Scan(&drift); err != nil || drift > 5 {
		t.Fatalf("last_seen_at drift = %v (%v)", drift, err)
	}

	// Revoke: flips status, drops primary, only once.
	ok, err = repo.Revoke(ctx(), id1, 0)
	if err != nil || !ok {
		t.Fatalf("revoke = %v %v", ok, err)
	}
	if ok, _ := repo.Revoke(ctx(), id1, 0); ok {
		t.Fatal("second revoke must report false")
	}
	if a, _ := repo.FindActiveByPhone(ctx(), "6281"); a != nil {
		t.Fatal("revoked must not be found as active")
	}
	if ok, _ := repo.UpdateAuthorization(ctx(), &contracts.Authorization{ID: id1, Phone: "6281", Name: "z", AccessLevel: "z"}, 0); ok {
		t.Fatal("revoked row must not be updatable")
	}
	taken, status, _ = repo.PhoneTaken(ctx(), "6281", 0)
	if !taken || status != "revoked" {
		t.Fatalf("revoked phone = %v %q", taken, status)
	}
	if got, _ := repo.ListAuthorizations(ctx(), false); len(got) != 2 {
		t.Fatalf("active list = %d", len(got))
	}
	all, _ := repo.ListAuthorizations(ctx(), true)
	if len(all) != 3 || all[0].IsPrimaryOwner {
		t.Fatalf("all list = %+v", all)
	}

	// Reactivate resets last seen and applies new attributes.
	if err := repo.Reactivate(ctx(), id1, &contracts.Authorization{Name: "A3", AccessLevel: "authorized_party", IsPrimaryOwner: false}, 3); err != nil {
		t.Fatal(err)
	}
	a, _ = repo.FindActiveByPhone(ctx(), "6281")
	if a == nil || a.Name != "A3" || a.Status != "active" || a.LastSeenAt != "" {
		t.Fatalf("reactivated = %+v", a)
	}
}

func TestSqlcAssistantThreadsRunsStats(t *testing.T) {
	freshDB(t)
	repo := assistantinfra.NewRepository(testMods.db)

	if id, err := repo.FindOpenThread(ctx(), "6281"); id != 0 || err != nil {
		t.Fatalf("no thread = %d %v", id, err)
	}
	t1, err := repo.CreateThread(ctx(), "6281", 0)
	if err != nil || t1 == 0 {
		t.Fatal(err)
	}
	t2, _ := repo.CreateThread(ctx(), "6281", 5)
	if _, err := repo.CreateThread(ctx(), "6282", 0); err != nil {
		t.Fatal(err)
	}
	var br *int64
	if err := testMods.db.QueryRow(`SELECT branch_id FROM assistant_threads WHERE id = ?`, t1).Scan(&br); err != nil || br != nil {
		t.Fatalf("branch 0 must be NULL: %v %v", br, err)
	}
	if id, _ := repo.FindOpenThread(ctx(), "6281"); id != t2 {
		t.Fatalf("latest open thread = %d, want %d", id, t2)
	}
	if _, err := testMods.db.Exec(`UPDATE assistant_threads SET status = 'closed' WHERE id = ?`, t2); err != nil {
		t.Fatal(err)
	}
	if id, _ := repo.FindOpenThread(ctx(), "6281"); id != t1 {
		t.Fatalf("closed skipped: got %d want %d", id, t1)
	}
	if _, err := testMods.db.Exec(`UPDATE assistant_threads SET last_message_at = '2000-01-01 00:00:00' WHERE id = ?`, t1); err != nil {
		t.Fatal(err)
	}
	if err := repo.TouchThread(ctx(), t1); err != nil {
		t.Fatal(err)
	}
	var drift float64
	if err := testMods.db.QueryRow(`SELECT ABS(TIMESTAMPDIFF(SECOND, last_message_at, UTC_TIMESTAMP(6))) FROM assistant_threads WHERE id = ?`, t1).Scan(&drift); err != nil || drift > 5 {
		t.Fatalf("touch drift = %v %v", drift, err)
	}

	// Messages.
	m1, err := repo.InsertMessage(ctx(), t1, "in", "halo", "received", 0)
	if err != nil || m1 == 0 {
		t.Fatal(err)
	}
	var runRef *int64
	var status string
	if err := testMods.db.QueryRow(`SELECT status, run_id FROM assistant_messages WHERE id = ?`, m1).Scan(&status, &runRef); err != nil || status != "received" || runRef != nil {
		t.Fatalf("message = %q %v %v", status, runRef, err)
	}

	// Runs.
	r1, err := repo.CreateRun(ctx(), t1, 0, 4, "6281", "stock", "rule_based")
	if err != nil || r1 == 0 {
		t.Fatal(err)
	}
	r2, _ := repo.CreateRun(ctx(), 0, 2, 0, "", "stock", "rule_based")
	r3, _ := repo.CreateRun(ctx(), t1, 0, 4, "6281", "help", "rule_based")
	if err := repo.SetMessageStatus(ctx(), m1, "processed", r1); err != nil {
		t.Fatal(err)
	}
	if err := testMods.db.QueryRow(`SELECT status, run_id FROM assistant_messages WHERE id = ?`, m1).Scan(&status, &runRef); err != nil || status != "processed" || runRef == nil || *runRef != r1 {
		t.Fatalf("message after = %q %v %v", status, runRef, err)
	}
	if err := repo.SetMessageStatus(ctx(), m1, "failed", 0); err != nil {
		t.Fatal(err)
	}
	if err := testMods.db.QueryRow(`SELECT status, run_id FROM assistant_messages WHERE id = ?`, m1).Scan(&status, &runRef); err != nil || status != "failed" || runRef != nil {
		t.Fatalf("message failed = %q %v %v", status, runRef, err)
	}
	var tid, bid *int64
	var phone *string
	if err := testMods.db.QueryRow(`SELECT thread_id, branch_id, phone FROM assistant_runs WHERE id = ?`, r2).Scan(&tid, &bid, &phone); err != nil || tid != nil || phone != nil || bid == nil || *bid != 2 {
		t.Fatalf("run NULLs = %v %v %v %v", tid, bid, phone, err)
	}
	if err := repo.FinishRun(ctx(), r1, "completed", "jawaban", 120, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishRun(ctx(), r2, "completed", "", 31, ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.FinishRun(ctx(), r3, "failed", "", 5, "gagal"); err != nil {
		t.Fatal(err)
	}
	var answer, failure *string
	if err := testMods.db.QueryRow(`SELECT answer, failure_reason FROM assistant_runs WHERE id = ?`, r3).Scan(&answer, &failure); err != nil || answer != nil || failure == nil || *failure != "gagal" {
		t.Fatalf("finish NULLs = %v %v %v", answer, failure, err)
	}
	// An old run (outside the 7-day window) must not be reported.
	if _, err := testMods.db.Exec(`INSERT INTO assistant_runs (intent, created_at) VALUES ('old', UTC_TIMESTAMP(6) - INTERVAL 30 DAY)`); err != nil {
		t.Fatal(err)
	}

	// Tool executions: empty output maps to NULL.
	if err := repo.InsertToolExecution(ctx(), r1, 1, "stock.get", `{"q":1}`, "", 9, "failed", "boom"); err != nil {
		t.Fatal(err)
	}
	if err := repo.InsertToolExecution(ctx(), r1, 2, "stock.get", `{"q":2}`, `{"ok":true}`, 3, "success", ""); err != nil {
		t.Fatal(err)
	}
	var out, errText *string
	var toolStatus string
	if err := testMods.db.QueryRow(`SELECT output_json, status, error_text FROM assistant_tool_executions WHERE seq = 1`).Scan(&out, &toolStatus, &errText); err != nil || out != nil {
		t.Fatalf("tool output NULL = %v %v", out, err)
	}
	if toolStatus != "failed" || errText == nil || *errText != "boom" {
		t.Fatalf("failed tool row = %q %v, want failed/boom", toolStatus, errText)
	}
	var okStatus string
	var okErr *string
	if err := testMods.db.QueryRow(`SELECT status, error_text FROM assistant_tool_executions WHERE seq = 2`).Scan(&okStatus, &okErr); err != nil || okStatus != "success" || okErr != nil {
		t.Fatalf("success tool row = %q %v %v", okStatus, okErr, err)
	}

	stats, err := repo.RunStats(ctx())
	if err != nil || len(stats) != 2 {
		t.Fatalf("stats = %+v %v", stats, err)
	}
	// stock: 2 runs, 2 completed, avg (120+31)/2 = 75.5 -> 76 (round half up);
	// help: 1 run, 0 completed, avg 5. Ordered by count desc within the day.
	if s := stats[0]; s.Intent != "stock" || s.Mode != "rule_based" || s.Total != 2 || s.Success != 2 || s.AvgDurationMs != 76 {
		t.Fatalf("stats[0] = %+v", s)
	}
	if s := stats[1]; s.Intent != "help" || s.Total != 1 || s.Success != 0 || s.AvgDurationMs != 5 {
		t.Fatalf("stats[1] = %+v", s)
	}
	t.Logf("RunStats day format = %q", stats[0].Day)
	if stats[0].Day != stats[1].Day {
		t.Fatalf("days differ: %q %q", stats[0].Day, stats[1].Day)
	}
	if want := time.Now().UTC().Format("2006-01-02T00:00:00Z"); stats[0].Day != want {
		t.Fatalf("day = %q, want %q", stats[0].Day, want)
	}
}

func TestSqlcAssistantPurge(t *testing.T) {
	freshDB(t)
	repo := assistantinfra.NewRepository(testMods.db)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := testMods.db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	cutoff := time.Now().UTC().Add(-24 * time.Hour)
	for i := 0; i < 5; i++ {
		exec(`INSERT INTO assistant_tool_executions (run_id, tool, input_json, created_at) VALUES (1, 'x', '{}', UTC_TIMESTAMP(6) - INTERVAL 3 DAY)`)
		exec(`INSERT INTO assistant_messages (thread_id, direction, body, created_at) VALUES (9, 'in', 'hai', UTC_TIMESTAMP(6) - INTERVAL 3 DAY)`)
		exec(`INSERT INTO assistant_runs (intent, created_at) VALUES ('help', UTC_TIMESTAMP(6) - INTERVAL 3 DAY)`)
	}
	exec(`INSERT INTO assistant_tool_executions (run_id, tool, input_json) VALUES (1, 'x', '{}')`)
	exec(`INSERT INTO assistant_messages (thread_id, direction, body) VALUES (9, 'in', 'baru')`)
	exec(`INSERT INTO assistant_runs (intent) VALUES ('baru')`)

	for name, fn := range map[string]func() (int64, error){
		"tool": func() (int64, error) { return repo.PurgeToolExecutions(ctx(), cutoff, 3) },
		"msg":  func() (int64, error) { return repo.PurgeMessages(ctx(), cutoff, 3) },
		"run":  func() (int64, error) { return repo.PurgeRuns(ctx(), cutoff, 3) },
	} {
		if n, err := fn(); err != nil || n != 3 {
			t.Fatalf("%s first batch = %d %v (limit 3)", name, n, err)
		}
		if n, err := fn(); err != nil || n != 2 {
			t.Fatalf("%s second batch = %d %v", name, n, err)
		}
		if n, err := fn(); err != nil || n != 0 {
			t.Fatalf("%s third batch = %d %v", name, n, err)
		}
	}
	for _, tbl := range []string{"assistant_tool_executions", "assistant_messages", "assistant_runs"} {
		if c := count(t, "SELECT COUNT(*) FROM "+tbl); c != 1 {
			t.Fatalf("%s left = %d, want the recent row", tbl, c)
		}
	}

	// Idle threads: only old threads without messages go.
	exec(`INSERT INTO assistant_threads (phone, last_message_at) VALUES ('1', UTC_TIMESTAMP(6) - INTERVAL 5 DAY)`) // id1: old, no msg -> purge
	exec(`INSERT INTO assistant_threads (phone, last_message_at) VALUES ('2', UTC_TIMESTAMP(6) - INTERVAL 5 DAY)`) // id2: old, has msg -> keep
	exec(`INSERT INTO assistant_threads (phone, last_message_at) VALUES ('3', UTC_TIMESTAMP(6))`)                  // id3: recent -> keep
	exec(`INSERT INTO assistant_threads (phone, last_message_at) VALUES ('4', UTC_TIMESTAMP(6) - INTERVAL 5 DAY)`) // id4: old, no msg -> purge
	exec(`INSERT INTO assistant_messages (thread_id, direction, body) VALUES (2, 'in', 'x')`)
	if n, err := repo.PurgeIdleThreads(ctx(), cutoff, 1); err != nil || n != 1 {
		t.Fatalf("idle batch 1 = %d %v", n, err)
	}
	if n, err := repo.PurgeIdleThreads(ctx(), cutoff, 10); err != nil || n != 1 {
		t.Fatalf("idle batch 2 = %d %v", n, err)
	}
	if n, err := repo.PurgeIdleThreads(ctx(), cutoff, 10); err != nil || n != 0 {
		t.Fatalf("idle batch 3 = %d %v", n, err)
	}
	if c := count(t, "SELECT COUNT(*) FROM assistant_threads"); c != 2 {
		t.Fatalf("threads left = %d, want 2", c)
	}
}
