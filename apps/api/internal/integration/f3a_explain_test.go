package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"mini-erp/internal/shared/listquery"
)

// F4-17: EXPLAIN gate for the hot list queries. Synthetic rows are bulk-loaded
// with a recursive CTE (tens of thousands, a few hundred ms), the tables are
// analyzed, and every query shape below must (a) never scan the big table
// (access_type ALL), (b) where stated, sort without a filesort and/or read
// from a covering index. A migration that drops or reshapes an index these
// queries rely on turns this test red.

const explainRows = 40000

type planNode struct {
	Table      string
	AccessType string
	Key        string
	Filesort   bool
	UsingIndex bool
}

// explainPlan returns one planNode per table of the plan.
func explainPlan(t *testing.T, ctx context.Context, query string, args ...any) []planNode {
	t.Helper()
	var raw string
	if err := testMods.db.QueryRowContext(ctx, "EXPLAIN FORMAT=JSON "+query, args...).Scan(&raw); err != nil {
		t.Fatalf("explain %q: %v", query, err)
	}
	var doc any
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("explain json: %v", err)
	}
	var out []planNode
	var walk func(n any, sorted bool)
	walk = func(n any, sorted bool) {
		switch v := n.(type) {
		case map[string]any:
			if b, ok := v["using_filesort"].(bool); ok && b {
				sorted = true
			}
			if name, ok := v["table_name"].(string); ok {
				pn := planNode{Table: name, Filesort: sorted}
				pn.AccessType, _ = v["access_type"].(string)
				pn.Key, _ = v["key"].(string)
				pn.UsingIndex, _ = v["using_index"].(bool)
				out = append(out, pn)
			}
			for _, c := range v {
				walk(c, sorted)
			}
		case []any:
			for _, c := range v {
				walk(c, sorted)
			}
		}
	}
	walk(doc, false)
	return out
}

func seedExplainData(t *testing.T, ctx context.Context) {
	t.Helper()
	conn, err := testMods.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := conn.ExecContext(ctx, q, args...); err != nil {
			t.Fatalf("seed: %v\n%s", err, q)
		}
	}
	exec(fmt.Sprintf("SET SESSION cte_max_recursion_depth = %d", explainRows+10))
	cte := fmt.Sprintf("WITH RECURSIVE seq (n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < %d) ", explainRows)
	// 95% of the rows belong to branch 1; the rest to branch 2.
	exec(`INSERT INTO sales_orders (number, branch_id, party_id, channel, order_date, payment_terms, status, grand_total) ` + cte + `SELECT CONCAT('SO-', n), IF(n % 20 = 0, 2, 1), n % 500, IF(n % 3 = 0, 'pos', 'regular'),
		       TIMESTAMP('2024-01-01') + INTERVAL (n % 700) DAY,
		       ELT(1 + n % 3, 'cod', 'net', 'prepaid'),
		       ELT(1 + n % 4, 'draft', 'confirmed', 'completed', 'cancelled'), n * 100 FROM seq`)
	exec(`INSERT INTO payments (number, branch_id, party_id, party_type, direction, amount, method, paid_at, status) ` + cte + `SELECT CONCAT('PAY-', n), IF(n % 20 = 0, 2, 1), n % 500, 'customer', 'in', 1000, ELT(1 + n % 2, 'cash', 'transfer'),
		       TIMESTAMP('2024-01-01') + INTERVAL (n % 700) DAY, 'active' FROM seq`)
	exec(`INSERT INTO payment_allocations (payment_id, order_type, order_id, amount) ` + cte + `SELECT n, 'sales', n, 1000 FROM seq`)
	exec(`INSERT INTO delivery_notes (number, branch_id, sales_order_id, delivery_date, status, document_kind) ` + cte + `SELECT CONCAT('SJ-', n), IF(n % 20 = 0, 2, 1), n, TIMESTAMP('2024-01-01') + INTERVAL (n % 700) DAY,
		       IF(n % 200 = 0, 'draft', 'confirmed'), 'order' FROM seq`)
	exec(`INSERT INTO stock_locations (branch_id, code, name) VALUES (1, 'EX1', 'Explain 1'), (2, 'EX2', 'Explain 2')`)
	exec(`INSERT INTO stock_movements (branch_id, product_id, variant_id, location_id, direction, movement_type, qty_base, ref_type, ref_id) ` + cte + `SELECT 1, n % 50 + 1, 1, 1, 'in', 'adjust', 1, 'explain', n FROM seq`)
	exec(`INSERT INTO stock_transfers (number, from_branch_id, to_branch_id, from_location_id, to_location_id, status) ` + cte + `SELECT CONCAT('TR-', n), IF(n % 2 = 0, 1, 2), IF(n % 2 = 0, 2, 1), IF(n % 2 = 0, 1, 2), IF(n % 2 = 0, 2, 1), 'received' FROM seq`)
	for _, tbl := range []string{"sales_orders", "payments", "payment_allocations", "delivery_notes", "stock_movements", "stock_transfers"} {
		var a, b, c, d string
		if err := conn.QueryRowContext(ctx, "ANALYZE TABLE "+tbl).Scan(&a, &b, &c, &d); err != nil {
			t.Fatalf("analyze %s: %v", tbl, err)
		}
	}
}

func TestF4ExplainHotQueries(t *testing.T) {
	freshDB(t)
	ctx := context.Background()
	seedExplainData(t, ctx)

	type check struct {
		name       string
		query      string
		args       []any
		bigTable   string // alias or name as EXPLAIN reports it
		mustKey    string // expected index on bigTable ("" = any non-ALL)
		noFilesort bool
		coveringIx bool
	}
	soWhere := func(mut func(w *listquery.Where)) (string, []any) {
		w := listquery.New().Eq("branch_id", int64(1))
		mut(w)
		return w.SQL()
	}
	var checks []check
	add := func(c check) { checks = append(checks, c) }

	w, a := soWhere(func(w *listquery.Where) {})
	add(check{"orders default page", "SELECT id FROM sales_orders WHERE " + w + " ORDER BY id DESC LIMIT 20 OFFSET 40", a, "sales_orders", "ix_sales_orders_branch_id", true, false})
	w, a = soWhere(func(w *listquery.Where) { w.StatusCSV("status", "confirmed") })
	add(check{"orders by status", "SELECT id FROM sales_orders WHERE " + w + " ORDER BY id DESC LIMIT 20", a, "sales_orders", "ix_sales_orders_status", true, false})
	w, a = soWhere(func(w *listquery.Where) { w.Eq("party_id", int64(7)) })
	add(check{"orders by party", "SELECT id FROM sales_orders WHERE " + w + " ORDER BY id DESC LIMIT 20", a, "sales_orders", "ix_sales_orders_party", true, false})
	w, a = soWhere(func(w *listquery.Where) { w.DateRange("order_date", "2024-03-01", "2024-03-03") })
	add(check{"orders by narrow date range", "SELECT id FROM sales_orders WHERE " + w + " ORDER BY id DESC LIMIT 20", a, "sales_orders", "ix_sales_orders_date", false, false})
	w, a = soWhere(func(w *listquery.Where) { w.StatusCSV("status", "confirmed") })
	add(check{"orders count by status", "SELECT COUNT(*) FROM sales_orders WHERE " + w, a, "sales_orders", "ix_sales_orders_status", false, true})
	w, a = soWhere(func(w *listquery.Where) {})
	add(check{"orders count unfiltered", "SELECT COUNT(*) FROM sales_orders WHERE " + w, a, "sales_orders", "", false, true})

	add(check{"payments by paid_at range",
		"SELECT id FROM payments WHERE branch_id = ? AND paid_at >= ? AND paid_at < ? ORDER BY id DESC LIMIT 20",
		[]any{int64(1), "2024-03-01 00:00:00", "2024-03-04 00:00:00"}, "payments", "ix_payments_paid_at", false, false})
	add(check{"payments by party", "SELECT id FROM payments WHERE branch_id = ? AND party_id = ? ORDER BY id DESC LIMIT 20",
		[]any{int64(1), int64(9)}, "payments", "", true, false})
	add(check{"paid for orders (covering)",
		"SELECT order_id, SUM(amount) FROM payment_allocations WHERE order_type = ? AND order_id IN (1,2,3,4,5) GROUP BY order_id",
		[]any{"sales"}, "payment_allocations", "ix_payment_allocations_cover", false, true})

	add(check{"delivery waiting return",
		"SELECT id FROM delivery_notes WHERE branch_id = ? AND status = 'draft' AND document_kind = 'order' AND delivery_date < ? ORDER BY delivery_date ASC, id ASC LIMIT 100",
		[]any{int64(1), "2025-12-01 00:00:00"}, "delivery_notes", "ix_delivery_notes_waiting", true, false})

	add(check{"has movements anywhere",
		"SELECT EXISTS(SELECT 1 FROM stock_movements WHERE product_id = ? LIMIT 1)", []any{int64(3)}, "stock_movements", "ix_stock_movements_product", false, false})
	add(check{"transfers from branch",
		"SELECT id FROM stock_transfers WHERE from_branch_id = ? ORDER BY id DESC LIMIT 20", []any{int64(1)}, "stock_transfers", "ix_stock_transfers_from", true, false})
	add(check{"transfers to branch",
		"SELECT id FROM stock_transfers WHERE to_branch_id = ? AND from_branch_id <> ? ORDER BY id DESC LIMIT 20", []any{int64(1), int64(1)}, "stock_transfers", "ix_stock_transfers_to", true, false})

	for _, c := range checks {
		c := c
		t.Run(c.name, func(t *testing.T) {
			plan := explainPlan(t, ctx, c.query, c.args...)
			var found bool
			for _, p := range plan {
				if p.Table != c.bigTable {
					continue
				}
				found = true
				if p.AccessType == "ALL" {
					t.Fatalf("full scan of %s: %+v", p.Table, p)
				}
				if c.mustKey != "" && p.Key != c.mustKey {
					t.Fatalf("%s: index %q, want %q (%+v)", c.name, p.Key, c.mustKey, p)
				}
				if c.noFilesort && p.Filesort {
					t.Fatalf("%s: unexpected filesort (%+v)", c.name, p)
				}
				if c.coveringIx && !p.UsingIndex {
					t.Fatalf("%s: not served from a covering index (%+v)", c.name, p)
				}
			}
			if !found {
				t.Fatalf("table %s not in plan: %+v", c.bigTable, plan)
			}
		})
	}
	_ = strings.TrimSpace
}
