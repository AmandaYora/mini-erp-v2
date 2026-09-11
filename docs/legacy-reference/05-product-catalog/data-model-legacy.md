# Data Model Legacy — Modul 05 Product / Catalog

**Kelompok B — cukup dipahami maksudnya, TIDAK untuk ditiru strukturnya.** Menjelaskan skema tabel
& relasi yang dipakai modul ini setingkat konsep. Presisi kolom mengikuti entity + migrasi
(`001_baseline`, `005`, `008`, `027`, `035`, `036`).

---

## 1. Tabel milik modul (3 + 1 pakai-bersama)

### `product_categories` — pohon kategori per perusahaan

Konsep: satu baris = satu node kategori. Pohon dibentuk via `id_parent_category` yang menunjuk baris
sejenis (self-reference); `null` = root. Urutan tampil = `sort_order` lalu nama. Arsip = soft
(`archived_at`); tidak ada status aktif/nonaktif, tidak ada restore.

Kolom: `id_product_category` (PK) · `id_company` (FK companies) · `id_parent_category` (FK
self, nullable, **tanpa validasi di API**) · `code` (50) · `name` (255) · `sort_order` · timestamps.
Unik: `(id_company, code)`.

### `products` — master produk per perusahaan

Konsep: satu baris = satu produk jual/beli. Tiga kelompok isi: identitas (kode, nama, tipe, status,
kategori, atribut bebas, ringkasan), operasional stok (tiga satuan + dua faktor + flag pantau +
varian + min stok), harga (beli/jual/min-jual). Arsip = soft; tidak ada status selain
`active/inactive` dan tidak ada restore.

Kolom: `id_product` (PK) · `id_company` · `id_product_category` (nullable, FK) · `product_code` (50) ·
`product_name` (255) · `uom` legacy (30, selalu ditulis = base) · `product_type`
(physical/service/bundle/non_stock) · `status` (active/inactive) · `stock_tracked` · `has_variants` ·
`base_uom`/`purchase_uom`/`sales_uom` (30) · `purchase_to_base_factor`/`sales_to_base_factor`
(decimal presisi tinggi) · `min_stock_qty` (nullable; null untuk non-stok) ·
`purchase_price`/`selling_price`/`min_selling_price` (`DECIMAL(18,2)`, nullable) · `attributes_json`
(JSON bebas) · `summary_text` · timestamps + `archived_at`. Unik: `(id_company, product_code)`.

Evolusi (jujur dari migrasi): baseline (kode/nama/tipe/status/`uom` tunggal/`standard_price` tunggal)
→ 005 (3 harga terpisah; `standard_price` ditinggalkan di DB, disalin ke `selling_price`) → 008
(dual-UOM + snapshot UOM di `order_items`) → 035 (`uom` tetap writable + backfill) → 036
(`has_variants` + tabel varian + kolom `id_product_variant` di 10 tabel transaksional + unique saldo
berubah ke `(branch, variant, location)`).

### `product_variants` — pilihan stok per produk

Konsep: satu baris = satu pilihan (warna/ukuran/dll.) yang punya saldo sendiri. Setiap produk punya
≥1 baris: produk tanpa varian terlihat punya 1 baris default tersembunyi (`Standar`,
kode=nama=barcode=kode produk). Keunikan **hanya di level aplikasi** (unique index sengaja
diturunkan menjadi non-unique `KEY` di migrasi 036 — dua baris kode sama masih mungkin bila guard
lolos, lalu DB melempar 1062 yang diterjemahkan jadi pesan duplikat).

Kolom: `id_product_variant` (PK) · `id_company` · `id_product` (FK products) · `variant_code` (80) ·
`variant_name` (255) · `attributes_json` (selalu `{}` dari alur modul ini) · `barcode` (80, nullable)
· `is_default` · `is_hidden` · `status` (active/inactive) · `sort_order` · timestamps + `archived_at`.
Index: `(company, variant_code)`, `(product, archived, status, sort)`, `(company, barcode)`.

### `media_files` (milik modul Storage, dipakai modul ini)

Konsep: metadata file generik; modul ini memakai irisan `owner_type='product'` +
`purpose='product_image'` + `owner_id=id_product`. Byte file milik Storage (lokal/S3); yang disimpan
di sini hanya kunci objek + metadata optimasi + flag utama/urutan. Arsip = soft; tidak ada restore.
Kontrak penuh ada di analisis lapisan shared; di sini hanya relevansi produk (lihat §3).

## 2. Relasi (konsep)

```
companies 1──* product_categories (self-tree via id_parent_category)
companies 1──* products ──* product_variants (1 default tersembunyi bila tanpa varian terlihat)
products *──1 product_categories (nullable; tanpa jaminan target non-arsip)
products 1──* media_files (owner product/product_image; 0..1 utama)
products 1──* order_items / goods_receipt_items / delivery_note_items (snapshot, bukan join live)
product_variants 1──* inventory_balances / movements / reservations / transfer_items /
                     cost_movements / journal_lines (kolom id_product_variant, NOT NULL pasca-036)
```

Semua relasi perusahaan diisolasi manual per query (`id_company` dari sesi — tidak ada RLS/filter
global). Tidak ada cascade delete (semuanya soft). FK kategori→produk tidak mencegah arsip kategori
yang masih dirujuk.

## 3. Kebutuhan data per fitur (jejak baca-tulis)

| Fitur | Baca | Tulis |
|---|---|---|
| Daftar/cari/detail produk | products (+kategori via join/relasi) + media utama + varian | — |
| search-options (picker) | products kolom ringan + varian (+ agregat `inventory_balances` bila prefer-stok) | — |
| Create/update produk | cek duplikat kode (produk + varian), cek movement (kunci UOM), cek saldo (kunci varian/arsip) | products + varian (transaksi) + audit |
| Arsip produk | jumlah `on_hand` seluruh cabang | `archived_at` + audit |
| Kategori | seluruh kategori aktif (tanpa paginasi) | baris kategori + audit (tanpa transaksi eksplisit) |
| Media | media per produk + signed URL Storage | baris media (transaksi) + byte Storage + audit |
| Import | seluruh kategori/produk/varian perusahaan (untuk resolusi kode) | kategori (1 txn) + produk (chunk 200 + fallback) + varian (1 txn) + 1 audit |

## 4. Anomali & catatan presisi (untuk arsitek sistem baru, bukan untuk ditiru)

1. **Dua presisi desimal hidup bersamaan**: entity `Product` mendeklarasikan faktor `DECIMAL(24,12)`
   sementara migrasi 008 membuatnya `DECIMAL(18,4)` tanpa migrasi lanjutan yang mengubahnya.
   **[PERLU KONFIRMASI]** nilai live yang benar (periksa `SHOW COLUMNS` di production) sebelum
   menetapkan presisi baru.
2. **`DATETIME(3)` baseline vs presisi 6 di entity** (pola sama seperti temuan modul 01): tabel
   `product_categories`/`products` lahir dengan `(3)`, entity meminta `(6)`. **[PERLU KONFIRMASI]**
   seperti butir 1.
3. **`standard_price` mati tapi ada**: kolom baseline tetap di DB, tidak dibaca/ditulis aplikasi.
   Buang di skema baru (dengan migrasi data `selling_price` yang sudah berjalan sejak 005).
4. **Unik varian non-unique di DB**: keunikan ditanggung aplikasi; dua request bersamaan masih bisa
   lolos ganda (tidak ada test konkurensi).
5. **`id_parent_category` tanpa jaminan**: API menerima id apa pun (termasuk milik perusahaan lain
   atau tak ada) — tree bisa menunjuk ke/eventual `null` diam-diam di FE (`?.`).
6. **Atribut bebas tanpa skema**: `attributes_json` map string→string; tidak ada daftar kunci sah,
   tidak ada tipe selain string.
7. **Dua pemilik kebenaran satuan**: `uom` vs tiga kolom baru (selalu disamakan saat simpan, tetapi
   dua kolom tetap ada — pola seakar KI-42).
8. **Arsip tanpa restore di seluruh tabel modul** — satu-satunya modul master tanpa restore
   (Business Party punya). Keputusan skema baru: tambah restore atau kunci satu-arah secara sadar.
