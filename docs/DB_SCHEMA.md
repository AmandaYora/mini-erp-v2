# Database Schema — Mini ERP (Revamp)

Status: **kontrak kepemilikan tabel yang sudah terkunci** — 69 tabel nyata: 18 berkas baseline per
modul, `apps/api/migrations/` (`000001_user` … `000018_assistant`), ditambah migrasi bernomor
sesudah baseline (`000019`–`000023`, `000031`–`000037`, `000039`–`000041`; nomor 000024–000030 dan 000038 sengaja kosong) — turun dari 70
entity sistem lama. Daftar tabel di §3 dibangkitkan dari berkas itu (`node scripts/db-schema-doc.mjs`;
`--check` dijalankan CI/verify). Dokumen ini mengikat kepemilikan per modul; §6 mencatat disposisi
tiap tabel warisan; **riwayat migrasi bernomor (kolom/tabel baru sesudah baseline) ada di §8**.

## 1. Aturan Wajib

1. **Setiap tabel dimiliki tepat satu modul.** Modul lain mengakses datanya lewat `contracts/`
   modul pemilik (function call di dalam proses Go), **tidak pernah** lewat join SQL lintas modul.
2. **Relasi lintas modul = primitive ID**, bukan foreign key fisik. Contoh: `sales_orders.party_id`
   menunjuk baris di tabel `parties` (modul lain) sebagai integer/UUID biasa — **tanpa**
   `FOREIGN KEY` constraint lintas modul di MySQL. FK **boleh** dipakai di dalam modul yang sama
   (mis. `sales_order_items.sales_order_id → sales_orders.id`).
3. **Base kolom seragam** di setiap tabel (lihat §2) — sekali distandarkan, bukan diulang manual
   per tabel seperti sistem lama (yang tidak punya base entity sama sekali di 70 tabelnya).
4. **Uang disimpan sebagai integer rupiah** (bukan desimal) — lihat
   [ADR-0005](../knowledge/decisions/ADR-0005-money-as-integer-rupiah.md). Kuantitas memakai
   `DECIMAL` dengan presisi yang cukup untuk faktor konversi UOM (ditetapkan saat modul `product`
   didesain).
5. **Timestamp `DATETIME(6)` UTC di kolom**, dikonversi ke `Asia/Jakarta` di lapisan aplikasi saat
   ditampilkan/difilter — bukan disimpan sudah dalam waktu lokal seperti sebagian sistem lama.
6. Migrasi lewat `golang-migrate`, **bernomor & terlacak** (`apps/api/migrations/NNNNNN_<modul>[_<perubahan>].up.sql` /
   `.down.sql`, satu berkas = satu modul) — ada tabel versi migrasi bawaan, tidak seperti runner sistem
   lama yang menjalankan ulang seluruh migrasi tiap start. Nama berkas = pemilik tabel (arch-check Rule T/F);
   cara menambah migrasi: [BACKEND_GUIDE](../knowledge/BACKEND_GUIDE.md).
7. **Tanpa scoping perusahaan** (lihat ADR-0009): tidak ada kolom `company_id` di tabel mana pun,
   tidak ada unique constraint ber-scope perusahaan — keunikan bersifat global, atau per-cabang
   untuk entity yang terikat cabang (ditetapkan per modul).

## 2. Base Kolom (setiap tabel, kecuali tabel pivot murni)

| Kolom | Tipe | Catatan |
|---|---|---|
| `id` | `BIGINT UNSIGNED AUTO_INCREMENT` atau `CHAR(26)` (ULID) | Ditetapkan satu kali di ADR saat modul `shared` dibuat; jangan campur strategi antar modul |
| `branch_id` | `BIGINT UNSIGNED NULL` | Ada di tabel yang datanya terikat cabang (transaksi, stok); NULL untuk data lintas-cabang (mis. produk) |
| `created_at`, `updated_at` | `DATETIME(6)` | UTC, auto-set |
| `created_by`, `updated_by` | `BIGINT UNSIGNED NULL` | Primitive ID user, resolve nama lewat `UserClient` bila perlu ditampilkan |
| `deleted_at` | `DATETIME(6) NULL` | Untuk modul yang punya konsep arsip/soft-delete — **bukan default di semua tabel**, tetapkan sadar per tabel |

## 3. Kepemilikan Tabel per Modul

Disarikan dari ~70 entity sistem lama (
[legacy-reference/00-overview.md](legacy-reference/00-overview.md) §1), dipetakan ke 20 modul
baru. Nama tabel **dikunci** dan sumber kebenarannya adalah berkas migrasi per modul — daftar
otomatis berikut dibangkitkan darinya; subbagian per modul sesudahnya adalah narasi tujuan tabel.

<!-- BEGIN GENERATED: tables (scripts/db-schema-doc.mjs) -->
Daftar di bawah DIBANGKITKAN dari berkas baseline (`node scripts/db-schema-doc.mjs`); jangan diedit manual.
Kepemilikan tabel = nama berkas migrasi (`NNNNNN_<modul>.up.sql`) dan ditegakkan arch-check Rule T/F.
Modul `reporting` tidak memiliki tabel; `dashboard` hanya memiliki tabel penundaan alert (ADR-0025).

| Modul | Berkas migrasi | Tabel | Jumlah |
|---|---|---|---|
| `user` | `000001_user.up.sql` | `authz_seed_applied`, `permissions`, `roles`, `role_permissions`, `users`, `user_branch_access`, `user_roles` | 7 |
| `auth` | `000002_auth.up.sql` | `user_sessions` | 1 |
| `branch` | `000003_branch.up.sql` | `branches`, `branch_document_sequences` | 2 |
| `company` | `000004_company.up.sql` | `company_profile`, `company_settings` | 2 |
| `media` | `000005_media.up.sql` | `media_files` | 1 |
| `product` | `000006_product.up.sql` | `product_categories`, `products`, `product_variants` | 3 |
| `party` | `000007_party.up.sql` | `member_types`, `parties`, `party_delivery_addresses` | 3 |
| `stock` | `000008_stock.up.sql`, `000021_stock_parity.up.sql`, `000041_stock_corrections.up.sql` | `stock_locations`, `stock_balances`, `stock_movements`, `stock_reservations`, `stock_transfers`, `stock_transfer_items` | 6 |
| `purchasing` | `000009_purchasing.up.sql`, `000022_purchasing_tax_invoice.up.sql`, `000034_purchasing_corrections.up.sql` | `purchase_orders`, `purchase_order_items`, `purchase_order_corrections`, `purchase_order_correction_lines` | 4 |
| `sales` | `000010_sales.up.sql`, `000019_sales.up.sql`, `000033_sales_corrections.up.sql`, `000043_sales_origin.up.sql`, `000047_sales_receipt_account.up.sql` | `sales_orders`, `sales_order_items` | 2 |
| `goodsreceipt` | `000011_goodsreceipt.up.sql`, `000039_goodsreceipt_corrections.up.sql` | `goods_receipts`, `goods_receipt_items` | 2 |
| `delivery` | `000012_delivery.up.sql`, `000037_delivery_corrections.up.sql`, `000042_delivery_direct.up.sql` | `delivery_notes`, `delivery_note_items` | 2 |
| `payment` | `000013_payment.up.sql`, `000023_payment_credit_source.up.sql`, `000032_payment_corrections.up.sql`, `000048_payment_cash_account.up.sql` | `payments`, `payment_allocations` | 2 |
| `salesreturn` | `000014_salesreturn.up.sql`, `000036_salesreturn_corrections.up.sql` | `sales_returns`, `sales_return_items`, `sales_return_replacement_items`, `sales_return_settlements` | 4 |
| `purchasereturn` | `000015_purchasereturn.up.sql`, `000040_purchasereturn_corrections.up.sql` | `purchase_returns`, `purchase_return_items`, `purchase_return_settlements` | 3 |
| `finance` | `000016_finance.up.sql`, `000031_finance_fn.up.sql`, `000035_finance_corrections.up.sql`, `000044_finance_opening_correction.up.sql`, `000046_finance_bank_accounts.up.sql` | `finance_accounts`, `business_expenses`, `finance_account_daily`, `finance_account_mappings`, `finance_cutover`, `finance_document_sequences`, `finance_inventory_cost_movements`, `finance_journal_entries`, `finance_journal_lines`, `finance_periods`, `finance_tax_periods`, `finance_tax_report_snapshots`, `finance_posting_ignores`, `finance_tax_adjustment_meta`, `finance_inventory_revaluations`, `finance_inventory_revaluation_lines`, `finance_bank_accounts` | 17 |
| `audit` | `000017_audit.up.sql` | `audit_logs`, `audit_log_details` | 2 |
| `assistant` | `000018_assistant.up.sql`, `000020_assistant_tool_status.up.sql` | `assistant_authorizations`, `assistant_channel`, `assistant_config`, `assistant_messages`, `assistant_runs`, `assistant_threads`, `assistant_tool_executions` | 7 |
| `dashboard` | `000045_dashboard_alert_snooze.up.sql` | `dashboard_alert_snoozes` | 1 |
| **Total** | 19 berkas modul | | **71** |
<!-- END GENERATED: tables -->

### `audit` (L0)

| Tabel | Untuk apa |
|---|---|
| `audit_logs` | `action_key` (`resource.action`), `entity_type`+`entity_id`, `branch_id` (NULL = event global), `actor_id`, `note` — ditulis modul penulis lewat `AuditClient` (jalur uang/stok: `LogTx` di DALAM transaksi yang sama, ADR-0011; jalur lain: `Log` best-effort setelah komit). Dikunci baseline `000017_audit` |
| `audit_log_details` | Satu baris per `audit_logs` (FK dalam modul, `ON DELETE CASCADE`): `before_json`, `after_json`, `metadata_json` — diff sebelum/sesudah dan konteks yang tak muat di `note` |

### `assistant` (L9)

Bot WA + tooling, tanpa RAG/AI. Sesi WA di file SQLite (`whatsapp_session.db`,
cryptostore whatsmeow — bukan MySQL). Tulis bot dikecualikan dari `audit_logs`;
`assistant_runs` + `assistant_tool_executions` ADALAH jejaknya.

| Tabel | Untuk apa |
|---|---|
| `assistant_channel` | Singleton status gateway (state, phone, last_error) |
| `assistant_config` | Singleton tuning (mode terkunci `rule_based`, rate_limit_per_minute) |
| `assistant_authorizations` | Whitelist nomor (phone unik, level owner/authorized_party, status, primary tunggal) |
| `assistant_threads` | Thread per nomor pengirim (branch opsional) |
| `assistant_messages` | Pesan in/out per thread (status received/processed/failed, run_id) |
| `assistant_runs` | Tiap eksekusi (intent, mode, status, answer, duration_ms, failure_reason) |
| `assistant_tool_executions` | Tiap tool per run (seq, input/output JSON, `status` success/failed, `error_text`; kolom status ditambah migrasi `000020_assistant_tool_status`, AS-02) |

### `media` (L0)

| Tabel | Untuk apa |
|---|---|
| `media_files` | Metadata file (driver, path/key, owner_type + owner_id primitif, mime, ukuran) — dipakai `product` (foto), `payment` (bukti bayar), `delivery` (bukti kirim) lewat `MediaClient` |

### `auth` (L1)

| Tabel | Untuk apa |
|---|---|
| `user_sessions` | Sesi aktif, hash refresh token (dirotasi tiap pakai), branch/role aktif |

### `user` (L1)

| Tabel | Untuk apa |
|---|---|
| `users` | Akun pengguna (kredensial, status) |
| `roles` | Role, termasuk role bawaan (owner/admin/kasir/dst.) — global, satu penanda `is_system`; tanpa scoping perusahaan (lihat ADR-0009) |
| `permissions` | Katalog permission — **satu sumber tunggal** (bukan tersebar di banyak tempat seperti sistem lama) |
| `role_permissions` | Pivot role↔permission |
| `user_roles` | Pivot user↔role |
| `user_branch_access` | Cabang mana yang boleh diakses tiap user + penanda default |
| `authz_seed_applied` | Pelacak pasangan role→permission yang sudah pernah di-seed, supaya seed ulang tidak menghidupkan kembali izin yang sengaja dicabut |

### `company` (L1)

| Tabel | Untuk apa |
|---|---|
| `company_profile` | Baris tunggal: nama, alamat, identitas dokumen cetak — **tanpa** `company_id`, dan **tidak ada tabel `companies`** (lihat ADR-0009) |
| `company_settings` | Pengaturan operasional (mode approval koreksi stok, kalibrasi kertas cetak, dst.) — **hanya setelan yang benar-benar dibaca kode**, tidak ada field mati seperti sistem lama (lihat KI-28/KI-33: zona waktu/mata uang/bahasa/feature-flag **tidak** dibawa) |

### `branch` (L1)

| Tabel | Untuk apa |
|---|---|
| `branches` | Cabang: kode (sumber prefix dokumen), nama, alamat, status aktif/nonaktif (**wajib bisa ditutup** — memperbaiki KI-36 sistem lama yang tidak pernah punya tombolnya) |
| `branch_document_sequences` | Penghitung nomor berjalan per jenis dokumen per cabang |

### `product` (L2)

| Tabel | Untuk apa |
|---|---|
| `products` | Data inti produk: nama, tipe (barang/jasa), harga jual, harga beli (basis HPP), status pantau-stok, **kolom UOM tetap** (`base_uom`, `purchase_uom`, `sales_uom` + 2 faktor konversi — cukup untuk kebutuhan terbukti; tanpa tabel konversi N-UOM, lihat §6) |
| `product_categories` | Kategori produk |
| `product_variants` | Varian (selalu minimal satu baris per produk — kontrak §7 SYSTEM_DESIGN) |

### `party` (L2)

| Tabel | Untuk apa |
|---|---|
| `parties` | Customer & supplier (satu tabel, dibedakan `party_type`) |
| `party_delivery_addresses` | Alamat kirim per customer |
| `member_types` | Jenis member **+ kolom aturan harga** (`price_basis`, arah/jenis/nilai penyesuaian, `rounding_mode`/`increment` — satu aturan per tipe terbukti cukup; tanpa tabel rules, lihat §6) |

### `stock` (L3)

| Tabel | Untuk apa |
|---|---|
| `stock_locations` | Lokasi/gudang, termasuk hierarki lokasi daun; atribut alokasi `is_picking_area`, `is_primary`, `sort_order` (migrasi `000021`, ST-01) |
| `stock_balances` | Saldo per produk/varian per lokasi |
| `stock_movements` | Setiap mutasi (in/out), selalu berpasangan dengan perubahan `stock_balances` (kontrak §7 SYSTEM_DESIGN) |
| `stock_transfers` + `stock_transfer_items` | Transfer antar lokasi (dicatat sebagai dua `stock_movements`, bukan tipe `transfer` sendiri); muka dokumen `driver_name`, `vehicle_plate`, `transfer_date` (WIB, migrasi `000021`, ST-02) |
| `stock_reservations` | Reservasi stok (mis. saat order dibuat sebelum dikirim) |

Saldo awal go-live dibukukan langsung sebagai mutasi `in` ber-referensi `opening`
(tanpa tabel staging — mutasinya sendiri adalah catatannya).

Tanpa tabel alokasi terpisah: baris `stock_movements` membawa referensi dokumen+lokasi+qty+biaya,
cukup untuk penelusuran di bawah HPP moving-average (lihat §6).

### `purchasing` (L4)

| Tabel | Untuk apa |
|---|---|
| `purchase_orders` | Header PO ke supplier (`party_id` primitif ke `party`). `000022_purchasing_tax_invoice`: `tax_invoice_number`/`tax_invoice_date` (faktur pajak supplier); `supplier_invoice_*` tetap sumber PPN Masukan; `assigned_user_id` = penanggung jawab (primitif ke `user`); `created_by` ditampilkan sebagai "Dibuat Oleh". `000034_purchasing_corrections`: `cancel_reason`/`cancelled_at`/`cancelled_by` (batal dengan alasan) dan `short_close_reason`/`short_closed_at`/`short_closed_by` ("Tutup sisa order") |
| `purchase_order_items` | Baris PO — **snapshot** harga & nama produk saat itu (kontrak §7), bukan join ke `products` hidup. `tax_rate` (`000022`, PO-03) = tarif pajak yang dipakai baris (tarif baris, atau tarif order; 0 bila bebas pajak); ETL memetakan `tax_rate_snapshot` legacy |
| `purchase_order_corrections` | Riwayat koreksi order terkonfirmasi/selesai (`000034`, ADR-0023): `kind` `price` (koreksi harga beli setelah penerimaan; id-nya = `source_doc_id` jurnal finance `purchase_price_correction`) atau `short_close` (tanpa jurnal); `reason`, total lama/baru, `value_delta`/`tax_delta` bagian yang sudah diterima |
| `purchase_order_correction_lines` | Baris koreksi: ekonomi baris lama/baru (qty, harga, diskon, DPP, PPN, total) dan `received_qty` saat koreksi — penerimaan sampai qty itu tetap dinilai dengan ekonomi lama di jurnal penerimaan dan ledger biaya (`contracts.LineAt`) |

### `sales` (L4)

| Tabel | Untuk apa |
|---|---|
| `sales_orders` | Header SO ke customer, termasuk `channel` (`regular`/`pos`); `assigned_user_id` = penanggung jawab; `credit_approved_by`/`credit_approved_at` (migrasi `000019_sales`, SL-09) = siapa/kapan kredit disetujui, diisi satu pernyataan dengan peralihan ke termin net; snapshot alamat kirim (`ship_to_*`) boleh berisi alamat sekali pakai dengan `ship_to_address_id` NULL (SL-01). `cancel_reason`/`cancelled_at`/`cancelled_by` dan `short_close_reason`/`short_closed_at`/`short_closed_by` (migrasi `000033_sales_corrections`) = alasan + siapa/kapan order dibatalkan atau ditutup sisanya. ETL memetakan `id_assigned_user`, `credit_approved_by/at` |
| `sales_order_items` | Baris SO — snapshot harga (termasuk harga member yang sudah dihitung saat itu) |

### `goodsreceipt` (L5)

| Tabel | Untuk apa |
|---|---|
| `goods_receipts` | Header penerimaan barang, menunjuk `purchase_order_id` |
| `goods_receipt_items` | Baris penerimaan per item PO |

### `delivery` (L5)

| Tabel | Untuk apa |
|---|---|
| `delivery_notes` | Header surat jalan, menunjuk `sales_order_id` |
| `delivery_note_items` | Baris SJ |

Bukti kirim (foto/dokumen) dicatat langsung sebagai baris `media_files` ber-owner
`delivery_notes` — tanpa tabel bukti tersendiri (lihat §6).

### `payment` (L5)

| Tabel | Untuk apa |
|---|---|
| `payments` | Pembayaran masuk/keluar, menunjuk order (primitif, lintas modul `purchasing`/`sales`). `000023_payment_credit_source` (N-41): `credit_source_type`/`credit_source_id` = retur sumber saldo pada pasangan kaki **pakai saldo uang muka** (kaki retur method `offset`, kaki tagihan method `advance`); jumlah saldo terpakai sebuah retur = Σ alokasi aktif ke retur itu oleh kaki yang menyebutnya sebagai sumber. Legacy tak punya alur ini, ETL tidak menyentuh kolomnya. `000032_payment_corrections`: `corrects_payment_id` = pembayaran yang digantikan lewat **koreksi pembayaran** (yang asli dibatalkan dalam transaksi yang sama); `cancel_reason`, `cancelled_at`, `cancelled_by` = jejak pembatalan (kosong untuk pembatalan sebelum migrasi) |
| `payment_allocations` | Alokasi satu pembayaran ke satu/lebih dokumen |

### `salesreturn` (L6)

| Tabel | Untuk apa |
|---|---|
| `sales_returns` + `sales_return_items` | Retur customer, menunjuk `sales_order_id`/`delivery_note_id` |
| `sales_return_replacement_items` | Baris barang pengganti pada retur jenis tukar barang (produk/varian/lokasi/qty/harga) |
| `sales_return_settlements` | Catatan penyelesaian retur (jenis, tanggal, jumlah, metode) yang melekat pada dokumen retur |

Aliran uangnya tetap satu: refund tunai/transfer dan potong tagihan dibukukan sebagai `payments` +
`payment_allocations` menunjuk dokumen retur (lihat §6); baris `sales_return_settlements` adalah catatan
di sisi retur. Tukar barang diselesaikan lewat surat jalan pengganti; selisih tukar yang lebih mahal
ditagih sebagai `payments` ber-alokasi `sales_return` (jenis *charge*), bisa sekaligus saat SJ pengganti
dikonfirmasi (K-04e).

**Baris `customer_credit` BUKAN lagi memo (K-04a, 2026-10).** Nilai retur yang dijadikan saldo pelanggan
adalah reklasifikasi yang dijurnal sendiri: `finance_journal_entries.source_doc_type =
'sales_return_settlement'`, `source_doc_id` = `sales_return_settlements.id`, Dr Piutang / Cr Uang Muka
Pelanggan (`customer_advance`, kewajiban) atas pelanggan order. Tanpa kolom/tabel baru: jenis penyelesaian
tetap `varchar(30)`. Aturan jenis (K-04c): retur murni / tukar lebih murah → `refund` | `customer_credit`
(≤ nilai terutang − `receivable_reduction` − yang sudah diselesaikan); tukar lebih mahal → `collect_payment`
(≤ selisih); `reduce_receivable` hanya ditulis sistem (`auto = 1`, offset otomatis saat konfirmasi).
Batal retur membalik jurnal setiap penyelesaian kredit; `finance_cutover` punya batas sendiri per jenis
(`sales_return_settlement`, `purchase_return_settlement`) agar ETL tidak melipatgandakan jurnal legacy.

### `purchasereturn` (L6)

| Tabel | Untuk apa |
|---|---|
| `purchase_returns` + `purchase_return_items` | Retur ke supplier, menunjuk `purchase_order_id`/`goods_receipt_id` |
| `purchase_return_settlements` | Catatan penyelesaian retur di sisi retur pembelian |

Penyelesaian retur pembelian memakai mekanisme yang sama: `payments` + `payment_allocations`
menunjuk dokumen retur (lihat §6); `purchase_return_settlements` adalah catatan di sisi retur.

**Kamus retur pembelian (K-04b):** `reduce_payable` (offset otomatis, ditulis sistem), `refund`,
`supplier_credit` — bukan kamus penjualan. Nilai lama `customer_credit` / `reduce_receivable` yang
pernah tersimpan di tabel ini dibaca sebagai `supplier_credit` / `reduce_payable` (tanpa migrasi data;
sistem belum production). **Baris `supplier_credit` dijurnal** (K-04a): `source_doc_type =
'purchase_return_settlement'`, `source_doc_id` = `purchase_return_settlements.id`, Dr Uang Muka Supplier
(`supplier_advance`, aset) / Cr Hutang atas supplier PO. Aturan: hanya kelebihan nilai retur di atas sisa
hutang (`payable_reduction`) yang bisa direfund atau dijadikan saldo supplier.

### `finance` (L7)

| Tabel | Untuk apa |
|---|---|
| `finance_accounts` + `finance_account_mappings` | Chart of Accounts (kolom penanda kas menggantikan tabel kas terpisah) + pemetaan akun otomatis (mis. akun kas per metode bayar) |
| `finance_journal_entries` + `finance_journal_lines` | Jurnal umum — **satu-satunya** sumber laporan keuangan; penyesuaian fiskal memakai tipe jurnal, bukan tabel tersendiri |
| `finance_account_daily` | Agregat harian per cabang/akun/lingkup (debit, kredit) yang dipelihara bersama jurnal — melayani footing dan tren tanpa memindai seluruh `finance_journal_lines` |
| `finance_cutover` | Per cabang dan jenis dokumen: id dokumen terakhir yang sudah tercakup saldo awal/jurnal (batas antrean posting) |
| `finance_periods` | Periode akuntansi (buka/tutup); alasan buka kembali periode dicatat di diff audit, bukan kolom (FN-05) |
| `finance_posting_ignores` | Penanda dokumen yang sengaja diabaikan dari antrean posting (`branch_id`, `doc_type`, `doc_id`, `reason`, `ignored_by`, `ignored_at`; PK gabungan) — migrasi `000031_finance_fn`, FN-01 |
| `finance_inventory_revaluations` + `finance_inventory_revaluation_lines` | Revaluasi harga modal (koreksi harga modal non-nol yang salah): header per komit (`branch_id`, `revaluation_date`, `reason`, `total_delta`, `journal_entry_id`, `client_ref` unik per cabang, `created_by`) + baris per posisi (`product_id`, `variant_id`, `qty_base`, harga satuan sebelum/sesudah, nilai sebelum/sesudah, `delta`, `cost_movement_id` = id sintetis baris ledgernya). Ledger biaya mendapat baris nilai saja `ref_type=inventory_revaluation`, `ref_id` = header — migrasi `000035_finance_corrections` |
| `finance_tax_adjustment_meta` | Klasifikasi (`positive`/`negative`/`final_income`/`non_object`) + catatan koreksi fiskal, 1:1 ke `finance_journal_entries` (FK dalam modul) — `000031`, FN-07 |
| `finance_inventory_cost_movements` | HPP per mutasi stok — rata-rata berjalan = baris mutasi terakhir (tanpa tabel state; ledger adalah kebenarannya). K-05: baris biaya-saja (`ref_type=opening_cost`, qty 0) memakai `stock_movement_id` ≥ 2^62 agar tak bertabrakan dengan mutasi nyata; kursor costing mengabaikan rentang itu. Tanpa perubahan skema |
| `finance_tax_periods` + `finance_tax_report_snapshots` | Snapshot laporan pajak per periode |
| `finance_document_sequences` | Penomoran dokumen keuangan (beda dari nomor dokumen cabang) |
| `business_expenses` | Biaya usaha di luar transaksi jual-beli; `party_id` opsional = pihak lawan (`000031`, FN-10) |
| `finance_bank_accounts` | Rekening perusahaan (ADR-0026): bank, nomor, atas nama, `show_on_documents`, `is_default`, status — `account_id` UNIQUE FK ke `finance_accounts` (akun aset kas sendiri per rekening). Tidak menyimpan saldo: saldo = saldo akun buku besarnya. `sales_orders.receipt_account_id` dan `payments.cash_account_id` menyimpan id akun itu sebagai ID primitif (`000046`) |

Tanpa antrean posting terpisah: antrean dan pratinjau dihitung dari dokumen sumber on-the-fly;
satu-satunya keadaan tersimpan adalah penanda abaikan `finance_posting_ignores` (FN-01). Tanpa tabel saldo awal
keuangan: cutover memakai jurnal saldo awal (nilai Persediaan dihitung server dari ledger biaya, K-05). Koreksi biaya
persediaan (`costrebuild`, OP-04) memposting satu jurnal `cost_restatement` tanpa tabel baru. Detail alasan tiap
peniadaan ada di §6.

### `dashboard` / `reporting` (L8)

Tidak memiliki tabel transaksional sendiri (baca-saja lewat kontrak modul lain), kecuali:

| Tabel | Untuk apa |
|---|---|
| `dashboard_alert_snoozes` (`dashboard`) | Penundaan alert kesehatan data per (cabang, alert), `uq (branch_id, alert_id)`: alasan, durasi, `snoozed_until` (UTC), jumlah & rupiah saat ditunda (tunda gugur bila masalah memburuk). Alert sendiri tidak disimpan — dihitung langsung (`000045`, ADR-0025) |
| `daily_operational_metrics` (`reporting`) | Hasil agregasi harian terjadwal (order, stok) — **DIHAPUS** (cron menulis tapi nol pembaca — KI-126 diputuskan hapus di L8; reporting baca finance live) |

## 4. Contoh Pola Relasi Lintas Modul (Primitive ID, Bukan FK)

```sql
-- modul `sales`, tabel sales_orders — BENAR
party_id     BIGINT UNSIGNED NOT NULL,  -- menunjuk parties (modul `party`), TANPA FK fisik
branch_id    BIGINT UNSIGNED NOT NULL,  -- menunjuk branches (modul `branch`), TANPA FK fisik

-- modul `sales`, tabel sales_order_items — FK DI DALAM modul yang sama BOLEH
sales_order_id BIGINT UNSIGNED NOT NULL REFERENCES sales_orders(id),
product_id     BIGINT UNSIGNED NOT NULL,  -- menunjuk products (modul `product`), TANPA FK fisik
product_name_snapshot VARCHAR(255) NOT NULL,  -- snapshot, bukan bergantung ke product hidup
```

Validasi keberadaan `party_id`/`product_id` dilakukan lewat pemanggilan `PartyClient.GetByID` /
`ProductClient.GetByID` di application layer modul `sales` saat order dibuat — bukan lewat
constraint database.

## 5. Keputusan Skema yang Semula Terbuka

- Primary key: `BIGINT UNSIGNED AUTO_INCREMENT` di semua tabel (tanpa ULID/UUID).
- Presisi desimal (dikunci dari bukti kode warisan): kuantitas & faktor konversi UOM =
  `DECIMAL(24,12)` (`order-item.entity.ts:47-51`, `product.entity.ts:48-52`); persen penyesuaian
  harga = `DECIMAL(18,4)` (`member-type.entity.ts:36`). Uang tetap integer rupiah (ADR-0005),
  tanpa desimal di mana pun.
- Soft-delete vs status kolom (`active`/`archived`) — tetapkan per modul sesuai kebutuhan bisnis
  modul itu (produk & party memang butuh arsip; tabel referensi seperti `permissions` tidak).
- Pintu darurat (hanya bila kebutuhan terbukti, bukan spekulasi): tabel konversi N-UOM bila ada
  produk yang benar-benar dijual dalam 3+ satuan; tabel rules harga bila perlu aturan per
  kategori/produk di luar satu-aturan-per-tipe. Keduanya reversibel dan murah ditambahkan belakangan.

## 6. Disposisi Tabel Legacy (70 → 67)

Realisasi di §3 total **67 tabel** (metrik harian tidak dipertahankan — KI-126 diputuskan
hapus di L8; 7 tabel `assistant` L9 ikut terhitung; dua tabel kecil `finance_posting_ignores` dan
`finance_tax_adjustment_meta` ditambah `000031` sebagai pengganti ringan, bukan memulihkan tabel legacy; `finance_inventory_revaluations` + `_lines` ditambah `000035` untuk revaluasi harga modal, fitur baru), turun dari **70 entity** sistem lama.
Setiap tabel di bawah lolos uji kebutuhan — fakta yang wajib persisten, bukan struktur
warisan. Mekanisme pengganti dicatat agar kemampuan user tidak hilang diam-diam:

| Tabel legacy | Disposisi | Status |
|---|---|---|
| `companies` | Dihapus — singleton `company_profile`, tanpa `company_id` di mana pun | Diputuskan (ADR-0009) |
| `company_features` | Dihapus — mati di 3 lapisan (KI-33) | Diputuskan |
| `orders` (satu tabel dua jenis) | Dipecah jadi `purchase_orders` + `sales_orders` (batas modul milik skill) | Diputuskan (§3) |
| `order_status_definitions`, `order_status_transitions`, `order_status_history` | Tanpa tabel — mesin status tetap di kode (status baru warisan lahir cacat/KI-35, tak pernah berfungsi penuh) | Diputuskan bersyarat — gugur bila pemilik butuh kelola status (KI-35) |
| `finance_posting_sources` (antrean posting 2-fase) | Tanpa tabel antrean — pratinjau dihitung on-the-fly. Abaikan/pulihkan (FN-01) = tabel penanda kecil `finance_posting_ignores` milik finance (`branch_id`, `doc_type`, `doc_id`, `reason`, `ignored_by`, `ignored_at`; migrasi `000031`), bukan kolom di modul sumber (tanpa lintas-modul); pulihkan = hapus baris, jejaknya di audit | Diputuskan |
| `finance_opening_balances`, `finance_opening_inventory_items` | Tanpa tabel — saldo awal stok = mutasi `in` ber-referensi `opening` (mutasinya sendiri catatannya, L3); saldo awal keuangan = jurnal saldo awal | Diputuskan |
| `finance_cash_accounts` | Kolom penanda kas di `finance_accounts` (saldo kas = saldo akun). Revisi ADR-0026: identitas rekening bank (bank/nomor/atas nama) kini di `finance_bank_accounts` 1:1 ke akun kas — tetap tanpa saldo tersendiri | Diputuskan; direvisi ADR-0026 |
| `finance_inventory_cost_states` | Tanpa tabel — rata-rata berjalan = baris terakhir `finance_inventory_cost_movements` | Diputuskan |
| `finance_tax_adjustments` | Tipe jurnal penyesuaian (tetap teraudit, tetap bisa dilaporkan). FN-07 (ADR-0018 D6): jenis koreksi + catatan disimpan di tabel pendamping `finance_tax_adjustment_meta` (1:1 ke jurnal, migrasi `000031`), lampiran di media (`owner_type=tax_adjustment`); model jurnal tidak berubah | Diputuskan |
| `stock_issue_allocations` | Tanpa tabel — referensi dokumen+lokasi+qty+biaya di `stock_movements` (cukup di bawah HPP moving-average; saran petik FIFO tetap dihitung, bukan disimpan) | Diputuskan |
| `sales_return_settlements`, `purchase_return_settlements` | Dipertahankan sebagai catatan sisi retur; uangnya mengalir lewat `payments` + `payment_allocations` menunjuk dokumen retur — satu aliran uang untuk finance | Direvisi saat implementasi |
| `delivery_proofs` | Baris `media_files`; tujuan bukti dibedakan lewat owner type `delivery_proof` (foto lokasi) dan `delivery_document` (scan SJ bertanda tangan) — DL-01, tanpa kolom `purpose`; ETL memetakan purpose legacy ke owner type | Diputuskan |
| (aturan harga member — warisan di kolom `member_types`) | Kolom aturan di `member_types` (`member-type.entity.ts:27-43` membuktikan satu aturan per tipe melayani user; tanpa tabel rules spekulatif) | Diputuskan — pintu darurat di §5 bila perlu aturan per kategori |
| `product_uom_conversions` (rencana awal, bukan warisan) | Kolom UOM tetap di `products` (`product.entity.ts:39-52` membuktikan 2 UOM + base melayani semua alur; line order membawa UOM-nya sendiri) | Diputuskan — pintu darurat di §5 bila ada produk 3+ satuan |
| `daily_operational_metrics` | Dihapus — L8 `reporting`/`dashboard` membaca finance live (`NetProfit` per hari, `InventoryValue`) tanpa tabel agregasi/cron; nol SELECT warisan tetap nol tabel | Diputuskan (L8, KI-126) |

Tabel baru yang memang diperlukan (bukan warisan, bukan spekulasi): `delivery_note_items`
terpisah dari SJ (penerimaan/pengiriman parsial per baris adalah kebutuhan gudang nyata),
`stock_reservations` (hold keranjang POS ber-TTL).

## 7. Referensi

- [SYSTEM_DESIGN.md §3, §7](SYSTEM_DESIGN.md) — peta modul & kontrak lintas modul
- [legacy-reference/*/data-model-legacy.md](legacy-reference/) — struktur tabel asli per modul,
  dipakai sebagai pembanding saat menulis migrasi modul baru
- `.claude/rules/database.md` — aturan boundary yang ditegakkan Claude Code saat menyunting migrasi

## 8. Riwayat migrasi bernomor sesudah baseline

Kebijakan: baseline per modul (`000001`–`000018`) tidak diedit; perubahan sesudahnya berupa migrasi bernomor maju bernama
`NNNNNN_<modul>[_<perubahan>]` (arch-check Rule T/F membaca nama berkas sebagai pemilik tabel; `scripts/squash-baseline.mjs`).
Semua kolom baru nullable/berdefault sehingga idempoten terhadap data ETL (`apps/api/cmd/etl`). Nomor 000024–000030 kosong dan boleh dipakai berikutnya.

| Migrasi | Tugas | Perubahan | ETL |
|---|---|---|---|
| `000045_dashboard_alert_snooze` | ADR-0025 (alert kesehatan data) | tabel baru `dashboard_alert_snoozes` (modul `dashboard`, tanpa FK lintas modul); `dashboard` pindah dari `noSql` ke `migrated` sqlc | tidak dimuat ETL (tabel mulai kosong) |
| `000046_finance_bank_accounts` | ADR-0026 (rekening tujuan) | tabel baru `finance_bank_accounts` (FK hanya ke `finance_accounts`, modul sama) | tidak dimuat ETL (rekening diatur sesudah cutover) |
| `000047_sales_receipt_account` | ADR-0026 | `sales_orders.receipt_account_id` (nullable, ID primitif akun rekening finance) | tak terpengaruh (NULL = belum ada rekening tujuan) |
| `000048_payment_cash_account` | ADR-0026 | `payments.cash_account_id` (nullable) + indeks; NULL = jurnal lewat pemetaan `cash`/`bank` seperti sebelumnya | tak terpengaruh (pembayaran legacy tanpa rekening) |
| `000019_sales` | SL-09 | `sales_orders.credit_approved_by`, `credit_approved_at` (nullable): siapa/kapan kredit disetujui | memetakan `credit_approved_*` dan `assigned_user_id` legacy |
| `000020_assistant_tool_status` | AS-02 | `assistant_tool_executions.status` (default `success`), `error_text`: tool gagal tercatat per baris | tak terpengaruh |
| `000021_stock_parity` | ST-01, ST-02 | `stock_locations.is_picking_area`, `is_primary`, `sort_order`; `stock_transfers.driver_name`, `vehicle_plate`, `transfer_date` (diisi hari WIB kirim/dibuat untuk dokumen lama) | spec load membawa `is_picking_area`/`is_primary`/`sort_order`, `driver_name`/`vehicle_number`/`transfer_date` |
| `000022_purchasing_tax_invoice` | PO-03 | `purchase_orders.tax_invoice_number`, `tax_invoice_date`; `purchase_order_items.tax_rate` | memetakan `tax_rate_snapshot` legacy |
| `000023_payment_credit_source` | N-41 | `payments.credit_source_type`, `credit_source_id` + indeks `ix_payments_credit_source` (retur sumber saldo uang muka pada pasangan kaki pakai-saldo) | legacy tak punya alur ini; kolom nullable, ETL tak menyentuh |
| `000034_purchasing_corrections` | ADR-0023 (koreksi order beli) | `purchase_orders.cancel_reason`, `cancelled_at`, `cancelled_by`, `short_close_reason`, `short_closed_at`, `short_closed_by`; tabel baru `purchase_order_corrections` + `purchase_order_correction_lines` | kolom nullable, legacy tak punya alur ini; ETL tak menyentuh |
| `000031_finance_fn` | FN-01, FN-07, FN-10 | tabel baru `finance_posting_ignores` dan `finance_tax_adjustment_meta`; kolom `business_expenses.party_id` (+ `ix_business_expenses_party`): pihak lawan biaya | mengisi `party_id` dari `id_business_party` legacy bila pihak itu termigrasi |
| `000032_payment_corrections` | Koreksi pembayaran | `payments.corrects_payment_id` (+ `ix_payments_corrects`), `cancel_reason`, `cancelled_at`, `cancelled_by` (semua nullable) | tak terpengaruh; kolom nullable, ETL tak menyentuh |
| `000035_finance_corrections` | Koreksi HPP lewat web | tabel baru `finance_inventory_revaluations` dan `finance_inventory_revaluation_lines` (revaluasi harga modal: satu jurnal per komit, satu baris ledger nilai saja per posisi) | tak terpengaruh: tabel baru kosong, legacy tak punya alur ini |
| `000036_salesreturn_corrections` | Koreksi salah input | `sales_returns.cancel_reason` (varchar 255), `cancelled_at`, `cancelled_by` (nullable): alasan, waktu, dan pelaku pembatalan; diisi bersama status batal dalam satu CAS, audit ikut transaksi itu | dokumen legacy yang batal tetap NULL (alasan tidak tercatat di sistem lama) |
| `000037_delivery_corrections` | Koreksi salah input | `delivery_notes.cancel_reason` (varchar 255), `cancelled_at`, `cancelled_by` (nullable): alasan, waktu, dan pelaku pembatalan (draf, termasuk SJ pengganti yang ditarik lewat retur tukar); diisi bersama status batal dalam satu CAS, audit ikut transaksi itu | dokumen legacy yang batal tetap NULL (alasan tidak tercatat di sistem lama) |
| `000039_goodsreceipt_corrections` | Koreksi salah input | `goods_receipts.cancel_reason` (varchar 255), `cancelled_at`, `cancelled_by` (nullable): alasan, waktu, dan pelaku pembatalan (penerimaan `pending` yang dibuang → `void`); diisi bersama status batal dalam satu CAS, audit ikut transaksi itu | dokumen legacy yang batal tetap NULL (alasan tidak tercatat di sistem lama) |
| `000040_purchasereturn_corrections` | Koreksi salah input | `purchase_returns.cancel_reason` (varchar 255), `cancelled_at`, `cancelled_by` (nullable): alasan, waktu, dan pelaku pembatalan; diisi bersama status batal dalam satu CAS, audit ikut transaksi itu | dokumen legacy yang batal tetap NULL (alasan tidak tercatat di sistem lama) |
| `000041_stock_corrections` | Koreksi salah input | `stock_transfers.cancel_reason` (varchar 255), `cancelled_at`, `cancelled_by` (nullable): alasan, waktu, dan pelaku pembatalan; diisi bersama status batal dalam satu CAS, audit ikut transaksi itu | dokumen legacy yang batal tetap NULL (alasan tidak tercatat di sistem lama) |
| `000042_delivery_direct` | ADR-0022 (SJ Langsung) | `delivery_notes.origin` (`'order'` bawaan \| `'direct'`), `party_id`, `ship_to_label/recipient/phone/address` (snapshot pelanggan SJ tanpa order), `linked_at`, `linked_by` (kapan SJ Langsung mendapat ordernya); indeks `ix_delivery_notes_direct_open (branch_id, origin, sales_order_id, status, delivery_date)`. `sales_order_id = 0` selama belum diorder | kolom berdefault/nullable; data legacy tetap `origin='order'`, ETL tak menyentuh |
| `000044_finance_opening_correction` | ADR-0024 (koreksi saldo awal) | baris data, tanpa skema: akun `3100 Koreksi Saldo Awal` (modal) + pemetaan `opening_correction`, hanya bila bagan akun sudah ada; mutasi stok `ref_type = opening_correction` dijurnal Persediaan lawan 3100 | ETL menambahkan akun dan pemetaan yang sama |
| `000043_sales_origin` | ADR-0022 (order dari SJ) | `sales_orders.origin` (`'standard'` bawaan \| `'delivery'` = order yang dibangun dari SJ Langsung, langsung `completed`) | kolom berdefault; ETL tak menyentuh |
| `000033_sales_corrections` | Koreksi salah input order jual | `sales_orders.cancel_reason`, `cancelled_at`, `cancelled_by` (alasan + siapa/kapan batal, diisi satu pernyataan CAS dengan status `cancelled`) dan `short_close_reason`, `short_closed_at`, `short_closed_by` ("tutup sisa order": baris dipotong ke qty terkirim, order `completed`) — semua nullable | legacy tak punya alur ini; kolom nullable, ETL tak menyentuh |

Rincian kolom tiap tabel ada di narasi §3 modul pemiliknya. Perubahan tanpa skema yang perlu diketahui pembaca skema:
bukti kirim bertipe = owner type media `delivery_document` (scan SJ) vs `delivery_proof` (foto lokasi), bukan kolom `purpose` (DL-01);
`archivedAt` produk = `updated_at` baris berstatus arsip (PR-04); jurnal `opening_supplement`, `cost_restatement`, `inventory_revaluation`, `sales_return_settlement`,
`purchase_return_settlement` adalah sumber posting baru tanpa tabel baru.
`archivedAt` produk = `updated_at` baris berstatus arsip (PR-04); jurnal `opening_supplement`, `cost_restatement`, `sales_return_settlement`,
`purchase_return_settlement` adalah sumber posting baru tanpa tabel baru; `purchase_price_correction` (ADR-0023) menunjuk
`purchase_order_corrections.id` dan menulis baris nilai-saja `ref_type = purchase_price_correction` di `finance_inventory_cost_movements`.
