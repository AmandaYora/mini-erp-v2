# F5 — Rancangan `tools/legacy-migration` (ETL) + Gladi & Cutover

Status: rancangan siap-implementasi (versi 1, ditulis 2026-10-08). Read-only terhadap kedua repo; bukti `file:baris` dari kode yang dibaca langsung.
Legacy = `H:/dimasprasetio/SAAS/mini-erp` (`L:` di bawah). Revamp = `H:/dimasprasetio/SAAS/mini-erp-revamp` (`R:` di bawah).
Rujukan lama (`docs/plan/penutupan-gap-kapabilitas/PLAN.md` Tahap K/L) dipakai hanya sebagai pembanding; koreksinya ada di §2.

---------------------------------------------------------------------------------------------------

## 0. Ringkasan eksekutif

1. Belum ada satu pun artefak ETL (`R:/tools` tidak ada; semua K1-K10/L1-L5 `[ ]` di `PLAN.md:506-568`). Rancangan ini menggantikan K/L dengan 14 tugas (F5-1 … F5-14).
2. PLAN lama keliru/tidak lengkap pada 12 hal (lihat §2). Yang paling berbahaya: (a) tenant tidak difilter lewat `id_company` di banyak tabel (orders, payments, inventory_*, delivery_notes tidak punya kolom itu); (b) pembayaran per-order legacy **tidak punya baris `payment_allocations`**; (c) bukti bayar legacy tidak ada di `media_files`; (d) ledger HPP revamp memakai kursor `MAX(stock_movement_id)` sehingga baris biaya harus ikut dimigrasi; (e) antrean posting revamp tidak punya batas tanggal; (f) jurnal legacy terpecah per-event sedangkan revamp satu jurnal per dokumen.
3. Arsitektur: Go module terpisah `tools/legacy-migration` — fase `preflight → extract → transform → load → media → sequences → verify → archive`, ID map di DB staging (satu server MySQL dengan target agar satu transaksi menulis target + id_map secara atomik), idempoten, `--company-id` WAJIB tanpa default, hanya membaca salinan legacy (penjaga `etl_copy_marker`).
4. Keputusan terkunci pemilik dipatuhi: saldo awal per cabang + jurnal tahun berjalan; dokumen sebelum tahun berjalan ditandai "dicakup saldo awal"; schema FINAL (hasil squash agen lain) — asumsi di §1.
5. Gladi: L1/L2 (DB kosong dua kali, hash keluaran identik), parallel-run 1–2 minggu, freeze, go/no-go checklist yang dapat diverifikasi, rollback (legacy read-only 90 hari sebagai jalur balik).

---------------------------------------------------------------------------------------------------

## 1. Asumsi terhadap pekerjaan agen lain (WAJIB dikonfirmasi saat integrasi)

Rancangan agen lain di `scratchpad/plan/plan-*.md` belum ada saat ditulis (folder kosong). Asumsi berikut harus dicek ulang; setiap asumsi punya titik adaptasi tunggal di kode ETL (`internal/target/schema.go` + `internal/mapping/*.yaml`) agar perubahan schema tidak merembet.

| # | Asumsi (schema FINAL hasil squash) | Dasar | Titik adaptasi |
|---|---|---|---|
| A1 | Kolom lebar legacy dipulihkan: `users.username` 100, `branches.code` 50, `product.code` 50, nomor dokumen 100, `payments.reference_number` 100, `discount_pct` DECIMAL(9,4), `tax_rate` DECIMAL(9,4) | Keputusan #2; legacy `order.entity.ts:18,86`, `branch.entity.ts:14` | `schema.go` (panjang maks) + tes "tidak ada pemotongan" (F5-6) |
| A2 | `payments` punya `reference_number`, `amount_tendered`, `client_ref`; `sales_orders` punya `client_ref` (UNIQUE, NULLable) | Keputusan #2/#4 | ETL mengisi `client_ref = NULL` untuk data lama |
| A3 | Item order membawa snapshot varian (`variant_code`, `variant_name`, `variant_attributes` JSON) | Keputusan #2; legacy `order-item.entity.ts:29-36` | mapping `order_items` |
| A4 | Ada penanda status posting per dokumen sumber (`posting_state`: `pending`/`posted`/`opening_covered`/`ignored` + `posting_reason`) di `delivery_notes`, `goods_receipts`, `payments`, `sales_returns`, `purchase_returns` — dirancang agen finance (F2b). ETL hanya MENGISI | Temuan audit #1; `finance/application/queue.go:61-139`, `readiness.go:67` | `internal/load/posting_marks.go` (adapter tipis; ETL menghasilkan `posting_marks.jsonl` kanonik) |
| A5 | COA final dari F2b; ETL membaca peta kode legacy→revamp dari `mapping/coa.yaml` (disusun bersama F2b). Draft di Lampiran A.3 memakai COA revamp saat ini (`finance/application/service.go:40-53`) | Temuan audit | `mapping/coa.yaml` |
| A6 | Peta permission legacy→revamp dari F3b; draft di Lampiran A.9 memakai katalog revamp saat ini | — | `mapping/permissions.yaml` |
| A7 | Media: tabel `media_files` revamp tetap `(owner_type, owner_id, file_key UNIQUE, original_name, mime, size_bytes, is_primary, created_by)`; berkas disimpan di subfolder `media/` (key = `media/<legacy object_key>`); upload bertanda tangan tidak memengaruhi baris lama | Keputusan #4; `R:/…/media/application/service.go:99-110` | `internal/media/keymap.go` |
| A8 | Penomoran: `branch_document_sequences(branch_id, doc_kind, current_value)`; kunci bulanan `sales_order:YYYY-MM`, `purchase_order:YYYY-MM`; berkelanjutan `payment`, `delivery_note`, `sales_return`, `purchase_return`, `stock_transfer` | `R:/…/branch/infrastructure/repository.go:226-238`; `finance_document_sequences(branch_id, seq_key)` `JV:YYYY`, `EXP:YYYY`, `taxadj:YYYY` (`finance/application/service.go:429-441`) | `internal/sequences/rules.go` |
| A9 | F2a (`scratchpad/plan/plan-F2a-ops-integrity.md` §P1, ada saat ini) menambah kunci unik `stock_movements(ref_type, ref_id, product_id, variant_id, location_id, direction)` dan `payments.client_ref`; ETL mematuhi kunci unik dengan MENGGABUNG mutasi legacy yang bertabrakan (F5-6, Q-STK-6) dan mengisi `client_ref=NULL` | F2a §1 P1 | `transform/stock.go` |
| A10 | `stock_movements.created_at` tetap menyimpan waktu mutasi (UTC) dan tidak ada `balance_before/after` | `R:/…/migrations/000008_stock.up.sql` | mapping movements |
| A11 | Tidak ada kolom `legacy_id` di tabel revamp (D7 `PLAN.md:57`); jejak migrasi hanya di DB staging | `PLAN.md:97-107` | — |

Bila F2b memilih mekanisme penanda posting lain (mis. tabel `finance_posting_marks`), hanya adapter A4 yang diganti; format `posting_marks.jsonl` tidak berubah.

---------------------------------------------------------------------------------------------------

## 2. Verifikasi temuan audit & koreksi atas PLAN lama

| # | Klaim | Hasil baca kode | Bukti |
|---|---|---|---|
| V1 | `tools/` tidak ada, K/L semuanya `[ ]` | **Benar** | `PLAN.md:506-568`; `R:/tools` tidak ada |
| V2 | PLAN K "filter `id_company`" | **Tidak cukup.** `orders`, `order_items`, `payments`, `payment_allocations`, `delivery_notes`, `inventory_balances`, `inventory_movements`, `stock_locations`, `stock_issue_allocations`, `stock_reservations`, `order_status_history` **tidak punya `id_company`**; tenant hanya lewat `branches.id_company`. Yang punya: `goods_receipts`, `sales_returns`, `purchase_returns`, `stock_transfers`, `finance_*`, `media_files`, `users`, `business_parties`, `products`… | `L:order.entity.ts:15` (hanya `id_branch`), `delivery-note.entity.ts:~23`, `payment.entity.ts` (tanpa id_company), `goods-receipt.entity.ts:~13`, `stock-transfer.entity.ts` |
| V3 | status via `status_group` | Benar, tetapi ada 4 grup (`pending/active/completed/cancelled`) dan status kustom mungkin ada; `in_progress` & `confirmed` sama-sama `active` | `L:seed-runner.ts:287-293`, `shared-types/enums.ts:8` |
| V4 | `archived_at` pada order ambigu | **Tidak ambigu**: arsip hanya boleh bila tak ada mutasi stok/pembayaran → `cancelled` | `L:order.service.ts:791-798` (`assertNoStockMovementOrPayment`) — tetap dibuktikan lewat Q-ORD-4 |
| V5 | `discount_percent DECIMAL(9,4)→(5,2)` | Benar, tetapi masalah sebenarnya lebih besar: diskon legacy **per satuan** dan **nominal otoritatif**; revamp memakai diskon **per baris** dan menerapkan `pct` DAN `nominal` sekaligus (`sales/domain/pricing.go:9-12`). Memetakan keduanya = diskon ganda | `L:order-pricing.service.ts:216-239`; `R:sales/domain/pricing.go` |
| V6 | Pembayaran legacy dgn `payment_allocations` | **Salah untuk pembayaran per-order**: `createForOrder` menyimpan `payments.id_order` TANPA alokasi; alokasi hanya untuk pembayaran per-pihak | `L:payment.service.ts:249-302` vs `304-384` |
| V7 | Bukti bayar di `media_files` | **Salah**: hanya `payments.proof_file_path`; tidak ada baris media | `L:payment.service.ts:434-490` |
| V8 | SJ legacy = SJ revamp | SJ legacy tidak punya status; stok keluar saat dibuat (`dispatched_at` NOT NULL); serah langsung (POS/COD) memakai `reference_type='order'` **tanpa SJ** | `L:delivery-note.entity.ts:47`, `delivery.service.ts:532` |
| V9 | "Jurnal legacy PPN/SJ parsial berbug" | Sebagian benar: penjualan diposting SEKALI per order (`finance-posting.service.ts:1332-1362`), bukan per SJ. Untuk PO/SO **inklusif**, `line_total_before_tax` = bruto (`order-pricing.service.ts:239-251`); jurnal penjualan inklusif **tidak seimbang → ditolak `assertBalanced`** (`:2922-2933`) sehingga tidak pernah terposting; jurnal penerimaan inklusif **seimbang tapi salah** (persediaan & hutang bruto + PPN masukan dobel) | baris disebut |
| V10 | Ledger HPP "tidak ada di PLAN" | **Benar** dan kritis: revamp `Sync` memakai kursor `MAX(stock_movement_id)` (`ledger.go:42`, `costing.go:331-336`); bila baris biaya tak dimigrasi, Sync pertama menghitung ulang SELURUH riwayat (opening berharga katalog "estimated", `costing.go:386-393`) dan menyimpang dari jurnal legacy | baris disebut |
| V11 | Antrean posting tanpa batas tanggal | **Benar** + cacat tambahan: tiap jenis dokumen dibatasi `QueueCap=200` terbaru (`queue.go:168,196,213,228`); dokumen lama di luar 200 tak pernah terlihat → blokir `Readiness` tidak konsisten. Sumber "sudah diposting" hanya indeks `(source_doc_type, source_doc_id)` (`journals.go:474-492`) | baris disebut |
| V12 | `finance_periods` | Legacy per-**perusahaan** (`period_code`, `date_from/to`), revamp per-**cabang** `(branch_id, year, month)` — periode tutup legacy harus direplikasi per cabang | `L:finance-period.entity.ts:13-18`; `R:000016_finance.up.sql` |
| V13 | Akun seed superadmin/superadmin123 | **Benar** dan lebih luas: seed membuat 5 akun berpassword default `superadmin123, owner123, admin123, staff123, "kasir 123"` | `L:seed-runner.ts:335-339` |
| V14 | Counter nomor | Kunci legacy `order_<kind>_<YYYY-MM>`, `sj`, `payment`, `sales_return`, `purchase_return`, `stock_transfer`; kolom `sequence_key` (bukan `seq_key`/`doc_kind`); nomor lama bisa ber-prefix kode cabang lama | `L:order.service.ts:851`, `delivery.service.ts:1403`, `stock-transfer.service.ts:470`, `branch.service.ts:47-52` |

---------------------------------------------------------------------------------------------------

## 3. Struktur `tools/legacy-migration`

```
tools/legacy-migration/
  go.mod                         # module mini-erp/tools/legacy-migration ; TIDAK mengimpor apps/api (D8, scripts/arch-check.mjs menolak id_company di apps/api)
  README.md                      # (hanya bila diminta) runbook singkat
  cmd/legacy-migrate/main.go     # subcommand: preflight|extract|transform|load|media|sequences|verify|archive|rehearse|daily-verify
  internal/
    cfg/        flags.go         # --company-id (WAJIB, tanpa default), --legacy-dsn, --staging-dsn, --target-dsn, --workdir, --opening-date (WAJIB), --freeze-at, --media-src, --media-dst, --run-id
    guard/      guard.go         # penjaga: legacy hanya READ ONLY; etl_copy_marker; target harus kosong/schema cocok; nama DB tidak boleh berisi 'prod'
    legacydb/   snapshot.go      # START TRANSACTION WITH CONSISTENT SNAPSHOT + SET SESSION TRANSACTION READ ONLY
    registry/   entities.go      # satu baris per tabel legacy: nama, PK, kolom, filter tenant (SQL), urutan, ukuran perkiraan
    extract/    keyset.go        # keyset pagination (WHERE pk > ? ORDER BY pk LIMIT 5000) + checkpoint; JSONL.gz + manifest sha256
    transform/  <entity>.go      # fungsi murni legacy-row -> revamp-row(s); money.go (MoneyRounder); status.go; split_sj.go; split_transfer.go; journal.go; costledger.go
    lookup/     mem.go bbolt.go  # indeks id -> data kecil (antarmuka Lookup; bbolt bila melewati --mem-budget)
    idmap/      idmap.go         # tabel staging id_map(run_id, entity, legacy_id, legacy_sub, new_id); alokasi ID eksplisit
    load/       <table>.go       # batch 500 baris, urutan dependensi, 1 transaksi = target + id_map
    media/      copy.go keymap.go# salin berkas ke media/ (local atau S3 CopyObject), verifikasi ukuran+checksum
    sequences/  rebuild.go       # K9
    verify/     checks.go sql/   # K10: daftar pemeriksaan, laporan md+json
    archive/    csv.go           # arsip data yang tak termigrasi
    report/     report.go        # adjustments.csv, findings.csv, rejects/, run-report.md
  mapping/       coa.yaml permissions.yaml status.yaml enums.yaml
  sql/facts/     Q-*.sql         # pemeriksa fakta (Lampiran B)
  testdata/      legacy_mini.sql expected/*.json broken/*.sql
  Makefile / scripts             # TIDAK menambah perintah gabungan di root (rule monorepo.md); jalankan `go run ./cmd/legacy-migrate …` dari tools/legacy-migration
```

Aturan yang ditegakkan kode (bukan dokumen):
- `--company-id` dan `--opening-date` tidak punya default; tidak diberikan → exit 2.
- Koneksi legacy: user MySQL ber-grant `SELECT` saja; ETL menolak jalan bila `SELECT COUNT(*) FROM etl_copy_marker` gagal (tabel penanda yang dibuat pemilik saat me-restore dump ke Laragon) — mencegah tak sengaja menunjuk DB production.
- Target: ETL menolak bila tabel non-seed sudah berisi data kecuali `--resume` dan `etl_run.run_id` cocok.
- Tidak ada `concurrently`/perintah gabungan di root (`.claude/rules/monorepo.md`).
- Uang: semua konversi lewat satu fungsi `MoneyRounder` (setengah menjauh dari nol, `shopspring/decimal` — tanpa float).

Keluaran deterministik: file kanonik diurutkan menurut kunci dan di-hash; `manifest.json` memuat jumlah baris & sha256 tiap berkas → dasar tes idempotensi dan perbandingan L1 vs L2.

---------------------------------------------------------------------------------------------------

## 4. Keputusan desain lintas tugas (D-ETL-n)

| ID | Keputusan | Alasan / bukti |
|---|---|---|
| D-ETL-1 | **ID eksplisit** dialokasikan ETL (bukan auto-increment). `new_id = nextID(table)`, penghitung di staging `id_alloc(table,next_id)`; setelah load `ALTER TABLE … AUTO_INCREMENT = max+1`. Urutan alokasi = urutan kronologis `(tanggal, id legacy)` | auto-increment multi-row tidak dijamin berurutan; kursor HPP `MAX(stock_movement_id)` (`ledger.go:42`) mensyaratkan ID baru pasca-cutover > semua ID migrasi |
| D-ETL-2 | Staging & target di **satu server MySQL**; satu transaksi per batch menulis baris target + `id_map` (lintas-schema, InnoDB) → atomik & idempoten | `PLAN.md` D7 |
| D-ETL-3 | **Tanggal jurnal**: semua jurnal migrasi disimpan sebagai tanggal kalender WIB pukul 00:00 (naif, seperti jurnal manual/saldo awal, `finance/application/posting.go:49-58`), BUKAN instan UTC. Klasifikasi "tahun berjalan" memakai tanggal WIB yang sama dengan laporan legacy | jurnal terposting revamp memakai string UTC dokumen (`builders.go:192`) → kuartal bulan bisa bergeser; ETL tidak menularkannya. Bila F2b menyeragamkan konvensi, ubah satu fungsi `journalDate()` |
| D-ETL-4 | **Jurnal legacy dimigrasi apa adanya** (per-event), `source_doc_type='legacy_je'`, `source_doc_id=id jurnal legacy`, memo diawali `[L:<event_key>:<no dokumen>]`; nomor jurnal legacy dipertahankan (`≤60` karakter; kolom legacy 120) | GL revamp harus sama persis dengan legacy (K10 tolerans 0); re-derivasi per dokumen tak mungkin karena legacy memposting penjualan per ORDER bukan per SJ (`finance-posting.service.ts:1332`). Konsekuensi: modul turnover/UMKM tidak bisa mengecualikan dokumen lama lewat tautan sumber (catat di Risiko R7) |
| D-ETL-5 | **Penanda posting dokumen** (A4) diisi dari `finance_posting_sources` legacy: dokumen `posted_legacy` bila SEMUA event wajibnya `posted`; `opening_covered` bila tanggal dokumen < `--opening-date`; `ignored(reason)` bila legacy `ignored`; selain itu `pending` (akan diposting revamp). Dokumen terposting sebagian → `ignored('legacy_partial')` + masuk `findings.csv` (mencegah posting ganda) | cegah dobel: antrean revamp menganggap dokumen tanpa `source_doc_type` revamp sebagai belum diposting (`journals.go:474-492`) |
| D-ETL-6 | Legacy **tidak diubah** dan tidak ada sinkronisasi delta; satu kali muat final dari dump beku (D3 `PLAN.md:53`). Parallel-run memakai DB revamp terpisah yang dibuang sebelum cutover final (§6) | konsisten keputusan lama D3 |
| D-ETL-7 | **Toleransi verifikasi = 0** untuk jumlah baris, uang, dan kuantitas (kuantitas dibandingkan pada 12 desimal). Selisih yang disebabkan cacat legacy dicatat sebagai "selisih terjelaskan" bernilai pasti (bukan tolerans) | permintaan pemilik K10 |
| D-ETL-8 | Tidak ada pembulatan diam-diam: setiap perubahan nilai uang ≠ 0 akibat konversi DECIMAL(18,2)→integer ditulis ke `adjustments.csv` (entity, legacy_id, field, sebelum, sesudah, aturan) | ADR-0005 `knowledge/decisions/ADR-0005-money-as-integer-rupiah.md` |

---------------------------------------------------------------------------------------------------

## 5. Tugas

Ukuran: S ≤ 1 hari, M 2–4 hari, L 5–10 hari. Urutan global: **ETL baru bisa dimulai setelah squash schema final + baseline migrasi selesai** (F2a/F2b/F3b/schema agent); F5-1 s.d. F5-3 dan F5-13 (fixture) boleh jalan lebih dulu terhadap definisi entity legacy.

### F5-1 — Kerangka modul, konfigurasi, penjaga keamanan (S)
- Tujuan: modul Go terpisah (struktur §3), flag wajib, penjaga anti-production, logging terstruktur, `run_id`, kode keluar (0 bersih, 1 temuan blocker, 2 salah penggunaan).
- Rancangan:
  - `cfg`: `--company-id` (int, wajib), `--opening-date` (YYYY-MM-DD, wajib), `--legacy-dsn`, `--staging-dsn`, `--target-dsn`, `--workdir`, `--media-src`, `--media-dst`, `--mem-budget` (default 1GiB).
  - `guard`: (1) DSN legacy dibuka dengan `SET SESSION TRANSACTION READ ONLY`; (2) tabel `etl_copy_marker(id, restored_at, source_host, note)` HARUS ada di DB legacy (dibuat pemilik setelah restore) — jika tidak ada, tolak; (3) host legacy harus `localhost/127.0.0.1` kecuali `--allow-remote-legacy` + marker; (4) target: `SELECT COUNT(*)` tabel bisnis = 0 (kecuali seed role/akun/COA) atau `--resume` dengan `etl_run` cocok; (5) schema fingerprint target = hash `information_schema.columns` dibandingkan konstanta `expectedFingerprint` (diperbarui bersama squash).
  - Registry entity (`registry/entities.go`) berisi per tabel: nama, PK, kolom, `tenantSQL` (mis. `orders`: `id_branch IN (SELECT id_branch FROM branches WHERE id_company=?)`; `goods_receipts`: `id_company=?`).
- Bukti: V2 (tenant lewat cabang), `guard` dari audit Risiko 6.
- Test: unit guard (marker hilang → exit 2; `--company-id` hilang → exit 2); fingerprint mismatch → exit 1.
- Volume: nol. Dependensi: tidak ada. Ukuran S.

### F5-2 — Preflight & pemeriksa fakta (M)
- Tujuan: sebelum extract, mengukur data nyata dan memutuskan strategi (memori vs bbolt), menjalankan Lampiran B otomatis → `facts/<run>/facts.md|json`, dan gagal bila ada temuan "blocker".
- Rancangan: `preflight` mengeksekusi semua `sql/facts/Q-*.sql` dengan parameter `@c` (company) dan `@open` (opening date); tiap file punya header metadata YAML (`id`, `severity: info|warn|blocker`, `threshold`, `action`). Hasil: jumlah, 5 contoh, tindakan. Juga: hitungan baris & ukuran per tabel (`information_schema.tables`), cek drift definisi kolom legacy vs entity (kolom yang diharapkan hilang → blocker).
- Blocker (otomatis gagal): akun/COA tak terpeta; nomor dokumen ganda lintas cabang; order dengan `id_related_party` yatim; mutasi stok tanpa produk/varian; baris item order tanpa varian; username ganda saat dipetakan; panjang data > lebar kolom target; jurnal tak seimbang >0,009 sebelum pembulatan; `payments` yatim; `delivery_note_items` tanpa alokasi lokasi (untuk SJ stok-terlacak).
- Bukti: daftar risiko audit (7) → Lampiran B; `finance-posting.service.ts:2922` (assertBalanced), `043_round_money…sql:18-45`.
- Test: preflight terhadap fixture mini → output stabil (golden).
- Volume: query agregat saja, tidak ada N+1. Dependensi: F5-1. Ukuran M.

### F5-3 — Extract (kursor streaming → JSONL) (M)
- Tujuan: salin tabel legacy ke `staging/raw/<table>.jsonl.gz` tanpa memuat tabel penuh ke memori; resumable; konsisten.
- Rancangan:
  1. Buka satu koneksi, `START TRANSACTION WITH CONSISTENT SNAPSHOT` (REPEATABLE READ) + READ ONLY → seluruh tabel dibaca pada snapshot yang sama (untuk salinan statis ini cukup; untuk dump yang masih diubah pemilik tetap aman).
  2. Per tabel keyset: `SELECT <cols> FROM t WHERE <tenant> AND pk > ? ORDER BY pk LIMIT 5000`; checkpoint `extract_checkpoint(table,last_pk,rows)` di staging tiap batch; bila proses mati, `--resume` meneruskan dari `last_pk`.
  3. DECIMAL dibaca sebagai string (tanpa float), DATETIME sebagai UTC string `2006-01-02T15:04:05.000000Z` (legacy `timezone:'Z'`, `database.module.ts:23`), JSON apa adanya (RawMessage), DATE sebagai `YYYY-MM-DD`.
  4. Tulis `manifest.json`: tabel, baris, min/max pk, sha256 berkas, waktu snapshot, `@@version`, `@@sql_mode`.
  5. Urutan tabel besar terakhir: `inventory_movements`, `finance_journal_lines`, `finance_inventory_cost_movements`, `audit_logs`, `order_status_history`.
- Tabel yang diextract (semua yang relevan, termasuk yang hanya untuk arsip): lihat Lampiran A.1.
- Bukti: ukuran tabel terbesar = `inventory_movements` (indeks `idx_inventory_movements` di `001_baseline.sql:~290`); audit Risiko 8.
- Test: integrasi fixture → jumlah baris manifest = `COUNT(*)`; resume setelah dipotong di tengah → berkas identik byte-per-byte; sha256 stabil dua kali.
- Cek volume: memori konstan (≈ 1 batch); throughput target ≥ 20k baris/dtk lokal; 5 juta baris mutasi < 5 menit. Dependensi: F5-1. Ukuran M.

### F5-4 — Transform master & uang (MoneyRounder, enum, status) (M)
- Tujuan: transformasi murni (tanpa menyentuh DB revamp) untuk branches, locations, users, roles, permissions, company, member types, categories, products, variants, parties, addresses.
- `MoneyRounder` (`transform/money.go`):
  - `Round(d decimal.Decimal) int64`: setengah menjauh dari nol (sama dengan `math.Round` di `sales/domain/pricing.go:11`); satu-satunya jalur konversi uang; tes properti: `Round(-x) = -Round(x)`.
  - `BalanceEntry(lines)`: setelah tiap baris dibulatkan, bila Σdebit ≠ Σkredit sesuaikan SATU baris bermagnitudo terbesar di sisi yang kurang sebesar selisih (K4); wajib catat `(journal_no, delta, line_id)` ke `adjustments.csv`; tolak bila |delta| > `--max-balance-delta` (default 5) → blocker (bukan pembulatan lagi, itu data rusak).
- Peta status: `mapping/status.yaml` — `status_group` pending→draft, active→confirmed, completed→completed, cancelled→cancelled; nilai di luar itu → **gagalkan transform** (bukan jatuh ke draft). `archived_at ≠ NULL` pada order → cancelled (V4). Status kustom (kode di luar 5 seed `seed-runner.ts:287-293`) dilaporkan di Q-ORD-1 tetapi tetap dipetakan lewat `status_group`.
- Detail peta kolom → Lampiran A.2–A.9. Aturan khusus master:
  - `users`: `username` dipertahankan (A1 lebar 100); `password_hash` bcrypt `$2a/$2b$12$` dibawa apa adanya (bcryptjs ≈ `golang.org/x/crypto/bcrypt`; tes F5-13 memverifikasi). Akun yang hash-nya cocok password default seed (5 kata sandi di V13) → `status='locked'` + dicatat; akun `superadmin` (id 1) TIDAK dimigrasi (kebijakan 042: peran cadangan pengembang).
  - `branches`: `is_default→is_head` (hanya satu head; bila banyak default → blocker); `default_stock_location_label` → nama lokasi sistem `GDG`; `code` dijaga ≤ lebar target (A1) dan unik global (legacy unik per perusahaan: cek lintas perusahaan tidak perlu karena satu `company-id`).
  - `stock_locations`: lokasi `is_default=1` → kode sistem `GDG`, nama dari legacy; lokasi `status='damaged'` → kode sistem `RUSAK` (beberapa lokasi rusak per cabang digabung, `id_map` many-to-one); lokasi lain kode apa adanya (≤30 karakter; legacy 50 → blocker bila lebih); `archived_at` → `status='archived'`.
  - `products`: `product_type` physical|bundle→`barang`, service|non_stock→`jasa` (+ `stock_tracked=0` untuk jasa/non_stock); harga `purchase_price/selling_price/min_selling_price` DECIMAL→integer lewat MoneyRounder; `min_stock_qty NULL→0`; `attributes_json`, `summary_text`, `legacy uom` dibuang→arsip (Lampiran C).
  - `product_variants`: `barcode=''`→NULL (unik); varian tersembunyi default (`is_hidden`) dipertahankan sebagai `is_default=1`; `attributes_json` dibuang ke arsip (varian revamp tidak punya atribut) tetapi snapshot atribut di item order dibawa (A3).
  - `business_parties`: `partner` → tidak didukung revamp (`party/contracts/client.go:11-12`) → temuan blocker bila ada (Q-PTY-2); `metadata_json` → arsip; `phone_e164` → `phone`.
- Test: unit untuk MoneyRounder (±0,5; negatif; 0,005; angka besar 18 digit), peta status (nilai asing → error), peta enum, penggabungan lokasi rusak.
- Dependensi: F5-3. Ukuran M.

### F5-5 — Transform dokumen transaksi (L)
Detail baris ke baris di Lampiran A.5–A.7. Aturan inti:

**A. `orders` → `sales_orders` / `purchase_orders`** (pecah menurut `order_kind`, `007_rename_order_kind_values.sql`):
- `number=order_number`, `party_id=id_related_party` (NULL → 0 = walk-in POS; `payment/application/service.go:253-259`), `channel`: `source='pos'`→`pos` else `regular`; `payment_terms`: `net|cod` apa adanya, **`prepaid`→`cod`** (revamp hanya net/cod, `sales/application/service.go:267`) + temuan Q-ORD-5.
- `tax_type`: `taxable=0→none`; `taxable=1 & price_includes_tax=1→include`; `taxable=1 & 0→exclude`; `tax_rate` = `tax_rate_snapshot` (A1 DECIMAL(9,4)).
- **Header** (definisi revamp `sales/application/service.go:247-256`): `subtotal=Σ net baris`, `discount_total=Σ(gross−net)`, `tax_total`, `grand_total`. ETL memakai nilai legacy: `grand_total=Round(total_amount)`, `tax_total=Round(tax_amount)`, `discount_total=Round(discount_amount)`, `subtotal=grand_total−tax_total` bila `exclude` else `grand_total`… (rumus pasti di mapping; tes memverifikasi `total = subtotal + tax_total` untuk eksklusif, `total = subtotal` untuk inklusif/none — invarian `000020` comment).
- **Item (diskon)**: legacy diskon per SATUAN dan nominal otoritatif (V5). Revamp: `unit_price = Round(unit_price_before_discount)`, `discount_pct = 0`, `discount_nominal = Round(qty×unit_price_before_discount) − Round(line_total_before_tax)` (memaksa `net` revamp = `line_total_before_tax` legacy yang dibulatkan; `CalcLine` `pricing.go:9-12`). `line_total = Round(line_total_after_tax)`. Bila Σ `line_total` ≠ `grand_total` header: baris bermagnitudo terbesar menyerap selisih (log). `uom=transaction_uom_snapshot`, `uom_factor`, `qty`, `qty_base=quantity_in_base_uom` (24,12 tanpa kehilangan), snapshot varian (A3).
- Kolom legacy tanpa padanan → arsip: `id_assigned_user`, `credit_approved*`, `payment_term_days`, `goods_received_at`, `goods_delivered_at` (dipakai F5-7 untuk tanggal jurnal), `metadata_json`, snapshot member/harga item (`pricing_source`, `*_snapshot`), `discount_reason`, `unit_cost_snapshot`/margin (`049`) — margin laporan revamp dihitung dari jurnal, bukan snapshot.
- `due_date`: `net` wajib punya due_date di revamp (`service.go:270`); legacy `net` tanpa due_date → isi `order_date + payment_term_days` bila ada, kalau tidak `order_date` + temuan.

**B. Surat jalan** `delivery_notes` + `delivery_note_items` + `stock_issue_allocations`:
- Lokasi revamp `delivery_note_items.location_id NOT NULL`, sedangkan lokasi legacy hanya ada di alokasi → `split_sj.go`: per item SJ legacy, gabung alokasi (`id_delivery_note_item`) → satu baris revamp per `(item, lokasi)` dengan `qty` dan `qty_base` dari alokasi; jumlah alokasi per item WAJIB = qty item (blocker bila tidak). Satuan/faktor diambil dari `order_items` (`delivery_note_items` legacy tidak membawa UOM).
- `status`: `archived_at≠NULL`→`cancelled`; selainnya `confirmed` (stok keluar saat dibuat, `delivery-note.entity.ts:47`); `dispatched_at/by`, `confirmed_at/by`, `driver_name`, `vehicle_plate`, `warehouse_staff_name`, `recipient_*`, `drop_location_note` apa adanya (`000021` didesain untuk ini). `delivery_date` DATE → `DATETIME(6)` = 00:00 WIB dalam UTC (17:00 hari sebelumnya).
- `document_kind='replacement'` + `id_sales_return` → `delivery_notes.document_kind='replacement'`, `sales_return_id` (map setelah return dimuat — urutan load).
- **Serah langsung tanpa SJ** (`reference_type='order'`, `delivery.service.ts:532`): ETL membuat **SJ sintetis** per order: nomor `SJ-<KODE>/MIG/<no urut order>` (tidak bertabrakan dengan generator `SJ-<KODE>/<tahun>/<n:05>`, `branch/application/service.go:289`), `status=confirmed`, `delivery_date=goods_delivered_at`, item dari mutasi `order`-ref (lokasi dari mutasi). Dicatat di `findings.csv` (jumlah dari Q-STK-3).

**C. `goods_receipts`**: `supplier_delivery_number` → kolom revamp (A-schema; bila tak ada → `notes` diawali `[SJ Supplier: …]`), `id_stock_location` header NULL → lokasi dari item (item revamp punya `location_id NOT NULL`; legacy item NULL → lokasi default cabang `GDG`); arsip `archived_at≠NULL` → baris dimigrasi dengan `posting_state=ignored('archived')`, dan mutasi pembaliknya ikut (lihat F5-6).

**D. `payments`/`payment_allocations`** (V6, V7): arah dari `financial_side` (`receivable→in`, `payable→out`; NULL → dari `orders.order_kind`); `party_id=id_business_party ?? order.id_related_party`; `method`: `cash→cash`, `bank_transfer→transfer`, **`check→transfer`** + `notes` diawali `[Cek]` + temuan Q-PAY-3 (opsi: tambah metode `check` di schema final; keputusan pemilik P4); `status`: `archived_at≠NULL→cancelled` else `active`. **Alokasi**: bila `payment_allocations` aktif ada → salin (order_type dari `financial_side`, `amount=allocated_amount`); bila tidak ada & `id_order` terisi → **sintesis 1 alokasi** `(order_type, order_id, amount=payment.amount)`; bila keduanya kosong → blocker. `amount_tendered`, `reference_number` → kolom A2. `proof_file_path` → baris `media_files` sintetis (owner_type `payment_proof`) — ditangani F5-9.
- Validasi uang: Σ alokasi = `amount` (setelah pembulatan) per pembayaran; selisih ≤1 → serap di alokasi terbesar + log; >1 → blocker (kasus pembayaran pecahan, Q-MNY-2).

**E. Retur**: `sales_returns` (+items, replacement items, settlements): `status completed→confirmed`, `cancelled→cancelled`; nilai: `subtotal=returned_subtotal_before_tax`, `tax_total=returned_tax_amount`, `total=returned_total_amount`, `tax_type/tax_rate` dari order asal; baris `line_type='returned'`→`sales_return_items`, `'replacement'`→`sales_return_replacement_items`; lokasi item NULL → `GDG` cabang; `return_condition='damaged'` → lokasi `RUSAK`; `settlements` → `sales_return_settlements` (1:1; tipe legacy `reduce_receivable` tidak punya baris legacy — hanya header `settlement_type`). **Sub-ledger**: untuk settlement `refund` ETL juga membuat `payments` (direction out, method dari settlement) + alokasi `order_type='sales_return'` agar `GetPartyBalance` (`payment/application/service.go:716-815`) sama dengan legacy; `collect_payment`/`customer_credit` pada retur tukar → daftar "selisih terjelaskan" (Q-RET-2) karena rumus saldo berbeda (legacy `salesReturnFinancialEffectForOrders`, `payment.service.ts:616-700`). `purchase_returns` simetris (`cancels_order`→arsip).

**F. `stock_transfers`**: header legacy tanpa lokasi (ada di item) → `split_transfer.go`: kelompokkan item per `(from_location, to_location)` → satu transfer revamp per kelompok; nomor kelompok pertama = nomor asli, berikutnya `<nomor>-B`, `-C`… (nomor unik global, `000008_stock.up.sql`); `status in_transit/received/cancelled/draft` apa adanya; `dispatched_at/received_at`.

- Test: unit (split SJ: alokasi 3 lokasi → 3 baris, jumlah tepat; transfer 2 grup; diskon per satuan→nominal baris; inklusif; `prepaid`; pembayaran tanpa alokasi → 1 alokasi sintetis); golden terhadap fixture.
- Dependensi: F5-4. Ukuran L.

### F5-6 — Transform stok & ledger HPP (L)
- **`inventory_movements` → `stock_movements`**: `direction`: `movement_type in→in, out→out`; `adjustment`→`direction` dari tanda `balance_after−balance_before` (movement_type `adjustment`); `qty_base=quantity` (24,12); `created_at=moved_at`; `notes=reason_text` (255; legacy TEXT → pemotongan dilaporkan); `created_by`.
- **ref_type** (peta `mapping/enums.yaml`, bukti daftar referensi di grep legacy): `delivery_note→delivery`, `goods_receipt→goods_receipt`, `sales_return→sales_return`, `sales_return_exchange→replacement`, `purchase_return→purchase_return`, `opening_stock→opening`, `manual_adjustment→adjustment`, `transfer|stock_transfer→transfer`, `damaged_stock|damaged_restore→damaged`, `damaged_write_off→write_off`, `order` (serah langsung) → `delivery` dengan `ref_id` = SJ sintetis (F5-5.B), NULL/tak dikenal → blocker (Q-STK-2). `ref_id` dipetakan lewat `id_map` dokumen (urutan load: dokumen sebelum mutasi).
- **Kunci unik idempotensi F2a** `(ref_type, ref_id, product, variant, location, direction)`: mutasi legacy yang bertabrakan **digabung** (jumlahkan qty & biaya; `notes` "gabungan N mutasi") dan dilaporkan (Q-STK-6). Mutasi dgn `ref_id NULL` tidak digabung.
- **`inventory_balances` → `stock_balances`**: salin `on_hand_qty`; `reserved=0` (reservasi POS bersifat sementara; pada freeze semua keranjang kedaluwarsa — Q-STK-5 harus 0 aktif; tidak memigrasi `stock_reservations`). Kolom `available_qty` dibuang (turunan).
- **Konsistensi saldo vs mutasi**: untuk tiap `(cabang, produk, varian, lokasi)` hitung Σ mutasi bertanda; bila ≠ `on_hand` legacy → ETL menambah **mutasi koreksi** `adjustment` bernotes `Koreksi migrasi` sebesar selisih (dicatat `findings.csv`; tanpa jurnal; baris biaya ikut dengan nilai rata-rata). Alasan: invarian revamp "saldo berubah selalu berpasangan dengan mutasi" (`stock/application` catatan `InsertMovementTx`).
- **Ledger HPP** (V10): sumber `finance_inventory_cost_movements` (+`finance_inventory_cost_states`, `finance_opening_inventory_items`):
  - Untuk setiap baris biaya dgn `id_inventory_movement`: `finance_inventory_cost_movements(branch, product, variant, stock_movement_id=new, ref_type, ref_id, direction, qty_base, unit_cost, total_cost, is_estimated=0)`. `direction`: `movement_type in|opening→in`, `out→out`, `reversal` → tanda `quantity_in_base_uom`/`total_cost_amount`. `total_cost = ±total_cost_amount` (rupiah bulat; sisa pecahan ke `adjustments.csv`), `unit_cost = total_cost/qty` 4 desimal.
  - Baris biaya `opening_balance` tanpa mutasi (`finance.service` opening) → ETL membuat mutasi `in` ber-`ref_type='opening'` sintetis di lokasi `GDG` (qty = `quantity_in_base_uom`) — **hanya bila** jumlah qty dari item pembukaan belum tercermin mutasi `opening_stock`; kasus ini diukur Q-HPP-2 dan diputuskan di preflight (default: sintetis + temuan).
  - Baris biaya yang hilang untuk mutasi legacy yang belum diposting (legacy menghitung biaya saat POSTING, `finance-inventory-cost.service.ts:251`): **tidak dibuat** — tetapi kursor revamp `MAX(stock_movement_id)` akan melompati mereka. Maka ETL WAJIB memastikan tidak ada mutasi tanpa baris biaya yang ID-nya lebih kecil dari baris biaya tertinggi: solusi → untuk mutasi yang belum terposting ETL membuat baris biaya `is_estimated=1` dengan aturan revamp `costOf` (rata-rata berjalan; `costing.go:395-405`) dan menandainya di `findings.csv` (Q-HPP-3). Alasan: Sync revamp tidak pernah mengulang ID di bawah kursor.
  - GR inklusif (V9): baris biaya mengikuti legacy (bruto) agar sama dengan jurnal legacy yang dimigrasi; selisih terhadap basis pajak dilaporkan sebagai temuan F-INCL (tindakan: jurnal koreksi manual, keputusan P2).
- Test: unit (peta ref_type; penggabungan kunci unik; mutasi koreksi; konversi baris biaya reversal); integrasi: Σ mutasi = saldo; Σ `total_cost` per produk = `inventory_value_after` terakhir legacy.
- Volume: `inventory_movements` bisa jutaan → stream terurut `(branch, id)`; penggabungan unik memakai lookup bbolt bila melewati `--mem-budget`. Dependensi: F5-5 (ID dokumen). Ukuran L.

### F5-7 — Transform keuangan (COA, saldo awal, jurnal tahun berjalan, penanda posting) (L)
- **COA**: `finance_accounts` → `finance_accounts` lewat `mapping/coa.yaml` (Lampiran A.3). Prinsip: peta KODE eksplisit per `system_key` (bukan kode, karena arti kode berbeda: legacy 1000=Kas, revamp 1000 tidak ada; legacy 1100=Piutang, revamp 1100=Kas!). Akun legacy tanpa `system_key` (akun buatan pengguna) → dibuat sebagai akun revamp baru dengan kode `L<kode legacy>` bila bentrok, tipe `account_type` dipetakan `asset→aset, liability→kewajiban, equity→modal, revenue→pendapatan, expense→beban` (`finance/contracts/client.go:11-15`), `is_cash` dari `finance_cash_accounts`. `finance_account_mappings`: `system_key→mapping_key` (`cash_cash→cash`, `cash_bank→bank`, `accounts_receivable→receivable`, `inventory→inventory`, `input_tax→ppn_in`, `accounts_payable→payable`, `output_tax→ppn_out`, `sales_revenue→revenue`, `cogs→cogs`, `stock_difference→adjustment`, `expense_other→expense`; `customer_advance`, `supplier_advance`, `rounding_difference`, `purchase_return_difference`, `opening_equity` tak punya kunci revamp → akun dibuat, dimigrasi sebagai akun biasa).
- **Saldo awal per cabang** (keputusan #1): `opening-date` = tanggal WIB. Untuk tiap `(cabang, akun)` hitung saldo bersih (debit−kredit) seluruh baris jurnal berstatus `posted|reversed` dengan tanggal WIB < `opening-date`; hanya akun neraca (asset/liability/equity); akun laba-rugi ditutup ke ekuitas (`3000`): baris penyeimbang = −Σ saldo neraca (dihitung, bukan ditebak; setara laba ditahan kumulatif). Satu jurnal per cabang `source_doc_type='opening'`, `source_doc_id=branch_id` (`opening.go:90-93`); baris piutang/hutang diberi `party_id` (dimensi `id_business_party` legacy; sisa tak teratribusi = baris tanpa dimensi), baris persediaan diberi `product_id/variant_id`. Jurnal `NULL`-cabang (opening legacy, beberapa beban) → cabang head (`is_default`) + temuan Q-JRN-1.
- **Jurnal tahun berjalan**: semua entri `posted|reversed` bertanggal WIB ≥ `opening-date` (D-ETL-3, D-ETL-4): header `number=journal_number`, `entry_date`, `memo=[L:event:no] description`, `status` (`reversed`→`reversed`, `reversed_by` = id entri pembalik via `id_reversal_of`), `created_by=posted_by`; baris: `account_id` terpeta, `debit/credit` = Round; dimensi `product_id/variant_id/party_id` dari `id_product/id_product_variant/id_business_party` (dimensi `id_order/id_payment/id_inventory_movement/metadata_json` dibuang, `000019_finance_line_dimensions.up.sql:3-11`); `description` 500→255 (dipotong + log). Entri `draft` tidak dimigrasi. Pembulatan: K4 `BalanceEntry` per entri.
- **Penanda posting** (D-ETL-5, A4): dari `finance_posting_sources` (event ↔ dokumen):

| Dokumen revamp | Event legacy wajib `posted` | Catatan |
|---|---|---|
| goods_receipt | `purchase_received` | 1:1 |
| delivery (SJ) | `sales_completed` (per ORDER) + `stock_issue_cogs` untuk SEMUA mutasi SJ itu | order bersama banyak SJ: tiap SJ bertanda mengikuti status order; bila hanya `sales_completed` terposting → `ignored('legacy_partial')` |
| payment | `customer_payment`/`supplier_payment` | pembayaran pada order terarsip tidak pernah dikandidatkan (`finance-posting.service.ts:1500`) → `ignored('order_archived')` |
| sales_return | `sales_return_completed` (+`sales_return_stock_in`, settlement events) | |
| purchase_return | `purchase_return_completed` (+`purchase_return_stock_out`) | |
  Dokumen bertanggal < `opening-date` → `opening_covered` (keputusan #1) tanpa melihat status legacy.
- **Periode**: `finance_periods` legacy (perusahaan) → baris `(cabang, tahun, bulan, status)` untuk SEMUA cabang; ditambah semua bulan < bulan `opening-date` ditutup; ditulis SETELAH jurnal dimuat (bypass `requireOpenPeriod`, `service.go:403-413`). `finance_tax_periods` serupa; `finance_tax_report_snapshots` → arsip (angka perusahaan, tidak punya cabang). `finance_tax_adjustments` → arsip (bukan jurnal akun; Lampiran C).
- **Biaya usaha**: `business_expenses` → `business_expenses` (cabang NULL→head; `expense_date` DATE→DATETIME; `id_finance_journal_entry`→id jurnal baru; `cancelled`→status; pihak/`description` 500→`notes` 255 + log).
- Test: unit (BalanceEntry; peta akun; klasifikasi tanggal WIB di batas 31 Des 17:00 UTC; saldo awal = Σ jurnal prior; dimensi); integrasi: neraca saldo per cabang per akun legacy (setelah peta COA) = revamp.
- Dependensi: F5-5/F5-6 (ID dokumen & mutasi), F2b (COA, penanda). Ukuran L.

### F5-8 — Load, IDMap, idempotensi (M)
- Tujuan: baca JSONL kanonik → INSERT batch 500 ke target dalam urutan dependensi; ulang aman.
- Urutan load (tiap tahap = satu `etl_stage` di staging, status `pending|running|done|verified`):
  1. seed referensi: `permissions`, `roles`, `role_permissions` → 2. `branches`, `company_profile`, `company_settings`, `member_types` → 3. `users`, `user_roles`, `user_branch_access` → 4. `product_categories`, `products`, `product_variants`, `parties`, `party_delivery_addresses` → 5. `stock_locations` (sistem `GDG`/`RUSAK` lebih dulu) → 6. `finance_accounts`, `finance_account_mappings` → 7. `purchase_orders`+items, `sales_orders`+items → 8. `goods_receipts`+items, `delivery_notes`+items (SJ sintetis), `sales_returns`(+items/replacement/settlements), `purchase_returns`(+…), `stock_transfers`(+items) → 9. `payments`+`payment_allocations` (termasuk sintetis dari refund retur) → 10. `stock_movements`, `stock_balances` → 11. `finance_inventory_cost_movements` → 12. `finance_journal_entries`+`lines` (opening dulu, lalu tahun berjalan), `business_expenses` → 13. penanda posting (A4) → 14. `finance_periods`/`finance_tax_periods` (tutup) → 15. `media_files` (setelah berkas tersalin, F5-9) → 16. sequences (F5-10) + `AUTO_INCREMENT`.
  Mutasi stok sengaja SETELAH dokumen (`ref_id` butuh `id_map`, `PLAN.md:534-538`), biaya setelah mutasi (FK logis `stock_movement_id`), jurnal terakhir.
- Idempotensi (tiga lapis): (1) `id_map(entity, legacy_id, legacy_sub, new_id)` — baris yang sudah terpeta dilewati; (2) `INSERT … ON DUPLICATE KEY UPDATE id=id` pada PK eksplisit; (3) `etl_stage` mencatat `rows_loaded` & `checksum` — saat `--resume`, batch yang hash-nya sama dilewati, hash berbeda → berhenti (data staging berubah).
- Transaksi: per batch `BEGIN; INSERT target…; INSERT id_map…; COMMIT` (D-ETL-2). Foreign key target tidak dimatikan; urutan dependensi menjamin FK dalam-modul.
- Larangan: ETL TIDAK memanggil service/handler revamp (data historis tidak boleh lewat aturan bisnis dokumen baru, `PLAN.md:117-119`); ETL menulis lintas modul (di luar `apps/api` → lolos `scripts/arch-check`).
- Test: jalankan dua kali → baris sama, `id_map` sama, hash tabel sama; matikan di tengah batch (kill) → resume menghasilkan hasil identik dgn lari penuh.
- Volume/kinerja: batch 500 baris, `max_allowed_packet` ≥ 64MB, `SET unique_checks=1, foreign_key_checks=1` (tetap ON); target ≥ 5k baris/dtk; indeks sekunder besar ditunda? Tidak — biarkan (tabel kosong, MySQL 8.4 cukup cepat); jurnal & mutasi diperkirakan paling lama.
- Dependensi: F5-4…F5-7, schema final. Ukuran M.

### F5-9 — Lampiran/berkas (media) (M)
- Sumber lampiran legacy (V7): (a) `media_files` (owner `product|delivery_note|payment|knowledge`, `purpose`, `bucket`, `object_key`); (b) `payments.proof_file_path` (key S3/lokal, tanpa baris media); (c) `finance_tax_adjustments.attachment_uri` (arsip).
- Pemetaan baris (A7): `purpose` melebur — `product_image→owner_type product`; `payment_proof` (atau `payments.proof_file_path`)→`payment_proof`; `delivery_location_photo|delivery_signed_document→delivery_proof` (pembeda purpose hilang → dicatat di `original_name` prefix `[foto-lokasi]`/`[dokumen-ttd]` agar tak hilang, Lampiran C); `knowledge_source` dibuang → arsip. `owner_id` WAJIB lewat `id_map` (nilai mentah = ID legacy yang sudah tidak berlaku, `PLAN.md:539-548`). `file_key = media/<object_key legacy>` (unik; `object_key` legacy VARCHAR(700) → revamp 255: key lebih panjang → blocker Q-MED-2 atau hash-ing `media/<sha1>.<ext>` + `original_name`); `is_primary` (produk: legacy `is_primary`; bukti bayar: true).
- Penyalinan berkas (`internal/media/copy.go`): sumber `local` (folder `--media-src`) atau `s3` (endpoint/bucket legacy; baca saja). Tujuan `--media-dst` = `storage/media/` revamp (driver `local`, `STORAGE_DIR`) atau bucket **terpisah** (S3 CopyObject lintas bucket). Jangan memakai bucket yang sama: `Archive` revamp menghapus objek (`media/application/service.go:162`). Verifikasi: ukuran (`size_bytes`) dan sha256 setelah salin; berkas hilang di sumber → baris media tetap dibuat? Tidak — dicatat `missing_media.csv` dan baris TIDAK dibuat (hindari tautan mati), kecuali `--keep-missing` (default tidak).
- Idempotensi: berkas dengan key sama & checksum sama dilewati; checksum beda → berhenti.
- Mime/ekstensi: whitelist revamp `.jpg .jpeg .png .webp .pdf` (`media/application/service.go:69-75`); selain itu → arsip, bukan media.
- Test: fixture dengan 3 berkas + 1 hilang; `owner_id` setelah load = ID revamp (tes K, `PLAN.md:597`); salin dua kali tanpa duplikasi.
- Volume: salinan paralel 8 worker; tampilkan total byte. Dependensi: F5-8 tahap 4,8,9 dan A7. Ukuran M.

### F5-10 — SequenceRebuilder (K9) (S)
Aturan (A8) — hasil = `max(nilai legacy, maksimum urutan terparse dari nomor yang termigrasi)`, tidak pernah turun:
| Kunci revamp (per cabang) | Sumber legacy | Parser nomor |
|---|---|---|
| `sales_order:YYYY-MM` | `order_sales_YYYY-MM` (`order.service.ts:851`) | `^ORD-.+/PJ/(\d{4})/(\d{2})/(\d+)$` |
| `purchase_order:YYYY-MM` | `order_purchase_YYYY-MM` | `…/PB/…` |
| `payment` | `payment` (kontinu lintas tahun, `payment.service.ts:715-723`) | `^PAY-.+/\d{4}/(\d+)$` |
| `delivery_note` | `sj` (`delivery.service.ts:1403`) | `^SJ-.+/\d{4}/(\d+)$` (SJ sintetis `/MIG/` DIABAIKAN) |
| `sales_return` / `purchase_return` | `sales_return` / `purchase_return` | `^RTR-…` / `^RTB-…` |
| `stock_transfer` | `stock_transfer` (`stock-transfer.service.ts:470`) | `^TRF-.+/\d{4}/(\d+)$` (+ sufiks `-B` diabaikan) |
Finance (per cabang): `JV:<tahun>` ≥ nomor `JV-…` yang dipakai ETL (jurnal saldo awal); `EXP:<tahun>` ≥ urutan nomor biaya legacy tahun itu; `taxadj:<tahun>` = 0.
- Parser memakai ekor angka, bukan awalan: prefix cabang bisa berubah di legacy (`branch.service.ts:68-83` menimpa prefix saat kode diubah) → nomor satu cabang bisa ber-prefix beda; counter revamp dipakai bersama sehingga memakai maksimum lintas prefix.
- Gagal-aman: setelah menulis, ETL memanggil fungsi format nomor revamp (disalin test-only dari `branch/application/service.go:268-305`) untuk 1 nomor berikutnya per `(cabang, jenis)` dan memastikan tidak ada di tabel dokumen (tes K9 `PLAN.md:598`). Juga `ALTER TABLE … AUTO_INCREMENT = max(id)+1` semua tabel yang diberi ID eksplisit.
- Test: fixture dengan nomor ber-prefix berbeda + SJ sintetis; `verify` V-18.
- Ukuran S. Dependensi: F5-8.

### F5-11 — Verify / rekonsiliasi (K10) (L)
Perintah `verify` membaca legacy (snapshot yang sama dgn extract) dan target, menulis `report/verify.md|json`. Toleransi 0 (D-ETL-7); "selisih terjelaskan" hanya bila tercantum di `findings.csv` dengan nilai pasti dan jumlahnya persis sama. Exit 1 bila ada selisih tak terjelaskan.

| ID | Pemeriksaan | Sumber (legacy) | Target (revamp) |
|---|---|---|---|
| V-01 | Jumlah baris per entity (termasuk pecahan: SJ item per lokasi, transfer per kelompok, SJ sintetis, alokasi sintetis, mutasi koreksi/gabungan) | `COUNT(*)` terfilter tenant | `COUNT(*)` + tabel faktor pecahan dari `findings` |
| V-02 | Total nilai order per cabang × jenis × status | `SUM(total_amount)` | `SUM(grand_total)` |
| V-03 | Σ pajak & diskon per cabang | `SUM(tax_amount), SUM(discount_amount)` | `SUM(tax_total), SUM(discount_total)` |
| V-04 | Per order: Σ `line_total` = `grand_total` (invarian target) dan `grand_total = Round(total_amount)` | — | SQL |
| V-05 | Saldo stok per `(cabang, produk, varian, lokasi)` | `inventory_balances.on_hand_qty` | `stock_balances.on_hand` (12 desimal) |
| V-06 | Σ mutasi bertanda = saldo (target) dan Σ qty per `ref_type`×arah | `inventory_movements` (peta enum) | `stock_movements` |
| V-07 | Pembayaran: jumlah & Σ `amount` per cabang × arah × metode × status; Σ alokasi = Σ amount | `payments` | `payments`, `payment_allocations` |
| V-08 | Sisa per order (rumus legacy `buildPaymentSummary` vs rumus revamp) | `payment.service.ts:721-760` | `GetPartyBalance` direplikasi SQL |
| V-09 | Piutang/hutang per party | `listPartyBalances` (`payment.service.ts:117`) | SQL setara `GetPartyBalance` (`service.go:629-714`) |
| V-10 | Saldo kas/bank per akun | GL legacy per akun kas (sampai freeze) | GL revamp akun terpeta |
| V-11 | **Neraca saldo** per cabang × akun (debit, kredit, saldo) sampai freeze; total debit = total kredit | `finance_journal_lines` (peta COA, NULL→head) | `finance_journal_lines` (saldo awal + tahun berjalan) |
| V-12 | Jumlah & Σ debit/kredit jurnal per cabang × bulan (tahun berjalan) | jurnal legacy | jurnal revamp |
| V-13 | Jurnal saldo awal: seimbang, 1 per cabang, Σ = saldo neraca per akun s.d. `opening-date − 1` | GL legacy | entri `opening` |
| V-14 | Persediaan: Σ `total_cost` per `(cabang, produk)` = `inventory_value_after` terakhir/`cost_states`; selisih GL(1300) − ledger legacy = selisih GL − ledger target (selisih bawaan legacy dibawa persis) | cost movements/states + GL | ledger + GL |
| V-15 | Lampiran per pemilik: jumlah per `(owner_type, owner_id→legacy id)`; berkas ada, ukuran & sha256 sama | `media_files` + `proof_file_path` | `media_files` + berkas |
| V-16 | User/role/permission: jumlah, dan himpunan izin efektif per user (setelah peta) identik | roles/permissions | roles/permissions |
| V-17 | Setiap nomor dokumen legacy ada persis (string sama) di target; tak ada duplikat | semua nomor | semua nomor |
| V-18 | Nomor berikutnya tiap `(cabang, jenis)` tidak ada di target | — | simulasi format |
| V-19 | Orphan/FK logis: item tanpa header, alokasi ke order lain pihak/cabang, SJ tanpa SO, mutasi tanpa dokumen | — | SQL |
| V-20 | Penanda posting: cacah per status; ukuran antrean `UnpostedSources` setelah load = jumlah `pending` yang diharapkan; `Readiness` cabang tidak diblokir oleh dokumen lama | `finance_posting_sources` | penanda |
| V-21 | Periode ditutup sesuai rencana | `finance_periods` | `finance_periods` |
| V-22 | Master: party, alamat, produk, varian, kategori, member type (jumlah + Σ harga) | — | — |
| V-23 | Retur & settlement: jumlah, Σ nilai; payments sintetis refund = Σ settlement refund | — | — |
| V-24 | Arsip lengkap: tiap tabel di Lampiran C punya berkas dan jumlah baris = sumber | — | `archive/manifest.json` |
| V-25 | Kontrol negatif (`verify --self-test`): merusak satu baris di salinan target (ubah 1 `amount`, hapus 1 mutasi, geser 1 owner_id) dan memastikan V-02/V-05/V-15 gagal | — | — |
Format laporan: tabel per pemeriksaan `id | deskripsi | sumber | target | selisih | terjelaskan | status`, ringkasan akhir `LULUS/GAGAL`, plus `top-20` selisih per cabang.
- Test: tes #K `PLAN.md:599` ("rekonsiliasi mendeteksi satu baris yang sengaja dirusak") diwujudkan oleh V-25 + tes integrasi di F5-13.
- Volume: tiap pemeriksaan satu query berkelompok per sisi (P3), tanpa N+1; sumber & target dibandingkan lewat `FULL OUTER` di Go (map terurut).
- Dependensi: F5-8…F5-10. Ukuran L.

### F5-12 — Arsip data tak termigrasi & keamanan akun (S)
- `archive`: ekspor CSV/JSONL (UTF-8, dengan `manifest.json`+sha256) untuk daftar Lampiran C: `order_status_history`, `order_status_definitions/transitions`, `knowledge_*`, `whatsapp_conversation_*`, `assistant_*`, `audit_logs` (penuh), `finance_tax_adjustments`, `finance_tax_report_snapshots`, `finance_opening_*`, `finance_posting_sources`, `orders.metadata_json`, kolom dibuang (order, produk, varian, pihak) sebagai `legacy_columns/<tabel>.jsonl` keyed `legacy_id`. Disimpan di `archive/<tanggal>/` di luar repo, 2 salinan, retensi sesuai pemilik.
- Akun: lihat F5-4 (hash dibawa; default seed dikunci; sesi/refresh token tidak dibawa → login ulang); daftar akun terkunci ke `findings.csv` dan ke checklist go/no-go.
- Audit log: hanya baris tahun berjalan (action_key, entity, entity_id terpeta, cabang, aktor, `note` ringkas 255 dari `after_json`) dimuat ke `audit_logs`; sisanya arsip (Q-AUD-1 mengukur ukuran).
- Ukuran S. Dependensi: F5-3.

### F5-13 — Rangkaian test ETL (L)
| Lapis | Isi | Menangkap |
|---|---|---|
| Unit | `MoneyRounder` (±0,5; properti simetri); `BalanceEntry` (delta, tolak >batas); peta status/enum (nilai asing → error); `split_sj`, `split_transfer`; diskon per-satuan→nominal baris; peta tax_type; parser nomor + K9; klasifikasi tanggal WIB (batas 31 Des 17:00 UTC); gabung mutasi unik | pembulatan, peta, pecahan |
| Kontrak schema | `expectedFingerprint` target = hasil migrasi baseline; panjang kolom ≥ legacy (A1) | drift schema |
| Integrasi | `testdata/legacy_mini.sql` (2 cabang, 6 produk 2 varian, 12 order campuran: inklusif/eksklusif/diskon/POS/serah langsung/arsip/`prepaid`; 4 SJ pecah lokasi; 2 GR; 3 pembayaran per-order + 2 per-pihak + 1 cek; 2 retur (1 tukar); 1 transfer 2 grup; jurnal 30 entri termasuk `branch NULL`, reversal, tak seimbang 0,01; cost ledger; 5 media) → restore ke MySQL Laragon → extract→transform→load→K9→verify **bersih** | alur utuh |
| Idempotensi | jalankan seluruh pipeline dua kali ke target yang sama → hash tiap tabel identik, `verify` bersih; kill di tengah load + resume | duplikasi |
| Determinisme | transform dua kali → `manifest` sha256 sama | nondeterminisme |
| Verifikasi | `testdata/broken/*.sql` merusak: ubah `payments.amount`, hapus 1 mutasi, geser `media.owner_id`, tukar akun jurnal → `verify` GAGAL pada V-07/V-06/V-15/V-11 | verifikasi tumpul |
| Kompatibilitas | bcrypt hash `$2a$12$` hasil `bcryptjs` diverifikasi `bcrypt.CompareHashAndPassword` (fixture hash dibuat dari `seed-runner` password `admin123`) | login gagal |
| Perilaku revamp | setelah load fixture: panggil service revamp read-only (GetPartyBalance, laporan neraca saldo, `UnpostedSources`) dan bandingkan dgn nilai sumber; `Sync` costing tidak mengubah apa pun (kursor di atas semua mutasi) | kursor HPP |
| Properti | acak 10k baris uang: Σ sebelum pembulatan ≈ Σ sesudah, selisih = log adjustments | integritas log |
- Uji dengan dump nyata dilakukan di F5-14 (L1).
- Ukuran L. Dependensi: F5-1…F5-11 (fixture dapat dimulai lebih awal dari definisi entity legacy).

### F5-14 — Gladi (L1/L2), parallel-run, freeze, cutover, rollback, komunikasi (M, sebagian non-kode)
**Prasyarat**: (1) squash baseline schema final & migrasi lulus; (2) dump legacy tersedia di Laragon (`C:/laragon/bin/mysql`, MySQL 8.4.3) dengan `etl_copy_marker`; (3) F5-1…F5-13 selesai terhadap fixture.

**Lini waktu** (T0 = tanggal freeze final; angka hari target, disesuaikan pemilik):
| Waktu | Kegiatan | Bukti keluar |
|---|---|---|
| T-30 | Terima dump #1; `preflight` → `facts.md`; keputusan pemilik atas temuan blocker (§Pertanyaan) | facts.md + keputusan tercatat |
| T-25 | **L1**: DB revamp kosong → `extract…verify` penuh | `verify` LULUS; laporan waktu per tahap |
| T-23 | **L2**: bangun ulang dari nol (drop target & staging) lalu ulang; bandingkan hash tabel & `manifest` L1 vs L2 | hash identik (kecuali kolom waktu dibuat otomatis) |
| T-22 | L3 manual pemilik: cetak 5 dokumen lama di dot-matrix (`PLAN.md:565`), 10 dokumen sampel dibandingkan dgn kertas arsip; cek login per peran | berita acara |
| T-21…T-8 | **Parallel-run** (lihat bawah) | rekonsiliasi harian ditandatangani |
| T-7 | Dump #2 (terbaru) → L1 ulang untuk mengukur drift data; keputusan go/no-go awal | go/no-go awal |
| T-1 | Komunikasi final ke pengguna; cadangan penuh legacy (mysqldump + berkas media) disimpan offline | cadangan + checksum |
| T0 (malam, di luar jam operasional) | **Freeze** (di bawah) → dump final → ETL final ke revamp BARU → K9 → verify → go/no-go | checklist terisi |
| T0+1 … T+14 | Hypercare; legacy read-only hidup | log insiden |
| T+90 | Matikan legacy; arsip + tag git `tools/legacy-migration`; hapus staging | berita acara penutupan |

**Parallel-run (1–2 minggu)** — model yang dirancang (dikonfirmasi pemilik, P5): legacy tetap sistem catatan yang sah dan **tetap bisa ditulis**; DB revamp-P dimuat dari dump #1/#2; sekelompok operator (kasir + gudang + finance) menginput transaksi hari itu ke revamp-P secara paralel (bisa subset); tiap malam `legacy-migrate daily-verify --date D`: bandingkan agregat harian legacy vs revamp-P (jumlah & Σ order, pembayaran, mutasi stok per produk, kas per akun) dan keluarkan daftar selisih untuk ditelusuri sebelum jam masuk besok. Revamp-P DIBUANG di cutover; data revamp-P tidak dimigrasi balik. Kriteria lulus: 5 hari kerja berturut-turut `daily-verify` tanpa selisih tak terjelaskan, tidak ada insiden uang/stok.

**Freeze legacy (T0)**: (1) umumkan, hentikan layanan NestJS (bukan sekadar `read_only` global agar aman); (2) cabut hak tulis user aplikasi (`REVOKE INSERT,UPDATE,DELETE`) atau `SET GLOBAL read_only=1` di MySQL legacy; (3) pastikan `SELECT COUNT(*) FROM stock_reservations WHERE status='active'`=0 dan tidak ada sesi aktif (`user_sessions.revoked_at IS NULL AND expires_at>NOW()` abaikan — sesi tak dibawa); (4) `mysqldump --single-transaction --routines` + salin folder/bucket media; checksum; (5) restore ke Laragon sebagai salinan + buat `etl_copy_marker`; (6) catat `freeze_at` (ETL `--freeze-at`) — jurnal/dokumen setelah itu tidak ada.

**Go/no-go (semua harus ya, tiap butir punya perintah/artefak pembuktian)**:
1. `verify` LULUS tanpa selisih tak terjelaskan (artefak `verify.md`, exit 0).
2. `findings.csv` blocker = 0; setiap temuan non-blocker punya keputusan tertulis (jurnal koreksi GR inklusif, retur tukar, cek, dst.).
3. Hash `manifest` L2 = L1 pada dump #2 (determinisme) .
4. V-18 lulus: dokumen pertama pasca-cutover tidak menabrak nomor (uji coba buat 1 SO + 1 pembayaran di revamp-staging lalu hapus rekamannya di salinan).
5. Antrean posting: `UnpostedSources` (via `GET …/finance/posting/queue` per cabang) sesuai angka `pending` yang diharapkan; `Readiness` bulan berjalan bisa lulus setelah dokumen pending diposting (uji pada salinan).
6. Neraca saldo revamp = legacy per cabang (V-11), laba rugi bulan berjalan sama.
7. Lampiran: sampel acak 50 (tiap jenis) terbuka dari UI revamp; V-15 lulus.
8. Login 1 akun per peran berhasil (password lama); akun default seed terkunci (query bukti).
9. Cadangan legacy + arsip C tersimpan 2 lokasi dengan checksum.
10. L3 (cetak dot-matrix) berita acara ditandatangani; `npm run verify` repo revamp hijau; bucket media revamp ≠ legacy (diperiksa).
11. Rencana rollback dibaca & disetujui; kontak darurat tersedia.
Keputusan: pemilik menandatangani; satu "tidak" = tunda.

**Rollback**: (a) sebelum go-live (T0 s.d. keputusan go): batalkan — unfreeze legacy (`read_only=0`, nyalakan NestJS), buang revamp; tidak ada data hilang. (b) setelah go-live: legacy tetap read-only 90 hari sebagai pembanding & jalur balik. Pemicu: selisih uang/stok yang tak bisa dijelaskan dalam 24 jam, atau tak bisa tutup hari. Prosedur: freeze revamp, `export-since <waktu go-live>` mengeluarkan CSV semua dokumen baru di revamp (order, SJ, pembayaran, mutasi) untuk di-input ulang manual ke legacy setelah di-unfreeze; keputusan balik maksimal dalam 72 jam (setelah itu volume re-input tidak praktis). Tidak ada ETL balik (di luar cakupan).

**Komunikasi pengguna**: (T-7) pengumuman jadwal & downtime; (T-1) panduan 1 halaman: semua pengguna **login ulang** (sesi tidak dibawa), password sama (hash dibawa) kecuali akun default seed yang dikunci → minta reset; alamat/URL baru; WhatsApp bot perlu **pairing ulang QR** (whatsmeow vs baileys); cetak dot-matrix: kalibrasi profil kertas harus disetel ulang; nomor dokumen lama tetap berlaku, nomor baru melanjutkan; bukti bayar/foto lama tetap terbuka; peran/izin dapat berbeda nama (lampirkan tabel peta); kontak dukungan jam kerja hypercare.
- Ukuran M (kode: `daily-verify`, `export-since`, `rehearse` skrip; sisanya runbook). Dependensi: semuanya.

---------------------------------------------------------------------------------------------------

## 6. Urutan terhadap fase lain & ringkasan ukuran

| Tugas | Ukuran | Bisa mulai | Bergantung pada |
|---|---|---|---|
| F5-1 kerangka/penjaga | S | sekarang | — |
| F5-2 preflight + SQL fakta | M | sekarang (terhadap entity; dijalankan saat dump ada) | F5-1 |
| F5-3 extract | M | sekarang | F5-1 |
| F5-13 fixture `legacy_mini.sql` | M (bagian) | sekarang | definisi entity legacy |
| F5-4 … F5-7 transform | M+L+L+L | **setelah squash schema final** (peta kolom target) | F2a (kunci unik stok, `client_ref`), F2b (COA, penanda posting), F3b (permission), schema agent |
| F5-8 load | M | setelah baseline migrasi final | F5-4…7 |
| F5-9 media | M | setelah A7 final (driver `media/`) | F5-8 |
| F5-10 K9, F5-11 K10 | S+L | setelah F5-8 | — |
| F5-12 arsip/akun | S | sekarang | F5-3 |
| F5-14 gladi | M | setelah dump + semua di atas | semua |
Perkiraan total ≈ 35–45 hari-orang (satu pengembang ±8 minggu), kritis: F5-5…F5-7.

---------------------------------------------------------------------------------------------------

# LAMPIRAN A — Tabel peta legacy → revamp

Legenda: **T**=tenant via `branches.id_company`; **C**=punya `id_company`. “→ arsip” = Lampiran C.

## A.1 Satu baris per tabel legacy
| Tabel legacy (entity) | Tenant | → Revamp | Catatan singkat |
|---|---|---|---|
| `companies` | — | `company_profile` (1 baris) | `name, legal_name`; `code`, `timezone`, `currency_code`, `locale`, `status` dibuang (konstanta ADR-0006) |
| `company_settings` | C | `company_settings` (key-value) | 5 kolom JSON diratakan (A.8) |
| `company_features` | C | — | mati (KI-33) → arsip |
| `branches` | C | `branches` | A.4 |
| `branch_document_sequences` | via branch | `branch_document_sequences` | dibangun ulang oleh K9, bukan disalin |
| `users` | C | `users` | A.9 |
| `roles` | C/NULL | `roles` | `is_system_role→is_system`, `description_text→description` |
| `permissions` | — | `permissions` | peta kode A.9 |
| `role_permissions`, `user_roles` | — | idem | lewat peta |
| `user_branch_accesses` | via user | `user_branch_access` | `is_default_branch→is_default` |
| `user_sessions` | C | — | tidak dibawa (login ulang) |
| `business_parties` | C | `parties` | A.5; `metadata_json`→arsip |
| `business_party_delivery_addresses` | C | `party_delivery_addresses` | `address_text`; `archived_at`→`status/archived_at` |
| `member_types` | C | `member_types` | kolom sama; `status/archived_at` |
| `product_categories` | C | `product_categories` | `id_parent_category→parent_id`; `sort_order`→arsip; `archived_at→status` |
| `products` | C | `products` | A.5 |
| `product_variants` | C | `product_variants` | A.5 |
| `orders` | T | `sales_orders`/`purchase_orders` | A.2 |
| `order_items` | via order | `sales_order_items`/`purchase_order_items` | A.2 |
| `order_status_definitions`/`_transitions`/`_history` | C/—/T | — | status dipakai untuk peta; sisanya arsip (D4) |
| `delivery_notes`, `delivery_note_items`, `stock_issue_allocations` | T | `delivery_notes`, `delivery_note_items` | A.3 (pecah per lokasi) |
| `goods_receipts`, `goods_receipt_items` | C | `goods_receipts`, `goods_receipt_items` | A.3 |
| `sales_returns` (+`_items`,`_settlements`) | C | `sales_returns`, `sales_return_items`, `sales_return_replacement_items`, `sales_return_settlements` | A.3 |
| `purchase_returns` (+`_items`,`_settlements`) | C | `purchase_returns`, `purchase_return_items`, `purchase_return_settlements` | A.3 |
| `payments`, `payment_allocations` | T | `payments`, `payment_allocations` | A.6 |
| `inventory_balances` | T | `stock_balances` | `available_qty` dibuang; `reserved=0` |
| `inventory_movements` | T | `stock_movements` | A.7 |
| `stock_locations` | T | `stock_locations` | A.7 |
| `stock_reservations` | T | — | transien; tidak dimigrasi (Q-STK-5 = 0) |
| `stock_transfers`, `stock_transfer_items` | C | `stock_transfers`, `stock_transfer_items` | A.7 (pecah per pasangan lokasi) |
| `finance_accounts` | C | `finance_accounts` | A.10 |
| `finance_account_mappings` | C | `finance_account_mappings` | A.10 |
| `finance_cash_accounts` | C | kolom `finance_accounts.is_cash` | akun terkait ditandai kas |
| `finance_journal_entries`/`_lines` | C | `finance_journal_entries`/`_lines` | A.10; hanya tahun berjalan + saldo awal |
| `finance_posting_sources` | C | penanda posting (A4) | A.10 |
| `finance_inventory_cost_movements` | C | `finance_inventory_cost_movements` | F5-6 |
| `finance_inventory_cost_states` | C | — (turunan baris terakhir) | dipakai verifikasi V-14 |
| `finance_opening_balances`, `finance_opening_inventory_items` | C | — (diganti jurnal `opening`) | → arsip |
| `finance_periods`, `finance_tax_periods` | C | `finance_periods`, `finance_tax_periods` | per cabang |
| `finance_tax_report_snapshots` | C | — | → arsip |
| `finance_tax_adjustments` | C | — | → arsip (bukan jurnal akun) |
| `finance_document_sequences` | C | `finance_document_sequences` | K9 |
| `business_expenses` | C | `business_expenses` | `id_branch NULL→head`, `description→notes` |
| `media_files` | C | `media_files` | F5-9 |
| `audit_logs` | C | `audit_logs` | hanya tahun berjalan; sisanya arsip |
| `daily_operational_metrics` | — | — | dihapus (KI-126) → tidak diarsipkan (turunan) |
| `knowledge_documents/versions/chunks` | C | — | → arsip (RAG dibuang) |
| `whatsapp_authorizations` | C | `assistant_authorizations` | `phone_e164→phone`, `display_name→name`, `is_primary_owner`; `id_user` dibuang |
| `whatsapp_channels`, `whatsapp_conversation_threads/messages`, `assistant_runs`, `assistant_tool_executions`, `system_event_logs` | C/— | — | → arsip (sesi WA harus pairing ulang) |

## A.2 `orders` / `order_items` → sales/purchase (risiko tinggi)
| Legacy | Revamp | Aturan |
|---|---|---|
| `id_order` | via `id_map` | |
| `order_kind` (`sales`/`purchase`; migrasi 007 mengganti nilai lama) | pemilah tabel | nilai lain → blocker |
| `id_branch` | `branch_id` | |
| `order_number` (100) | `number` | dipertahankan; unik global → Q-ORD-2 |
| `source` (`manual`/`pos`) | `channel` (`regular`/`pos`) | hanya penjualan |
| `id_related_party` | `party_id` | NULL→0 (walk-in) |
| `id_ship_to_address`+`ship_to_*_snapshot` | `ship_to_address_id`,`ship_to_label`,`ship_to_recipient`,`ship_to_phone`,`ship_to_address` | `ship_to_address` 255 ← TEXT (potong+log) |
| `id_current_status`→`status_group` | `status` | pending→draft, active→confirmed, completed→completed, cancelled→cancelled; `archived_at`→cancelled |
| `order_date`, `due_date` | `order_date`, `due_date` | UTC apa adanya (DATETIME(3)→(6)) |
| `supplier_invoice_number/date` | `supplier_invoice_number/date` | purchase; (100→50 → A1) |
| `tax_invoice_number/date` | `tax_invoice_number/date` | sales; |
| `payment_terms` | `payment_terms` | `prepaid`→`cod` |
| `taxable`,`price_includes_tax` | `tax_type` | none/include/exclude |
| `tax_rate_snapshot` (9,4) | `tax_rate` | A1 |
| `subtotal_amount`,`discount_amount`,`subtotal_before_tax`,`tax_amount`,`subtotal_after_tax`,`total_amount` | `subtotal`,`discount_total`,`tax_total`,`grand_total` | A.2-header di F5-5; sisanya turunan dibuang |
| `notes` TEXT | `notes` 255 | potong + log (Q-LEN) |
| `id_created_by` | `created_by` | via id_map user |
| `created_at`,`updated_at` | idem | |
| `id_assigned_user`,`credit_approved(*)`,`payment_term_days`,`goods_received_at`,`goods_delivered_at`,`metadata_json` | — | arsip (`credit_approved` dipakai Q-ORD untuk validasi termin) |
| item: `line_no` | urutan baris (id berurut) | |
| item: `product/variant_*_snapshot` | `product_code`,`product_name`, A3 | |
| item: `transaction_uom_snapshot`,`uom_to_base_factor`,`quantity`,`quantity_in_base_uom` | `uom`,`uom_factor`,`qty`,`qty_base` | |
| item: `unit_price_before_discount` | `unit_price` | pembulatan; selisih log |
| item: `line_discount_amount`,`line_discount_percent`,`discount_reason` | `discount_nominal` (total baris) ,`discount_pct=0` | V5 |
| item: `taxable`,`tax_rate_snapshot`,`tax_base_amount`,`tax_amount`,`line_total_before_tax`,`line_total_after_tax` | — (turunan) ; `line_total=Round(line_total_after_tax)` | campuran taxable → temuan |
| item: `pricing_source`,`*member*snapshot`,`basis_price_snapshot`,`stock_tracked_snapshot`,`unit_cost_snapshot`…`margin_costed_at`,`notes` | — | arsip |

## A.3 Dokumen turunan
| Legacy | Revamp | Aturan |
|---|---|---|
| `delivery_notes.sj_number` | `delivery_notes.number` | dipertahankan; SJ sintetis `…/MIG/…` |
| `id_order` NULL & `document_kind='replacement'` | `sales_order_id` (dari retur) , `document_kind`, `sales_return_id` | map retur dulu |
| `delivery_date` DATE | `delivery_date` DATETIME | 00:00 WIB→UTC |
| `dispatched_at/by`,`confirmed_at/by`,`driver_name`,`vehicle_plate`,`warehouse_staff_name`,`recipient_*`,`drop_location_note`,`notes` | kolom sama (`000021`) | |
| tanpa status | `status` | archived→cancelled; else confirmed |
| `delivery_note_items`(+alokasi) | `delivery_note_items` | pecah per lokasi; UOM dari `order_items` |
| `goods_receipts.supplier_delivery_number` | kolom/notes | A1 |
| `goods_receipts.id_order` | `purchase_order_id` | |
| `goods_receipts.received_at`,`notes`,`id_received_by` | `received_at`,`notes`,`created_by` | |
| `goods_receipt_items` | `goods_receipt_items` | `location_id` NULL→`GDG`; uom dari snapshot |
| `sales_returns.return_number` | `number` | |
| `…returned_subtotal_before_tax/tax/total` | `subtotal/tax_total/total` | |
| `replacement_*`,`difference_amount`,`settlement_type`,`reason_text`(NOT NULL),`cancelled_*`,`source`,`id_related_party` | `notes` (reason), sisanya arsip | `replacement_*` dipakai verifikasi V-23 |
| `return_mode` | `return_mode` | |
| `replacement_delivery_status` | `replacement_delivery_status` (`delivered`→`confirmed`) | |
| `sales_return_items` returned | `sales_return_items` (+`tax_base`,`tax_amount`,`discount_*`) | |
| `sales_return_items` replacement | `sales_return_replacement_items` | |
| `*_settlements` | `*_settlements` | `notes` 255; `id_created_by`→`created_by`; arsip bila `archived_at` |
| `purchase_returns.cancels_order`,`reason_text` | arsip / `notes` | |
| `stock_transfers.transfer_number`,`status`,`transfer_date`,`dispatched_at`,`received_at`,`notes` | idem; `driver_name`,`vehicle_number`,`cancelled_*`,`id_*_by` → arsip/created_by | |

## A.4 Cabang
| Legacy | Revamp |
|---|---|
| `code` (50) | `code` (A1) |
| `name`,`city`,`address_text`,`phone_e164`,`status` | `name`,`city`,`address`,`phone`,`status` |
| `is_default` | `is_head` |
| `default_stock_location_label` | nama lokasi sistem `GDG` |
| `created_at/updated_at` | idem |

## A.5 Master
| Legacy | Revamp | Catatan |
|---|---|---|
| `business_parties.party_type` customer/supplier/partner | `party_type` | partner → blocker/keputusan |
| `code`,`name`,`phone_e164`,`email`,`address_text`,`notes`,`archived_at`,`id_member_type` | `code`,`name`,`phone`,`email`,`address_text`,`notes`,`status/archived_at`,`member_type_id` | unik `(code,party_type)` |
| `products.product_code`,`product_name`,`id_product_category`,`product_type`,`stock_tracked`,`base_uom`,`purchase_uom`,`sales_uom`,`*_factor`,`min_stock_qty`,`purchase_price`,`selling_price`,`min_selling_price`,`status`,`archived_at` | `code`,`name`,`category_id`,`type`,`stock_tracked`,… | `has_variants`,`attributes_json`,`summary_text`,`uom` → arsip |
| `product_variants.variant_code`,`variant_name`,`barcode`,`is_default`,`status`,`archived_at` | `code`,`name`,`barcode`,`is_default`,`status` | `''`→NULL; `attributes_json`,`is_hidden`,`sort_order` → arsip |
| `member_types.*` | `member_types.*` | `price_adjustment_direction` add/subtract → plus/minus (revamp default `minus`, `000007`) — peta eksplisit di `enums.yaml` |

## A.6 Pembayaran
| Legacy | Revamp |
|---|---|
| `payment_number` | `number` |
| `id_branch` | `branch_id` |
| `id_business_party` / `orders.id_related_party` | `party_id`; `party_type` dari sisi |
| `financial_side` receivable/payable | `direction` in/out; `party_type` customer/supplier |
| `payment_date` | `paid_at` |
| `amount` | `amount` (Round) |
| `amount_tendered` | `amount_tendered` (A2) |
| `payment_method` cash/bank_transfer/check | `method` cash/transfer/transfer(+`[Cek]`) |
| `reference_number` | `reference_number` (A2) |
| `notes` | `notes` |
| `proof_file_path` | `media_files(payment_proof)` |
| `archived_at` | `status=cancelled` |
| `payment_allocations.allocated_amount/id_order/allocated_at/allocation_method` | `payment_allocations.amount/order_id/order_type`; sisanya arsip |
| (payments.id_order tanpa alokasi) | alokasi sintetis |

## A.7 Stok
| Legacy | Revamp |
|---|---|
| `stock_locations.code/name/id_parent_location/status/archived_at/is_default` | `code/name/parent_id/status/is_system`; `is_picking_area`,`is_primary`,`sort_order` → arsip (atau kolom bila schema final menambahkan) |
| `inventory_movements.movement_type/quantity/balance_*` | `direction/movement_type/qty_base` (`balance_*` bila F2a-9 menambah kolom: isi; kalau tidak → arsip) |
| `reference_type` → `ref_type` | peta di F5-6 |
| `id_reference` → `ref_id` | via id_map |
| `reason_text` | `notes` (255) |
| `id_moved_by`,`moved_at` | `created_by`,`created_at` |
| `metadata_json` | arsip |
| `stock_transfer_items.id_from_location/id_to_location` | pecah header |

## A.8 `company_settings` (5 JSON → key-value)
| Legacy kolom | Kunci revamp | Aturan |
|---|---|---|
| `business_labels_json` | — | label bisnis tak dipakai revamp → arsip |
| `operational_rules_json.stockAdjustmentApprovalMode` | tidak setara (revamp memakai ambang numerik `ApprovalThresholdSetting`, `stock/application/transactions.go:346-358`) | arsip; default revamp |
| `ui_preferences_json` (profil kertas dot-matrix) | `print.profiles` (C6) | diterjemahkan per profil: `widthMm`, `heightMm`, `marginRightMm` `37.91`, `showLetterhead` (`PLAN.md:84-91`); nilai tak dikenal → arsip |
| `assistant_preferences_json` | — | arsip |
| `reporting_preferences_json` | — | arsip (cek kunci via Q-CFG-1) |

## A.9 Pengguna, peran, izin
- `users`: `username`(100),`email`,`password_hash`,`full_name`(255→150! potong+log),`phone_e164`,`status` (`inactive|locked`→`inactive`), `last_login_at`→dibuang, `display_title`→arsip.
- Peta kode permission (draft memakai katalog revamp saat ini, `*/contracts/client.go` PermissionCatalog; **final dari F3b**):

| Legacy | Revamp |
|---|---|
| `dashboard.view` | `dashboard.view` |
| `product.view/create/update/archive` | `products.view/create/update/archive` |
| `member_type.view` , `.manage` | `customers.view` (peta bebas, F3b) , `member_types.manage` |
| `order.view/create/update/archive` | `sales.*` **dan** `purchasing.*` (order legacy mencakup jual+beli → gabungan) |
| `order.export` | `reporting.view` |
| `sales_return.view/create/refund/cancel` | `salesreturn.view/create/…`, refund→`salesreturn.create`, cancel→`salesreturn.archive` |
| `purchase_return.*` | `purchasereturn.*` |
| `stock.view/update/transfer/adjust` | `stock.view/manage/manage/adjust` ; `stock.adjust.approve`→`stock.approve` |
| `reporting.view` | `reporting.view` |
| `user.view/create/update/archive` | `users.*` |
| `company_config.view/manage`,`branch.view/manage` | `company.*`,`branches.*` |
| `role.manage` | `roles.manage` |
| `knowledge.*`,`whatsapp.simulate` | — (dibuang) |
| `whatsapp.view/manage` | `assistant.view/manage` |
| `audit_log.view` | `audit.view` |
| `payment.view/create/archive` | `payment.view/create/archive` |
| `finance.view` ; `.manage`,`.close.force` | `finance.view` ; `finance.manage` |
| `finance.posting.*`,`finance.journal.*`,`finance.expense.*` | `finance.view/post/manage` |
| `finance.report.view`,`finance.tax_report.view`,`finance.tax_adjustment.*` | `finance.view`,`finance.manage` |
- Peran sistem legacy `superadmin` tidak dimigrasi; `owner/admin/staff/kasir` dibuat ulang bila belum ada dan diisi himpunan izin hasil peta (satu izin legacy → banyak izin revamp; tidak pernah menambah izin yang tak diberikan legacy: gunakan gabungan minimal). V-16 membuktikan himpunan efektif tidak melebar.

## A.10 Keuangan
**COA** (draft; final dari F2b — arti kode BERBEDA, peta lewat `system_key`):
| Legacy (`kode`, `system_key`) | Revamp (`kode`, `mapping_key`) |
|---|---|
| 1000 Kas Tunai `cash_cash` | 1100 Kas Tunai `cash` |
| 1010 Rekening Bank `cash_bank` | 1110 Bank Transfer `bank` |
| 1100 Piutang Customer `accounts_receivable` | 1200 Piutang Usaha `receivable` |
| 1200 Persediaan `inventory` | 1300 Persediaan `inventory` |
| 1300 PPN Masukan `input_tax` | 1400 PPN Masukan `ppn_in` |
| 2000 Hutang Supplier `accounts_payable` | 2100 Hutang Usaha `payable` |
| 2100 PPN Keluaran `output_tax` | 2200 PPN Keluaran `ppn_out` |
| 2200 Uang Muka Customer `customer_advance` | **akun baru** (F2b; tak ada di COA saat ini) |
| 2300 Uang Muka Supplier `supplier_advance` | **akun baru** |
| 3000 Modal Awal `opening_equity` | 3000 Modal Awal |
| 4000 Penjualan Barang `sales_revenue` | 4000 Penjualan `revenue` |
| (retur penjualan dibukukan ke `sales_revenue` legacy) | 4100 Retur Penjualan (dipakai builder baru; tak ada saldo migrasi) |
| 5000 HPP `cogs` | 5000 HPP `cogs` |
| 5100 Selisih Stok `stock_difference` | 6100/6200 (`writeoff`/`adjustment`) — F2b memilih |
| 5200 Pembulatan `rounding_difference` | **akun baru** atau 6200 |
| 5300 Selisih Retur Pembelian `purchase_return_difference` | 6200 `adjustment` (`builders.go:478-482`) atau akun baru |
| 6000 Gaji, 6010 Listrik, 6020 Sewa, 6030 Internet, 6040 Transport, 6050 Admin Bank, 6900 Lain-lain | beban: 6000 Beban Operasional atau akun beban terpisah (direkomendasikan: tetap terpisah agar laporan beban lama utuh, kode `61xx`; keputusan F2b) |
Akun buatan pengguna: dimigrasi dengan kode asli bila tak bentrok, bila bentrok `L<kode>`; peta tersimpan di `id_map` dan `coa_map.csv`. **Setiap akun yang punya baris jurnal harus terpeta; sisanya blocker (Q-JRN-5).**
- `finance_journal_entries`: lihat F5-7. Kolom dibuang: `posting_type` (manual→`source NULL`; reversal→`reversed_by`), `event_key`,`posted_at`,`archived_at`,`metadata_json`, `id_reversal_of` (jadi `reversed_by` terbalik).
- `finance_journal_lines`: dibuang `id_order`,`id_payment`,`id_inventory_movement`,`metadata_json`,`line_no` (urut insert), `id_branch`,`id_company`.
- `finance_posting_sources` → `posting_marks.jsonl` (D-ETL-5): `(doc_type,doc_id,state,reason,legacy_journal_ids)`.

---------------------------------------------------------------------------------------------------

# LAMPIRAN B — Pemeriksa fakta (SQL, dijalankan terhadap salinan/dump legacy)

Parameter: `SET @c = <id_company>; SET @open = '2026-01-01';` (tanggal WIB). Alias `b` = cabang perusahaan: `branches b WHERE b.id_company=@c`. Tiap butir: **[severity] tujuan → tindakan bila ada**. Contoh baris: tambahkan `LIMIT 5`.

```sql
-- ===== ORDER =====
-- Q-ORD-1 [blocker bila status_group di luar 4 grup] status kustom & sebarannya
SELECT d.code,d.label,d.status_group,COUNT(o.id_order) n
FROM order_status_definitions d LEFT JOIN orders o ON o.id_current_status=d.id_order_status_definition
WHERE d.id_company=@c GROUP BY d.id_order_status_definition;
-- tindakan: peta lewat status_group; grup tak dikenal → hentikan, minta keputusan.

-- Q-ORD-2 [blocker] nomor dokumen ganda (revamp: UNIQUE global; legacy: per cabang)
SELECT 'orders' t,order_number num,COUNT(*) n,COUNT(DISTINCT id_branch) nb FROM orders
 WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) GROUP BY order_number HAVING n>1
UNION ALL SELECT 'payments',payment_number,COUNT(*),COUNT(DISTINCT id_branch) FROM payments
 WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) GROUP BY payment_number HAVING COUNT(*)>1
UNION ALL SELECT 'delivery_notes',sj_number,COUNT(*),COUNT(DISTINCT id_branch) FROM delivery_notes
 WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) GROUP BY sj_number HAVING COUNT(*)>1
UNION ALL SELECT 'sales_returns',return_number,COUNT(*),COUNT(DISTINCT id_branch) FROM sales_returns WHERE id_company=@c GROUP BY return_number HAVING COUNT(*)>1
UNION ALL SELECT 'purchase_returns',return_number,COUNT(*),COUNT(DISTINCT id_branch) FROM purchase_returns WHERE id_company=@c GROUP BY return_number HAVING COUNT(*)>1
UNION ALL SELECT 'stock_transfers',transfer_number,COUNT(*),COUNT(DISTINCT id_from_branch) FROM stock_transfers WHERE id_company=@c GROUP BY transfer_number HAVING COUNT(*)>1
UNION ALL SELECT 'journals',journal_number,COUNT(*),COUNT(DISTINCT id_branch) FROM finance_journal_entries WHERE id_company=@c GROUP BY journal_number HAVING COUNT(*)>1;
-- tindakan: akhiri nomor ganda dgn sufiks "-<kode cabang>" (dicatat); keputusan pemilik bila di dokumen tercetak.

-- Q-ORD-3 [blocker bila > lebar target] panjang maksimum kolom teks
SELECT MAX(CHAR_LENGTH(order_number)) mx_no, MAX(CHAR_LENGTH(notes)) mx_notes,
       SUM(CHAR_LENGTH(notes)>255) notes_gt255, MAX(CHAR_LENGTH(supplier_invoice_number)) mx_sinv,
       MAX(CHAR_LENGTH(tax_invoice_number)) mx_tinv, SUM(CHAR_LENGTH(ship_to_address_snapshot)>255) ship_gt255
FROM orders WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c);
-- (ulangi untuk users.username/full_name, branches.code, products.product_code, variants, locations.code, payments.reference_number/notes, journals.description, media.object_key)

-- Q-ORD-4 [blocker] order terarsip yang punya mutasi/SJ/pembayaran (membantah V4)
SELECT o.id_order,o.order_number FROM orders o
WHERE o.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND o.archived_at IS NOT NULL
 AND (EXISTS(SELECT 1 FROM inventory_movements im WHERE im.reference_type='order' AND im.id_reference=o.id_order)
   OR EXISTS(SELECT 1 FROM delivery_notes dn WHERE dn.id_order=o.id_order)
   OR EXISTS(SELECT 1 FROM payments p WHERE p.id_order=o.id_order)
   OR EXISTS(SELECT 1 FROM payment_allocations pa WHERE pa.id_order=o.id_order));
-- tindakan: tentukan status per kasus; default: cancelled + temuan.

-- Q-ORD-5 [warn] termin prepaid & net tanpa due_date
SELECT payment_terms,order_kind,COUNT(*) n,SUM(payment_terms='net' AND due_date IS NULL) net_no_due,
       SUM(id_related_party IS NULL) no_party
FROM orders WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) GROUP BY 1,2;
-- tindakan: prepaid→cod; due_date = order_date (+payment_term_days) bila kosong.

-- Q-ORD-6 [warn] order batal (grup cancelled) yang punya pembayaran/SJ/mutasi; Q-ORD-7 draft (pending) yang punya itu
SELECT os.status_group,COUNT(DISTINCT o.id_order) n,
  SUM(EXISTS(SELECT 1 FROM payments p WHERE p.id_order=o.id_order AND p.archived_at IS NULL)) w_pay,
  SUM(EXISTS(SELECT 1 FROM delivery_notes dn WHERE dn.id_order=o.id_order AND dn.archived_at IS NULL)) w_sj
FROM orders o JOIN order_status_definitions os ON os.id_order_status_definition=o.id_current_status
WHERE o.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND os.status_group IN ('cancelled','pending') GROUP BY 1;
-- tindakan: cancelled+bayar → pembayaran dimigrasi sebagai cancelled + temuan; pending+SJ → naikkan ke confirmed (temuan).

-- Q-ORD-8 [warn] pajak campuran per baris & penyimpangan total
SELECT o.id_order,o.order_number,COUNT(DISTINCT oi.taxable) mixed,
       ROUND(o.total_amount-SUM(oi.line_total_after_tax),2) diff_total,
       SUM(oi.quantity_in_base_uom<>oi.quantity*oi.uom_to_base_factor) bad_base
FROM orders o JOIN order_items oi ON oi.id_order=o.id_order
WHERE o.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c)
GROUP BY o.id_order HAVING mixed>1 OR ABS(diff_total)>0.009 OR bad_base>0;

-- Q-ORD-9 [info] sebaran per tahun (ukuran + dampak opening-date)
SELECT order_kind,YEAR(order_date) yr,COUNT(*) n,SUM(total_amount) total FROM orders
WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) GROUP BY 1,2 ORDER BY 1,2;

-- ===== UANG / DESIMAL (V9, risiko 2) =====
-- Q-MNY-1 [warn] nilai berpecahan (043 hanya memotong order tanpa pajak/diskon & tidak diarsip)
SELECT 'orders' t,SUM(total_amount<>FLOOR(total_amount)) frac,SUM(total_amount<>FLOOR(total_amount) AND archived_at IS NOT NULL) frac_archived,
       SUM(total_amount<>FLOOR(total_amount) AND (discount_amount<>0 OR subtotal_before_tax<>subtotal_after_tax)) frac_tax_or_disc
FROM orders WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c)
UNION ALL SELECT 'order_items',SUM(unit_price<>FLOOR(unit_price) OR unit_price_before_discount<>FLOOR(unit_price_before_discount)),SUM(line_total<>FLOOR(line_total)),NULL
 FROM order_items WHERE id_order IN (SELECT id_order FROM orders WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c))
UNION ALL SELECT 'payments',SUM(amount<>FLOOR(amount)),NULL,NULL FROM payments WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c)
UNION ALL SELECT 'journal_lines',SUM(debit_amount<>FLOOR(debit_amount) OR credit_amount<>FLOOR(credit_amount)),NULL,NULL FROM finance_journal_lines WHERE id_company=@c;
-- tindakan: tiap baris berpecahan masuk adjustments.csv; bila SUM selisih per partai ≠ 0 → daftar "selisih Rp1 piutang" ke akuntan (Q-MNY-2).

-- Q-MNY-2 [warn] sisa order Rp<1 / ±Rp1 setelah pembulatan (order berpajak/berdiskon & pembayarannya)
SELECT o.id_order,o.order_number,o.total_amount,
  COALESCE((SELECT SUM(p.amount) FROM payments p WHERE p.id_order=o.id_order AND p.archived_at IS NULL),0)
 +COALESCE((SELECT SUM(pa.allocated_amount) FROM payment_allocations pa WHERE pa.id_order=o.id_order AND pa.archived_at IS NULL),0) paid
FROM orders o WHERE o.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND o.archived_at IS NULL
HAVING ABS(o.total_amount-paid)>0 AND ABS(o.total_amount-paid)<1.5;

-- Q-INC-1 [warn] order inklusif pajak per jenis & status posting (V9)
SELECT o.order_kind,COUNT(*) n,SUM(o.tax_amount) tax,
  SUM(EXISTS(SELECT 1 FROM finance_posting_sources s WHERE s.id_company=@c AND s.event_key IN ('sales_completed','purchase_received')
            AND ((s.source_type='order' AND s.id_source=o.id_order) OR (s.source_type='goods_receipt' AND s.id_source IN (SELECT id_goods_receipt FROM goods_receipts WHERE id_order=o.id_order)))
            AND s.posting_status='posted')) posted_n
FROM orders o WHERE o.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND o.taxable=1 AND o.price_includes_tax=1 GROUP BY 1;
-- tindakan: sales inklusif yang 'posted'>0 mustahil (assertBalanced) → selidiki; GR inklusif posted → kandidat jurnal koreksi (nilai = tax_amount dobel di persediaan/hutang).

-- ===== PEMBAYARAN =====
-- Q-PAY-1 [info] pembayaran per-order tanpa alokasi (perlu alokasi sintetis) & yatim
SELECT SUM(p.id_order IS NOT NULL AND NOT EXISTS(SELECT 1 FROM payment_allocations a WHERE a.id_payment=p.id_payment AND a.archived_at IS NULL)) need_synth,
       SUM(p.id_order IS NULL AND NOT EXISTS(SELECT 1 FROM payment_allocations a WHERE a.id_payment=p.id_payment AND a.archived_at IS NULL)) orphan,
       SUM(p.id_order IS NOT NULL AND EXISTS(SELECT 1 FROM payment_allocations a WHERE a.id_payment=p.id_payment AND a.archived_at IS NULL)) both_forms
FROM payments p WHERE p.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND p.archived_at IS NULL;
-- orphan>0 → blocker.
-- Q-PAY-2 [blocker] Σ alokasi ≠ jumlah pembayaran
SELECT p.id_payment,p.payment_number,p.amount,SUM(a.allocated_amount) alloc FROM payments p JOIN payment_allocations a ON a.id_payment=p.id_payment AND a.archived_at IS NULL
WHERE p.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND p.archived_at IS NULL GROUP BY p.id_payment HAVING ABS(p.amount-alloc)>0.009;
-- Q-PAY-3 [warn] metode & cek; bukti
SELECT payment_method,financial_side,COUNT(*) n,SUM(proof_file_path IS NOT NULL) w_proof,SUM(amount_tendered IS NOT NULL) w_tend,SUM(archived_at IS NOT NULL) arch
FROM payments WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) GROUP BY 1,2;
-- check>0 → keputusan P4 (peta ke transfer + catatan, atau tambah metode di schema).
-- Q-PAY-4 [warn] pembayaran pada order terarsip/batal (tidak pernah dikandidatkan posting, finance-posting.service.ts:1500)
SELECT COUNT(*) n,SUM(p.amount) amt FROM payments p JOIN orders o ON o.id_order=p.id_order
WHERE p.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND p.archived_at IS NULL AND o.archived_at IS NOT NULL;

-- ===== PIHAK & PRODUK =====
-- Q-PTY-1 [blocker bila partner>0] tipe pihak & kode ganda
SELECT party_type,COUNT(*) n,SUM(archived_at IS NOT NULL) arch FROM business_parties WHERE id_company=@c GROUP BY 1;
-- Q-PRD-1 [warn] tipe produk × stock_tracked × punya mutasi
SELECT p.product_type,p.stock_tracked,COUNT(*) n,SUM(EXISTS(SELECT 1 FROM inventory_movements im WHERE im.id_product=p.id_product)) w_mov
FROM products p WHERE p.id_company=@c GROUP BY 1,2;
-- tindakan: service/non_stock→jasa; jasa dgn mutasi → temuan.
-- Q-PRD-2 [blocker] barcode '' / ganda / panjang
SELECT SUM(barcode='') empty_bc, COUNT(*)-COUNT(DISTINCT NULLIF(barcode,'')) dup_or_null_bc, MAX(CHAR_LENGTH(variant_code)) mx_vcode FROM product_variants WHERE id_company=@c;
SELECT NULLIF(barcode,'') bc,COUNT(*) FROM product_variants WHERE id_company=@c AND barcode<>'' GROUP BY 1 HAVING COUNT(*)>1;

-- ===== STOK =====
-- Q-STK-1 [warn→keputusan] drift saldo vs Σ mutasi (balance_after−balance_before)
SELECT ib.id_branch,ib.id_product,ib.id_product_variant,ib.id_stock_location,ib.on_hand_qty,COALESCE(m.net,0) mov_net
FROM inventory_balances ib LEFT JOIN (SELECT id_branch,id_product,id_product_variant,id_stock_location,SUM(balance_after-balance_before) net
   FROM inventory_movements GROUP BY 1,2,3,4) m USING(id_branch,id_product,id_product_variant,id_stock_location)
WHERE ib.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND ABS(ib.on_hand_qty-COALESCE(m.net,0))>0.000000000001;
-- tindakan: mutasi koreksi 'Koreksi migrasi' (F5-6) + daftar ke pemilik.
-- Q-STK-2 [blocker bila NULL/tak dikenal] sebaran reference_type
SELECT reference_type,movement_type,COUNT(*) n FROM inventory_movements WHERE id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) GROUP BY 1,2 ORDER BY 3 DESC;
-- Q-STK-3 [info] serah langsung tanpa SJ → SJ sintetis
SELECT COUNT(DISTINCT id_reference) orders_direct,COUNT(*) mov FROM inventory_movements WHERE reference_type='order' AND id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c);
-- Q-STK-4 [blocker] item SJ stok-terlacak tanpa alokasi lengkap
SELECT dni.id_delivery_note_item,dni.quantity_in_base_uom,COALESCE(SUM(a.quantity_in_base_uom),0) alloc
FROM delivery_note_items dni JOIN delivery_notes dn ON dn.id_delivery_note=dni.id_delivery_note
LEFT JOIN stock_issue_allocations a ON a.id_delivery_note_item=dni.id_delivery_note_item
WHERE dn.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND dn.archived_at IS NULL
GROUP BY dni.id_delivery_note_item HAVING ABS(dni.quantity_in_base_uom-alloc)>0.000000000001;
-- tindakan: lokasi diturunkan dari mutasi 'delivery_note' ber-id_reference; sisa → 'GDG' + temuan.
-- Q-STK-5 [blocker bila >0 saat freeze] reservasi aktif
SELECT COUNT(*) FROM stock_reservations WHERE status='active' AND id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c);
-- Q-STK-6 [info] tabrakan kunci unik idempotensi F2a (akan digabung)
SELECT reference_type,id_reference,id_product,id_product_variant,id_stock_location,movement_type,COUNT(*) n FROM inventory_movements
WHERE id_reference IS NOT NULL AND id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) GROUP BY 1,2,3,4,5,6 HAVING n>1;
-- Q-STK-7 [warn] lokasi: default≠1 per cabang, lokasi rusak, kode >30
SELECT b.id_branch,SUM(l.is_default) defaults,SUM(l.status='damaged') damaged,MAX(CHAR_LENGTH(l.code)) mx_code
FROM branches b JOIN stock_locations l ON l.id_branch=b.id_branch WHERE b.id_company=@c GROUP BY 1;
-- Q-STK-8 [info] transfer dengan >1 pasangan lokasi
SELECT t.id_stock_transfer,t.transfer_number,COUNT(DISTINCT CONCAT(i.id_from_location,'>',i.id_to_location)) pairs
FROM stock_transfers t JOIN stock_transfer_items i ON i.id_stock_transfer=t.id_stock_transfer WHERE t.id_company=@c GROUP BY 1 HAVING pairs>1;

-- ===== HPP =====
-- Q-HPP-1 [info] sebaran baris biaya; yang tanpa mutasi
SELECT movement_type,source_type,COUNT(*) n,SUM(id_inventory_movement IS NULL) no_mov,SUM(id_product_variant IS NULL) no_var FROM finance_inventory_cost_movements WHERE id_company=@c GROUP BY 1,2;
-- Q-HPP-2 [warn] kuantitas pembukaan di cost vs mutasi opening_stock (risiko ganda hitung)
SELECT c.id_branch,c.id_product,SUM(c.quantity_in_base_uom) cost_open,
  COALESCE((SELECT SUM(im.quantity) FROM inventory_movements im WHERE im.id_branch=c.id_branch AND im.id_product=c.id_product AND im.reference_type='opening_stock'),0) mov_open
FROM finance_inventory_cost_movements c WHERE c.id_company=@c AND c.movement_type='opening' GROUP BY 1,2;
-- Q-HPP-3 [warn] mutasi masuk/keluar tanpa baris biaya (belum diposting) per cabang/ref
SELECT im.id_branch,im.reference_type,COUNT(*) n FROM inventory_movements im
WHERE im.id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c) AND im.movement_type IN ('in','out')
  AND NOT EXISTS(SELECT 1 FROM finance_inventory_cost_movements c WHERE c.id_inventory_movement=im.id_inventory_movement) GROUP BY 1,2;
-- tindakan: baris biaya estimasi (F5-6) bila ID < kursor.
-- Q-HPP-4 [info] nilai ledger vs state
SELECT s.id_branch,s.id_product,s.inventory_value,
  (SELECT c.inventory_value_after FROM finance_inventory_cost_movements c WHERE c.id_branch=s.id_branch AND c.id_product=s.id_product ORDER BY c.id_finance_inventory_cost_movement DESC LIMIT 1) last_after
FROM finance_inventory_cost_states s WHERE s.id_company=@c HAVING ABS(inventory_value-COALESCE(last_after,0))>0.009;

-- ===== JURNAL =====
-- Q-JRN-1 [warn] jurnal tanpa cabang (pusat) per event/status
SELECT event_key,status,COUNT(*) n,SUM(YEAR(journal_date)>=YEAR(@open)) cur FROM finance_journal_entries WHERE id_company=@c AND id_branch IS NULL GROUP BY 1,2;
-- tindakan: → cabang head + temuan; opening_balance legacy tidak dimigrasi (diganti jurnal saldo awal).
-- Q-JRN-2 [blocker >0,009 / warn pecahan] jurnal tak seimbang
SELECT e.id_finance_journal_entry,e.journal_number,e.event_key,SUM(l.debit_amount) d,SUM(l.credit_amount) c
FROM finance_journal_entries e JOIN finance_journal_lines l ON l.id_finance_journal_entry=e.id_finance_journal_entry
WHERE e.id_company=@c AND e.status IN ('posted','reversed') GROUP BY e.id_finance_journal_entry HAVING ABS(d-c)>0.0001;
-- Q-JRN-3 [info] sebaran status/posting_type/tahun
SELECT status,posting_type,event_key,YEAR(journal_date) yr,COUNT(*) n FROM finance_journal_entries WHERE id_company=@c GROUP BY 1,2,3,4 ORDER BY yr,event_key;
-- Q-JRN-4 [blocker] nomor jurnal >60 karakter / deskripsi >255
SELECT MAX(CHAR_LENGTH(journal_number)) mx_no,SUM(CHAR_LENGTH(description)>255) desc_gt255 FROM finance_journal_entries WHERE id_company=@c;
-- Q-JRN-5 [blocker] akun yang dipakai jurnal tanpa peta COA
SELECT a.id_finance_account,a.account_code,a.account_name,a.system_key,COUNT(l.id_finance_journal_line) n_lines
FROM finance_accounts a LEFT JOIN finance_journal_lines l ON l.id_finance_account=a.id_finance_account WHERE a.id_company=@c GROUP BY a.id_finance_account
HAVING n_lines>0 AND (a.system_key IS NULL OR a.system_key NOT IN ('cash_cash','cash_bank','accounts_receivable','inventory','input_tax','accounts_payable','output_tax','sales_revenue','cogs','opening_equity','customer_advance','supplier_advance','stock_difference','rounding_difference','purchase_return_difference'));
-- baris hasil = akun buatan pengguna yang dipakai jurnal: wajib punya entri di mapping/coa.yaml (bukan error bila ada, tetapi wajib terpeta)
-- Q-JRN-6 [info] jurnal sebelum/sesudah opening-date, per cabang
SELECT id_branch,SUM(DATE(CONVERT_TZ(journal_date,'+00:00','+07:00'))<@open) prior_n,SUM(DATE(CONVERT_TZ(journal_date,'+00:00','+07:00'))>=@open) cur_n FROM finance_journal_entries WHERE id_company=@c AND status IN ('posted','reversed') GROUP BY 1;
-- Q-JRN-7 [warn] jurnal GR inklusif terposting (kandidat koreksi, V9)
SELECT COUNT(*) n,SUM(s.source_amount) amt FROM finance_posting_sources s JOIN goods_receipts gr ON gr.id_goods_receipt=s.id_source JOIN orders o ON o.id_order=gr.id_order
WHERE s.id_company=@c AND s.event_key='purchase_received' AND s.posting_status='posted' AND o.taxable=1 AND o.price_includes_tax=1;
-- Q-JRN-8 [info] antrean posting legacy per event/status/tahun (menentukan angka pending pasca-cutover)
SELECT event_key,posting_status,YEAR(source_date) yr,COUNT(*) n FROM finance_posting_sources WHERE id_company=@c GROUP BY 1,2,3 ORDER BY yr,event_key;
-- Q-JRN-9 [warn] dokumen terposting sebagian (sales_completed posted tapi COGS pending, dst.)
SELECT o.id_order,o.order_number FROM orders o JOIN finance_posting_sources s ON s.source_type='order' AND s.id_source=o.id_order AND s.event_key='sales_completed' AND s.posting_status='posted'
WHERE o.order_kind='sales' AND EXISTS(SELECT 1 FROM finance_posting_sources c WHERE c.id_company=@c AND c.event_key='stock_issue_cogs' AND c.posting_status<>'posted'
  AND c.id_source IN (SELECT id_inventory_movement FROM inventory_movements WHERE (reference_type='order' AND id_reference=o.id_order) OR (reference_type='delivery_note' AND id_reference IN (SELECT id_delivery_note FROM delivery_notes WHERE id_order=o.id_order))));

-- ===== RETUR =====
-- Q-RET-1/2 [warn] retur per mode/status/penyelesaian; retur tukar
SELECT return_mode,status,settlement_type,COUNT(*) n,SUM(replacement_total_amount) repl,SUM(difference_amount) diff FROM sales_returns WHERE id_company=@c GROUP BY 1,2,3;
SELECT status,settlement_type,COUNT(*),SUM(returned_total_amount) FROM purchase_returns WHERE id_company=@c GROUP BY 1,2;
SELECT srs.settlement_type,COUNT(*),SUM(srs.amount) FROM sales_return_settlements srs JOIN sales_returns sr ON sr.id_sales_return=srs.id_sales_return WHERE sr.id_company=@c AND srs.archived_at IS NULL GROUP BY 1;
-- Q-RET-3 [warn] item retur tanpa lokasi / kondisi rusak
SELECT SUM(id_stock_location IS NULL) no_loc,SUM(return_condition='damaged') damaged,SUM(return_condition='not_returned') not_returned FROM sales_return_items WHERE id_sales_return IN (SELECT id_sales_return FROM sales_returns WHERE id_company=@c);

-- ===== PENGGUNA, MEDIA, LAIN =====
-- Q-USR-1 [blocker bila dup] username/email ganda (case-insensitive), pengguna tanpa peran, hash non-bcrypt, status
SELECT LOWER(username) u,COUNT(*) FROM users WHERE id_company=@c GROUP BY 1 HAVING COUNT(*)>1;
SELECT SUBSTRING(password_hash,1,4) pref,status,COUNT(*) FROM users WHERE id_company=@c GROUP BY 1,2;
SELECT u.id_user,u.username FROM users u WHERE u.id_company=@c AND NOT EXISTS(SELECT 1 FROM user_roles r WHERE r.id_user=u.id_user);
-- Cek password default seed dilakukan di Go (bcrypt.CompareHashAndPassword) — hasil ke findings.csv.
-- Q-ROL-1 [blocker] permission/role legacy tanpa peta
SELECT p.permission_code,COUNT(rp.id_role) n_roles FROM permissions p LEFT JOIN role_permissions rp USING(id_permission) GROUP BY 1;
SELECT code,id_company,is_system_role,COUNT(*) FROM roles WHERE id_company=@c OR id_company IS NULL GROUP BY 1,2,3;
-- Q-MED-1 [info] media per pemilik/purpose/driver
SELECT owner_type,purpose,storage_driver,bucket,COUNT(*) n,SUM(size_bytes) bytes,SUM(archived_at IS NOT NULL) arch,MAX(CHAR_LENGTH(object_key)) mx_key FROM media_files WHERE id_company=@c GROUP BY 1,2,3,4;
-- Q-MED-2 [blocker] key > 255 / pemilik yatim
SELECT COUNT(*) FROM media_files WHERE id_company=@c AND CHAR_LENGTH(object_key)>255;
SELECT m.id_media_file FROM media_files m WHERE m.id_company=@c AND m.owner_type='delivery_note' AND NOT EXISTS(SELECT 1 FROM delivery_notes d WHERE d.id_delivery_note=m.owner_id);
-- Q-MED-3 payments.proof_file_path (tanpa baris media)
SELECT COUNT(*) n,SUM(CHAR_LENGTH(proof_file_path)>255) long_key FROM payments WHERE proof_file_path IS NOT NULL AND id_branch IN (SELECT id_branch FROM branches WHERE id_company=@c);
-- Q-AUD-1 [info] ukuran audit log per tahun
SELECT YEAR(happened_at) yr,COUNT(*) n FROM audit_logs WHERE id_company=@c GROUP BY 1;
-- Q-CFG-1 [info] kunci pengaturan perusahaan
SELECT JSON_KEYS(operational_rules_json) ops,JSON_KEYS(ui_preferences_json) ui,JSON_KEYS(reporting_preferences_json) rep FROM company_settings WHERE id_company=@c;
-- Q-PER-1 [info] periode & pajak tertutup
SELECT status,MIN(date_from),MAX(date_to),COUNT(*) FROM finance_periods WHERE id_company=@c GROUP BY 1;
-- Q-SEQ-1 [info] counter legacy
SELECT b.code,s.sequence_key,s.prefix,s.current_value FROM branch_document_sequences s JOIN branches b USING(id_branch) WHERE b.id_company=@c ORDER BY 1,2;
SELECT sequence_key,prefix,current_value FROM finance_document_sequences WHERE id_company=@c;
```

---------------------------------------------------------------------------------------------------

# LAMPIRAN C — Data yang tidak dapat dimigrasi tanpa kehilangan + penanganan

| Data legacy | Mengapa hilang | Penanganan usulan |
|---|---|---|
| `order_status_history` (riwayat status order) | fitur & tabel dibuang (D4) | arsip JSONL/CSV + indeks per nomor order; tidak dimuat |
| `order_status_definitions/transitions` (status kustom) | status tetap di kode | arsip |
| `orders.metadata_json`, `id_assigned_user`, `credit_approved/by/at`, `payment_term_days`, `goods_received_at`, `goods_delivered_at` | tak ada padanan | arsip `legacy_columns/orders.jsonl`; `credit_approved*` dipakai hanya untuk validasi termin; `goods_delivered_at` ikut tanggal SJ sintetis |
| `order_items`: snapshot harga member, `pricing_source`, `discount_reason`, `unit_cost_snapshot`/margin | tak ada kolom | arsip; margin dihitung ulang dari jurnal/HPP revamp |
| Diskon per-satuan & persentase (`line_discount_percent`) | revamp diskon per baris | direpresentasikan nominal baris; persentase asli di arsip |
| `delivery_notes`: tidak ada status; jejak alur dispatch↔konfirmasi | revamp 2-status | status disimpulkan; stempel waktu asli dibawa |
| `stock_issue_allocations` (alokasi/saran FIFO) | tabel dibuang (`DB_SCHEMA §6`) | lokasi masuk `delivery_note_items`; sisanya arsip |
| `stock_reservations` | transien | tidak dimigrasi |
| `inventory_movements.metadata_json`, `balance_before/after` | tak ada kolom (kecuali F2a-9) | arsip; saldo akhir dibawa |
| `finance_journal_*`: `event_key`, `metadata_json`, dimensi `id_order/id_payment/id_inventory_movement`, `posting_type` | P1/P2 desain (`000019`) | arsip `legacy_columns/journals.jsonl`; `event_key` ada di memo `[L:…]` |
| Jurnal sebelum tahun berjalan (detail) | keputusan #1 (saldo awal) | **arsip penuh** jurnal & baris tahun-tahun sebelumnya (CSV) + neraca saldo per akhir tahun sebagai bukti saldo awal |
| `finance_opening_balances`, `finance_opening_inventory_items`, `finance_inventory_cost_states` | diganti jurnal saldo awal / ledger | arsip |
| `finance_posting_sources` (antrean 2-fase, pesan error, hash) | diganti penanda | arsip; hanya status/alasan dibawa (A4) |
| `finance_tax_adjustments` (laba komersial, koreksi +/−, laba fiskal) | revamp memakai jurnal koreksi per akun; data legacy agregat tanpa akun | **arsip + input ulang manual oleh akuntan** sebagai jurnal `tax_adjustment` bila masih diperlukan (alternatif: ETL membuat 1 jurnal per periode dgn akun koreksi yang disepakati — butuh keputusan P6) |
| `finance_tax_report_snapshots` (angka SPT per periode) | per perusahaan, bukan per cabang | arsip (PDF/CSV) |
| `finance_periods` perusahaan | revamp per cabang | direplikasi ke semua cabang |
| `payments.proof_file_path` | tidak di media | dijadikan baris `media_files` (tidak hilang) |
| Media `knowledge_source`; purpose `delivery_location_photo` vs `delivery_signed_document` | owner `knowledge` dibuang; purpose melebur | knowledge → arsip; purpose dijaga di prefix `original_name` |
| `product.attributes_json`, `summary_text`, `variant.attributes_json`, `is_hidden`, `sort_order`, `has_variants` | revamp varian tanpa atribut | arsip; atribut varian pada snapshot item order dibawa (A3) |
| `business_parties.metadata_json`; party_type `partner` | revamp customer/supplier saja | arsip; `partner` perlu keputusan |
| `user_sessions` | sesi tidak dibawa | tidak dimigrasi (login ulang) |
| `users.last_login_at`, `display_title` | tak ada | arsip |
| `audit_logs` sebelum tahun berjalan + `before_json/after_json` | revamp: hanya aksi+note | arsip penuh; tahun berjalan dimuat ringkas |
| `knowledge_*`, `whatsapp_conversation_*`, `assistant_*`, `whatsapp_channels` | RAG dibuang; bot beda; sesi WA harus dipasangkan ulang | arsip; `whatsapp_authorizations` dimigrasi |
| `daily_operational_metrics`, `company_features`, `company_settings.business_labels/assistant/reporting` | mati/dibuang | arsip bila pemilik minta |
| `payment_allocations.allocation_method`, `allocated_at` | tak ada | arsip |
| Nilai pecahan sen (DECIMAL(18,2)) | ADR-0005 | `adjustments.csv` (setiap perubahan) |

---------------------------------------------------------------------------------------------------

# RISIKO

| ID | Risiko | Dampak | Mitigasi |
|---|---|---|---|
| R1 | Schema final berubah setelah ETL dibangun | transform/verify rusak | titik adaptasi tunggal (§1), fingerprint schema, fixture di CI; ETL baru final setelah squash |
| R2 | Penanda posting (A4) tak terwujud / berbeda | antrean berisi ribuan dokumen lama; `SafeClose` terblokir (`readiness.go:67`); atau posting ganda | adapter + `V-20`; jangan cutover tanpa A4; batas `QueueCap=200` juga perlu F2b (V11) |
| R3 | Data pecahan legacy → selisih Rp1 piutang | rekonsiliasi gagal | `adjustments.csv` + V-08/V-09 kategori "pecahan"; keputusan akuntan: serap ke akun `5200` migrasi atau biarkan sisa Rp1 (P3) |
| R4 | Jurnal GR inklusif legacy salah (persediaan/hutang bruto) | GL ≠ sub-ledger setelah dimigrasi apa adanya | dimigrasi apa adanya + temuan; jurnal koreksi manual (P2) |
| R5 | Ledger HPP: baris biaya tak lengkap / kursor | HPP penjualan baru salah | F5-6 aturan estimasi + tes "Sync tak mengubah apa pun"; V-14 |
| R6 | `inventory_movements` ukuran besar | memori/waktu | keyset + gz + bbolt; ukur di preflight |
| R7 | Jurnal bertipe `legacy_je` tak bisa dikecualikan fitur turnover/UMKM per dokumen (`finance/application/turnover.go`) | batas peredaran bruto tahun berjalan menghitung semua | beri tahu akuntan; atau pemetaan sumber untuk jurnal 1:1 (payment/GR) bila F2b menghendaki |
| R8 | Konvensi tanggal jurnal (UTC vs WIB) | pergeseran bulan | D-ETL-3; satu fungsi |
| R9 | Bucket S3 bersama | `Archive` revamp menghapus objek legacy | bucket terpisah, `guard` memeriksa nama bucket berbeda |
| R10 | Password default seed masih aktif di production | akses tak sah | kunci akun + paksa reset; daftar di go/no-go |
| R11 | Dump tidak konsisten (diambil saat legacy berjalan) | selisih | `mysqldump --single-transaction` + freeze; marker mencatat waktu |
| R12 | Parallel-run menggandakan beban kerja | kelelahan operator | batasi subset (kasir+1 cabang), waktu 1–2 minggu |
| R13 | `partner`, cek, retur tukar, order batal-berbayar ada di data nyata | peta tak cukup | Q-PTY-1, Q-PAY-3, Q-RET-1, Q-ORD-6 jalan di T-30 |
| R14 | Perbedaan perilaku bcryptjs vs Go | login gagal | tes kompatibilitas F5-13; fallback reset password massal |
| R15 | Waktu henti cutover | operasional terganggu | gladi mengukur durasi tiap tahap; jendela malam; target < 4 jam |

---------------------------------------------------------------------------------------------------

# PERTANYAAN TERBUKA (hanya yang butuh keputusan pemilik)

| # | Pertanyaan | Rekomendasi default | Perlu sebelum |
|---|---|---|---|
| P1 | Berapa `--opening-date` (awal tahun berjalan, mis. 2026-01-01) dan tanggal target cutover? | 2026-01-01 | F5-7 |
| P2 | Perlakuan jurnal penerimaan barang PO inklusif pajak yang sudah terposting di legacy (persediaan & hutang bruto, PPN masukan dobel): (a) migrasi apa adanya + jurnal koreksi manual akuntan, (b) ETL membuat jurnal koreksi otomatis per cabang/bulan, (c) kecualikan jurnal itu & posting ulang oleh revamp | (a) dengan laporan nilai per dokumen | F5-7, setelah Q-JRN-7 |
| P3 | Sisa selisih Rp1 akibat pecahan legacy pada piutang/hutang: diserap ke akun selisih pembulatan, atau dibiarkan | serap lewat 1 jurnal migrasi per cabang | setelah Q-MNY-1/2 |
| P4 | Pembayaran metode `check` (cek): tambah metode `check` di schema final (→ akun Bank), atau petakan ke `transfer` + catatan | tambah metode bila jumlah > 0 | squash schema |
| P5 | Model parallel-run: input paralel ke dua sistem (legacy tetap catatan sah) — dikonfirmasi? Alternatif: legacy read-only dan revamp langsung catatan sah (tanpa parallel) | model yang dirancang | T-30 |
| P6 | Koreksi fiskal legacy (`finance_tax_adjustments`): arsip + input ulang manual, atau ETL membuat jurnal koreksi dengan akun yang disepakati | arsip + input ulang manual | F5-7 |
| P7 | Party bertipe `partner` (bila ada) serta retur tukar (selisih sub-ledger): dipetakan ke customer/supplier atau dikecualikan? | ditentukan setelah Q-PTY-1 / Q-RET-1 | preflight |
| P8 | Audit log: cukup tahun berjalan dimuat, sisanya arsip? | ya | F5-12 |
| P9 | Retensi legacy read-only pasca-cutover (90 hari) & lokasi arsip Lampiran C | 90 hari, 2 lokasi | go/no-go |


