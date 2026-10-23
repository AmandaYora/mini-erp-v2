package application

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"time"

	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/health"
	"mini-erp/internal/shared/money"
	"mini-erp/internal/shared/timeutil"
)

// The data-health alert catalog (ADR-0025, spec §1). Owning modules only count
// facts; everything a person reads — severity, title, impact, action — is
// decided here. An id is permanent (it keys snoozes): change a definition by
// adding a new id, never by renaming one.
//
// A3 finance.posting_backlog is deferred to phase 2 (owner's decision): it
// needs a read-only variant of the posting queue.

// Alert ids.
const (
	AlertSeedPassword      = "security.seed_password"
	AlertCostAbovePrice    = "inventory.cost_above_price"
	AlertUnbalancedJournal = "finance.unbalanced_journal"
	AlertPeriodUnclosed    = "finance.period_unclosed"
	AlertNegativeStock     = "stock.negative"
	AlertOpeningUnverified = "inventory.opening_unverified"
	AlertMinPrice          = "master.min_price_above_selling"
	AlertDuplicateProduct  = "master.duplicate_product_name"
	AlertDuplicateCustomer = "master.duplicate_customer_name"
	AlertDeliveryUnsigned  = "delivery.unsigned"
	AlertPaidUnshipped     = "sales.paid_unshipped"
	AlertOpsIntegrity      = "ops.integrity"
	AlertOrdersOverdue     = "ops.orders_overdue"
	AlertStockCritical     = "ops.stock_critical"
)

// costRatio is A2's threshold: ledger cost above 1.2 × selling price.
const costRatio = 1.2

// sampleSize is how many example rows an alert carries.
const sampleSize = 5

var areaLabels = map[string]string{
	dashboardcontracts.AreaFinance:  "Keuangan",
	dashboardcontracts.AreaStock:    "Persediaan",
	dashboardcontracts.AreaMaster:   "Master",
	dashboardcontracts.AreaDocument: "Dokumen",
	dashboardcontracts.AreaSecurity: "Keamanan",
}

// areaOrder is the fixed area order of the data-health page.
var areaOrder = []string{
	dashboardcontracts.AreaFinance, dashboardcontracts.AreaStock, dashboardcontracts.AreaMaster,
	dashboardcontracts.AreaDocument, dashboardcontracts.AreaSecurity,
}

// viewer is what a check may tailor to the session reading it.
type viewer struct {
	userID   int64
	roleCode string
	security bool
}

// checkDef defines one data-health check and the alert it raises.
type checkDef struct {
	id, label, area, tier  string
	title, impact          string
	countUnit, amountLabel string
	run                    func(ctx context.Context, h *Health, branchID int64, now time.Time) (health.Result, bool, error)
	severity               func(r health.Result, now time.Time) string
	action                 func(r health.Result, v viewer) (*dashboardcontracts.AlertAction, string)
	items                  func(ctx context.Context, h *Health, branchID int64, page, limit int) ([]health.Item, int64, error)
	snoozable              bool
}

func fixed(sev string) func(health.Result, time.Time) string {
	return func(health.Result, time.Time) string { return sev }
}

func goTo(label, to, perm string) func(health.Result, viewer) (*dashboardcontracts.AlertAction, string) {
	return func(health.Result, viewer) (*dashboardcontracts.AlertAction, string) {
		return &dashboardcontracts.AlertAction{Label: label, To: to, Perm: perm}, ""
	}
}

// olderThan reports whether the result's OldestAt (YYYY-MM-DD, WIB) lies more
// than days calendar days before today (WIB).
func olderThan(r health.Result, now time.Time, days int) bool {
	if len(r.OldestAt) < 10 {
		return false
	}
	day, err := time.ParseInLocation("2006-01-02", r.OldestAt[:10], timeutil.Jakarta)
	if err != nil {
		return false
	}
	today := now.In(timeutil.Jakarta)
	limit := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, timeutil.Jakarta).AddDate(0, 0, -days)
	return day.Before(limit)
}

// catalog lists every check in a stable order.
func catalog() []checkDef {
	return []checkDef{
		{
			id: AlertSeedPassword, label: "Sandi bawaan superadmin", area: dashboardcontracts.AreaSecurity, tier: TierFast,
			title:  "Superadmin masih memakai sandi bawaan",
			impact: "Siapa pun yang tahu sandi bawaan bisa masuk dengan akses penuh.", countUnit: "akun",
			run:      runSeedPassword,
			severity: fixed(dashboardcontracts.SeverityCritical),
			action: func(r health.Result, v viewer) (*dashboardcontracts.AlertAction, string) {
				// Superadmin holders are hidden from every other session
				// (user module), so only a superadmin can open the account.
				if v.roleCode == "superadmin" && len(r.Sample) > 0 {
					return &dashboardcontracts.AlertAction{Label: "Ganti sandi",
						To: "/users?search=" + url.QueryEscape(r.Sample[0].Label), Perm: "users.view"}, ""
				}
				return nil, "Minta pemegang akun superadmin login lalu mengganti sandinya."
			},
		},
		{
			id: AlertCostAbovePrice, label: "Harga modal vs harga jual", area: dashboardcontracts.AreaStock, tier: TierFast,
			title:     "Harga modal > 1,2× harga jual",
			impact:    "Nilai persediaan membengkak dan HPP salah setiap kali produk ini terjual.",
			countUnit: "produk", amountLabel: "nilai persediaan",
			run: func(ctx context.Context, h *Health, branchID int64, _ time.Time) (health.Result, bool, error) {
				r, err := h.src.Finance.CostAbovePrice(ctx, branchID, costRatio, sampleSize)
				return r, true, err
			},
			severity: fixed(dashboardcontracts.SeverityCritical),
			action: func(r health.Result, _ viewer) (*dashboardcontracts.AlertAction, string) {
				return &dashboardcontracts.AlertAction{Label: fmt.Sprintf("Tinjau %d produk", r.Count),
					To: "/data-health?alert=" + AlertCostAbovePrice, Perm: dashboardcontracts.PermAlertsViewAll}, ""
			},
			items: func(ctx context.Context, h *Health, branchID int64, page, limit int) ([]health.Item, int64, error) {
				return h.src.Finance.CostAbovePriceItems(ctx, branchID, costRatio, page, limit)
			},
			snoozable: true,
		},
		{
			id: AlertUnbalancedJournal, label: "Keseimbangan jurnal", area: dashboardcontracts.AreaFinance, tier: TierHeavy,
			title: "Jurnal tidak seimbang", impact: "Neraca tidak akan seimbang dan tutup bulan diblokir.", countUnit: "jurnal",
			run:       runUnbalanced,
			severity:  fixed(dashboardcontracts.SeverityCritical),
			action:    goTo("Periksa integritas", "/finance/integrity", "finance.manage"),
			snoozable: true,
		},
		{
			id: AlertPeriodUnclosed, label: "Periode yang belum ditutup", area: dashboardcontracts.AreaFinance, tier: TierFast,
			title:  "Periode lewat belum ditutup",
			impact: "Dokumen bertanggal mundur masih bisa mengubah laba bulan lalu tanpa tanda.", countUnit: "periode",
			run: runPeriods,
			severity: func(r health.Result, now time.Time) string {
				// Closing last month in its first half is routine, not a problem.
				today := now.In(timeutil.Jakarta)
				prev := today.AddDate(0, 0, -today.Day())
				if r.Count == 1 && r.Meta["latestYM"] == int64(prev.Year()*100+int(prev.Month())) && today.Day() <= 15 {
					return dashboardcontracts.SeverityInfo
				}
				return dashboardcontracts.SeverityWarning
			},
			action:    goTo("Tutup periode", "/finance/periods", "finance.manage"),
			snoozable: true,
		},
		{
			id: AlertNegativeStock, label: "Stok negatif", area: dashboardcontracts.AreaStock, tier: TierFast,
			title:  "Stok negatif",
			impact: "Stok tercatat di bawah nol: HPP dan ketersediaan di POS tidak bisa dipercaya.", countUnit: "posisi",
			run: func(ctx context.Context, h *Health, branchID int64, _ time.Time) (health.Result, bool, error) {
				r, err := h.src.Stock.NegativeSummary(ctx, branchID, sampleSize)
				return r, true, err
			},
			severity:  fixed(dashboardcontracts.SeverityCritical),
			action:    goTo("Lihat saldo stok", "/stock/balances?negative=1", "stock.view"),
			snoozable: true,
		},
		{
			id: AlertOpeningUnverified, label: "Stok awal belum diopname", area: dashboardcontracts.AreaStock, tier: TierFast,
			title:     "Stok awal belum pernah diopname",
			impact:    "Stok di POS dan nilai persediaan di neraca untuk produk ini kemungkinan besar bukan stok fisik.",
			countUnit: "produk", amountLabel: "nilai persediaan",
			run: func(ctx context.Context, h *Health, branchID int64, _ time.Time) (health.Result, bool, error) {
				r, err := h.src.Finance.OpeningUnverified(ctx, branchID, sampleSize)
				return r, true, err
			},
			severity: fixed(dashboardcontracts.SeverityWarning),
			action: func(r health.Result, _ viewer) (*dashboardcontracts.AlertAction, string) {
				return &dashboardcontracts.AlertAction{Label: fmt.Sprintf("Tinjau %d produk", r.Count),
					To: "/data-health?alert=" + AlertOpeningUnverified, Perm: dashboardcontracts.PermAlertsViewAll}, ""
			},
			items: func(ctx context.Context, h *Health, branchID int64, page, limit int) ([]health.Item, int64, error) {
				return h.src.Finance.OpeningUnverifiedItems(ctx, branchID, page, limit)
			},
			snoozable: true,
		},
		{
			id: AlertMinPrice, label: "Harga minimum vs harga jual", area: dashboardcontracts.AreaMaster, tier: TierFast,
			title:  "Harga minimum di atas harga jual",
			impact: "Kasir tidak bisa menjual dengan harga katalog dan harus mengetik harga manual.", countUnit: "produk",
			run: func(ctx context.Context, h *Health, _ int64, _ time.Time) (health.Result, bool, error) {
				r, err := h.src.Products.MinPriceAboveSelling(ctx, sampleSize)
				return r, true, err
			},
			severity:  fixed(dashboardcontracts.SeverityWarning),
			action:    goTo("Perbaiki harga", "/products?priceIssue=min_above_selling", "products.view"),
			snoozable: true,
		},
		{
			id: AlertDuplicateProduct, label: "Nama produk ganda", area: dashboardcontracts.AreaMaster, tier: TierFast,
			title: "Nama produk ganda", impact: "Kasir bisa memilih produk yang salah.", countUnit: "grup",
			run: func(ctx context.Context, h *Health, _ int64, _ time.Time) (health.Result, bool, error) {
				r, err := h.src.Products.DuplicateNames(ctx, sampleSize)
				return r, true, err
			},
			severity:  fixed(dashboardcontracts.SeverityInfo),
			action:    goTo("Tinjau duplikat", "/products?duplicateName=1", "products.view"),
			snoozable: true,
		},
		{
			id: AlertDuplicateCustomer, label: "Nama pelanggan ganda", area: dashboardcontracts.AreaMaster, tier: TierFast,
			title: "Nama pelanggan ganda", impact: "Piutang pelanggan yang sama terpecah ke beberapa kode.", countUnit: "grup",
			run: func(ctx context.Context, h *Health, _ int64, _ time.Time) (health.Result, bool, error) {
				r, err := h.src.Parties.DuplicateNames(ctx, "customer", sampleSize)
				return r, true, err
			},
			severity:  fixed(dashboardcontracts.SeverityInfo),
			action:    goTo("Tinjau duplikat", "/customers?duplicateName=1", "customers.view"),
			snoozable: true,
		},
		{
			id: AlertDeliveryUnsigned, label: "Tanda tangan surat jalan", area: dashboardcontracts.AreaDocument, tier: TierFast,
			title:     "SJ terkirim belum bertanda tangan",
			impact:    "Tanpa SJ bertanda tangan, bukti serah terima lemah bila pelanggan menyanggah.",
			countUnit: "surat jalan", amountLabel: "nilai order terkait",
			run: func(ctx context.Context, h *Health, branchID int64, _ time.Time) (health.Result, bool, error) {
				r, err := h.src.Delivery.UnsignedSummary(ctx, branchID, sampleSize)
				return r, true, err
			},
			severity: func(r health.Result, now time.Time) string {
				if olderThan(r, now, 3) {
					return dashboardcontracts.SeverityWarning
				}
				return dashboardcontracts.SeverityInfo
			},
			action:    goTo("Lihat surat jalan", "/deliveries?status=confirmed&signature=none", "delivery.view"),
			snoozable: true,
		},
		{
			id: AlertPaidUnshipped, label: "Order dibayar belum dikirim", area: dashboardcontracts.AreaDocument, tier: TierFast,
			title:     "Sudah dibayar, barang belum dikirim",
			impact:    "Uang pelanggan sudah diterima tetapi barang belum keluar; risiko keluhan atau pengembalian dana.",
			countUnit: "order", amountLabel: "uang diterima",
			run: runPaidUnshipped,
			severity: func(r health.Result, now time.Time) string {
				if olderThan(r, now, 7) {
					return dashboardcontracts.SeverityWarning
				}
				return dashboardcontracts.SeverityInfo
			},
			action:    goTo("Tinjau order", "/sales-orders?status=confirmed&paymentStatus=received&sort=date&dir=asc", "sales.view"),
			snoozable: true,
		},
		{
			id: AlertOpsIntegrity, label: "Konsistensi dokumen", area: dashboardcontracts.AreaDocument, tier: TierHeavy,
			title:     "Dokumen terhenti di tengah proses",
			impact:    "Stok atau status dokumen tidak sinkron sampai operator mengulang atau membatalkan aksinya.",
			countUnit: "dokumen",
			run:       runIntegrity,
			severity:  fixed(dashboardcontracts.SeverityCritical),
			action:    goTo("Buka konsistensi dokumen", "/ops/integrity", "stock.update"),
			snoozable: true,
		},
		{
			id: AlertOrdersOverdue, label: "Order lewat jatuh tempo", area: dashboardcontracts.AreaDocument, tier: TierFast,
			title:  "Order lewat jatuh tempo",
			impact: "Piutang/hutang lewat jatuh tempo belum diselesaikan.", countUnit: "order", amountLabel: "nilai order",
			run:       runOrdersOverdue,
			severity:  fixed(dashboardcontracts.SeverityWarning),
			action:    goTo("Lihat order", "/sales-orders?status=confirmed&paymentStatus=overdue", "sales.view"),
			snoozable: true,
		},
		{
			id: AlertStockCritical, label: "Stok di bawah minimum", area: dashboardcontracts.AreaStock, tier: TierFast,
			title: "Stok di bawah minimum", impact: "Barang laris bisa habis sebelum dipesan ulang.", countUnit: "produk",
			run:       runStockCritical,
			severity:  fixed(dashboardcontracts.SeverityWarning),
			action:    goTo("Lihat stok kritis", "/reporting/stock?critical=1", "reporting.view"),
			snoozable: true,
		},
	}
}

// runSeedPassword is A1: ready=false while the user module still compares.
func runSeedPassword(ctx context.Context, h *Health, _ int64, _ time.Time) (health.Result, bool, error) {
	accounts, ready, err := h.src.Users.SeedPasswordAccounts(ctx)
	if err != nil || !ready {
		return health.Result{}, ready, err
	}
	r := health.Result{Count: int64(len(accounts))}
	for _, a := range accounts[:min(sampleSize, len(accounts))] {
		r.Sample = append(r.Sample, health.Sample{Label: a.Username, Value: "sandi bawaan"})
	}
	return r, true, nil
}

var monthShort = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

func ymLabel(ym int) string {
	m := ym % 100
	if m < 1 || m > 12 {
		return fmt.Sprint(ym)
	}
	return fmt.Sprintf("%s %d", monthShort[m-1], ym/100)
}

// runPeriods is A5: OldestAt is the last day of the oldest open month.
func runPeriods(ctx context.Context, h *Health, branchID int64, now time.Time) (health.Result, bool, error) {
	today := now.In(timeutil.Jakarta)
	months, err := h.src.Finance.UnclosedPastPeriods(ctx, branchID, today.Year()*100+int(today.Month()))
	if err != nil {
		return health.Result{}, true, err
	}
	r := health.Result{Count: int64(len(months))}
	if len(months) == 0 {
		return r, true, nil
	}
	first := months[0]
	end := time.Date(first/100, time.Month(first%100)+1, 0, 0, 0, 0, 0, timeutil.Jakarta)
	r.OldestAt = end.Format("2006-01-02")
	r.Meta = map[string]int64{"oldestYM": int64(first), "latestYM": int64(months[len(months)-1])}
	for _, ym := range months[:min(sampleSize, len(months))] {
		r.Sample = append(r.Sample, health.Sample{Label: ymLabel(ym), Value: "belum ditutup"})
	}
	return r, true, nil
}

// runUnbalanced is A4 over the journals of the earliest open month onward.
func runUnbalanced(ctx context.Context, h *Health, branchID int64, now time.Time) (health.Result, bool, error) {
	today := now.In(timeutil.Jakarta)
	months, err := h.src.Finance.UnclosedPastPeriods(ctx, branchID, today.Year()*100+int(today.Month()))
	if err != nil {
		return health.Result{}, true, err
	}
	from := today.Format("2006-01") + "-01"
	if len(months) > 0 {
		from = fmt.Sprintf("%04d-%02d-01", months[0]/100, months[0]%100)
	}
	r, err := h.src.Finance.UnbalancedJournals(ctx, branchID, from, sampleSize)
	return r, true, err
}

// runPaidUnshipped is A12: the draft/confirmed orders (paged) that already
// received money, oldest first. One grouped payment read per 1.000 orders.
func runPaidUnshipped(ctx context.Context, h *Health, branchID int64, _ time.Time) (health.Result, bool, error) {
	type paidOrder struct {
		number, date string
		grand, paid  int64
	}
	var found []paidOrder
	var after int64
	for {
		rows, err := h.src.Sales.UnshippedOrderTotals(ctx, branchID, after, 1000)
		if err != nil {
			return health.Result{}, true, err
		}
		if len(rows) == 0 {
			break
		}
		ids := make([]int64, 0, len(rows))
		for _, r := range rows {
			ids = append(ids, r.ID)
		}
		paid, err := h.src.Payments.PaidForOrders(ctx, paymentcontracts.OrderSales, ids)
		if err != nil {
			return health.Result{}, true, err
		}
		for _, r := range rows {
			if p := paid[r.ID]; p > 0 {
				found = append(found, paidOrder{number: r.Number, date: r.OrderDate, grand: r.GrandTotal, paid: p})
			}
		}
		after = rows[len(rows)-1].ID
		if len(rows) < 1000 {
			break
		}
	}
	sort.SliceStable(found, func(i, j int) bool { return found[i].date < found[j].date })
	res := health.Result{Count: int64(len(found))}
	if len(found) == 0 {
		return res, true, nil
	}
	var total int64
	for _, f := range found {
		total += f.paid
	}
	res.AmountRp = health.Amount(total)
	res.OldestAt = found[0].date
	for _, f := range found[:min(sampleSize, len(found))] {
		res.Sample = append(res.Sample, health.Sample{Label: f.number,
			Value: fmt.Sprintf("dibayar %s dari %s", money.Rupiah(f.paid), money.Rupiah(f.grand))})
	}
	return res, true, nil
}

// runIntegrity is A13 through the composition root's integrity collector.
func runIntegrity(ctx context.Context, h *Health, branchID int64, _ time.Time) (health.Result, bool, error) {
	if h.src.Integrity == nil {
		return health.Result{}, true, apperror.Internal(fmt.Errorf("integrity source not wired"))
	}
	issues, err := h.src.Integrity(ctx, branchID)
	if err != nil {
		return health.Result{}, true, err
	}
	r := health.Result{Count: int64(len(issues))}
	for _, is := range issues[:min(sampleSize, len(issues))] {
		r.Sample = append(r.Sample, health.Sample{Label: is.Number, Value: is.Detail})
	}
	return r, true, nil
}

// runOrdersOverdue is A14: the "Lewat jatuh tempo" rows of the priority list
// (capped like GET /dashboard/priority-orders).
func runOrdersOverdue(ctx context.Context, h *Health, branchID int64, _ time.Time) (health.Result, bool, error) {
	items, err := h.svc.PriorityOrders(ctx, branchID, maxWidgetLimit)
	if err != nil {
		return health.Result{}, true, err
	}
	var r health.Result
	var total, sales, purchase int64
	for _, it := range items {
		if it.Reason != dashboardcontracts.ReasonOverdue {
			continue
		}
		r.Count++
		total += it.GrandTotal
		if it.Kind == "purchase" {
			purchase++
		} else {
			sales++
		}
		if due := it.DueDate; len(due) >= 10 && (r.OldestAt == "" || due[:10] < r.OldestAt) {
			r.OldestAt = due[:10]
		}
		if len(r.Sample) < sampleSize {
			r.Sample = append(r.Sample, health.Sample{Label: it.Number + " · " + it.PartyName, Value: money.Rupiah(it.GrandTotal).String()})
		}
	}
	if r.Count > 0 {
		r.AmountRp = health.Amount(total)
		r.Meta = map[string]int64{"sales": sales, "purchase": purchase}
	}
	return r, true, nil
}

// runStockCritical is A15: tracked products at/below minimum stock.
func runStockCritical(ctx context.Context, h *Health, branchID int64, _ time.Time) (health.Result, bool, error) {
	critical, _, err := h.svc.stock.CriticalSummary(ctx, branchID)
	if err != nil {
		return health.Result{}, true, err
	}
	r := health.Result{Count: critical}
	if critical == 0 {
		return r, true, nil
	}
	items, err := h.svc.stock.CriticalStock(ctx, branchID, sampleSize)
	if err != nil {
		return health.Result{}, true, err
	}
	for _, it := range items {
		r.Sample = append(r.Sample, health.Sample{Label: it.ProductCode + " " + it.ProductName,
			Value: fmt.Sprintf("tersedia %s / min %s", trimFloat(it.Available), trimFloat(it.MinStock))})
	}
	return r, true, nil
}

func trimFloat(f float64) string {
	return fmt.Sprintf("%g", f)
}
