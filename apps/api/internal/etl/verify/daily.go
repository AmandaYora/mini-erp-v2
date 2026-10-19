package verify

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"mini-erp/internal/etl/cfg"
	"mini-erp/internal/etl/legacydb"
	"mini-erp/internal/etl/transform"

	"context"
)

// Delta is one difference between the legacy day aggregate and the target.
type Delta struct {
	Section   string `json:"section"`
	Key       string `json:"key"`
	Legacy    string `json:"legacy"`
	Target    string `json:"target"`
	Explained string `json:"explained,omitempty"`
}

// DailyReport is the parallel-run reconciliation of one WIB calendar day
// (design F5-14): legacy stays the book of record while operators key the
// day's transactions into the parallel revamp database; every evening the
// aggregates of both sides are compared and the differences listed.
type DailyReport struct {
	Date        string         `json:"date"`
	GeneratedAt string         `json:"generated_at"`
	Keys        map[string]int `json:"keys_compared"`
	Deltas      []Delta        `json:"deltas"`
	Unexplained int            `json:"unexplained"`
	New         []string       `json:"new_since_previous_run,omitempty"`
	Resolved    []string       `json:"resolved_since_previous_run,omitempty"`
	FullVerify  *Report        `json:"full_verify,omitempty"`
	Pass        bool           `json:"pass"`
}

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

const dailyStatus = `CASE WHEN o.archived_at IS NOT NULL THEN 'cancelled' WHEN d.status_group='pending' THEN 'draft' WHEN d.status_group='active' THEN 'confirmed'
		        WHEN d.status_group='completed' THEN 'completed' ELSE 'cancelled' END`

type dailySection struct {
	name, legacy, target string
}

func dailySections(day string) []dailySection {
	lit := "'" + day + "'"
	return []dailySection{
		{"orders", `SELECT b.code, o.order_kind, ` + dailyStatus + ` st, 'n', COUNT(*) FROM {orders} o JOIN {branches} b ON b.id_branch=o.id_branch
		   JOIN {order_status_definitions} d ON d.id_order_status_definition=o.id_current_status
		  WHERE o.` + byBranch + ` AND DATE(DATE_ADD(o.order_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2,3
		 UNION ALL SELECT b.code, o.order_kind, ` + dailyStatus + `, 'grand', SUM(ROUND(o.total_amount)) FROM {orders} o JOIN {branches} b ON b.id_branch=o.id_branch
		   JOIN {order_status_definitions} d ON d.id_order_status_definition=o.id_current_status
		  WHERE o.` + byBranch + ` AND DATE(DATE_ADD(o.order_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2,3`,
			`SELECT b.code, x.k, x.st, x.m, x.v FROM (
		   SELECT branch_id, 'sales' k, status st, 'n' m, COUNT(*) v FROM sales_orders WHERE DATE(DATE_ADD(order_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'sales', status, 'grand', SUM(grand_total) FROM sales_orders WHERE DATE(DATE_ADD(order_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'purchase', status, 'n', COUNT(*) FROM purchase_orders WHERE DATE(DATE_ADD(order_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,3
		   UNION ALL SELECT branch_id, 'purchase', status, 'grand', SUM(grand_total) FROM purchase_orders WHERE DATE(DATE_ADD(order_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,3) x
		 JOIN branches b ON b.id = x.branch_id`},
		{"payments", `SELECT b.code, COALESCE(p.financial_side, CASE o.order_kind WHEN 'sales' THEN 'receivable' ELSE 'payable' END) side,
		        CASE p.payment_method WHEN 'bank_transfer' THEN 'transfer' ELSE p.payment_method END, CASE WHEN p.archived_at IS NULL THEN 'active' ELSE 'cancelled' END, 'n', COUNT(*)
		   FROM {payments} p JOIN {branches} b ON b.id_branch=p.id_branch LEFT JOIN {orders} o ON o.id_order=p.id_order
		  WHERE p.` + byBranch + ` AND DATE(DATE_ADD(p.payment_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2,3,4
		 UNION ALL SELECT b.code, COALESCE(p.financial_side, CASE o.order_kind WHEN 'sales' THEN 'receivable' ELSE 'payable' END) side,
		        CASE p.payment_method WHEN 'bank_transfer' THEN 'transfer' ELSE p.payment_method END, CASE WHEN p.archived_at IS NULL THEN 'active' ELSE 'cancelled' END, 'sum', SUM(ROUND(p.amount))
		   FROM {payments} p JOIN {branches} b ON b.id_branch=p.id_branch LEFT JOIN {orders} o ON o.id_order=p.id_order
		  WHERE p.` + byBranch + ` AND DATE(DATE_ADD(p.payment_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2,3,4`,
			`SELECT b.code, CASE p.direction WHEN 'in' THEN 'receivable' ELSE 'payable' END, p.method, p.status, m, v FROM (
		   SELECT branch_id, direction, method, status, 'n' m, COUNT(*) v FROM payments WHERE number NOT LIKE 'PAY-%/MIG/%' AND method <> 'offset' AND DATE(DATE_ADD(paid_at, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2,3,4
		   UNION ALL SELECT branch_id, direction, method, status, 'sum', SUM(amount) FROM payments WHERE number NOT LIKE 'PAY-%/MIG/%' AND method <> 'offset' AND DATE(DATE_ADD(paid_at, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2,3,4) p
		 JOIN branches b ON b.id = p.branch_id`},
		{"stock", `SELECT b.code, p.product_code, SUM(CASE m.movement_type WHEN 'in' THEN m.quantity WHEN 'out' THEN -m.quantity ELSE m.balance_after - m.balance_before END)
		   FROM {inventory_movements} m JOIN {branches} b ON b.id_branch=m.id_branch JOIN {products} p ON p.id_product=m.id_product
		  WHERE m.` + byBranch + ` AND DATE(DATE_ADD(m.moved_at, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2`,
			`SELECT b.code, p.code, SUM(CASE m.direction WHEN 'in' THEN m.qty_base ELSE -m.qty_base END) FROM stock_movements m JOIN branches b ON b.id=m.branch_id JOIN products p ON p.id=m.product_id
		  WHERE COALESCE(m.notes,'') <> 'Koreksi migrasi' AND DATE(DATE_ADD(m.created_at, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2`},
		{"cash", `SELECT COALESCE(b.code, (SELECT code FROM {branches} WHERE {co} = :c AND is_default = 1 ORDER BY id_branch LIMIT 1)),
		        CASE a.system_key WHEN 'cash_cash' THEN 'cash' ELSE 'bank' END, SUM(ROUND(l.debit_amount)) - SUM(ROUND(l.credit_amount))
		   FROM {finance_journal_lines} l JOIN {finance_journal_entries} e ON e.id_finance_journal_entry=l.id_finance_journal_entry
		   JOIN {finance_accounts} a ON a.id_finance_account=l.id_finance_account LEFT JOIN {branches} b ON b.id_branch=e.id_branch
		  WHERE e.{co} = :c AND e.status IN ('posted','reversed') AND a.system_key IN ('cash_cash','cash_bank') AND DATE(DATE_ADD(e.journal_date, INTERVAL 7 HOUR)) = ` + lit + ` GROUP BY 1,2`,
			`SELECT b.code, m.mapping_key, SUM(l.debit) - SUM(l.credit) FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id=l.entry_id
		   JOIN finance_account_mappings m ON m.account_id=l.account_id AND m.mapping_key IN ('cash','bank') JOIN branches b ON b.id=e.branch_id
		  WHERE DATE(e.entry_date) = ` + lit + ` GROUP BY 1,2`},
	}
}

// LoadExplained reads "key,reason" rows (key = "section|key"); a missing file
// means nothing is explained.
func LoadExplained(path string) (map[string]string, error) {
	out := map[string]string{}
	if path == "" {
		return out, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	for _, rec := range rows {
		if len(rec) >= 2 && !strings.HasPrefix(rec[0], "#") && strings.TrimSpace(rec[0]) != "" {
			out[strings.TrimSpace(rec[0])] = strings.TrimSpace(rec[1])
		}
	}
	return out, nil
}

// Daily compares the day aggregates of legacy and target. It never writes to
// either database. With full set it also runs the whole verification (useful
// right after a load; during a parallel run the target holds newer data, so the
// counts of the full verify are expected to differ).
func Daily(ctx context.Context, ldb *legacydb.DB, target Querier, c *cfg.Config, work string, full bool) (*DailyReport, error) {
	day := c.Date
	if !dateRe.MatchString(day) {
		return nil, fmt.Errorf("%w: --date harus YYYY-MM-DD", cfg.ErrUsage)
	}
	if _, err := time.Parse("2006-01-02", day); err != nil {
		return nil, fmt.Errorf("%w: --date harus tanggal valid", cfg.ErrUsage)
	}
	explained, err := LoadExplained(c.ExplainFile)
	if err != nil {
		return nil, fmt.Errorf("--explain-file: %w", err)
	}
	v := &verifier{ctx: ctx, l: ldb, t: target, cfg: c, work: work}
	rep := &DailyReport{Date: day, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Keys: map[string]int{}}
	for _, sec := range dailySections(day) {
		src, err := queryMap(ctx, ldb.SQL, v.lq(sec.legacy))
		if err != nil {
			return nil, fmt.Errorf("daily-verify %s (legacy): %w", sec.name, err)
		}
		tgt, err := queryMap(ctx, target, sec.target)
		if err != nil {
			return nil, fmt.Errorf("daily-verify %s (target): %w", sec.name, err)
		}
		keys := map[string]bool{}
		for k := range src {
			keys[k] = true
		}
		for k := range tgt {
			keys[k] = true
		}
		rep.Keys[sec.name] = len(keys)
		for k := range keys {
			a, b := src[k], tgt[k]
			if a == "" {
				a = "0"
			}
			if b == "" {
				b = "0"
			}
			if a != b {
				rep.Deltas = append(rep.Deltas, Delta{Section: sec.name, Key: k, Legacy: a, Target: b, Explained: explained[sec.name+"|"+k]})
			}
		}
	}
	sort.Slice(rep.Deltas, func(i, j int) bool {
		if rep.Deltas[i].Section != rep.Deltas[j].Section {
			return rep.Deltas[i].Section < rep.Deltas[j].Section
		}
		return rep.Deltas[i].Key < rep.Deltas[j].Key
	})
	for _, d := range rep.Deltas {
		if d.Explained == "" {
			rep.Unexplained++
		}
	}
	// delta against the previous run of the same day
	prevPath := filepath.Join(transform.ReportDir(work), "daily", day+".json")
	if raw, err := os.ReadFile(prevPath); err == nil {
		var prev DailyReport
		if json.Unmarshal(raw, &prev) == nil {
			was := map[string]bool{}
			for _, d := range prev.Deltas {
				was[d.Section+"|"+d.Key] = true
			}
			now := map[string]bool{}
			for _, d := range rep.Deltas {
				k := d.Section + "|" + d.Key
				now[k] = true
				if !was[k] {
					rep.New = append(rep.New, k)
				}
			}
			for k := range was {
				if !now[k] {
					rep.Resolved = append(rep.Resolved, k)
				}
			}
			sort.Strings(rep.Resolved)
		}
	}
	if full {
		ex, err := transform.ReadExplain(work)
		if err != nil {
			return nil, fmt.Errorf("--full-verify: hasil transform tidak ada: %w", err)
		}
		v.explain = ex
		fr, err := Run(ctx, ldb, target, c, work)
		if err != nil {
			return nil, err
		}
		rep.FullVerify = fr
	}
	rep.Pass = rep.Unexplained == 0 && (rep.FullVerify == nil || rep.FullVerify.Pass)
	return rep, nil
}

// WriteDaily stores report/daily/<date>.json and .md and returns the markdown.
func WriteDaily(work string, r *DailyReport) (string, error) {
	dir := filepath.Join(transform.ReportDir(work), "daily")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	raw, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, r.Date+".json"), raw, 0o644); err != nil {
		return "", err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "# Rekonsiliasi harian %s\n\nDibuat %s. Kunci dibandingkan: ", r.Date, r.GeneratedAt)
	var names []string
	for k := range r.Keys {
		names = append(names, k)
	}
	sort.Strings(names)
	for i, k := range names {
		if i > 0 {
			sb.WriteString(", ")
		}
		fmt.Fprintf(&sb, "%s=%d", k, r.Keys[k])
	}
	sb.WriteString("\n\n")
	if len(r.Deltas) == 0 {
		sb.WriteString("Tidak ada selisih.\n")
	} else {
		sb.WriteString("| bagian | kunci | legacy | target | terjelaskan |\n|---|---|---|---|---|\n")
		for _, d := range r.Deltas {
			fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s |\n", d.Section, strings.ReplaceAll(d.Key, "|", " / "), d.Legacy, d.Target, d.Explained)
		}
	}
	if len(r.New) > 0 {
		sb.WriteString("\nSelisih baru sejak lari sebelumnya: " + strings.Join(r.New, "; ") + "\n")
	}
	if len(r.Resolved) > 0 {
		sb.WriteString("\nSelisih yang sudah hilang sejak lari sebelumnya: " + strings.Join(r.Resolved, "; ") + "\n")
	}
	if r.FullVerify != nil {
		if r.FullVerify.Pass {
			sb.WriteString("\nVerify penuh: LULUS\n")
		} else {
			sb.WriteString("\nVerify penuh: GAGAL pada " + strings.Join(r.FullVerify.Failed(), ", ") + "\n")
		}
	}
	if r.Pass {
		fmt.Fprintf(&sb, "\n**LULUS** (selisih tak terjelaskan: %d)\n", r.Unexplained)
	} else {
		fmt.Fprintf(&sb, "\n**GAGAL**: %d selisih tak terjelaskan\n", r.Unexplained)
	}
	md := sb.String()
	return md, os.WriteFile(filepath.Join(dir, r.Date+".md"), []byte(md), 0o644)
}

var _ = errors.New
var _ *sql.DB
