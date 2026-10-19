// Package registry lists every legacy table the ETL reads: its key, how to
// restrict it to the migrated company, and what happens to it. Tenant
// restriction is explicit per table because most legacy tables carry no
// company column at all (design V2): they are reached through the branch.
package registry

// Entity describes one legacy table. Tenant is a SQL predicate template (see
// legacydb.Q). PK has several columns for link tables, which are streamed in
// one ordered pass instead of keyset pages.
type Entity struct {
	Name   string
	PK     []string
	Tenant string
	// Disposition documents what the transform does with it.
	Disposition string
}

// tenantPK is the legacy company key (assembled: see legacydb.tenantCol).
const tenantPK = "id_" + "company"

const byBranch = "id_branch IN (SELECT id_branch FROM {branches} WHERE {co} = :c)"

// All is the extraction order; the biggest tables are last (design F5-3).
var All = []Entity{
	{"companies", []string{tenantPK}, "{co} = :c", "company_profile"},
	{"branches", []string{"id_branch"}, "{co} = :c", "branches"},
	{"users", []string{"id_user"}, "{co} = :c", "users"},
	{"roles", []string{"id_role"}, "({co} = :c OR {co} IS NULL)", "roles"},
	{"permissions", []string{"id_permission"}, "1 = 1", "role_permissions (mapped)"},
	{"role_permissions", []string{"id_role", "id_permission"},
		"id_role IN (SELECT id_role FROM {roles} WHERE {co} = :c OR {co} IS NULL)", "role_permissions (mapped)"},
	{"user_roles", []string{"id_user", "id_role"},
		"id_user IN (SELECT id_user FROM {users} WHERE {co} = :c)", "user_roles"},
	{"user_branch_accesses", []string{"id_user", "id_branch"}, byBranch, "user_branch_access"},
	{"member_types", []string{"id_member_type"}, "{co} = :c", "member_types"},
	{"product_categories", []string{"id_product_category"}, "{co} = :c", "product_categories"},
	{"products", []string{"id_product"}, "{co} = :c", "products"},
	{"product_variants", []string{"id_product_variant"}, "{co} = :c", "product_variants"},
	{"business_parties", []string{"id_business_party"}, "{co} = :c", "parties"},
	{"business_party_delivery_addresses", []string{"id_address"}, "{co} = :c", "party_delivery_addresses"},
	{"stock_locations", []string{"id_stock_location"}, byBranch, "stock_locations"},
	{"order_status_definitions", []string{"id_order_status_definition"}, "{co} = :c", "status map only"},
	{"orders", []string{"id_order"}, byBranch, "sales_orders / purchase_orders"},
	{"order_items", []string{"id_order_item"},
		"id_order IN (SELECT id_order FROM {orders} WHERE " + byBranch + ")", "*_order_items"},
	{"delivery_notes", []string{"id_delivery_note"}, byBranch, "delivery_notes"},
	{"delivery_note_items", []string{"id_delivery_note_item"},
		"id_delivery_note IN (SELECT id_delivery_note FROM {delivery_notes} WHERE " + byBranch + ")", "delivery_note_items (split per location)"},
	{"stock_issue_allocations", []string{"id_stock_issue_allocation"}, byBranch, "location source for delivery items"},
	{"goods_receipts", []string{"id_goods_receipt"}, "{co} = :c", "goods_receipts"},
	{"goods_receipt_items", []string{"id_goods_receipt_item"},
		"id_goods_receipt IN (SELECT id_goods_receipt FROM {goods_receipts} WHERE {co} = :c)", "goods_receipt_items"},
	{"payments", []string{"id_payment"}, byBranch, "payments"},
	{"payment_allocations", []string{"id_payment_allocation"},
		"id_payment IN (SELECT id_payment FROM {payments} WHERE " + byBranch + ")", "payment_allocations"},
	{"stock_transfers", []string{"id_stock_transfer"}, "{co} = :c", "stock_transfers (split per location pair)"},
	{"stock_transfer_items", []string{"id_stock_transfer_item"},
		"id_stock_transfer IN (SELECT id_stock_transfer FROM {stock_transfers} WHERE {co} = :c)", "stock_transfer_items"},
	{"stock_reservations", []string{"id_stock_reservation"}, byBranch, "not migrated (transient)"},
	{"finance_accounts", []string{"id_finance_account"}, "{co} = :c", "finance_accounts"},
	{"finance_cash_accounts", []string{"id_finance_cash_account"}, "{co} = :c", "finance_accounts.is_cash"},
	{"finance_periods", []string{"id_finance_period"}, "{co} = :c", "finance_periods"},
	{"finance_posting_sources", []string{"id_finance_posting_source"}, "{co} = :c", "posting marks"},
	{"business_expenses", []string{"id_business_expense"}, "{co} = :c", "business_expenses"},
	{"media_files", []string{"id_media_file"}, "{co} = :c", "media_files"},
	{"branch_document_sequences", []string{"id_branch_document_sequence"}, byBranch, "sequences (rebuilt)"},
	{"company_settings", []string{tenantPK}, "{co} = :c", "company_settings (mapped keys) + archive"},
	{"sales_returns", []string{"id_sales_return"}, "{co} = :c", "sales_returns"},
	{"sales_return_items", []string{"id_sales_return_item"},
		"id_sales_return IN (SELECT id_sales_return FROM {sales_returns} WHERE {co} = :c)", "sales_return_items / sales_return_replacement_items"},
	{"sales_return_settlements", []string{"id_sales_return_settlement"},
		"id_sales_return IN (SELECT id_sales_return FROM {sales_returns} WHERE {co} = :c)", "sales_return_settlements (+ payments)"},
	{"purchase_returns", []string{"id_purchase_return"}, "{co} = :c", "purchase_returns"},
	{"purchase_return_items", []string{"id_purchase_return_item"},
		"id_purchase_return IN (SELECT id_purchase_return FROM {purchase_returns} WHERE {co} = :c)", "purchase_return_items"},
	{"purchase_return_settlements", []string{"id_purchase_return_settlement"},
		"id_purchase_return IN (SELECT id_purchase_return FROM {purchase_returns} WHERE {co} = :c)", "purchase_return_settlements (+ payments)"},
	{"audit_logs", []string{"id_audit_log"}, "{co} = :c", "audit_logs (current year loaded, all archived)"},
	// Largest tables last.
	{"inventory_balances", []string{"id_inventory_balance"}, byBranch, "stock_balances"},
	{"inventory_movements", []string{"id_inventory_movement"}, byBranch, "stock_movements"},
	{"finance_inventory_cost_movements", []string{"id_finance_inventory_cost_movement"}, "{co} = :c", "finance_inventory_cost_movements"},
	{"finance_journal_entries", []string{"id_finance_journal_entry"}, "{co} = :c", "finance_journal_entries"},
	{"finance_journal_lines", []string{"id_finance_journal_line"}, "{co} = :c", "finance_journal_lines"},
}

// ByName finds an entity.
func ByName(name string) (Entity, bool) {
	for _, e := range All {
		if e.Name == name {
			return e, true
		}
	}
	return Entity{}, false
}
