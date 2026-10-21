package integration

import (
	"errors"
	"strings"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	salesapp "mini-erp/internal/modules/sales/application"
	"mini-erp/internal/shared/apperror"
)

func isValidationErr(err error) bool {
	var ae *apperror.AppError
	return errors.As(err, &ae) && ae.Code == apperror.CodeValidation
}

// posPaidOrder runs a walk-in POS checkout (COD, settled on the spot in cash)
// and returns the order id, its total and the payment recorded.
func posPaidOrder(t *testing.T, fx shopFixture, cart string, qty float64) (int64, int64, *paymentcontracts.Payment) {
	t.Helper()
	sales, deliv := testMods.sales.Service(), testMods.delivery.Service()
	o, _, err := sales.CreateOrderIdempotent(ctx(), 0, fx.branchID, posOrderInput(fx, cart, qty))
	if err != nil {
		t.Fatalf("create POS order: %v", err)
	}
	if _, err := sales.ConfirmOrder(ctx(), 0, fx.branchID, o.ID); err != nil {
		t.Fatalf("confirm POS order: %v", err)
	}
	lines := []deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: qty}}
	dn, _, err := deliv.CreateIdempotent(ctx(), 0, fx.branchID, o.ID, "", "", deliveryapp.DocInput{ClientRef: cart}, lines)
	if err != nil {
		t.Fatalf("create POS delivery: %v", err)
	}
	total := orderTotal(t, o.ID)
	if _, err := deliv.Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{
		Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow, Method: "cash", AmountTendered: total + 5000},
	}); err != nil {
		t.Fatalf("confirm POS delivery: %v", err)
	}
	var payID int64
	if err := testMods.db.QueryRow(`SELECT p.id FROM payments p JOIN payment_allocations a ON a.payment_id = p.id
		WHERE a.order_type = 'sales' AND a.order_id = ? AND p.status = 'active'`, o.ID).Scan(&payID); err != nil {
		t.Fatalf("POS payment not recorded: %v", err)
	}
	pay, err := testMods.payment.Service().GetByID(ctx(), payID)
	if err != nil || pay == nil {
		t.Fatalf("get POS payment: %v", err)
	}
	return o.ID, total, pay
}

func paidFor(t *testing.T, orderID int64) int64 {
	t.Helper()
	paid, err := testMods.payment.Service().PaidForOrders(ctx(), paymentcontracts.OrderSales, []int64{orderID})
	if err != nil {
		t.Fatalf("paid: %v", err)
	}
	return paid[orderID]
}

func inPostingQueue(t *testing.T, branchID, paymentID int64) bool {
	t.Helper()
	q, err := testMods.finance.Service().UnpostedSources(ctx(), branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	for _, it := range q {
		if it.DocType == "payment" && it.DocID == paymentID {
			return true
		}
	}
	return false
}

// A POS (walk-in, COD, completed) payment entered with the wrong method is
// corrected: the order stays settled, the original is cancelled with the
// reason and its journal reversed, the replacement waits in the posting queue,
// repeating the request is a no-op, and the replacement can be corrected again.
func TestPaymentCorrectPOSPayment(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.payment.Service()
	orderID, total, old := posPaidOrder(t, fx, "cart-correct", 2)
	if old.PartyID != 0 || old.Method != "cash" {
		t.Fatalf("POS payment = party %d method %s; want walk-in cash", old.PartyID, old.Method)
	}
	// A plain cancel is still refused (it would reopen the settled order).
	if _, err := svc.CancelWithReason(ctx(), 0, fx.branchID, old.ID, "salah metode"); !isConflict(err) {
		t.Fatalf("plain cancel = %v, want conflict", err)
	}
	if _, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "payment", old.ID); err != nil {
		t.Fatalf("post original: %v", err)
	}

	in := paymentapp.CorrectInput{
		ActorID: 0, BranchID: fx.branchID, PaymentID: old.ID,
		Method: "transfer", Amount: total, PaidAt: "2026-09-11", Reference: "TRF-778",
		Reason: "kasir salah pilih metode",
	}
	nw, replayed, err := svc.Correct(ctx(), in)
	if err != nil || replayed {
		t.Fatalf("correct: replayed=%v err=%v", replayed, err)
	}
	if nw.Method != "transfer" || nw.Amount != total || nw.Reference != "TRF-778" || nw.PartyID != 0 ||
		nw.CorrectsPaymentID != old.ID || nw.CorrectsPaymentNumber != old.Number || !strings.HasPrefix(nw.PaidAt, "2026-09-11") {
		t.Fatalf("replacement = %+v", nw)
	}
	if len(nw.Allocations) != 1 || nw.Allocations[0].OrderID != orderID || nw.Allocations[0].Amount != total {
		t.Fatalf("replacement allocations = %+v, want the whole order", nw.Allocations)
	}
	if got := paidFor(t, orderID); got != total {
		t.Fatalf("order paid after correction = %d, want %d (still settled)", got, total)
	}
	if so, _ := testMods.sales.Service().GetByID(ctx(), orderID); so == nil || so.Status != "completed" {
		t.Fatalf("order status = %+v, want completed", so)
	}
	was, _ := svc.GetByID(ctx(), old.ID)
	if was.Status != "cancelled" || was.CancelReason != in.Reason || was.CancelledAt == "" ||
		was.CorrectedByID != nw.ID || was.CorrectedByNumber != nw.Number {
		t.Fatalf("original after correction = %+v", was)
	}
	// Journals: the original's is reversed, the replacement is queued and posts.
	if n := liveEntries(t, "payment", old.ID); n != 0 {
		t.Fatalf("live journals of the original = %d, want 0", n)
	}
	if inPostingQueue(t, fx.branchID, old.ID) {
		t.Fatal("the corrected payment came back into the posting queue")
	}
	if !inPostingQueue(t, fx.branchID, nw.ID) {
		t.Fatal("the replacement is not in the posting queue")
	}
	if _, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "payment", nw.ID); err != nil {
		t.Fatalf("post replacement: %v", err)
	}
	if n := liveEntries(t, "payment", nw.ID); n != 1 {
		t.Fatalf("live journals of the replacement = %d, want 1", n)
	}

	// The same request again replays: no third payment, no second reversal.
	again, replayed, err := svc.Correct(ctx(), in)
	if err != nil || !replayed || again.ID != nw.ID {
		t.Fatalf("replay: id=%v replayed=%v err=%v; want the first replacement", again, replayed, err)
	}
	var count int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM payments`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("payments = %d, %v; want 2", count, err)
	}
	// Other data for the same original is refused: correct the replacement.
	diff := in
	diff.Method = "check"
	if _, _, err := svc.Correct(ctx(), diff); !isConflict(err) || !strings.Contains(err.Error(), nw.Number) {
		t.Fatalf("second correction of the original = %v, want conflict naming %s", err, nw.Number)
	}
	var audits int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_logs l JOIN audit_log_details d ON d.log_id = l.id
		WHERE l.action_key = 'payments.correct' AND l.entity_id = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(d.metadata_json, '$.reason')) = ?
		  AND JSON_EXTRACT(d.before_json, '$.method') = 'cash' AND JSON_EXTRACT(d.after_json, '$.method') = 'transfer'`,
		old.ID, in.Reason).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("payments.correct audit rows = %d, %v; want 1 with reason and before/after", audits, err)
	}

	// Chained: the replacement itself is corrected back to cash with change.
	third, _, err := svc.Correct(ctx(), paymentapp.CorrectInput{
		BranchID: fx.branchID, PaymentID: nw.ID, Method: "cash", Amount: total,
		AmountTendered: total + 2000, Reason: "ternyata memang tunai",
	})
	if err != nil {
		t.Fatalf("correct the replacement: %v", err)
	}
	if third.AmountTendered != total+2000 || third.CorrectsPaymentID != nw.ID || !strings.HasPrefix(third.PaidAt, "2026-09-11") {
		t.Fatalf("second replacement = %+v (date must carry over)", third)
	}
	if n := liveEntries(t, "payment", nw.ID); n != 0 {
		t.Fatalf("live journals of the first replacement = %d, want 0", n)
	}
	if got := paidFor(t, orderID); got != total {
		t.Fatalf("order paid after second correction = %d, want %d", got, total)
	}
}

// The corrected amount is spread over the original's orders as if the
// original were gone; more than they owe is refused.
func TestPaymentCorrectAmountReallocates(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.payment.Service()
	net, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 4}},
	})
	if err != nil {
		t.Fatalf("create net order: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, net.ID); err != nil {
		t.Fatalf("confirm net: %v", err)
	}
	total := orderTotal(t, net.ID)
	pay, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "uang muka", total/2, payAll(net.ID, total/2))
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	// Above what the order owes once the original is set aside.
	if _, _, err := svc.Correct(ctx(), paymentapp.CorrectInput{
		BranchID: fx.branchID, PaymentID: pay.ID, Method: "cash", Amount: total + 1, Reason: "salah ketik nominal",
	}); !isConflict(err) {
		t.Fatalf("correct above the bill = %v, want conflict", err)
	}
	// Exactly the whole bill fits (the original's half counts as free).
	nw, _, err := svc.Correct(ctx(), paymentapp.CorrectInput{
		BranchID: fx.branchID, PaymentID: pay.ID, Method: "transfer", Amount: total, Reason: "salah ketik nominal",
	})
	if err != nil {
		t.Fatalf("correct to the whole bill: %v", err)
	}
	if nw.Notes != "uang muka" || len(nw.Allocations) != 1 || nw.Allocations[0].Amount != total {
		t.Fatalf("replacement = %+v / %+v", nw, nw.Allocations)
	}
	if got := paidFor(t, net.ID); got != total {
		t.Fatalf("paid = %d, want %d", got, total)
	}
	// And back down: a net order may stay open.
	low, _, err := svc.Correct(ctx(), paymentapp.CorrectInput{
		BranchID: fx.branchID, PaymentID: nw.ID, Method: "transfer", Amount: total / 4, Reason: "nominal sebenarnya",
	})
	if err != nil {
		t.Fatalf("correct down: %v", err)
	}
	if got := paidFor(t, net.ID); got != low.Amount {
		t.Fatalf("paid after decrease = %d, want %d", got, low.Amount)
	}
}

// Refusals: a completed COD order cannot be left unpaid nor overpaid, offset
// legs, cancelled payments and missing reasons are refused, and nothing
// changes when they are.
func TestPaymentCorrectRefusals(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.payment.Service()
	_, total, pos := posPaidOrder(t, fx, "cart-refuse", 2)

	cases := []struct {
		name string
		in   paymentapp.CorrectInput
		code string
	}{
		{"leaves the COD order unpaid", paymentapp.CorrectInput{Method: "cash", Amount: total - 1000, Reason: "kurang bayar"}, "conflict"},
		{"overpays the COD order", paymentapp.CorrectInput{Method: "cash", Amount: total + 1000, Reason: "lebih bayar"}, "conflict"},
		{"reason too short", paymentapp.CorrectInput{Method: "transfer", Amount: total, Reason: "x"}, "validation"},
		{"offset method", paymentapp.CorrectInput{Method: "offset", Amount: total, Reason: "pakai offset"}, "validation"},
		{"tendered on transfer", paymentapp.CorrectInput{Method: "transfer", Amount: total, AmountTendered: total, Reason: "uang diterima"}, "validation"},
	}
	for _, c := range cases {
		c.in.BranchID, c.in.PaymentID = fx.branchID, pos.ID
		_, _, err := svc.Correct(ctx(), c.in)
		if (c.code == "conflict" && !isConflict(err)) || (c.code == "validation" && !isValidationErr(err)) {
			t.Fatalf("%s: err = %v, want %s", c.name, err, c.code)
		}
	}
	if still, _ := svc.GetByID(ctx(), pos.ID); still.Status != "active" {
		t.Fatalf("refused corrections changed the payment: %s", still.Status)
	}
	var count int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM payments`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("payments = %d, %v; want 1 (refusals write nothing)", count, err)
	}

	// Offset legs (a confirmed return's automatic offset pair) are refused.
	soID := sellFixture(t, ctx(), fx, 20)
	deliverAll(t, ctx(), fx, soID, 10)
	ret := createSalesReturn(t, fx, soID, 4)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	var legID int64
	if err := testMods.db.QueryRow(`SELECT id FROM payments WHERE method = 'offset' AND status = 'active' LIMIT 1`).Scan(&legID); err != nil {
		t.Fatalf("offset leg: %v", err)
	}
	if _, _, err := svc.Correct(ctx(), paymentapp.CorrectInput{
		BranchID: fx.branchID, PaymentID: legID, Method: "cash", Amount: 1000, Reason: "koreksi offset",
	}); !isConflict(err) {
		t.Fatalf("correct an offset leg = %v, want conflict", err)
	}

	// A cancelled payment has nothing left to correct.
	if _, err := svc.CancelWithReason(ctx(), 0, fx.branchID, legID, "lepas offset"); err != nil {
		t.Fatalf("cancel offset pair: %v", err)
	}
	other, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, other.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	p, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", 1000, payAll(other.ID, 1000))
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if _, err := svc.Cancel(ctx(), 0, fx.branchID, p.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, _, err := svc.Correct(ctx(), paymentapp.CorrectInput{
		BranchID: fx.branchID, PaymentID: p.ID, Method: "transfer", Amount: 1000, Reason: "sudah batal",
	}); !isConflict(err) {
		t.Fatalf("correct a cancelled payment = %v, want conflict", err)
	}
	// Another branch never sees the payment.
	if _, _, err := svc.Correct(ctx(), paymentapp.CorrectInput{
		BranchID: fx.branchID + 999, PaymentID: pos.ID, Method: "transfer", Amount: total, Reason: "cabang lain",
	}); err == nil || !strings.Contains(err.Error(), "tidak ditemukan") {
		t.Fatalf("cross-branch correction = %v, want not found", err)
	}
}

// The cancel reason is stored with who/when on every leg and lands in the
// audit entry; an empty reason keeps working for internal callers.
func TestPaymentCancelReasonStored(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.payment.Service()
	soID := sellFixture(t, ctx(), fx, 20)
	deliverAll(t, ctx(), fx, soID, 10)
	total := orderTotal(t, soID)
	pay, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total/4, payAll(soID, total/4))
	if err != nil {
		t.Fatalf("pay: %v", err)
	}
	if _, err := svc.CancelWithReason(ctx(), 7, fx.branchID, pay.ID, strings.Repeat("x", 256)); err == nil {
		t.Fatal("a 256-character reason was accepted")
	}
	got, err := svc.CancelWithReason(ctx(), 7, fx.branchID, pay.ID, "  dobel input  ")
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if got.Status != "cancelled" || got.CancelReason != "dobel input" || got.CancelledBy != 7 || got.CancelledAt == "" {
		t.Fatalf("cancelled payment = %+v", got)
	}
	var audits int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_logs l JOIN audit_log_details d ON d.log_id = l.id
		WHERE l.action_key = 'payments.cancel' AND l.entity_id = ?
		  AND JSON_UNQUOTE(JSON_EXTRACT(d.after_json, '$.reason')) = 'dobel input'`, pay.ID).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("payments.cancel audit with reason = %d, %v; want 1", audits, err)
	}
	// Replaying the cancel changes nothing (the first reason stays).
	again, err := svc.CancelWithReason(ctx(), 8, fx.branchID, pay.ID, "alasan lain")
	if err != nil || again.CancelReason != "dobel input" || again.CancelledBy != 7 {
		t.Fatalf("replayed cancel = %+v, %v", again, err)
	}
}
