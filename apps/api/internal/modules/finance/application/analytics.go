package application

import (
	"context"
	"sort"
	"strings"

	financecontracts "mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// endOfDay turns an optional YYYY-MM-DD cutoff into an inclusive timestamp,
// defaulting to today in Jakarta. Balances are asked "as of" a day, and a
// bare date would silently exclude everything booked during that day.
func endOfDay(asOf string) (string, error) {
	asOf = strings.TrimSpace(asOf)
	if asOf == "" {
		asOf = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	} else if _, err := timeutil.ParseDateInput(asOf); err != nil {
		return "", apperror.Validation("", []apperror.FieldError{{Field: "asOf", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	return asOf + " 23:59:59", nil
}

// Laporan yang berdiri di atas dimensi analitik baris jurnal
// (migrasi 000019). Semuanya membaca BUKU, bukan tabel operasional: itu
// bedanya "margin menurut catatan penjualan" dan "margin menurut pembukuan".

// ProductMargin is re-exported from contracts so dashboard/reporting readers
// can consume MarginByProduct through the FinanceClient interface (Rule A:
// other modules may only import contracts/).
type ProductMargin = financecontracts.ProductMargin

// MarginByProduct ranks products by realised gross margin.
//
// Revenue nets sales against sales returns (4000 credit − 4000 debit − 4100
// net) and COGS nets deliveries against restocks, so a returned sale removes
// both its revenue and its cost instead of leaving a phantom margin.
//
// Cost: one grouped query for the whole report plus one product lookup per
// distinct product in the result — never per journal line.
func (s *Service) MarginByProduct(ctx context.Context, branchID int64, from, to string, limit int) ([]ProductMargin, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	revenue, err := s.repo.AccountByCode(ctx, "4000")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	salesReturn, err := s.repo.AccountByCode(ctx, "4100")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	cogs, err := s.repo.AccountByCode(ctx, "5000")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if revenue == nil || salesReturn == nil || cogs == nil {
		return []ProductMargin{}, nil
	}
	footings, err := s.repo.ProductFootings(ctx, branchID, start, end,
		[]int64{revenue.ID, salesReturn.ID, cogs.ID})
	if err != nil {
		return nil, apperror.Internal(err)
	}
	type key struct{ product, variant int64 }
	acc := map[key]*ProductMargin{}
	for _, f := range footings {
		k := key{f.ProductID, f.VariantID}
		row, ok := acc[k]
		if !ok {
			row = &ProductMargin{ProductID: f.ProductID, VariantID: f.VariantID}
			acc[k] = row
		}
		switch f.AccountID {
		case revenue.ID:
			// Income account: credit raises revenue, debit (a correction)
			// lowers it.
			row.Revenue += f.Credit - f.Debit
		case salesReturn.ID:
			// Contra-income: debit lowers revenue.
			row.Revenue -= f.Debit - f.Credit
		case cogs.ID:
			// Expense account: debit raises cost, credit (restock) lowers it.
			row.Cogs += f.Debit - f.Credit
		}
	}
	out := make([]ProductMargin, 0, len(acc))
	for _, row := range acc {
		row.Gross = row.Revenue - row.Cogs
		if row.Revenue > 0 {
			row.Percent = float64(row.Gross) * 100 / float64(row.Revenue)
		}
		if p, err := s.products.GetByID(ctx, row.ProductID); err == nil && p != nil {
			row.ProductCode, row.ProductName = p.Code, p.Name
		}
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Gross == out[j].Gross {
			return out[i].ProductCode < out[j].ProductCode
		}
		return out[i].Gross > out[j].Gross
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// PartyBalance is one counterparty's outstanding balance from the books.
type PartyBalance struct {
	PartyID   int64  `json:"partyId"`
	PartyName string `json:"partyName"`
	PartyCode string `json:"partyCode"`
	Debit     int64  `json:"debit"`
	Credit    int64  `json:"credit"`
	Balance   int64  `json:"balance"`
}

// Receivables lists what customers still owe as of a date, from the books.
func (s *Service) Receivables(ctx context.Context, branchID int64, asOf string) ([]PartyBalance, int64, error) {
	return s.partyBalances(ctx, branchID, "1200", asOf, +1)
}

// Payables lists what the company still owes suppliers as of a date.
func (s *Service) Payables(ctx context.Context, branchID int64, asOf string) ([]PartyBalance, int64, error) {
	return s.partyBalances(ctx, branchID, "2100", asOf, -1)
}

// partyBalances foots one control account by counterparty.
//
// `sign` turns the raw footing into a positive "still owed" figure: an asset
// (piutang) is debit-heavy, a liability (hutang) credit-heavy. Fully settled
// parties are dropped — a list of zeroes is noise, and the ones that matter
// are the ones that are left.
func (s *Service) partyBalances(ctx context.Context, branchID int64, accountCode, asOf string, sign int64) ([]PartyBalance, int64, error) {
	cutoff, err := endOfDay(asOf)
	if err != nil {
		return nil, 0, err
	}
	account, err := s.repo.AccountByCode(ctx, accountCode)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	if account == nil {
		return []PartyBalance{}, 0, nil
	}
	footings, err := s.repo.PartyFootings(ctx, branchID, account.ID, cutoff)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	out := make([]PartyBalance, 0, len(footings))
	var total int64
	for _, f := range footings {
		balance := sign * (f.Debit - f.Credit)
		if balance == 0 {
			continue
		}
		row := PartyBalance{PartyID: f.PartyID, Debit: f.Debit, Credit: f.Credit, Balance: balance}
		if s.parties != nil {
			if p, err := s.parties.GetByID(ctx, f.PartyID); err == nil && p != nil {
				row.PartyName, row.PartyCode = p.Name, p.Code
			}
		}
		out = append(out, row)
		total += balance
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Balance == out[j].Balance {
			return out[i].PartyName < out[j].PartyName
		}
		return out[i].Balance > out[j].Balance
	})
	return out, total, nil
}
