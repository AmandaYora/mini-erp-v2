package integration

import (
	"context"
	"fmt"
	"testing"
)

// F4-17 (finance + audit extension of f3a_explain_test.go): the finance rollup
// readers, journal lines by account/date, the aging page + its covering
// payment lookup and the audit list must never fall back to a full scan.
// Data is synthetic (CTE bulk insert) and small enough to run in a second or
// two; ANALYZE gives the optimizer real statistics.

const explainFinRows = 30000

func seedExplainFinance(t *testing.T, ctx context.Context) {
	t.Helper()
	conn, err := testMods.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	exec := func(q string) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, q); err != nil {
			t.Fatalf("seed: %v\n%s", err, q)
		}
	}
	exec(fmt.Sprintf("SET SESSION cte_max_recursion_depth = %d", explainFinRows+10))
	cte := fmt.Sprintf("WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < %d) ", explainFinRows)
	// 100 accounts x 300 days of rollups for branch 1 (PK is unique per n).
	exec(`INSERT INTO finance_account_daily (branch_id, account_id, entry_date, scope, debit, credit) ` + cte +
		`SELECT 1, n % 100 + 1, DATE('2024-01-01') + INTERVAL (n DIV 100) DAY, 0, n, n FROM seq`)
	// 10.000 entries with 3 one-sided lines each.
	exec(`INSERT INTO finance_journal_entries (branch_id, number, entry_date, memo) ` + cte +
		`SELECT 1, CONCAT('JE-', n), TIMESTAMP('2024-01-01') + INTERVAL (n % 300) DAY, 'm' FROM seq WHERE n <= 10000`)
	exec(`INSERT INTO finance_journal_lines (entry_id, account_id, debit, credit) ` + cte +
		`SELECT (n + 2) DIV 3, n % 100 + 1, IF(n % 2 = 0, 100, 0), IF(n % 2 = 0, 0, 100) FROM seq`)
	// Audit trail: 90% branch 1, 5% branch 2, 5% global (NULL).
	exec(`INSERT INTO audit_logs (action_key, entity_type, entity_id, branch_id, actor_type) ` + cte +
		`SELECT ELT(1 + n % 5, 'sales.create', 'sales.confirm', 'payment.create', 'stock.adjust', 'user.login'), 'order', n % 3000,
		        CASE WHEN n % 20 = 0 THEN NULL WHEN n % 20 = 1 THEN 2 ELSE 1 END, 'user' FROM seq`)
	for _, tbl := range []string{"finance_account_daily", "finance_journal_entries", "finance_journal_lines", "audit_logs"} {
		var a, b, c, d string
		if err := conn.QueryRowContext(ctx, "ANALYZE TABLE "+tbl).Scan(&a, &b, &c, &d); err != nil {
			t.Fatalf("analyze %s: %v", tbl, err)
		}
	}
}

func TestF4ExplainFinanceAndAudit(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	seedExplainData(t, ctx) // orders + allocations for the aging queries
	seedExplainFinance(t, ctx)

	type check struct {
		name, query string
		args        []any
		table       string
		mustKey     string // "" = any index
		noFilesort  bool
		covering    bool
	}
	checks := []check{
		{"account footings from rollup",
			"SELECT account_id, COALESCE(SUM(debit), 0), COALESCE(SUM(credit), 0) FROM finance_account_daily WHERE branch_id = ? AND scope = 0 AND entry_date >= ? AND entry_date <= ? GROUP BY account_id",
			[]any{int64(1), "2024-02-01", "2024-03-31"}, "finance_account_daily", "", false, false},
		{"daily footings from rollup",
			"SELECT DATE_FORMAT(entry_date, '%Y-%m-%d'), account_id, debit, credit FROM finance_account_daily WHERE branch_id = ? AND scope = 0 AND entry_date >= ? AND entry_date <= ? ORDER BY entry_date ASC",
			[]any{int64(1), "2024-02-01", "2024-03-31"}, "finance_account_daily", "", false, false},
		{"journal lines of one account (lines side)",
			"SELECT l.id FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id WHERE l.account_id = ? AND e.branch_id = ? AND e.entry_date >= ? AND e.entry_date <= ?",
			[]any{int64(5), int64(1), "2024-02-01 00:00:00", "2024-02-29 23:59:59"}, "l", "", false, false},
		{"journal lines of one account (entries side)",
			"SELECT l.id FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id WHERE l.account_id = ? AND e.branch_id = ? AND e.entry_date >= ? AND e.entry_date <= ?",
			[]any{int64(5), int64(1), "2024-02-01 00:00:00", "2024-02-29 23:59:59"}, "e", "", false, false},
		{"lines of an entry batch",
			"SELECT l.entry_id, l.account_id, l.debit, l.credit FROM finance_journal_lines l WHERE l.entry_id IN (10, 11, 12, 13) ORDER BY l.entry_id ASC, l.id ASC",
			nil, "l", "fk_finance_journal_lines_entry", false, false},
		{"entries by date range",
			"SELECT id FROM finance_journal_entries WHERE branch_id = ? AND entry_date >= ? AND entry_date <= ? ORDER BY entry_date, id LIMIT 50",
			[]any{int64(1), "2024-02-01 00:00:00", "2024-02-29 23:59:59"}, "finance_journal_entries", "ix_finance_journal_branch_date", false, false},
		{"aging: open orders page (keyset)",
			"SELECT id, number, party_id, grand_total, order_date, due_date FROM sales_orders FORCE INDEX (ix_sales_orders_branch_id) WHERE branch_id = ? AND status IN ('confirmed', 'completed') AND id > ? ORDER BY id ASC LIMIT 1000",
			[]any{int64(1), int64(0)}, "sales_orders", "", true, false},
		{"aging: paid per order (covering)",
			"SELECT order_id, SUM(amount) FROM payment_allocations WHERE order_type = ? AND order_id IN (1,2,3,4,5,6,7,8) GROUP BY order_id",
			[]any{"sales"}, "payment_allocations", "ix_payment_allocations_cover", false, true},
		{"audit list: branch leg",
			"SELECT id FROM audit_logs WHERE branch_id = ? ORDER BY id DESC LIMIT 20",
			[]any{int64(1)}, "audit_logs", "", true, false}, // PK walk or ix_audit_logs_branch: both stop after 20 rows
		// Global leg: only "no full scan" is pinned. On this data the optimizer prefers the older
		// covering ix_audit_logs_branch_time (+ filesort of the NULL rows) over ix_audit_logs_branch.
		{"audit list: global leg",
			"SELECT id FROM audit_logs WHERE branch_id IS NULL ORDER BY id DESC LIMIT 20",
			nil, "audit_logs", "", false, false},
		{"audit list: branch + action",
			"SELECT id FROM audit_logs WHERE branch_id = ? AND action_key = ? ORDER BY id DESC LIMIT 20",
			[]any{int64(1), "sales.create"}, "audit_logs", "ix_audit_logs_branch_action", true, false},
		{"audit list: one entity history",
			"SELECT id FROM audit_logs WHERE entity_type = ? AND entity_id = ? ORDER BY id DESC LIMIT 20",
			[]any{"order", int64(42)}, "audit_logs", "ix_audit_logs_entity", true, false},
		{"audit count by branch (covering)",
			"SELECT COUNT(*) FROM audit_logs WHERE branch_id = ?",
			[]any{int64(1)}, "audit_logs", "", false, true},
	}
	for _, c := range checks {
		c := c
		t.Run(c.name, func(t *testing.T) {
			var found bool
			for _, p := range explainPlan(t, ctx, c.query, c.args...) {
				if p.Table != c.table {
					continue
				}
				found = true
				if p.AccessType == "ALL" {
					t.Fatalf("full scan: %+v", p)
				}
				if c.mustKey != "" && p.Key != c.mustKey {
					t.Fatalf("index %q, want %q (%+v)", p.Key, c.mustKey, p)
				}
				if c.noFilesort && p.Filesort {
					t.Fatalf("unexpected filesort (%+v)", p)
				}
				if c.covering && !p.UsingIndex {
					t.Fatalf("not covered by an index (%+v)", p)
				}
			}
			if !found {
				t.Fatalf("table %s not in plan", c.table)
			}
		})
	}
}
