-- generated fixture data (see legacy_schema.sql)
INSERT INTO legacy_etl_copy_marker (id, restored_at, source_host, note) VALUES
(1, '2025-11-01 00:00:00.000000', 'localhost', 'fixture');
INSERT INTO legacy_companies (@@CO@@, code, name, legal_name, timezone, currency_code, locale, status, created_at, updated_at) VALUES
(1, 'ACME', 'PT Acme', 'PT Acme Sejahtera', 'Asia/Jakarta', 'IDR', 'id-ID', 'active', '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 'OTH', 'PT Lain', NULL, 'Asia/Jakarta', 'IDR', 'id-ID', 'active', '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_branches (id_branch, @@CO@@, code, name, city, address_text, phone_e164, default_stock_location_label, status, is_default, created_at, updated_at) VALUES
(1, 1, 'PST', 'Pusat', 'Jakarta', 'Jl. Uji 1', '+62810000001', 'Gudang Utama', 'active', 1, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 1, 'CBG', 'Cabang Dua', 'Jakarta', 'Jl. Uji 2', '+62810000002', 'Gudang Utama', 'active', 0, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(9, 2, 'XXX', 'Luar Perusahaan', 'Jakarta', 'Jl. Uji 9', '+62810000009', 'Gudang Utama', 'active', 1, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_users (id_user, @@CO@@, full_name, email, username, password_hash, phone_e164, display_title, status, last_login_at, created_at, updated_at) VALUES
(1, 1, 'Super Admin', 'superadmin@acme.test', 'superadmin', '$2a$12$xnw/bwZKY/Lhg2BHoJYvTOYSDFibiEnEAzPWz/dSbCgC0PfwRtrlK', NULL, 'Staf', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 1, 'Pemilik Toko', 'owner@acme.test', 'owner', '$2a$12$FtLWPoCTCFHvbAhFkbtDFOa2/ImcT/VxRq0nuCG22StQJXXL4NpHC', NULL, 'Staf', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(3, 1, 'Admin Toko', 'admin@acme.test', 'admin', '$2a$12$xnw/bwZKY/Lhg2BHoJYvTOYSDFibiEnEAzPWz/dSbCgC0PfwRtrlK', NULL, 'Staf', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(4, 1, 'Kasir Satu', 'kasir@acme.test', 'kasir', '$2a$12$te6A9z1418IvlOosbTLfTewkTDZFlaNJQftP3YDuXi1/fk8mJeNSu', NULL, 'Staf', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(5, 2, 'Orang Lain', 'lain@acme.test', 'lain', '$2a$12$FtLWPoCTCFHvbAhFkbtDFOa2/ImcT/VxRq0nuCG22StQJXXL4NpHC', NULL, 'Staf', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_roles (id_role, @@CO@@, code, name, description_text, is_system_role, created_at, updated_at) VALUES
(1, NULL, 'superadmin', 'Super Admin', NULL, 1, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 1, 'owner', 'Owner', NULL, 1, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(3, 1, 'admin', 'Admin', NULL, 1, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(4, 1, 'kasir', 'Kasir', NULL, 0, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_permissions (id_permission, permission_code) VALUES
(1, 'order.view'),
(2, 'order.create'),
(3, 'payment.view'),
(4, 'knowledge.view');
INSERT INTO legacy_role_permissions (id_role, id_permission) VALUES
(2, 1),
(2, 2),
(2, 3),
(4, 1),
(4, 4);
INSERT INTO legacy_user_roles (id_user, id_role) VALUES
(1, 1),
(2, 2),
(3, 3),
(4, 4);
INSERT INTO legacy_user_branch_accesses (id_user, id_branch, is_default_branch, created_at) VALUES
(2, 1, 1, '2025-11-01 00:00:00.000000'),
(2, 2, 0, '2025-11-01 00:00:00.000000'),
(4, 2, 1, '2025-11-01 00:00:00.000000');
INSERT INTO legacy_member_types (id_member_type, @@CO@@, code, name, description_text, price_basis, price_adjustment_direction, price_adjustment_type, price_adjustment_value, rounding_mode, rounding_increment, status, created_at, updated_at, archived_at) VALUES
(1, 1, 'GOLD', 'Gold', NULL, 'selling_price', 'subtract', 'percent', '5.0000', 'none', NULL, 'active', '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_product_categories (id_product_category, @@CO@@, id_parent_category, code, name, sort_order, created_at, updated_at, archived_at) VALUES
(1, 1, NULL, 'ELEK', 'Elektronik', 1, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(2, 1, 1, 'HP', 'Handphone', 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_products (id_product, @@CO@@, id_product_category, product_code, product_name, uom, product_type, status, stock_tracked, has_variants, base_uom, purchase_uom, sales_uom, purchase_to_base_factor, sales_to_base_factor, min_stock_qty, purchase_price, selling_price, min_selling_price, attributes_json, summary_text, created_at, updated_at, archived_at) VALUES
(1, 1, 1, 'BRG-001', 'Barang Satu', 'pcs', 'physical', 'active', 1, 0, 'pcs', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '5.000000000000', '10000.00', '15000.50', '14000.00', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(2, 1, 1, 'BRG-002', 'Barang Dua', 'pcs', 'physical', 'active', 1, 0, 'pcs', 'dus', 'pcs', '12.000000000000', '1.000000000000', '5.000000000000', '12000.00', '20000.00', '18000.00', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(3, 1, 2, 'BRG-003', 'Barang Tiga', 'pcs', 'physical', 'active', 1, 0, 'pcs', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '5.000000000000', '8000.00', '11100.00', '10000.00', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(4, 1, NULL, 'JSA-001', 'Jasa Servis', 'pcs', 'service', 'active', 0, 0, 'pcs', 'pcs', 'pcs', '1.000000000000', '1.000000000000', NULL, NULL, '50000.00', NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(5, 1, 1, 'BDL-001', 'Paket Bundel', 'pcs', 'bundle', 'active', 1, 0, 'pcs', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '5.000000000000', '30000.00', '45000.00', '40000.00', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(6, 1, NULL, 'NST-001', 'Non Stok', 'pcs', 'non_stock', 'active', 0, 0, 'pcs', 'pcs', 'pcs', '1.000000000000', '1.000000000000', NULL, '1000.00', '2000.00', '1500.00', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_product_variants (id_product_variant, @@CO@@, id_product, variant_code, variant_name, attributes_json, barcode, is_default, is_hidden, status, sort_order, created_at, updated_at, archived_at) VALUES
(1, 1, 1, 'BRG-001-DEF', 'Default', NULL, '', 1, 0, 'active', 1, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(2, 1, 2, 'BRG-002-S', 'Ukuran S', NULL, '8991', 1, 0, 'active', 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(3, 1, 2, 'BRG-002-M', 'Ukuran M', NULL, '8992', 0, 0, 'active', 3, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(4, 1, 3, 'BRG-003-DEF', 'Default', NULL, '', 1, 0, 'active', 4, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(5, 1, 4, 'JSA-001-DEF', 'Default', NULL, NULL, 1, 0, 'active', 5, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(6, 1, 5, 'BDL-001-DEF', 'Default', NULL, NULL, 1, 0, 'active', 6, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(7, 1, 6, 'NST-001-DEF', 'Default', NULL, NULL, 1, 0, 'active', 7, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_business_parties (id_business_party, @@CO@@, party_type, id_member_type, code, name, phone_e164, email, address_text, notes, metadata_json, created_at, updated_at, archived_at) VALUES
(1, 1, 'customer', 1, 'CUS-001', 'Pelanggan Satu', '+628111', NULL, 'Alamat Pelanggan Satu', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(2, 1, 'customer', NULL, 'CUS-002', 'Pelanggan Dua', '+628112', NULL, 'Alamat Pelanggan Dua', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(3, 1, 'supplier', NULL, 'SUP-001', 'Pemasok Satu', '+628113', NULL, 'Alamat Pemasok Satu', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(4, 1, 'supplier', NULL, 'SUP-002', 'Pemasok Dua', '+628114', NULL, 'Alamat Pemasok Dua', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_business_party_delivery_addresses (id_address, @@CO@@, id_business_party, label, recipient_name, phone_e164, address_text, is_primary, sort_order, created_at, updated_at, archived_at) VALUES
(1, 1, 1, 'Gudang', 'Budi', '+62811', 'Jl. Kirim 1', 1, 1, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_stock_locations (id_stock_location, id_branch, id_parent_location, code, name, is_default, is_picking_area, is_primary, sort_order, status, archived_at, created_at, updated_at) VALUES
(1, 1, NULL, 'GDG', 'Gudang Utama', 1, 0, 1, 1, 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 1, NULL, 'RAK-A', 'Rak A', 0, 0, 0, 2, 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(3, 1, NULL, 'RSK1', 'Rusak 1', 0, 0, 0, 3, 'damaged', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(4, 1, NULL, 'RSK2', 'Rusak 2', 0, 0, 0, 4, 'damaged', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(5, 2, NULL, 'GDG2', 'Gudang Cabang', 1, 0, 1, 5, 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_order_status_definitions (id_order_status_definition, @@CO@@, code, label, status_group, applicable_order_kind, is_initial, is_terminal, sort_order) VALUES
(1, 1, 'draft', 'draft', 'pending', 'all', 1, 0, 1),
(2, 1, 'confirmed', 'confirmed', 'active', 'all', 0, 0, 2),
(3, 1, 'in_progress', 'in_progress', 'active', 'all', 0, 0, 3),
(4, 1, 'completed', 'completed', 'completed', 'all', 0, 1, 4),
(5, 1, 'cancelled', 'cancelled', 'cancelled', 'all', 0, 1, 5);
INSERT INTO legacy_orders (id_order, id_branch, order_number, order_kind, source, id_related_party, id_ship_to_address, ship_to_label_snapshot, ship_to_recipient_snapshot, ship_to_phone_snapshot, ship_to_address_snapshot, id_current_status, id_assigned_user, order_date, due_date, supplier_invoice_number, supplier_invoice_date, payment_term_days, payment_terms, subtotal_amount, discount_amount, taxable, price_includes_tax, tax_rate_snapshot, subtotal_before_tax, tax_amount, subtotal_after_tax, tax_invoice_number, tax_invoice_date, total_amount, notes, metadata_json, id_created_by, created_at, updated_at, goods_received_at, goods_delivered_at, credit_approved, credit_approved_by, credit_approved_at, archived_at) VALUES
(1, 1, 'ORD-PST/PJ/2025/12/00001', 'sales', 'manual', 1, 1, 'Gudang', 'Budi', NULL, 'Jl. Kirim 1', 4, NULL, '2025-12-10 03:00:00.000000', '2026-01-09 03:00:00.000000', NULL, NULL, NULL, 'net', '210000.00', '5000.00', 1, 0, '11.0000', '205000.00', '22550.00', '227550.00', '010.000-25.0001', NULL, '227550.00', NULL, NULL, 2, '2025-12-10 03:00:00.000000', '2025-12-10 03:00:00.000000', NULL, NULL, 0, NULL, NULL, NULL),
(2, 1, 'ORD-PST/PJ/2026/01/00001', 'sales', 'manual', 2, NULL, NULL, NULL, NULL, NULL, 4, NULL, '2026-01-15 03:00:00.000000', NULL, NULL, NULL, NULL, 'cod', '55500.00', '0.00', 1, 1, '11.0000', '55500.00', '5500.00', '55500.00', NULL, NULL, '55500.00', NULL, NULL, 2, '2026-01-15 03:00:00.000000', '2026-01-15 03:00:00.000000', NULL, NULL, 0, NULL, NULL, NULL),
(3, 1, 'ORD-PST/PJ/2026/02/00001', 'sales', 'pos', NULL, NULL, NULL, NULL, NULL, NULL, 4, NULL, '2026-02-02 03:00:00.000000', NULL, NULL, NULL, NULL, 'cod', '30001.00', '0.00', 0, 0, '0.0000', '30001.00', '0.00', '30001.00', NULL, NULL, '30001.00', NULL, NULL, 2, '2026-02-02 03:00:00.000000', '2026-02-02 03:00:00.000000', NULL, '2026-02-02 04:00:00.000000', 0, NULL, NULL, NULL),
(4, 1, 'ORD-PST/PJ/2026/02/00002', 'sales', 'manual', 1, NULL, NULL, NULL, NULL, NULL, 5, NULL, '2026-02-05 03:00:00.000000', NULL, NULL, NULL, NULL, 'cod', '15000.00', '0.00', 0, 0, '0.0000', '15000.00', '0.00', '15000.00', NULL, NULL, '15000.00', NULL, NULL, 2, '2026-02-05 03:00:00.000000', '2026-02-05 03:00:00.000000', NULL, NULL, 0, NULL, NULL, '2026-02-06 03:00:00.000000'),
(5, 1, 'ORD-PST/PJ/2026/03/00001', 'sales', 'manual', 1, NULL, NULL, NULL, NULL, NULL, 1, NULL, '2026-03-01 03:00:00.000000', NULL, NULL, NULL, NULL, 'prepaid', '20000.00', '0.00', 0, 0, '0.0000', '20000.00', '0.00', '20000.00', NULL, NULL, '20000.00', NULL, NULL, 2, '2026-03-01 03:00:00.000000', '2026-03-01 03:00:00.000000', NULL, NULL, 0, NULL, NULL, NULL),
(6, 2, 'ORD-CBG/PJ/2026/02/00001', 'sales', 'manual', 2, NULL, NULL, NULL, NULL, NULL, 4, NULL, '2026-02-10 03:00:00.000000', NULL, NULL, NULL, 14, 'net', '60000.00', '0.00', 0, 0, '0.0000', '60000.00', '0.00', '60000.00', NULL, NULL, '60000.00', NULL, NULL, 2, '2026-02-10 03:00:00.000000', '2026-02-10 03:00:00.000000', NULL, NULL, 0, NULL, NULL, NULL),
(7, 1, 'ORD-PST/PB/2025/12/00001', 'purchase', 'manual', 3, NULL, NULL, NULL, NULL, NULL, 4, NULL, '2025-12-05 03:00:00.000000', '2026-01-04 03:00:00.000000', 'INV-S-1', '2025-12-05 03:00:00.000000', NULL, 'net', '1000000.00', '0.00', 1, 0, '11.0000', '1000000.00', '110000.00', '1110000.00', NULL, NULL, '1110000.00', NULL, NULL, 2, '2025-12-05 03:00:00.000000', '2025-12-05 03:00:00.000000', NULL, NULL, 0, NULL, NULL, NULL),
(8, 1, 'ORD-PST/PB/2026/01/00001', 'purchase', 'manual', 4, NULL, NULL, NULL, NULL, NULL, 4, NULL, '2026-01-20 03:00:00.000000', '2026-02-19 03:00:00.000000', NULL, NULL, NULL, 'net', '444000.00', '0.00', 1, 1, '11.0000', '444000.00', '44000.00', '444000.00', NULL, NULL, '444000.00', NULL, NULL, 2, '2026-01-20 03:00:00.000000', '2026-01-20 03:00:00.000000', NULL, NULL, 0, NULL, NULL, NULL),
(9, 1, 'ORD-PST/PB/2026/03/00001', 'purchase', 'manual', 3, NULL, NULL, NULL, NULL, NULL, 1, NULL, '2026-03-05 03:00:00.000000', NULL, NULL, NULL, NULL, 'cod', '18000.00', '0.00', 0, 0, '0.0000', '18000.00', '0.00', '18000.00', NULL, NULL, '18000.00', NULL, NULL, 2, '2026-03-05 03:00:00.000000', '2026-03-05 03:00:00.000000', NULL, NULL, 0, NULL, NULL, NULL),
(10, 9, 'ORD-XXX/PJ/2026/01/00001', 'sales', 'manual', NULL, NULL, NULL, NULL, NULL, NULL, 4, NULL, '2026-01-05 03:00:00.000000', NULL, NULL, NULL, NULL, 'cod', '1000.00', '0.00', 0, 0, '0.0000', '1000.00', '0.00', '1000.00', NULL, NULL, '1000.00', NULL, NULL, 2, '2026-01-05 03:00:00.000000', '2026-01-05 03:00:00.000000', NULL, NULL, 0, NULL, NULL, NULL);
INSERT INTO legacy_order_items (id_order_item, id_order, id_product, id_product_variant, line_no, product_name_snapshot, product_code_snapshot, variant_name_snapshot, variant_code_snapshot, variant_attributes_snapshot, quantity, transaction_uom_snapshot, base_uom_snapshot, uom_to_base_factor, quantity_in_base_uom, pricing_source, unit_price_before_discount, unit_price, line_discount_amount, line_discount_percent, discount_reason, taxable, tax_rate_snapshot, tax_base_amount, tax_amount, line_total_before_tax, line_total_after_tax, line_total, stock_tracked_snapshot, notes, created_at) VALUES
(1, 1, 1, 1, 1, 'Barang Satu', 'CODE-1', NULL, 'V-1', NULL, '10.000000000000', 'pcs', 'pcs', '1.000000000000', '10.000000000000000000000000', 'catalog', '15000.00', '14500.00', '500.00', '0.0000', NULL, 1, '11.0000', '145000.00', '15950.00', '145000.00', '160950.00', '160950.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(2, 1, 2, 2, 2, 'Barang Dua', 'CODE-2', 'Ukuran S', 'V-2', '{"warna":"hitam"}', '3.000000000000', 'pcs', 'pcs', '1.000000000000', '3.000000000000000000000000', 'catalog', '20000.00', '20000.00', '0.00', '0.0000', NULL, 1, '11.0000', '60000.00', '6600.00', '60000.00', '66600.00', '66600.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(3, 2, 3, 4, 1, 'Barang Tiga', 'CODE-3', NULL, 'V-4', NULL, '5.000000000000', 'pcs', 'pcs', '1.000000000000', '5.000000000000000000000000', 'catalog', '11100.00', '11100.00', '0.00', '0.0000', NULL, 1, '11.0000', '50000.00', '5500.00', '55500.00', '55500.00', '55500.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(4, 3, 1, 1, 1, 'Barang Satu', 'CODE-1', NULL, 'V-1', NULL, '2.000000000000', 'pcs', 'pcs', '1.000000000000', '2.000000000000000000000000', 'catalog', '15000.50', '15000.50', '0.00', '0.0000', NULL, 0, '0.0000', '30001.00', '0.00', '30001.00', '30001.00', '30001.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(5, 4, 1, 1, 1, 'Barang Satu', 'CODE-1', NULL, 'V-1', NULL, '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000000000000000', 'catalog', '15000.00', '15000.00', '0.00', '0.0000', NULL, 0, '0.0000', '15000.00', '0.00', '15000.00', '15000.00', '15000.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(6, 5, 2, 2, 1, 'Barang Dua', 'CODE-2', 'Ukuran S', 'V-2', '{"warna":"hitam"}', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000000000000000', 'catalog', '20000.00', '20000.00', '0.00', '0.0000', NULL, 0, '0.0000', '20000.00', '0.00', '20000.00', '20000.00', '20000.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(7, 6, 1, 1, 1, 'Barang Satu', 'CODE-1', NULL, 'V-1', NULL, '4.000000000000', 'pcs', 'pcs', '1.000000000000', '4.000000000000000000000000', 'catalog', '15000.00', '15000.00', '0.00', '0.0000', NULL, 0, '0.0000', '60000.00', '0.00', '60000.00', '60000.00', '60000.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(8, 7, 1, 1, 1, 'Barang Satu', 'CODE-1', NULL, 'V-1', NULL, '100.000000000000', 'pcs', 'pcs', '1.000000000000', '100.000000000000000000000000', 'catalog', '10000.00', '10000.00', '0.00', '0.0000', NULL, 1, '11.0000', '1000000.00', '110000.00', '1000000.00', '1110000.00', '1110000.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(9, 8, 3, 4, 1, 'Barang Tiga', 'CODE-3', NULL, 'V-4', NULL, '50.000000000000', 'pcs', 'pcs', '1.000000000000', '50.000000000000000000000000', 'catalog', '8880.00', '8880.00', '0.00', '0.0000', NULL, 1, '11.0000', '400000.00', '44000.00', '444000.00', '444000.00', '444000.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(10, 9, 2, 2, 1, 'Barang Dua', 'CODE-2', 'Ukuran S', 'V-2', '{"warna":"hitam"}', '1.000000000000', 'dus', 'pcs', '12.000000000000', '12.000000000000000000000000', 'catalog', '18000.00', '18000.00', '0.00', '0.0000', NULL, 0, '0.0000', '18000.00', '0.00', '18000.00', '18000.00', '18000.00', 1, NULL, '2025-11-01 00:00:00.000000'),
(11, 10, 1, 1, 1, 'Barang Satu', 'CODE-1', NULL, 'V-1', NULL, '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000000000000000', 'catalog', '1000.00', '1000.00', '0.00', '0.0000', NULL, 0, '0.0000', '1000.00', '0.00', '1000.00', '1000.00', '1000.00', 1, NULL, '2025-11-01 00:00:00.000000');
INSERT INTO legacy_delivery_notes (id_delivery_note, id_order, document_kind, id_sales_return, id_branch, sj_number, driver_name, vehicle_plate, warehouse_staff_name, delivery_date, notes, dispatched_at, dispatched_by, confirmed_at, confirmed_by, recipient_signature_status, recipient_name, recipient_signature_missing_reason, drop_location_note, archived_at, created_at) VALUES
(1, 1, 'order', NULL, 1, 'SJ-PST/2025/00001', 'Pak Sopir', 'B 1234 XY', 'Gudang', '2025-12-11', NULL, '2025-12-11 04:00:00.000000', 2, '2025-12-11 05:00:00.000000', 2, 'signed', 'Budi', NULL, NULL, NULL, '2025-12-11 04:00:00.000000'),
(2, 2, 'order', NULL, 1, 'SJ-PST/2026/00001', 'Pak Sopir', 'B 1234 XY', 'Gudang', '2026-01-16', NULL, '2026-01-16 04:00:00.000000', 2, '2026-01-16 05:00:00.000000', 2, 'signed', 'Budi', NULL, NULL, NULL, '2026-01-16 04:00:00.000000'),
(3, 6, 'order', NULL, 2, 'SJ-CBG/2026/00001', 'Pak Sopir', 'B 1234 XY', 'Gudang', '2026-02-11', NULL, '2026-02-11 04:00:00.000000', 2, '2026-02-11 05:00:00.000000', 2, 'signed', 'Budi', NULL, NULL, NULL, '2026-02-11 04:00:00.000000'),
(4, 1, 'order', NULL, 1, 'SJ-PST/2026/00002', 'Pak Sopir', 'B 1234 XY', 'Gudang', '2026-01-30', NULL, '2026-01-30 04:00:00.000000', 2, '2026-01-30 05:00:00.000000', 2, 'signed', 'Budi', NULL, NULL, '2026-01-31 03:00:00.000000', '2026-01-30 04:00:00.000000');
INSERT INTO legacy_delivery_note_items (id_delivery_note_item, id_delivery_note, id_order_item, id_sales_return_item, id_product_variant, quantity, quantity_in_base_uom) VALUES
(1, 1, 1, NULL, 1, '10.000000000000', '10.000000000000'),
(2, 1, 2, NULL, 2, '3.000000000000', '3.000000000000'),
(3, 2, 3, NULL, 4, '5.000000000000', '5.000000000000'),
(4, 3, 7, NULL, 1, '4.000000000000', '4.000000000000'),
(5, 4, 1, NULL, 1, '1.000000000000', '1.000000000000');
INSERT INTO legacy_stock_issue_allocations (id_stock_issue_allocation, id_branch, id_order, id_order_item, id_delivery_note, id_delivery_note_item, id_product, id_product_variant, id_stock_location, quantity, quantity_in_base_uom, source_reference_type, id_source_reference, id_inventory_movement, created_at) VALUES
(1, 1, 1, 1, 1, 1, 1, 1, 1, '6.000000000000', '6.000000000000', 'delivery_note', 1, NULL, '2025-11-01 00:00:00.000000'),
(2, 1, 1, 1, 1, 1, 1, 1, 2, '4.000000000000', '4.000000000000', 'delivery_note', 1, NULL, '2025-11-01 00:00:00.000000'),
(3, 1, 1, 2, 1, 2, 2, 2, 1, '3.000000000000', '3.000000000000', 'delivery_note', 1, NULL, '2025-11-01 00:00:00.000000'),
(4, 1, 2, 3, 2, 3, 3, 4, 2, '5.000000000000', '5.000000000000', 'delivery_note', 2, NULL, '2025-11-01 00:00:00.000000'),
(5, 2, 6, 7, 3, 4, 1, 1, 5, '4.000000000000', '4.000000000000', 'delivery_note', 3, NULL, '2025-11-01 00:00:00.000000'),
(6, 1, 1, 1, 4, 5, 1, 1, 1, '1.000000000000', '1.000000000000', 'delivery_note', 4, NULL, '2025-11-01 00:00:00.000000');
INSERT INTO legacy_goods_receipts (id_goods_receipt, @@CO@@, id_branch, id_order, id_stock_location, supplier_delivery_number, received_at, notes, id_received_by, created_at, updated_at, archived_at) VALUES
(1, 1, 1, 7, 1, 'SJS-001', '2025-12-06 03:00:00.000000', NULL, 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(2, 1, 1, 8, NULL, NULL, '2026-01-21 03:00:00.000000', 'diterima gudang', 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_goods_receipt_items (id_goods_receipt_item, id_goods_receipt, id_order, id_order_item, id_product, id_product_variant, id_stock_location, quantity, quantity_in_base_uom, transaction_uom_snapshot, base_uom_snapshot, source_reference_type, id_source_reference, id_inventory_movement, created_at, updated_at) VALUES
(1, 1, 7, 8, 1, 1, 1, '100.000000000000', '100.000000000000', 'pcs', 'pcs', NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 2, 8, 9, 3, 4, NULL, '50.000000000000', '50.000000000000', 'pcs', 'pcs', NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_payments (id_payment, id_order, id_branch, id_business_party, financial_side, payment_number, payment_date, amount, amount_tendered, payment_method, reference_number, notes, proof_file_path, id_created_by, created_at, archived_at) VALUES
(1, 1, 1, NULL, 'receivable', 'PAY-PST/2025/00001', '2025-12-31 17:30:00.000000', '100000.00', NULL, 'cash', NULL, NULL, 'transfer/2025-12-31/proof1.jpg', 2, '2025-12-31 17:30:00.000000', NULL),
(2, NULL, 1, 2, 'receivable', 'PAY-PST/2026/00002', '2026-01-16 03:00:00.000000', '55500.00', NULL, 'bank_transfer', 'TRX-1', NULL, NULL, 2, '2026-01-16 03:00:00.000000', NULL),
(3, 3, 1, NULL, NULL, 'PAY-PST/2026/00003', '2026-02-02 03:30:00.000000', '30001.00', '50000.00', 'cash', NULL, NULL, NULL, 2, '2026-02-02 03:30:00.000000', NULL),
(4, 7, 1, NULL, 'payable', 'PAY-PST/2025/00004', '2025-12-20 03:00:00.000000', '500000.00', NULL, 'bank_transfer', NULL, NULL, NULL, 2, '2025-12-20 03:00:00.000000', NULL),
(5, 6, 2, NULL, 'receivable', 'PAY-CBG/2026/00001', '2026-02-11 03:00:00.000000', '60000.00', NULL, 'check', NULL, 'cek BCA 12345', NULL, 2, '2026-02-11 03:00:00.000000', NULL);
INSERT INTO legacy_payment_allocations (id_payment_allocation, id_payment, id_order, id_branch, id_business_party, financial_side, allocated_amount, allocation_method, allocated_at, created_at, archived_at) VALUES
(1, 2, 2, 1, 2, 'receivable', '55500.00', 'manual', '2026-01-16 03:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_inventory_movements (id_inventory_movement, id_branch, id_product, id_product_variant, id_stock_location, movement_type, quantity, balance_before, balance_after, reference_type, id_reference, reason_text, id_moved_by, moved_at, metadata_json) VALUES
(1, 1, 2, 2, 1, 'in', '20.000000000000', '0E-12', '20.000000000000', 'opening_stock', NULL, NULL, 2, '2025-11-30 02:00:00.000000', NULL),
(2, 1, 2, 3, 1, 'in', '5.000000000000', '0E-12', '5.000000000000', 'opening_stock', NULL, NULL, 2, '2025-11-30 02:00:00.000000', NULL),
(3, 1, 3, 4, 2, 'in', '10.000000000000', '0E-12', '10.000000000000', 'opening_stock', NULL, NULL, 2, '2025-11-30 02:00:00.000000', NULL),
(4, 1, 1, 1, 2, 'in', '10.000000000000', '0E-12', '10.000000000000', 'opening_stock', NULL, NULL, 2, '2025-11-30 02:00:00.000000', NULL),
(5, 1, 1, 1, 1, 'in', '100.000000000000', '0E-12', '100.000000000000', 'goods_receipt', 1, NULL, 2, '2025-12-06 03:00:00.000000', NULL),
(6, 1, 1, 1, 1, 'out', '6.000000000000', '100.000000000000', '94.000000000000', 'delivery_note', 1, NULL, 2, '2025-12-11 04:00:00.000000', NULL),
(7, 1, 1, 1, 2, 'out', '4.000000000000', '10.000000000000', '6.000000000000', 'delivery_note', 1, NULL, 2, '2025-12-11 04:00:00.000000', NULL),
(8, 1, 2, 2, 1, 'out', '3.000000000000', '20.000000000000', '17.000000000000', 'delivery_note', 1, NULL, 2, '2025-12-11 04:00:00.000000', NULL),
(9, 1, 1, 1, 1, 'out', '5.000000000000', '94.000000000000', '89.000000000000', 'stock_transfer', 1, NULL, 2, '2026-01-05 03:00:00.000000', NULL),
(10, 2, 1, 1, 5, 'in', '5.000000000000', '0E-12', '5.000000000000', 'stock_transfer', 1, NULL, 2, '2026-01-05 03:00:00.000000', NULL),
(11, 1, 3, 4, 2, 'out', '3.000000000000', '10.000000000000', '7.000000000000', 'stock_transfer', 1, NULL, 2, '2026-01-05 03:00:00.000000', NULL),
(12, 2, 3, 4, 5, 'in', '3.000000000000', '0E-12', '3.000000000000', 'stock_transfer', 1, NULL, 2, '2026-01-05 03:00:00.000000', NULL),
(13, 1, 3, 4, 2, 'out', '5.000000000000', '7.000000000000', '2.000000000000', 'delivery_note', 2, NULL, 2, '2026-01-16 04:00:00.000000', NULL),
(14, 1, 3, 4, 1, 'in', '50.000000000000', '0E-12', '50.000000000000', 'goods_receipt', 2, NULL, 2, '2026-01-21 03:00:00.000000', NULL),
(15, 1, 1, 1, 1, 'out', '2.000000000000', '89.000000000000', '87.000000000000', 'order', 3, NULL, 2, '2026-02-02 04:00:00.000000', NULL),
(16, 1, 1, 1, 1, 'adjustment', '1.000000000000', '87.000000000000', '86.000000000000', 'manual_adjustment', NULL, 'koreksi hitung', 2, '2026-02-03 04:00:00.000000', NULL),
(17, 2, 1, 1, 5, 'out', '2.000000000000', '5.000000000000', '3.000000000000', 'delivery_note', 3, NULL, 2, '2026-02-11 04:00:00.000000', NULL),
(18, 2, 1, 1, 5, 'out', '2.000000000000', '3.000000000000', '1.000000000000', 'delivery_note', 3, NULL, 2, '2026-02-11 04:00:00.000000', NULL),
(19, 1, 1, 1, 1, 'out', '1.000000000000', '86.000000000000', '85.000000000000', 'damaged_stock', NULL, 'rusak', 2, '2026-02-12 04:00:00.000000', NULL),
(20, 1, 1, 1, 3, 'in', '1.000000000000', '0E-12', '1.000000000000', 'damaged_stock', NULL, 'rusak', 2, '2026-02-12 04:00:00.000000', NULL),
(21, 1, 1, 1, 4, 'in', '1.000000000000', '0E-12', '1.000000000000', 'damaged_stock', NULL, 'rusak lain', 2, '2026-02-13 04:00:00.000000', NULL);
INSERT INTO legacy_inventory_balances (id_inventory_balance, id_branch, id_product, id_product_variant, id_stock_location, on_hand_qty, reserved_qty, available_qty, updated_at) VALUES
(1, 1, 1, 1, 1, '85.000000000000', '0.000000000000', '85.000000000000', '2025-11-01 00:00:00.000000'),
(2, 1, 1, 1, 2, '6.000000000000', '0.000000000000', '6.000000000000', '2025-11-01 00:00:00.000000'),
(3, 1, 1, 1, 3, '1.000000000000', '0.000000000000', '1.000000000000', '2025-11-01 00:00:00.000000'),
(4, 1, 1, 1, 4, '1.000000000000', '0.000000000000', '1.000000000000', '2025-11-01 00:00:00.000000'),
(5, 1, 2, 2, 1, '17.000000000000', '0.000000000000', '17.000000000000', '2025-11-01 00:00:00.000000'),
(6, 1, 2, 3, 1, '6.000000000000', '0.000000000000', '6.000000000000', '2025-11-01 00:00:00.000000'),
(7, 1, 3, 4, 1, '50.000000000000', '0.000000000000', '50.000000000000', '2025-11-01 00:00:00.000000'),
(8, 1, 3, 4, 2, '2.000000000000', '0.000000000000', '2.000000000000', '2025-11-01 00:00:00.000000'),
(9, 2, 1, 1, 5, '1.000000000000', '0.000000000000', '1.000000000000', '2025-11-01 00:00:00.000000'),
(10, 2, 3, 4, 5, '3.000000000000', '0.000000000000', '3.000000000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_stock_transfers (id_stock_transfer, @@CO@@, id_from_branch, id_to_branch, transfer_number, status, transfer_date, dispatched_at, received_at, cancelled_at, driver_name, vehicle_number, notes, id_created_by, id_dispatched_by, id_received_by, id_cancelled_by, archived_at, created_at, updated_at) VALUES
(1, 1, 1, 2, 'TRF-PST/2026/00001', 'received', '2026-01-05 03:00:00.000000', '2026-01-05 03:00:00.000000', '2026-01-05 03:30:00.000000', NULL, 'Pak Sopir', 'B 1', 'mutasi uji', 2, 2, 2, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_stock_transfer_items (id_stock_transfer_item, id_stock_transfer, id_product, id_product_variant, id_from_location, id_to_location, quantity, id_out_movement, id_in_movement, notes) VALUES
(1, 1, 1, 1, 1, 5, '5.000000000000', NULL, NULL, NULL),
(2, 1, 3, 4, 2, 5, '3.000000000000', NULL, NULL, 'rak a');
INSERT INTO legacy_finance_accounts (id_finance_account, @@CO@@, account_code, account_name, account_type, normal_balance, parent_account_id, sort_order, system_key, status, archived_at, created_at, updated_at) VALUES
(1, 1, '1000', 'Kas Tunai', 'asset', 'debit', NULL, 1, 'cash_cash', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 1, '1010', 'Rekening Bank', 'asset', 'debit', NULL, 2, 'cash_bank', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(3, 1, '1100', 'Piutang Customer', 'asset', 'debit', NULL, 3, 'accounts_receivable', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(4, 1, '1200', 'Persediaan', 'asset', 'debit', NULL, 4, 'inventory', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(5, 1, '1300', 'PPN Masukan', 'asset', 'debit', NULL, 5, 'input_tax', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(6, 1, '2000', 'Hutang Supplier', 'liability', 'credit', NULL, 6, 'accounts_payable', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(7, 1, '2100', 'PPN Keluaran', 'liability', 'credit', NULL, 7, 'output_tax', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(8, 1, '2200', 'Uang Muka Customer', 'liability', 'credit', NULL, 8, 'customer_advance', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(9, 1, '2300', 'Uang Muka Supplier', 'asset', 'debit', NULL, 9, 'supplier_advance', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(10, 1, '3000', 'Modal Awal', 'equity', 'credit', NULL, 10, 'opening_equity', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(11, 1, '4000', 'Penjualan Barang', 'revenue', 'credit', NULL, 11, 'sales_revenue', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(12, 1, '5000', 'HPP', 'expense', 'debit', NULL, 12, 'cogs', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(13, 1, '5100', 'Selisih Stok', 'expense', 'debit', NULL, 13, 'stock_difference', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(14, 1, '5200', 'Pembulatan', 'expense', 'debit', NULL, 14, 'rounding_difference', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(15, 1, '5300', 'Selisih Retur Beli', 'expense', 'debit', NULL, 15, 'purchase_return_difference', 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(16, 1, '6900', 'Beban Lain-lain', 'expense', 'debit', NULL, 16, NULL, 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(17, 1, '6110', 'Beban Parkir', 'expense', 'debit', NULL, 17, NULL, 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_finance_cash_accounts (id_finance_cash_account, @@CO@@, cash_account_code, cash_account_name, cash_account_type, id_finance_account, is_default, status, archived_at, created_at, updated_at) VALUES
(1, 1, 'KAS', 'Kas', 'cash', 1, 1, 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 1, 'BANK', 'Bank', 'bank', 2, 0, 'active', NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_finance_journal_entries (id_finance_journal_entry, @@CO@@, id_branch, journal_number, journal_date, posting_type, source_type, id_source, event_key, description, status, id_reversal_of, posted_at, posted_by, reversed_at, reversed_by, metadata_json, created_at, updated_at, archived_at) VALUES
(1, 1, 1, 'JE-PST/2025/00001', '2025-12-06 03:00:00.000000', 'auto', 'goods_receipt', 1, 'purchase_received', 'Penerimaan barang SJS-001', 'posted', NULL, '2025-12-06 03:00:00.000000', 2, NULL, NULL, NULL, '2025-12-06 03:00:00.000000', '2025-12-06 03:00:00.000000', NULL),
(2, 1, 1, 'JE-PST/2025/00002', '2025-12-10 03:00:00.000000', 'auto', 'order', 1, 'sales_completed', 'Penjualan ORD-PST/PJ/2025/12/00001', 'posted', NULL, '2025-12-10 03:00:00.000000', 2, NULL, NULL, NULL, '2025-12-10 03:00:00.000000', '2025-12-10 03:00:00.000000', NULL),
(3, 1, 1, 'JE-PST/2025/00003', '2025-12-11 04:00:00.000000', 'auto', 'inventory_movement', 6, 'stock_issue_cogs', 'HPP SJ-PST/2025/00001', 'posted', NULL, '2025-12-11 04:00:00.000000', 2, NULL, NULL, NULL, '2025-12-11 04:00:00.000000', '2025-12-11 04:00:00.000000', NULL),
(4, 1, 1, 'JE-PST/2026/00001', '2025-12-31 17:30:00.000000', 'auto', 'payment', 1, 'customer_payment', 'Pembayaran PAY-PST/2025/00001', 'posted', NULL, '2025-12-31 17:30:00.000000', 2, NULL, NULL, NULL, '2025-12-31 17:30:00.000000', '2025-12-31 17:30:00.000000', NULL),
(5, 1, NULL, 'JE-OPEN/2025/00001', '2025-11-30 02:00:00.000000', 'auto', NULL, NULL, 'opening_balance', 'Saldo awal legacy', 'posted', NULL, '2025-11-30 02:00:00.000000', 2, NULL, NULL, NULL, '2025-11-30 02:00:00.000000', '2025-11-30 02:00:00.000000', NULL),
(6, 1, 1, 'JE-PST/2026/00002', '2026-01-16 03:00:00.000000', 'auto', 'payment', 2, 'customer_payment', 'Pembayaran PAY-PST/2026/00002', 'posted', NULL, '2026-01-16 03:00:00.000000', 2, NULL, NULL, NULL, '2026-01-16 03:00:00.000000', '2026-01-16 03:00:00.000000', NULL),
(7, 1, 1, 'JE-PST/2026/00003', '2026-01-20 03:00:00.000000', 'auto', 'business_expense', 1, 'business_expense', 'Parkir Januari', 'posted', NULL, '2026-01-20 03:00:00.000000', 2, NULL, NULL, NULL, '2026-01-20 03:00:00.000000', '2026-01-20 03:00:00.000000', NULL),
(8, 1, 1, 'JE-PST/2026/00004', '2026-02-02 03:00:00.000000', 'auto', 'order', 3, 'sales_completed', 'Penjualan POS', 'posted', NULL, '2026-02-02 03:00:00.000000', 2, NULL, NULL, NULL, '2026-02-02 03:00:00.000000', '2026-02-02 03:00:00.000000', NULL),
(9, 1, 1, 'JE-PST/2026/00005', '2026-02-03 03:00:00.000000', 'manual', NULL, NULL, NULL, 'Jurnal manual berpecahan', 'posted', NULL, '2026-02-03 03:00:00.000000', 2, NULL, NULL, NULL, '2026-02-03 03:00:00.000000', '2026-02-03 03:00:00.000000', NULL),
(10, 1, 2, 'JE-CBG/2026/00001', '2026-02-11 03:00:00.000000', 'auto', 'payment', 5, 'customer_payment', 'Pembayaran cek', 'reversed', NULL, '2026-02-11 03:00:00.000000', 2, '2026-02-12 03:00:00.000000', 2, NULL, '2026-02-11 03:00:00.000000', '2026-02-11 03:00:00.000000', NULL),
(11, 1, 2, 'JE-CBG/2026/00002', '2026-02-12 03:00:00.000000', 'reversal', NULL, NULL, NULL, 'Pembalik JE-CBG/2026/00001', 'posted', 10, '2026-02-12 03:00:00.000000', 2, NULL, NULL, NULL, '2026-02-12 03:00:00.000000', '2026-02-12 03:00:00.000000', NULL),
(12, 1, 1, 'JE-PST/2026/00006', '2026-02-14 03:00:00.000000', 'manual', NULL, NULL, NULL, 'Draf tidak dimigrasi', 'draft', NULL, '2026-02-14 03:00:00.000000', 2, NULL, NULL, NULL, '2026-02-14 03:00:00.000000', '2026-02-14 03:00:00.000000', NULL),
(13, 1, NULL, 'JE-PST/2026/00007', '2026-02-15 03:00:00.000000', 'manual', NULL, NULL, NULL, 'Beban pusat', 'posted', NULL, '2026-02-15 03:00:00.000000', 2, NULL, NULL, NULL, '2026-02-15 03:00:00.000000', '2026-02-15 03:00:00.000000', NULL),
(14, 1, 1, 'JE-PST/2025/00004', '2025-12-20 03:00:00.000000', 'auto', 'payment', 4, 'supplier_payment', 'Pembayaran PAY-PST/2025/00004', 'posted', NULL, '2025-12-20 03:00:00.000000', 2, NULL, NULL, NULL, '2025-12-20 03:00:00.000000', '2025-12-20 03:00:00.000000', NULL);
INSERT INTO legacy_finance_journal_lines (id_finance_journal_line, id_finance_journal_entry, @@CO@@, id_branch, id_finance_account, line_no, debit_amount, credit_amount, description, id_product, id_product_variant, id_business_party, id_order, id_payment, id_inventory_movement, metadata_json, created_at) VALUES
(1, 1, 1, 1, 4, 1, '1000000.00', '0.00', NULL, 1, 1, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(2, 1, 1, 1, 5, 2, '110000.00', '0.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(3, 1, 1, 1, 6, 3, '0.00', '1110000.00', NULL, NULL, NULL, 3, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(4, 2, 1, 1, 3, 1, '227550.00', '0.00', NULL, NULL, NULL, 1, 1, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(5, 2, 1, 1, 11, 2, '0.00', '205000.00', NULL, NULL, NULL, NULL, 1, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(6, 2, 1, 1, 7, 3, '0.00', '22550.00', NULL, NULL, NULL, NULL, 1, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(7, 3, 1, 1, 12, 1, '136000.00', '0.00', NULL, 1, 1, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(8, 3, 1, 1, 4, 2, '0.00', '136000.00', NULL, 1, 1, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(9, 4, 1, 1, 1, 1, '100000.00', '0.00', NULL, NULL, NULL, NULL, NULL, 1, NULL, NULL, '2025-11-01 00:00:00.000000'),
(10, 4, 1, 1, 3, 2, '0.00', '100000.00', NULL, NULL, NULL, 1, NULL, 1, NULL, NULL, '2025-11-01 00:00:00.000000'),
(11, 5, 1, NULL, 1, 1, '500000.00', '0.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(12, 5, 1, NULL, 10, 2, '0.00', '500000.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(13, 6, 1, 1, 2, 1, '55500.00', '0.00', NULL, NULL, NULL, NULL, NULL, 2, NULL, NULL, '2025-11-01 00:00:00.000000'),
(14, 6, 1, 1, 3, 2, '0.00', '55500.00', NULL, NULL, NULL, 2, NULL, 2, NULL, NULL, '2025-11-01 00:00:00.000000'),
(15, 7, 1, 1, 17, 1, '25000.00', '0.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(16, 7, 1, 1, 1, 2, '0.00', '25000.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(17, 8, 1, 1, 1, 1, '30001.00', '0.00', NULL, NULL, NULL, NULL, 3, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(18, 8, 1, 1, 11, 2, '0.00', '30001.00', NULL, NULL, NULL, NULL, 3, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(19, 9, 1, 1, 1, 1, '100.50', '0.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(20, 9, 1, 1, 11, 2, '0.00', '50.25', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(21, 9, 1, 1, 7, 3, '0.00', '50.25', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(22, 10, 1, 2, 2, 1, '60000.00', '0.00', NULL, NULL, NULL, NULL, NULL, 5, NULL, NULL, '2025-11-01 00:00:00.000000'),
(23, 10, 1, 2, 3, 2, '0.00', '60000.00', NULL, NULL, NULL, NULL, NULL, 5, NULL, NULL, '2025-11-01 00:00:00.000000'),
(24, 11, 1, 2, 3, 1, '60000.00', '0.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(25, 11, 1, 2, 2, 2, '0.00', '60000.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(26, 12, 1, 1, 16, 1, '1000.00', '0.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(27, 12, 1, 1, 1, 2, '0.00', '1000.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(28, 13, 1, NULL, 16, 1, '10000.00', '0.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(29, 13, 1, NULL, 1, 2, '0.00', '10000.00', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000'),
(30, 14, 1, 1, 6, 1, '500000.00', '0.00', NULL, NULL, NULL, 3, NULL, 4, NULL, NULL, '2025-11-01 00:00:00.000000'),
(31, 14, 1, 1, 2, 2, '0.00', '500000.00', NULL, NULL, NULL, NULL, NULL, 4, NULL, NULL, '2025-11-01 00:00:00.000000');
INSERT INTO legacy_finance_posting_sources (id_finance_posting_source, @@CO@@, id_branch, source_type, id_source, event_key, source_number, source_date, source_amount, posting_status, id_journal_entry, error_message, pending_reason_code, source_hash, metadata_json, created_at, updated_at, posted_at, posted_by) VALUES
(1, 1, 1, 'goods_receipt', 1, 'purchase_received', 'GR1', '2025-12-06 03:00:00.000000', '1110000.00', 'posted', 1, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(2, 1, 1, 'goods_receipt', 2, 'purchase_received', 'GR2', '2026-01-21 03:00:00.000000', '444000.00', 'pending', NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(3, 1, 1, 'order', 1, 'sales_completed', 'O1', '2025-12-10 03:00:00.000000', '227550.00', 'posted', 2, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(4, 1, 1, 'inventory_movement', 6, 'stock_issue_cogs', 'M', '2025-12-11 04:00:00.000000', '100000.00', 'posted', 3, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(5, 1, 1, 'inventory_movement', 7, 'stock_issue_cogs', 'M', '2025-12-11 04:00:00.000000', '40000.00', 'posted', 3, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(6, 1, 1, 'inventory_movement', 8, 'stock_issue_cogs', 'M', '2025-12-11 04:00:00.000000', '36000.00', 'posted', 3, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(7, 1, 1, 'order', 2, 'sales_completed', 'O2', '2026-01-15 03:00:00.000000', '55500.00', 'pending', NULL, NULL, 'inclusive_tax', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(8, 1, 1, 'order', 3, 'sales_completed', 'O3', '2026-02-02 03:00:00.000000', '30001.00', 'posted', 8, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(9, 1, 2, 'order', 6, 'sales_completed', 'O6', '2026-02-10 03:00:00.000000', '60000.00', 'pending', NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(10, 1, 1, 'payment', 1, 'customer_payment', 'P1', '2025-12-31 17:30:00.000000', '100000.00', 'posted', 4, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(11, 1, 1, 'payment', 2, 'customer_payment', 'P2', '2026-01-16 03:00:00.000000', '55500.00', 'posted', 6, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(12, 1, 1, 'payment', 3, 'customer_payment', 'P3', '2026-02-02 03:30:00.000000', '30001.00', 'pending', NULL, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(13, 1, 1, 'payment', 4, 'supplier_payment', 'P4', '2025-12-20 03:00:00.000000', '500000.00', 'posted', 14, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL),
(14, 1, 2, 'payment', 5, 'customer_payment', 'P5', '2026-02-11 03:00:00.000000', '60000.00', 'posted', 10, NULL, NULL, NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL, NULL);
INSERT INTO legacy_finance_inventory_cost_movements (id_finance_inventory_cost_movement, @@CO@@, id_branch, id_product, id_product_variant, id_inventory_movement, movement_type, source_type, id_source, movement_date, quantity_in_base_uom, unit_cost_before, unit_cost_after, total_cost_amount, inventory_value_before, inventory_value_after, metadata_json, created_at) VALUES
(1, 1, 1, 2, 2, 1, 'opening', 'opening_stock', 0, '2025-11-30 02:00:00.000000', '20', '0.000000', '12000.000000', '240000.00', '0', '240000.00', NULL, '2025-11-30 02:00:00.000000'),
(2, 1, 1, 2, 3, 2, 'opening', 'opening_stock', 0, '2025-11-30 02:00:00.000000', '5', '0.000000', '12000.000000', '60000.00', '0', '60000.00', NULL, '2025-11-30 02:00:00.000000'),
(3, 1, 1, 3, 4, 3, 'opening', 'opening_stock', 0, '2025-11-30 02:00:00.000000', '10', '0.000000', '8000.000000', '80000.00', '0', '80000.00', NULL, '2025-11-30 02:00:00.000000'),
(4, 1, 1, 1, 1, 4, 'opening', 'opening_stock', 0, '2025-11-30 02:00:00.000000', '10', '0.000000', '10000.000000', '100000.00', '0', '100000.00', NULL, '2025-11-30 02:00:00.000000'),
(5, 1, 1, 1, 1, 5, 'in', 'goods_receipt', 1, '2025-12-06 03:00:00.000000', '100', '10000.000000', '10000.000000', '1000000.00', '100000.00', '1100000.00', NULL, '2025-12-06 03:00:00.000000'),
(6, 1, 1, 1, 1, 6, 'out', 'delivery_note', 1, '2025-12-11 04:00:00.000000', '6', '10000.000000', '10000.000000', '60000.00', '1100000.00', '1040000.00', NULL, '2025-12-11 04:00:00.000000'),
(7, 1, 1, 1, 1, 7, 'out', 'delivery_note', 1, '2025-12-11 04:00:00.000000', '4', '10000.000000', '10000.000000', '40000.00', '1040000.00', '1000000.00', NULL, '2025-12-11 04:00:00.000000'),
(8, 1, 1, 2, 2, 8, 'out', 'delivery_note', 1, '2025-12-11 04:00:00.000000', '3', '12000.000000', '12000.000000', '36000.00', '240000.00', '204000.00', NULL, '2025-12-11 04:00:00.000000'),
(9, 1, 1, 3, 4, 14, 'in', 'goods_receipt', 2, '2026-01-21 03:00:00.000000', '50', '8000.000000', '8733.333333', '444000.00', '80000.00', '524000.00', NULL, '2026-01-21 03:00:00.000000');
INSERT INTO legacy_business_expenses (id_business_expense, @@CO@@, id_branch, expense_number, expense_date, id_finance_account_expense, id_finance_account_payment, id_business_party, description, amount, status, id_finance_journal_entry, id_reversal_journal_entry, cancelled_at, cancelled_by, cancel_reason, created_by, created_at, updated_at) VALUES
(1, 1, 1, 'EXP-PST/2026/00001', '2026-01-20', 17, 1, NULL, 'Parkir Januari', '25000.00', 'posted', 7, NULL, NULL, NULL, NULL, 2, '2026-01-20 03:00:00.000000', '2026-01-20 03:00:00.000000');
INSERT INTO legacy_finance_periods (id_finance_period, @@CO@@, period_code, date_from, date_to, status, closed_at, closed_by, created_at, updated_at) VALUES
(1, 1, '2025-12', '2025-12-01', '2025-12-31', 'closed', '2026-01-03 03:00:00.000000', 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(2, 1, '2026-01', '2026-01-01', '2026-01-31', 'closed', '2026-02-03 03:00:00.000000', 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000'),
(3, 1, '2026-02', '2026-02-01', '2026-02-28', 'open', NULL, NULL, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000');
INSERT INTO legacy_media_files (id_media_file, @@CO@@, owner_type, owner_id, purpose, storage_driver, bucket, object_key, original_name, mime_type, size_bytes, visibility, checksum, is_primary, sort_order, metadata_json, id_created_by, created_at, updated_at, archived_at) VALUES
(1, 1, 'product', 1, 'product_image', 'local', NULL, 'products/p1.jpg', 'p1.jpg', 'image/jpeg', 14, 'private', NULL, 1, 0, NULL, 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(2, 1, 'product', 2, 'product_image', 'local', NULL, 'products/p2.png', 'p2.png', 'image/png', 14, 'private', NULL, 1, 0, NULL, 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(3, 1, 'delivery_note', 1, 'delivery_signed_document', 'local', NULL, 'delivery/2025-12-11/sj1.pdf', 'sj1.pdf', 'application/pdf', 13, 'private', NULL, 1, 0, NULL, 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(4, 1, 'delivery_note', 2, 'delivery_location_photo', 'local', NULL, 'delivery/2026-01-16/missing.jpg', 'hilang.jpg', 'image/jpeg', 10, 'private', NULL, 1, 0, NULL, 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL),
(5, 1, 'knowledge', 1, 'knowledge_source', 'local', NULL, 'knowledge/doc1.pdf', 'doc1.pdf', 'application/pdf', 10, 'private', NULL, 1, 0, NULL, 2, '2025-11-01 00:00:00.000000', '2025-11-01 00:00:00.000000', NULL);
INSERT INTO legacy_branch_document_sequences (id_branch_document_sequence, id_branch, sequence_key, prefix, current_value, reset_policy, format_template, updated_at) VALUES
(1, 1, 'order_sales_2026-03', NULL, 1, 'monthly', NULL, '2025-11-01 00:00:00.000000'),
(2, 1, 'order_sales_2026-02', NULL, 2, 'monthly', NULL, '2025-11-01 00:00:00.000000'),
(3, 1, 'order_purchase_2026-03', NULL, 1, 'monthly', NULL, '2025-11-01 00:00:00.000000'),
(4, 1, 'payment', NULL, 6, 'monthly', NULL, '2025-11-01 00:00:00.000000'),
(5, 1, 'sj', NULL, 3, 'monthly', NULL, '2025-11-01 00:00:00.000000'),
(6, 1, 'stock_transfer', NULL, 1, 'monthly', NULL, '2025-11-01 00:00:00.000000'),
(7, 2, 'payment', NULL, 1, 'monthly', NULL, '2025-11-01 00:00:00.000000'),
(8, 2, 'sj', NULL, 1, 'monthly', NULL, '2025-11-01 00:00:00.000000'),
(9, 2, 'order_sales_2026-02', NULL, 1, 'monthly', NULL, '2025-11-01 00:00:00.000000');
INSERT INTO legacy_finance_document_sequences (id_finance_document_sequence, @@CO@@, sequence_key, prefix, current_value) VALUES
(1, 1, 'journal', 'JE', 14);
INSERT INTO legacy_audit_logs (id_audit_log, @@CO@@, id_branch, actor_type, id_actor, action_key, entity_type, id_entity, before_json, after_json, metadata_json, happened_at) VALUES
(1, 1, 1, 'user', 2, 'order.create', 'order', 1, NULL, '{"x":1}', NULL, '2026-02-01 03:00:00.000000');

-- ======================================================================
-- Returns (sales + purchase), replacement delivery, company settings and
-- extra audit rows. Appended on top of the base fixture; balances/sequences
-- of the base rows are adjusted by the UPDATEs below.
-- ======================================================================
INSERT INTO legacy_sales_returns (id_sales_return, @@CO@@, id_branch, return_number, id_original_order, id_related_party, source, status, return_mode, return_date, reason_text,
  returned_subtotal_before_tax, returned_tax_amount, returned_total_amount, replacement_subtotal_before_tax, replacement_tax_amount, replacement_total_amount, difference_amount,
  settlement_type, replacement_delivery_status, id_created_by, created_at, updated_at, cancelled_at, cancelled_by, cancel_reason, archived_at) VALUES
(1, 1, 1, 'RTR-PST/2025/00001', 1, 1, 'manual', 'completed', 'return_only', '2025-12-20 04:00:00.000000', 'Barang tidak sesuai pesanan', '14500.00', '1595.00', '16095.00', '0.00', '0.00', '0.00', '-16095.00', 'reduce_receivable', 'not_required', 2, '2025-12-20 04:00:00.000000', '2025-12-20 04:00:00.000000', NULL, NULL, NULL, NULL),
(2, 1, 1, 'RTR-PST/2026/00002', 2, 2, 'manual', 'completed', 'exchange', '2026-01-25 04:00:00.000000', 'Tukar barang lain', '22200.00', '2200.00', '22200.00', '13514.00', '1486.00', '15000.00', '-7200.00', 'customer_credit', 'delivered', 2, '2026-01-25 04:00:00.000000', '2026-01-28 06:00:00.000000', NULL, NULL, NULL, NULL),
(3, 1, 2, 'RTR-CBG/2026/00001', 6, 2, 'manual', 'completed', 'return_only', '2026-02-15 04:00:00.000000', 'Salah kirim', '15000.00', '0.00', '15000.00', '0.00', '0.00', '0.00', '-15000.00', 'customer_credit', 'not_required', 2, '2026-02-15 04:00:00.000000', '2026-02-15 04:00:00.000000', NULL, NULL, NULL, NULL),
(4, 1, 1, 'RTR-PST/2026/00003', 1, 1, 'manual', 'completed', 'return_only', '2026-02-20 04:00:00.000000', 'Cacat produksi', '49000.00', '5390.00', '54390.00', '0.00', '0.00', '0.00', '-54390.00', 'reduce_receivable', 'not_required', 2, '2026-02-20 04:00:00.000000', '2026-02-20 04:00:00.000000', NULL, NULL, NULL, NULL),
(5, 1, 1, 'RTR-PST/2026/00004', 3, NULL, 'pos', 'completed', 'exchange', '2026-02-25 04:00:00.000000', 'Tukar di kasir', '15001.00', '0.00', '15001.00', '11100.00', '0.00', '11100.00', '-3901.00', 'refund', 'not_required', 4, '2026-02-25 04:00:00.000000', '2026-02-25 04:00:00.000000', NULL, NULL, NULL, NULL),
(6, 1, 1, 'RTR-PST/2026/00005', 2, 2, 'manual', 'cancelled', 'return_only', '2026-03-02 04:00:00.000000', 'Salah input', '11100.00', '1100.00', '11100.00', '0.00', '0.00', '0.00', '-11100.00', 'customer_credit', 'not_required', 2, '2026-03-02 04:00:00.000000', '2026-03-02 05:00:00.000000', '2026-03-02 05:00:00.000000', 2, 'dibatalkan pelanggan', NULL),
(7, 1, 1, 'RTR-PST/2026/00006', 1, 1, 'manual', 'completed', 'return_only', '2026-03-05 04:00:00.000000', 'Kelebihan kirim', '72500.00', '7975.00', '80475.00', '0.00', '0.00', '0.00', '-80475.00', 'refund', 'not_required', 2, '2026-03-05 04:00:00.000000', '2026-03-05 04:00:00.000000', NULL, NULL, NULL, NULL),
(8, 1, 1, 'RTR-PST/2026/00007', 1, 1, 'manual', 'completed', 'exchange', '2026-03-12 04:00:00.000000', 'Tukar ukuran', '20000.00', '2200.00', '22200.00', '20000.00', '2200.00', '22200.00', '0.00', 'none', 'pending', 2, '2026-03-12 04:00:00.000000', '2026-03-12 04:00:00.000000', NULL, NULL, NULL, NULL);
INSERT INTO legacy_sales_return_items (id_sales_return_item, id_sales_return, line_type, id_original_order_item, id_product, id_product_variant, line_no, product_name_snapshot, product_code_snapshot,
  quantity, transaction_uom_snapshot, base_uom_snapshot, uom_to_base_factor, quantity_in_base_uom, unit_price_before_discount, unit_price, line_discount_amount, line_discount_percent,
  taxable, tax_rate_snapshot, tax_base_amount, tax_amount, line_total_before_tax, line_total_after_tax, line_total, stock_tracked_snapshot, return_condition, id_stock_location, id_inventory_movement, notes) VALUES
(1, 1, 'returned', 1, 1, 1, 1, 'Barang Satu', 'CODE-1', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '15000.00', '14500.00', '500.00', '0.0000', 1, '11.0000', '14500.00', '1595.00', '14500.00', '16095.00', '16095.00', 1, 'normal', 1, 22, NULL),
(2, 2, 'returned', 3, 3, 4, 1, 'Barang Tiga', 'CODE-3', '2.000000000000', 'pcs', 'pcs', '1.000000000000', '2.000000000000', '11100.00', '11100.00', '0.00', '0.0000', 1, '11.0000', '20000.00', '2200.00', '22200.00', '22200.00', '22200.00', 1, 'normal', 1, 26, NULL),
(3, 2, 'replacement', NULL, 1, 1, 1, 'Barang Satu', 'BRG-001', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '15000.00', '15000.00', '0.00', '0.0000', 1, '11.0000', '13514.00', '1486.00', '13514.00', '15000.00', '15000.00', 1, NULL, 1, 27, NULL),
(4, 3, 'returned', 7, 1, 1, 1, 'Barang Satu', 'CODE-1', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '15000.00', '15000.00', '0.00', '0.0000', 0, '0.0000', '15000.00', '0.00', '15000.00', '15000.00', '15000.00', 1, 'normal', 5, 31, NULL),
(5, 4, 'returned', 1, 1, 1, 1, 'Barang Satu', 'CODE-1', '2.000000000000', 'pcs', 'pcs', '1.000000000000', '2.000000000000', '15000.00', '14500.00', '500.00', '0.0000', 1, '11.0000', '29000.00', '3190.00', '29000.00', '32190.00', '32190.00', 1, 'normal', 1, 23, NULL),
(6, 4, 'returned', 2, 2, 2, 2, 'Barang Dua', 'CODE-2', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '20000.00', '20000.00', '0.00', '0.0000', 1, '11.0000', '20000.00', '2200.00', '20000.00', '22200.00', '22200.00', 1, 'damaged', 3, 24, 'pecah'),
(7, 5, 'returned', 4, 1, 1, 1, 'Barang Satu', 'CODE-1', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '15000.50', '15000.50', '0.00', '0.0000', 0, '0.0000', '15001.00', '0.00', '15001.00', '15001.00', '15001.00', 1, 'normal', 1, 29, NULL),
(8, 5, 'replacement', NULL, 3, 4, 1, 'Barang Tiga', 'BRG-003', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '11100.00', '11100.00', '0.00', '0.0000', 0, '0.0000', '11100.00', '0.00', '11100.00', '11100.00', '11100.00', 1, NULL, 1, 30, NULL),
(9, 6, 'returned', 3, 3, 4, 1, 'Barang Tiga', 'CODE-3', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '11100.00', '11100.00', '0.00', '0.0000', 1, '11.0000', '10000.00', '1100.00', '11100.00', '11100.00', '11100.00', 1, 'normal', 1, NULL, NULL),
(10, 7, 'returned', 1, 1, 1, 1, 'Barang Satu', 'CODE-1', '5.000000000000', 'pcs', 'pcs', '1.000000000000', '5.000000000000', '15000.00', '14500.00', '500.00', '0.0000', 1, '11.0000', '72500.00', '7975.00', '72500.00', '80475.00', '80475.00', 1, 'normal', 2, 25, NULL),
(11, 8, 'returned', 2, 2, 2, 1, 'Barang Dua', 'CODE-2', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '20000.00', '20000.00', '0.00', '0.0000', 1, '11.0000', '20000.00', '2200.00', '20000.00', '22200.00', '22200.00', 1, 'normal', 1, 28, NULL),
(12, 8, 'replacement', NULL, 2, 3, 1, 'Barang Dua', 'BRG-002', '1.000000000000', 'pcs', 'pcs', '1.000000000000', '1.000000000000', '20000.00', '20000.00', '0.00', '0.0000', 1, '11.0000', '20000.00', '2200.00', '20000.00', '22200.00', '22200.00', 1, NULL, NULL, NULL, NULL);
INSERT INTO legacy_sales_return_settlements (id_sales_return_settlement, id_sales_return, settlement_type, settlement_date, amount, payment_method, reference_number, notes, id_created_by, created_at, archived_at) VALUES
(1, 2, 'customer_credit', '2026-01-25 04:00:00.000000', '7200.00', NULL, NULL, NULL, 2, '2026-01-25 04:00:00.000000', NULL),
(2, 3, 'customer_credit', '2026-02-15 04:00:00.000000', '15000.00', NULL, NULL, 'jadi saldo', 2, '2026-02-15 04:00:00.000000', NULL),
(3, 5, 'refund', '2026-02-25 04:00:00.000000', '3901.00', 'cash', NULL, NULL, 4, '2026-02-25 04:00:00.000000', NULL),
(4, 7, 'refund', '2026-03-05 04:00:00.000000', '23410.00', 'cash', 'KW-77', 'refund kelebihan', 2, '2026-03-05 04:00:00.000000', NULL),
(5, 6, 'customer_credit', '2026-03-02 04:00:00.000000', '11100.00', NULL, NULL, NULL, 2, '2026-03-02 04:00:00.000000', '2026-03-02 05:00:00.000000');
INSERT INTO legacy_purchase_returns (id_purchase_return, @@CO@@, id_branch, return_number, id_original_order, id_related_party, status, return_date, reason_text, returned_subtotal_before_tax,
  returned_tax_amount, returned_total_amount, settlement_type, cancels_order, id_created_by, created_at, updated_at, archived_at) VALUES
(1, 1, 1, 'RTB-PST/2026/00001', 7, 3, 'completed', '2026-02-18 04:00:00.000000', 'Barang cacat dari pemasok', '100000.00', '11000.00', '111000.00', 'reduce_payable', 0, 2, '2026-02-18 04:00:00.000000', '2026-02-18 04:00:00.000000', NULL),
(2, 1, 1, 'RTB-PST/2026/00002', 7, 3, 'completed', '2026-03-10 04:00:00.000000', 'Retur besar', '600000.00', '66000.00', '666000.00', 'refund', 0, 2, '2026-03-10 04:00:00.000000', '2026-03-10 04:00:00.000000', NULL),
(3, 1, 1, 'RTB-PST/2026/00003', 8, 4, 'cancelled', '2026-03-11 04:00:00.000000', 'Salah catat', '88800.00', '8800.00', '88800.00', 'reduce_payable', 0, 2, '2026-03-11 04:00:00.000000', '2026-03-11 05:00:00.000000', NULL);
INSERT INTO legacy_purchase_return_items (id_purchase_return_item, id_purchase_return, id_original_order_item, id_product, id_product_variant, line_no, product_name_snapshot, product_code_snapshot,
  quantity, transaction_uom_snapshot, base_uom_snapshot, uom_to_base_factor, quantity_in_base_uom, unit_price, tax_rate_snapshot, tax_base_amount, tax_amount, line_total_before_tax,
  line_total, stock_tracked_snapshot, id_stock_location, id_inventory_movement, notes) VALUES
(1, 1, 8, 1, 1, 1, 'Barang Satu', 'CODE-1', '10.000000000000', 'pcs', 'pcs', '1.000000000000', '10.000000000000', '10000.00', '11.0000', '100000.00', '11000.00', '100000.00', '111000.00', 1, 1, 32, NULL),
(2, 2, 8, 1, 1, 1, 'Barang Satu', 'CODE-1', '60.000000000000', 'pcs', 'pcs', '1.000000000000', '60.000000000000', '10000.00', '11.0000', '600000.00', '66000.00', '600000.00', '666000.00', 1, 1, 33, NULL),
(3, 3, 9, 3, 4, 1, 'Barang Tiga', 'CODE-3', '10.000000000000', 'pcs', 'pcs', '1.000000000000', '10.000000000000', '8880.00', '11.0000', '80000.00', '8800.00', '88800.00', '88800.00', 1, NULL, NULL, NULL);
INSERT INTO legacy_purchase_return_settlements (id_purchase_return_settlement, id_purchase_return, settlement_type, settlement_date, amount, payment_method, reference_number, notes, id_created_by, created_at, archived_at) VALUES
(1, 2, 'refund', '2026-03-10 04:00:00.000000', '167000.00', 'bank_transfer', 'TRX-RB-1', NULL, 2, '2026-03-10 04:00:00.000000', NULL);
INSERT INTO legacy_delivery_notes (id_delivery_note, id_order, document_kind, id_sales_return, id_branch, sj_number, driver_name, vehicle_plate, warehouse_staff_name, delivery_date, notes, dispatched_at, dispatched_by,
  confirmed_at, confirmed_by, recipient_signature_status, recipient_name, recipient_signature_missing_reason, drop_location_note, archived_at, created_at) VALUES
(5, NULL, 'replacement', 2, 1, 'SJ-PST/2026/00003', 'Pak Sopir', 'B 1234 XY', 'Gudang', '2026-01-28', 'pengganti', '2026-01-28 04:00:00.000000', 2, '2026-01-28 06:00:00.000000', 2, 'signed', 'Siti', NULL, NULL, NULL, '2026-01-28 04:00:00.000000');
INSERT INTO legacy_delivery_note_items (id_delivery_note_item, id_delivery_note, id_order_item, id_sales_return_item, id_product_variant, quantity, quantity_in_base_uom) VALUES
(6, 5, NULL, 3, 1, '1.000000000000', '1.000000000000');
INSERT INTO legacy_inventory_movements (id_inventory_movement, id_branch, id_product, id_product_variant, id_stock_location, movement_type, quantity, balance_before, balance_after, reference_type, id_reference, reason_text, id_moved_by, moved_at, metadata_json) VALUES
(22, 1, 1, 1, 1, 'in', '1.000000000000', '85.000000000000', '86.000000000000', 'sales_return', 1, 'Retur barang dari RTR-PST/2025/00001', 2, '2025-12-20 04:00:00.000000', '{"idSalesReturn":1}'),
(23, 1, 1, 1, 1, 'in', '2.000000000000', '86.000000000000', '88.000000000000', 'sales_return', 4, 'Retur barang dari RTR-PST/2026/00003', 2, '2026-02-20 04:00:00.000000', '{"idSalesReturn":4}'),
(24, 1, 2, 2, 3, 'in', '1.000000000000', '0E-12', '1.000000000000', 'sales_return', 4, 'Retur barang dari RTR-PST/2026/00003', 2, '2026-02-20 04:00:00.000000', '{"idSalesReturn":4,"returnCondition":"damaged"}'),
(25, 1, 1, 1, 2, 'in', '5.000000000000', '6.000000000000', '11.000000000000', 'sales_return', 7, 'Retur barang dari RTR-PST/2026/00006', 2, '2026-03-05 04:00:00.000000', '{"idSalesReturn":7}'),
(26, 1, 3, 4, 1, 'in', '2.000000000000', '50.000000000000', '52.000000000000', 'sales_return', 2, 'Retur barang dari RTR-PST/2026/00002', 2, '2026-01-25 04:00:00.000000', '{"idSalesReturn":2}'),
(27, 1, 1, 1, 1, 'out', '1.000000000000', '88.000000000000', '87.000000000000', 'sales_return_exchange', 2, 'Barang pengganti RTR-PST/2026/00002 via SJ-PST/2026/00003', 2, '2026-01-28 04:00:00.000000', '{"idSalesReturn":2,"idDeliveryNote":5,"lineType":"replacement"}'),
(28, 1, 2, 2, 1, 'in', '1.000000000000', '17.000000000000', '18.000000000000', 'sales_return', 8, 'Retur barang dari RTR-PST/2026/00007', 2, '2026-03-12 04:00:00.000000', '{"idSalesReturn":8}'),
(29, 1, 1, 1, 1, 'in', '1.000000000000', '87.000000000000', '88.000000000000', 'sales_return', 5, 'Retur barang dari RTR-PST/2026/00004', 4, '2026-02-25 04:00:00.000000', '{"idSalesReturn":5}'),
(30, 1, 3, 4, 1, 'out', '1.000000000000', '52.000000000000', '51.000000000000', 'sales_return_exchange', 5, 'Barang pengganti dari RTR-PST/2026/00004', 4, '2026-02-25 04:00:00.000000', '{"idSalesReturn":5,"lineType":"replacement"}'),
(31, 2, 1, 1, 5, 'in', '1.000000000000', '1.000000000000', '2.000000000000', 'sales_return', 3, 'Retur barang dari RTR-CBG/2026/00001', 2, '2026-02-15 04:00:00.000000', '{"idSalesReturn":3}'),
(32, 1, 1, 1, 1, 'out', '10.000000000000', '88.000000000000', '78.000000000000', 'purchase_return', 1, 'Retur ke pemasok RTB-PST/2026/00001', 2, '2026-02-18 04:00:00.000000', '{"idPurchaseReturn":1}'),
(33, 1, 1, 1, 1, 'out', '60.000000000000', '78.000000000000', '18.000000000000', 'purchase_return', 2, 'Retur ke pemasok RTB-PST/2026/00002', 2, '2026-03-10 04:00:00.000000', '{"idPurchaseReturn":2}');
UPDATE legacy_inventory_balances SET on_hand_qty = '18.000000000000', available_qty = '18.000000000000' WHERE id_inventory_balance = 1;
UPDATE legacy_inventory_balances SET on_hand_qty = '11.000000000000', available_qty = '11.000000000000' WHERE id_inventory_balance = 2;
UPDATE legacy_inventory_balances SET on_hand_qty = '18.000000000000', available_qty = '18.000000000000' WHERE id_inventory_balance = 5;
UPDATE legacy_inventory_balances SET on_hand_qty = '51.000000000000', available_qty = '51.000000000000' WHERE id_inventory_balance = 7;
UPDATE legacy_inventory_balances SET on_hand_qty = '2.000000000000', available_qty = '2.000000000000' WHERE id_inventory_balance = 9;
INSERT INTO legacy_inventory_balances (id_inventory_balance, id_branch, id_product, id_product_variant, id_stock_location, on_hand_qty, reserved_qty, available_qty, updated_at) VALUES
(11, 1, 2, 2, 3, '1.000000000000', '0.000000000000', '1.000000000000', '2026-02-20 04:00:00.000000');
INSERT INTO legacy_finance_posting_sources (id_finance_posting_source, @@CO@@, id_branch, source_type, id_source, event_key, source_number, source_date, source_amount, posting_status, id_journal_entry, error_message, pending_reason_code, source_hash, metadata_json, created_at, updated_at, posted_at, posted_by) VALUES
(15, 1, 1, 'sales_return', 2, 'sales_return_completed', 'RTR2', '2026-01-25 04:00:00.000000', '22200.00', 'posted', NULL, NULL, NULL, NULL, NULL, '2026-01-25 04:00:00.000000', '2026-01-25 04:00:00.000000', NULL, NULL),
(16, 1, 1, 'inventory_movement', 26, 'sales_return_stock_in', 'RTR2', '2026-01-25 04:00:00.000000', '0.00', 'posted', NULL, NULL, NULL, NULL, NULL, '2026-01-25 04:00:00.000000', '2026-01-25 04:00:00.000000', NULL, NULL),
(17, 1, 1, 'sales_return_settlement', 1, 'sales_return_customer_credit', 'RTR2', '2026-01-25 04:00:00.000000', '7200.00', 'posted', NULL, NULL, NULL, NULL, NULL, '2026-01-25 04:00:00.000000', '2026-01-25 04:00:00.000000', NULL, NULL),
(18, 1, 1, 'inventory_movement', 27, 'stock_issue_cogs', 'RTR2', '2026-01-28 04:00:00.000000', '0.00', 'posted', NULL, NULL, NULL, NULL, NULL, '2026-01-28 04:00:00.000000', '2026-01-28 04:00:00.000000', NULL, NULL),
(19, 1, 1, 'purchase_return', 1, 'purchase_return_completed', 'RTB1', '2026-02-18 04:00:00.000000', '111000.00', 'posted', NULL, NULL, NULL, NULL, NULL, '2026-02-18 04:00:00.000000', '2026-02-18 04:00:00.000000', NULL, NULL),
(20, 1, 1, 'inventory_movement', 32, 'purchase_return_stock_out', 'RTB1', '2026-02-18 04:00:00.000000', '0.00', 'posted', NULL, NULL, NULL, NULL, NULL, '2026-02-18 04:00:00.000000', '2026-02-18 04:00:00.000000', NULL, NULL);
INSERT INTO legacy_branch_document_sequences (id_branch_document_sequence, id_branch, sequence_key, prefix, current_value, reset_policy, format_template, updated_at) VALUES
(10, 1, 'sales_return', NULL, 7, 'never', NULL, '2026-03-12 04:00:00.000000'),
(11, 2, 'sales_return', NULL, 1, 'never', NULL, '2026-02-15 04:00:00.000000'),
(12, 1, 'purchase_return', NULL, 3, 'never', NULL, '2026-03-11 04:00:00.000000');
INSERT INTO legacy_company_settings (@@CO@@, business_labels_json, operational_rules_json, ui_preferences_json, assistant_preferences_json, reporting_preferences_json, updated_at) VALUES
(1, '{"order":"Pesanan"}', '{"stockAdjustmentApprovalMode":"strict","defaultOrderKind":"sales"}',
 '{"documentProfile":{"headerName":"PT Acme Sejahtera","addressLine":"Jl. Uji 1","phones":[{"label":"Toko","number":"021-123"}],"continuousNota":{"widthMm":241.3,"heightMm":139.7,"marginRightMm":37.91}},"theme":"light"}',
 '{"enabled":false}', '{"defaultRange":"month"}', '2026-01-01 00:00:00.000000'),
(2, NULL, '{"stockAdjustmentApprovalMode":"simple"}', NULL, NULL, NULL, '2026-01-01 00:00:00.000000');
INSERT INTO legacy_audit_logs (id_audit_log, @@CO@@, id_branch, actor_type, id_actor, action_key, entity_type, id_entity, before_json, after_json, metadata_json, happened_at) VALUES
(2, 1, 1, 'user', 2, 'sales_return.create', 'sales_return', 1, NULL, '{"returnNumber":"RTR-PST/2025/00001","itemCount":1}', NULL, '2025-12-20 04:00:00.000000'),
(3, 1, 1, 'user', 2, 'sales_return.create', 'sales_return', 4, NULL, '{"returnNumber":"RTR-PST/2026/00003","itemCount":2,"differenceAmount":-54390}', NULL, '2026-02-20 04:00:00.000000'),
(4, 1, 1, 'user', 2, 'payment.create', 'payment', 2, NULL, '{"paymentNumber":"PAY-PST/2026/00002"}', NULL, '2026-01-16 03:00:00.000000'),
(5, 1, NULL, 'user', 2, 'role.update', 'role', 3, '{"x":1}', '{"x":2}', NULL, '2026-02-18 03:00:00.000000'),
(6, 1, 1, 'system', NULL, 'whatsapp.send', 'whatsapp_conversation', 9, NULL, NULL, NULL, '2026-02-19 03:00:00.000000'),
(7, 1, 1, 'user', 2, 'order.create', 'order', 7, NULL, '{"orderNumber":"ORD-PST/PB/2025/12/00001"}', NULL, '2026-02-21 03:00:00.000000'),
(8, 1, 1, 'user', 1, 'product.update', 'product', 99999, NULL, '{"name":"hilang"}', NULL, '2026-02-22 03:00:00.000000'),
(9, 1, 2, 'user', 4, 'delivery.confirm', 'delivery_note', 5, NULL, '{"sjNumber":"SJ-PST/2026/00003"}', NULL, '2026-01-28 06:00:00.000000');
