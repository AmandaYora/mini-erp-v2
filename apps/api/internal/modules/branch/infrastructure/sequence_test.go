package infrastructure

import (
	"testing"
	"time"

	"mini-erp/internal/modules/branch/contracts"
)

// TestSequenceKeyJakartaMonth (F2a-11): the monthly counter key follows the
// Jakarta calendar date, matching the printed label. 2026-09-30 18:00 UTC
// is already 2026-10-01 01:00 WIB: the old UTC key (2026-09) with a Jakarta
// label (10) collided with the next month's counter after 07:00.
func TestSequenceKeyJakartaMonth(t *testing.T) {
	utc := func(s string) time.Time {
		tm, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	cases := []struct {
		name string
		now  time.Time
		want string
	}{
		{"batas bulan WIB", utc("2026-09-30T18:00:00Z"), "sales_order:2026-10"},
		{"batas tahun WIB", utc("2026-12-31T17:00:00Z"), "sales_order:2027-01"},
		{"siang WIB biasa", utc("2026-10-15T05:00:00Z"), "sales_order:2026-10"},
		{"pagi WIB tanggal 1 (00:30 UTC = 07:30 WIB)", utc("2026-10-01T00:30:00Z"), "sales_order:2026-10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := SequenceKey(contracts.DocSalesOrder, tc.now); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
	// Non-order kinds stay continuous (no month suffix).
	if got := SequenceKey(contracts.DocPayment, utc("2026-09-30T18:00:00Z")); got != "payment" {
		t.Fatalf("payment key = %q, want continuous", got)
	}
	if got := SequenceKey(contracts.DocGoodsReceipt, utc("2026-09-30T18:00:00Z")); got != "goods_receipt" {
		t.Fatalf("goods_receipt key = %q, want continuous", got)
	}
}
