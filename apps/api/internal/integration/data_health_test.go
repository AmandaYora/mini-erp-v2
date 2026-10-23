package integration

import (
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	dashboardapp "mini-erp/internal/modules/dashboard/application"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	partyapp "mini-erp/internal/modules/party/application"
	productapp "mini-erp/internal/modules/product/application"
	salesapp "mini-erp/internal/modules/sales/application"
	"mini-erp/internal/shared/timeutil"
)

// Data-health checks (ADR-0025, spec §8.2): each owning module's read-only
// check raises its alert on a fixture and drops it once the data is fixed.

// healthCostRow inserts one cost-ledger row (stock_movement_id must be unique).
func healthCostRow(t *testing.T, branchID, productID, variantID, movementID int64, refType, dir string, qty, total float64) {
	t.Helper()
	var unit any // value-only rows (qty 0) carry no unit cost
	if qty != 0 {
		unit = total / qty
	}
	if _, err := testMods.db.Exec(`INSERT INTO finance_inventory_cost_movements
		(branch_id, product_id, variant_id, stock_movement_id, ref_type, direction, qty_base, unit_cost, total_cost, moved_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(6))`,
		branchID, productID, variantID, movementID, refType, dir, qty, unit, total); err != nil {
		t.Fatalf("cost row: %v", err)
	}
}

func healthProduct(t *testing.T, code, name string, selling, minSelling int64) int64 {
	t.Helper()
	p, err := testMods.product.Service().CreateProduct(ctx(), 0, productapp.ProductInput{
		Code: code, Name: name, Type: "barang", Tracked: true, BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs",
		PurchaseFactor: 1, SalesFactor: 1, PurchasePrice: 1000, SellingPrice: selling,
	})
	if err != nil {
		t.Fatalf("create product %s: %v", code, err)
	}
	// The product form refuses min > selling; legacy data carries it (A8),
	// so the fixture writes it directly.
	if minSelling > 0 {
		if _, err := testMods.db.Exec(`UPDATE products SET min_selling_price = ? WHERE id = ?`, minSelling, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	return p.ID
}

// TestDataHealthFinanceChecks: A2 ratio boundary (exactly 1.2 stays out),
// stockless products ignored; A7 opening-only positions (opening_cost does not
// count as a movement, a delivery does); A5 unclosed past months; A4 range;
// and none of them writes a row.
func TestDataHealthFinanceChecks(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	fin := testMods.finance.Health()
	// Clear the fixture's own stock-in ledger rows (if costing ran) so the
	// branch ledger holds only what this test writes.
	if _, err := testMods.db.Exec(`DELETE FROM finance_inventory_cost_movements WHERE branch_id = ?`, fx.branchID); err != nil {
		t.Fatal(err)
	}
	exact := healthProduct(t, "HC-1", "Modal pas", 15000, 0)
	above := healthProduct(t, "HC-2", "Modal lebih", 15000, 0)
	empty := healthProduct(t, "HC-3", "Stok habis", 15000, 0)
	healthCostRow(t, fx.branchID, exact, 0, 9001, "opening", "in", 10, 180000) // 18.000/unit = 1,2 × 15.000
	healthCostRow(t, fx.branchID, above, 0, 9003, "opening", "in", 10, 181000) // 18.100/unit > 1,2×
	healthCostRow(t, fx.branchID, above, 0, 9002, "opening_cost", "in", 0, 500)
	healthCostRow(t, fx.branchID, empty, 0, 9004, "opening", "in", 10, 500000)
	healthCostRow(t, fx.branchID, empty, 0, 9005, "delivery", "out", 10, 500000)
	var before int64
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM finance_inventory_cost_movements`).Scan(&before)

	a2, err := fin.CostAbovePrice(ctx(), fx.branchID, 1.2, 5)
	if err != nil {
		t.Fatal(err)
	}
	if a2.Count != 1 || a2.AmountRp == nil || *a2.AmountRp != 181500 || len(a2.Sample) != 1 || !strings.HasPrefix(a2.Sample[0].Label, "HC-2") {
		t.Fatalf("A2 = %+v (amount %v); want only HC-2 at Rp181.500", a2, a2.AmountRp)
	}
	items, total, err := fin.CostAbovePriceItems(ctx(), fx.branchID, 1.2, 1, 20)
	if err != nil || total != 1 || len(items) != 1 || len(items[0].Links) != 2 {
		t.Fatalf("A2 items = %+v total %d err %v", items, total, err)
	}

	a7, err := fin.OpeningUnverified(ctx(), fx.branchID, 5)
	if err != nil {
		t.Fatal(err)
	}
	// exact + above are opening-only (opening_cost is a value fix, not a
	// movement); "empty" sold its stock (delivery) and holds none.
	if a7.Count != 2 || a7.Meta["positions"] != 2 {
		t.Fatalf("A7 = %+v; want 2 products / 2 positions", a7)
	}
	// One sale (delivery row) verifies the product: it leaves the list.
	healthCostRow(t, fx.branchID, above, 0, 9006, "delivery", "out", 1, 18150)
	if a7, _ = fin.OpeningUnverified(ctx(), fx.branchID, 5); a7.Count != 1 {
		t.Fatalf("A7 after a delivery = %d, want 1", a7.Count)
	}

	// A5: July has activity (no period row = open), August closed, September
	// an open row without activity; the current month never counts.
	now := timeutil.NowUTC().In(timeutil.Jakarta)
	cur := now.Year()*100 + int(now.Month())
	if _, err := testMods.db.Exec(`INSERT INTO finance_account_daily (branch_id, account_id, entry_date, debit) VALUES (?, 1, '2026-07-15', 100)`, fx.branchID); err != nil {
		t.Fatal(err)
	}
	if _, err := testMods.db.Exec(`INSERT INTO finance_periods (branch_id, year, month, status) VALUES (?, 2026, 8, 'closed'), (?, 2026, 9, 'open')`, fx.branchID, fx.branchID); err != nil {
		t.Fatal(err)
	}
	months, err := fin.UnclosedPastPeriods(ctx(), fx.branchID, cur)
	if err != nil {
		t.Fatal(err)
	}
	var want []int
	for _, ym := range []int{202607, 202609} {
		if ym < cur {
			want = append(want, ym)
		}
	}
	if len(months) != len(want) || (len(want) > 0 && months[0] != want[0]) {
		t.Fatalf("A5 months = %v, want %v", months, want)
	}

	// A4: an unbalanced September journal counts from September, not from
	// October onward.
	res, err := testMods.db.Exec(`INSERT INTO finance_journal_entries (number, branch_id, entry_date, memo) VALUES ('JU-RUSAK', ?, '2026-09-20 10:00:00', 'uji')`, fx.branchID)
	if err != nil {
		t.Fatal(err)
	}
	entryID, _ := res.LastInsertId()
	if _, err := testMods.db.Exec(`INSERT INTO finance_journal_lines (entry_id, account_id, debit) VALUES (?, 1, 500)`, entryID); err != nil {
		t.Fatal(err)
	}
	a4, err := fin.UnbalancedJournals(ctx(), fx.branchID, "2026-09-01", 5)
	if err != nil || a4.Count != 1 || a4.Sample[0].Label != "JU-RUSAK" {
		t.Fatalf("A4 from September = %+v, %v", a4, err)
	}
	if a4, _ = fin.UnbalancedJournals(ctx(), fx.branchID, "2026-10-01", 5); a4.Count != 0 {
		t.Fatalf("A4 from October = %d, want 0", a4.Count)
	}

	var after int64
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM finance_inventory_cost_movements`).Scan(&after)
	if after != before+1 { // +1 = the delivery row this test inserted itself
		t.Fatalf("health checks wrote ledger rows: before %d after %d", before, after)
	}
}

// TestDataHealthMasterAndStockChecks: A6 negative stock, A8 min price above
// selling + the products priceIssue filter, A9/A10 duplicate names + the
// duplicateName list filters (case and spaces ignored, archived not counted).
func TestDataHealthMasterAndStockChecks(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if r, _ := testMods.stock.Health().NegativeSummary(ctx(), fx.branchID, 5); r.Count != 0 {
		t.Fatalf("A6 on a clean shop = %d", r.Count)
	}
	if _, err := testMods.db.Exec(`UPDATE stock_balances SET on_hand = -2 WHERE branch_id = ? AND product_id = ?`, fx.branchID, fx.productID); err != nil {
		t.Fatal(err)
	}
	if r, err := testMods.stock.Health().NegativeSummary(ctx(), fx.branchID, 5); err != nil || r.Count != 1 || r.Meta["products"] != 1 || !strings.HasPrefix(r.Sample[0].Label, "TST-001") {
		t.Fatalf("A6 = %+v, %v", r, err)
	}

	bad := healthProduct(t, "MP-1", "Semen Mahal", 50000, 60000)
	healthProduct(t, "MP-2", "Semen Murah", 50000, 40000)
	healthProduct(t, "DN-1", "Pasir  Hitam", 1000, 0)
	healthProduct(t, "DN-2", " pasir hitam ", 1000, 0)
	prod := testMods.product.Health()
	if r, err := prod.MinPriceAboveSelling(ctx(), 5); err != nil || r.Count != 1 || !strings.HasPrefix(r.Sample[0].Label, "MP-1") {
		t.Fatalf("A8 = %+v, %v", r, err)
	}
	list, err := testMods.product.Service().List(ctx(), productapp.ListFilter{PriceIssue: productapp.PriceIssueMinAboveSelling}, 1, 20)
	if err != nil || list.Total != 1 || list.Products[0].ID != bad {
		t.Fatalf("priceIssue filter = %+v, %v", list, err)
	}
	// "Pasir  Hitam" (double space inside) differs from "pasir hitam": only
	// case and OUTER spaces are ignored, so no group yet.
	if r, _ := prod.DuplicateNames(ctx(), 5); r.Count != 0 {
		t.Fatalf("A9 = %+v, want none", r)
	}
	healthProduct(t, "DN-3", "PASIR HITAM", 1000, 0)
	r, err := prod.DuplicateNames(ctx(), 5)
	if err != nil || r.Count != 1 || r.Meta["rows"] != 2 {
		t.Fatalf("A9 = %+v, %v; want 1 group of 2", r, err)
	}
	list, _ = testMods.product.Service().List(ctx(), productapp.ListFilter{DuplicateName: true}, 1, 20)
	if list.Total != 2 {
		t.Fatalf("duplicateName filter = %d products, want 2", list.Total)
	}
	// Fixing the price clears A8.
	if _, err := testMods.db.Exec(`UPDATE products SET min_selling_price = 0 WHERE id = ?`, bad); err != nil {
		t.Fatal(err)
	}
	if r, _ := prod.MinPriceAboveSelling(ctx(), 5); r.Count != 0 {
		t.Fatalf("A8 after fix = %d", r.Count)
	}

	for _, name := range []string{"Toko Maju", "toko maju "} {
		if _, err := testMods.party.Service().CreateCustomer(ctx(), 0, partyapp.PartyInput{Name: name, Phone: "081234567890"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := testMods.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Toko Maju", Phone: "081234567891"}); err != nil {
		t.Fatal(err)
	}
	pr, err := testMods.party.Health().DuplicateNames(ctx(), "customer", 5)
	if err != nil || pr.Count != 1 || pr.Meta["rows"] != 2 {
		t.Fatalf("A10 = %+v, %v; suppliers must not join the customer groups", pr, err)
	}
	parties, err := testMods.party.Service().List(ctx(), partyapp.ListFilter{Type: "customer", DuplicateName: true}, 1, 20)
	if err != nil || parties.Total != 2 {
		t.Fatalf("customers duplicateName filter = %+v, %v", parties, err)
	}
}

// TestDataHealthDeliveryUnsigned: A11 counts regular notes (driver set)
// confirmed without a signature status; a counter (POS) hand-over has no
// driver and never counts, and a recorded "missing" is a decision. The list
// filter signature=none matches the same notes.
func TestDataHealthDeliveryUnsigned(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	ship := func(qty float64, doc deliveryapp.DocInput) int64 {
		t.Helper()
		soID := sellFixture(t, ctx(), fx, qty)
		dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "", "", doc, []deliveryapp.LineInput{{
			ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: qty}})
		if err != nil {
			t.Fatalf("create delivery: %v", err)
		}
		if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
			t.Fatalf("confirm delivery: %v", err)
		}
		return dn.ID
	}
	regular := ship(1, deliveryapp.DocInput{DriverName: "Pak Sopir", VehiclePlate: "B 1 X"})
	counter := ship(1, deliveryapp.DocInput{})
	decided := ship(1, deliveryapp.DocInput{DriverName: "Pak Sopir"})
	if _, err := m.db.Exec(`UPDATE delivery_notes SET recipient_signature_status = 'missing', recipient_signature_missing_reason = 'penerima tidak ada' WHERE id = ?`, decided); err != nil {
		t.Fatal(err)
	}
	var driver *string
	_ = m.db.QueryRow(`SELECT driver_name FROM delivery_notes WHERE id = ?`, counter).Scan(&driver)
	if driver != nil && *driver != "" {
		t.Fatalf("a counter hand-over must not carry a driver (A11 marker), got %q", *driver)
	}
	r, err := m.delivery.Health().UnsignedSummary(ctx(), fx.branchID, 5)
	if err != nil || r.Count != 1 || r.AmountRp == nil || *r.AmountRp <= 0 || r.OldestAt == "" {
		t.Fatalf("A11 = %+v, %v; want only the regular unsigned note", r, err)
	}
	list, err := m.delivery.Service().ListFiltered(ctx(), fx.branchID, deliveryapp.ListFilter{Statuses: []string{"confirmed"}, Signature: "none"}, 1, 20)
	if err != nil || list.Total != 1 || list.Notes[0].ID != regular {
		t.Fatalf("signature=none filter = %+v, %v", list, err)
	}
	if list, _ = m.delivery.Service().ListFiltered(ctx(), fx.branchID, deliveryapp.ListFilter{Signature: "missing"}, 1, 20); list.Total != 1 {
		t.Fatalf("signature=missing filter = %d", list.Total)
	}
}

// TestDataHealthPaidUnshippedAndSnooze: A12 composes unshipped orders with
// payments (paymentStatus=received lists the same orders), and the snooze
// round-trip writes its audit rows in the snooze transaction.
func TestDataHealthPaidUnshippedAndSnooze(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	paidID := sellFixture(t, ctx(), fx, 2)
	sellFixture(t, ctx(), fx, 1) // confirmed, unpaid: not an alert
	total := orderTotal(t, paidID)
	if _, err := testMods.payment.Service().Create(ctx(), 0, fx.branchID, fx.customerID, "cash", "", "", total/2, payAll(paidID, total/2)); err != nil {
		t.Fatalf("down payment: %v", err)
	}
	h := testMods.dashboard.Health()
	ownerView := dashboardapp.Viewer{UserID: 0, RoleCode: "owner", Security: true}
	page, err := h.DataHealth(ctx(), fx.branchID, ownerView, true)
	if err != nil {
		t.Fatal(err)
	}
	var a12 *int64
	for _, area := range page.Areas {
		for _, a := range area.Items {
			if a.ID == dashboardapp.AlertPaidUnshipped {
				if a.Count != 1 {
					t.Fatalf("A12 count = %d", a.Count)
				}
				a12 = a.AmountRp
			}
		}
	}
	if a12 == nil || *a12 != total/2 {
		t.Fatalf("A12 amount = %v, want %d", a12, total/2)
	}
	orders, err := testMods.sales.Service().List(ctx(), fx.branchID, salesapp.ListFilter{PaymentStatus: salesapp.PayReceived}, 1, 20)
	if err != nil || orders.Total != 1 || orders.Orders[0].ID != paidID {
		t.Fatalf("paymentStatus=received = %+v, %v", orders, err)
	}

	got, err := h.Snooze(ctx(), fx.branchID, ownerView, dashboardapp.AlertPaidUnshipped, dashboardapp.SnoozeInput{Days: 3, Reason: "Menunggu armada"})
	if err != nil {
		t.Fatalf("snooze: %v", err)
	}
	if got.Snooze.SnoozedUntil == nil || got.Snooze.CountAtSnooze == nil || *got.Snooze.CountAtSnooze != 1 {
		t.Fatalf("snooze state = %+v", got.Snooze)
	}
	var rows, audits int64
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM dashboard_alert_snoozes WHERE branch_id = ?`, fx.branchID).Scan(&rows)
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action_key = 'data_health_snoozes.create' AND branch_id = ?`, fx.branchID).Scan(&audits)
	if rows != 1 || audits != 1 {
		t.Fatalf("snooze rows %d audits %d, want 1/1", rows, audits)
	}
	// Re-snoozing replaces the one row (unique per branch+alert).
	if _, err := h.Snooze(ctx(), fx.branchID, ownerView, dashboardapp.AlertPaidUnshipped, dashboardapp.SnoozeInput{Days: 7, Reason: "Armada servis"}); err != nil {
		t.Fatal(err)
	}
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM dashboard_alert_snoozes WHERE branch_id = ?`, fx.branchID).Scan(&rows)
	if rows != 1 {
		t.Fatalf("re-snooze rows = %d, want 1", rows)
	}
	page, _ = h.DataHealth(ctx(), fx.branchID, ownerView, false)
	if len(page.Snoozed) != 1 || page.Snoozed[0].ID != dashboardapp.AlertPaidUnshipped {
		t.Fatalf("snoozed = %+v", page.Snoozed)
	}
	if existed, err := h.Unsnooze(ctx(), fx.branchID, ownerView, dashboardapp.AlertPaidUnshipped); err != nil || !existed {
		t.Fatalf("unsnooze = %v %v", existed, err)
	}
	_ = testMods.db.QueryRow(`SELECT COUNT(*) FROM audit_logs WHERE action_key LIKE 'data_health_snoozes.%' AND branch_id = ?`, fx.branchID).Scan(&audits)
	if audits != 3 {
		t.Fatalf("audit rows = %d, want create+create+delete", audits)
	}
}

// TestDataHealthSeedPassword: A1 compares in the background (ready=false
// first), caches per hash, and a password change invalidates the verdict.
// Only superadmin holders count (owner's decision).
func TestDataHealthSeedPassword(t *testing.T) {
	freshDB(t)
	m := testMods
	hash := func(p string) string {
		h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		return string(h)
	}
	for _, q := range []string{
		`INSERT INTO roles (id, code, name, is_system, is_reserved) VALUES (1, 'superadmin', 'Super Admin', 1, 1), (2, 'owner', 'Owner', 1, 0)`,
	} {
		if _, err := m.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := m.db.Exec(`INSERT INTO users (id, username, password_hash, full_name) VALUES (1, 'superadmin', ?, 'Dev'), (2, 'pemilik', ?, 'Owner')`,
		hash("superadmin123"), hash("owner123")); err != nil {
		t.Fatal(err)
	}
	if _, err := m.db.Exec(`INSERT INTO user_roles (user_id, role_id) VALUES (1, 1), (2, 2)`); err != nil {
		t.Fatal(err)
	}
	users := m.user.Health()
	wait := func() []string {
		t.Helper()
		for i := 0; i < 200; i++ {
			got, ready, err := users.SeedPasswordAccounts(ctx())
			if err != nil {
				t.Fatal(err)
			}
			if ready {
				var names []string
				for _, a := range got {
					names = append(names, a.Username)
				}
				return names
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("seed-password verdicts never became ready")
		return nil
	}
	if names := wait(); len(names) != 1 || names[0] != "superadmin" {
		t.Fatalf("A1 accounts = %v, want only superadmin (owner is out of scope)", names)
	}
	if _, err := m.db.Exec(`UPDATE users SET password_hash = ? WHERE id = 1`, hash("rahasia-baru-123")); err != nil {
		t.Fatal(err)
	}
	if _, ready, _ := users.SeedPasswordAccounts(ctx()); ready {
		t.Fatal("a changed hash must invalidate the cached verdict")
	}
	if names := wait(); len(names) != 0 {
		t.Fatalf("A1 after password change = %v, want none", names)
	}
}
