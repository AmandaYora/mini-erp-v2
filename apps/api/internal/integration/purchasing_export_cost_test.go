package integration

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	goodsreceiptapp "mini-erp/internal/modules/goodsreceipt/application"
	partyapp "mini-erp/internal/modules/party/application"
	paymentapp "mini-erp/internal/modules/payment/application"
	productapp "mini-erp/internal/modules/product/application"
	purchasingapp "mini-erp/internal/modules/purchasing/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	stockapp "mini-erp/internal/modules/stock/application"
	"mini-erp/internal/shared/apperror"
)

// PO-02: the PO export is a three-sheet workbook (Ringkasan, Daftar Order,
// Detail Item) with paid/outstanding/DPP/financial status/method per order,
// the active filter is applied, and the keyset walk reads every page once.
func TestPurchaseOrderExportWorkbook(t *testing.T) {
	freshDB(t)
	fx := purchaseProduct(t, newShop(t, ctx()))
	m := testMods
	sup, err := m.party.Service().CreateSupplier(ctx(), 0, partyapp.PartyInput{Name: "Pemasok Ekspor"})
	if err != nil {
		t.Fatalf("supplier: %v", err)
	}
	po, err := m.purchasing.Service().CreateOrder(ctx(), 0, fx.branchID, purchasingapp.OrderInput{
		PartyID: sup.ID, PaymentTerms: "net", DueDate: "2037-12-31", TaxType: "exclude", TaxRate: 11,
		SupplierInvoiceNumber: "FKT-EXP-1", TaxInvoiceNumber: "010.000-26.00000077", Notes: "ekspor",
		Items: []purchasingapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 2, UnitPrice: 10000}},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := m.purchasing.Service().ConfirmOrder(ctx(), 0, fx.branchID, po.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := m.goodsreceipt.Service().Create(ctx(), 0, fx.branchID, po.ID, "2026-09-10", "",
		[]goodsreceiptapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}}); err != nil {
		t.Fatalf("receive: %v", err)
	}
	if _, err := m.payment.Service().Create(ctx(), 0, fx.branchID, sup.ID, "cash", "", "", 5000,
		[]paymentapp.AllocationInput{{OrderType: "purchase", OrderID: po.ID, Amount: 5000}}); err != nil {
		t.Fatalf("pay: %v", err)
	}
	// 1100 filler orders push the walk across two full keyset pages.
	if _, err := m.db.Exec(`INSERT /*+ SET_VAR(cte_max_recursion_depth=5000) */ INTO purchase_orders
		(number, branch_id, party_id, order_date, payment_terms, tax_type, status, grand_total, subtotal)
		WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 1100)
		SELECT CONCAT('PBULK-', n), ?, ?, '2026-10-01 00:00:00', 'cod', 'none', 'confirmed', 1000, 1000 FROM seq`, fx.branchID, sup.ID); err != nil {
		t.Fatalf("bulk insert: %v", err)
	}

	open := func(f purchasingapp.ListFilter) *excelize.File {
		t.Helper()
		file, err := m.purchasing.Service().Export(ctx(), fx.branchID, f, "xlsx")
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
		return wb
	}

	wb := open(purchasingapp.ListFilter{})
	defer func() { _ = wb.Close() }()
	if got := wb.GetSheetList(); len(got) != 3 || got[0] != "Ringkasan" || got[1] != "Daftar Order" || got[2] != "Detail Item" {
		t.Fatalf("sheets = %v", got)
	}
	list, err := wb.GetRows("Daftar Order")
	if err != nil {
		t.Fatalf("rows: %v", err)
	}
	if len(list) != 1+1+1100 {
		t.Fatalf("Daftar Order rows = %d, want header + 1 order + 1100 filler (every page read once)", len(list))
	}
	col := map[string]int{}
	for i, h := range list[0] {
		col[h] = i
	}
	for _, h := range []string{"Dibayar", "Sisa", "DPP", "Status Finansial", "Metode Bayar", "Status Bayar", "No Faktur Pajak"} {
		if _, ok := col[h]; !ok {
			t.Fatalf("column %q missing in %v", h, list[0])
		}
	}
	var row []string
	for _, r := range list[1:] {
		if len(r) > 0 && r[0] == po.Number {
			row = r
		}
	}
	if row == nil {
		t.Fatal("the PO row is missing")
	}
	want := map[string]string{
		"Total Order": "22200", "DPP": "20000", "Pajak": "2200", "Dibayar": "5000", "Sisa": "17200",
		"Status Bayar": "Bayar Sebagian", "Status Finansial": "Dibayar Sebagian", "Metode Bayar": "cash",
		"No Faktur Supplier": "FKT-EXP-1", "No Faktur Pajak": "010.000-26.00000077", "Supplier": "Pemasok Ekspor",
	}
	for k, v := range want {
		if got := row[col[k]]; got != v {
			t.Fatalf("%s = %q, want %q (row %v)", k, got, v, row)
		}
	}
	items, _ := wb.GetRows("Detail Item")
	if len(items) != 2 || items[1][0] != po.Number {
		t.Fatalf("Detail Item rows = %v, want header + the PO's one line", items)
	}
	sum, _ := wb.GetRows("Ringkasan")
	flat := ""
	for _, r := range sum {
		flat += strings.Join(r, "|") + "\n"
	}
	if !strings.Contains(flat, "Jumlah Order|1101") || !strings.Contains(flat, "Total Dibayar|5000") {
		t.Fatalf("summary = %s", flat)
	}

	// The active filter rides along: a search for the PO number exports just it.
	wb2 := open(purchasingapp.ListFilter{Search: po.Number})
	defer func() { _ = wb2.Close() }()
	list2, _ := wb2.GetRows("Daftar Order")
	if len(list2) != 2 || list2[1][0] != po.Number {
		t.Fatalf("filtered rows = %v, want header + the PO", list2)
	}
}

func costProduct(t *testing.T, code string, price int64) productapp.ProductInput {
	t.Helper()
	return productapp.ProductInput{
		Code: code, Name: "Barang " + code, Type: "barang", Tracked: true,
		BaseUOM: "pcs", PurchaseUOM: "pcs", SalesUOM: "pcs",
		PurchaseFactor: 1, SalesFactor: 1, PurchasePrice: price, SellingPrice: 15000,
	}
}

// RT-04: replacement goods need a cost basis before they ship — the moving
// average finance holds first, then the catalog purchase price. With neither,
// the replacement shipment is refused.
func TestReplacementCostBasisGuard(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	repl, err := m.product.Service().CreateProduct(ctx(), 0, costProduct(t, "TST-R0", 1000))
	if err != nil {
		t.Fatalf("product: %v", err)
	}
	vid := repl.Variants[0].ID
	if err := m.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: repl.ID, VariantID: vid, Location: fx.locationID, Mode: "in", QtyDelta: 5, Reason: "stok uji tanpa biaya",
	}); err != nil {
		t.Fatalf("stock: %v", err)
	}
	// Stock needs a priced product to come in; legacy data can still end up with
	// no price and no ledger cost, which is what the guard exists for.
	if _, err := m.db.Exec(`UPDATE products SET purchase_price = 0 WHERE id = ?`, repl.ID); err != nil {
		t.Fatalf("zero price: %v", err)
	}
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}},
		[]salesreturnapp.ReplacementInput{{ProductID: repl.ID, VariantID: vid, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	costErr := func() error {
		_, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID, salesreturnapp.ReplacementDispatchInput{})
		return err
	}
	// Neither an average cost nor a catalog price: refused with a clear message.
	var ae *apperror.AppError
	if err := costErr(); !errors.As(err, &ae) || ae.Code != apperror.CodeConflict || !strings.Contains(ae.Message, "harga modal") {
		t.Fatalf("no cost basis = %v, want conflict about harga modal", err)
	}
	// Catalog price only (average still unknown) is enough.
	if _, err := testMods.db.Exec(`UPDATE products SET purchase_price = 7000 WHERE id = ?`, repl.ID); err != nil {
		t.Fatalf("set price: %v", err)
	}
	if err := costErr(); err != nil {
		t.Fatalf("catalog price fallback: %v", err)
	}
}

// RT-04: the moving average wins over a zero catalog price.
func TestReplacementCostBasisFromAverage(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	repl, err := m.product.Service().CreateProduct(ctx(), 0, costProduct(t, "TST-R1", 1000))
	if err != nil {
		t.Fatalf("product: %v", err)
	}
	if _, err := m.db.Exec(`UPDATE products SET purchase_price = 0 WHERE id = ?`, repl.ID); err != nil {
		t.Fatalf("zero price: %v", err)
	}
	vid := repl.Variants[0].ID
	rfx := fx
	rfx.productID, rfx.variantID = repl.ID, vid
	receivePO(t, rfx, 12000, 3) // posts a receipt at 12.000 into fx.locationID
	// N-42: no manual Sync — the guard drains the cost ledger itself.
	if avg, err := m.finance.Service().AverageUnitCost(ctx(), fx.branchID, repl.ID, vid); err != nil || avg <= 0 {
		t.Fatalf("average cost = %d err %v, want > 0 after the receipt", avg, err)
	}
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}},
		[]salesreturnapp.ReplacementInput{{ProductID: repl.ID, VariantID: vid, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID, salesreturnapp.ReplacementDispatchInput{}); err != nil {
		t.Fatalf("average cost must satisfy the guard with a zero catalog price: %v", err)
	}
}
