package application

import (
	"context"
	"sort"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
	"strings"
)

// LedgerAccountRow is one account in the all-accounts general ledger (FN-08):
// where it started, what moved, where it ended. Opening and Closing are in the
// account's normal direction (assets and expenses debit-normal, the rest
// credit-normal), exactly like the single-account ledger's running balance.
type LedgerAccountRow struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Normal  string `json:"normal"` // debit | credit
	Opening int64  `json:"opening"`
	Debit   int64  `json:"debit"`
	Credit  int64  `json:"credit"`
	Closing int64  `json:"closing"`
}

// LedgerOverview lists every account with opening balance or activity in the
// range. TotalDebit/TotalCredit are over ALL accounts, not just the page, so
// they equal the trial balance of the same range.
type LedgerOverview struct {
	Rows        []LedgerAccountRow `json:"rows"`
	Total       int64              `json:"total"`
	TotalDebit  int64              `json:"totalDebit"`
	TotalCredit int64              `json:"totalCredit"`
}

// GeneralLedgerOverview is the general ledger without picking an account: one
// row per account with its opening balance (everything before `from`), the
// debit/credit movement inside [from, to] and the closing balance. Two grouped
// footings feed it — no per-account query.
func (s *Service) GeneralLedgerOverview(ctx context.Context, branchID int64, from, to string, page, limit int) (*LedgerOverview, error) {
	rows, totalDebit, totalCredit, err := s.ledgerAccountRows(ctx, branchID, from, to, fullBooks)
	if err != nil {
		return nil, err
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 500 {
		limit = 100
	}
	out := &LedgerOverview{Total: int64(len(rows)), TotalDebit: totalDebit, TotalCredit: totalCredit}
	lo := (page - 1) * limit
	if lo > len(rows) {
		lo = len(rows)
	}
	hi := lo + limit
	if hi > len(rows) {
		hi = len(rows)
	}
	out.Rows = rows[lo:hi]
	return out, nil
}

// ledgerAccountRows computes the per-account ledger summary within a book scope.
func (s *Service) ledgerAccountRows(ctx context.Context, branchID int64, from, to string, sc bookScope) ([]LedgerAccountRow, int64, int64, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, 0, 0, err
	}
	day, err := timeutil.ParseDateInput(strings.TrimSpace(from))
	if err != nil {
		return nil, 0, 0, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	cutoff := day.AddDate(0, 0, -1).Format("2006-01-02") + " 23:59:59"
	prior, err := s.footings(ctx, branchID, "", cutoff, sc)
	if err != nil {
		return nil, 0, 0, apperror.Internal(err)
	}
	within, err := s.footings(ctx, branchID, start, end, sc)
	if err != nil {
		return nil, 0, 0, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, 0, 0, apperror.Internal(err)
	}
	var rows []LedgerAccountRow
	var totalDebit, totalCredit int64
	for _, a := range accounts {
		p, w := prior[a.ID], within[a.ID]
		if p[0] == 0 && p[1] == 0 && w[0] == 0 && w[1] == 0 {
			continue
		}
		debitNormal := a.Type == contracts.AccountAsset || a.Type == contracts.AccountExpense
		row := LedgerAccountRow{Code: a.Code, Name: a.Name, Type: a.Type, Debit: w[0], Credit: w[1]}
		if debitNormal {
			row.Normal = "debit"
			row.Opening = p[0] - p[1]
			row.Closing = row.Opening + w[0] - w[1]
		} else {
			row.Normal = "credit"
			row.Opening = p[1] - p[0]
			row.Closing = row.Opening + w[1] - w[0]
		}
		totalDebit += w[0]
		totalCredit += w[1]
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Code < rows[j].Code })
	return rows, totalDebit, totalCredit, nil
}
