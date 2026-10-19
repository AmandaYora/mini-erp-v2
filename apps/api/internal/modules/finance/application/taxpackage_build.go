package application

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"mini-erp/internal/modules/finance/infrastructure"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// TaxPackage renders one month's tax working paper as a single .xlsx.
//
// Two variants, deliberately NOT the same workbook with a flag flipped —
// they answer different questions, so they carry different sheets:
//
//	PackageActual  the commercial books as they stand: profit & loss,
//	               financial position, trial balance, VAT detail (PPN
//	               Keluaran and PPN Masukan), fiscal reconciliation and the
//	               general ledger. This is the accounting truth.
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
	return s.TaxPackageRange(ctx, branchID, monthStart(year, month)[:10], monthEnd(year, month)[:10], variant)
}

// TaxPackageRange renders the working paper for any inclusive YYYY-MM-DD range
// (FN-04): a month, a quarter, a year or a custom span. The capped variant
// accumulates the ceiling from 1 January of the range's year, so a capped range
// must lie inside ONE tax year.
func (s *Service) TaxPackageRange(ctx context.Context, branchID int64, from, to, variant string) ([]byte, string, error) {
	if _, _, err := validateRange(from, to); err != nil {
		return nil, "", err
	}
	if !ValidPackageVariant(variant) {
		return nil, "", apperror.Validation("", []apperror.FieldError{{Field: "variant", Message: "jenis paket tidak dikenal"}})
	}
	if variant == PackageCapped {
		return s.buildCappedPackage(ctx, branchID, from, to)
	}
	return s.buildActualPackage(ctx, branchID, from, to)
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
	// Ledger is the general ledger of every account in the range (FN-04).
	Ledger []LedgerBlock
	// Multi is true when the package spans several branches (?branch=all|<other>):
	// the sheets then carry a Cabang column and the cover names the branches.
	Multi        bool
	BranchLabels []string
	Start, End   string
}

// LedgerBlock is one account of the package's general-ledger sheet: the
// summary row plus every leg inside the range, with running balance.
type LedgerBlock struct {
	Account LedgerAccountRow
	Lines   []LedgerLine
}

// maxPackageLedgerLines bounds the general-ledger sheet. A range with more legs
// is refused loudly (narrow the range) instead of being cut short.
const maxPackageLedgerLines = 60000

// Reports computes one month's working-paper figures for the given variant.
func (s *Service) Reports(ctx context.Context, branchID int64, year, month int, variant string) (*PackageReports, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	return s.ReportsRange(ctx, branchID, monthStart(year, month)[:10], monthEnd(year, month)[:10], variant)
}

// ReportsRange computes the working-paper figures of an inclusive range.
func (s *Service) ReportsRange(ctx context.Context, branchID int64, from, to, variant string) (*PackageReports, error) {
	start, end, verr := validateRange(from, to)
	if verr != nil {
		return nil, verr
	}
	if !ValidPackageVariant(variant) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "variant", Message: "jenis paket tidak dikenal"}})
	}
	out := &PackageReports{Start: from, End: to}
	scope := []int64{branchID}
	if ids, ok := infrastructure.BranchSetFrom(ctx); ok {
		scope = ids
	}
	out.Multi = len(scope) > 1
	names := s.branchNames(ctx, scope)
	for _, id := range scope {
		label := names[id]
		if label == "" {
			label = "#" + strconv.FormatInt(id, 10)
		}
		out.BranchLabels = append(out.BranchLabels, label)
	}
	sc := fullBooks
	if variant == PackageCapped {
		if from[:4] != to[:4] {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "to",
				Message: "paket dibatasi plafon hanya untuk rentang dalam satu tahun pajak"}})
		}
		year, _ := strconv.Atoi(to[:4])
		basis, err := s.grossTurnoverBasisThrough(ctx, year, end)
		if err != nil {
			return nil, err
		}
		out.Basis = basis
		sc = bookScope{excluded: basis.ExcludedEntryIDs()}
	}
	var err error
	if out.Summary, err = s.taxSummaryWindow(ctx, branchID, start, end, sc, 1, 0); err != nil {
		return nil, err
	}
	out.Summary.From, out.Summary.To = from, to
	if out.ProfitLoss, err = s.profitLoss(ctx, branchID, start, end, sc); err != nil {
		return nil, err
	}
	out.ProfitLoss.From, out.ProfitLoss.To = out.Start, out.End
	if out.Balance, err = s.balanceSheetScoped(ctx, branchID, out.End, sc); err != nil {
		return nil, err
	}
	if out.Trial, err = s.trialBalanceWindow(ctx, branchID, start, end, sc); err != nil {
		return nil, err
	}
	if out.Fiscal, err = s.fiscalSummaryScoped(ctx, branchID, out.Start, out.End, sc); err != nil {
		return nil, err
	}
	// The VAT working paper carries the SAME rows as the TaxDetailReport screen:
	// sales, sales returns, purchases (PPN Masukan) and purchase returns.
	detail, err := s.taxRows(ctx, branchID, start, end, taxSourcesAll)
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
	if out.Ledger, err = s.ledgerBlocks(ctx, branchID, from, to, sc, out.Multi, names); err != nil {
		return nil, err
	}
	return out, nil
}

// ledgerBlocks builds the all-accounts ledger of the package: per account the
// summary row, then each leg with a running balance in the account's normal
// direction. Entries the capped scope removed are removed here too.
func (s *Service) ledgerBlocks(ctx context.Context, branchID int64, from, to string, sc bookScope, multi bool, branchNames map[int64]string) ([]LedgerBlock, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	summary, _, _, err := s.ledgerAccountRows(ctx, branchID, from, to, sc)
	if err != nil {
		return nil, err
	}
	legs, err := s.repo.LedgerLinesAll(ctx, branchID, start, end, maxPackageLedgerLines+len(sc.excluded))
	if err != nil {
		if errors.Is(err, infrastructure.ErrLedgerTooLarge) {
			return nil, apperror.Conflict("Buku besar rentang ini terlalu besar untuk satu berkas; persempit rentang tanggal")
		}
		return nil, apperror.Internal(err)
	}
	excluded := map[int64]bool{}
	for _, id := range sc.excluded {
		excluded[id] = true
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	codeOf := map[int64]string{}
	for _, a := range accounts {
		codeOf[a.ID] = a.Code
	}
	byCode := map[string]*LedgerBlock{}
	order := make([]*LedgerBlock, 0, len(summary))
	running := map[string]int64{}
	for _, row := range summary {
		b := &LedgerBlock{Account: row}
		byCode[row.Code] = b
		order = append(order, b)
		running[row.Code] = row.Opening
	}
	var partyIDs, productIDs []int64
	for _, l := range legs {
		if l.PartyID != 0 {
			partyIDs = append(partyIDs, l.PartyID)
		}
		if l.ProductID != 0 {
			productIDs = append(productIDs, l.ProductID)
		}
	}
	parties := s.partyLabels(ctx, partyIDs)
	products := s.productLabels(ctx, productIDs)
	for _, l := range legs {
		if excluded[l.EntryID] {
			continue
		}
		code := codeOf[l.AccountID]
		b := byCode[code]
		if b == nil {
			continue
		}
		if b.Account.Normal == "debit" {
			running[code] += l.Debit - l.Credit
		} else {
			running[code] += l.Credit - l.Debit
		}
		line := LedgerLine{EntryID: l.EntryID, Number: l.Number, Date: l.Date, Memo: l.Memo,
			Debit: l.Debit, Credit: l.Credit, Balance: running[code],
			PartyID: l.PartyID, ProductID: l.ProductID, Description: l.Description}
		if lab, ok := parties[l.PartyID]; ok {
			line.PartyName = lab.Name
		}
		if lab, ok := products[l.ProductID]; ok {
			line.ProductName = lab.Name
		}
		if multi {
			line.BranchName = branchNames[l.BranchID]
		}
		b.Lines = append(b.Lines, line)
	}
	out := make([]LedgerBlock, 0, len(order))
	for _, b := range order {
		out = append(out, *b)
	}
	return out, nil
}

// coverSheet writes the shared identity block both variants open with.
func (s *Service) coverSheet(f *excelize.File, variant string, from, to string, rep *PackageReports) *sheet {
	sh := newSheet(f, "Ringkasan")
	sh.widths(34, 30, 18, 18, 18)
	sh.write("PAKET KERTAS KERJA PAJAK")
	sh.write("Masa pajak", periodLabelOf(from, to))
	if rep.Multi {
		sh.write("Cabang", strings.Join(rep.BranchLabels, ", "))
	} else if len(rep.BranchLabels) == 1 {
		sh.write("Cabang", rep.BranchLabels[0])
	}
	sh.write("Versi data", packageMetas[variant].dataLabel)
	sh.write("Dicetak", timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02 15:04"))
	sh.blank()
	return sh
}

// writeVatCover lays the VAT block of the cover sheet (both variants).
func writeVatCover(cover *sheet, sum *TaxSummary) {
	cover.write("PPN")
	cover.write("DPP penjualan", sum.SalesDpp)
	cover.write("PPN Keluaran", sum.PpnOut)
	cover.write("DPP pembelian", sum.PurchaseDpp)
	cover.write("PPN Masukan", sum.PpnIn)
	cover.write("Kurang (lebih) bayar", sum.Payable)
	cover.write("Transaksi kena pajak tanpa faktur", sum.ExceptionsMeta.Total)
}

// buildActualPackage renders the full commercial books.
func (s *Service) buildActualPackage(ctx context.Context, branchID int64, from, to string) ([]byte, string, error) {
	rep, err := s.ReportsRange(ctx, branchID, from, to, PackageActual)
	if err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	cover := s.coverSheet(f, PackageActual, from, to, rep)
	writeVatCover(cover, rep.Summary)
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
	return raw, packageFileName(PackageActual, from, to), nil
}

// buildCappedPackage renders the SAME sheets as the actual books, computed
// in a narrowed scope.
//
// Every figure comes from Service.ReportsRange, exactly like the actual package
// — only the scope differs. An excluded order therefore cannot survive
// anywhere in this workbook: there is one exclusion list, applied at the
// single point every report reads its totals from.
//
// Two sheets are added ON TOP, not in place of anything: the ceiling working
// and the list of what was removed. A limited figure nobody can trace back
// to a rule is not a working paper, just a smaller number.
func (s *Service) buildCappedPackage(ctx context.Context, branchID int64, from, to string) ([]byte, string, error) {
	rep, err := s.ReportsRange(ctx, branchID, from, to, PackageCapped)
	if err != nil {
		return nil, "", err
	}
	basis := rep.Basis
	f := excelize.NewFile()
	cover := s.coverSheet(f, PackageCapped, from, to, rep)
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
	writeVatCover(cover, rep.Summary)
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
	return raw, packageFileName(PackageCapped, from, to), nil
}

// writeReportSheets lays down the statement sheets both packages carry, in the
// same order, from the same figures.
func writeReportSheets(f *excelize.File, rep *PackageReports) {
	writeProfitLossSheet(f, rep.ProfitLoss, rep.Start, rep.End)
	writeBalanceSheet(f, rep.Balance)
	writeTrialBalanceSheet(f, rep.Trial)
	writeVatSheet(f, rep.Detail, rep.Multi)
	writeInputVatSheet(f, rep.Detail, rep.Multi)
	writeFiscalSheet(f, rep.Fiscal)
	writeLedgerSheet(f, rep.Ledger, rep.Multi)
}

var taxKindLabels = map[string]string{
	"sales": "Penjualan", "sales_return": "Retur penjualan",
	"purchase": "Pembelian", "purchase_return": "Retur pembelian",
}

// writeVatSheet renders the PPN Keluaran working paper: sales and sales
// returns. Filtering already happened in ReportsRange(), so this writer cannot
// disagree with the other sheets.
func writeVatSheet(f *excelize.File, detail []TaxDetailRow, multi bool) {
	vat := newSheet(f, "Rincian PPN")
	vat.widths(14, 22, 18, 34, 28, 22, 24, 14, 18, 18, 16, 18)
	vat.write(bcol(multi, "Cabang", "Tanggal", "Nomor", "Jenis", "Keterangan", "Pelanggan", "Pesanan", "No. Faktur", "Tgl Faktur", "Omzet", "DPP", "PPN Keluaran")...)
	var omzet, dpp, ppn int64
	for _, r := range detail {
		if r.Kind != "sales" && r.Kind != "sales_return" {
			continue
		}
		vat.write(bcol(multi, r.BranchName, r.Date, r.Number, taxKindLabels[r.Kind], r.Memo, r.PartyName, r.OrderNumber, r.TaxInvoiceNumber, r.TaxInvoiceDate, r.Revenue, r.Dpp, r.Ppn)...)
		omzet += r.Revenue
		dpp += r.Dpp
		ppn += r.Ppn
	}
	vat.blank()
	vat.write(bcol(multi, "", "", "", "", "TOTAL", "", "", "", "", omzet, dpp, ppn)...)
}

// bcol inserts the branch cell after the first column when the package spans
// several branches (a header passes "Cabang", a data row its branch name).
func bcol(multi bool, branch string, cols ...any) []any {
	if !multi {
		return cols
	}
	return append([]any{cols[0], branch}, cols[1:]...)
}

// writeInputVatSheet renders the PPN Masukan working paper (FN-04): receipts
// that carry input VAT and purchase returns that take it back, flagging the
// purchases that lack the supplier's invoice number.
func writeInputVatSheet(f *excelize.File, detail []TaxDetailRow, multi bool) {
	vat := newSheet(f, "PPN Masukan")
	vat.widths(14, 22, 18, 34, 28, 22, 24, 14, 18, 16, 14, 18)
	vat.write(bcol(multi, "Cabang", "Tanggal", "Nomor", "Jenis", "Keterangan", "Pemasok", "Pesanan", "No. Faktur Pemasok", "Tgl Faktur", "DPP", "PPN Masukan", "Tanpa faktur")...)
	var dpp, ppn int64
	for _, r := range detail {
		if r.Kind != "purchase" && r.Kind != "purchase_return" {
			continue
		}
		vat.write(bcol(multi, r.BranchName, r.Date, r.Number, taxKindLabels[r.Kind], r.Memo, r.PartyName, r.OrderNumber, r.TaxInvoiceNumber, r.TaxInvoiceDate, r.Dpp, r.PpnIn, boolLabel(r.MissingInvoice))...)
		dpp += r.Dpp
		ppn += r.PpnIn
	}
	vat.blank()
	vat.write(bcol(multi, "", "", "", "", "TOTAL", "", "", "", "", dpp, ppn, "")...)
}

// writeLedgerSheet lays the general ledger of every account (FN-04): a summary
// row per account followed by its legs with the running balance.
func writeLedgerSheet(f *excelize.File, blocks []LedgerBlock, multi bool) {
	gl := newSheet(f, "Buku Besar")
	gl.widths(14, 22, 36, 26, 26, 16, 16, 18, 18)
	for _, b := range blocks {
		gl.write(bcol(multi, "", b.Account.Code, b.Account.Name, "Saldo awal", "", "", "", "", b.Account.Opening)...)
		gl.write(bcol(multi, "Cabang", "Tanggal", "Nomor", "Keterangan", "Pihak", "Produk", "Debit", "Kredit", "Saldo")...)
		for _, l := range b.Lines {
			gl.write(bcol(multi, l.BranchName, l.Date, l.Number, l.Memo, l.PartyName, l.ProductName, l.Debit, l.Credit, l.Balance)...)
		}
		gl.write(bcol(multi, "", "", "", "Total / saldo akhir", "", "", b.Account.Debit, b.Account.Credit, b.Account.Closing)...)
		gl.blank()
	}
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
	sh.widths(40, 20)
	sh.write("Periode", start+" s/d "+end)
	sh.blank()
	section := func(title string, lines []PLLine, total int64) {
		sh.write(title, total)
		for _, l := range lines {
			sh.write("   "+l.Code+" "+l.Name, l.Amount)
		}
	}
	section("Pendapatan", pl.RevenueLines, pl.Revenue)
	section("Harga Pokok Penjualan", pl.CogsLines, pl.Cogs)
	sh.write("Laba Kotor", pl.Gross)
	section("Beban Usaha", pl.ExpenseLines, pl.Expense)
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
	if len(fiscal.Classes) > 0 {
		sh.blank()
		sh.write("Per jenis koreksi", "", "", "Jumlah koreksi", "Efek ke laba")
		for _, c := range fiscal.Classes {
			sh.write(c.Label, "", "", c.Entries, c.Effect)
		}
	}
}
