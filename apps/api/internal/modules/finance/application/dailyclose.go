package application

import (
	"context"
	"errors"
	"sort"
	"strings"

	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// Daily close overview (FN-02). The legacy "tutup hari" screen showed today's
// human figures, a status for today alone, the tasks only the owner can unblock
// and the backlog of earlier days. The revamp derives all of it on read:
// figures come from the live journals of the day (so they always match the
// books), the queue comes from UnpostedSources, and the "why is it stuck" reason
// comes from running the very builder that Post would run.

// Today statuses (legacy vocabulary).
const (
	DayEmpty    = "kosong"          // nothing happened today (not "already clean")
	DayPending  = "belum"           // documents are ready to post
	DayNotes    = "selesai_catatan" // routine postings done, owner tasks remain
	DayComplete = "lengkap"         // there was activity and all of it is in the books
)

// Reason codes of a document the books cannot take yet (legacy
// NEEDS_INPUT_REASON_CODES, plus the revamp's mapping/period causes).
const (
	ReasonMissingCost    = "missing_cost_basis"
	ReasonMissingOrder   = "missing_source_order"
	ReasonMissingMapping = "missing_account_mapping"
	// ReasonAwaitingOrder: a direct delivery note (SJ Langsung, ADR-0022) whose
	// order has not been built yet — the sales admin, not the accountant,
	// unblocks it.
	ReasonAwaitingOrder = "awaiting_order"
	ReasonOther         = "build_failed"
)

// maxOverviewProbe bounds how many unposted documents the overview tries to
// build (each probe is a full journal build). The count of the rest stays exact.
const maxOverviewProbe = 200

// DayTotals are the figures of one day, read from the live journals.
type DayTotals struct {
	Sales     DayAmount `json:"sales"`
	Purchases DayAmount `json:"purchases"`
	Returns   DayAmount `json:"returns"`
	CashIn    int64     `json:"cashIn"`
	CashOut   int64     `json:"cashOut"`
}

// DayAmount is a sum with the number of journals behind it.
type DayAmount struct {
	Amount int64 `json:"amount"`
	Count  int64 `json:"count"`
}

// OverviewTaskItem is one stuck document inside a task group.
type OverviewTaskItem struct {
	DocType   string `json:"docType"`
	DocID     int64  `json:"docId"`
	Number    string `json:"number"`
	Date      string `json:"date"`
	Amount    int64  `json:"amount"`
	PartyName string `json:"partyName,omitempty"`
	Message   string `json:"message"`
}

// OverviewTask groups stuck documents by root cause.
type OverviewTask struct {
	Code  string             `json:"code"`
	Label string             `json:"label"`
	Hint  string             `json:"hint"`
	Count int                `json:"count"`
	Items []OverviewTaskItem `json:"items"`
}

// DayBacklog is the work left over from earlier days. It never changes today's
// status.
type DayBacklog struct {
	Count      int            `json:"count"`
	Ready      int            `json:"ready"`
	NeedsInput int            `json:"needsInput"`
	Tasks      []OverviewTask `json:"tasks"`
}

// DailyOverview is the state of the daily-close screen.
type DailyOverview struct {
	Date     string         `json:"date"`
	Status   string         `json:"status"`
	Activity int            `json:"activity"`
	Totals   DayTotals      `json:"totals"`
	Ready    ReadyCounts    `json:"ready"`
	Tasks    []OverviewTask `json:"tasks"`
	Backlog  DayBacklog     `json:"backlog"`
	// Ignored counts documents deliberately left out of the queue (FN-01).
	Ignored int `json:"ignored"`
	// DataWarning counts unposted documents with no usable date: they ride along
	// every close and need an owner's eye.
	DataWarning DataWarning `json:"dataWarning"`
	// Probed/ProbeCapped tell the UI the task grouping looked at a bounded
	// number of documents (the counts above stay exact).
	Probed      int  `json:"probed"`
	ProbeCapped bool `json:"probeCapped"`
}

// ReadyCounts splits what can be posted right now into today and earlier days.
type ReadyCounts struct {
	Today   int `json:"today"`
	Backlog int `json:"backlog"`
}

// DataWarning flags documents with an unusable date.
type DataWarning struct {
	InvalidDateCount int `json:"invalidDateCount"`
}

// DailyCloseOverview builds the screen state for one day (default today in
// Jakarta). Read-only.
func (s *Service) DailyCloseOverview(ctx context.Context, branchID int64, day string) (*DailyOverview, error) {
	day = strings.TrimSpace(day)
	if day == "" {
		day = timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	} else if _, err := timeutil.ParseDateInput(day); err != nil {
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "date", Message: "format tanggal harus YYYY-MM-DD"}})
	}
	totals, paymentCount, err := s.dayTotals(ctx, branchID, day)
	if err != nil {
		return nil, err
	}
	queue, err := s.UnpostedSources(ctx, branchID)
	if err != nil {
		return nil, err
	}

	out := &DailyOverview{Date: day, Totals: totals, Tasks: []OverviewTask{}, Backlog: DayBacklog{Tasks: []OverviewTask{}}}
	var today, backlog []QueueItem
	for _, q := range queue {
		if q.Ignored {
			out.Ignored++
			continue
		}
		if len(q.Date) < 10 {
			out.DataWarning.InvalidDateCount++
			continue
		}
		if q.PeriodClosed {
			continue // late documents of closed months are surfaced by readiness
		}
		switch d := q.Date[:10]; {
		case d == day:
			today = append(today, q)
		case d < day:
			backlog = append(backlog, q)
		}
	}
	out.Backlog.Count = len(backlog)

	probed := 0
	probe := func(items []QueueItem) (ready int, groups map[string]*OverviewTask, err error) {
		groups = map[string]*OverviewTask{}
		for _, it := range items {
			if probed >= maxOverviewProbe {
				out.ProbeCapped = true
				break
			}
			probed++
			_, _, _, berr := s.buildLines(ctx, branchID, it.DocType, it.DocID)
			if berr == nil {
				ready++
				continue
			}
			var ae *apperror.AppError
			if !errors.As(berr, &ae) {
				return 0, nil, berr
			}
			code := classifyBuildFailure(ae.Message)
			g := groups[code]
			if g == nil {
				label, hint := reasonText(code)
				g = &OverviewTask{Code: code, Label: label, Hint: hint, Items: []OverviewTaskItem{}}
				groups[code] = g
			}
			g.Count++
			g.Items = append(g.Items, OverviewTaskItem{DocType: it.DocType, DocID: it.DocID, Number: it.Number,
				Date: it.Date, Amount: it.Amount, PartyName: it.PartyName, Message: ae.Message})
		}
		return ready, groups, nil
	}

	readyToday, todayGroups, err := probe(today)
	if err != nil {
		return nil, err
	}
	readyBack, backGroups, err := probe(backlog)
	if err != nil {
		return nil, err
	}
	out.Tasks = orderedTasks(todayGroups)
	out.Backlog.Tasks = orderedTasks(backGroups)
	out.Ready = ReadyCounts{Today: readyToday, Backlog: readyBack}
	out.Backlog.Ready = readyBack
	for _, t := range out.Backlog.Tasks {
		out.Backlog.NeedsInput += t.Count
	}
	out.Probed = probed

	todayStuck := 0
	for _, t := range out.Tasks {
		todayStuck += t.Count
	}
	// Activity: documents of the day, posted (journals) or still waiting.
	posted := int(totals.Sales.Count + totals.Purchases.Count + totals.Returns.Count + paymentCount)
	out.Activity = posted + len(today)
	switch {
	case out.Activity == 0:
		out.Status = DayEmpty
	case len(today)-todayStuck > 0:
		// Documents that are ready (or were not probed) still wait to be posted.
		out.Status = DayPending
	case todayStuck > 0:
		out.Status = DayNotes
	default:
		out.Status = DayComplete
	}
	return out, nil
}

// dayTotals reads the day's figures off the live journals: sales = revenue
// credited by shipments, purchases = inventory debited by receipts, returns =
// revenue reversed by sales returns, cash = cash/bank accounts moved by payments.
func (s *Service) dayTotals(ctx context.Context, branchID int64, day string) (DayTotals, int64, error) {
	var res DayTotals
	foot, counts, err := s.repo.DaySourceFootings(ctx, branchID, day+" 00:00:00", day+" 23:59:59")
	if err != nil {
		return res, 0, apperror.Internal(err)
	}
	revenue, err := s.repo.MappingAccount(ctx, "revenue")
	if err != nil {
		return res, 0, apperror.Internal(err)
	}
	salesReturn, err := s.repo.MappingAccount(ctx, "sales_return")
	if err != nil {
		return res, 0, apperror.Internal(err)
	}
	inventory, err := s.repo.MappingAccount(ctx, "inventory")
	if err != nil {
		return res, 0, apperror.Internal(err)
	}
	accounts, err := s.repo.ListAccounts(ctx, "")
	if err != nil {
		return res, 0, apperror.Internal(err)
	}
	cash := map[int64]bool{}
	for _, a := range accounts {
		if a.IsCash {
			cash[a.ID] = true
		}
	}
	idOf := func(id int64, set ...int64) bool {
		for _, x := range set {
			if x != 0 && x == id {
				return true
			}
		}
		return false
	}
	var revID, retID, invID int64
	if revenue != nil {
		revID = revenue.ID
	}
	if salesReturn != nil {
		retID = salesReturn.ID
	}
	if inventory != nil {
		invID = inventory.ID
	}
	for _, f := range foot {
		switch f.SourceType {
		case SourceDelivery:
			if idOf(f.AccountID, revID) {
				res.Sales.Amount += f.Credit - f.Debit
			}
		case SourceGoodsReceipt:
			if idOf(f.AccountID, invID) {
				res.Purchases.Amount += f.Debit - f.Credit
			}
		case SourceSalesReturn:
			// Revenue given back: debit on the contra account (4100) or on revenue itself.
			if idOf(f.AccountID, retID, revID) {
				res.Returns.Amount += f.Debit - f.Credit
			}
		case SourcePayment:
			if cash[f.AccountID] {
				res.CashIn += f.Debit
				res.CashOut += f.Credit
			}
		}
	}
	res.Sales.Count = counts[SourceDelivery]
	res.Purchases.Count = counts[SourceGoodsReceipt]
	res.Returns.Count = counts[SourceSalesReturn]
	return res, counts[SourcePayment], nil
}

func classifyBuildFailure(msg string) string {
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "menunggu order"):
		return ReasonAwaitingOrder
	case strings.Contains(m, "harga modal") || strings.Contains(m, "biaya persediaan"):
		return ReasonMissingCost
	case strings.Contains(m, "butuh order") || strings.Contains(m, "order dengan data"):
		return ReasonMissingOrder
	case strings.Contains(m, "pemetaan akun") || strings.Contains(m, "tidak tersedia"):
		return ReasonMissingMapping
	default:
		return ReasonOther
	}
}

func reasonText(code string) (label, hint string) {
	switch code {
	case ReasonMissingCost:
		return "Harga modal belum ada", "Lengkapi harga modal produk (saldo awal / penerimaan), lalu posting ulang."
	case ReasonMissingOrder:
		return "Pesanan asal tanpa data pihak", "Dokumen butuh pesanan asal dengan customer/supplier yang lengkap."
	case ReasonMissingMapping:
		return "Pemetaan akun belum lengkap", "Atur pemetaan akun otomatis di halaman Akun."
	case ReasonAwaitingOrder:
		return "Surat jalan langsung menunggu order", "Admin penjualan membuat order dari surat jalan di menu Order Jual (Tambah order → Dari surat jalan); jurnal terbentuk setelah ordernya ada."
	default:
		return "Tidak bisa dijurnal", "Periksa pesan di tiap dokumen; abaikan dokumen yang memang tidak perlu dijurnal."
	}
}

func orderedTasks(groups map[string]*OverviewTask) []OverviewTask {
	out := make([]OverviewTask, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Code < out[j].Code
	})
	return out
}
