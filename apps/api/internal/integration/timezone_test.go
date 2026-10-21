package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"mini-erp/internal/bootstrap"
	stockapp "mini-erp/internal/modules/stock/application"
)

// Datetime convention (knowledge/DATABASE_GUIDE.md): system stamps are UTC
// instants, document dates are WIB. Everything derived from a stamp — a
// journal day, a day filter, a day bucket, the wire value — must use the WIB
// calendar. 2026-09-30 18:00 UTC is 2026-10-01 01:00 WIB, the hour where a UTC
// reading lands on the wrong day.
const (
	stampUTC = "2026-09-30 18:00:00"
	stampWIB = "2026-10-01 01:00:00"
	dayWIB   = "2026-10-01"
)

// A stock adjustment made at 01:00 WIB is journaled on that WIB day.
func TestStockMovementJournalUsesWIBDay(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	p := productWith(t, "TZADJ", 0, 1)
	vid := p.Variants[0].ID
	seedInCost(t, fx, p.ID, vid, fx.locationID, 10, 1000)
	if err := testMods.stock.Service().Adjust(ctx(), 0, fx.branchID, stockapp.AdjustInput{
		ProductID: p.ID, VariantID: vid, Location: fx.locationID, Mode: "in", QtyDelta: 1, Reason: "opname lebih",
	}); err != nil {
		t.Fatalf("adjust: %v", err)
	}
	ids := queuedStockMovements(t, fx.branchID)
	if len(ids) != 1 {
		t.Fatalf("queued = %v", ids)
	}
	if _, err := testMods.db.Exec(`UPDATE finance_inventory_cost_movements SET moved_at = ? WHERE stock_movement_id = ?`, stampUTC, ids[0]); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	q, err := testMods.finance.Service().UnpostedSources(ctx(), fx.branchID)
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	for _, it := range q {
		if it.DocType == "stock_movement" && it.Date != dayWIB {
			t.Fatalf("queue date = %q, want the WIB day %s", it.Date, dayWIB)
		}
	}
	e, err := testMods.finance.Service().Post(ctx(), 0, fx.branchID, "stock_movement", ids[0])
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if e.Date[:10] != dayWIB {
		t.Fatalf("journal date = %q, want %s", e.Date, dayWIB)
	}
}

// The movement list day filter and the top-products day bucket read UTC
// stamps on WIB days.
func TestStampDayFiltersUseWIB(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	p := productWith(t, "TZDAY", 0, 1)
	vid := p.Variants[0].ID
	seedInCost(t, fx, p.ID, vid, fx.locationID, 5, 1000)
	if _, err := testMods.db.Exec(`UPDATE stock_movements SET moved_at = ? WHERE product_id = ?`, stampUTC, p.ID); err != nil {
		t.Fatalf("backdate movement: %v", err)
	}
	svc := testMods.stock.Service()
	count := func(from, to string) int64 {
		t.Helper()
		res, err := svc.ListMovements(ctx(), fx.branchID, p.ID, vid, 0, "", from, to, 1, 10)
		if err != nil {
			t.Fatalf("list %s..%s: %v", from, to, err)
		}
		return res.Total
	}
	if n := count(dayWIB, dayWIB); n != 1 {
		t.Fatalf("movements on %s = %d, want 1", dayWIB, n)
	}
	if n := count("2026-09-30", "2026-09-30"); n != 0 {
		t.Fatalf("movements on the UTC day = %d, want 0", n)
	}
	if _, err := svc.ListMovements(ctx(), fx.branchID, 0, 0, 0, "", "30-09-2026", "", 1, 10); mustAppErr(t, err).Fields[0].Field != "dateFrom" {
		t.Fatalf("malformed day must be a validation error, got %v", err)
	}

	soID := sellAndDeliver(t, fx, p.ID, vid, 2)
	var orderID int64
	if err := testMods.db.QueryRow(`SELECT sales_order_id FROM delivery_notes WHERE id = ?`, soID).Scan(&orderID); err != nil {
		t.Fatalf("order of delivery: %v", err)
	}
	if _, err := testMods.db.Exec(`UPDATE sales_orders SET status = 'completed', completed_at = ? WHERE id = ?`, stampUTC, orderID); err != nil {
		t.Fatalf("complete: %v", err)
	}
	top, err := testMods.sales.Service().TopProducts(ctx(), fx.branchID, dayWIB, dayWIB, 10)
	if err != nil {
		t.Fatalf("top products: %v", err)
	}
	if len(top) != 1 || top[0].ProductID != p.ID {
		t.Fatalf("top products on %s = %+v, want the product sold at 01:00 WIB", dayWIB, top)
	}
}

// Every datetime on the wire is a WIB wall clock: a stamp stored at 18:00 UTC
// reads 01:00 the next day, whatever the browser's own zone is.
func TestWireStampsAreWIB(t *testing.T) {
	freshDB(t)
	fx := newShop(t, ctx())
	clearFixtureStock(t)
	if err := bootstrap.EnsureAuthz(ctx(), testMods.user.Service()); err != nil {
		t.Fatalf("seed authz: %v", err)
	}
	p := productWith(t, "TZWIRE", 0, 1)
	seedInCost(t, fx, p.ID, p.Variants[0].ID, fx.locationID, 1, 1000)
	if _, err := testMods.db.Exec(`UPDATE stock_movements SET moved_at = ?, created_at = ? WHERE product_id = ?`, stampUTC, stampUTC, p.ID); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	h := routeServer(t).Handler()
	tok := twoBranchUser(t, "jam", []string{"stock.view"}, fx.branchID)
	rec := doReq(h, http.MethodGet, "/api/v1/stock/movements?productId="+itoa(p.ID), tok, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("movements = %d: %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || len(env.Data) != 1 {
		t.Fatalf("decode: %v %s", err, rec.Body.String())
	}
	if env.Data[0]["movedAt"] != stampWIB || env.Data[0]["createdAt"] != stampWIB {
		t.Fatalf("movedAt/createdAt = %v / %v, want %s", env.Data[0]["movedAt"], env.Data[0]["createdAt"], stampWIB)
	}
}
