package transform_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/etltest"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/legacydb"
	"mini-erp/internal/etl/transform"
)

func extractFixture(t *testing.T, env *etltest.Env, dir string) {
	t.Helper()
	ldb, err := legacydb.Open(env.DSN, etltest.Prefix, etltest.CompanyID)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ldb.Close() }()
	if _, err := extract.Run(context.Background(), ldb, dir, false, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
}

func testCfg(work string) *cfg.Config {
	return &cfg.Config{CompanyID: etltest.CompanyID, OpeningDate: "2026-01-01", WorkDir: work, MaxBalanceDelta: 5, RunID: "t"}
}

func runTransform(t *testing.T, work string) *transform.Output {
	t.Helper()
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	out, err := transform.Run(transform.Input{Cfg: testCfg(work), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestTransformFixture(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	for _, f := range out.Log.Findings {
		t.Logf("FINDING %s %s %s #%d %s", f.Severity, f.Code, f.Entity, f.LegacyID, f.Detail)
	}
	for _, a := range out.Log.Adjustments {
		t.Logf("ADJUST %s #%d %s %s -> %s (%s)", a.Entity, a.LegacyID, a.Field, a.Before, a.After, a.Rule)
	}
	for tbl, rows := range out.Tables {
		t.Logf("TABLE %-34s %d", tbl, len(rows))
	}
	for _, m := range out.Marks {
		t.Logf("MARK %+v", m)
	}
	for _, c := range out.Cutovers {
		t.Logf("CUT %+v", c)
	}
	t.Logf("EXPLAIN %v", out.Explain)
}

// K-04a: a customer_credit / supplier_credit settlement is journalled on its
// own in the revamp, so the ETL must (a) carry its own cutover boundary and
// (b) mark the rows of returns already booked in legacy as covered while the
// rows of pending returns stay above the boundary as pending queue items.
// Otherwise the legacy journal and the new queue would book it twice.
func TestTransformCreditSettlementCutover(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)

	through := map[string]int64{}
	for _, c := range out.Cutovers {
		if c.DocType == "sales_return_settlement" || c.DocType == "purchase_return_settlement" {
			through[c.DocType] = c.Through
		}
	}
	if through["sales_return_settlement"] != 2 {
		t.Errorf("sales_return_settlement cutover = %d, want 2 (settlement 2 belongs to a return posted in legacy)", through["sales_return_settlement"])
	}
	if v, ok := through["purchase_return_settlement"]; !ok || v != 0 {
		t.Errorf("purchase_return_settlement cutover = %d (present=%v), want an explicit 0 (fixture has no supplier_credit)", v, ok)
	}

	marks := map[int64]string{}
	for _, m := range out.Marks {
		if m.DocType == "sales_return_settlement" {
			marks[m.DocID] = m.State
		}
	}
	// Only credit rows of CONFIRMED returns are queue items: the cancelled
	// return's credit row has no mark at all.
	if len(marks) != 2 || marks[2] != "posted_legacy" || marks[3] != "pending" {
		t.Errorf("settlement marks = %v, want {2: posted_legacy, 3: pending}", marks)
	}
	// A pending mark must sit ABOVE the boundary and a covered one at/below it.
	for id, st := range marks {
		above := id > through["sales_return_settlement"]
		if (st == "pending") != above {
			t.Errorf("settlement %d state %s vs boundary %d: pending iff above", id, st, through["sales_return_settlement"])
		}
	}
}

// ADR-0018 D13: the legacy developer superadmin is migrated, stays active
// despite its seed password (other seed accounts are locked), and is linked
// to the reserved superadmin role without copying that role's legacy grants.
func TestTransformMigratesSuperadmin(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)

	userID := map[string]int64{}
	for _, u := range out.Tables["users"] {
		userID[u.Str("username")] = u.Int("id")
		switch u.Str("username") {
		case "superadmin":
			if u.Str("status") != "active" {
				t.Errorf("superadmin status = %q, want active", u.Str("status"))
			}
		case "admin":
			if u.Str("status") != "inactive" {
				t.Errorf("admin (seed password) status = %q, want inactive", u.Str("status"))
			}
		}
	}
	if _, ok := userID["superadmin"]; !ok {
		t.Fatal("superadmin user not migrated")
	}
	roleID := map[string]int64{}
	for _, r := range out.Tables["roles"] {
		roleID[r.Str("code")] = r.Int("id")
	}
	sa, ok := roleID["superadmin"]
	if !ok {
		t.Fatal("superadmin role not emitted for the loader to match by code")
	}
	linked := false
	for _, ur := range out.Tables["user_roles"] {
		if ur.Int("user_id") == userID["superadmin"] && ur.Int("role_id") == sa {
			linked = true
		}
	}
	if !linked {
		t.Error("superadmin user not linked to the superadmin role")
	}
	for _, rp := range out.Tables["role_permissions"] {
		if rp.Int("role_id") == sa {
			t.Errorf("legacy grant copied to the reserved role: %v", rp)
		}
	}
	warned := false
	for _, f := range out.Log.Findings {
		if f.Code == "USER_SUPERADMIN_SEED_PASSWORD" {
			warned = true
		}
	}
	if !warned {
		t.Error("missing USER_SUPERADMIN_SEED_PASSWORD finding")
	}
}
