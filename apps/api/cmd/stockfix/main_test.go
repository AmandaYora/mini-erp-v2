package main

import (
	"os"
	"path/filepath"
	"testing"

	"mini-erp/cmd/stockfix/fix"
)

func TestReadLinesJSONAndCSV(t *testing.T) {
	dir := t.TempDir()
	want := fix.Line{ProductCode: "KBL-MK-ROL-001", VariantID: 6390, LocationID: 1, Direction: "out", Qty: 50,
		Reason: "netralkan koreksi bug tahanan POS", RefMovementID: 38533}
	files := map[string]string{
		"a.json": `[{"productCode":"KBL-MK-ROL-001","variantId":6390,"locationId":1,"direction":"out","qty":50,"reason":"netralkan koreksi bug tahanan POS","refMovementId":38533}]`,
		"b.json": `{"lines":[{"productCode":"KBL-MK-ROL-001","variantId":6390,"locationId":1,"direction":"out","qty":50,"reason":"netralkan koreksi bug tahanan POS","refMovementId":38533}]}`,
		"c.csv":  "refMovementId,productCode,variantId,locationId,direction,qty,reason\n38533,KBL-MK-ROL-001,6390,1,out,50,netralkan koreksi bug tahanan POS\n",
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		got, err := readLines(p)
		if err != nil || len(got) != 1 || got[0] != want {
			t.Errorf("%s: %+v err=%v", name, got, err)
		}
	}
	bad := filepath.Join(dir, "bad.csv")
	_ = os.WriteFile(bad, []byte("refMovementId,qty\nx,1\n"), 0o600)
	if _, err := readLines(bad); err == nil {
		t.Error("a CSV with a non-numeric id was accepted")
	}
}

func TestUsageWithoutRequiredFlags(t *testing.T) {
	if code := run([]string{"--branch-id", "1"}, nil, os.Stdout, os.Stderr); code != 2 {
		t.Fatalf("exit = %d, want 2", code)
	}
}
