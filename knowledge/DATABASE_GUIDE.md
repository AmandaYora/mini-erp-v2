# Database Guide — mini-erp

Cara bekerja dengan database: aturan tabel, migrasi, sqlc, dan DB test. Daftar tabel per modul dan
riwayat migrasi ada di **[../docs/DB_SCHEMA.md](../docs/DB_SCHEMA.md)**; aturan yang ditegakkan saat
menyunting ada di `.claude/rules/database.md`.

## Aturan

- Setiap tabel dimiliki **tepat satu modul**. Modul lain akses lewat `contracts/`, tidak pernah
  lewat join SQL lintas modul.
- Relasi lintas modul = **primitive ID** (`party_id`, `product_id`, dst.), **tanpa** foreign key
  fisik lintas modul. FK boleh dipakai di dalam modul yang sama.
- Base kolom seragam di tabel domain: `id` (`BIGINT UNSIGNED AUTO_INCREMENT`), `branch_id` (nullable
  bila tidak terikat cabang), `created_at`/`updated_at`/`created_by`/`updated_by`, `deleted_at` (bila
  modul itu butuh arsip). **Tanpa `company_id`** — instalasi standalone single-tenant (ADR-0009).
- Uang = integer rupiah ([ADR-0005](decisions/ADR-0005-money-as-integer-rupiah.md)); kuantitas &
  faktor konversi `DECIMAL(24,12)`. Kolom `DATETIME(6)` punya dua makna, dan tiap kolom hanya satu:
  - **stempel waktu sistem** (`created_at`, `updated_at`, `completed_at`, `dispatched_at`,
    `confirmed_at`, `moved_at`, audit): instan **UTC** (sesi DB `time_zone='+00:00'`);
  - **tanggal dokumen bisnis** (`order_date`, `due_date`, `*_invoice_date`, `paid_at`, `return_date`,
    `settlement_date`, `received_at` GR, `delivery_date`, `transfer_date`, `expense_date`, `entry_date`):
    tanggal/jam dinding **WIB** naif (`timeutil.DateOrToday`), dibaca apa adanya oleh filter
    (`listquery.DateRange`) dan jurnal. Data migrasi mengikuti aturan yang sama (RUNBOOK §5b).
  - Turunan dari stempel UTC selalu lewat kalender WIB: hari jurnal/antrean `timeutil.WIBDay`, filter
    hari `listquery.DateRangeWIB` / `timeutil.StartOfDayWIB`, nilai di response `timeutil.FormatWIB`
    (format kawat: `docs/API_CONTRACT.md` §2). Jangan memotong `[:10]` dari stempel UTC.
- Satu transaksi = satu koneksi: baca dan tutup `rows` sepenuhnya sebelum query berikutnya di tx yang
  sama.
- Tabel baru dengan endpoint daftar → indeks `(branch_id, id)` di migrasi yang sama
  ([PERFORMANCE_GUIDE.md](PERFORMANCE_GUIDE.md) §4).

## Migrasi

Skema dasar berupa **18 berkas baseline, satu per modul** (`000001_user` … `000018_assistant` di
`apps/api/migrations/`; `dashboard` dan `reporting` tidak punya tabel), ditambah migrasi bernomor
sesudahnya (daftarnya di [DB_SCHEMA.md §8](../docs/DB_SCHEMA.md)). Baseline dibangkitkan
`scripts/squash-baseline.mjs` dan dibuktikan identik dengan rantai lama oleh `scripts/schema-compare.mjs`;
jangan diedit untuk mengubah skema — tambahkan migrasi baru.

- **Satu migrasi = satu modul.** Nama berkas menentukan pemilik tabel: `NNNNNN_<modul>[_<perubahan>].up.sql`.
  `scripts/arch-check.mjs` memakai nama itu untuk Rule T (SQL sebuah modul hanya menyebut tabel miliknya)
  dan Rule F (tak ada `FOREIGN KEY` lintas modul), dan `scripts/sqlc-sync.mjs` memakainya untuk
  menyusun `sqlc.yaml`.
- **Menambah migrasi:** `npm run migrate:create -- <modul>_<perubahan>`. Tulis `.up.sql` dan `.down.sql`
  yang hanya menyentuh tabel modul itu, lalu `npm run migrate:up`. Migrasi yang sudah dirilis tidak diubah.
  Kolom/tabel baru harus aman untuk data hasil ETL legacy.
- **Setelah daftar tabel berubah:** `node scripts/db-schema-doc.mjs` (memperbarui DB_SCHEMA §3), lalu
  tambahkan baris di DB_SCHEMA §8.
- **Membangun ulang DB dev dari nol:** `DROP DATABASE`/`CREATE DATABASE` database dev di MySQL host,
  lalu `npm run migrate:up`. (Dev DB lama yang dibangun dari rantai sebelum squash harus dibuang, bukan
  dimigrasi.)
- **Produksi:** migrasi adalah langkah deploy eksplisit; biner menolak start di skema lama —
  [../docs/DEPLOYMENT.md](../docs/DEPLOYMENT.md) §5.

## sqlc

Migrasi ke sqlc berjalan **bertahap per modul** ([ADR-0012](decisions/ADR-0012-sqlc-adoption-and-raw-sql-policy.md)).
`scripts/sqlc-migration-status.json` adalah ratchet-nya: setiap modul ada di tepat satu daftar —
`migrated`, `pending`, atau `noSql` (tanpa tabel).

- Modul `migrated`: query di `infrastructure/queries/*.sql`, kode hasil generate di `infrastructure/sqlc/`
  (jangan diedit manual), `mapper.go` mengubah baris hasil generate menjadi tipe `contracts`. SQL mentah
  (`QueryContext`/`ExecContext`) dilarang (arch-check Rule R) kecuali tercatat di `scripts/raw-sql-allow.json`
  dengan alasan dan komentar `// raw-sql: <alasan>`.
- Modul `pending`: repository masih `database/sql` tulisan tangan. Memindahkannya ke `migrated` dilakukan
  di commit yang sama dengan `queries/*.sql` dan `sqlc/db.go` hasil generate; arch-check Rule C menolak
  daftar yang tak cocok dengan pohon.
- `apps/api/sqlc.yaml` DIBANGKITKAN `node scripts/sqlc-sync.mjs` (jangan edit manual): satu blok per modul
  bertabel, `schema` = hanya berkas migrasi modul itu, sehingga query yang menyebut tabel modul lain gagal
  saat generate. `npm run sqlc:generate` menjalankan sync lalu `sqlc generate`.
- Helper konversi null dipusatkan di `internal/shared/dbutil` (`NullIfEmpty`, `NullIfZero`, `NullPtr`,
  `Bool`, `ParseDecimal`, `FormatNullTime`) — jangan menyalinnya per modul.

## DB test

- `mini_erp_test` adalah satu-satunya DB test dan **dipakai bersama** test integrasi Go dan E2E. Siapkan
  dengan `node scripts/test-db.mjs up`; selalu jalankan test integrasi lewat `scripts/db-lock.mjs`.
- Harness integrasi memigrasi dari berkas yang sama dan menyimpan hash migrasi; hash yang berubah
  otomatis membangun ulang skema. Paksa bangun ulang total dengan `node scripts/test-db.mjs reset`, atau
  `TESTDB_REBUILD=1 node scripts/db-lock.mjs -- go test ./internal/integration -run '<pola>' -count=1`
  (dari `apps/api`).
