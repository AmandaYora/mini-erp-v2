package integration

import (
	"strings"
	"testing"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
)

func detailRow(t *testing.T, logID int64) (before, after, meta string, found bool) {
	t.Helper()
	var b, a, m []byte
	err := testMods.db.QueryRow(
		"SELECT before_json, after_json, metadata_json FROM audit_log_details WHERE log_id = ?", logID,
	).Scan(&b, &a, &m)
	if err != nil {
		return "", "", "", false
	}
	return string(b), string(a), string(m), true
}

func lastLogID(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := testMods.db.QueryRow("SELECT MAX(id) FROM audit_logs").Scan(&id); err != nil {
		t.Fatalf("max id: %v", err)
	}
	return id
}

// TestAuditDetailsAndActorType (F4-9): diffs land in the side table (narrow
// list rows unaffected), actor_type persists, oversize truncates with marker.
func TestAuditDetailsAndActorType(t *testing.T) {
	freshDB(t)
	svc := testMods.audit.Service()

	if err := svc.Log(ctx(), auditcontracts.Entry{
		Action: "users.update", Entity: "user", EntityID: 9, ActorID: 1, ActorType: "system",
		Before: map[string]any{"username": "lama"}, After: map[string]any{"username": "baru"},
	}); err != nil {
		t.Fatalf("log: %v", err)
	}
	id := lastLogID(t)
	var actorType string
	if err := testMods.db.QueryRow("SELECT actor_type FROM audit_logs WHERE id = ?", id).Scan(&actorType); err != nil {
		t.Fatalf("actor_type: %v", err)
	}
	if actorType != "system" {
		t.Fatalf("actor_type = %q, want system", actorType)
	}
	before, after, _, found := detailRow(t, id)
	if !found {
		t.Fatal("detail row missing")
	}
	if !strings.Contains(before, `"lama"`) || !strings.Contains(after, `"baru"`) {
		t.Fatalf("detail = %q / %q", before, after)
	}

	// Oversize diff truncates with a marker instead of failing.
	big := strings.Repeat("x", 20*1024)
	if err := svc.Log(ctx(), auditcontracts.Entry{
		Action: "t.oversize", Entity: "t", After: map[string]any{"blob": big},
	}); err != nil {
		t.Fatalf("log oversize: %v", err)
	}
	_, afterBig, _, found := detailRow(t, lastLogID(t))
	if !found || !strings.Contains(afterBig, "_truncated") {
		t.Fatalf("oversize detail = %q, want truncation marker", afterBig)
	}

	// Plain entries write no detail row.
	if err := svc.Log(ctx(), auditcontracts.Entry{Action: "t.plain", Entity: "t"}); err != nil {
		t.Fatalf("log plain: %v", err)
	}
	if _, _, _, found := detailRow(t, lastLogID(t)); found {
		t.Fatal("plain entry must not write a detail row")
	}
}

// TestAuditLogTxRollsBack (ADR-0011): a trail row written via LogTx vanishes
// with the caller's rollback, and commits with it.
func TestAuditLogTxRollsBack(t *testing.T) {
	freshDB(t)
	svc := testMods.audit.Service()

	tx, err := testMods.db.Begin()
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if err := svc.LogTx(ctx(), tx, auditcontracts.Entry{Action: "uang.jalan", Entity: "uang"}); err != nil {
		_ = tx.Rollback()
		t.Fatalf("logtx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	records, total, err := svc.List(ctx(), 0, "uang.jalan", "uang", 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 0 || len(records) != 0 {
		t.Fatalf("rolled-back trail survived: total=%d", total)
	}

	tx2, err := testMods.db.Begin()
	if err != nil {
		t.Fatalf("begin2: %v", err)
	}
	if err := svc.LogTx(ctx(), tx2, auditcontracts.Entry{Action: "uang.jalan", Entity: "uang"}); err != nil {
		_ = tx2.Rollback()
		t.Fatalf("logtx2: %v", err)
	}
	if err := tx2.Commit(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	_, total, err = svc.List(ctx(), 0, "uang.jalan", "uang", 1, 10)
	if err != nil || total != 1 {
		t.Fatalf("committed trail missing: total=%d err=%v", total, err)
	}
}
