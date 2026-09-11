package application

import (
	"context"
	"strconv"
	"strings"
	"time"

	"mini-erp/internal/modules/delivery/infrastructure"
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
}

// WorkQueue is the warehouse worklist: shippable orders + waiting drafts.
type WorkQueue struct {
	CreateSJ      []QueueOrder  `json:"createSj"`
	WaitingReturn []WaitingNote `json:"waitingReturn"`
	Limit         int           `json:"limit"`
}

// WorkQueue composes the warehouse worklist (J3) in a bounded, constant
// number of grouped reads (P3): 1 open-shipments read (sales) + 1 delivered
// bulk + 1 draft counts + 1 waiting drafts — plus rare per-order fallbacks
// when a draft SJ outlives its order's confirmed status. Text search filters
// the capped worklist in memory (number/customer/SJ), like the finance
// posting queue's operator-worklist cap.
func (s *Service) WorkQueue(ctx context.Context, branchID int64, search, from, to, age string, limit int) (*WorkQueue, error) {
	if limit <= 0 || limit > QueueMaxLimit {
		limit = QueueDefaultLimit
	}
	switch age {
	case "", "today", "gt_2", "gt_7":
	default:
		return nil, apperror.Validation("", []apperror.FieldError{{Field: "age", Message: "harus today, gt_2, atau gt_7"}})
	}
	if strings.TrimSpace(from) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(from)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "from", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	}
	if strings.TrimSpace(to) != "" {
		if _, err := timeutil.ParseDateInput(strings.TrimSpace(to)); err != nil {
			return nil, apperror.Validation("", []apperror.FieldError{{Field: "to", Message: "format tanggal harus YYYY-MM-DD"}})
		}
	}

	open, err := s.sales.OpenShipments(ctx, branchID, limit)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(open))
	for _, o := range open {
		ids = append(ids, o.OrderID)
	}
	delivered, err := s.repo.DeliveredQtyBulk(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	drafts, err := s.repo.DraftCounts(ctx, ids)
	if err != nil {
		return nil, apperror.Internal(err)
	}
	waiting, err := s.repo.WaitingReturn(ctx, branchID,
		strings.TrimSpace(from), strings.TrimSpace(to), age, limit)
	if err != nil {
		return nil, apperror.Internal(err)
	}

	byOrder := make(map[int64]*salescontracts.OpenShipment, len(open))
	for _, o := range open {
		byOrder[o.OrderID] = o
	}
	needles := strings.Fields(strings.ToLower(strings.TrimSpace(search)))
	match := func(haystacks ...string) bool {
		if len(needles) == 0 {
			return true
		}
		for _, n := range needles {
			found := false
			for _, h := range haystacks {
				if strings.Contains(strings.ToLower(h), n) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}

	createSJ := make([]QueueOrder, 0, len(open))
	for _, o := range open {
		q := buildQueueOrder(o, delivered, drafts[o.OrderID])
		if len(q.Lines) == 0 {
			continue
		}
		if !match(o.Number, o.PartyName) {
			continue
		}
		createSJ = append(createSJ, *q)
	}

	today := timeutil.NowUTC().In(timeutil.Jakarta)
	waitingOut := make([]WaitingNote, 0, len(waiting))
	for _, n := range waiting {
		orderNumber, partyName := "", ""
		if o, ok := byOrder[n.SalesOrderID]; ok {
			orderNumber, partyName = o.Number, o.PartyName
		} else {
			// Rare edge: a draft SJ outliving its order's confirmed status
			// (the order completed through another SJ). One fallback read.
			so, err := s.sales.GetByID(ctx, n.SalesOrderID)
			if err != nil {
				return nil, err
			}
			if so == nil || so.BranchID != branchID {
				continue
			}
			orderNumber, partyName = so.Number, so.PartyName
		}
		if !match(n.Number, orderNumber, partyName) {
			continue
		}
		waitingOut = append(waitingOut, WaitingNote{
			ID: n.ID, Number: n.Number, SalesOrderID: n.SalesOrderID,
			OrderNumber: orderNumber, PartyName: partyName,
			DeliveryDate: n.DeliveryDate, AgeDays: ageDays(n.DeliveryDate, today),
			DriverName: n.DriverName, VehiclePlate: n.VehiclePlate,
		})
	}
	return &WorkQueue{CreateSJ: createSJ, WaitingReturn: waitingOut, Limit: limit}, nil
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
