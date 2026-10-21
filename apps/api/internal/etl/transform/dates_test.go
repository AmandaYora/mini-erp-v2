package transform_test

import (
	"testing"

	"mini-erp/internal/etl/etltest"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/row"
	"mini-erp/internal/shared/timeutil"
)

// D-ETL-3b: the legacy app wrote document datetimes in UTC (TypeORM
// timezone 'Z') while the database filled created_at/updated_at with the WIB
// server clock. The v2 runtime keeps document dates as WIB calendar dates and
// system stamps as UTC instants, so the transform converts each kind to the
// v2 convention: a payment made at 00:30 WIB on 1 January must not land on
// 31 December, and created_at must be the true instant.
func TestTransformWritesLegacyDatesAsWIB(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	byNumber := func(table, col, number string) row.Row {
		t.Helper()
		for _, r := range out.Tables[table] {
			if r.Str(col) == number {
				return r
			}
		}
		t.Fatalf("%s %s not migrated", table, number)
		return nil
	}

	// Payment 1: 2025-12-31 17:30 UTC = 2026-01-01 00:30 WIB.
	pay := byNumber("payments", "number", "PAY-PST/2025/00001")
	if got := pay.Str("paid_at"); got != "2026-01-01 00:30:00.000000" {
		t.Fatalf("paid_at = %q, want the WIB wall clock 2026-01-01 00:30", got)
	}
	// Its created_at (database default, WIB digits 17:30) is 10:30 UTC.
	if got := pay.Str("created_at"); got != "2025-12-31 10:30:00.000000" {
		t.Fatalf("created_at = %q, want the UTC instant 2025-12-31 10:30", got)
	}

	// Every migrated document date equals the legacy screen's WIB value.
	legacyPay := map[string]string{}
	for _, r := range raw["payments"] {
		if tm, ok := r.Time("payment_date"); ok {
			legacyPay[r.Str("payment_number")] = tm.In(timeutil.Jakarta).Format("2006-01-02 15:04:05.000000")
		}
	}
	checked := 0
	for _, r := range out.Tables["payments"] {
		if want, ok := legacyPay[r.Str("number")]; ok {
			if r.Str("paid_at") != want {
				t.Errorf("payment %s paid_at = %q, want %q", r.Str("number"), r.Str("paid_at"), want)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no migrated payment matched a legacy payment number")
	}

	// Order 1: order_date 2025-12-10 03:00 UTC = 10:00 WIB; due 2026-01-09 03:00 UTC.
	so := byNumber("sales_orders", "number", "ORD-PST/PJ/2025/12/00001")
	if so.Str("order_date") != "2025-12-10 10:00:00.000000" || so.Str("due_date") != "2026-01-09 10:00:00.000000" {
		t.Fatalf("order_date/due_date = %q / %q, want WIB wall clock", so.Str("order_date"), so.Str("due_date"))
	}
}
