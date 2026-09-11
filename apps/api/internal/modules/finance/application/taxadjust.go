package application

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"sort"
	"strconv"
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
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	commercial, err := s.profitLoss(ctx, branchID, start, end)
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

// TaxPackage bundles the month's tax working papers as a ZIP: PPN summary,
// PPN detail (SPT working paper), and the fiscal worksheet. Deliberately no
// Rp4,8 M threshold logic (PRD §5 no. 7, open business decision) — the
// package carries the numbers, policy stays with the tax consultant.
func (s *Service) TaxPackage(ctx context.Context, branchID int64, year, month int) ([]byte, string, error) {
	if !validYearMonth(year, month) {
		return nil, "", apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	sum, err := s.TaxSummary(ctx, branchID, year, month)
	if err != nil {
		return nil, "", err
	}
	detail, err := s.TaxDetail(ctx, branchID, year, month)
	if err != nil {
		return nil, "", err
	}
	start, end := monthStart(year, month)[:10], monthEnd(year, month)[:10]
	fiscal, err := s.FiscalSummary(ctx, branchID, start, end)
	if err != nil {
		return nil, "", err
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeCSV := func(name string, header []string, rows [][]string) error {
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		cw := csv.NewWriter(w)
		if err := cw.Write(header); err != nil {
			return err
		}
		for _, r := range rows {
			if err := cw.Write(r); err != nil {
				return err
			}
		}
		cw.Flush()
		return cw.Error()
	}
	if err := writeCSV("ringkasan-ppn.csv",
		[]string{"tahun", "bulan", "ppn_keluaran", "ppn_masukan", "kurang_bayar"},
		[][]string{{strconv.Itoa(year), strconv.Itoa(month),
			strconv.FormatInt(sum.PpnOut, 10), strconv.FormatInt(sum.PpnIn, 10),
			strconv.FormatInt(sum.Payable, 10)}}); err != nil {
		return nil, "", apperror.Internal(err)
	}
	det := make([][]string, 0, len(detail))
	for _, r := range detail {
		det = append(det, []string{r.Date, r.Number, r.Memo, r.TaxInvoiceNumber, r.TaxInvoiceDate,
			strconv.FormatInt(r.Revenue, 10), strconv.FormatInt(r.Ppn, 10)})
	}
	if err := writeCSV("rincian-ppn.csv",
		[]string{"tanggal", "nomor", "keterangan", "no_faktur", "tgl_faktur", "omzet", "ppn"}, det); err != nil {
		return nil, "", apperror.Internal(err)
	}
	fis := make([][]string, 0, len(fiscal.Lines)+1)
	fis = append(fis, []string{"KOMERSIL", "", "", "", strconv.FormatInt(fiscal.Commercial.Profit, 10)})
	for _, l := range fiscal.Lines {
		fis = append(fis, []string{l.Code, l.Name, l.Type,
			strconv.FormatInt(l.Debit, 10), strconv.FormatInt(l.Effect, 10)})
	}
	fis = append(fis, []string{"FISKAL", "", "", "", strconv.FormatInt(fiscal.FiscalProfit, 10)})
	if err := writeCSV("rekonsiliasi-fiskal.csv",
		[]string{"kode", "akun", "tipe", "nominal", "efek_laba"}, fis); err != nil {
		return nil, "", apperror.Internal(err)
	}
	if err := zw.Close(); err != nil {
		return nil, "", apperror.Internal(err)
	}
	name := fmt.Sprintf("paket-pajak-%04d-%02d.zip", year, month)
	return buf.Bytes(), name, nil
}
