package application

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"mini-erp/internal/shared/dberr"
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
	// PartyID is the optional counterparty of the expense (FN-10): a supplier
	// of electricity, a landlord, a courier. Zero = none.
	PartyID int64
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
	// HPP is what the stock ledger relieved; an operating expense booked there
	// would make the P&L cost of goods disagree with the margin reports.
	if cogs, err := s.repo.MappingAccount(ctx, "cogs"); err != nil {
		return nil, apperror.Internal(err)
	} else if cogs != nil && cogs.ID == exp.ID {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "expenseAccountId",
			Message: "akun HPP diisi otomatis dari mutasi stok; catat biaya ini di akun beban operasional"}})
	}
	pay, err := s.repo.AccountByID(ctx, in.PayAccountID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if pay == nil || pay.Status != "active" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "payAccountId", Message: "akun tidak ditemukan"}})
	}
	if pay.Type != contracts.AccountAsset || !pay.IsCash {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "payAccountId", Message: "pembayar harus akun kas/bank (aset berkas kas)"}})
	}
	if in.ExpenseAccountID == in.PayAccountID {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "payAccountId", Message: "harus berbeda dari akun beban"}})
	}
	if in.Amount <= 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "amount", Message: "harus lebih dari 0"}})
	}
	if in.PartyID < 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "pihak tidak valid"}})
	}
	if in.PartyID > 0 {
		p, perr := s.parties.GetByID(ctx, in.PartyID)
		if perr != nil {
			return nil, apperror.Internal(perr)
		}
		if p == nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "partyId", Message: "pihak tidak ditemukan"}})
		}
	}
	date, err := normalizeEntryDate(in.Date, "date")
	if err != nil {
		return nil, err
	}
	if err := s.requireOpenPeriod(ctx, branchID, date); err != nil {
		return nil, err
	}
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil || b == nil {
		return nil, apperror.Internal(fmt.Errorf("branch %d: %v", branchID, err))
	}
	year := date[:4]
	// One transaction: expense row, its EXP number, the journal and the link.
	id, err := s.repo.CreateExpenseWithEntry(ctx, &infrastructure.Expense{
		BranchID: branchID, ExpenseAccount: in.ExpenseAccountID, PayAccount: in.PayAccountID,
		Amount: in.Amount, Date: date, Notes: in.Notes, PartyID: in.PartyID,
	}, &contracts.Numbering{Prefix: "EXP", BranchCode: b.Code, Year: year},
		&contracts.JournalEntry{
			Number: "", BranchID: branchID, Date: date, SourceType: SourceExpense,
			Numbering: &contracts.Numbering{Prefix: "JV", BranchCode: b.Code, Year: year},
			Lines: []*contracts.JournalLine{
				{AccountID: in.ExpenseAccountID, Debit: in.Amount, PartyID: in.PartyID},
				{AccountID: in.PayAccountID, Credit: in.Amount},
			},
		}, actorID, func(ctx context.Context, tx *sql.Tx, id int64, number string) error {
			return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
				Action: "expenses.create", Entity: "expense", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number,
				After: map[string]any{"amount": in.Amount, "expenseAccountId": in.ExpenseAccountID, "payAccountId": in.PayAccountID, "date": date, "partyId": in.PartyID},
			})
		})
	if err != nil {
		if perr := periodError(err); perr != nil {
			return nil, perr
		}
		return nil, dberr.Map(err)
	}
	e, err := s.repo.GetExpense(ctx, id)
	if err != nil || e == nil {
		return nil, apperror.Internal(fmt.Errorf("expense vanished after insert: %v", err))
	}
	s.nameExpenseParties(ctx, []*infrastructure.Expense{e})
	return e, nil
}

// CancelExpense reverses the auto-posted entry and voids the expense. The
// reason is mandatory and kept with who/when. Already-reversed entries
// (reversed manually beforehand) just flip status.
func (s *Service) CancelExpense(ctx context.Context, actorID, branchID, id int64, reason string) (*infrastructure.Expense, error) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "alasan pembatalan wajib diisi"}})
	}
	if len(reason) > 255 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "maksimal 255 karakter"}})
	}
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
		if _, err := s.Reverse(ctx, actorID, branchID, e.JournalID, "Batal "+e.Number+": "+reason); err != nil {
			return nil, err
		}
	}
	if won, err := s.repo.CancelExpense(ctx, id, reason, actorID, func(ctx context.Context, tx *sql.Tx) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "expenses.cancel", Entity: "expense", EntityID: id, BranchID: branchID, ActorID: actorID, Note: e.Number + ": " + reason,
			Before: map[string]any{"status": "posted"}, After: map[string]any{"status": "cancelled", "reason": reason},
		})
	}); err != nil {
		return nil, apperror.Internal(err)
	} else if !won {
		return nil, apperror.Conflict("Biaya sudah dibatalkan")
	}
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
	s.nameExpenseParties(ctx, []*infrastructure.Expense{e})
	return e, nil
}

// nameExpenseParties resolves counterparty names for a page of expenses in one
// grouped read (best effort: a missing name renders blank, never an error).
func (s *Service) nameExpenseParties(ctx context.Context, list []*infrastructure.Expense) {
	var ids []int64
	for _, e := range list {
		if e.PartyID != 0 {
			ids = append(ids, e.PartyID)
		}
	}
	if len(ids) == 0 {
		return
	}
	labels := s.partyLabels(ctx, ids)
	for _, e := range list {
		if l, ok := labels[e.PartyID]; ok {
			e.PartyName = l.Name
		}
	}
}

// ListExpensesResult is a paginated expense page.
type ListExpensesResult struct {
	Expenses []*infrastructure.Expense
	Total    int64
}

// ListExpenses lists newest-first.
func (s *Service) ListExpenses(ctx context.Context, branchID int64, f infrastructure.ExpenseFilter, page, limit int) (*ListExpensesResult, error) {
	total, err := s.repo.CountExpenses(ctx, branchID, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	expenses, err := s.repo.ListExpenses(ctx, branchID, f, limit, (page-1)*limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if expenses == nil {
		expenses = []*infrastructure.Expense{}
	}
	s.nameExpenseParties(ctx, expenses)
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
	// Status flip, snapshot and trail commit together (ADR-0011).
	if err := s.repo.SetTaxPeriodStatus(ctx, branchID, year, month, "closed", string(raw), actorID,
		s.periodAudit("fiscal_periods.close", "pajak ", "closed", branchID, actorID, year, month, "")); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// ReopenTaxPeriod unlocks a month (snapshots stay as history). A reason is
// mandatory and lands in the audit diff (FN-05).
func (s *Service) ReopenTaxPeriod(ctx context.Context, actorID, branchID int64, year, month int, reason string) error {
	if !validYearMonth(year, month) {
		return apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	reason, rerr := reopenReason(reason)
	if rerr != nil {
		return rerr
	}
	if err := s.repo.SetTaxPeriodStatus(ctx, branchID, year, month, "open", "", actorID,
		s.periodAudit("fiscal_periods.reopen", "pajak ", "open", branchID, actorID, year, month, reason)); err != nil {
		return apperror.Internal(err)
	}
	return nil
}

// ListTaxPeriods returns touched tax months, newest first.
func (s *Service) ListTaxPeriods(ctx context.Context, branchID int64) ([]*infrastructure.TaxPeriod, error) {
	return s.repo.ListTaxPeriods(ctx, branchID)
}
