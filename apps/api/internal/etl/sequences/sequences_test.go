package sequences

import "testing"

// The legacy expense series (EXP/<year>/n, company-wide) continues in the
// revamp's EXP-<branch>/<year>/n counter instead of restarting at 00001.
func TestExpenseCounterContinuesLegacySeries(t *testing.T) {
	for _, c := range []struct {
		number, key string
		value       int64
		ok          bool
	}{
		{"EXP/2026/00255", "EXP:2026", 255, true},
		{"EXP-BLR/2026/00007", "EXP:2026", 7, true},
		{"JV-BLR/2026/00001", "", 0, false},
		{"EXP/26/00001", "", 0, false},
	} {
		key, value, ok := ExpenseCounter(c.number)
		if key != c.key || value != c.value || ok != c.ok {
			t.Errorf("ExpenseCounter(%q) = %q %d %v, want %q %d %v", c.number, key, value, ok, c.key, c.value, c.ok)
		}
	}
	key, value, _ := ExpenseCounter("EXP/2026/00255")
	if next, ok := FinanceNextNumber("BLR", key, value); !ok || next != "EXP-BLR/2026/00256" {
		t.Errorf("next expense number = %q %v, want EXP-BLR/2026/00256", next, ok)
	}
	if _, ok := FinanceNextNumber("BLR", "EXP", 1); ok {
		t.Error("a key without year must not render")
	}
}
