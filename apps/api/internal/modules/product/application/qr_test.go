package application

import (
	"testing"

	"mini-erp/internal/modules/product/contracts"
	"mini-erp/internal/modules/product/infrastructure"
)

// TestSelectQRPayload (§7 Tahap J2): explicit variants resolve barcode
// first, code second; the product default prefers the default barcode.
func TestSelectQRPayload(t *testing.T) {
	p := &contracts.Product{Code: "PRD", Variants: []*contracts.Variant{
		{ID: 1, Code: "V-1", Barcode: "B-1"},
		{ID: 2, Code: "V-2", IsDefault: true, Barcode: "B-2"},
		{ID: 3, Code: "V-3"},
	}}
	cases := []struct {
		name      string
		variantID int64
		want      string
		wantErr   bool
	}{
		{"explicit barcode", 1, "B-1", false},
		{"explicit code fallback", 3, "V-3", false},
		{"unknown variant", 99, "", true},
		{"product default", 0, "B-2", false},
	}
	for _, c := range cases {
		got, err := selectQRPayload(p, c.variantID)
		if c.wantErr != (err != nil) {
			t.Errorf("%s: err = %v, wantErr %v", c.name, err, c.wantErr)
			continue
		}
		if !c.wantErr && got != c.want {
			t.Errorf("%s: payload = %q, want %q", c.name, got, c.want)
		}
	}
	if got, _ := selectQRPayload(&contracts.Product{Code: "PRD"}, 0); got != "PRD" {
		t.Errorf("no variants: payload = %q, want PRD", got)
	}
	if _, err := selectQRPayload(&contracts.Product{}, 0); err == nil {
		t.Error("empty code: expected conflict, got nil")
	}
}

// TestAssembleQRTargets: variant products fan out per variant (no parent
// target), plain products collapse to one, jasa never appears (filtered in
// SQL — absent rows simply assemble to nothing).
func TestAssembleQRTargets(t *testing.T) {
	rows := []infrastructure.QRSource{
		{ProductID: 1, ProductCode: "P1", ProductName: "Induk", Category: "Kat",
			VariantID: 11, VariantCode: "V1", VariantName: "Merah", Barcode: "B11"},
		{ProductID: 1, ProductCode: "P1", ProductName: "Induk", Category: "Kat",
			VariantID: 12, VariantCode: "V2", VariantName: "Biru"},
		{ProductID: 2, ProductCode: "P2", ProductName: "Satuan", Category: ""},
	}
	got := assembleQRTargets(rows)
	if len(got) != 3 {
		t.Fatalf("targets = %d, want 3", len(got))
	}
	if got[0].Payload != "B11" || got[0].Name != "Induk - Merah" || got[0].Code != "V1" {
		t.Errorf("variant barcode target = %+v", got[0])
	}
	if got[1].Payload != "V2" || got[1].Name != "Induk - Biru" {
		t.Errorf("variant code fallback target = %+v", got[1])
	}
	if got[2].Payload != "P2" || got[2].Category != "" {
		t.Errorf("product target = %+v", got[2])
	}
	if len(assembleQRTargets(nil)) != 0 {
		t.Error("nil rows must assemble to empty, not nil-panic")
	}
}

// TestQRFileName: illegal characters become dashes, duplicates suffix.
func TestQRFileName(t *testing.T) {
	seen := map[string]int{}
	a := qrFileName("BRG/01", "Kopi: Kapal", seen)
	if a != "BRG-01_Kopi- Kapal.png" {
		t.Errorf("sanitized = %q", a)
	}
	b := qrFileName("BRG/01", "Kopi: Kapal", seen)
	if b != "BRG-01_Kopi- Kapal_2.png" {
		t.Errorf("dedup = %q", b)
	}
	if got := sanitizeQRName("   ", 50); got != "tanpa-nama" {
		t.Errorf("blank = %q", got)
	}
	long := ""
	for i := 0; i < 200; i++ {
		long += "x"
	}
	if got := sanitizeQRName(long, 50); len([]rune(got)) != 50 {
		t.Errorf("cap runes = %d, want 50", len([]rune(got)))
	}
}

// TestClampQRSize: garbage sizes fall back, extremes clamp.
func TestClampQRSize(t *testing.T) {
	if clampQRSize(0) != QRDefaultSize || clampQRSize(-5) != QRDefaultSize {
		t.Error("non-positive must fall back to default")
	}
	if clampQRSize(5000) != QRMaxSize {
		t.Error("oversize must clamp to max")
	}
	if clampQRSize(480) != 480 {
		t.Error("in-range size must pass through")
	}
}
