package integration

import (
	"net/http"
	"strings"
	"testing"

	"mini-erp/internal/bootstrap"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	purchasereturnapp "mini-erp/internal/modules/purchasereturn/application"
	salesreturnapp "mini-erp/internal/modules/salesreturn/application"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	"mini-erp/internal/shared/apperror"
)

// Human-error corrections: every document cancel records WHY (reason), WHEN
// and BY WHOM on the document and in the audit row that co-commits with the
// status flip; a replay writes nothing twice.

func lastAuditNote(t *testing.T, action string) string {
	t.Helper()
	var note string
	if err := testMods.db.QueryRow(`SELECT COALESCE(note, '') FROM audit_logs WHERE action_key = ? ORDER BY id DESC LIMIT 1`, action).Scan(&note); err != nil {
		t.Fatalf("audit note %s: %v", action, err)
	}
	return note
}

func expectReasonAudited(t *testing.T, action, reason string) {
	t.Helper()
	if n := auditCount(t, action); n != 1 {
		t.Fatalf("%s audit rows = %d, want 1", action, n)
	}
	if note := lastAuditNote(t, action); !strings.Contains(note, reason) {
		t.Fatalf("%s audit note %q lacks the reason %q", action, note, reason)
	}
	if _, after, _ := lastDetail(t, action); !strings.Contains(after, reason) {
		t.Fatalf("%s audit after %s lacks the reason %q", action, after, reason)
	}
}

func isValidation(err error) bool {
	ae, ok := err.(*apperror.AppError)
	return ok && ae.Code == apperror.CodeValidation
}

func TestCancelReasonsStoredAndAudited(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods

	// Delivery note (draft). The audit row rides the CAS transaction: when
	// it cannot be written, the note stays a draft.
	soID := sellFixture(t, ctx(), fx, 10)
	note, err := draftNote(t, fx, soID, 4)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if _, err := m.delivery.Service().Cancel(ctx(), 0, fx.branchID, note.ID, strings.Repeat("x", 256)); !isValidation(err) {
		t.Fatalf("256-char reason = %v, want validation error", err)
	}
	release := blockAudit(t, "deliveries.cancel")
	if _, err := m.delivery.Service().Cancel(ctx(), 7, fx.branchID, note.ID, "salah jumlah"); err == nil {
		t.Fatal("cancel committed although its trail failed")
	}
	if got, _ := m.delivery.Service().GetByID(ctx(), note.ID); got.Status != "draft" || got.CancelReason != "" {
		t.Fatalf("after failed audit: %+v", got)
	}
	release()
	dn, err := m.delivery.Service().Cancel(ctx(), 7, fx.branchID, note.ID, "  salah jumlah  ")
	if err != nil {
		t.Fatalf("cancel note: %v", err)
	}
	if dn.Status != "cancelled" || dn.CancelReason != "salah jumlah" || dn.CancelledBy != 7 || dn.CancelledAt == "" {
		t.Fatalf("note cancel record: status=%s reason=%q by=%d at=%q", dn.Status, dn.CancelReason, dn.CancelledBy, dn.CancelledAt)
	}
	expectReasonAudited(t, "deliveries.cancel", "salah jumlah")
	if again, err := m.delivery.Service().Cancel(ctx(), 8, fx.branchID, note.ID, "lain"); err != nil || again.CancelReason != "salah jumlah" {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	if n := auditCount(t, "deliveries.cancel"); n != 1 {
		t.Fatalf("replay wrote a second audit row (%d)", n)
	}

	// Goods receipt (pending) discard.
	poID := confirmedPO(t, fx, 10)
	recID := insertPendingReceipt(t, fx, poID, 10, "GR-ALASAN-1")
	rec, err := m.goodsreceipt.Service().Discard(ctx(), 7, fx.branchID, recID, "supplier salah kirim")
	if err != nil {
		t.Fatalf("discard: %v", err)
	}
	if rec.Status != "void" || rec.CancelReason != "supplier salah kirim" || rec.CancelledBy != 7 || rec.CancelledAt == "" {
		t.Fatalf("receipt discard record: %+v", rec)
	}
	expectReasonAudited(t, "goods_receipts.discard", "supplier salah kirim")
	if _, err := m.goodsreceipt.Service().Discard(ctx(), 7, fx.branchID, recID, "lagi"); err != nil {
		t.Fatalf("discard replay: %v", err)
	}
	if n := auditCount(t, "goods_receipts.discard"); n != 1 {
		t.Fatalf("discard replay wrote a second audit row (%d)", n)
	}

	// Sales return (draft).
	soID2 := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID2, 2)
	sret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID2, "2026-09-10", "", "",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}}, nil)
	if err != nil {
		t.Fatalf("create sales return: %v", err)
	}
	release = blockAudit(t, "sales_returns.cancel")
	if _, err := m.salesreturn.Service().Cancel(ctx(), 7, fx.branchID, sret.ID, "pelanggan batal"); err == nil {
		t.Fatal("return cancel committed although its trail failed")
	}
	release()
	sr, err := m.salesreturn.Service().Cancel(ctx(), 7, fx.branchID, sret.ID, "pelanggan batal")
	if err != nil {
		t.Fatalf("cancel sales return: %v", err)
	}
	if sr.Status != "cancelled" || sr.CancelReason != "pelanggan batal" || sr.CancelledBy != 7 || sr.CancelledAt == "" {
		t.Fatalf("sales return cancel record: %+v", sr)
	}
	expectReasonAudited(t, "sales_returns.cancel", "pelanggan batal")

	// Purchase return (draft).
	poID2 := purchaseFixture(t, ctx(), fx, 10000, 2)
	pret, err := m.purchasereturn.Service().Create(ctx(), 0, fx.branchID, poID2, "2026-09-10", "",
		[]purchasereturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("create purchase return: %v", err)
	}
	pr, err := m.purchasereturn.Service().Cancel(ctx(), 7, fx.branchID, pret.ID, "salah pilih PO")
	if err != nil {
		t.Fatalf("cancel purchase return: %v", err)
	}
	if pr.Status != "cancelled" || pr.CancelReason != "salah pilih PO" || pr.CancelledBy != 7 || pr.CancelledAt == "" {
		t.Fatalf("purchase return cancel record: %+v", pr)
	}
	expectReasonAudited(t, "purchase_returns.cancel", "salah pilih PO")

	// Stock transfer (dispatched: stock comes back to the source).
	_, _, trID := transferTo(t, fx, 5)
	before := onHand(t, fx, fx.productID, fx.variantID, fx.locationID)
	if _, err := m.stock.Service().DispatchTransfer(ctx(), 0, fx.branchID, trID); err != nil {
		t.Fatalf("dispatch transfer: %v", err)
	}
	release = blockAudit(t, "stock_transfers.cancel")
	if _, err := m.stock.Service().CancelTransfer(ctx(), 7, fx.branchID, trID, "gudang tujuan salah"); err == nil {
		t.Fatal("transfer cancel committed although its trail failed")
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != before-5 {
		t.Fatalf("failed cancel moved stock: onhand %v, want %v", got, before-5)
	}
	release()
	tr, err := m.stock.Service().CancelTransfer(ctx(), 7, fx.branchID, trID, "gudang tujuan salah")
	if err != nil {
		t.Fatalf("cancel transfer: %v", err)
	}
	if tr.Status != "cancelled" || tr.CancelReason != "gudang tujuan salah" || tr.CancelledBy != 7 || tr.CancelledAt == "" {
		t.Fatalf("transfer cancel record: %+v", tr)
	}
	if got := onHand(t, fx, fx.productID, fx.variantID, fx.locationID); got != before {
		t.Fatalf("transfer cancel: onhand %v, want %v", got, before)
	}
	expectReasonAudited(t, "stock_transfers.cancel", "gudang tujuan salah")
	if _, err := m.stock.Service().CancelTransfer(ctx(), 7, fx.branchID, trID, "lagi"); err != nil {
		t.Fatalf("transfer cancel replay: %v", err)
	}
	if n := auditCount(t, "stock_transfers.cancel"); n != 1 {
		t.Fatalf("transfer replay wrote a second audit row (%d)", n)
	}
}

// The web endpoints make the reason mandatory; the services keep accepting ""
// for programmatic callers.
func TestCancelReasonRequiredOverHTTP(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	h := routeServer(t).Handler()
	tok := guardUser(t, fx.branchID, "koreksi", catalogCodes())

	paths := []string{
		"/api/v1/deliveries/999999/cancel",
		"/api/v1/goods-receipts/999999/discard",
		"/api/v1/sales-returns/999999/cancel",
		"/api/v1/sales-returns/999999/replacement-delivery/cancel",
		"/api/v1/purchase-returns/999999/cancel",
		"/api/v1/stock/transfers/999999/cancel",
	}
	for _, p := range paths {
		for _, body := range []string{"", `{}`, `{"reason":"   "}`} {
			rec := doReq(h, http.MethodPost, p, tok, body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("POST %s body %q = %d, want 400 (%s)", p, body, rec.Code, rec.Body.String())
			}
		}
	}

	soID := sellFixture(t, ctx(), fx, 4)
	note, err := draftNote(t, fx, soID, 4)
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	rec := doReq(h, http.MethodPost, "/api/v1/deliveries/"+itoa(note.ID)+"/cancel", tok, `{"reason":"sopir salah"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel with reason = %d (%s)", rec.Code, rec.Body.String())
	}
	var view struct {
		Status       string `json:"status"`
		CancelReason string `json:"cancelReason"`
		CancelledAt  string `json:"cancelledAt"`
	}
	decodeData(t, rec, &view)
	if view.Status != "cancelled" || view.CancelReason != "sopir salah" || view.CancelledAt == "" {
		t.Fatalf("cancel view = %+v", view)
	}
}

// exchangeDispatched builds a confirmed exchange (2 returned, 1 replacement)
// whose replacement shipment has been dispatched (a DRAFT SJ).
func exchangeDispatched(t *testing.T, fx shopFixture) (retID int64, dn *deliverycontracts.DeliveryNote) {
	t.Helper()
	m := testMods
	soID := sellFixture(t, ctx(), fx, 2)
	deliverAll(t, ctx(), fx, soID, 2)
	ret, err := m.salesreturn.Service().Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "exchange",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 2}},
		[]salesreturnapp.ReplacementInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}})
	if err != nil {
		t.Fatalf("create exchange: %v", err)
	}
	if _, err := m.salesreturn.Service().Confirm(ctx(), 0, fx.branchID, ret.ID); err != nil {
		t.Fatalf("confirm return: %v", err)
	}
	dn, err = m.salesreturn.Service().CreateReplacementDelivery(ctx(), 0, fx.branchID, ret.ID, salesreturnapp.ReplacementDispatchInput{})
	if err != nil {
		t.Fatalf("dispatch replacement: %v", err)
	}
	return ret.ID, dn
}

// A wrong replacement shipment that is still a draft is taken back through
// the return: the draft SJ is voided (stray stock reversed), the exchange goes
// back to "pending", a corrected replacement can be dispatched and confirmed.
func TestExchangeReplacementCancelFlow(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	svc := m.salesreturn.Service()

	retID, dn1 := exchangeDispatched(t, fx)
	if avail := available(t, fx); avail != 50 {
		t.Fatalf("setup: available=%v, want 50", avail)
	}
	// A confirm that died after its stock posting left stock out on the draft.
	if _, err := m.stock.Service().Post(ctx(), stockcontracts.PostRequest{
		BranchID: fx.branchID, Direction: "out", RefType: stockcontracts.RefDelivery, RefID: dn1.ID,
		Lines: []stockcontracts.PostLine{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, Qty: 1}},
	}); err != nil {
		t.Fatalf("stray posting: %v", err)
	}
	if avail := available(t, fx); avail != 49 {
		t.Fatalf("stray posting: available=%v, want 49", avail)
	}

	// The delivery contract refuses order notes; the return refuses a non-exchange.
	soID := sellFixture(t, ctx(), fx, 1)
	orderNote, err := draftNote(t, fx, soID, 1)
	if err != nil {
		t.Fatalf("draft order note: %v", err)
	}
	if _, err := m.delivery.Service().CancelReplacement(ctx(), 0, fx.branchID, orderNote.ID, "x"); !isConflict(err) {
		t.Fatalf("CancelReplacement on an order note = %v, want conflict", err)
	}

	ret, err := svc.CancelReplacementDelivery(ctx(), 7, fx.branchID, retID, "barang pengganti salah")
	if err != nil {
		t.Fatalf("cancel replacement: %v", err)
	}
	if ret.ReplacementDeliveryStatus != "pending" || ret.Status != "confirmed" {
		t.Fatalf("after cancel: status=%s repl=%s, want confirmed/pending", ret.Status, ret.ReplacementDeliveryStatus)
	}
	gone, err := m.delivery.Service().GetByID(ctx(), dn1.ID)
	if err != nil || gone.Status != "cancelled" || gone.CancelReason != "barang pengganti salah" || gone.CancelledBy != 7 {
		t.Fatalf("replacement SJ after cancel = %+v, %v", gone, err)
	}
	if avail := available(t, fx); avail != 50 {
		t.Fatalf("stray stock not reversed: available=%v, want 50", avail)
	}
	expectReasonAudited(t, "sales_returns.cancel_replacement", "barang pengganti salah")
	if note := lastAuditNote(t, "sales_returns.cancel_replacement"); !strings.Contains(note, dn1.Number) {
		t.Fatalf("audit note %q lacks the SJ number %s", note, dn1.Number)
	}
	if n := auditCount(t, "deliveries.cancel"); n != 1 {
		t.Fatalf("delivery cancel audit rows = %d, want 1", n)
	}

	// Replay: success, nothing written twice.
	if again, err := svc.CancelReplacementDelivery(ctx(), 7, fx.branchID, retID, "lagi"); err != nil || again.ReplacementDeliveryStatus != "pending" {
		t.Fatalf("replay = %+v, %v", again, err)
	}
	if n := auditCount(t, "sales_returns.cancel_replacement"); n != 1 {
		t.Fatalf("replay wrote a second audit row (%d)", n)
	}

	// Dispatch a corrected replacement and confirm it.
	dn2, err := svc.CreateReplacementDelivery(ctx(), 0, fx.branchID, retID, salesreturnapp.ReplacementDispatchInput{Notes: "pengganti benar"})
	if err != nil {
		t.Fatalf("re-dispatch: %v", err)
	}
	if dn2.ID == dn1.ID || dn2.Status != "draft" {
		t.Fatalf("re-dispatch note = %+v", dn2)
	}
	if _, err := svc.ConfirmReplacementDelivery(ctx(), 0, fx.branchID, retID, dn2.ID, deliverycontracts.ReplacementConfirm{}); err != nil {
		t.Fatalf("confirm corrected replacement: %v", err)
	}
	if avail := available(t, fx); avail != 49 {
		t.Fatalf("corrected replacement stock out: available=%v, want 49", avail)
	}
	done, _ := svc.GetByID(ctx(), retID)
	if done.ReplacementDeliveryStatus != "confirmed" {
		t.Fatalf("repl status = %s, want confirmed", done.ReplacementDeliveryStatus)
	}

	// A confirmed replacement cannot be taken back.
	if _, err := svc.CancelReplacementDelivery(ctx(), 7, fx.branchID, retID, "terlambat"); !isConflict(err) {
		t.Fatalf("cancel after confirm = %v, want conflict", err)
	}
	if n := auditCount(t, "sales_returns.cancel_replacement"); n != 1 {
		t.Fatalf("refusal wrote an audit row (%d)", n)
	}
}

// A replacement SJ confirmed while the return still says "dispatched" (a
// half-finished confirm) is refused too, and a return-only return has no
// replacement to cancel.
func TestExchangeReplacementCancelRefusals(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	m := testMods
	svc := m.salesreturn.Service()

	retID, dn := exchangeDispatched(t, fx)
	if _, err := m.delivery.Service().ConfirmReplacement(ctx(), 0, fx.branchID, dn.ID, deliverycontracts.ReplacementConfirm{}); err != nil {
		t.Fatalf("confirm SJ directly: %v", err)
	}
	if _, err := svc.CancelReplacementDelivery(ctx(), 0, fx.branchID, retID, "salah"); !isConflict(err) {
		t.Fatalf("cancel with a confirmed SJ = %v, want conflict", err)
	}
	if got, _ := svc.GetByID(ctx(), retID); got.ReplacementDeliveryStatus != "dispatched" {
		t.Fatalf("refusal moved the return: %s", got.ReplacementDeliveryStatus)
	}
	if got, _ := m.delivery.Service().GetByID(ctx(), dn.ID); got.Status != "confirmed" {
		t.Fatalf("refusal touched the SJ: %s", got.Status)
	}

	soID := sellFixture(t, ctx(), fx, 1)
	deliverAll(t, ctx(), fx, soID, 1)
	plain, err := svc.Create(ctx(), 0, fx.branchID, soID, "2026-09-10", "", "",
		[]salesreturnapp.LineInput{{ProductID: fx.productID, VariantID: fx.variantID, LocationID: fx.locationID, UOM: "pcs", Qty: 1}}, nil)
	if err != nil {
		t.Fatalf("create return: %v", err)
	}
	if _, err := svc.CancelReplacementDelivery(ctx(), 0, fx.branchID, plain.ID, "x"); !isConflict(err) {
		t.Fatalf("cancel replacement of a return-only return = %v, want conflict", err)
	}
}
