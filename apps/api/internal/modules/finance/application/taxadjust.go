package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/apperror"
)

// Fiscal worksheet without a worksheet table (H-revisi): corrections live
// as typed journals (source 'tax_adjustment') in the same ledger for audit,
// while every commercial footing/ledger excludes them by construction (see
// commercialOnly). Books stay append-only: corrections are created and
// reversed, never edited — exactly like every other journal.

// CreateTaxAdjustment books one fiscal-correction journal. It never touches
// commercial profit: readers of the commercial books exclude this source
// type, and the fiscal worksheet adds it back explicitly.
func (s *Service) CreateTaxAdjustment(ctx context.Context, actorID, branchID int64, date, memo string, in []ManualLine) (*contracts.JournalEntry, error) {
	if strings.TrimSpace(memo) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "memo", Message: "wajib diisi"}})
	}
	date, err := normalizeEntryDate(date, "date")
	if err != nil {
		return nil, err
	}
	lines, err := s.resolveOpeningLines(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := s.requireOpenPeriod(ctx, branchID, date); err != nil {
		return nil, err
	}
	seq, err := s.repo.NextSequence(ctx, branchID, "taxadj:"+date[:4])
	if err != nil {
		return nil, apperror.Internal(err)
	}
	entry, err := s.insertEntry(ctx, actorID, branchID, date,
		"Koreksi fiskal: "+strings.TrimSpace(memo),
		SourceTaxAdjustment, seq, stripPreview(lines))
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "journals.tax_adjust", Entity: "journal", EntityID: entry.ID, BranchID: branchID, ActorID: actorID, Note: entry.Number})
	return entry, nil
}

// TaxAdjustmentRow is one correction journal with its legs.
type TaxAdjustmentRow struct {
	Entry *contracts.JournalEntry `json:"entry"`
}

// ListTaxAdjustments lists correction journals newest-first (cap 100 —
// worksheet history, not a paginated browser; the journal browser covers
// deep history).
func (s *Service) ListTaxAdjustments(ctx context.Context, branchID int64) ([]*contracts.JournalEntry, error) {
	entries, err := s.repo.EntriesBySourceType(ctx, branchID, SourceTaxAdjustment, 100, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if entries == nil {
		entries = []*contracts.JournalEntry{}
	}
	full := make([]*contracts.JournalEntry, 0, len(entries))
	for _, e := range entries {
		f, err := s.repo.GetEntry(ctx, e.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		full = append(full, f)
	}
	return full, nil
}

// FiscalLine is one account's fiscal correction (commercial → fiscal delta).
type FiscalLine struct {
	Code   string `json:"code"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Debit  int64  `json:"debit"`
	Credit int64  `json:"credit"`
	// Effect is the signed contribution to fiscal profit (+ menaikkan laba).
	Effect int64 `json:"effect"`
}

// FiscalSummary reconciles commercial profit to fiscal profit for a range:
// fiscal = commercial + Σ corrections. Balance-sheet corrections list but
// stay profit-neutral (reklasifikasi tidak mengubah laba).
func (s *Service) FiscalSummary(ctx context.Context, branchID int64, from, to string) (*FiscalSummaryResult, error) {
	return s.fiscalSummaryScoped(ctx, branchID, from, to, fullBooks)
}

// fiscalSummaryScoped reconciles within the given book scope. Only the
// COMMERCIAL leg narrows: fiscal corrections are typed journals with no
// sales order behind them, so the turnover ceiling never touches them.
func (s *Service) fiscalSummaryScoped(ctx context.Context, branchID int64, from, to string, sc bookScope) (*FiscalSummaryResult, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	commercial, err := s.profitLoss(ctx, branchID, start, end, sc)
	if err != nil {
		return nil, err
	}
	footings, err := s.repo.TaxAdjustmentFootings(ctx, branchID, start, end)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	byID := map[int64]*contracts.Account{}
	for _, a := range accounts {
		byID[a.ID] = a
	}
	res := &FiscalSummaryResult{Commercial: commercial, FiscalProfit: commercial.Profit}
	for id, f := range footings {
		a, ok := byID[id]
		if !ok {
			return nil, apperror.Internal(fmt.Errorf("akun koreksi fiskal %d tidak dikenal", id))
		}
		line := FiscalLine{Code: a.Code, Name: a.Name, Type: a.Type, Debit: f[0], Credit: f[1]}
		switch a.Type {
		case contracts.AccountIncome:
			line.Effect = f[1] - f[0]
			res.FiscalProfit += line.Effect
		case contracts.AccountExpense:
			line.Effect = -(f[0] - f[1])
			res.FiscalProfit += line.Effect
		default:
			line.Effect = 0
		}
		res.Lines = append(res.Lines, line)
	}
	sort.Slice(res.Lines, func(i, j int) bool { return res.Lines[i].Code < res.Lines[j].Code })
	res.From, res.To = from, to
	return res, nil
}

// FiscalSummaryResult is the commercial → fiscal reconciliation.
type FiscalSummaryResult struct {
	Commercial   *ProfitLoss  `json:"commercial"`
	Lines        []FiscalLine `json:"lines"`
	FiscalProfit int64        `json:"fiscalProfit"`
	From         string       `json:"from"`
	To           string       `json:"to"`
}
