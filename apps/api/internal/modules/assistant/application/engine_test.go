package application

import "testing"

func TestDetect(t *testing.T) {
	cases := []struct {
		in     string
		intent string
		arg    string
	}{
		{in: "", intent: IntentHelp},
		{in: "bantuan", intent: IntentHelp},
		{in: "/help", intent: IntentHelp},
		{in: "ringkasan operasional", intent: IntentOperationalSummary},
		{in: "rekap bulan ini", intent: IntentOperationalSummary},
		{in: "omzet hari ini", intent: IntentTodayPerformance},
		{in: "omzet bulan ini", intent: IntentSalesSummary},
		{in: "laba kemarin", intent: IntentSalesSummary},
		{in: "order pending", intent: IntentPendingOrders},
		{in: "tagihan", intent: IntentPendingOrders},
		{in: "hutang supplier", intent: IntentPendingOrders},
		{in: "status ORD-UTM/PJ/2026/09/00001", intent: IntentOrderStatus, arg: "ORD-UTM/PJ/2026/09/00001"},
		{in: "cek order", intent: IntentOrderStatus},
		{in: "stok kritis", intent: IntentCriticalStock},
		{in: "barang habis apa saja", intent: IntentCriticalStock},
		{in: "stok", intent: IntentCriticalStock},
		{in: "info H-001", intent: IntentProductInfo, arg: "h-001"},
		{in: "harga beras", intent: IntentProductInfo, arg: "beras"},
		{in: "stok H-001", intent: IntentProductInfo, arg: "h-001"},
		{in: "halo apa kabar", intent: IntentHelp},
	}
	for _, c := range cases {
		d := Detect(c.in)
		if d.Intent != c.intent || (c.arg != "" && d.Arg != c.arg) {
			t.Errorf("Detect(%q) = (%q, %q), want (%q, %q)",
				c.in, d.Intent, d.Arg, c.intent, c.arg)
		}
	}
}

func TestFmtIDR(t *testing.T) {
	cases := map[int64]string{
		0:       "Rp 0",
		15500:   "Rp 15.500",
		1000000: "Rp 1.000.000",
		-93000:  "-Rp 93.000",
		5:       "Rp 5",
	}
	for in, want := range cases {
		if got := fmtIDR(in); got != want {
			t.Errorf("fmtIDR(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestDateRange(t *testing.T) {
	from, to, label := dateRange("omzet")
	if from != to || label != "hari ini" {
		t.Errorf("default range = %q..%q %q, want today", from, to, label)
	}
	_, _, label = dateRange("omzet bulan ini")
	if label != "bulan ini" {
		t.Errorf("month label = %q", label)
	}
	_, _, label = dateRange("rekap 7 hari")
	if label != "7 hari terakhir" {
		t.Errorf("week label = %q", label)
	}
	_, _, label = dateRange("kemarin")
	if label != "kemarin" {
		t.Errorf("yesterday label = %q", label)
	}
}
