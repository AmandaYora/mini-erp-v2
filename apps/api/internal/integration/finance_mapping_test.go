package integration

import (
	"strings"
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
)

func accountIDByCode(t *testing.T, code string) int64 {
	t.Helper()
	var id int64
	if err := testMods.db.QueryRow(`SELECT id FROM finance_accounts WHERE code = ?`, code).Scan(&id); err != nil {
		t.Fatalf("account %s: %v", code, err)
	}
	return id
}

func postedLegs(t *testing.T, docType string, docID int64) map[string][2]int64 {
	t.Helper()
	e, err := testMods.finance.Service().Post(ctx(), 0, 1, docType, docID)
	if err != nil {
		t.Fatalf("post %s %d: %v", docType, docID, err)
	}
	legs := map[string][2]int64{}
	for _, l := range e.Lines {
		cur := legs[l.AccountCode]
		legs[l.AccountCode] = [2]int64{cur[0] + l.Debit, cur[1] + l.Credit}
	}
	return legs
}

// F2b-8: the seeded chart uses the legacy codes and every mapping is set.
func TestChartSeedUsesLegacyCodesAndAllKeys(t *testing.T) {
	freshDB(t)
	newShop(t, ctx())
	svc := testMods.finance.Service()
	if err := svc.EnsureChart(ctx(), 0); err != nil { // idempotent
		t.Fatalf("re-seed: %v", err)
	}
	maps, err := svc.GetMappings(ctx())
	if err != nil {
		t.Fatalf("mappings: %v", err)
	}
	if len(maps) != 19 {
		t.Fatalf("mappings = %d, want 19", len(maps))
	}
	want := map[string]string{"cash": "1000", "bank": "1010", "receivable": "1100", "payable": "2000",
		"ppn_out": "2100", "customer_advance": "2200", "rounding_difference": "5200", "purchase_return_difference": "5300",
		"opening_correction": "3100"}
	for k, code := range want {
		if maps[k] == nil || maps[k].Code != code {
			t.Fatalf("mapping %s = %+v, want account %s", k, maps[k], code)
		}
	}
}

// F2b-7: a builder follows the mapping (revenue re-pointed before any journal
// exists), and re-pointing a key whose old account carries journals is refused.
func TestMappingIsUsedAndGuarded(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.finance.Service()

	newRev, err := svc.CreateAccount(ctx(), 0, "4010", "Penjualan Jasa", "pendapatan", false)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := svc.SetMapping(ctx(), 0, "revenue", newRev.ID); err != nil {
		t.Fatalf("remap before any journal: %v", err)
	}
	soID := sellFixture(t, ctx(), fx, 2)
	note, _ := draftNote(t, fx, soID, 2)
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, note.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	legs := postedLegs(t, "delivery", note.ID)
	if legs["4010"][1] == 0 || legs["4000"][1] != 0 {
		t.Fatalf("revenue legs = %v, want the credit on remapped 4010", legs)
	}
	// Now 4010 carries journals: pointing the key elsewhere is refused.
	err = svc.SetMapping(ctx(), 0, "revenue", accountIDByCode(t, "4000"))
	if !isConflict(err) || !strings.Contains(err.Error(), "sudah dipakai") {
		t.Fatalf("remap after journals = %v, want conflict", err)
	}
}

// F2b-1: payment journals follow what the payment settles — a customer refund
// debits receivable (not payable), an offset never reaches the books, and a
// `check` goes to the bank account.
func TestPaymentKindsJournal(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	pay := testMods.payment.Service()

	soID := sellFixture(t, ctx(), fx, 4)
	deliverAll(t, ctx(), fx, soID, 4)
	total := orderTotal(t, soID)
	p1, err := pay.Create(ctx(), 0, fx.branchID, fx.customerID, "check", "", "", total, payAll(soID, total))
	if err != nil {
		t.Fatalf("check payment: %v", err)
	}
	legs := postedLegs(t, "payment", p1.ID)
	if legs["1010"][0] != total || legs["1100"][1] != total {
		t.Fatalf("check receipt legs = %v, want Dr 1010 / Cr 1100 %d", legs, total)
	}

	// Return, paid order => nothing to offset; refund the customer in cash.
	ret := createSalesReturn(t, fx, soID, 2)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	refund, err := pay.Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", ret.Total,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSalesReturn, OrderID: ret.ID, Amount: ret.Total}})
	if err != nil {
		t.Fatalf("refund: %v", err)
	}
	rl := postedLegs(t, "payment", refund.ID)
	if rl["1100"][0] != ret.Total || rl["1000"][1] != ret.Total || rl["2000"] != ([2]int64{}) {
		t.Fatalf("refund legs = %v, want Dr 1100 / Cr 1000 %d and nothing on payable", rl, ret.Total)
	}
}

// F2b-1: an offset pair is not journalled and Post refuses it.
func TestOffsetPaymentNotJournalled(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 10)
	deliverAll(t, ctx(), fx, soID, 10)
	ret := createSalesReturn(t, fx, soID, 3)
	if _, err := testMods.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	var id int64
	if err := testMods.db.QueryRow(`SELECT id FROM payments WHERE method='offset' LIMIT 1`).Scan(&id); err != nil {
		t.Fatalf("no offset leg written: %v", err)
	}
	if _, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "payment", id); !isConflict(err) {
		t.Fatalf("post offset = %v, want conflict", err)
	}
	q, err := testMods.finance.Service().UnpostedSources(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	for _, it := range q {
		if it.DocType == "payment" {
			t.Fatalf("offset payment queued for posting: %+v", it)
		}
	}
}
