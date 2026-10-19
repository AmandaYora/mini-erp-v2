# Rencana F8 — Arsitektur/Redundancy, Migrasi sqlc, Jaring Pengaman Test

Status: LENGKAP (v2). Disusun dari pembacaan kode langsung; tiap klaim audit diberi label TERBUKTI / TERBANTAH / SEBAGIAN.
Ukuran: S (<= 1 hari), M (2-4 hari), L (>= 1 minggu). Repo revamp = `H:/dimasprasetio/SAAS/mini-erp-revamp` (API di `apps/api`),
legacy = `H:/dimasprasetio/SAAS/mini-erp`. Semua jalur file relatif terhadap `apps/api` kecuali disebut lain.
Skrip bukti (scratchpad): `inv.mjs`, `tblown.mjs`, `tblacc.mjs`, `match.js`, `graph.mjs`, `dups.mjs`, folder `sqlctrial/` (percobaan sqlc).

## 0. Keputusan terkunci pemilik yang mengikat rancangan ini

(a) sqlc bertahap, fase PALING AKHIR; (b) schema di-squash jadi baseline sebelum ETL; (c) atomisitas = idempotensi + CAS;
(d) POS = `client_ref` idempoten tanpa modul baru; (e) test integrasi di MySQL Laragon 8.4.3, database `mini_erp_test`.

## 1. Urutan kerja yang aman

| Urut | Tugas | Alasan urutan |
|---|---|---|
| 1 | F8-1 fondasi test + `internal/bootstrap` | Prasyarat semua test; menghapus duplikasi wiring main.go vs integration_test.go |
| 2 | F8-2 test rute/guard, F8-3 kontrak FE<->router | Murah, tak mengubah perilaku, langsung menangkap regresi guard |
| 3 | F8-4 test integrasi wajib (uang, konkurensi, sesi) | Jaring pengaman SEBELUM refactor apa pun menyentuh uang |
| 4 | F8-7 penyatuan rumus pajak/prorata | Setelah test properti ada, SEBELUM agen finance/retur mengubah builders (hindari konflik) |
| 5 | F8-8..F8-14 (helper, envelope, kode mati, wiring, tx/ADR) | Kecil, independen, risiko rendah |
| 6 | F8-15 arch-check diperkuat + self-test | Jadi pagar untuk F8-18 |
| 7 | F8-5 E2E Playwright, F8-6 target cakupan | Setelah UI/POS `client_ref` stabil |
| 8 | F8-18 migrasi sqlc | PALING AKHIR: setelah baseline final, ETL, deploy |

Dependensi lintas agen: baseline squash (agen schema) harus memakai BERKAS PER MODUL (lihat F8-18 §A, kritis); POS `client_ref`
(agen POS); doclock -> CAS (agen atomisitas); reporting/inventory (agen finance F2b-13 dan frontend F3a-10); shutdown (agen deploy/ops).

## 2. Verifikasi temuan audit (hasil bacaan sendiri)

| Klaim audit | Hasil | Bukti |
|---|---|---|
| sqlc.yaml 19 blok; queries/sqlc hanya .gitkeep; 319 panggilan mentah | TERBUKTI | 319 `QueryContext/ExecContext/QueryRowContext` di 24 berkas `*/infrastructure/*.go` (stock 41 di repository.go; finance 48 di 5 berkas; hitung kasar `.Query/.Exec` 25 assistant, 33 user, 30 product). Nol di luar infrastructure. Blok `dashboard` & `reporting` tak punya SQL (sia-sia); `assistant` (24 panggilan) tak punya blok. Modul ber-SQL = 18. |
| PARITY_AUDIT §5.4 "sqlc di 19 dari 20 modul" | TERBANTAH | 0 modul memakai sqlc; `grep sqlc` di `internal/` kosong. |
| Rule C arch-check hanya YAML | TERBUKTI | `scripts/arch-check.mjs` blok "Rule C": hanya membandingkan string queries/out. Tidak memeriksa berkas hasil generate. |
| Rule A longgar | TERBUKTI | regex `modules/<m>/(?!contracts...)` mensyaratkan `/` setelah nama modul: impor paket akar `.../modules/stock` lolos. Tak ada Rule untuk layer (domain/infra/app), tabel, FK, siklus, shared. |
| 21 dari 28 berkas infra memindai ke contracts | TERBUKTI | `grep -l modules/*/contracts */infrastructure/*.go` = 21 dari 28. |
| application memakai tipe infrastructure.* (26 berkas) | TERBUKTI | 26 berkas; `*infrastructure.Repository` 37x, `Transfer` 9x, `Expense` 6x. Hanya 3 berkas application mengimpor `database/sql`: stock/service.go (`*sql.Tx`), auth/service.go:306 (`toNullInt`), media/service.go:143 (`sql.ErrNoRows`). |
| stock 11 titik repo.Begin | SEBAGIAN | 12: service.go:191,242,345,420,443; transactions.go:102,154,196,300,436,480; opening.go:282. |
| domain 203 baris/6 berkas; 15 dari 20 modul tanpa domain | SEBAGIAN | 203 baris di 5 berkas non-test (party/pricing.go 86, product/uom.go 19, purchasing/pricing.go 39, sales/pricing.go 30, user/permissions.go 29). Application 16.259 baris, infrastructure 8.157, presentation 6.155, contracts 1.927 (TERBUKTI). `domain/events/` kosong di semua modul; `internal/router/` kosong. |
| 5 salinan rumus | TERBUKTI | sales/domain/pricing.go:8 == purchasing/domain/pricing.go:17 (identik byte); finance/application/builders.go:105(lineEconomics),:127(prorate); salesreturn/application/service.go:814,836; purchasereturn/application/service.go:596,618. `finance/application/posting.go:60 round()`. `shared/money/money.go` hanya `type Rupiah` + `String()`; TIDAK ada pengimpor. |
| Duplikasi helper kecil | TERBUKTI | nullIfEmpty x15, actorID x15, branchID x13, guarded x13, `type Permission func` x18, toFloat x7, nullIfZero x4 (+nullInt64 x3 +nullRefID x2 semantik sama), boolToInt x3, truncateCell x2, formatNullTime x2, resolveDate x2, openWorkbook/openOpeningBook x2 (badan identik). Total badan identik >= 5 baris: 466 baris (`dups.mjs`) = 1,3% dari 35.200 baris non-test. |
| Twin sales/purchasing 69%, salesreturn/purchasereturn 66% | SEBAGIAN | Setelah normalisasi nama, service sales vs purchasing berbeda 295 dari ~1000 baris; yang IDENTIK persis hanya SetStatus(22), StatusCounts(16), totals(10), ListByParty(10), OrderParties(7), resolveDate(6). Kemiripan struktural bukan duplikasi mekanis; aturan bisnisnya berbeda (kredit, ship-to, member price vs faktur pemasok). |
| Envelope error di-hardcode di 4 tempat | SEBAGIAN | 2 berkas: auth/contracts/session.go:98-115 (3 fungsi) + :185-189 (`deny`) dan server/middleware.go:28. `finance/presentation/closing.go:268` BUKAN envelope (itu `w.Write(raw)` unduhan xlsx): TERBANTAH. |
| `map[string]any` 81 titik | TERBUKTI | 81 literal `map[string]any{` (109 sebutan) di 6.155 baris presentation; terbanyak stock 12, finance handler 11, salesreturn 11. |
| 2 pengecualian parse limit manual | TERBUKTI | dashboard/presentation/handler.go:41 `limitParam` (Atoi tanpa batas atas); finance/presentation/reports.go:208 (`limit, _ := strconv.Atoi`). |
| api-standard.md `total_pages` | TERBUKTI salah di dokumen | `shared/response.Meta` JSON `totalPages`; FE `http-client.ts` & API_CONTRACT.md `totalPages`. Ubah dokumen. |
| Siklus setter (6 pasang) | SEBAGIAN | Setter nyata = 5: `userMod.SetAuthzInvalidator`, `branchMod.SetAuthzInvalidator` (main.go ~l.70-71), `salesreturnMod.SetPayments`, `purchasereturnMod.SetPayments`, `financeMod.SetOps`. `finance.SetOps` BUKAN siklus: sales/purchasing dibangun sebelum finance dan tak mengimpor finance. salesreturn/purchasereturn hanya memakai SATU metode payment (`HasActiveAllocations`, salesreturn/service.go:453, purchasereturn/service.go:406). "Siklus" audit<->user dst. hanyalah semua modul mengimpor `auth/contracts` & `user/contracts` (PermissionSeed); Go menolak siklus paket, jadi bukan siklus nyata. Setelah mengabaikan auth/user/audit, satu-satunya siklus modul = payment<->salesreturn dan payment<->purchasereturn (`graph.mjs`). |
| 8 item `unused` | TERBUKTI persis | `golangci-lint run -E unused` (v2.14.0): assistant/application/service.go:138, assistant/presentation/handler.go:30, finance/application/posting.go:41, finance/application/turnover.go:262, party/infrastructure/repository.go:322, purchasing/application/service.go:125,212,219. Plus 4 temuan lain: errcheck stock_flows_test.go:15, ineffassign cmd/seed/main.go:106, staticcheck QF1003 stock/opening.go:141, QF1008 server/compress.go:86. Accessor `SalesOps/PurchasingOps/Dashboard/Reporting/Assistant/Checker`: nol pemanggil (TERBUKTI). |
| 6 endpoint tak dipanggil FE | TERBUKTI | `match.js`: 253/253 panggilan FE cocok router; rute tak terpakai: auth/switch-role, finance/costing/{sync,uncosted}, stock/allocations/suggest, stock/reservations/{hold,release} (+ auth/refresh, dipanggil lewat interceptor, bukan wrapper). 217 rute backend. |
| Dependensi tak terpakai | TERBANTAH (Go & web) | `go mod tidy -diff` kosong; `modernc.org/sqlite` dipakai gateway WhatsApp (store whatsmeow, assistant/infrastructure/gateway.go:24). Semua dependensi `apps/web` diimpor. Yang mati: workspace `packages/shared` (`export {}`) dan `packages/api-contract` (README saja). |
| Tabel/FK lintas modul | TERBANTAH (bersih) | `tblacc.mjs`: 0 akses tabel milik modul lain di literal SQL; `tblown.mjs`: 59 tabel, 32 FK, semuanya satu modul. Perlu penjaga otomatis agar tetap bersih (F8-15). |
| Backend 35 file/97 Test; 15 integrasi | TERBUKTI (dengan catatan) | 35 berkas `_test.go`, 97 `func Test` termasuk `TestMain` => 96 test nyata, 39 di integrasi. Tidak ada satu pun `go func`/`WaitGroup` di test (nol uji konkurensi), tidak ada `httptest` untuk rute (hanya compress_test & session_test). PARITY_AUDIT §5.1 ("73 func, 35 test integrasi") sudah usang. |
| Unit coverage | BARU | Unit-only (`go test -cover`, tanpa integrasi): application stock 0%, user 0%, sales 0%, salesreturn 3%; domain sales 93%; shared/money 100%. Cakupan integrasi (`-coverpkg`) belum diukur: langkah pertama F8-1. |

Temuan baru yang tidak ada di audit:
1. `GET /api/v1/<rute-tak-dikenal>` mengembalikan `index.html` 200 (catch-all SPA `server.MountStatic`), bukan 404 JSON (server/server.go:43-56).
2. Harness integrasi tak punya pengaman nama DB: `TEST_DB_DSN` apa pun akan di-TRUNCATE semua tabel (`integration_test.go:79-105`) dan `TESTDB_REBUILD` men-DROP DATABASE. Selain itu "reusing migrated test db" memakai schema lama diam-diam bila migrasi berubah.
3. `auth.Rotate` (auth/infrastructure/repository.go:105) `UPDATE ... WHERE id = ?` tanpa CAS pada `refresh_hash`: dua refresh bersamaan dengan token sama sama-sama sukses; token milik pemenang pertama menjadi `prev_refresh_hash` sehingga pemakaian berikutnya dianggap replay dan MENCABUT SEMUA sesi user (service.go:342-353). Hipotesis dari pembacaan kode; test konkurensi F8-4 akan membuktikan.
4. Wiring integrasi (`integration_test.go:158-190`) adalah salinan main.go yang sudah menyimpang: tanpa auth, dashboard, reporting, assistant, sehingga rute & guard tak pernah diuji.
5. Ketidaksesuaian dokumen: DEPLOYMENT.md "Go 1.21+" vs `go.mod` `go 1.26.0`/toolchain 1.26.7; BACKEND_GUIDE.md:58 "CLI migrate/sqlc belum terpasang" (sudah: sqlc v1.31.1, migrate, air, golangci-lint 2.14.0); migrasi bernomor 000001-000025 lalu 000030-000031 (celah 000026-000029; 27 berkas up, CLAUDE.md menulis 26).

---

## 3. Tugas

### F8-1 Fondasi test: guard DB, bootstrap bersama, fixture, ukur baseline  — ukuran M

Masalah: lihat temuan baru 2 dan 4. `internal/integration/integration_test.go` menyalin wiring main.go dan tak punya pengaman.

Rancangan:
1. `internal/bootstrap/bootstrap.go`: `func Build(cfg *config.Config, db *sql.DB, log *slog.Logger, opts ...Option) (*App, error)` berisi seluruh konstruksi modul (isi main.go l.57-103 dan integration_test.go l.158-190), mengembalikan `App{Modules, Mux-registrar}` dengan `RegisterRoutes(mux Router)`. `cmd/server/main.go` dan `cmd/seed/main.go` dan test integrasi memakai fungsi yang SAMA. `internal/bootstrap` dicatat sebagai composition root yang sah mengimpor akar modul (F8-15 Rule A).
2. `Router` = `interface{ Handle(pattern string, h http.Handler) }` menggantikan `*http.ServeMux` pada 19 `RegisterRoutes` (mekanis). Dipakai F8-2 untuk merekam pola rute.
3. Guard DB: `testdb.Open(t)` menolak bila nama DB tidak cocok `^[a-z0-9_]+_test$` atau host bukan `localhost/127.0.0.1` (kecuali `CI=true`); menolak `TESTDB_REBUILD` pada nama lain. Nama default tetap `mini_erp_test`.
4. Deteksi schema usang: tabel `_test_schema(hash CHAR(64))`; hash = SHA-256 gabungan isi `migrations/*.up.sql`; beda => DROP+CREATE otomatis (menggantikan "reusing migrated test db" yang diam-diam salah). Setelah squash, hash baseline.
5. `internal/testkit` (paket biasa, bukan `_test`, agar bisa dipakai integrasi dan `cmd/seed -profile=e2e`): `NewShop`, `NewCustomer`, `NewSupplier`, `Product(..)`, `Stock(..)`, `LoginAs(role|perms...)` (membuat user+role dengan izin tertentu lalu menandatangani token lewat auth service), `Clock`. `shopFixture/newShop/sellFixture` dari integration_test.go dipindah ke sini.
6. Isolasi: satu paket integrasi, SERIAL (tanpa `t.Parallel`); `freshDB` TRUNCATE tetap. Paralel antar-paket hanya dengan DB berbeda (`mini_erp_test_<slot>`) bila nanti diperlukan; tidak sekarang (59 tabel x ~100 test masih murah). Skenario konkurensi memakai goroutine DI DALAM satu test (DB sama).
7. Ukur baseline: `go test -coverpkg=./internal/modules/...,./internal/shared/... ./... -coverprofile=cover.out` ke `coverage-baseline.json` (jadi pegangan F8-6). Tambah `-race` pada job CI untuk paket non-integrasi (integrasi + race opsional karena lambat).
8. CI: `.github/workflows/ci.yml` (asumsi GitHub Actions; lihat Pertanyaan terbuka #3): job `go` (MySQL 8.4 service, `go vet`, golangci-lint v2.14.0 dengan `.golangci.yml` yang mengaktifkan unused, staticcheck, errcheck, ineffassign, govet; `go test ./...`), job `web` (tsc, eslint, vitest), job `arch` (`node scripts/arch-check.mjs`, `sqlc diff` setelah F8-18), job `e2e` (F8-5). Saat ini TIDAK ADA CI sama sekali (tak ada `.github/`).
Reuse terverifikasi: `truncateAll`, `splitDSN`, pola `mods` di integration_test.go; `cmd/seed/main.go` (dev seed, user admin).
Test: `TestBuildWiresEverything` (Build sukses di DB uji), `TestRefuseNonTestDB` (tabel nama), `TestSchemaHashRebuild`.
Dependensi: tidak ada (mulai paling awal). Risiko: konflik dengan agen yang menyunting main.go; lakukan sebagai PR kecil pertama.

### F8-2 Test HTTP rute + guard permission otomatis — ukuran M

Rancangan: `internal/integration/routes_test.go` memakai `bootstrap.Build` dengan `Router` perekam: `recorder.Handle(pattern, h)`; `perm` palsu `func(code, next) http.Handler` membungkus handler dalam `taggedHandler{code,next}` sehingga pola->kode izin terbaca tanpa parsing sumber. Hasil:
1. Golden `testdata/routes.golden` (pattern, kode izin, ada-tidaknya guard cabang), dibuat dengan `-update`. Perubahan izin jadi diff yang harus ditinjau. Sumber yang sama dipakai F8-3.
2. Table-driven, SETIAP rute (217 hari ini) x kasus: (a) tanpa Authorization -> 401 + envelope `{success:false,message}`; (b) token valid user TANPA izin tsb (`testkit.LoginAs()` tanpa izin) -> 403; (c) bila rute berguard cabang: token ber-izin tapi tanpa cabang aktif -> 403 "Pilih cabang aktif..."; (d) hanya untuk GET (tanpa efek samping): token ber-izin+cabang -> bukan 401/403. POST/PUT/DELETE tidak dijalankan sampai handler (hindari efek samping, mis. `assistant/connect` membuka WhatsApp); daftar kecualian eksplisit `skipPassThrough` dengan alasan.
3. Rute publik (health, auth/login, auth/refresh) harus ada di allowlist bernama; rute lain dengan kode izin kosong -> gagal.
4. Rute tak dikenal `/api/v1/xyz` -> 404 envelope JSON (butuh perbaikan: daftarkan `mux.HandleFunc("/api/", notFoundJSON)` di `server.New` yang lebih spesifik dari catch-all SPA; metode salah pada rute ada ikut 404 JSON, diterima).
5. Bentuk respons: validasi tiap respons error mengikuti envelope; snapshot JSON (golden) untuk 15 endpoint inti sebagai prasyarat DTO bertipe (F8-19).
Test: itu sendiri. Performa: tanpa handler berat, < 5 detik. Dependensi: F8-1. Risiko: `perm` fake harus identik semantik dengan `authMod.Permission` -> sebagian kasus (b)/(c) dijalankan dengan `authMod.Permission` asli (jalur produksi) bukan fake; fake hanya untuk enumerasi.

### F8-3 Kontrak FE <-> router otomatis — ukuran S

Rancangan: `scripts/contract-check.mjs` = produksi dari `match.js` (sudah terbukti 253/253): membaca `apps/api/internal/integration/testdata/routes.golden` (bukan parsing sumber) dan memindai `apps/web/src/**` untuk wrapper `api*()`/`httpClient.*` dengan literal/template. Gagal bila: (1) panggilan FE tak punya rute+metode di golden; (2) rute backend tak dipanggil FE dan tak ada di `scripts/contract-allow.json` (alasan wajib: hold/release = POS client_ref, switch-role = UI akan datang, suggest = POS, costing = keputusan F8-12); (3) string izin FE (`perm:` di registry.tsx/protected.routes.tsx) tak ada di katalog izin golden; (4) panggilan dengan URL non-literal (tak dapat diverifikasi) di luar allowlist. Jalan di job `arch`/CI dan di `npm run verify`.
Test: fixture mini (FE+golden palsu) di `scripts/__fixtures__/contract/` + `node --test scripts/contract-check.test.mjs`.
Dependensi: F8-2 (golden). Catatan batas: panggilan yang dibangun dari variabel tak terdeteksi; tambahkan aturan ESLint "argumen pertama apiX harus literal/template".

### F8-4 Test integrasi Go yang WAJIB (uang, konkurensi, sesi) — ukuran L

Prinsip (keputusan (c)): setiap operasi yang bisa diklik dua kali atau di-retry harus punya (i) test serial "retry setelah gagal di tengah" dan (ii) test konkurensi N goroutine yang menegaskan invarian akhir. Helper `race(t, n, func(i int) error)` melepas goroutine bersamaan (barrier channel) dan mengembalikan hasil. Nama berkas `internal/integration/<domain>_test.go`.

Daftar WAJIB (ID T-xx; status "ada" = sudah ada test serupa):
Auth/sesi (`auth_session_test.go`, via bootstrap + HTTP): T-01 login sukses/gagal/dibatasi laju; T-02 refresh merotasi, token lama ditolak; T-03 replay token lama mencabut semua sesi; T-04 DUA refresh bersamaan token sama -> tepat satu sukses, sesi pihak lain TIDAK dicabut (akan merah sampai CAS dipasang; lihat Pertanyaan #7); T-05 cabut izin role -> request berikutnya 403 (invalidator hub, F8-13); T-06 cabang ditutup -> 403; T-07 switch-branch/role mengubah cakupan data.
Stok: T-10 dua pengeluaran bersamaan atas stok 1 -> tepat satu sukses, saldo >= 0 (adjust/delivery); T-11 `Hold` kunci sama bersamaan -> satu reservasi (ada `TestReservationHoldRelease` serial); T-12 `Release` ganda; T-13 transfer: dispatch/receive diklik dua kali; T-14 impor stok awal diulang idempoten (ada `TestOpeningRejectsUnbalancedAndIdempotent` sebagian); T-15 pindah lokasi satu langkah (ada).
Pembelian: T-20 PO confirm -> GR parsial -> GR kedua bersamaan melebihi sisa -> tepat satu sukses (doclock `po:` hari ini; harus tetap benar setelah diganti CAS); T-21 GR -> posting jurnal (Dr Persediaan/PPN Masukan, Cr Hutang) seimbang dan idempoten; T-22 PO prabayar wajib lunas sebelum GR; T-23 retur pembelian parsial lalu penuh bersamaan (ada `TestPurchaseReturnBothDiffSigns` serial).
Penjualan: T-30 rantai SO confirm -> SJ create+confirm -> bayar sebagian -> bayar lunas -> posting -> laporan: tegaskan Aset = Kewajiban + Ekuitas pada neraca, laba rugi = penjualan - HPP - biaya, saldo kas = total pembayaran; ini test "rantai" utama (ada potongan: `TestDeliveryEmptiesStockBooksFullCOGS`, `TestPaymentCreateAllocation`); T-31 SJ confirm dua kali bersamaan -> stok keluar sekali; T-32 nomor dokumen unik di bawah 50 `CreateOrder` bersamaan (`NextDocumentNumber`/LAST_INSERT_ID); T-33 pembayaran bersamaan total > grand -> satu ditolak (ada `TestPaymentOverpayRejected` serial); T-34 batalkan pembayaran dan hitung ulang (ada); T-35 retur penjualan melebihi qty bersamaan -> satu ditolak (ada serial `TestSettlementCap`, `TestReturnUnmatchedLineRejected`).
POS (setelah `client_ref`): T-40 checkout tunai: SO+confirm+SJ+confirm+bayar = 1 order, stok -qty, kas +total; T-41 checkout bayar-nanti wajib customer, masuk piutang; T-42 kirim `client_ref` sama dua kali (berurutan dan bersamaan) -> satu order, satu pengurangan stok; T-43 gagal di tengah (stub stok/pembayaran melempar error pada panggilan ke-k, k=1..5) lalu retry dengan `client_ref` sama -> selesai tanpa ganda dan tanpa setengah jadi; T-44 stok kurang ditolak tanpa sisa dokumen.
Finance: T-50 `Post` dokumen sama bersamaan 10 goroutine -> satu jurnal (ada serial `TestFinancePostIdempotent`); T-51 `NextSequence` 50 goroutine -> nomor unik, tanpa lubang; T-52 laporan laba rugi/neraca/arus kas konsisten dengan jurnal untuk dataset tetap (angka golden); T-53 tutup periode vs posting bersamaan; T-54 safe-close gate (ada).
Master/impor: T-60 impor produk xlsx: baris valid/invalid, ulang idempoten (tak duplikat), batas ukuran; T-61 unggah media: tipe/ukuran, driver lokal, nama aman; T-62 impor stok awal xlsx (ada sebagian).
WhatsApp/assistant: T-70 mesin aturan dengan gateway palsu (antarmuka `Gateway` di application sudah dipisah; engine_test.go ada 3 test): whitelist nomor, rate limit, perintah gagal tak membocorkan; gateway asli tidak diuji.
Handler HTTP: T-80 envelope + paginasi `meta.totalPages` benar (httptest di F8-2).
Injeksi gagal: `testkit.FailOn(client, method, nth)` — pembungkus klien kontrak (stock/payment/sales) yang mengembalikan error pada panggilan ke-n; dipakai T-43 dan retur.
Performa/volume: satu benchmark `BenchmarkPostingQueue1000`, `BenchmarkListOrders50k` (data pembangkit) sebagai pengganti legacy spec 11 (non-CI, manual).
Reuse: pola `mods`/`shopFixture`; 39 test integrasi yang ada tetap dipakai sebagai oracle untuk F8-7 dan F8-18.
Dependensi: F8-1; T-04/T-20/T-31/T-42 menjadi pagar untuk agen atomisitas (doclock -> CAS) — sebaiknya ditulis SEBELUM doclock diganti. Risiko: test konkurensi bisa flaky bila bergantung pada timing; gunakan barrier + asersi invarian akhir, bukan urutan.

### F8-5 E2E Playwright minimal-tapi-bermakna — ukuran L

Struktur: workspace baru `apps/e2e` (tambahkan ke `workspaces`), mengikuti legacy `mini-erp/apps/e2e` (playwright.config.ts, global-setup.ts, helpers/business.ts 1.024 baris, 27 spec/8.101 baris) tetapi disederhanakan:
- DB: `mini_erp_e2e` (terpisah dari `mini_erp_test` agar tak bentrok TRUNCATE integrasi; guard nama `^[a-z0-9_]+_(e2e|test)$`). Pertanyaan #2.
- Server: BUKAN dev server Vite; bangun web (`npm run build:web`) dan jalankan binari Go dengan `PUBLIC_DIR=apps/web/dist` pada port 18080 (identik dengan produksi satu-kontainer). `webServer` Playwright menjalankan `go run ./cmd/server` dengan env uji.
- Data: `go run ./cmd/seed -profile=e2e` deterministik (kode tetap): cabang HQ + cabang kedua, 4 user/peran (admin, kasir, gudang, akuntan; password tetap uji), 2 lokasi per cabang, 20 produk (+varian+barcode), pelanggan "Tunai" + 5 pelanggan, 3 pemasok, stok awal, chart akun. Tanggal relatif hari ini (tanpa jam beku).
- Setup via REST (helper `api.ts`, token per peran, `storageState` per peran) agar cepat; UI dipakai pada interaksi kritis saja.
- Satu worker, `retries: 1` di CI, trace `on-first-retry`, laporan HTML diunggah.
Spesifikasi (10, dipetakan dari legacy):
E01 auth+akses (login gagal/sukses, refresh bertahan setelah reload, logout, kasir tak melihat menu finance, tabel hak akses) <- legacy 00, 18-role-access, 10.
E02 POS tunai (scan barcode -> keranjang -> bayar tunai -> struk; stok berkurang; klik Bayar dua kali -> satu order) dan POS bayar-nanti (wajib HP/pelanggan, piutang FIFO) <- legacy 07, 15-real-user, 17, 20.
E03 SO -> SJ -> bayar -> posting -> laporan (tempo; SJ parsial lalu penuh; bayar sebagian+lunas; antrean posting; laba rugi/neraca cocok) <- legacy 06, 09, 13, 03.
E04 PO -> GR -> bayar (parsial, over-receipt ditolak, prabayar, bayar supplier) <- legacy 05.
E05 retur penjualan/pembelian parsial + tukar barang pengganti <- legacy 08, 17, 18-exchange.
E06 stok: penyesuaian dengan persetujuan, transfer antar lokasi/cabang (alur kertas), impor stok awal xlsx <- legacy 08, 12.
E07 impor produk xlsx + CRUD pelanggan (kode otomatis, edit tak menggandakan, arsip berpiutang) <- legacy 04, 20-22.
E08 cetak (SJ, nota, struk POS, label QR, surat transfer: render + angka benar; bukan verifikasi fisik) <- legacy 14, 17.
E09 tutup buku (readiness -> safe-close -> reopen, tutup periode pajak, paket pajak terunduh) <- legacy 15-tutup-buku, 09, 13.
E10 menu & rute smoke dibangkitkan dari `registry.tsx` (setiap path menu termuat tanpa error runtime) <- legacy 01.
Tidak diporting: legacy 11 (volume 50 PO/100 SO; jadi benchmark F8-4), 12 (screenshot), 19 (WhatsApp UI; opsional E11 hanya UI), zz-manual-finance (manual).
Cetak fisik tetap manual (daftar centang di F8-6).
Dependensi: F8-1, POS `client_ref`, UI stabil. Waktu CI target < 12 menit. Risiko: flaky timing pada pemilih (gunakan role/label, bukan CSS); selektor bergantung teks Indonesia (kunci di satu berkas `labels.ts`).

### F8-6 Target cakupan dan definisi "cukup untuk cutover" — ukuran S (kebijakan) + ongoing

Target realistis (diukur dengan `-coverpkg`, unit+integrasi):
- `application` modul uang (finance, payment, stock, delivery, sales, purchasing, goodsreceipt, salesreturn, purchasereturn): >= 65% statement; auth/application >= 70%; modul lain >= 40%. Cakupan global hanya informasi. Ratchet: CI gagal bila cakupan turun dari `coverage-baseline.json`.
- Frontend: `modules/*/lib`, `shared/services`, schema Zod >= 60% (Vitest); halaman dicakup E2E, bukan unit.
- Rute: 100% rute lolos F8-2; kontrak FE 100% (F8-3).
"Cukup untuk cutover" (semua harus terpenuhi): (1) seluruh T-xx berlabel WAJIB hijau 3 kali beruntun di CI; (2) E01-E10 hijau 3 kali beruntun tanpa retry; (3) F8-2 + F8-3 hijau; (4) `go vet`, golangci-lint 0 temuan, arch-check 0 pelanggaran; (5) rekonsiliasi ETL (agen ETL) memakai `testkit` dan lolos; (6) daftar centang cetak fisik (SJ, nota, struk termal, label QR) ditandatangani manual; (7) skenario pemulihan (kill -9 di tengah checkout, restart) lolos T-43.

### F8-7 Penyatuan rumus pajak/diskon/prorata (satu sumber) — ukuran M

Masalah (TERBUKTI): 5 salinan; komentar "duplikasi disengaja" membalik logika: yang menjamin finance == dokumen adalah SATU implementasi, bukan lima yang dijaga tangan. Finance menghitung ulang dari item PO/SO (`builders.go:172,248`), jadi satu selisih pembulatan = jurnal tak sama dengan dokumen.
Evaluasi tempat:
- Opsi A (DIREKOMENDASIKAN): `internal/shared/money` ditambah `Round(f float64) int64`, `Prorate(amount int64, part, whole float64) int64`, `type Line struct{Net, Base, Tax, Total int64}`, `LineAmounts(qty float64, unit int64, discountPct float64, discountNominal int64, taxType string, taxRate float64) Line`. Pembenaran: fungsi murni numerik (tanpa entitas, state, I/O), kebijakan pembulatan sudah ditetapkan ADR-0005 (uang = integer rupiah). Ini pengecualian tertulis terhadap "shared/ hanya utilitas teknis" dan butuh ADR-0010 + persetujuan pemilik (Pertanyaan #1). Paket `money` saat ini mati (nol pengimpor), jadi tidak ada beban lama.
- Opsi B: sales/contracts mengekspos fungsi + klien — ditolak: purchasing dan finance bergantung pada sales untuk matematika pembelian = arah dependensi salah.
- Opsi C: simpan `tax_base`/`tax_amount` per baris saat dokumen dibuat (baseline schema sedang di-squash, jadi murah) sehingga finance & retur membaca angka tersimpan, bukan menghitung ulang. Menghapus 3 salinan tetapi tetap menyisakan 2 `CalcLine` (sales, purchasing) + 3 `prorate`, dan mengubah kontrak dokumen + ETL. Opsi lanjutan opsional, bukan prasyarat.
Fallback bila pemilik menolak A: pindahkan hanya `Round`+`Prorate` ke shared (netral) dan pertahankan 2 `CalcLine` + 1 test silang yang mengimpor keduanya dari paket integrasi.
Rancangan A (urut aman):
1. Tulis `money/legacy_ref_test.go` berisi SALINAN LAMA kelima fungsi sebagai oracle (`refSales`, `refPurchasing`, `refFinance`, `refSalesReturn`, `refPurchaseReturn`).
2. Test properti (tanpa dependensi baru; `math/rand` seed tetap + `testing/quick`): 1.000.000 kasus acak: qty `0.0001..10000` (termasuk pecahan .5 dan 12 desimal), unit `0..5e9`, pct `0..100` (0,5; 33,333; 100), nominal `0..gross`, taxType {none, exclude, include}, rate {0, 11, 12, 0,5} + kasus tepi pembulatan x.5 negatif/positif. Asersi: (a) `LineAmounts` == kelima oracle untuk Net/Base/Tax(/Total); (b) invarian: none: tax=0,total=net; exclude: base=net,total=net+tax; include: base+tax=net,total=net; (c) `Prorate(x,part,whole)` == oracle dan `whole<=0 -> 0`; (d) untuk part==whole, Prorate == amount; (e) `Prorate` monoton terhadap part.
3. Ganti pemakai: sales/application/service.go:219 dan purchasing/application/service.go:189 memanggil `money.LineAmounts`; hapus `*/domain/pricing.go` (dan pindahkan tes `pricing_test.go` ke money); finance/builders.go, salesreturn, purchasereturn memanggil `money.LineAmounts(...)`+`money.Prorate`; hapus `round()` posting.go:60 bila tak terpakai lagi. Tes lama (`builders_test.go`, `returns_math_test.go`) diarahkan ke money.
4. Jalankan seluruh test integrasi uang (`money_paths`, `returns`, `exchange`, `purchase_returns`, `turnover`) — angka tidak boleh berubah satu rupiah.
5. arch-check Rule P: dilarang mendefinisikan fungsi bernama `lineEconomics|CalcLine|prorate` di luar `shared/money`.
Cek volume/performa: tidak relevan (aritmetika murni). Dependensi: F8-4 (T-30/T-21 minimal) sebelum langkah 3; sebelum perubahan finance oleh agen lain.

### F8-8 Helper kecil bersama — ukuran M

Dua kelompok, urutan berbeda agar tak ada churn ganda dengan sqlc:
A. Presentation (kerjakan sekarang, tak terpengaruh sqlc): tambahkan ke `auth/contracts` (sudah diimpor semua presentation; ia sudah memuat middleware, jadi sah): `ActorID(r)`, `BranchID(r)`, `type Permission func(perm string, next func(http.ResponseWriter,*http.Request)) http.Handler`, `Guard(next, branchGuard)`. Hapus 15 `actorID`, 13 `branchID`, 13 `guarded`, 18 `type Permission` (nilai `authMod.Permission` tetap bisa ditetapkan karena tipe dasar sama). Sekaligus `assistant/presentation/handler.go:30 branchID` yang mati. Auth user/presentation/actor.go ikut.
B. Infrastructure (dikerjakan sebagai langkah 0 F8-18, bukan sekarang): `internal/shared/dbutil` dengan `NullString(s) sql.NullString`, `NullInt64(v int64) sql.NullInt64`, `NullIfZero`, `Bool(b) int`; `toFloat`/`parseDecimal` hilang karena override `decimal -> float64` di sqlc (terverifikasi: scan []byte -> float64 identik dengan ParseFloat(TrimSpace)). Menunda menghindari menulis ulang 15 salinan dua kali.
C. Teknis lain: `timeutil.TodayJakartaISO()` (ganti `resolveDate` x2), `timeutil.FormatNullTime` (x2); `shared/xlsx` dengan `OpenWorkbook(io.Reader)` (gabung `openWorkbook` product/import.go:173 dan `openOpeningBook` stock/opening.go:74, badan identik) dan `TruncateCell(s, max)` (purchasing/export.go:151, sales/export.go:152); hapus `itoa/itoa64` mati.
Test: pindahkan test yang ada; tambah tabel uji kecil per helper. Ukuran fungsi 5-20 baris; efek bersih ~ -350 baris.

### F8-9 Envelope, parse limit, dokumen paginasi — ukuran S

1. `shared/response`: tambah `Unauthorized(w)`, `Forbidden(w)`, `ForbiddenMsg(w,msg)`, `Internal(w)` yang memanggil `Fail` dengan `apperror`; `auth/contracts/session.go` (unauthorized/forbidden/serverError/deny) dan `server/middleware.go:28 recoverer` memakainya (`shared/response` hanya mengimpor `shared/apperror`, tak ada siklus). Test: golden byte-demi-byte ke-4 pesan lama tetap sama (`{"success":false,"message":"..."}`; catatan: `Errors` kosong di-omit).
2. `pagination.ParseDisplayLimit(raw string, def, max int) int` untuk dashboard `limitParam` (handler.go:41, hari ini tanpa batas atas) dan finance/presentation/reports.go:208; batas maksimum eksplisit.
3. `.claude/rules/api-standard.md`: `total_pages` -> `totalPages`.
4. Arch-check Rule M: literal `"success":` hanya boleh di `shared/response`.
5. 404 JSON untuk `/api/` (lihat F8-2.4).
Dependensi: tak ada. Risiko: nol perilaku (diuji golden).

### F8-10 Modul kembar — keputusan: TIDAK membuat mesin dokumen generik — ukuran S (keputusan+ADR)

Fakta terukur: duplikasi mekanis identik hanya 466 baris di seluruh kode; twin sales/purchasing ~3,4k baris, salesreturn/purchasereturn ~3,3k baris. Mesin generik (Go generics atau antarmuka) hanya akan menghapus sekitar 1.0-1.5k baris (~4%) tetapi menyatukan alur uang yang aturannya berbeda (kredit & approval, ship-to, member pricing, faktur pajak vs faktur pemasok, tukar barang, penyelesaian retur) — risiko regresi pada jalur uang tinggi, manfaat kecil, keterbacaan turun. ADR-0014: "Modul kembar tetap terpisah; kemiripan dijaga lewat test paritas, bukan mesin bersama."
Aman yang dikerjakan: (1) F8-7 (rumus) dan F8-8C (utilitas teknis); (2) `totals(lines)` (10 baris) dan `StatusCounts`/`OrderParties`/`ListByParty`/`SetStatus` (63 baris) tetap salinan, dengan komentar saling-rujuk dan satu test paritas di integrasi yang menjalankan skenario yang sama atas SO dan PO; (3) repository kembar akan memakai pola sqlc yang sama (F8-18) sehingga kemiripannya terlihat dalam query, bukan kode Go.
Nilai: menghindari ~1 minggu pekerjaan berisiko; dicatat agar tidak diusulkan ulang.

### F8-11 Tumpang tindih reporting vs finance inventory-value — ukuran S

Terbukti: `GET /reporting/inventory` (reporting/presentation/handler.go:59) dan `GET /finance/reports/inventory-value` (finance/presentation/handler.go:553) sama-sama `FinanceClient.InventoryValue` (reporting/application/service.go:`Inventory`); modul reporting = 192 baris, 2 rute, satu-satunya nilai unik = validasi rentang `SalesTrend`. FE: `modules/reporting/types.ts InventoryPosition` menggandakan `modules/finance/types.ts`; `ReportingPage` dan `ReportsPage` merender tabel yang sama; `sales-trend` dikonsumsi `DashboardPage.tsx:95` dan `ReportingPage.tsx:46`. Izin berbeda (`reporting.view` vs `finance.view`) — hanya `reporting/contracts/client.go:32` yang mendefinisikan `reporting.view`.
Rancangan (default minimal, tanpa menghapus modul): hapus tipe ganda FE (impor `InventoryPosition` dari satu tempat), jadikan `reporting.Inventory` alias dokumentasi eksplisit; JANGAN hapus rute (dua izin berbeda = dua audiens sah). Opsi lebih besar (Pertanyaan #4): lebur modul reporting ke finance/dashboard (-192 baris, -1 modul, -1 izin) — butuh keputusan UX menu "Tren & Stok" dan seed izin; koordinasikan dengan F2b-13 dan F3a-10.
Test: kontrak FE (F8-3) memastikan tak ada rute yatim. Ukuran S (default).

### F8-12 Kode mati dan endpoint tak terpakai — ukuran S

1. Hapus 8 item `unused` + temuan lint lain (errcheck test, ineffassign seed, QF1003/QF1008); aktifkan `.golangci.yml` di CI (F8-1).
2. Hapus accessor tak terpakai `SalesOps/PurchasingOps/Dashboard/Reporting/Assistant/Checker`; main.go meneruskan `salesMod.Service()` ke klien ops — lebih baik meneruskan `SalesOps()` agar antarmuka sempit yang dipakai (ubah pemanggil, bukan hapus `SalesOps`/`PurchasingOps`; hapus yang lain).
3. Endpoint: PERTAHANKAN `stock/reservations/{hold,release}` (POS), `auth/switch-role` (UI), `stock/allocations/suggest` (dipakai FE POS legacy). `finance/costing/{sync,uncosted}`: tidak dipakai FE legacy maupun revamp; default = pertahankan `uncosted` (kartu "biaya estimasi" di panel kesiapan tutup buku, tugas FE) dan `sync` (tombol admin); bila tak ada tugas FE yang mengambil, hapus keduanya. Semuanya masuk `contract-allow.json` (F8-3) dengan alasan sampai dipakai.
4. `packages/shared` & `packages/api-contract`: dibiarkan (dibuat skill monorepo; biaya nol) — dicatat tidak aktif.
Dependensi: tak ada. Test: `go vet`, lint bersih.

### F8-13 Wiring: hapus setter, tambah verifikasi — ukuran M

Rancangan (menggantikan "dokumentasikan sebagai ADR" karena setter ternyata bisa dihilangkan):
1. `finance.SetOps` -> parameter konstruktor `finance.NewModule(..., salesOps, purchOps)`; hapus `errOpsUnwired` dan nil-guard (finance/application/service.go:86-172). Tak ada siklus.
2. `payment`<->`salesreturn`/`purchasereturn`: kontrak sempit sisi konsumen `paymentcontracts.AllocationChecker { HasActiveAllocations(ctx, orderType, orderID) (bool, error) }`. Dibangun dari repository payment tanpa dependensi retur: `payment.NewAllocationChecker(db)` (membungkus `repo.PaidForOrder`, payment/application/service.go:817). Urutan main/bootstrap: allocChecker -> salesreturn/purchasereturn (parameter konstruktor) -> payment (butuh klien retur). Hapus `SetPayments` x2 dan `errPaymentsNotWired`.
3. Invalidator authz: `authcontracts.NewAuthzHub()` (fan-out sederhana `Subscribe/InvalidateAll/InvalidateUser`, dibuat PERTAMA), diberikan ke `user.NewModule`, `branch.NewModule`; `auth.NewModule(..., hub)` mendaftarkan service-nya sebagai pelanggan. Hapus `SetAuthzInvalidator` x2 modul/x2 service. Perilaku "nil invalidator = no-op" tetap.
4. Hasil: nol metode `Set*` pada `Module`; Rule arch-check: dilarang metode `Set[A-Z]*` pada tipe `Module` di berkas `*.module.go` (kecuali daftar kecil berisi alasan).
5. Test: `TestBuildWiresEverything` (F8-1) + T-05 (izin dicabut berlaku di request berikutnya, membuktikan hub tersambung) + test unit hub (subscribe/fan-out/nil).
Dependensi: F8-1 (bootstrap). Ukuran M (7 berkas modul + main + seed + test).

### F8-14 Pola transaksi dan ADR jujur (transaction script) — ukuran M

Penilaian ("ide paling sederhana yang memenuhi tujuan"): TIDAK merombak ke domain-driven penuh. Fakta: domain 203 baris, application 16.259 baris = gaya transaction-script/use-case, dan itu sesuai untuk ERP single-tenant satu tim. contracts menjadi struct sekaligus API publik dan model baris (21/28 berkas) — diterima karena skema dimiliki modul yang sama dan tak ada konsumen lain; dipisah hanya bila bentuk DB dan API menyimpang.
ADR-0011 menetapkan: (1) application = use case/transaction script; aturan murni yang dipakai ulang atau rawan salah (harga, UOM, izin, prorata) wajib masuk `domain/` atau `shared/money` sebagai fungsi murni ber-test tabel; (2) tx dimulai di lapisan yang mengorkestrasi beberapa langkah DALAM satu modul; repository bertransaksi sendiri hanya untuk operasi agregat tunggal (header+baris); (3) application tidak mengimpor `database/sql`.
Perapian terbatas (blast radius kecil): stock mengekspos `repo.InTx(ctx, func(tx *infrastructure.Tx) error)` dengan tipe `Tx` buram yang membawa metode `EnsureBalance/AddBalance/InsertMovement/...` (sekarang suffix `Tx` menerima `*sql.Tx`); 12 titik `Begin` jadi `InTx` (menghapus boilerplate Rollback/Commit dan impor `database/sql` dari stock/application). `auth.toNullInt` dan `media` `sql.ErrNoRows` dipindah ke infrastructure (`repo.X` mengembalikan `nil,nil`/sentinel). Digabung dengan migrasi sqlc stock (F8-18) karena bentuk `WithTx` sqlc sama. Tak ada tipe port/interface baru untuk repository (application tetap memakai `*infrastructure.Repository` konkret): menambah interface untuk 20 repository tanpa konsumen kedua = beban tanpa manfaat; interface dibuat di sisi konsumen hanya saat ada kebutuhan test-double (mis. gateway WhatsApp, sudah ada).
Test: seluruh integrasi stok (`stock_flows`, `movelocation`, `transfer_scope`) dan T-10..T-15 sebagai oracle.

### F8-15 arch-check diperkuat + cara menguji arch-check — ukuran M

Pemeriksaan tambahan (semua murni teks/graf, Node, tanpa dependensi):
- Rule A': impor akar modul (`.../modules/<m>` tanpa `/`) dilarang kecuali dari `cmd/**`, `internal/bootstrap/**`, `internal/integration/**`; impor `contracts` modul lain tetap boleh.
- Rule E (arah layer, per modul): `domain` hanya boleh mengimpor stdlib + domain sendiri + `shared/money|timeutil`; `infrastructure` tak boleh mengimpor `application`/`presentation`; `application` tak boleh mengimpor `presentation` atau `database/sql` (daftar kecualian sementara untuk 3 berkas di F8-14); `presentation` tak boleh mengimpor `infrastructure`; `shared/**` tak boleh mengimpor `modules/**` (hari ini 0).
- Rule T (kepemilikan tabel): peta `scripts/table-owners.json` (tabel -> modul) WAJIB cocok 1:1 dengan semua `CREATE TABLE` di migrasi (tabel baru tanpa pemilik = gagal). Parser: dari literal string Go di `*/infrastructure/**` dan `queries/*.sql`, ambil nama tabel setelah `FROM|JOIN|INTO|UPDATE|TABLE|DELETE FROM`; tabel milik modul lain = gagal. (`tblacc.mjs` sudah membuktikan metodenya: 0 pelanggaran saat ini.) Untuk SQL yang dirakit dari penggabungan string, aturan melarang literal tabel milik modul lain di berkas mana pun dalam modul.
- Rule F (FK): `REFERENCES` di migrasi hanya ke tabel dengan pemilik sama (hari ini 32 FK, 0 lintas).
- Rule S (siklus): graf modul dari impor `contracts` dengan mengabaikan auth, user, audit sebagai lintas-sektor; harus DAG (setelah F8-13 tanpa pengecualian; sebelumnya allowlist `payment<->salesreturn`, `payment<->purchasereturn`).
- Rule K (kebocoran contracts): `contracts/` tak boleh mengimpor `application|infrastructure|presentation` modul mana pun; `contracts` modul A hanya mengimpor `contracts` modul lain + `shared`.
- Rule R (SQL mentah, aktif setelah F8-18): `.QueryContext(|.ExecContext(|.QueryRowContext(|.PrepareContext(` dilarang di berkas Go selain `*/infrastructure/sqlc/*` dan berkas pada `scripts/raw-sql-allow.json` (jalur+alasan+tautan ADR).
- Rule C' (sqlc nyata): tiap blok `sqlc.yaml` punya `queries/*.sql` >= 1 dan `sqlc/db.go` hasil generate ada; `sqlc.yaml` harus sama dengan hasil `scripts/sqlc-sync.mjs` (dibangkitkan dari daftar modul + berkas schema per modul); CI menjalankan `sqlc generate && git diff --exit-code apps/api` (generate tak ketinggalan). Blok `dashboard`/`reporting` dihapus karena tak ber-SQL.
- Rule M: literal `"success":` hanya di `shared/response`. Rule P: tak ada `lineEconomics|CalcLine|prorate` di luar `shared/money`. Rule W: tak ada metode `Set*` pada `Module` (F8-13).
Menguji arch-check: refaktor skrip menjadi `scripts/arch-check/lib.mjs` (fungsi `checkRepo(rootDir) -> Violation[]`) + `scripts/arch-check.mjs` (CLI). `scripts/arch-check/__fixtures__/<rule>/{pass,fail}/` = pohon mini yang dimulai dari `apps/api/internal/modules/...` untuk tiap rule; `node --test scripts/arch-check/` mengecek: pohon `pass` -> 0 pelanggaran, tiap `fail` -> persis pelanggaran dengan kode rule yang diharapkan (golden JSON). Satu test integrasi: jalankan terhadap repo asli = 0 pelanggaran. Wajib di CI.
Dependensi: peta pemilik tabel bergantung pada baseline per modul (F8-18 §A); sebelum squash dibangun dari nama berkas migrasi + koreksi manual (tblown.mjs).

### F8-16 Koreksi dokumen dan ADR baru — ukuran S

Koreksi:
- `.claude/rules/api-standard.md`: `total_pages` -> `totalPages`.
- `docs/PARITY_AUDIT.md`: §5.4 hapus "sqlc 19/20" (ganti: "sqlc terkonfigurasi, belum dipakai; migrasi = fase terakhir"); §5.1 angka test (35 berkas, 96 test + TestMain, 39 integrasi, 0 E2E, 0 CI); baris ~712 "C0 git ditunda" sudah usang (`.git` ada); jumlah endpoint (217 rute revamp); klaim "tanpa join lintas modul" tetap benar dan kini terukur (F8-15).
- `CLAUDE.md`: "~30k lines" -> ~35k; "26 migrations" -> 27 berkas up (celah 26-29, selesai saat squash); tambahkan status "sqlc: terkonfigurasi, belum dimigrasi".
- `knowledge/BACKEND_GUIDE.md` §Migrasi & sqlc (l.58 "CLI belum terpasang" usang); `knowledge/ARCHITECTURE_OVERVIEW.md:16` tandai sqlc = target; `docs/SYSTEM_DESIGN.md:33` idem.
- `docs/DEPLOYMENT.md`: "Go 1.21+" -> `go.mod` `go 1.26.0` (toolchain 1.26.7), MySQL uji 8.4.3, pin versi sqlc (1.31.1), golangci-lint (2.14.0).
- `knowledge/PERFORMANCE_GUIDE.md:43`: "konstan terhadap besar buku" -> "jumlah query konstan; biaya tiap query tetap tumbuh dengan jumlah baris jurnal dalam rentang" (GROUP BY atas `finance_journal_lines`).
- `docs/API_CONTRACT.md`: sinkronkan setelah F8-11/F8-12.
ADR baru (nomor final dikoordinasikan dengan agen lain): 0010 shared/money sebagai kernel aritmetika (F8-7); 0011 application = transaction script + contracts sebagai model data (F8-14); 0012 adopsi sqlc + kebijakan SQL tulisan tangan (F8-18); 0013 tanpa mesin dokumen generik (F8-10); 0014 strategi test (integrasi di mini_erp_test, E2E di mini_erp_e2e, definisi cukup-untuk-cutover); 0015 (bila disetujui) atomisitas = idempotensi + CAS, pensiun doclock (agen atomisitas yang menulis).

### F8-17 Graceful shutdown dan siklus hidup goroutine — ukuran S (koordinasi dengan agen deploy/ops)

Terbukti (cmd/server/main.go): `go func(){ boot, cancel := context.WithTimeout(context.Background(), 60*time.Second); assistantMod.Boot(boot) }()` tidak terikat sinyal; shutdown hanya `httpSrv.Shutdown(10s)`; gateway WhatsApp (`assistant/infrastructure/gateway.go`, store sqlite) tak pernah `Disconnect`/ditutup; `ReadHeaderTimeout`/`IdleTimeout` tak diset.
Rancangan: `ctx` sinyal dibuat SEBELUM Boot; Boot memakai `context.WithTimeout(ctx, 60s)`; tambah `assistant.Module.Shutdown(ctx)` (Disconnect + tutup store) dipanggil setelah `httpSrv.Shutdown`; set `ReadHeaderTimeout: 10s`, `IdleTimeout: 60s`; Docker compose `stop_grace_period: 30s` dan Shutdown timeout 25s. Test: integrasi kecil "Shutdown menyelesaikan request berjalan lalu menutup gateway palsu". Dimasukkan ke `bootstrap.App.Close()`.

### F8-18 Migrasi sqlc bertahap — PALING AKHIR — ukuran L (total ~55-65 hari-kerja bila F8-4 belum ada; ~35-40 bila sudah)

#### A. Konfigurasi dan hasil percobaan (sqlc v1.31.1, folder `scratchpad/sqlctrial`)
Terverifikasi dengan percobaan nyata terhadap migrasi repo:
- `schema:` boleh daftar berkas; hanya `*.up.sql` yang dibaca (down diabaikan). 59 tabel diparse tanpa galat.
- KRITIS: dengan `schema` = SATU berkas modul, query yang menyebut tabel modul lain GAGAL generate (`relation "branches" does not exist`). Maka baseline squash HARUS berupa berkas per modul (`NNNNNN_<modul>.up.sql`, 18 modul ber-tabel, sesuai aturan BACKEND_GUIDE "satu migrasi = satu modul") dan `sqlc.yaml` per modul hanya menunjuk berkas modul itu: aturan "tak ada join/FK lintas modul" ditegakkan kompilator, bukan hanya skrip. Migrasi susulan menambah berkas `NNNNNN_<modul>_<perubahan>.up.sql`; `scripts/sqlc-sync.mjs` membangkitkan `sqlc.yaml` (daftar berkas per modul dari konvensi nama).
- Tipe: `BIGINT UNSIGNED` -> `uint64` secara bawaan; override `db_type: bigint, unsigned: true -> int64` (+`nullable: true -> database/sql.NullInt64`) terbukti. `DECIMAL(24,12)/(18,4)` -> `string`; override `decimal -> float64` identik perilaku (hapus `toFloat` x7, `parseDecimal`).
- Lolos analyzer: `sqlc.narg`, `sqlc.slice` (diperluas saat runtime; slice kosong jadi `NULL`), `sqlc.embed`, `INSERT IGNORE`, `ON DUPLICATE KEY UPDATE ... VALUES()`, `FOR UPDATE`, `LAST_INSERT_ID()` (`UPDATE x SET v = LAST_INSERT_ID(v+1)` lalu `SELECT LAST_INSERT_ID()` pada tx yang sama), subquery-di-FROM (kolom ambigu harus dikualifikasi), JOIN+GROUP BY+SUM, `DATEDIFF`, `DATE()`, `CASE`, `COALESCE`, parameter bernama dipakai ulang (otomatis diikat dua kali).
- DITOLAK: `UTC_TIMESTAMP(6)` (54 pemakaian: "function utc_timestamp(unknown) does not exist"; `CAST` tak membantu). Solusi: `sqlc.arg(now)` diisi `timeutil.NowUTC()` dari repository (`r.now()`); satu instans sehingga jam aplikasi = jam DB secara praktis (Pertanyaan #9). Juga `CURDATE()` (4x) -> parameter `today` dari `timeutil` (sekaligus memperbaiki risiko zona waktu sesi MySQL).
- Konfigurasi per modul:
```
- engine: mysql
  schema: [migrations/000008_stock.up.sql]      # dibangkitkan sqlc-sync.mjs
  queries: internal/modules/stock/infrastructure/queries
  gen: { go: { package: sqlc, out: internal/modules/stock/infrastructure/sqlc,
       emit_empty_slices: true,
       overrides: [ {db_type: bigint, unsigned: true, go_type: int64},
                    {db_type: bigint, unsigned: true, nullable: true, go_type: database/sql.NullInt64},
                    {db_type: decimal, go_type: float64}, {db_type: decimal, nullable: true, go_type: database/sql.NullFloat64} ] } }
```
Tanpa `emit_json_tags` (JSON tetap milik contracts). `emit_interface` mati (Repository konkret).
- Performa pola filter opsional (diuji pada MySQL 8.4.3, `stock_movements` 400.000 baris, EXPLAIN dengan prepared statement): `(? IS NULL OR col = ?)` menghasilkan rencana yang SAMA dengan SQL dinamis buatan tangan (`ref ix_lookup` bila produk diisi; `Backward index scan` pada PRIMARY bila tak ada filter). MySQL mengoptimalkan ulang per eksekusi dengan nilai parameter sebagai konstanta, jadi `sqlc.narg` aman untuk list+count (yang hari ini menulis kondisi dua kali: sales/repository.go:213 vs :248, purchasing, party, product, user, branch, audit, stock). Pengecualian: `LIKE '%q%'` tetap tak memakai indeks seperti sekarang (bukan regresi).

#### B. Inventaris query per modul (heuristik `inv.mjs`; panggilan = QueryContext/ExecContext/QueryRowContext) dan jenisnya
Jenis: S=statis CRUD; N=filter opsional dinamis (list+count; -> `sqlc.narg`); L=IN-list (-> `sqlc.slice`, chunking tetap di Go); U=UPSERT/INSERT IGNORE; K=FOR UPDATE/CAS; T=tx berurutan banyak langkah; A=agregat GROUP BY/JOIN; Z=subquery/fungsi khusus.
| Modul | Panggilan | Jenis dominan | Catatan | Ukuran |
|---|---|---|---|---|
| media | 6 | S, T(1) | tx kecil (media_files) | S |
| company | 5 | S, U(1), T(1) | upsert pengaturan di loop | S |
| audit | 3 | N(2), S | list+count 3 filter opsional (branch OR NULL, action, entity) | S |
| auth | 8 | S | `user_sessions`; CAS rotasi (F8-4 T-04) ikut dibenahi | S |
| branch | 13 | S, N(2), L(1), U(1), T(2) | `branch_document_sequences` INSERT IGNORE+LAST_INSERT_ID | M |
| goodsreceipt | 7 | S, T(1), A(1) | | S/M |
| purchasereturn | 10 | S, T(1) | | M |
| payment | 10 | S, L(2), A | args-2 pada `repository.go:242` perlu diperiksa | M |
| party | 22 | S, N(3), L(2), Z(1), A | subquery PromoteOldestActive | M |
| purchasing | 15 | S, N(2), L(2), T(2), A | filter status IN, `number LIKE` | M |
| salesreturn | 15 | S, L(1), T(1) | | M |
| sales | 20 | S, N(3), L(4), T(2), A(2) | aggregates.go 7 query berkelompok | M |
| delivery | 13 | S, N(1), L(3), A(2), Z | `DATEDIFF` bucket umur; tx konfirmasi | M |
| assistant | 24 | S, U(2), A | belum punya blok sqlc; gateway sqlite whatsmeow TETAP di luar sqlc | M |
| product | 27 | S, N(2), L(3), T(2) | 4 JOIN, list+count | L |
| user | 32 | S, N(2), L(1), U(1), T(4) | RBAC; invalidator hub | L |
| stock | 41 | S, N(5), K(2), U(1), T(10 primitif + 12 titik tx) | `InTx` (F8-14), reservasi tanpa gap lock (komentar repository.go:300 WAJIB dipertahankan) | L |
| finance | 48 | S, U(4), L(3, chunk), A(10), J(13) | `commercialOnly` jadi boolean `sqlc.arg`; rute laporan terbesar; bagi 3 langkah (chart/period, journals/ledger, turnover/daily/reports) | L |
Total sekitar 25 berkas, 319 panggilan.

#### C. Pola per jenis
- S: satu file `queries/<entitas>.sql`, nama `Get<X>`, `List<X>`, `Create<X>`...; SELECT dengan kolom EKSPLISIT (bukan `*`) atau `sqlc.embed(tabel)`.
- N (filter opsional): `WHERE (sqlc.narg('status') IS NULL OR status = sqlc.narg('status'))`; list dan count berbagi kondisi lewat dua query berdampingan dalam berkas yang sama (kondisi identik; test paritas list-vs-count). Wrapper Repository mengubah `""`/`0` jadi `sql.Null*{}` lewat `dbutil`.
- L (IN-list): `sqlc.slice('ids')`; chunking (`footingChunk`) tetap loop di Go memanggil metode hasil generate; slice kosong ditangani wrapper (jangan jalankan query) agar tak menghasilkan `IN (NULL)`.
- U: `INSERT ... ON DUPLICATE KEY UPDATE` / `INSERT IGNORE` ditulis apa adanya.
- K: `... FOR UPDATE` apa adanya, hanya lewat `WithTx`.
- T: Repository menyediakan `InTx(ctx, fn func(*TxRepo) error)`; `TxRepo` membungkus `*sqlc.Queries` hasil `q.WithTx(tx)`; urutan "baca & tutup rows sebelum query lain di tx yang sama" (aturan database.md) dipertahankan: metode `:many` sqlc sudah menutup rows sebelum return.
- Tipe baris -> contracts: berkas `infrastructure/mapper.go` per modul berisi `func toX(m sqlc.X) *contracts.X` dan konversi null (`sql.NullString -> string`, `NullInt64 -> int64`, waktu -> string `2006-01-02 15:04:05` seperti `scanOrder`). Contracts TIDAK diubah (tetap API publik; tak ada `emit_json_tags`). Pemetaan 1-ke-1 per entitas; proyeksi khusus (`ListXRow`) dipetakan ke tipe contracts yang sudah ada (mis. `OrderSummary`).
- Repository: `type Repository struct { db *sql.DB; q *sqlc.Queries }`, `NewRepository(db)`; seluruh metode publik dan tanda tangannya TIDAK berubah, sehingga application dan test tak tersentuh selama migrasi per modul.
- Waktu: `r.now()` -> argumen `now` (54 titik). Uang tetap `int64`.

#### D. Kapan SQL tulisan tangan sah dipertahankan (kebijakan ADR-0012)
Hanya bila SEMUA berlaku: (1) sqlc tidak dapat mengekspresikan (mis. jumlah kolom/tabel dinamis, ORDER BY dinamis, INSERT multi-baris berarity variabel, fitur MySQL yang ditolak analyzer), (2) tak ada padanan `sqlc.narg/slice/embed`, (3) ada komentar `// raw-sql: <alasan>` dan entri `scripts/raw-sql-allow.json` {berkas, alasan, ADR, tanggal tinjau}, (4) query diuji integrasi. Ekspektasi dari inventaris: 0-3 pengecualian (kandidat: ekspor/laporan finance berukuran sangat dinamis). Rule R (F8-15) menegakkan. Tidak ada query-builder runtime.

#### E. Urutan modul dan cara menjaga perilaku
Prasyarat gerbang: baseline per modul final; F8-4 (T-xx) hijau; F8-15 aktif; `dbutil` (F8-8B) dibuat sebagai langkah 0 bersama `sqlc-sync.mjs`, `Makefile`/npm `sqlc:generate`, CI `sqlc diff`.
Urutan (kecil -> besar, tanpa dinamis dulu): 1 media, 2 company, 3 audit (pertama dengan narg), 4 auth, 5 branch, 6 goodsreceipt, 7 purchasereturn, 8 payment, 9 party, 10 purchasing, 11 salesreturn, 12 sales, 13 delivery, 14 assistant, 15 product, 16 user, 17 stock (+InTx), 18 finance (3 PR).
Per modul (PR tersendiri, bisa di-revert):
1. Test karakterisasi lebih dulu: untuk setiap metode publik Repository tanpa test integrasi, tulis test tabel `repository_test.go` terhadap `mini_erp_test` (fixture testkit) yang memukul setiap cabang filter, slice kosong/besar, null, ordering, paginasi (limit/offset), dan nilai batas.
2. Tulis query + generate + mapper; ganti metode satu per satu; test lama hijau tiap langkah.
3. Pembanding bayangan (sementara, dihapus saat merge): `*_legacy_test.go` menyalin SQL lama sebagai oracle dan membandingkan keluaran lama vs baru pada dataset acak untuk metode N/L/A (tidak untuk CRUD sederhana).
4. EXPLAIN-diff untuk 10-15 query panas (list order, movements, journal lines, aging) memakai skrip `scripts/explain-diff.mjs` terhadap data sintetis 100k-400k baris; rencana tak boleh lebih buruk.
5. Hapus `rows.Scan` mentah, helper lokal, `.gitkeep`; jalankan seluruh `go test ./...` + arch-check.
Ukuran per modul: S = 0,5-1 hari (media, company, audit, auth, goodsreceipt); M = 2-3 hari (branch, purchasereturn, payment, party, purchasing, salesreturn, sales, delivery, assistant); L = 4-6 hari (product, user, stock) dan 8-10 hari (finance). Total sekitar 55-65 hari-kerja orang termasuk test karakterisasi; 35-40 bila T-xx dan repository test sudah ada.
Risiko spesifik: (a) deadlock koneksi tx (aturan database.md) — `TxRepo` tak boleh menyimpan `rows` terbuka; (b) perbedaan pembulatan DECIMAL->float64 vs ParseFloat (diverifikasi identik, tetap diuji pada nilai 12 desimal); (c) `slice` kosong; (d) kesalahan penerjemahan `nullIfEmpty` semantik (string kosong = NULL) — dikunci test; (e) `uint64`->`int64` untuk id sangat besar (tak relevan).

### F8-19 DTO respons bertipe (pengganti `map[string]any`) — ukuran L, OPSIONAL (tidak diprioritaskan)

Nilai: keamanan tipe di 81 titik, dokumentasi bentuk JSON, bukan kebutuhan fungsional. Biaya/risiko: kontrak FE rawan berubah diam-diam (nama field), 6.155 baris presentation. Rekomendasi: JANGAN jadikan fase; terapkan oportunistik saat suatu handler disentuh, dengan golden JSON (F8-2.5) sebagai pagar, DTO diletakkan di `presentation/dto.go` per modul (bukan contracts). Prioritas pertama bila dikerjakan: `sales.orderView`, `purchasing.orderView`, return views, journal views.

---

## 4. Risiko

1. Penyatuan rumus (F8-7) menyentuh jalur uang: pagar = oracle salinan lama + 1 juta kasus + seluruh test integrasi uang; tanpa F8-4 jangan mulai.
2. Migrasi sqlc besar dan paling akhir: perubahan skema/fitur lain setelah baseline menambah query; `sqlc-sync` + CI `sqlc diff` mencegah drift. Baseline BUKAN per modul = enforcement tabel lewat kompilator hilang (jatuh ke Rule T berbasis skrip).
3. Test konkurensi flaky bila memakai timing; gunakan barrier dan asersi invarian. T-04 diperkirakan merah dulu (cacat nyata).
4. Menggeser `UTC_TIMESTAMP(6)` ke jam aplikasi: aman untuk satu instans (doclock sendiri sudah mengasumsikan itu); bila kelak multi-instans, jam harus disinkronkan.
5. E2E bergantung teks UI Indonesia dan data seed; sentralkan label; jaga durasi CI.
6. `Router` interface (F8-1.2) menyentuh 19 `RegisterRoutes`: mekanis tetapi lebar; kerjakan satu PR sebelum agen lain menyunting rute.
7. `internal/bootstrap` memperluas permukaan "composition root"; Rule A' harus eksplisit mengizinkannya.
8. Menghapus modul/rute reporting (opsi besar F8-11) memengaruhi izin tersimpan; seed role harus dimigrasikan.
9. golangci-lint 2.x menghasilkan temuan baru saat CI pertama; siapkan `.golangci.yml` ketat tapi realistis agar tak memblokir.

## 5. Pertanyaan terbuka (HANYA yang butuh keputusan pemilik)

1. ADR-0010: setujukah `internal/shared/money` memuat `LineAmounts` (kernel pajak-diskon murni) + `Round` + `Prorate` sebagai pengecualian tertulis atas "shared/ hanya utilitas teknis"? Default: ya. Bila tidak: fallback F8-7 (hanya Round+Prorate di shared, dua `CalcLine` dipertahankan dengan test silang).
2. E2E memakai database terpisah `mini_erp_e2e` (bukan `mini_erp_test`) agar tak bentrok dengan TRUNCATE integrasi — setuju? Default: ya.
3. Platform CI (rencana mengasumsikan GitHub Actions dengan service MySQL 8.4; repo belum punya CI). Bila CI lain/self-hosted, hanya bentuk berkas yang berubah.
4. Menu "Tren & Stok" (modul reporting): pertahankan apa adanya (default, hanya hapus tipe FE ganda) atau lebur ke Dasbor/Laporan Keuangan (-1 modul, -1 izin)?
5. Endpoint `finance/costing/{sync,uncosted}`: pertahankan dengan UI kecil di panel kesiapan tutup buku (default) atau hapus?
6. Target cakupan F8-6 (65% application modul uang, 70% auth, 40% lain, ratchet) dan definisi "cukup untuk cutover" — disetujui atau diubah angkanya?
7. Perilaku refresh bersamaan (T-04): pasang CAS pada `refresh_hash` dan beri toleransi (grace) 10 detik — permintaan kedua dengan token lama yang baru saja dirotasi mendapat sesi yang sama/token baru tanpa mencabut semua sesi — atau sikap ketat (yang kalah 401)? Default: grace 10 detik. Ini keputusan keamanan-vs-kenyamanan.
8. Baseline schema: wajib BERKAS PER MODUL (usulan, agar sqlc menegakkan batas modul)? Perlu keputusan bersama agen schema sebelum squash.
9. `UTC_TIMESTAMP(6)` DB -> jam aplikasi (`timeutil.NowUTC`) saat migrasi sqlc: diterima (asumsi satu instans)?
