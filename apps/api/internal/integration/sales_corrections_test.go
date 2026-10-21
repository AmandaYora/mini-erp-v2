package integration

import (
	"encoding/json"
	"strings"
	"testing"

	auditcontracts "mini-erp/internal/modules/audit/contracts"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	productapp "mini-erp/internal/modules/product/application"
	salesapp "mini-erp/internal/modules/sales/application"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	stockapp "mini-erp/internal/modules/stock/application"
)

// Corrections of an order entered wrong (edit confirmed, cancel with reason,
// short-close): the server stays the authority and every correction leaves a
// before/after trail.

// salesTrail returns the newest audit record of an action on an order.
func salesTrail(t *testing.T, action string, soID int64) auditcontracts.Record {
	t.Helper()
	recs, _, err := testMods.audit.Service().Query(ctx(), auditcontracts.ListFilter{
		AllBranches: true, Action: action, Entity: "sales_order", EntityID: soID, Page: 1, Limit: 10,
	})
	if err != nil || len(recs) == 0 {
		t.Fatalf("audit %s for order %d: %v (%d rows)", action, soID, err, len(recs))
	}
	return recs[0]
}

func jsonMap(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	out := map[string]any{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return out
}

// glFootprint is the whole journal as a comparable fingerprint (entries,
// lines, debit and credit sums).
func glFootprint(t *testing.T) [4]int64 {
	t.Helper()
	var f [4]int64
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM finance_journal_entries`).Scan(&f[0]); err != nil {
		t.Fatalf("gl entries: %v", err)
	}
	if err := testMods.db.QueryRow(`SELECT COUNT(*), COALESCE(SUM(debit),0), COALESCE(SUM(credit),0) FROM finance_journal_lines`).Scan(&f[1], &f[2], &f[3]); err != nil {
		t.Fatalf("gl lines: %v", err)
	}
	return f
}

// TestSalesShortCloseKeepsBooksAndBalances: an order shipped in part is cut to
// what was delivered. Lines are re-priced with their own price/discount, the
// undelivered line leaves, the GL is untouched, and the payment module's party
// balance equals the books' receivable (delivered value minus paid).
func TestSalesShortCloseKeepsBooksAndBalances(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	svc := m.sales.Service()

	// A second stocked product the order will never ship.
	p2, err := m.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: "TST-002", Name: "Barang Kedua", Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs", PurchaseFactor: 1, SalesFactor: 1,
		PurchasePrice: 5000, SellingPrice: 8000,
	})
	if err != nil {
		t.Fatalf("product 2: %v", err)
	}
	if err := m.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: p2.ID, VariantID: p2.Variants[0].ID, Location: fx.locationID, Mode: "in", QtyDelta: 10, Reason: "uji",
	}); err != nil {
		t.Fatalf("stock 2: %v", err)
	}

	so, err := svc.CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none",
		Items: []salesapp.LineInput{
			{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 10, DiscountNominal: 1000},
			{ProductID: p2.ID, VariantID: p2.Variants[0].ID, UOM: "pcs", Qty: 5},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	// 10 x 15.000 - 1.000 + 5 x 8.000 = 189.000
	if so.GrandTotal != 189000 {
		t.Fatalf("grand total = %d, want 189000", so.GrandTotal)
	}

	// Guard: nothing shipped yet → refuse (cancel is the tool then).
	if _, err := svc.ShortClose(ctx(), 7, fx.branchID, so.ID, "pelanggan batal sisa"); !isConflict(err) {
		t.Fatalf("short-close without a shipment = %v, want conflict", err)
	}

	dn, err := draftNote(t, fx, so.ID, 4)
	if err != nil {
		t.Fatalf("draft SJ: %v", err)
	}
	// Guard: a draft shipment must be settled first.
	if _, err := svc.ShortClose(ctx(), 7, fx.branchID, so.ID, "pelanggan batal sisa"); !isConflict(err) {
		t.Fatalf("short-close with a draft SJ = %v, want conflict", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm SJ: %v", err)
	}
	if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, financeapp.SourceDelivery, dn.ID); err != nil {
		t.Fatalf("post SJ: %v", err)
	}
	pay, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", 20000, payAll(so.ID, 20000))
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if _, err := m.finance.Service().Post(ctx(), 0, fx.branchID, financeapp.SourcePayment, pay.ID); err != nil {
		t.Fatalf("post payment: %v", err)
	}

	// Guard: the reason is required.
	if _, err := svc.ShortClose(ctx(), 7, fx.branchID, so.ID, "  "); err == nil {
		t.Fatal("short-close without a reason succeeded")
	}

	// Preview: 4 of line 1 delivered → 4 x 15.000 - 400 (pro-rated nominal) =
	// 59.600; line 2 leaves the order.
	plan, err := svc.ShortClosePreview(ctx(), fx.branchID, so.ID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if plan.NewGrandTotal != 59600 || plan.OldGrandTotal != 189000 || len(plan.Lines) != 2 ||
		plan.Lines[0].DeliveredQty != 4 || plan.Lines[0].NewLineTotal != 59600 || !plan.Lines[1].Removed {
		t.Fatalf("preview = %+v", plan)
	}
	if plan.PaidAmount != 20000 || plan.OutstandingAfter != 39600 || plan.RefundDueAfter != 0 {
		t.Fatalf("preview money = paid %d outstanding %d refund %d", plan.PaidAmount, plan.OutstandingAfter, plan.RefundDueAfter)
	}

	glBefore := glFootprint(t)
	out, err := svc.ShortClose(ctx(), 7, fx.branchID, so.ID, "pelanggan batal sisa")
	if err != nil {
		t.Fatalf("short-close: %v", err)
	}
	if out.Status != "completed" || out.GrandTotal != 59600 || out.Subtotal != 59600 || out.DiscountTotal != 400 || len(out.Items) != 1 {
		t.Fatalf("closed order = status %s total %d subtotal %d discount %d items %d",
			out.Status, out.GrandTotal, out.Subtotal, out.DiscountTotal, len(out.Items))
	}
	if it := out.Items[0]; it.Qty != 4 || it.QtyBase != 4 || it.DiscountNominal != 400 || it.LineTotal != 59600 || it.TaxBase != 59600 {
		t.Fatalf("closed line = %+v", it)
	}
	if glFootprint(t) != glBefore {
		t.Fatalf("GL changed by the short-close: %v -> %v", glBefore, glFootprint(t))
	}

	// Detail read carries who/when/why.
	if err := svc.DecorateDetail(ctx(), []*salescontracts.SalesOrder{out}); err != nil {
		t.Fatalf("decorate: %v", err)
	}
	if out.ShortCloseReason != "pelanggan batal sisa" || out.ShortClosedBy != 7 || out.ShortClosedAt == "" {
		t.Fatalf("short-close stamp = %q by %d at %q", out.ShortCloseReason, out.ShortClosedBy, out.ShortClosedAt)
	}

	// Sub-ledger = books: delivered value minus paid.
	bal, err := m.payment.Service().GetPartyBalance(ctx(), fx.branchID, fx.customerID)
	if err != nil {
		t.Fatalf("party balance: %v", err)
	}
	if bal.Outstanding != 59600-20000 {
		t.Fatalf("party balance = %d, want %d", bal.Outstanding, 59600-20000)
	}
	recv, _, err := m.finance.Service().Receivables(ctx(), fx.branchID, "")
	if err != nil {
		t.Fatalf("receivables: %v", err)
	}
	var books int64
	for _, r := range recv {
		if r.PartyID == fx.customerID {
			books = r.Balance
		}
	}
	if books != bal.Outstanding {
		t.Fatalf("books receivable %d != payment party balance %d", books, bal.Outstanding)
	}

	// The trail: before/after of the cut line, the removed line and the totals.
	rec := salesTrail(t, "sales_orders.close_short", so.ID)
	before, after, meta := jsonMap(t, rec.Before), jsonMap(t, rec.After), jsonMap(t, rec.Metadata)
	if before["grandTotal"] != float64(189000) || after["grandTotal"] != float64(59600) ||
		before["status"] != "confirmed" || after["status"] != "completed" || meta["reason"] != "pelanggan batal sisa" {
		t.Fatalf("trail before=%v after=%v meta=%v", before, after, meta)
	}
	bl, _ := before["lines"].(map[string]any)
	al, _ := after["lines"].(map[string]any)
	if _, gone := al["TST-002 pcs"]; gone || bl["TST-002 pcs"] == nil || al["TST-001 pcs"] == nil {
		t.Fatalf("trail lines before=%v after=%v", bl, al)
	}

	// A repeat replays; the order no longer takes shipments.
	if _, err := svc.ShortClose(ctx(), 7, fx.branchID, so.ID, "pelanggan batal sisa"); err != nil {
		t.Fatalf("repeat short-close: %v", err)
	}
	if _, err := draftNote(t, fx, so.ID, 1); err == nil {
		t.Fatal("a shipment was drafted on a short-closed order")
	}
}

// TestSalesShortClosePreviewRefundDue: closing an order paid above its reduced
// total is allowed; the preview and the order summary report the refund due.
func TestSalesShortClosePreviewRefundDue(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	soID := sellFixture(t, ctx(), fx, 10) // cod, 10 x 15.000
	pay, err := m.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", 100000, payAll(soID, 100000))
	if err != nil || pay == nil {
		t.Fatalf("pay: %v", err)
	}
	dn, err := draftNote(t, fx, soID, 4)
	if err != nil {
		t.Fatalf("draft SJ: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm SJ: %v", err)
	}
	plan, err := m.sales.Service().ShortClosePreview(ctx(), fx.branchID, soID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if plan.NewGrandTotal != 60000 || plan.RefundDueAfter != 40000 || plan.OutstandingAfter != 0 {
		t.Fatalf("preview = total %d refund %d outstanding %d", plan.NewGrandTotal, plan.RefundDueAfter, plan.OutstandingAfter)
	}
	out, err := m.sales.Service().ShortClose(ctx(), 0, fx.branchID, soID, "stok habis")
	if err != nil {
		t.Fatalf("short-close: %v", err)
	}
	sums, err := m.sales.Service().PaymentSummaries(ctx(), []*salescontracts.SalesOrder{out})
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	if s := sums[soID]; s.RefundDue != 40000 || s.OutstandingAmount != 0 {
		t.Fatalf("summary after close = %+v", s)
	}
}

// TestSalesConfirmedOrderCorrections: a confirmed order with nothing hanging
// off it takes a full edit; once a shipment exists only the header-light
// fields change; both leave before/after + reason in the trail. Cancel stores
// its reason.
func TestSalesConfirmedOrderCorrections(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.sales.Service()
	soID := sellFixture(t, ctx(), fx, 10)
	o, _ := svc.GetByID(ctx(), soID)

	st, err := svc.Correction(ctx(), o)
	if err != nil || st.EditMode != salesapp.EditFull || st.CanCloseShort {
		t.Fatalf("correction of a clean confirmed order = %+v, %v", st, err)
	}
	in := salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none", Reason: "salah ketik qty",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 8}},
	}
	got, err := svc.UpdateOrder(ctx(), 5, fx.branchID, soID, in)
	if err != nil || got.GrandTotal != 120000 || got.Status != "confirmed" {
		t.Fatalf("full edit of confirmed = %+v, %v", got, err)
	}
	rec := salesTrail(t, "sales_orders.update", soID)
	before, after, meta := jsonMap(t, rec.Before), jsonMap(t, rec.After), jsonMap(t, rec.Metadata)
	if before["grandTotal"] != float64(150000) || after["grandTotal"] != float64(120000) ||
		meta["reason"] != "salah ketik qty" || meta["mode"] != "full" {
		t.Fatalf("full-edit trail before=%v after=%v meta=%v", before, after, meta)
	}
	if _, ok := before["notes"]; ok {
		t.Fatalf("trail carries an unchanged field: %v", before)
	}

	if _, err := draftNote(t, fx, soID, 3); err != nil {
		t.Fatalf("draft SJ: %v", err)
	}
	o, _ = svc.GetByID(ctx(), soID)
	st, err = svc.Correction(ctx(), o)
	if err != nil || st.EditMode != salesapp.EditHeader || !strings.Contains(st.EditLockReason, "surat jalan") || st.CanCloseShort {
		t.Fatalf("correction with a draft SJ = %+v, %v", st, err)
	}
	// Economic change refused while a shipment hangs off the order.
	in.Items[0].Qty = 9
	if _, err := svc.UpdateOrder(ctx(), 5, fx.branchID, soID, in); !isConflict(err) {
		t.Fatalf("qty change with a shipment = %v, want conflict", err)
	}
	// Header-light change passes and is trailed with only what changed.
	in.Items[0].Qty = 8
	in.Notes, in.TaxInvoiceNumber, in.Reason = "kirim pagi", "010.000-26.00000001", "catatan kurir"
	if _, err := svc.UpdateOrder(ctx(), 5, fx.branchID, soID, in); err != nil {
		t.Fatalf("header-light edit: %v", err)
	}
	rec = salesTrail(t, "sales_orders.update", soID)
	before, after, meta = jsonMap(t, rec.Before), jsonMap(t, rec.After), jsonMap(t, rec.Metadata)
	if after["notes"] != "kirim pagi" || after["taxInvoiceNumber"] != "010.000-26.00000001" || before["notes"] != "" ||
		meta["mode"] != "header" || meta["reason"] != "catatan kurir" || after["grandTotal"] != nil {
		t.Fatalf("header trail before=%v after=%v meta=%v", before, after, meta)
	}

	// Cancel with a reason (on a second, clean order).
	other := sellFixture(t, ctx(), fx, 1)
	if _, err := svc.CancelOrderWithReason(ctx(), 5, fx.branchID, other, strings.Repeat("x", 256)); err == nil {
		t.Fatal("a 256-char reason was accepted")
	}
	c, err := svc.CancelOrderWithReason(ctx(), 5, fx.branchID, other, "pelanggan batal")
	if err != nil || c.Status != "cancelled" {
		t.Fatalf("cancel = %+v, %v", c, err)
	}
	if err := svc.DecorateDetail(ctx(), []*salescontracts.SalesOrder{c}); err != nil {
		t.Fatalf("decorate: %v", err)
	}
	if c.CancelReason != "pelanggan batal" || c.CancelledBy != 5 || c.CancelledAt == "" {
		t.Fatalf("cancel stamp = %q by %d at %q", c.CancelReason, c.CancelledBy, c.CancelledAt)
	}
	rec = salesTrail(t, "sales_orders.cancel", other)
	if a := jsonMap(t, rec.After); a["cancelReason"] != "pelanggan batal" || a["status"] != "cancelled" {
		t.Fatalf("cancel trail after = %v", a)
	}
}

// TestSalesHeaderLightKeepsNetDueDate: a net order with shipments keeps a due
// date — the header-light path refuses clearing it.
func TestSalesHeaderLightKeepsNetDueDate(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.sales.Service()
	in := salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2}},
	}
	so, err := svc.CreateOrder(ctx(), 0, fx.branchID, in)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := svc.ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := draftNote(t, fx, so.ID, 1); err != nil {
		t.Fatalf("draft SJ: %v", err)
	}
	in.DueDate = ""
	if _, err := svc.UpdateOrder(ctx(), 0, fx.branchID, so.ID, in); err == nil || isConflict(err) {
		t.Fatalf("clearing a net due date = %v, want validation", err)
	}
	in.DueDate = "2027-01-15"
	got, err := svc.UpdateOrder(ctx(), 0, fx.branchID, so.ID, in)
	if err != nil || !strings.HasPrefix(got.DueDate, "2027-01-15") {
		t.Fatalf("moving the due date = %+v, %v", got, err)
	}
}

// TestSalesHeaderOnlyNeverReprices: the "ubah info order" dialog sends
// headerOnly — on a clean confirmed order it still takes the light path (no
// re-price from the live catalog) and refuses an economic change.
func TestSalesHeaderOnlyNeverReprices(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.sales.Service()
	soID := sellFixture(t, ctx(), fx, 2) // 2 x 15.000
	if _, err := testMods.product.Service().UpdateProduct(ctx(), 0, fx.productID, productapp.ProductInput{
		Code: "TST-001", Name: "Barang Uji", Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs", PurchaseFactor: 1, SalesFactor: 1,
		PurchasePrice: 10000, SellingPrice: 99000,
	}); err != nil {
		t.Fatalf("catalog price: %v", err)
	}
	in := salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none", HeaderOnly: true,
		Notes: "hanya catatan",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2}},
	}
	got, err := svc.UpdateOrder(ctx(), 0, fx.branchID, soID, in)
	if err != nil || got.GrandTotal != 30000 || got.Notes != "hanya catatan" {
		t.Fatalf("header-only edit = %+v, %v", got, err)
	}
	in.Items[0].Qty = 3
	if _, err := svc.UpdateOrder(ctx(), 0, fx.branchID, soID, in); !isConflict(err) {
		t.Fatalf("header-only qty change = %v, want conflict", err)
	}
	// Without headerOnly the same clean order re-prices (full correction).
	in.HeaderOnly, in.Items[0].Qty = false, 2
	if got, err := svc.UpdateOrder(ctx(), 0, fx.branchID, soID, in); err != nil || got.GrandTotal != 198000 {
		t.Fatalf("full edit = %+v, %v", got, err)
	}
}
