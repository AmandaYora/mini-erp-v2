-- Fixture of the LEGACY (NestJS/TypeORM) schema, reduced to the columns the
-- ETL reads. Every table carries the prefix `legacy_` so it can live next to
-- the revamp tables inside the single mini_erp_test database. The placeholder
-- @@CO@@ stands for the legacy tenant column; the test harness substitutes it
-- (the repository's architecture check bans that literal from apps/api).
-- Column names and types follow the legacy entities (L:apps/api/src/modules/*/entities).

CREATE TABLE legacy_etl_copy_marker (id INT PRIMARY KEY, restored_at DATETIME, source_host VARCHAR(100), note VARCHAR(200));
CREATE TABLE legacy_companies (@@CO@@ INT PRIMARY KEY, code VARCHAR(50), name VARCHAR(255), legal_name VARCHAR(255) NULL,
  timezone VARCHAR(100), currency_code VARCHAR(10), locale VARCHAR(20), status VARCHAR(30), created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_branches (id_branch INT PRIMARY KEY, @@CO@@ INT, code VARCHAR(50), name VARCHAR(255), city VARCHAR(120),
  address_text TEXT NULL, phone_e164 VARCHAR(30) NULL, default_stock_location_label VARCHAR(255), status VARCHAR(30),
  is_default TINYINT, created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_users (id_user INT PRIMARY KEY, @@CO@@ INT, full_name VARCHAR(255), email VARCHAR(255) NULL, username VARCHAR(100),
  password_hash TEXT, phone_e164 VARCHAR(30) NULL, display_title VARCHAR(100) NULL, status VARCHAR(30), last_login_at DATETIME(6) NULL,
  created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_roles (id_role INT PRIMARY KEY, @@CO@@ INT NULL, code VARCHAR(50), name VARCHAR(100), description_text TEXT NULL,
  is_system_role TINYINT, created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_permissions (id_permission INT PRIMARY KEY, permission_code VARCHAR(100));
CREATE TABLE legacy_role_permissions (id_role INT, id_permission INT, PRIMARY KEY (id_role, id_permission));
CREATE TABLE legacy_user_roles (id_user INT, id_role INT, PRIMARY KEY (id_user, id_role));
CREATE TABLE legacy_user_branch_accesses (id_user INT, id_branch INT, is_default_branch TINYINT, created_at DATETIME(6), PRIMARY KEY (id_user, id_branch));
CREATE TABLE legacy_member_types (id_member_type INT PRIMARY KEY, @@CO@@ INT, code VARCHAR(50), name VARCHAR(120), description_text TEXT NULL,
  price_basis VARCHAR(30), price_adjustment_direction VARCHAR(20), price_adjustment_type VARCHAR(20), price_adjustment_value DECIMAL(18,4),
  rounding_mode VARCHAR(20), rounding_increment DECIMAL(18,2) NULL, status VARCHAR(30), created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_product_categories (id_product_category INT PRIMARY KEY, @@CO@@ INT, id_parent_category INT NULL, code VARCHAR(50), name VARCHAR(255),
  sort_order INT, created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_products (id_product INT PRIMARY KEY, @@CO@@ INT, id_product_category INT NULL, product_code VARCHAR(50), product_name VARCHAR(255),
  uom VARCHAR(30), product_type VARCHAR(30), status VARCHAR(30), stock_tracked TINYINT, has_variants TINYINT, base_uom VARCHAR(30), purchase_uom VARCHAR(30),
  sales_uom VARCHAR(30), purchase_to_base_factor DECIMAL(24,12), sales_to_base_factor DECIMAL(24,12), min_stock_qty DECIMAL(24,12) NULL,
  purchase_price DECIMAL(18,2) NULL, selling_price DECIMAL(18,2) NULL, min_selling_price DECIMAL(18,2) NULL, attributes_json JSON NULL, summary_text TEXT NULL,
  created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_product_variants (id_product_variant INT PRIMARY KEY, @@CO@@ INT, id_product INT, variant_code VARCHAR(80), variant_name VARCHAR(255),
  attributes_json JSON NULL, barcode VARCHAR(80) NULL, is_default TINYINT, is_hidden TINYINT, status VARCHAR(30), sort_order INT,
  created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_business_parties (id_business_party INT PRIMARY KEY, @@CO@@ INT, party_type VARCHAR(30), id_member_type INT NULL, code VARCHAR(50),
  name VARCHAR(255), phone_e164 VARCHAR(30) NULL, email VARCHAR(255) NULL, address_text TEXT NULL, notes TEXT NULL, metadata_json JSON NULL,
  created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_business_party_delivery_addresses (id_address INT PRIMARY KEY, @@CO@@ INT, id_business_party INT, label VARCHAR(120) NULL,
  recipient_name VARCHAR(150) NULL, phone_e164 VARCHAR(30) NULL, address_text TEXT, is_primary TINYINT, sort_order INT,
  created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_stock_locations (id_stock_location INT PRIMARY KEY, id_branch INT, id_parent_location INT NULL, code VARCHAR(50), name VARCHAR(255),
  is_default TINYINT, is_picking_area TINYINT, is_primary TINYINT, sort_order INT, status VARCHAR(30), archived_at DATETIME(6) NULL,
  created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_order_status_definitions (id_order_status_definition INT PRIMARY KEY, @@CO@@ INT, code VARCHAR(50), label VARCHAR(100),
  status_group VARCHAR(20), applicable_order_kind VARCHAR(30), is_initial TINYINT, is_terminal TINYINT, sort_order INT);
CREATE TABLE legacy_orders (id_order INT PRIMARY KEY, id_branch INT, order_number VARCHAR(100), order_kind VARCHAR(30), source VARCHAR(16),
  id_related_party INT NULL, id_ship_to_address INT NULL, ship_to_label_snapshot VARCHAR(120) NULL, ship_to_recipient_snapshot VARCHAR(150) NULL,
  ship_to_phone_snapshot VARCHAR(30) NULL, ship_to_address_snapshot TEXT NULL, id_current_status INT, id_assigned_user INT NULL,
  order_date DATETIME(6), due_date DATETIME(6) NULL, supplier_invoice_number VARCHAR(100) NULL, supplier_invoice_date DATETIME(6) NULL,
  payment_term_days INT NULL, payment_terms VARCHAR(20), subtotal_amount DECIMAL(18,2), discount_amount DECIMAL(18,2), taxable TINYINT,
  price_includes_tax TINYINT, tax_rate_snapshot DECIMAL(9,4), subtotal_before_tax DECIMAL(18,2), tax_amount DECIMAL(18,2),
  subtotal_after_tax DECIMAL(18,2), tax_invoice_number VARCHAR(100) NULL, tax_invoice_date DATETIME(6) NULL, total_amount DECIMAL(18,2),
  notes TEXT NULL, metadata_json JSON NULL, id_created_by INT, created_at DATETIME(6), updated_at DATETIME(6),
  goods_received_at DATETIME(6) NULL, goods_delivered_at DATETIME(6) NULL, credit_approved TINYINT, credit_approved_by INT NULL,
  credit_approved_at DATETIME(6) NULL, archived_at DATETIME(6) NULL);
CREATE TABLE legacy_order_items (id_order_item INT PRIMARY KEY, id_order INT, id_product INT, id_product_variant INT, line_no INT,
  product_name_snapshot VARCHAR(255), product_code_snapshot VARCHAR(50), variant_name_snapshot VARCHAR(255) NULL, variant_code_snapshot VARCHAR(80) NULL,
  variant_attributes_snapshot JSON NULL, quantity DECIMAL(24,12), transaction_uom_snapshot VARCHAR(30), base_uom_snapshot VARCHAR(30),
  uom_to_base_factor DECIMAL(24,12), quantity_in_base_uom DECIMAL(24,12), pricing_source VARCHAR(30), unit_price_before_discount DECIMAL(18,2),
  unit_price DECIMAL(18,2), line_discount_amount DECIMAL(18,2), line_discount_percent DECIMAL(9,4), discount_reason VARCHAR(255) NULL,
  taxable TINYINT, tax_rate_snapshot DECIMAL(9,4), tax_base_amount DECIMAL(18,2), tax_amount DECIMAL(18,2), line_total_before_tax DECIMAL(18,2),
  line_total_after_tax DECIMAL(18,2), line_total DECIMAL(18,2), stock_tracked_snapshot TINYINT, notes TEXT NULL, created_at DATETIME(6));
CREATE TABLE legacy_delivery_notes (id_delivery_note INT PRIMARY KEY, id_order INT NULL, document_kind VARCHAR(20), id_sales_return INT NULL, id_branch INT,
  sj_number VARCHAR(100), driver_name VARCHAR(100), vehicle_plate VARCHAR(20) NULL, warehouse_staff_name VARCHAR(100), delivery_date DATE, notes TEXT NULL,
  dispatched_at DATETIME(6), dispatched_by INT, confirmed_at DATETIME(6) NULL, confirmed_by INT NULL, recipient_signature_status VARCHAR(30) NULL,
  recipient_name VARCHAR(150) NULL, recipient_signature_missing_reason TEXT NULL, drop_location_note TEXT NULL, archived_at DATETIME(6) NULL, created_at DATETIME(6));
CREATE TABLE legacy_delivery_note_items (id_delivery_note_item INT PRIMARY KEY, id_delivery_note INT, id_order_item INT NULL, id_sales_return_item INT NULL,
  id_product_variant INT, quantity DECIMAL(24,12), quantity_in_base_uom DECIMAL(24,12));
CREATE TABLE legacy_stock_issue_allocations (id_stock_issue_allocation INT PRIMARY KEY, id_branch INT, id_order INT, id_order_item INT,
  id_delivery_note INT NULL, id_delivery_note_item INT NULL, id_product INT, id_product_variant INT, id_stock_location INT,
  quantity DECIMAL(24,12), quantity_in_base_uom DECIMAL(24,12), source_reference_type VARCHAR(50) NULL, id_source_reference INT NULL,
  id_inventory_movement INT NULL, created_at DATETIME(6));
CREATE TABLE legacy_goods_receipts (id_goods_receipt INT PRIMARY KEY, @@CO@@ INT, id_branch INT, id_order INT, id_stock_location INT NULL,
  supplier_delivery_number VARCHAR(100) NULL, received_at DATETIME(6), notes TEXT NULL, id_received_by INT NULL,
  created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_goods_receipt_items (id_goods_receipt_item INT PRIMARY KEY, id_goods_receipt INT, id_order INT, id_order_item INT, id_product INT,
  id_product_variant INT, id_stock_location INT NULL, quantity DECIMAL(24,12), quantity_in_base_uom DECIMAL(24,12), transaction_uom_snapshot VARCHAR(30),
  base_uom_snapshot VARCHAR(30), source_reference_type VARCHAR(50) NULL, id_source_reference INT NULL, id_inventory_movement INT NULL,
  created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_payments (id_payment INT PRIMARY KEY, id_order INT NULL, id_branch INT, id_business_party INT NULL, financial_side VARCHAR(20) NULL,
  payment_number VARCHAR(100), payment_date DATETIME(6), amount DECIMAL(18,2), amount_tendered DECIMAL(18,2) NULL, payment_method VARCHAR(30),
  reference_number VARCHAR(100) NULL, notes TEXT NULL, proof_file_path VARCHAR(500) NULL, id_created_by INT, created_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_payment_allocations (id_payment_allocation INT PRIMARY KEY, id_payment INT, id_order INT, id_branch INT, id_business_party INT NULL,
  financial_side VARCHAR(20), allocated_amount DECIMAL(18,2), allocation_method VARCHAR(30), allocated_at DATETIME(6), created_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_stock_transfers (id_stock_transfer INT PRIMARY KEY, @@CO@@ INT, id_from_branch INT, id_to_branch INT, transfer_number VARCHAR(80),
  status VARCHAR(30), transfer_date DATETIME(6), dispatched_at DATETIME(6) NULL, received_at DATETIME(6) NULL, cancelled_at DATETIME(6) NULL,
  driver_name VARCHAR(150) NULL, vehicle_number VARCHAR(80) NULL, notes TEXT NULL, id_created_by INT NULL, id_dispatched_by INT NULL, id_received_by INT NULL,
  id_cancelled_by INT NULL, archived_at DATETIME(6) NULL, created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_stock_transfer_items (id_stock_transfer_item INT PRIMARY KEY, id_stock_transfer INT, id_product INT, id_product_variant INT,
  id_from_location INT, id_to_location INT, quantity DECIMAL(24,12), id_out_movement INT NULL, id_in_movement INT NULL, notes TEXT NULL);
CREATE TABLE legacy_stock_reservations (id_stock_reservation INT PRIMARY KEY, id_branch INT, reservation_key VARCHAR(100), source_type VARCHAR(30),
  id_user INT NULL, id_order INT NULL, id_product INT, id_product_variant INT, id_stock_location INT, quantity_in_base_uom DECIMAL(24,12),
  status VARCHAR(20), expires_at DATETIME(6), released_at DATETIME(6) NULL, consumed_at DATETIME(6) NULL, created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_inventory_balances (id_inventory_balance INT PRIMARY KEY, id_branch INT, id_product INT, id_product_variant INT, id_stock_location INT,
  on_hand_qty DECIMAL(24,12), reserved_qty DECIMAL(24,12), available_qty DECIMAL(24,12), updated_at DATETIME(6));
CREATE TABLE legacy_inventory_movements (id_inventory_movement INT PRIMARY KEY, id_branch INT, id_product INT, id_product_variant INT, id_stock_location INT,
  movement_type VARCHAR(30), quantity DECIMAL(24,12), balance_before DECIMAL(24,12), balance_after DECIMAL(24,12), reference_type VARCHAR(30) NULL,
  id_reference INT NULL, reason_text TEXT NULL, id_moved_by INT NULL, moved_at DATETIME(6), metadata_json JSON NULL);
CREATE TABLE legacy_finance_accounts (id_finance_account INT PRIMARY KEY, @@CO@@ INT, account_code VARCHAR(50), account_name VARCHAR(255), account_type VARCHAR(30),
  normal_balance VARCHAR(10), parent_account_id INT NULL, sort_order INT, system_key VARCHAR(80) NULL, status VARCHAR(30), archived_at DATETIME(6) NULL,
  created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_finance_cash_accounts (id_finance_cash_account INT PRIMARY KEY, @@CO@@ INT, cash_account_code VARCHAR(50), cash_account_name VARCHAR(255),
  cash_account_type VARCHAR(20), id_finance_account INT, is_default TINYINT, status VARCHAR(30), archived_at DATETIME(6) NULL,
  created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_finance_periods (id_finance_period INT PRIMARY KEY, @@CO@@ INT, period_code VARCHAR(20), date_from DATE, date_to DATE, status VARCHAR(20),
  closed_at DATETIME(6) NULL, closed_by INT NULL, created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_finance_posting_sources (id_finance_posting_source INT PRIMARY KEY, @@CO@@ INT, id_branch INT NULL, source_type VARCHAR(50), id_source INT,
  event_key VARCHAR(80), source_number VARCHAR(120) NULL, source_date DATETIME(6), source_amount DECIMAL(18,2), posting_status VARCHAR(20),
  id_journal_entry INT NULL, error_message TEXT NULL, pending_reason_code VARCHAR(40) NULL, source_hash VARCHAR(64) NULL, metadata_json JSON NULL,
  created_at DATETIME(6), updated_at DATETIME(6), posted_at DATETIME(6) NULL, posted_by INT NULL);
CREATE TABLE legacy_finance_journal_entries (id_finance_journal_entry INT PRIMARY KEY, @@CO@@ INT, id_branch INT NULL, journal_number VARCHAR(120),
  journal_date DATETIME(6), posting_type VARCHAR(20), source_type VARCHAR(50) NULL, id_source INT NULL, event_key VARCHAR(80) NULL, description VARCHAR(500) NULL,
  status VARCHAR(20), id_reversal_of INT NULL, posted_at DATETIME(6) NULL, posted_by INT NULL, reversed_at DATETIME(6) NULL, reversed_by INT NULL,
  metadata_json JSON NULL, created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_finance_journal_lines (id_finance_journal_line INT PRIMARY KEY, id_finance_journal_entry INT, @@CO@@ INT, id_branch INT NULL,
  id_finance_account INT, line_no INT, debit_amount DECIMAL(18,2), credit_amount DECIMAL(18,2), description VARCHAR(500) NULL, id_product INT NULL,
  id_product_variant INT NULL, id_business_party INT NULL, id_order INT NULL, id_payment INT NULL, id_inventory_movement INT NULL, metadata_json JSON NULL,
  created_at DATETIME(6));
CREATE TABLE legacy_finance_inventory_cost_movements (id_finance_inventory_cost_movement INT PRIMARY KEY, @@CO@@ INT, id_branch INT, id_product INT,
  id_product_variant INT NULL, id_inventory_movement INT NULL, movement_type VARCHAR(30), source_type VARCHAR(50), id_source INT, movement_date DATETIME(6),
  quantity_in_base_uom DECIMAL(24,12), unit_cost_before DECIMAL(24,12), unit_cost_after DECIMAL(24,12), total_cost_amount DECIMAL(18,2),
  inventory_value_before DECIMAL(18,2), inventory_value_after DECIMAL(18,2), metadata_json JSON NULL, created_at DATETIME(6));
CREATE TABLE legacy_business_expenses (id_business_expense INT PRIMARY KEY, @@CO@@ INT, id_branch INT NULL, expense_number VARCHAR(60), expense_date DATE,
  id_finance_account_expense INT, id_finance_account_payment INT, id_business_party INT NULL, description VARCHAR(500) NULL, amount DECIMAL(18,2),
  status VARCHAR(20), id_finance_journal_entry INT NULL, id_reversal_journal_entry INT NULL, cancelled_at DATETIME(6) NULL, cancelled_by INT NULL,
  cancel_reason VARCHAR(500) NULL, created_by INT NULL, created_at DATETIME(6), updated_at DATETIME(6));
CREATE TABLE legacy_media_files (id_media_file INT PRIMARY KEY, @@CO@@ INT, owner_type VARCHAR(50), owner_id INT, purpose VARCHAR(50), storage_driver VARCHAR(20),
  bucket VARCHAR(255) NULL, object_key VARCHAR(700), original_name VARCHAR(255) NULL, mime_type VARCHAR(120), size_bytes BIGINT, visibility VARCHAR(20),
  checksum VARCHAR(128) NULL, is_primary TINYINT, sort_order INT, metadata_json JSON NULL, id_created_by INT NULL, created_at DATETIME(6),
  updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_branch_document_sequences (id_branch_document_sequence INT PRIMARY KEY, id_branch INT, sequence_key VARCHAR(50), prefix VARCHAR(30) NULL,
  current_value BIGINT, reset_policy VARCHAR(30), format_template VARCHAR(100) NULL, updated_at DATETIME(6));
CREATE TABLE legacy_finance_document_sequences (id_finance_document_sequence INT PRIMARY KEY, @@CO@@ INT, sequence_key VARCHAR(50), prefix VARCHAR(30) NULL,
  current_value BIGINT);
CREATE TABLE legacy_sales_returns (id_sales_return INT PRIMARY KEY, @@CO@@ INT, id_branch INT, return_number VARCHAR(100), id_original_order INT,
  id_related_party INT NULL, source VARCHAR(30), status VARCHAR(30), return_mode VARCHAR(30), return_date DATETIME(6), reason_text TEXT,
  returned_subtotal_before_tax DECIMAL(18,2), returned_tax_amount DECIMAL(18,2), returned_total_amount DECIMAL(18,2),
  replacement_subtotal_before_tax DECIMAL(18,2), replacement_tax_amount DECIMAL(18,2), replacement_total_amount DECIMAL(18,2), difference_amount DECIMAL(18,2),
  settlement_type VARCHAR(30), replacement_delivery_status VARCHAR(20), id_created_by INT, created_at DATETIME(6), updated_at DATETIME(6),
  cancelled_at DATETIME(6) NULL, cancelled_by INT NULL, cancel_reason TEXT NULL, archived_at DATETIME(6) NULL);
CREATE TABLE legacy_sales_return_items (id_sales_return_item INT PRIMARY KEY, id_sales_return INT, line_type VARCHAR(30), id_original_order_item INT NULL,
  id_product INT, id_product_variant INT, line_no INT, product_name_snapshot VARCHAR(255), product_code_snapshot VARCHAR(50),
  quantity DECIMAL(24,12), transaction_uom_snapshot VARCHAR(30), base_uom_snapshot VARCHAR(30), uom_to_base_factor DECIMAL(24,12), quantity_in_base_uom DECIMAL(24,12),
  unit_price_before_discount DECIMAL(18,2), unit_price DECIMAL(18,2), line_discount_amount DECIMAL(18,2), line_discount_percent DECIMAL(9,4),
  taxable TINYINT, tax_rate_snapshot DECIMAL(9,4), tax_base_amount DECIMAL(18,2), tax_amount DECIMAL(18,2), line_total_before_tax DECIMAL(18,2),
  line_total_after_tax DECIMAL(18,2), line_total DECIMAL(18,2), stock_tracked_snapshot TINYINT, return_condition VARCHAR(30) NULL,
  id_stock_location INT NULL, id_inventory_movement INT NULL, notes TEXT NULL);
CREATE TABLE legacy_sales_return_settlements (id_sales_return_settlement INT PRIMARY KEY, id_sales_return INT, settlement_type VARCHAR(30),
  settlement_date DATETIME(6), amount DECIMAL(18,2), payment_method VARCHAR(30) NULL, reference_number VARCHAR(100) NULL, notes TEXT NULL,
  id_created_by INT, created_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_purchase_returns (id_purchase_return INT PRIMARY KEY, @@CO@@ INT, id_branch INT, return_number VARCHAR(100), id_original_order INT,
  id_related_party INT NULL, status VARCHAR(30), return_date DATETIME(6), reason_text TEXT, returned_subtotal_before_tax DECIMAL(18,2),
  returned_tax_amount DECIMAL(18,2), returned_total_amount DECIMAL(18,2), settlement_type VARCHAR(30), cancels_order TINYINT, id_created_by INT,
  created_at DATETIME(6), updated_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_purchase_return_items (id_purchase_return_item INT PRIMARY KEY, id_purchase_return INT, id_original_order_item INT,
  id_product INT, id_product_variant INT, line_no INT, product_name_snapshot VARCHAR(255), product_code_snapshot VARCHAR(50),
  quantity DECIMAL(24,12), transaction_uom_snapshot VARCHAR(30), base_uom_snapshot VARCHAR(30), uom_to_base_factor DECIMAL(24,12), quantity_in_base_uom DECIMAL(24,12),
  unit_price DECIMAL(18,2), tax_rate_snapshot DECIMAL(9,4), tax_base_amount DECIMAL(18,2), tax_amount DECIMAL(18,2), line_total_before_tax DECIMAL(18,2),
  line_total DECIMAL(18,2), stock_tracked_snapshot TINYINT, id_stock_location INT NULL, id_inventory_movement INT NULL, notes TEXT NULL);
CREATE TABLE legacy_purchase_return_settlements (id_purchase_return_settlement INT PRIMARY KEY, id_purchase_return INT, settlement_type VARCHAR(30),
  settlement_date DATETIME(6), amount DECIMAL(18,2), payment_method VARCHAR(30) NULL, reference_number VARCHAR(100) NULL, notes TEXT NULL,
  id_created_by INT, created_at DATETIME(6), archived_at DATETIME(6) NULL);
CREATE TABLE legacy_company_settings (@@CO@@ INT PRIMARY KEY, business_labels_json JSON NULL, operational_rules_json JSON NULL, ui_preferences_json JSON NULL,
  assistant_preferences_json JSON NULL, reporting_preferences_json JSON NULL, updated_at DATETIME(6));
CREATE TABLE legacy_audit_logs (id_audit_log INT PRIMARY KEY, @@CO@@ INT, id_branch INT NULL, actor_type VARCHAR(30), id_actor INT NULL, action_key VARCHAR(100),
  entity_type VARCHAR(50), id_entity INT NULL, before_json JSON NULL, after_json JSON NULL, metadata_json JSON NULL, happened_at DATETIME(6));
CREATE TABLE legacy_finance_inventory_cost_states (id_finance_inventory_cost_state INT PRIMARY KEY, @@CO@@ INT, id_branch INT, id_product INT,
  id_product_variant INT NULL, inventory_qty DECIMAL(24,12), inventory_value DECIMAL(18,2), unit_cost DECIMAL(24,12), updated_at DATETIME(6));
