package verify

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/legacydb"
	"mini-erp/internal/etl/transform"
)

// Self-test statuses.
const (
	CaseCaught  = "TERTANGKAP"
	CaseMissed  = "LOLOS (verifier tumpul)"
	CaseSkipped = "DILEWATI (tidak ada baris untuk dirusak)"
)

// CaseResult is one deliberate corruption and what the verifier said.
type CaseResult struct {
	Name   string   `json:"name"`
	Expect []string `json:"expect"`
	Failed []string `json:"failed"`
	Status string   `json:"status"`
}

// SelfTestResult is the verdict of the negative control (design V-25).
type SelfTestResult struct {
	Baseline bool         `json:"baseline_pass"`
	Cases    []CaseResult `json:"cases"`
	Pass     bool         `json:"pass"`
}

type corruption struct {
	name   string
	expect []string // at least one of these checks must fail
	apply  func(ctx context.Context, tx *sql.Tx) (bool, error)
}

func execOne(ctx context.Context, tx *sql.Tx, q string, args ...any) (bool, error) {
	res, err := tx.ExecContext(ctx, q, args...)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func scanInt(ctx context.Context, tx *sql.Tx, q string, args ...any) (int64, bool, error) {
	var v sql.NullInt64
	if err := tx.QueryRowContext(ctx, q, args...).Scan(&v); err != nil {
		return 0, false, err
	}
	return v.Int64, v.Valid, nil
}

func corruptions() []corruption {
	return []corruption{
		{"nominal pembayaran +1", []string{"V-07"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM payments WHERE number NOT LIKE 'PAY-%/MIG/%' AND method <> 'offset'")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "UPDATE payments SET amount = amount + 1 WHERE id = ?", id)
		}},
		{"alokasi pembayaran +1", []string{"V-07b", "V-08-sales", "V-08-purchase", "V-09a"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM payment_allocations WHERE order_type IN ('sales','purchase')")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "UPDATE payment_allocations SET amount = amount + 1 WHERE id = ?", id)
		}},
		{"hapus satu mutasi stok", []string{"V-06", "V-01v", "V-05"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM stock_movements")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "DELETE FROM stock_movements WHERE id = ?", id)
		}},
		{"saldo stok +1", []string{"V-05", "V-06"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM stock_balances")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "UPDATE stock_balances SET on_hand = on_hand + 1 WHERE id = ?", id)
		}},
		{"pemilik media bergeser", []string{"V-15"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM media_files WHERE origin = 'legacy'")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "UPDATE media_files SET owner_id = owner_id + 500000 WHERE id = ?", id)
		}},
		{"akun baris jurnal tertukar", []string{"V-11"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			var lineID, acct int64
			err := tx.QueryRowContext(ctx, "SELECT id, account_id FROM finance_journal_lines WHERE debit + credit > 0 ORDER BY id LIMIT 1").Scan(&lineID, &acct)
			if err == sql.ErrNoRows {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			other, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM finance_accounts WHERE id <> ?", acct)
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "UPDATE finance_journal_lines SET account_id = ? WHERE id = ?", other, lineID)
		}},
		{"total order +100", []string{"V-02", "V-04-sales", "V-04-purchase"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM sales_orders")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "UPDATE sales_orders SET grand_total = grand_total + 100 WHERE id = ?", id)
		}},
		{"total retur penjualan +100", []string{"V-23a", "V-23g"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM sales_returns")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "UPDATE sales_returns SET total = total + 100 WHERE id = ?", id)
		}},
		{"pengurangan piutang retur diubah", []string{"V-23e"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM sales_returns WHERE receivable_reduction > 0")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "UPDATE sales_returns SET receivable_reduction = receivable_reduction + 1 WHERE id = ?", id)
		}},
		{"jumlah audit log berkurang", []string{"V-01ah"}, func(ctx context.Context, tx *sql.Tx) (bool, error) {
			id, ok, err := scanInt(ctx, tx, "SELECT MIN(id) FROM audit_logs")
			if err != nil || !ok {
				return false, err
			}
			return execOne(ctx, tx, "DELETE FROM audit_logs WHERE id = ?", id)
		}},
	}
}

// SelfTest runs the verification once on the real target (it must pass), then
// repeats it against the same target inside a transaction in which one value
// has been damaged, and requires the matching checks to fail. Every damage is
// rolled back: the target is never changed. A damage the verifier does not
// notice makes the whole self-test fail (a blunt verifier is worse than none).
func SelfTest(ctx context.Context, ldb *legacydb.DB, target *sql.DB, c *cfg.Config, work string) (*SelfTestResult, error) {
	base, err := Run(ctx, ldb, target, c, work)
	if err != nil {
		return nil, err
	}
	res := &SelfTestResult{Baseline: base.Pass, Pass: base.Pass}
	if !base.Pass {
		return res, fmt.Errorf("verify dasar GAGAL pada %s: self-test hanya bermakna pada target yang lulus", strings.Join(base.Failed(), ", "))
	}
	for _, cs := range corruptions() {
		cr := CaseResult{Name: cs.name, Expect: cs.expect}
		tx, err := target.BeginTx(ctx, nil)
		if err != nil {
			return nil, err
		}
		changed, err := cs.apply(ctx, tx)
		if err != nil {
			_ = tx.Rollback()
			return nil, fmt.Errorf("self-test %q: %w", cs.name, err)
		}
		if !changed {
			_ = tx.Rollback()
			cr.Status = CaseSkipped
			res.Cases = append(res.Cases, cr)
			continue
		}
		rep, err := Run(ctx, ldb, tx, c, work)
		_ = tx.Rollback()
		if err != nil {
			return nil, err
		}
		cr.Failed = rep.Failed()
		cr.Status = CaseMissed
		for _, want := range cs.expect {
			for _, got := range cr.Failed {
				if got == want {
					cr.Status = CaseCaught
				}
			}
		}
		if cr.Status != CaseCaught {
			res.Pass = false
		}
		res.Cases = append(res.Cases, cr)
	}
	return res, nil
}

// WriteSelfTest stores report/selftest.json and selftest.md.
func WriteSelfTest(work string, r *SelfTestResult) error {
	dir := transform.ReportDir(work)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "selftest.json"), raw, 0o644); err != nil {
		return err
	}
	var sb strings.Builder
	sb.WriteString("# Self-test verifikasi (V-25)\n\n| kerusakan sengaja | harus gagal | gagal | status |\n|---|---|---|---|\n")
	for _, c := range r.Cases {
		fmt.Fprintf(&sb, "| %s | %s | %s | %s |\n", c.Name, strings.Join(c.Expect, ", "), strings.Join(c.Failed, ", "), c.Status)
	}
	if r.Pass {
		sb.WriteString("\n**LULUS**: setiap kerusakan terdeteksi.\n")
	} else {
		sb.WriteString("\n**GAGAL**\n")
	}
	return os.WriteFile(filepath.Join(dir, "selftest.md"), []byte(sb.String()), 0o644)
}

// Missed lists the corruptions the verifier did not catch.
func (r *SelfTestResult) Missed() []string {
	var out []string
	for _, c := range r.Cases {
		if c.Status == CaseMissed {
			out = append(out, c.Name)
		}
	}
	return out
}
