package integration

import (
	"context"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	productapp "mini-erp/internal/modules/product/application"
	salesapp "mini-erp/internal/modules/sales/application"
)

// bigTicketPrice / bigTicketCost price a product so that a handful of units
// crosses the Rp4,8 M ceiling, keeping fixtures small and the arithmetic
// obvious: 2 M per unit → the third unit breaches.
const (
	bigTicketPrice = 2_000_000_000
	bigTicketCost  = 1_000_000_000
)

// newBigTicket swaps the fixture onto a high-value product whose stock comes
// from a RECEIPT, not from an opening adjustment.
//
// That distinction matters: adjustment stock carries no cost, so deliveries
// of it book zero COGS. Buying it in gives the cost ledger a real basis, and
// only then can a test prove that excluding an order removes its COGS too —
// not just its revenue.
func newBigTicket(t *testing.T, ctx context.Context, fx shopFixture) shopFixture {
	t.Helper()
	p, err := testMods.product.Service().CreateProduct(ctx, 0, productapp.ProductInput{
		Code: "BIG-001", Name: "Barang Nilai Besar", Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs",
		PurchaseFactor: 1, SalesFactor: 1,
		PurchasePrice: bigTicketCost,
		SellingPrice:  bigTicketPrice,
	})
	if err != nil {
		t.Fatalf("create big-ticket product: %v", err)
	}
	fx.productID = p.ID
	fx.variantID = p.Variants[0].ID
	receivePO(t, fx, bigTicketCost, 20)
	return fx
}

// sellAndPost creates → confirms → ships → posts one order on a given date,
// returning the sales order id. Posting is what puts revenue in the books,
// which is the only thing the ceiling reads.
func sellAndPost(t *testing.T, ctx context.Context, fx shopFixture, qty float64, date string) int64 {
	t.Helper()
	m := testMods
	so, err := m.sales.Service().CreateOrder(ctx, 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		OrderDate: date,
		Items: []salesapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty,
		}},
	})
	if err != nil {
		t.Fatalf("create SO: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx, 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	dn, err := m.delivery.Service().Create(ctx, 0, fx.branchID, so.ID, date, "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: qty,
		}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx, 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	if _, err := m.finance.Service().Post(ctx, 0, fx.branchID,
		financeapp.SourceDelivery, dn.ID); err != nil {
		t.Fatalf("post delivery: %v", err)
	}
	return so.ID
}

// TestGrossTurnoverCeiling locks the PP23 ceiling semantics: turnover is
// accumulated year-to-date in chronological order, and the order that breaks
// the ceiling is dropped WHOLE — not trimmed to fit.
func TestGrossTurnoverCeiling(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fx = newBigTicket(t, ctx(), fx)

	// 2 M (Jan) + 2 M (Feb) = 4 M under the ceiling; the March order would
	// take the running total to 6 M, so March falls outside.
	janSO := sellAndPost(t, ctx(), fx, 1, "2026-01-15")
	febSO := sellAndPost(t, ctx(), fx, 1, "2026-02-15")
	marSO := sellAndPost(t, ctx(), fx, 1, "2026-03-15")

	basis, err := testMods.finance.Service().GrossTurnoverBasis(ctx(), 2026, 3)
	if err != nil {
		t.Fatalf("turnover basis: %v", err)
	}
	if basis.Ceiling != financeapp.GrossTurnoverCeiling {
		t.Fatalf("ceiling = %d, want %d", basis.Ceiling, financeapp.GrossTurnoverCeiling)
	}
	if basis.IncludedTurnover != 2*bigTicketPrice {
		t.Fatalf("included turnover = %d, want %d", basis.IncludedTurnover, 2*bigTicketPrice)
	}
	if basis.ExcludedTurnover != bigTicketPrice {
		t.Fatalf("excluded turnover = %d, want %d", basis.ExcludedTurnover, bigTicketPrice)
	}

	state := map[int64]bool{}
	for _, o := range basis.Orders {
		state[o.OrderID] = o.Included
	}
	if !state[janSO] || !state[febSO] {
		t.Fatalf("January/February must stay inside the ceiling: %+v", state)
	}
	if state[marSO] {
		t.Fatal("March order breached the ceiling but was kept")
	}
	// The breach drops the order ENTIRELY — its revenue is not trimmed to
	// the remaining headroom. Partial inclusion would invent a sale that
	// never happened at that amount.
	if basis.IncludedTurnover+basis.ExcludedTurnover != 3*bigTicketPrice {
		t.Fatalf("turnover split lost money: %d + %d", basis.IncludedTurnover, basis.ExcludedTurnover)
	}
	if basis.ExcludedCount() == 0 {
		t.Fatal("excluded order contributed no excluded journal entries")
	}
}

// TestGrossTurnoverIsCumulativeYearToDate proves the ceiling is a YEAR
// figure, not a per-month one: asking for February must already carry
// January's turnover, otherwise every month would look far under the limit.
func TestGrossTurnoverIsCumulativeYearToDate(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fx = newBigTicket(t, ctx(), fx)

	sellAndPost(t, ctx(), fx, 2, "2026-01-20") // 4 M
	sellAndPost(t, ctx(), fx, 1, "2026-02-20") // +2 M → breach

	jan, err := testMods.finance.Service().GrossTurnoverBasis(ctx(), 2026, 1)
	if err != nil {
		t.Fatalf("january basis: %v", err)
	}
	if jan.IncludedTurnover != 2*bigTicketPrice || jan.ExcludedTurnover != 0 {
		t.Fatalf("january: included=%d excluded=%d, want 4 M / 0",
			jan.IncludedTurnover, jan.ExcludedTurnover)
	}
	feb, err := testMods.finance.Service().GrossTurnoverBasis(ctx(), 2026, 2)
	if err != nil {
		t.Fatalf("february basis: %v", err)
	}
	// February alone is only 2 M — well under the ceiling. It is excluded
	// only because January's 4 M is counted first. That is the whole point.
	if feb.ExcludedTurnover != bigTicketPrice {
		t.Fatalf("february excluded = %d, want %d (cumulative basis not applied)",
			feb.ExcludedTurnover, bigTicketPrice)
	}
	if feb.IncludedTurnover != 2*bigTicketPrice {
		t.Fatalf("february included = %d, want %d", feb.IncludedTurnover, 2*bigTicketPrice)
	}
}

// TestTaxPackageVariantsDiffer renders both working papers and asserts they
// are real, distinct artefacts — and that neither leaks the internal
// "layer" vocabulary into its file name.
func TestTaxPackageVariantsDiffer(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fx = newBigTicket(t, ctx(), fx)
	sellAndPost(t, ctx(), fx, 3, "2026-01-10") // 6 M → breach

	svc := testMods.finance.Service()
	actual, actualName, err := svc.TaxPackage(ctx(), fx.branchID, 2026, 1, financeapp.PackageActual)
	if err != nil {
		t.Fatalf("actual package: %v", err)
	}
	capped, cappedName, err := svc.TaxPackage(ctx(), fx.branchID, 2026, 1, financeapp.PackageCapped)
	if err != nil {
		t.Fatalf("capped package: %v", err)
	}
	if len(actual) == 0 || len(capped) == 0 {
		t.Fatal("a package rendered empty")
	}
	if actualName == cappedName {
		t.Fatalf("both variants produced the same file name %q", actualName)
	}
	for _, name := range []string{actualName, cappedName} {
		if containsFold(name, "layer") {
			t.Fatalf("file name %q leaks internal layer vocabulary", name)
		}
		if !hasSuffix(name, ".xlsx") {
			t.Fatalf("file name %q is not an .xlsx workbook", name)
		}
	}
	if _, _, err := svc.TaxPackage(ctx(), fx.branchID, 2026, 1, "layer2"); err == nil {
		t.Fatal("unknown variant was accepted")
	}
}

func containsFold(s, sub string) bool {
	ls, lsub := toLower(s), toLower(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func hasSuffix(s, suf string) bool {
	return len(s) >= len(suf) && s[len(s)-len(suf):] == suf
}

func toLower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

// TestGrossTurnoverIncludesLastDayOfMonth guards a boundary that is easy to
// get wrong and silent when wrong: entry_date is DATETIME, so a bound passed
// as a bare "YYYY-MM-DD" is read as midnight and drops everything booked
// later on the closing day. A month's last-day sales must count.
func TestGrossTurnoverIncludesLastDayOfMonth(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fx = newBigTicket(t, ctx(), fx)

	// 31 January — the last day of the month, and the only turnover there is.
	sellAndPost(t, ctx(), fx, 1, "2026-01-31")

	basis, err := testMods.finance.Service().GrossTurnoverBasis(ctx(), 2026, 1)
	if err != nil {
		t.Fatalf("turnover basis: %v", err)
	}
	if basis.IncludedTurnover != bigTicketPrice {
		t.Fatalf("last-day turnover = %d, want %d — closing-day sales were dropped",
			basis.IncludedTurnover, bigTicketPrice)
	}
	if len(basis.Orders) != 1 {
		t.Fatalf("orders = %d, want 1", len(basis.Orders))
	}
}

// TestCappedViewIsConsistentEverywhere is the guarantee that matters: an
// order pushed outside the ceiling must not be counted ANYWHERE in the
// limited working paper. Not in profit & loss, not in the balance sheet, not
// in the trial balance, not in VAT. One exclusion list, applied once, at the
// single point every report reads its totals from.
func TestCappedViewIsConsistentEverywhere(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fx = newBigTicket(t, ctx(), fx)

	// 2 M + 2 M fit; the third 2 M breaches and is dropped whole.
	sellAndPost(t, ctx(), fx, 1, "2026-01-10")
	sellAndPost(t, ctx(), fx, 1, "2026-01-15")
	sellAndPost(t, ctx(), fx, 1, "2026-01-20")

	svc := testMods.finance.Service()
	basis, err := svc.GrossTurnoverBasis(ctx(), 2026, 1)
	if err != nil {
		t.Fatalf("basis: %v", err)
	}
	if basis.ExcludedTurnover != bigTicketPrice {
		t.Fatalf("excluded turnover = %d, want %d", basis.ExcludedTurnover, bigTicketPrice)
	}

	// Both workbooks are rendered from this same call — the test therefore
	// checks the figures the sheets actually carry, not a parallel formula.
	full, err := svc.Reports(ctx(), fx.branchID, 2026, 1, financeapp.PackageActual)
	if err != nil {
		t.Fatalf("actual reports: %v", err)
	}
	capped, err := svc.Reports(ctx(), fx.branchID, 2026, 1, financeapp.PackageCapped)
	if err != nil {
		t.Fatalf("capped reports: %v", err)
	}

	// 1. Profit & loss: revenue drops by exactly the excluded turnover.
	if got := full.ProfitLoss.Revenue - capped.ProfitLoss.Revenue; got != bigTicketPrice {
		t.Fatalf("revenue drop = %d, want %d", got, bigTicketPrice)
	}
	// 2. COGS drops too — the excluded sale took its cost with it. Removing
	//    revenue alone would leave the margin nonsensical.
	if capped.ProfitLoss.Cogs >= full.ProfitLoss.Cogs {
		t.Fatalf("COGS did not drop: full=%d capped=%d",
			full.ProfitLoss.Cogs, capped.ProfitLoss.Cogs)
	}
	// 3. The balance sheet still balances after whole entries were removed.
	if !capped.Balance.Balanced {
		t.Fatalf("limited balance sheet does not balance: assets=%d liab+eq=%d",
			capped.Balance.TotalAssets, capped.Balance.TotalLiaEq)
	}
	// 4. Trial balance drops as well, and stays square.
	var fullDebit, capDebit, capCredit int64
	for _, r := range full.Trial {
		fullDebit += r.Debit
	}
	for _, r := range capped.Trial {
		capDebit += r.Debit
		capCredit += r.Credit
	}
	if capDebit >= fullDebit {
		t.Fatalf("trial balance debit did not drop: full=%d capped=%d", fullDebit, capDebit)
	}
	if capDebit != capCredit {
		t.Fatalf("limited trial balance is lopsided: %d vs %d", capDebit, capCredit)
	}
	// 5. The VAT working paper lists only the surviving rows.
	if len(capped.Detail) >= len(full.Detail) {
		t.Fatalf("VAT rows not filtered: full=%d capped=%d",
			len(full.Detail), len(capped.Detail))
	}
	// 6. Every sheet agrees with the SAME exclusion list.
	if capped.ProfitLoss.Revenue != basis.IncludedTurnover {
		t.Fatalf("profit & loss revenue %d disagrees with the ceiling working %d",
			capped.ProfitLoss.Revenue, basis.IncludedTurnover)
	}
	if capped.Summary.PpnOut > full.Summary.PpnOut {
		t.Fatal("limited VAT payable exceeds the real books")
	}
}
