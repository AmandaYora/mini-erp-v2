package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
	"mini-erp/internal/shared/timeutil"
)

// Intents, first-match-wins (legacy order, minus policy_qna: no knowledge
// in L9 scope — SOP lookup does not exist, so the intent is gone, not stubbed).
const (
	IntentHelp               = "help"
	IntentOperationalSummary = "operational_summary"
	IntentSalesSummary       = "sales_summary"
	IntentPendingOrders      = "pending_orders"
	IntentOrderStatus        = "order_status"
	IntentCriticalStock      = "critical_stock"
	IntentTodayPerformance   = "today_performance"
	IntentProductInfo        = "product_info"
)

// ToolCall is one recorded tool invocation (persisted to tool executions).
type ToolCall struct {
	Tool       string
	Input      any
	Output     any
	DurationMs int64
}

// Engine answers in Indonesian plain text (WhatsApp-safe: no markdown
// tables, no links). All tools are read-only.
type Engine struct {
	branches   branchcontracts.BranchClient
	products   productcontracts.ProductClient
	sales      salescontracts.SalesOrderClient
	purchasing purchasingcontracts.PurchaseOrderClient
	stock      stockcontracts.StockClient
	finance    financecontracts.FinanceClient
}

// NewEngine wires the intent engine over provider contracts.
func NewEngine(
	branches branchcontracts.BranchClient,
	products productcontracts.ProductClient,
	sales salescontracts.SalesOrderClient,
	purchasing purchasingcontracts.PurchaseOrderClient,
	stock stockcontracts.StockClient,
	finance financecontracts.FinanceClient,
) *Engine {
	return &Engine{branches: branches, products: products, sales: sales,
		purchasing: purchasing, stock: stock, finance: finance}
}

// Detection is one intent plus an optional argument (order number or
// product query). Empty text falls back to help (legacy E-03).
type Detection struct {
	Intent string
	Arg    string
}

// Detect classifies raw text. Order matters — first match wins.
func Detect(text string) Detection {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return Detection{Intent: IntentHelp}
	}
	if hasAny(t, "/help", "bantuan", "cara pakai", "mulai", "menu", "help") {
		return Detection{Intent: IntentHelp}
	}
	if num := findOrderNumber(text); num != "" {
		return Detection{Intent: IntentOrderStatus, Arg: num}
	}
	if hasAny(t, "ringkasan", "rekap", "operasional") {
		return Detection{Intent: IntentOperationalSummary}
	}
	if hasAny(t, "hari ini", "performa", "kinerja", "bagaimana hari") {
		return Detection{Intent: IntentTodayPerformance}
	}
	if hasAny(t, "omzet", "omset", "penjualan", "laba", "pendapatan", "rugi") {
		return Detection{Intent: IntentSalesSummary}
	}
	if hasAny(t, "pending", "tertunda", "menunggu", "belum bayar", "belum lunas",
		"jatuh tempo", "hutang", "piutang", "tagihan") {
		return Detection{Intent: IntentPendingOrders}
	}
	if hasAny(t, "status", "cek order", "cek po", "cek so", "lacak", "di mana", "dimana", "sampai mana") {
		return Detection{Intent: IntentOrderStatus}
	}
	if hasAny(t, "kritis", "menipis", "habis", "restock", "restok", "kurang") ||
		t == "stok" || strings.HasSuffix(t, " stok") {
		return Detection{Intent: IntentCriticalStock}
	}
	if q := trailingToken(t, "info", "produk", "barang", "harga", "stok", "cari"); q != "" {
		return Detection{Intent: IntentProductInfo, Arg: q}
	}
	return Detection{Intent: IntentHelp}
}

func hasAny(t string, words ...string) bool {
	for _, w := range words {
		if strings.Contains(t, w) {
			return true
		}
	}
	return false
}

// findOrderNumber extracts a "ORD-…" token (our numbering shape).
func findOrderNumber(text string) string {
	for _, f := range strings.Fields(text) {
		u := strings.ToUpper(strings.Trim(f, ".,;:!?()\"'"))
		if strings.HasPrefix(u, "ORD-") && len(u) > 8 {
			return u
		}
	}
	return ""
}

// trailingToken returns the token after a leading keyword ("info H-001").
func trailingToken(t string, keywords ...string) string {
	fields := strings.Fields(t)
	if len(fields) < 2 {
		return ""
	}
	for _, k := range keywords {
		if fields[0] == k {
			return strings.Join(fields[1:], " ")
		}
	}
	return ""
}

// dateRange parses Indonesian range words (legacy F-01.3). Default: today.
// All bounds are WIB calendar days (YYYY-MM-DD).
func dateRange(text string) (from, to, label string) {
	today := timeutil.NowUTC().In(timeutil.Jakarta)
	t := strings.ToLower(text)
	day := today.Format("2006-01-02")
	switch {
	case strings.Contains(t, "kemarin"):
		y := today.AddDate(0, 0, -1).Format("2006-01-02")
		return y, y, "kemarin"
	case hasAny(t, "minggu ini", "7 hari", "seminggu"):
		return today.AddDate(0, 0, -6).Format("2006-01-02"), day, "7 hari terakhir"
	case hasAny(t, "bulan ini", "sebulan"):
		return today.Format("2006-01") + "-01", day, "bulan ini"
	default:
		return day, day, "hari ini"
	}
}

// recorder stamps per-tool durations within one Execute (single goroutine,
// so no locking needed despite the shared Engine).
type recorder struct {
	t0    time.Time
	calls []ToolCall
}

func newRecorder() *recorder {
	return &recorder{t0: time.Now()}
}

func (r *recorder) add(name string, input, output any) {
	r.calls = append(r.calls, ToolCall{
		Tool: name, Input: input, Output: output,
		DurationMs: time.Since(r.t0).Milliseconds(),
	})
}

// Execute runs detection through tools. branchID 0 = whole company (active
// branches). It returns the intent, the reply text, and the tool calls made
// (help makes none). Tool errors abort with a friendly message.
func (e *Engine) Execute(ctx context.Context, branchID int64, text string) (intent, answer string, calls []ToolCall, err error) {
	d := Detect(text)
	branches, err := e.scopeBranches(ctx, branchID)
	if err != nil {
		return d.Intent, "", nil, err
	}
	rec := newRecorder()
	switch d.Intent {
	case IntentHelp:
		return d.Intent, helpText(), nil, nil
	case IntentOperationalSummary:
		from, to, label := dateRange(text)
		ans, err := e.operationalSummary(ctx, branches, from, to, label, rec)
		return d.Intent, ans, rec.calls, err
	case IntentSalesSummary:
		from, to, label := dateRange(text)
		ans, err := e.salesSummary(ctx, branches, from, to, label, rec)
		return d.Intent, ans, rec.calls, err
	case IntentPendingOrders:
		ans, err := e.pendingOrders(ctx, branches, rec)
		return d.Intent, ans, rec.calls, err
	case IntentOrderStatus:
		ans, err := e.orderStatus(ctx, branches, d.Arg, rec)
		return d.Intent, ans, rec.calls, err
	case IntentCriticalStock:
		ans, err := e.criticalStock(ctx, branches, rec)
		return d.Intent, ans, rec.calls, err
	case IntentTodayPerformance:
		ans, err := e.todayPerformance(ctx, branches, rec)
		return d.Intent, ans, rec.calls, err
	case IntentProductInfo:
		ans, err := e.productInfo(ctx, branches, d.Arg, rec)
		return d.Intent, ans, rec.calls, err
	default:
		return IntentHelp, helpText(), nil, nil
	}
}

// BranchLabel resolves the answer scope label ("Semua cabang" for 0).
// A miss reads as the raw id — never fail an answer over a label.
func (e *Engine) BranchLabel(ctx context.Context, branchID int64) string {
	if branchID == 0 {
		return "Semua cabang"
	}
	all, err := e.branches.ListAll(ctx)
	if err != nil {
		return fmt.Sprintf("Cabang %d", branchID)
	}
	for _, b := range all {
		if b.ID == branchID {
			return b.Name
		}
	}
	return fmt.Sprintf("Cabang %d", branchID)
}

// CheckBranch rejects unknown explicit branches before a run is recorded
// (console branch picker only lists real branches; anything else is a
// tampered request, not a chat failure).
func (e *Engine) CheckBranch(ctx context.Context, branchID int64) error {
	if branchID == 0 {
		return nil
	}
	_, err := e.scopeBranches(ctx, branchID)
	return err
}

// scopeBranches resolves the working set: one explicit branch or all active.
func (e *Engine) scopeBranches(ctx context.Context, branchID int64) ([]*branchcontracts.Branch, error) {
	all, err := e.branches.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	if branchID == 0 {
		out := []*branchcontracts.Branch{}
		for _, b := range all {
			if b.Status == "active" {
				out = append(out, b)
			}
		}
		return out, nil
	}
	for _, b := range all {
		if b.ID == branchID {
			return []*branchcontracts.Branch{b}, nil
		}
	}
	return nil, apperror.NotFound("Cabang")
}

func helpText() string {
	return "Halo! Saya asisten operasional. Tanya saya, misalnya:\n" +
		"- omzet hari ini / omzet bulan ini\n" +
		"- order pending / tagihan / hutang\n" +
		"- status ORD-UTM/PJ/2026/09/00001\n" +
		"- stok kritis\n" +
		"- info H-001"
}

// --- tools --------------------------------------------------------------------

// salesSummary reports revenue/profit (+count/AOV) per branch for a range.
func (e *Engine) salesSummary(ctx context.Context, branches []*branchcontracts.Branch, from, to, label string, rec *recorder) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Omzet %s:\n", label)
	for _, br := range branches {
		rev, _, profit, err := e.finance.NetProfit(ctx, br.ID, from, to)
		if err != nil {
			return "", err
		}
		rows, err := e.sales.ListOrders(ctx, br.ID, nil, from, to, 50)
		if err != nil {
			return "", err
		}
		rec.add("sales_summary", map[string]any{"branch": br.Code, "from": from, "to": to},
			map[string]any{"revenue": rev, "profit": profit, "orders": len(rows)})
		aov := int64(0)
		if len(rows) > 0 {
			aov = rev / int64(len(rows))
		}
		more := ""
		if len(rows) == 50 {
			more = " (50+)"
		}
		fmt.Fprintf(&b, "- %s: omzet %s, laba %s, %d order%s, rata-rata %s\n",
			br.Name, fmtIDR(rev), fmtIDR(profit), len(rows), more, fmtIDR(aov))
	}
	return b.String(), nil
}

// pendingOrders lists open orders (confirmed first) per branch, top 5 each.
func (e *Engine) pendingOrders(ctx context.Context, branches []*branchcontracts.Branch, rec *recorder) (string, error) {
	var b strings.Builder
	b.WriteString("Order pending:\n")
	empty := true
	for _, br := range branches {
		so, err := e.sales.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 5)
		if err != nil {
			return "", err
		}
		po, err := e.purchasing.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 5)
		if err != nil {
			return "", err
		}
		rec.add("pending_orders", map[string]any{"branch": br.Code},
			map[string]any{"sales": len(so), "purchase": len(po)})
		if len(so)+len(po) == 0 {
			continue
		}
		empty = false
		fmt.Fprintf(&b, "%s:\n", br.Name)
		for _, o := range so {
			fmt.Fprintf(&b, "- %s (%s, %s) %s\n", o.Number, orderStatusID(o.Status), partyOrTunai(o), fmtIDR(o.GrandTotal))
		}
		for _, o := range po {
			fmt.Fprintf(&b, "- %s (beli, %s, %s) %s\n", o.Number, orderStatusID(o.Status), o.PartyName, fmtIDR(o.GrandTotal))
		}
	}
	if empty {
		return "Tidak ada order pending. Semua beres.", nil
	}
	return b.String(), nil
}

// orderStatus resolves one document by exact number across both flows.
func (e *Engine) orderStatus(ctx context.Context, branches []*branchcontracts.Branch, number string, rec *recorder) (string, error) {
	if strings.TrimSpace(number) == "" {
		return "Kirim nomor ordernya, contoh: status ORD-UTM/PJ/2026/09/00001", nil
	}
	for _, br := range branches {
		if o, err := e.sales.GetByNumber(ctx, br.ID, number); err != nil {
			return "", err
		} else if o != nil {
			rec.add("order_status", map[string]any{"number": number, "flow": "sales"},
				map[string]any{"status": o.Status, "total": o.GrandTotal})
			return fmt.Sprintf("%s (%s)\nStatus: %s\nCustomer: %s\nTanggal: %s\nTotal: %s",
				o.Number, br.Name, orderStatusID(o.Status), partyOrTunaiSales(o),
				shortDate(o.OrderDate), fmtIDR(o.GrandTotal)), nil
		}
		if o, err := e.purchasing.GetByNumber(ctx, br.ID, number); err != nil {
			return "", err
		} else if o != nil {
			rec.add("order_status", map[string]any{"number": number, "flow": "purchase"},
				map[string]any{"status": o.Status, "total": o.GrandTotal})
			return fmt.Sprintf("%s (%s)\nStatus: %s\nSupplier: %s\nTanggal: %s\nTotal: %s",
				o.Number, br.Name, orderStatusID(o.Status), o.PartyName,
				shortDate(o.OrderDate), fmtIDR(o.GrandTotal)), nil
		}
	}
	rec.add("order_status", map[string]any{"number": number}, map[string]any{"found": false})
	return fmt.Sprintf("Order %s tidak ditemukan. Cek lagi nomornya.", number), nil
}

// criticalStock lists scarcest tracked products per branch (max 5 shown).
func (e *Engine) criticalStock(ctx context.Context, branches []*branchcontracts.Branch, rec *recorder) (string, error) {
	var b strings.Builder
	b.WriteString("Stok kritis:\n")
	empty := true
	for _, br := range branches {
		rows, err := e.stock.CriticalStock(ctx, br.ID, 5)
		if err != nil {
			return "", err
		}
		rec.add("critical_stock", map[string]any{"branch": br.Code}, map[string]any{"items": len(rows)})
		if len(rows) == 0 {
			continue
		}
		empty = false
		fmt.Fprintf(&b, "%s:\n", br.Name)
		for _, r := range rows {
			fmt.Fprintf(&b, "- %s %s: sisa %s (min %s)\n",
				r.ProductCode, r.ProductName, fmtQty(r.Available), fmtQty(r.MinStock))
		}
	}
	if empty {
		return "Tidak ada stok kritis. Semua di atas minimum.", nil
	}
	return b.String(), nil
}

// todayPerformance composes today's sales + pending + critical counts.
func (e *Engine) todayPerformance(ctx context.Context, branches []*branchcontracts.Branch, rec *recorder) (string, error) {
	today := timeutil.NowUTC().In(timeutil.Jakarta).Format("2006-01-02")
	var b strings.Builder
	b.WriteString("Performa hari ini:\n")
	for _, br := range branches {
		rev, _, profit, err := e.finance.NetProfit(ctx, br.ID, today, today)
		if err != nil {
			return "", err
		}
		so, err := e.sales.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 50)
		if err != nil {
			return "", err
		}
		po, err := e.purchasing.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 50)
		if err != nil {
			return "", err
		}
		crit, err := e.stock.CriticalStock(ctx, br.ID, 50)
		if err != nil {
			return "", err
		}
		rec.add("today_performance", map[string]any{"branch": br.Code, "date": today},
			map[string]any{"revenue": rev, "profit": profit, "pending": len(so) + len(po), "critical": len(crit)})
		fmt.Fprintf(&b, "- %s: omzet %s, laba %s, %d order pending, %d produk kritis\n",
			br.Name, fmtIDR(rev), fmtIDR(profit), len(so)+len(po), len(crit))
	}
	return b.String(), nil
}

// operationalSummary composes range sales + pending + critical.
func (e *Engine) operationalSummary(ctx context.Context, branches []*branchcontracts.Branch, from, to, label string, rec *recorder) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Ringkasan operasional %s:\n", label)
	for _, br := range branches {
		rev, exp, profit, err := e.finance.NetProfit(ctx, br.ID, from, to)
		if err != nil {
			return "", err
		}
		so, err := e.sales.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 50)
		if err != nil {
			return "", err
		}
		po, err := e.purchasing.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 50)
		if err != nil {
			return "", err
		}
		crit, err := e.stock.CriticalStock(ctx, br.ID, 50)
		if err != nil {
			return "", err
		}
		rec.add("operational_summary", map[string]any{"branch": br.Code, "from": from, "to": to},
			map[string]any{"revenue": rev, "expense": exp, "profit": profit})
		fmt.Fprintf(&b, "- %s: omzet %s, beban %s, laba %s, %d order pending, %d produk kritis\n",
			br.Name, fmtIDR(rev), fmtIDR(exp), fmtIDR(profit), len(so)+len(po), len(crit))
	}
	return b.String(), nil
}

// productInfo resolves exact code first, then name contains (KI-140: fixed
// order, no guessing). Shows catalog price + total branch stock.
func (e *Engine) productInfo(ctx context.Context, branches []*branchcontracts.Branch, query string, rec *recorder) (string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return "Sebutkan kode atau nama produknya, contoh: info H-001", nil
	}
	p, err := e.products.GetByCode(ctx, strings.ToUpper(q))
	if err != nil {
		return "", err
	}
	if p == nil {
		stocked, err := e.products.ListStocked(ctx)
		if err != nil {
			return "", err
		}
		lq := strings.ToLower(q)
		for _, s := range stocked {
			if strings.Contains(strings.ToLower(s.Name), lq) || strings.Contains(strings.ToLower(s.Code), lq) {
				p, err = e.products.GetByID(ctx, s.ID)
				if err != nil {
					return "", err
				}
				break
			}
		}
	}
	if p == nil {
		rec.add("product_info", map[string]any{"query": q}, map[string]any{"found": false})
		return fmt.Sprintf("Produk \"%s\" tidak ditemukan di katalog.", q), nil
	}
	rec.add("product_info", map[string]any{"query": q}, map[string]any{"code": p.Code})
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\nHarga jual %s\n", p.Code, p.Name, fmtIDR(p.SellingPrice))
	for _, br := range branches {
		positions, _, err := e.finance.InventoryValue(ctx, br.ID)
		if err != nil {
			return "", err
		}
		var qty float64
		for _, pos := range positions {
			if pos.ProductID == p.ID {
				qty += pos.Qty
			}
		}
		fmt.Fprintf(&b, "- %s: stok %s\n", br.Name, fmtQty(qty))
	}
	return b.String(), nil
}

// --- formatting (WhatsApp plain text, id-ID) -----------------------------------

// fmtIDR renders integer rupiah with dot thousands ("Rp 15.500", "-Rp 1.000").
func fmtIDR(n int64) string {
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	var out []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	if neg {
		return "-Rp " + string(out)
	}
	return "Rp " + string(out)
}

// fmtQty renders quantities without trailing zeros (up to 3 decimals).
func fmtQty(f float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.3f", f), "0"), ".")
}

// shortDate cuts "2006-01-02 15:04:05" to the date part.
func shortDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// orderStatusID labels document states in Indonesian.
func orderStatusID(s string) string {
	switch s {
	case "draft":
		return "draf"
	case "confirmed":
		return "terkonfirmasi"
	case "completed":
		return "selesai"
	case "cancelled":
		return "dibatalkan"
	default:
		return s
	}
}

func partyOrTunai(o *salescontracts.OrderSummary) string {
	if o.PartyName != "" {
		return o.PartyName
	}
	if o.PartyID == 0 {
		return "Tunai"
	}
	return "-"
}

func partyOrTunaiSales(o *salescontracts.SalesOrder) string {
	if o.PartyName != "" {
		return o.PartyName
	}
	if o.PartyID == 0 {
		return "Tunai"
	}
	return "-"
}
