package integration

import (
	"strings"
	"testing"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
)

func mustLog(t *testing.T, e auditcontracts.Entry) {
	t.Helper()
	if err := testMods.audit.Service().Log(ctx(), e); err != nil {
		t.Fatalf("log %s: %v", e.Action, err)
	}
}

// TestAuditQueryFiltersAndCrossBranch (F3b-8): the console view filters by
// action prefix / entity / actor / WIB day range, returns before/after, and
// the cross-branch view sees other branches while the default scope shows
// the session branch plus global events only.
func TestAuditQueryFiltersAndCrossBranch(t *testing.T) {
	freshDB(t)
	svc := testMods.audit.Service()

	mustLog(t, auditcontracts.Entry{Action: "product.update", Entity: "product", EntityID: 5, BranchID: 1, ActorID: 11,
		Before: map[string]any{"price": 100}, After: map[string]any{"price": 150}})
	mustLog(t, auditcontracts.Entry{Action: "product.create", Entity: "product", EntityID: 6, BranchID: 2, ActorID: 12})
	mustLog(t, auditcontracts.Entry{Action: "roles.permissions", Entity: "role", EntityID: 3, ActorID: 11}) // global
	mustLog(t, auditcontracts.Entry{Action: "stock.adjust", Entity: "stock_adjustment", BranchID: 2, ActorID: 12})

	q := func(f auditcontracts.ListFilter) ([]auditcontracts.Record, int64) {
		t.Helper()
		f.Page, f.Limit = 1, 50
		recs, total, err := svc.Query(ctx(), f)
		if err != nil {
			t.Fatalf("query %+v: %v", f, err)
		}
		return recs, total
	}

	// Default scope: branch 1 + global.
	recs, total := q(auditcontracts.ListFilter{BranchID: 1})
	if total != 2 || len(recs) != 2 {
		t.Fatalf("branch 1 scope total=%d len=%d, want 2", total, len(recs))
	}
	// Newest first.
	if recs[0].Action != "roles.permissions" {
		t.Fatalf("order: first = %s", recs[0].Action)
	}

	// Cross-branch.
	_, total = q(auditcontracts.ListFilter{BranchID: 1, AllBranches: true})
	if total != 4 {
		t.Fatalf("all branches total = %d, want 4", total)
	}
	// Another branch explicitly.
	_, total = q(auditcontracts.ListFilter{BranchID: 2})
	if total != 3 { // 2 of its own + 1 global
		t.Fatalf("branch 2 total = %d, want 3", total)
	}

	// Action prefix, with LIKE wildcards escaped.
	_, total = q(auditcontracts.ListFilter{AllBranches: true, Action: "product."})
	if total != 2 {
		t.Fatalf("prefix product. = %d, want 2", total)
	}
	_, total = q(auditcontracts.ListFilter{AllBranches: true, Action: "product_"})
	if total != 0 {
		t.Fatalf("underscore must be literal, got %d", total)
	}
	_, total = q(auditcontracts.ListFilter{AllBranches: true, Entity: "role"})
	if total != 1 {
		t.Fatalf("entity role = %d", total)
	}
	_, total = q(auditcontracts.ListFilter{AllBranches: true, ActorID: 12})
	if total != 2 {
		t.Fatalf("actor 12 = %d", total)
	}
	recs, _ = q(auditcontracts.ListFilter{AllBranches: true, Entity: "product", EntityID: 5})
	if len(recs) != 1 || !strings.Contains(string(recs[0].Before), "100") || !strings.Contains(string(recs[0].After), "150") {
		t.Fatalf("before/after not returned: %+v", recs)
	}
	if recs[0].ActorType != "user" {
		t.Fatalf("actor type = %q", recs[0].ActorType)
	}

	// Date range: today (WIB) matches, a past window does not.
	today := time.Now().UTC().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
	_, total = q(auditcontracts.ListFilter{AllBranches: true, From: today, To: today})
	if total != 4 {
		t.Fatalf("today range = %d, want 4", total)
	}
	_, total = q(auditcontracts.ListFilter{AllBranches: true, From: "2020-01-01", To: "2020-01-31"})
	if total != 0 {
		t.Fatalf("past range = %d, want 0", total)
	}

	// Pagination keeps the exact total.
	recs, total, err := svc.Query(ctx(), auditcontracts.ListFilter{AllBranches: true, Page: 2, Limit: 3})
	if err != nil || total != 4 || len(recs) != 1 {
		t.Fatalf("page 2: total=%d len=%d err=%v", total, len(recs), err)
	}
}

// TestAuditPurgeRetention (F4-9): purge is bounded, oldest first, idempotent,
// cascades details and never touches recent rows.
func TestAuditPurgeRetention(t *testing.T) {
	freshDB(t)
	svc := testMods.audit.Service()
	for i := 0; i < 5; i++ {
		mustLog(t, auditcontracts.Entry{Action: "t.old", Entity: "t", After: map[string]any{"i": i}})
	}
	mustLog(t, auditcontracts.Entry{Action: "t.new", Entity: "t", After: map[string]any{"k": 1}})
	if _, err := testMods.db.Exec(
		"UPDATE audit_logs SET created_at = UTC_TIMESTAMP(6) - INTERVAL 800 DAY WHERE action_key = 't.old'"); err != nil {
		t.Fatalf("age rows: %v", err)
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -730)

	n, err := svc.Purge(ctx(), cutoff, 2)
	if err != nil || n != 2 {
		t.Fatalf("first batch = %d err=%v, want 2", n, err)
	}
	n, _ = svc.Purge(ctx(), cutoff, 100)
	if n != 3 {
		t.Fatalf("second batch = %d, want 3", n)
	}
	n, _ = svc.Purge(ctx(), cutoff, 100)
	if n != 0 {
		t.Fatalf("idempotent run deleted %d", n)
	}
	var logs, details int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM audit_logs").Scan(&logs)
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM audit_log_details").Scan(&details)
	if logs != 1 || details != 1 {
		t.Fatalf("after purge logs=%d details=%d, want 1/1 (cascade)", logs, details)
	}
}
