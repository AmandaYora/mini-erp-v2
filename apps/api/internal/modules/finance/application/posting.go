package application

import (
	"context"
	"fmt"
	"math"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/timeutil"
)

// Source document types the posting endpoint accepts. Adjustments, damaged
// moves, write-offs, transfers, and openings post no automatic journal in
// L7 scope — their economics live in the cost ledger, and operators book
// them via manual journals (documented gap, not an oversight).
const (
	SourceGoodsReceipt   = "goods_receipt"
	SourceDelivery       = "delivery"
	SourcePayment        = "payment"
	SourceSalesReturn    = "sales_return"
	SourcePurchaseReturn = "purchase_return"
	SourceExpense        = "business_expense"
	SourceManual         = "manual"
	SourceReversal       = "reversal"
	// SourceOpening marks the one-time cutover balance journal per branch
	// (source_doc_id = branch_id). No opening table exists by design: the
	// opening IS a journal, so every report reads it with zero UNIONs.
	SourceOpening = "opening"
	// SourceTaxAdjustment marks fiscal-correction journals. They live in the
	// same ledger for audit, but every commercial footing/ledger excludes
	// them by construction (see commercialOnly) — commercial profit must
	// never see fiscal corrections.
	SourceTaxAdjustment = "tax_adjustment"
)

// line is one balanced leg under construction (account resolved already).
type line struct {
	accountID int64
	debit     int64
	credit    int64
}

// normalizeEntryDate validates YYYY-MM-DD and returns it for storage
// (MySQL coerces date-only into midnight DATETIME).
func normalizeEntryDate(raw, field string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02"), nil
	}
	if _, err := timeutil.ParseDateInput(raw); err != nil {
		return "", apperror.Validation("", []apperror.FieldError{{Field: field, Message: "format tanggal harus YYYY-MM-DD"}})
	}
	return raw, nil
}

func round(f float64) int64 {
	return int64(math.Round(f))
}

// Preview builds the compound entry for a source document without writing.
// Same builders as Post — preview never drifts from reality.
func (s *Service) Preview(ctx context.Context, branchID int64, docType string, docID int64) (*contracts.JournalEntry, error) {
	lines, memo, date, err := s.buildLines(ctx, branchID, docType, docID)
	if err != nil {
		return nil, err
	}
	return &contracts.JournalEntry{
		BranchID: branchID, Date: date, Memo: memo,
		SourceType: docType, SourceID: docID, Status: "posted", Lines: lines,
	}, nil
}

// Post writes the compound entry idempotently: re-posting the same document
// returns the existing entry instead of double-booking. Costing syncs first
// (drained to completion) so COGS-bearing builders never silently book zero
// for want of a sync the operator forgot.
func (s *Service) Post(ctx context.Context, actorID, branchID int64, docType string, docID int64) (*contracts.JournalEntry, error) {
	if existing, err := s.repo.EntryBySource(ctx, branchID, docType, docID); err != nil {
		return nil, apperror.Internal(err)
	} else if existing != nil {
		full, err := s.repo.GetEntry(ctx, existing.ID)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		return full, nil
	}
	if needsCosting(docType) {
		if err := s.drainCosting(ctx, branchID); err != nil {
			return nil, err
		}
	}
	lines, memo, date, err := s.buildLines(ctx, branchID, docType, docID)
	if err != nil {
		return nil, err
	}
	entry, err := s.insertEntry(ctx, actorID, branchID, date, memo, docType, docID, lines)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "journals.post", Entity: "journal", EntityID: entry.ID, BranchID: branchID, ActorID: actorID, Note: entry.Number})
	return entry, nil
}

// needsCosting reports whether the document's lines read the cost ledger.
func needsCosting(docType string) bool {
	switch docType {
	case SourceDelivery, SourceSalesReturn, SourcePurchaseReturn:
		return true
	default:
		return false
	}
}

// drainCosting runs sync batches until the cursor stops (cap 50 batches ≈
// 25k movements — past that, something else is wrong and the operator
// should sync explicitly and investigate).
func (s *Service) drainCosting(ctx context.Context, branchID int64) error {
	for i := 0; i < 50; i++ {
		res, err := s.Sync(ctx, branchID)
		if err != nil {
			return err
		}
		if res.Ingested == 0 {
			return nil
		}
	}
	return apperror.Conflict("Antrian costing terlalu besar, sinkronisasi manual dulu")
}

// insertEntry numbers, period-checks, and stores one balanced entry.
func (s *Service) insertEntry(ctx context.Context, actorID, branchID int64, date, memo, sourceType string, sourceID int64, lines []*contracts.JournalLine) (*contracts.JournalEntry, error) {
	if len(date) < 7 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	var debit, credit int64
	for _, l := range lines {
		if l.Debit < 0 || l.Credit < 0 || (l.Debit == 0 && l.Credit == 0) || (l.Debit > 0 && l.Credit > 0) {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "setiap baris tepat satu sisi"}})
		}
		debit += l.Debit
		credit += l.Credit
	}
	if debit <= 0 || debit != credit {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: "total debit harus sama dengan kredit"}})
	}
	if err := s.requireOpenPeriod(ctx, branchID, date); err != nil {
		return nil, err
	}
	year := date[:4]
	number, err := s.nextNumber(ctx, branchID, "JV", year)
	if err != nil {
		return nil, err
	}
	id, err := s.repo.CreateEntry(ctx, &contracts.JournalEntry{
		Number: number, BranchID: branchID, Date: date, Memo: memo,
		SourceType: sourceType, SourceID: sourceID, Lines: lines,
	}, actorID)
	if err != nil {
		return nil, dberr.Map(err)
	}
	return s.repo.GetEntry(ctx, id)
}

// Reverse books the mirror entry and flags the original. Reversals of
// reversals are rejected — write a manual entry instead.
func (s *Service) Reverse(ctx context.Context, actorID, branchID, entryID int64, reason string) (*contracts.JournalEntry, error) {
	orig, err := s.repo.GetEntry(ctx, entryID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if orig == nil || orig.BranchID != branchID {
		return nil, apperror.NotFound("Jurnal")
	}
	if orig.Status == "reversed" {
		return nil, apperror.Conflict("Jurnal sudah dibalik")
	}
	if orig.SourceType == SourceReversal {
		return nil, apperror.Conflict("Jurnal pembalik tidak dapat dibalik lagi")
	}
	if strings.TrimSpace(reason) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "reason", Message: "wajib diisi"}})
	}
	mirror := make([]*contracts.JournalLine, 0, len(orig.Lines))
	for _, l := range orig.Lines {
		mirror = append(mirror, &contracts.JournalLine{
			AccountID: l.AccountID, Debit: l.Credit, Credit: l.Debit,
		})
	}
	date := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	rev, err := s.insertEntry(ctx, actorID, branchID, date,
		"Balik "+orig.Number+": "+strings.TrimSpace(reason),
		SourceReversal, orig.ID, mirror)
	if err != nil {
		return nil, err
	}
	if err := s.repo.MarkReversed(ctx, orig.ID, rev.ID); err != nil {
		return nil, apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "journals.reverse", Entity: "journal", EntityID: rev.ID, BranchID: branchID, ActorID: actorID, Note: rev.Number})
	return rev, nil
}

// ManualLine is one caller-supplied leg (account by code).
type ManualLine struct {
	AccountCode string
	Debit       int64
	Credit      int64
}

// Manual books a free-form balanced entry (adjustments, openings, corrections).
func (s *Service) Manual(ctx context.Context, actorID, branchID int64, date, memo string, in []ManualLine) (*contracts.JournalEntry, error) {
	if strings.TrimSpace(memo) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "memo", Message: "wajib diisi"}})
	}
	date, err := normalizeEntryDate(date, "date")
	if err != nil {
		return nil, err
	}
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
	entry, err := s.insertEntry(ctx, actorID, branchID, date, strings.TrimSpace(memo), "", 0, lines)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "journals.manual", Entity: "journal", EntityID: entry.ID, BranchID: branchID, ActorID: actorID, Note: entry.Number})
	return entry, nil
}

// GetEntry resolves one entry with lines (branch-scoped).
func (s *Service) GetEntry(ctx context.Context, branchID, id int64) (*contracts.JournalEntry, error) {
	e, err := s.repo.GetEntry(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if e == nil || e.BranchID != branchID {
		return nil, nil
	}
	return e, nil
}

// ListEntriesResult is a paginated journal page.
type ListEntriesResult struct {
	Entries []*contracts.JournalEntry
	Total   int64
}

// ListEntries searches entries newest-first (lines loaded on detail only).
// Raw YYYY-MM-DD bounds normalize to full-day datetimes here — passing them
// raw would silently drop the `to` day (OQ-A32 class).
func (s *Service) ListEntries(ctx context.Context, branchID int64, from, to string, accountID int64, page, limit int) (*ListEntriesResult, error) {
	if from != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(from)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		from = strings.TrimSpace(from) + " 00:00:00"
	}
	if to != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(to)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		to = strings.TrimSpace(to) + " 23:59:59"
	}
	total, err := s.repo.CountEntries(ctx, branchID, from, to, accountID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	entries, err := s.repo.ListEntries(ctx, branchID, from, to, accountID, limit, (page-1)*limit, false)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if entries == nil {
		entries = []*contracts.JournalEntry{}
	}
	return &ListEntriesResult{Entries: entries, Total: total}, nil
}
