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

Galat validasi datang sebagai `{message: "Validasi gagal", errors: [{field, message}]}`. Handler tulis
yang tidak memetakan `errors[]` ke input form wajib menampilkan `formatApiError(err)`
(`shared/services/http-client.ts`: `message` + pesan per-field), bukan `toApiError(err).message` —
tanpa itu pengguna hanya melihat "Validasi gagal".

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
- Halaman cetak `/print/*` (SJ `print/deliveries/:id`, Surat Transfer `print/transfers/:id`, faktur `print/sales/:id`, kwitansi `print/payments/:id`, struk POS `print/pos/:orderId`, label QR `print/labels`, kalibrasi `print/calibration`) didaftarkan di `app/routes/protected.routes.tsx`. Toggle "Sembunyikan diskon" POS sampai ke struk/faktur lewat query `?hideDiscount=1` (`modules/pos/lib/receipt-links.ts`, SL-05); berlaku hanya untuk dokumen lunas — aturannya satu tempat, `hideDiscountAllowed` di `modules/print/lib/sales-document.ts` (ADR-0018 D12). Rute baru wajib punya izin (`perm`) di registry; `app/routes/route-access.test.ts` menjaga kecocokan menu vs izin.
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
- **Daftar ber-filter URL** (`useUrlFilters`): beri `DataTable` sebuah `columnsKey` (pilihan kolom) dan pasang
  `<SavedViews listKey columnsKey />` di slot `right` Toolbar — "Tampilan" menyimpan query string (tanpa `page`) + kolom
  tersembunyi dengan sebuah nama. Kolom, tampilan, dan "Terakhir dibuka" disimpan di `localStorage` **per pengguna**
  lewat `userKey()` (`shared/stores/ui-prefs.store.ts`, pemilik diisi store auth) dan `shared/lib/list-views.ts`.
- **Command palette** (Ctrl/Cmd+K): menu, aksi cepat, terakhir dibuka, dan **lompat ke dokumen** — mengetik nomor
  (`ORD-…/PJ|PB`, `PAY-`, `SJ-`, `RTR-`, `RTB-`) mencari lewat endpoint daftar yang sudah ada
  (`app/routes/document-search.ts`, dimuat malas). Jenis dokumen baru yang daftarnya punya `search` cukup ditambah di
  `SOURCES` di file itu.
- **Label & pindai**: label produk yang dicetak sistem adalah **QR** (isi: barcode varian, else kode varian/produk —
  `product/application/qr.go`). Pemindai kamera membaca QR dan barcode pabrik (EAN dsb.), dan `stock/scan-product`
  mencocokkan barcode/kode varian/kode produk. Istilah di UI: "Pindai QR / kode"; "Barcode" hanya untuk isian data
  barcode varian di form produk. Dijaga e2e E16.
- **Responsif (U7-3, dijaga e2e V02)**: halaman tidak boleh menggulir menyamping di 390/768; tabel menggulir di dalam
  kartunya. Grid yang memakai `md:/lg:/xl:grid-cols-[…]` wajib punya kolom dasar `grid-cols-1` (= `minmax(0,1fr)`):
  tanpa itu kolom implisit `auto` melebar mengikuti tabel `min-w-[640px]` di dalamnya.
- **LineItemsEditor** berganti ke mode kartu (label di atas isian, tanpa gulir menyamping) bila lebar **wadahnya**
  < 600px; render kolom tetap satu, jadi isian tidak perlu tahu modenya.
- **Keyboard (U7-2, dijaga e2e E15)**: semua alur back-office harus bisa dikerjakan tanpa tetikus. Pemilih
  (`SearchSelect`/`AsyncSearchSelect`/`HierarchicalSelect`): Enter membuka, ketik, Enter memilih hasil pertama
  (`AsyncSearchSelect` menunggu hasil kata kunci terbaru, tidak memilih dari daftar basi), ↓/↑ berpindah opsi;
  opsinya ber-`role="option"`. Event keyboard dari popover (portal) tidak boleh merambat ke baris `LineItemsEditor`.
  Pemilih di dalam baris wajib punya `aria-label` per baris (mis. `Lokasi ${produk}`, `Produk baris ${n}`).
  `ConfirmDialog` memfokuskan "Batal" saat terbuka (Radix `AlertDialog.Cancel`) — jangan ganti dengan tombol biasa.
- **Pemilih berhalaman**: daftar yang bisa melebihi satu halaman memakai `AsyncSearchSelect paged` (pola katalog POS:
  `loadOptions(search, signal, page)` → `{options, hasMore}`, halaman berikutnya dimuat saat daftar digulir, tanpa
  duplikat, kembali ke halaman 1 saat kata kunci berubah). Pemilih barang Order Jual/Beli memakai
  `modules/products/lib/product-picker` (GET `/products` berhalaman); `search-options` tidak berhalaman (maks. 30).
  Label pilihan tersimpan yang belum tentu ada di halaman 1 diberikan lewat `selectedOption` (objek stabil).
- **Penempatan alur**: dialog ≤ 6 isian; `FormSheet` untuk CRUD master & aksi dokumen; halaman penuh untuk dokumen
  ber-baris barang (retur, surat jalan, penerimaan, transfer, jurnal manual: `…/new?from=<id>`).
- **State**: skeleton (bukan teks "Memuat…"), `ErrorState` dengan `message` API asli + "Coba lagi", Empty dengan aksi.
  Backend (Go) mengirim `null` untuk slice kosong: `DataTable.rows` menerimanya; gunakan `?? []` di tempat lain.
- **Satu `<h1>`** per halaman (dari `PageHeader`); breadcrumb ada di kerangka; halaman detail memanggil
  `usePageCrumb(nomor)`.
- **Kepadatan** (Ringkas/Standar/Lega) lewat `data-density` di `<html>`; pilihan di menu akun.
- **Cetak**: halaman `/print/*` tidak berubah (kertas fisik); `LEGACY_UA_CSS` memulihkan default browser.
  Nota, surat jalan, bukti pembayaran, dan struk POS dicetak sama dengan sistem lama (identitas perusahaan,
  ADR-0018 D12): rumus angka di `modules/print/lib/*`, format tanggal/terbilang di `modules/print/document.ts`
  (bukan `shared/lib/format`). Mengubah isi dokumen pelanggan = keputusan pemilik, bukan perbaikan UI.
- **Penjaga**: `npm run ui:ratchet` (angka pola lama per modul tidak boleh naik, plus **audit token** nol toleransi:
  hex/`rgb()`/`oklch()`/kelas palet Tailwind seperti `text-white` di mana pun di `src` selain `theme/` dan
  `modules/print`; pengecualian sah diberi `ui-ratchet-ignore` + alasan), ESLint melarang impor primitif dari modul.
- Tema gelap ditunda; komponen tetap hanya memakai token (termasuk `*-foreground` dan `--shadow-*`) agar kelak cukup
  menambah blok `.dark`.
