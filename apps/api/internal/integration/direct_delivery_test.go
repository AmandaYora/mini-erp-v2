package integration

import (
	"strings"
	"sync"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	financeapp "mini-erp/internal/modules/finance/application"
	partyapp "mini-erp/internal/modules/party/application"
	productapp "mini-erp/internal/modules/product/application"
	salesapp "mini-erp/internal/modules/sales/application"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	"mini-erp/internal/shared/doclock"
)

// SJ Langsung (ADR-0022): a surat jalan drafted before any order exists, the
// order built from it afterwards. These tests cover PLAN sj-langsung D-03,
// D-04, D-05, D-06, S-02, F-01 and F-02.

// directNote drafts a direct note of qty units of the fixture product for the
// fixture customer.
func directNote(t *testing.T, fx shopFixture, qty float64, clientRef string) (*deliveryapp.Service, *deliverycontracts.DeliveryNote) {
	t.Helper()
	svc := testMods.delivery.Service()
	n, _, err := svc.CreateDirect(ctx(), 0, fx.branchID, deliveryapp.DirectInput{
		PartyID: fx.customerID, DeliveryDate: "2026-09-10",
		Doc:   deliveryapp.DocInput{DriverName: "Budi", WarehouseStaffName: "Sari", ClientRef: clientRef},
		Lines: []deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: qty}},
	})
	if err != nil {
		t.Fatalf("create direct note: %v", err)
	}
	return svc, n
}

// confirmedDirect drafts and confirms a direct note.
func confirmedDirect(t *testing.T, fx shopFixture, qty float64, clientRef string) *deliverycontracts.DeliveryNote {
	t.Helper()
	svc, n := directNote(t, fx, qty, clientRef)
	out, err := svc.Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{RecipientName: "Pak Agus", RecipientSignatureStatus: "signed"})
	if err != nil {
		t.Fatalf("confirm direct note: %v", err)
	}
	return out
}

// D-03/D-04/F-01/F-02: a direct note ships without an order, waits in the
// posting queue as "menunggu order", cannot be ignored or journalled, blocks
// the close, and is voided with its stock back.
func TestDirectDeliveryLifecycleWithoutOrder(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.delivery.Service()

	// Guards: a service product and a missing customer are refused.
	jasa, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: "JASA-1", Name: "Ongkos kirim", Type: "jasa", Tracked: false,
		BaseUOM: "x", PurchaseUOM: "x", SalesUOM: "x", PurchaseFactor: 1, SalesFactor: 1, SellingPrice: 20000,
	})
	if err != nil {
		t.Fatalf("create service product: %v", err)
	}
	if _, _, err := svc.CreateDirect(ctx(), 0, fx.branchID, deliveryapp.DirectInput{
		PartyID: fx.customerID,
		Lines:   []deliveryapp.LineInput{{ProductID: jasa.ID, VariantID: jasa.Variants[0].ID, UOM: "x", Qty: 1}},
	}); !isValidation(err) {
		t.Fatalf("service line on a direct note = %v, want validation", err)
	}
	if _, _, err := svc.CreateDirect(ctx(), 0, fx.branchID, deliveryapp.DirectInput{
		PartyID: 999999,
		Lines:   []deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	}); !isValidation(err) {
		t.Fatalf("unknown customer = %v, want validation", err)
	}

	_, n := directNote(t, fx, 10, "cart-direct-1")
	if n.Origin != deliverycontracts.OriginDirect || n.SalesOrderID != 0 || n.PartyID != fx.customerID || n.Status != "draft" {
		t.Fatalf("draft = origin %s so %d party %d status %s", n.Origin, n.SalesOrderID, n.PartyID, n.Status)
	}
	if n.PartyName != "Customer Uji" {
		t.Fatalf("party name = %q", n.PartyName)
	}
	if len(n.Warnings) == 0 {
		t.Fatal("a draft direct note must warn that stock is not held")
	}
	// Same clientRef replays the draft.
	if again, replayed, err := svc.CreateDirect(ctx(), 0, fx.branchID, deliveryapp.DirectInput{
		PartyID: fx.customerID, Doc: deliveryapp.DocInput{ClientRef: "cart-direct-1"},
		Lines: []deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 10}},
	}); err != nil || !replayed || again.ID != n.ID {
		t.Fatalf("replay = %v %v %v", again, replayed, err)
	}
	// A settlement makes no sense without an order.
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); !isValidation(err) {
		t.Fatalf("settlement on direct note = %v, want validation", err)
	}
	out, err := svc.Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{RecipientName: "Pak Agus", RecipientSignatureStatus: "signed"})
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if out.Status != "confirmed" || out.SalesOrderID != 0 {
		t.Fatalf("after confirm: status %s so %d", out.Status, out.SalesOrderID)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand after direct confirm = %v, want 40", got)
	}
	// Confirming again is a replay: the stock leaves once.
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm replay: %v", err)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand after replay = %v, want 40", got)
	}

	// Open-direct readers (D-06).
	open, err := svc.ListOpenDirect(ctx(), fx.branchID, 0, 10)
	if err != nil || len(open) != 1 || open[0].ID != n.ID || open[0].PartyName != "Customer Uji" {
		t.Fatalf("ListOpenDirect = %v, %v", open, err)
	}
	sum, err := svc.OpenDirectSummary(ctx(), fx.branchID, fx.customerID)
	if err != nil || sum.Count != 1 || sum.OldestDate != "2026-09-10" {
		t.Fatalf("OpenDirectSummary = %+v, %v", sum, err)
	}
	listed, err := svc.ListFiltered(ctx(), fx.branchID, deliveryapp.ListFilter{Unordered: true}, 1, 10)
	if err != nil || listed.Total != 1 {
		t.Fatalf("list unordered = %v, %v", listed, err)
	}

	// Finance (F-01/F-02): queued as awaiting order, not journallable, not
	// ignorable, and a blocker of the month.
	fin := testMods.finance.Service()
	queue, err := fin.UnpostedSources(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	var item *financeapp.QueueItem
	for i := range queue {
		if queue[i].DocType == financeapp.SourceDelivery && queue[i].DocID == n.ID {
			item = &queue[i]
		}
	}
	if item == nil || !item.AwaitingOrder || item.PartyID != fx.customerID || item.PartyName != "Customer Uji" {
		t.Fatalf("queue item = %+v", item)
	}
	if _, err := fin.Post(ctx(), 0, fx.branchID, financeapp.SourceDelivery, n.ID); !isConflict(err) || !strings.Contains(err.Error(), "menunggu order") {
		t.Fatalf("post unordered direct note = %v, want conflict 'menunggu order'", err)
	}
	if _, err := fin.IgnoreSource(ctx(), 0, fx.branchID, financeapp.SourceDelivery, n.ID, "coba abaikan"); !isConflict(err) {
		t.Fatalf("ignore unordered direct note = %v, want conflict", err)
	}
	ready, err := fin.Readiness(ctx(), fx.branchID, 2026, 9)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	var direct *financeapp.ReadinessCheck
	for i := range ready.Checks {
		if ready.Checks[i].Key == "direct_unordered" {
			direct = &ready.Checks[i]
		}
	}
	if direct == nil || direct.Status != financeapp.CheckFailed || direct.Count != 1 || !strings.Contains(direct.Detail, "2026-09-10") {
		t.Fatalf("direct_unordered check = %+v", direct)
	}

	// Cancel after shipping: stock back, reason kept, replay safe.
	if _, err := svc.Cancel(ctx(), 0, fx.branchID, n.ID, ""); err != nil {
		t.Fatalf("cancel confirmed direct note: %v", err)
	}
	if _, err := svc.Cancel(ctx(), 0, fx.branchID, n.ID, "salah pelanggan"); err != nil {
		t.Fatalf("cancel replay: %v", err)
	}
	cur, _ := svc.GetByID(ctx(), n.ID)
	if cur.Status != "cancelled" {
		t.Fatalf("status after cancel = %s", cur.Status)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 50 {
		t.Fatalf("onhand after cancel = %v, want 50", got)
	}
	if sum, _ := svc.OpenDirectSummary(ctx(), fx.branchID, 0); sum.Count != 0 {
		t.Fatalf("open after cancel = %+v", sum)
	}
}

// S-02/D-05: two direct notes of one customer become one completed order; the
// notes link, the journals post at the order's prices, and a replay returns
// the same order.
func TestOrderFromDeliveries(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	a := confirmedDirect(t, fx, 6, "sjl-a")
	b := confirmedDirect(t, fx, 4, "sjl-b")
	sales := testMods.sales.Service()
	jasa, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: "JASA-2", Name: "Ongkos kirim", Type: "jasa", Tracked: false,
		BaseUOM: "x", PurchaseUOM: "x", SalesUOM: "x", PurchaseFactor: 1, SalesFactor: 1, SellingPrice: 20000,
	})
	if err != nil {
		t.Fatalf("create service product: %v", err)
	}

	pv, err := sales.PreviewFromDeliveries(ctx(), fx.branchID, salesapp.FromDeliveriesInput{DeliveryIDs: []int64{a.ID, b.ID}, PaymentTerms: "net", DueDate: "2026-12-31"})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if pv.PartyID != fx.customerID || len(pv.Items) != 1 || pv.Items[0].Qty != 10 || pv.GrandTotal != 150000 || len(pv.Notes) != 2 {
		t.Fatalf("preview = party %d items %d qty %v total %d notes %d", pv.PartyID, len(pv.Items), pv.Items[0].Qty, pv.GrandTotal, len(pv.Notes))
	}
	// Guards.
	if _, err := sales.PreviewFromDeliveries(ctx(), fx.branchID, salesapp.FromDeliveriesInput{DeliveryIDs: []int64{a.ID}, PaymentTerms: "prepaid"}); !isValidation(err) {
		t.Fatalf("prepaid = %v, want validation", err)
	}
	if _, err := sales.PreviewFromDeliveries(ctx(), fx.branchID, salesapp.FromDeliveriesInput{DeliveryIDs: []int64{a.ID},
		Lines: []salesapp.FromDeliveriesLine{{ProductID: jasa.ID, VariantID: jasa.Variants[0].ID, UOM: "x"}}}); !isValidation(err) {
		t.Fatalf("price line not on the notes = %v, want validation", err)
	}
	if _, err := sales.PreviewFromDeliveries(ctx(), fx.branchID, salesapp.FromDeliveriesInput{DeliveryIDs: []int64{a.ID},
		ExtraItems: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}}}); !isValidation(err) {
		t.Fatalf("stock product as extra item = %v, want validation", err)
	}
	if _, err := sales.CreateFromDeliveries(ctx(), 0, fx.branchID, salesapp.FromDeliveriesInput{DeliveryIDs: []int64{a.ID, b.ID}, PaymentTerms: "cod", ClientRef: "sjl-cod"}, nil); !isValidation(err) {
		t.Fatalf("cod without settlement = %v, want validation", err)
	}

	// Cod, switched to net at completion, plus a service line.
	res, err := sales.CreateFromDeliveries(ctx(), 0, fx.branchID, salesapp.FromDeliveriesInput{
		DeliveryIDs: []int64{a.ID, b.ID}, PaymentTerms: "cod", ClientRef: "sjl-1",
		ExtraItems: []salesapp.LineInput{{ProductID: jasa.ID, VariantID: jasa.Variants[0].ID, UOM: "x", Qty: 1}},
		Settlement: &salesapp.FromDeliveriesSettlement{Mode: salesapp.SettleSwitchToNet, DueDate: "2026-12-31"},
	}, nil)
	if err != nil {
		t.Fatalf("create from deliveries: %v", err)
	}
	o := res.Order
	if o.Status != salescontracts.StatusCompleted || o.Origin != salescontracts.OriginDelivery || o.PaymentTerms != "net" || o.GrandTotal != 170000 || len(o.Items) != 2 {
		t.Fatalf("order = status %s origin %s terms %s total %d items %d", o.Status, o.Origin, o.PaymentTerms, o.GrandTotal, len(o.Items))
	}
	for _, id := range []int64{a.ID, b.ID} {
		n, _ := testMods.delivery.Service().GetByID(ctx(), id)
		if n.SalesOrderID != o.ID || n.LinkedAt == "" || n.OrderNumber != o.Number {
			t.Fatalf("note %d after link: so %d linked %q number %q", id, n.SalesOrderID, n.LinkedAt, n.OrderNumber)
		}
	}
	// Replay with the same clientRef.
	again, err := sales.CreateFromDeliveries(ctx(), 0, fx.branchID, salesapp.FromDeliveriesInput{DeliveryIDs: []int64{a.ID, b.ID}, PaymentTerms: "cod", ClientRef: "sjl-1"}, nil)
	if err != nil || !again.Replayed || again.Order.ID != o.ID {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	// The notes are taken: a second order cannot use them.
	if _, err := sales.CreateFromDeliveries(ctx(), 0, fx.branchID, salesapp.FromDeliveriesInput{DeliveryIDs: []int64{a.ID}, PaymentTerms: "net", DueDate: "2026-12-31", ClientRef: "sjl-2"}, nil); !isConflict(err) {
		t.Fatalf("reuse linked note = %v, want conflict", err)
	}
	// The completed order is final: no edit, no cancel, no short-close.
	corr, err := sales.Correction(ctx(), o)
	if err != nil || corr.EditMode != salesapp.EditNone || corr.CanCloseShort {
		t.Fatalf("correction = %+v, %v", corr, err)
	}
	// Journals: each note books its share at the order's prices, dated on the note.
	fin := testMods.finance.Service()
	for _, id := range []int64{a.ID, b.ID} {
		entry, err := fin.Post(ctx(), 0, fx.branchID, financeapp.SourceDelivery, id)
		if err != nil {
			t.Fatalf("post note %d: %v", id, err)
		}
		if entry.ID == 0 || len(entry.Lines) == 0 {
			t.Fatalf("entry for note %d = %+v", id, entry)
		}
	}
	ready, err := fin.Readiness(ctx(), fx.branchID, 2026, 9)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	for _, c := range ready.Checks {
		if c.Key == "direct_unordered" && c.Status != financeapp.CheckOK {
			t.Fatalf("direct_unordered after link = %+v", c)
		}
	}
	if sum, _ := testMods.delivery.Service().OpenDirectSummary(ctx(), fx.branchID, 0); sum.Count != 0 {
		t.Fatalf("open after link = %+v", sum)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != 40 {
		t.Fatalf("onhand = %v, want 40", got)
	}
}

// S-02: a cash order from notes paid on the spot records one payment,
// idempotently, and the order ends completed and paid.
func TestOrderFromDeliveriesPayNow(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	a := confirmedDirect(t, fx, 5, "sjl-pay")
	sales := testMods.sales.Service()
	res, err := sales.CreateFromDeliveries(ctx(), 0, fx.branchID, salesapp.FromDeliveriesInput{
		DeliveryIDs: []int64{a.ID}, PaymentTerms: "cod", ClientRef: "sjl-pay-1",
		Settlement: &salesapp.FromDeliveriesSettlement{Mode: salesapp.SettlePayNow, Method: "cash", PaymentDate: "2026-09-10"},
	}, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	paid, err := testMods.payment.Payments().PaidForOrders(ctx(), "sales", []int64{res.Order.ID})
	if err != nil || paid[res.Order.ID] != res.Order.GrandTotal || res.Order.Status != salescontracts.StatusCompleted {
		t.Fatalf("paid = %v (%v), status %s", paid, err, res.Order.Status)
	}
	if _, err := sales.CreateFromDeliveries(ctx(), 0, fx.branchID, salesapp.FromDeliveriesInput{
		DeliveryIDs: []int64{a.ID}, PaymentTerms: "cod", ClientRef: "sjl-pay-1",
		Settlement: &salesapp.FromDeliveriesSettlement{Mode: salesapp.SettlePayNow, Method: "cash"},
	}, nil); err != nil {
		t.Fatalf("replay: %v", err)
	}
	paid, _ = testMods.payment.Payments().PaidForOrders(ctx(), "sales", []int64{res.Order.ID})
	if paid[res.Order.ID] != res.Order.GrandTotal {
		t.Fatalf("paid after replay = %v, want once", paid)
	}
}

// D-05: LinkDirect refuses notes of another customer, unconfirmed notes and
// notes already linked elsewhere, and never links a partial set.
func TestLinkDirectGuards(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	other, err := testMods.party.Service().CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: "Pelanggan Lain", Phone: "081234567891"})
	if err != nil {
		t.Fatalf("other customer: %v", err)
	}
	a := confirmedDirect(t, fx, 3, "lk-a")
	svc := testMods.delivery.Service()
	_, draft := directNote(t, fx, 2, "lk-draft")
	foreign, _, err := svc.CreateDirect(ctx(), 0, fx.branchID, deliveryapp.DirectInput{
		PartyID: other.ID, Lines: []deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}},
	})
	if err != nil {
		t.Fatalf("foreign note: %v", err)
	}
	if _, err := svc.Confirm(ctx(), 0, fx.branchID, foreign.ID, deliveryapp.ConfirmInput{RecipientName: "X", RecipientSignatureStatus: "signed"}); err != nil {
		t.Fatalf("confirm foreign: %v", err)
	}
	sales := testMods.sales.Service()
	so, err := sales.CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none", Origin: salescontracts.OriginDelivery,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 3}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := sales.ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm order: %v", err)
	}
	if err := svc.LinkDirect(ctx(), 0, fx.branchID, so.ID, []int64{a.ID, draft.ID}); !isConflict(err) {
		t.Fatalf("link with a draft = %v, want conflict", err)
	}
	if err := svc.LinkDirect(ctx(), 0, fx.branchID, so.ID, []int64{a.ID, foreign.ID}); !isConflict(err) {
		t.Fatalf("link foreign note = %v, want conflict", err)
	}
	if n, _ := svc.GetByID(ctx(), a.ID); n.SalesOrderID != 0 {
		t.Fatalf("a partially linked set: note a so = %d", n.SalesOrderID)
	}
	if err := svc.LinkDirect(ctx(), 0, fx.branchID, so.ID, []int64{a.ID}); err != nil {
		t.Fatalf("link: %v", err)
	}
	if err := svc.LinkDirect(ctx(), 0, fx.branchID, so.ID, []int64{a.ID}); err != nil {
		t.Fatalf("link replay: %v", err)
	}
	// Linked: no cancel here any more (sales return instead), and no second order.
	if _, err := svc.Cancel(ctx(), 0, fx.branchID, a.ID, "coba"); !isConflict(err) {
		t.Fatalf("cancel linked note = %v, want conflict", err)
	}
	so2, err := sales.CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none", Origin: salescontracts.OriginDelivery,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 3}},
	})
	if err != nil {
		t.Fatalf("second order: %v", err)
	}
	if _, err := sales.ConfirmOrder(ctx(), 0, fx.branchID, so2.ID); err != nil {
		t.Fatalf("confirm second: %v", err)
	}
	if err := svc.LinkDirect(ctx(), 0, fx.branchID, so2.ID, []int64{a.ID}); !isConflict(err) {
		t.Fatalf("link to a second order = %v, want conflict", err)
	}
	// A quantity mismatch is refused.
	so3, _ := sales.CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none", Origin: salescontracts.OriginDelivery,
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 9}},
	})
	_, _ = sales.ConfirmOrder(ctx(), 0, fx.branchID, so3.ID)
	b := confirmedDirect(t, fx, 4, "lk-b")
	if err := svc.LinkDirect(ctx(), 0, fx.branchID, so3.ID, []int64{b.ID}); !isConflict(err) {
		t.Fatalf("qty mismatch = %v, want conflict", err)
	}
}

// S-02: two admins build an order from the same note at once (process lock
// off, so the CAS link decides): exactly one order completes and the loser's
// order is voided, never left half-made.
func TestOrderFromDeliveriesRaceCompensates(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	a := confirmedDirect(t, fx, 2, "race-a")
	doclock.SetBypass(true)
	defer doclock.SetBypass(false)
	sales := testMods.sales.Service()
	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = sales.CreateFromDeliveries(ctx(), 0, fx.branchID, salesapp.FromDeliveriesInput{
				DeliveryIDs: []int64{a.ID}, PaymentTerms: "net", DueDate: "2026-12-31", ClientRef: "race-" + string(rune('a'+i)),
			}, nil)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
		} else if !isConflict(err) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("completed orders = %d, want 1 (errs %v)", ok, errs)
	}
	var dangling int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM sales_orders WHERE origin = 'delivery' AND status IN ('draft', 'confirmed')`).Scan(&dangling); err != nil {
		t.Fatalf("count: %v", err)
	}
	if dangling != 0 {
		t.Fatalf("dangling orders = %d, want 0", dangling)
	}
	n, _ := testMods.delivery.Service().GetByID(ctx(), a.ID)
	if n.SalesOrderID == 0 {
		t.Fatal("note not linked")
	}
}

// D-06: the warehouse queue shows a direct draft with its customer, and an
// order built from notes never appears as "to ship".
func TestDirectDeliveryWorkQueue(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	_, draft := directNote(t, fx, 1, "wq-draft")
	q, err := testMods.delivery.Service().WorkQueue(ctx(), fx.branchID, "", "", "", "", 50)
	if err != nil {
		t.Fatalf("work queue: %v", err)
	}
	found := false
	for _, w := range q.WaitingReturn {
		if w.ID == draft.ID {
			found = true
			if w.Origin != deliverycontracts.OriginDirect || w.PartyName != "Customer Uji" || w.OrderNumber != "" {
				t.Fatalf("waiting row = %+v", w)
			}
		}
	}
	if !found {
		t.Fatal("direct draft missing from the waiting tab")
	}
	a := confirmedDirect(t, fx, 2, "wq-a")
	res, err := testMods.sales.Service().CreateFromDeliveries(ctx(), 0, fx.branchID, salesapp.FromDeliveriesInput{
		DeliveryIDs: []int64{a.ID}, PaymentTerms: "net", DueDate: "2026-12-31", ClientRef: "wq-1",
	}, nil)
	if err != nil {
		t.Fatalf("order from note: %v", err)
	}
	q, _ = testMods.delivery.Service().WorkQueue(ctx(), fx.branchID, "", "", "", "", 50)
	for _, o := range q.CreateSJ {
		if o.OrderID == res.Order.ID {
			t.Fatal("order from notes listed as shippable")
		}
	}
}
