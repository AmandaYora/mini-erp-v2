package pipeline_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"mini-erp/internal/etl/pipeline"
	"mini-erp/internal/etl/verify"
)

func (f *fixture) migrate(t *testing.T) {
	t.Helper()
	for _, p := range []string{"preflight", "extract", "transform", "media", "load", "sequences", "archive"} {
		f.mustRun(t, p)
	}
}

// The negative control: the verifier must fail on every deliberate damage and
// the self-test must leave the target exactly as it found it.
func TestVerifySelfTest(t *testing.T) {
	f := newFixture(t)
	f.migrate(t)
	f.expectPass(t, "baseline")
	before := f.hashes(t)
	f.cfg.SelfTest = true
	f.mustRun(t, "verify")
	f.cfg.SelfTest = false
	for tb, h := range f.hashes(t) {
		if before[tb] != h {
			t.Errorf("self-test mengubah tabel %s", tb)
		}
	}
	raw, err := os.ReadFile(filepath.Join(f.work, "report", "selftest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var res verify.SelfTestResult
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatal(err)
	}
	caught := 0
	for _, c := range res.Cases {
		switch c.Status {
		case verify.CaseCaught:
			caught++
		case verify.CaseSkipped:
			t.Logf("dilewati: %s", c.Name)
		default:
			t.Errorf("kerusakan %q lolos: gagal=%v", c.Name, c.Failed)
		}
	}
	if !res.Pass || caught < 8 {
		t.Errorf("self-test: pass=%v tertangkap=%d", res.Pass, caught)
	}
}

// Returns, balances and permissions: damage made on purpose must be noticed.
func TestVerifyReturnsAndBalances(t *testing.T) {
	f := newFixture(t)
	f.migrate(t)
	f.expectPass(t, "baseline")
	env := f.env
	cases := []struct {
		name, mutate, restore string
		want                  []string
	}{
		{"offset tidak seimbang", "UPDATE payments SET amount = amount + 5 WHERE number = 'PAY-PST/MIG/RTR00004-B'", "UPDATE payments SET amount = amount - 5 WHERE number = 'PAY-PST/MIG/RTR00004-B'", []string{"V-23f"}},
		{"sisa tagihan order berubah", "UPDATE payment_allocations a JOIN payments p ON p.id=a.payment_id SET a.amount = a.amount - 1000 WHERE p.number = 'PAY-PST/MIG/RTR00004-B'",
			"UPDATE payment_allocations a JOIN payments p ON p.id=a.payment_id SET a.amount = a.amount + 1000 WHERE p.number = 'PAY-PST/MIG/RTR00004-B'", []string{"V-08-sales", "V-09a"}},
		{"pengurangan piutang retur", "UPDATE sales_returns SET receivable_reduction = receivable_reduction + 1 WHERE number = 'RTR-PST/2026/00006'",
			"UPDATE sales_returns SET receivable_reduction = receivable_reduction - 1 WHERE number = 'RTR-PST/2026/00006'", []string{"V-23e"}},
		{"refund retur hilang", "UPDATE payment_allocations a JOIN payments p ON p.id=a.payment_id SET a.amount = a.amount + 1 WHERE p.number = 'PAY-PST/MIG/RTR00007-S4'", "UPDATE payment_allocations a JOIN payments p ON p.id=a.payment_id SET a.amount = a.amount - 1 WHERE p.number = 'PAY-PST/MIG/RTR00007-S4'", []string{"V-23d", "V-07b"}},
		{"izin peran hilang", "DELETE FROM role_permissions WHERE role_id = (SELECT id FROM roles WHERE code='owner') LIMIT 1", "", []string{"V-16e"}},
		{"pengaturan perusahaan", "UPDATE company_settings SET setting_value = 'simple' WHERE setting_key = 'stock_adjust_approval_mode'", "UPDATE company_settings SET setting_value = 'strict' WHERE setting_key = 'stock_adjust_approval_mode'", []string{"V-16f"}},
	}
	for _, c := range cases {
		if c.restore == "" {
			// restoring a deleted link needs its id: remember it first
			var role, perm int64
			if err := env.DB.QueryRow("SELECT role_id, permission_id FROM role_permissions WHERE role_id = (SELECT id FROM roles WHERE code='owner') ORDER BY permission_id LIMIT 1").Scan(&role, &perm); err != nil {
				t.Fatal(err)
			}
			env.Exec(t, "DELETE FROM role_permissions WHERE role_id = ? AND permission_id = ?", role, perm)
			f.expectFail(t, c.name, c.want...)
			env.Exec(t, "INSERT INTO role_permissions (role_id, permission_id) VALUES (?, ?)", role, perm)
			f.expectPass(t, c.name+" dipulihkan")
			continue
		}
		env.Exec(t, c.mutate)
		f.expectFail(t, c.name, c.want...)
		env.Exec(t, c.restore)
		f.expectPass(t, c.name+" dipulihkan")
	}
}

func TestDailyVerify(t *testing.T) {
	f := newFixture(t)
	f.migrate(t)
	f.mustRun(t, "verify")
	for _, day := range []string{"2026-01-15", "2026-01-16", "2026-02-02", "2026-02-11", "2026-02-20", "2026-03-10"} {
		f.cfg.Date = day
		f.mustRun(t, "daily-verify")
		raw, err := os.ReadFile(filepath.Join(f.work, "report", "daily", day+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var rep verify.DailyReport
		if err := json.Unmarshal(raw, &rep); err != nil {
			t.Fatal(err)
		}
		if !rep.Pass || len(rep.Deltas) != 0 {
			t.Errorf("%s: seharusnya tanpa selisih: %+v", day, rep.Deltas)
		}
		if rep.Keys["orders"]+rep.Keys["payments"]+rep.Keys["stock"]+rep.Keys["cash"] == 0 {
			t.Errorf("%s: tidak ada yang dibandingkan", day)
		}
	}
	// a transaction keyed into the parallel target but not into legacy: a delta, then explained
	f.env.Exec(t, "UPDATE payments SET amount = amount + 7 WHERE number = 'PAY-PST/2026/00002'")
	f.cfg.Date = "2026-01-16"
	err := f.run(t, "daily-verify")
	if !errors.Is(err, pipeline.ErrBlockers) {
		t.Fatalf("selisih tak terjelaskan harus ErrBlockers: %v", err)
	}
	raw, _ := os.ReadFile(filepath.Join(f.work, "report", "daily", "2026-01-16.json"))
	var rep verify.DailyReport
	_ = json.Unmarshal(raw, &rep)
	if rep.Unexplained == 0 || len(rep.Deltas) == 0 || rep.Deltas[0].Section != "payments" {
		t.Fatalf("delta pembayaran tidak terdeteksi: %+v", rep)
	}
	// second run of the same day: no NEW deltas
	_ = f.run(t, "daily-verify")
	raw, _ = os.ReadFile(filepath.Join(f.work, "report", "daily", "2026-01-16.json"))
	rep = verify.DailyReport{}
	_ = json.Unmarshal(raw, &rep)
	if len(rep.New) != 0 {
		t.Errorf("lari ulang tidak boleh melaporkan selisih baru: %v", rep.New)
	}
	explain := filepath.Join(t.TempDir(), "explain.csv")
	var lines []string
	for _, d := range rep.Deltas {
		lines = append(lines, d.Section+"|"+d.Key+",operator menginput ulang")
	}
	if err := os.WriteFile(explain, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.cfg.ExplainFile = explain
	f.mustRun(t, "daily-verify")
	f.env.Exec(t, "UPDATE payments SET amount = amount - 7 WHERE number = 'PAY-PST/2026/00002'")
	f.cfg.ExplainFile = ""
	f.mustRun(t, "daily-verify")
	raw, _ = os.ReadFile(filepath.Join(f.work, "report", "daily", "2026-01-16.json"))
	rep = verify.DailyReport{}
	_ = json.Unmarshal(raw, &rep)
	if len(rep.Resolved) == 0 {
		t.Errorf("selisih yang sudah hilang harus dilaporkan: %+v", rep)
	}
	// invalid date is a usage error
	f.cfg.Date = "16-01-2026"
	if err := f.run(t, "daily-verify"); err == nil || errors.Is(err, pipeline.ErrBlockers) {
		t.Errorf("tanggal salah harus galat penggunaan: %v", err)
	}
}

func TestExportSince(t *testing.T) {
	f := newFixture(t)
	f.cfg.Since = "2026-02-15 00:00:00"
	cursor := filepath.Join(t.TempDir(), "cursor.json")
	f.cfg.CursorFile = cursor
	f.mustRun(t, "export-since")
	raw, err := os.ReadFile(filepath.Join(f.work, "delta", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var man pipeline.DeltaManifest
	if err := json.Unmarshal(raw, &man); err != nil {
		t.Fatal(err)
	}
	// expected: legacy rows changed after the timestamp, counted straight from the database
	count := func(table, cols string) int64 {
		var n int64
		q := "SELECT COUNT(*) FROM legacy_" + table + " WHERE " + cols
		q = strings.ReplaceAll(q, "@@CO@@", "id_"+"company")
		if err := f.env.DB.QueryRow(q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	ts := "'2026-02-15 00:00:00'"
	if want, got := count("sales_returns", "id_"+"company = 1 AND (updated_at > "+ts+" OR created_at > "+ts+" OR cancelled_at > "+ts+" OR archived_at > "+ts+")"), man.Tables["sales_returns"].Rows; want != got || want == 0 {
		t.Errorf("sales_returns: ekspor %d, harapan %d", got, want)
	}
	if want, got := count("audit_logs", "id_"+"company = 1 AND happened_at > "+ts), man.Tables["audit_logs"].Rows; want != got || want == 0 {
		t.Errorf("audit_logs: ekspor %d, harapan %d", got, want)
	}
	if _, ok := man.Tables["role_permissions"]; ok {
		t.Error("tabel penghubung tanpa kolom perubahan tidak boleh diekspor")
	}
	if man.Skipped["role_permissions"] == "" {
		t.Error("tabel yang tak dapat dilacak harus dilaporkan")
	}
	if _, err := os.Stat(filepath.Join(f.work, "delta", "sales_returns.jsonl.gz")); err != nil {
		t.Errorf("berkas delta tidak ada: %v", err)
	}
	// the cursor now sits at the top: a run with the cursor alone finds nothing new
	var cur map[string]int64
	craw, err := os.ReadFile(cursor)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(craw, &cur); err != nil || cur["sales_returns"] != 8 || cur["audit_logs"] != 9 {
		t.Fatalf("kursor: %v %v", cur, err)
	}
	f.cfg.Since = ""
	f.mustRun(t, "export-since")
	raw, _ = os.ReadFile(filepath.Join(f.work, "delta", "manifest.json"))
	man = pipeline.DeltaManifest{}
	_ = json.Unmarshal(raw, &man)
	if len(man.Tables) != 0 {
		t.Errorf("kursor di puncak: tidak boleh ada baris baru, dapat %v", man.Rows())
	}
	// a row appended after the cursor is the only one exported
	f.env.Exec(t, `INSERT INTO legacy_audit_logs (id_audit_log, @@CO@@, id_branch, actor_type, id_actor, action_key, entity_type, id_entity, before_json, after_json, metadata_json, happened_at)
		VALUES (10, 1, 1, 'user', 2, 'order.update', 'order', 1, NULL, NULL, NULL, '2026-03-20 03:00:00.000000')`)
	f.mustRun(t, "export-since")
	raw, _ = os.ReadFile(filepath.Join(f.work, "delta", "manifest.json"))
	man = pipeline.DeltaManifest{}
	_ = json.Unmarshal(raw, &man)
	if got := man.Rows(); len(got) != 1 || got["audit_logs"] != 1 {
		t.Errorf("hanya satu baris baru yang boleh diekspor: %v", got)
	}
	// neither a timestamp nor a cursor is a usage error
	f.cfg.CursorFile = ""
	if err := f.run(t, "export-since"); err == nil || errors.Is(err, pipeline.ErrBlockers) {
		t.Errorf("tanpa --since/--cursor-file harus galat penggunaan: %v", err)
	}
	f.cfg.Since = "kemarin"
	if err := f.run(t, "export-since"); err == nil {
		t.Error("--since tak terbaca harus ditolak")
	}
}

func TestRehearse(t *testing.T) {
	f := newFixture(t)
	f.cfg.AllowTargetDB = f.env.DBName
	// the target name guard: a plain *_test database is refused without the explicit allowance
	refused := *f.cfg
	refused.AllowTargetDB = ""
	if _, err := pipeline.Rehearse(context.Background(), &refused, quiet); err == nil || !strings.Contains(err.Error(), "_rehearsal") {
		t.Errorf("rehearse ke database tanpa akhiran _rehearsal harus ditolak: %v", err)
	}
	first, err := pipeline.Rehearse(context.Background(), f.cfg, quiet)
	if err != nil || !first.Pass {
		t.Fatalf("gladi pertama: pass=%v err=%v", first != nil && first.Pass, err)
	}
	names := map[string]bool{}
	for _, p := range first.Phases {
		names[p.Phase] = true
		if p.Status != "LULUS" {
			t.Errorf("fase %s = %s", p.Phase, p.Status)
		}
	}
	for _, want := range []string{"preflight", "extract", "transform", "load", "sequences", "verify"} {
		if !names[want] {
			t.Errorf("fase %s tidak ada di laporan", want)
		}
	}
	for _, name := range []string{"rehearsal.json", "rehearsal.md"} {
		if _, err := os.Stat(filepath.Join(f.work, "report", name)); err != nil {
			t.Errorf("%s tidak ditulis: %v", name, err)
		}
	}
	if first.ManifestDigest == "" || first.TotalSeconds <= 0 {
		t.Errorf("digest/total kosong: %+v", first)
	}
	// L2: a fresh database and work dir, same dump -> identical output
	f.env.TruncateTarget(t)
	second := *f.cfg
	second.WorkDir = t.TempDir()
	second.CompareWith = f.work
	second.MediaDst = t.TempDir()
	r2, err := pipeline.Rehearse(context.Background(), &second, quiet)
	if err != nil || !r2.Pass || r2.Compare == nil || !r2.Compare.Identical {
		t.Fatalf("gladi kedua: err=%v compare=%+v", err, r2.Compare)
	}
	if r2.ManifestDigest != first.ManifestDigest {
		t.Error("digest L1 != L2")
	}
	// a changed dump is caught by the comparison
	f.env.TruncateTarget(t)
	f.env.Exec(t, "UPDATE legacy_orders SET notes = 'diubah' WHERE id_order = 1")
	third := second
	third.WorkDir = t.TempDir()
	third.MediaDst = t.TempDir()
	r3, err := pipeline.Rehearse(context.Background(), &third, quiet)
	if err == nil || r3 == nil || r3.Compare == nil || r3.Compare.Identical {
		t.Errorf("dump berbeda harus terdeteksi: err=%v", err)
	}
}
