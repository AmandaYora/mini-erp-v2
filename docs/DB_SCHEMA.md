# Database Schema — Mini ERP (Revamp)

Status: **kontrak kepemilikan tabel yang sudah terkunci** — 56 tabel nyata di
`apps/api/migrations/` (000001–000018), turun dari 70 entity sistem lama.
Dokumen ini mengikat kepemilikan per modul; §6 mencatat disposisi tiap tabel warisan.

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
6. Migrasi lewat `golang-migrate`, **bernomor & terlacak** (`apps/api/migrations/NNNNNN_*.up.sql` /
   `.down.sql`) — ada tabel versi migrasi bawaan, tidak seperti runner sistem lama yang menjalankan
   ulang seluruh migrasi tiap start.
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
baru. Nama tabel di bawah **dikunci** — cerminan migrasi 000001–000018 yang sudah terbit.

### `audit` (L0)

| Tabel | Untuk apa |
|---|---|
| `audit_logs` | `action_key` (`resource.action`), `entity_type`+`entity_id`, `branch_id` (NULL = event global), `actor_id`, `note` — ditulis semua modul penulis via `AuditClient.Log` **best-effort setelah komit** (tak pernah menggagalkan operasi); v1 mencatat aksi, bukan diff before/after. Dikunci migrasi `000017` (L8) |

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
| `assistant_tool_executions` | Tiap tool per run (seq, input/output JSON) |

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
| `stock_locations` | Lokasi/gudang, termasuk hierarki lokasi daun |
| `stock_balances` | Saldo per produk/varian per lokasi |
| `stock_movements` | Setiap mutasi (in/out), selalu berpasangan dengan perubahan `stock_balances` (kontrak §7 SYSTEM_DESIGN) |
| `stock_transfers` + `stock_transfer_items` | Transfer antar lokasi (dicatat sebagai dua `stock_movements`, bukan tipe `transfer` sendiri) |
| `stock_reservations` | Reservasi stok (mis. saat order dibuat sebelum dikirim) |

Saldo awal go-live dibukukan langsung sebagai mutasi `in` ber-referensi `opening`
(tanpa tabel staging — mutasinya sendiri adalah catatannya).

Tanpa tabel alokasi terpisah: baris `stock_movements` membawa referensi dokumen+lokasi+qty+biaya,
cukup untuk penelusuran di bawah HPP moving-average (lihat §6).

### `purchasing` (L4)

| Tabel | Untuk apa |
|---|---|
| `purchase_orders` | Header PO ke supplier (`party_id` primitif ke `party`) |
| `purchase_order_items` | Baris PO — **snapshot** harga & nama produk saat itu (kontrak §7), bukan join ke `products` hidup |

### `sales` (L4)

| Tabel | Untuk apa |
|---|---|
| `sales_orders` | Header SO ke customer, termasuk `channel` (`regular`/`pos`) |
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
| `payments` | Pembayaran masuk/keluar, menunjuk order (primitif, lintas modul `purchasing`/`sales`) |
| `payment_allocations` | Alokasi satu pembayaran ke satu/lebih dokumen |

### `salesreturn` (L6)

| Tabel | Untuk apa |
|---|---|
| `sales_returns` + `sales_return_items` | Retur customer, menunjuk `sales_order_id`/`delivery_note_id` |

Penyelesaian retur (refund tunai/transfer, potong tagihan) dimodelkan sebagai `payments` +
`payment_allocations` menunjuk dokumen retur — satu aliran uang (lihat §6). Tukar barang
diselesaikan lewat surat jalan pengganti, tanpa uang.

### `purchasereturn` (L6)

| Tabel | Untuk apa |
|---|---|
| `purchase_returns` + `purchase_return_items` | Retur ke supplier, menunjuk `purchase_order_id`/`goods_receipt_id` |

Penyelesaian retur pembelian memakai mekanisme yang sama: `payments` + `payment_allocations`
menunjuk dokumen retur (lihat §6).

### `finance` (L7)

| Tabel | Untuk apa |
|---|---|
| `finance_accounts` + `finance_account_mappings` | Chart of Accounts (kolom penanda kas menggantikan tabel kas terpisah) + pemetaan akun otomatis (mis. akun kas per metode bayar) |
| `finance_journal_entries` + `finance_journal_lines` | Jurnal umum — **satu-satunya** sumber laporan keuangan; penyesuaian fiskal memakai tipe jurnal, bukan tabel tersendiri |
| `finance_periods` | Periode akuntansi (buka/tutup) |
| `finance_inventory_cost_movements` | HPP per mutasi stok — rata-rata berjalan = baris mutasi terakhir (tanpa tabel state; ledger adalah kebenarannya) |
| `finance_tax_periods` + `finance_tax_report_snapshots` | Snapshot laporan pajak per periode |
| `finance_document_sequences` | Penomoran dokumen keuangan (beda dari nomor dokumen cabang) |
| `business_expenses` | Biaya usaha di luar transaksi jual-beli |

Tanpa antrean posting terpisah: status posting + pratinjau dihitung dari dokumen sumber
(kolom status di dokumen, pratinjau dihitung on-the-fly). Tanpa tabel saldo awal keuangan:
cutover memakai jurnal saldo awal. Detail alasan tiap
peniadaan ada di §6.

### `dashboard` / `reporting` (L8)

Tidak memiliki tabel transaksional sendiri (baca-saja lewat kontrak modul lain), kecuali:

| Tabel | Untuk apa |
|---|---|
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

## 5. Belum Ditetapkan (diputuskan saat modul terkait didesain)

- Strategi primary key: auto-increment integer vs ULID/UUID. Pilih satu, konsisten di semua modul.
- Presisi desimal (dikunci dari bukti kode warisan): kuantitas & faktor konversi UOM =
  `DECIMAL(24,12)` (`order-item.entity.ts:47-51`, `product.entity.ts:48-52`); persen penyesuaian
  harga = `DECIMAL(18,4)` (`member-type.entity.ts:36`). Uang tetap integer rupiah (ADR-0005),
  tanpa desimal di mana pun.
- Soft-delete vs status kolom (`active`/`archived`) — tetapkan per modul sesuai kebutuhan bisnis
  modul itu (produk & party memang butuh arsip; tabel referensi seperti `permissions` tidak).
- Pintu darurat (hanya bila kebutuhan terbukti, bukan spekulasi): tabel konversi N-UOM bila ada
  produk yang benar-benar dijual dalam 3+ satuan; tabel rules harga bila perlu aturan per
  kategori/produk di luar satu-aturan-per-tipe. Keduanya reversibel dan murah ditambahkan belakangan.

## 6. Disposisi Tabel Legacy (70 → 56)

Realisasi di §3 total **56 tabel** (metrik harian tidak dipertahankan — KI-126 diputuskan
hapus di L8; 7 tabel `assistant` L9 ikut terhitung), turun dari **70 entity** sistem lama.
Setiap tabel di bawah lolos uji kebutuhan — fakta yang wajib persisten, bukan struktur
warisan. Mekanisme pengganti dicatat agar kemampuan user tidak hilang diam-diam:

| Tabel legacy | Disposisi | Status |
|---|---|---|
| `companies` | Dihapus — singleton `company_profile`, tanpa `company_id` di mana pun | Diputuskan (ADR-0009) |
| `company_features` | Dihapus — mati di 3 lapisan (KI-33) | Diputuskan |
| `orders` (satu tabel dua jenis) | Dipecah jadi `purchase_orders` + `sales_orders` (batas modul milik skill) | Diputuskan (§3) |
| `order_status_definitions`, `order_status_transitions`, `order_status_history` | Tanpa tabel — mesin status tetap di kode (status baru warisan lahir cacat/KI-35, tak pernah berfungsi penuh) | Diputuskan bersyarat — gugur bila pemilik butuh kelola status (KI-35) |
| `finance_posting_sources` (antrean posting 2-fase) | Tanpa tabel — kolom status di dokumen sumber + pratinjau dihitung on-the-fly; abaikan/pulihkan = kolom + alasan | Diputuskan |
| `finance_opening_balances`, `finance_opening_inventory_items` | Tanpa tabel — saldo awal stok = mutasi `in` ber-referensi `opening` (mutasinya sendiri catatannya, L3); saldo awal keuangan = jurnal saldo awal | Diputuskan |
| `finance_cash_accounts` | Kolom penanda kas di `finance_accounts` (saldo kas = saldo akun) | Diputuskan |
| `finance_inventory_cost_states` | Tanpa tabel — rata-rata berjalan = baris terakhir `finance_inventory_cost_movements` | Diputuskan |
| `finance_tax_adjustments` | Tipe jurnal penyesuaian (tetap teraudit, tetap bisa dilaporkan) | Diputuskan |
| `stock_issue_allocations` | Tanpa tabel — referensi dokumen+lokasi+qty+biaya di `stock_movements` (cukup di bawah HPP moving-average; saran petik FIFO tetap dihitung, bukan disimpan) | Diputuskan |
| `sales_return_settlements`, `purchase_return_settlements` | `payments` + `payment_allocations` menunjuk dokumen retur — satu aliran uang untuk finance | Diputuskan |
| `delivery_proofs` | Baris `media_files` ber-owner `delivery_notes` (warisan pun hanya simpan key storage) | Diputuskan |
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
