package application

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/dberr"
	"mini-erp/internal/shared/doclock"
	"mini-erp/internal/shared/timeutil"
)

// Source document types the posting endpoint accepts. Stock adjustments,
// write-offs and damaged arrivals are journaled one movement at a time
// (SourceStockMovement); transfers and damaged-location moves are
// value-neutral and post nothing (ADR-0020).
const (
	SourceGoodsReceipt   = "goods_receipt"
	SourceDelivery       = "delivery"
	SourcePayment        = "payment"
	SourceSalesReturn    = "sales_return"
	SourcePurchaseReturn = "purchase_return"
	SourceExpense        = "business_expense"
	// SourceSalesReturnSettlement / SourcePurchaseReturnSettlement journal the
	// return value kept as a party balance (K-04a): source_doc_id is the
	// settlement row id (the return itself is journalled under its own type).
	// customer_credit: Dr Piutang / Cr Uang Muka Pelanggan; supplier_credit:
	// Dr Uang Muka Supplier / Cr Hutang. Cash-moving settlements (refund,
	// collect_payment) are journalled by the payment module, never here.
	SourceSalesReturnSettlement    = "sales_return_settlement"
	SourcePurchaseReturnSettlement = "purchase_return_settlement"
	// SourceStockMovement journals stock adjustments (opname differences) and
	// write-offs from the cost ledger; source_doc_id is the stock movement id
	// (these movements have no document of their own).
	SourceStockMovement = "stock_movement"
	SourceManual        = "manual"
	SourceReversal      = "reversal"
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
	// A journal is booked for something that happened: a future date would
	// show in every lifetime footing (dashboard balances) before its day.
	if timeutil.AfterTodayWIB(raw) {
		return "", apperror.Validation("", []apperror.FieldError{{Field: field, Message: "tidak boleh di masa depan"}})
	}
	return raw, nil
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
	var out *contracts.JournalEntry
	err := doclock.Lock("post:"+docType+":"+strconv.FormatInt(docID, 10), func() error {
		e, err := s.postLocked(ctx, actorID, branchID, docType, docID)
		out = e
		return err
	})
	return out, err
}

func (s *Service) postLocked(ctx context.Context, actorID, branchID int64, docType string, docID int64) (*contracts.JournalEntry, error) {
	return s.postDocument(ctx, actorID, branchID, docType, docID, false)
}

// PostCatchUp journals a document whose own month is already closed into
// today's (open) period, marked "Susulan" (A-4): the closed month is not
// touched, the books stay complete. Idempotent like Post.
func (s *Service) PostCatchUp(ctx context.Context, actorID, branchID int64, docType string, docID int64) (*contracts.JournalEntry, error) {
	var out *contracts.JournalEntry
	err := doclock.Lock("post:"+docType+":"+strconv.FormatInt(docID, 10), func() error {
		e, err := s.postDocument(ctx, actorID, branchID, docType, docID, true)
		out = e
		return err
	})
	return out, err
}

func (s *Service) postDocument(ctx context.Context, actorID, branchID int64, docType string, docID int64, catchUp bool) (*contracts.JournalEntry, error) {
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
	if catchUp {
		date = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
		memo = "Susulan: " + memo
	}
	entry, err := s.insertEntry(ctx, actorID, branchID, date, memo, docType, docID, lines, "journals.post")
	if err != nil {
		// A racing twin (another instance, or the lock bypassed) already wrote
		// the journal: post is idempotent, so hand back that one.
		if ae, ok := err.(*apperror.AppError); ok && ae.Code == apperror.CodeConflict {
			if existing, lerr := s.repo.EntryBySource(ctx, branchID, docType, docID); lerr == nil && existing != nil {
				return existing, nil
			}
		}
		return nil, err
	}
	return entry, nil
}

// needsCosting reports whether the document's lines read the cost ledger.
func needsCosting(docType string) bool {
	switch docType {
	case SourceDelivery, SourceSalesReturn, SourcePurchaseReturn, SourceStockMovement:
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
//
// The audit row (action) is written inside the journal's own transaction
// (ADR-0011): a failing trail rolls the entry back, and a replay that never
// reaches the insert writes no second row.
func (s *Service) insertEntry(ctx context.Context, actorID, branchID int64, date, memo, sourceType string, sourceID int64, lines []*contracts.JournalLine, action string) (*contracts.JournalEntry, error) {
	return s.insertEntryWith(ctx, actorID, branchID, date, memo, sourceType, sourceID, lines, action, nil, nil)
}

// insertEntryWith is insertEntry with two optional extras: extraMeta is merged
// into the audit metadata, and extra runs inside the journal's transaction
// (FN-07 stores the fiscal-correction class there, so entry and class commit
// together).
func (s *Service) insertEntryWith(ctx context.Context, actorID, branchID int64, date, memo, sourceType string, sourceID int64, lines []*contracts.JournalLine, action string, extraMeta map[string]any, extra func(ctx context.Context, tx *sql.Tx, id int64) error) (*contracts.JournalEntry, error) {
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
	// Every journal — manual or posted from a document — is dated on or
	// before today: a document dated ahead waits in the queue until its day.
	if timeutil.AfterTodayWIB(date) {
		return nil, apperror.Conflict("Tanggal jurnal " + date[:10] + " di masa depan; dokumen dijurnal saat tanggalnya tiba")
	}
	if err := s.requireOpenPeriod(ctx, branchID, date); err != nil {
		return nil, err
	}
	year := date[:4]
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if b == nil {
		return nil, apperror.NotFound("Cabang")
	}
	// The number is allocated inside the insert transaction (no gap after a
	// failed insert).
	id, err := s.repo.CreateEntry(ctx, &contracts.JournalEntry{
		BranchID: branchID, Date: date, Memo: memo,
		Numbering:  &contracts.Numbering{Prefix: "JV", BranchCode: b.Code, Year: year},
		SourceType: sourceType, SourceID: sourceID, Lines: lines,
	}, actorID, func(ctx context.Context, tx *sql.Tx, id int64, number string) error {
		meta := map[string]any{"date": date, "sourceType": sourceType, "sourceId": sourceID, "total": debit, "lines": len(lines)}
		for k, v := range extraMeta {
			meta[k] = v
		}
		if extra != nil {
			if err := extra(ctx, tx, id); err != nil {
				return err
			}
		}
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: action, Entity: "journal", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number,
			Metadata: meta,
		})
	})
	if err != nil {
		if perr := periodError(err); perr != nil {
			return nil, perr
		}
		return nil, dberr.Map(err)
	}
	return s.repo.GetEntry(ctx, id)
}

// Reverse books the mirror entry and flags the original. Reversals of
// reversals are rejected — write a manual entry instead.
func (s *Service) Reverse(ctx context.Context, actorID, branchID, entryID int64, reason string) (*contracts.JournalEntry, error) {
	return s.reverse(ctx, actorID, branchID, entryID, reason, true)
}

// ledgerBackedSources are journals booked together with a value-only
// cost-ledger row (ADR-0020/0023). No document re-posts them, so reversing the
// journal alone would move Persediaan in the books but not in the ledger: the
// correction is another revaluation/restatement, never "Balik Jurnal".
var ledgerBackedSources = map[string]bool{
	SourceInventoryRevaluation: true,
	SourceCostRestatement:      true,
	SourceOpeningSupplement:    true,
}

// refuseLedgerOnlyReversal refuses reversing a journal whose Persediaan leg has
// a cost-ledger counterpart nothing would undo: the value-only sources above,
// and a migration correction that touches Persediaan (its ledger side is the
// migrated ledger itself).
func (s *Service) refuseLedgerOnlyReversal(ctx context.Context, orig *contracts.JournalEntry) error {
	inventory := ledgerBackedSources[orig.SourceType]
	if !inventory && orig.SourceType == "migration_correction" {
		acct, err := s.repo.MappingAccount(ctx, "inventory")
		if err != nil {
			return apperror.Internal(err)
		}
		for _, l := range orig.Lines {
			if acct != nil && l.AccountID == acct.ID {
				inventory = true
				break
			}
		}
	}
	if inventory {
		return apperror.Conflict("Jurnal ini dibukukan bersama nilai ledger biaya persediaan; membaliknya hanya menggeser buku. " +
			"Koreksi lewat Revaluasi Harga Modal atau Hitung Ulang HPP")
	}
	return nil
}

// reverse books the mirror (dimensions copied, scope inherited) and flags the
// original in one transaction with a CAS. requeue says whether the source
// document returns to the posting queue (the default: it is re-posted from
// its current data) or stays out (cancelled documents).
func (s *Service) reverse(ctx context.Context, actorID, branchID, entryID int64, reason string, requeue bool) (*contracts.JournalEntry, error) {
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
	if err := s.refuseLedgerOnlyReversal(ctx, orig); err != nil {
		return nil, err
	}
	// The mirror carries the original's product/variant/party dimensions, so
	// margin and counterparty reports net to zero without status filters.
	mirror := make([]*contracts.JournalLine, 0, len(orig.Lines))
	for _, l := range orig.Lines {
		mirror = append(mirror, &contracts.JournalLine{
			AccountID: l.AccountID, Debit: l.Credit, Credit: l.Debit,
			ProductID: l.ProductID, VariantID: l.VariantID, PartyID: l.PartyID,
			Description: "Balik – " + l.Description,
		})
	}
	date := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	if err := s.requireOpenPeriod(ctx, branchID, date); err != nil {
		return nil, err
	}
	b, err := s.branches.GetByID(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if b == nil {
		return nil, apperror.NotFound("Cabang")
	}
	revID, err := s.repo.ReverseEntry(ctx, &contracts.JournalEntry{
		BranchID: branchID, Date: date,
		Numbering:  &contracts.Numbering{Prefix: "JV", BranchCode: b.Code, Year: date[:4]},
		Memo:       "Balik " + orig.Number + ": " + strings.TrimSpace(reason),
		SourceType: SourceReversal, SourceID: orig.ID, Scope: orig.Scope, Lines: mirror,
	}, orig.ID, requeue, actorID, func(ctx context.Context, tx *sql.Tx, id int64, number string) error {
		return s.audit.LogTx(ctx, tx, auditcontracts.Entry{
			Action: "journals.reverse", Entity: "journal", EntityID: id, BranchID: branchID, ActorID: actorID, Note: number,
			Metadata: map[string]any{"reversedEntryId": orig.ID, "reversedNumber": orig.Number, "reason": strings.TrimSpace(reason)},
		})
	})
	if err != nil {
		if errors.Is(err, infrastructure.ErrAlreadyReversed) {
			return nil, apperror.Conflict("Jurnal sudah dibalik")
		}
		if perr := periodError(err); perr != nil {
			return nil, perr
		}
		return nil, dberr.Map(err)
	}
	rev, err := s.repo.GetEntry(ctx, revID)
	if err != nil || rev == nil {
		return nil, apperror.Internal(errors.New("mirror vanished after insert"))
	}
	return rev, nil
}

// ManualLine is one caller-supplied leg (account by code).
type ManualLine struct {
	AccountCode string
	Debit       int64
	Credit      int64
	// PartyID is the customer/supplier of a leg on a per-party control
	// account (partyControlKeys); 0 on every other leg (see lineParty).
	PartyID int64
}

// Manual books a free-form balanced entry (adjustments, openings, corrections).
// Legs on Piutang/Hutang/Uang Muka must name their party (F5).
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
	// Persediaan is the stock ledger's control account: it moves only with
	// stock, so the book value always equals the ledger value (ADR-0020).
	inventory, err := s.repo.MappingAccount(ctx, "inventory")
	if err != nil {
		return nil, apperror.Internal(err)
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
		if inventory != nil && inventory.ID == a.ID {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "lines", Message: fmt.Sprintf(
				"baris %d: akun %s %s hanya bergerak bersama mutasi stok (penerimaan, pengiriman, retur, koreksi stok); "+
					"jurnal manual ke akun ini membuat nilai buku dan nilai stok tidak cocok", i+1, a.Code, a.Name)}})
		}
		party, err := s.lineParty(ctx, controls, partyRequired, i, a, l.PartyID)
		if err != nil {
			return nil, err
		}
		lines = append(lines, &contracts.JournalLine{AccountID: a.ID, Debit: l.Debit, Credit: l.Credit, PartyID: party})
	}
	entry, err := s.insertEntry(ctx, actorID, branchID, date, strings.TrimSpace(memo), "", 0, lines, "journals.manual")
	if err != nil {
		return nil, err
	}
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
func (s *Service) ListEntries(ctx context.Context, branchID int64, from, to string, accountID int64, page, limit int, filter ...infrastructure.EntryFilter) (*ListEntriesResult, error) {
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
	total, err := s.repo.CountEntries(ctx, branchID, from, to, accountID, filter...)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	entries, err := s.repo.ListEntries(ctx, branchID, from, to, accountID, limit, (page-1)*limit, false, filter...)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if entries == nil {
		entries = []*contracts.JournalEntry{}
	}
	return &ListEntriesResult{Entries: entries, Total: total}, nil
}

// ReverseForSource reverses the live journal of a cancelled source document
// (payments, returns). Idempotent and safe to call for documents never
// journalled. The document is NOT re-queued (requeue=0): it is cancelled.
func (s *Service) ReverseForSource(ctx context.Context, actorID, branchID int64, docType string, docID int64, reason string) (bool, error) {
	entry, err := s.repo.EntryBySource(ctx, branchID, docType, docID)
	if err != nil {
		return false, apperror.Internal(err)
	}
	if entry == nil {
		return false, nil
	}
	if _, err := s.reverse(ctx, actorID, branchID, entry.ID, reason, false); err != nil {
		if ae, ok := err.(*apperror.AppError); ok && ae.Code == apperror.CodeConflict && strings.Contains(ae.Message, "sudah dibalik") {
			return false, nil // a concurrent canceller already did it
		}
		return false, err
	}
	return true, nil
}
