package integration

import (
	"strings"
	"testing"

	branchapp "mini-erp/internal/modules/branch/application"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	financeapp "mini-erp/internal/modules/finance/application"
	salesapp "mini-erp/internal/modules/sales/application"
	stockapp "mini-erp/internal/modules/stock/application"
	"mini-erp/internal/shared/apperror"
)

// ADR-0020: the cost ledger never holds value the books cannot explain. Every
// movement is either journaled at exactly its recorded value or is
// value-neutral; these tests pin each kind.

type costRow struct {
	total     float64
	estimated int
}

// refRows returns the ledger rows of one ref type for a product, in order.
func refRows(t *testing.T, branchID, productID int64, refType string) []costRow {
	t.Helper()
	rows, err := testMods.db.Query(`SELECT total_cost, is_estimated FROM finance_inventory_cost_movements
		WHERE branch_id = ? AND product_id = ? AND ref_type = ? ORDER BY stock_movement_id`, branchID, productID, refType)
	if err != nil {
		t.Fatalf("ref rows: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var out []costRow
	for rows.Next() {
		var r costRow
		if err := rows.Scan(&r.total, &r.estimated); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out = append(out, r)
	}
	return out
}

// positionOf syncs the branch and reads one position from the ledger.
func positionOf(t *testing.T, branchID, productID int64) (qty, total float64) {
	t.Helper()
	if _, err := testMods.finance.Service().Sync(ctx(), branchID); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if err := testMods.db.QueryRow(`SELECT
			COALESCE(SUM(CASE WHEN direction = 'in' THEN qty_base ELSE -qty_base END), 0), COALESCE(SUM(total_cost), 0)
		FROM finance_inventory_cost_movements WHERE branch_id = ? AND product_id = ?`, branchID, productID).Scan(&qty, &total); err != nil {
		t.Fatalf("position: %v", err)
	}
	return qty, total
}

// A cancellation is priced at the original document's own cost, never at the
// average of the moment it is cancelled: the pair nets to exactly zero.
func TestCancelMirrorsOriginalCost(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	svc := testMods.stock.Service()
	p := productWith(t, "CANCELCOST", 0, 1)
	vid := p.Variants[0].ID
	seedInCost(t, fx, p.ID, vid, fx.locationID, 10, 1000)
	shelf, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "RAK", Name: "Rak"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	tr, err := svc.CreateTransfer(ctx(), 0, fx.branchID, fx.branchID, fx.locationID, shelf.ID, "uji",
		[]stockapp.TransferItemInput{{ProductID: p.ID, VariantID: vid, Qty: 5}})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if _, err := svc.DispatchTransfer(ctx(), 0, fx.branchID, tr.ID); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	positionOf(t, fx.branchID, p.ID)
	// The average moves before the cancel: 5 x 1.000 + 10 x 4.000 = 3.000 each.
	seedInCost(t, fx, p.ID, vid, fx.locationID, 10, 4000)
	if _, err := svc.CancelTransfer(ctx(), 0, fx.branchID, tr.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	qty, total := positionOf(t, fx.branchID, p.ID)
	rows := refRows(t, fx.branchID, p.ID, "transfer_cancel")
	if len(rows) != 1 || rows[0].total != 5000 || rows[0].estimated != 0 {
		t.Fatalf("cancel rows = %+v, want one exact +5000 (the dispatched value, not 5 x 3.000)", rows)
	}
	if qty != 20 || total != 50000 {
		t.Fatalf("position = %v / %v, want 20 / 50000", qty, total)
	}
}

// A transfer received from another branch is priced at what the sending branch
// relieved — even when the receiving branch syncs first — so the company's
// stock value is conserved across branches.
func TestCrossBranchTransferCarriesSourceCost(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	m := testMods
	br, err := m.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: "CB2", Name: "Cabang 2"})
	if err != nil {
		t.Fatalf("branch: %v", err)
	}
	locB, err := m.stock.Service().CreateLocation(ctx(), 0, br.ID, stockapp.LocationInput{Code: "CB2", Name: "Gudang CB2"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	fxB := shopFixture{branchID: br.ID, locationID: locB.ID}
	p := productWith(t, "XFERCOST", 0, 1)
	vid := p.Variants[0].ID
	seedInCost(t, fx, p.ID, vid, fx.locationID, 10, 1000)
	seedInCost(t, fxB, p.ID, vid, locB.ID, 10, 5000)
	tr, err := m.stock.Service().CreateTransfer(ctx(), 0, fx.branchID, br.ID, fx.locationID, locB.ID, "antar cabang",
		[]stockapp.TransferItemInput{{ProductID: p.ID, VariantID: vid, Qty: 4}})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	if _, err := m.stock.Service().DispatchTransfer(ctx(), 0, fx.branchID, tr.ID); err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if _, err := m.stock.Service().ReceiveTransfer(ctx(), 0, br.ID, tr.ID); err != nil {
		t.Fatalf("receive: %v", err)
	}
	if n := countRows(t, `SELECT COUNT(*) FROM finance_inventory_cost_movements`); n != 0 {
		t.Fatalf("ledger rows before the first sync = %d", n)
	}
	qtyB, totalB := positionOf(t, br.ID, p.ID) // the receiving branch syncs first
	in := refRows(t, br.ID, p.ID, "transfer")
	if len(in) != 1 || in[0].total != 4000 || in[0].estimated != 0 {
		t.Fatalf("received leg = %+v, want exact +4000 (4 x the sender's 1.000)", in)
	}
	if qtyB != 14 || totalB != 54000 {
		t.Fatalf("receiving position = %v / %v, want 14 / 54000", qtyB, totalB)
	}
	qtyA, totalA := positionOf(t, fx.branchID, p.ID)
	if qtyA != 6 || totalA != 6000 {
		t.Fatalf("sending position = %v / %v, want 6 / 6000", qtyA, totalA)
	}
}

// Values are whole rupiah and the out that empties a position takes exactly
// what is left: a product with no stock has no value, and the shipments' COGS
// add up to what was bought.
func TestEmptyingOutLeavesNoValue(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	p := productWith(t, "THIRDS", 0, 1)
	vid := p.Variants[0].ID
	seedInCost(t, fx, p.ID, vid, fx.locationID, 3, 333.3333) // 999,9999 → Rp 1.000
	var cogs int64
	for i := 0; i < 3; i++ {
		noteID := sellAndDeliver(t, fx, p.ID, vid, 1)
		cogs += postedLegs(t, "delivery", noteID)["5000"][0]
	}
	qty, total := positionOf(t, fx.branchID, p.ID)
	if qty != 0 || total != 0 {
		t.Fatalf("emptied position = %v / %v, want 0 / 0", qty, total)
	}
	if cogs != 1000 {
		t.Fatalf("COGS = %d, want 1000 (333 + 333 + 334)", cogs)
	}
	var fractional int
	if err := testMods.db.QueryRow(`SELECT COUNT(*) FROM finance_inventory_cost_movements
		WHERE product_id = ? AND total_cost <> ROUND(total_cost)`, p.ID).Scan(&fractional); err != nil {
		t.Fatalf("fraction check: %v", err)
	}
	if fractional != 0 {
		t.Fatalf("%d ledger rows carry a fraction of a rupiah", fractional)
	}
}

// A damaged-location move is value-neutral even when it moves every unit: the
// in-leg mirrors the out-leg instead of falling back to the last inflow cost.
func TestDamagedMoveIsNeutralWhenItEmptiesPosition(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	p := productWith(t, "DMGALL", 0, 1)
	vid := p.Variants[0].ID
	seedInCost(t, fx, p.ID, vid, fx.locationID, 1, 1000)
	seedInCost(t, fx, p.ID, vid, fx.locationID, 1, 2000) // average 1.500, last inflow 2.000
	if err := testMods.stock.Service().MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: p.ID, VariantID: vid, LocationID: fx.locationID, Qty: 2, Reason: "pecah",
	}); err != nil {
		t.Fatalf("damaged: %v", err)
	}
	qty, total := positionOf(t, fx.branchID, p.ID)
	rows := refRows(t, fx.branchID, p.ID, "damaged")
	if len(rows) != 2 || rows[0].total != -3000 || rows[1].total != 3000 || rows[1].estimated != 0 {
		t.Fatalf("damaged rows = %+v, want -3000 / +3000 exact", rows)
	}
	if qty != 2 || total != 3000 {
		t.Fatalf("position = %v / %v, want 2 / 3000", qty, total)
	}
}

// Goods that arrive damaged with no source position add stock, so they are
// journaled like a surplus; the book-vs-ledger reconciliation stays exact.
func TestDamagedArrivalIsJournaled(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	p := productWith(t, "DMGIN", 0, 1)
	vid := p.Variants[0].ID
	seedInCost(t, fx, p.ID, vid, fx.locationID, 2, 1500)
	if err := testMods.stock.Service().MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: p.ID, VariantID: vid, Qty: 1, Reason: "datang pecah",
	}); err != nil {
		t.Fatalf("damaged arrival: %v", err)
	}
	ids := queuedStockMovements(t, fx.branchID)
	if len(ids) != 1 {
		t.Fatalf("queued = %v, want the arrival", ids)
	}
	legs := postedLegs(t, "stock_movement", ids[0])
	if legs["1200"][0] != 1500 || legs["5100"][1] != 1500 {
		t.Fatalf("arrival legs = %v, want Dr 1200 / Cr 5100 1500", legs)
	}
	rec, err := testMods.finance.Service().ReconcileInventory(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("recon: %v", err)
	}
	if !rec.Consistent || rec.Unexplained != 0 {
		t.Fatalf("recon = %+v, want exact", rec)
	}
}

// Persediaan moves only with stock and HPP only with the cost ledger: a manual
// journal on the first and an expense on the second are refused.
func TestStockControlAccountsRefuseHandEntries(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Service()
	_, err := fin.Manual(ctx(), 0, fx.branchID, "2026-09-10", "koreksi", []financeapp.ManualLine{
		{AccountCode: "1200", Debit: 1000}, {AccountCode: "1000", Credit: 1000},
	})
	if ae := mustAppErr(t, err); ae.Code != apperror.CodeValidation || len(ae.Fields) != 1 || !strings.Contains(ae.Fields[0].Message, "mutasi stok") {
		t.Fatalf("manual journal on Persediaan = %+v, want a validation error", ae)
	}
	_, err = fin.CreateExpense(ctx(), 0, fx.branchID, financeapp.CreateExpenseInput{
		ExpenseAccountID: accountIDByCode(t, "5000"), PayAccountID: accountIDByCode(t, "1000"),
		Amount: 7000, Date: "2026-09-10", Notes: "ongkir",
	})
	if ae := mustAppErr(t, err); ae.Code != apperror.CodeValidation {
		t.Fatalf("expense on HPP = %+v, want a validation error", ae)
	}
	if _, err := fin.Manual(ctx(), 0, fx.branchID, "2026-09-10", "koreksi beban", []financeapp.ManualLine{
		{AccountCode: "6000", Debit: 1000}, {AccountCode: "1000", Credit: 1000},
	}); err != nil {
		t.Fatalf("an ordinary manual journal must still post: %v", err)
	}
}

// Unjournaled documents explain the gap by their own ledger value, nothing
// more: a pending adjustment no longer turns an unrelated drift into a mere
// warning.
func TestReconFailsOnUnexplainedGapDespitePendingDocs(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	fin := testMods.finance.Service()
	p := productWith(t, "RECONGAP", 0, 1)
	vid := p.Variants[0].ID
	seedInCost(t, fx, p.ID, vid, fx.locationID, 10, 1000)
	noteID := sellAndDeliver(t, fx, p.ID, vid, 2)
	postedLegs(t, "delivery", noteID)
	if err := testMods.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: p.ID, VariantID: vid, Location: fx.locationID, Mode: "in", QtyDelta: 1, Reason: "opname lebih",
	}); err != nil {
		t.Fatalf("adjust: %v", err)
	}
	rec, err := fin.ReconcileInventory(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("recon: %v", err)
	}
	if !rec.Consistent || rec.PendingValue != 1000 || rec.Unexplained != 0 {
		t.Fatalf("recon with a pending surplus = %+v, want explained (pending 1000, unexplained 0)", rec)
	}

	// Drift the shipment's ledger row with no journal behind it.
	if _, err := testMods.db.Exec(`UPDATE finance_inventory_cost_movements SET total_cost = total_cost - 500
		WHERE ref_type = 'delivery' AND product_id = ?`, p.ID); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	rec, err = fin.ReconcileInventory(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("recon: %v", err)
	}
	if rec.Consistent || rec.Unexplained != 500 {
		t.Fatalf("recon after drift = %+v, want unexplained 500", rec)
	}
	ready, err := fin.Readiness(ctx(), fx.branchID, 2026, 10)
	if err != nil {
		t.Fatalf("readiness: %v", err)
	}
	if c := checkOf(t, ready, "inventory_recon"); c.Status != financeapp.CheckFailed || c.Count != 500 {
		t.Fatalf("inventory_recon = %+v, want failed with 500 unexplained", c)
	}
}

// One shipment of one product drawn from two locations books its COGS once:
// each line takes its share of the document's recorded value, and the lines
// add up to exactly what the cost ledger relieved (no double booking).
func TestMultiLocationShipmentBooksCostOnce(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	p := productWith(t, "TWOLOC", 0, 1)
	vid := p.Variants[0].ID
	shelf, err := testMods.stock.Service().CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "RAK2", Name: "Rak 2"})
	if err != nil {
		t.Fatalf("location: %v", err)
	}
	seedInCost(t, fx, p.ID, vid, fx.locationID, 3, 1000)
	seedInCost(t, fx, p.ID, vid, shelf.ID, 3, 1000)
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: p.ID, VariantID: vid, UOM: "pcs", Qty: 5}},
	})
	if err != nil {
		t.Fatalf("SO: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm SO: %v", err)
	}
	n, err := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-09-10", "", deliveryapp.DocInput{},
		[]deliveryapp.LineInput{
			{ProductID: p.ID, VariantID: vid, LocationID: fx.locationID, UOM: "pcs", Qty: 3},
			{ProductID: p.ID, VariantID: vid, LocationID: shelf.ID, UOM: "pcs", Qty: 2},
		})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm shipment: %v", err)
	}
	legs := postedLegs(t, "delivery", n.ID)
	if legs["5000"][0] != 5000 || legs["1200"][1] != 5000 {
		t.Fatalf("legs = %v, want COGS/inventory 5000 once (not 5000 per line)", legs)
	}
	rec, err := testMods.finance.Service().ReconcileInventory(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("recon: %v", err)
	}
	if !rec.Consistent || rec.Unexplained != 0 {
		t.Fatalf("recon = %+v, want exact", rec)
	}
}
