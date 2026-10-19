package application

import "testing"

// F2b-8: every mapping key has a default account, every default account that
// a key points at exists, and the codes follow the legacy chart.
func TestDefaultChartCoversAllMappingKeys(t *testing.T) {
	accounts, mappings := DefaultChart()
	codes := map[string]bool{}
	for _, a := range accounts {
		if codes[a.code] {
			t.Fatalf("duplicate account code %s", a.code)
		}
		codes[a.code] = true
	}
	if len(mappingKeys) != 18 {
		t.Fatalf("mapping keys = %d, want 18", len(mappingKeys))
	}
	for _, k := range mappingKeys {
		code, ok := mappings[k]
		if !ok {
			t.Fatalf("no default mapping for %q", k)
		}
		if !codes[code] {
			t.Fatalf("mapping %q -> %s points at a missing account", k, code)
		}
		if len(mappingTypes[k]) == 0 {
			t.Fatalf("no type rule for %q", k)
		}
	}
	// Legacy codes the accountants know.
	want := map[string]string{"cash": "1000", "bank": "1010", "receivable": "1100", "inventory": "1200",
		"ppn_in": "1300", "payable": "2000", "ppn_out": "2100", "revenue": "4000", "cogs": "5000"}
	for k, code := range want {
		if mappings[k] != code {
			t.Fatalf("%s -> %s, want legacy code %s", k, mappings[k], code)
		}
	}
}

// Every legacy system_key resolves to a known mapping key.
func TestLegacySystemKeysMapToKnownKeys(t *testing.T) {
	known := map[string]bool{}
	for _, k := range mappingKeys {
		known[k] = true
	}
	for legacy, key := range LegacySystemKeyToMappingKey {
		if !known[key] {
			t.Fatalf("legacy %q maps to unknown key %q", legacy, key)
		}
	}
}
