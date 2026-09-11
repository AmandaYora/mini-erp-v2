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
	start, end := monthStart(year, month)[:10], monthEnd(year, month)[:10]

	sum, err := s.TaxSummary(ctx, branchID, year, month)
	if err != nil {
		return nil, "", err
	}
	detail, err := s.TaxDetail(ctx, branchID, year, month)
	if err != nil {
		return nil, "", err
	}
	pl, err := s.ProfitLoss(ctx, branchID, start, end)
	if err != nil {
		return nil, "", err
	}
	bs, err := s.BalanceSheet(ctx, branchID, end)
	if err != nil {
		return nil, "", err
	}
	tb, err := s.TrialBalance(ctx, branchID, year, month)
	if err != nil {
		return nil, "", err
	}
	fiscal, err := s.FiscalSummary(ctx, branchID, start, end)
	if err != nil {
		return nil, "", err
	}

	f := excelize.NewFile()
	cover := s.coverSheet(f, PackageActual, year, month)
	cover.write("PPN")
	cover.write("PPN Keluaran", sum.PpnOut)
	cover.write("PPN Masukan", sum.PpnIn)
	cover.write("Kurang (lebih) bayar", sum.Payable)
	cover.blank()
	cover.write("LABA")
	cover.write("Laba komersial", pl.Profit)
	cover.write("Laba fiskal", fiscal.FiscalProfit)

	pls := newSheet(f, "Untung Rugi")
	pls.widths(30, 20)
	pls.write("Periode", start+" s/d "+end)
	pls.blank()
	pls.write("Pendapatan", pl.Revenue)
	pls.write("Harga Pokok Penjualan", pl.Cogs)
	pls.write("Laba Kotor", pl.Gross)
	pls.write("Beban Usaha", pl.Expense)
	pls.write("Laba Bersih", pl.Profit)

	bss := newSheet(f, "Posisi Harta Hutang")
	bss.widths(12, 40, 20)
	bss.write("Per tanggal", bs.Date)
	writeTrialGroup(bss, "HARTA", bs.Assets)
	writeTrialGroup(bss, "HUTANG", bs.Liabilities)
	writeTrialGroup(bss, "MODAL", bs.Equity)
	bss.blank()
	bss.write("Total Harta", "", bs.TotalAssets)
	bss.write("Total Hutang + Modal", "", bs.TotalLiaEq)
	bss.write("Seimbang", "", boolLabel(bs.Balanced))

	tbs := newSheet(f, "Cek Saldo Akun")
	tbs.widths(12, 40, 16, 18, 18)
	tbs.write("Kode", "Akun", "Tipe", "Debit", "Kredit")
	var totalDebit, totalCredit int64
	for _, r := range tb {
		tbs.write(r.Code, r.Name, r.Type, r.Debit, r.Credit)
		totalDebit += r.Debit
		totalCredit += r.Credit
	}
	tbs.write("", "TOTAL", "", totalDebit, totalCredit)

	writeVatSheet(f, detail, nil)

	fis := newSheet(f, "Rekonsiliasi Fiskal")
	fis.widths(12, 40, 16, 18, 18)
	fis.write("Laba komersial", "", "", "", fiscal.Commercial.Profit)
	fis.write("Kode", "Akun", "Tipe", "Nominal", "Efek ke laba")
	for _, l := range fiscal.Lines {
		fis.write(l.Code, l.Name, l.Type, l.Debit, l.Effect)
	}
	fis.write("Laba fiskal", "", "", "", fiscal.FiscalProfit)

	for _, sh := range []*sheet{cover, pls, bss, tbs, fis} {
		if sh.err != nil {
			return nil, "", apperror.Internal(sh.err)
		}
	}
	raw, err := buildWorkbook(f, "Ringkasan")
	if err != nil {
		return nil, "", apperror.Internal(err)
	}
	return raw, packageFileName(PackageActual, year, month), nil
}

// buildCappedPackage renders the gross-turnover-limited view.
func (s *Service) buildCappedPackage(ctx context.Context, branchID int64, year, month int) ([]byte, string, error) {
	basis, err := s.GrossTurnoverBasis(ctx, year, month)
	if err != nil {
		return nil, "", err
	}
	detail, err := s.TaxDetail(ctx, branchID, year, month)
	if err != nil {
		return nil, "", err
	}

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
	cover.blank()
	cover.write("Jumlah order di dalam plafon", countIncluded(basis))
	cover.write("Jumlah order di luar plafon", len(basis.Orders)-countIncluded(basis))
	cover.write("Jurnal dikeluarkan dari tampilan", basis.ExcludedCount())
	cover.blank()
	cover.write("Catatan", "Peredaran bruto dihitung kumulatif sejak 1 Januari tahun pajak,")
	cover.write("", "berurutan menurut tanggal transaksi. Order yang melewati plafon")
	cover.write("", "dikeluarkan seutuhnya (penjualan, HPP, PPN, dan pelunasannya)")
	cover.write("", "sehingga setiap laporan tetap seimbang.")

	rec := newSheet(f, "Peredaran Bruto")
	rec.widths(14, 24, 30, 18, 20, 14)
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
	exc.widths(14, 24, 30, 18, 40)
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

	writeVatSheet(f, detail, basis)

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

// writeVatSheet renders VAT detail. When basis is non-nil, rows whose journal
// entry sits outside the ceiling are dropped — the same single source of
// exclusion the cover sheet reports, so the two can never disagree.
func writeVatSheet(f *excelize.File, detail []TaxDetailRow, basis *TurnoverBasis) {
	vat := newSheet(f, "Rincian PPN")
	vat.widths(14, 22, 34, 24, 14, 18, 16)
	vat.write("Tanggal", "Nomor", "Keterangan", "No. Faktur", "Tgl Faktur", "Omzet", "PPN")
	var omzet, ppn int64
	for _, r := range detail {
		if basis != nil && basis.IsExcluded(r.EntryID) {
			continue
		}
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
