package transform_test

import (
	"testing"

	"mini-erp/internal/etl/etltest"
	"mini-erp/internal/etl/extract"
	"mini-erp/internal/etl/report"
	"mini-erp/internal/etl/transform"
)

// DL-01: the legacy media purpose picks the revamp owner type of a delivery proof.
func TestTransformDeliveryProofPurposeOwnerType(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	got := map[int64]string{}
	for _, r := range out.Tables["media_candidates"] {
		got[r.Int("legacy_id")] = r.Str("owner_type")
	}
	if got[3] != "delivery_document" {
		t.Errorf("signed document (legacy 3) owner_type = %q, want delivery_document", got[3])
	}
	if got[4] != "delivery_proof" {
		t.Errorf("location photo (legacy 4) owner_type = %q, want delivery_proof", got[4])
	}
	if got[1] != "product" {
		t.Errorf("product image owner_type = %q, want product", got[1])
	}
}

// ST-01/ST-02: location attributes and the transfer's driver/vehicle/date travel.
func TestTransformLocationAttributesAndTransferFace(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)
	byCode := map[string]int64{}
	var rakSort, gdgPrimary, damagedFlags int64
	for _, r := range out.Tables["stock_locations"] {
		byCode[r.Str("code")] = r.Int("id")
		switch r.Str("code") {
		case "RAK-A":
			rakSort = r.Int("sort_order")
		case "GDG":
			gdgPrimary = r.Int("is_primary")
		case "RUSAK":
			damagedFlags = r.Int("is_primary") + r.Int("is_picking_area") + r.Int("sort_order")
		}
	}
	if len(byCode) == 0 || rakSort != 2 || gdgPrimary != 1 || damagedFlags != 0 {
		t.Errorf("location attrs: rak sort=%d gdg primary=%d damaged flags=%d (codes %v)", rakSort, gdgPrimary, damagedFlags, byCode)
	}
	rows := out.Tables["stock_transfers"]
	if len(rows) == 0 {
		t.Fatal("no transfers")
	}
	r := rows[0]
	if r.Str("driver_name") != "Pak Sopir" || r.Str("vehicle_plate") != "B 1" || len(r.Str("transfer_date")) < 10 {
		t.Errorf("transfer face = driver %q plate %q date %q", r.Str("driver_name"), r.Str("vehicle_plate"), r.Str("transfer_date"))
	}
}

// Q-HPP-2: legacy books the finance opening as cost rows WITHOUT a movement,
// one per (branch, product), while the opening quantity already sits in the
// opening_stock movements. The value must land on those movements (pro rata
// qty, real cost) and no synthetic stock movement may appear.
func TestTransformOpeningCostSpread(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	out := runTransform(t, work)

	newMov := map[int64]int64{}
	for _, e := range out.IDMap {
		if e.Entity == "stock_movements" && e.Sub == 0 {
			newMov[e.LegacyID] = e.NewID
		}
	}
	cost := map[int64]struct{ total, est string }{}
	for _, r := range out.Tables["finance_inventory_cost_movements"] {
		cost[r.Int("stock_movement_id")] = struct{ total, est string }{r.Str("total_cost"), r.Str("is_estimated")}
	}
	// Legacy opening movements 1 (20) and 2 (5) share product 2's pooled
	// Rp300.000,50 -> Rp300.001; the odd rupiah goes to the larger remainder.
	// Movement 3 takes product 3's row whole; movement 4 keeps its own linked row.
	want := map[int64]string{1: "240001.00", 2: "60000.00", 3: "80000.00", 4: "100000.00"}
	for lid, total := range want {
		got := cost[newMov[lid]]
		if got.total != total || got.est != "0" {
			t.Errorf("opening movement %d: cost %q estimated %q, want %s real", lid, got.total, got.est, total)
		}
	}
	opening := 0
	for _, r := range out.Tables["stock_movements"] {
		if r.Str("ref_type") == "opening" {
			opening++
		}
	}
	if opening != 4 {
		t.Errorf("opening stock movements = %d, want the 4 legacy ones (no synthetic movement)", opening)
	}
	codes := map[string]int{}
	for _, f := range out.Log.Findings {
		codes[f.Code]++
	}
	if codes["COST_NO_MOVEMENT"] != 0 || codes["COST_OPENING_UNMATCHED"] != 0 || codes["COST_NOOP"] != 1 || codes["COST_OPENING_SPLIT"] != 2 {
		t.Errorf("findings = %v, want COST_NOOP 1, COST_OPENING_SPLIT 2, no COST_NO_MOVEMENT/COST_OPENING_UNMATCHED", codes)
	}
	if out.Explain["opening_cost_split_extra"] != 1 || out.Explain["cost_rows_unmigrated"] != 1 {
		t.Errorf("explain split_extra=%d unmigrated=%d, want 1 and 1", out.Explain["opening_cost_split_extra"], out.Explain["cost_rows_unmigrated"])
	}
}

// Q-HPP-2: when the opening quantity in the cost ledger disagrees with the
// opening_stock movements, the value cannot be placed without changing stock;
// the transform refuses (blocker) instead of guessing.
func TestTransformOpeningCostUnmatchedBlocks(t *testing.T) {
	env := etltest.Setup(t)
	work := t.TempDir()
	extractFixture(t, env, work)
	raw, err := extract.Load(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range raw["finance_inventory_cost_movements"] {
		if r.Int("id_finance_inventory_cost_movement") == 3 {
			r["quantity_in_base_uom"] = "12.000000000000"
		}
	}
	out, err := transform.Run(transform.Input{Cfg: testCfg(work), Raw: raw})
	if err != nil {
		t.Fatal(err)
	}
	var blocked bool
	for _, f := range out.Log.Findings {
		if f.Code == "COST_OPENING_UNMATCHED" && f.LegacyID == 3 && f.Severity == report.Blocker {
			blocked = true
		}
	}
	if !blocked {
		t.Error("qty mismatch must raise blocker COST_OPENING_UNMATCHED for legacy cost row 3")
	}
}
