package integration

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	assistantapp "mini-erp/internal/modules/assistant/application"
	assistantinfra "mini-erp/internal/modules/assistant/infrastructure"
	branchapp "mini-erp/internal/modules/branch/application"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/apperror"
)

// TestAssistantBranchScopeFromTextAndAccess (AS-01): with the production wiring,
// a console operator can only reach their own branches — by name in the text,
// by explicit pick, or through "all branches" — while a system caller (and a
// whitelisted WhatsApp number) still sees the whole company.
func TestAssistantBranchScopeFromTextAndAccess(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	f3bSeed(t)
	br2, err := testMods.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{
		Code: "BDG", Name: "Cabang Bandung", City: "Bandung",
	})
	if err != nil {
		t.Fatalf("create branch: %v", err)
	}
	var shopName string
	if err := testMods.db.QueryRow(`SELECT name FROM branches WHERE id = ?`, fx.branchID).Scan(&shopName); err != nil {
		t.Fatal(err)
	}

	role := f3bRole(t, "asisten-uji", "assistant.simulate")
	mkUser := func(name string, branches ...int64) int64 {
		var acc []usercontracts.BranchAccess
		for i, b := range branches {
			acc = append(acc, usercontracts.BranchAccess{BranchID: b, IsDefault: i == 0})
		}
		u, err := testMods.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
			Username: name, FullName: name, Password: "rahasia-kuat-1", RoleIDs: []int64{role}, Branches: acc,
		})
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		return u.ID
	}
	onlyBandung := mkUser("hanya-bandung", br2.ID)
	both := mkUser("dua-cabang", fx.branchID, br2.ID)
	svc := testMods.assistant.Service()

	// Operator with Bandung only: "all" is Bandung alone; the shop's name is
	// refused; an explicit pick of the shop is forbidden.
	ans, err := svc.Ask(ctx(), onlyBandung, 0, "omzet hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Text, "Cabang Bandung") || strings.Contains(ans.Text, shopName+":") {
		t.Errorf("only-bandung 'all' = %q", ans.Text)
	}
	ans, err = svc.Ask(ctx(), onlyBandung, 0, "omzet "+shopName+" hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Text, "tidak punya akses") || strings.Contains(ans.Text, "Omzet") {
		t.Errorf("forbidden branch by name = %q", ans.Text)
	}
	_, err = svc.Ask(ctx(), onlyBandung, fx.branchID, "omzet hari ini")
	if ae, ok := err.(*apperror.AppError); !ok || ae.Code != apperror.CodeForbidden {
		t.Errorf("explicit forbidden branch err = %v, want 403", err)
	}

	// Operator with both: naming Bandung narrows to it and labels the answer;
	// the run row records that branch.
	ans, err = svc.Ask(ctx(), both, 0, "penjualan Bandung hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if ans.BranchID != br2.ID || ans.BranchName != "Cabang Bandung" || !strings.HasPrefix(ans.Text, "Cabang: Cabang Bandung\n") {
		t.Errorf("named branch answer = %+v", ans)
	}
	var runBranch int64
	if err := testMods.db.QueryRow(`SELECT branch_id FROM assistant_runs WHERE id = ?`, ans.RunID).Scan(&runBranch); err != nil || runBranch != br2.ID {
		t.Errorf("run branch = %d %v, want %d", runBranch, err, br2.ID)
	}
	// No name: every branch of theirs.
	ans, err = svc.Ask(ctx(), both, 0, "omzet hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Text, "Cabang Bandung") || !strings.Contains(ans.Text, shopName) || ans.BranchID != 0 {
		t.Errorf("both 'all' = %+v", ans)
	}

	// System caller (no actor): unrestricted, still resolves the named branch.
	ans, err = svc.Ask(ctx(), 0, 0, "omzet hari ini")
	if err != nil || !strings.Contains(ans.Text, "Cabang Bandung") || !strings.Contains(ans.Text, shopName) {
		t.Errorf("system 'all' = %+v %v", ans, err)
	}
}

type failingFinance struct{ financecontracts.FinanceClient }

func (failingFinance) NetProfit(context.Context, int64, string, string) (int64, int64, int64, error) {
	return 0, 0, 0, errors.New("ledger offline")
}

// TestAssistantFailedToolIsPersisted (AS-02): a tool that errors is stored as
// a failed tool_execution with its error text, the run is failed, and the
// caller still gets a friendly answer.
func TestAssistantFailedToolIsPersisted(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	gw := assistantinfra.NewGateway(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := assistantapp.NewService(assistantinfra.NewRepository(m.db), gw,
		m.branch.Branches(), m.product.Products(), m.purchasing.PurchaseOrders(), m.sales.SalesOrders(),
		m.stock.Stock(), failingFinance{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)

	ans, err := svc.Ask(ctx(), 0, fx.branchID, "omzet hari ini")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ans.Text, "gagal mengambil data") {
		t.Errorf("answer = %q", ans.Text)
	}
	var runStatus, toolStatus, tool, errText string
	if err := m.db.QueryRow(`SELECT status FROM assistant_runs WHERE id = ?`, ans.RunID).Scan(&runStatus); err != nil || runStatus != "failed" {
		t.Fatalf("run status = %q %v", runStatus, err)
	}
	if err := m.db.QueryRow(`SELECT tool, status, error_text FROM assistant_tool_executions WHERE run_id = ?`, ans.RunID).
		Scan(&tool, &toolStatus, &errText); err != nil {
		t.Fatalf("tool row: %v", err)
	}
	if tool != "sales_summary" || toolStatus != "failed" || errText != "ledger offline" {
		t.Errorf("tool row = %q %q %q", tool, toolStatus, errText)
	}
}
