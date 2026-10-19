package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// validateRange checks YYYY-MM-DD bounds (inclusive, OQ-A32: full last day).
func validateRange(from, to string) (string, string, error) {
	if _, err := timeutil.ParseDateInput(strings.TrimSpace(from)); err != nil {
		return "", "", apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	if _, err := timeutil.ParseDateInput(strings.TrimSpace(to)); err != nil {
		return "", "", apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	if from > to {
		return "", "", apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "tidak boleh sebelum dari"}})
	}
	return from + " 00:00:00", to + " 23:59:59", nil
}

// TrialRow is one account footing.
type TrialRow struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Debit  int64  `json:"debit"`
	Credit int64  `json:"credit"`
}

// bookScope selects WHICH journals a report reads.
//
// The zero value (fullBooks) is the real books. A scope carrying excluded
// entry ids produces the gross-turnover-limited view, and because every
// report below funnels through Service.footings, an excluded transaction
// disappears from ALL of them at once — profit & loss, balance sheet, trial
// balance, VAT summary, fiscal reconciliation. There is no second path a
// dropped order could still be counted through.
type bookScope struct {
	excluded []int64
}

// fullBooks is the ordinary, unfiltered view.
var fullBooks = bookScope{}

// footings is the ONE place reports read account totals from.
func (s *Service) footings(ctx context.Context, branchID int64, from, to string, sc bookScope) (map[int64][2]int64, error) {
	if len(sc.excluded) == 0 {
		return s.repo.AccountFootings(ctx, branchID, from, to)
	}
	return s.repo.AccountFootingsExcluding(ctx, branchID, from, to, sc.excluded)
}

// TrialBalance lists every touched account with footings for a month.
func (s *Service) TrialBalance(ctx context.Context, branchID int64, year, month int) ([]TrialRow, error) {
	return s.trialBalanceScoped(ctx, branchID, year, month, fullBooks)
}

func (s *Service) trialBalanceScoped(ctx context.Context, branchID int64, year, month int, sc bookScope) ([]TrialRow, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	return s.trialBalanceWindow(ctx, branchID, monthStart(year, month), monthEnd(year, month), sc)
}

// trialBalanceWindow foots every touched account over [start, end] timestamps.
func (s *Service) trialBalanceWindow(ctx context.Context, branchID int64, start, end string, sc bookScope) ([]TrialRow, error) {
	footings, err := s.footings(ctx, branchID, start, end, sc)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rows := make([]TrialRow, 0, len(accounts))
	for _, a := range accounts {
		f, ok := footings[a.ID]
		if !ok {
			continue
		}
		rows = append(rows, TrialRow{Code: a.Code, Name: a.Name, Type: a.Type, Debit: f[0], Credit: f[1]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Code < rows[j].Code })
	return rows, nil
}

// TrialBalanceAsOf is the cumulative trial balance: every account's lifetime
// footing up to and including asOf (YYYY-MM-DD), with the balanced flag.
func (s *Service) TrialBalanceAsOf(ctx context.Context, branchID int64, asOf string) ([]TrialRow, error) {
	day, err := timeutil.ParseDateInput(strings.TrimSpace(asOf))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "asOf", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	footings, err := s.footings(ctx, branchID, "", day.Format("2006-01-02")+" 23:59:59", fullBooks)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rows := make([]TrialRow, 0, len(accounts))
	for _, a := range accounts {
		f, ok := footings[a.ID]
		if !ok {
			continue
		}
		rows = append(rows, TrialRow{Code: a.Code, Name: a.Name, Type: a.Type, Debit: f[0], Credit: f[1]})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Code < rows[j].Code })
	return rows, nil
}

func monthStart(year, month int) string {
	return fmt.Sprintf("%04d-%02d-01 00:00:00", year, month)
}

func monthEnd(year, month int) string {
	days := 31
	switch month {
	case 4, 6, 9, 11:
		days = 30
	case 2:
		days = 28
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			days = 29
		}
	}
	return fmt.Sprintf("%04d-%02d-%02d 23:59:59", year, month, days)
}

// PLLine is one account's contribution to a profit-and-loss section (FN-03).
// Amount is signed in the section's natural direction: income accounts credit −
// debit, expense accounts debit − credit, so a section's lines sum to its total.
type PLLine struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Amount int64  `json:"amount"`
}

// ProfitLoss is the income statement. The five totals are the summary; the
// three line lists are the per-account breakdown behind them (the chart of
// accounts is small and the report is a statement, not a browser, so the lists
// are not paginated).
type ProfitLoss struct {
	Revenue      int64    `json:"revenue"`
	Cogs         int64    `json:"cogs"`
	Gross        int64    `json:"gross"`
	Expense      int64    `json:"expense"`
	Profit       int64    `json:"profit"`
	RevenueLines []PLLine `json:"revenueLines"`
	CogsLines    []PLLine `json:"cogsLines"`
	ExpenseLines []PLLine `json:"expenseLines"`
	From         string   `json:"from"`
	To           string   `json:"to"`
}

// ProfitLoss computes revenue − cogs − expenses for a range. Contra accounts
// (4100 Retur Penjualan) net naturally since footing is credit − debit.
func (s *Service) ProfitLoss(ctx context.Context, branchID int64, from, to string) (*ProfitLoss, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	rep, err := s.profitLoss(ctx, branchID, start, end, fullBooks)
	if err != nil {
		return nil, err
	}
	rep.From, rep.To = from, to
	return rep, nil
}

func (s *Service) profitLoss(ctx context.Context, branchID int64, start, end string, sc bookScope) (*ProfitLoss, error) {
	footings, err := s.footings(ctx, branchID, start, end, sc)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	cogsAcc, err := s.repo.MappingAccount(ctx, "cogs")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	var cogsID int64
	if cogsAcc != nil {
		cogsID = cogsAcc.ID
	}
	rep := &ProfitLoss{RevenueLines: []PLLine{}, CogsLines: []PLLine{}, ExpenseLines: []PLLine{}}
	// accounts arrive ordered by code, so every list is already in chart order.
	for _, a := range accounts {
		f, ok := footings[a.ID]
		if !ok {
			continue
		}
		switch a.Type {
		case contracts.AccountIncome:
			amt := f[1] - f[0]
			rep.Revenue += amt
			rep.RevenueLines = append(rep.RevenueLines, PLLine{Code: a.Code, Name: a.Name, Amount: amt})
		case contracts.AccountExpense:
			amt := f[0] - f[1]
			if a.ID == cogsID {
				rep.Cogs += amt
				rep.CogsLines = append(rep.CogsLines, PLLine{Code: a.Code, Name: a.Name, Amount: amt})
			} else {
				rep.Expense += amt
				rep.ExpenseLines = append(rep.ExpenseLines, PLLine{Code: a.Code, Name: a.Name, Amount: amt})
			}
		}
	}
	rep.Gross = rep.Revenue - rep.Cogs
	rep.Profit = rep.Gross - rep.Expense
	return rep, nil
}

// BalanceSheet is assets vs liabilities + equity at a date.
type BalanceSheet struct {
	Assets      []TrialRow `json:"assets"`
	Liabilities []TrialRow `json:"liabilities"`
	Equity      []TrialRow `json:"equity"`
	TotalAssets int64      `json:"totalAssets"`
	TotalLiaEq  int64      `json:"totalLiabilitiesEquity"`
	Balanced    bool       `json:"balanced"`
	Date        string     `json:"date"`
}

// BalanceSheet computes lifetime footings through a date.
func (s *Service) BalanceSheet(ctx context.Context, branchID int64, date string) (*BalanceSheet, error) {
	return s.balanceSheetScoped(ctx, branchID, date, fullBooks)
}

func (s *Service) balanceSheetScoped(ctx context.Context, branchID int64, date string, sc bookScope) (*BalanceSheet, error) {
	if _, err := timeutil.ParseDateInput(strings.TrimSpace(date)); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	footings, err := s.footings(ctx, branchID, "", date+" 23:59:59", sc)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	bs := &BalanceSheet{Date: date}
	add := func(dst *[]TrialRow, a *contracts.Account, balance int64, sum *int64) {
		*dst = append(*dst, TrialRow{Code: a.Code, Name: a.Name, Type: a.Type, Debit: balance, Credit: 0})
		*sum += balance
	}
	for _, a := range accounts {
		f, ok := footings[a.ID]
		if !ok {
			continue
		}
		switch a.Type {
		case contracts.AccountAsset:
			add(&bs.Assets, a, f[0]-f[1], &bs.TotalAssets)
		case contracts.AccountLiability:
			add(&bs.Liabilities, a, f[1]-f[0], &bs.TotalLiaEq)
		case contracts.AccountEquity:
			add(&bs.Equity, a, f[1]-f[0], &bs.TotalLiaEq)
		}
	}
	// Retained earnings: lifetime P&L through the date. Without this, profit
	// would vanish from equity and the sheet could never balance.
	rep, err := s.profitLoss(ctx, branchID, "1000-01-01 00:00:00", date+" 23:59:59", sc)
	if err != nil {
		return nil, err
	}
	if rep.Profit != 0 {
		bs.Equity = append(bs.Equity, TrialRow{Code: "", Name: "Laba ditahan", Type: contracts.AccountEquity, Debit: rep.Profit})
		bs.TotalLiaEq += rep.Profit
	}
	bs.Balanced = bs.TotalAssets == bs.TotalLiaEq
	return bs, nil
}

// LedgerLine is one GL row with running balance.
type LedgerLine struct {
	EntryID int64  `json:"entryId"`
	Number  string `json:"number"`
	Date    string `json:"date"`
	Memo    string `json:"memo"`
	Debit   int64  `json:"debit"`
	Credit  int64  `json:"credit"`
	Balance int64  `json:"balance"`
	// Dimensions of the leg (FN-08), with names resolved in one batch read.
	PartyID     int64  `json:"partyId,omitempty"`
	PartyName   string `json:"partyName,omitempty"`
	// BranchName is set only in the multi-branch tax package ledger.
	BranchName  string `json:"branchName,omitempty"`
	ProductID   int64  `json:"productId,omitempty"`
	ProductName string `json:"productName,omitempty"`
	Description string `json:"description,omitempty"`
}

// LedgerPage is one window of a general ledger with the exact line count.
type LedgerPage struct {
	Lines   []LedgerLine
	Total   int64
	Opening int64
}

// GeneralLedgerPage lists a window of an account's legs, in date order, with a
// running balance computed in SQL over the whole filtered range. The opening
// balance (everything before `from`) seeds it, so a window never pretends to
// start from zero.
func (s *Service) GeneralLedgerPage(ctx context.Context, branchID int64, code, from, to string, page, limit int) (*LedgerPage, error) {
	a, err := s.repo.AccountByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if a == nil {
		return nil, apperror.NotFound("Akun")
	}
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	debitNormal := a.Type == contracts.AccountAsset || a.Type == contracts.AccountExpense
	opening, err := s.openingFooting(ctx, branchID, a.ID, debitNormal, from)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 5000 {
		limit = 1000
	}
	rows, total, err := s.repo.LedgerPage(ctx, branchID, a.ID, start, end, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := &LedgerPage{Total: total, Opening: opening, Lines: make([]LedgerLine, 0, len(rows))}
	// One grouped read per dimension for the whole page, never one per line.
	var partyIDs, productIDs []int64
	for _, r := range rows {
		if r.PartyID != 0 {
			partyIDs = append(partyIDs, r.PartyID)
		}
		if r.ProductID != 0 {
			productIDs = append(productIDs, r.ProductID)
		}
	}
	parties := s.partyLabels(ctx, partyIDs)
	products := s.productLabels(ctx, productIDs)
	for _, r := range rows {
		bal := opening + r.Running
		if !debitNormal {
			bal = opening - r.Running
		}
		line := LedgerLine{EntryID: r.EntryID, Number: r.Number, Date: r.Date,
			Memo: r.Memo, Debit: r.Debit, Credit: r.Credit, Balance: bal,
			PartyID: r.PartyID, ProductID: r.ProductID, Description: r.Description}
		if l, ok := parties[r.PartyID]; ok {
			line.PartyName = l.Name
		}
		if l, ok := products[r.ProductID]; ok {
			line.ProductName = l.Name
		}
		out.Lines = append(out.Lines, line)
	}
	return out, nil
}

// GeneralLedger lists an account's entries with running balance.
// The opening footing (everything before `from`) seeds the running total so
// a filtered range never pretends to start from zero.
func (s *Service) GeneralLedger(ctx context.Context, branchID int64, code, from, to string) ([]LedgerLine, error) {
	a, err := s.repo.AccountByCode(ctx, strings.ToUpper(strings.TrimSpace(code)))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if a == nil {
		return nil, apperror.NotFound("Akun")
	}
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	debitNormal := a.Type == contracts.AccountAsset || a.Type == contracts.AccountExpense
	opening, err := s.openingFooting(ctx, branchID, a.ID, debitNormal, from)
	if err != nil {
		return nil, err
	}
	entries, err := s.repo.ListEntries(ctx, branchID, start, end, a.ID, 10000, 0, true)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// Ascending for running balance.
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
	running := opening
	out := make([]LedgerLine, 0, len(entries)+1)
	if opening != 0 {
		out = append(out, LedgerLine{Date: from, Memo: "Saldo awal", Balance: opening})
	}
	// One grouped read for every entry's legs. Fetching them per entry cost
	// two queries a row, which on a full-year ledger meant tens of thousands
	// of round trips for what a single JOIN returns.
	linesByEntry, err := s.entryLines(ctx, entries)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		for _, l := range linesByEntry[e.ID] {
			if l.AccountID != a.ID {
				continue
			}
			if debitNormal {
				running += l.Debit - l.Credit
			} else {
				running += l.Credit - l.Debit
			}
			out = append(out, LedgerLine{EntryID: e.ID, Number: e.Number,
				Date: e.Date, Memo: e.Memo, Debit: l.Debit, Credit: l.Credit, Balance: running})
		}
	}
	return out, nil
}

// openingFooting nets an account's lifetime activity strictly before `from`
// (from is YYYY-MM-DD; the bound sits at its midnight, before any same-day
// midnight-coerced entry).
func (s *Service) openingFooting(ctx context.Context, branchID, accountID int64, debitNormal bool, from string) (int64, error) {
	day, err := timeutil.ParseDateInput(strings.TrimSpace(from))
	if err != nil {
		return 0, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	cutoff := day.AddDate(0, 0, -1).Format("2006-01-02") + " 23:59:59"
	footings, err := s.repo.AccountFootings(ctx, branchID, "", cutoff)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	f, ok := footings[accountID]
	if !ok {
		return 0, nil
	}
	if debitNormal {
		return f[0] - f[1], nil
	}
	return f[1] - f[0], nil
}

// InventoryPosition is re-exported from contracts for L8 readers.
type InventoryPosition = contracts.InventoryPosition

// InventoryTotal values every cost position and returns only the sum.
//
// The dashboard wants one number, not a named catalog: this skips product
// decoration entirely, leaving a single positions read.
func (s *Service) InventoryTotal(ctx context.Context, branchID int64) (int64, error) {
	positions, err := s.repo.CostPositions(ctx, branchID)
	if err != nil {
		return 0, apperror.Internal(err)
	}
	var total int64
	for _, p := range positions {
		if p.Qty <= 0 {
			continue
		}
		total += money.Round(p.Qty * p.Average())
	}
	return total, nil
}

// BranchValue is one branch's share of a multi-branch figure (FN-09).
type BranchValue struct {
	BranchID   int64  `json:"branchId"`
	BranchName string `json:"branchName"`
	Total      int64  `json:"total"`
	Positions  int    `json:"positions"`
}

// InventoryReport is the inventory valuation with, for an "all branches"
// scope, the per-branch split (FN-09).
type InventoryReport struct {
	Positions []InventoryPosition
	Total     int64
	ByBranch  []BranchValue
}

// InventoryValue values every cost position at moving average, decorated
// with product code/name. Single-branch view; see InventoryReport for the
// multi-branch one.
func (s *Service) InventoryValue(ctx context.Context, branchID int64) ([]InventoryPosition, int64, error) {
	rep, err := s.InventoryReport(ctx, branchID)
	if err != nil {
		return nil, 0, err
	}
	return rep.Positions, rep.Total, nil
}

// InventoryReport values the cost positions of the branch in scope: the session
// branch, or every branch of an "all branches" report (the handler narrowed the
// ctx to what the user may see, FN-09). Positions of the same product merge
// across branches (quantity and value add up); ByBranch keeps the split.
//
// Cost: one positions read per branch plus ONE grouped product read and one
// branch-name read, whatever the catalog size.
func (s *Service) InventoryReport(ctx context.Context, branchID int64) (*InventoryReport, error) {
	scope := []int64{branchID}
	if ids, ok := infrastructure.BranchSetFrom(ctx); ok {
		scope = ids
	}
	type key struct{ product, variant int64 }
	merged := map[key]*InventoryPosition{}
	rep := &InventoryReport{}
	for _, b := range scope {
		positions, err := s.repo.CostPositions(ctx, b)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		bv := BranchValue{BranchID: b}
		for _, p := range positions {
			if p.Qty <= 0 {
				continue
			}
			avg := p.Average()
			value := money.Round(p.Qty * avg)
			bv.Total += value
			bv.Positions++
			k := key{p.ProductID, p.VariantID}
			cur, ok := merged[k]
			if !ok {
				merged[k] = &InventoryPosition{ProductID: p.ProductID, VariantID: p.VariantID, Qty: p.Qty, AvgCost: avg, Value: value}
				continue
			}
			cur.Qty += p.Qty
			cur.Value += value
			cur.AvgCost = float64(cur.Value) / cur.Qty
		}
		rep.Total += bv.Total
		rep.ByBranch = append(rep.ByBranch, bv)
	}
	out := make([]InventoryPosition, 0, len(merged))
	ids := make([]int64, 0, len(merged))
	for _, p := range merged {
		out = append(out, *p)
		ids = append(ids, p.ProductID)
	}
	labels := s.productLabels(ctx, ids)
	for i := range out {
		if l, ok := labels[out[i].ProductID]; ok {
			out[i].ProductCode, out[i].ProductName = l.Code, l.Name
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProductCode == out[j].ProductCode {
			return out[i].VariantID < out[j].VariantID
		}
		return out[i].ProductCode < out[j].ProductCode
	})
	rep.Positions = out
	names := s.branchNames(ctx, scope)
	for i := range rep.ByBranch {
		rep.ByBranch[i].BranchName = names[rep.ByBranch[i].BranchID]
	}
	return rep, nil
}

// branchNames resolves branch names with one grouped read (best effort).
func (s *Service) branchNames(ctx context.Context, ids []int64) map[int64]string {
	names := map[int64]string{}
	if len(ids) == 0 || s.branches == nil {
		return names
	}
	list, err := s.branches.GetByIDs(ctx, ids)
	if err != nil {
		return names
	}
	for _, b := range list {
		names[b.ID] = b.Name
	}
	return names
}

// BranchMargin is one branch's margin inside a multi-branch Margin (FN-09).
type BranchMargin struct {
	BranchID   int64   `json:"branchId"`
	BranchName string  `json:"branchName"`
	Revenue    int64   `json:"revenue"`
	Cogs       int64   `json:"cogs"`
	Gross      int64   `json:"gross"`
	Percent    float64 `json:"percent"`
}

// Margin is revenue/cogs/gross for a range. ByBranch is filled when the report
// spans an "all branches" scope: it carries one row per branch and sums to the
// totals above.
type Margin struct {
	Revenue  int64          `json:"revenue"`
	Cogs     int64          `json:"cogs"`
	Gross    int64          `json:"gross"`
	Percent  float64        `json:"percent"`
	ByBranch []BranchMargin `json:"byBranch,omitempty"`
}

func marginPercent(gross, revenue int64) float64 {
	if revenue <= 0 {
		return 0
	}
	return float64(gross) * 100 / float64(revenue)
}

// Margin computes the gross margin from the same core as P&L.
func (s *Service) Margin(ctx context.Context, branchID int64, from, to string) (*Margin, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	rep, err := s.profitLoss(ctx, branchID, start, end, fullBooks)
	if err != nil {
		return nil, err
	}
	m := &Margin{Revenue: rep.Revenue, Cogs: rep.Cogs, Gross: rep.Gross, Percent: marginPercent(rep.Gross, rep.Revenue)}
	if ids, ok := infrastructure.BranchSetFrom(ctx); ok && len(ids) > 1 {
		// "All branches": the same core, once per branch, so the split always
		// sums to the total (FN-09).
		names := s.branchNames(ctx, ids)
		for _, b := range ids {
			one, err := s.profitLoss(infrastructure.WithBranchSet(ctx, []int64{b}), b, start, end, fullBooks)
			if err != nil {
				return nil, err
			}
			m.ByBranch = append(m.ByBranch, BranchMargin{BranchID: b, BranchName: names[b], Revenue: one.Revenue,
				Cogs: one.Cogs, Gross: one.Gross, Percent: marginPercent(one.Gross, one.Revenue)})
		}
	}
	return m, nil
}

// CashRow is one cash/bank account position for a month.
type CashRow struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Debit  int64  `json:"debit"`
	Credit int64  `json:"credit"`
	// Balance is debit-normal net (cash accounts are assets).
	Balance int64 `json:"balance"`
}

// CashFlowRow is one cash account over a range: where it started, what came in
// and went out, and where it ended.
type CashFlowRow struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Opening int64  `json:"opening"`
	In      int64  `json:"in"`
	Out     int64  `json:"out"`
	Closing int64  `json:"closing"`
}

// CashFlow reports the cash accounts' opening balance (everything before
// from), receipts (debits), payments (credits) and closing balance for an
// inclusive date range. Opening + in - out = closing by construction.
func (s *Service) CashFlow(ctx context.Context, branchID int64, from, to string) ([]CashFlowRow, int64, int64, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, 0, 0, err
	}
	day, err := timeutil.ParseDateInput(strings.TrimSpace(from))
	if err != nil {
		return nil, 0, 0, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	before := day.AddDate(0, 0, -1).Format("2006-01-02") + " 23:59:59"
	prior, err := s.repo.AccountFootings(ctx, branchID, "", before)
	if err != nil {
		return nil, 0, 0, apperror.Internal(err)
	}
	within, err := s.repo.AccountFootings(ctx, branchID, start, end)
	if err != nil {
		return nil, 0, 0, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, 0, 0, apperror.Internal(err)
	}
	rows := []CashFlowRow{}
	var openingTotal, closingTotal int64
	for _, a := range accounts {
		if !a.IsCash {
			continue
		}
		p, w := prior[a.ID], within[a.ID]
		if _, ok := prior[a.ID]; !ok {
			if _, ok := within[a.ID]; !ok {
				continue
			}
		}
		opening := p[0] - p[1]
		row := CashFlowRow{Code: a.Code, Name: a.Name, Opening: opening, In: w[0], Out: w[1], Closing: opening + w[0] - w[1]}
		rows = append(rows, row)
		openingTotal += row.Opening
		closingTotal += row.Closing
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Code < rows[j].Code })
	return rows, openingTotal, closingTotal, nil
}

// CashSummary lists month footings for is_cash accounts — the cash report
// without a cash-accounts table (H-revisi): the flag on the account IS the
// registry, so the report is one grouped footing + one account list (P3).
func (s *Service) CashSummary(ctx context.Context, branchID int64, year, month int) ([]CashRow, int64, error) {
	if !validYearMonth(year, month) {
		return nil, 0, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, monthStart(year, month), monthEnd(year, month))
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	rows := []CashRow{}
	var total int64
	for _, a := range accounts {
		if !a.IsCash {
			continue
		}
		f, ok := footings[a.ID]
		if !ok {
			continue
		}
		bal := f[0] - f[1]
		rows = append(rows, CashRow{Code: a.Code, Name: a.Name, Debit: f[0], Credit: f[1], Balance: bal})
		total += bal
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Code < rows[j].Code })
	return rows, total, nil
}
