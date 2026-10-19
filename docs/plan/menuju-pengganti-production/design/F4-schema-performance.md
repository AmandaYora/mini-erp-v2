# Rancangan F4 — Schema & Performa (siap-implementasi)

Cakupan: seluruh `apps/api/migrations` (27 migrasi, 54 berkas), repository.go tiap modul, `cmd/server/main.go`, `database/db.go`, `scripts/migrate.mjs`, `internal/integration/integration_test.go`.
Basis: bacaan kode saya sendiri (revamp + entity legacy `mini-erp/apps/api/src/**/*.entity.ts`). Nomor baris = hasil baca saya. Estimasi angka ditandai (EST).
Aturan yang dipatuhi: satu tabel satu modul; tanpa FK/join lintas modul; uang integer rupiah (ADR-0005); tanpa Redis; satu container; golang-migrate; transaksi menahan satu koneksi; seed TIDAK ada di migrasi (terbukti: `grep -il insert migrations/*.sql` kosong; seed di `cmd/seed`).

## 0. Ringkasan eksekutif

1. Schema revamp sudah bersih di sisi kepemilikan, tetapi punya 5 masalah nyata: (a) laporan keuangan membaca seumur hidup tiap muat (tak ada rollup, `CostPositions` menjumlah SELURUH ledger biaya tiap kali `averageCost` dipanggil); (b) indeks daftar/laporan tidak cocok dengan bentuk query (`product_id` saja di stock_movements, `party_id` di orders, `from/to_branch_id` di transfer, `status+delivery_date` di surat jalan); (c) kolom terlalu sempit untuk data legacy (username 50 vs 100, branch.code 10 vs 50, notes 255 vs TEXT, tax_rate/discount DECIMAL(5,2) vs (9,4), nomor dokumen 60 vs 100, dst.); (d) tipe `BIGINT` untuk semua id padahal single-company (INT UNSIGNED cukup; terukur: tabel baris jurnal 164 -> 83 MB per 900k baris; total DB EST -24% bersama perubahan indeks); (e) tak ada retensi (audit_logs, stock_reservations, user_sessions, assistant_*).
2. Keputusan struktur: baseline bersih = 18 berkas, SATU PER MODUL bertabel (`000001_user` .. `000018_assistant`, selaras F8-18/sqlc), dibuat SETELAH semua blok schema fase 2-4 selesai dan SEBELUM ETL; kesetaraan dibuktikan dengan skrip pembanding information_schema (F4-1 + Lampiran C). Seed tidak menyulitkan: migrasi memang tanpa data.
3. Tiga rollup baru di modul finance (`finance_account_daily`, `finance_party_daily`, `finance_product_daily`) + `finance_cost_positions`/`finance_cost_state` (posisi biaya + kursor, selaras F2b M-E) menggantikan SELURUH agregasi seumur-hidup; semuanya diperbarui di transaksi yang sama dengan penulisan baris sumber dan diverifikasi oleh query pembanding (F4-7).
4. Temuan baru di luar audit: `PostedSourceIDs` + antrean posting hanya melihat 200 dokumen terbaru (dokumen lama yang belum terposting tak pernah muncul); `CriticalStock` memotong 5000 produk diam-diam; `TaxDetail` memotong 10.000 entri diam-diam (kertas kerja SPT bisa kurang); `DATEDIFF(CURDATE(),...)` bergantung zona waktu server DB; race cache sesi/chart (bagian C); `INSERT IGNORE`+`FOR UPDATE` di REPEATABLE READ = sumber deadlock (rekomendasi READ COMMITTED).
5. Semua rekomendasi: dampak API/UI = TIDAK ADA kecuali disebut.

### 0.1 Daftar tugas (ringkas)

| ID | Judul | Ukuran | Kapan |
|---|---|---|---|
| F4-1 | Squash: baseline per-modul + pembanding information_schema | L | di AKHIR fase schema, sebelum F5 ETL & F8-18 sqlc |
| F4-2 | Pelebaran kolom agar data legacy muat (migrasi dev per modul `000050+`) | M | awal fase 2 (sebelum rehearsal ETL) |
| F4-3 | Tipe & ukuran baris: id INT UNSIGNED, PK komposit, ENUM/CHECK, charset (edit baseline) | M | saat squash (tahap 2) |
| F4-4 | Stok: `stock_movements`/`stock_balances`/`stock_reservations`/`stock_transfers` — indeks, kolom, retensi | M | bersama F2a-0 |
| F4-5 | Sales/purchasing: indeks daftar, pencarian, `client_ref`, `ListByParty` | M | bersama F2a-12/F3a-2 |
| F4-6 | Finance: rollup saldo (akun/pihak/produk), posisi biaya, indeks lines, kursor | L | bersama F2b-14a (fondasi performa) |
| F4-7 | N+1 jalur baca/tulis: TaxDetail, ledger payment, priceLines, antrean posting, MoveOut batch | M | setelah F4-6 |
| F4-8 | Payment/delivery/retur: indeks penutup, `WaitingReturn` sargable | S | bersama F2a-2/6/8 |
| F4-9 | Audit: indeks, tabel detail before/after, retensi | M | bersama F3b-audit |
| F4-10 | Housekeeping/retensi: reservasi, sesi, assistant, audit | S | setelah F4-9 |
| F4-11 | Ekspor: keyset + streaming + timeout per rute | M | bersama F6-4 |
| F4-12 | DSN: isolation READ COMMITTED, `time_zone`, jaminan schema untuk retry deadlock, pool, parameter server | S | awal |
| F4-13 | Race cache chart-of-accounts & sesi | S | awal |
| F4-14 | Dashboard: singleflight per kunci (+ endpoint agregat opsional) | M | setelah F4-6 |
| F4-15 | `CriticalStock` tanpa batas 5000 | S | kapan saja |
| F4-16 | Penomoran satu-statement | S | bersama F2a-11 |
| F4-17 | Harness EXPLAIN/benchmark + verifikasi rollup | M | paralel, gerbang sebelum squash |

Urutan: F4-12 -> F4-13 -> F4-2 -> (F4-4, F4-5, F4-8, F4-9 menyatu dengan migrasi F2a/F3a/F3b) -> F4-6 -> F4-7/F4-14 -> F4-17 -> **gerbang "schema beku"** -> F4-1 (+F4-3) -> F5 ETL -> F8-18.

---

## 1. Dasar kerja

### 1.1 Asumsi volume (E) — SEMUA ESTIMASI, dasar: tidak ada dump legacy di repo; angka pemilik "200k order, 1jt mutasi, 3jt baris jurnal" dipakai apa adanya, sisanya turunan saya

| Entitas | Perkiraan 3 tahun (EST) | Catatan |
|---|---|---|
| cabang / user / role | 3 / 30 / 5 | tabel kecil |
| party (customer+supplier) | 15.000 | `parties` LIKE %x% masih murah |
| produk / varian | 8.000 / 12.000 | |
| `sales_orders` / item | 200.000 / 600.000 | ~60% kanal POS (EST) |
| `purchase_orders` / item | 15.000 / 60.000 | |
| `delivery_notes` / item | 150.000 / 450.000 | POS tidak punya SJ |
| `payments` / alokasi | 250.000 / 260.000 | |
| `stock_movements` | 1.000.000 | angka pemilik |
| `finance_inventory_cost_movements` | 1.000.000 | 1:1 dengan movement |
| `finance_journal_entries` / `_lines` | 1.000.000 / 3.000.000 | angka pemilik (3 baris/entri) |
| `stock_balances` (posisi) | 120.000 | 12k varian x ~10 lokasi (EST) |
| `stock_reservations` (tanpa retensi) | 400.000 | hold POS per baris keranjang (EST) |
| `audit_logs` | 1.500.000 | 6-8 aksi/order (EST); bertambah bila before/after (F3b-12) |
| `user_sessions` | 35.000 | 30 user x 1 login/hari (EST) |

Ukuran total schema SAAT INI pada volume itu ~2,1 GB (EST, ekstrapolasi linier dari probe 1.2); setelah rekomendasi F4 ~1,6 GB (EST, -24%). Working set panas (indeks + 90 hari terakhir) < 1 GB: `innodb_buffer_pool_size` >= 1 GB cukup; probe berjalan dengan buffer pool 128 MB bawaan Laragon (jadi angka "sebelum" sengaja pesimistis karena I/O).

### 1.2 Metode (statis dulu, probe seperlunya)
- Membaca SELURUH `migrations/*.up.sql` (27 berkas up), seluruh `*/infrastructure/*.go`, service terkait, entity legacy (`mini-erp/apps/api/src/**/*.entity.ts`), dan 8 berkas `plan-*.md` agen lain.
- Pemakaian kolom: skrip `scratchpad/f4_cols2.mjs` membandingkan setiap kolom 59 tabel dengan seluruh Go non-test. Hasil: **tidak ada kolom yang tak pernah disebut**; tabel WRITE-ONLY (ditulis, tak pernah dibaca endpoint mana pun): `finance_tax_report_snapshots` (`ledger.go:337` satu-satunya), `assistant_tool_executions` (jejak, memang by design), kolom `finance_inventory_cost_movements.unit_cost` (hanya INSERT `ledger.go:66`). Tidak ada kolom dihapus karena ini; ditandai di §3 S-9.
- Probe EXPLAIN/waktu: database SEMENTARA `zz_schema_probe` di MySQL 8.4.3 Laragon (dibuat, dipakai, SEMUA tabel dibangun dari migrasi repo; tak menyentuh DB lain). Data sintetis: 300k entri/900k baris jurnal, 200k SO, 500k mutasi stok, 500k baris biaya, 200k SJ, 200k pembayaran+alokasi, 1M audit, 20k transfer (`f4_seed.sql`). Waktu = wall-clock klien `mysql.exe` (lantai ~100 ms karena start proses; angka <=110 ms = "praktis < 10 ms"). Skrip: `scratchpad/f4_bench.sh` (sebelum), `f4_after.sql` + `f4_bench_after.sh` (sesudah). Hasil ringkas di Lampiran D. Database `zz_schema_probe` sudah saya DROP setelah selesai (skrip membuat ulang dari migrasi repo; tak ada database lain disentuh).

### 1.3 Hasil probe (ringkas; detail Lampiran D)

| Query (bentuk dari kode) | Sebelum | Sesudah rancangan |
|---|---|---|
| `AccountFootings` seumur hidup (`journals.go:308`) | **17.033 ms** (900k baris) | **~105 ms** (rollup) |
| `CostPositions` (`ledger.go:116`) | 3.207 ms (500k) | 176 ms (tabel posisi) |
| `MaxCostMovementID` (`ledger.go:42`) | 2.983 ms (full scan) | 100 ms |
| `PartyFootings` AR (`journals.go:410`) | 3.206 ms | 181 ms |
| `DailyFootings` 1 tahun (`daily.go`) | 2.422 ms | 132 ms |
| `ProductFootings` (`journals.go:355`) | 1.935 ms | 143 ms |
| `PostedSourceIDs` (`journals.go:474`) | 1.456 ms (seluruh id) | 104 ms (anti-join 200 id) |
| `ListByParty` SO (`repository.go:280`) | 762 ms | 132 ms |
| `WaitingReturn` (`delivery/repository.go:202`) | 642 ms | 103 ms |
| audit daftar `(branch OR NULL)` | 543 ms | 102 ms (UNION ALL) |
| `COUNT` audit + filter aksi | 1.125 ms | 632 ms (butuh indeks `(branch_id, action_key, id)`, lihat F4-9) |
| pencarian SO `LIKE %x%` + COUNT | 288 + 748 ms | 226 + 212 ms |

Ukuran (probe, 900k baris jurnal): tabel baris jurnal 164 MB (63,6 data + 100,3 indeks) -> **83 MB** (48,6 + 34,1) dengan INT UNSIGNED + 2 indeks sekunder + CHECK; `stock_movements` 134 MB -> 123 MB WALAUPUN ditambah `balance_before/after`, `moved_at`, dan kunci unik idempotensi.

---

## 2. Audit schema — temuan (A)

> Peta schema akhir ringkas: Lampiran A. Setiap temuan: bukti -> rekomendasi -> tugas. "Sah" = denormalisasi yang memang snapshot dokumen.

| # | Temuan | Bukti | Rekomendasi (tugas) |
|---|---|---|---|
| S-1 | Semua id `BIGINT UNSIGNED` padahal single-company; setiap baris membayar 4 byte ekstra per kolom id dan tiap indeks sekunder menyalinnya (PK ikut di setiap entri indeks) | migrasi 000001-000031; probe ukuran di §1.3 | INT UNSIGNED untuk semua id (maks 4,29 mld; legacy INT signed muat); BIGINT hanya uang, penghitung, `size_bytes` (F4-3) |
| S-2 | `stock_balances` punya `id` surrogate yang tak pernah dipakai + unique (branch, produk, varian, lokasi) = dua struktur untuk satu kunci | `000008:23-26`; grep `stock_balances` di Go: akses SELALU lewat tupel 4 kolom (`repository.go:196,205,243,261,285,353,369,391,416`) | PK komposit `(branch_id, product_id, variant_id, location_id)`, hapus id + unique (F4-4) |
| S-3 | `branch_document_sequences`, `finance_document_sequences` punya `id` tak terpakai + unique | `000003:25`, `000016:35`; `NextNumber` hanya `WHERE branch_id=? AND doc_kind=?` | PK `(branch_id, doc_kind)` / `(branch_id, seq_key)` (F4-3) |
| S-4 | `stock_movements`: tak ada indeks yang diawali `product_id`; kolom `direction`/`movement_type`/`ref_type` VARCHAR lebar tanpa CHECK; `ref_type VARCHAR(50) DEFAULT ''` (string kosong sebagai "tak ada" padahal `ref_id` NULL) | `000008:36-50`; `HasMovementsAnywhere` `repository.go:480` (`WHERE product_id = ?` = pemindaian penuh indeks `lookup` karena kolom depannya `branch_id`; di probe 500k baris masih ~lantai 100 ms saat buffer hangat, tetapi biayanya linier terhadap seluruh riwayat dan dipakai guard edit produk + F3b `ProductStockState`); F3b meminta `(product_id, variant_id)` juga | F4-4 |
| S-5 | `finance_journal_lines`: 4 indeks sekunder, **`ix_finance_journal_lines_account (account_id)` (000016) adalah PREFIX dari `ix_finance_lines_account_product`/`_party` (000019)** = redundan; total ~111 B/baris indeks (probe) | `000016:56`, `000019:29-30` | ganti dengan `(entry_id)` (FK) + `(account_id, entry_id)`; laporan dari rollup (F4-6) |
| S-6 | Indeks daftar dokumen tumpang tindih setelah 000031: `ix_sales_orders_branch (branch_id,status)` dan `(branch_id,id)`; F2a menambah `(sales_order_id,status,kind)` yang membuat `ix_delivery_notes_so (sales_order_id)` redundan; sama untuk GR/retur | `000010:27`, `000031:23-31`; F2a-1/2/3 | satu set indeks per tabel (F4-5, F4-8) |
| S-7 | Tak ada CHECK invarian: baris jurnal bisa debit=kredit=0 atau dua sisi sekaligus; qty movement <= 0; direction/amount liar. (debit=kredit PER ENTRI tak bisa CHECK baris; dijaga `CreateEntry` `journals.go:22-27` + `CountUnbalanced`) | `000016:51-58`; `posting.go:140-143` hanya di aplikasi | CHECK baris (F2b M-B), qty>0, `reserved>=0`, direction (F4-3) |
| S-8 | `finance_inventory_cost_movements`: `CostPositions` menjumlah SELURUH ledger cabang tiap panggilan `averageCost` (per baris dokumen!) | `costing.go:15,48`, `reports.go:471,492`, `ledger.go:116`; probe 3,2 s | tabel posisi (F2b M-E) + kursor, F4-6 |
| S-9 | Write-only: `finance_tax_report_snapshots` (tak ada pembaca), `finance_inventory_cost_movements.unit_cost`, `assistant_tool_executions` | §1.2 | pertahankan (fitur "beku laporan pajak" legacy; jangan hapus tanpa keputusan), tandai; retensi untuk assistant (F4-10); PERTANYAAN #3 |
| S-10 | `audit_logs` tumbuh tanpa batas; daftar `OR branch_id IS NULL` + `ORDER BY id` memaksa PK scan 543 ms; `COUNT` + filter aksi 1,1 s; `note VARCHAR(255)` tanpa before/after | `audit/infrastructure/repository.go:30-60`; `000017:16-17` | F4-9 |
| S-11 | `stock_reservations`: tak ada indeks sapuan `(status, expires_at)`, tak ada retensi (baris `expired/consumed/released` menumpuk), indeks pencarian `(branch,product,variant,status)` ikut membengkak | `000008:76-77`; `repository.go:301-340` | F4-4, F4-10 |
| S-12 | `user_sessions`: sesi kedaluwarsa/dicabut tak pernah dihapus; tak ada indeks `expires_at`/`revoked_at` | `000002` | F4-10 |
| S-13 | `finance_periods` (id) dan `finance_tax_periods` bentuk kembar | `000016:23-30,63-71` | SAH: dua kunci berbeda (akuntansi vs pajak); jangan digabung |
| S-14 | Denormalisasi: `payments.party_type`, `sales_orders.member_code`, item order `product_code/name/uom` | `000013:13`, `000010:13`, `000010:34-37` | SAH (snapshot dokumen / KI nota tak bergeser). Tak ada denormalisasi tanpa alasan yang ditemukan |
| S-15 | `DATETIME(6)` di semua kolom waktu (8 byte); `created_at`+`updated_at` di tiap tabel | semua migrasi | pertahankan. DATETIME(0) hemat 3 B/kolom (<3% total) dan MySQL MEMBULATKAN pecahan (detik berikutnya) — tidak sepadan. TIMESTAMP ditolak (batas 2038, konversi zona sesi) |
| S-16 | Charset/collation tak ditulis di migrasi (mewarisi default database). MySQL 8.4 default `utf8mb4_0900_ai_ci` (terverifikasi di Laragon) — case/accent-insensitive: unik `username/email/code` tak membedakan huruf besar kecil (sama dengan legacy MySQL ber-collation CI) | `SHOW VARIABLES`; migrasi tanpa `CHARSET` | tulis eksplisit `DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci` per tabel di baseline agar tak bergantung pada server; hash/kunci teknis `CHARACTER SET ascii` (F4-3) |
| S-17 | `entry_date` jurnal bertipe DATETIME(6) tetapi SELALU berisi tanggal kalender WIB tengah malam (`normalizeEntryDate` `posting.go:48-58`, `DeliveryDate` string `YYYY-MM-DD`, `PaidAt` idem `payment/service.go:281-286`) | kode dibaca | `DATE NOT NULL` (3 B; invarian tak bisa dilanggar; rollup `day = entry_date`). Output API tetap `"2026-01-05 00:00:00"` karena repo memformat `time.Time` (F4-3). **Catatan lintas agen:** F2a mengubah `order_date` menjadi instan UTC berjam-jam; builder jurnal wajib mengonversi ke tanggal WIB sebelum `entry_date` |
| S-18 | Qty/faktor `DECIMAL(24,12)` (12 B) | `000006/8/9`; legacy `039_expand_quantity_precision` | PERTAHANKAN (legacy memperlebarnya karena kebutuhan nyata). Risiko kode: qty `float64` + `on_hand + ?` (`repository.go:243`) dapat meninggalkan sisa 1e-12 -> posisi hantu pada `on_hand <> 0` (`repository.go:187,369,391`); mitigasi di F4-4 (toleransi epsilon di filter atau kolom dibulatkan 12 dp sudah otomatis oleh DECIMAL, risiko ada pada akumulasi float di Go) |
| S-19 | FK intra-modul: semua ada pada pasangan header-baris dan `stock_*`->`stock_locations` | migrasi | PERTAHANKAN. Biaya: kunci S pada baris induk saat insert (aman: induk baru dibuat di tx yang sama). `fk_stock_movements_location` menambah kunci S pada baris lokasi untuk tiap gerakan; di READ COMMITTED (F4-12) tak menimbulkan gap lock — tetap dipertahankan demi integritas |

---

## 3. Tugas

### F4-1 — Squash: baseline per-modul + pembanding information_schema (ukuran L)

**Masalah.** `apps/api/migrations` = 27 migrasi bernomor 1-25, 30, 31 (celah 26-29), dan 000024 menyentuh dua modul (`sales_returns` + `delivery_notes`), 000020 menyentuh 4 tabel di 2 modul, 000031 tersebar di 7 modul — melanggar "satu migrasi = satu modul" (BACKEND_GUIDE). Agen lain menambah blok `000032-000039` (F3b), `000040-000043` (F2a), M-A..M-H (F2b), tiga migrasi kecil F1/F6, `0000NN` F3a. Pemilik mengunci: di akhir fase schema semuanya jadi satu baseline bersih; ETL dan produksi mulai dari DB kosong.

**Keputusan rancangan (selaras F8-18 §A, kritis untuk sqlc):** baseline = **18 berkas, satu per modul bertabel**, bernomor sama dengan 18 migrasi pertama yang sudah ada (jadi tak ada gagasan baru, hanya "isi final per modul"):

| Berkas baseline | Modul | Tabel (akhir, per Lampiran A) | Berasal dari (lama) |
|---|---|---|---|
| `000001_user` | user | users, roles, permissions, role_permissions, user_roles, user_branch_access, authz_seed_applied | 000001 + F3b 32,33 + F1 |
| `000002_auth` | auth | user_sessions | 000002 + F3b 32 + F1-10 |
| `000003_branch` | branch | branches, branch_document_sequences | 000003 + F3b 38 |
| `000004_company` | company | company_profile, company_settings | 000004 |
| `000005_media` | media | media_files | 000005 + F3b 36 + F1-9 |
| `000006_product` | product | product_categories, products, product_variants | 000006 + F3b 35 |
| `000007_party` | party | member_types, parties, party_delivery_addresses | 000007 + F3b 37 |
| `000008_stock` | stock | stock_locations, stock_balances, stock_movements, stock_transfers(+items), stock_reservations, stock_adjustments | 000008 + F2a 40 + 31 (idx) |
| `000009_purchasing` | purchasing | purchase_orders(+items) | 000009 + 23 + 31 + F2a 41 |
| `000010_sales` | sales | sales_orders(+items) | 000010 + 22 + 23 + 31 + F2a 41 |
| `000011_goodsreceipt` | goodsreceipt | goods_receipts(+items) | 000011 + 31 + F2a 41 |
| `000012_delivery` | delivery | delivery_notes(+items) | 000012 + 21 + 24(bagian SJ) + 31 + F2a 41 |
| `000013_payment` | payment | payments, payment_allocations | 000013 + 31 + F2a 42 |
| `000014_salesreturn` | salesreturn | sales_returns(+items, replacement_items, settlements) | 000014 + 20 + 24 + 25 + 31 + F2a 43 |
| `000015_purchasereturn` | purchasereturn | purchase_returns(+items, settlements) | 000015 + 20 + 25 + 31 + F2a 43 |
| `000016_finance` | finance | accounts, mappings, periods, sequences, journal_entries/lines, tax_periods, tax_snapshots, business_expenses, cost_movements, **cost_positions, cost_state, cutover, account_daily, party_daily, product_daily** | 000016 + 19 + 30 + 31 + F2b M-A..M-H + F4-6 |
| `000017_audit` | audit | audit_logs, audit_log_details | 000017 + F3b 39 (diganti F4-9) |
| `000018_assistant` | assistant | 7 tabel assistant | 000018 |

Migrasi sesudah baseline mulai dari `000019_<modul>_<perubahan>`. Tidak ada FK lintas modul sehingga urutan 1..18 bebas. Migrasi yang berisi DATA (INSERT/UPDATE: F3a `perm_member_types_view`, F3b `UPDATE member_types SET rounding_mode`, `UPDATE stock_locations SET kind='damaged'`) **tidak ikut baseline**: DB kosong tak punya baris untuk di-UPDATE; INSERT permission/role dipindah ke `EnsurePermissions`/seed idempoten (sudah pola `cmd/seed`; terbukti migrasi lama nol `INSERT`).

**Prosedur kesetaraan terbukti (jalankan di CI dan lokal; pakai MySQL sementara, bukan DB pemilik):**
1. **Beku.** Semua PR schema fase 2-4 merged; `git tag schema-freeze`. Kriteria: nol migrasi tertunda di semua `plan-*.md`, `golang-migrate` rantai lama bisa `up` dari kosong ke versi terakhir, tes integrasi hijau.
2. **Snapshot lama S_old.** `CREATE DATABASE zz_squash_old`; terapkan SELURUH rantai lama berurutan (`migrate up`); jalankan `scripts/schema-fingerprint.mjs zz_squash_old > S_old.txt` (kueri di Lampiran C: kolom, indeks, FK, CHECK, kolom generated, engine/collation/row_format; **tanpa** `ordinal_position`, `AUTO_INCREMENT`, nama FK bila di-auto).
3. **Tulis baseline TAHAP 1 (squash murni).** Dari `SHOW CREATE TABLE` tiap tabel di `zz_squash_old`, kelompokkan per modul sesuai tabel di atas, urutkan tabel (FK intra-modul: induk dulu), tulis `*.up.sql` + `*.down.sql` (`SET FOREIGN_KEY_CHECKS=0; DROP TABLE IF EXISTS ... ; SET FOREIGN_KEY_CHECKS=1`). Komentar migrasi lama yang masih berguna (alasan indeks) dipindah sebagai komentar kolom/blok.
4. **Bukti TAHAP 1.** `zz_squash_new` <- baseline; `S_new.txt`; `diff S_old.txt S_new.txt` HARUS kosong. Lalu `migrate up`, `migrate down` (semua tabel hilang), `migrate up` lagi lewat CLI (bukan loader tes).
5. **TAHAP 2 (edit disetujui: F4-3 + sisa F4-2).** Terapkan edit pada berkas baseline; jalankan ulang; `diff` HARUS SAMA PERSIS dengan `scripts/schema-expected-diff.txt` (manifest edit: tiap baris = satu perubahan yang disetujui, mis. `columns|users|id|bigint unsigned -> int unsigned`). Perubahan tak terdaftar = gagal. Tahap 1 dan 2 dipisah commit agar review bisa memverifikasi tahap 1 nol-diff.
6. **Regresi.** `TESTDB_REBUILD=1 go test ./internal/integration/...` hijau; `sqlc generate` per modul sukses (F8-18); `scripts/arch-check.mjs` hijau (F8 Rule T `table-owners.json` 1:1 dengan `CREATE TABLE`).
7. **Hapus** berkas lama `000019..000031` + blok F2a/F3b/F2b setelah tahap 5-6 hijau (git history menyimpan). Perbarui `docs/DB_SCHEMA.md` (jumlah tabel), `knowledge/DATABASE_GUIDE.md`, `CLAUDE.md` ("27 migrasi", celah 26-29 hilang).

**golang-migrate / dirty / versi.** Produksi dan staging mulai DB kosong -> `migrate up` mencatat versi **18**, `dirty=false`. DB dev lama (versi 31/43) tidak kompatibel: `migrate` akan gagal "no migration found for version 31". Prosedur resmi: `DROP DATABASE` + buat ulang (jangan `migrate force 18` kecuali schema sudah dibuktikan identik lewat fingerprint). Pagar F6-3 (`schema_migrations` vs versi maksimum yang disematkan) otomatis menyesuaikan karena versi maksimum = 18 (+ migrasi susulan). `scripts/migrate.mjs` tak perlu diubah (membaca `.env`, memanggil CLI, `-path migrations`); `sqlc.yaml` dibuat ulang per modul dengan daftar berkas (F8-18: `schema: [migrations/000008_stock.up.sql]`). `internal/integration/integration_test.go:152-186` membaca `../../migrations/*.up.sql` terurut -> bekerja tanpa perubahan; yang perlu: deteksi schema usang (F0-1/F8-1 `_it_schema(hash)`), karena cabang "reusing migrated test db" (`:158-163`) akan diam-diam memakai schema lama pasca-squash.

**Seed.** Tak ada seed di migrasi (sudah dibuktikan). COA/pemetaan = `EnsureChart` (F2b), permission = `EnsurePermissionCatalog`, role/cabang/admin = `cmd/seed` (dev) atau ETL (produksi, F5). Jadi squash tak memengaruhi seed; DB kosong + `cmd/seed` harus menghasilkan aplikasi yang bisa login (cek di tes smoke F6).

**Dampak API/UI:** tidak ada. **Risiko:** (1) sesi dev lama harus rebuild; (2) edit tahap 2 yang lupa masuk manifest = gagal CI (disengaja); (3) F5 `expectedFingerprint` memakai hash `S_new` tahap 2 (Lampiran C menghasilkan sha256). **Dependensi:** semua agen schema beku; F4-3. **Memblokir:** F5-4..8 (peta kolom target), F8-18, F8-1 (hash).

---

### F4-2 — Pelebaran kolom agar data legacy muat (ukuran M)

**Masalah.** Legacy punya lebar yang lebih besar dari revamp pada kolom yang dimigrasi; F5 mengakui "potong + log" untuk `notes` dan `ship_to_address`, dan `branches.code 10` membuat 40-char kode legacy ditolak. Karena produksi mulai kosong lalu ETL, pelebaran harus ada di schema SEBELUM rehearsal ETL (supaya tes F5 "tidak ada pemotongan" bermakna), cukup `MODIFY` tanpa data.

**Tabel lebar akhir (sumber: entity legacy `*.entity.ts`, bukti baris di kolom 3; pemilik migrasi = agen yang sudah merencanakannya; sisanya F4 lewat migrasi dev per modul `000050+`. Pelebaran VARCHAR dalam kelas panjang-byte yang sama (<=255 byte) bersifat in-place; yang melewati 255 byte (63 -> 64+ karakter utf8mb4) menyalin tabel — tak masalah di DB dev kecil):**

| Tabel.kolom | Revamp sekarang | Legacy | Target baseline | Pemilik migrasi |
|---|---|---|---|---|
| users.username | VARCHAR(50) | 100 (`user.entity.ts`) | **100** | F3b `000032` |
| users.full_name | 150 | 255 | **255** | F3b |
| users.phone / display_title | (tak ada) | 30 / 100 | 30 / 100 | F3b |
| branches.code | 10 | 50 | **50** | F3b `000038` |
| branches.name | 150 | 255 | **255** | F3b |
| branches.address | VARCHAR(255) | TEXT | **TEXT** | F3b |
| branches.city | 100 | 120 | **120** | F3b |
| company_profile.name / legal_name | 150 | 255 | **255** | F4 (company, migrasi `000050+`) |
| media_files.file_key | 255 | object_key 700 | **700** (`uq` pada 700 x4 B = 2800 B < 3072: aman; atau awalan unik 255 + hash — lihat catatan) | F4 (media) |
| media_files.mime | 100 | 120 | **120** | F4 |
| product_categories.name | 150 | 255 | **255** | F3b `000035` |
| product_variants.code / name | 50 / 150 | 80 / 255 | **80 / 255** | F3b |
| product_variants.barcode | 100 | 80 | 100 (cukup) | — |
| stock_locations.code / name | 30 / 150 | 50 / 255 | **50 / 255** | F4 (stock) |
| stock_transfers.number | 60 | 80 | **100** | F4 |
| semua `number` dokumen (SO, PO, SJ, payment, retur, transfer, GR, expense) | 60 | order 100, SJ 100, payment 100, return 100 | **100** (nomor 80 char bila kode cabang 50: `ORD-<50>/PB/2026/09/00001`) | F4 per modul |
| sales/purchase_orders.notes | VARCHAR(255) | TEXT | **TEXT** | F2a `000041` |
| sales_orders.tax_rate, *_items.discount_pct, sales/purchase_returns.tax_rate, *_return_items.discount_pct | DECIMAL(5,2) | (9,4) | **DECIMAL(9,4)** | F4 (sales, purchasing, salesreturn, purchasereturn) |
| sales_orders.tax_invoice_number / purchase_orders.supplier_invoice_number | 50 | 100 | **100** | F4 |
| sales_orders.ship_to_address | 255 | TEXT | **TEXT** | F4 (sales) |
| sales_orders.ship_to_label | 100 | 120 | **120** | F4 |
| goods_receipts.notes, delivery_notes.notes, payments.notes, sales/purchase_returns.notes, *_settlements.notes, stock_transfers.notes, stock_transfer_items.notes, business_expenses.notes | 255 | TEXT | **TEXT** (return `reason` juga TEXT; F2a menulis 255 -> naikkan) | F4 |
| stock_movements.notes | 255 | reason_text TEXT | **VARCHAR(500)** (movement = tabel terbesar; F5 memotong+log) | F4 (stock) |
| stock_movements.ref_type | VARCHAR(50) | reference_type 30 | **VARCHAR(30) ascii NOT NULL** (nilai terpanjang `purchase_return_cancel`=22) | F4 |
| goods_receipts.supplier_delivery_number | (F2a 60) | 100 | **100** | F2a `000041` (naikkan) |
| payments.reference_number | (F2a 100) | 100 | 100 | F2a `000042` |
| finance_accounts.code / name | 20 / 150 | 50 / 255 | **50 / 255** | F2b/F4 (finance) |
| finance_journal_entries.number | 60 | 120 | **120** | F4 |
| finance_journal_entries.memo / lines.description | 255 / 255 | 500 / 500 | **500 / 500** | F4 |
| audit_logs.note | 255 | (before/after JSON; note 500 di F3b) | **500** + `audit_log_details` (F4-9) | F4 |
| order item catatan (`line_note`) | (F2a VARCHAR(255)) | TEXT | **TEXT** | F2a `000041` (naikkan) |
| `status` kolom master | VARCHAR(20) | 30 | tetap 20 (nilai dari kode, terpendek-terpanjang `not_required`=12) | — |

Catatan `media_files.file_key`: `UNIQUE` pada VARCHAR(700) utf8mb4 = 2800 B (batas indeks InnoDB 3072 B) -> muat, tetapi entri indeks besar. Alternatif tak memerlukan perubahan perilaku: `file_key VARCHAR(700) CHARACTER SET ascii` bila F5 membuktikan semua key legacy ASCII (kunci S3 hasil `buildKey`); bila tidak, utf8mb4. Keputusan di F5 (Q-LEN) — ditandai Pertanyaan #4.
Migrasi data tidak diperlukan (kolom hanya melebar). Pra-cek tak perlu: DB dev kecil.

**Dampak API/UI:** tidak ada selain batas validasi aplikasi yang harus dinaikkan agar sejajar (mis. validator `branch.code` max 10 -> 50 di `branch/application`, `username` max 100; tes `validator`). Daftar titik validasi yang mengeraskan panjang lama: grep `len(` pada service/handler modul user/branch/stock/finance (F3b mencatat sebagiannya).
**Test:** tes integrasi "insert baris selebar maksimum legacy" per tabel (tabel di atas dijadikan data tes) + F5-6 "tidak ada pemotongan"; pembanding schema (F4-1) memuat lebar baru.
**Estimasi dampak:** nol biaya ukuran (VARCHAR menyimpan panjang aktual); `DECIMAL(5,2)->(9,4)` = 3 B -> 5 B/kolom (item SO/PO 600k x 2 kolom = ~2 MB). **Dependensi:** F3b/F2a menulis sebagian; F5 tak menunggu. **Ukuran:** M.

---

### F4-3 — Tipe & ukuran baris (diterapkan sebagai edit baseline, TAHAP 2 squash) (ukuran M)

**Rancangan (semua tanpa mengubah perilaku API/UI):**
1. **Semua kolom id/FK-primitif `BIGINT UNSIGNED` -> `INT UNSIGNED`** (PK, `*_id`, `created_by/updated_by`, `ref_id`, `source_doc_id`, ...). Tetap BIGINT: uang (`amount`, `debit`, `credit`, harga), penghitung (`current_value`), `media_files.size_bytes`. Alasan: single-company, volume 3 th <= puluhan juta baris; 4,29 mld id; hemat 4 B per kolom id **dan per entri indeks sekunder** (PK ikut disalin). Terukur di probe (900k baris jurnal): 164 MB -> 83 MB. Legacy id = INT signed -> muat. F5 (ID eksplisit) -> muat. Tipe tunggal di semua tabel (tak ada konversi tersirat pada join intra-modul).
   - Go: `database/sql` memindai ke `int64` tanpa perubahan; `LastInsertId` sama. sqlc (F8-18) butuh override `db_type: "int", unsigned: true -> int64` selain override `bigint unsigned` yang sudah diuji F8 (satu baris config).
   - Tak berlaku: `assistant_channel/assistant_config.id TINYINT` (singleton).
2. **PK komposit tanpa id surrogate** untuk tabel yang selalu diakses lewat tupel: `stock_balances(branch_id, product_id, variant_id, location_id)` (hapus `id` + `uq_stock_balances`), `branch_document_sequences(branch_id, doc_kind)`, `finance_document_sequences(branch_id, seq_key)`. Bukti `id` tak terpakai: grep §2 S-2/S-3. Hemat 1 indeks + 4-8 B/baris; `ProductAvailability`/`SuggestLocations` membaca rentang clustered.
3. **ENUM hanya untuk himpunan tertutup di tabel terbesar:** `stock_movements.direction ENUM('in','out')`, `finance_inventory_cost_movements.direction ENUM('in','out')`. Selain itu `VARCHAR` + (tidak ada CHECK status) agar mesin status bisa berubah lewat migrasi kode tanpa ALTER ENUM. Go memindai ENUM sebagai string.
4. **CHECK invarian (MySQL 8.0.16+, ditegakkan; terbukti di probe: pelanggaran -> error 3819):**
   - `finance_journal_lines`: `chk_finance_lines_one_side` (`debit>=0 AND credit>=0 AND ((debit>0 AND credit=0) OR (credit>0 AND debit=0))`) — selaras F2b M-B. **PERHATIAN:** kolom debit/credit tetap `BIGINT` bertanda; JANGAN `UNSIGNED` (probe: `CAST(5 AS UNSIGNED)-CAST(7 AS UNSIGNED)` -> ERROR 1690; `l.credit - l.debit` dipakai di `turnover.go:44`).
   - `stock_movements`: `chk_mv_qty (qty_base > 0)`; `stock_balances`: `chk_sb_reserved (reserved >= 0)` (TIDAK untuk `on_hand`: saldo legacy bisa negatif; keputusan F5); `stock_reservations`: `qty_remaining >= 0 AND qty_remaining <= qty_total`.
   - `payments`: `amount > 0`; `payment_allocations`: `amount > 0`; item dokumen: `qty > 0`; `*_total >= 0` pada header.
   - Tidak dibuat: `debit=kredit per entri` (tak bisa CHECK baris). Pengganti sah: `CreateEntry` (`journals.go:22-27`, sudah menolak) + `VerifyRollups`/`CountUnbalanced` pada gerbang tutup periode (F2b) + tes properti.
5. **`finance_journal_entries.entry_date` -> `DATE NOT NULL`** (S-17). `rollup.day = entry_date` tanpa fungsi. Format keluaran TIDAK berubah: driver (parseTime=true) memindai DATE maupun DATETIME sebagai `time.Time`; `ListEntries` memformat `"2006-01-02 15:04:05"` (`journals.go:221-267`) dan `GetEntry` (`journals.go:76`) memindai ke string (RFC3339) persis seperti sekarang — keduanya sama untuk DATE dan DATETIME. Filter `from+" 00:00:00"`/`to+" 23:59:59"` tetap benar (diverifikasi: EXPLAIN memakai PK, hasil sama, probe `c1=c2=1095`). Kontrak baru yang wajib ditaati F2b: builder jurnal mengirim `YYYY-MM-DD` hasil konversi WIB.
6. **Charset/collation eksplisit** di setiap `CREATE TABLE`: `ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci ROW_FORMAT=DYNAMIC`. Kolom teknis ASCII: `user_sessions.refresh_hash/prev_refresh_hash CHAR(64) CHARACTER SET ascii`, `*.client_ref VARCHAR(64) CHARACTER SET ascii`, `ref_type`, `source_doc_type`, `doc_kind/seq_key`. (utf8mb4 mereservasi 4 B/char pada buffer sort/temp-table; ASCII 1 B.)
7. **Integer kecil:** `finance_periods.year SMALLINT UNSIGNED, month TINYINT UNSIGNED` (juga `tax_periods`), `sort_order SMALLINT`. Remeh tetapi gratis di baseline.
8. **Yang SENGAJA tidak diubah:** `DECIMAL(24,12)` qty/faktor, `DATETIME(6)`, `VARCHAR` status (S-15, S-18), FK intra-modul (S-19), `JSON` snapshot pajak.

**Dampak API/UI:** nol. **Test:** pembanding schema tahap 2 (manifest), tes integrasi penuh pada baseline, tes CHECK per invarian (insert melanggar -> `Error 3819`; `dberr.Map` diperluas: 3819 -> `apperror.Validation` "Data tidak memenuhi aturan"), tes ENUM invalid. **Estimasi:** -24% ukuran DB (EST §1.1), indeks lebih ringkas -> lebih banyak working set di buffer pool. **Risiko:** (a) lupa ubah satu kolom referensi -> join tak memakai indeks: manifest + grep otomatis "semua kolom `*_id` bertipe sama"; (b) overflow INT UNSIGNED = tidak realistis (4,29 mld). **Ukuran:** M.

---

### F4-4 — Modul `stock`: indeks, kolom, retensi, daftar mutasi (ukuran M)

**Masalah + bukti.**
- `ListMovements/CountMovements` (`stock/infrastructure/repository.go:506,575`): filter tanggal pada `created_at`, `ORDER BY id DESC` -> tak ada indeks yang melayani "cabang + urut id"/"cabang + tanggal + urut"; `(branch_id, created_at)` + sort id = filesort pada rentang. Plus `COUNT(*)` tiap halaman.
- `HasMovementsAnywhere` (`:480`) `WHERE product_id = ?` (kolom depan indeks = branch_id) — dipakai guard edit produk dan F3b `ProductStockState`.
- `ListTransfers/CountTransfers` (`:740,769`) `(from_branch_id = ? OR to_branch_id = ?)` tanpa indeks sama sekali (probe 99/106 ms pada 20k baris, linier).
- `ListCostMovements` (`:440`, dipakai `Sync` tiap `Post` bertipe SJ/retur) `WHERE branch_id = ? AND id > ? ORDER BY id` -> membaca PK dari `id > cursor` menyaring cabang.
- `stock_reservations`: sapuan kedaluwarsa (F2a-12: `WHERE status='active' AND expires_at <= NOW`) tanpa indeks; baris `expired/consumed/released` menumpuk (400k EST) dan ikut di indeks pencarian `ExpireReservationsTx` (`repository.go:305`).
- `stock_balances`: F3b butuh `SUM(on_hand) WHERE product_id=?` lintas cabang.
- `BalancesByLocation` `WHERE location_id = ? AND (on_hand <> 0 ...)`: dilayani indeks FK `location_id`.

**DDL (baseline 000008_stock; ikut kolom/kebutuhan F2a, F3b):**
```sql
CREATE TABLE stock_locations (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT, branch_id INT UNSIGNED NOT NULL,
  code VARCHAR(50) NOT NULL, name VARCHAR(255) NOT NULL, parent_id INT UNSIGNED NULL,
  kind VARCHAR(12) NOT NULL DEFAULT 'normal',          -- F2a-9: 'normal'|'damaged'
  is_system TINYINT(1) NOT NULL DEFAULT 0, status VARCHAR(20) NOT NULL DEFAULT 'active',
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  created_by INT UNSIGNED NULL, updated_by INT UNSIGNED NULL,
  PRIMARY KEY (id), UNIQUE KEY uq_stock_locations (branch_id, code),
  KEY ix_stock_locations_parent (parent_id),
  CONSTRAINT fk_stock_locations_parent FOREIGN KEY (parent_id) REFERENCES stock_locations (id));

CREATE TABLE stock_balances (
  branch_id INT UNSIGNED NOT NULL, product_id INT UNSIGNED NOT NULL, variant_id INT UNSIGNED NOT NULL,
  location_id INT UNSIGNED NOT NULL,
  on_hand DECIMAL(24,12) NOT NULL DEFAULT 0, reserved DECIMAL(24,12) NOT NULL DEFAULT 0,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (branch_id, product_id, variant_id, location_id),
  KEY ix_stock_balances_location (location_id),
  KEY ix_stock_balances_product (product_id, variant_id),        -- F3b-000034
  CONSTRAINT fk_stock_balances_location FOREIGN KEY (location_id) REFERENCES stock_locations (id),
  CONSTRAINT chk_sb_reserved CHECK (reserved >= 0));

CREATE TABLE stock_movements (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT, branch_id INT UNSIGNED NOT NULL,
  product_id INT UNSIGNED NOT NULL, variant_id INT UNSIGNED NOT NULL, location_id INT UNSIGNED NOT NULL,
  direction ENUM('in','out') NOT NULL, movement_type VARCHAR(20) NOT NULL,
  qty_base DECIMAL(24,12) NOT NULL,
  balance_before DECIMAL(24,12) NULL, balance_after DECIMAL(24,12) NULL,     -- F2a-0 (NULL = riwayat lama)
  ref_type VARCHAR(30) CHARACTER SET ascii NOT NULL DEFAULT '', ref_id INT UNSIGNED NULL,
  notes VARCHAR(500) NULL, unit_cost DECIMAL(18,4) NULL,                       -- unit_cost: F2b (saldo awal berbiaya)
  created_by INT UNSIGNED NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  moved_at   DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),                 -- waktu efektif (F2a-0)
  PRIMARY KEY (id),
  UNIQUE KEY uq_stock_movements_ref (ref_type, ref_id, product_id, variant_id, location_id, direction),  -- idempotensi (F2a P1)
  KEY ix_stock_movements_position (branch_id, product_id, variant_id, location_id),
  KEY ix_stock_movements_branch_moved (branch_id, moved_at),
  KEY ix_stock_movements_product (product_id, variant_id),
  CONSTRAINT fk_stock_movements_location FOREIGN KEY (location_id) REFERENCES stock_locations (id),
  CONSTRAINT chk_mv_qty CHECK (qty_base > 0));

-- stock_transfers
KEY ix_stock_transfers_from (from_branch_id, id), KEY ix_stock_transfers_to (to_branch_id, id)
-- stock_transfer_items: + from_location_id INT UNSIGNED NULL, to_location_id INT UNSIGNED NULL  (F2a-5)
-- stock_reservations
KEY ix_stock_reservations_exp (status, expires_at)                              -- sapuan (F2a-12)
-- stock_adjustments (F2a-9): KEY (branch_id, id), KEY (product_id, id)
```
Kunci unik idempotensi: ukuran entri = 30 (ascii) + 4 + 4 + 4 + 4 + 1 + PK 4 ~ 51 B x 1M = ~51 MB (probe `movements_new` memakai kunci yang sama: total tabel 123 MB vs 134 MB lama). Pra-cek duplikat F2a dipertahankan hanya untuk DB dev; baseline kosong.

**Perubahan kode (file:baris):**
1. `ListMovements/CountMovements` (`repository.go:506,575`): kolom tanggal `created_at` -> `moved_at`; `ORDER BY moved_at DESC, id DESC` (dilayani `(branch_id, moved_at)`; tanpa filesort bila tak ada filter produk). Beda tampilan hanya bila ada gerakan ber-backdate (itu tujuan `moved_at`).
2. `HasMovementsAnywhere` -> `SELECT EXISTS(SELECT 1 FROM stock_movements WHERE product_id = ? LIMIT 1)` (selaras F3b).
3. `ListTransfers/CountTransfers`: pecah OR: `SELECT ... WHERE from_branch_id = ? UNION ALL SELECT ... WHERE to_branch_id = ? AND from_branch_id <> ? ORDER BY id DESC LIMIT ? OFFSET ?` (cabang sama/`move-location` tidak dihitung dua kali — probe: UNION ALL 111 ms vs OR).
4. `ListCostMovements`: pakai kursor tabel `finance_cost_state` (F2b) + indeks `PRIMARY` tetap; atau `WHERE id > ? AND branch_id = ?` sudah memakai PK range; tak perlu indeks baru karena `ix_..._branch_moved` bukan untuk ini. Cukup batas `created_at <= UTC_TIMESTAMP(6) - INTERVAL 3 SECOND` (F2a A5) — tidak memengaruhi indeks.
5. Posting batch (F2a-0 `Post`) menggantikan Ensure+Add+Insert (3 statement per posisi + `ExpireReservations` + `GetBalanceTx`): `INSERT INTO stock_balances (...) VALUES (...) ON DUPLICATE KEY UPDATE on_hand = on_hand + VALUES(on_hand), reserved = reserved + VALUES(reserved)` (menggabungkan `EnsureBalanceTx` + `AddBalanceTx`, `repository.go:194-211`; gunakan `VALUES()` bukan row-alias `AS new` agar tetap lolos parser sqlc F8 yang diuji dengan `ON DUPLICATE KEY UPDATE ... VALUES()`); `balance_before/after` dihitung dari baris `GetBalanceTx ... FOR UPDATE` yang sudah dibaca. Per posisi: SELECT FOR UPDATE + UPSERT + INSERT movement = 3 statement (dari ~6).
6. Toleransi epsilon (S-18): filter `on_hand <> 0 OR reserved <> 0` (`repository.go:187,369,391`) -> `ABS(on_hand) > 0.000000000001 OR ABS(reserved) > ...` TIDAK perlu bila akumulasi memakai `DECIMAL` string (disarankan: parameter qty dikirim `strconv.FormatFloat(q,'f',12,64)` agar MySQL melakukan aritmetika DECIMAL, bukan DOUBLE). Satu fungsi `decimalArg(float64) string` di repository menutup seluruh `on_hand + ?`.

**Dampak API/UI:** nol (urutan daftar mutasi sama kecuali backdate). **Test:** integrasi: Post idempoten (F2a), `ListMovements` tanggal+produk, transfer cabang sama tak ganda, `HasMovementsAnywhere`; EXPLAIN harness F4-17 (tidak ada `Using filesort` pada `ListMovements` tanpa filter produk; `type=range` pada `ix_..._branch_moved`). **Estimasi (probe/EST):** daftar transfer ~ linier -> konstan; `HasMovementsAnywhere` konstan; indeks `(product_id, variant_id)` +12 MB/1M baris. **Dependensi:** F2a-0 (kolom), F3b-000034 (indeks produk — selaras). **Ukuran:** M.

---

### F4-5 — `sales_orders`/`purchase_orders`: indeks daftar, pencarian, `client_ref`, `ListByParty` (ukuran M)

**Masalah + bukti.**
- `sales_orders`/`purchase_orders` hanya punya `(branch_id,status)` + `(branch_id,id)` (`000010:27`, `000031:23-24`). Tak ada indeks untuk filter tanggal (`TopProducts` `aggregates.go:48`, `ListSummaries` `repository.go:328-336`, F3a-2 filter tanggal/pihak), untuk `party_id` (`ListByParty` `repository.go:280` = `WHERE branch_id=? AND party_id=? ORDER BY id DESC` **tanpa LIMIT**, dipakai `payment` untuk saldo dan FIFO `payment/application/service.go:391,420,476,487,640,672`): probe 762 ms -> 132 ms dengan `(branch_id, party_id, id)`.
- Pencarian `number LIKE '%w%'` (hanya nomor, `repository.go:221,256`) + `COUNT(*)` tiap halaman: probe 288 + 748 ms -> 226 + 212 ms dengan indeks penutup `(branch_id, number)` + deferred-join.
- `COUNT` dengan filter `status`+`channel`: 284 ms -> 110 ms dengan `(branch_id, channel, status, id)`.
- F2a/F2b menambah `client_ref` (POS) dan `completed_at`.

**DDL (per tabel; baseline 000010_sales; 000009_purchasing tanpa `channel`, `client_ref`):**
```sql
-- sales_orders: kolom baru (F2a): client_ref VARCHAR(64) CHARACTER SET ascii NULL, assigned_user_id INT UNSIGNED NULL,
--   completed_at DATETIME(6) NULL (F2b), notes TEXT, tax_rate DECIMAL(9,4), ship_to_address TEXT ...
UNIQUE KEY uq_sales_orders_number (number),
UNIQUE KEY uq_sales_orders_client_ref (branch_id, client_ref),                 -- NULL bebas (F2a-12)
KEY ix_sales_orders_branch_number (branch_id, number),                         -- pencarian + GetOrderByNumber
KEY ix_sales_orders_branch_id (branch_id, id),                                 -- daftar default
KEY ix_sales_orders_status (branch_id, status, id),                            -- menggantikan (branch_id, status)
KEY ix_sales_orders_channel (branch_id, channel, status, id),
KEY ix_sales_orders_party (branch_id, party_id, id),
KEY ix_sales_orders_date (branch_id, order_date, id)
-- purchase_orders: sama tanpa channel/client_ref.
```
Biaya tulis: 7 indeks sekunder x 200k = ~34 MB, tulis SO ~200/hari — tak berarti. Hapus `ix_sales_orders_branch (branch_id,status)` (digantikan, S-6).

**Perubahan kode:**
1. `List/Count` (`repository.go:206-271`): pencarian memakai deferred-join agar penyortiran hanya pada id: `SELECT cols FROM sales_orders o JOIN (SELECT id FROM sales_orders WHERE branch_id=? [AND status/channel] AND number LIKE ? ... ORDER BY id DESC LIMIT ? OFFSET ?) x ON x.id = o.id ORDER BY o.id DESC`; `Count` memakai kondisi yang sama (indeks penutup, tanpa baca baris). **Semantik pencarian tak berubah (substring, per kata).** Alasan menolak FULLTEXT ngram sekarang: tersedia (`ngram_token_size=2` terverifikasi) tetapi semantik token/stopword berbeda dan tabel < 1 jt baris — biaya sub-detik tanpa itu (probe 226 ms pada 200k, warm); sisakan sebagai opsi bila tabel > 2 jt.
2. `ListByParty` (`repository.go:280`) JANGAN dipanggil tanpa batas: (a) FIFO (`payment/service.go:391,420`) -> kontrak baru `ListOpenByParty(ctx, branchID, partyID, afterID int64, limit int)` urut `id ASC` (keyset) yang berhenti saat jumlah alokasi terpenuhi; (b) saldo (`GetPartyBalance` `:640`) -> agregat SQL di modul sales/purchasing: `PartyTotals(ctx, branchID, partyID) (billed int64, ...)` = `SUM(grand_total) WHERE branch_id=? AND party_id=? AND status IN ('confirmed','completed')` — dan daftar order terbuka dibatasi `limit` + `Paid` batch lewat `PaidForOrders` (sudah batch). Indeks `(branch_id, party_id, id)` melayani keduanya.
3. `StatusCounts` (`aggregates.go:14`) dan `OpenHeaders` (`aggregates.go:78`): memakai `ix_..._status` (index-only / range) tanpa perubahan kode.
4. `CountMissingTaxInvoice` (`aggregates.go:177`): tetap (jalur gerbang tutup periode, per cabang; memakai `ix_status`); bila F2b memindah ke per-periode, tambah `(branch_id, tax_type, status)` hanya bila terukur lambat.

**Dampak API/UI:** nol (pencarian/urutan/total identik). **Test:** integrasi daftar dengan kombinasi filter (status, channel, pencarian dua kata, tanggal, pihak) membandingkan hasil query lama vs baru pada data acak 5k baris; EXPLAIN harness: tak ada `Using filesort` untuk daftar default/status/pihak, `Using index` untuk COUNT tanpa pencarian. **Estimasi (probe):** ListByParty 762 -> 132 ms (-83%); COUNT status+channel 284 -> 110 ms; pencarian 288/748 -> 226/212 ms. **Dependensi:** F2a-12 (client_ref), F3a-2 (filter). **Ukuran:** M.

---

### F4-6 — Finance: rollup saldo (akun/pihak/produk), posisi biaya, indeks lines, kursor (ukuran L; DDL + indeks milik F4, logika milik F2b-14a)

**Masalah + bukti (verifikasi klaim #1 — TERBUKTI, lebih buruk dari klaim).**
- `AccountFootings` (`finance/infrastructure/journals.go:308`) menjumlah seumur hidup via join `lines x entries` (`commercialOnly` `:296` memaksa baca `source_doc_type` tiap entri). Dipanggil tiap muat: `AccountBalance`/`AccountBalances` (`application/service.go:460,497`), `BalanceSheet` (`reports.go:194`) **dua kali** (footing + `profitLoss(1000-01-01..)` `reports.go:223`), `openingFooting` (`reports.go:309`), `TrialBalance`/`TaxSummary` (`reports.go:55,569`), dashboard 7 request paralel. **Probe: 17.033 ms pada 900k baris jurnal** (3 jt baris ~ 55 dtk, EST linier, buffer pool 128 MB; pada RAM besar lebih cepat tetapi tetap linier terhadap umur buku).
- `PartyFootings` (`journals.go:410`) 3,2 s, `ProductFootings` (`:355`) 1,9 s, `DailyFootings` (`daily.go`, `DATE_FORMAT` pada kolom) 2,4 s (probe).
- `CostPositions` (`ledger.go:116`) menjumlah seluruh ledger cabang tiap `averageCost` (`costing.go:15`, dipanggil PER BARIS dokumen di `builders.go:260,392,463`) dan `Sync` (`costing.go:48`); `MaxCostMovementID` (`ledger.go:42`) = `MAX` dengan saringan cabang tanpa indeks (probe 2,98 s pada 500k baris biaya).
- Klaim PERFORMANCE_GUIDE §2 ("dashboard konstan terhadap besar buku") SALAH: `AccountBalances` mengurangi jumlah panggilan, bukan biaya footing; setiap panggilan tetap membaca seluruh sejarah.

**Keputusan:** rollup tabel (F2b-14a menolak cache in-memory: setuju, tak ada state basi). Dua perbedaan dengan draf F2b yang saya usulkan (perlu disepakati; dampak: F2b M-B/M-D berubah):
1. **Tiga rollup berkunci tanggal** (akun, pihak, produk) menggantikan DUA indeks penutup lebar F2b M-B (`(account_id, party_id, entry_id, debit, credit)` ~ 36 B x 3 jt = ~108 MB dan `(account_id, product_id, variant_id, entry_id, debit, credit)` ~ 120 MB, EST) **dan** `finance_party_balances` seumur hidup (M-D), karena party/produk "as-of tanggal lampau" kini dilayani rollup yang sama tanpa pindai lines. Probe: rollup pihak 30k baris/2,5 MB, produk 6k baris, akun 3,6k baris untuk 900k baris jurnal.
2. `finance_journal_lines` hanya punya DUA indeks sekunder: `(entry_id)` (FK) dan `(account_id, entry_id)` (GL per akun / `ListEntries(accountID)`), menggantikan 3 indeks lama (termasuk yang redundan, S-5). Terukur: 100 MB -> 34 MB indeks per 900k baris.

**DDL final (baseline `000016_finance`; nama kolom mengikuti F2b: `day`, `scope`):**
```sql
CREATE TABLE finance_journal_entries (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  number VARCHAR(120) NOT NULL,
  branch_id INT UNSIGNED NOT NULL,
  entry_date DATE NOT NULL,                                  -- tanggal kalender WIB (F4-3 #5)
  memo VARCHAR(500) NOT NULL,
  source_doc_type VARCHAR(30) CHARACTER SET ascii NULL,
  source_doc_id INT UNSIGNED NULL,
  scope TINYINT NOT NULL DEFAULT 0,                          -- 0 komersial, 1 fiskal (F2b M-A); menggantikan commercialOnly
  requeue TINYINT NOT NULL DEFAULT 1,
  status VARCHAR(20) NOT NULL DEFAULT 'posted',
  source_active TINYINT GENERATED ALWAYS AS (CASE WHEN status = 'posted' AND source_doc_type IS NOT NULL THEN 1 END) VIRTUAL,
  reversed_by INT UNSIGNED NULL,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), created_by INT UNSIGNED NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_finance_journal_number (number),
  UNIQUE KEY uq_finance_journal_source (branch_id, source_doc_type, source_doc_id, source_active),  -- F2b M-A; juga melayani PostedAmong
  KEY ix_finance_journal_branch_date (branch_id, entry_date),
  KEY ix_finance_journal_date (entry_date)                   -- TurnoverEntries lintas cabang (turnover.go:44)
);
CREATE TABLE finance_journal_lines (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT, entry_id INT UNSIGNED NOT NULL, account_id INT UNSIGNED NOT NULL,
  product_id INT UNSIGNED NULL, variant_id INT UNSIGNED NULL, party_id INT UNSIGNED NULL,
  debit BIGINT NOT NULL DEFAULT 0, credit BIGINT NOT NULL DEFAULT 0, description VARCHAR(500) NULL,
  PRIMARY KEY (id),
  KEY ix_finance_lines_entry (entry_id),
  KEY ix_finance_lines_account_entry (account_id, entry_id),
  CONSTRAINT fk_finance_lines_entry   FOREIGN KEY (entry_id)   REFERENCES finance_journal_entries (id),
  CONSTRAINT fk_finance_lines_account FOREIGN KEY (account_id) REFERENCES finance_accounts (id),   -- baru, intra-modul
  CONSTRAINT chk_finance_lines_one_side CHECK (debit >= 0 AND credit >= 0 AND ((debit > 0 AND credit = 0) OR (credit > 0 AND debit = 0))));

-- Rollup (semua intra-modul; diperbarui di TRANSAKSI YANG SAMA dengan CreateEntry/ReverseEntry):
CREATE TABLE finance_account_daily (
  branch_id INT UNSIGNED NOT NULL, scope TINYINT NOT NULL, account_id INT UNSIGNED NOT NULL, day DATE NOT NULL,
  debit BIGINT NOT NULL DEFAULT 0, credit BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (branch_id, scope, account_id, day));
CREATE TABLE finance_party_daily (                           -- hanya scope 0
  branch_id INT UNSIGNED NOT NULL, account_id INT UNSIGNED NOT NULL, party_id INT UNSIGNED NOT NULL, day DATE NOT NULL,
  debit BIGINT NOT NULL DEFAULT 0, credit BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (branch_id, account_id, party_id, day));
CREATE TABLE finance_product_daily (                         -- hanya scope 0; variant_id 0 = tanpa varian
  branch_id INT UNSIGNED NOT NULL, account_id INT UNSIGNED NOT NULL, day DATE NOT NULL,
  product_id INT UNSIGNED NOT NULL, variant_id INT UNSIGNED NOT NULL DEFAULT 0,
  debit BIGINT NOT NULL DEFAULT 0, credit BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (branch_id, account_id, day, product_id, variant_id));

-- Posisi biaya + kursor (F2b M-E, nama sama):
CREATE TABLE finance_cost_positions (
  branch_id INT UNSIGNED NOT NULL, product_id INT UNSIGNED NOT NULL, variant_id INT UNSIGNED NOT NULL,
  qty DECIMAL(24,12) NOT NULL DEFAULT 0, total DECIMAL(24,4) NOT NULL DEFAULT 0, last_unit_cost DECIMAL(24,12) NULL,
  estimated_rows INT UNSIGNED NOT NULL DEFAULT 0,            -- tambahan F4: Uncosted tanpa pindai ledger
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (branch_id, product_id, variant_id));
CREATE TABLE finance_cost_state (branch_id INT UNSIGNED NOT NULL, last_movement_id INT UNSIGNED NOT NULL DEFAULT 0,
  updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6), PRIMARY KEY (branch_id));
-- finance_inventory_cost_movements: unit_cost DECIMAL(24,12), total_cost DECIMAL(24,4), ref_type/ref_id (sudah), moved_at (F2b M-F);
--   DROP ix_finance_cost_position (branch_id, product_id, variant_id); ADD ix_cost_estimated (branch_id, is_estimated, product_id, variant_id)
--   KEEP uq_finance_cost_movement (stock_movement_id), ix_cost_moves_ref (ref_type, ref_id); ADD F2b ix_cost_ref_type (branch_id, ref_type, stock_movement_id)
-- finance_cutover (branch_id, doc_type, covered_through_id, set_by, set_at; PK (branch_id, doc_type)) — F2b M-G apa adanya.
```
Syarat konsistensi presisi: `finance_cost_positions.total` HARUS tepat sama dengan SUM `total_cost` ledger pada presisi yang sama (karena itu ledger dinaikkan ke `DECIMAL(24,4)` bersama F2b-11; bila F2b tetap `(18,2)`, posisi mengikuti `(18,2)`).

**Pemeliharaan (di `CreateEntry`, `journals.go:20-65`, setelah insert baris; kunci diurut menaik anti-deadlock; pakai `VALUES()` agar lolos parser sqlc F8):**
```sql
INSERT INTO finance_account_daily (branch_id, scope, account_id, day, debit, credit) VALUES (?,?,?,?,?,?), ...   -- diagregasi per akun di Go, diurut account_id
  ON DUPLICATE KEY UPDATE debit = debit + VALUES(debit), credit = credit + VALUES(credit);
INSERT INTO finance_party_daily  ... (hanya baris ber-party_id, scope 0)   ON DUPLICATE KEY UPDATE ...;
INSERT INTO finance_product_daily ... (hanya baris ber-product_id, scope 0) ON DUPLICATE KEY UPDATE ...;
```
Tiga statement multi-row per entri (bukan per baris). Reversal = entri baru (dimensi disalin, F2b-4) -> jalur sama; `MarkReversed` tak menyentuh rollup. Kontensi: posting operator-driven (`MaxBatch=50`, `queue.go`); bila kelak otomatis per penjualan, baris kas/pendapatan harian menjadi titik panas — mitigasi (tak dibangun sekarang): kolom `slot TINYINT` (= entry_id mod 8) di PK dan `SUM` saat baca.

**Pembaca dialihkan (file:baris -> sumber baru):**

| Fungsi lama | Sumber baru |
|---|---|
| `AccountFootings(from,to)` `journals.go:308` | `SELECT account_id, SUM(debit), SUM(credit) FROM finance_account_daily WHERE branch_id=? AND scope=0 [AND day BETWEEN ? AND ?] GROUP BY account_id` (tanggal = `[:10]` dari `YYYY-MM-DD hh:mm:ss`) |
| `AccountFootingsExcluding` `turnover.go:~75` | rollup penuh dikurangi `footingsOfEntries` (tetap join `lines` per chunk id, kini `e.scope=0` bukan `commercialOnly`) |
| `TaxAdjustmentFootings` `journals.go:~440` | `finance_account_daily WHERE scope=1` |
| `DailyFootings` `daily.go:22` | `SELECT day, account_id, debit, credit FROM finance_account_daily ...` (tanpa `DATE_FORMAT`/`GROUP BY`) |
| `PartyFootings` `journals.go:410` | `finance_party_daily WHERE branch_id=? AND account_id=? [AND day <= ?] GROUP BY party_id` |
| `ProductFootings` `journals.go:355` | `finance_product_daily WHERE branch_id=? AND account_id IN (...) AND day BETWEEN ... GROUP BY product_id, variant_id, account_id` |
| `CostPositions` `ledger.go:116`, `averageCost` `costing.go:15` | `finance_cost_positions` (satu baris per kunci: `PositionFor`) |
| `MaxCostMovementID` `ledger.go:42` | `finance_cost_state.last_movement_id` |
| `EstimatedProducts` `ledger.go:~150` | `finance_cost_positions WHERE branch_id=? AND estimated_rows > 0` |
| `BalanceSheet` (2x seumur hidup) `reports.go:194,223` | satu pembacaan rollup; laba ditahan dihitung dari footing yang sama di Go (hapus panggilan `profitLoss` kedua) |
| `CountUnbalanced` `journals.go:497` | tetap (rentang satu bulan, `(branch_id, entry_date)` + FK `entry_id`; gerbang tutup periode, bukan jalur panas) |
| `TurnoverEntries` `turnover.go:44` | tetap (rentang tahun, lintas cabang); memakai `ix_finance_journal_date`; EST < 1 dtk pada 1 jt entri/3 th (belum diprobe) |

**Verifikasi & pemulihan (F4-17 + F2b-14a #6):** `VerifyRollups(branch)`:
```sql
-- selisih akun (harus nol baris); analog untuk party & product
SELECT x.account_id, x.d, x.c, r.d, r.c FROM
 (SELECT l.account_id, SUM(l.debit) d, SUM(l.credit) c FROM finance_journal_lines l JOIN finance_journal_entries e ON e.id = l.entry_id
   WHERE e.branch_id = ? AND e.scope = 0 GROUP BY l.account_id) x
 LEFT JOIN (SELECT account_id, SUM(debit) d, SUM(credit) c FROM finance_account_daily WHERE branch_id = ? AND scope = 0 GROUP BY account_id) r USING (account_id)
 WHERE r.account_id IS NULL OR x.d <> r.d OR x.c <> r.c;
```
`RebuildRollups(branch)` = hapus-per-cabang + `INSERT ... SELECT ... GROUP BY` (skrip probe `f4_after.sql` menjalankannya untuk 900k baris dalam hitungan detik). ETL (F5) memuat jurnal lalu memanggil `RebuildRollups`; sesudahnya jalur normal. Backfill tak diperlukan di produksi (mulai kosong).

**Dampak API/UI:** NOL (angka sama persis; dijamin `VerifyRollups` + tes ekuivalensi: jalankan N skenario posting/reversal/pajak lalu bandingkan semua laporan query-lama vs rollup pada DB yang sama SEBELUM query lama dihapus). **Estimasi (probe):** `AccountFootings` 17.033 -> ~105 ms; `CostPositions` 3.207 -> 176 ms (tabel posisi 3.000 baris); `MaxCostMovementID` 2.983 -> 100 ms; pihak 3.206 -> 181 ms; harian-1-tahun 2.422 -> 132 ms; produk 1.935 -> 143 ms. Ukuran tambahan rollup EST < 20 MB / 3 th; indeks lines -66%. **Dependensi:** F2b M-A/M-E/M-G (kolom `scope`, posisi, kursor); F4-3 (tipe). **Ukuran:** L.

---

### F4-7 — N+1 jalur baca & tulis (ukuran M)

**Verifikasi klaim #5 — TERBUKTI.**
- `TaxDetail` (`finance/application/reports.go:419-426`): `getSO` memanggil `sales.GetByID` per SO unik (kontrak mengembalikan header + baris = 2 query) — **2 x jumlah SO unik per bulan** (EST 5,5k SO/bulan -> ~11k query; klaim 24k setara ~12k SO/bulan), ditambah `ListEntries(...10000...)` yang MEMOTONG diam-diam kertas kerja SPT di atas 10.000 entri (temuan baru; F2b-9 menggantinya dengan paginasi).
- `payment.Ledger` -> `enrich` per pembayaran (`payment/application/service.go:78,837`): `AllocationsByPayment` (1) + `orderNumber` per alokasi (`GetByID` = 2 query; retur 1-2) + `parties.GetByID` per baris (2) = ~5 x 20 baris = **~100 query per halaman**.
- `priceLines` (`sales/application/service.go:176,192`): per baris `products.GetByID` (produk+kategori+varian ~ 3 query) + `ResolveUOM` (`product/application/service.go:135` = `GetProduct`, 1) + `party.Quote` (`party/application/service.go:780`, `products.GetByID` per baris lagi ~ 3) = **~7 query/baris**.
- `MoveIn/MoveOut` per baris (`stock/application/service.go:410-466`): `resolveProduct` (3) + `resolveLocation` (2) + tx (Begin, Ensure, ExpireReservations SELECT, GetBalance FOR UPDATE, postLocked: Ensure, Add, Insert, Commit) = **~12 statement/baris**.
- Antrean posting (`finance/application/queue.go:54`): `PostedSourceIDs` memuat SEMUA id sumber per jenis (`journals.go:474`; probe 1,46 s untuk 66k id), 5 kali per pembukaan antrean, lalu di-diff dengan hanya 200 kandidat terbaru (`QueueCap`, `queue.go:33`): **dokumen lama yang belum terposting tak pernah muncul** (temuan baru; F2b-9 mengganti dengan pindai berkursor).

**Rancangan (semua lewat `contracts/`, tanpa join lintas modul):**
1. **Sales/purchasing:** perluas `OrderSummary` dengan `TaxInvoiceNumber, TaxInvoiceDate` (sales) / `SupplierInvoiceNumber/Date` (purchasing) dan beri `purchasing.SummariesByIDs` (sales sudah punya, `aggregates.go:~190`). `TaxDetail` -> satu panggilan per chunk 1000 id (`WHERE id IN (...)`, PK): **1-6 query**, bukan ~11k.
2. **Payment:** `repo.AllocationsByPayments(ids)` (satu `IN`), `salesreturn/purchasereturn.NumbersByIDs`, `sales/purchasing.SummariesByIDs`, `party.NamesByIDs` (sudah ada) -> `Ledger`/`List` memakai 4-5 query per halaman.
3. **Pricing:** `ProductClient.GetByIDs(ctx, ids) (map[id]*Product, error)` (2 query: produk+kategori, varian `IN`) dipakai `priceLines`, `ResolveUOM` (faktor dari produk yang sudah dimuat) dan `party.Quote` (satu pemuatan bersama per request). Target <= 3 query per request, bukan 7/baris.
4. **Stok tulis:** `Post` batch F2a-0 + `ProductClient.GetByIDs` + cache lokasi per request (`CheckLocation` dari peta satu `SELECT ... WHERE id IN`), upsert saldo (F4-4 #5): per posisi 3 statement; per dokumen 1 `GetByIDs` + 1 `locations IN`.
5. **Antrean:** `PostedAmong(branch, type, ids)` = `SELECT source_doc_id FROM finance_journal_entries WHERE branch_id=? AND source_doc_type=? AND source_doc_id IN (<=1000) AND source_active = 1` (memakai `uq_finance_journal_source`; probe anti-join 200 id ~lantai vs 1.456 ms), dipanggil per halaman kandidat berkursor (F2b-9 `ListPostable(afterID, limit)`).

**Dampak API/UI:** nol (data identik; antrean menampilkan dokumen yang tadinya tersembunyi = perbaikan). **Test:** penghitung statement (pembungkus `sql.DB` di tes integrasi; assert `<= N` untuk `TaxDetail` 1.200 SO, `Ledger` 20 baris, `priceLines` 30 baris, `Post` 10 posisi) — sekaligus gerbang regresi performa. **Estimasi:** `TaxDetail` ~11.000 -> <= 8 query; `Ledger` ~100 -> 5; `priceLines` 30 baris ~210 -> 3; MoveOut per baris 12 -> 3 + overhead tetap. **Dependensi:** F2a-0 (Post), F2b-9. **Ukuran:** M.

---

### F4-8 — Payment / delivery / retur / penerimaan: indeks & `WaitingReturn` sargable (ukuran S)

**Bukti.** `WaitingReturn` (`delivery/infrastructure/repository.go:202-235`) menyaring `branch_id, status='draft', document_kind='order'` lalu `DATEDIFF(CURDATE(), delivery_date)` (fungsi pada kolom = tak sargable) dan `ORDER BY delivery_date, id` — probe 642 ms -> 103 ms dengan indeks urut. `CURDATE()` memakai zona SESI MySQL (Laragon `time_zone=SYSTEM`; host Linux/Docker biasanya UTC): antara 00:00-07:00 WIB "hari ini" bergeser -> pita umur "today/>2/>7" salah (bug perilaku nyata, bukan sekadar performa).

**DDL (modul masing-masing, baseline):**
```sql
-- delivery_notes: UNIQUE uq_delivery_notes_number (number); UNIQUE uq_delivery_notes_client_ref (branch_id, client_ref) [F2a];
--   KEY ix_delivery_notes_branch_id (branch_id, id);
--   KEY ix_delivery_notes_so (sales_order_id, status, document_kind);                  -- F2a; menggantikan (sales_order_id)
--   KEY ix_delivery_notes_return (sales_return_id);
--   KEY ix_delivery_notes_waiting (branch_id, status, document_kind, delivery_date, id);   -- probe 642 -> 103 ms
-- delivery_note_items: FK delivery_id (indeks implisit); reservation_key VARCHAR(100) NULL [F2a]
-- goods_receipts: UNIQUE number; UNIQUE (branch_id, client_ref); KEY (branch_id, id); KEY (purchase_order_id, status)   -- menggantikan (purchase_order_id)
-- sales_returns / purchase_returns: KEY (branch_id, id); KEY (sales_order_id, status) / (purchase_order_id, status)    -- menggantikan (..._id)
-- payments: UNIQUE number; UNIQUE (branch_id, client_ref); KEY (branch_id, id); KEY (party_id, status) [urut id gratis: PK ikut indeks]; KEY (pair_id)
-- payment_allocations: KEY ix_payment_allocations_order (order_type, order_id, payment_id, amount)  -- penutup untuk PaidForOrders; FK payment_id (indeks implisit)
```
**Kode:** `WaitingReturn`: hitung `today := timeutil.NowUTC().In(timeutil.Jakarta)` di Go, lalu hari ini -> `delivery_date >= ? AND delivery_date < ?`, `gt_2` -> `delivery_date < today-2h`, `gt_7` -> `delivery_date < today-7h` (tanggal `YYYY-MM-DD`; kolom berisi tengah malam WIB). Setara semantik `DATEDIFF` (hari kalender) dan bebas zona sesi. Hal sama dicatat F8-18 untuk `CURDATE()` 4x.

**Dampak API/UI:** nol (memperbaiki batas hari pukul 00-07 WIB bila server tidak WIB). **Test:** unit pita umur dengan jam palsu; integrasi dengan sesi `time_zone` UTC dan `+07:00` -> hasil identik; EXPLAIN `ix_delivery_notes_waiting` tanpa filesort. **Estimasi:** 642 -> 103 ms (probe). **Ukuran:** S.

---

### F4-9 — Audit: indeks, tabel detail before/after, retensi (ukuran M)

**Masalah + bukti (verifikasi klaim #9 — TERBUKTI).** `audit_logs` (`000017`): `ix (branch_id, created_at)`, `ix (action_key)`; `List` (`audit/infrastructure/repository.go:30-60`) = `(branch_id = ? OR branch_id IS NULL) ... ORDER BY id DESC LIMIT/OFFSET` + `COUNT(*)` — tak ada indeks yang melayani OR + urut id: probe pada 1 jt baris **543 ms (halaman 1), 591 ms (offset 200k), `COUNT` + filter aksi 1.125 ms**. Tumbuh tanpa batas; tiap mutasi menulis 1 baris di koneksi sendiri (best-effort setelah komit). F3b menambah `before_json/after_json/metadata_json` LANGSUNG di `audit_logs` (`000039`): baris yang dibaca daftar (kolom sempit) ikut melebar -> halaman InnoDB berisi JSON, daftar membaca lebih banyak halaman.

**Rancangan (menggantikan F3b `000039`; DDL baseline `000017_audit`):**
```sql
CREATE TABLE audit_logs (
  id INT UNSIGNED NOT NULL AUTO_INCREMENT,
  action_key VARCHAR(100) NOT NULL, entity_type VARCHAR(50) NOT NULL, entity_id INT UNSIGNED NULL,
  branch_id INT UNSIGNED NULL, actor_type VARCHAR(20) NOT NULL DEFAULT 'user', actor_id INT UNSIGNED NULL,
  note VARCHAR(500) NULL, has_detail TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
  PRIMARY KEY (id),
  KEY ix_audit_logs_branch (branch_id, id),                       -- menggantikan (branch_id, created_at): created_at monoton dengan id
  KEY ix_audit_logs_branch_action (branch_id, action_key, id),
  KEY ix_audit_logs_entity (entity_type, entity_id, id)           -- riwayat satu dokumen (untuk before/after F3b)
);                                                                  -- ix_audit_logs_action (action_key) dibuang: selektivitas rendah
CREATE TABLE audit_log_details (
  log_id INT UNSIGNED NOT NULL, before_json JSON NULL, after_json JSON NULL, metadata_json JSON NULL,
  PRIMARY KEY (log_id),
  CONSTRAINT fk_audit_log_details_log FOREIGN KEY (log_id) REFERENCES audit_logs (id) ON DELETE CASCADE);
```
`Log` menulis detail hanya bila ada diff (`has_detail=1`; dua INSERT dalam satu tx singkat — tetap best-effort). **Kode `List/Count`** (`repository.go:30-95`): `(branch OR NULL)` dipecah jadi `UNION ALL` dua cabang berindeks (`WHERE branch_id = ?` dan `WHERE branch_id IS NULL`, tiap cabang `ORDER BY id DESC LIMIT offset+limit`), digabung dan dipotong `LIMIT/OFFSET`; `Count` = jumlah dua `COUNT` berindeks.
**Retensi:** job harian (F4-10) menghapus `audit_logs` berumur > N bulan dalam batch 5.000 (`DELETE ... WHERE id <= ? ORDER BY id LIMIT 5000`, FK cascade menghapus detail). N = Pertanyaan #2 (usulan 24 bulan online; arsip CSV bila pemilik minta). **Dampak API/UI:** nol (daftar identik; `Total` tetap eksak). **Test:** EXPLAIN (tak ada filesort; memakai `ix_audit_logs_branch`), integrasi list branch+global+filter aksi/entitas membandingkan hasil lama vs baru pada 5k baris acak, tes retensi (batas + cascade). **Estimasi (probe 1 jt baris):** halaman 1: 543 -> ~100 ms (UNION ALL); `COUNT` filter aksi: 1.125 -> ~170 ms (`ix_audit_logs_branch_action`); `COUNT` tanpa filter 1 jt baris tetap ~400 ms (index-only) — retensi 24 bulan menahan ~1 jt baris. Opsi `COUNT` terbatas ("10.000+") TIDAK diambil karena mengubah UI. **Dependensi:** F3b-12 (kontrak detail). **Ukuran:** M.

---

### F4-10 — Housekeeping/retensi (ukuran S)

**Masalah.** Tabel yang tumbuh tanpa batas dan tak pernah dipangkas: `stock_reservations` (baris `expired/consumed/released` ~400k EST; menumpuk di indeks pencarian `ExpireReservationsTx` `repository.go:305`), `user_sessions` (sesi dicabut/kedaluwarsa), `audit_logs` (F4-9), `assistant_runs/messages/tool_executions/threads` (jejak bot; `assistant_tool_executions` write-only), `finance_tax_report_snapshots` (kecil, dipertahankan).
**Rancangan.** `internal/shared/housekeeping` (satu goroutine di satu container, dijalankan dari `main.go` bersama penyapu reservasi F2a-12): sekali per hari 02:00 WIB, `GET_LOCK('mini_erp_housekeeping', 0)` (jaga bila dua proses), batch kecil dengan jeda; tiap tabel dipangkas oleh modul pemiliknya lewat metode `Purge(ctx, olderThan, limit)` pada repository modul itu (bukan SQL lintas modul):
- `stock_reservations`: `DELETE ... WHERE status <> 'active' AND expires_at < UTC_TIMESTAMP(6) - INTERVAL 7 DAY LIMIT 5000` (indeks `(status, expires_at)`).
- `user_sessions`: `DELETE ... WHERE (revoked_at IS NOT NULL AND revoked_at < now-30d) OR absolute_expires_at < now-30d LIMIT 5000` — tambah `KEY ix_user_sessions_expiry (absolute_expires_at)` dan `KEY (revoked_at)` (modul auth).
- `assistant_*`: `tool_executions`/`messages`/`runs` > 90 hari (indeks `created_at` ada di `assistant_runs`; tambah `assistant_tool_executions (created_at)`, `assistant_messages (created_at)`), `threads` tanpa pesan > 180 hari.
- `audit_logs`: F4-9.
Nilai retensi dapat diatur lewat `company_settings` (kunci `retention.*`, default di atas) — bukan konstanta tersembunyi; tidak ada penghapusan data keuangan/dokumen.
**Dampak API/UI:** nol. **Test:** integrasi per `Purge` (baris lama terhapus, baru utuh, batas batch, idempoten); `GET_LOCK` dua goroutine -> satu jalan. **Estimasi:** `stock_reservations` ~ konstan (< 20k baris) bukan 400k+ -> `ExpireReservationsTx` tetap seek kecil. **Ukuran:** S.

---

### F4-11 — Ekspor SO/PO: keyset + streaming + timeout per rute (ukuran M)

**Verifikasi klaim #6 — TERBUKTI.** `sales/application/export.go:62-110` (`exportXLSX`/`exportPDF`): halaman 500 baris dengan `OFFSET (page-1)*500` (`repo.List` `repository.go:209`) — biaya total kuadratik: halaman ke-k melewati 500·k baris (200k order = 400 halaman, ~40 jt baris dilewati); hasil akhir dikumpulkan di `bytes.Buffer` lalu `[]byte` penuh dikembalikan ke handler (`Export` mengembalikan `[]byte`) — memori O(ukuran berkas) per permintaan, bersama PDF yang dibangun utuh di memori (`fpdf`). `purchasing/application/export.go:76,122` identik. `cmd/server/main.go:146` `WriteTimeout: 30s` berlaku untuk SEMUA rute, jadi ekspor besar terputus diam-diam di tengah unduhan. (Catatan: `StreamWriter` excelize memang tidak menahan seluruh sel, tetapi `f.Write(&buf)` mengumpulkan zip akhir di memori.)

**Rancangan.**
1. **Keyset:** `repo.ListAfter(ctx, branchID, search, status, channel, afterID, limit)` = `WHERE id < ? ... ORDER BY id DESC LIMIT 500` (memakai `(branch_id, id)`/`(branch_id, status, id)`/`(branch_id, channel, status, id)`; pencarian lewat deferred-join F4-5). Urutan hasil = urutan daftar saat ini (id turun). Biaya per halaman konstan; total linear.
2. **Streaming:** `Export(ctx, w io.Writer, ...)` menulis langsung ke `http.ResponseWriter` (`Content-Disposition`, `Content-Type` diset sebelum byte pertama; setelah itu galat hanya bisa memutus koneksi + log): XLSX = `excelize.NewStreamWriter` lalu `f.Write(w)` (data lembar disimpan excelize di berkas sementara untuk lembar besar — verifikasi `StreamChunkSize` saat implementasi); PDF = `pdf.Output(w)` setelah halaman dibangun (format PDF perlu tabel xref, jadi data sumber tetap dipaging tetapi dokumen dibangun di memori: **batasi 10.000 baris untuk PDF** dengan pesan "persempit filter atau pilih XLSX"; Pertanyaan #5).
3. **Timeout per rute** (selaras F6-4: `ReadHeaderTimeout 10s`, `WriteTimeout 60s` global): middleware `server.Extend(read, write)` memanggil `http.NewResponseController(w).SetWriteDeadline(now+120s)` hanya pada `.../export*` (dan upload). Prasyarat F6-4: `Unwrap()` pada `statusWriter` dan `gzipResponseWriter` (`middleware.go:9-17`, `compress.go:48`). `ctx` dari request menghentikan loop bila klien memutus.
4. **Perlindungan pool:** semaphore proses `maxConcurrentExports = 2` (permintaan ke-3 menunggu <= 10 dtk lalu 429 `apperror.RateLimited`) — ekspor memegang satu koneksi pada satu waktu (paging), jadi pool 25 aman.
**Dampak API/UI:** berkas identik (kolom, urutan, isi); satu-satunya perbedaan perilaku: PDF > 10.000 baris ditolak dengan pesan jelas (tadinya menghabiskan memori/terputus). **Test:** unit keyset (urutan sama dengan offset pada 1.234 baris acak, batas halaman 500), integrasi ekspor 20k baris selesai dalam < 60 dtk dan memori terbatas (`runtime.ReadMemStats` selisih < 50 MB), tes `ResponseController` (F6-4). **Estimasi:** 200k baris: kini ~40 jt baris dilewati + >10 dtk -> terpotong 30 dtk; sesudah ~400 query x ~3 ms = ~1-2 dtk data + penulisan XLSX (EST 5-15 dtk). **Dependensi:** F4-5 (indeks), F6-4. **Ukuran:** M.

---

### F4-12 — DSN: isolation, zona waktu, deadlock, pool, konfigurasi server MySQL (ukuran S)

**Bukti.** `database/db.go:27-44`: pool 25/10, tanpa param selain `parseTime`; MySQL default `REPEATABLE READ` (terverifikasi `transaction_isolation=REPEATABLE-READ`) dan `time_zone=SYSTEM`. Pola kode yang berisiko di RR: `INSERT IGNORE` pada kunci unik lalu `SELECT ... FOR UPDATE` (`EnsureBalanceTx`+`GetBalanceTx`, `stock/infrastructure/repository.go:194-300`), penyapu reservasi yang sudah dikomentari "gap lock -> self-deadlock" (`:297-302`), `NextNumber` `INSERT IGNORE` + `UPDATE`. RR memegang next-key/gap lock pada pemindaian rentang dan pada cek duplikat kunci unik; dua dokumen yang mengunci posisi stok dalam urutan baris berbeda -> 1213 (F2a-0 akan mengurutkan + retry). `CURDATE()`/`NOW()` bergantung zona sesi (F4-8).

**Rancangan.**
1. `ensureParams` menambah (bila belum ada): `loc=UTC` (eksplisit), `transaction_isolation=%27READ-COMMITTED%27`, `time_zone=%27%2B00%3A00%27`; plus F6-4: `timeout=5s&readTimeout=30s&writeTimeout=30s` (batas sambungan/IO; ETL dan `RebuildRollups` memakai DSN sendiri tanpa readTimeout). Terverifikasi (driver 1.9.3, MySQL 8.4.3): sesi melaporkan `READ-COMMITTED` dan `+00:00`.
2. **READ COMMITTED** menghapus gap lock (kecuali cek FK/unik), mengurangi 1213, dan tidak mengubah semantik kode: seluruh jalur kritis sudah memakai `FOR UPDATE`/CAS eksplisit; tak ada pembacaan "snapshot konsisten banyak query dalam satu tx" yang dibutuhkan (laporan berjalan di luar tx). Syarat server: `binlog_format=ROW` (default 8.x). **Gerbang:** seluruh tes integrasi + tes deadlock F2a-0 (50 iterasi paralel) hijau di RC; bila ada tes yang bergantung snapshot RR, tandai (Pertanyaan #3 meminta persetujuan karena lintas-cutting).
3. **Retry 1213/1205:** implementasi `dberr.RetryTx` = F2a-0 (maks 3x, backoff 20/60/150 ms + jitter) — F4 menjamin sisi schema: setiap `UPDATE/SELECT ... FOR UPDATE` memakai PK/unique (stok: PK komposit F4-3; reservasi `WHERE id`; urutan; dokumen `WHERE id`), tidak ada UPDATE rentang tanpa indeks; urutan kunci baku: reservasi -> saldo (F2a A4); upsert rollup diurut `account_id`; penomoran di autocommit sendiri (F4-16).
4. **Pool:** nilai F6-4 (25/10/5m + `ConnMaxIdleTime 2m`) cukup setelah F4-6/F4-14: puncak dashboard turun dari 7 agregasi berat menjadi 7 query milidetik. Ukur `db.Stats().WaitCount` (F6-5) sebelum menaikkan.
5. **Rekomendasi server MySQL (ops, di luar repo):** `innodb_buffer_pool_size` >= 1 GB (probe memakai 128 MB bawaan Laragon -> angka "sebelum" pesimistis), `innodb_redo_log_capacity` >= 1 GB, `innodb_lock_wait_timeout=10` (bawaan 50 dtk melewati `WriteTimeout` -> klien putus sebelum galat), `max_connections` >= 100, `time_zone='+00:00'` global, `sql_mode` strict (sudah). Catat di `docs/DEPLOYMENT.md` (F6).
**Dampak API/UI:** nol. **Test:** tes DSN (query `SELECT @@transaction_isolation, @@time_zone`), suite integrasi di RC, skenario deadlock F2a-0. **Ukuran:** S.

---

### F4-13 — Race cache chart-of-accounts & sesi (ukuran S)

**Verifikasi klaim #12 — TERBUKTI.**
- `finance/infrastructure/chartcache.go:48-81` (`ensureChart`): pemuat membaca akun (T1) tanpa lock; bila `invalidate()` (tulis akun/mapping) terjadi antara pembacaan dan `r.chart.loaded = true` (`:78-80`), cache menyimpan data basi dan baru dikoreksi pada tulisan berikutnya (tanpa TTL) -> posting memakai pemetaan akun lama sampai restart/tulisan lain.
- `auth/application/sessioncache.go:76-82` (`put`) menandai entri dengan `gen: c.gen` SAAT put, bukan saat pembacaan DB dimulai (`service.go:521-533`): jika `bump()` (`:108`, perubahan permission/status cabang) atau `dropUser` terjadi selama 6 query Verify, sesi basi tersimpan sebagai valid selama TTL (<= 30 dtk). Dampak: izin yang dicabut tetap berlaku <= 30 dtk pada jendela yang sempit.
**Perbaikan.** Chart: `chartCache.gen uint64`; `ensureChart` mencatat `gen` sebelum membaca; saat menyimpan, bila `gen` berubah -> jangan simpan (kembalikan hasil untuk panggilan ini saja); tambah TTL 10 menit sebagai jaring pengaman (PERFORMANCE_GUIDE sudah menyebut TTL sebagai jaring pengaman tetapi chart tidak punya). Sesi: `token := c.token(userID)` (gen global + epoch per-user, peta kecil <= jumlah user) diambil SEBELUM Verify membaca DB; `put(sessionID, sess, now, token)` menolak bila token berubah. **Test:** unit deterministik dengan hook (`invalidate` disisipkan di antara baca dan simpan) — sebelum perbaikan gagal, sesudah lulus. **Dampak API/UI:** nol. **Ukuran:** S.

---

### F4-14 — Dashboard: 7 request paralel (ukuran M; prioritas rendah setelah F4-6)

**Bukti.** `DashboardPage` memanggil tujuh endpoint (`web/src/modules/dashboard/services/dashboard.service.ts:14-40`: summary, sales-trend, critical-stock, priority-orders, order-status, top-sellers, top-margin); tiap panggilan membaca footing seumur hidup/agregasi (F4-6). Pool 25 koneksi: empat dashboard terbuka bersamaan = 28 query berat yang menunggu pool. Setelah F4-6/F4-15 tiap endpoint = milidetik; sisa risiko = ledakan permintaan identik saat banyak pengguna membuka dashboard cabang yang sama serentak.
**Rancangan.** (1) `singleflight` per kunci `(branch, endpoint, parameter)` pada handler dashboard: permintaan identik yang bersamaan berbagi satu eksekusi — tanpa kedaluwarsa data (UX tak berubah). (2) Opsional, hanya bila terukur perlu: endpoint agregat `GET /api/v1/dashboard/overview` yang menjalankan ketujuh bagian di server dengan batas paralel 3 (`errgroup.SetLimit(3)`) dan mengembalikan satu respons; endpoint lama tetap (kompatibel). (3) Cache TTL 15 dtk TIDAK dipasang (mengubah kesegaran angka); bila kelak diminta, kunci per cabang. **Dampak API/UI:** nol (opsional: satu request). **Test:** integrasi 50 request paralel -> hitung eksekusi repository (singleflight = 1 per kunci). **Ukuran:** M (S bila hanya singleflight).

---

### F4-15 — `CriticalStock` tanpa batas 5000 (ukuran S)

**Bukti (klaim #15 — TERBUKTI).** `product/infrastructure/repository.go:449-452` (`ListStocked`, `ORDER BY code ASC LIMIT 5000`) lalu `stock/application/service.go:286-290` (`CriticalStock`) mem-filter di Go: produk ke-5001 dst. TIDAK PERNAH diperiksa (hasil "kritis" bisa kurang tanpa pesan). Tidak bisa "dihitung di SQL" lintas modul (`min_stock_qty` milik product, saldo milik stock; join dilarang).
**Rancangan.** Kontrak `ListStocked(ctx, afterID, limit)` (keyset, 2.000 per halaman, urut `id`); `CriticalStock` mengambil `ProductAvailability(branch)` (satu `GROUP BY` terindeks PK komposit F4-3, ~jumlah posisi) lalu menelusuri halaman produk sampai habis, memilih top-N dengan heap (memori O(limit)). Tanpa DDL. **Dampak API/UI:** nol (hasil lengkap; urutan "paling kritis dulu" sama). **Test:** 6.000 produk tracked -> semua diperiksa (produk ke-5.500 ber-stok nol muncul). **Estimasi:** 8k-20k produk = 4-10 halaman x ~3 ms. **Ukuran:** S.

---

### F4-16 — Penomoran dokumen: satu statement, tanpa transaksi (ukuran S)

**Bukti.** `branch/infrastructure/repository.go:253` `NextNumber` = `BeginTx` + `INSERT IGNORE` + `UPDATE ... LAST_INSERT_ID(...)` + `SELECT LAST_INSERT_ID()` + `Commit` = 5 round trip per nomor (tiap SO + pembayaran POS = 10); `finance.NextSequence` (`finance/infrastructure/repository.go:236`) sama. Kontensi (klaim #16): baris `(branch_id, doc_kind)` dikunci selama tx kecil itu — sempit, tetapi round trip menambah jendela kunci.
**Rancangan (terbukti pada MySQL 8.4.3 + driver 1.9.3: 400 goroutine paralel -> nomor 1..400 persis, tanpa duplikat, `f4seq/main.go`):**
```go
res, err := db.ExecContext(ctx,
  `UPDATE branch_document_sequences SET current_value = LAST_INSERT_ID(current_value + 1) WHERE branch_id = ? AND doc_kind = ?`, branchID, key)
if n, _ := res.RowsAffected(); n == 1 { return res.LastInsertId() }          // 1 round trip pada kasus normal
_, _ = db.ExecContext(ctx, `INSERT IGNORE INTO branch_document_sequences (branch_id, doc_kind, current_value) VALUES (?, ?, 0)`, ...) // baris belum ada (kunci bulanan baru)
// ulangi UPDATE satu kali
```
`LastInsertId()` dari paket OK `UPDATE ... LAST_INSERT_ID(expr)` = nilai expr (per koneksi, tak butuh tx dan tak bergantung pada koneksi pool lain). Kunci bulanan dihitung dalam WIB (F2a-11: bug UTC vs label WIB). PK komposit (F4-3) membuat `WHERE branch_id=? AND doc_kind=?` seek PK. Provisioning `ensureSequences` saat buat cabang dipertahankan. **Dampak API/UI:** nol (nomor sama; lubang akibat kegagalan insert dokumen tetap mungkin = Pertanyaan F2a #2). **Test:** paralel 200 goroutine -> kontigu tanpa duplikat; kunci bulan baru -> baris dibuat. **Estimasi:** 5 -> 1 round trip per nomor. **Ukuran:** S.

---

### F4-17 — Harness EXPLAIN / benchmark + verifikasi rollup (ukuran M)

**Tujuan.** Menjadikan temuan probe sebagai gerbang regresi tetap, bukan sekali pakai.
1. `scripts/perf/seed.sql` (dari `f4_seed.sql`, parametrik: N entri/order/mutasi) + `scripts/perf/bench.sh` (dari `f4_bench.sh`): hanya dijalankan pada DB bernama `zz_perf_*` (guard nama + menolak bila DB berisi data non-sintetis), dipanggil manual/nightly — tidak per commit (seed 1 jt baris ~ 5 menit).
2. `internal/integration/explain_test.go` (build tag `perf`): untuk ~14 query kunci (daftar SO default/status/pihak/tanggal, ListMovements, ListTransfers, WaitingReturn, daftar audit, AccountFootings rollup, ListByParty, GL per akun) menjalankan `EXPLAIN FORMAT=JSON` di DB tes lalu menegaskan: `access_type != ALL`, `key` = indeks yang diharapkan, tidak ada `using_filesort` pada daftar tanpa pencarian, `using_index` untuk COUNT penutup. Perubahan indeks yang merusak rencana = tes merah.
3. Tes penghitung statement (F4-7) untuk `TaxDetail`, `Ledger`, `priceLines`, `Post`.
4. `VerifyRollups` (F4-6) dijalankan setelah tiap skenario tes finance dan dalam `GET /finance/integrity`.
5. Dokumen hasil: `docs/PERF_BASELINE.md` berisi tabel Lampiran D yang diperbarui (angka baru dari data ETL nyata setelah F5).
**Dampak:** nol pada produk. **Dependensi:** F4-4..F4-9; F8-1 (DB uji). **Ukuran:** M.


---

## 4. Urutan, dependensi, dan KAPAN squash

```
(awal)  F4-12 DSN/RC ──► F4-13 race cache ──► F4-2 pelebaran (dev, per modul) ──► [F5 rehearsal ETL boleh mulai di sini terhadap schema dev]
(fase 2-4, menyatu dengan migrasi agen lain)
        F2a-0 + F4-4 (stok) ─┐
        F3a-2 + F4-5 (SO/PO) ┼─► F4-8 ─► F4-9 ─► F4-10
        F2b-14a + F4-6 ──────┘            └─► F4-7 ─► F4-14, F4-15, F4-16, F4-11
        F4-17 (harness; paralel, wajib hijau sebelum beku)
                         │
                  ═══ GERBANG "SCHEMA BEKU" (tag `schema-freeze`) ═══
                         │
        F4-1 tahap 1 (squash murni, diff nol) ─► F4-3/F4-2 sisa = tahap 2 (diff = manifest) ─► hapus migrasi lama
                         │
        F5 ETL (peta kolom target + fingerprint) ─► F8-18 sqlc (per modul) ─► rehearsal cutover (F5)
```
Aturan: **squash dilakukan SETELAH semua perubahan schema fase 2-4 selesai dan SEBELUM ETL memuat data** (F5 sendiri menyatakan "ETL baru bisa dimulai setelah squash"; F8-18 sqlc butuh baseline per modul). Selama pengembangan, perubahan = migrasi tambahan `000032+` seperti yang direncanakan agen lain; F4 hanya menambah migrasi dev yang bersifat aditif (pelebaran kolom, indeks, tabel rollup) — perubahan tipe massal (INT UNSIGNED, PK komposit, CHECK, DATE) SENGAJA ditunda ke tahap 2 squash karena berisiko tinggi sebagai ALTER bertahap pada DB dev berisi data dan tidak memengaruhi perilaku Go (int64).

Penomoran migrasi dev (agar tak bentrok; nomor final bisa bergeser, urutan jangan): F3b `000032-000039` (kecuali `000039` digantikan F4-9), F2a `000040-000043`, F2b M-A..M-H `000044-000049`, F4 `000050+` (`000050_f4_legacy_widths_*` per modul, `000051_f4_finance_rollups`), F1/F6 kecil di antara. Karena semuanya dilipat, kerapian nomor tak penting; yang penting urutan dependensi (mis. rollup setelah kolom `scope`).

---

## 5. Risiko

1. **Edit tahap 2 salah satu tipe kolom referensi** (mis. `*_id` tetap BIGINT di satu tabel): join intra-modul tak memakai indeks optimal. Mitigasi: pemeriksa otomatis "semua kolom bernama `id`/`*_id`/`*_by` bertipe sama" di `scripts/schema-lint.mjs` + manifest diff.
2. **Rollup menyimpang dari lines** (bug jalur tulis, restore parsial, ETL): laporan salah tanpa terlihat. Mitigasi: `VerifyRollups` di gerbang tutup periode + endpoint integritas + tes ekuivalensi + `RebuildRollups`; rollup SELALU dapat dibangun ulang dari lines (sumber kebenaran).
3. **Perubahan isolation ke READ COMMITTED** lintas-cutting: kode yang tanpa sadar bergantung pada pembacaan berulang konsisten. Mitigasi: gerbang suite integrasi + tes deadlock; flag DSN dapat dicabut seketika.
4. **PDF dibatasi 10.000 baris** (F4-11) mengubah perilaku ekstrem; diumumkan lewat pesan galat.
5. **Float64 untuk qty** (S-18): akumulasi `on_hand + ?` sebagai DOUBLE dapat meninggalkan sisa 1e-12 pada DECIMAL(24,12); `decimalArg` (F4-4 #6) menutup jalur tulis, tetapi kolom `qty float64` di kontrak tetap mengundang kesalahan pembulatan pada perbandingan `Available() < qty`. Rekomendasi (di luar F4): epsilon 1e-9 pada perbandingan stok (F2a memakai `1e-9` di retur).
6. **Probe ≠ produksi:** data sintetis seragam, buffer pool 128 MB, klien `mysql.exe` (lantai 100 ms), satu cabang dominan (90%). Angka dipakai untuk BENTUK rencana (ALL -> seek, filesort -> urut indeks) dan urutan besaran; ulangi dengan data ETL nyata (F4-17 #5).
7. **Retensi menghapus data yang dianggap berharga** (audit): default usulan konservatif + dapat diatur + arsip CSV opsional; tidak menyentuh data keuangan.
8. **Unique idempotensi `stock_movements` (F2a) + ETL**: ETL harus menjumlahkan mutasi legacy bertabrakan (F5 A9); bukan risiko F4 tetapi memengaruhi ukuran baseline.
9. **Squash menghilangkan histori migrasi**: produksi tak punya histori (DB kosong) jadi tak masalah; dev lama harus rebuild; simpan tag git `schema-freeze` untuk audit.
10. **Pelebaran `media_files.file_key` ke 700** membesarkan indeks unik; bila F5 membuktikan kunci legacy pendek/ASCII, kembali ke 255 (ascii) — keputusan data, bukan desain.
11. **`INSERT ... AS new` (alias baris)** tidak dipakai agar lolos sqlc; `VALUES()` ber-warning deprecated di 8.4 tetapi berfungsi (terverifikasi di probe); bila F8 mengganti parser, ganti serempak.

---

## 6. Pertanyaan terbuka (hanya yang butuh keputusan pemilik)

1. **Ganti semua id ke `INT UNSIGNED`?** (F4-3 #1; -24% ukuran DB EST, indeks lebih kecil; menyentuh seluruh baseline, konfigurasi sqlc, dan ETL ID eksplisit.) Rekomendasi: YA. Alternatif: tetap `BIGINT UNSIGNED` (nol risiko tipe, ~+24% ruang).
2. **Retensi data:** (a) `audit_logs` berapa bulan online (usulan 24; arsip CSV sebelum hapus: ya/tidak); (b) sesi kedaluwarsa 30 hari, reservasi POS non-aktif 7 hari, jejak bot 90 hari — setuju? (c) tampilan total audit tetap eksak (usulan) atau "10.000+" demi kecepatan?
3. **Persetujuan operasional MySQL:** `READ COMMITTED` sebagai isolation aplikasi, dan penetapan parameter server (buffer pool >= 1 GB, `innodb_lock_wait_timeout=10`, `time_zone` UTC) pada host produksi (pemilik/ops, bukan kode).
4. **`finance_tax_report_snapshots` write-only:** tabel ini ditulis saat tutup periode pajak tetapi tak ada layar/endpoint yang membacanya. Pertahankan (parity legacy "beku laporan pajak") dan tambahkan pembaca di agen finance, atau hapus tabel? Rekomendasi: pertahankan + pembaca.
5. **PDF ekspor dibatasi 10.000 baris** (F4-11) — setuju, atau pilih batas lain / PDF dipecah per berkas?

---

## Lampiran A — Peta schema akhir ringkas (68 tabel = 59 sekarang + 9 baru)

Tipe: `i`=INT UNSIGNED, `I`=BIGINT, `d12`=DECIMAL(24,12), `dt`=DATETIME(6), `D`=DATE, `T`=TEXT. Kolom audit standar (`created_at/updated_at dt`, `created_by/updated_by i`) tak diulang. Hanya indeks bermakna ditulis (PK selalu).

| Modul / tabel | PK | Kolom utama (ringkas) | Indeks sekunder / unik |
|---|---|---|---|
| user: `users` | id | username V100 U, email V255 U, password_hash, full_name V255, phone V30, display_title V100, status, last_login_at | uq username, uq email |
| `roles` | id | code V50 U, name V100, description T, is_system, is_reserved | uq code |
| `permissions` | id | code V100 U, name V150 | uq code |
| `role_permissions` / `user_roles` | (a,b) | pivot | FK 2 sisi |
| `user_branch_access` | (user_id, branch_id) | is_default | — (branch_id tanpa FK, lintas modul) |
| `authz_seed_applied` | (role_code, permission_code) | applied_at | — |
| auth: `user_sessions` | id | user_id, refresh_hash C64 ascii U, prev_refresh_hash, id_active_branch/role, user_agent V255, ip V64, rotated_at, expires_at, absolute_expires_at, last_activity_at, revoked_at | uq refresh, ix user, **ix expiry (absolute_expires_at), ix revoked_at** |
| branch: `branches` | id | code V50 U, name V255, address T, city V120, phone, status, is_head, default_location_label V150 | uq code |
| `branch_document_sequences` | (branch_id, doc_kind) | current_value I | FK branch |
| company: `company_profile`, `company_settings` | id / setting_key | profil singleton; kv T | — |
| media: `media_files` | id | owner_type V50, owner_id i, file_key V700 U, original_name, mime V120, size_bytes I, is_primary, origin ENUM, width/height, sort_order | uq key, ix (owner_type, owner_id) |
| product: `product_categories` | id | code V50 U, name V255, parent_id, sort_order, status | FK parent |
| `products` | id | code V50 U, name V255, category_id, type, stock_tracked, 3 uom V30, 2 faktor d12, 3 harga I, min_stock_qty d12, summary T, attributes JSON, status | uq code, **ix name**, FK category |
| `product_variants` | id | product_id, code V80, name V255, barcode V100 U, is_default, is_hidden, attributes JSON, sort_order, status | uq (product_id, code), uq barcode |
| party: `member_types` | id | code V50 U, name V120, price_* (d(18,4)), rounding, status | uq code |
| `parties` | id | code V50, party_type V20, name V255, phone V30, email, address_text T, notes T, member_type_id, attributes JSON, status, archived_at | uq (code, party_type), **ix (party_type, name)** |
| `party_delivery_addresses` | id | party_id, label V120, recipient V150, phone, address T, is_primary, sort_order, status | FK party |
| stock: `stock_locations` | id | branch_id, code V50, name V255, parent_id, kind, is_system, status | uq (branch_id, code) |
| `stock_balances` | (branch, product, variant, location) | on_hand d12, reserved d12, updated_at | ix location, **ix (product_id, variant_id)** |
| `stock_movements` | id | branch, product, variant, location, direction ENUM, movement_type, qty_base d12, balance_before/after d12 NULL, ref_type V30 ascii, ref_id, notes V500, unit_cost, created_at, moved_at | **uq (ref_type, ref_id, product, variant, location, direction)**, ix position, ix (branch_id, moved_at), ix (product_id, variant_id) |
| `stock_transfers` (+`_items`) | id | number V100 U, from/to_branch_id, from/to_location_id, status, notes T, dispatched_at, received_at; items: transfer_id, product, variant, qty d12, from/to_location_id | **ix (from_branch_id, id), ix (to_branch_id, id)** |
| `stock_reservations` | id | reservation_key V100 U, branch, product, variant, location, qty_total/remaining d12, status, expires_at | ix lookup (branch, product, variant, status), **ix (status, expires_at)** |
| `stock_adjustments` | id | branch, kind, product, variant, location, mode, qty_delta, qty_after, reason, requested_by, approved_by, approval_mode | ix (branch_id, id), ix (product_id, id) |
| purchasing: `purchase_orders` (+`_items`) | id | number V100 U, branch, party, order_date dt, due_date, payment_terms, payment_term_days, tax_type, tax_rate d(9,4), subtotal/discount/tax/grand I, status, notes T, supplier_invoice_* V100, assigned_user_id; items: snapshot lengkap (F2a) | ix (branch,number), (branch,id), (branch,status,id), (branch,party,id), (branch,order_date,id) |
| sales: `sales_orders` (+`_items`) | id | idem + channel, member_code, client_ref V64 ascii, ship_to_* , tax_invoice_*, completed_at | + uq (branch_id, client_ref), (branch,channel,status,id) |
| goodsreceipt: `goods_receipts` (+`_items`) | id | number V100 U, branch, purchase_order_id, received_at, supplier_delivery_number V100, status, client_ref, notes T | uq number, uq (branch, client_ref), (branch,id), (po_id, status) |
| delivery: `delivery_notes` (+`_items`) | id | number V100 U, branch, sales_order_id, delivery_date dt, document_kind, sales_return_id, status, driver…, recipient…, dispatched/confirmed_*, client_ref; items: reservation_key | uq number, uq client_ref, (branch,id), (so_id,status,kind), **(branch,status,kind,delivery_date,id)** |
| payment: `payments` | id | number V100 U, branch, party, party_type, direction, amount I, method, paid_at, reference_number V100, amount_tendered I, client_ref, pair_id, status, notes T | uq number, uq client_ref, (branch,id), (party_id,status), (pair_id) |
| `payment_allocations` | id | payment_id, order_type, order_id, amount I | **(order_type, order_id, payment_id, amount)** |
| salesreturn: `sales_returns` (+items, replacement_items, settlements) | id | number V100 U, branch, sales_order_id, return_date, totals I, tax d(9,4), return_mode, reason T, replacement_*, difference_amount, receivable_reduction, status | (branch,id), (so_id,status) |
| purchasereturn: `purchase_returns` (+items, settlements) | id | idem + payable_reduction | (branch,id), (po_id,status) |
| finance: `finance_accounts` | id | code V50 U, name V255, type, is_cash, status | uq code |
| `finance_account_mappings` | mapping_key | account_id | FK |
| `finance_periods` / `finance_tax_periods` | id | branch, year SMALLINT, month TINYINT, status, closed_at | uq (branch, year, month) |
| `finance_document_sequences` | (branch_id, seq_key) | current_value I | — |
| `finance_journal_entries` | id | lihat F4-6 | uq number, uq source (4 kolom), (branch,entry_date), (entry_date) |
| `finance_journal_lines` | id | lihat F4-6 | (entry_id), (account_id, entry_id), CHECK |
| `finance_account_daily`, `_party_daily`, `_product_daily` | komposit | lihat F4-6 | — |
| `finance_inventory_cost_movements` | id | branch, product, variant, stock_movement_id U, ref_type, ref_id, direction ENUM, qty_base d12, unit_cost d12, total_cost d(24,4), is_estimated, moved_at | uq stock_movement_id, ix ref, ix (branch, ref_type, stock_movement_id), ix estimated |
| `finance_cost_positions` / `finance_cost_state` / `finance_cutover` | komposit | lihat F4-6, F2b M-E/M-G | — |
| `finance_tax_report_snapshots` | id | tax_period_id, snapshot_json | FK |
| `business_expenses` | id | number V60 U, branch, expense/pay account, amount I, expense_date, notes T, journal_entry_id, status, cancel_* | uq number, (branch,id), (branch, expense_date) |
| audit: `audit_logs`, `audit_log_details` | id / log_id | lihat F4-9 | (branch,id), (branch,action,id), (entity_type,entity_id,id) |
| assistant: 7 tabel | — | tak berubah selain tipe id; + `assistant_tool_executions (created_at)`, `assistant_messages (created_at)` untuk retensi | — |

---

## Lampiran B — Daftar kebutuhan schema dari semua agen (konsolidasi)

| Agen | Permintaan | Keputusan F4 | Catatan |
|---|---|---|---|
| F2a | `stock_movements`: `balance_before/after`, `moved_at`, `uq_stock_movements_ref`, indeks `(branch_id, moved_at)` | DITERIMA; `ref_type` jadi `VARCHAR(30) ascii`; `direction` ENUM; qty>0 CHECK | F4-4; urutan list memakai `moved_at, id` |
| F2a | `stock_locations.kind`; `stock_transfer_items.from/to_location_id`; `stock_adjustments`; indeks `stock_reservations(status, expires_at)` | DITERIMA | F4-4 |
| F2a | `delivery_notes.client_ref`+unik, `ix(so,status,kind)`; `delivery_note_items.reservation_key` | DITERIMA; `client_ref VARCHAR(64) ascii`; indeks lama `(sales_order_id)` dihapus (prefix) | F4-8 |
| F2a | `goods_receipts`: number, supplier_delivery_number, status, client_ref, unik, `ix(po,status)` | DITERIMA; number V100, supplier_delivery_number **V100** (legacy 100, F2a menulis 60) | F4-2/8 |
| F2a | SO/PO: `notes TEXT`, `assigned_user_id`, SO `client_ref`+unik, PO `payment_term_days` | DITERIMA | F4-5 |
| F2a | item SO/PO: `stock_tracked`, `item_type`, `variant_name/code`, `base_uom`, `pricing_source`, `base_price`, `discount_reason`, `line_note`, `taxable`, `tax_base`, `tax_amount` | DITERIMA; **konflik `pricing_source` (lihat bawah)**; `line_note` jadi TEXT | F4-2 |
| F2a | `payments`: `client_ref`+unik, `amount_tendered`, `reference_number`, `pair_id`+indeks | DITERIMA | F4-8 |
| F2a | retur: `reason`, `replacement_*`, `difference_amount`, `*_reduction`, `return_condition`, `tax_base/amount` penggantian, `*_settlements.auto`, `ix(order_id,status)` | DITERIMA; `reason` jadi TEXT | F4-8 |
| F2b M-A | `finance_journal_entries`: `scope`, `requeue`, `source_active` (VIRTUAL), unik 4 kolom | DITERIMA persis | F4-6 |
| F2b M-B | CHECK `chk_finance_lines_one_side` + 2 indeks penutup lebar | CHECK DITERIMA; **indeks penutup DITOLAK, diganti rollup pihak/produk + indeks `(account_id, entry_id)`** | F4-6 |
| F2b M-C | `finance_account_daily` | DITERIMA (PK `(branch_id, scope, account_id, day)`) | F4-6 |
| F2b M-D | `finance_party_balances` seumur hidup | **DIGANTI** `finance_party_daily` + `finance_product_daily` (as-of tanggal apa pun) | F4-6 |
| F2b M-E/F/G/H | cost positions + state, `moved_at` + indeks ledger, `finance_cutover`, kolom cancel `business_expenses` + indeks | DITERIMA; positions + `estimated_rows`; ledger `unit_cost (24,12)`, `total_cost (24,4)` | F4-6 |
| F2b (modul lain) | `sales_return_replacement_items.tax_*`, `sales_orders.completed_at`, `stock_movements.unit_cost` | DITERIMA | F4-4/5 |
| F3a | indeks `(branch_id, order_date)`, `(branch_id, party_id, id)` SO/PO; `sales_order_items.pricing_source VARCHAR(20) 'standard'`; migrasi DATA permission member_types | indeks DITERIMA (+id di akhir); pricing_source lihat konflik; **migrasi data TIDAK ikut baseline** (pindah ke seed idempoten) | F4-5, F4-1 |
| F3b | widths users/branches/variants/categories; `users.phone/display_title/last_login_at`; `user_sessions.user_agent/ip_address`; `roles.is_reserved`; `ix stock (product_id, variant_id)` movements+balances; `products.summary/attributes`; variants `attributes/is_hidden/sort_order`; media `width/height/original_size_bytes/sort_order`; `parties.attributes`; branches `default_location_label`; audit before/after | DITERIMA semua kecuali audit: **before/after pindah ke `audit_log_details`** (F4-9); migrasi DATA `rounding_mode` tidak ikut baseline | F4-2/4/9 |
| F1/F6 | `authz_seed_applied`; `media_files.origin ENUM('app','legacy')`; `user_sessions.rotated_at`; DSN `timeout/readTimeout/writeTimeout`; pool dapat diatur; skema versi pagar | DITERIMA; tabel/kolom ikut baseline modul masing-masing | F4-1/12 |
| F5 | lebar kolom >= legacy (A1); `client_ref` NULLable; item membawa snapshot varian; `expectedFingerprint` dari baseline; ID eksplisit + `AUTO_INCREMENT` disetel ulang; tanpa `legacy_id` | DITERIMA; fingerprint = sha256 keluaran Lampiran C tahap 2; INT UNSIGNED memuat id legacy | A4 "posting_state per tabel dokumen" TIDAK dibuat — F2b memilih `finance_cutover` (kolom milik modul lain tak boleh diubah finance) |
| F8 | baseline per-modul; `_it_schema/_test_schema(hash)`; `table-owners.json` 1:1 `CREATE TABLE`; override sqlc `int unsigned -> int64`; opsi C `tax_base/tax_amount` tersimpan | DITERIMA | F4-1 |

**Konflik/penyatuan yang harus disepakati saat merge:**
1. `sales_order_items.pricing_source`: F2a `VARCHAR(12) DEFAULT 'catalog'` vs F3a `VARCHAR(20) DEFAULT 'standard'` -> SATU kolom `VARCHAR(20) NOT NULL DEFAULT 'standard'` bernilai `standard|member_rule|manual` (nilai legacy langsung; tak perlu mapping ETL).
2. `reason`/`line_note`/`notes`: F2a menulis `VARCHAR(255)`; legacy TEXT -> TEXT (F4-2).
3. Nomor dokumen `VARCHAR(60)` (semua agen) -> `VARCHAR(100)`; `supplier_delivery_number` 60 -> 100.
4. Audit before/after: F3b `000039` (kolom di `audit_logs`) digantikan F4-9 (tabel samping).
5. Rollup pihak/produk: F2b M-B/M-D digantikan F4-6.
6. F5 A4 (`posting_state` per dokumen) vs F2b M-G (`finance_cutover`): pakai F2b.
7. `entry_date` jurnal `DATE` (F4-3) vs F2a `order_date` datetime-instan: builder jurnal wajib mengonversi ke tanggal WIB.

---

## Lampiran C — Skrip pembanding schema (rancangan; dipakai F4-1 tahap 1/2 dan F5/F8)

`scripts/schema-fingerprint.mjs <db>` menjalankan lima kueri via klien `mysql` (`-N -B`), mengurutkan, mencetak teks deterministik; `sha256` teks itu = sidik jari. `scripts/schema-compare.mjs A B` = diff teks (+ ringkasan per tabel) dan, bila diberi `--expected scripts/schema-expected-diff.txt`, memastikan diff == manifest.

```sql
-- C: kolom (tanpa ordinal_position/AUTO_INCREMENT agar urutan ALTER tak berarti)
SELECT 'C', table_name, column_name, column_type, is_nullable, IFNULL(column_default,'<NULL>'), extra,
       IFNULL(character_set_name,''), IFNULL(collation_name,''), IFNULL(generation_expression,'')
FROM information_schema.columns WHERE table_schema = DATABASE() ORDER BY table_name, column_name;
-- I: indeks
SELECT 'I', table_name, index_name, non_unique, seq_in_index, IFNULL(column_name,''), IFNULL(expression,''),
       IFNULL(sub_part,''), IFNULL(collation,''), index_type, is_visible
FROM information_schema.statistics WHERE table_schema = DATABASE() ORDER BY table_name, index_name, seq_in_index;
-- F: FK
SELECT 'F', k.table_name, k.constraint_name, k.column_name, k.referenced_table_name, k.referenced_column_name, r.update_rule, r.delete_rule
FROM information_schema.key_column_usage k JOIN information_schema.referential_constraints r
  ON r.constraint_schema = k.constraint_schema AND r.constraint_name = k.constraint_name
WHERE k.table_schema = DATABASE() AND k.referenced_table_name IS NOT NULL ORDER BY k.table_name, k.constraint_name, k.ordinal_position;
-- K: CHECK
SELECT 'K', t.table_name, c.constraint_name, c.check_clause
FROM information_schema.check_constraints c JOIN information_schema.table_constraints t
  ON t.constraint_schema = c.constraint_schema AND t.constraint_name = c.constraint_name
WHERE t.table_schema = DATABASE() AND t.constraint_type = 'CHECK' ORDER BY t.table_name, c.constraint_name;
-- T: tabel
SELECT 'T', table_name, engine, table_collation, row_format, IFNULL(create_options,'')
FROM information_schema.tables WHERE table_schema = DATABASE() AND table_type = 'BASE TABLE' ORDER BY table_name;
```
Normalisasi: nama FK/CHECK bebas-auto dihapus dari diff bila bernama otomatis (`<tabel>_ibfk_N`, `<tabel>_chk_N`); baseline menamai semuanya eksplisit. Kolom urutan (`ordinal_position`) tidak dibandingkan karena kode memakai daftar kolom eksplisit di setiap query; urutan hanya memengaruhi `SELECT *` (tak dipakai; arch-check dapat menegaskan nol `SELECT *`).

Ekuivalensi data (bukan hanya schema) untuk rollup: `VerifyRollups` (F4-6). Ekuivalensi migrasi down: `migrate up -> down -> up` sidik jari sama.

---

## Lampiran D — Hasil probe (MySQL 8.4.3 Laragon, DB sementara `zz_schema_probe`, 2026-10-08)

Data sintetis: 300k entri/900k baris jurnal, 200k SO, 500k mutasi stok, 500k baris biaya, 200k SJ, 200k pembayaran+alokasi, 1M audit, 20k transfer. Waktu = wall-clock klien; lantai tak-ada-op = 101 ms. "Sesudah" = rencana F4 diterapkan di atas schema lama (`f4_after.sql`).

| Query | Sebelum (ms) | Sesudah (ms) |
|---|---|---|
| AccountFootings seumur hidup | 17.033 | 105 (rollup) |
| CostPositions | 3.207 | 176 |
| MaxCostMovementID | 2.983 | 100 |
| PartyFootings AR | 3.206 | 181 |
| DailyFootings 1 th | 2.422 | 132 |
| ProductFootings | 1.935 | 143 |
| PostedSourceIDs | 1.456 | 104 (anti-join 200 id) |
| ListByParty SO | 762 | 132 |
| WaitingReturn | 642 | 103 |
| Audit daftar halaman 1 | 543 | 102 |
| Audit daftar offset 200k | 591 | (kunci: batasi offset) |
| Audit COUNT + aksi | 1.125 | ~170 (`(branch_id, action_key, id)`) |
| Audit COUNT tanpa filter (1 jt) | — | ~407 (index-only; retensi) |
| SO cari LIKE | 288 | 226 |
| SO cari COUNT | 748 | 212 |
| SO COUNT status+channel | 284 | 110 |
| MovedPositions (DISTINCT 450k) | 306 | 310 (tak berubah; sekali per impor) |
| Q yang sudah baik (<=110): ListTransfers OR/COUNT pada 20k baris, ListMovements, HasMovementsAnywhere(500k), PaidForOrders 500 id, payments per party | 99-126 | 103-122 |

Ukuran tabel (probe, 900k/500k baris): baris jurnal 63,6+100,3 -> 48,6+34,1 MB; `stock_movements` 55,6+78,3 -> 57,6+65,3 MB (dengan kolom baru F2a + unik idempotensi). Rollup pihak 2,5 MB/30k baris, produk 0,5 MB/6k, akun 0,2 MB/3,6k, posisi biaya 0,2 MB/3k.

Skrip & hasil mentah: `scratchpad/f4_seed.sql`, `f4_bench.sh` + `f4_bench_before.txt`, `f4_after.sql`, `f4_bench_after.sh` + `f4_bench_after.txt`, `f4_size.sql`, `f4seq/main.go`, `f4_cols2.mjs`. Database `zz_schema_probe` sudah di-DROP setelah selesai (tidak menyentuh DB lain).





