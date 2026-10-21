package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	"mini-erp/internal/shared/apperror"
)

// Fiscal worksheet without a worksheet table (H-revisi): corrections live
// as typed journals (source 'tax_adjustment') in the same ledger for audit,
// while every commercial footing/ledger excludes them by construction (see
// commercialOnly). Books stay append-only: corrections are created and
// reversed, never edited — exactly like every other journal.
//
// FN-07 / ADR-0018 D6: the journal model is kept. A correction additionally
// carries a class (positive / negative / final income / non-object), an
// optional note, and supporting scans — all beside the journal, never in it.

// Fiscal correction classes (FN-07).
const (
	TaxClassPositive  = "positive"     // koreksi fiskal positif (menambah laba kena pajak)
	TaxClassNegative  = "negative"     // koreksi fiskal negatif (mengurangi)
	TaxClassFinal     = "final_income" // penghasilan yang dikenai PPh final
	TaxClassNonObject = "non_object"   // penghasilan bukan objek pajak
)

var taxClassLabels = map[string]string{
	TaxClassPositive:  "Koreksi positif",
	TaxClassNegative:  "Koreksi negatif",
	TaxClassFinal:     "Penghasilan final",
	TaxClassNonObject: "Bukan objek pajak",
	"":                "Belum diklasifikasi",
}

// TaxClassLabel names a class for display ("" = booked before classes existed).
func TaxClassLabel(c string) string {
	if l, ok := taxClassLabels[c]; ok {
		return l
	}
	return c
}

// ValidTaxClass reports whether c is a known class.
func ValidTaxClass(c string) bool {
	switch c {
	case TaxClassPositive, TaxClassNegative, TaxClassFinal, TaxClassNonObject:
		return true
	}
	return false
}

// TaxAdjustInput is the class and note of a new correction.
type TaxAdjustInput struct {
	Classification string
	Note           string
}

// CreateTaxAdjustment books one fiscal-correction journal without a class (the
// pre-FN-07 shape; the correction reads "Belum diklasifikasi"). Prefer
// CreateTaxAdjustmentWith.
func (s *Service) CreateTaxAdjustment(ctx context.Context, actorID, branchID int64, date, memo string, in []ManualLine) (*contracts.JournalEntry, error) {
	return s.createTaxAdjustment(ctx, actorID, branchID, date, memo, in, TaxAdjustInput{})
}

// CreateTaxAdjustmentWith books one fiscal-correction journal with its class
// and note. It never touches commercial profit: readers of the commercial books
// exclude this source type, and the fiscal worksheet adds it back explicitly.
func (s *Service) CreateTaxAdjustmentWith(ctx context.Context, actorID, branchID int64, date, memo string, in []ManualLine, meta TaxAdjustInput) (*contracts.JournalEntry, error) {
	if !ValidTaxClass(meta.Classification) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "classification",
			Message: "pilih jenis koreksi: positif, negatif, penghasilan final, atau bukan objek pajak"}})
	}
	return s.createTaxAdjustment(ctx, actorID, branchID, date, memo, in, meta)
}

func (s *Service) createTaxAdjustment(ctx context.Context, actorID, branchID int64, date, memo string, in []ManualLine, meta TaxAdjustInput) (*contracts.JournalEntry, error) {
	if strings.TrimSpace(memo) == "" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "memo", Message: "wajib diisi"}})
	}
	meta.Note = strings.TrimSpace(meta.Note)
	if len([]rune(meta.Note)) > 1000 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "note", Message: "catatan maksimal 1000 karakter"}})
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
	var extra func(ctx context.Context, tx *infrastructure.Tx, id int64) error
	extraMeta := map[string]any{}
	if meta.Classification != "" {
		extraMeta["classification"] = meta.Classification
		if meta.Note != "" {
			extraMeta["note"] = meta.Note
		}
		extra = func(ctx context.Context, tx *infrastructure.Tx, id int64) error {
			return infrastructure.InsertTaxAdjustMetaTx(ctx, tx, infrastructure.TaxAdjustMeta{
				EntryID: id, Classification: meta.Classification, Note: meta.Note})
		}
	}
	entry, err := s.insertEntryWith(ctx, actorID, branchID, date,
		"Koreksi fiskal: "+strings.TrimSpace(memo),
		SourceTaxAdjustment, seq, stripPreview(lines), "journals.tax_adjust", extraMeta, extra)
	if err != nil {
		return nil, err
	}
	return entry, nil
}

// TaxAdjustmentRow is one correction journal with its legs and FN-07 detail.
type TaxAdjustmentRow struct {
	Entry          *contracts.JournalEntry `json:"entry"`
	Classification string                  `json:"classification"`
	ClassLabel     string                  `json:"classLabel"`
	Note           string                  `json:"note"`
}

// ListTaxAdjustments lists correction journals newest-first (cap 100 —
// worksheet history, not a paginated browser; the journal browser covers
// deep history), each with its class and note.
func (s *Service) ListTaxAdjustments(ctx context.Context, branchID int64) ([]*TaxAdjustmentRow, error) {
	entries, err := s.repo.EntriesBySourceType(ctx, branchID, SourceTaxAdjustment, 100, 0)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	// The headers are already in hand; only the legs and the class/note are
	// missing, and one grouped read fetches each.
	linesByEntry, err := s.entryLines(ctx, entries)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(entries))
	for _, e := range entries {
		e.Lines = linesByEntry[e.ID]
		ids = append(ids, e.ID)
	}
	metas, err := s.repo.TaxAdjustMetaByEntries(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]*TaxAdjustmentRow, 0, len(entries))
	for _, e := range entries {
		m := metas[e.ID]
		out = append(out, &TaxAdjustmentRow{Entry: e, Classification: m.Classification,
			ClassLabel: TaxClassLabel(m.Classification), Note: m.Note})
	}
	return out, nil
}

// TaxAdjustmentFile is one supporting scan of a correction.
type TaxAdjustmentFile struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	MIME      string `json:"mime"`
	SizeBytes int64  `json:"sizeBytes"`
	URL       string `json:"url"`
}

// taxAdjustmentEntry resolves a correction journal of this branch.
func (s *Service) taxAdjustmentEntry(ctx context.Context, branchID, id int64) (*contracts.JournalEntry, error) {
	e, err := s.repo.GetEntry(ctx, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if e == nil || e.BranchID != branchID || e.SourceType != SourceTaxAdjustment {
		return nil, apperror.NotFound("Koreksi fiskal")
	}
	return e, nil
}

// AttachTaxAdjustment stores a supporting scan or photo of a correction.
func (s *Service) AttachTaxAdjustment(ctx context.Context, actorID, branchID, id int64, up mediacontracts.Upload) (*TaxAdjustmentFile, error) {
	if s.media == nil {
		return nil, apperror.Internal(fmt.Errorf("media client belum terpasang"))
	}
	if _, err := s.taxAdjustmentEntry(ctx, branchID, id); err != nil {
		return nil, err
	}
	f, err := s.media.Upload(ctx, mediacontracts.OwnerTaxAdjustment, id, up, actorID)
	if err != nil {
		return nil, err
	}
	url, err := s.media.GetURL(ctx, f)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return &TaxAdjustmentFile{ID: f.ID, Name: f.OriginalName, MIME: f.MIME, SizeBytes: f.SizeBytes, URL: url}, nil
}

// TaxAdjustmentFiles lists the supporting scans of a correction.
func (s *Service) TaxAdjustmentFiles(ctx context.Context, branchID, id int64) ([]*TaxAdjustmentFile, error) {
	if s.media == nil {
		return nil, apperror.Internal(fmt.Errorf("media client belum terpasang"))
	}
	if _, err := s.taxAdjustmentEntry(ctx, branchID, id); err != nil {
		return nil, err
	}
	files, err := s.media.List(ctx, mediacontracts.OwnerTaxAdjustment, id)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := make([]*TaxAdjustmentFile, 0, len(files))
	for _, f := range files {
		url, err := s.media.GetURL(ctx, f)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		out = append(out, &TaxAdjustmentFile{ID: f.ID, Name: f.OriginalName, MIME: f.MIME, SizeBytes: f.SizeBytes, URL: url})
	}
	return out, nil
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

// FiscalClass is the contribution of one correction class to fiscal profit
// (FN-07): the reconciliation separates positive/negative corrections from
// final-income and non-object items instead of netting them into one line.
type FiscalClass struct {
	Classification string `json:"classification"`
	Label          string `json:"label"`
	Entries        int64  `json:"entries"`
	Effect         int64  `json:"effect"`
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
	effectOf := func(a *contracts.Account, debit, credit int64) int64 {
		switch a.Type {
		case contracts.AccountIncome:
			return credit - debit
		case contracts.AccountExpense:
			return -(debit - credit)
		}
		return 0
	}
	res := &FiscalSummaryResult{Commercial: commercial, FiscalProfit: commercial.Profit, Lines: []FiscalLine{}, Classes: []FiscalClass{}}
	for id, f := range footings {
		a, ok := byID[id]
		if !ok {
			return nil, apperror.Internal(fmt.Errorf("akun koreksi fiskal %d tidak dikenal", id))
		}
		line := FiscalLine{Code: a.Code, Name: a.Name, Type: a.Type, Debit: f[0], Credit: f[1], Effect: effectOf(a, f[0], f[1])}
		res.FiscalProfit += line.Effect
		res.Lines = append(res.Lines, line)
	}
	sort.Slice(res.Lines, func(i, j int) bool { return res.Lines[i].Code < res.Lines[j].Code })

	// The same corrections, split by class (FN-07). The class effects sum to
	// the account-level effects above by construction (one source of rows).
	classFootings, err := s.repo.TaxAdjustmentFootingsByClass(ctx, branchID, start, end)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	counts, err := s.repo.TaxAdjustmentCountsByClass(ctx, branchID, start, end)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	effect := map[string]int64{}
	for _, f := range classFootings {
		if a, ok := byID[f.AccountID]; ok {
			effect[f.Classification] += effectOf(a, f.Debit, f.Credit)
		}
	}
	seen := map[string]bool{}
	for class := range effect {
		seen[class] = true
	}
	for class := range counts {
		seen[class] = true
	}
	for class := range seen {
		res.Classes = append(res.Classes, FiscalClass{Classification: class, Label: TaxClassLabel(class),
			Entries: counts[class], Effect: effect[class]})
	}
	sort.Slice(res.Classes, func(i, j int) bool { return res.Classes[i].Classification < res.Classes[j].Classification })
	res.From, res.To = from, to
	return res, nil
}

// FiscalSummaryResult is the commercial → fiscal reconciliation.
type FiscalSummaryResult struct {
	Commercial   *ProfitLoss   `json:"commercial"`
	Lines        []FiscalLine  `json:"lines"`
	Classes      []FiscalClass `json:"classes"`
	FiscalProfit int64         `json:"fiscalProfit"`
	From         string        `json:"from"`
	To           string        `json:"to"`
}
