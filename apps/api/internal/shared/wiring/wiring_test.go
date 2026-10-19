package wiring

import (
	"strings"
	"testing"
)

func TestCheck(t *testing.T) {
	if err := Check("m", Dep{"a", true}, Dep{"b", true}); err != nil {
		t.Fatalf("all set must pass: %v", err)
	}
	if err := Check("m"); err != nil {
		t.Fatalf("no deps must pass: %v", err)
	}
	err := Check("delivery", Dep{"payments", false}, Dep{"settings", true}, Dep{"x", false})
	if err == nil {
		t.Fatal("missing deps must fail")
	}
	for _, want := range []string{"delivery", "payments", "x"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "settings") {
		t.Errorf("set dep reported as missing: %v", err)
	}
}
