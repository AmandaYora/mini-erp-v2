package integration

import (
	"sync"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	salesapp "mini-erp/internal/modules/sales/application"
)

func orderTotal(t *testing.T, soID int64) int64 {
	t.Helper()
	so, err := testMods.sales.Service().GetByID(ctx(), soID)
	if err != nil || so == nil {
		t.Fatalf("get SO: %v", err)
	}
	return so.GrandTotal
}

func payAll(soID, total int64) []paymentapp.AllocationInput {
	return []paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: soID, Amount: total}}
}

// F2a-6a #5: the same client reference never records a second payment.
func TestPaymentClientRefReplay(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 2)
	total := orderTotal(t, soID)
	svc := testMods.payment.Service()
	first, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total, payAll(soID, total),
		paymentapp.WithClientRef("pay-1"))
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total, payAll(soID, total),
		paymentapp.WithClientRef("pay-1"))
	if err != nil {
		t.Fatalf("replay: %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("replay created payment %d, want %d", second.ID, first.ID)
	}
	var n int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM payments`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("payments = %d, %v; want 1", n, err)
	}
}

// F2a-6a #8 + #7: cash tendered must cover the amount and is stored; change is
// derived; `check` is an accepted method and unknown methods are rejected.
func TestPaymentTenderedAndMethods(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 4)
	total := orderTotal(t, soID)
	svc := testMods.payment.Service()
	half := total / 2

	if _, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", half, payAll(soID, half),
		paymentapp.WithAmountTendered(half-1)); err == nil {
		t.Fatal("tendered below amount accepted")
	}
	pay, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", half, payAll(soID, half),
		paymentapp.WithAmountTendered(half+5000))
	if err != nil {
		t.Fatalf("cash with tendered: %v", err)
	}
	if pay.AmountTendered != half+5000 {
		t.Fatalf("tendered stored = %d, want %d", pay.AmountTendered, half+5000)
	}
	if _, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "check", "", "", total-half, payAll(soID, total-half),
		paymentapp.WithReference("CEK-001")); err != nil {
		t.Fatalf("check payment: %v", err)
	}
	if _, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "bitcoin", "", "", 1000, payAll(soID, 1000)); err == nil {
		t.Fatal("unknown method accepted")
	}
}

// F2a-6a: RecordForOrder settles the whole outstanding by default, derives
// party and direction from the order, and is idempotent per ClientRef.
func TestRecordForOrderIdempotent(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 3)
	total := orderTotal(t, soID)
	svc := testMods.payment.Service()
	in := paymentcontracts.RecordInput{
		BranchID: fx.branchID, OrderType: paymentcontracts.OrderSales, OrderID: soID,
		Method: "cash", ClientRef: "sj:77",
	}
	first, replayed, err := svc.RecordForOrder(ctx(), in)
	if err != nil || replayed {
		t.Fatalf("first = %+v, replayed=%v, %v", first, replayed, err)
	}
	if first.Amount != total {
		t.Fatalf("amount = %d, want the whole outstanding %d", first.Amount, total)
	}
	again, replayed, err := svc.RecordForOrder(ctx(), in)
	if err != nil || !replayed || again.ID != first.ID {
		t.Fatalf("replay = %+v, replayed=%v, %v", again, replayed, err)
	}
	paid, err := svc.PaidForOrders(ctx(), paymentcontracts.OrderSales, []int64{soID})
	if err != nil || paid[soID] != total {
		t.Fatalf("paid = %v, %v; want %d once", paid, err, total)
	}
}

// F2a-6a #5: twenty walk-in checkouts on different orders all succeed — they
// do not share one lock and none can overpay another.
func TestWalkInPaymentsRunIndependently(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	var orders []int64
	for i := 0; i < 20; i++ {
		so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
			PartyID: 0, Channel: "pos", PaymentTerms: "cod", TaxType: "none",
			Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
		})
		if err != nil {
			t.Fatalf("create walk-in order: %v", err)
		}
		if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
			t.Fatalf("confirm: %v", err)
		}
		orders = append(orders, so.ID)
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(orders))
	for _, id := range orders {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			_, _, err := testMods.payment.Service().RecordForOrder(ctx(), paymentcontracts.RecordInput{
				BranchID: fx.branchID, OrderType: paymentcontracts.OrderSales, OrderID: id,
				Method: "cash", ClientRef: "walkin-" + string(rune('A'+id%26)) + "-" + itoa(id),
			})
			errs <- err
		}(id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("walk-in payment: %v", err)
		}
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// F2a-6b #3: cancelling the payment of a COMPLETED cash-terms order is refused
// (it would silently become unpaid); the same on a net order is allowed.
func TestPaymentCancelGuardCompletedCODOrder(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.payment.Service()

	cod := sellFixture(t, ctx(), fx, 2) // cod
	note, err := draftNote(t, fx, cod, 2)
	if err != nil {
		t.Fatalf("draft shipment: %v", err)
	}
	// The last shipment settles the COD order on the spot (pay_now).
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{
		Settlement: &deliveryapp.Settlement{Mode: deliveryapp.SettlePayNow, Method: "cash"},
	}); err != nil {
		t.Fatalf("confirm with pay_now: %v", err)
	}
	var payID int64
	if err := testMods.db.QueryRow(`SELECT id FROM payments WHERE client_ref = ?`, "sj:"+itoa(note.ID)).Scan(&payID); err != nil {
		t.Fatalf("settlement payment not recorded: %v", err)
	}
	pay, err := svc.GetByID(ctx(), payID)
	if err != nil || pay == nil {
		t.Fatalf("get payment: %v", err)
	}
	if _, err := svc.Cancel(ctx(), 0, fx.branchID, pay.ID); !isConflict(err) {
		t.Fatalf("cancel on completed COD order = %v, want conflict", err)
	}
	still, _ := svc.GetByID(ctx(), pay.ID)
	if still.Status != "active" {
		t.Fatalf("payment status = %s, want active (guard must not cancel)", still.Status)
	}

	// Net terms: the order stays payable later, so cancelling is fine.
	net, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: "2026-12-31", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	})
	if err != nil {
		t.Fatalf("create net order: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, net.ID); err != nil {
		t.Fatalf("confirm net: %v", err)
	}
	deliverAll(t, ctx(), fx, net.ID, 1)
	netTotal := orderTotal(t, net.ID)
	npay, err := svc.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", netTotal, payAll(net.ID, netTotal))
	if err != nil {
		t.Fatalf("pay net: %v", err)
	}
	if _, err := svc.Cancel(ctx(), 0, fx.branchID, npay.ID); err != nil {
		t.Fatalf("cancel on net order: %v", err)
	}
}

// F2a-6b #4: a credit offset is two legs written atomically with a shared
// pair id; cancelling one leg cancels both; two simultaneous cancels cancel
// once.
func TestSettleCreditPairAtomicAndCancelledTogether(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	// Half the order ships, so the SO stays open (cancel guard only protects
	// COMPLETED cash-terms orders) and 10 delivered units can come back.
	soID := sellFixture(t, ctx(), fx, 20)
	deliverAll(t, ctx(), fx, soID, 10)
	ret := createSalesReturn(t, fx, soID, 4)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	svc := testMods.payment.Service()
	retTotal := ret.Total
	_ = retTotal
	// Confirming the return already offset it against the open bill: that is a
	// settle-credit pair written by the system.
	var billLeg int64
	if err := testMods.db.QueryRow(`SELECT id FROM payments WHERE method='offset' AND direction='in' LIMIT 1`).Scan(&billLeg); err != nil {
		t.Fatalf("automatic offset pair missing: %v", err)
	}
	leg, err := svc.GetByID(ctx(), billLeg)
	if err != nil || leg == nil {
		t.Fatalf("get leg: %v", err)
	}
	if leg.PairID == 0 {
		t.Fatal("bill leg has no pair id")
	}
	other, err := svc.GetByID(ctx(), leg.PairID)
	if err != nil || other == nil || other.PairID != leg.ID {
		t.Fatalf("paired leg = %+v, %v; want it to point back at %d", other, err, leg.ID)
	}
	var active int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM payments WHERE status = 'active'`).Scan(&active); err != nil || active != 2 {
		t.Fatalf("active legs = %d, %v; want 2", active, err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = svc.Cancel(ctx(), 0, fx.branchID, leg.ID)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("cancel %d: %v", i, err)
		}
	}
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM payments WHERE status = 'active'`).Scan(&active); err != nil || active != 0 {
		t.Fatalf("active legs after cancel = %d, %v; want 0 (both legs cancelled)", active, err)
	}
}
