package application

import "testing"

func TestNormalizeSettings(t *testing.T) {
	out, err := normalizeSettings(map[string]string{
		"stock_adjust_always_approve":     " TRUE ",
		"delivery_require_evidence":       "0",
		"stock_adjust_approval_mode":      "Strict",
		"stock_adjust_approval_threshold": "250.50",
		"document_profile":                `{"title":"Toko"}`,
		"custom.flag":                     "x",
	})
	if err != nil {
		t.Fatalf("valid: %v", err)
	}
	want := map[string]string{
		"stock_adjust_always_approve":     "true",
		"delivery_require_evidence":       "false",
		"stock_adjust_approval_mode":      "strict",
		"stock_adjust_approval_threshold": "250.5",
		"document_profile":                `{"title":"Toko"}`,
		"custom.flag":                     "x",
	}
	for k, v := range want {
		if out[k] != v {
			t.Errorf("%s = %q, want %q", k, out[k], v)
		}
	}
}

func TestNormalizeSettingsRejects(t *testing.T) {
	for name, in := range map[string]map[string]string{
		"bool":      {"stock_adjust_always_approve": "maybe"},
		"enum":      {"stock_adjust_approval_mode": "paranoid"},
		"negative":  {"stock_adjust_approval_threshold": "-1"},
		"nan":       {"stock_adjust_approval_threshold": "abc"},
		"json":      {"document_profile": "bukan json"},
		"jsonarray": {"document_profile": "[1]"},
		"key":       {"Bad Key!": "x"},
	} {
		if _, err := normalizeSettings(in); err == nil {
			t.Errorf("%s: want validation error", name)
		}
	}
}

func TestWithDefaults(t *testing.T) {
	got := withDefaults(map[string]string{"stock_adjust_always_approve": "false"})
	if got["stock_adjust_always_approve"] != "false" {
		t.Error("stored value must win over default")
	}
	if got["stock_adjust_approval_mode"] != "simple" || got["stock_adjust_approval_threshold"] != "100" {
		t.Errorf("defaults missing: %v", got)
	}
	if _, ok := got["document_profile"]; ok {
		t.Error("document_profile has no default and must stay absent")
	}
}
