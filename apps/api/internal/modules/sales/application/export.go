package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/tableexport"
	"mini-erp/internal/shared/timeutil"
)

// Order list export (J1, F4-11, SL-04): .xlsx (stream writer, three sheets) and
// .pdf (capped). Rows walk the SAME filter as the list by keyset (id < last id),
// 500 per page, so every page costs the same however deep the export goes; each
// page resolves its customer names, user names, paid amounts, return effects,
// payment methods, drivers and lines in a constant number of grouped reads.
//
// xlsx sheets (legacy parity): "Ringkasan" (totals, written last from the
// running sums), "Daftar Order" (28 columns) and "Detail Item".

const exportPageSize = 500

// MarginReader is the books' estimate of the gross margin over a date range. It
// is implemented outside this module (finance owns the figure; sales may not
// import finance without a dependency cycle) and wired in bootstrap. Nil = the
// summary simply omits the margin line.
type MarginReader interface {
	// GrossMargin sums the realised gross margin of the branch over an inclusive
	// YYYY-MM-DD range; incomplete reports that part of the cost is an estimate.
	GrossMargin(ctx context.Context, branchID int64, from, to string) (gross int64, incomplete bool, err error)
}

// SetMarginReader closes the late-bound edge to the margin source.
func (s *Service) SetMarginReader(r MarginReader) { s.margin = r }

// ExportOptions are the per-request switches of the export.
type ExportOptions struct {
	// IncludeMargin adds the margin line to the summary; the handler sets it only
	// for a caller allowed to see books figures.
	IncludeMargin bool
}

var salesExportHeader = []string{
	"Nomor Order", "Tanggal Order", "Customer", "Kanal", "Status Order", "Termin", "Jatuh Tempo",
	"Subtotal", "Diskon", "DPP", "Pajak", "Total Order", "Penyesuaian Retur", "Total Bersih",
	"Dibayar", "Sisa", "Status Bayar", "Status Finansial", "Metode Bayar", "Kasir",
	"Dibuat Oleh", "Penanggung Jawab", "Sopir", "No Plat", "No Faktur Pajak", "Tgl Faktur Pajak",
	"Persetujuan Kredit", "Catatan",
}

var salesExportItemHeader = []string{
	"Nomor Order", "Tanggal Order", "Customer", "Kode Produk", "Nama Produk", "Varian",
	"Qty", "Satuan", "Satuan Dasar", "Faktor", "Qty Dasar", "Harga Satuan",
	"Diskon %", "Diskon Nominal", "DPP Baris", "Pajak Baris", "Total Baris", "Sumber Harga",
}

// exportBatch is one keyset page, fully resolved.
type exportBatch struct {
	orders []*contracts.SalesOrder
	sums   map[int64]PaymentSummary
	labels paymentLabels
	trans  map[int64]deliverycontracts.OrderTransport
}

// nextBatch reads the next keyset page (after = last id of the previous one),
// applies the payment-status filter and resolves the page. done reports that the
// walk reached the end; a page can be empty while more remain (every row of it
// was filtered out), which the caller must not mistake for the end.
func (s *Service) nextBatch(ctx context.Context, branchID int64, f ListFilter, after *int64) (b *exportBatch, done bool, err error) {
	raw, err := s.repo.ListAfter(ctx, branchID, f, *after, exportPageSize)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	if len(raw) == 0 {
		return nil, true, nil
	}
	*after = raw[len(raw)-1].ID
	done = len(raw) < exportPageSize
	sums, err := s.PaymentSummaries(ctx, raw)
	if err != nil {
		return nil, false, err
	}
	orders := raw
	if f.PaymentStatus != "" {
		orders = orders[:0:0]
		for _, o := range raw {
			if matchesPayment(sums[o.ID], f.PaymentStatus) {
				orders = append(orders, o)
			}
		}
	}
	if len(orders) == 0 {
		return &exportBatch{}, done, nil
	}
	if err := s.decorateParties(ctx, orders); err != nil {
		return nil, false, err
	}
	if err := s.decorateUsers(ctx, orders); err != nil {
		return nil, false, err
	}
	ids := make([]int64, 0, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	b = &exportBatch{orders: orders, sums: sums, labels: s.paymentLabelsFor(ctx, ids)}
	if s.dep.deliveries != nil {
		// Best-effort: a missing driver label never fails the export.
		if t, terr := s.dep.deliveries.TransportByOrders(ctx, ids); terr == nil {
			b.trans = t
		}
	}
	return b, done, nil
}

func shortDate(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

var (
	exportStatusLabel = map[string]string{
		contracts.StatusDraft: "Draf", contracts.StatusConfirmed: "Terkonfirmasi",
		contracts.StatusCompleted: "Selesai", contracts.StatusCancelled: "Dibatalkan",
	}
	exportPayLabel = map[string]string{PayUnpaid: "Belum Bayar", PayPartial: "Bayar Sebagian", PayPaid: "Lunas"}
	exportFinLabel = map[string]string{
		"settled": "Lunas", "overdue": "Jatuh Tempo", "partial": "Dibayar Sebagian", "open": "Terbuka",
	}
	exportTermsLabel   = map[string]string{"cod": "COD / Tunai", "net": "Net / Tempo", "prepaid": "Prabayar"}
	exportChannelLabel = map[string]string{contracts.ChannelRegular: "Reguler", contracts.ChannelPOS: "POS"}
)

func labelOr(m map[string]string, k string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return k
}

// orderCells renders the 28 "Daftar Order" columns. Money stays numeric.
func (b *exportBatch) orderCells(o *contracts.SalesOrder) []any {
	sum := b.sums[o.ID]
	t := b.trans[o.ID]
	credit := ""
	if o.CreditApprovedAt != "" {
		credit = strings.TrimSpace(o.CreditApprovedByName + " · " + o.CreditApprovedAt)
	}
	cashier := b.labels.payers[o.ID]
	if cashier == "" && o.Channel == contracts.ChannelPOS {
		cashier = o.CreatedByName
	}
	return []any{
		o.Number, shortDate(o.OrderDate), o.PartyName, labelOr(exportChannelLabel, o.Channel),
		labelOr(exportStatusLabel, o.Status), labelOr(exportTermsLabel, o.PaymentTerms), shortDate(o.DueDate),
		o.Subtotal, o.DiscountTotal, o.GrandTotal - o.TaxTotal, o.TaxTotal, o.GrandTotal,
		sum.ReturnAdjustment, sum.NetTotal,
		sum.PaidAmount, sum.OutstandingAmount, labelOr(exportPayLabel, sum.PaymentStatus),
		labelOr(exportFinLabel, sum.FinancialStatus), b.labels.methods[o.ID], cashier,
		o.CreatedByName, o.AssignedUserName, t.Drivers, t.Plates, o.TaxInvoiceNumber, shortDate(o.TaxInvoiceDate),
		credit, o.Notes,
	}
}

// exportSummary accumulates the "Ringkasan" figures while the pages stream by.
type exportSummary struct {
	orders                                     int64
	byStatus                                   map[string]int64
	original, adjustment, net                  int64
	paid, outstanding, refundDue               int64
	dpp, tax                                   int64
	unpaid, partial, paidFull, overdue         int64
	completedGross, completedAdj, completedNet int64
}

func (e *exportSummary) add(o *contracts.SalesOrder, sum PaymentSummary) {
	e.orders++
	if e.byStatus == nil {
		e.byStatus = map[string]int64{}
	}
	e.byStatus[o.Status]++
	if o.Status == contracts.StatusCancelled {
		return
	}
	e.original += o.GrandTotal
	e.adjustment += sum.ReturnAdjustment
	e.net += sum.NetTotal
	e.paid += sum.PaidAmount
	e.outstanding += sum.OutstandingAmount
	e.refundDue += sum.RefundDue
	e.dpp += o.GrandTotal - o.TaxTotal
	e.tax += o.TaxTotal
	if o.Status == contracts.StatusDraft {
		return
	}
	switch sum.PaymentStatus {
	case PayUnpaid:
		e.unpaid++
	case PayPartial:
		e.partial++
	case PayPaid:
		e.paidFull++
	}
	if sum.IsOverdue {
		e.overdue++
	}
	if o.Status == contracts.StatusCompleted {
		e.completedGross += o.GrandTotal
		e.completedAdj += sum.ReturnAdjustment
		e.completedNet += sum.NetTotal
	}
}

// filterLines describes the active filter for the summary sheet.
func filterLines(f ListFilter) [][]any {
	var out [][]any
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			out = append(out, []any{k, v})
		}
	}
	add("Pencarian", f.Search)
	add("Status order", strings.Join(f.Statuses, ", "))
	add("Kanal", labelOr(exportChannelLabel, f.Channel))
	add("Termin", labelOr(exportTermsLabel, f.PaymentTerms))
	add("Status bayar", labelOr(map[string]string{PayUnpaid: "Belum bayar", PayPartial: "Sebagian", PayPaid: "Lunas", PayOverdue: "Jatuh tempo"}, f.PaymentStatus))
	if f.From != "" || f.To != "" {
		add("Periode order", strings.TrimSpace(f.From+" s.d. "+f.To))
	}
	return out
}

func (s *Service) writeSummary(ctx context.Context, wb *tableexport.Workbook, branchID int64, f ListFilter, sum *exportSummary, opt ExportOptions) error {
	rows := [][]any{
		{"DAFTAR ORDER JUAL"},
		{"Dibuat pada", timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02 15:04:05") + " WIB"},
	}
	if s.branches != nil {
		if br, err := s.branches.GetByID(ctx, branchID); err == nil && br != nil {
			rows = append(rows, []any{"Cabang", strings.TrimSpace(br.Code + " - " + br.Name)})
		}
	}
	rows = append(rows, filterLines(f)...)
	rows = append(rows, []any{},
		[]any{"Ringkasan", "Nilai", "Keterangan"},
		[]any{"Jumlah Order", sum.orders, "Order sesuai filter (semua status)"},
		[]any{"Order Draf", sum.byStatus[contracts.StatusDraft], ""},
		[]any{"Order Terkonfirmasi", sum.byStatus[contracts.StatusConfirmed], ""},
		[]any{"Order Selesai", sum.byStatus[contracts.StatusCompleted], ""},
		[]any{"Order Dibatalkan", sum.byStatus[contracts.StatusCancelled], "Tidak masuk jumlah nilai di bawah"},
		[]any{"Total Nilai Order", sum.original, "Total order bukan batal, sebelum retur"},
		[]any{"Penyesuaian Retur", sum.adjustment, "Retur terkonfirmasi (mengurangi) dan tukar lebih mahal (menambah)"},
		[]any{"Total Bersih", sum.net, "Total order setelah retur — sama dengan pembukuan"},
		[]any{"Total DPP", sum.dpp, "Total order dikurangi pajak"},
		[]any{"Total Pajak", sum.tax, "PPN pada order sesuai filter"},
		[]any{"Total Dibayar", sum.paid, "Uang yang benar-benar diterima, setelah refund"},
		[]any{"Total Sisa (Piutang)", sum.outstanding, "Sisa tagihan order, tidak pernah negatif"},
		[]any{"Total Harus Dikembalikan", sum.refundDue, "Kelebihan bayar akibat retur yang belum dikembalikan ke pelanggan"},
		[]any{"Order Belum Bayar", sum.unpaid, "Order terkonfirmasi/selesai"},
		[]any{"Order Bayar Sebagian", sum.partial, ""},
		[]any{"Order Lunas", sum.paidFull, ""},
		[]any{"Order Jatuh Tempo", sum.overdue, "Termin net lewat jatuh tempo dan masih ada sisa"},
		[]any{"Penjualan Selesai (Bruto)", sum.completedGross, "Order berstatus Selesai"},
		[]any{"Penyesuaian Retur (Selesai)", sum.completedAdj, ""},
		[]any{"Penjualan Bersih", sum.completedNet, "Penjualan selesai setelah retur"},
	)
	if opt.IncludeMargin {
		switch {
		case s.margin == nil:
		case f.From == "" || f.To == "":
			rows = append(rows, []any{"Margin Kotor (estimasi)", "-", "Pilih rentang tanggal order (dari & sampai) untuk menampilkan margin"})
		default:
			gross, incomplete, err := s.margin.GrossMargin(ctx, branchID, f.From, f.To)
			note := "Estimasi dari pembukuan periode filter (bukan per order)"
			if err != nil {
				rows = append(rows, []any{"Margin Kotor (estimasi)", "-", "Gagal dihitung"})
			} else {
				if incomplete {
					note += "; sebagian biaya berupa perkiraan"
				}
				rows = append(rows, []any{"Margin Kotor (estimasi)", gross, note})
			}
		}
	}
	for _, r := range rows {
		if err := wb.Row("Ringkasan", r...); err != nil {
			return err
		}
	}
	return nil
}

// itemCells renders one "Detail Item" row.
func itemCells(o *contracts.SalesOrder, it *contracts.Item) []any {
	return []any{
		o.Number, shortDate(o.OrderDate), o.PartyName, it.ProductCode, it.ProductName, it.VariantName,
		it.Qty, it.UOM, it.BaseUOM, it.UOMFactor, it.QtyBase, it.UnitPrice,
		it.DiscountPct, it.DiscountNominal, it.TaxBase, it.TaxAmount, it.LineTotal,
		pricingLabel(it.PricingSource),
	}
}

func pricingLabel(src string) string {
	switch src {
	case contracts.PricingMemberRule:
		return "Harga Member"
	case contracts.PricingManual:
		return "Harga Manual"
	case contracts.PricingCatalog:
		return "Harga Katalog"
	}
	return src
}

// buildWorkbook walks the filtered orders once and feeds the three sheets.
func (s *Service) buildWorkbook(ctx context.Context, name string, branchID int64, f ListFilter, opt ExportOptions) (*tableexport.File, error) {
	wb, err := tableexport.NewWorkbook("Ringkasan", "Daftar Order", "Detail Item")
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*tableexport.File, error) {
		wb.Abort()
		return nil, err
	}
	if err := wb.Widths("Ringkasan", 34, 22, 70); err != nil {
		return fail(err)
	}
	if err := wb.Widths("Daftar Order", 20, 12, 28, 9, 14, 13, 12, 14, 12, 14, 12, 14, 14, 14, 14, 14, 14, 16, 16, 18, 18, 18, 18, 14, 20, 12, 30, 30); err != nil {
		return fail(err)
	}
	if err := wb.Widths("Detail Item", 20, 12, 28, 16, 32, 16, 10, 10, 10, 10, 10, 14, 10, 14, 14, 14, 14, 16); err != nil {
		return fail(err)
	}
	hdr := make([]any, len(salesExportHeader))
	for i, h := range salesExportHeader {
		hdr[i] = h
	}
	if err := wb.Row("Daftar Order", hdr...); err != nil {
		return fail(err)
	}
	ihdr := make([]any, len(salesExportItemHeader))
	for i, h := range salesExportItemHeader {
		ihdr[i] = h
	}
	if err := wb.Row("Detail Item", ihdr...); err != nil {
		return fail(err)
	}
	var sum exportSummary
	var after int64
	for {
		b, done, err := s.nextBatch(ctx, branchID, f, &after)
		if err != nil {
			return fail(err)
		}
		if b != nil && len(b.orders) > 0 {
			ids := make([]int64, 0, len(b.orders))
			for _, o := range b.orders {
				ids = append(ids, o.ID)
			}
			items, err := s.repo.ItemsByOrdersFull(ctx, ids)
			if err != nil {
				return fail(apperror.Internal(err))
			}
			for _, o := range b.orders {
				sum.add(o, b.sums[o.ID])
				if err := wb.Row("Daftar Order", b.orderCells(o)...); err != nil {
					return fail(err)
				}
				for _, it := range items[o.ID] {
					if err := wb.Row("Detail Item", itemCells(o, it)...); err != nil {
						return fail(err)
					}
				}
			}
		}
		if done {
			break
		}
	}
	if err := s.writeSummary(ctx, wb, branchID, f, &sum, opt); err != nil {
		return fail(err)
	}
	file, err := wb.Finish(name)
	if err != nil {
		return nil, err
	}
	return file, nil
}

// pdfRow renders the PDF's 12 columns (a subset of the sheet: A4 landscape).
func (b *exportBatch) pdfRow(o *contracts.SalesOrder) []string {
	sum := b.sums[o.ID]
	money := func(n int64) string { return strconv.FormatInt(n, 10) }
	return []string{
		o.Number, shortDate(o.OrderDate), o.PartyName, labelOr(exportTermsLabel, o.PaymentTerms), shortDate(o.DueDate),
		money(o.Subtotal), money(o.DiscountTotal), money(o.TaxTotal), money(sum.NetTotal),
		money(sum.PaidAmount), money(sum.OutstandingAmount), labelOr(exportPayLabel, sum.PaymentStatus),
	}
}

// ExportPager returns a keyset pager over the filtered list (the PDF's rows):
// every call yields the next page of rendered rows, an empty page at the end.
func (s *Service) ExportPager(branchID int64, f ListFilter) tableexport.Pager {
	var after int64
	done := false
	return func(ctx context.Context) ([][]string, error) {
		for !done {
			b, end, err := s.nextBatch(ctx, branchID, f, &after)
			if err != nil {
				return nil, err
			}
			done = end
			if b == nil || len(b.orders) == 0 {
				continue // a filtered-out page: keep walking until rows or the end
			}
			rows := make([][]string, len(b.orders))
			for i, o := range b.orders {
				rows[i] = b.pdfRow(o)
			}
			return rows, nil
		}
		return nil, nil
	}
}

// Export builds the download for the filtered list. format is "xlsx" or "pdf".
// The filter is validated and the party search resolved exactly like List.
func (s *Service) Export(ctx context.Context, branchID int64, f ListFilter, format string) (*tableexport.File, error) {
	return s.ExportWith(ctx, branchID, f, format, ExportOptions{})
}

// ExportWith is Export with per-request options (the margin line).
func (s *Service) ExportWith(ctx context.Context, branchID int64, f ListFilter, format string, opt ExportOptions) (*tableexport.File, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "xlsx" && format != "pdf" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "format", Message: "harus xlsx atau pdf"}})
	}
	if err := s.prepareFilter(ctx, &f); err != nil {
		return nil, err
	}
	stamp := timeutil.NowUTC().In(timeutil.Jakarta).Format("20060102-150405")
	if format == "xlsx" {
		return s.buildWorkbook(ctx, fmt.Sprintf("order-jual-%s.xlsx", stamp), branchID, f, opt)
	}
	return tableexport.PDF(ctx, fmt.Sprintf("order-jual-%s.pdf", stamp), tableexport.PDFSpec{
		Title: "Daftar Order Jual",
		Header: []string{"Nomor", "Tanggal", "Customer", "Termin", "Jatuh Tempo", "Subtotal", "Diskon", "Pajak",
			"Total Bersih", "Dibayar", "Sisa", "Status Bayar"},
		Widths:   []float64{34, 18, 36, 18, 18, 20, 17, 17, 22, 22, 22, 22},
		RightCol: map[int]bool{5: true, 6: true, 7: true, 8: true, 9: true, 10: true},
	}, s.ExportPager(branchID, f))
}
