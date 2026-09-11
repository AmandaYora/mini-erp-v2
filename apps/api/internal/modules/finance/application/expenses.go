package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
)

// CreateExpenseInput is the expense payload.
type CreateExpenseInput struct {
	ExpenseAccountID int64
	PayAccountID     int64
	Amount           int64
	Date             string
	Notes            string
}

// CreateExpense validates, numbers, stores, and auto-posts one expense
// (Dr beban / Cr kas-hutang). The journal carries the expense as source,
// so cancel reverses cleanly through the same path as every entry.
func (s *Service) CreateExpense(ctx context.Context, actorID, branchID int64, in CreateExpenseInput) (*infrastructure.Expense, error) {
	exp, err := s.repo.AccountByID(ctx, in.ExpenseAccountID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if exp == nil || exp.Status != "active" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "expenseAccountId", Message: "akun tidak ditemukan"}})
	}
	if exp.Type != "beban" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "expenseAccountId", Message: "harus akun beban"}})
	}
	pay, err := s.repo.AccountByID(ctx, in.PayAccountID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if pay == nil || pay.Status != "active" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "payAccountId", Message: "akun tidak ditemukan"}})
	}
	if in.ExpenseAccountID == in.PayAccountID {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "payAccountId", Message: "harus berbeda dari akun beban"}})
	}
	if in.Amount <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	date, err := normalizeEntryDate(in.Date, "date")
	if err != nil {
		return nil, err
	}
	if err := s.requireOpenPeriod(ctx, branchID, date); err != nil {
		return nil, err
	}
	year := date[:4]
	number, err := s.nextNumber(ctx, branchID, "EXP", year)
	if err != nil {
		return nil, err
	}
	id, err := s.repo.CreateExpense(ctx, &infrastructure.Expense{
		Number: number, BranchID: branchID,
		ExpenseAccount: in.ExpenseAccountID, PayAccount: in.PayAccountID,
		Amount: in.Amount, Date: date, Notes: in.Notes,
	}, actorID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	entry, err := s.insertEntry(ctx, actorID, branchID, date,
		"Biaya "+number+": "+strings.TrimSpace(in.Notes),
		SourceExpense, id, []*contracts.JournalLine{
			{AccountID: in.ExpenseAccountID, Debit: in.Amount},
			{AccountID: in.PayAccountID, Credit: in.Amount},
		})
	if err != nil {
		return nil, err
	}
	if err := s.repo.LinkExpenseJournal(ctx, id, entry.ID); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "expenses.create", Entity: "expense", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number})
	return s.repo.GetExpense(ctx, id)
}

// CancelExpense reverses the auto-posted entry and voids the expense.
// Already-reversed entries (reversed manually beforehand) just flip status.
func (s *Service) CancelExpense(ctx context.Context, actorID, branchID, id int64) (*infrastructure.Expense, error) {
	e, err := s.repo.GetExpense(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if e == nil || e.BranchID != branchID {
		return nil, apperror.NotFound("Biaya")
	}
	if e.Status != "posted" {
		return nil, apperror.Conflict("Biaya sudah dibatalkan")
	}
	if entry, err := s.repo.GetEntry(ctx, e.JournalID); err != nil {
		return nil, apperror.Internal(err)
	} else if entry == nil || entry.Status != "reversed" {
		if _, err := s.Reverse(ctx, actorID, branchID, e.JournalID, "Batal "+e.Number); err != nil {
			return nil, err
		}
	}
	if err := s.repo.SetExpenseStatus(ctx, id, "cancelled"); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "expenses.cancel", Entity: "expense", EntityID: id, BranchID: branchID, ActorID: actorID, Note: e.Number})
	return s.repo.GetExpense(ctx, id)
}

// GetExpense resolves one expense (branch-scoped).
func (s *Service) GetExpense(ctx context.Context, branchID, id int64) (*infrastructure.Expense, error) {
	e, err := s.repo.GetExpense(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if e == nil || e.BranchID != branchID {
		return nil, nil
	}
	return e, nil
}

// ListExpensesResult is a paginated expense page.
type ListExpensesResult struct {
	Expenses []*infrastructure.Expense
	Total    int64
}

// ListExpenses lists newest-first.
func (s *Service) ListExpenses(ctx context.Context, branchID int64, page, limit int) (*ListExpensesResult, error) {
	total, err := s.repo.CountExpenses(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	expenses, err := s.repo.ListExpenses(ctx, branchID, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if expenses == nil {
		expenses = []*infrastructure.Expense{}
	}
	return &ListExpensesResult{Expenses: expenses, Total: total}, nil
}

// --- tax periods ------------------------------------------------------------

// CloseTaxPeriod freezes the month's summary snapshot and locks it.
func (s *Service) CloseTaxPeriod(ctx context.Context, actorID, branchID int64, year, month int) error {
	if !validYearMonth(year, month) {
		return apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	sum, err := s.TaxSummary(ctx, branchID, year, month)
	if err != nil {
		return err
	}
	detail, err := s.TaxDetail(ctx, branchID, year, month)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(map[string]any{"summary": sum, "detail": detail})
	if err != nil {
		return apperror.Internal(err)
	}
	if err := s.repo.SetTaxPeriodStatus(ctx, branchID, year, month, "closed"); err != nil {
		return apperror.Internal(err)
	}
	p, err := s.repo.GetTaxPeriod(ctx, branchID, year, month)
	if err != nil {
		return apperror.Internal(err)
	}
	if p == nil {
		return apperror.Internal(fmt.Errorf("tax period vanished after close"))
	}
	if err := s.repo.SaveTaxSnapshot(ctx, p.ID, string(raw), actorID); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "fiscal_periods.close", Entity: "fiscal_period", BranchID: branchID, ActorID: actorID, Note: "pajak " + fmt.Sprintf("%04d-%02d", year, month)})
	return nil
}

// ReopenTaxPeriod unlocks a month (snapshots stay as history).
func (s *Service) ReopenTaxPeriod(ctx context.Context, actorID, branchID int64, year, month int) error {
	if !validYearMonth(year, month) {
		return apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	if err := s.repo.SetTaxPeriodStatus(ctx, branchID, year, month, "open"); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "fiscal_periods.reopen", Entity: "fiscal_period", BranchID: branchID, ActorID: actorID, Note: "pajak " + fmt.Sprintf("%04d-%02d", year, month)})
	return nil
}

// ListTaxPeriods returns touched tax months, newest first.
func (s *Service) ListTaxPeriods(ctx context.Context, branchID int64) ([]*infrastructure.TaxPeriod, error) {
	return s.repo.ListTaxPeriods(ctx, branchID)
}
