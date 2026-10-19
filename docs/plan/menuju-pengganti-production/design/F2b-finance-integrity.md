# Rencana F2b — Integritas FINANCE (modul finance + dashboard/reporting yang membacanya)

Status dokumen: v1 (lengkap per tugas, ringkas per tugas; diperluas bila perlu). Semua path relatif ke
`H:/dimasprasetio/SAAS/mini-erp-revamp/apps/api` (revamp) dan `H:/dimasprasetio/SAAS/mini-erp/apps/api/src/modules` (legacy).
Dasar: saya membaca sendiri kode kedua repo (read-only). Penanda: **[TERBUKTI]**, **[SEBAGIAN]**, **[TERBANTAH]**, **[BARU]** (temuan di luar audit).
Verifikasi MySQL 8.4.3 (database scratch, sudah di-drop): unique key atas kolom generated VIRTUAL berfungsi sebagai "unik hanya untuk jurnal berstatus posted"; CHECK per baris ditegakkan; window function `SUM() OVER` untuk saldo berjalan berfungsi.

---------------------------------------------------------------------------------------------------
## 0. Ringkasan verifikasi audit

| # | Klaim audit | Status | Catatan inti |
|---|---|---|---|
| 1 | buildPayment hanya lihat arah+metode | TERBUKTI | `finance/application/builders.go:327-350`. Refund retur jual (out) jadi Dr 2100/Cr Kas; refund retur beli (in) jadi Dr Kas/Cr 1200; offset dijurnal seperti kas (:313-317). Alokasi `customer_credit` TIDAK ADA di payment revamp (`payment/contracts/client.go:15-20` hanya purchase/sales/sales_return/purchase_return) -> SEBAGIAN: yang ada adalah settlement memo `customer_credit` (`salesreturn/contracts/client.go:24-32`) yang tak dijurnal |
| 2 | Retur tukar: SJ pengganti tak dijurnal | TERBUKTI | `builders.go:218-220`, dikunci `integration/exchange_test.go:98-101`. Item pengganti tak punya pajak sama sekali (`salesreturn/application/service.go:879-884`: hanya `LineTotal=qty×harga katalog`) |
| 3 | Selisih stok/rusak tidak diposting | TERBUKTI | `posting.go:16-19`; kunci `writeoff`/`adjustment` tak dibaca; `6100` tak dipakai; `6200` dipakai hard-code untuk selisih retur beli (`builders.go:480`). Movement adjustment/write_off punya `ref_id=0` (`stock/application/transactions.go:335,496`) -> sumber jurnal harus id movement, bukan ref_id |
| 4 | Reverse membuang dimensi; EntryBySource tanpa filter status | TERBUKTI | `posting.go:188-191`, `journals.go:104-117,360,411,474-492`, unique `migrations/000016_finance.up.sql:62` (tanpa branch_id & tanpa status). Laporan tak butuh filter status bila cermin menyalin dimensi (asli+cermin net nol) |
| 5 | Cancel setelah posting tak membalik jurnal | TERBUKTI | `payment/application/service.go:516-529`, `salesreturn/application/service.go:449-484`, `purchasereturn/application/service.go:392-433`. SJ terkonfirmasi tak bisa dibatalkan (`delivery/application/service.go:403-409`) -> bukan yatim. Tambahan: `CancelOrder` SO/PO berstatus confirmed bisa dibatalkan walau sudah ada SJ/penerimaan/bayar (`sales/application/service.go:439-449`, `purchasing/application/service.go:413-423`) |
| 6 | Post by ID tak cek status dokumen | TERBUKTI | hanya `buildPayment` cek status (:304). `buildDelivery/SalesReturn/PurchaseReturn` tidak; HPP draft memakai rata-rata hidup (:256-265) |
| 7 | Mapping kosmetik | TERBUKTI | 13 kunci (`service.go:28-32`); builder/laporan hard-code `1300/1400/2100/2200/4000/4100/5000/6200` (grep: builders.go, analytics.go:48-56,133,138, reports.go:342-346,396-400); `contracts/client.go:63-68` konstanta kode dipakai dashboard (`dashboard/application/service.go:73-78`). `SetMapping` tak ada guard riwayat (`service.go:301-327`); legacy ada (`finance.service.ts:277-290`) |
| 8 | Seed COA beda arti pada kode sama | TERBUKTI | tabel perbandingan di F2b-8 |
| 9 | Batas diam-diam | TERBUKTI (semua 6 butir) | GL/TaxDetail LIMIT 10000 (`reports.go:266,392`) dan ambil jurnal TERBARU dulu (`ORDER BY e.id DESC`, journals.go:243) lalu saldo awal dari "sebelum from" -> saldo berjalan salah bila >10000; antrean 200/jenis (`queue.go:33,61,78,93`); `ListByBranch` retur 10000 (`salesreturn/application/service.go:89`, `purchasereturn`); `ListStocked LIMIT 5000` (`product/infrastructure/repository.go:446-452`); `TurnoverEntries` company-wide (SENGAJA, lihat F2b-15) |
| 10 | Antrean/Readiness vs migrasi | SEBAGIAN | antrean PER CABANG sesi (`UnpostedSources(ctx, branchID)`), bukan "seluruh cabang"; tetapi tanpa batas tanggal/id, tanpa filter periode, retur tanpa tanggal (`queue.go:119,136` `Date:""`), faktur kosong = blocker dan dihitung SEUMUR HIDUP (`sales/infrastructure/aggregates.go:177-185`, bukan per periode) |
| 11 | HPP 0 diam-diam | TERBUKTI + 2 temuan baru | `builders.go:266` `if cogs != 0`; `costing.go:109-113`. BARU: retur ke posisi kosong dicatat biaya 0 (N4); harga katalog tanpa bagi faktor UOM beli (N5) |
| 12 | Laporan/biaya/izin | TERBUKTI (semua sub-butir) | detail di F2b-12 |
| 13 | Dashboard definisi beda | TERBUKTI | `dashboard/application/operations.go:23,77-106` + `sales/infrastructure/repository.go:315-345` (`ORDER BY id DESC LIMIT`); `sales/infrastructure/aggregates.go:40-72`; stok kritis menjumlah SEMUA lokasi termasuk rusak (`stock/infrastructure/repository.go:259-264`). Modul reporting hanya meneruskan ke finance (UI tak memakai `/reporting/inventory`; UI memakai `/finance/reports/inventory-value` dan `/reporting/sales-trend`) |
| 14 | Biaya posting tumbuh dgn ledger | TERBUKTI | `posting.go:92,121-130`, `costing.go:14-21,40-48`, `ledger.go:42,116`; WriteTimeout 30 dtk (`cmd/server/main.go:146`) |
| 15 | Paket pajak plafon | SEBAGIAN | aritmetika akumulasi identik dgn legacy layer 2 (`running+rev <= cap`), tetapi urutan (kronologis vs CRC32), cakupan (YTD perusahaan vs rentang+cabang) dan atribusi (sumber dokumen vs `l.id_order`) SENGAJA beda; BARU: jurnal cermin (reversal) tak teratribusi (N10); CHECK DB belum ada |

### Temuan BARU (di luar audit)
- **N1** Reverse atas jurnal `tax_adjustment` menghasilkan cermin bersumber `reversal` yang LOLOS filter `commercialOnly` (`journals.go:296`, `posting.go:196`) -> laba komersial bergeser sebesar koreksi fiskal.
- **N2** Unique `(source_doc_type, source_doc_id)` global tanpa `branch_id`, sedangkan `CreateTaxAdjustment` memakai seq per cabang per tahun (`taxadjust.go:41-43`): koreksi fiskal pertama cabang ke-2, atau koreksi pertama tahun berikutnya, bentrok.
- **N3** `MoveValues` hanya `ref_type='delivery'` (`ledger.go:91-98`); jalur rollback `delivery_rollback`/`sales_return_rollback` (`delivery/application/service.go:322-324`) tak ikut -> HPP SJ yang pernah gagal-lalu-ulang terhitung 2x.
- **N4** `costOf` untuk movement masuk non-GR/non-opening pada posisi qty<=0 memberi biaya 0 (`costing.go:109-112`) dan `Average()` kosong=0 (`ledger.go:19-24`): retur penjualan ke posisi yang sudah habis tidak memulihkan persediaan/HPP.
- **N5** `costOf` opening memakai `purchase_price` katalog langsung per satuan dasar (`costing.go:101`; `product/infrastructure/repository.go:626`); legacy membagi dengan `purchase_to_base_factor` (`resolveBaseUomCost`). Bila harga katalog per UOM beli dan faktor ≠ 1 -> biaya membengkak sebesar faktor (butuh konfirmasi semantik harga katalog).
- **N6** `queue.go:98,112,129` mengembalikan `apperror.Internal(err)` dengan `err` nil (penyebab hilang); retur tanpa tanggal ikut setiap `CloseDay`.
- **N7** Post: cek-lalu-insert tanpa lock (`posting.go:82-100`) -> duplikat kembali sebagai konflik, bukan idempoten; cek periode di luar transaksi insert (`posting.go:150`) -> SafeClose bisa menyalip; `Reverse` = insert cermin + `MarkReversed` terpisah, tanpa CAS (`posting.go:194-200`); `InsertCostRow` INSERT polos (`ledger.go:63`) -> dua Sync bersamaan = 500.
- **N8** Biaya: baris biaya dibuat lalu jurnal di transaksi terpisah (`expenses.go:63-82`) -> baris `posted` tanpa jurnal bila jurnal gagal; handler cancel tak menerima alasan (`handler.go:427-440`); tabel tak punya kolom alasan/waktu batal (`000016`).
- **N9** Dashboard `Kas` hanya akun 1100 (kas tunai), bank diabaikan (`contracts/client.go:64`, `dashboard/service.go:73-78`).

---------------------------------------------------------------------------------------------------
## 1. Keputusan desain lintas tugas

### 1.1 Konvensi
- Builder memakai **kunci mapping**, bukan kode: `rawLeg{key:"receivable", ...}`; `finish()` meresolve lewat `repo.MappingAccount(key)` (sudah cache, `chartcache.go`). Helper baru `s.acct(ctx,key)`. Laporan memakai helper yang sama.
- Satu fungsi pemetaan eksplisit per jenis dokumen (tabel di tiap tugas), diuji unit murni (tanpa DB) + integrasi.
- Atomisitas = idempotensi + CAS (keputusan 3): tiap transisi (post, reverse, tutup periode, sync costing) memakai unique key / `UPDATE ... WHERE status=? ` + cek RowsAffected / lock baris periode; BUKAN UnitOfWork lintas modul.
- Kode database/sql gaya yang ada; migrasi golang-migrate; nomor migrasi akhir ditetapkan agen schema (di sini diberi label `M-A..M-H`).

### 1.2 Migrasi yang dibutuhkan (konsolidasi; final oleh agen schema)
| Label | Tabel | Perubahan | Dipakai tugas |
|---|---|---|---|
| M-A | `finance_journal_entries` | `ADD scope TINYINT NOT NULL DEFAULT 0` (0 komersial, 1 fiskal); `ADD requeue TINYINT NOT NULL DEFAULT 1`; `ADD source_active TINYINT GENERATED ALWAYS AS (CASE WHEN status='posted' AND source_doc_type IS NOT NULL THEN 1 END) VIRTUAL`; `DROP INDEX uq_finance_journal_source`; `ADD UNIQUE uq_finance_journal_source (branch_id, source_doc_type, source_doc_id, source_active)`; backfill `scope=1` untuk `tax_adjustment` dan cerminnya | 4,5,12,14,15 |
| M-B | `finance_journal_lines` | `ADD CONSTRAINT chk_finance_lines_one_side CHECK ((debit>0 AND credit=0) OR (credit>0 AND debit=0))`; indeks `ix_finance_lines_account_party (account_id, party_id, entry_id, debit, credit)` (menggantikan `ix_finance_lines_account_party`), `ix_finance_lines_account_product (account_id, product_id, variant_id, entry_id, debit, credit)` | 4,14 |
| M-C | baru `finance_account_daily` | `(branch_id, account_id, day DATE, scope TINYINT, debit BIGINT, credit BIGINT, PK(branch_id,account_id,day,scope))` rollup harian; backfill `INSERT..SELECT` dari lines | 14,12 |
| M-D | baru `finance_party_balances` | `(branch_id, account_id, party_id, debit, credit, PK(...))` rollup seumur hidup (scope 0) | 14,12,13 |
| M-E | baru `finance_cost_positions` + `finance_cost_state` | posisi biaya per `(branch,product,variant)`: `qty DECIMAL(24,12), total DECIMAL(24,4), last_unit_cost DECIMAL(18,4), updated_at`; kursor per cabang `(branch_id PK, last_movement_id)` | 11,14 |
| M-F | `finance_inventory_cost_movements` | `ADD moved_at DATETIME(6) NULL` (tanggal movement stok, untuk tanggal jurnal selisih stok), `ADD INDEX ix_cost_ref_type (branch_id, ref_type, stock_movement_id)` | 3 |
| M-G | baru `finance_cutover` | `(branch_id, doc_type, covered_through_id BIGINT UNSIGNED, set_by, set_at, PK(branch_id,doc_type))` — tabel kecil (≤ cabang×6 baris) | 10 |
| M-H | `business_expenses` | `ADD cancel_reason VARCHAR(255) NULL, cancelled_at DATETIME(6) NULL, cancelled_by BIGINT UNSIGNED NULL`; indeks `(branch_id, expense_date)` | 12 |
| (modul lain) | `salesreturn`: `sales_return_replacement_items ADD tax_base BIGINT NOT NULL DEFAULT 0, tax_amount BIGINT NOT NULL DEFAULT 0`; `sales_orders ADD completed_at DATETIME(6) NULL`; `stock_movements ADD unit_cost DECIMAL(18,4) NULL` (saldo awal berbiaya); `business_expenses` di atas milik finance | 2,11,13 |

### 1.3 Antarmuka (kontrak) yang dibutuhkan dari modul lain
(ditulis sebagai signature; pemilik modul mengimplementasi; finance hanya mengimpor `contracts/`)
- **payment**: `PaymentClient.ListCancelledIDs(ctx, branchID, afterID int64, limit int) ([]int64, error)`; `ListPostable(ctx, branchID, afterID int64, limit int) ([]PostableRef, error)` (id asc, status active, **Method** disertakan, tanggal); `OpenDocuments(ctx, branchID int64, side string, asOf string) ([]OpenDoc, error)` (outstanding per dokumen + tanggal dokumen + jatuh tempo, untuk aging/dashboard); tipe alokasi baru `sales_exchange` (tambah bayar retur tukar).
- **delivery**: `ListPostable(ctx, branchID, afterID, limit)` (status confirmed, kind apa pun, id asc); `ConfirmedByOrder(ctx, soID) ([]*DeliveryNote, error)` (untuk sisa pembulatan).
- **goodsreceipt**: `ListPostable(ctx, branchID, afterID, limit)`; `ByOrder(ctx, poID)`.
- **salesreturn / purchasereturn**: `Return` ditambah `ReturnDate`; `ListPostable(ctx, branchID, afterID, limit)` (status confirmed); `ListCancelledIDs`; `ListConfirmedByOrders(ctx, orderIDs)` (mengganti `ListByBranch` cap 10000 di payment); salesreturn: `ReplacementItem.TaxBase/TaxAmount`, `SalesReturn.ReplacementGrandTotal`; settlement `customer_credit`: `ListCreditSettlementsAfter(ctx, branchID, afterID, limit)`, `GetSettlement(ctx, id)`; `NetReturnAdjustment(ctx, branchID, from, to) int64` (untuk dashboard).
- **stock**: `CostMovement.Notes` dan `UnitCost` (hint saldo awal); `CriticalStock` menyaring lokasi aktif/bukan rusak dan memindai produk dengan keyset (`product.ListStockedAfter(afterID, limit)`); `CriticalCount`.
- **sales / purchasing**: `CountMissingTaxInvoice(ctx, branchID, from, to)` (+purchasing `CountMissingSupplierInvoice` ber-rentang); `PriorityOrders(ctx, branchID, today, limit)` (skor legacy dihitung di SQL modul masing-masing); `NetSalesCompleted(ctx, branchID, from, to) (count, total int64)`; `CountOrders(ctx, branchID, from, to)`; `TopProducts` hanya order `completed` dinetkan retur; `CancelOrder` ditolak bila ada SJ/penerimaan/pembayaran aktif.
- **product**: `PurchaseBaseCostsByIDs(ctx, ids) map[int64]float64` (harga ÷ faktor UOM beli); `ListStockedAfter`; `CountTracked`.
- **user**: pakai `UserClient.GetBranchAccess(ctx,userID)` (sudah ada) untuk guard laporan "semua cabang".
- **finance (ke luar)**: `FinanceClient.BalancesByKey(ctx, branchIDs, keys)`, `ReverseForSource(ctx, actorID, branchID, docType, docID, reason) (bool, error)`, `OutstandingByParty(ctx, branchIDs, side) (positive, negative int64)`; konstanta `CodeCash..` dihapus.

### 1.4 Urutan pengerjaan (dependensi)
1. F2b-8 (COA) + F2b-7 (mapping) → fondasi nama akun/kunci.
2. M-A/M-B + F2b-4 (reverse, unique, scope) → F2b-6 (guard) → F2b-5 (auto-balik).
3. F2b-14a (posisi biaya, kursor, rollup) → F2b-11 (HPP) → F2b-3 (selisih stok) → F2b-1 (payment) → F2b-2 (retur tukar).
4. F2b-9 + F2b-10 (antrean berkursor, cutover, gate).
5. F2b-12 (laporan, biaya, izin) → F2b-13 (dashboard/reporting) → F2b-15.

---------------------------------------------------------------------------------------------------
## F2b-8 — COA: peta kode legacy → revamp (Ukuran: M) [dikerjakan paling awal]

**Masalah/bukti [TERBUKTI]** Legacy (`database/seed-runner.ts:17-37` + `migrations/050_purchase_returns.sql:129`): 1000 Kas, 1010 Bank, 1100 Piutang, 1200 Persediaan, 1300 PPN Masukan, 2000 Hutang, 2100 PPN Keluaran, 2200 Uang Muka Customer, 2300 Uang Muka Supplier (aset), 3000 Modal/Saldo Awal, 4000 Penjualan, 5000 HPP, 5100 Selisih Stok, 5200 Pembulatan, 5300 Selisih Retur Pembelian, 6000-6050 & 6900 beban. Revamp (`finance/application/service.go:38-51`): 1100 Kas, 1110 Bank, 1200 Piutang, 1300 Persediaan, 1400 PPN Masukan, 2100 Hutang, 2200 PPN Keluaran, 6100/6200. Kode sama beda arti (1100, 1200, 1300, 2100, 2200).

**Rekomendasi (alasan):** revamp **mengikuti kode legacy** (peta identitas). Alasan: (1) pengguna/akuntan sudah hafal; (2) ETL saldo awal & jurnal tahun berjalan menjadi pemetaan identitas, tanpa risiko "kode sama beda arti" saat input manual; (3) revamp belum produksi (stage K/L belum jalan) sehingga biaya perubahan hanya seed + ±50 referensi kode di tes. Tetap sediakan peta eksplisit berbasis `system_key` legacy → kunci mapping untuk ETL, agar akun kustom produksi (yang mungkin berubah kode) tetap benar.

| Kode final | Nama | Tipe | is_cash | Kunci mapping | Legacy system_key | Kode revamp lama |
|---|---|---|---|---|---|---|
| 1000 | Kas Tunai | aset | ya | cash | cash_cash | 1100 |
| 1010 | Rekening Bank | aset | ya | bank | cash_bank | 1110 |
| 1100 | Piutang Usaha | aset | - | receivable | accounts_receivable | 1200 |
| 1200 | Persediaan Barang | aset | - | inventory | inventory | 1300 |
| 1300 | PPN Masukan | aset | - | ppn_in | input_tax | 1400 |
| 2000 | Hutang Usaha | kewajiban | - | payable | accounts_payable | 2100 |
| 2100 | PPN Keluaran | kewajiban | - | ppn_out | output_tax | 2200 |
| 2200 | Uang Muka Pelanggan | kewajiban | - | customer_advance (BARU) | customer_advance | - |
| 2300 | Uang Muka Supplier | aset | - | supplier_advance (BARU) | supplier_advance | - |
| 3000 | Modal Awal / Saldo Awal | modal | - | opening_equity (BARU) | opening_equity | 3000 |
| 4000 | Penjualan Barang | pendapatan | - | revenue | sales_revenue | 4000 |
| 4100 | Retur Penjualan (kontra) | pendapatan | - | sales_return | (legacy mendebit 4000) | 4100 |
| 5000 | Harga Pokok Penjualan | beban | - | cogs | cogs | 5000 |
| 5100 | Selisih Stok | beban | - | adjustment, writeoff | stock_difference | 6200/6100 |
| 5200 | Pembulatan/Selisih Posting | beban | - | rounding_difference (BARU) | rounding_difference | - |
| 5300 | Selisih Retur Pembelian | beban | - | purchase_return_difference (BARU) | purchase_return_difference | 6200 (hard-code) |
| 6000/6010/6020/6030/6040/6050/6900 | Gaji, Listrik&Air, Sewa, Internet&Telepon, Transport&BBM, Admin Bank, Lain-lain | beban | - | expense → 6900 (default saran UI) | expense_* | 6000 |

Catatan: `writeoff` & `adjustment` default ke 5100 (paritas legacy: satu akun selisih stok) dan boleh dipisah kemudian; 6100/6200 dihapus dari seed. 4100 tak punya saldo legacy (ETL: tak ada baris). Total kunci mapping: 13 → 18.

**Rancangan perubahan**
- `finance/application/service.go`: `DefaultChart()` ganti tabel di atas; `mappingKeys` + `mappingTypes` tambah 5 kunci (tipe: customer_advance=kewajiban; supplier_advance=aset; opening_equity=modal; rounding_difference & purchase_return_difference=beban).
- `EnsureChart`: jangan menimpa nama akun yang sudah berjurnal; bila kode ada tetapi **tipe berbeda** dari seed → error eksplisit (cegah reseed merusak). Pada DB pra-cutover tanpa jurnal: migrasi reseed aman. Tambahkan perintah `cmd/devseed` reset COA (cek: jalankan hanya bila `finance_journal_lines` kosong).
- Peta ETL: berkas `apps/api/internal/modules/finance/application/legacymap.go` berisi `LegacySystemKeyToMappingKey` (cash_cash→cash, cash_bank→bank, accounts_receivable→receivable, input_tax→ppn_in, accounts_payable→payable, output_tax→ppn_out, sales_revenue→revenue, stock_difference→adjustment, dst.) dan aturan: akun legacy non-sistem dibuat sebagai akun baru dengan kode sama bila bebas, jika bentrok → `L-<kode>` dan dilaporkan.
- Perbarui ±50 literal kode di tes (`integration/*_test.go`; sebaiknya helper `codeOf(t, key)` yang membaca mapping) dan frontend `finance.schema.ts`, `AppLayout.tsx` (cek referensi kode).

**Reuse:** `EnsureAccount` (upsert), `SetMapping`, chart cache.
**Tes:** integrasi `coa_seed_test.go`: seed menghasilkan 18 kunci semua terpetakan ke akun aktif bertipe benar; reseed idempoten; reseed ditolak bila tipe berubah dan akun berjurnal; peta legacy mencakup semua `system_key` legacy.
**Volume:** nol (data kecil). **Dependensi:** tidak ada (kerjakan pertama).

---------------------------------------------------------------------------------------------------
## F2b-7 — Mapping akun benar-benar dipakai + guard (Ukuran: M)

**Masalah [TERBUKTI]** hanya cash/bank/receivable/payable/cogs dibaca (`builders.go:321-339`, `reports.go:144`); lainnya literal (daftar di §0 baris 7). Ubah mapping revenue/inventory tak berefek; `SetMapping` tanpa guard.

**Rancangan**
1. `rawLeg.code` → `rawLeg.key`; `finish()` meresolve key; hapus literal di `builders.go` (baris 176,185,187,252,268,269,278,281,400,401,409,418,420,448,450,471,480,482).
2. Laporan: `analytics.go:48-56` revenue/sales_return/cogs via `s.acct`; `partyBalances` menerima key (`receivable`/`payable`) bukan kode (`analytics.go:133,138`); `reports.go:342-346` (ppn_out, ppn_in), `:396-400` (revenue, ppn_out).
3. `contracts/client.go`: hapus `CodeCash..CodePayable`; tambah `BalancesByKey(ctx, branchIDs, keys)` (1 query rollup); dashboard `Summary` memakai kunci `cash`,`bank`,`receivable`,`payable`,`inventory` dan menampilkan "Kas & Bank" = cash+bank (N9).
4. Guard `SetMapping` (`service.go:301-327`): bila mapping lama ada, akun berbeda, dan `repo.AccountInUse(oldAccountID)` → `Conflict("Mapping '<key>' tidak bisa diubah: akun lama sudah dipakai jurnal. Buat akun baru untuk transaksi ke depan")` (teks legacy `finance.service.ts:277-290`). Pengecualian: pemetaan awal (belum ada).
5. Kelengkapan: `Service.MissingMappings(ctx)` dipakai Readiness (F2b-10) dan Post (pesan jelas, sudah ada via `mustMap`).
6. UI `AccountsPage` panel mapping: tampilkan 18 kunci + "dipakai oleh" (teks statis). Ukuran S.

**Contoh:** ubah `revenue` dari 4000 ke 4010 SEBELUM ada jurnal → SJ berikutnya mengkredit 4010 (sebelum perbaikan tetap 4000). Setelah ada jurnal di 4000 → ditolak.
**Tes:** unit `builders_test.go` (kunci→akun custom); integrasi: remap lalu post SJ, assert akun; remap ditolak setelah berjurnal; laporan margin/PPN mengikuti mapping.
**Volume:** nol. **Dependensi:** F2b-8.

---------------------------------------------------------------------------------------------------
## F2b-4 — Reverse menyalin dimensi, repost setelah reverse, laporan konsisten (Ukuran: L)

**Masalah/bukti [TERBUKTI + N1,N2,N7]**
- `Reverse` hanya menyalin akun/debit/kredit (`posting.go:188-191`); `ProductFootings`/`PartyFootings` (`journals.go:355-435`) tanpa filter status → asli terhitung, cermin tidak. Contoh: SJ 100.000 (Dr 1100 111.000 pihak A, Cr 4000 100.000 produk P, Cr 2200 11.000; HPP 60.000 produk P). Setelah di-reverse: margin P = 100.000−60.000 = 40.000 (seharusnya 0) dan piutang pihak A = 111.000 (seharusnya 0).
- `EntryBySource` (`journals.go:104-117`) dan `PostedSourceIDs` (:474-492) tak filter status; unique (000016:62) → `Post` ulang mengembalikan jurnal yang sudah dibalik; antrean menganggap sudah posted.
- N1: cermin jurnal fiskal bocor ke buku komersial. N2: unique tanpa branch.

**Rancangan**
1. Migrasi M-A, M-B.
2. `Reverse(ctx, actor, branch, entryID, reason, opts ReverseOptions{IgnoreSource bool})`:
   - cermin menyalin `ProductID, VariantID, PartyID`, deskripsi diawali "Balik – ", `scope` mewarisi asli, sumber `reversal`/`orig.ID` (penanda), tanggal = hari ini (Jakarta).
   - satu transaksi di repo `ReverseEntry(ctx, orig, mirror)`: nomor jurnal dialokasikan DALAM transaksi (F2b-12h), insert cermin + rollup, `UPDATE finance_journal_entries SET status='reversed', reversed_by=?, requeue=? WHERE id=? AND status='posted'` → `RowsAffected==1` jika tidak `Conflict("Jurnal sudah dibalik")` (CAS).
   - `requeue = 0` bila `IgnoreSource` atau sumber bukan dokumen; default 1 (dokumen kembali ke antrean seperti `cancel-posting next_status=ready` legacy `finance-posting.service.ts:691-737`).
3. `EntryBySource` → `AND status='posted'`; `PostedSourceIDs` diganti `PostedAmong(ctx, branch, type, ids)`: `status='posted' OR (status='reversed' AND requeue=0)` (dipakai antrean berkursor, F2b-9).
4. `Post`: bungkus `doclock.Lock("post:"+type+":"+id)`; bila insert gagal duplikat → ambil entri existing dan kembalikan (idempoten, bukan 409).
5. `commercialOnly` (`journals.go:296`) diganti `AND e.scope = 0`; `TaxAdjustmentFootings` memakai `scope=1`. `CreateTaxAdjustment` tetap sumber `tax_adjustment` + seq (kini unik per cabang berkat M-A).
6. Laporan TIDAK perlu filter status: asli + cermin net nol per dimensi. (Audit meminta filter; terbantah sebagai kebutuhan, diganti salin-dimensi.) Daftar jurnal tetap menampilkan keduanya; UI `JournalDetailPage` menampilkan tautan cermin (sudah ada `reversedBy`).
7. Repost: setelah reverse, dokumen muncul lagi di antrean; `Post` membuat jurnal baru (nomor baru) dari data dokumen saat ini.

**Reuse:** `MarkReversed` (diganti CAS), `insertEntry`, `nextNumber`, audit log.
**Tes integrasi** (`finance_reverse_test.go`, MySQL nyata):
- reverse SJ → `MarginByProduct`=0 baris/nol, `Receivables` pihak A = 0 (sebelum: 40.000 dan 111.000).
- repost: antrean memuat SJ setelah reverse; `Post` → id baru; reverse kedua ditolak untuk id lama; `IgnoreSource` → tak muncul di antrean.
- reverse `tax_adjustment` → `ProfitLoss` komersial tak berubah (N1).
- tax adjustment pertama di dua cabang, dan di dua tahun → keduanya sukses (N2).
- 8 goroutine `Post` dokumen sama → 1 entri, semua kembalikan id sama; 8 goroutine `Reverse` → 1 sukses.
- CHECK DB: insert baris debit=credit>0 / keduanya 0 via SQL mentah ditolak.
**Volume:** indeks unique baru 4 kolom, nol dampak baca. **Dependensi:** F2b-8 (opsional), mendahului F2b-5/9/10. 

---------------------------------------------------------------------------------------------------
## F2b-6 — Guard status dokumen pada Post/Preview (Ukuran: S)

**Bukti [TERBUKTI]** `builders.go:205-289` (SJ), `:362-426`, `:432-489` tanpa cek status; draft memakai rata-rata hidup (:256-265).
**Rancangan**
- `buildDelivery`: `note.Status == confirmed` else `Conflict("Surat jalan belum dikonfirmasi atau sudah dibatalkan")`. `buildSalesReturn`/`buildPurchaseReturn`: `Status == confirmed`. `buildGoodsReceipt`: ada (tanpa siklus). `buildPayment`: aktif (ada) dan `Method != offset` (F2b-1).
- Hapus fallback `averageCost()` pada SJ/retur (`builders.go:259-265,391-396,462-467`): setelah `drainCosting` bila `MoveValues` kosong → `Conflict("Biaya persediaan dokumen belum tercatat — sinkronkan costing")`. (Fallback hanya relevan data pra-migrasi 000030; belum ada produksi.)
- `Preview` memakai builder yang sama → ikut tervalidasi.
**Tes:** post SJ draft, SJ batal, retur draft/batal → Conflict; SJ terkonfirmasi sukses. Contoh numerik: SJ draft 2 pcs avg 10.000: sebelum = HPP 20.000 dijurnal dari rata-rata hidup; sesudah = ditolak.
**Dependensi:** tidak ada.

---------------------------------------------------------------------------------------------------
## F2b-14a — Posisi biaya, kursor, rollup saldo (Ukuran: L) [fondasi performa; dikerjakan sebelum F2b-11]

**Bukti [TERBUKTI]** tiap `Post` bertipe SJ/retur memanggil `drainCosting` (≥2 `Sync`) (`posting.go:92,121-130`); `Sync` menjalankan `MaxCostMovementID` (MAX atas unique global, saring cabang) dan `CostPositions` (GROUP BY seluruh ledger cabang) (`costing.go:40-48`, `ledger.go:42,116`); fallback `averageCost` memanggil `CostPositions` per baris (`builders.go:260,392,463`). `PostBatch` 50 dokumen dalam WriteTimeout 30 dtk. Dashboard/laporan menjumlah seluruh sejarah tiap muat (`service.go:460,497`, `reports.go:223,289`, `journals.go:308,410`), 7 request paralel.
**Perkiraan sebelum/sesudah** (ledger 500k baris/cabang, ±0,25 dtk per GROUP BY): PostBatch 50 SJ = 50×2×0,25 = 25 dtk (≈ timeout). Sesudah: 1 Sync/batch + lookup posisi per kunci = <100 ms.

**Rancangan**
1. **Posisi biaya persisten** (M-E): `Sync` berjalan dalam satu transaksi per batch: insert baris ledger (`INSERT ... ON DUPLICATE KEY UPDATE id=id` agar idempoten) + `UPSERT finance_cost_positions` (qty/total/last_unit_cost) + update kursor `finance_cost_state`. Kunci `doclock.Lock("fincost:"+branch)` (satu container) + unique key sebagai pengaman DB (N7). `averageCost()` membaca satu posisi (`PositionFor`).
2. `Post` memanggil costing **malas**: `needsCosting` → bila `MoveValues(ref)` sudah ada, lewati Sync; `PostBatch` melakukan satu `Sync` di awal.
3. **Rollup saldo harian** (M-C) diperbarui di transaksi `CreateEntry`/`ReverseEntry`: `INSERT INTO finance_account_daily ... ON DUPLICATE KEY UPDATE debit=debit+VALUES(debit), credit=credit+VALUES(credit)` (urut account_id agar tanpa deadlock). Granularitas harian cukup karena semua laporan berbatas hari penuh. Ukuran ≈ cabang×akun-tersentuh×365 ≈ puluhan ribu baris/tahun (kecil vs jutaan baris lines).
4. **Saldo pihak** (M-D) untuk `PartyFootings` as-of hari ini; as-of tanggal lampau memakai pemindaian lines berindeks (M-B).
5. Pembaca dialihkan: `AccountFootings`, `DailyFootings` (tanpa `DATE_FORMAT`), `AccountBalances`, `BalancesByKey`, `TrialBalance`, `ProfitLoss`, `CashSummary`, `TaxSummary` → `finance_account_daily`. `ProductFootings` tetap dari lines (indeks M-B).
6. Alat integritas: `Service.VerifyRollups(ctx, branchID)` (bandingkan SUM lines vs rollup, posisi vs ledger) + endpoint `GET /api/v1/finance/integrity` (finance.manage); `RebuildRollups` untuk ETL. Readiness hanya memakai cek murah (total debit=kredit per cabang dari rollup).
7. Alternatif cache dengan invalidasi **tidak dipilih** (rollup lebih sederhana, tak ada state in-memory yang bisa basi).
**Tes:** integrasi `rollup_test.go` — setelah tiap skenario (post, reverse, expense, opening, tax adjustment) `VerifyRollups` kosong; benchmark ringan `BenchmarkPostBatch50` (target <3 dtk, 20k baris ledger sintetis); `Sync` dua goroutine → tanpa error, tanpa duplikat.
**Dependensi:** M-A..M-E dari agen schema.

---------------------------------------------------------------------------------------------------
## F2b-11 — HPP: tahan "menunggu data", biaya saldo awal eksplisit, ETL (Ukuran: L)

**Bukti [TERBUKTI + N3,N4,N5]** `builders.go:266` membuang baris HPP nol; `costing.go:109-113` out tanpa riwayat = 0 estimated; opening dari harga katalog saat Sync (`costing.go:95-101`), selalu estimated; template stok awal tanpa harga (`stock/application/opening.go:21`: kolom `product_code,variant_code,location_code,qty`); UI uncosted tak ada. Legacy: tahan "Menunggu data" + fallback harga beli master (`finance-inventory-cost.service.ts:405-415`), average_cost eksplisit (:70-143).

**Rancangan**
1. **Rantai biaya** untuk movement keluar/masuk non-GR: (a) nilai tercatat; (b) rata-rata posisi bila qty>0; (c) `last_unit_cost` posisi (N4: retur ke posisi kosong memakai biaya terakhir, bukan 0); (d) harga beli katalog ÷ faktor UOM beli (`product.PurchaseBaseCostsByIDs`, N5), ditandai estimated; (e) kosong → **tidak dijurnal**: `Conflict("Harga modal produk X belum tersedia")`; dokumen tetap di antrean berstatus `blocked: cost_missing` (bukan HPP 0 diam-diam). `buildDelivery`: hapus `if cogs != 0` untuk produk bertipe stok; HPP nol hanya jika produk non-stok.
2. `MoveValues` menyertakan `ref_type IN (t, t||'_rollback', t||'_cancel')` agar nilai netto (N3).
3. **Saldo awal berbiaya**: template stok awal + kolom `unit_cost` (opsional) → `stock_movements.unit_cost`; `CostMovement.UnitCost`; `costOf(opening)`: hint>0 → eksak, bukan estimated; tanpa hint → katalog (estimated, terlihat di uncosted).
4. **UI**: halaman "Biaya belum lengkap" memakai `GET /finance/costing/uncosted` (sudah ada, `handler.go:452`) + tautan ke produk; tampil di Posting & Readiness (hitungan). Ukuran S frontend.
5. **ETL** (rekomendasi): jangan memutar ulang `finance_inventory_cost_movements` legacy. Muat **posisi saat freeze** dari `finance_inventory_cost_states` (qty, average_cost, value) sebagai movement `opening` berbiaya eksplisit lewat impor stok awal; set kursor biaya ke movement terakhir sebelum cutover; rekonsiliasi: Σ(qty×avg) ≈ saldo akun persediaan pada jurnal saldo awal (selisih dilaporkan, toleransi pembulatan). Alasan: ledger revamp diturunkan dari movement stok dengan id baru; memuat baris legacy tak punya padanan `stock_movement_id`.
**Contoh N4:** beli 1 @10.000, jual 1 (HPP 10.000), retur 1. Sebelum: persediaan 0, HPP tetap 10.000, posisi qty 1 nilai 0 → jual ulang @15.000 ber-HPP 0 (margin 100%). Sesudah: retur Dr Persediaan 10.000/Cr HPP 10.000; jual ulang HPP 10.000, margin 5.000.
**Tes:** integrasi: retur ke posisi kosong; SJ dgn produk tanpa biaya → blocked + pesan; opening berbiaya eksplisit tidak estimated; rollback SJ lalu ulang → HPP 1x; produk UOM beli dus(12) harga 120.000 → biaya/satuan 10.000.
**Dependensi:** F2b-14a, F2b-6, kontrak stock/product.

---------------------------------------------------------------------------------------------------
## F2b-3 — Jurnal otomatis selisih stok / opname / barang rusak (Ukuran: M)

**Bukti [TERBUKTI]** `posting.go:16-19`; legacy `finance-posting.service.ts:134,1803,2742-2764` (event `stock_adjustment` untuk `manual_adjustment`, `damaged_write_off`, movement `adjustment`; Dr/Cr Persediaan vs Selisih Stok). Revamp: ref `adjustment` (`stock/application/transactions.go:330-335`), `write_off` (:496), `damaged` (:455-459, out+in setara → netral), `transfer` (antar cabang), `opening`.
**Rancangan**
- Sumber baru `SourceStockMovement="stock_movement"`, `source_doc_id = stock_movement_id`. **Antrean diturunkan dari ledger biaya milik finance sendiri** (anti-join SQL satu modul, bukan lintas modul): baris `finance_inventory_cost_movements` ber-`ref_type IN ('adjustment','write_off')` yang belum punya jurnal `stock_movement` aktif dan `stock_movement_id > cutover[stock_movement]`. Antrean memanggil `Sync` dulu (kursor murah).
- Builder `buildStockMovement`: nilai = |total_cost| baris ledger (eksak, bukan rata-rata hidup); tanggal = `moved_at` (M-F); dimensi produk/varian di kedua kaki.
  | Kejadian | Debit | Kredit |
  |---|---|---|
  | adjustment masuk (selisih lebih) | inventory | adjustment (5100) |
  | adjustment keluar (selisih kurang) | adjustment (5100) | inventory |
  | write_off (keluar dari lokasi rusak) | writeoff (5100 default) | inventory |
- Tidak dijurnal: `opening` (saldo awal), `damaged` (netral), `transfer` (paritas legacy; selisih biaya antar-cabang dicatat sebagai batas yang diakui di `costing.go:104-108`; opsi akun "persediaan dalam perjalanan" ditunda).
- Biaya nol/estimated → blocked (F2b-11).
- Readiness: peringatan "Persediaan buku vs ledger" = |GL(inventory, semua cabang) − Σ nilai posisi| > toleransi (default Rp1.000 + 0,001% nilai); per cabang informatif (transfer bisa membuat selisih). Dashboard menampilkan satu angka "Persediaan (buku)" + lencana selisih bila ada (`DashboardPage.tsx:186-188,507`).
**Contoh:** posisi P qty 50 @10.000 (500.000). Opname fisik 48 → Dr 5100 20.000 / Cr 1200 20.000; GL = ledger = 480.000 (sebelum: GL 500.000 vs ledger 480.000, selisih 20.000 tampil berdampingan). Opname +3 → Dr 1200 30.000 / Cr 5100 30.000. Write-off 5 pcs → Dr 5100 50.000 / Cr 1200 50.000.
**Tes:** integrasi: adjustment ± dan write_off menghasilkan jurnal & antrean; damaged-move & transfer tidak masuk antrean; idempoten; GL=ledger setelah serangkaian operasi; selisih >toleransi memunculkan peringatan; produk tanpa biaya → blocked.
**Volume:** anti-join berindeks `(branch_id, ref_type, stock_movement_id)`. **Dependensi:** F2b-11, F2b-4 (unique), M-F.

---------------------------------------------------------------------------------------------------
## F2b-1 — Pemetaan akun per jenis alokasi pembayaran (Ukuran: M)

**Bukti [TERBUKTI/SEBAGIAN]** lihat §0 baris 1. Payment: satu pembayaran satu arah; alokasi sejenis (`payment/application/service.go:309-322`); SettleCredit membuat dua leg `offset` (`:603-609`).
**Rancangan** (`finance/application/payment_kinds.go`, fungsi murni `classifyPayment`)
| Jenis | Alokasi | Arah | Debit | Kredit |
|---|---|---|---|---|
| Terima dari pelanggan | sales | in | cash/bank | receivable (pihak) |
| Refund ke pelanggan | sales_return | out | receivable (pihak) | cash/bank |
| Bayar supplier | purchase | out | payable (pihak) | cash/bank |
| Refund dari supplier | purchase_return | in | cash/bank | payable (pihak) |
| Tambah bayar retur tukar | sales_exchange | in | cash/bank | receivable (pihak) |
| Offset settle-credit | metode `offset` | - | **tidak dijurnal** | - |
- Validasi: arah harus sesuai jenis; alokasi campur → Conflict. Antrean mengecualikan `offset` (kontrak payment menyertakan `Method`). Alasan offset: retur sudah mengkredit Piutang/mendebit Hutang (`builders.go:420,448`); offset hanya alokasi sub-ledger.
- Settlement memo `customer_credit` (penjualan) → sumber `sales_return_settlement`: Dr receivable(pihak)/Cr customer_advance(pihak); sisi beli: Dr supplier_advance/Cr payable (paritas `finance-posting.service.ts:2609-2616,2660-2668`). Tipe `collect_payment/refund/reduce_receivable` TIDAK dijurnal (uangnya sudah lewat modul payment). Aturan pendamping (modul payment): retur yang dikreditkan sebagai saldo pelanggan tidak boleh dipakai lagi di SettleCredit melebihi sisa. Lihat Pertanyaan 3.
**Contoh numerik**
- Refund retur jual: tagihan 100 dibayar 100 (AR 0), retur 30 (AR −30), refund tunai 30. Sebelum: Dr 2100 30 (pihak pelanggan)/Cr Kas 30 → AR −30, Hutang debit 30. Sesudah: Dr 1100 30/Cr 1000 30 → AR 0, Hutang 0.
- Refund retur beli: PO 100 dibayar 100, retur 30 (Hutang −30), supplier refund 30. Sebelum: Dr Kas/Cr 1200 Piutang 30 (Hutang tetap −30, Piutang −30). Sesudah: Dr Kas/Cr 2000 30 → Hutang 0.
- Offset: tagihan 100, retur 30, offset 30. Sebelum: AR = 100 −30 (retur) −30 (leg offset) = 40, sub-ledger payment = 70; plus Hutang debit 30 palsu. Sesudah: AR 70 = sub-ledger 70.
**Tes:** integrasi `payment_kinds_test.go`: tiga skenario di atas + properti "setelah semua diposting, saldo GL piutang/hutang per pihak == `payment.GetPartyBalance().Outstanding`" untuk campuran jual/retur/refund/offset; offset tak masuk antrean & `Post` ditolak; alokasi campur ditolak.
**Dependensi:** F2b-7, F2b-4; payment: `Method` di kontrak, tipe `sales_exchange`.

---------------------------------------------------------------------------------------------------
## F2b-2 — Retur tukar: jurnal penjualan pengganti (Ukuran: M)

**Bukti [TERBUKTI]** `builders.go:218-220`, `queue.go:69-71`, tes `exchange_test.go:98-101`; legacy `finance-posting.service.ts:2412-2423` (Dr Piutang / Cr Pendapatan / Cr PPN Keluaran) + HPP `sales_return_exchange` (:1557).
**Rancangan**
- Pengakuan di **SJ pengganti dikonfirmasi** (konsisten dengan pengakuan pendapatan saat SJ). `buildDelivery` cabang `DocumentKind==replacement` → `buildReplacementDelivery`: baca `salesReturns.GetByID(note.SalesReturnID)`; pasangkan item SJ ke `ReplacementItems`; Dr receivable (pihak SO) = Σ(TaxBase+TaxAmount); Cr revenue per produk = TaxBase; Cr ppn_out = TaxAmount; HPP Dr cogs/Cr inventory per produk dari `MoveValues(branch,"delivery",noteID)` (SJ pengganti memakai ref `delivery`, `delivery/application/service.go:586`). Hapus skip replacement di antrean.
- Pajak pengganti mengikuti SO asal: `salesreturn.priceReplacements` menghitung `TaxBase/TaxAmount` dengan `lineEconomics(qty, harga, 0,0, so.TaxType, so.TaxRate)` dan menyimpannya (migrasi modul salesreturn); finance hanya membaca.
- Selisih bayar: tak ada jurnal tambahan (saldo piutang bersih = selisih); pembayaran selisih dialokasikan ke `sales_exchange` (payment) → jenis "terima" (F2b-1). Selisih kebalikan (pengganti lebih murah) = refund via alokasi `sales_return` dengan sisa refundable = max(0, Total − GrandTotal pengganti) − refund.
**Contoh (PPN 11% eksklusif):** SJ asli 1 pcs A: Dr AR 111.000, Cr 4000 100.000, Cr 2100 11.000, HPP 60.000. Retur A: Dr 4100 100.000, Dr 2100 11.000, Cr AR 111.000; Dr 1200 60.000/Cr 5000 60.000. Pengganti B 1 pcs harga 120.000 biaya 70.000. Sebelum: tak ada jurnal → pendapatan, PPN 13.200, AR 133.200, HPP 70.000 hilang; GL persediaan 70.000 lebih tinggi dari ledger. Sesudah: Dr AR 133.200 / Cr 4000 120.000 / Cr 2100 13.200; Dr 5000 70.000 / Cr 1200 70.000 → AR bersih pelanggan 22.200, laba kotor 120.000−70.000 = 50.000.
**Tes:** ubah `exchange_test.go:98-101` menjadi: Post SJ pengganti sukses, 5 kaki sesuai angka di atas, margin per produk, antrean memuat lalu mengosongkan, properti GL AR == sub-ledger; skenario tanpa pajak dan inklusif.
**Dependensi:** F2b-1, F2b-6, F2b-11, salesreturn (pajak pengganti), payment (`sales_exchange`).

---------------------------------------------------------------------------------------------------
## F2b-5 — Cancel dokumen setelah posting → balik jurnal (Ukuran: M)

**Bukti [TERBUKTI]** lihat §0 baris 5.
**Rancangan (tanpa tabel baru)**
1. **Otomatis**: kontrak `finance.SourceReverser.ReverseForSource(ctx, actor, branch, docType, docID, reason) (reversed bool, err)` — idempoten: `EntryBySource(status posted)` → `Reverse(requeue=0)`; tanpa jurnal → `(false,nil)`. Modul penerima di-wiring via setter di composition root (pola `SetPayments`). Dipanggil SESUDAH status dokumen berubah: `payment.Cancel`, `salesreturn.Cancel` (cabang confirmed), `purchasereturn.Cancel` (cabang confirmed). Kegagalan hanya dicatat (audit `finance.autoreverse_failed`), pembatalan tetap sah → jaring pengaman di bawah.
2. **Jaring pengaman** `Orphans(ctx, branch)`: per jenis, `ListCancelledIDs` (keyset, set kecil) ∩ `PostedAmong(status posted)` → item antrean `kind:"reverse"`; `PostBatch` mendukung aksi `reverse`; Readiness: blocker "N jurnal dokumen batal belum dibalik".
3. Penegakan di sumber (modul lain): `CancelOrder` SO/PO ditolak bila ada SJ/penerimaan terkonfirmasi atau pembayaran aktif.
**Contoh:** pembayaran 100 diposting (Dr Kas 100/Cr AR 100) lalu dibatalkan. Sebelum: kas +100 & AR −100 tetap, sub-ledger menunjukkan tagihan terbuka 100 (selisih 100). Sesudah: cermin → kas/AR kembali; entri `requeue=0`.
**Tes:** cancel payment/retur → cermin ada & saldo kembali; hook gagal (reverser dimatikan) → `Orphans` mendeteksi → `PostBatch(reverse)` memperbaiki; panggilan ganda idempoten; periode bulan itu tertutup → cermin tetap dibuat di hari ini (periode terbuka).
**Dependensi:** F2b-4; kontrak `ListCancelledIDs`.

---------------------------------------------------------------------------------------------------
## F2b-9 — Batas diam-diam (Ukuran: L)

| Lokasi | Perbaikan |
|---|---|
| Antrean 200/jenis (`queue.go:33,61-139`) + `PostedSourceIDs` memuat semua id (`journals.go:474`) | Pindai berkursor menaik per jenis lewat `ListPostable(afterID,limit=1000)`; per halaman satu query `PostedAmong(ids)` (IN berindeks unique baru); mulai dari `covered_through_id` (F2b-10). Respons: `data` tetap array item (UI kompatibel) + `meta` {page,limit,total,total_pages} dengan `total` eksak per filter `docType`. `Readiness.Unposted` = total eksak, bukan `len(queue)` (`readiness.go:48`). Lonjakan awal terkendali: biaya O(dokumen sejak cutover / 1000) query |
| GL `ListEntries` 10000 + urut id (`reports.go:266,271`) | Query jendela: `SUM(debit-credit) OVER (ORDER BY entry_date,id,line_id)` di SQL, `ORDER BY entry_date,id`, `LIMIT/OFFSET` (paginasi `page/limit` + total). Saldo awal dari rollup (F2b-14a) |
| TaxDetail 10000 (`reports.go:392`) | Agregasi SQL per entri (GROUP BY, `CASE` akun revenue/ppn_out/ppn_in), dipaginasi; tanpa muat semua |
| `ListByBranch` retur 10000 dipakai saldo pihak & antrean (`salesreturn/application/service.go:89`, `purchasereturn:92`, `payment/application/service.go:718,766`) | Antrean: `ListPostable`; saldo pihak: `ListConfirmedByOrders(orderIDs)` (IN berchunk); hapus `ListByBranch` dari kontrak payment |
| `ListStocked LIMIT 5000` (`product/infrastructure/repository.go:452`) | `ListStockedAfter(afterID,limit)` + loop keyset di `stock.CriticalStock` |
| `TurnoverEntries` seluruh tahun (`turnover.go:36`) | SENGAJA company-wide (plafon per wajib pajak). Optimasi opsional: muat hanya entri berpendapatan dulu, entri pembayaran hanya bila ada order terkecualikan; indeks `(entry_date,id)`; ambang tinjau 200k entri/tahun |
| Bug kecil | `queue.go:98,112,129` bungkus `errPostedIndex` |
**Tes:** integrasi dgn 1.200 SJ sintetis (SQL langsung) → antrean total 1.200 (sebelum 200) dan Readiness memblok; GL 12.000 baris saldo berjalan akhir == footing akun; TaxDetail 10.500 SJ → 10.500 baris; retur >10.000 tetap masuk saldo pihak.
**Dependensi:** kontrak `ListPostable`, F2b-10, F2b-14a.

---------------------------------------------------------------------------------------------------
## F2b-10 — Antrean/Readiness vs migrasi, gate tutup periode (Ukuran: L)

**Keputusan terkunci 1:** saldo awal per cabang + jurnal tahun berjalan dimigrasi dari legacy; dokumen sebelum tahun berjalan ditandai "sudah dijurnal oleh saldo awal".
**Mekanisme tanpa tabel besar:** tabel kecil `finance_cutover(branch_id, doc_type, covered_through_id)` (M-G). Aturan: dokumen dengan `id <= covered_through_id[tipe]` dianggap sudah dijurnal (oleh saldo awal atau jurnal legacy yang dimigrasi) → tak pernah masuk antrean. Tipe: delivery, goods_receipt, payment, sales_return, purchase_return, stock_movement. ETL memuat dokumen "sudah dijurnal di legacy" lebih dulu (id kecil) lalu yang belum dijurnal; batas = id terakhir kelompok pertama. Prasyarat gerbang cutover (checklist ETL): legacy tanpa sumber `ready/pending/failed` (readiness legacy `finance-close.service.ts:46-65`) sebelum freeze. Admin: `PUT /api/v1/finance/cutover` (finance.manage, idempoten; ditolak bila sudah ada jurnal posting-an revamp dgn id ≤ batas) + tampil di wizard Saldo Awal.
**Jurnal tahun berjalan dari legacy:** dimuat apa adanya dengan `source_doc_type='legacy_journal'`, `source_doc_id`=id jurnal legacy (unik berkat M-A), status/`reversed_by` dipertahankan, dimensi produk/pihak dipetakan (id_order tak dibawa). Saldo awal = jurnal `opening` per cabang pada tanggal awal tahun.
**Gate (`readiness.go`)** — ubah ke bentuk `checks[]` (key,label,status ok|warning|failed,detail,count) + pertahankan field lama untuk UI:
1. Antrean tak terposting **per periode** (tanggal dokumen ≤ akhir bulan, tidak dalam periode yang sudah tertutup); dokumen di periode tertutup → `periodClosed:true`, dikecualikan dari `PostBatch/CloseDay` (tak lagi macet di `posting.go:150`), muncul sebagai peringatan; opsi posting susulan (Pertanyaan 4).
2. Faktur kosong → **warning** (paritas legacy `finance-close.service.ts:104-130`), terhitung per bulan (kontrak berrentang; sekarang seumur hidup).
3. Persediaan negatif (posisi qty<0 atau nilai<0) → failed (paritas `:140-154`).
4. Jurnal tak seimbang, neraca tak seimbang (`BalanceSheet.Balanced`), pemetaan lengkap (F2b-7), yatim batal (F2b-5), dokumen blocked biaya (F2b-11) → failed; selisih persediaan buku vs ledger → warning.
5. Tutup aman atomik (CAS, keputusan 3): `SafeClose` memegang baris `finance_periods` `FOR UPDATE` dan mengevaluasi ulang antrean di transaksi yang sama; `insertEntry` mengambil baris periode `FOR SHARE` (INSERT IGNORE baris `open` lebih dulu) dalam transaksi insert → tak ada posting menyalip.
6. `finance.close.force`: `POST /finance/periods/close` (tutup tanpa gerbang) dilindungi izin baru ini (sekarang `finance.manage`, `handler.go:530`).
**Contoh:** 250 SJ terkonfirmasi, 200 terbaru sudah diposting, 50 lama gagal. Sebelum: antrean memuat 200 terbaru → semua terposting → `Unposted=0` → tutup LOLOS (50 SJ tak pernah terjurnal). Sesudah: total eksak 50 → blocker.
**Tes:** cutover: dokumen id ≤ batas tak masuk antrean, > batas masuk; gate per periode; dokumen di periode tertutup tak memblok bulan lain & tak membuat `PostBatch` gagal; race `SafeClose` vs `Post` (goroutine) → entri tak mendarat di periode tertutup; izin force.
**Dependensi:** F2b-9, F2b-4, F2b-5, F2b-11, kontrak sales/purchasing berrentang.

---------------------------------------------------------------------------------------------------
## F2b-12 — Laporan, biaya usaha, penomoran, pembulatan, izin (Ukuran: L; sub-tugas)

**12a Neraca saldo kumulatif** (legacy `finance-reporting.service.ts:837-910`): `GET reports/trial-balance?asOf=` → per akun debit_total, credit_total, saldo debit/kredit, total, `is_balanced`; mode `year&month` lama tetap (mutasi). Dari rollup. S.
**12b Laporan kas** (legacy :152-206; revamp `reports.go:565-593` mutasi 1 bulan): `cash-summary?from=&to=` → saldo awal (sebelum from), masuk (Σ debit), keluar (Σ kredit), saldo akhir per akun `is_cash`. S.
**12c Aging per dokumen** (legacy `partyBalanceReport` :545-557; revamp `analytics.go:147-189` per pihak): `reports/receivables|payables?asOf=&view=documents` dari `payment.OpenDocuments` (outstanding per dokumen; umur = asOf − tanggal dokumen, paritas bucket 0-30/31-60/61-90/>90, ditambah hari lewat jatuh tempo); ringkasan bucket per pihak dari buku. Pembulatan: GL pihak vs sub-ledger dilaporkan sebagai "selisih rekonsiliasi". M.
**12d Semua cabang** (keputusan 2): parameter `branch=all|<id>` pada semua laporan; handler memotong dengan `UserClient.GetBranchAccess(userID)` (cabang di luar akses ditolak; `all` = gabungan cabang yang diizinkan); repo menerima `branchIDs []int64` (`IN (...)`; satu cabang → `=`). Perubahan di `journals.go:222,270,309`, `handler.go:34-39`. M.
**12e Rincian PPN** (legacy :329-431, 1119-1219): baris untuk SJ **dan** retur jual (negatif) **dan** penerimaan (PPN masukan, DPP, no. faktur supplier); daftar pengecualian faktur kosong dari kontrak sales/purchasing; dipaginasi (F2b-9). M.
**12f Buku besar urut tanggal**: `ORDER BY entry_date,id,line_id` (F2b-9). S.
**12g Biaya usaha** (`expenses.go`): pembayar harus `aset && is_cash` aktif (legacy `business-expense.service.ts:226-229`; flag `is_cash` menggantikan tabel kas); satu transaksi baris biaya + jurnal (`repo.CreateExpenseWithEntry`, memakai `insertEntryTx`) — hilangkan N8; cancel wajib `reason` (handler + kolom M-H); filter list `q,from,to,status,expenseAccountId,payAccountId`. M.
**12h Jurnal**: `GET journals?q=&sourceType=&status=` (nomor/memo); nomor jurnal dialokasikan **di transaksi insert** (`NextSequence` ke dalam tx `CreateEntry`) agar tak ada celah nomor bila insert gagal (`posting.go:154`, `service.go:430-443`). S.
**12i Sisa pembulatan prorata** (`builders.go:127-132`): SJ/penerimaan yang menutup baris (kumulatif qty ≥ qty baris) mengambil sisa = nilai baris − Σ prorata SJ sebelumnya (dihitung ulang deterministik dari `delivery.ConfirmedByOrder`/`goodsreceipt.ByOrder`). Contoh: bersih 100.000, 3 SJ @1/3: sebelum 33.333×3 = 99.999 → piutang −1 setelah pelunasan 100.000; sesudah 33.333 + 33.333 + 33.334 = 100.000. Berlaku untuk pendapatan dan pajak. M.
**12j Izin**: tambah `finance.close.force`, `finance.journal.reverse` (pisah dari manage, dipakai route reverse), `finance.report.all_branches` (opsional); tabel pemetaan 14 kode legacy → kode revamp di `contracts/client.go:PermissionCatalog`; seed peran: owner/admin dapat semua, staf hanya view. S.
**Tes:** satu berkas per sub-tugas pada `integration/finance_reports_test.go` (angka tetap): neraca kumulatif Des→Jan, kas saldo awal/akhir, aging 4 bucket, cabang-semua dengan user terbatas ditolak, biaya dgn pembayar non-kas ditolak, 3 SJ pecah 100.000, nomor jurnal tanpa celah setelah kegagalan insert.

---------------------------------------------------------------------------------------------------
## F2b-13 — Dashboard & reporting (Ukuran: L)

| Item | Legacy | Revamp sekarang | Rancangan |
|---|---|---|---|
| Penjualan hari ini/bulan | order selesai incl. PPN + selisih retur, tanpa perlu posting (`dashboard.service.ts:244-273`) | omzet dari buku tanpa PPN, kosong sampai diposting (`dashboard/application/service.go:60-106`) | tambah `sales_orders.completed_at`; `NetSalesCompleted` + `NetReturnAdjustment` (retur tukar: pengganti − retur; return-only: −total); buku tetap untuk laba, diberi label "dari pembukuan" + lencana jumlah dokumen belum diposting |
| Piutang/utang | per order min 0 (:445-509) | saldo GL bisa negatif | `OutstandingByParty`: Σ saldo pihak positif sebagai Piutang/Hutang, saldo negatif sebagai "uang muka/kredit" (selisih definisi dari legacy didokumentasikan; Pertanyaan 5) |
| Prioritas | skor semua order pending/aktif termasuk PO (:331-399) | SO confirmed ≤7 hari, 50 terbaru (`operations.go:23,77-106`) | `PriorityOrders` per modul (SQL, skor legacy: lewat tempo 100, ≤2 hari 80, pending 60, lain 40, + min(umur,30)), dashboard menggabung SO+PO top-5 |
| Terlaris | order selesai dinetkan retur (:560-631) | draft+confirmed, tanpa retur, by order_date (`sales/infrastructure/aggregates.go:40-72`) | hanya status `completed` by `completed_at`, dikurangi qty retur terkonfirmasi (kontrak `ReturnedByProduct`) |
| Margin | tanpa penanda (:1039-1067) | tanpa penanda | `ProductMargin.CostIncomplete` bila produk punya baris biaya estimated/blocked |
| Widget hilang | jumlah order hari/bulan, total transaksi, stok kritis, produk terlacak | — | `CountOrders`, `CriticalCount`, `CountTracked` pada Summary |
| Stok kritis | — | semua lokasi (`stock/infrastructure/repository.go:259-264`) | join `stock_locations` aktif & bukan lokasi rusak (tabel milik stock); keyset produk (F2b-9) |
| Reporting | `orders` (filter tanggal/status/jenis + total) & `stock` (critical_only) (`reporting.service.ts:14-69`) | hanya sales-trend & inventory | `GET /reporting/orders`, `/reporting/stock?criticalOnly=` di modul reporting (komposisi kontrak sales/purchasing/stock) |
**Peleburan reporting↔finance tanpa memutus UI:** jangan hapus modul; UI hanya memakai `/reporting/sales-trend` dan `/finance/reports/inventory-value`. `reporting.Inventory` (`/reporting/inventory`, tak dipakai UI) ditandai deprecated lalu dihapus satu rilis kemudian; implementasi tunggal tetap di finance (`InventoryValue`). Modul reporting menjadi rumah laporan operasional (orders, stock, trend). Dashboard memakai `BalancesByKey` (F2b-7).
**Tes:** dashboard integrasi: penjualan hari ini muncul tanpa posting dan incl. PPN; prioritas menemukan SO jatuh tempo 90 hari lalu di antara 300 SO terbaru; terlaris mengurangi retur; stok kritis mengabaikan lokasi rusak; angka widget baru.
**Dependensi:** F2b-14a, F2b-7, kontrak sales/salesreturn/stock/product.

---------------------------------------------------------------------------------------------------
## F2b-15 — Paket pajak plafon (Ukuran: S–M)
- Aritmetika **terverifikasi**: akumulasi berurutan `running+rev <= 4.800.000.000` (`turnover.go:210`) identik dgn legacy layer 2 (`finance-reporting.service.ts` `runningOmzet + omzet <= LAYER_2_CAP_AMOUNT`); order omzet ≤0 tidak makan plafon (legacy `HAVING omzet > 0.009`). Beda SENGAJA yang diakui (komentar `turnover.go:11-19,169-173`): urutan kronologis (legacy CRC32 id_order), cakupan YTD perusahaan (legacy rentang diminta + id_branch opsional), atribusi via dokumen sumber (legacy `l.id_order`), pembayaran lintas order "straddling" dipertahankan dan dilaporkan.
- Perbaikan: (N10) entri `reversal` harus diatribusikan lewat entri asli: di `TurnoverEntries` LEFT JOIN entri asli bila `source_doc_type='reversal'` dan pakai tipe/id sumber asli; tanpa ini order yang jurnalnya dibalik tetap bernilai penuh atau negatif sepihak. Tes: SJ lalu reverse → omzet order 0.
- CHECK DB per baris (M-B). Keseimbangan per entri tak bisa CHECK di MySQL → tetap dijaga `CreateEntry` + `CountUnbalanced` + `VerifyRollups` (F2b-14a).
- Catat batas diakui: transfer antar-cabang tak dijurnal; entri fiskal (`scope=1`) tak masuk plafon.

---------------------------------------------------------------------------------------------------
## 2. Rekomendasi ETL jurnal tahun berjalan: bawa apa adanya vs hitung ulang
**Rekomendasi: bawa apa adanya (angka historis legacy), tanpa menghitung ulang.** Alasan: (1) angka bulan tertutup/terlapor (PPN, SPT) tak boleh berubah; (2) jurnal legacy per-peristiwa (order, movement, settlement) tak punya padanan 1:1 dgn jurnal gabungan revamp (unique `(type,id)`), menghitung ulang berarti mengganti seluruh buku dan kehilangan jejak audit; (3) selisih nyata hanya pada jurnal PO/SO **inklusif pajak** (angka legacy salah; revamp benar, `builders.go:134-141`) — tangani dengan laporan "jurnal legacy inklusif beda dari hitungan revamp" (per order: selisih persediaan/PPN/hutang) lalu koreksi lewat jurnal manual per bulan yang masih terbuka, bukan menimpa. Jurnal eksklusif sudah setara; jangan "memperbaiki" ke arah legacy. Catatan: contoh numerik legacy inklusif BELUM saya re-trace di kode legacy (mengikuti catatan pemilik/komentar revamp) — perlu sampel data produksi saat rehearsal (stage L).

---------------------------------------------------------------------------------------------------
## 3. Risiko
1. Realign COA menyentuh ±50 literal tes + frontend; risiko salah ganti → helper `codeOf` + tes seed.
2. Rollup harian/pihak memperkenalkan state turunan: drift bila ada jalur tulis lain → satu jalur tulis (`CreateEntry/ReverseEntry`), `VerifyRollups` di tiap tes integrasi, `RebuildRollups` untuk ETL.
3. Auto-reverse via hook: kegagalan parsial → jaring pengaman `Orphans` + blocker readiness.
4. Boundary cutover bergantung disiplin ETL (urutan id); kesalahan = dokumen lama terposting ganda atau terlewat. Mitigasi: `PUT cutover` menolak bila ada jurnal revamp di atas batas; laporan rekonsiliasi hitungan per tipe.
5. Blokir "biaya belum tersedia" dapat menahan banyak SJ awal produksi bila master harga beli kosong; sediakan halaman uncosted + impor saldo awal berbiaya sebelum go-live.
6. Perubahan bentuk Readiness/antrean harus kompatibel UI (field lama dipertahankan).
7. Migrasi `DROP/ADD UNIQUE` dan kolom generated pada tabel jurnal besar: lakukan saat sepi (online DDL, indeks virtual).
8. N5: bila harga katalog ternyata per satuan dasar, pembagian faktor akan MENGECILKAN biaya — konfirmasi dulu (Pertanyaan 6).

## 4. Pertanyaan terbuka (hanya yang butuh keputusan pemilik/akuntan)
1. COA revamp mengikuti kode legacy (rekomendasi) — setuju? Dampak: mengubah arti kode 1100/1200/1300/2100/2200 di revamp (belum produksi).
2. Jurnal legacy inklusif yang angkanya salah: dibiarkan + koreksi manual (rekomendasi) atau direstatemen untuk bulan yang belum dilaporkan?
3. Settlement `customer_credit`/kredit supplier: dijurnal reklasifikasi Piutang→Uang Muka (paritas legacy) atau cukup disajikan sebagai saldo negatif piutang di laporan?
4. Dokumen terkonfirmasi yang jatuh di periode sudah tertutup dan belum terposting: boleh "posting susulan" ke periode terbuka (memo "Susulan", izin finance.close.force) atau wajib buka periode?
5. Definisi Piutang/Hutang dashboard: saldo pihak positif (rekomendasi) vs per-order min 0 persis legacy?
6. Semantik `products.purchase_price`: per satuan beli (legacy, dibagi faktor) atau per satuan dasar?
7. Transfer antar-cabang: tetap tak dijurnal (paritas legacy) atau tambah akun "persediaan dalam perjalanan" agar buku per cabang = ledger?
