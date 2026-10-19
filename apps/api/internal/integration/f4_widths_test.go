package integration

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// F4-2: legacy data must fit. For every widened column the test inserts a
// maximum-length legacy value and reads it back: a column that is still too
// narrow truncates (the session runs non-strict on purpose so the insert
// itself succeeds and the readback proves the stored length). Rows go to a
// TEMPORARY copy (CREATE TEMPORARY TABLE ... LIKE) on one dedicated
// connection, so no real table or other test is touched.

type widthCase struct {
	table, column string
	chars         int // minimum character capacity (text kinds)
	decimal       string
}

// widthSatisfy supplies columns a CHECK constraint needs, per table, as
// "col=value" pairs (a journal line must be one-sided).
var widthSatisfy = map[string]string{"finance_journal_lines": "debit=1,credit=0"}

var widthCases = []widthCase{
	// 000050 / 000052 / 000054-000056 (auth, branch, numbers, product)
	{"users", "username", 100, ""}, {"users", "full_name", 255, ""},
	{"branches", "code", 50, ""}, {"branches", "name", 255, ""}, {"branches", "city", 120, ""},
	{"branches", "address", 30000, ""},
	{"stock_transfers", "number", 100, ""}, {"purchase_orders", "number", 100, ""}, {"sales_orders", "number", 100, ""},
	{"delivery_notes", "number", 100, ""}, {"payments", "number", 100, ""}, {"sales_returns", "number", 100, ""},
	{"purchase_returns", "number", 100, ""}, {"business_expenses", "number", 100, ""}, {"goods_receipts", "number", 100, ""},
	{"finance_journal_entries", "number", 120, ""},
	{"product_variants", "code", 80, ""}, {"product_variants", "name", 255, ""}, {"product_categories", "name", 255, ""},
	// 000069 stock / company / media
	{"stock_locations", "code", 50, ""}, {"stock_locations", "name", 255, ""},
	{"stock_movements", "notes", 500, ""},
	{"stock_transfers", "notes", 30000, ""}, {"stock_transfer_items", "notes", 30000, ""},
	{"company_profile", "name", 255, ""}, {"company_profile", "legal_name", 255, ""},
	{"media_files", "file_key", 700, ""}, {"media_files", "mime", 120, ""},
	// 000070 sales / purchasing
	{"sales_orders", "tax_invoice_number", 100, ""}, {"sales_orders", "ship_to_address", 30000, ""},
	{"sales_orders", "ship_to_label", 120, ""}, {"sales_order_items", "line_note", 30000, ""},
	{"purchase_orders", "supplier_invoice_number", 100, ""}, {"purchase_order_items", "line_note", 30000, ""},
	{"sales_orders", "tax_rate", 0, "99999.9999"}, {"sales_order_items", "discount_pct", 0, "99999.9999"},
	{"purchase_orders", "tax_rate", 0, "99999.9999"}, {"purchase_order_items", "discount_pct", 0, "99999.9999"},
	// 000071 documents
	{"goods_receipts", "notes", 30000, ""}, {"goods_receipts", "supplier_delivery_number", 100, ""},
	{"delivery_notes", "notes", 30000, ""}, {"payments", "notes", 30000, ""},
	{"sales_returns", "notes", 30000, ""}, {"sales_returns", "reason", 30000, ""},
	{"purchase_returns", "notes", 30000, ""}, {"purchase_returns", "reason", 30000, ""},
	{"sales_return_settlements", "notes", 30000, ""}, {"purchase_return_settlements", "notes", 30000, ""},
	{"sales_returns", "tax_rate", 0, "99999.9999"}, {"purchase_returns", "tax_rate", 0, "99999.9999"},
	{"sales_return_items", "discount_pct", 0, "99999.9999"}, {"purchase_return_items", "discount_pct", 0, "99999.9999"},
	// 000072 finance / audit
	{"finance_accounts", "code", 50, ""}, {"finance_accounts", "name", 255, ""},
	{"finance_journal_entries", "memo", 500, ""}, {"finance_journal_lines", "description", 500, ""},
	{"business_expenses", "notes", 30000, ""}, {"audit_logs", "note", 500, ""},
}

func TestF4LegacyMaxLengthsFit(t *testing.T) {
	ctx := context.Background()
	conn, err := testMods.db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()

	var oldMode string
	if err := conn.QueryRowContext(ctx, "SELECT @@SESSION.sql_mode").Scan(&oldMode); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ExecContext(ctx, "SET SESSION sql_mode = ''"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		// The connection returns to the pool: put the strict mode back.
		_, _ = conn.ExecContext(context.Background(), "SET SESSION sql_mode = ?", oldMode)
	}()

	for _, tc := range widthCases {
		tc := tc
		t.Run(tc.table+"."+tc.column, func(t *testing.T) {
			tmp := "tmp_f4w"
			_, _ = conn.ExecContext(ctx, "DROP TEMPORARY TABLE IF EXISTS "+tmp)
			if _, err := conn.ExecContext(ctx, fmt.Sprintf("CREATE TEMPORARY TABLE %s LIKE `%s`", tmp, tc.table)); err != nil {
				t.Fatalf("temp copy of %s: %v", tc.table, err)
			}
			defer func() { _, _ = conn.ExecContext(context.Background(), "DROP TEMPORARY TABLE IF EXISTS "+tmp) }()

			if tc.decimal != "" {
				if _, err := conn.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (`%s`) VALUES (?)", tmp, tc.column), tc.decimal); err != nil {
					t.Fatalf("insert: %v", err)
				}
				var got string
				if err := conn.QueryRowContext(ctx, fmt.Sprintf("SELECT CAST(`%s` AS CHAR) FROM %s", tc.column, tmp)).Scan(&got); err != nil {
					t.Fatal(err)
				}
				if got != tc.decimal {
					t.Fatalf("%s.%s stored %s, want %s (precision too small)", tc.table, tc.column, got, tc.decimal)
				}
				return
			}
			// Multi-byte on purpose: utf8mb4 characters, not bytes, are the capacity.
			val := strings.Repeat("é", tc.chars)
			cols, marks := "`"+tc.column+"`", "?"
			for _, kv := range strings.Split(widthSatisfy[tc.table], ",") {
				if kv = strings.TrimSpace(kv); kv != "" {
					parts := strings.SplitN(kv, "=", 2)
					cols += ", `" + parts[0] + "`"
					marks += ", " + parts[1]
				}
			}
			if _, err := conn.ExecContext(ctx, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tmp, cols, marks), val); err != nil {
				t.Fatalf("insert: %v", err)
			}
			var n int
			if err := conn.QueryRowContext(ctx, fmt.Sprintf("SELECT CHAR_LENGTH(`%s`) FROM %s", tc.column, tmp)).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n != tc.chars {
				t.Fatalf("%s.%s kept %d of %d characters (column too narrow)", tc.table, tc.column, n, tc.chars)
			}
		})
	}
}
