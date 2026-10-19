package integration

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/xuri/excelize/v2"

	deliveryapp "mini-erp/internal/modules/delivery/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	salesapp "mini-erp/internal/modules/sales/application"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	userapp "mini-erp/internal/modules/user/application"
	usercontracts "mini-erp/internal/modules/user/contracts"
	"mini-erp/internal/shared/tableexport"
)

// netOrder creates and confirms a net-terms SO of qty x 15.000 due on dueDate.
func netOrder(t *testing.T, fx shopFixture, qty float64, dueDate string) int64 {
	t.Helper()
	so, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "net", DueDate: dueDate, TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: qty}},
	})
	if err != nil {
		t.Fatalf("create net SO: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm net SO: %v", err)
	}
	return so.ID
}

func payOrder(t *testing.T, fx shopFixture, actorID, soID, amount int64) {
	t.Helper()
	if _, err := testMods.payment.Service().Create(ctx(), actorID, fx.branchID, fx.customerID, "cash", "", "", amount,
		[]paymentapp.AllocationInput{{OrderType: paymentcontracts.OrderSales, OrderID: soID, Amount: amount}}); err != nil {
		t.Fatalf("pay order %d: %v", soID, err)
	}
}

func summaryOf(t *testing.T, soID int64) salesapp.PaymentSummary {
	t.Helper()
	o, err := testMods.sales.Service().GetByID(ctx(), soID)
	if err != nil || o == nil {
		t.Fatalf("get SO %d: %v", soID, err)
	}
	sums, err := testMods.sales.Service().PaymentSummaries(ctx(), []*salescontracts.SalesOrder{o})
	if err != nil {
		t.Fatalf("summaries: %v", err)
	}
	return sums[soID]
}

// TestSalesOrderListPaymentStatusFilter (SL-02): the list filters by payment
// status on the server (paid amounts live in the payment module), keeps drafts
// and cancelled orders out, honours overdue, paginates by the matches and the
// export follows the same filter.
func TestSalesOrderListPaymentStatusFilter(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	overdueUnpaid := netOrder(t, fx, 1, "2026-01-05") // 15.000, due long ago, nothing paid
	futureUnpaid := netOrder(t, fx, 1, "2099-01-01")  // 15.000, not due
	partial := netOrder(t, fx, 2, "2099-01-01")       // 30.000, 10.000 paid
	paid := netOrder(t, fx, 1, "2026-01-05")          // 15.000, fully paid (past due but settled)
	payOrder(t, fx, 0, partial, 10000)
	payOrder(t, fx, 0, paid, 15000)
	if _, err := m.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{ // draft
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	}); err != nil {
		t.Fatalf("draft: %v", err)
	}

	ids := func(f salesapp.ListFilter, page, limit int) ([]int64, int64) {
		t.Helper()
		res, err := m.sales.Service().List(ctx(), fx.branchID, f, page, limit)
		if err != nil {
			t.Fatalf("list %+v: %v", f, err)
		}
		out := make([]int64, 0, len(res.Orders))
		for _, o := range res.Orders {
			out = append(out, o.ID)
		}
		return out, res.Total
	}
	same := func(got []int64, want ...int64) bool {
		if len(got) != len(want) {
			return false
		}
		for i := range got {
			if got[i] != want[i] {
				return false
			}
		}
		return true
	}

	if got, total := ids(salesapp.ListFilter{PaymentStatus: "unpaid"}, 1, 10); !same(got, futureUnpaid, overdueUnpaid) || total != 2 {
		t.Fatalf("unpaid = %v (total %d), want [%d %d]", got, total, futureUnpaid, overdueUnpaid)
	}
	if got, total := ids(salesapp.ListFilter{PaymentStatus: "partial"}, 1, 10); !same(got, partial) || total != 1 {
		t.Fatalf("partial = %v (total %d), want [%d]", got, total, partial)
	}
	if got, total := ids(salesapp.ListFilter{PaymentStatus: "paid"}, 1, 10); !same(got, paid) || total != 1 {
		t.Fatalf("paid = %v (total %d), want [%d]", got, total, paid)
	}
	// Overdue = net, past due AND a balance: the settled past-due order is out.
	if got, total := ids(salesapp.ListFilter{PaymentStatus: "overdue"}, 1, 10); !same(got, overdueUnpaid) || total != 1 {
		t.Fatalf("overdue = %v (total %d), want [%d]", got, total, overdueUnpaid)
	}
	// Pagination counts the matches, not the table.
	if got, total := ids(salesapp.ListFilter{PaymentStatus: "unpaid"}, 2, 1); !same(got, overdueUnpaid) || total != 2 {
		t.Fatalf("unpaid page 2 = %v (total %d), want [%d] total 2", got, total, overdueUnpaid)
	}
	// Only drafts asked for: nothing can match.
	if got, total := ids(salesapp.ListFilter{PaymentStatus: "unpaid", Statuses: []string{"draft"}}, 1, 10); len(got) != 0 || total != 0 {
		t.Fatalf("draft + unpaid = %v (total %d), want empty", got, total)
	}
	// Sorting is honoured on the filtered path (total ascending).
	if got, _ := ids(salesapp.ListFilter{PaymentStatus: "unpaid", OrderBy: "grand_total ASC"}, 1, 10); len(got) != 2 {
		t.Fatalf("sorted unpaid = %v", got)
	}
	if _, err := m.sales.Service().List(ctx(), fx.branchID, salesapp.ListFilter{PaymentStatus: "bogus"}, 1, 10); err == nil {
		t.Fatal("unknown payment status must be rejected")
	}

	// The export follows the filter: only the partial order's number appears.
	file, err := m.sales.Service().Export(ctx(), fx.branchID, salesapp.ListFilter{PaymentStatus: "partial"}, "xlsx")
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	defer file.Close()
	rows := readSheet(t, file, "Daftar Order")
	if len(rows) != 2 { // header + 1 order
		t.Fatalf("export rows = %d, want header + 1 order", len(rows))
	}
}

// readSheet streams the download into memory and reads one sheet back.
func readSheet(t *testing.T, file *tableexport.File, name string) [][]string {
	t.Helper()
	var buf bytes.Buffer
	if err := file.Send(&buf); err != nil {
		t.Fatalf("send: %v", err)
	}
	wb, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer func() { _ = wb.Close() }()
	rows, err := wb.GetRows(name)
	if err != nil {
		t.Fatalf("rows %s: %v", name, err)
	}
	return rows
}

// TestSalesOrderSummaryNetOfReturns (SL-03): the order's money picture counts
// the confirmed returns (net total = books), keeps the automatic offset out of
// "paid", never goes negative and shows what is owed back to the customer.
func TestSalesOrderSummaryNetOfReturns(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	// Unpaid net order of 2 x 15.000; one unit comes back: the return offsets
	// the open bill, so the customer simply owes 15.000 less.
	so := netOrder(t, fx, 2, "2099-01-01")
	deliverAll(t, ctx(), fx, so, 2)
	ret := createSalesReturn(t, fx, so, 1)
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	sum := summaryOf(t, so)
	if sum.OriginalTotal != 30000 || sum.ReturnAdjustment != -15000 || sum.NetTotal != 15000 {
		t.Fatalf("totals = original %d adjustment %d net %d, want 30000/-15000/15000", sum.OriginalTotal, sum.ReturnAdjustment, sum.NetTotal)
	}
	if sum.ReturnOffset != 15000 || sum.PaidAmount != 0 || sum.OutstandingAmount != 15000 || sum.RefundDue != 0 {
		t.Fatalf("offset %d paid %d outstanding %d refundDue %d, want 15000/0/15000/0", sum.ReturnOffset, sum.PaidAmount, sum.OutstandingAmount, sum.RefundDue)
	}
	if sum.PaymentStatus != "unpaid" {
		t.Fatalf("status = %q, want unpaid (nothing paid in cash)", sum.PaymentStatus)
	}

	// Paid in full BEFORE the return: no open bill, so the value comes back as a
	// refund due — the balance is zero, never negative, and a refund clears it.
	paidSO := netOrder(t, fx, 2, "2099-01-01")
	deliverAll(t, ctx(), fx, paidSO, 2)
	payOrder(t, fx, 0, paidSO, 30000)
	ret2 := createSalesReturn(t, fx, paidSO, 1)
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret2.ID); err != nil {
		t.Fatalf("confirm return 2: %v", err)
	}
	sum = summaryOf(t, paidSO)
	if sum.NetTotal != 15000 || sum.PaidAmount != 30000 || sum.OutstandingAmount != 0 || sum.RefundDue != 15000 || sum.PaymentStatus != "paid" {
		t.Fatalf("paid-then-returned = %+v, want net 15000 paid 30000 outstanding 0 refundDue 15000 paid", sum)
	}
	if _, err := m.salesreturn.Service().AddSettlement(ctx(), 0, fx.branchID, ret2.ID,
		salesreturnapp.SettlementInput{Type: "refund", Amount: 15000}); err != nil {
		t.Fatalf("refund settlement: %v", err)
	}
	sum = summaryOf(t, paidSO)
	if sum.RefundDue != 0 || sum.ReturnRefunded != 15000 || sum.PaidAmount != 15000 || sum.OutstandingAmount != 0 {
		t.Fatalf("after refund = %+v, want refundDue 0 refunded 15000 paid 15000 outstanding 0", sum)
	}

	// The list view carries the same figures (one grouped read per page).
	res, err := m.sales.Service().List(ctx(), fx.branchID, salesapp.ListFilter{}, 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	sums, err := m.sales.Service().PaymentSummaries(ctx(), res.Orders)
	if err != nil {
		t.Fatalf("list summaries: %v", err)
	}
	if sums[so].NetTotal != 15000 || sums[paidSO].RefundDue != 0 {
		t.Fatalf("list summaries inconsistent: %+v / %+v", sums[so], sums[paidSO])
	}
}

// TestSalesOrderExportWorkbook (SL-04): three sheets with the legacy columns,
// values that match the order, and a keyset walk that stays correct across the
// 500-row page boundary.
func TestSalesOrderExportWorkbook(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	role, err := m.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "R-kasir", Name: "Kasir"})
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	kasir, err := m.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
		Username: "kasir1", FullName: "Sari Kasir", Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID},
		Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
	})
	if err != nil {
		t.Fatalf("user: %v", err)
	}

	so, err := m.sales.Service().CreateOrder(ctx(), kasir.ID, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "exclude", TaxRate: 11,
		AssignedUserID: kasir.ID, Notes: "kirim pagi", TaxInvoiceNumber: "010.000-26.00000009",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2, DiscountNominal: 1000}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.sales.Service().ConfirmOrder(ctx(), kasir.ID, fx.branchID, so.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	dn, err := m.delivery.Service().Create(ctx(), 0, fx.branchID, so.ID, "2026-10-12", "",
		deliveryapp.DocInput{DriverName: "Pak Budi", VehiclePlate: "B 1234 XY"},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("delivery: %v", err)
	}
	if _, err := m.delivery.Service().Confirm(ctx(), 0, fx.branchID, dn.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm delivery: %v", err)
	}
	// Net now; part payment by the cashier.
	payOrder(t, fx, kasir.ID, so.ID, 10000)

	// 1100 filler orders (no lines) push the walk across two full keyset pages.
	if _, err := m.db.Exec(`INSERT /*+ SET_VAR(cte_max_recursion_depth=5000) */ INTO sales_orders
		(number, branch_id, party_id, channel, order_date, payment_terms, tax_type, status, grand_total, subtotal)
		WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 1100)
		SELECT CONCAT('BULK-', n), ?, 0, 'regular', '2026-10-01 00:00:00', 'cod', 'none', 'confirmed', 1000, 1000 FROM seq`, fx.branchID); err != nil {
		t.Fatalf("bulk insert: %v", err)
	}

	file, err := m.sales.Service().ExportWith(ctx(), fx.branchID, salesapp.ListFilter{}, "xlsx", salesapp.ExportOptions{})
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	defer file.Close()
	var buf bytes.Buffer
	if err := file.Send(&buf); err != nil {
		t.Fatalf("send: %v", err)
	}
	wb, err := excelize.OpenReader(&buf)
	if err != nil {
		t.Fatalf("open workbook: %v", err)
	}
	defer func() { _ = wb.Close() }()
	if got := wb.GetSheetList(); len(got) != 3 || got[0] != "Ringkasan" || got[1] != "Daftar Order" || got[2] != "Detail Item" {
		t.Fatalf("sheets = %v, want Ringkasan/Daftar Order/Detail Item", got)
	}
	list, err := wb.GetRows("Daftar Order")
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(list) != 1+1+1100 {
		t.Fatalf("Daftar Order rows = %d, want header + 1 order + 1100 filler (every page read once)", len(list))
	}
	if len(list[0]) < 22 {
		t.Fatalf("Daftar Order has %d columns, want >= 22", len(list[0]))
	}
	col := map[string]int{}
	for i, h := range list[0] {
		col[h] = i
	}
	var row []string
	for _, r := range list[1:] {
		if r[col["Nomor Order"]] == so.Number {
			row = r
		}
	}
	if row == nil {
		t.Fatalf("order %s missing from the export", so.Number)
	}
	want := map[string]string{
		"Customer": "Customer Uji", "Termin": "Net / Tempo", "Dibayar": "10000", "Sisa": "19540",
		"Metode Bayar": "Tunai", "Kasir": "Sari Kasir", "Dibuat Oleh": "Sari Kasir", "Penanggung Jawab": "Sari Kasir",
		"Sopir": "Pak Budi", "No Plat": "B 1234 XY", "No Faktur Pajak": "010.000-26.00000009",
		"Catatan": "kirim pagi", "Status Bayar": "Bayar Sebagian", "Status Order": "Selesai",
	}
	// 2 x 15.000 - 1.000 = 29.000 net; PPN 11% = 3.190; total 32.190; paid 10.000.
	want["Total Order"] = "32190"
	want["Pajak"] = "3190"
	want["DPP"] = "29000"
	want["Total Bersih"] = "32190"
	want["Sisa"] = "22190"
	for name, v := range want {
		if got := row[col[name]]; got != v {
			t.Errorf("column %q = %q, want %q", name, got, v)
		}
	}

	items, err := wb.GetRows("Detail Item")
	if err != nil {
		t.Fatalf("item rows: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("Detail Item rows = %d, want header + 1 line", len(items))
	}
	icol := map[string]int{}
	for i, h := range items[0] {
		icol[h] = i
	}
	if items[1][icol["Kode Produk"]] != "TST-001" || items[1][icol["Qty"]] != "2" || items[1][icol["Total Baris"]] != "32190" {
		t.Fatalf("item row = %v", items[1])
	}

	summary, err := wb.GetRows("Ringkasan")
	if err != nil {
		t.Fatalf("summary rows: %v", err)
	}
	figures := map[string]string{}
	for _, r := range summary {
		if len(r) >= 2 {
			figures[r[0]] = r[1]
		}
	}
	// 1 real order (32.190) + 1100 fillers of 1.000.
	if figures["Jumlah Order"] != strconv.Itoa(1101) {
		t.Errorf("Jumlah Order = %q, want 1101", figures["Jumlah Order"])
	}
	if figures["Total Nilai Order"] != strconv.Itoa(32190+1100*1000) {
		t.Errorf("Total Nilai Order = %q, want %d", figures["Total Nilai Order"], 32190+1100*1000)
	}
	if figures["Total Dibayar"] != "10000" {
		t.Errorf("Total Dibayar = %q, want 10000", figures["Total Dibayar"])
	}
	if _, ok := figures["Margin Kotor (estimasi)"]; ok {
		t.Error("margin line must be absent without the permission option")
	}
}

// TestSalesOrderPeopleAndCreditApproval (SL-09/SL-10): the order carries who
// created it, who is in charge and who approved the credit (and when);
// ApproveCredit accepts cod and prepaid alike (same eligibility as
// SwitchToNet) and refuses net; the cashier is whoever recorded the payment.
func TestSalesOrderPeopleAndCreditApproval(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	role, err := m.user.Service().CreateRole(ctx(), 0, userapp.CreateRoleInput{Code: "R-staf", Name: "Staf"})
	if err != nil {
		t.Fatalf("role: %v", err)
	}
	mk := func(username, full string) int64 {
		u, err := m.user.Service().CreateUser(ctx(), 0, userapp.CreateUserInput{
			Username: username, FullName: full, Password: "rahasia-kuat-1", RoleIDs: []int64{role.ID},
			Branches: []usercontracts.BranchAccess{{BranchID: fx.branchID, IsDefault: true}},
		})
		if err != nil {
			t.Fatalf("user %s: %v", username, err)
		}
		return u.ID
	}
	creator, owner, approver, cashier := mk("pembuat", "Dewi Pembuat"), mk("pj", "Rudi PJ"), mk("penyetuju", "Ani Penyetuju"), mk("kasir2", "Tono Kasir")

	line := []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}}
	mkOrder := func(terms string, assigned int64) *salescontracts.SalesOrder {
		o, err := m.sales.Service().CreateOrder(ctx(), creator, fx.branchID, salesapp.OrderInput{
			PartyID: fx.customerID, Channel: "regular", PaymentTerms: terms, TaxType: "none",
			AssignedUserID: assigned, Items: line,
		})
		if err != nil {
			t.Fatalf("create %s: %v", terms, err)
		}
		if _, err := m.sales.Service().ConfirmOrder(ctx(), creator, fx.branchID, o.ID); err != nil {
			t.Fatalf("confirm %s: %v", terms, err)
		}
		return o
	}

	cod := mkOrder("cod", owner)
	got, err := m.sales.Service().GetByID(ctx(), cod.ID)
	if err != nil || got == nil {
		t.Fatalf("get: %v", err)
	}
	if err := m.sales.Service().DecorateDetail(ctx(), []*salescontracts.SalesOrder{got}); err != nil {
		t.Fatalf("decorate: %v", err)
	}
	if got.CreatedBy != creator || got.CreatedByName != "Dewi Pembuat" || got.AssignedUserID != owner || got.AssignedUserName != "Rudi PJ" {
		t.Fatalf("people = created %d/%q assigned %d/%q", got.CreatedBy, got.CreatedByName, got.AssignedUserID, got.AssignedUserName)
	}
	if got.CreditApprovedBy != 0 || got.CreditApprovedAt != "" {
		t.Fatalf("credit stamped before approval: %+v", got)
	}

	// Unknown / unknown-active user cannot be the person in charge.
	if _, err := m.sales.Service().CreateOrder(ctx(), creator, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "cod", TaxType: "none", AssignedUserID: 987654, Items: line,
	}); err == nil {
		t.Fatal("unknown assigned user must be refused")
	}

	// ApproveCredit: cod and prepaid both switch and stamp; net is refused.
	appr, err := m.sales.Service().ApproveCredit(ctx(), approver, fx.branchID, cod.ID, "2099-02-01")
	if err != nil {
		t.Fatalf("approve cod: %v", err)
	}
	if appr.PaymentTerms != "net" || appr.CreditApprovedBy != approver || appr.CreditApprovedAt == "" {
		t.Fatalf("approved order = terms %s by %d at %q", appr.PaymentTerms, appr.CreditApprovedBy, appr.CreditApprovedAt)
	}
	if _, err := m.sales.Service().ApproveCredit(ctx(), approver, fx.branchID, cod.ID, "2099-02-01"); err == nil {
		t.Fatal("approving an order that is already net must be refused")
	}
	prepaid := mkOrder("prepaid", 0)
	appr2, err := m.sales.Service().ApproveCredit(ctx(), approver, fx.branchID, prepaid.ID, "2099-02-01")
	if err != nil {
		t.Fatalf("approve prepaid: %v", err)
	}
	if appr2.PaymentTerms != "net" || appr2.CreditApprovedBy != approver {
		t.Fatalf("prepaid approval = %+v", appr2)
	}
	// SwitchToNet (delivery settlement path) accepts the same eligibility.
	prepaid2 := mkOrder("prepaid", 0)
	if err := m.sales.Service().SwitchToNet(ctx(), fx.branchID, prepaid2.ID, "2099-03-01", approver); err != nil {
		t.Fatalf("switch prepaid: %v", err)
	}

	// Names + approval render on the detail read.
	d, _ := m.sales.Service().GetByID(ctx(), cod.ID)
	_ = m.sales.Service().DecorateDetail(ctx(), []*salescontracts.SalesOrder{d})
	if d.CreditApprovedByName != "Ani Penyetuju" {
		t.Fatalf("credit approver name = %q", d.CreditApprovedByName)
	}

	// Cashier = the user who recorded the payment, not the order's creator.
	payOrder(t, fx, cashier, cod.ID, 5000)
	d, _ = m.sales.Service().GetByID(ctx(), cod.ID)
	_ = m.sales.Service().DecorateDetail(ctx(), []*salescontracts.SalesOrder{d})
	if d.CashierName != "Tono Kasir" {
		t.Fatalf("cashier = %q, want Tono Kasir", d.CashierName)
	}

	// The list page names the creators in one batch.
	res, err := m.sales.Service().List(ctx(), fx.branchID, salesapp.ListFilter{}, 1, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if err := m.sales.Service().DecorateList(ctx(), res.Orders); err != nil {
		t.Fatalf("decorate list: %v", err)
	}
	for _, o := range res.Orders {
		if o.CreatedByName != "Dewi Pembuat" {
			t.Fatalf("list order %d created-by name = %q", o.ID, o.CreatedByName)
		}
	}
}
