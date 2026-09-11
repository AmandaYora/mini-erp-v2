package application

import (
	"testing"
	"time"

	"mini-erp/internal/modules/delivery/infrastructure"
	salescontracts "mini-erp/internal/modules/sales/contracts"
)

// TestBuildQueueOrderPartial (§7 Tahap J3): delivered sums subtract per
// (product, variant) aggregate and fully-shipped lines drop out.
func TestBuildQueueOrderPartial(t *testing.T) {
	o := &salescontracts.OpenShipment{OrderID: 7, Number: "SO-7", Items: []*salescontracts.Item{
		{ProductID: 1, VariantID: 1, ProductCode: "A", ProductName: "Barang A", UOM: "pcs", QtyBase: 10},
		{ProductID: 1, VariantID: 1, ProductCode: "A", ProductName: "Barang A", UOM: "pcs", QtyBase: 4},
		{ProductID: 2, VariantID: 3, ProductCode: "B", ProductName: "Barang B", UOM: "dus", QtyBase: 5},
	}}
	delivered := map[string]float64{
		infrastructure.BulkKey(7, 1, 1): 6,
		infrastructure.BulkKey(7, 2, 3): 5,
	}
	q := buildQueueOrder(o, delivered, 1)
	if len(q.Lines) != 1 {
		t.Fatalf("Lines = %d, want 1 (B fully shipped drops out)", len(q.Lines))
	}
	l := q.Lines[0]
	if l.Ordered != 14 || l.Delivered != 6 || l.Remaining != 8 {
		t.Fatalf("line = %+v, want ordered 14 delivered 6 remaining 8", l)
	}
	if q.TotalOrdered != 14 || q.TotalDelivered != 6 {
		t.Fatalf("totals = %v/%v, want 14/6", q.TotalOrdered, q.TotalDelivered)
	}
	if q.PendingDrafts != 1 {
		t.Fatalf("PendingDrafts = %d, want 1", q.PendingDrafts)
	}
}

// TestBuildQueueOrderFullyShipped: no remaining line means the order leaves
// the create_sj tab entirely.
func TestBuildQueueOrderFullyShipped(t *testing.T) {
	o := &salescontracts.OpenShipment{OrderID: 9, Items: []*salescontracts.Item{
		{ProductID: 1, VariantID: 1, QtyBase: 3},
	}}
	delivered := map[string]float64{infrastructure.BulkKey(9, 1, 1): 3}
	if q := buildQueueOrder(o, delivered, 0); len(q.Lines) != 0 {
		t.Fatalf("Lines = %d, want 0", len(q.Lines))
	}
}

// TestAgeDays pins the warehouse age badge: calendar days in Jakarta,
// garbage dates read as 0 instead of failing the queue.
func TestAgeDays(t *testing.T) {
	jakarta, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Skip("tzdata unavailable: " + err.Error())
	}
	today := time.Date(2026, 9, 11, 10, 0, 0, 0, jakarta)
	cases := []struct {
		name  string
		input string
		want  int
	}{
		{"today", "2026-09-11 08:00:00", 0},
		{"three days", "2026-09-08 17:30:00", 3},
		{"eight days", "2026-09-03 09:00:00", 8},
		{"date only", "2026-09-01", 10},
		{"empty", "", 0},
		{"garbage", "kapan-kapan", 0},
	}
	for _, c := range cases {
		if got := ageDays(c.input, today); got != c.want {
			t.Errorf("%s: ageDays(%q) = %d, want %d", c.name, c.input, got, c.want)
		}
	}
}
