// Package load writes the transformed rows into the revamp database (F5-8).
//
// It talks SQL directly: historical data must not pass through the business
// rules of the new documents. Every statement names its columns explicitly,
// and the live schema is compared with these specs before anything is written
// so a column added by another change is reported instead of silently NULLed.
package load

// Spec describes how one output file lands in one table.
type Spec struct {
	Table string
	Cols  []string
	// PK are the columns that identify a row for the "already loaded?" check.
	PK []string
	// Optional columns are skipped when the live table does not have them.
	Optional map[string]bool
	// Remap names, per column, the resolver table whose temporary ids must be
	// translated to the ids found/created in the target (roles, accounts).
	Remap map[string]string
	// SelfRef is a self-referencing parent column; parents are inserted first.
	SelfRef string
	// Upsert rows overwrite on key conflict (mapping tables); others insert once.
	Upsert bool
	// Source is the output file name when it differs from Table.
	Source string
}

func opt(cols ...string) map[string]bool {
	m := map[string]bool{}
	for _, c := range cols {
		m[c] = true
	}
	return m
}

// Stages is the dependency-ordered load plan. Special stages (roles, accounts,
// role_permissions, company_profile, cutover, rollup, media) are handled in
// load.go between these entries.
var Stages = []Spec{
	{Table: "branches", PK: []string{"id"}, Cols: []string{"id", "code", "name", "address", "city", "phone", "status", "is_head", "created_at", "updated_at"}},
	{Table: "member_types", PK: []string{"id"}, Cols: []string{"id", "code", "name", "description_text", "price_basis", "price_adjustment_direction", "price_adjustment_type", "price_adjustment_value", "rounding_mode", "rounding_increment", "status", "archived_at", "created_at", "updated_at"}},
	{Table: "users", PK: []string{"id"}, Cols: []string{"id", "username", "email", "password_hash", "full_name", "status", "created_at", "updated_at"}},
	{Table: "user_roles", PK: []string{"user_id", "role_id"}, Cols: []string{"user_id", "role_id"}, Remap: map[string]string{"role_id": "roles"}},
	{Table: "user_branch_access", PK: []string{"user_id", "branch_id"}, Cols: []string{"user_id", "branch_id", "is_default"}},
	{Table: "product_categories", PK: []string{"id"}, SelfRef: "parent_id", Cols: []string{"id", "code", "name", "parent_id", "status", "created_at", "updated_at"}},
	{Table: "products", PK: []string{"id"}, Cols: []string{"id", "code", "name", "category_id", "type", "stock_tracked", "base_uom", "purchase_uom", "sales_uom", "purchase_to_base_factor", "sales_to_base_factor", "purchase_price", "selling_price", "min_selling_price", "min_stock_qty", "status", "created_at", "updated_at"}},
	{Table: "product_variants", PK: []string{"id"}, Cols: []string{"id", "product_id", "code", "name", "barcode", "is_default", "status", "created_at", "updated_at"}},
	{Table: "parties", PK: []string{"id"}, Cols: []string{"id", "code", "party_type", "name", "phone", "email", "address_text", "notes", "member_type_id", "status", "archived_at", "created_at", "updated_at"}},
	{Table: "party_delivery_addresses", PK: []string{"id"}, Cols: []string{"id", "party_id", "label", "recipient_name", "phone", "address_text", "is_primary", "sort_order", "status", "archived_at", "created_at", "updated_at"}},
	{Table: "stock_locations", PK: []string{"id"}, SelfRef: "parent_id", Cols: []string{"id", "branch_id", "code", "name", "parent_id", "is_system", "kind", "is_picking_area", "is_primary", "sort_order", "status", "created_at", "updated_at"}, Optional: opt("kind", "is_picking_area", "is_primary", "sort_order")},
	{Table: "finance_account_mappings", PK: []string{"mapping_key"}, Upsert: true, Cols: []string{"mapping_key", "account_id"}, Remap: map[string]string{"account_id": "finance_accounts"}},
	{Table: "purchase_orders", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "party_id", "order_date", "due_date", "payment_terms", "tax_type", "tax_rate", "supplier_invoice_number", "supplier_invoice_date", "tax_invoice_number", "tax_invoice_date", "assigned_user_id", "subtotal", "discount_total", "tax_total", "grand_total", "status", "notes", "created_at", "created_by", "payment_term_days"}, Optional: opt("payment_term_days")},
	{Table: "purchase_order_items", PK: []string{"id"}, Cols: []string{"id", "order_id", "product_id", "variant_id", "product_code", "product_name", "uom", "uom_factor", "qty", "qty_base", "unit_price", "discount_pct", "discount_nominal", "line_total", "stock_tracked", "item_type", "variant_name", "variant_code", "base_uom", "discount_reason", "line_note", "taxable", "tax_rate", "tax_base", "tax_amount"}},
	{Table: "sales_orders", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "party_id", "ship_to_address_id", "ship_to_label", "ship_to_recipient", "ship_to_phone", "ship_to_address", "channel", "member_code", "order_date", "due_date", "payment_terms", "tax_type", "tax_rate", "tax_invoice_number", "tax_invoice_date", "subtotal", "discount_total", "tax_total", "grand_total", "status", "notes", "created_at", "created_by", "client_ref", "completed_at", "assigned_user_id", "credit_approved_by", "credit_approved_at"}, Optional: opt("client_ref", "completed_at", "assigned_user_id", "credit_approved_by", "credit_approved_at")},
	{Table: "sales_order_items", PK: []string{"id"}, Cols: []string{"id", "order_id", "product_id", "variant_id", "product_code", "product_name", "uom", "uom_factor", "qty", "qty_base", "unit_price", "discount_pct", "discount_nominal", "line_total", "stock_tracked", "item_type", "variant_name", "variant_code", "base_uom", "pricing_source", "base_price", "discount_reason", "line_note", "taxable", "tax_base", "tax_amount"}},
	{Table: "sales_returns", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "sales_order_id", "return_date", "total", "subtotal", "discount_total", "tax_total", "tax_type", "tax_rate", "return_mode", "replacement_delivery_status", "status", "notes", "created_at", "updated_at", "created_by", "reason", "replacement_subtotal", "replacement_tax_total", "replacement_total", "difference_amount", "receivable_reduction"}},
	{Table: "sales_return_items", PK: []string{"id"}, Cols: []string{"id", "return_id", "product_id", "variant_id", "location_id", "uom", "uom_factor", "qty", "qty_base", "unit_price", "discount_pct", "discount_nominal", "tax_base", "tax_amount", "line_total", "return_condition"}},
	{Table: "sales_return_replacement_items", PK: []string{"id"}, Cols: []string{"id", "return_id", "product_id", "variant_id", "location_id", "uom", "uom_factor", "qty", "qty_base", "unit_price", "line_total", "tax_base", "tax_amount"}},
	{Table: "sales_return_settlements", PK: []string{"id"}, Cols: []string{"id", "return_id", "settlement_type", "settlement_date", "amount", "payment_method", "reference_number", "notes", "created_at", "created_by", "auto"}},
	{Table: "purchase_returns", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "purchase_order_id", "return_date", "total", "subtotal", "discount_total", "tax_total", "tax_type", "tax_rate", "status", "notes", "created_at", "updated_at", "created_by", "reason", "payable_reduction"}},
	{Table: "purchase_return_items", PK: []string{"id"}, Cols: []string{"id", "return_id", "product_id", "variant_id", "location_id", "uom", "uom_factor", "qty", "qty_base", "unit_price", "discount_pct", "discount_nominal", "tax_base", "tax_amount", "line_total"}},
	{Table: "purchase_return_settlements", PK: []string{"id"}, Cols: []string{"id", "return_id", "settlement_type", "settlement_date", "amount", "payment_method", "reference_number", "notes", "created_at", "created_by"}},
	{Table: "goods_receipts", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "purchase_order_id", "received_at", "notes", "created_at", "created_by", "supplier_delivery_number", "status", "client_ref"}, Optional: opt("client_ref")},
	{Table: "goods_receipt_items", PK: []string{"id"}, Cols: []string{"id", "receipt_id", "product_id", "variant_id", "location_id", "uom", "uom_factor", "qty", "qty_base"}},
	{Table: "delivery_notes", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "sales_order_id", "delivery_date", "driver_name", "vehicle_plate", "warehouse_staff_name", "recipient_name", "recipient_signature_status", "recipient_signature_missing_reason", "drop_location_note", "dispatched_at", "dispatched_by", "confirmed_at", "confirmed_by", "status", "document_kind", "sales_return_id", "notes", "created_at", "created_by", "client_ref"}, Optional: opt("client_ref")},
	{Table: "delivery_note_items", PK: []string{"id"}, Cols: []string{"id", "delivery_id", "product_id", "variant_id", "location_id", "uom", "uom_factor", "qty", "qty_base", "reservation_key"}},
	{Table: "stock_transfers", PK: []string{"id"}, Cols: []string{"id", "number", "from_branch_id", "to_branch_id", "from_location_id", "to_location_id", "status", "notes", "driver_name", "vehicle_plate", "transfer_date", "dispatched_at", "received_at", "created_at", "created_by"}, Optional: opt("driver_name", "vehicle_plate", "transfer_date")},
	{Table: "stock_transfer_items", PK: []string{"id"}, Cols: []string{"id", "transfer_id", "product_id", "variant_id", "qty_base", "notes", "to_location_id"}, Optional: opt("to_location_id")},
	{Table: "payments", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "party_id", "party_type", "direction", "amount", "method", "paid_at", "notes", "status", "created_at", "created_by", "client_ref", "amount_tendered", "reference_number", "pair_id"}, Optional: opt("client_ref", "amount_tendered", "reference_number", "pair_id")},
	{Table: "payment_allocations", PK: []string{"id"}, Cols: []string{"id", "payment_id", "order_type", "order_id", "amount"}},
	{Table: "stock_movements", PK: []string{"id"}, Cols: []string{"id", "branch_id", "product_id", "variant_id", "location_id", "direction", "movement_type", "qty_base", "balance_before", "balance_after", "unit_cost", "ref_type", "ref_id", "notes", "created_by", "created_at", "moved_at"}, Optional: opt("balance_before", "balance_after", "unit_cost", "moved_at")},
	{Table: "stock_balances", PK: []string{"id"}, Cols: []string{"id", "branch_id", "product_id", "variant_id", "location_id", "on_hand", "reserved", "updated_at"}},
	{Table: "finance_inventory_cost_movements", PK: []string{"id"}, Cols: []string{"id", "branch_id", "product_id", "variant_id", "stock_movement_id", "ref_type", "ref_id", "direction", "qty_base", "unit_cost", "total_cost", "is_estimated", "created_at", "moved_at"}, Optional: opt("moved_at")},
	{Table: "finance_journal_entries", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "entry_date", "memo", "source_doc_type", "source_doc_id", "status", "reversed_by", "created_at", "created_by", "scope", "requeue"}, Optional: opt("scope", "requeue")},
	{Table: "finance_journal_lines", PK: []string{"id"}, Cols: []string{"id", "entry_id", "account_id", "product_id", "variant_id", "party_id", "debit", "credit", "description"}, Remap: map[string]string{"account_id": "finance_accounts"}},
	{Table: "business_expenses", PK: []string{"id"}, Cols: []string{"id", "number", "branch_id", "expense_account_id", "pay_account_id", "amount", "expense_date", "notes", "journal_entry_id", "status", "created_at", "created_by", "cancel_reason", "cancelled_at", "cancelled_by", "party_id"}, Optional: opt("cancel_reason", "cancelled_at", "cancelled_by", "party_id"), Remap: map[string]string{"expense_account_id": "finance_accounts", "pay_account_id": "finance_accounts"}},
	{Table: "finance_periods", PK: []string{"id"}, Cols: []string{"id", "branch_id", "year", "month", "status", "closed_at"}},
	{Table: "company_settings", PK: []string{"setting_key"}, Upsert: true, Cols: []string{"setting_key", "setting_value", "updated_at"}},
	{Table: "audit_logs", PK: []string{"id"}, Cols: []string{"id", "action_key", "entity_type", "entity_id", "branch_id", "actor_type", "actor_id", "note", "created_at"}},
	{Table: "media_files", Source: "media_files_final", PK: []string{"file_key"}, Cols: []string{"owner_type", "owner_id", "file_key", "original_name", "mime", "size_bytes", "is_primary", "created_by", "origin"}, Optional: opt("origin")},
}

// Tables lists every table the loader writes (used by the empty-target guard
// and the schema fingerprint).
func Tables() []string {
	seen := map[string]bool{}
	var out []string
	add := func(t string) {
		if !seen[t] {
			seen[t] = true
			out = append(out, t)
		}
	}
	for _, t := range []string{"roles", "role_permissions", "company_profile", "finance_accounts", "finance_cutover", "finance_account_daily", "branch_document_sequences", "finance_document_sequences"} {
		add(t)
	}
	for _, s := range Stages {
		add(s.Table)
	}
	return out
}
