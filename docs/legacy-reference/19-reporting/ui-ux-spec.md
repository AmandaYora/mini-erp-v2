# UI/UX Spec — Modul 19 Reporting

**Kelompok A. Tidak ada UI (konfirmasi ganda!).** Modul ini tanpa folder web
(`apps/web/src/modules/`: assistant/audit-log/auth/business-party/company/core/
dashboard/finance/member-types/orders/pos/products/purchase-returns/sales-returns/
shared/stock/users — tanpa `reporting`!), tanpa rute `/reporting` di
`module-registry.tsx` (944 baris; satu-satunya ringkasan visual = `/dashboard` 200–207!),
tanpa slice/hook/halaman/komponen — diverifikasi via grep (`reporting/` di
`apps/web/src` hanya `role-access-config.test.ts:16` + `reporting.view` di
`role-access-config.ts:111–114`!). Bandingkan dashboard yang 1 endpoint + 1 halaman
(`dashboard.controller.ts:15–20` + `dashboard-page.tsx` 628 baris) — reporting 2 endpoint
+ 0 halaman. Satu-satunya "permukaan" yang terlihat user:

## 1. Label Izin di Halaman Role & Akses

- Grup **"Laporan Operasional"** dengan satu izin: `reporting.view` =
  **"Lihat Laporan"** (`role-access-config.ts`).
- Izin ini **system-only**: tidak tampil untuk di-assign ke custom role
  (dijaga oleh test `role-access-config.test.ts` bersama `whatsapp.simulate`).
  User custom-role tidak akan pernah melihatnya — hanya role bawaan
  (superadmin/owner/admin) yang memilikinya via default.

## 2. Tidak Ada Layar Lain

- Tanpa halaman daftar, filter, tabel, tombol, modal, toast, empty-state,
  loading, maupun error-view milik modul ini.
- Konsumen endpoint (`reporting/orders`, `reporting/stock`) hari ini hanyalah
  test E2E `10-observability-permission-hardening.spec.ts` dan pemanggil API
  langsung — tidak ada layar yang memanggilnya.
- Error yang mungkin terlihat pemanggil API: `401` (tanpa token),
  `403` (+ `Branch aktif belum dipilih` bila tanpa cabang aktif) — envelope
  error global, bukan tampilan modul.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [user-flows.md](user-flows.md)
