package transform_test

import (
	"testing"

	"mini-erp/internal/etl/etltest"
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
