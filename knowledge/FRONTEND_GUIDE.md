# Frontend Guide — mini-erp

Stack terkunci (React 19, Tailwind 4, react-router-dom, Zustand, Zod, Axios, `@/*`, lazy routes,
`frontend-design` skill sebagai otoritas UI) ada di skill `monorepo-standard` →
`references/frontend.md`, ditegakkan lewat `.claude/rules/frontend-react.md`. File ini hanya berisi
hal yang **spesifik proyek ini**.

## Modul Frontend (`apps/web/src/modules/`)

Folder modul frontend adalah pengelompokan UX, tidak wajib 1:1 dengan modul backend. Pemetaan
saat ini:

| Folder modul frontend | Menyentuh modul backend |
|---|---|
| `auth` | `auth` |
| `users` | `user` (pengguna + role) |
| `branches` | `branch` |
| `company` | `company` |
| `products` | `product` (produk, kategori, impor) |
| `party` | `party` (customer, supplier, tipe member) |
| `purchasing` | `purchasing` |
| `sales` | `sales` (kanal regular; POS di folder sendiri) |
| `goodsreceipt` | `goodsreceipt` |
| `delivery` | `delivery` |
| `payment` | `payment` (pembayaran + saldo pihak) |
| `salesreturn`, `purchasereturn` | `salesreturn`, `purchasereturn` |
| `pos` | `sales` (kanal `pos`), `party`, `product`, `stock`, `payment` — **tanpa modul backend sendiri** |
| `stock` | `stock` |
| `finance` | `finance` (transaksi + laporan) |
| `dashboard`, `reporting`, `audit` | `dashboard`, `reporting`, `audit` |
| `assistant` | `assistant` (penyiapan WA + konfigurasi + konsol uji) |
| `ops` | `GET /ops/integrity` (pemeriksaan integritas lintas modul, dirakit di `bootstrap`) |
| `print` | Halaman cetak `/print/*` (lihat §"Pesan Server, Draf Form, dan Cetak") |

POS tidak punya modul backend ([ADR-0007](decisions/ADR-0007-pos-no-dedicated-module.md)): halaman
`pos` memanggil endpoint `sales` (dengan `channel: "pos"`), `party`, `product`, `stock`, dan `payment`
langsung.

## Hal yang Sudah Benar di Sistem Lama — Pertahankan Polanya

- Refresh token sekali pakai dengan rotasi hash (properti keamanan auth).
- Snapshot dokumen historis (nota tidak berubah saat master data berubah).
- Mode baca-saja tiga lapis (kendali nonaktif + teks penjelas + submit diabaikan) untuk pengaturan
  sensitif.

Detail & sumber lengkap ada di
[../docs/legacy-reference/00-overview.md §7](../docs/legacy-reference/00-overview.md).

## Umpan Balik ke Pengguna

Setiap aksi tulis yang gagal **wajib** menampilkan `message` asli dari API
(lihat [API_GUIDE.md](API_GUIDE.md)) — jangan menggantinya dengan teks generik seperti "Gagal
menyimpan data" tanpa alasan. Ini memperbaiki pola berulang di sistem lama (KI-04, KI-17, KI-29,
KI-43 di [../docs/legacy-reference/known-issues.md](../docs/legacy-reference/known-issues.md)).

## Ketahanan Muat & Galat (OP-01/OP-03)

- Halaman lazy didaftarkan di `app/routes/route-pages.ts`, yang memakai `lazyWithRetry` (`shared/lib/chunk-recovery.ts`) sebagai pengganti `React.lazy`: chunk basi pasca-deploy → muat ulang SEKALI (penjaga cap waktu di `sessionStorage`, cooldown `CHUNK_RELOAD_COOLDOWN_MS` = 30 dtk; tanpa `sessionStorage` TIDAK memuat ulang agar tak loop), lalu kartu galat. `vite:preloadError` dipasang di `main.tsx`. Tes: `chunk-recovery.test.tsx`, `route-error.test.tsx`, e2e E11.
- Galat render/rute tidak boleh layar putih: `errorElement` (`app/routes/RouteErrorBoundary.tsx`) ada di akar router dan di dalam tiap layout (sidebar tetap tampil); `ErrorBoundary` (`shared/components/feedback/error-boundary.tsx`) membungkus `<App/>`. Kartu = `CrashCard` (Indonesia, id kesalahan `ERR-…` dari `shared/lib/error-id.ts`, tombol muat ulang).
- Unggah gambar (bukti bayar, foto produk, bukti kirim) dikompres di klien lewat `compressImageForUpload` (`shared/lib/image-compression.ts`; `containWithin` menghitung ukuran) DI DALAM service unggah (`payment.service.ts` `uploadProof`, `delivery.service.ts`, `products.service.ts` `uploadMedia`); format tidak diubah, gagal → berkas asli. Unggahan gambar baru pakai util yang sama (`image-compression.test.ts`).
- Aset statis (OP-02): `apps/web/public/` memuat `favicon.svg`, `logo.svg`, ikon PNG (32/180/192/512), `robots.txt` (disallow), dirujuk `index.html` (ikon, apple-touch, `theme-color`, noindex); dibangkitkan ulang dengan `scripts/gen-web-icons.mjs`.

## Daftar Master: Default "Aktif" + Arsip (PR-04/PM-02)

- Daftar produk, pelanggan, pemasok, dan jenis member dibuka dengan filter status `active`; "Semua status" = nilai `all` di URL (terpisah agar tahan di URL walau default-nya `active`), arsip = `archived`. Pemilih (party/POS/`search-options`) selalu memakai `status=active` eksplisit.
- Pintasan satu klik antara daftar aktif dan arsip: `ArchiveViewToggle` (`shared/components/patterns/archive-view-toggle.tsx`, domain-agnostik; `showingArchive` + `onChange`), dipakai `ProductsPage`, `CustomersPage`, `SuppliersPage`, `MemberTypesPage`. Daftar yang menampilkan arsip memunculkan kolom tanggal arsip (`archivedAt`).
- Filter di URL lewat `shared/hooks/use-url-filters.ts`; param dibersihkan dengan `compactParams` di `shared/lib/list-query.ts` (search, status CSV, `from`/`to`, `partyId`, `sort`/`dir` seragam di semua daftar).

## Label Audit (U-02, OP-06)

`modules/audit/lib/audit-labels.ts` memetakan setiap kunci aksi (`users.create`, `error.unexpected`, …) dan entitas ke label Indonesia (`actionLabel`, `entityName`; kunci tak dikenal tampil apa adanya). Daftar kunci bersumber dari kode backend: `audit-labels.test.ts` memindai seluruh sumber Go dan **gagal** bila backend menulis aksi/entitas tanpa label di sini (atau label menunjuk kunci yang tak lagi ditulis). Menambah aksi audit baru di backend = menambah labelnya di file itu pada tugas yang sama.

## Pesan Server, Draf Form, dan Cetak

- Toast tulis memakai pesan server: `toast.fromServer(res.message, fallback, deskripsi)` (`shared/stores/toast.store.ts`) bersama varian service `api*Full` yang mempertahankan `{data, message}`; jangan menggantinya dengan teks generik.
- Data async lewat `useAsyncData` (`shared/hooks/use-async-data.ts`); form panjang menyimpan draf di `localStorage` lewat `use-form-draft` + `shared/lib/draft-storage.ts` (kunci per pengguna, kedaluwarsa, dibersihkan saat logout, semua akses dibungkus try/catch).
- Halaman cetak `/print/*` (SJ `print/deliveries/:id`, Surat Transfer `print/transfers/:id`, faktur `print/sales/:id`, kwitansi `print/payments/:id`, struk POS `print/pos/:orderId`, label QR `print/labels`, kalibrasi `print/calibration`) didaftarkan di `app/routes/protected.routes.tsx`. Toggle "Sembunyikan diskon" POS sampai ke struk/faktur lewat query `?hideDiscount=1` (`modules/pos/lib/receipt-links.ts`, SL-05). Rute baru wajib punya izin (`perm`) di registry; `app/routes/route-access.test.ts` menjaga kecocokan menu vs izin.
- Pemindaian POS = `modules/pos/components/ScanDialog.tsx` (kamera `@zxing/browser`).

## Design System v2 "Bingkai Navy" (ADR-0017)

Sumber lengkap: [`docs/plan/redesain-ui/PLAN.md`](../docs/plan/redesain-ui/PLAN.md); resep migrasi per halaman:
[`PANDUAN-MIGRASI-HALAMAN.md`](../docs/plan/redesain-ui/PANDUAN-MIGRASI-HALAMAN.md). Ringkasan aturan kerja:

- **Lapisan**: `theme/theme.css` (token) → `shared/components/primitives` (shadcn/Radix, jangan diimpor modul) →
  `shared/components/ui` (fasad: Button, Modal, DataTable, FormField, …) → `shared/components/patterns`
  (Page, DocumentRail, LineItemsEditor, Toolbar, DescriptionList, StickyFooter, FileDropzone, NavTabs, ActionMenu, …).
- **Warna & tipografi**: hanya kelas semantik (`bg-card`, `text-muted-foreground`, `border-border`,
  `text-success|warning|destructive`); tanpa hex dan tanpa palet Tailwind. Skala `text-2xs … text-4xl`; angka besar
  `font-display`, nomor dokumen `font-mono` (`DocNumber`). Uang/qty: `Money`/`Qty` (tabular, rata kanan).
- **Status**: selalu `StatusBadge`/`statusTone` (neutral = belum jalan, info = berjalan, warning = perlu perhatian,
  success = selesai, muted = berhenti, danger = masalah).
- **Template halaman**: daftar (Toolbar + DataTable + Pagination, sortir lewat header), detail dokumen (PageHeader dengan
  aksi + DocumentRail + PageColumns; maksimal satu tombol primary), form berbaris barang (LineItemsEditor + StickyFooter),
  master data (FormSheet), laporan (NavTabs, format akuntansi), pengaturan (FormSection + footer bila ada perubahan).
- **Penempatan alur**: dialog ≤ 6 isian; `FormSheet` untuk CRUD master & aksi dokumen; halaman penuh untuk dokumen
  ber-baris barang (retur, surat jalan, penerimaan, transfer, jurnal manual: `…/new?from=<id>`).
- **State**: skeleton (bukan teks "Memuat…"), `ErrorState` dengan `message` API asli + "Coba lagi", Empty dengan aksi.
  Backend (Go) mengirim `null` untuk slice kosong: `DataTable.rows` menerimanya; gunakan `?? []` di tempat lain.
- **Satu `<h1>`** per halaman (dari `PageHeader`); breadcrumb ada di kerangka; halaman detail memanggil
  `usePageCrumb(nomor)`.
- **Kepadatan** (Ringkas/Standar/Lega) lewat `data-density` di `<html>`; pilihan di menu akun.
- **Cetak**: halaman `/print/*` tidak berubah (kertas fisik); `LEGACY_UA_CSS` memulihkan default browser.
- **Penjaga**: `npm run ui:ratchet` (angka pola lama per modul tidak boleh naik), ESLint melarang impor primitif dari modul.
- Tema gelap ditunda; komponen tetap hanya memakai token agar kelak cukup menambah blok `.dark`.
