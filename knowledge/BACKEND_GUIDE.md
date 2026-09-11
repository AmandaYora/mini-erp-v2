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
│   ├── queries/           # *.sql, sumber generate sqlc
│   ├── sqlc/               # kode hasil generate — jangan diedit manual
│   └── repository.go
├── presentation/          # HTTP handler
└── <nama>.module.go        # wiring/bootstrap + komentar tanggung jawab modul
```

20 modul sudah terisi penuh — lihat
[MODULE_MAP.md](MODULE_MAP.md) untuk daftar & urutan pengerjaan. `scripts/scaffold-modules.sh`
adalah skrip sekali-pakai yang membuatnya; aman dihapus setelah semua modul terisi.

## Go Toolchain

`apps/api/go.mod` menyatakan `go 1.21` — **samakan dengan versi yang benar-benar terpasang**
(`go version`, atau `GOTOOLCHAIN=local go version` bila `go.mod` sempat menyebut versi lebih baru
dari yang terpasang). Jangan naikkan versi di `go.mod` melebihi toolchain lokal tanpa mengecek,
supaya `go build`/`air` tidak mencoba mengunduh toolchain baru saat offline.

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

## Migrasi & sqlc

- Migrasi: `npm run migrate:create -- <nama_deskriptif>` dari root → menghasilkan pasangan
  `NNNNNN_<nama>.up.sql` / `.down.sql` di `apps/api/migrations/`.
- Query: tulis SQL di `internal/modules/<modul>/infrastructure/queries/*.sql`, lalu
  `npm run sqlc:generate` untuk menghasilkan kode Go type-safe.
- Satu migrasi = satu modul. Jangan menulis migrasi yang membuat tabel milik dua modul sekaligus.
- Sementara (CLI `migrate`/`sqlc` belum terpasang): repository ditulis langsung dengan
  `database/sql`, direktori `queries/` dicadangkan untuk sumber generate; migrasi dijalankan
  manual berurutan. Begitu CLI tersedia, query dipindah ke `queries/*.sql` + generate, tanpa
  mengubah kontrak modul.
