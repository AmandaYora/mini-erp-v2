package application

import (
	"context"
	"fmt"
	"strings"

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
// rule: the opening demands a party on Piutang/Hutang/Uang Muka legs (they
// open per-party balances); fiscal adjustments only check one when given.
// Shared by preview and post (and fiscal adjustments) so they can never disagree.
func (s *Service) resolveOpeningLines(ctx context.Context, in []ManualLine, rule partyRule) ([]*contracts.JournalLine, error) {
	if len(in) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "wajib diisi"}})
	}
	controls, err := s.partyControls(ctx)
	if err != nil {
		return nil, err
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
		party, err := s.lineParty(ctx, controls, rule, i, a, l.PartyID)
		if err != nil {
			return nil, err
		}
		lines = append(lines, &contracts.JournalLine{AccountID: a.ID, Debit: l.Debit, Credit: l.Credit, PartyID: party})
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

// buildOpeningLines is the K-05 cutover builder: the caller's own legs (cash,
// bank, receivables, payables, equity ...) must balance by themselves — the
// server never silently plugs a typo — and the Persediaan account cannot be
// typed at all: its value comes from the cost ledger, with the matching
// Cr Modal Awal appended, so a cutover with stock stays balanced without the
// operator doing arithmetic. Shared by preview and post.
func (s *Service) buildOpeningLines(ctx context.Context, branchID int64, in []ManualLine) ([]*contracts.JournalLine, []openingPosition, error) {
	valued, _, err := s.openingPositions(ctx, branchID)
	if err != nil {
		return nil, nil, err
	}
	var lines []*contracts.JournalLine
	if len(in) > 0 || len(valued) == 0 {
		lines, err = s.resolveOpeningLines(ctx, in, partyRequired)
		if err != nil {
			return nil, nil, err
		}
	}
	maps, err := s.repo.ListMappings(ctx)
	if err != nil {
		return nil, nil, apperror.Internal(err)
	}
	if inv := maps["inventory"]; inv != nil {
		for i, l := range lines {
			if l.AccountID == inv.ID {
				return nil, nil, apperror.Validation("", []apperror.FieldError{{Field: "lines",
					Message: fmt.Sprintf("baris %d: akun Persediaan dihitung otomatis dari stok awal, tidak boleh diisi manual", i+1)}})
			}
		}
	}
	if len(valued) > 0 {
		inv, eq, err := s.openingAccounts(ctx)
		if err != nil {
			return nil, nil, err
		}
		legs, _ := serverOpeningLegs(inv, eq, valued)
		lines = append(lines, legs...)
	}
	return lines, valued, nil
}

// PreviewOpening validates the cutover legs without writing. The per-position
// inventory legs are shown as ONE line (the posted journal keeps one per
// product for per-product reports; the totals are identical).
func (s *Service) PreviewOpening(ctx context.Context, branchID int64, date, memo string, in []ManualLine) (*contracts.JournalEntry, error) {
	date, err := normalizeEntryDate(date, "date")
	if err != nil {
		return nil, err
	}
	lines, valued, err := s.buildOpeningLines(ctx, branchID, in)
	if err != nil {
		return nil, err
	}
	outs := make([]*contracts.JournalLine, 0, len(lines))
	var invID, invTotal int64
	if len(valued) > 0 {
		if maps, err := s.repo.ListMappings(ctx); err == nil && maps["inventory"] != nil {
			invID = maps["inventory"].ID
		}
	}
	var invAt = -1
	for _, l := range lines {
		a, err := s.repo.AccountByID(ctx, l.AccountID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		if invID != 0 && l.AccountID == invID {
			invTotal += l.Debit
			if invAt < 0 {
				invAt = len(outs)
				outs = append(outs, &contracts.JournalLine{AccountID: l.AccountID, AccountCode: a.Code, AccountName: a.Name})
			}
			continue
		}
		outs = append(outs, &contracts.JournalLine{
			AccountID: l.AccountID, Debit: l.Debit, Credit: l.Credit, PartyID: l.PartyID,
			AccountCode: a.Code, AccountName: a.Name, Description: l.Description,
		})
	}
	if invAt >= 0 {
		outs[invAt].Debit = invTotal
		outs[invAt].Description = fmt.Sprintf("Persediaan %d produk (dihitung server dari stok awal)", len(valued))
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
// The same holds for a branch whose opening journal came from the legacy ETL.
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
	lines, _, err := s.buildOpeningLines(ctx, branchID, in)
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
	entry, err := s.insertEntry(ctx, actorID, branchID, date, m, SourceOpening, branchID, stripPreviewKeepDims(lines), "journals.opening")
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// stripPreviewKeepDims drops display-only account labels before persistence;
// product/variant/party dimensions and per-line memos stay.
func stripPreviewKeepDims(lines []*contracts.JournalLine) []*contracts.JournalLine {
	out := make([]*contracts.JournalLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, &contracts.JournalLine{AccountID: l.AccountID, Debit: l.Debit, Credit: l.Credit,
			ProductID: l.ProductID, VariantID: l.VariantID, PartyID: l.PartyID, Description: l.Description})
	}
	return out
}

// stripPreview drops display-only account labels before persistence (the
// party dimension stays).
func stripPreview(lines []*contracts.JournalLine) []*contracts.JournalLine {
	out := make([]*contracts.JournalLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, &contracts.JournalLine{AccountID: l.AccountID, Debit: l.Debit, Credit: l.Credit, PartyID: l.PartyID})
	}
	return out
}
