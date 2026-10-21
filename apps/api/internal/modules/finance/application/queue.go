package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	"mini-erp/internal/modules/finance/infrastructure"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// errPostedIndex surfaces a journal-index read failure distinctly so a
// half-built queue is never presented as "nothing to post".
var errPostedIndex = errors.New("gagal membaca indeks jurnal")

// Derived posting queue without a posting table (G-revisi): "unposted" is
// confirmed-documents minus the journal index, computed on read. It cannot
// drift — there is no second store to sync — and posting itself stays
// idempotent through Post().
//
// Query count for N candidates: 5 candidate reads (one bounded list per
// document kind) + 5 posted-set reads (one grouped query per source type).
// Never one query per document (P3). Expenses are absent on purpose: they
// auto-post on create, so they can never be unposted.

// QueuePage is how many source documents the queue scan reads per step. The
// scan is exact and unbounded — it walks every confirmed document after the
// cutover boundary in id order — so the page only sets the query size.
const QueuePage = 1000

// MaxBatch caps one batch-post call so a fat-fingered "post all" cannot
// lock the books for minutes.
const MaxBatch = 50

// QueueItem is one unposted document awaiting its journal.
type QueueItem struct {
	DocType string `json:"docType"`
	DocID   int64  `json:"docId"`
	Number  string `json:"number"`
	Date    string `json:"date"`
	// PeriodClosed: the document falls in an accounting month that is already
	// closed. It stays visible (a warning) but is excluded from batch posting
	// and from the gate of any other month.
	PeriodClosed bool `json:"periodClosed"`
	// Ignored (FN-01): an accountant took this stuck document out of the queue
	// on purpose, with a reason. It stays listed (restorable) but never blocks a
	// close and is skipped by batch posting.
	Ignored      bool   `json:"ignored"`
	IgnoreReason string `json:"ignoreReason,omitempty"`
	IgnoredAt    string `json:"ignoredAt,omitempty"`
	IgnoredByID  int64  `json:"ignoredBy,omitempty"`
	// Amount / party / summary (FN-10): what the document is worth and whom it
	// concerns, so the accountant can recognise it without opening it.
	// AmountKind says what Amount is: "document" (the document's own value) or
	// "order" (the parent order's total, for shipments and receipts, whose own
	// value is only known when the journal is built).
	Amount     int64  `json:"amount"`
	AmountKind string `json:"amountKind,omitempty"`
	PartyID    int64  `json:"partyId,omitempty"`
	PartyName  string `json:"partyName,omitempty"`
	Summary    string `json:"summary,omitempty"`
	// AwaitingOrder (ADR-0022): a direct delivery note whose order is not built
	// yet. It cannot be journalled, cannot be ignored (the goods are gone and
	// must be billed) and blocks the month's close until the order exists.
	AwaitingOrder bool `json:"awaitingOrder,omitempty"`

	// orderKind/orderID resolve party and summary in one batch read.
	orderKind string
	orderID   int64
}

const (
	orderKindSales    = "sales"
	orderKindPurchase = "purchase"
)

// queueDoc is one candidate source document read from its owner module.
type queueDoc struct {
	id           int64
	number, date string
	amount       int64
	amountKind   string
	partyID      int64
	partyName    string
	summary      string
	orderKind    string
	orderID      int64
	awaiting     bool
}

// queuePage is one page of candidates plus the raw cursor information: last is
// the highest id READ (even when some rows were filtered out) and read how many
// rows the owner returned, so the scan ends on the owner's page size, never on
// the filtered size.
type queuePage struct {
	docs []queueDoc
	last int64
	read int
}

// queueSource is one document kind the queue scans: its journal source type
// and how to read the next page of candidates after a cursor id.
type queueSource struct {
	docType string
	next    func(ctx context.Context, afterID int64, limit int) (queuePage, error)
}

func (s *Service) queueSources(branchID int64) []queueSource {
	day := func(v string) string {
		if len(v) >= 10 {
			return v[:10]
		}
		return v
	}
	return []queueSource{
		{SourceDelivery, func(ctx context.Context, after int64, limit int) (queuePage, error) {
			notes, err := s.delivery.ListConfirmedAfter(ctx, branchID, after, limit)
			pg := queuePage{read: len(notes)}
			for _, n := range notes {
				if n.Origin == deliverycontracts.OriginDirect && n.SalesOrderID == 0 {
					// No order yet: the customer is on the note, the value is
					// unknown until the order prices it.
					pg.docs = append(pg.docs, queueDoc{id: n.ID, number: n.Number, date: day(n.DeliveryDate),
						amountKind: "document", partyID: n.PartyID, partyName: n.PartyName,
						summary: "SJ Langsung · menunggu order", awaiting: true})
					pg.last = n.ID
					continue
				}
				pg.docs = append(pg.docs, queueDoc{id: n.ID, number: n.Number, date: day(n.DeliveryDate),
					amountKind: "order", summary: "Pesanan", orderKind: orderKindSales, orderID: n.SalesOrderID})
				pg.last = n.ID
			}
			return pg, err
		}},
		{SourceGoodsReceipt, func(ctx context.Context, after int64, limit int) (queuePage, error) {
			recs, err := s.goodsreceipt.ListPostedAfter(ctx, branchID, after, limit)
			pg := queuePage{read: len(recs)}
			for _, r := range recs {
				pg.docs = append(pg.docs, queueDoc{id: r.ID, number: "Penerimaan " + r.Number, date: day(r.ReceivedAt),
					amountKind: "order", summary: "Pesanan", orderKind: orderKindPurchase, orderID: r.PurchaseOrderID})
				pg.last = r.ID
			}
			return pg, err
		}},
		{SourcePayment, func(ctx context.Context, after int64, limit int) (queuePage, error) {
			ps, err := s.payment.ListActiveAfter(ctx, branchID, after, limit)
			pg := queuePage{read: len(ps)}
			for _, p := range ps {
				pg.last = p.ID
				if p.Method == paymentcontracts.MethodOffset {
					continue // offsets are sub-ledger only; never journalled (F2b-1)
				}
				summary := "Pembayaran masuk"
				if p.Direction == paymentcontracts.DirectionOut {
					summary = "Pembayaran keluar"
				}
				pg.docs = append(pg.docs, queueDoc{id: p.ID, number: p.Number, date: day(p.PaidAt), amount: p.Amount,
					amountKind: "document", partyID: p.PartyID, partyName: p.PartyName, summary: summary + " (" + p.Method + ")"})
			}
			return pg, err
		}},
		{SourceSalesReturn, func(ctx context.Context, after int64, limit int) (queuePage, error) {
			rs, err := s.salesReturns.ListConfirmedAfter(ctx, branchID, after, limit)
			pg := queuePage{read: len(rs)}
			for _, r := range rs {
				pg.docs = append(pg.docs, queueDoc{id: r.ID, number: r.Number, date: day(r.Date), amount: r.Total,
					amountKind: "document", summary: "Retur penjualan", orderKind: orderKindSales, orderID: r.OrderID})
				pg.last = r.ID
			}
			return pg, err
		}},
		{SourcePurchaseReturn, func(ctx context.Context, after int64, limit int) (queuePage, error) {
			rs, err := s.purchaseReturns.ListConfirmedAfter(ctx, branchID, after, limit)
			pg := queuePage{read: len(rs)}
			for _, r := range rs {
				pg.docs = append(pg.docs, queueDoc{id: r.ID, number: r.Number, date: day(r.Date), amount: r.Total,
					amountKind: "document", summary: "Retur pembelian", orderKind: orderKindPurchase, orderID: r.OrderID})
				pg.last = r.ID
			}
			return pg, err
		}},
		// Return value kept as a party balance (K-04a): one journal per
		// customer_credit / supplier_credit settlement of a confirmed return.
		{SourceSalesReturnSettlement, func(ctx context.Context, after int64, limit int) (queuePage, error) {
			rs, err := s.salesReturns.ListCreditSettlementsAfter(ctx, branchID, after, limit)
			pg := queuePage{read: len(rs)}
			for _, r := range rs {
				pg.docs = append(pg.docs, queueDoc{id: r.ID, number: "Saldo customer " + r.ReturnNumber, date: day(r.Date), amount: r.Amount,
					amountKind: "document", summary: "Saldo kredit customer", orderKind: orderKindSales, orderID: r.OrderID})
				pg.last = r.ID
			}
			return pg, err
		}},
		{SourcePurchaseReturnSettlement, func(ctx context.Context, after int64, limit int) (queuePage, error) {
			rs, err := s.purchaseReturns.ListCreditSettlementsAfter(ctx, branchID, after, limit)
			pg := queuePage{read: len(rs)}
			for _, r := range rs {
				pg.docs = append(pg.docs, queueDoc{id: r.ID, number: "Saldo supplier " + r.ReturnNumber, date: day(r.Date), amount: r.Amount,
					amountKind: "document", summary: "Saldo kredit supplier", orderKind: orderKindPurchase, orderID: r.OrderID})
				pg.last = r.ID
			}
			return pg, err
		}},
		// A purchase price correction after receipt (ADR-0023): the value
		// difference of the received goods. Corrections of nothing received
		// carry no value and never reach this list.
		{SourcePurchasePriceCorrection, func(ctx context.Context, after int64, limit int) (queuePage, error) {
			cs, err := s.purchasing.ListPriceCorrectionsAfter(ctx, branchID, after, limit)
			pg := queuePage{read: len(cs)}
			for _, c := range cs {
				pg.docs = append(pg.docs, queueDoc{id: c.ID, number: "Koreksi harga " + c.OrderNumber, date: day(c.Date),
					amount: c.ValueDelta + c.TaxDelta, amountKind: "document", summary: "Koreksi harga beli",
					orderKind: orderKindPurchase, orderID: c.OrderID})
				pg.last = c.ID
			}
			return pg, err
		}},
	}
}

// UnpostedSources returns confirmed-but-unjournalled documents, newest first.
//
// The scan is exact (F2b-9): every document after the cutover boundary is
// compared with the journal index, page by page — there is no "newest 200"
// window that could hide an old failed posting. Cancelled documents are
// excluded by the owner modules' own candidate lists. Replacement shipments are
// included (they book their own sale). Documents inside an already-closed
// accounting month are flagged PeriodClosed.
func (s *Service) UnpostedSources(ctx context.Context, branchID int64) ([]QueueItem, error) {
	cutover, err := s.repo.Cutovers(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	closed, err := s.closedMonths(ctx, branchID)
	if err != nil {
		return nil, err
	}
	ignores, err := s.repo.PostingIgnores(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	withIgnore := func(it QueueItem) QueueItem {
		if ig, ok := ignores[infrastructure.IgnoreKey(it.DocType, it.DocID)]; ok {
			it.Ignored, it.IgnoreReason, it.IgnoredAt, it.IgnoredByID = true, ig.Reason, timeutil.FormatWIB(ig.IgnoredAt), ig.IgnoredBy
		}
		return it
	}
	out := []QueueItem{}
	for _, src := range s.queueSources(branchID) {
		after := cutover[src.docType]
		for {
			pg, err := src.next(ctx, after, QueuePage)
			if err != nil {
				return nil, apperror.Internal(err)
			}
			if pg.read == 0 {
				break
			}
			ids := make([]int64, 0, len(pg.docs))
			for _, d := range pg.docs {
				ids = append(ids, d.id)
			}
			posted, err := s.repo.PostedAmong(ctx, branchID, src.docType, ids)
			if err != nil {
				return nil, apperror.Internal(errors.Join(errPostedIndex, err))
			}
			for _, d := range pg.docs {
				if posted[d.id] {
					continue
				}
				out = append(out, withIgnore(QueueItem{DocType: src.docType, DocID: d.id, Number: d.number, Date: d.date,
					PeriodClosed: len(d.date) >= 7 && closed[d.date[:7]],
					Amount:       d.amount, AmountKind: d.amountKind, PartyID: d.partyID, PartyName: d.partyName,
					Summary: d.summary, AwaitingOrder: d.awaiting, orderKind: d.orderKind, orderID: d.orderID}))
			}
			after = pg.last
			if pg.read < QueuePage {
				break
			}
		}
	}

	// Stock adjustments / write-offs come from the finance cost ledger (the
	// anti-join never leaves finance's own tables). Sync first so freshly
	// adjusted stock is visible.
	if err := s.drainCosting(ctx, branchID); err != nil {
		return nil, err
	}
	after := cutover[SourceStockMovement]
	for {
		movements, err := s.repo.UnpostedStockMovements(ctx, branchID, after, QueuePage)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, m := range movements {
			date := timeutil.WIBDay(m.MovedAt) // UTC instant -> WIB day (same as its journal)
			label := "Koreksi stok #" + strconv.FormatInt(m.MovementID, 10)
			switch m.RefType {
			case "opening_correction":
				label = "Koreksi saldo awal stok #" + strconv.FormatInt(m.MovementID, 10)
			case "write_off":
				label = "Hapus stok #" + strconv.FormatInt(m.MovementID, 10)
			case "damaged_arrival":
				label = "Barang rusak masuk #" + strconv.FormatInt(m.MovementID, 10)
			}
			amount := int64(math.Round(math.Abs(m.TotalCost)))
			out = append(out, withIgnore(QueueItem{DocType: SourceStockMovement, DocID: m.MovementID, Number: label, Date: date,
				PeriodClosed: len(date) >= 7 && closed[date[:7]], Amount: amount, AmountKind: "document",
				Summary: "Selisih/hapus stok"}))
			after = m.MovementID
		}
		if len(movements) < QueuePage {
			break
		}
	}

	if err := s.enrichQueue(ctx, branchID, out); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date > out[j].Date
		}
		if out[i].DocType != out[j].DocType {
			return out[i].DocType < out[j].DocType
		}
		return out[i].DocID > out[j].DocID
	})
	return out, nil
}

// enrichQueue fills party and order totals of the unposted items in two batch
// reads, one per order kind, never one per document (FN-10). Items whose owner
// module cannot resolve the order keep what the candidate read already gave.
func (s *Service) enrichQueue(ctx context.Context, branchID int64, items []QueueItem) error {
	var salesIDs, purchaseIDs []int64
	seenS, seenP := map[int64]bool{}, map[int64]bool{}
	for _, it := range items {
		switch {
		case it.orderKind == orderKindSales && it.orderID != 0 && !seenS[it.orderID]:
			seenS[it.orderID] = true
			salesIDs = append(salesIDs, it.orderID)
		case it.orderKind == orderKindPurchase && it.orderID != 0 && !seenP[it.orderID]:
			seenP[it.orderID] = true
			purchaseIDs = append(purchaseIDs, it.orderID)
		}
	}
	salesDocs := map[int64]*salescontracts.OrderSummary{}
	if len(salesIDs) > 0 {
		var err error
		if salesDocs, err = s.sales.SummariesByIDs(ctx, salesIDs); err != nil {
			return apperror.Internal(err)
		}
	}
	purchaseDocs := map[int64]*purchasingcontracts.OrderSummary{}
	if len(purchaseIDs) > 0 {
		var err error
		if purchaseDocs, err = s.purchasing.SummariesByIDs(ctx, purchaseIDs); err != nil {
			return apperror.Internal(err)
		}
	}
	withNumber := func(it *QueueItem, number string) {
		if number == "" {
			return
		}
		if it.Summary == "Pesanan" {
			it.Summary = "Pesanan " + number
		} else {
			it.Summary += " · " + number
		}
	}
	for i := range items {
		it := &items[i]
		switch it.orderKind {
		case orderKindSales:
			if so := salesDocs[it.orderID]; so != nil && so.BranchID == branchID {
				it.PartyID, it.PartyName = so.PartyID, so.PartyName
				withNumber(it, so.Number)
				if it.AmountKind == "order" {
					it.Amount = so.GrandTotal
				}
			}
		case orderKindPurchase:
			if po := purchaseDocs[it.orderID]; po != nil && po.BranchID == branchID {
				it.PartyID, it.PartyName = po.PartyID, po.PartyName
				withNumber(it, po.Number)
				if it.AmountKind == "order" {
					it.Amount = po.GrandTotal
				}
			}
		}
	}
	// A summary row may carry the party id without its name: one grouped label
	// read names the rest.
	var missing []int64
	seen := map[int64]bool{}
	for _, it := range items {
		if it.PartyID != 0 && it.PartyName == "" && !seen[it.PartyID] {
			seen[it.PartyID] = true
			missing = append(missing, it.PartyID)
		}
	}
	if len(missing) > 0 {
		labels := s.partyLabels(ctx, missing)
		for i := range items {
			if l, ok := labels[items[i].PartyID]; ok && items[i].PartyName == "" {
				items[i].PartyName = l.Name
			}
		}
	}
	return nil
}

// closedMonths returns the set of "YYYY-MM" months already closed.
func (s *Service) closedMonths(ctx context.Context, branchID int64) (map[string]bool, error) {
	periods, err := s.repo.ListPeriods(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	out := map[string]bool{}
	for _, p := range periods {
		if p.Status == "closed" {
			out[fmt.Sprintf("%04d-%02d", p.Year, p.Month)] = true
		}
	}
	return out, nil
}

// SetCutover declares that documents of docType with id <= coveredThrough are
// already journalled elsewhere (opening balances or migrated legacy journals)
// and must never enter the posting queue. Idempotent. Refused when this system
// has itself journalled a document at or below the boundary: that would claim
// documents already in the books.
func (s *Service) SetCutover(ctx context.Context, actorID, branchID int64, docType string, coveredThrough int64) error {
	valid := map[string]bool{SourceDelivery: true, SourceGoodsReceipt: true, SourcePayment: true,
		SourceSalesReturn: true, SourcePurchaseReturn: true, SourceStockMovement: true,
		SourceSalesReturnSettlement: true, SourcePurchaseReturnSettlement: true}
	if !valid[docType] {
		return apperror.Validation("", []apperror.FieldError{{Field: "docType", Message: "jenis dokumen tidak dikenal"}})
	}
	if coveredThrough < 0 {
		return apperror.Validation("", []apperror.FieldError{{Field: "coveredThroughId", Message: "tidak boleh negatif"}})
	}
	if coveredThrough > 0 {
		if min, err := s.repo.MinPostedSourceID(ctx, branchID, docType, coveredThrough); err != nil {
			return apperror.Internal(err)
		} else if min != 0 {
			return apperror.Conflict("Batas cutover menutupi dokumen " + docType + " #" + strconv.FormatInt(min, 10) +
				" yang sudah dijurnal sistem ini")
		}
	}
	if err := s.repo.SetCutover(ctx, branchID, docType, coveredThrough, actorID); err != nil {
		return apperror.Internal(err)
	}
	_ = s.audit.Log(ctx, auditcontracts.Entry{Action: "finance.cutover", Entity: "finance_cutover", BranchID: branchID, ActorID: actorID,
		Note: docType + "<=" + strconv.FormatInt(coveredThrough, 10)})
	return nil
}

// GetCutovers returns the boundaries of a branch.
func (s *Service) GetCutovers(ctx context.Context, branchID int64) (map[string]int64, error) {
	c, err := s.repo.Cutovers(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	return c, nil
}

// BatchItem is one posting request inside a batch.
type BatchItem struct {
	DocType string `json:"docType"`
	DocID   int64  `json:"docId"`
	// Action is "post" (default), "catchup" (journal a closed-month document
	// into the open period, marked Susulan) or "reverse" (an orphan: the journal of a
	// cancelled document that was not reversed automatically).
	Action string `json:"action,omitempty"`
}

// BatchResult is the per-item outcome. One item's failure never aborts the
// rest — the accountant fixes the failure and re-runs; idempotency makes
// re-runs safe.
type BatchResult struct {
	DocType     string `json:"docType"`
	DocID       int64  `json:"docId"`
	EntryID     int64  `json:"entryId,omitempty"`
	EntryNumber string `json:"entryNumber,omitempty"`
	Error       string `json:"error,omitempty"`
}

// PostBatch posts up to MaxBatch documents, collecting per-item outcomes.
func (s *Service) PostBatch(ctx context.Context, actorID, branchID int64, items []BatchItem) ([]BatchResult, error) {
	if len(items) == 0 {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "wajib diisi"}})
	}
	if len(items) > MaxBatch {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "items", Message: "maksimal 50 dokumen per batch"}})
	}
	out := make([]BatchResult, 0, len(items))
	ignores, err := s.repo.PostingIgnores(ctx, branchID)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, it := range items {
		res := BatchResult{DocType: it.DocType, DocID: it.DocID}
		if _, ignored := ignores[infrastructure.IgnoreKey(it.DocType, it.DocID)]; ignored && it.Action != "reverse" {
			// An ignored document is out of the queue on purpose (FN-01):
			// restore it first, then post.
			res.Error = "Dokumen sedang diabaikan; pulihkan dulu sebelum diposting"
			out = append(out, res)
			continue
		}
		if it.Action == "reverse" {
			if _, rerr := s.ReverseForSource(ctx, actorID, branchID, it.DocType, it.DocID, "Dokumen dibatalkan"); rerr != nil {
				if appErr, ok := rerr.(*apperror.AppError); ok {
					res.Error = appErr.Message
				} else {
					res.Error = rerr.Error()
				}
			}
			out = append(out, res)
			continue
		}
		post := s.Post
		if it.Action == "catchup" {
			post = s.PostCatchUp
		}
		entry, err := post(ctx, actorID, branchID, it.DocType, it.DocID)
		if err != nil {
			if appErr, ok := err.(*apperror.AppError); ok {
				res.Error = appErr.Message
			} else {
				res.Error = err.Error()
			}
		} else {
			res.EntryID, res.EntryNumber = entry.ID, entry.Number
		}
		out = append(out, res)
	}
	return out, nil
}

// MaxCloseDay caps how many documents one close-day call posts in total (MaxBatch
// per round, several rounds). A bigger queue is reported through Remaining so the
// caller runs it again; one HTTP request never turns into an unbounded job.
const MaxCloseDay = 500

// CloseDayResult is the outcome of CloseDay: every attempted document, how many
// went through, and how many eligible documents are STILL queued afterwards
// (failures and anything beyond MaxCloseDay), so a close never leaves a silent rest.
type CloseDayResult struct {
	Results   []BatchResult `json:"results"`
	Posted    int           `json:"posted"`
	Failed    int           `json:"failed"`
	Remaining int           `json:"remaining"`
}

// closeDayEligible lists the queue items a close for `day` may post: not in a
// closed month, not ignored (FN-01), dated on or before the day (dateless ones
// ride along so they are never stranded outside every daily close).
func closeDayEligible(queue []QueueItem, day string) []QueueItem {
	var out []QueueItem
	for _, q := range queue {
		if q.PeriodClosed || q.Ignored {
			continue
		}
		if q.Date != "" && len(q.Date) >= 10 && q.Date[:10] > day {
			continue
		}
		out = append(out, q)
	}
	return out
}

// CloseDay posts every queued document dated on or before day (YYYY-MM-DD)
// — the "tutup harian" that replaces the legacy close-day endpoint. It posts in
// rounds of MaxBatch until nothing eligible is left or MaxCloseDay documents were
// attempted; a document that failed is not retried within the same call.
func (s *Service) CloseDay(ctx context.Context, actorID, branchID int64, day string) (*CloseDayResult, error) {
	day = strings.TrimSpace(day)
	if _, err := timeutil.ParseDateInput(day); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	// A day that has not come yet cannot be closed: its documents (a shipment
	// dated ahead) are journaled by the close of their own day.
	if timeutil.AfterTodayWIB(day) {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "tidak boleh di masa depan"}})
	}
	res := &CloseDayResult{Results: []BatchResult{}}
	attempted := map[string]bool{}
	for len(attempted) < MaxCloseDay {
		queue, err := s.UnpostedSources(ctx, branchID)
		if err != nil {
			return nil, err
		}
		batch := make([]BatchItem, 0, MaxBatch)
		for _, q := range closeDayEligible(queue, day) {
			key := fmt.Sprintf("%s:%d", q.DocType, q.DocID)
			if attempted[key] {
				continue
			}
			batch = append(batch, BatchItem{DocType: q.DocType, DocID: q.DocID})
			if len(batch) == MaxBatch || len(attempted)+len(batch) >= MaxCloseDay {
				break
			}
		}
		if len(batch) == 0 {
			break
		}
		out, err := s.PostBatch(ctx, actorID, branchID, batch)
		if err != nil {
			return nil, err
		}
		for _, r := range out {
			attempted[fmt.Sprintf("%s:%d", r.DocType, r.DocID)] = true
			if r.Error != "" {
				res.Failed++
			} else {
				res.Posted++
			}
		}
		res.Results = append(res.Results, out...)
	}
	queue, err := s.UnpostedSources(ctx, branchID)
	if err != nil {
		return nil, err
	}
	res.Remaining = len(closeDayEligible(queue, day))
	return res, nil
}
