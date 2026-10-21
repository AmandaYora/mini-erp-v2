package bootstrap

// Legacy permission map (F1-6): translates the old system's permission codes
// (packages/shared-contracts/src/permission-code.ts) to this repo's codes.
// The ETL role migration (stage K) imports this instead of duplicating the
// table. Codes that exist in neither the map nor DroppedLegacyCodes are a
// bug: MapLegacyCodes keeps them out, and TestLegacyMap fails loudly.
//
// Mapping rules follow design F3b Lampiran A: legacy order.* fans out to the
// revamp order domains; finance/stock/refund/export codes keep legacy
// granularity; legacy knowledge.* (RAG) is by-design dropped.

import (
	"sort"

	usercontracts "mini-erp/internal/modules/user/contracts"
)

// legacyToRevamp maps one legacy code to revamp codes (design F3b Lampiran A,
// one row per legacy code). Empty = dropped (must then appear in
// DroppedLegacyCodes, enforced by test).
func legacyToRevamp() map[string][]string {
	return map[string][]string{
		"dashboard.view":     {"dashboard.view"},
		"product.view":       {"products.view"},
		"product.create":     {"products.create", "product_import.manage"},
		"product.update":     {"products.update", "product_categories.manage"},
		"product.archive":    {"products.archive", "product_categories.manage"},
		"member_type.view":   {"member_types.view"},
		"member_type.manage": {"member_types.manage", "member_types.view"},
		"order.view": {
			"sales.view", "purchasing.view", "customers.view", "suppliers.view",
			"goodsreceipt.view", "delivery.view",
		},
		"order.create": {
			"sales.create", "purchasing.create", "customers.create", "suppliers.create",
			"pricing.quote",
		},
		// Legacy order.update covers confirm/approve-credit/receive/deliver
		// and cancelling a delivery note (Lampiran A rows 10-11).
		"order.update": {
			"sales.update", "sales.approve", "purchasing.update", "customers.update",
			"suppliers.update", "delivery.create", "delivery.archive", "goodsreceipt.create",
		},
		"order.archive":          {"sales.archive", "purchasing.archive", "customers.archive", "suppliers.archive"},
		"order.export":           {"sales.export", "purchasing.export"},
		"sales_return.view":      {"salesreturn.view"},
		"sales_return.create":    {"salesreturn.create"},
		"sales_return.refund":    {"salesreturn.refund"},
		"sales_return.cancel":    {"salesreturn.archive"},
		"purchase_return.view":   {"purchasereturn.view"},
		"purchase_return.create": {"purchasereturn.create"},
		"purchase_return.refund": {"purchasereturn.refund"},
		"purchase_return.cancel": {"purchasereturn.archive"},
		"stock.view":             {"stock.view"},
		"stock.update":           {"stock.update"},
		"stock.transfer":         {"stock.transfer"},
		"stock.adjust":           {"stock.adjust"},
		"stock.adjust.approve":   {"stock.approve"},
		"reporting.view":         {"reporting.view"},
		"user.view":              {"users.view"},
		"user.create":            {"users.create"},
		"user.update":            {"users.update"},
		"user.archive":           {"users.archive"},
		"company_config.view":    {"company.view"},
		"company_config.manage":  {"company.manage"},
		"branch.view":            {"branches.view"},
		"branch.manage":          {"branches.manage"},
		"role.manage":            {"roles.view", "roles.manage"},
		"whatsapp.view":          {"assistant.view"},
		"whatsapp.manage":        {"assistant.manage"},
		"whatsapp.simulate":      {"assistant.simulate"},
		"audit_log.view":         {"audit.view"},
		"payment.view":           {"payment.view"},
		"payment.create":         {"payment.create"},
		"payment.archive":        {"payment.archive"},
		// Finance keeps legacy's granular codes 1:1 (F3b-2).
		"finance.view":                  {"finance.view"},
		"finance.manage":                {"finance.manage"},
		"finance.close.force":           {"finance.close.force"},
		"finance.posting.view":          {"finance.posting.view"},
		"finance.posting.post":          {"finance.posting.post"},
		"finance.journal.view":          {"finance.journal.view"},
		"finance.journal.reverse":       {"finance.journal.reverse"},
		"finance.report.view":           {"finance.report.view"},
		"finance.tax_report.view":       {"finance.tax_report.view"},
		"finance.expense.view":          {"finance.expense.view"},
		"finance.expense.create":        {"finance.expense.create"},
		"finance.expense.cancel":        {"finance.expense.cancel"},
		"finance.tax_adjustment.view":   {"finance.tax_adjustment.view"},
		"finance.tax_adjustment.manage": {"finance.tax_adjustment.manage"},
	}
}

// DroppedLegacyCodes are legacy codes with no revamp equivalent (by-design:
// knowledge/RAG never ships here). Anything unmapped outside this list fails
// TestLegacyMap.
func DroppedLegacyCodes() []string {
	return []string{"knowledge.view", "knowledge.create", "knowledge.update", "knowledge.archive"}
}

// MapLegacyCodes translates legacy codes to deduplicated revamp codes that
// exist in this repo's catalog. Holders of legacy order.create additionally
// receive pricing.quote (member pricing use) and stock.reserve (POS holds). Unknown codes are dropped
// (reported by UnmappedLegacyCodes, asserted in tests).
func MapLegacyCodes(legacy []string) []string {
	table := legacyToRevamp()
	known := map[string]bool{}
	for _, s := range AllPermissionSeeds() {
		known[s.Code] = true
	}
	out := map[string]bool{}
	hasOrderCreate := false
	for _, code := range legacy {
		if code == "order.create" {
			hasOrderCreate = true
		}
		for _, mapped := range table[code] {
			if known[mapped] {
				out[mapped] = true
			}
		}
	}
	if hasOrderCreate && known["pricing.quote"] {
		out["pricing.quote"] = true
	}
	// POS carts hold stock while they are built (F2a-12).
	if hasOrderCreate && known["stock.reserve"] {
		out["stock.reserve"] = true
	}
	list := make([]string, 0, len(out))
	for code := range out {
		list = append(list, code)
	}
	sort.Strings(list)
	return list
}

// UnmappedLegacyCodes returns input codes that map to nothing (neither a
// known revamp code nor the drop list).
func UnmappedLegacyCodes(legacy []string) []string {
	table := legacyToRevamp()
	known := map[string]bool{}
	for _, s := range AllPermissionSeeds() {
		known[s.Code] = true
	}
	dropped := map[string]bool{}
	for _, c := range DroppedLegacyCodes() {
		dropped[c] = true
	}
	var out []string
	for _, code := range legacy {
		mapped, ok := table[code]
		if !ok {
			if !dropped[code] {
				out = append(out, code)
			}
			continue
		}
		anyKnown := false
		for _, m := range mapped {
			if known[m] {
				anyKnown = true
				break
			}
		}
		if !anyKnown && !dropped[code] {
			out = append(out, code)
		}
	}
	return out
}

// legacyAdmin/Staff/Kasir are verbatim copies of the old system's defaults
// (permission-code.ts DEFAULT_ROLE_PERMISSIONS + seed-runner.ts kasirPerms),
// the input DefaultGrants maps to revamp codes.
func legacyAdmin() []string {
	return []string{
		"dashboard.view",
		"product.view", "product.create", "product.update", "product.archive",
		"member_type.view", "member_type.manage",
		"order.view", "order.create", "order.update", "order.archive", "order.export",
		"sales_return.view", "sales_return.create", "sales_return.refund", "sales_return.cancel",
		"purchase_return.view", "purchase_return.create", "purchase_return.refund", "purchase_return.cancel",
		"stock.view", "stock.update", "stock.transfer", "stock.adjust", "stock.adjust.approve",
		"reporting.view",
		"user.view",
		"company_config.view",
		"branch.view",
		"role.manage",
		"knowledge.view", "knowledge.create",
		"whatsapp.view",
		"audit_log.view",
		"payment.view", "payment.create", "payment.archive",
		"finance.view", "finance.posting.view", "finance.journal.view", "finance.report.view", "finance.tax_report.view",
		"finance.expense.view", "finance.expense.create",
		"finance.tax_adjustment.view",
	}
}

func legacyStaff() []string {
	return []string{
		"dashboard.view",
		"product.view",
		"order.view", "order.create", "order.update",
		"sales_return.view", "sales_return.create",
		"purchase_return.view",
		"stock.view", "stock.adjust",
		"payment.view", "payment.create",
	}
}

func legacyKasir() []string {
	return []string{
		"dashboard.view", "product.view", "order.view", "order.create", "order.update",
		"sales_return.view", "sales_return.create",
		"stock.view", "stock.adjust",
	}
}

// kasirPaymentGrants are granted to the default kasir role on top of its
// legacy codes. Legacy kasir had no payment.* (BR-18 in
// docs/legacy-reference/15-pos), so a cash POS checkout created the order and
// then failed at "confirm delivery" (a cash order needs its settlement):
// an orphan order. Taking payment is the cashier's job, so the default role
// carries it; payment.view lets the POS find the payment for the receipt.
// Being a default grant, it reaches existing databases once on the next start
// (authz_seed_applied) and is never re-added if an admin removes it.
var kasirPaymentGrants = []string{"payment.create", "payment.view"}

// DefaultGrants maps role code -> revamp codes granted once on first seed.
// Admin/staff/kasir keep their customizations afterwards (tracked in
// authz_seed_applied); owner/superadmin re-sync to everything each run.
func DefaultGrants() map[string][]string {
	return map[string][]string{
		"admin": MapLegacyCodes(legacyAdmin()),
		"staff": MapLegacyCodes(legacyStaff()),
		"kasir": append(MapLegacyCodes(legacyKasir()), kasirPaymentGrants...),
	}
}

// SystemRoles are the roles EnsureAuthz maintains on every start.
func SystemRoles() []usercontracts.SystemRoleDef {
	return []usercontracts.SystemRoleDef{
		{Code: "superadmin", Name: "Super Admin", Description: "Akses developer default (disembunyikan)", System: true, SyncAll: true},
		{Code: "owner", Name: "Owner", Description: "Akses penuh", System: true, SyncAll: true},
		{Code: "admin", Name: "Admin", Description: "Admin operasional", System: true},
		{Code: "staff", Name: "Staff", Description: "Staff operasional", System: true},
		{Code: "kasir", Name: "Kasir", Description: "Kasir POS", System: false},
	}
}
