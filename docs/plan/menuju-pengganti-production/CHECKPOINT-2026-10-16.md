# Checkpoint 2026-10-16 — gladi ETL dengan data produksi nyata

> **RIWAYAT — snapshot 2026-10-16.** Catatan gladi ETL pada tanggal itu; perubahan kode yang disebut "belum di-commit" sudah masuk `main`. Prosedur yang berlaku: `docs/RUNBOOK.md` §5b/§5c; status terkini: [docs/STATUS.md](../../STATUS.md).

Tujuan dokumen: agar AI/pengembang berikutnya bisa melanjutkan **dengan urutan yang benar**
tanpa menemukan ulang jebakan yang sama. Baca bersama `docs/RUNBOOK.md` §5b dan
`docs/PARITY_AUDIT.md`. Semua perubahan kode di bawah **belum di-commit** (working tree).

## 1. Status ringkas

| Hal | Status |
|---|---|
| Kode aplikasi | Fitur lengkap; `npm run verify` **HIJAU penuh** (go build/vet/test+integrasi, tsc, eslint, vitest, arch-check, sqlc sync, contract-check) pada kode terbaru |
| Gladi L1 pada dump produksi nyata + media | **LULUS** (`rehearse pass=true`, 86 pemeriksaan verify, 3.625 media) — diulang setelah perbaikan terakhir, lulus lagi |
| Self-test verifier | **LULUS** (10 kasus) |
| Idempotensi (`load --resume`) | `inserted=0` |
| Determinisme keluaran (L2 `--compare-with`) | **BELUM terbukti**: L2 pertama menemukan `id_map.jsonl` berbeda urutan (sudah diperbaiki di `transform/stock.go`, `mergedInto` diurutkan); L2 ulang **terputus** sebelum selesai |
| E2E Playwright | **BELUM**: 24 lulus (API), 13 gagal hanya karena **Chromium Playwright belum terpasang**, 2 dilewati; belum ada kegagalan aplikasi yang terbukti |
| UAT, cetak fisik, simulasi rollback, gladi L2 | Belum |
| Keputusan pemilik/akuntan | Belum (tanggal saldo awal, perlakuan jurnal inklusif pajak, selisih Rp1) |
| Deploy produksi + cutover nyata | **Belum boleh** sebelum daftar §4 selesai |

## 2. Lingkungan lokal (kondisi saat checkpoint)

- Drive **E:** (USB ADATA) tidak stabil: sempat putus berkali-kali, Windows menandai `Full Repair Needed`.
  Jangan simpan hal kritis hanya di sana. Backup repo+dump: `C:\Users\Neuron-NB167\backup-mini-erp-20261016\`.
  **C: hampir penuh (~1,3 GB bebas)** — jangan pasang apa pun yang besar di C: maupun di E:
  tanpa membereskan dulu (pemilik meminta tidak memasang Chromium di E:).
- MySQL Laragon (root tanpa sandi, dipakai bersama project lain): `mini_erp_legacy_copy`,
  `mini_erp_rehearsal` (hasil gladi), `mini_erp_test`. DB lokal `mini_erp` **jangan disentuh**.
- Go: bangun dengan `GOCACHE=e:/dimasprasetio/.gocache GOTMPDIR/TMP/TEMP=e:/dimasprasetio/.gotmp`.
  Binari ETL dijalankan dari C: (`C:\Users\Neuron-NB167\etl-bin\etl.exe`); skrip: `scripts/ops/etl-rehearsal.sh`.
- Dump produksi: `E:\dimasprasetio\SAAS\mini-erp-backups\mini_erp_for_revamp_etl_20261016_101403.sql.gz`
  (sha256 `c32876b2…`, 71 tabel, company_id=1). **Data pelanggan nyata — jangan di-commit.**
  Media legacy: `E:\dimasprasetio\legacy-media\media-export` (3.625 file, 285 MB, sha256 arsip `0bb692bc…`).
- Server lama: SSH alias `mini-erp`; backup baru tertinggal di `~/apps/mini-erp-shared/db-backups/`
  (`mini_erp_for_revamp_etl_20261016_101403.sql.gz`). File sementara media sudah dibersihkan.

## 3. Perubahan kode (belum di-commit)

ETL (`apps/api/internal/etl`):
- `transform/documents.go`: `syntheticTransfers` — pindah-lokasi legacy tanpa dokumen (out/in `transfer`
  dengan `id_reference` NULL) dibuatkan transfer `received` bernomor `TRF-<cabang>/MIG/<id>`.
- `transform/stock.go`: `refFor` memetakan mutasi tersebut; iterasi `mergedInto` diurutkan (determinisme `id_map`).
- `transform/master.go`: kode varian dipakai-ulang setelah arsip diberi sufiks `~<id>` (26 pasang).
- `transform/finance.go`: jurnal yang semua barisnya Rp0 tidak dimigrasi (`JRN_ZERO_VALUE`).
- `verify/verify.go`: V-01t/u, V-01x, V-05, V-12, V-17e diselaraskan dengan empat perubahan di atas.

Stok & test:
- `modules/stock/infrastructure/repository.go`: `ListCostMovements` memformat `moved_at` sebagai teks tetap `YYYY-MM-DD HH:MM:SS.ffffff`.
- `internal/integration/sqlc_stock_test.go` (file baru, belum ter-track): commit transaksi sebelum dibaca; fixture `MovementType: dir`.

Dokumen/skrip: `docs/RUNBOOK.md` §5b (prosedur ETL terbukti), `scripts/ops/etl-rehearsal.sh`, dokumen ini.

## 4. Urutan melanjutkan (jangan dibalik)

1. **Amankan lingkungan**: salin repo + dump + media ke disk sehat; `chkdsk` E: (lihat `docs/RUNBOOK.md`
   dan catatan §2). Kosongkan ruang C: sebelum memasang apa pun.
2. **Commit perubahan** §3 (pisahkan: ETL, stok+test, docs) setelah `npm run verify` hijau lagi.
3. **Gladi L1 → L2**: `scripts/ops/etl-rehearsal.sh` (L1), salin `etl-l1` ke `etl-l1-run1`, lalu
   `EXTRA="--compare-with <run1>"` (L2). Harapan: `pass=true` dan hash keluaran transform identik.
4. **E2E**: pasang Chromium Playwright (butuh ±150–300 MB — setelah ruang tersedia; set
   `PLAYWRIGHT_BROWSERS_PATH` ke disk yang sehat). Jalankan dengan
   `NODE_OPTIONS=--no-experimental-require-module E2E_SKIP_WEB_BUILD=1 npm run test:e2e` *(jangan beri flag itu ke `vite build`: crash)*.
   Latar: Node 22 memilih kondisi `module-sync` pada `generator-function` (via `exceljs`) dan loader Playwright
   gagal ("request for './index.js' is not in cache"); flag menonaktifkan `require(esm)`. Harapan: 46 test, 44+ lulus.
5. **Rekonsiliasi manual** oleh pemilik/akuntan dari `mini_erp_rehearsal` + laporan di `etl-l1/report`
   (neraca saldo, piutang/hutang per pihak, kas, nilai persediaan, stok per lokasi). Putuskan
   `--opening-date` (gladi memakai 2026-01-01) dan perlakuan `adjustments.csv`.
6. **UAT** di staging per peran (kasir, gudang, akuntan, admin) dan **verifikasi cetak fisik**
   (SJ, faktur, kwitansi, struk termal).
7. **Pasang CI** (`.github/workflows/ci.yml` ditulis, belum pernah dijalankan).
8. **Gladi cutover final** (L3) dari dump segar setelah freeze legacy (reservasi `active` = 0,
   collation diseragamkan) + simulasi rollback (`docs/RUNBOOK.md` §5) + uji restore backup.
9. Baru **deploy produksi dan cutover** (`docs/DEPLOYMENT.md`, `docs/RUNBOOK.md`).

## 5. Temuan data legacy yang harus diingat saat cutover nyata

- Collation tidak seragam (`payments`, `delivery_notes`, `delivery_note_items` = `utf8mb4_0900_ai_ci`).
- Pindah-lokasi satu langkah legacy tidak punya dokumen transfer (34 mutasi pada dump 2026-10-16).
- Kode varian dipakai ulang setelah arsip (26 pasang); revamp melarang duplikat (`uq_product_variants`).
- Satu jurnal koreksi persediaan berisi baris Rp0 saja (`JRN/2026/20770`).
- `seed init` membuat cabang UTM/profil/sequence/audit yang membuat target "tidak kosong" — kosongkan
  tabel itu dulu (katalog izin/peran/akun yang dibutuhkan ETL tetap).
- Daftar `object_key` dari `mysql` Windows berakhiran `\r`; media di S3 produksi (`vioni/production/upload/...`),
  unduh lewat server lama dengan SDK dan kredensialnya sendiri (GET saja; jangan bawa kunci keluar server).

## 6. Pembaruan (sesi lanjutan, 2026-10-16 siang) — langkah 1–4 SELESAI

| Hal | Hasil |
|---|---|
| `npm run verify` penuh (go test + integrasi 2.031 dtk, tsc, eslint, vitest, arch, sqlc, db-schema, contract) | **HIJAU** |
| Gladi L1 (dump 2026-10-16, sha256 `c32876b2…`) | `pass=true`, 86 pemeriksaan, `inserted=498179` |
| Gladi L2 `--compare-with` | `pass=true`, digest transform `47d1f107ae89…ee75de` **identik** dengan L1 → determinisme terbukti |
| E2E Playwright (Chromium terpasang) | **44 lulus, 2 dilewati, 0 gagal** (46 test) |
| Media | 3.502 tersalin; **281 `missing_source`** (objek tidak ada di S3; lihat `report/missing_media.csv`) — verifier V-15 mencatatnya terbuka. Konfirmasikan dengan pemilik apakah hilang sejak di legacy |

Catatan lingkungan: skrip `scripts/ops/etl-rehearsal.sh` kini berparameter (`OPS_ROOT`, `MEDIA_SRC`,
`WORKDIR`, `EXTRA`, `OPENING_DATE`) — taruh di SSD internal, **jangan USB** (disk USB putus berulang
dan membuat gladi harus diulang; skrip memang selalu mulai dari nol, bukan resume). Playwright:
`PLAYWRIGHT_BROWSERS_PATH=<disk sehat>` dan `NODE_OPTIONS=--no-experimental-require-module`.

Sisa (butuh manusia/lingkungan nyata): §4 langkah 5 (rekonsiliasi pemilik/akuntan + `--opening-date`),
6 (UAT + cetak fisik), 7 (CI jalan pertama kali di GitHub), 8 (gladi L3 dari dump segar setelah freeze +
simulasi rollback + uji restore), 9 (deploy & cutover).
