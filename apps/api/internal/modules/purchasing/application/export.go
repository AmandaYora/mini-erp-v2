package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"mini-erp/internal/modules/purchasing/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/tableexport"
	"mini-erp/internal/shared/timeutil"
)

// Order list export (J1, F4-11, PO-02): .xlsx (stream writer, three sheets) and
// .pdf (capped). Rows walk the SAME filter as the list by keyset (id < last id),
// 500 per page, so every page costs the same however deep the export goes; each
// page resolves supplier names, user names, paid amounts, return states and
// lines in a constant number of grouped reads.
//
// xlsx sheets (legacy parity, twin of the sales export): "Ringkasan" (totals,
// written last from the running sums), "Daftar Order" and "Detail Item".

const exportPageSize = 500

var purchExportHeader = []string{
	"Nomor Order", "Tanggal Order", "Supplier", "Status Order", "Termin", "Jatuh Tempo",
	"Subtotal", "Diskon", "DPP", "Pajak", "Total Order",
	"Dibayar", "Sisa", "Status Bayar", "Status Finansial", "Metode Bayar", "Status Retur",
	"No Faktur Supplier", "Tgl Faktur Supplier", "No Faktur Pajak", "Tgl Faktur Pajak",
	"Dibuat Oleh", "Penanggung Jawab", "Catatan",
}

var purchExportItemHeader = []string{
	"Nomor Order", "Tanggal Order", "Supplier", "Kode Produk", "Nama Produk", "Varian",
	"Qty", "Satuan", "Satuan Dasar", "Faktor", "Qty Dasar", "Harga Satuan",
	"Diskon %", "Diskon Nominal", "Tarif Pajak %", "DPP Baris", "Pajak Baris", "Total Baris",
}

var (
	purchStatusLabel = map[string]string{
		contracts.StatusDraft: "Draf", contracts.StatusConfirmed: "Terkonfirmasi",
		contracts.StatusCompleted: "Selesai", contracts.StatusCancelled: "Dibatalkan",
	}
	purchPayLabel    = map[string]string{"unpaid": "Belum Bayar", "partial": "Bayar Sebagian", "paid": "Lunas"}
	purchTermsLabel  = map[string]string{"cod": "COD / Tunai", "net": "Net / Tempo", "prepaid": "Prabayar"}
	purchReturnLabel = map[string]string{ReturnNone: "-", ReturnPartial: "Diretur Sebagian", ReturnFull: "Diretur Penuh"}
)

func labelOr(m map[string]string, k string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return k
}

func shortDate(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 10 {
		return v[:10]
	}
	return v
}

// financialStatus mirrors the sales sheet: settled / overdue / partial / open.
func financialStatus(o *contracts.PurchaseOrder, sum PaymentSummary) string {
	switch {
	case o.Status == contracts.StatusCancelled || o.Status == contracts.StatusDraft:
		return "-"
	case sum.OutstandingAmount == 0:
		return "Lunas"
	case sum.IsOverdue:
		return "Jatuh Tempo"
	case sum.PaidAmount > 0:
		return "Dibayar Sebagian"
	}
	return "Terbuka"
}

// exportBatch is one keyset page, fully resolved.
type exportBatch struct {
	orders  []*contracts.PurchaseOrder
	sums    map[int64]PaymentSummary
	returns map[int64]string
	methods map[int64]string
}

// nextBatch reads the next keyset page (after = last id of the previous one) and
// resolves it; done reports the end of the walk.
func (s *Service) nextBatch(ctx context.Context, branchID int64, f ListFilter, after *int64) (*exportBatch, bool, error) {
	orders, err := s.repo.ListAfter(ctx, branchID, f, *after, exportPageSize)
	if err != nil {
		return nil, false, apperror.Internal(err)
	}
	if len(orders) == 0 {
		return nil, true, nil
	}
	*after = orders[len(orders)-1].ID
	done := len(orders) < exportPageSize
	if err := s.decorateParties(ctx, orders); err != nil {
		return nil, false, err
	}
	s.decorateUsers(ctx, orders)
	sums, err := s.PaymentSummaries(ctx, orders)
	if err != nil {
		return nil, false, err
	}
	returns, err := s.ReturnSummaries(ctx, orders)
	if err != nil {
		return nil, false, err
	}
	b := &exportBatch{orders: orders, sums: sums, returns: returns, methods: map[int64]string{}}
	if s.dep.payments != nil {
		ids := make([]int64, 0, len(orders))
		for _, o := range orders {
			ids = append(ids, o.ID)
		}
		// Best-effort: a missing label never fails the export.
		if info, ierr := s.dep.payments.PaymentInfoByOrders(ctx, "purchase", ids); ierr == nil {
			for id, in := range info {
				b.methods[id] = strings.Join(in.Methods, ", ")
			}
		}
	}
	return b, done, nil
}

func (b *exportBatch) orderCells(o *contracts.PurchaseOrder) []any {
	sum := b.sums[o.ID]
	return []any{
		o.Number, shortDate(o.OrderDate), o.PartyName, labelOr(purchStatusLabel, o.Status),
		labelOr(purchTermsLabel, o.PaymentTerms), shortDate(sum.EffectiveDueDate),
		o.Subtotal, o.DiscountTotal, o.GrandTotal - o.TaxTotal, o.TaxTotal, o.GrandTotal,
		sum.PaidAmount, sum.OutstandingAmount, labelOr(purchPayLabel, sum.PaymentStatus),
		financialStatus(o, sum), b.methods[o.ID], labelOr(purchReturnLabel, b.returns[o.ID]),
		o.SupplierInvoiceNumber, shortDate(o.SupplierInvoiceDate), o.TaxInvoiceNumber, shortDate(o.TaxInvoiceDate),
		o.CreatedByName, o.AssignedUserName, o.Notes,
	}
}

func itemCells(o *contracts.PurchaseOrder, it *contracts.Item) []any {
	return []any{
		o.Number, shortDate(o.OrderDate), o.PartyName, it.ProductCode, it.ProductName, it.VariantName,
		it.Qty, it.UOM, it.BaseUOM, it.UOMFactor, it.QtyBase, it.UnitPrice,
		it.DiscountPct, it.DiscountNominal, it.TaxRate, it.TaxBase, it.TaxAmount, it.LineTotal,
	}
}

// exportSummary accumulates the "Ringkasan" figures while the pages stream by.
type exportSummary struct {
	orders                             int64
	byStatus                           map[string]int64
	total, dpp, tax                    int64
	paid, outstanding                  int64
	unpaid, partial, paidFull, overdue int64
}

func (e *exportSummary) add(o *contracts.PurchaseOrder, sum PaymentSummary) {
	e.orders++
	if e.byStatus == nil {
		e.byStatus = map[string]int64{}
	}
	e.byStatus[o.Status]++
	if o.Status == contracts.StatusCancelled {
		return
	}
	e.total += o.GrandTotal
	e.dpp += o.GrandTotal - o.TaxTotal
	e.tax += o.TaxTotal
	if o.Status == contracts.StatusDraft {
		return
	}
	e.paid += sum.PaidAmount
	e.outstanding += sum.OutstandingAmount
	switch sum.PaymentStatus {
	case "unpaid":
		e.unpaid++
	case "partial":
		e.partial++
	case "paid":
		e.paidFull++
	}
	if sum.IsOverdue {
		e.overdue++
	}
}

func filterLines(f ListFilter) [][]any {
	var out [][]any
	add := func(k, v string) {
		if strings.TrimSpace(v) != "" {
			out = append(out, []any{k, v})
		}
	}
	add("Pencarian", f.Search)
	add("Status order", strings.Join(f.Statuses, ", "))
	add("Termin", labelOr(purchTermsLabel, f.PaymentTerms))
	if f.From != "" || f.To != "" {
		add("Periode order", strings.TrimSpace(f.From+" s.d. "+f.To))
	}
	return out
}

func (s *Service) writeSummary(ctx context.Context, wb *tableexport.Workbook, branchID int64, f ListFilter, sum *exportSummary) error {
	rows := [][]any{
		{"DAFTAR ORDER BELI"},
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
		[]any{"Total Nilai Order", sum.total, "Total order bukan batal"},
		[]any{"Total DPP", sum.dpp, "Total order dikurangi pajak"},
		[]any{"Total Pajak", sum.tax, "PPN pada order sesuai filter"},
		[]any{"Total Dibayar", sum.paid, "Pembayaran aktif ke supplier (order terkonfirmasi/selesai)"},
		[]any{"Total Sisa (Hutang)", sum.outstanding, "Sisa tagihan supplier, tidak pernah negatif"},
		[]any{"Order Belum Bayar", sum.unpaid, "Order terkonfirmasi/selesai"},
		[]any{"Order Bayar Sebagian", sum.partial, ""},
		[]any{"Order Lunas", sum.paidFull, ""},
		[]any{"Order Jatuh Tempo", sum.overdue, "Termin net lewat jatuh tempo dan masih ada sisa"},
	)
	for _, r := range rows {
		if err := wb.Row("Ringkasan", r...); err != nil {
			return err
		}
	}
	return nil
}

func headerRow(h []string) []any {
	out := make([]any, len(h))
	for i, v := range h {
		out[i] = v
	}
	return out
}

// buildWorkbook walks the filtered orders once and feeds the three sheets.
func (s *Service) buildWorkbook(ctx context.Context, name string, branchID int64, f ListFilter) (*tableexport.File, error) {
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
	if err := wb.Widths("Daftar Order", 20, 12, 28, 14, 14, 12, 14, 12, 14, 12, 14, 14, 14, 16, 16, 16, 16, 20, 14, 20, 14, 18, 18, 30); err != nil {
		return fail(err)
	}
	if err := wb.Widths("Detail Item", 20, 12, 28, 16, 32, 16, 10, 10, 10, 10, 10, 14, 10, 14, 12, 14, 14, 14); err != nil {
		return fail(err)
	}
	if err := wb.Row("Daftar Order", headerRow(purchExportHeader)...); err != nil {
		return fail(err)
	}
	if err := wb.Row("Detail Item", headerRow(purchExportItemHeader)...); err != nil {
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
			items, err := s.repo.ItemsByOrders(ctx, ids)
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
	if err := s.writeSummary(ctx, wb, branchID, f, &sum); err != nil {
		return fail(err)
	}
	return wb.Finish(name)
}

// pdfRow renders the PDF's columns (a subset of the sheet: A4 landscape).
func (b *exportBatch) pdfRow(o *contracts.PurchaseOrder) []string {
	sum := b.sums[o.ID]
	money := func(n int64) string { return strconv.FormatInt(n, 10) }
	return []string{
		o.Number, shortDate(o.OrderDate), o.PartyName, labelOr(purchTermsLabel, o.PaymentTerms), shortDate(sum.EffectiveDueDate),
		money(o.Subtotal), money(o.TaxTotal), money(o.GrandTotal),
		money(sum.PaidAmount), money(sum.OutstandingAmount), labelOr(purchStatusLabel, o.Status),
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
				continue
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

var purchPDFHeader = []string{
	"Nomor", "Tanggal", "Supplier", "Termin", "Jatuh Tempo", "Subtotal", "Pajak", "Total", "Dibayar", "Sisa", "Status",
}

// Export builds the download for the filtered list. format is "xlsx" or "pdf".
// The filter is validated and the party search resolved exactly like List.
func (s *Service) Export(ctx context.Context, branchID int64, f ListFilter, format string) (*tableexport.File, error) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format != "xlsx" && format != "pdf" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "format", Message: "harus xlsx atau pdf"}})
	}
	if err := s.prepareFilter(ctx, &f); err != nil {
		return nil, err
	}
	stamp := timeutil.NowUTC().In(timeutil.Jakarta).Format("20060102-150405")
	if format == "xlsx" {
		return s.buildWorkbook(ctx, fmt.Sprintf("order-beli-%s.xlsx", stamp), branchID, f)
	}
	return tableexport.PDF(ctx, fmt.Sprintf("order-beli-%s.pdf", stamp), tableexport.PDFSpec{
		Title:    "Daftar Order Beli",
		Header:   purchPDFHeader,
		Widths:   []float64{34, 20, 42, 18, 22, 24, 22, 26, 24, 24, 24},
		RightCol: map[int]bool{5: true, 6: true, 7: true, 8: true, 9: true},
	}, s.ExportPager(branchID, f))
}
