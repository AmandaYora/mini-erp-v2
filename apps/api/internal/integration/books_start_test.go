package integration

import (
	"strings"
	"testing"

	financeapp "mini-erp/internal/modules/finance/application"
)

// Audit F3: once a month is closed (the migration closes every month before
// the opening balance), a month earlier than the first period row is outside
// the books. An entry there would move every balance after the opening, so it
// is refused like a closed month — and refusing it must not create the row.
func TestEntryBeforeBooksStartIsRefused(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	if _, err := testMods.db.Exec(`INSERT INTO finance_periods (branch_id, year, month, status) VALUES (?, 2026, 4, 'closed')`, fx.branchID); err != nil {
		t.Fatalf("seed closed period: %v", err)
	}
	lines := []financeapp.ManualLine{{AccountCode: "1000", Debit: 1000}, {AccountCode: "2000", Credit: 1000}}

	_, err := fin.Manual(ctx(), 0, fx.branchID, "2026-03-15", "sebelum buku", lines)
	if err == nil || !strings.Contains(mustAppErr(t, err).Message, "awal pembukuan") {
		t.Fatalf("entry before books start = %v, want refusal", err)
	}
	var n int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM finance_periods WHERE branch_id = ? AND year = 2026 AND month = 3`, fx.branchID).Scan(&n); err != nil || n != 0 {
		t.Fatalf("refused entry created a period row (n=%d err=%v)", n, err)
	}
	if _, err := fin.Manual(ctx(), 0, fx.branchID, "2026-04-10", "bulan tutup", lines); err == nil {
		t.Fatal("entry in a closed month accepted")
	}
	if _, err := fin.Manual(ctx(), 0, fx.branchID, "2026-05-02", "sesudah", lines); err != nil {
		t.Fatalf("entry after the closed month: %v", err)
	}
}
