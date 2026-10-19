# Backend Guide — mini-erp

Stack terkunci (Go, Air, golang-migrate, sqlc, `database/sql`, bukan GORM) ada di skill
`monorepo-standard` → `references/backend-go.md` dan `references/backend-modular-monolith.md`,
ditegakkan lewat `.claude/rules/backend-modular-monolith.md` dan `.claude/rules/database.md`. File
ini hanya berisi hal yang **spesifik proyek ini**.

## Struktur Modul (sudah discaffold, isi satu per satu)

```txt
apps/api/internal/modules/<nama>/
├── contracts/            # satu-satunya yang boleh diimpor modul lain
├── application/          # use case, orkestrasi
├── domain/
│   └── events/
├── infrastructure/
│   ├── queries/           # *.sql, sumber generate sqlc (diisi saat modul migrated)
│   ├── sqlc/               # kode hasil generate — jangan diedit manual
│   └── repository.go       # tulisan tangan hanya untuk modul pending / raw-sql-allow.json
├── presentation/          # HTTP handler
└── <nama>.module.go        # wiring/bootstrap + komentar tanggung jawab modul
```

20 modul sudah terisi penuh — lihat
[MODULE_MAP.md](MODULE_MAP.md) untuk daftar & urutan pengerjaan. `scripts/scaffold-modules.sh`
adalah skrip sekali-pakai yang membuatnya; aman dihapus setelah semua modul terisi.

## Go Toolchain

`apps/api/go.mod` menyatakan `go 1.26.0` dan mengunci `toolchain go1.26.7` (A4: semua mesin dan CI
membangun dengan toolchain yang sama). Alat bantu yang dipakai repo: `golang-migrate` (CLI `migrate`),
`sqlc` v1.31.1, `air` (hot reload), `golangci-lint` v2.14.0 (belum ada `.golangci.yml`; konfigurasi bawaan dipakai). Jangan menurunkan
`go`/`toolchain` di `go.mod`; bila mesin offline dan toolchain belum terunduh, pasang versi itu lebih dulu.

## Konvensi yang Wajib Sejak Modul Pertama

Lihat alasan lengkap di [../docs/PRD.md §6](../docs/PRD.md#6-perbaikan-arsitektur-yang-wajib-diadopsi-bukan-opsional):

1. Validasi payload eksplisit di setiap handler `presentation/` yang menerima body.
2. Base kolom seragam (lihat [DATABASE_GUIDE.md](DATABASE_GUIDE.md)) — definisikan sekali di
   `internal/shared/`, jangan diulang manual per modul.
3. Permission guard fail-closed — endpoint tanpa deklarasi permission eksplisit ditolak di
   middleware, bukan lolos.
4. Scope sesi (`branchId`/`userId`) selalu diambil dari middleware `auth`, tidak pernah
   dari parameter request yang bisa dimanipulasi klien.
5. Setiap operasi tulis memanggil `AuditClient.Log` (modul `audit`, L0) dengan `action_key`
   `snake_case` `resource.action`.
6. Uang sebagai integer rupiah — lihat
   [decisions/ADR-0005-money-as-integer-rupiah.md](decisions/ADR-0005-money-as-integer-rupiah.md).

## Migrasi

Skema dasar sudah di-squash menjadi **18 berkas baseline, satu per modul** (`NNNNNN_<modul>.up.sql` /
`.down.sql` di `apps/api/migrations/`): `000001_user`, `000002_auth`, `000003_branch`, `000004_company`,
`000005_media`, `000006_product`, `000007_party`, `000008_stock`, `000009_purchasing`, `000010_sales`,
`000011_goodsreceipt`, `000012_delivery`, `000013_payment`, `000014_salesreturn`, `000015_purchasereturn`,
`000016_finance`, `000017_audit`, `000018_assistant` (dashboard dan reporting tidak punya tabel). Baseline
dibangkitkan `scripts/squash-baseline.mjs` dan dibuktikan identik dengan rantai lama oleh
`scripts/schema-compare.mjs`; jangan diedit untuk mengubah skema — tambahkan migrasi baru.

- **Satu migrasi = satu modul.** Nama berkas menentukan pemilik tabel: `NNNNNN_<modul>[_<perubahan>].up.sql`.
  `scripts/arch-check.mjs` memakai nama itu untuk Rule T (SQL sebuah modul hanya menyebut tabel miliknya)
  dan Rule F (tak ada `FOREIGN KEY` lintas modul), serta `scripts/sqlc-sync.mjs` memakainya untuk
  menyusun `sqlc.yaml`.
- **Menambah migrasi:** `npm run migrate:create -- <modul>_<perubahan>` (nomor berikutnya mulai dari
  `000019`). Tulis `.up.sql` dan `.down.sql` yang hanya menyentuh tabel modul itu, lalu
  `npm run migrate:up`. Migrasi yang sudah dirilis tidak diubah.
- **Membangun ulang DB dev dari nol:** `DROP DATABASE`/`CREATE DATABASE` database dev di MySQL host,
  lalu `npm run migrate:up`. (golang-migrate melacak versi di `schema_migrations`; dev DB lama yang
  dibangun dari rantai sebelum squash harus dibuang, bukan dimigrasi.)
- **DB test (`mini_erp_test`):** harness integrasi memigrasi dari berkas yang sama dan menyimpan hash
  migrasi; hash yang berubah otomatis membangun ulang skema (tabel di-drop lalu dimigrasi dari nol).
  Paksa bangun ulang total (DROP DATABASE) dengan `TESTDB_REBUILD=1`
  (`TESTDB_REBUILD=1 node scripts/db-lock.mjs -- go test ./internal/integration -run '<satu pola>' -count=1`,
  dari `apps/api`) atau `node scripts/test-db.mjs reset`. DB test dipakai bersama: selalu jalankan test
  integrasi lewat `scripts/db-lock.mjs`.

## sqlc (status: migrasi bertahap, 16/18 modul migrated)

Akses data: modul `migrated` (lihat `scripts/sqlc-migration-status.json`)
memakai query di `infrastructure/queries/*.sql` + kode hasil generate di
`infrastructure/sqlc/`; modul `pending` masih `database/sql` tulisan tangan.
**Target** (ADR-0012): query pindah ke `infrastructure/queries/*.sql` + kode hasil generate di
`infrastructure/sqlc/`, modul demi modul, sebagai fase terakhir.

- `apps/api/sqlc.yaml` DIBANGKITKAN `node scripts/sqlc-sync.mjs` (jangan edit manual): satu blok per
  modul bertabel, `schema` = hanya berkas migrasi modul itu, sehingga query yang menyebut tabel modul
  lain gagal saat generate. `npm run sqlc:generate` menjalankan sync lalu `sqlc generate`.
- `scripts/sqlc-migration-status.json` memuat daftar `migrated` / `pending` / `noSql`. Pindahkan modul
  dari `pending` ke `migrated` di commit yang sama dengan `queries/*.sql` dan `sqlc/db.go` hasil generate;
  arch-check Rule C menolak daftar yang tak cocok dengan pohon, dan Rule R melarang `QueryContext`/
  `ExecContext` mentah di modul `migrated` (kecuali `scripts/raw-sql-allow.json` dengan alasan).
- Sampai sebuah modul `migrated`, repository-nya tetap `database/sql` mentah. Helper konversi null
  dipusatkan di `internal/shared/dbutil` (`NullIfEmpty`, `NullIfZero`, `NullPtr`, `Bool`, `ParseDecimal`,
  `FormatNullTime`) — jangan menyalinnya per modul.

## Wiring antarmodul

`internal/bootstrap.Wire` adalah satu-satunya tempat yang melihat semua modul; server, seed E2E, dan test
integrasi memakai fungsi yang sama. Ketergantungan yang bisa menjadi argumen konstruktor SUDAH argumen
konstruktor. Yang tersisa berupa setter adalah siklus konstruksi nyata (daftarnya ada di komentar `Wire`
dan di `scripts/arch-allow.json`, aturan W). `Wire` diakhiri `Modules.Verify()` sehingga setter yang
terlupa gagal saat start, bukan pada request pertama; test `internal/bootstrap/wire_test.go` menjaganya.
Jangan menambah metode `Set*` baru pada `Module` tanpa alasan di allow-list.

## Penjaga arsitektur

`node scripts/arch-check.mjs` (aturan A, B, C, D, E, F, K, M, P, R, S, T, W; lihat header skripnya) wajib
hijau; `node scripts/arch-check.mjs --self-test` membuktikan tiap aturan memang menyala pada fixture.
Pengecualian yang sah ditulis di `scripts/arch-allow.json` dengan alasan; entri yang tak lagi dibutuhkan
membuat check gagal sehingga daftar hanya menyusut.

## Deploy & operasi

- Image: `infra/docker/Dockerfile`; compose: `docker-compose.yml` (satu container,
  DB di host); proxy contoh: `infra/nginx/nginx.conf`.
- Migrasi adalah langkah deploy eksplisit (`docker compose --profile ops run --rm migrate up`);
  biner menolak start di schema lama (`internal/database/schemacheck.go`, F6-3).
- Operasi harian (backup/restore/rollback): `docs/RUNBOOK.md` + `scripts/ops/`.
