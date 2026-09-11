package application

import (
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// TaxPackage renders one month's tax working paper as a single .xlsx.
//
// Two variants, deliberately NOT the same workbook with a flag flipped —
// they answer different questions, so they carry different sheets:
//
//	PackageActual  the commercial books as they stand: profit & loss,
//	               financial position, trial balance, VAT detail, fiscal
//	               reconciliation. This is the accounting truth.
//	PackageCapped  the PP23 turnover view: how much of the year's gross
//	               turnover fits under the ceiling, which orders fall
//	               outside it, and the VAT that belongs to what remains.
//
// The capped variant carries NO balance sheet or trial balance on purpose.
// Once whole transactions are removed, a statement of financial position no
// longer describes anything real — printing one would invite a reader to
// treat a simulation as the company's actual standing.
func (s *Service) TaxPackage(ctx context.Context, branchID int64, year, month int, variant string) ([]byte, string, error) {
	if !validYearMonth(year, month) {
		return nil, "", apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	if !ValidPackageVariant(variant) {
		return nil, "", apperror.Validation("", []apperror.FieldError{{Field: "variant", Message: "jenis paket tidak dikenal"}})
	}
	if variant == PackageCapped {
		return s.buildCappedPackage(ctx, branchID, year, month)
	}
	return s.buildActualPackage(ctx, branchID, year, month)
}

// PackageReports is every figure a working paper shows, computed in ONE
// scope. Both packages render from this struct, and the limited variant is
// simply the same computation with a narrowed scope.
//
// Keeping the computation in one place is what makes the two workbooks
// comparable: they cannot drift into using different formulas, because they
// do not have different formulas.
type PackageReports struct {
	// Basis is nil for the actual books, set for the limited view.
	Basis      *TurnoverBasis
	Summary    *TaxSummary
	ProfitLoss *ProfitLoss
	Balance    *BalanceSheet
	Trial      []TrialRow
	Fiscal     *FiscalSummaryResult
	Detail     []TaxDetailRow
	Start, End string
}

// Reports computes one month's working-paper figures for the given variant.
func (s *Service) Reports(ctx context.Context, branchID int64, year, month int, variant string) (*PackageReports, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	if !ValidPackageVariant(variant) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "variant", Message: "jenis paket tidak dikenal"}})
	}
	out := &PackageReports{
		Start: monthStart(year, month)[:10],
		End:   monthEnd(year, month)[:10],
	}
	sc := fullBooks
	if variant == PackageCapped {
		basis, err := s.GrossTurnoverBasis(ctx, year, month)
		if err != nil {
			return nil, err
		}
		out.Basis = basis
		sc = bookScope{excluded: basis.ExcludedEntryIDs()}
	}
	var err error
	if out.Summary, err = s.taxSummaryScoped(ctx, branchID, year, month, sc); err != nil {
		return nil, err
	}
	if out.ProfitLoss, err = s.profitLoss(ctx, branchID, monthStart(year, month), monthEnd(year, month), sc); err != nil {
		return nil, err
	}
	out.ProfitLoss.From, out.ProfitLoss.To = out.Start, out.End
	if out.Balance, err = s.balanceSheetScoped(ctx, branchID, out.End, sc); err != nil {
		return nil, err
	}
	if out.Trial, err = s.trialBalanceScoped(ctx, branchID, year, month, sc); err != nil {
		return nil, err
	}
	if out.Fiscal, err = s.fiscalSummaryScoped(ctx, branchID, out.Start, out.End, sc); err != nil {
		return nil, err
	}
	detail, err := s.TaxDetail(ctx, branchID, year, month)
	if err != nil {
		return nil, err
	}
	// The VAT working paper is row-level, so it filters by the SAME
	// exclusion list rather than by a re-derived rule.
	if out.Basis != nil {
		kept := make([]TaxDetailRow, 0, len(detail))
		for _, r := range detail {
			if out.Basis.IsExcluded(r.EntryID) {
				continue
			}
			kept = append(kept, r)
		}
		detail = kept
	}
	out.Detail = detail
	return out, nil
}

// coverSheet writes the shared identity block both variants open with.
func (s *Service) coverSheet(f *excelize.File, variant string, year, month int) *sheet {
	sh := newSheet(f, "Ringkasan")
	sh.widths(34, 30, 18, 18, 18)
	sh.write("PAKET KERTAS KERJA PAJAK")
	sh.write("Masa pajak", fmt.Sprintf("%s %d", monthLabel(month), year))
	sh.write("Versi data", packageMetas[variant].dataLabel)
	sh.write("Dicetak", timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02 15:04"))
	sh.blank()
	return sh
}

// buildActualPackage renders the full commercial books.
func (s *Service) buildActualPackage(ctx context.Context, branchID int64, year, month int) ([]byte, string, error) {
	rep, err := s.Reports(ctx, branchID, year, month, PackageActual)
	if err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	cover := s.coverSheet(f, PackageActual, year, month)
	cover.write("PPN")
	cover.write("PPN Keluaran", rep.Summary.PpnOut)
	cover.write("PPN Masukan", rep.Summary.PpnIn)
	cover.write("Kurang (lebih) bayar", rep.Summary.Payable)
	cover.blank()
	cover.write("LABA")
	cover.write("Laba komersial", rep.ProfitLoss.Profit)
	cover.write("Laba fiskal", rep.Fiscal.FiscalProfit)
	writeReportSheets(f, rep)
	if cover.err != nil {
		return nil, "", apperror.Internal(cover.err)
	}
	raw, err := buildWorkbook(f, "Ringkasan")
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	return raw, packageFileName(PackageActual, year, month), nil
}

// buildCappedPackage renders the SAME sheets as the actual books, computed
// in a narrowed scope.
//
// Every figure comes from Service.Reports, exactly like the actual package —
// only the scope differs. An excluded order therefore cannot survive
// anywhere in this workbook: there is one exclusion list, applied at the
// single point every report reads its totals from.
//
// Two sheets are added ON TOP, not in place of anything: the ceiling working
// and the list of what was removed. A limited figure nobody can trace back
// to a rule is not a working paper, just a smaller number.
func (s *Service) buildCappedPackage(ctx context.Context, branchID int64, year, month int) ([]byte, string, error) {
	rep, err := s.Reports(ctx, branchID, year, month, PackageCapped)
	if err != nil {
		return nil, "", err
	}
	basis := rep.Basis
	f := excelize.NewFile()
	cover := s.coverSheet(f, PackageCapped, year, month)
	cover.write("DASAR PEMBATASAN")
	cover.write("Tahun pajak", basis.Year)
	cover.write("Dihitung sampai", basis.CutoffDate)
	cover.write("Lingkup", "Seluruh cabang (per wajib pajak, bukan per cabang)")
	cover.write("Plafon setahun", basis.Ceiling)
	cover.write("Peredaran bruto dipakai", basis.IncludedTurnover)
	cover.write("Sisa plafon", basis.ceilingHeadroom())
	cover.write("Peredaran bruto di luar plafon", basis.ExcludedTurnover)
	cover.write("Order di dalam plafon", countIncluded(basis))
	cover.write("Order di luar plafon", len(basis.Orders)-countIncluded(basis))
	cover.write("Jurnal dikeluarkan", basis.ExcludedCount())
	cover.blank()
	cover.write("PPN")
	cover.write("PPN Keluaran", rep.Summary.PpnOut)
	cover.write("PPN Masukan", rep.Summary.PpnIn)
	cover.write("Kurang (lebih) bayar", rep.Summary.Payable)
	cover.blank()
	cover.write("LABA")
	cover.write("Laba komersial", rep.ProfitLoss.Profit)
	cover.write("Laba fiskal", rep.Fiscal.FiscalProfit)
	cover.blank()
	cover.write("Catatan", "Seluruh angka di berkas ini sudah mengeluarkan order yang melewati")
	cover.write("", "plafon peredaran bruto tahun berjalan — berikut HPP, PPN, dan")
	cover.write("", "pelunasannya. Order dikeluarkan seutuhnya, tidak dipotong pas")
	cover.write("", "plafon, sehingga peredaran yang dipakai berada DI BAWAH plafon,")
	cover.write("", "bukan tepat di angkanya.")

	writeReportSheets(f, rep)

	rec := newSheet(f, "Peredaran Bruto")
	rec.widths(14, 24, 30, 18, 20, 16)
	rec.write("Tanggal", "No. Order", "Pelanggan", "Peredaran bruto", "Kumulatif", "Status")
	for _, o := range basis.Orders {
		if !o.Included {
			continue
		}
		rec.write(o.FirstDate, o.Number, o.PartyName, o.Revenue, o.Cumulative, "Di dalam plafon")
	}
	rec.blank()
	rec.write("", "", "TOTAL", basis.IncludedTurnover, "", "")

	exc := newSheet(f, "Transaksi Dikecualikan")
	exc.widths(14, 24, 30, 18, 44)
	exc.write("Tanggal", "No. Order", "Pelanggan", "Peredaran bruto", "Alasan")
	for _, o := range basis.Orders {
		if o.Included {
			continue
		}
		exc.write(o.FirstDate, o.Number, o.PartyName, o.Revenue,
			"Melewati plafon peredaran bruto tahun berjalan")
	}
	exc.blank()
	exc.write("", "", "TOTAL", basis.ExcludedTurnover, "")
	if len(basis.StraddlingPayments) > 0 {
		exc.blank()
		exc.write("Perhatian", "Pelunasan berikut menyangkut order di dalam DAN di luar plafon,")
		exc.write("", "sehingga tetap ditampilkan agar kas order yang masih masuk tidak hilang:")
		for _, ref := range basis.StraddlingPayments {
			exc.write("", ref)
		}
	}

	for _, sh := range []*sheet{cover, rec, exc} {
		if sh.err != nil {
			return nil, "", apperror.Internal(sh.err)
		}
	}
	raw, err := buildWorkbook(f, "Ringkasan")
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	return raw, packageFileName(PackageCapped, year, month), nil
}

// writeReportSheets lays down the five statement sheets both packages carry,
// in the same order, from the same figures.
func writeReportSheets(f *excelize.File, rep *PackageReports) {
	writeProfitLossSheet(f, rep.ProfitLoss, rep.Start, rep.End)
	writeBalanceSheet(f, rep.Balance)
	writeTrialBalanceSheet(f, rep.Trial)
	writeVatSheet(f, rep.Detail)
	writeFiscalSheet(f, rep.Fiscal)
}

// writeVatSheet renders the VAT working paper. Filtering already happened in
// Reports(), so this writer cannot disagree with the other sheets.
func writeVatSheet(f *excelize.File, detail []TaxDetailRow) {
	vat := newSheet(f, "Rincian PPN")
	vat.widths(14, 22, 34, 24, 14, 18, 16)
	vat.write("Tanggal", "Nomor", "Keterangan", "No. Faktur", "Tgl Faktur", "Omzet", "PPN")
	var omzet, ppn int64
	for _, r := range detail {
		vat.write(r.Date, r.Number, r.Memo, r.TaxInvoiceNumber, r.TaxInvoiceDate, r.Revenue, r.Ppn)
		omzet += r.Revenue
		ppn += r.Ppn
	}
	vat.blank()
	vat.write("", "", "TOTAL", "", "", omzet, ppn)
}

func writeTrialGroup(sh *sheet, title string, rows []TrialRow) {
	sh.blank()
	sh.write(title)
	sh.write("Kode", "Akun", "Saldo")
	for _, r := range rows {
		sh.write(r.Code, r.Name, r.Debit-r.Credit)
	}
}

func countIncluded(b *TurnoverBasis) int {
	n := 0
	for _, o := range b.Orders {
		if o.Included {
			n++
		}
	}
	return n
}

func boolLabel(v bool) string {
	if v {
		return "Ya"
	}
	return "Tidak"
}

// The sheet writers below are shared by BOTH packages. Same layout, same
// arithmetic — the only difference between the two workbooks is the scope
// the figures were computed in, never the way they are presented.

func writeProfitLossSheet(f *excelize.File, pl *ProfitLoss, start, end string) {
	sh := newSheet(f, "Untung Rugi")
	sh.widths(30, 20)
	sh.write("Periode", start+" s/d "+end)
	sh.blank()
	sh.write("Pendapatan", pl.Revenue)
	sh.write("Harga Pokok Penjualan", pl.Cogs)
	sh.write("Laba Kotor", pl.Gross)
	sh.write("Beban Usaha", pl.Expense)
	sh.write("Laba Bersih", pl.Profit)
}

func writeBalanceSheet(f *excelize.File, bs *BalanceSheet) {
	sh := newSheet(f, "Posisi Harta Hutang")
	sh.widths(12, 40, 20)
	sh.write("Per tanggal", bs.Date)
	writeTrialGroup(sh, "HARTA", bs.Assets)
	writeTrialGroup(sh, "HUTANG", bs.Liabilities)
	writeTrialGroup(sh, "MODAL", bs.Equity)
	sh.blank()
	sh.write("Total Harta", "", bs.TotalAssets)
	sh.write("Total Hutang + Modal", "", bs.TotalLiaEq)
	sh.write("Seimbang", "", boolLabel(bs.Balanced))
}

func writeTrialBalanceSheet(f *excelize.File, tb []TrialRow) {
	sh := newSheet(f, "Cek Saldo Akun")
	sh.widths(12, 40, 16, 18, 18)
	sh.write("Kode", "Akun", "Tipe", "Debit", "Kredit")
	var totalDebit, totalCredit int64
	for _, r := range tb {
		sh.write(r.Code, r.Name, r.Type, r.Debit, r.Credit)
		totalDebit += r.Debit
		totalCredit += r.Credit
	}
	sh.write("", "TOTAL", "", totalDebit, totalCredit)
}

func writeFiscalSheet(f *excelize.File, fiscal *FiscalSummaryResult) {
	sh := newSheet(f, "Rekonsiliasi Fiskal")
	sh.widths(12, 40, 16, 18, 18)
	sh.write("Laba komersial", "", "", "", fiscal.Commercial.Profit)
	sh.write("Kode", "Akun", "Tipe", "Nominal", "Efek ke laba")
	for _, l := range fiscal.Lines {
		sh.write(l.Code, l.Name, l.Type, l.Debit, l.Effect)
	}
	sh.write("Laba fiskal", "", "", "", fiscal.FiscalProfit)
}
