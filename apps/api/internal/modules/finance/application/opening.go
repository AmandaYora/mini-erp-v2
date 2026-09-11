package application

import (
	"context"
	"fmt"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/apperror"
)

// Opening balance without an opening table (G-revisi): the cutover balance
// IS one journal per branch (source 'opening', source_doc_id = branch_id),
// so every report reads it with zero UNIONs. The wizard is stateless —
// preview validates, post writes — the same preview/commit shape as
// stock/opening (no staging table there either).

// OpeningStatus reports whether the branch already posted its cutover
// balance. Reviewers use it as the "sudah cutover" marker — no flag table.
func (s *Service) OpeningStatus(ctx context.Context, branchID int64) (*contracts.JournalEntry, error) {
	e, err := s.repo.EntryBySource(ctx, branchID, SourceOpening, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if e == nil {
		return nil, nil
	}
	return s.repo.GetEntry(ctx, e.ID)
}

// resolveOpeningLines maps caller legs (account by code) to balanced lines.
// Shared by preview and post so the two can never disagree.
func (s *Service) resolveOpeningLines(ctx context.Context, in []ManualLine) ([]*contracts.JournalLine, error) {
	if len(in) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "wajib diisi"}})
	}
	lines := make([]*contracts.JournalLine, 0, len(in))
	for i, l := range in {
		a, err := s.repo.AccountByCode(ctx, strings.ToUpper(strings.TrimSpace(l.AccountCode)))
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if a == nil || a.Status != "active" {
			return nil, apperror.Validation("", []apperror.FieldError{
				{Field: "lines", Message: fmt.Sprintf("baris %d: akun tidak ditemukan", i+1)}})
		}
		lines = append(lines, &contracts.JournalLine{AccountID: a.ID, Debit: l.Debit, Credit: l.Credit})
	}
	var debit, credit int64
	for _, l := range lines {
		if l.Debit < 0 || l.Credit < 0 || (l.Debit == 0 && l.Credit == 0) || (l.Debit > 0 && l.Credit > 0) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "setiap baris tepat satu sisi"}})
		}
		debit += l.Debit
		credit += l.Credit
	}
	// P4 at the boundary: an unbalanced opening is rejected here AND in
	// insertEntry/CreateEntry — the book can never hold one.
	if debit <= 0 || debit != credit {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "total debit harus sama dengan kredit"}})
	}
	return lines, nil
}

// PreviewOpening validates the cutover legs without writing.
func (s *Service) PreviewOpening(ctx context.Context, branchID int64, date, memo string, in []ManualLine) (*contracts.JournalEntry, error) {
	date, err := normalizeEntryDate(date, "date")
	if err != nil {
		return nil, err
	}
	lines, err := s.resolveOpeningLines(ctx, in)
	if err != nil {
		return nil, err
	}
	outs := make([]*contracts.JournalLine, 0, len(lines))
	for _, l := range lines {
		a, err := s.repo.AccountByID(ctx, l.AccountID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		outs = append(outs, &contracts.JournalLine{
			AccountID: l.AccountID, Debit: l.Debit, Credit: l.Credit,
			AccountCode: a.Code, AccountName: a.Name,
		})
	}
	m := strings.TrimSpace(memo)
	if m == "" {
		m = "Saldo awal"
	}
	return &contracts.JournalEntry{
		BranchID: branchID, Date: date, Memo: m,
		SourceType: SourceOpening, SourceID: branchID, Status: "posted", Lines: outs,
	}, nil
}

// PostOpening writes the branch's cutover journal idempotently: a second
// call returns the existing entry instead of double-booking the opening.
func (s *Service) PostOpening(ctx context.Context, actorID, branchID int64, date, memo string, in []ManualLine) (*contracts.JournalEntry, error) {
	if existing, err := s.OpeningStatus(ctx, branchID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	date, err := normalizeEntryDate(date, "date")
	if err != nil {
		return nil, err
	}
	lines, err := s.resolveOpeningLines(ctx, in)
	if err != nil {
		return nil, err
	}
	m := strings.TrimSpace(memo)
	if m == "" {
		m = "Saldo awal"
	}
	if err := s.requireOpenPeriod(ctx, branchID, date); err != nil {
		return nil, err
	}
	entry, err := s.insertEntry(ctx, actorID, branchID, date, m, SourceOpening, branchID, stripPreview(lines))
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "journals.opening", Entity: "journal", EntityID: entry.ID, BranchID: branchID, ActorID: actorID, Note: entry.Number})
	return entry, nil
}

// stripPreview drops display-only account labels before persistence.
func stripPreview(lines []*contracts.JournalLine) []*contracts.JournalLine {
	out := make([]*contracts.JournalLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, &contracts.JournalLine{AccountID: l.AccountID, Debit: l.Debit, Credit: l.Credit})
	}
	return out
}
