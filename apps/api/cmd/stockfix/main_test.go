package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

// Not a terminal (pipe/file): the first line is the password; only the prompt
// is written, never the password itself.
func TestReadPasswordFromPipe(t *testing.T) {
	for _, in := range []string{"rahasia-kuat-1\nbaris-berikut\n", "rahasia-kuat-1\r\n", "rahasia-kuat-1"} {
		var prompt bytes.Buffer
		got, err := readPassword(strings.NewReader(in), &prompt)
		if err != nil || got != "rahasia-kuat-1" {
			t.Errorf("%q: %q err=%v", in, got, err)
		}
		if strings.Contains(prompt.String(), "rahasia") || prompt.String() != "Kata sandi penyetuju: " {
			t.Errorf("prompt = %q", prompt.String())
		}
	}
	if _, err := readPassword(strings.NewReader("\n"), &bytes.Buffer{}); err == nil {
		t.Error("an empty password was accepted")
	}
	// An *os.File that is not a terminal (here a pipe) also takes the line path.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.WriteString("dari-pipa\n")
	_ = w.Close()
	if got, err := readPassword(r, &bytes.Buffer{}); err != nil || got != "dari-pipa" {
		t.Errorf("pipe: %q err=%v", got, err)
	}
}

// The report carries the approver's name only: no field can hold the password.
func TestReportHasNoPassword(t *testing.T) {
	b, _ := json.Marshal(fix.Report{Approver: "bosstok", Lines: []fix.Result{{Reason: "x"}}})
	if strings.Contains(strings.ToLower(string(b)), "password") || strings.Contains(strings.ToLower(string(b)), "sandi") {
		t.Fatalf("report JSON = %s", b)
	}
}
