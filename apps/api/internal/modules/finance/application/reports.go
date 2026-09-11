package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/apperror"
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

// TrialBalance lists every touched account with footings for a month.
func (s *Service) TrialBalance(ctx context.Context, branchID int64, year, month int) ([]TrialRow, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	start := monthStart(year, month)
	end := monthEnd(year, month)
	footings, err := s.repo.AccountFootings(ctx, branchID, start, end)
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

// ProfitLoss is the income statement.
type ProfitLoss struct {
	Revenue int64  `json:"revenue"`
	Cogs    int64  `json:"cogs"`
	Gross   int64  `json:"gross"`
	Expense int64  `json:"expense"`
	Profit  int64  `json:"profit"`
	From    string `json:"from"`
	To      string `json:"to"`
}

// ProfitLoss computes revenue − cogs − expenses for a range. Contra accounts
// (4100 Retur Penjualan) net naturally since footing is credit − debit.
func (s *Service) ProfitLoss(ctx context.Context, branchID int64, from, to string) (*ProfitLoss, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	rep, err := s.profitLoss(ctx, branchID, start, end)
	if err != nil {
		return nil, err
	}
	rep.From, rep.To = from, to
	return rep, nil
}

func (s *Service) profitLoss(ctx context.Context, branchID int64, start, end string) (*ProfitLoss, error) {
	footings, err := s.repo.AccountFootings(ctx, branchID, start, end)
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
	rep := &ProfitLoss{}
	for _, a := range accounts {
		f, ok := footings[a.ID]
		if !ok {
			continue
		}
		switch a.Type {
		case contracts.AccountIncome:
			rep.Revenue += f[1] - f[0]
		case contracts.AccountExpense:
			if a.ID == cogsID {
				rep.Cogs += f[0] - f[1]
			} else {
				rep.Expense += f[0] - f[1]
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
	if _, err := timeutil.ParseDateInput(strings.TrimSpace(date)); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, "", date+" 23:59:59")
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
	rep, err := s.profitLoss(ctx, branchID, "1000-01-01 00:00:00", date+" 23:59:59")
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
	for _, e := range entries {
		full, err := s.repo.GetEntry(ctx, e.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, l := range full.Lines {
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

// TaxSummary is one month's VAT position.
type TaxSummary struct {
	Year    int   `json:"year"`
	Month   int   `json:"month"`
	PpnOut  int64 `json:"ppnOut"`
	PpnIn   int64 `json:"ppnIn"`
	Payable int64 `json:"payable"`
}

// TaxSummary nets PPN Keluaran credits against PPN Masukan debits.
func (s *Service) TaxSummary(ctx context.Context, branchID int64, year, month int) (*TaxSummary, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	out, err := s.repo.AccountByCode(ctx, "2200")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	in, err := s.repo.AccountByCode(ctx, "1400")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, monthStart(year, month), monthEnd(year, month))
	if err != nil {
		return nil, apperror.Internal(err)
	}
	sum := &TaxSummary{Year: year, Month: month}
	if out != nil {
		if f, ok := footings[out.ID]; ok {
			sum.PpnOut = f[1] - f[0]
		}
	}
	if in != nil {
		if f, ok := footings[in.ID]; ok {
			sum.PpnIn = f[0] - f[1]
		}
	}
	sum.Payable = sum.PpnOut - sum.PpnIn
	return sum, nil
}

// TaxDetailRow is one revenue posting for export.
type TaxDetailRow struct {
	Date             string `json:"date"`
	Number           string `json:"number"`
	Memo             string `json:"memo"`
	Revenue          int64  `json:"revenue"`
	Ppn              int64  `json:"ppn"`
	TaxInvoiceNumber string `json:"taxInvoiceNumber"`
	TaxInvoiceDate   string `json:"taxInvoiceDate"`
}

// TaxDetail lists delivery-sourced entries in a month (SPT working papers).
// Invoice numbers come from the source SO (C3); SOs repeat across rows, so
// each distinct order is fetched once.
func (s *Service) TaxDetail(ctx context.Context, branchID int64, year, month int) ([]TaxDetailRow, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	entries, err := s.repo.ListEntries(ctx, branchID, monthStart(year, month), monthEnd(year, month), 0, 10000, 0, true)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	rev, err := s.repo.AccountByCode(ctx, "4000")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	ppn, err := s.repo.AccountByCode(ctx, "2200")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	invoices := map[int64][2]string{}
	noteOrders := map[int64]int64{}
	getSO := func(noteID int64) (string, string) {
		soID, ok := noteOrders[noteID]
		if !ok {
			if note, err := s.delivery.GetByID(ctx, noteID); err == nil && note != nil {
				soID = note.SalesOrderID
			}
			noteOrders[noteID] = soID
		}
		if soID == 0 {
			return "", ""
		}
		inv, ok := invoices[soID]
		if !ok {
			if so, err := s.sales.GetByID(ctx, soID); err == nil && so != nil && so.BranchID == branchID {
				inv = [2]string{so.TaxInvoiceNumber, so.TaxInvoiceDate}
			}
			invoices[soID] = inv
		}
		return inv[0], inv[1]
	}
	var out []TaxDetailRow
	for _, e := range entries {
		if e.SourceType != SourceDelivery {
			continue
		}
		full, err := s.repo.GetEntry(ctx, e.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		row := TaxDetailRow{Date: e.Date, Number: e.Number, Memo: e.Memo}
		for _, l := range full.Lines {
			if rev != nil && l.AccountID == rev.ID {
				row.Revenue += l.Credit - l.Debit
			}
			if ppn != nil && l.AccountID == ppn.ID {
				row.Ppn += l.Credit - l.Debit
			}
		}
		row.TaxInvoiceNumber, row.TaxInvoiceDate = getSO(e.SourceID)
		out = append(out, row)
	}
	if out == nil {
		out = []TaxDetailRow{}
	}
	return out, nil
}

// InventoryPosition is re-exported from contracts for L8 readers.
type InventoryPosition = contracts.InventoryPosition

// InventoryValue values every cost position at moving average.
func (s *Service) InventoryValue(ctx context.Context, branchID int64) ([]InventoryPosition, int64, error) {
	positions, err := s.repo.CostPositions(ctx, branchID)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	var out []InventoryPosition
	var total int64
	for _, p := range positions {
		if p.Qty <= 0 {
			continue
		}
		avg := p.Average()
		value := round(p.Qty * avg)
		item := InventoryPosition{ProductID: p.ProductID, VariantID: p.VariantID,
			Qty: p.Qty, AvgCost: avg, Value: value}
		if prod, err := s.products.GetByID(ctx, p.ProductID); err == nil && prod != nil {
			item.ProductCode, item.ProductName = prod.Code, prod.Name
		}
		out = append(out, item)
		total += value
	}
	if out == nil {
		out = []InventoryPosition{}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ProductCode == out[j].ProductCode {
			return out[i].VariantID < out[j].VariantID
		}
		return out[i].ProductCode < out[j].ProductCode
	})
	return out, total, nil
}

// Margin is revenue/cogs/gross for a range.
type Margin struct {
	Revenue int64   `json:"revenue"`
	Cogs    int64   `json:"cogs"`
	Gross   int64   `json:"gross"`
	Percent float64 `json:"percent"`
}

// Margin computes the gross margin from the same core as P&L.
func (s *Service) Margin(ctx context.Context, branchID int64, from, to string) (*Margin, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	rep, err := s.profitLoss(ctx, branchID, start, end)
	if err != nil {
		return nil, err
	}
	m := &Margin{Revenue: rep.Revenue, Cogs: rep.Cogs, Gross: rep.Gross}
	if m.Revenue > 0 {
		m.Percent = float64(m.Gross) * 100 / float64(m.Revenue)
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
