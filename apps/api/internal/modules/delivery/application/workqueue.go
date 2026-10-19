package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/modules/delivery/infrastructure"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// Work-queue caps mirror legacy (F-06.2): limit 1–200, default 100. The queue
// is an operator worklist, not a report — beyond the cap the warehouse ships
// in batches and the list shrinks.
const (
	QueueDefaultLimit = 100
	QueueMaxLimit     = 200
)

// QueueLine is one shippable aggregate: ordered (base) minus confirmed
// delivered (base). Lines aggregate per (product, variant) like the
// over-SO guard in Create, so a SO sold in two UOMs shows one row.
type QueueLine struct {
	ProductID   int64   `json:"productId"`
	VariantID   int64   `json:"variantId"`
	ProductCode string  `json:"productCode"`
	ProductName string  `json:"productName"`
	UOM         string  `json:"uom"`
	Ordered     float64 `json:"ordered"`
	Delivered   float64 `json:"delivered"`
	Remaining   float64 `json:"remaining"`
}

// QueueOrder is one confirmed order that still needs shipment.
type QueueOrder struct {
	OrderID        int64       `json:"orderId"`
	Number         string      `json:"number"`
	PartyName      string      `json:"partyName"`
	OrderDate      string      `json:"orderDate"`
	DueDate        string      `json:"dueDate"`
	PaymentTerms   string      `json:"paymentTerms"`
	GrandTotal     int64       `json:"grandTotal"`
	Lines          []QueueLine `json:"lines"`
	TotalOrdered   float64     `json:"totalOrdered"`
	TotalDelivered float64     `json:"totalDelivered"`
	PendingDrafts  int64       `json:"pendingDrafts"`
	// ItemCount is the number of product lines still to ship (DL-02 card).
	ItemCount int `json:"itemCount"`
}

// WaitingNote is one draft SJ whose physical papers have not come back.
type WaitingNote struct {
	ID           int64  `json:"id"`
	Number       string `json:"number"`
	SalesOrderID int64  `json:"salesOrderId"`
	OrderNumber  string `json:"orderNumber"`
	PartyName    string `json:"partyName"`
	DeliveryDate string `json:"deliveryDate"`
	AgeDays      int    `json:"ageDays"`
	DriverName   string `json:"driverName"`
	VehiclePlate string `json:"vehiclePlate"`
	// HasSignedDocument says the signed-SJ scan/photo is already uploaded (DL-01).
	HasSignedDocument bool `json:"hasSignedDocument"`
}

// WorkQueue is the warehouse worklist: shippable orders + waiting drafts.
type WorkQueue struct {
	CreateSJ      []QueueOrder  `json:"createSj"`
	WaitingReturn []WaitingNote `json:"waitingReturn"`
	Limit         int           `json:"limit"`
}

// queueScanPages bounds how many pages of open orders one request may scan to
// fill the list after hiding unpaid prepaid orders (DL-02).
const queueScanPages = 5

// WorkQueue composes the warehouse worklist (J3) in a bounded number of grouped
// reads (P3). DL-02: the search and both date filters run in SQL BEFORE the
// page cut (orders beyond the first page are findable); an order on prepaid
// terms that is not fully paid is hidden (no SJ may be made for it); each card
// carries its item count.
//
// create_sj: per scanned page 1 open-shipments read (sales; search = number OR
// customer through the party contract) + 1 delivered bulk + 1 draft counts, plus
// 1 paid-for-orders read when the page has prepaid orders. waiting_return: 1
// order-id search (only with a search term) + 1 notes read + 1 order summaries +
// 1 signed-document presence read.
func (s *Service) WorkQueue(ctx context.Context, branchID int64, search, from, to, age string, limit int) (*WorkQueue, error) {
	if limit <= 0 || limit > QueueMaxLimit {
		limit = QueueDefaultLimit
	}
	switch age {
	case "", "today", "gt_2", "gt_7":
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "age", Message: "harus today, gt_2, atau gt_7"}})
	}
	from, to, search = strings.TrimSpace(from), strings.TrimSpace(to), strings.TrimSpace(search)
	if from != "" {
		if _, err := timeutil.ParseDateInput(from); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	}
	if to != "" {
		if _, err := timeutil.ParseDateInput(to); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	}

	createSJ, err := s.queueCreateSJ(ctx, branchID, search, from, to, limit)
	if err != nil {
		return nil, err
	}
	waitingOut, err := s.queueWaiting(ctx, branchID, search, from, to, age, limit)
	if err != nil {
		return nil, err
	}
	return &WorkQueue{CreateSJ: createSJ, WaitingReturn: waitingOut, Limit: limit}, nil
}

// queueCreateSJ builds the shippable-orders tab.
func (s *Service) queueCreateSJ(ctx context.Context, branchID int64, search, from, to string, limit int) ([]QueueOrder, error) {
	out := make([]QueueOrder, 0, limit)
	for page := 0; page < queueScanPages && len(out) < limit; page++ {
		open, err := s.sales.OpenShipmentsFiltered(ctx, branchID, salescontracts.OpenShipmentFilter{
			Search: search, From: from, To: to, Limit: limit, Offset: page * limit})
		if err != nil {
			return nil, err
		}
		if len(open) == 0 {
			break
		}
		ids := make([]int64, 0, len(open))
		var prepaid []int64
		for _, o := range open {
			ids = append(ids, o.OrderID)
			if o.PaymentTerms == "prepaid" {
				prepaid = append(prepaid, o.OrderID)
			}
		}
		unpaid, err := s.unpaidPrepaid(ctx, open, prepaid)
		if err != nil {
			return nil, err
		}
		delivered, err := s.repo.DeliveredQtyBulk(ctx, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		drafts, err := s.repo.DraftCounts(ctx, ids)
		if err != nil {
			return nil, apperror.Internal(err)
		}
		for _, o := range open {
			if unpaid[o.OrderID] || len(out) >= limit {
				continue
			}
			q := buildQueueOrder(o, delivered, drafts[o.OrderID])
			if len(q.Lines) == 0 {
				continue
			}
			out = append(out, *q)
		}
		if len(open) < limit {
			break // last page
		}
	}
	return out, nil
}

// unpaidPrepaid marks the prepaid orders that still owe money: no shipment can
// be drafted for them, so the warehouse queue must not offer them (DL-02).
func (s *Service) unpaidPrepaid(ctx context.Context, open []*salescontracts.OpenShipment, prepaid []int64) (map[int64]bool, error) {
	out := map[int64]bool{}
	if len(prepaid) == 0 || s.payments == nil {
		return out, nil
	}
	paid, err := s.payments.PaidForOrders(ctx, paymentcontracts.OrderSales, prepaid)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	for _, o := range open {
		if o.PaymentTerms == "prepaid" && o.GrandTotal-paid[o.OrderID] > 0 {
			out[o.OrderID] = true
		}
	}
	return out, nil
}

// queueWaiting builds the "waiting for the signed SJ" tab.
func (s *Service) queueWaiting(ctx context.Context, branchID int64, search, from, to, age string, limit int) ([]WaitingNote, error) {
	var orderIDs []int64
	if search != "" {
		ids, err := s.sales.SearchOrderIDs(ctx, branchID, search, 0)
		if err != nil {
			return nil, err
		}
		orderIDs = ids
	}
	waiting, err := s.repo.WaitingReturn(ctx, branchID, search, orderIDs, from, to, age, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	if len(waiting) == 0 {
		return []WaitingNote{}, nil
	}
	soIDs := make([]int64, 0, len(waiting))
	noteIDs := make([]int64, 0, len(waiting))
	for _, n := range waiting {
		soIDs = append(soIDs, n.SalesOrderID)
		noteIDs = append(noteIDs, n.ID)
	}
	orders, err := s.sales.SummariesByIDs(ctx, soIDs)
	if err != nil {
		return nil, err
	}
	signed := map[int64]string{}
	if s.media != nil {
		if signed, err = s.media.PrimaryByOwners(ctx, mediacontracts.OwnerDeliveryDocument, noteIDs); err != nil {
			return nil, apperror.Internal(err)
		}
	}
	today := timeutil.NowUTC().In(timeutil.Jakarta)
	out := make([]WaitingNote, 0, len(waiting))
	for _, n := range waiting {
		o := orders[n.SalesOrderID]
		if o == nil || o.BranchID != branchID {
			continue
		}
		_, has := signed[n.ID]
		out = append(out, WaitingNote{
			ID: n.ID, Number: n.Number, SalesOrderID: n.SalesOrderID,
			OrderNumber: o.Number, PartyName: o.PartyName,
			DeliveryDate: n.DeliveryDate, AgeDays: ageDays(n.DeliveryDate, today),
			DriverName: n.DriverName, VehiclePlate: n.VehiclePlate, HasSignedDocument: has,
		})
	}
	return out, nil
}

// buildQueueOrder aggregates one open order's lines per (product, variant)
// and keeps only aggregates that still need shipment. Pure: unit-tested
// without a database.
func buildQueueOrder(o *salescontracts.OpenShipment, delivered map[string]float64, pendingDrafts int64) *QueueOrder {
	q := &QueueOrder{
		OrderID: o.OrderID, Number: o.Number, PartyName: o.PartyName,
		OrderDate: o.OrderDate, DueDate: o.DueDate, PaymentTerms: o.PaymentTerms,
		GrandTotal: o.GrandTotal, PendingDrafts: pendingDrafts,
		Lines: []QueueLine{},
	}
	agg := map[string]*QueueLine{}
	order := []string{}
	for _, l := range o.Items {
		key := lineKey(l.ProductID, l.VariantID)
		row, ok := agg[key]
		if !ok {
			row = &QueueLine{ProductID: l.ProductID, VariantID: l.VariantID,
				ProductCode: l.ProductCode, ProductName: l.ProductName, UOM: l.UOM}
			agg[key] = row
			order = append(order, key)
		}
		row.Ordered += l.QtyBase
	}
	for _, key := range order {
		row := agg[key]
		parts := strings.Split(key, "/")
		pid, _ := strconv.ParseInt(parts[0], 10, 64)
		vid, _ := strconv.ParseInt(parts[1], 10, 64)
		row.Delivered = delivered[infrastructure.BulkKey(o.OrderID, pid, vid)]
		row.Remaining = row.Ordered - row.Delivered
		if row.Remaining <= 1e-9 {
			continue
		}
		q.Lines = append(q.Lines, *row)
		q.TotalOrdered += row.Ordered
		q.TotalDelivered += row.Delivered
	}
	q.ItemCount = len(q.Lines)
	return q
}

// ageDays counts calendar days from a delivery datetime ("2006-01-02
// 15:04:05") to today. Unparseable dates read as 0, never error — the queue
// must not break on data it does not know.
func ageDays(deliveryDate string, today time.Time) int {
	day := strings.TrimSpace(deliveryDate)
	if len(day) > 10 {
		day = day[:10]
	}
	from, err := time.Parse("2006-01-02", day)
	if err != nil {
		return 0
	}
	y, m, d := today.Date()
	n := time.Date(y, m, d, 0, 0, 0, 0, from.Location())
	diff := n.Sub(time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location()))
	return int(diff.Hours() / 24)
}
