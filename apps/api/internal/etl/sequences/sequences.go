// Package sequences rebuilds the document counters after the load (K9, F5-10).
// A counter is max(legacy counter, highest number parsed from the migrated
// documents) and never goes down: the first document created after cutover
// must not collide with a migrated number.
package sequences

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/etl/transform"
	"mini-erp/internal/shared/timeutil"
)

// Counter is one (branch, key) value.
type Counter struct {
	Branch int64
	Key    string
	Value  int64
}

// Report is what Rebuild wrote.
type Report struct {
	Branch  []Counter
	Finance []Counter
}

var (
	reSales    = regexp.MustCompile(`/PJ/(\d{4})/(\d{2})/(\d+)$`)
	rePurchase = regexp.MustCompile(`/PB/(\d{4})/(\d{2})/(\d+)$`)
	rePayment  = regexp.MustCompile(`^PAY-.+/\d{4}/(\d+)$`)
	reSJ       = regexp.MustCompile(`^SJ-.+/\d{4}/(\d+)$`) // synthetic /MIG/ numbers do not match
	reRTR      = regexp.MustCompile(`^RTR-.+/\d{4}/(\d+)$`)
	reRTB      = regexp.MustCompile(`^RTB-.+/\d{4}/(\d+)$`)
	reGR       = regexp.MustCompile(`^GR-.+/\d{4}/(\d+)$`)
	reTransfer = regexp.MustCompile(`^TRF-.+/\d{4}/(\d+)(-[A-Z])?$`)
	reJV       = regexp.MustCompile(`^JV-.+/(\d{4})/(\d+)$`)
	// Revamp EXP-<branch>/<year>/n and the legacy company series EXP/<year>/n:
	// the revamp's counter continues after the highest legacy number.
	reEXP = regexp.MustCompile(`^EXP(?:-[^/]+)?/(\d{4})/(\d+)$`)
)

// ExpenseCounter parses an expense number into its finance counter key
// (EXP:<year>) and value; ok is false for any other format.
func ExpenseCounter(number string) (key string, value int64, ok bool) {
	v, m, ok := tail(reEXP, number, 2)
	if !ok {
		return "", 0, false
	}
	return "EXP:" + m[1], v, true
}

func tail(re *regexp.Regexp, s string, group int) (int64, []string, bool) {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return 0, nil, false
	}
	n, err := strconv.ParseInt(m[group], 10, 64)
	return n, m, err == nil
}

type acc map[int64]map[string]int64

func (a acc) bump(branch int64, key string, v int64) {
	if a[branch] == nil {
		a[branch] = map[string]int64{}
	}
	if v > a[branch][key] {
		a[branch][key] = v
	}
}

// legacyKey maps a legacy sequence key to the revamp key (design A8).
func legacyKey(k string) (string, bool) {
	switch {
	case strings.HasPrefix(k, "order_sales_"):
		return "sales_order:" + strings.TrimPrefix(k, "order_sales_"), true
	case strings.HasPrefix(k, "order_purchase_"):
		return "purchase_order:" + strings.TrimPrefix(k, "order_purchase_"), true
	}
	switch k {
	case "payment", "sales_return", "purchase_return", "stock_transfer", "goods_receipt":
		return k, true
	case "sj":
		return "delivery_note", true
	}
	return "", false
}

func numbers(ctx context.Context, db *sql.DB, q string, fn func(branch int64, number string)) error {
	rows, err := db.QueryContext(ctx, q)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var b int64
		var n sql.NullString
		if err := rows.Scan(&b, &n); err != nil {
			return err
		}
		if n.Valid {
			fn(b, n.String)
		}
	}
	return rows.Err()
}

// Compute derives the counters without writing (used by Rebuild and tests).
func Compute(ctx context.Context, db *sql.DB, work string) (branch, fin acc, err error) {
	branch, fin = acc{}, acc{}
	idm, err := transform.ReadRows(work, "id_map")
	if err != nil {
		return nil, nil, err
	}
	newBranch := map[int64]int64{}
	for _, r := range idm {
		if r.Str("entity") == "branches" {
			newBranch[r.Int("legacy_id")] = r.Int("new_id")
		}
	}
	legacy, err := extract.ReadTable(work, "branch_document_sequences")
	if err != nil {
		return nil, nil, err
	}
	for _, r := range legacy {
		b, ok := newBranch[r.Int("id_branch")]
		if !ok {
			continue
		}
		if k, ok := legacyKey(r.Str("sequence_key")); ok {
			branch.bump(b, k, r.Int("current_value"))
		}
	}
	type src struct {
		q  string
		fn func(b int64, n string)
	}
	srcs := []src{
		{"SELECT branch_id, number FROM sales_orders", func(b int64, n string) {
			if v, m, ok := tail(reSales, n, 3); ok {
				branch.bump(b, "sales_order:"+m[1]+"-"+m[2], v)
			}
		}},
		{"SELECT branch_id, number FROM purchase_orders", func(b int64, n string) {
			if v, m, ok := tail(rePurchase, n, 3); ok {
				branch.bump(b, "purchase_order:"+m[1]+"-"+m[2], v)
			}
		}},
		{"SELECT branch_id, number FROM payments", func(b int64, n string) {
			if v, _, ok := tail(rePayment, n, 1); ok {
				branch.bump(b, "payment", v)
			}
		}},
		{"SELECT branch_id, number FROM delivery_notes", func(b int64, n string) {
			if v, _, ok := tail(reSJ, n, 1); ok {
				branch.bump(b, "delivery_note", v)
			}
		}},
		{"SELECT branch_id, number FROM sales_returns", func(b int64, n string) {
			if v, _, ok := tail(reRTR, n, 1); ok {
				branch.bump(b, "sales_return", v)
			}
		}},
		{"SELECT branch_id, number FROM purchase_returns", func(b int64, n string) {
			if v, _, ok := tail(reRTB, n, 1); ok {
				branch.bump(b, "purchase_return", v)
			}
		}},
		{"SELECT branch_id, number FROM goods_receipts", func(b int64, n string) {
			if v, _, ok := tail(reGR, n, 1); ok {
				branch.bump(b, "goods_receipt", v)
			}
		}},
		{"SELECT from_branch_id, number FROM stock_transfers", func(b int64, n string) {
			if v, _, ok := tail(reTransfer, n, 1); ok {
				branch.bump(b, "stock_transfer", v)
			}
		}},
		{"SELECT branch_id, number FROM finance_journal_entries", func(b int64, n string) {
			if v, m, ok := tail(reJV, n, 2); ok {
				fin.bump(b, "JV:"+m[1], v)
			}
		}},
		{"SELECT branch_id, number FROM business_expenses", func(b int64, n string) {
			if k, v, ok := ExpenseCounter(n); ok {
				fin.bump(b, k, v)
			}
		}},
	}
	for _, s := range srcs {
		if err := numbers(ctx, db, s.q, s.fn); err != nil {
			return nil, nil, err
		}
	}
	return branch, fin, nil
}

func flatten(a acc) []Counter {
	var out []Counter
	for b, m := range a {
		for k, v := range m {
			out = append(out, Counter{b, k, v})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Branch != out[j].Branch {
			return out[i].Branch < out[j].Branch
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// Rebuild writes the counters (GREATEST, so they only ever rise).
func Rebuild(ctx context.Context, db *sql.DB, work string) (*Report, error) {
	b, f, err := Compute(ctx, db, work)
	if err != nil {
		return nil, err
	}
	rep := &Report{Branch: flatten(b), Finance: flatten(f)}
	for _, c := range rep.Branch {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO branch_document_sequences (branch_id, doc_kind, current_value) VALUES (?,?,?)
			 ON DUPLICATE KEY UPDATE current_value = GREATEST(current_value, VALUES(current_value))`, c.Branch, c.Key, c.Value); err != nil {
			return nil, err
		}
	}
	for _, c := range rep.Finance {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO finance_document_sequences (branch_id, seq_key, current_value) VALUES (?,?,?)
			 ON DUPLICATE KEY UPDATE current_value = GREATEST(current_value, VALUES(current_value))`, c.Branch, c.Key, c.Value); err != nil {
			return nil, err
		}
	}
	return rep, nil
}

// NextNumber renders the number the revamp would hand out next for a counter
// key, with the same formats as branch.NextDocumentNumber (test-only copy: the
// ETL may not import that module's internals).
func NextNumber(branchCode, key string, value int64, now time.Time) (string, bool) {
	year := now.In(timeutil.Jakarta).Format("2006")
	n := value + 1
	kind, month, _ := strings.Cut(key, ":")
	switch kind {
	case "sales_order":
		return fmt.Sprintf("ORD-%s/PJ/%s/%05d", branchCode, strings.Replace(month, "-", "/", 1), n), true
	case "purchase_order":
		return fmt.Sprintf("ORD-%s/PB/%s/%05d", branchCode, strings.Replace(month, "-", "/", 1), n), true
	case "payment":
		return fmt.Sprintf("PAY-%s/%s/%05d", branchCode, year, n), true
	case "delivery_note":
		return fmt.Sprintf("SJ-%s/%s/%05d", branchCode, year, n), true
	case "sales_return":
		return fmt.Sprintf("RTR-%s/%s/%05d", branchCode, year, n), true
	case "purchase_return":
		return fmt.Sprintf("RTB-%s/%s/%05d", branchCode, year, n), true
	case "stock_transfer":
		return fmt.Sprintf("TRF-%s/%s/%05d", branchCode, year, n), true
	case "goods_receipt":
		return fmt.Sprintf("GR-%s/%s/%05d", branchCode, year, n), true
	}
	return "", false
}

// FinanceNextNumber renders the next number of a finance counter key
// (PREFIX:YEAR), as the finance module's numberInTx does.
func FinanceNextNumber(branchCode, key string, value int64) (string, bool) {
	prefix, year, ok := strings.Cut(key, ":")
	if !ok || prefix == "" || len(year) != 4 {
		return "", false
	}
	return fmt.Sprintf("%s-%s/%s/%05d", prefix, branchCode, year, value+1), true
}

// Flatten exposes the counters of a Compute result.
func Flatten(a map[int64]map[string]int64) []Counter { return flatten(acc(a)) }

var _ = row.Row{}
