package application

import (
	"context"
	"strings"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// trimDate strips surrounding whitespace from a YYYY-MM-DD input. validateRange
// accepts padded input for parsing but echoes it raw, so the dense-series loop
// below must normalise before parsing again.
func trimDate(s string) string {
	return strings.TrimSpace(s)
}

// DailyTrend returns per-day revenue/expense/profit for an inclusive
// YYYY-MM-DD range, days with no postings included as zeroes.
//
// Query count: exactly 3, all constant in the range length — ListAccounts,
// MappingAccount(cogs), and one GROUP BY DATE(entry_date),account_id footing
// query. The old per-day NetProfit loop needed 3N+ queries for N days (P3).
//
// Semantics mirror profitLoss exactly (revenue nets contra-income, expense
// excludes COGS, profit = revenue − cogs − expense), so replacing the loop
// with this call cannot drift the reported numbers.
func (s *Service) DailyTrend(ctx context.Context, branchID int64, from, to string) ([]contracts.DailyPoint, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
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
	typeOf := make(map[int64]string, len(accounts))
	for _, a := range accounts {
		typeOf[a.ID] = a.Type
	}
	rows, err := s.repo.DailyFootings(ctx, branchID, start, end)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byDay := map[string]*contracts.DailyPoint{}
	cogsByDay := map[string]int64{}
	for _, r := range rows {
		p, ok := byDay[r.Date]
		if !ok {
			p = &contracts.DailyPoint{Date: r.Date}
			byDay[r.Date] = p
		}
		switch typeOf[r.AccountID] {
		case contracts.AccountIncome:
			p.Revenue += r.Credit - r.Debit
		case contracts.AccountExpense:
			if r.AccountID == cogsID {
				cogsByDay[r.Date] += r.Debit - r.Credit
			} else {
				p.Expense += r.Debit - r.Credit
			}
		}
	}
	// Dense series: every calendar day in the range is emitted, zeroes when
	// the books have nothing that day, so charts never show gaps.
	first, err := timeutil.ParseDateInput(trimDate(from))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	last, err := timeutil.ParseDateInput(trimDate(to))
	if err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	out := make([]contracts.DailyPoint, 0, int(last.Sub(first).Hours()/24)+1)
	for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
		day := d.Format("2006-01-02")
		p, ok := byDay[day]
		if !ok {
			p = &contracts.DailyPoint{Date: day}
		}
		p.Profit = p.Revenue - cogsByDay[day] - p.Expense
		out = append(out, *p)
	}
	return out, nil
}
