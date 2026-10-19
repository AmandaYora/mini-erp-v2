package bootstrap

// Composition root helper (F1-6): the single aggregated permission catalog.
// Every module describes its own codes in its contracts/ package; this is
// the only place that lists them together, so the server-start seed and the
// ETL role mapping read the same source. Modules meet only through
// contracts/ — bootstrap sees no application/infrastructure/domain internals.
//
// When a new module (or code) is added, append one line to catalogGroups:
// TestCatalogCoversRoutes fails otherwise.

import (
	assistantcontracts "mini-erp/internal/modules/assistant/contracts"
	auditcontracts "mini-erp/internal/modules/audit/contracts"
	dashboardcontracts "mini-erp/internal/modules/dashboard/contracts"
	deliverycontracts "mini-erp/internal/modules/delivery/contracts"
	financecontracts "mini-erp/internal/modules/finance/contracts"
	goodsreceiptcontracts "mini-erp/internal/modules/goodsreceipt/contracts"
	partycontracts "mini-erp/internal/modules/party/contracts"
	paymentcontracts "mini-erp/internal/modules/payment/contracts"
	productcontracts "mini-erp/internal/modules/product/contracts"
	purchasereturncontracts "mini-erp/internal/modules/purchasereturn/contracts"
	purchasingcontracts "mini-erp/internal/modules/purchasing/contracts"
	reportingcontracts "mini-erp/internal/modules/reporting/contracts"
	salescontracts "mini-erp/internal/modules/sales/contracts"
	salesreturncontracts "mini-erp/internal/modules/salesreturn/contracts"
	stockcontracts "mini-erp/internal/modules/stock/contracts"
	usercontracts "mini-erp/internal/modules/user/contracts"
)

// catalogGroups lists every module catalog in seed order (user/L1 first).
func catalogGroups() [][]usercontracts.PermissionSeed {
	return [][]usercontracts.PermissionSeed{
		usercontracts.PermissionCatalog(),
		productcontracts.PermissionCatalog(),
		partycontracts.PermissionCatalog(),
		stockcontracts.PermissionCatalog(),
		purchasingcontracts.PermissionCatalog(),
		salescontracts.PermissionCatalog(),
		goodsreceiptcontracts.PermissionCatalog(),
		deliverycontracts.PermissionCatalog(),
		paymentcontracts.PermissionCatalog(),
		salesreturncontracts.PermissionCatalog(),
		purchasereturncontracts.PermissionCatalog(),
		financecontracts.PermissionCatalog(),
		auditcontracts.PermissionCatalog(),
		assistantcontracts.PermissionCatalog(),
		dashboardcontracts.PermissionCatalog(),
		reportingcontracts.PermissionCatalog(),
	}
}

// AllPermissionSeeds returns every known permission code exactly once.
// Duplicate codes across catalogs are a wiring bug: the first occurrence
// wins here, and DuplicateCodes (asserted empty in tests) reports them.
func AllPermissionSeeds() []usercontracts.PermissionSeed {
	seen := map[string]bool{}
	var out []usercontracts.PermissionSeed
	for _, g := range catalogGroups() {
		for _, p := range g {
			if seen[p.Code] {
				continue
			}
			seen[p.Code] = true
			out = append(out, p)
		}
	}
	return out
}

// AllPermissionCodes is the code list for owner/superadmin sync.
func AllPermissionCodes() []string {
	seeds := AllPermissionSeeds()
	out := make([]string, 0, len(seeds))
	for _, s := range seeds {
		out = append(out, s.Code)
	}
	return out
}

// DuplicateCodes reports codes listed by more than one module catalog
// (must stay empty: one code, one owner).
func DuplicateCodes() []string {
	seen := map[string]int{}
	var dups []string
	for _, g := range catalogGroups() {
		for _, p := range g {
			seen[p.Code]++
			if seen[p.Code] == 2 {
				dups = append(dups, p.Code)
			}
		}
	}
	return dups
}
