# Reports List — Modul 05 Product / Catalog

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Laporan yang dihasilkan modul ini
+ logika perhitungannya. Diverifikasi dari controller, service, seluruh halaman FE, dan E2E.

---

## 1. Keputusan: modul ini TIDAK punya laporan

Modul Product / Catalog **tidak menghasilkan satu pun laporan** — tidak ada endpoint reporting, tidak
ada halaman rekap, tidak ada ekspor CSV/PDF rekap, dan tidak ada agregasi terjadwal milik modul ini:

| Yang dicari | Hasil |
|---|---|
| Endpoint `reporting/*` milik produk | Tidak ada. Satu-satunya endpoint reporting di repo (`reporting/orders`, `reporting/stock`) milik modul 19 Reporting |
| Halaman rekap di `modules/products/` | Tidak ada (4 halaman: daftar, form, detail, kategori — semuanya operasional, bukan rekap) |
| Ekspor rekap (CSV/XLS rekap stok, rekap harga, kartu produk) | Tidak ada. Satu-satunya unduhan adalah **template import** (input, bukan output) dan **ZIP QR** (media operasional, bukan laporan) |
| Job agregasi (`metrics-job`, dsb.) | Tidak ada di modul ini |
| Test yang mengasumsikan laporan produk | Tidak ada |

Satu-satunya angka agregat yang tampil — `"{total} produk ditemukan."` di kartu daftar — adalah
`meta.total` dari query list (hitung baris non-arsip per filter), bukan laporan.

## 2. Data modul ini sebagai BAHAN laporan modul lain

Meski tidak punya laporan, tabel modul ini adalah dimensi/bahan untuk laporan modul lain. Kontrak
yang harus dijaga agar laporan-laporan itu tetap benar:

| Laporan (modul pemilik) | Field produk yang dipakai | Catatan kontrak |
|---|---|---|
| Stok & mutasi (16 Stock: `/stock`, `/stock/movements`, saldo awal) | `base_uom`, faktor konversi, `stock_tracked`, `has_variants`, `id_product_variant` | Saldo selalu dalam base; label tampil memakai faktor live |
| HPP & margin & laba (17 Finance) | `purchase_price`, `selling_price`, snapshot order-item | Harga beli kosong → status `"Menunggu data"` di Finance |
| Order & nota cetak (08–11 Order) | snapshot kode/nama/UOM/varian saat order dibuat | Perubahan master tidak mengubah dokumen lama |
| Dashboard `tracked_product_count` (18) | `stock_tracked=true` non-arsip | E2E 10 mengunci `> 0` setelah produk dibuat |
| Metrik operasional harian (19 Reporting job) | produk via order/stock | Tidak membaca modul ini langsung |
| Riwayat Aktivitas / Audit Log (20) | 10 `actionKey` modul ini | Satu-satunya "jejak tertulis" modul ini (lihat §3) |

## 3. Bahan mentah yang tersedia bila laporan produk ingin dibuat

Tanpa membangun apa pun, sistem baru bisa menyusun laporan produk dari sumber ini:

| Sumber | Isi | Batas yang harus diketahui |
|---|---|---|
| `POST products/list` (page/limit, maks 100/halaman) | Semua field + foto utama + varian + `meta.total` | Hanya non-arsip; perlu paginasi untuk sensus penuh (pola `listAllProducts` di slice: 100/halaman sampai total terpenuhi) |
| `POST product-categories/list` | Seluruh kategori aktif sekaligus | Tanpa paginasi/filter; arsip tak terlihat |
| `POST products/media/list` per produk | Foto + URL bertanda tangan (300 dtk) | Satu request per produk (N+1); URL kedaluwarsa |
| `audit_logs` (`product.*`, `product_category.*`, `product_media.*`, `product.bulk_import`) | Siapa + kapan + (create/update kategori: nama before/after) | Update produk hanya mencatat id+nama — **tidak bisa menjawab "harga berubah dari berapa ke berapa"** |
| Template + preview import | Definisi kolom operasional (`PRODUCT_HEADERS`, contoh, panduan) | Dokumen definisi, bukan data |

## 4. Yang eksplisit BUKAN laporan (jangan diperlakukan sebagai laporan saat rebuild)

- **ZIP QR** (`Download QR (ZIP)` / per baris): media operasional untuk cetak-tempel, tanpa angka,
  tanpa periode, tanpa agregasi. Kontraknya ada di feature-inventory F-08, bukan di sini.
- **Preview import** (tabel 8 produk + isu): validasi pra-simpan, datanya hilang saat modal ditutup.
- **Kartu Kondisi Stok** di detail: angka real-time dari modul Stock, bukan milik modul ini.
- **Kartu angka di modal bulk** (8 angka): hitungan draft file, bukan hitungan database.
