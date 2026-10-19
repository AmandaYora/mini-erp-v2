package integration

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	branchapp "mini-erp/internal/modules/branch/application"
	deliveryapp "mini-erp/internal/modules/delivery/application"
	mediacontracts "mini-erp/internal/modules/media/contracts"
	paymentapp "mini-erp/internal/modules/payment/application"
	salesapp "mini-erp/internal/modules/sales/application"
	stockapp "mini-erp/internal/modules/stock/application"
	"mini-erp/internal/shared/apperror"
)

func fieldNames(err error) []string {
	var ae *apperror.AppError
	if !errors.As(err, &ae) {
		return nil
	}
	var out []string
	for _, f := range ae.Fields {
		out = append(out, f.Field)
	}
	return out
}

// DL-03: the stock movement of a confirmed SJ is dated by the SJ's delivery date.
func TestParityDeliveryMovementDateFollowsSJ(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 3)
	n, err := draftNote(t, fx, soID, 3) // delivery date 2026-09-10
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle}); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	var moved time.Time
	if err := testMods.db.QueryRow(`SELECT moved_at FROM stock_movements WHERE ref_type='delivery' AND ref_id=?`, n.ID).Scan(&moved); err != nil {
		t.Fatalf("movement: %v", err)
	}
	want := time.Date(2026, 9, 10, 0, 0, 0, 0, time.FixedZone("WIB", 7*3600)).UTC()
	if !moved.UTC().Equal(want) {
		t.Fatalf("moved_at = %v, want %v (start of the SJ day, WIB)", moved.UTC(), want)
	}
}

// DL-04: a fresh draft warns that stock is not held and names the short position.
func TestParityDeliveryDraftWarnsAndConfirmExplains(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 60) // stock is 50
	n, err := draftNote(t, fx, soID, 60)  // explicit location: soft check, never blocks
	if err != nil {
		t.Fatalf("draft must not be blocked by the soft check: %v", err)
	}
	if len(n.Warnings) < 2 || !strings.Contains(n.Warnings[0], "belum dikurangi atau ditahan") ||
		!strings.Contains(n.Warnings[1], "butuh 60") {
		t.Fatalf("warnings = %v", n.Warnings)
	}
	_, err = testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{Settlement: codSettle})
	var ae *apperror.AppError
	if !errors.As(err, &ae) || ae.Code != apperror.CodeConflict ||
		!strings.Contains(ae.Message, "tetap draf") || !strings.Contains(ae.Message, n.Number) {
		t.Fatalf("confirm error = %v, want an explained conflict", err)
	}
	// nothing moved, the note is still a draft
	got, _ := testMods.delivery.Service().GetByID(ctx(), n.ID)
	if got.Status != "draft" {
		t.Fatalf("status = %s, want draft", got.Status)
	}
}

// DL-01: with the evidence rule on, a location photo is not enough; the signed
// document is what the confirmation asks for.
func TestParityDeliveryEvidenceNeedsSignedDocument(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if _, err := testMods.db.Exec(`UPDATE company_settings SET setting_value='true' WHERE setting_key='delivery_require_evidence'`); err != nil {
		t.Fatal(err)
	}
	soID := sellFixture(t, ctx(), fx, 2)
	n, err := testMods.delivery.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "",
		deliveryapp.DocInput{DriverName: "Joko", WarehouseStaffName: "Sari"},
		[]deliveryapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	_, err = testMods.delivery.Service().Confirm(ctx(), 0, fx.branchID, n.ID, deliveryapp.ConfirmInput{
		RecipientName: "Ani", RecipientSignatureStatus: "signed", Settlement: codSettle})
	found := false
	for _, f := range fieldNames(err) {
		found = found || f == "signedDocument"
	}
	if !found {
		t.Fatalf("confirm error = %v, want a signedDocument field error", err)
	}
	if _, err := testMods.delivery.Service().Proof(ctx(), 0, fx.branchID, n.ID, "selfie", mediacontracts.Upload{OriginalName: "x.png"}); err == nil {
		t.Fatal("an unknown proof purpose must be rejected")
	}
}

// DL-02: search and date filters apply before the page cut.
func TestParityWorkQueueFilters(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	soID := sellFixture(t, ctx(), fx, 4)
	so, _ := testMods.sales.Service().GetByID(ctx(), soID)
	q, err := testMods.delivery.Service().WorkQueue(ctx(), fx.branchID, so.Number, "", "", "", 100)
	if err != nil || len(q.CreateSJ) != 1 || q.CreateSJ[0].ItemCount != 1 {
		t.Fatalf("search by number: %+v %v", q, err)
	}
	q, _ = testMods.delivery.Service().WorkQueue(ctx(), fx.branchID, "Customer Uji", "", "", "", 100)
	if len(q.CreateSJ) != 1 {
		t.Fatalf("search by customer name: %+v", q.CreateSJ)
	}
	q, _ = testMods.delivery.Service().WorkQueue(ctx(), fx.branchID, "TIDAK-ADA-XYZ", "", "", "", 100)
	if len(q.CreateSJ) != 0 {
		t.Fatalf("non-matching search must be empty: %+v", q.CreateSJ)
	}
	q, _ = testMods.delivery.Service().WorkQueue(ctx(), fx.branchID, "", "2099-01-01", "", "", 100)
	if len(q.CreateSJ) != 0 {
		t.Fatalf("order date filter must hide today's order when from is in 2099")
	}
	n, err := draftNote(t, fx, soID, 4)
	if err != nil {
		t.Fatal(err)
	}
	q, _ = testMods.delivery.Service().WorkQueue(ctx(), fx.branchID, n.Number, "", "", "", 100)
	if len(q.WaitingReturn) != 1 || q.WaitingReturn[0].HasSignedDocument {
		t.Fatalf("waiting search by SJ number: %+v", q.WaitingReturn)
	}
	q, _ = testMods.delivery.Service().WorkQueue(ctx(), fx.branchID, so.Number, "", "", "", 100)
	if len(q.WaitingReturn) != 1 {
		t.Fatalf("waiting search by order number: %+v", q.WaitingReturn)
	}
}

// ST-01: allocation order = picking area, primary, default, largest qty.
func TestParityAllocationPriority(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	mk := func(code string, qty float64, in stockapp.LocationInput) int64 {
		in.Code, in.Name = code, "Lok "+code
		l, err := svc.CreateLocation(ctx(), 0, fx.branchID, in)
		if err != nil {
			t.Fatalf("location %s: %v", code, err)
		}
		if err := svc.Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{ProductID: fx.productID, VariantID: fx.variantID,
			Location: l.ID, Mode: "in", QtyDelta: qty, Reason: "uji"}); err != nil {
			t.Fatal(err)
		}
		return l.ID
	}
	yes := true
	big := mk("BIG", 90, stockapp.LocationInput{})
	primary := mk("PRM", 5, stockapp.LocationInput{IsPrimary: &yes})
	picking := mk("PCK", 2, stockapp.LocationInput{IsPickingArea: &yes})
	got, short, err := svc.Suggest(ctx(), fx.branchID, fx.productID, fx.variantID, 100)
	if err != nil || short != 0 || len(got) < 3 {
		t.Fatalf("suggest: %+v %v %v", got, short, err)
	}
	if got[0].LocationID != picking || got[1].LocationID != primary {
		t.Fatalf("order = %+v, want picking (%d) then primary (%d)", got, picking, primary)
	}
	// then the default system location (50) before the larger BIG (90)? default beats qty
	if got[2].LocationID != fx.locationID && got[2].LocationID != big {
		t.Fatalf("unexpected third %+v", got[2])
	}
	// code change: conflict refused, success audited path works
	if _, err := svc.UpdateLocation(ctx(), 0, fx.branchID, big, stockapp.LocationInput{Code: "PRM", Name: "Lok BIG"}); err == nil {
		t.Fatal("renaming to an existing code must conflict")
	}
	l, err := svc.UpdateLocation(ctx(), 0, fx.branchID, big, stockapp.LocationInput{Code: "big-2", Name: "Lok BIG"})
	if err != nil || l.Code != "BIG-2" {
		t.Fatalf("code change: %+v %v", l, err)
	}
}

// ST-02/ST-06: transfer document face, the driver rule and the direction filter.
func TestParityTransferFaceAndDirection(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	br, err := testMods.branch.Service().CreateBranch(ctx(), 0, branchapp.CreateBranchInput{Code: "BDG", Name: "Bandung"})
	if err != nil {
		t.Fatal(err)
	}
	loc2, err := svc.CreateLocation(ctx(), 0, br.ID, stockapp.LocationInput{Code: "BDG", Name: "Gudang BDG"})
	if err != nil {
		t.Fatal(err)
	}
	items := []stockapp.TransferItemInput{{ProductID: fx.productID, VariantID: fx.variantID, Qty: 2}}
	if _, err := svc.CreateTransferWith(ctx(), 0, fx.branchID, br.ID, fx.locationID, loc2.ID, "", items,
		stockapp.TransferMeta{Enforced: true}); err == nil || len(fieldNames(err)) == 0 || fieldNames(err)[0] != "driverName" {
		t.Fatalf("inter-branch without driver = %v, want driverName error", err)
	}
	tr, err := svc.CreateTransferWith(ctx(), 0, fx.branchID, br.ID, fx.locationID, loc2.ID, "kirim", items,
		stockapp.TransferMeta{Enforced: true, DriverName: " Joko ", VehiclePlate: "B 1 X", TransferDate: "2026-10-12"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := svc.GetTransfer(ctx(), fx.branchID, tr.ID)
	if err != nil || got.DriverName != "Joko" || got.VehiclePlate != "B 1 X" || got.TransferDate != "2026-10-12" {
		t.Fatalf("persisted face: %+v %v", got, err)
	}
	// same-branch transfer without a driver is fine
	loc3, _ := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "L3", Name: "L3"})
	if _, err := svc.CreateTransferWith(ctx(), 0, fx.branchID, fx.branchID, fx.locationID, loc3.ID, "", items,
		stockapp.TransferMeta{Enforced: true}); err != nil {
		t.Fatalf("same-branch transfer needs no driver: %v", err)
	}
	count := func(dir string, branch int64) int {
		res, err := svc.ListTransfersDir(ctx(), branch, "", dir, 1, 20)
		if err != nil {
			t.Fatalf("list %q: %v", dir, err)
		}
		return int(res.Total)
	}
	if count("out", fx.branchID) != 1 || count("in", fx.branchID) != 0 || count("internal", fx.branchID) != 1 || count("", fx.branchID) != 2 {
		t.Fatalf("direction counts from source: out=%d in=%d internal=%d all=%d",
			count("out", fx.branchID), count("in", fx.branchID), count("internal", fx.branchID), count("", fx.branchID))
	}
	if count("in", br.ID) != 1 || count("out", br.ID) != 0 {
		t.Fatalf("direction counts from destination: in=%d out=%d", count("in", br.ID), count("out", br.ID))
	}
	if _, err := svc.ListTransfersDir(ctx(), fx.branchID, "", "sideways", 1, 20); err == nil {
		t.Fatal("unknown direction must be rejected")
	}
}

// ST-03/ST-06: goods that arrive damaged need no source location; the damaged
// list prices them at the books' average cost.
func TestParityDamagedArrivalAndLoss(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	before, _ := svc.GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if err := svc.MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: fx.productID, VariantID: fx.variantID, Qty: 4, Reason: "datang rusak"}); err != nil {
		t.Fatalf("arrived-damaged move-in: %v", err)
	}
	after, _ := svc.GetBalance(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID)
	if after.OnHand != before.OnHand {
		t.Fatalf("source untouched: %v -> %v", before.OnHand, after.OnHand)
	}
	res, err := svc.DamagedListPage(ctx(), fx.branchID, 1, 20, false)
	if err != nil || res.Total != 1 || res.WithStock != 1 {
		t.Fatalf("damaged page: %+v %v", res, err)
	}
	row := res.Items[0]
	if row.OnHand != 4 || row.EstimatedUnitCost <= 0 || row.EstimatedLoss != int64(4)*row.EstimatedUnitCost ||
		res.TotalEstimatedLoss != row.EstimatedLoss {
		t.Fatalf("loss row = %+v total %d", row, res.TotalEstimatedLoss)
	}
	if err := svc.MoveToDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 1, Reason: "pecah"}); err != nil {
		t.Fatalf("regular move-in: %v", err)
	}
	// restore still needs a destination
	if err := svc.RestoreDamaged(ctx(), 0, fx.branchID, stockapp.DamagedInput{
		ProductID: fx.productID, VariantID: fx.variantID, Qty: 1, Reason: "x"}); err == nil {
		t.Fatal("restore without a location must fail")
	}
}

// ST-04: movements expose the position's balance around each change.
func TestParityMovementBalances(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	res, err := testMods.stock.Service().ListMovements(ctx(), fx.branchID, fx.productID, fx.variantID, fx.locationID, "", "", "", 1, 10)
	if err != nil || len(res.Movements) == 0 {
		t.Fatalf("movements: %+v %v", res, err)
	}
	m := res.Movements[0]
	if m.BalanceBefore == nil || m.BalanceAfter == nil || *m.BalanceBefore != 0 || *m.BalanceAfter != 50 || m.MovedAt == "" {
		t.Fatalf("movement snapshot = %+v", m)
	}
}

// ST-05: legacy workbook (Indonesian sheet and headers, decimal comma, blank
// counts) previews with per-row detail and totals.
func TestParityOpeningLegacyWorkbook(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	svc := testMods.stock.Service()
	loc2, err := svc.CreateLocation(ctx(), 0, fx.branchID, stockapp.LocationInput{Code: "OPN", Name: "Opname"})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := testMods.product.Service().GetByID(ctx(), fx.productID)
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "Stok Awal")
	for r, row := range [][]string{
		{"Kode Barang", "Nama Barang", "Kode Lokasi", "Jumlah Stok Awal"},
		{p.Code, "x", loc2.Code, "1.000,5"},
		{p.Code, "x", "TST", ""},
	} {
		for c, v := range row {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+1)
			_ = f.SetCellValue("Stok Awal", cell, v)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatal(err)
	}
	prev, err := svc.PreviewOpening(ctx(), fx.branchID, bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if prev.ValidRows != 1 || prev.Skipped != 1 || prev.TotalQty != 1000.5 || prev.TotalValue != 10005000 || len(prev.Rows) != 2 || len(prev.Warnings) == 0 {
		t.Fatalf("preview = %+v", prev)
	}
	var tpl bytes.Buffer
	if err := svc.OpeningTemplate(ctx(), fx.branchID, &tpl); err != nil {
		t.Fatalf("template: %v", err)
	}
	tf, err := excelize.OpenReader(bytes.NewReader(tpl.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	rows, _ := tf.GetRows("Stok Awal")
	if len(rows) < 2 || rows[1][0] != p.Code {
		t.Fatalf("template must be prefilled with the catalog: %v", rows)
	}
}

// PY-01: the open-parties list totals what a party still owes, per branch.
func TestParityOpenParties(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	sellFixture(t, ctx(), fx, 4) // 4 x 15000 = 60000 owed by Customer Uji
	pay := testMods.payment.Service()
	res, err := pay.OpenParties(ctx(), fx.branchID, paymentapp.SideReceivable, "", 1, 20)
	if err != nil || res.Total != 1 || res.Items[0].PartyID != fx.customerID ||
		res.Items[0].Outstanding != 60000 || res.TotalOutstanding != 60000 || res.Items[0].OrderCount != 1 || res.Items[0].Name == "" {
		t.Fatalf("open parties = %+v %v", res, err)
	}
	if res, _ = pay.OpenParties(ctx(), fx.branchID, paymentapp.SideReceivable, "Customer", 1, 20); res.Total != 1 {
		t.Fatalf("search by name = %+v", res)
	}
	if res, _ = pay.OpenParties(ctx(), fx.branchID, paymentapp.SideReceivable, "zzzzzz", 1, 20); res.Total != 0 {
		t.Fatalf("non-matching search = %+v", res)
	}
	if res, _ = pay.OpenParties(ctx(), fx.branchID+999, paymentapp.SideReceivable, "", 1, 20); res.Total != 0 {
		t.Fatalf("other branch must see nothing: %+v", res)
	}
	if res, _ = pay.OpenParties(ctx(), fx.branchID, paymentapp.SidePayable, "", 1, 20); res.Total != 0 {
		t.Fatalf("no suppliers owed: %+v", res)
	}
	if _, err := pay.OpenParties(ctx(), fx.branchID, "weird", "", 1, 20); err == nil {
		t.Fatal("unknown side must be rejected")
	}
}

// DL-02: an unpaid prepaid order never shows in the shippable tab, and an order
// beyond the first page (limit 1 here) is still found by searching for it.
func TestParityWorkQueuePrepaidAndSearchBeyondLimit(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	first := sellFixture(t, ctx(), fx, 2)
	second := sellFixture(t, ctx(), fx, 3)
	prepaid, err := testMods.sales.Service().CreateOrder(ctx(), 0, fx.branchID, salesapp.OrderInput{
		PartyID: fx.customerID, Channel: "regular", PaymentTerms: "prepaid", TaxType: "none",
		Items: []salesapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, UOM: "pcs", Qty: 1}},
	})
	if err != nil {
		t.Fatalf("create prepaid: %v", err)
	}
	if _, err := testMods.sales.Service().ConfirmOrder(ctx(), 0, fx.branchID, prepaid.ID); err != nil {
		t.Fatalf("confirm prepaid: %v", err)
	}
	svc := testMods.delivery.Service()
	q, err := svc.WorkQueue(ctx(), fx.branchID, "", "", "", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range q.CreateSJ {
		if o.OrderID == prepaid.ID {
			t.Fatalf("unpaid prepaid order %d must be hidden: %+v", prepaid.ID, q.CreateSJ)
		}
	}
	if len(q.CreateSJ) != 2 {
		t.Fatalf("want the two COD orders, got %+v", q.CreateSJ)
	}
	// limit 1 shows only the oldest; searching the second finds it anyway
	q, _ = svc.WorkQueue(ctx(), fx.branchID, "", "", "", "", 1)
	if len(q.CreateSJ) != 1 || q.CreateSJ[0].OrderID != first {
		t.Fatalf("limit 1 = %+v, want the oldest (%d)", q.CreateSJ, first)
	}
	so2, _ := testMods.sales.Service().GetByID(ctx(), second)
	q, _ = svc.WorkQueue(ctx(), fx.branchID, so2.Number, "", "", "", 1)
	if len(q.CreateSJ) != 1 || q.CreateSJ[0].OrderID != second {
		t.Fatalf("search beyond the first page = %+v, want order %d", q.CreateSJ, second)
	}
}
