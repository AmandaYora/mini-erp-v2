# Database Guide — mini-erp

Ringkasan gateway. Kepemilikan tabel per modul & aturan penuh ada di
**[../docs/DB_SCHEMA.md](../docs/DB_SCHEMA.md)** — baca itu sebelum menulis migrasi baru, dan
tegakkan `.claude/rules/database.md` saat menyunting kode.

## Ringkas

- Setiap tabel dimiliki **tepat satu modul**. Modul lain akses lewat `contracts/`, tidak pernah
  lewat join SQL lintas modul.
- Relasi lintas modul = **primitive ID** (`party_id`, `product_id`, dst.), **tanpa** foreign key
  fisik lintas modul. FK boleh dipakai di dalam modul yang sama.
- Base kolom seragam di semua tabel domain: `id`, `branch_id` (nullable bila tidak
  terikat cabang), `created_at`/`updated_at`/`created_by`/`updated_by`, `deleted_at` (bila modul
  itu butuh arsip). **Tanpa `company_id`** — instalasi standalone single-tenant (ADR-0009).
- Uang = integer rupiah (lihat
  [decisions/ADR-0005-money-as-integer-rupiah.md](decisions/ADR-0005-money-as-integer-rupiah.md)).
  Timestamp disimpan UTC, dikonversi ke `Asia/Jakarta` di lapisan aplikasi.
- Migrasi lewat `golang-migrate`, bernomor & terlacak (`apps/api/migrations/NNNNNN_*.up.sql` /
  `.down.sql`). Generate akses data lewat `sqlc` dari query di
  `internal/modules/<modul>/infrastructure/queries/`.

## Kepemilikan Tabel per Modul

Tabel lengkap ada di [../docs/DB_SCHEMA.md §3](../docs/DB_SCHEMA.md#3-kepemilikan-tabel-per-modul).
Belum ada migrasi nyata — daftar di sana adalah rencana, dikunci saat modul itu diimplementasikan.
