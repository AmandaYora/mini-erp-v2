package bootstrap

import (
	"sort"
	"testing"
)

func knownCodes() map[string]bool {
	known := map[string]bool{}
	for _, s := range AllPermissionSeeds() {
		known[s.Code] = true
	}
	return known
}

// TestLegacyMap (F1-6): every legacy default code maps to an existing revamp
// code or is an explicit by-design drop. A revamp code removed or renamed
// without updating the map fails here instead of silently shrinking grants.
func TestLegacyMap(t *testing.T) {
	known := knownCodes()
	for role, legacy := range map[string][]string{
		"admin": legacyAdmin(), "staff": legacyStaff(), "kasir": legacyKasir(),
	} {
		if unmapped := UnmappedLegacyCodes(legacy); len(unmapped) > 0 {
			t.Fatalf("role %s: kode legacy tanpa peta: %v", role, unmapped)
		}
		for _, code := range MapLegacyCodes(legacy) {
			if !known[code] {
				t.Fatalf("role %s: peta menghasilkan kode tak dikenal %q", role, code)
			}
		}
	}
}

// TestLegacyDropsAreByDesign pins the drop list to knowledge/RAG only.
func TestLegacyDropsAreByDesign(t *testing.T) {
	want := []string{"knowledge.view", "knowledge.create", "knowledge.update", "knowledge.archive"}
	got := append([]string{}, DroppedLegacyCodes()...)
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("drops = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("drops = %v, want %v", got, want)
		}
	}
}

// TestMapLegacyExamples pins representative translations (paritas K-4).
func TestMapLegacyExamples(t *testing.T) {
	has := func(list []string, code string) bool {
		for _, c := range list {
			if c == code {
				return true
			}
		}
		return false
	}
	admin := MapLegacyCodes(legacyAdmin())
	for _, want := range []string{
		"sales.create", "purchasing.create", "customers.view", "suppliers.view",
		"goodsreceipt.create", "delivery.create", "salesreturn.create",
		"purchasereturn.create", "stock.update", "stock.transfer", "stock.approve",
		"salesreturn.refund", "purchasereturn.refund", "sales.export", "purchasing.export",
		"member_types.view", "finance.posting.view", "finance.report.view",
		"finance.expense.create", "finance.journal.view",
		"users.view", "roles.manage", "company.view", "branches.view",
		"assistant.view", "audit.view", "payment.create",
		"finance.view", "reporting.view", "pricing.quote",
		"product_categories.manage", "product_import.manage", "member_types.manage",
	} {
		if !has(admin, want) {
			t.Errorf("admin: want %q dari peta legacy", want)
		}
	}
	// Admin legacy tanpa users.create/update/archive, finance.manage,
	// company/branches.manage: paritas — pastikan peta tidak memberinya
	// diam-diam. finance.post sah via expense.create (paritas mencatat
	// beban; posting jurnal ikut karena granularity kasar — lihat F3b-2).
	for _, notWant := range []string{"users.create", "users.update", "users.archive", "finance.manage", "company.manage", "branches.manage", "finance.posting.post", "finance.journal.reverse"} {
		if has(admin, notWant) {
			t.Errorf("admin: %q tidak boleh muncul (paritas legacy)", notWant)
		}
	}
	staff := MapLegacyCodes(legacyStaff())
	if !has(staff, "sales.create") || has(staff, "finance.view") || has(staff, "roles.manage") {
		t.Errorf("staff: batas paritas dilanggar: %v", staff)
	}
	kasir := MapLegacyCodes(legacyKasir())
	if has(kasir, "payment.create") || has(kasir, "sales.archive") {
		t.Errorf("kasir legacy tanpa payment.*: paritas dilanggar: %v", kasir)
	}
}
