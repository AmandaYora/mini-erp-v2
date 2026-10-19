package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mini-erp/internal/modules/assistant/contracts"
	branchcontracts "mini-erp/internal/modules/branch/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/timeutil"
)

// Intents, first-match-wins (legacy order, minus policy_qna). Knowledge/RAG is
// out of scope by product decision (PRD scope L9; ADR-0018 D1), so the intent
// is gone, not stubbed.
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
	// Status is contracts.ToolSuccess or contracts.ToolFailed; Error carries
	// the failure text of a failed call (AS-02).
	Status string
	Error  string
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
// product query). Empty text falls back to help (legacy E-03). Statuses is
// set for the order list-per-status mode (order_status without a number).
type Detection struct {
	Intent   string
	Arg      string
	Statuses []string
}

// helpWords are help-like only when they make up the whole message ("menu
// makan" must not trigger help).
var helpWords = map[string]bool{
	"/help": true, "help": true, "bantuan": true, "menu": true, "fitur": true,
	"mulai": true, "/start": true, "cara pakai": true,
}

func isHelp(t string) bool {
	trimmed := strings.Trim(t, " .,;:!?")
	if helpWords[trimmed] {
		return true
	}
	return hasAny(t, "apa saja fitur", "apa saja yang bisa", "fitur apa", "cara pakai", "bisa apa")
}

// statusWords maps Indonesian status words to R order statuses (R has no
// in_progress: "diproses/aktif" read as confirmed).
var statusWords = []struct {
	word   string
	status string
}{
	{"draft", "draft"}, {"draf", "draft"},
	{"dikonfirmasi", "confirmed"}, {"konfirmasi", "confirmed"}, {"diproses", "confirmed"},
	{"proses", "confirmed"}, {"in progress", "confirmed"}, {"aktif", "confirmed"},
	{"selesai", "completed"},
	{"dibatalkan", "cancelled"}, {"batal", "cancelled"},
}

// statusesIn returns the order statuses a message asks for (deduplicated,
// stable order) when it also names an order-like noun.
func statusesIn(t string) []string {
	if !hasAny(t, "order", "pesanan", "transaksi") {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, sw := range statusWords {
		if strings.Contains(t, sw.word) && !seen[sw.status] {
			seen[sw.status] = true
			out = append(out, sw.status)
		}
	}
	return out
}

// Detect classifies raw text. Order matters — first match wins (legacy order:
// help, summary, sales, pending, order status, critical stock, performance,
// product info; sales BEFORE performance so "penjualan hari ini" is sales).
func Detect(text string) Detection {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return Detection{Intent: IntentHelp}
	}
	if isHelp(t) {
		return Detection{Intent: IntentHelp}
	}
	if num := findOrderNumber(text); num != "" {
		return Detection{Intent: IntentOrderStatus, Arg: num}
	}
	if hasAny(t, "ringkasan", "summary", "kondisi bisnis", "rekap", "operasional") {
		return Detection{Intent: IntentOperationalSummary}
	}
	if hasAny(t, "omzet", "omset", "penjualan", "sales", "revenue", "laba", "pendapatan", "rugi") {
		return Detection{Intent: IntentSalesSummary}
	}
	if hasAny(t, "pending", "tertunda", "menunggu", "belum bayar", "belum lunas", "belum diproses",
		"belum selesai", "jatuh tempo", "hutang", "piutang", "tagihan") {
		return Detection{Intent: IntentPendingOrders}
	}
	if st := statusesIn(t); len(st) > 0 {
		return Detection{Intent: IntentOrderStatus, Statuses: st}
	}
	if hasAny(t, "status", "cek order", "cek po", "cek so", "lacak", "di mana", "dimana", "sampai mana") {
		return Detection{Intent: IntentOrderStatus}
	}
	if hasAny(t, "kritis", "menipis", "tipis", "habis", "restock", "restok") ||
		t == "stok" || strings.HasSuffix(t, " stok") {
		return Detection{Intent: IntentCriticalStock}
	}
	if hasAny(t, "hari ini", "performa", "performance", "kinerja", "bagaimana hari") {
		return Detection{Intent: IntentTodayPerformance}
	}
	if q, ok := extractProductQuery(text); ok {
		return Detection{Intent: IntentProductInfo, Arg: q}
	}
	return Detection{Intent: IntentHelp}
}

// productKeywords are dropped from a product query ("info produk beras").
var productKeywords = map[string]bool{
	"info": true, "detail": true, "status": true, "stok": true, "produk": true,
	"barang": true, "item": true, "harga": true, "cari": true, "tolong": true, "dong": true,
}

// extractProductQuery ports the legacy rule: text in quotes wins; else the
// message must mention produk/barang/item or start with info/harga/stok/cari/
// detail, and the query is what is left after the keywords. An empty query
// still routes to product_info (it asks for a code) when the message names a
// product noun.
func extractProductQuery(text string) (string, bool) {
	if q := quoted(text); q != "" {
		return strings.ToLower(q), true
	}
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(text)))
	if len(fields) == 0 {
		return "", false
	}
	mentions := false
	for _, f := range fields {
		if f == "produk" || f == "barang" || f == "item" {
			mentions = true
		}
	}
	switch fields[0] {
	case "info", "harga", "stok", "cari", "detail":
		mentions = true
	}
	if !mentions {
		return "", false
	}
	var rest []string
	for _, f := range fields {
		if !productKeywords[f] {
			rest = append(rest, f)
		}
	}
	return strings.Join(rest, " "), true
}

// quoted returns the first quoted fragment (straight or curly quotes).
func quoted(text string) string {
	pairs := [][2]string{{`"`, `"`}, {"“", "”"}, {"'", "'"}}
	for _, p := range pairs {
		i := strings.Index(text, p[0])
		if i < 0 {
			continue
		}
		rest := text[i+len(p[0]):]
		j := strings.Index(rest, p[1])
		if j > 0 {
			return strings.TrimSpace(rest[:j])
		}
	}
	return ""
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
		DurationMs: time.Since(r.t0).Milliseconds(), Status: contracts.ToolSuccess,
	})
}

// fail records one failed tool call (with the error text) and returns err so
// call sites stay `return "", rec.fail(...)`. The run still ends friendly; the
// row is what lets support see which tool broke (AS-02).
func (r *recorder) fail(name string, input any, err error) error {
	r.calls = append(r.calls, ToolCall{
		Tool: name, Input: input,
		DurationMs: time.Since(r.t0).Milliseconds(), Status: contracts.ToolFailed, Error: err.Error(),
	})
	return err
}

// Execute runs detection through tools with no access restriction (system
// callers and tests). branchID 0 = every active branch unless the text names
// one (AS-01). It returns the intent, the reply text, and the tool calls made.
func (e *Engine) Execute(ctx context.Context, branchID int64, text string) (intent, answer string, calls []ToolCall, err error) {
	out, err := e.Run(ctx, Scope{BranchID: branchID}, text)
	if out == nil {
		return "", "", nil, err
	}
	return out.Intent, out.Answer, out.Calls, err
}

// dispatch runs the detected intent's tool over the working branch set.
func (e *Engine) dispatch(ctx context.Context, d Detection, branches []*branchcontracts.Branch, text string, rec *recorder) (intent, answer string, err error) {
	switch d.Intent {
	case IntentHelp:
		return d.Intent, helpText(), nil
	case IntentOperationalSummary:
		from, to, label := dateRange(text)
		ans, err := e.operationalSummary(ctx, branches, from, to, label, rec)
		return d.Intent, ans, err
	case IntentSalesSummary:
		from, to, label := dateRange(text)
		ans, err := e.salesSummary(ctx, branches, from, to, label, rec)
		return d.Intent, ans, err
	case IntentPendingOrders:
		ans, err := e.pendingOrders(ctx, branches, rec)
		return d.Intent, ans, err
	case IntentOrderStatus:
		if len(d.Statuses) > 0 {
			ans, err := e.orderList(ctx, branches, d.Statuses, text, rec)
			return d.Intent, ans, err
		}
		ans, err := e.orderStatus(ctx, branches, d.Arg, rec)
		return d.Intent, ans, err
	case IntentCriticalStock:
		ans, err := e.criticalStock(ctx, branches, rec)
		return d.Intent, ans, err
	case IntentTodayPerformance:
		ans, err := e.todayPerformance(ctx, branches, rec)
		return d.Intent, ans, err
	case IntentProductInfo:
		ans, err := e.productInfo(ctx, branches, d.Arg, rec)
		return d.Intent, ans, err
	default:
		return IntentHelp, helpText(), nil
	}
}

func helpText() string {
	return "Halo! Saya asisten operasional. Tanya saya, misalnya:\n" +
		"- omzet hari ini / omzet bulan ini\n" +
		"- order pending / tagihan / hutang\n" +
		"- status ORD-UTM/PJ/2026/09/00001\n" +
		"- stok kritis\n" +
		"- info H-001\n" +
		"Sebut nama, kode, atau kota cabang untuk data satu cabang, contoh: omzet Bandung hari ini. " +
		"Tanpa nama cabang, saya jawab untuk semua cabang yang boleh kamu akses."
}

// --- tools --------------------------------------------------------------------

// salesSummary reports revenue/profit (+count/AOV) per branch for a range.
func (e *Engine) salesSummary(ctx context.Context, branches []*branchcontracts.Branch, from, to, label string, rec *recorder) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Omzet %s:\n", label)
	for _, br := range branches {
		rev, _, profit, err := e.finance.NetProfit(ctx, br.ID, from, to)
		if err != nil {
			return "", rec.fail("sales_summary", map[string]any{"branch": br.Code}, err)
		}
		rows, err := e.sales.ListOrders(ctx, br.ID, nil, from, to, 50)
		if err != nil {
			return "", rec.fail("sales_summary", map[string]any{"branch": br.Code}, err)
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
			return "", rec.fail("pending_orders", map[string]any{"branch": br.Code}, err)
		}
		po, err := e.purchasing.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 5)
		if err != nil {
			return "", rec.fail("pending_orders", map[string]any{"branch": br.Code}, err)
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
			return "", rec.fail("order_status", map[string]any{"number": number, "flow": "sales", "branch": br.Code}, err)
		} else if o != nil {
			rec.add("order_status", map[string]any{"number": number, "flow": "sales"},
				map[string]any{"status": o.Status, "total": o.GrandTotal})
			return fmt.Sprintf("%s (%s)\nStatus: %s\nCustomer: %s\nTanggal: %s\nTotal: %s",
				o.Number, br.Name, orderStatusID(o.Status), partyOrTunaiSales(o),
				shortDate(o.OrderDate), fmtIDR(o.GrandTotal)), nil
		}
		if o, err := e.purchasing.GetByNumber(ctx, br.ID, number); err != nil {
			return "", rec.fail("order_status", map[string]any{"number": number, "flow": "purchase", "branch": br.Code}, err)
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
			return "", rec.fail("critical_stock", map[string]any{"branch": br.Code}, err)
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
			return "", rec.fail("today_performance", map[string]any{"branch": br.Code}, err)
		}
		so, err := e.sales.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 50)
		if err != nil {
			return "", rec.fail("today_performance", map[string]any{"branch": br.Code}, err)
		}
		po, err := e.purchasing.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 50)
		if err != nil {
			return "", rec.fail("today_performance", map[string]any{"branch": br.Code}, err)
		}
		crit, err := e.stock.CriticalStock(ctx, br.ID, 50)
		if err != nil {
			return "", rec.fail("today_performance", map[string]any{"branch": br.Code}, err)
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
			return "", rec.fail("operational_summary", map[string]any{"branch": br.Code}, err)
		}
		so, err := e.sales.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 50)
		if err != nil {
			return "", rec.fail("operational_summary", map[string]any{"branch": br.Code}, err)
		}
		po, err := e.purchasing.ListOrders(ctx, br.ID, []string{"confirmed", "draft"}, "", "", 50)
		if err != nil {
			return "", rec.fail("operational_summary", map[string]any{"branch": br.Code}, err)
		}
		crit, err := e.stock.CriticalStock(ctx, br.ID, 50)
		if err != nil {
			return "", rec.fail("operational_summary", map[string]any{"branch": br.Code}, err)
		}
		rec.add("operational_summary", map[string]any{"branch": br.Code, "from": from, "to": to},
			map[string]any{"revenue": rev, "expense": exp, "profit": profit})
		fmt.Fprintf(&b, "- %s: omzet %s, beban %s, laba %s, %d order pending, %d produk kritis\n",
			br.Name, fmtIDR(rev), fmtIDR(exp), fmtIDR(profit), len(so)+len(po), len(crit))
	}
	return b.String(), nil
}

// productInfo resolves a product by exact code, then by code/name fragment
// over ALL active products (services and non-tracked items included). One
// match answers; several matches ask for the code instead of guessing.
func (e *Engine) productInfo(ctx context.Context, branches []*branchcontracts.Branch, query string, rec *recorder) (string, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return "Sebutkan kode atau nama produknya, contoh: info H-001", nil
	}
	pin := map[string]any{"query": q}
	p, err := e.products.GetByCode(ctx, strings.ToUpper(q))
	if err != nil {
		return "", rec.fail("product_info", pin, err)
	}
	if p != nil && p.Status != "active" {
		p = nil
	}
	if p == nil {
		ids, err := e.products.SearchIDs(ctx, q, 20)
		if err != nil {
			return "", rec.fail("product_info", pin, err)
		}
		var found []*productcontracts.Product
		for _, id := range ids {
			c, err := e.products.GetByID(ctx, id)
			if err != nil {
				return "", rec.fail("product_info", pin, err)
			}
			if c != nil && c.Status == "active" {
				found = append(found, c)
			}
		}
		lq := strings.ToLower(q)
		for _, c := range found {
			if strings.ToLower(c.Name) == lq {
				p = c
				break
			}
		}
		if p == nil && len(found) == 1 {
			p = found[0]
		}
		if p == nil && len(found) > 1 {
			rec.add("product_info", map[string]any{"query": q}, map[string]any{"matches": len(found)})
			var b strings.Builder
			fmt.Fprintf(&b, "Ada %d produk cocok dengan \"%s\". Sebutkan kodenya:\n", len(found), q)
			for i, c := range found {
				if i == 5 {
					b.WriteString("- ...\n")
					break
				}
				fmt.Fprintf(&b, "- %s %s\n", c.Code, c.Name)
			}
			return b.String(), nil
		}
	}
	if p == nil {
		rec.add("product_info", map[string]any{"query": q}, map[string]any{"found": false})
		return fmt.Sprintf("Produk \"%s\" tidak ditemukan di katalog.", q), nil
	}
	rec.add("product_info", pin, map[string]any{"code": p.Code})
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %s\n", p.Code, p.Name)
	fmt.Fprintf(&b, "Status: %s\n", productStatusID(p.Status))
	cat := p.CategoryName
	if cat == "" {
		cat = "-"
	}
	fmt.Fprintf(&b, "Kategori: %s\n", cat)
	fmt.Fprintf(&b, "Harga jual %s\n", fmtIDR(p.SellingPrice))
	fmt.Fprintf(&b, "Satuan beli/jual: %s/%s\n", uomOrDash(p.PurchaseUOM), uomOrDash(p.SalesUOM))
	if p.Tracked {
		fmt.Fprintf(&b, "Minimum stok: %s %s\n", fmtQty(p.MinStock), p.BaseUOM)
	} else {
		b.WriteString("Stok tidak dipantau\n")
	}
	for _, br := range branches {
		positions, _, err := e.finance.InventoryValue(ctx, br.ID)
		if err != nil {
			return "", rec.fail("product_info", map[string]any{"query": q, "branch": br.Code}, err)
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

// orderList lists up to 5 newest sales and purchase orders per status
// (legacy order_status list mode). A range applies only when the text names
// one; otherwise all dates are searched.
func (e *Engine) orderList(ctx context.Context, branches []*branchcontracts.Branch, statuses []string, text string, rec *recorder) (string, error) {
	from, to, label := "", "", "semua tanggal"
	if hasAny(strings.ToLower(text), "hari ini", "kemarin", "minggu ini", "7 hari", "seminggu", "bulan ini", "sebulan") {
		from, to, label = dateRange(text)
	}
	var names []string
	for _, st := range statuses {
		names = append(names, orderStatusID(st))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Order %s (%s):\n", strings.Join(names, "/"), label)
	empty := true
	for _, br := range branches {
		so, err := e.sales.ListOrders(ctx, br.ID, statuses, from, to, 5)
		if err != nil {
			return "", rec.fail("order_status", map[string]any{"branch": br.Code}, err)
		}
		po, err := e.purchasing.ListOrders(ctx, br.ID, statuses, from, to, 5)
		if err != nil {
			return "", rec.fail("order_status", map[string]any{"branch": br.Code}, err)
		}
		rec.add("order_status", map[string]any{"branch": br.Code, "statuses": statuses, "from": from, "to": to},
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
		return fmt.Sprintf("Tidak ada order %s (%s).", strings.Join(names, "/"), label), nil
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

func productStatusID(s string) string {
	switch s {
	case "active":
		return "aktif"
	case "inactive":
		return "nonaktif"
	case "archived":
		return "diarsipkan"
	default:
		return s
	}
}

func uomOrDash(u string) string {
	if u == "" {
		return "-"
	}
	return u
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
