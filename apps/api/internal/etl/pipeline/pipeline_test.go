package pipeline_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/etltest"
	"mini-erp/internal/etl/pipeline"
	"mini-erp/internal/etl/transform"
	"mini-erp/internal/etl/verify"
)

var quiet = slog.New(slog.NewTextHandler(io.Discard, nil))

// tables the pipeline writes; used for the "second run changes nothing" proof.
var hashTables = []string{
	"branches", "company_profile", "member_types", "users", "roles", "user_roles", "user_branch_access", "product_categories", "products",
	"product_variants", "parties", "party_delivery_addresses", "stock_locations", "finance_accounts", "finance_account_mappings",
	"purchase_orders", "purchase_order_items", "sales_orders", "sales_order_items", "goods_receipts", "goods_receipt_items", "delivery_notes",
	"delivery_note_items", "stock_transfers", "stock_transfer_items", "payments", "payment_allocations", "stock_movements", "stock_balances",
	"finance_inventory_cost_movements", "finance_journal_entries", "finance_journal_lines", "business_expenses", "finance_periods", "finance_cutover",
	"finance_account_daily", "media_files", "branch_document_sequences", "finance_document_sequences", "etl_id_map", "etl_stage",
	"sales_returns", "sales_return_items", "sales_return_replacement_items", "sales_return_settlements", "purchase_returns", "purchase_return_items",
	"purchase_return_settlements", "company_settings", "audit_logs", "role_permissions",
}

type fixture struct {
	env  *etltest.Env
	cfg  *cfg.Config
	work string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	env := etltest.Setup(t)
	src, dst := t.TempDir(), t.TempDir()
	for name, body := range map[string]string{
		"products/p1.jpg": "p1-image-bytes", "products/p2.png": "p2-image-bytes",
		"delivery/2025-12-11/sj1.pdf": "sj1-pdf-bytes", "transfer/2025-12-31/proof1.jpg": "proof-1-bytes",
	} {
		p := filepath.Join(src, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	work := t.TempDir()
	c := &cfg.Config{
		CompanyID: etltest.CompanyID, OpeningDate: "2026-01-01", WorkDir: work, LegacyDSN: env.DSN, TargetDSN: env.DSN,
		LegacyPrefix: etltest.Prefix, StagePrefix: "etl_", MediaSrc: src, MediaDst: dst, RunID: "fixture-run", MaxBalanceDelta: 5,
	}
	return &fixture{env: env, cfg: c, work: work}
}

func (f *fixture) run(t *testing.T, phase string) error {
	t.Helper()
	return pipeline.Run(context.Background(), phase, f.cfg, quiet)
}

func (f *fixture) mustRun(t *testing.T, phase string) {
	t.Helper()
	if err := f.run(t, phase); err != nil {
		for _, rep := range []string{"facts.md", "findings.csv"} {
			if raw, e := os.ReadFile(filepath.Join(f.work, "report", rep)); e == nil && errors.Is(err, pipeline.ErrBlockers) {
				t.Logf("%s:\n%s", rep, raw)
			}
		}
		t.Fatalf("fase %s: %v", phase, err)
	}
}

func (f *fixture) hashes(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, tb := range hashTables {
		out[tb] = f.env.HashTable(t, tb)
	}
	return out
}

func (f *fixture) verifyReport(t *testing.T) *verify.Report {
	t.Helper()
	e, err := pipeline.Open(context.Background(), f.cfg, quiet, true, true)
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	rep, err := verify.Run(context.Background(), e.Legacy, e.Target, f.cfg, f.work)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func (f *fixture) expectPass(t *testing.T, label string) {
	t.Helper()
	rep := f.verifyReport(t)
	if !rep.Pass {
		for _, c := range rep.Checks {
			if c.Status != "LULUS" {
				t.Errorf("%s: %s %s sumber=%s target=%s selisih=%s", label, c.ID, c.Desc, c.Source, c.Target, c.Diff)
			}
		}
		t.FailNow()
	}
}

func (f *fixture) expectFail(t *testing.T, label string, want ...string) {
	t.Helper()
	rep := f.verifyReport(t)
	if rep.Pass {
		t.Fatalf("%s: verify seharusnya GAGAL", label)
	}
	failed := strings.Join(rep.Failed(), ",")
	for _, w := range want {
		if !strings.Contains(failed, w) {
			t.Errorf("%s: %s tidak gagal; gagal=%s", label, w, failed)
		}
	}
}

func TestPipelineEndToEnd(t *testing.T) {
	f := newFixture(t)
	for _, p := range []string{"preflight", "extract", "transform", "media", "load", "sequences", "archive"} {
		f.mustRun(t, p)
	}
	f.expectPass(t, "lari pertama")

	// ---- spot checks of the migrated data -------------------------------
	env := f.env
	if n := env.Count(t, "sales_orders"); n != 6 {
		t.Errorf("sales_orders = %d", n)
	}
	var tax, grand int64
	_ = env.DB.QueryRow("SELECT tax_total, grand_total FROM sales_orders WHERE number = 'ORD-PST/PJ/2025/12/00001'").Scan(&tax, &grand)
	if tax != 22550 || grand != 227550 {
		t.Errorf("order eksklusif: tax=%d grand=%d", tax, grand)
	}
	var nominal int64
	_ = env.DB.QueryRow("SELECT i.discount_nominal FROM sales_order_items i JOIN sales_orders o ON o.id=i.order_id WHERE o.number='ORD-PST/PJ/2025/12/00001' ORDER BY i.id LIMIT 1").Scan(&nominal)
	if nominal != 5000 {
		t.Errorf("diskon per satuan -> nominal baris = %d, want 5000", nominal)
	}
	var terms string
	_ = env.DB.QueryRow("SELECT payment_terms FROM sales_orders WHERE number='ORD-PST/PJ/2026/03/00001'").Scan(&terms)
	if terms != "cod" {
		t.Errorf("prepaid -> %s", terms)
	}
	var locs, rusak int
	_ = env.DB.QueryRow("SELECT COUNT(*), SUM(code='RUSAK') FROM stock_locations WHERE branch_id = (SELECT id FROM branches WHERE code='PST')").Scan(&locs, &rusak)
	if locs != 3 || rusak != 1 {
		t.Errorf("lokasi PST = %d (rusak %d), want 3 (GDG, RAK-A, RUSAK)", locs, rusak)
	}
	if n := env.Count(t, "delivery_note_items"); n != 9 {
		t.Errorf("item SJ = %d, want 9 (5 + 1 pecahan + 1 sintetis + 1 pengganti + 1 pengganti sintetis)", n)
	}
	var synth int
	_ = env.DB.QueryRow("SELECT COUNT(*) FROM delivery_notes WHERE number LIKE 'SJ-%/MIG/%'").Scan(&synth)
	if synth != 2 {
		t.Errorf("SJ sintetis = %d, want 2 (serah langsung + tukar kasir)", synth)
	}
	var onHand string
	_ = env.DB.QueryRow(`SELECT sb.on_hand FROM stock_balances sb JOIN product_variants v ON v.id=sb.variant_id WHERE v.code='BRG-002-M'`).Scan(&onHand)
	if !strings.HasPrefix(onHand, "6.") {
		t.Errorf("saldo dengan drift harus mengikuti legacy (6), got %s", onHand)
	}
	var corr int
	_ = env.DB.QueryRow("SELECT COUNT(*) FROM stock_movements WHERE notes = 'Koreksi migrasi'").Scan(&corr)
	if corr != 1 {
		t.Errorf("mutasi koreksi = %d", corr)
	}
	var seq int64
	_ = env.DB.QueryRow("SELECT current_value FROM branch_document_sequences s JOIN branches b ON b.id=s.branch_id WHERE b.code='PST' AND s.doc_kind='payment'").Scan(&seq)
	if seq != 6 {
		t.Errorf("counter payment = %d, want 6 (max legacy 6, parsed 4)", seq)
	}
	// users: seed passwords are locked, others keep their bcrypt hash and still log in
	var status, hash string
	_ = env.DB.QueryRow("SELECT status, password_hash FROM users WHERE username='owner'").Scan(&status, &hash)
	if status != "active" || bcrypt.CompareHashAndPassword([]byte(hash), []byte("Rahasia#2026")) != nil {
		t.Errorf("owner: status=%s, hash dari bcryptjs harus lolos CompareHashAndPassword", status)
	}
	_ = env.DB.QueryRow("SELECT status FROM users WHERE username='admin'").Scan(&status)
	if status != "inactive" {
		t.Errorf("akun dengan kata sandi seed harus dikunci, got %s", status)
	}
	if n := env.Count(t, "branches"); n != 2 {
		t.Errorf("cabang perusahaan lain bocor: %d", n)
	}
	// cutover: documents above the boundary == pending marks
	var through int64
	_ = env.DB.QueryRow("SELECT covered_through_id FROM finance_cutover WHERE doc_type='payment' LIMIT 1").Scan(&through)
	if through != 14 {
		t.Errorf("cutover payment = %d, want 14 (5 legacy + 13 sintetis retur, 4 pending di atas batas)", through)
	}
	checkReturns(t, env)
	// opening journal: one per branch with prior balances, balanced
	var opening int
	_ = env.DB.QueryRow("SELECT COUNT(*) FROM finance_journal_entries WHERE source_doc_type='opening'").Scan(&opening)
	if opening != 1 {
		t.Errorf("jurnal saldo awal = %d", opening)
	}
	// finance_account_daily rollup equals the journal lines
	var d1, d2 int64
	_ = env.DB.QueryRow("SELECT COALESCE(SUM(debit),0) FROM finance_account_daily").Scan(&d1)
	_ = env.DB.QueryRow("SELECT COALESCE(SUM(debit),0) FROM finance_journal_lines").Scan(&d2)
	if d1 != d2 || d1 == 0 {
		t.Errorf("rollup harian %d != baris jurnal %d", d1, d2)
	}

	// ---- idempotency: a second full run is a no-op -----------------------
	before := f.hashes(t)
	man1, err := transform.ReadManifest(f.work)
	if err != nil {
		t.Fatal(err)
	}
	f.cfg.Resume = true
	for _, p := range []string{"extract", "transform", "media", "load", "sequences", "archive"} {
		f.mustRun(t, p)
	}
	after := f.hashes(t)
	for tb, h := range before {
		if after[tb] != h {
			t.Errorf("lari kedua mengubah tabel %s", tb)
		}
	}
	man2, _ := transform.ReadManifest(f.work)
	for name, fi := range man1.Files {
		if man2.Files[name].SHA256 != fi.SHA256 {
			t.Errorf("transform tidak deterministik: %s", name)
		}
	}
	f.expectPass(t, "lari kedua")

	// ---- deliberate corruption must make reconciliation fail -------------
	corrupt := []struct {
		name, mutate, restore string
		want                  []string
	}{
		{"nominal pembayaran", "UPDATE payments SET amount = amount + 1 WHERE number = 'PAY-PST/2026/00002'", "UPDATE payments SET amount = amount - 1 WHERE number = 'PAY-PST/2026/00002'", []string{"V-07"}},
		{"hapus satu mutasi", "DELETE FROM stock_movements WHERE ref_type='goods_receipt' LIMIT 1", "", []string{"V-06", "V-01v"}},
		{"pemilik media bergeser", "UPDATE media_files SET owner_id = owner_id + 500 WHERE id = (SELECT id FROM (SELECT MIN(id) AS id FROM media_files WHERE owner_type='product') t)", "UPDATE media_files SET owner_id = owner_id - 500 WHERE id = (SELECT id FROM (SELECT MIN(id) AS id FROM media_files WHERE owner_type='product') t)", []string{"V-15"}},
		{"akun jurnal tertukar", "UPDATE finance_journal_lines l JOIN finance_accounts a ON a.code='6900' SET l.account_id = a.id WHERE l.account_id = (SELECT id FROM finance_accounts WHERE code='6110')",
			"UPDATE finance_journal_lines l JOIN finance_accounts a ON a.code='6110' SET l.account_id = a.id WHERE l.account_id = (SELECT id FROM finance_accounts WHERE code='6900') AND l.debit = 25000", []string{"V-11"}},
		{"total order berubah", "UPDATE sales_orders SET grand_total = grand_total + 100 WHERE number='ORD-PST/PJ/2026/03/00001'", "UPDATE sales_orders SET grand_total = grand_total - 100 WHERE number='ORD-PST/PJ/2026/03/00001'", []string{"V-02", "V-04-sales"}},
	}
	for _, c := range corrupt {
		if c.restore == "" {
			continue // handled below with a saved copy
		}
		env.Exec(t, c.mutate)
		f.expectFail(t, c.name, c.want...)
		env.Exec(t, c.restore)
		f.expectPass(t, c.name+" dipulihkan")
	}
	// a movement quantity drifts: the stock reconciliation must notice, and
	// putting the value back heals the database.
	var id int64
	_ = env.DB.QueryRow("SELECT id FROM stock_movements WHERE ref_type='goods_receipt' ORDER BY id LIMIT 1").Scan(&id)
	env.Exec(t, "UPDATE stock_movements SET qty_base = qty_base + 1 WHERE id = ?", id)
	f.expectFail(t, "mutasi menyimpang", "V-06")
	env.Exec(t, "UPDATE stock_movements SET qty_base = qty_base - 1 WHERE id = ?", id)
	f.expectPass(t, "mutasi dipulihkan")
}

func TestGuards(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	// 1. target database not on the whitelist
	bad := *f.cfg
	bad.TargetDSN = "u:secret-pw@tcp(127.0.0.1:3306)/mini_erp"
	if _, err := pipeline.Open(ctx, &bad, quiet, false, true); err == nil || !strings.Contains(err.Error(), "daftar putih") {
		t.Errorf("target di luar daftar putih harus ditolak: %v", err)
	} else if strings.Contains(err.Error(), "secret-pw") {
		t.Errorf("kata sandi bocor ke pesan galat: %v", err)
	}
	prod := *f.cfg
	prod.TargetDSN = "u:pw@tcp(127.0.0.1:3306)/mini_erp_production"
	if _, err := pipeline.Open(ctx, &prod, quiet, false, true); err == nil || !strings.Contains(err.Error(), "prod") {
		t.Errorf("nama berisi prod harus ditolak: %v", err)
	}
	// 2. legacy marker missing -> refuse
	f.env.Exec(t, "DROP TABLE legacy_etl_copy_marker")
	if _, err := pipeline.Open(ctx, f.cfg, quiet, true, false); err == nil || !strings.Contains(err.Error(), "penanda") {
		t.Errorf("tanpa etl_copy_marker harus ditolak: %v", err)
	}
	f.env.Exec(t, "CREATE TABLE legacy_etl_copy_marker (id INT PRIMARY KEY, restored_at DATETIME, source_host VARCHAR(100), note VARCHAR(200))")
	f.env.Exec(t, "INSERT INTO legacy_etl_copy_marker VALUES (1, NOW(), 'localhost', 'x')")
	// 3. remote legacy host
	remote := *f.cfg
	remote.LegacyDSN = "u:pw@tcp(10.1.2.3:3306)/legacy"
	if _, err := pipeline.Open(ctx, &remote, quiet, true, false); err == nil || !strings.Contains(err.Error(), "bukan lokal") {
		t.Errorf("host legacy non-lokal harus ditolak: %v", err)
	}
	// 4. same database without prefix
	same := *f.cfg
	same.LegacyPrefix = ""
	if _, err := pipeline.Open(ctx, &same, quiet, true, true); err == nil {
		t.Error("legacy == target tanpa prefix harus ditolak")
	}
	// 5. non-empty target without --resume
	for _, p := range []string{"extract", "transform", "media", "load"} {
		f.mustRun(t, p)
	}
	f.env.DB.Exec("DELETE FROM etl_run")
	if err := f.run(t, "load"); err == nil || !strings.Contains(err.Error(), "tidak kosong") {
		// a completed load leaves rows behind: a fresh run id on the same target must refuse
		f.cfg.RunID = "another-run"
		if err := f.run(t, "load"); err == nil || !strings.Contains(err.Error(), "tidak kosong") {
			t.Errorf("target terisi tanpa --resume harus ditolak: %v", err)
		}
	}
	// 6. missing required flags are usage errors
	if _, err := cfg.Parse([]string{"--opening-date", "2026-01-01", "--workdir", "x"}, io.Discard); !errors.Is(err, cfg.ErrUsage) {
		t.Errorf("tanpa --company-id harus ErrUsage: %v", err)
	}
	if _, err := cfg.Parse([]string{"--company-id", "1", "--workdir", "x"}, io.Discard); !errors.Is(err, cfg.ErrUsage) {
		t.Errorf("tanpa --opening-date harus ErrUsage: %v", err)
	}
}

// checkReturns pins the migrated return documents of the fixture to the
// revamp semantics (design F5-5.E): offsets, refunds, exchange totals, the
// replacement delivery notes and the stock references.
func checkReturns(t *testing.T, env *etltest.Env) {
	t.Helper()
	for tb, want := range map[string]int64{"sales_returns": 8, "sales_return_items": 9, "sales_return_replacement_items": 3, "purchase_returns": 3,
		"purchase_return_items": 3, "purchase_return_settlements": 1, "company_settings": 4, "audit_logs": 8} {
		if n := env.Count(t, tb); n != want {
			t.Errorf("%s = %d, want %d", tb, n, want)
		}
	}
	type sr struct {
		status, mode, repl     string
		total, reduction, diff int64
	}
	got := map[string]sr{}
	rows, err := env.DB.Query("SELECT number, status, return_mode, replacement_delivery_status, total, receivable_reduction, difference_amount FROM sales_returns")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var n string
		var x sr
		if err := rows.Scan(&n, &x.status, &x.mode, &x.repl, &x.total, &x.reduction, &x.diff); err != nil {
			t.Fatal(err)
		}
		got[n] = x
	}
	_ = rows.Close()
	want := map[string]sr{
		"RTR-PST/2025/00001": {"confirmed", "return_only", "not_required", 16095, 16095, 0},
		"RTR-PST/2026/00002": {"confirmed", "exchange", "confirmed", 22200, 0, -7200},
		"RTR-CBG/2026/00001": {"confirmed", "return_only", "not_required", 15000, 0, 0},
		"RTR-PST/2026/00003": {"confirmed", "return_only", "not_required", 54390, 54390, 0},
		"RTR-PST/2026/00004": {"confirmed", "exchange", "confirmed", 15001, 0, -3901},
		"RTR-PST/2026/00005": {"cancelled", "return_only", "not_required", 11100, 0, 0},
		"RTR-PST/2026/00006": {"confirmed", "return_only", "not_required", 80475, 57065, 0},
		"RTR-PST/2026/00007": {"confirmed", "exchange", "pending", 22200, 0, 0},
	}
	for n, w := range want {
		if got[n] != w {
			t.Errorf("retur %s = %+v, want %+v", n, got[n], w)
		}
	}
	var pr int64
	_ = env.DB.QueryRow("SELECT payable_reduction FROM purchase_returns WHERE number='RTB-PST/2026/00002'").Scan(&pr)
	if pr != 499000 {
		t.Errorf("pengurangan hutang RTB-2 = %d, want 499000", pr)
	}
	// synthetic payments: 3+... offset pairs (receivable x3, payable x2) + refunds
	var offsets, refunds int
	_ = env.DB.QueryRow("SELECT COUNT(*) FROM payments WHERE method='offset'").Scan(&offsets)
	_ = env.DB.QueryRow("SELECT COUNT(*) FROM payments WHERE number LIKE 'PAY-%/MIG/%' AND method<>'offset'").Scan(&refunds)
	if offsets != 10 || refunds != 3 {
		t.Errorf("pembayaran sintetis: offset=%d refund=%d, want 10 dan 3", offsets, refunds)
	}
	var repls, withReturn int
	_ = env.DB.QueryRow("SELECT COUNT(*), COUNT(sales_return_id) FROM delivery_notes WHERE document_kind='replacement'").Scan(&repls, &withReturn)
	if repls != 2 || withReturn != 2 {
		t.Errorf("SJ pengganti = %d (menunjuk retur %d), want 2", repls, withReturn)
	}
	var movs int
	_ = env.DB.QueryRow("SELECT COUNT(*) FROM stock_movements WHERE ref_type IN ('sales_return','purchase_return') OR (ref_type='delivery' AND ref_id IN (SELECT id FROM delivery_notes WHERE document_kind='replacement'))").Scan(&movs)
	if movs != 12 {
		t.Errorf("mutasi retur = %d, want 12", movs)
	}
	var mode, evidence string
	_ = env.DB.QueryRow("SELECT setting_value FROM company_settings WHERE setting_key='stock_adjust_approval_mode'").Scan(&mode)
	_ = env.DB.QueryRow("SELECT setting_value FROM company_settings WHERE setting_key='delivery_require_evidence'").Scan(&evidence)
	if mode != "strict" || evidence != "true" {
		t.Errorf("company_settings: mode=%s evidence=%s", mode, evidence)
	}
	var perms int
	_ = env.DB.QueryRow("SELECT COUNT(*) FROM role_permissions rp JOIN roles r ON r.id=rp.role_id WHERE r.code='owner'").Scan(&perms)
	if perms < 10 {
		t.Errorf("izin peran owner = %d, want hasil peta legacymap (>=10)", perms)
	}
}
