package integration

import (
	"testing"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	"mini-erp/internal/shared/apperror"
)

// TestOpeningRejectsUnbalancedAndIdempotent guards G-revisi (P4): an
// unbalanced cutover is rejected at preview AND at post, and re-posting
// returns the existing opening journal instead of double-booking.
func TestOpeningRejectsUnbalancedAndIdempotent(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	if _, err := m.finance.Service().PreviewOpening(ctx(), fx.branchID, "2026-09-01", "uji",
		[]financeapp.ManualLine{
			{AccountCode: "1100", Debit: 5000},
			{AccountCode: "3000", Credit: 4999},
		}); err == nil {
		t.Fatal("unbalanced preview accepted, want validation")
	} else if appErr := mustAppErr(t, err); appErr.Code != apperror.CodeValidation {
		t.Fatalf("preview code = %s, want validation", appErr.Code)
	}

	if _, err := m.finance.Service().PostOpening(ctx(), 0, fx.branchID, "2026-09-01", "uji",
		[]financeapp.ManualLine{
			{AccountCode: "1100", Debit: 5000},
			{AccountCode: "3000", Credit: 4999},
		}); err == nil {
		t.Fatal("unbalanced post accepted, want validation")
	}

	first, err := m.finance.Service().PostOpening(ctx(), 0, fx.branchID, "2026-09-01", "Saldo awal uji",
		[]financeapp.ManualLine{
			{AccountCode: "1100", Debit: 5000},
			{AccountCode: "3000", Credit: 5000},
		})
	if err != nil {
		t.Fatalf("post opening: %v", err)
	}
	second, err := m.finance.Service().PostOpening(ctx(), 0, fx.branchID, "2026-09-01", "Saldo awal kedua",
		[]financeapp.ManualLine{
			{AccountCode: "1110", Debit: 9000},
			{AccountCode: "3000", Credit: 9000},
		})
	if err != nil {
		t.Fatalf("re-post opening: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("re-post created entry %d, want dedupe to %d", second.ID, first.ID)
	}
	status, err := m.finance.Service().OpeningStatus(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("opening status: %v", err)
	}
	if status == nil || status.ID != first.ID {
		t.Fatal("opening status missing after post")
	}
}

// TestPostingQueueDerived guards the table-free queue: a confirmed delivery
// shows up unposted, batch-posts, then disappears from the queue.
func TestPostingQueueDerived(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 2)
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 2,
		}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}

	queue, err := m.finance.Service().UnpostedSources(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	found := false
	for _, q := range queue {
		if q.DocType == "delivery" && q.DocID == dn.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("confirmed delivery %d missing from queue (%d items)", dn.ID, len(queue))
	}

	res, err := m.finance.Service().PostBatch(ctx(), 0, fx.branchID,
		[]financeapp.BatchItem{{DocType: "delivery", DocID: dn.ID}})
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if len(res) != 1 || res[0].Error != "" || res[0].EntryNumber == "" {
		t.Fatalf("batch result = %+v, want one posted entry", res)
	}

	after, err := m.finance.Service().UnpostedSources(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("queue after: %v", err)
	}
	for _, q := range after {
		if q.DocType == "delivery" && q.DocID == dn.ID {
			t.Fatal("posted delivery still queued")
		}
	}
}

// TestTaxAdjustmentExcludedFromCommercial guards the fiscal filter
// discipline: a correction moves the fiscal worksheet but commercial profit
// does not budge.
func TestTaxAdjustmentExcludedFromCommercial(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	before, err := m.finance.Service().ProfitLoss(ctx(), fx.branchID, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("profit before: %v", err)
	}
	if _, err := m.finance.Service().CreateTaxAdjustment(ctx(), 0, fx.branchID, "2026-09-15", "koreksi uji",
		[]financeapp.ManualLine{
			{AccountCode: "6000", Debit: 1000},
			{AccountCode: "1100", Credit: 1000},
		}); err != nil {
		t.Fatalf("tax adjustment: %v", err)
	}
	after, err := m.finance.Service().ProfitLoss(ctx(), fx.branchID, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("profit after: %v", err)
	}
	if after.Profit != before.Profit {
		t.Fatalf("commercial profit moved %d -> %d after fiscal correction", before.Profit, after.Profit)
	}
	fiscal, err := m.finance.Service().FiscalSummary(ctx(), fx.branchID, "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatalf("fiscal summary: %v", err)
	}
	if fiscal.FiscalProfit != before.Profit-1000 {
		t.Fatalf("fiscal profit = %d, want %d", fiscal.FiscalProfit, before.Profit-1000)
	}
	if len(fiscal.Lines) == 0 {
		t.Fatal("fiscal worksheet empty, want the correction line")
	}
}

// TestSafeCloseGate guards H4: unposted documents block the close, and a
// clean month seals.
func TestSafeCloseGate(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	soID := sellFixture(t, ctx(), fx, 1)
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID,
			LocationID: fx.locationID, UOM: "pcs", Qty: 1,
		}})
	if err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}

	if err := m.finance.Service().SafeClose(ctx(), 0, fx.branchID, 2026, 9); err == nil {
		t.Fatal("safe-close passed with an unposted delivery, want conflict")
	} else if appErr := mustAppErr(t, err); appErr.Code != apperror.CodeConflict {
		t.Fatalf("safe-close code = %s, want conflict", appErr.Code)
	}

	if _, err := m.finance.Service().PostBatch(ctx(), 0, fx.branchID,
		[]financeapp.BatchItem{{DocType: "delivery", DocID: dn.ID}}); err != nil {
		t.Fatalf("batch: %v", err)
	}
	check, err := m.finance.Service().Readiness(ctx(), fx.branchID, 2026, 9)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if !check.Ready {
		t.Fatalf("readiness blockers after posting: %v", check.Blockers)
	}
	if err := m.finance.Service().SafeClose(ctx(), 0, fx.branchID, 2026, 9); err != nil {
		t.Fatalf("safe-close clean month: %v", err)
	}
}
