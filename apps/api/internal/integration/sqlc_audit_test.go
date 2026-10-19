package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	auditinfra "mini-erp/internal/modules/audit/infrastructure"
)

// Characterization tests for the audit Repository (F8-18): every filter
// branch, both branch legs, pagination across legs, detail NULL handling,
// truncation, Purge bounds, and the contract List/Count wrappers.
func TestSqlcAuditRepository(t *testing.T) {
	c := context.Background()
	freshDB(t)
	repo := auditinfra.NewRepository(testMods.db)

	// Dataset: branch 1 (10 rows), branch 2 (6 rows), global (8 rows).
	for i := 0; i < 10; i++ {
		auditMust(t, repo.Log(c, auditcontracts.Entry{Action: "a.one", Entity: "ent1", EntityID: int64(i + 1), BranchID: 1, ActorID: 21, Note: fmt.Sprintf("n%d", i)}))
	}
	for i := 0; i < 6; i++ {
		auditMust(t, repo.Log(c, auditcontracts.Entry{Action: "b.two", Entity: "ent2", EntityID: 50, BranchID: 2, ActorID: 22}))
	}
	for i := 0; i < 8; i++ {
		auditMust(t, repo.Log(c, auditcontracts.Entry{Action: "g.glob", Entity: "ent1", ActorID: 0, ActorType: "system", Note: "  "}))
	}
	// Interleave ids by adding more branch-1 rows after the globals.
	auditMust(t, repo.Log(c, auditcontracts.Entry{Action: "a.late", Entity: "ent3", BranchID: 1, ActorID: 21,
		Before: map[string]any{"x": 1}}))
	auditMust(t, repo.Log(c, auditcontracts.Entry{Action: "a.late", Entity: "ent3", BranchID: 1, ActorID: 21,
		After: map[string]any{"y": 2}}))
	auditMust(t, repo.Log(c, auditcontracts.Entry{Action: "a.late", Entity: "ent3", BranchID: 1, ActorID: 21,
		Metadata: map[string]any{"z": 3}}))
	// Empty-ish details produce no detail row.
	auditMust(t, repo.Log(c, auditcontracts.Entry{Action: "a.empty", Entity: "ent3", BranchID: 1, Before: map[string]any{}, After: []any{}, Metadata: ""}))
	// Oversize detail is replaced by a truncation marker.
	auditMust(t, repo.Log(c, auditcontracts.Entry{Action: "a.big", Entity: "ent3", BranchID: 1, After: strings.Repeat("x", 20000)}))

	total := 10 + 6 + 8 + 3 + 2 // 29
	q := func(f auditcontracts.ListFilter) ([]auditcontracts.Record, int64) {
		t.Helper()
		if f.Page == 0 {
			f.Page = 1
		}
		if f.Limit == 0 {
			f.Limit = 100
		}
		recs, n, err := repo.Query(c, f)
		if err != nil {
			t.Fatalf("query %+v: %v", f, err)
		}
		return recs, n
	}
	desc := func(recs []auditcontracts.Record) {
		t.Helper()
		for i := 1; i < len(recs); i++ {
			if recs[i-1].ID <= recs[i].ID {
				t.Fatalf("not newest-first at %d: %d then %d", i, recs[i-1].ID, recs[i].ID)
			}
		}
	}

	// Scope matrix.
	recs, n := q(auditcontracts.ListFilter{AllBranches: true})
	if n != int64(total) || len(recs) != total {
		t.Fatalf("all: %d/%d", n, len(recs))
	}
	desc(recs)
	recs, n = q(auditcontracts.ListFilter{BranchID: 1})
	if n != 10+8+3+2 || len(recs) != int(n) {
		t.Fatalf("b1+global: %d/%d", n, len(recs))
	}
	desc(recs)
	for _, r := range recs {
		if r.BranchID != 0 && r.BranchID != 1 {
			t.Fatalf("foreign branch leaked: %+v", r)
		}
	}
	recs, n = q(auditcontracts.ListFilter{BranchID: 2})
	if n != 6+8 || len(recs) != 14 {
		t.Fatalf("b2+global: %d/%d", n, len(recs))
	}
	recs, n = q(auditcontracts.ListFilter{BranchID: 999})
	if n != 8 || len(recs) != 8 {
		t.Fatalf("unknown branch sees global only: %d/%d", n, len(recs))
	}
	recs, n = q(auditcontracts.ListFilter{}) // BranchID<=0 = unscoped
	if n != int64(total) {
		t.Fatalf("branch 0 unscoped: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{BranchID: -3})
	if n != int64(total) {
		t.Fatalf("negative branch unscoped: %d", n)
	}

	// Individual filters and combinations.
	_, n = q(auditcontracts.ListFilter{AllBranches: true, Action: "a."})
	if n != 15 {
		t.Fatalf("action a.: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, Action: "a.o"})
	if n != 10 {
		t.Fatalf("action a.o: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, Action: "a%"})
	if n != 0 {
		t.Fatalf("percent literal: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, Entity: "ent1"})
	if n != 18 {
		t.Fatalf("entity ent1: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, Entity: "ent1", EntityID: 3})
	if n != 1 {
		t.Fatalf("entity+id: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, ActorID: 22})
	if n != 6 {
		t.Fatalf("actor 22: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{BranchID: 1, Entity: "ent1"})
	if n != 10+8 {
		t.Fatalf("branch1 + entity: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{BranchID: 1, Action: "g.", ActorID: 0})
	if n != 8 {
		t.Fatalf("branch1 + global action: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{BranchID: 1, ActorID: 21, Entity: "ent1", Action: "a.one"})
	if n != 10 {
		t.Fatalf("all combined: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{BranchID: 1, ActorID: 22})
	if n != 0 {
		t.Fatalf("branch1 actor 22: %d", n)
	}

	// Dates: from-only, to-only, invalid (ignored), future/past.
	today := time.Now().UTC().In(time.FixedZone("WIB", 7*3600)).Format("2006-01-02")
	tomorrow := time.Now().UTC().In(time.FixedZone("WIB", 7*3600)).AddDate(0, 0, 1).Format("2006-01-02")
	_, n = q(auditcontracts.ListFilter{AllBranches: true, From: today})
	if n != int64(total) {
		t.Fatalf("from today: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, From: tomorrow})
	if n != 0 {
		t.Fatalf("from tomorrow: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, To: today})
	if n != int64(total) {
		t.Fatalf("to today: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, To: "2020-01-01"})
	if n != 0 {
		t.Fatalf("to 2020: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, From: "garbage", To: "also-bad"})
	if n != int64(total) {
		t.Fatalf("invalid dates must be ignored: %d", n)
	}
	// Time boundaries: age one row to exactly the WIB start/end of a day.
	if _, err := testMods.db.Exec("UPDATE audit_logs SET created_at = '2024-05-09 17:00:00.000000' WHERE action_key = 'a.empty'"); err != nil {
		t.Fatal(err) // 2024-05-10 00:00 WIB
	}
	if _, err := testMods.db.Exec("UPDATE audit_logs SET created_at = '2024-05-10 16:59:59.999999' WHERE action_key = 'a.big'"); err != nil {
		t.Fatal(err) // last instant of 2024-05-10 WIB
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, From: "2024-05-10", To: "2024-05-10"})
	if n != 2 {
		t.Fatalf("WIB day boundaries inclusive: %d", n)
	}
	_, n = q(auditcontracts.ListFilter{AllBranches: true, From: "2024-05-11", To: "2024-05-11"})
	if n != 0 {
		t.Fatalf("next WIB day: %d", n)
	}
	// ids no longer match time order for these two rows: ORDER BY id must hold.
	recs, _ = q(auditcontracts.ListFilter{AllBranches: true})
	desc(recs)

	// Pagination: pages concatenate to the full ordered list, across legs.
	full, _ := q(auditcontracts.ListFilter{BranchID: 1})
	var paged []auditcontracts.Record
	for p := 1; p <= 8; p++ {
		pr, pn, err := repo.Query(c, auditcontracts.ListFilter{BranchID: 1, Page: p, Limit: 4})
		if err != nil || pn != int64(len(full)) {
			t.Fatalf("page %d: total=%d err=%v", p, pn, err)
		}
		paged = append(paged, pr...)
	}
	if len(paged) != len(full) {
		t.Fatalf("paged len %d want %d", len(paged), len(full))
	}
	for i := range full {
		if full[i].ID != paged[i].ID {
			t.Fatalf("paged[%d]=%d want %d", i, paged[i].ID, full[i].ID)
		}
	}
	if pr, pn, err := repo.Query(c, auditcontracts.ListFilter{BranchID: 1, Page: 99, Limit: 10}); err != nil || len(pr) != 0 || pn != int64(len(full)) {
		t.Fatalf("page beyond end: %d %d %v", len(pr), pn, err)
	}
	if pr, _, err := repo.Query(c, auditcontracts.ListFilter{AllBranches: true, Page: 1, Limit: total}); err != nil || len(pr) != total {
		t.Fatalf("exact-limit page: %d %v", len(pr), err)
	}
	if pr, _, err := repo.Query(c, auditcontracts.ListFilter{AllBranches: true, Page: 2, Limit: total - 1}); err != nil || len(pr) != 1 {
		t.Fatalf("last partial page: %d %v", len(pr), err)
	}
	if pr, pn, _ := repo.Query(c, auditcontracts.ListFilter{AllBranches: true, Page: 1, Limit: 1}); len(pr) != 1 || pn != int64(total) {
		t.Fatalf("limit 1: %d %d", len(pr), pn)
	}

	// Field round trip: NULL ids read 0, system actor, empty-note, details.
	recs, _ = q(auditcontracts.ListFilter{AllBranches: true, Action: "g."})
	for _, r := range recs {
		if r.EntityID != 0 || r.BranchID != 0 || r.ActorID != 0 || r.ActorType != "system" || r.Note != "  " ||
			r.Before != nil || r.After != nil || r.Metadata != nil || len(r.CreatedAt) != 19 {
			t.Fatalf("global record mismatch: %+v", r)
		}
	}
	recs, _ = q(auditcontracts.ListFilter{AllBranches: true, Action: "a.one", EntityID: 4})
	if len(recs) != 1 || recs[0].Note != "n3" || recs[0].ActorID != 21 || recs[0].BranchID != 1 || recs[0].EntityID != 4 || recs[0].ActorType != "user" {
		t.Fatalf("a.one rec: %+v", recs)
	}
	recs, _ = q(auditcontracts.ListFilter{AllBranches: true, Action: "a.late"})
	if len(recs) != 3 {
		t.Fatalf("a.late: %d", len(recs))
	}
	// newest first => metadata-only, after-only, before-only
	if recs[0].Metadata == nil || recs[0].Before != nil || recs[0].After != nil ||
		recs[1].After == nil || recs[1].Before != nil || recs[1].Metadata != nil ||
		recs[2].Before == nil || recs[2].After != nil || recs[2].Metadata != nil {
		t.Fatalf("partial detail columns: %+v", recs)
	}
	if !strings.Contains(string(recs[2].Before), `"x"`) {
		t.Fatalf("before json: %s", recs[2].Before)
	}
	recs, _ = q(auditcontracts.ListFilter{AllBranches: true, Action: "a.empty"})
	if len(recs) != 1 || recs[0].Before != nil || recs[0].After != nil || recs[0].Metadata != nil {
		t.Fatalf("empty details should store no row: %+v", recs)
	}
	var detailRows int
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM audit_log_details").Scan(&detailRows)
	if detailRows != 4 { // 3 late + 1 big
		t.Fatalf("detail rows = %d, want 4", detailRows)
	}
	recs, _ = q(auditcontracts.ListFilter{AllBranches: true, Action: "a.big"})
	if len(recs) != 1 || !strings.Contains(string(recs[0].After), "_truncated") {
		t.Fatalf("oversize marker: %+v", recs)
	}

	// Contract wrappers List / Count.
	lr, err := repo.List(c, 1, "a.o", "ent1", 4, 4)
	if err != nil || len(lr) != 4 {
		t.Fatalf("List page 2: %d %v", len(lr), err)
	}
	if lr2, err := repo.List(c, 0, "", "", 5, 0); err != nil || len(lr2) != 5 {
		t.Fatalf("List unscoped: %d %v", len(lr2), err)
	}
	if cn, err := repo.Count(c, 1, "a.o", "ent1"); err != nil || cn != 10 {
		t.Fatalf("Count: %d %v", cn, err)
	}
	if cn, err := repo.Count(c, 0, "", ""); err != nil || cn != int64(total) {
		t.Fatalf("Count unscoped: %d %v", cn, err)
	}

	// LogTx: borrowed tx; rollback drops the row.
	tx, err := testMods.db.BeginTx(c, nil)
	auditMust(t, err)
	auditMust(t, repo.LogTx(c, tx, auditcontracts.Entry{Action: "tx.rolled", Entity: "e", After: map[string]any{"a": 1}}))
	auditMust(t, tx.Rollback())
	if _, n := q(auditcontracts.ListFilter{AllBranches: true, Action: "tx."}); n != 0 {
		t.Fatalf("rolled-back LogTx visible: %d", n)
	}
	tx, _ = testMods.db.BeginTx(c, nil)
	auditMust(t, repo.LogTx(c, tx, auditcontracts.Entry{Action: "tx.kept", Entity: "e", ActorType: " system "}))
	auditMust(t, tx.Commit())
	if r, n := q(auditcontracts.ListFilter{AllBranches: true, Action: "tx."}); n != 1 || r[0].ActorType != "system" {
		t.Fatalf("committed LogTx: %d %+v", n, r)
	}

	// Purge: bounded, oldest first, default limit on <=0, cascades details.
	if _, err := testMods.db.Exec("UPDATE audit_logs SET created_at = '2001-01-01 00:00:00' WHERE action_key IN ('a.late','b.two')"); err != nil {
		t.Fatal(err)
	}
	cutoff := time.Now().UTC().AddDate(-1, 0, 0)
	gone, err := repo.Purge(c, cutoff, 4)
	if err != nil || gone != 4 {
		t.Fatalf("purge limit 4: %d %v", gone, err)
	}
	gone, err = repo.Purge(c, cutoff, 0) // default 5000
	if err != nil || gone != 7 { // 3 late + 6 b.two + a.empty + a.big (aged 2024) - 4 already purged
		t.Fatalf("purge rest: %d %v", gone, err)
	}
	gone, err = repo.Purge(c, cutoff, 10)
	if err != nil || gone != 0 {
		t.Fatalf("purge idempotent: %d %v", gone, err)
	}
	if _, n := q(auditcontracts.ListFilter{AllBranches: true, Action: "a.late"}); n != 0 {
		t.Fatalf("a.late should be purged, left %d", n)
	}
	_ = testMods.db.QueryRow("SELECT COUNT(*) FROM audit_log_details").Scan(&detailRows)
	if detailRows != 0 {
		t.Fatalf("details must cascade, left %d", detailRows)
	}
	if _, n := q(auditcontracts.ListFilter{AllBranches: true}); n != int64(total+1-11) {
		t.Fatalf("remaining rows: %d", n)
	}
}

func auditMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
