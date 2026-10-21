package application

import (
	"context"
	"sort"
	"strconv"

	"mini-erp/internal/modules/finance/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
)

// VAT reporting: monthly summary (with DPP and the taxable documents that lack
// an invoice number), the per-entry detail, and the row set the tax package
// prints. Every list is read from the books (journal entries) and decorated in
// a constant number of batch reads.

func itoa(v int64) string { return strconv.FormatInt(v, 10) }

// TaxException is one taxable document whose tax invoice number is missing
// (FN-06): a sales shipment without a faktur pajak, or a goods receipt without
// the supplier's faktur. One row per parent order.
type TaxException struct {
	OrderID     int64  `json:"orderId"`
	OrderNumber string `json:"orderNumber"`
	Kind        string `json:"kind"` // sales | purchase
	Date        string `json:"date"`
	PartyName   string `json:"partyName"`
	Dpp         int64  `json:"dpp"`
	Ppn         int64  `json:"ppn"`
}

// ExceptionsMeta paginates TaxSummary.Exceptions (limit 0 returns them all).
type ExceptionsMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

// TaxSummary is one month's VAT position. SalesDpp/PurchaseDpp are the tax
// bases (dasar pengenaan pajak) of the taxable documents behind PpnOut/PpnIn,
// net of returns.
type TaxSummary struct {
	Year           int            `json:"year"`
	Month          int            `json:"month"`
	From           string         `json:"from,omitempty"`
	To             string         `json:"to,omitempty"`
	PpnOut         int64          `json:"ppnOut"`
	PpnIn          int64          `json:"ppnIn"`
	Payable        int64          `json:"payable"`
	SalesDpp       int64          `json:"salesDpp"`
	PurchaseDpp    int64          `json:"purchaseDpp"`
	Exceptions     []TaxException `json:"exceptions"`
	ExceptionsMeta ExceptionsMeta `json:"exceptionsMeta"`
}

// TaxSummary nets PPN Keluaran credits against PPN Masukan debits and lists
// every taxable document without an invoice number.
func (s *Service) TaxSummary(ctx context.Context, branchID int64, year, month int) (*TaxSummary, error) {
	return s.taxSummaryPaged(ctx, branchID, year, month, fullBooks, 1, 0)
}

// TaxSummaryPage is TaxSummary with the exceptions list paginated (limit <= 0
// returns all of them).
func (s *Service) TaxSummaryPage(ctx context.Context, branchID int64, year, month, page, limit int) (*TaxSummary, error) {
	return s.taxSummaryPaged(ctx, branchID, year, month, fullBooks, page, limit)
}

func (s *Service) taxSummaryScoped(ctx context.Context, branchID int64, year, month int, sc bookScope) (*TaxSummary, error) {
	return s.taxSummaryPaged(ctx, branchID, year, month, sc, 1, 0)
}

func (s *Service) taxSummaryPaged(ctx context.Context, branchID int64, year, month int, sc bookScope, page, limit int) (*TaxSummary, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	sum, err := s.taxSummaryWindow(ctx, branchID, monthStart(year, month), monthEnd(year, month), sc, page, limit)
	if err != nil {
		return nil, err
	}
	sum.Year, sum.Month = year, month
	return sum, nil
}

// taxSummaryWindow is the VAT summary of any [start, end] window (timestamps).
func (s *Service) taxSummaryWindow(ctx context.Context, branchID int64, start, end string, sc bookScope, page, limit int) (*TaxSummary, error) {
	out, err := s.repo.MappingAccount(ctx, "ppn_out")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	in, err := s.repo.MappingAccount(ctx, "ppn_in")
	if err != nil {
		return nil, apperror.Internal(err)
	}
	footings, err := s.footings(ctx, branchID, start, end, sc)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	sum := &TaxSummary{}
	if out != nil {
		if f, ok := footings[out.ID]; ok {
			sum.PpnOut = f[1] - f[0]
		}
	}
	if in != nil {
		if f, ok := footings[in.ID]; ok {
			sum.PpnIn = f[0] - f[1]
		}
	}
	sum.Payable = sum.PpnOut - sum.PpnIn

	rows, err := s.taxRows(ctx, branchID, start, end, taxSourcesAll)
	if err != nil {
		return nil, err
	}
	excluded := map[int64]bool{}
	for _, id := range sc.excluded {
		excluded[id] = true
	}
	sum.Exceptions = []TaxException{}
	seen := map[string]bool{}
	for _, r := range rows {
		if excluded[r.EntryID] {
			continue
		}
		switch r.Kind {
		case "sales", "sales_return":
			sum.SalesDpp += r.Dpp
		case "purchase", "purchase_return":
			sum.PurchaseDpp += r.Dpp
		}
		if !r.MissingInvoice {
			continue
		}
		key := r.Kind + ":" + itoa(r.OrderID)
		if r.OrderID == 0 {
			key = r.Kind + ":e" + itoa(r.EntryID)
		}
		if seen[key] {
			// Partial shipments of one order: the order is listed once, its
			// tax base and tax accumulate.
			for i := range sum.Exceptions {
				e := &sum.Exceptions[i]
				if e.Kind == r.Kind && e.OrderID == r.OrderID && r.OrderID != 0 {
					e.Dpp += r.Dpp
					e.Ppn += r.Ppn + r.PpnIn
				}
			}
			continue
		}
		seen[key] = true
		sum.Exceptions = append(sum.Exceptions, TaxException{OrderID: r.OrderID, OrderNumber: r.OrderNumber, Kind: r.Kind,
			Date: r.Date, PartyName: r.PartyName, Dpp: r.Dpp, Ppn: r.Ppn + r.PpnIn})
	}
	sort.SliceStable(sum.Exceptions, func(i, j int) bool {
		if sum.Exceptions[i].Date != sum.Exceptions[j].Date {
			return sum.Exceptions[i].Date > sum.Exceptions[j].Date
		}
		return sum.Exceptions[i].OrderID > sum.Exceptions[j].OrderID
	})
	total := int64(len(sum.Exceptions))
	meta := ExceptionsMeta{Page: 1, Limit: int(total), Total: total, TotalPages: 1}
	if limit > 0 {
		if limit > 500 {
			limit = 500
		}
		if page < 1 {
			page = 1
		}
		from := (page - 1) * limit
		if from > len(sum.Exceptions) {
			from = len(sum.Exceptions)
		}
		to := from + limit
		if to > len(sum.Exceptions) {
			to = len(sum.Exceptions)
		}
		sum.Exceptions = sum.Exceptions[from:to]
		meta = ExceptionsMeta{Page: page, Limit: limit, Total: total, TotalPages: int((total + int64(limit) - 1) / int64(limit))}
	}
	sum.ExceptionsMeta = meta
	return sum, nil
}

// TaxDetailRow is one journal entry of the VAT working paper.
type TaxDetailRow struct {
	// EntryID makes each row traceable back to the journal entry; OrderID is the
	// parent order behind it (sales order for sales/sales_return, purchase order
	// for purchase). The gross-turnover working paper uses EntryID to show only
	// the rows that sit under the ceiling.
	EntryID int64 `json:"entryId"`
	// BranchID/BranchName name the branch of the journal in an "all branches"
	// report (empty for a single-branch one).
	BranchID   int64  `json:"branchId,omitempty"`
	BranchName string `json:"branchName,omitempty"`
	OrderID    int64  `json:"orderId"`
	Date       string `json:"date"`
	Number     string `json:"number"`
	Memo       string `json:"memo"`
	Revenue    int64  `json:"revenue"`
	Ppn        int64  `json:"ppn"`
	// Kind is sales | sales_return (negative amounts) | purchase |
	// purchase_return (negative amounts); purchases carry PpnIn, not Ppn.
	Kind             string `json:"kind"`
	PpnIn            int64  `json:"ppnIn"`
	TaxInvoiceNumber string `json:"taxInvoiceNumber"`
	TaxInvoiceDate   string `json:"taxInvoiceDate"`
	// Dpp is the tax base of a TAXABLE row (0 when the row carries no tax).
	Dpp         int64  `json:"dpp"`
	OrderNumber string `json:"orderNumber"`
	PartyName   string `json:"partyName"`
	// MissingInvoice marks a taxable sale/purchase whose invoice number is empty
	// (FN-06). Returns never carry one.
	MissingInvoice bool `json:"missingInvoice"`
}

var (
	taxSourcesSales = []string{SourceDelivery}
	taxSourcesAll   = []string{SourceDelivery, SourceSalesReturn, SourceGoodsReceipt, SourcePurchaseReturn}
)

// taxDetailPageSize is how many entries one aggregation page reads.
const taxDetailPageSize = 2000

// TaxDetail lists delivery-sourced entries in a month (SPT sales working
// paper). Invoice numbers come from the source SO (C3).
func (s *Service) TaxDetail(ctx context.Context, branchID int64, year, month int) ([]TaxDetailRow, error) {
	if !validYearMonth(year, month) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	return s.taxRows(ctx, branchID, monthStart(year, month), monthEnd(year, month), taxSourcesSales)
}

// TaxDetailRange lists every VAT row (sales, sales returns, purchases with PPN
// Masukan, purchase returns) of an inclusive YYYY-MM-DD range: the row set of
// the tax package, identical to what TaxDetailReport pages through.
func (s *Service) TaxDetailRange(ctx context.Context, branchID int64, from, to string) ([]TaxDetailRow, error) {
	start, end, err := validateRange(from, to)
	if err != nil {
		return nil, err
	}
	return s.taxRows(ctx, branchID, start, end, taxSourcesAll)
}

// taxRows walks every page; each page is aggregated in SQL, so there is no row
// cap.
func (s *Service) taxRows(ctx context.Context, branchID int64, start, end string, sources []string) ([]TaxDetailRow, error) {
	out := []TaxDetailRow{}
	for offset := 0; ; offset += taxDetailPageSize {
		rows, total, err := s.taxDetailPage(ctx, branchID, start, end, sources, taxDetailPageSize, offset)
		if err != nil {
			return nil, err
		}
		out = append(out, rows...)
		if int64(offset+taxDetailPageSize) >= total {
			break
		}
	}
	return out, nil
}

// TaxDetailReport is the PPN detail report: sales (positive), sales returns
// (negative: they reverse revenue and output VAT), goods receipts (input VAT)
// and purchase returns (negative input VAT), one row per journal entry,
// paginated with an exact total.
func (s *Service) TaxDetailReport(ctx context.Context, branchID int64, year, month, page, limit int) ([]TaxDetailRow, int64, error) {
	if !validYearMonth(year, month) {
		return nil, 0, apperror.Validation("", []apperror.FieldError{{Field: "period", Message: "periode tidak valid"}})
	}
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 2000 {
		limit = 500
	}
	return s.taxDetailPage(ctx, branchID, monthStart(year, month), monthEnd(year, month), taxSourcesAll, limit, (page-1)*limit)
}

func (s *Service) taxDetailPage(ctx context.Context, branchID int64, start, end string, sources []string, limit, offset int) ([]TaxDetailRow, int64, error) {
	ppnOut, err := s.repo.MappingAccount(ctx, "ppn_out")
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	ppnIn, err := s.repo.MappingAccount(ctx, "ppn_in")
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	payable, err := s.repo.MappingAccount(ctx, "payable")
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	idOf := func(a *contracts.Account) int64 {
		if a == nil {
			return 0
		}
		return a.ID
	}
	entries, total, err := s.repo.TaxEntries(ctx, branchID, start, end,
		idOf(ppnOut), idOf(ppnIn), idOf(payable), sources, limit, offset)
	if err != nil {
		return nil, 0, apperror.Internal(err)
	}
	// Resolve every source document in a constant number of reads (F4-7):
	// notes/returns -> sales order ids, receipts -> purchase order ids, then
	// ONE summaries read per order kind (tax invoice pair + branch + party),
	// instead of a GetByID (2 queries) per unique document.
	var noteIDs, returnIDs, receiptIDs []int64
	for _, e := range entries {
		switch e.SourceType {
		case SourceDelivery:
			noteIDs = append(noteIDs, e.SourceID)
		case SourceSalesReturn:
			returnIDs = append(returnIDs, e.SourceID)
		case SourceGoodsReceipt:
			receiptIDs = append(receiptIDs, e.SourceID)
		}
	}
	noteOrders := map[int64]int64{}
	if len(noteIDs) > 0 {
		if noteOrders, err = s.delivery.OrderIDsByNotes(ctx, noteIDs); err != nil {
			return nil, 0, apperror.Internal(err)
		}
	}
	returnOrders := map[int64]int64{}
	if len(returnIDs) > 0 {
		if returnOrders, err = s.salesReturns.OrderIDsByReturns(ctx, returnIDs); err != nil {
			return nil, 0, apperror.Internal(err)
		}
	}
	receiptOrders := map[int64]int64{}
	if len(receiptIDs) > 0 {
		if receiptOrders, err = s.goodsreceipt.PurchaseOrderIDsByReceipts(ctx, receiptIDs); err != nil {
			return nil, 0, apperror.Internal(err)
		}
	}
	salesIDs := make([]int64, 0, len(noteOrders)+len(returnOrders))
	for _, id := range noteOrders {
		salesIDs = append(salesIDs, id)
	}
	for _, id := range returnOrders {
		salesIDs = append(salesIDs, id)
	}
	purchaseIDs := make([]int64, 0, len(receiptOrders))
	for _, id := range receiptOrders {
		purchaseIDs = append(purchaseIDs, id)
	}
	salesDocs := map[int64]*salescontracts.OrderSummary{}
	if len(salesIDs) > 0 {
		if salesDocs, err = s.sales.SummariesByIDs(ctx, salesIDs); err != nil {
			return nil, 0, apperror.Internal(err)
		}
	}
	purchaseDocs := map[int64]*purchasingcontracts.OrderSummary{}
	if len(purchaseIDs) > 0 {
		if purchaseDocs, err = s.purchasing.SummariesByIDs(ctx, purchaseIDs); err != nil {
			return nil, 0, apperror.Internal(err)
		}
	}
	// Branch scope of the read: the session branch, or every branch of an
	// "all branches" report. A document of another branch never decorates a row.
	inScope := map[int64]bool{branchID: true}
	if ids, ok := infrastructure.BranchSetFrom(ctx); ok {
		inScope = map[int64]bool{}
		for _, id := range ids {
			inScope[id] = true
		}
	}
	type doc struct {
		invoiceNo, invoiceDate, number string
		orderID, partyID               int64
	}
	salesDoc := func(soID int64) doc {
		if so := salesDocs[soID]; so != nil && inScope[so.BranchID] {
			return doc{so.TaxInvoiceNumber, so.TaxInvoiceDate, so.Number, soID, so.PartyID}
		}
		return doc{orderID: soID}
	}
	docOf := func(e infrastructure.TaxEntryRow) doc {
		switch e.SourceType {
		case SourceDelivery:
			return salesDoc(noteOrders[e.SourceID])
		case SourceSalesReturn:
			// A return carries the order's party and number, never its invoice.
			d := salesDoc(returnOrders[e.SourceID])
			d.invoiceNo, d.invoiceDate = "", ""
			return d
		case SourceGoodsReceipt:
			poID := receiptOrders[e.SourceID]
			if po := purchaseDocs[poID]; po != nil && inScope[po.BranchID] {
				return doc{po.SupplierInvoiceNumber, po.SupplierInvoiceDate, po.Number, poID, po.PartyID}
			}
			return doc{orderID: poID}
		}
		return doc{}
	}
	// OrderSummary carries only the party id (never the name), so the label comes from
	// one batch read through the party contract; without it the "Pelanggan"/"Pemasok"
	// columns of the working paper and the tax-package sheets stay empty (N-51).
	partyIDs := make([]int64, 0, len(entries))
	seenParty := map[int64]bool{}
	for _, e := range entries {
		if pid := docOf(e).partyID; pid != 0 && !seenParty[pid] {
			seenParty[pid] = true
			partyIDs = append(partyIDs, pid)
		}
	}
	partyNames := s.partyLabels(ctx, partyIDs)
	var branchNames map[int64]string
	if len(inScope) > 1 {
		ids := make([]int64, 0, len(inScope))
		for id := range inScope {
			ids = append(ids, id)
		}
		branchNames = s.branchNames(ctx, ids)
	}
	out := make([]TaxDetailRow, 0, len(entries))
	for _, e := range entries {
		d := docOf(e)
		kind := kindOfSource(e.SourceType)
		row := TaxDetailRow{EntryID: e.EntryID, OrderID: d.orderID, Date: e.Date, Number: e.Number, Memo: e.Memo,
			Kind: kind, Revenue: e.Revenue, Ppn: e.PpnOut, PpnIn: e.PpnIn,
			TaxInvoiceNumber: d.invoiceNo, TaxInvoiceDate: d.invoiceDate, OrderNumber: d.number, PartyName: partyNames[d.partyID].Name}
		switch kind {
		case "sales", "sales_return":
			if e.PpnOut != 0 {
				row.Dpp = e.Revenue
			}
		case "purchase", "purchase_return":
			if e.PpnIn != 0 {
				row.Dpp = e.Payable - e.PpnIn
			}
		}
		if branchNames != nil {
			row.BranchID, row.BranchName = e.BranchID, branchNames[e.BranchID]
		}
		row.MissingInvoice = d.invoiceNo == "" &&
			((kind == "sales" && e.PpnOut != 0) || (kind == "purchase" && e.PpnIn != 0))
		out = append(out, row)
	}
	return out, total, nil
}

func kindOfSource(t string) string {
	switch t {
	case SourceSalesReturn:
		return "sales_return"
	case SourceGoodsReceipt:
		return "purchase"
	case SourcePurchaseReturn:
		return "purchase_return"
	default:
		return "sales"
	}
}
