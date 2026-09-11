# Reports List — Modul 20 Audit Log

**Kelompok A.** Modul ini menghasilkan **1 daftar**: jejak aktivitas itu sendiri
(bukan laporan analitik — tanpa agregat, total, grafik, maupun ekspor).

## 1. Daftar Jejak — `audit-logs/list` (+ halaman `/audit-logs`)

- **Kontrak:** `POST /api/v1/audit-logs/list`, JWT + `audit_log.view`, tanpa BranchGuard;
  body `{ data: { id_branch?, entity_type?, action_key?, page?, limit? } }`
  (amplop `{ data }` global — `api.ts:3-4,114-116`; controller membaca `@Body('data')`,
  `audit-log.controller.ts:17`); respons `data = { items, meta: { page, limit, total } }`
  (amplop sukses global `{ code: 0, info, data }` — `api.ts:4`).
- **Filter:** `id_branch` / `entity_type` (persis `=`) / `action_key` (substring `LIKE %v%`) —
  didukung API, **tak dipakai halaman** (selalu tanpa filter, 25/halaman).
- **Kolom:** Waktu (`id-ID` tanpa detik) | Pengguna (nama/`User #`/`System`) |
  Aktivitas (label Indonesia/fallback) | Data Terkait (`tipe #id` + label) |
  Keterangan (selalu default).
- **Logika:** append-only perusahaan, terbaru dulu, paginasi server (default 20/cap 100);
  nama di-resolve dari store (100 user pertama + cabang).
- **Konsumsi nyata:** review pemilik/admin + E2E 10 (3 asersi aksi:
  `order.create`/`order`, `stock.adjust`/`inventory_balance`, `payment.create`/`payment`)
  + E2E 08 (`stock.adjust`/`inventory_balance`) + prefetch slice (100 pertama).
