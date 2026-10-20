# Backend Guide — mini-erp

Stack terkunci (Go, Air, golang-migrate, sqlc, `database/sql`, bukan GORM) berasal dari skill
`monorepo-standard` → `references/backend-go.md` dan `references/backend-modular-monolith.md`,
ditegakkan lewat `.claude/rules/backend-modular-monolith.md` dan `.claude/rules/database.md`. File
ini hanya berisi hal yang **spesifik proyek ini**. Migrasi, sqlc, dan DB test:
[DATABASE_GUIDE.md](DATABASE_GUIDE.md).

## Struktur Modul

```txt
apps/api/internal/modules/<nama>/
├── contracts/            # satu-satunya yang boleh diimpor modul lain
├── application/          # use case, orkestrasi (transaction script, ADR-0014)
├── domain/
│   └── events/
├── infrastructure/
│   ├── queries/           # *.sql, sumber generate sqlc (modul migrated)
│   ├── sqlc/              # kode hasil generate — jangan diedit manual
│   └── repository.go      # SQL tulisan tangan hanya untuk modul pending / raw-sql-allow.json
├── presentation/          # HTTP handler + routes
└── <nama>.module.go       # wiring + komentar tanggung jawab modul
```

Daftar modul, lapisan, dan dependensinya: [MODULE_MAP.md](MODULE_MAP.md). Program pendukung di
`apps/api/cmd/`: `server`, `seed` (`init`, `admin`), `etl` (migrasi data legacy), `costrebuild`
(restatement biaya persediaan, lihat RUNBOOK §6b).

## Go Toolchain

`apps/api/go.mod` menyatakan `go 1.26.0` dan mengunci `toolchain go1.26.7` (semua mesin dan CI
membangun dengan toolchain yang sama). Alat bantu: `golang-migrate` (CLI `migrate`), `sqlc` v1.31.1,
`air` (hot reload), `golangci-lint` v2.14.0 (konfigurasi bawaan; belum ada `.golangci.yml`). Jangan
menurunkan `go`/`toolchain` di `go.mod`; bila mesin offline dan toolchain belum terunduh, pasang versi
itu lebih dulu. Perintah instalasinya: [../docs/DEPLOYMENT.md](../docs/DEPLOYMENT.md) §1.

## Konvensi Wajib di Setiap Modul

Alasan lengkap di [../docs/PRD.md §6](../docs/PRD.md#6-perbaikan-arsitektur-yang-wajib-diadopsi-bukan-opsional):

1. Validasi payload eksplisit di setiap handler `presentation/` yang menerima body.
2. Base kolom seragam (lihat [DATABASE_GUIDE.md](DATABASE_GUIDE.md)) — definisikan sekali di
   `internal/shared/`, jangan diulang manual per modul.
3. Permission guard fail-closed — endpoint tanpa deklarasi permission eksplisit ditolak di
   middleware, bukan lolos.
4. Scope sesi (`branchId`/`userId`) selalu diambil dari middleware `auth`, tidak pernah
   dari parameter request yang bisa dimanipulasi klien.
5. Setiap operasi tulis memanggil `AuditClient.Log` (modul `audit`, L0) dengan `action_key`
   `snake_case` `resource.action`, di dalam transaksi pemanggil bila ada ([ADR-0011](decisions/ADR-0011-audit-in-transaction.md)).
   Aksi baru wajib diberi label di `apps/web/src/modules/audit/lib/audit-labels.ts`.
6. Uang sebagai integer rupiah, aritmetika lewat `shared/money` ([ADR-0005](decisions/ADR-0005-money-as-integer-rupiah.md),
   [ADR-0010](decisions/ADR-0010-shared-money-kernel.md)).
7. Langkah lintas modul idempoten + CAS, bukan transaksi terdistribusi ([ADR-0016](decisions/ADR-0016-atomicity-by-idempotency-and-cas.md)).

## Wiring Antarmodul

`internal/bootstrap.Wire` adalah satu-satunya tempat yang melihat semua modul; server, seed E2E, dan test
integrasi memakai fungsi yang sama. Ketergantungan yang bisa menjadi argumen konstruktor SUDAH argumen
konstruktor. Yang tersisa berupa setter adalah siklus konstruksi nyata (daftarnya ada di komentar `Wire`
dan di `scripts/arch-allow.json`, aturan W). `Wire` diakhiri `Modules.Verify()` sehingga setter yang
terlupa gagal saat start, bukan pada request pertama; test `internal/bootstrap/wire_test.go` menjaganya.
Jangan menambah metode `Set*` baru pada `Module` tanpa alasan di allow-list.

## Penjaga Arsitektur

`node scripts/arch-check.mjs` (aturan A, B, C, D, E, F, K, M, P, R, S, T, W; lihat header skripnya) wajib
hijau; `node scripts/arch-check.mjs --self-test` membuktikan tiap aturan memang menyala pada fixture.
Pengecualian yang sah ditulis di `scripts/arch-allow.json` dengan alasan; entri yang tak lagi dibutuhkan
membuat check gagal sehingga daftar hanya menyusut. `npm run contract:check` mencocokkan setiap panggilan
frontend dan kode izin dengan router; tabel rute di `docs/API_CONTRACT.md` §6 tetap diperbarui manual.

Deploy dan operasi: [../docs/DEPLOYMENT.md](../docs/DEPLOYMENT.md), [../docs/RUNBOOK.md](../docs/RUNBOOK.md).
