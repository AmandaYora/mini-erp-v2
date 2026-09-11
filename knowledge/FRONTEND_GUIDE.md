# Frontend Guide — mini-erp

Stack terkunci (React 19, Tailwind 4, react-router-dom, Zustand, Zod, Axios, `@/*`, lazy routes,
`frontend-design` skill sebagai otoritas UI) ada di skill `monorepo-standard` →
`references/frontend.md`, ditegakkan lewat `.claude/rules/frontend-react.md`. File ini hanya berisi
hal yang **spesifik proyek ini**.

## Rencana Modul Frontend (`apps/web/src/modules/`)

Struktur modul frontend tidak wajib 1:1 dengan 20 modul backend (sistem lama juga tidak 1:1 — 17
folder web memetakan ke 18 modul API). Saat sebuah modul backend mulai punya UI, buat folder
modul frontend yang menaunginya, mengikuti pengelompokan alami produk:

| Folder modul frontend (terbangun) | Menyentuh modul backend |
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

**Catatan:** backend sengaja memecah `order/` lama menjadi 6 modul terpisah (`purchasing`,
`sales`, `goodsreceipt`, `delivery`, `salesreturn`, `purchasereturn`) untuk memperbaiki masalah
maintainability sistem lama. Frontend **boleh** tetap mengelompokkan halaman-halaman itu di bawah
satu folder `orders/` bila alur UX-nya memang menyatu (mis. detail SO dan surat jalannya tampil di
satu halaman) — folder frontend adalah pengelompokan UX, bukan cermin wajib dari boundary backend.
Putuskan saat modul itu didesain, bukan sekarang.

## POS

POS bukan modul backend sendiri (lihat [../docs/SYSTEM_DESIGN.md §3](../docs/SYSTEM_DESIGN.md)) —
ia adalah halaman frontend yang memanggil endpoint `sales` (dengan `channel: "pos"`), `party`,
`product`, `stock`, dan `payment` langsung. Jangan menunggu modul backend "pos" — tidak akan ada.

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
