package application

import (
	"context"
	"sort"
	"time"

	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// AgingBuckets are the age bands of an open document, in days since the
// document date (legacy parity: 0-30 / 31-60 / 61-90 / over 90).
type AgingBuckets struct {
	Current int64 `json:"d0_30"`
	D31_60  int64 `json:"d31_60"`
	D61_90  int64 `json:"d61_90"`
	Over90  int64 `json:"over90"`
}

// AgingDoc is one open order with what is still owed.
type AgingDoc struct {
	OrderID     int64  `json:"orderId"`
	Number      string `json:"number"`
	PartyID     int64  `json:"partyId"`
	PartyName   string `json:"partyName"`
	OrderDate   string `json:"orderDate"`
	DueDate     string `json:"dueDate"`
	GrandTotal  int64  `json:"grandTotal"`
	Paid        int64  `json:"paid"`
	Outstanding int64  `json:"outstanding"`
	AgeDays     int    `json:"ageDays"`
	OverdueDays int    `json:"overdueDays"`
}

// AgingReport is the per-document aging of receivables or payables.
//
// GLBalance/Difference reconcile it with the books (F4): the control
// account's balance as of the same date (Piutang Usaha / Hutang Usaha, every
// line — with or without a party) against the documents' Total. Opening
// balances and adjustments without a document, unposted documents, and
// walk-in lines explain the difference; the page shows it, never hides it.
type AgingReport struct {
	AsOf        string       `json:"asOf"`
	Documents   []AgingDoc   `json:"documents"`
	Buckets     AgingBuckets `json:"buckets"`
	Total       int64        `json:"total"`
	AccountCode string       `json:"accountCode"`
	AccountName string       `json:"accountName"`
	GLBalance   int64        `json:"glBalance"`
	Difference  int64        `json:"difference"`
}

type agingRow struct {
	id, party, grand int64
	number, od, due  string
}

// AgingDocuments lists the open sales (kind "receivable") or purchase orders
// (kind "payable") with their outstanding amount and age as of a date. Age
// counts from the document date; overdue days from its due date. One grouped
// payment read per 1.000 orders — no per-document query.
//
// "As of" is a real cut-off (F4): orders dated after it are left out, and
// only payments (offset legs of returns included) dated on or before it are
// subtracted. Both dates are WIB wall-clock document dates, compared by day.
func (s *Service) AgingDocuments(ctx context.Context, branchID int64, kind, asOf string) (*AgingReport, error) {
	if s.salesOps == nil || s.purchOps == nil {
		return nil, apperror.Internal(errOpsUnwired)
	}
	day := timeutil.NowUTC().In(timeutil.Jakarta)
	if asOf != "" {
		d, err := timeutil.ParseDateInput(asOf)
		if err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "asOf", Message: "format tanggal harus YYYY-MM-DD"}})
		}
		day = d.In(timeutil.Jakarta)
	}
	if kind != "receivable" && kind != "payable" {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "kind", Message: "harus receivable atau payable"}})
	}
	rep := &AgingReport{AsOf: day.Format("2006-01-02"), Documents: []AgingDoc{}}
	orderType := paymentcontracts.OrderSales
	if kind == "payable" {
		orderType = paymentcontracts.OrderPurchase
	}
	var after int64
	partyIDs := map[int64]bool{}
	for {
		rows, err := s.agingPage(ctx, branchID, kind, after)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			break
		}
		after = rows[len(rows)-1].id
		ids := make([]int64, 0, len(rows))
		dated := make([]agingRow, 0, len(rows))
		for _, r := range rows {
			if r.od > rep.AsOf { // YYYY-MM-DD: string order is date order
				continue
			}
			ids = append(ids, r.id)
			dated = append(dated, r)
		}
		paid, err := s.payment.PaidForOrdersAsOf(ctx, orderType, ids, rep.AsOf)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, r := range dated {
			out := r.grand - paid[r.id]
			if out <= 0 {
				continue
			}
			doc := AgingDoc{OrderID: r.id, Number: r.number, PartyID: r.party, OrderDate: r.od, DueDate: r.due,
				GrandTotal: r.grand, Paid: paid[r.id], Outstanding: out}
			if t, perr := time.ParseInLocation("2006-01-02", r.od, timeutil.Jakarta); perr == nil {
				doc.AgeDays = int(day.Sub(t).Hours() / 24)
			}
			if r.due != "" {
				if t, perr := time.ParseInLocation("2006-01-02", r.due, timeutil.Jakarta); perr == nil {
					if d := int(day.Sub(t).Hours() / 24); d > 0 {
						doc.OverdueDays = d
					}
				}
			}
			switch {
			case doc.AgeDays <= 30:
				rep.Buckets.Current += out
			case doc.AgeDays <= 60:
				rep.Buckets.D31_60 += out
			case doc.AgeDays <= 90:
				rep.Buckets.D61_90 += out
			default:
				rep.Buckets.Over90 += out
			}
			rep.Total += out
			partyIDs[r.party] = true
			rep.Documents = append(rep.Documents, doc)
		}
		if len(rows) < 1000 {
			break
		}
	}
	ids := make([]int64, 0, len(partyIDs))
	for id := range partyIDs {
		ids = append(ids, id)
	}
	if err := s.agingReconcile(ctx, branchID, kind, rep); err != nil {
		return nil, err
	}
	labels := s.partyLabels(ctx, ids)
	for i := range rep.Documents {
		if l, ok := labels[rep.Documents[i].PartyID]; ok {
			rep.Documents[i].PartyName = l.Name
		} else if rep.Documents[i].PartyID == 0 {
			rep.Documents[i].PartyName = "Tunai"
		}
	}
	sort.Slice(rep.Documents, func(i, j int) bool {
		if rep.Documents[i].AgeDays != rep.Documents[j].AgeDays {
			return rep.Documents[i].AgeDays > rep.Documents[j].AgeDays
		}
		return rep.Documents[i].OrderID < rep.Documents[j].OrderID
	})
	return rep, nil
}

func (s *Service) agingPage(ctx context.Context, branchID int64, kind string, after int64) ([]agingRow, error) {
	var rows []agingRow
	if kind == "receivable" {
		page, err := s.salesOps.OpenOrderTotals(ctx, branchID, after, 1000)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, r := range page {
			rows = append(rows, agingRow{r.ID, r.PartyID, r.GrandTotal, r.Number, r.OrderDate, r.DueDate})
		}
		return rows, nil
	}
	page, err := s.purchOps.OpenOrderTotals(ctx, branchID, after, 1000)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, r := range page {
		rows = append(rows, agingRow{r.ID, r.PartyID, r.GrandTotal, r.Number, r.OrderDate, r.DueDate})
	}
	return rows, nil
}

// agingReconcile fills the control-account side of the report: the mapped
// receivable/payable account's commercial balance as of the same day, signed
// as "still owed", and its difference from the documents' total.
func (s *Service) agingReconcile(ctx context.Context, branchID int64, kind string, rep *AgingReport) error {
	account, err := s.repo.MappingAccount(ctx, kind)
	if err != nil {
		return apperror.Internal(err)
	}
	if account == nil {
		rep.Difference = -rep.Total
		return nil
	}
	footings, err := s.repo.AccountFootings(ctx, branchID, "", rep.AsOf+" 23:59:59")
	if err != nil {
		return apperror.Internal(err)
	}
	f := footings[account.ID]
	rep.AccountCode, rep.AccountName = account.Code, account.Name
	rep.GLBalance = f[0] - f[1] // piutang is debit-heavy
	if kind == "payable" {
		rep.GLBalance = f[1] - f[0] // hutang is credit-heavy
	}
	rep.Difference = rep.GLBalance - rep.Total
	return nil
}
