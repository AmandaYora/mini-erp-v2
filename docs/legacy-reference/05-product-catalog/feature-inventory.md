# Feature Inventory — Modul 05 Product / Catalog

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 16 — `product-categories/{list,create,update,archive}` (4), `products/{list,search-options,detail,create,update,archive}` (6), `products/media/{list,upload,archive,set-primary}` (4), `product-imports/{preview,commit}` (2) |
| Halaman | 5 rute: `/products`, `/products/create`, `/products/:productId`, `/products/:productId/edit`, `/product-categories` (4 komponen halaman) |
| Menu sidebar | 1, grup **"Produk & Mitra"**, label **"Produk"** (`/products`, izin `product.view`) |
| Permission | `product.view` (lihat + cari), `product.create` (tambah + import), `product.update` (ubah + kelola foto), `product.archive` (arsip produk + arsip kategori) |
| Tabel yang dimiliki | `products`, `product_categories`, `product_variants` |
| Tabel yang ikut ditulis | `media_files` (`owner_type='product'`, `purpose='product_image'`) |
| Laporan | Tidak ada laporan milik sendiri — lihat [reports-list.md](reports-list.md) |
| Penomoran dokumen | **Tidak ada** — semua kode diisi manual. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | 10 `actionKey`: `product.create`, `product.update`, `product.archive`, `product_category.create`, `product_category.update`, `product_category.archive`, `product_media.upload`, `product_media.archive`, `product_media.set_primary`, `product.bulk_import` |
| Aksi yang TIDAK ada | Pulihkan (restore) produk/kategori/varian yang diarsipkan, hapus permanen, ubah urutan foto, ubah kode kategori, kelola varian tanpa lewat form produk |

**Catatan lingkup.** Modul ini adalah **master data yang dibaca hampir semua modul transaksional**:
Order mengambil snapshot produk saat order dibuat, Stock memakai `base_uom` + faktor konversi sebagai
sumber kebenaran saldo, Finance memakai harga beli sebagai basis HPP, POS/picker memakai
`products/search-options`. Karena itu guard modul ini (kunci UOM setelah ada movement, stok wajib nol
sebelum arsip, harga beli wajib untuk produk berstok) adalah **penjaga integritas lintas modul**,
bukan sekadar validasi form. Mengubah/melonggarkan guard ini merusak modul Order, Stock, dan Finance
sekaligus — lihat [business-rules.md](business-rules.md) §8.

Yang **bukan** bagian modul ini: saldo/stok per gudang (modul 16 Stock), harga khusus member
(modul 07 Member Pricing — modul ini hanya menyimpan `selling_price`/`min_selling_price` standar),
penentuan cabang aktif (modul 04 Branch; modul ini hanya memakai `id_company` dari sesi +
`id_branch` sesi untuk pengurutan stok).

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar Produk (`/products`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Tampilkan produk per halaman 20 | Paginasi server (`page`, `limit: 20` tetap di FE); teks kartu `"{total} produk ditemukan."` atau `"Memuat…"` saat memuat |
| F-01.2 | Cari kode/nama (debounce 400ms → URL `search`) | Pencarian tokenized per-kata di server, tahan spasi ganda & urutan kata (mis. `budi santoso` menemukan `Santoso Budi`) |
| F-01.3 | Filter kategori | Dropdown hierarki (`ProductCategorySelect`, mode filter); tersimpan di URL `category_id` |
| F-01.4 | Filter tipe | Semua tipe / Barang fisik / Jasa / Bundle / Non-stock (`item_type`) |
| F-01.5 | Filter status | **Default `active`**; pilihan Aktif / Nonaktif / Semua (`status`; `all` = tidak dikirim) |
| F-01.6 | Filter pantau stok | Semua / Dipantau / Tanpa stok (`stock_tracked=true/false`) |
| F-01.7 | Urutan stok-tersedia-dulu | Selalu dikirim `prefer_stock_available: true`: produk berstok-tersedia di cabang aktif naik ke atas, produk berstok-habis turun ke bawah; non-stok/jasa di tengah |
| F-01.8 | Kolom Produk (foto + nama + kode) | Thumbnail 44px dari foto utama (atau gambar default bila belum ada foto) |
| F-01.9 | Kolom Kategori | Nama kategori dari data workspace; `"—"` bila tanpa kategori |
| F-01.10 | Kolom Tipe | Badge: Barang fisik / Jasa / Bundle / Non-stock |
| F-01.11 | Kolom Harga Beli | `formatCurrency(purchasePrice)` + sub-baris `/ {purchaseUom}` |
| F-01.12 | Kolom Harga Jual | Harga normal + sub-baris `Min {minSellingPrice} / {salesUom}` |
| F-01.13 | Kolom Stok | Produk berstok: stok tersedia + `Min {minStockQty}`; bila `baseUom ≠ salesUom` tampil ganda (`5 meter (0.5 roll)`); produk non-stok: teks redup `"Tidak dipantau"`; sebelum data siap: `"Memuat stok..."` |
| F-01.14 | Kolom Status | Badge hijau **Aktif** / kuning **Nonaktif** |
| F-01.15 | Tombol QR per baris | 1 target → buka modal QR; >1 target (punya varian aktif terlihat) → unduh ZIP `{kode}-QR-Varian.zip` |
| F-01.16 | Tombol Detail per baris | Ke `/products/:id` |
| F-01.17 | Tombol "Download QR (ZIP)" | Ekspor **seluruh** produk (paginasi 100/halaman) ke ZIP per folder kategori |
| F-01.18 | Tombol scan QR/barcode | Buka `QRScannerModal`; hasil scan mengisi kotak cari (bukan navigasi langsung) |
| F-01.19 | Tombol "Kelola Kategori" | Selalu tampil (halaman kategori sendiri hanya butuh `product.view`); ke `/product-categories` |
| F-01.20 | Tombol "Upload Bulk" + "Tambah Produk" | Hanya bila `product.create` |
| F-01.21 | Keadaan kosong | `EmptyState` **"Daftar produk kosong"** / `"Belum ada produk yang cocok dengan filter ini."** + tombol Tambah (bila berhak) |
| F-01.22 | Stok per halaman | Setelah daftar dimuat, FE memanggil `stock/balances` untuk id produk berstok di halaman itu; gagal → sel kolom memakai agregat workspace, lalu `"Memuat stok..."` |

### F-02 — Tambah / Edit Produk (`/products/create`, `/products/:productId/edit`, satu komponen)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Pilih tipe lebih dulu | Dropdown Tipe Produk: Barang fisik / Jasa / **Bundle ringan** / Non-stock; mengganti tipe menyembunyikan/menampilkan seksi form **tanpa menghapus draft** (draft satuan fisik dijaga saat pindah tipe lalu kembali — ada test khusus) |
| F-02.2 | Informasi Dasar | Kode produk* (wajib, teks bebas), Nama produk* (wajib), Kategori (dropdown hierarki, mode assignment: **kategori induk non-leaf disabled** dengan alasan `"Pilih sub-kategori paling bawah"`; kosong = `"Tanpa kategori"`), Tipe, Status (Aktif/Nonaktif, default Aktif) |
| F-02.3 | Foto | Mode tambah: **satu** field foto awal (pilih file + pratinjau lokal, jadi foto utama setelah simpan; gagal upload → tetap pindah ke halaman detail); mode edit: panel media penuh (upload banyak + jadikan utama + arsipkan) |
| F-02.4 | Seksi "Stok & Satuan" (fisik) | Checkbox `"Pantau stok produk ini"` (default centang) + hint `"Jika aktif, produk akan muncul di modul stok dan bisa memiliki minimum stok."`; field: Satuan stok*, Satuan beli, Konversi beli ke stok (`1 box = 12 pcs → isi 12`), Satuan jual, **Isi jual per stok** (bahasa operasional: `1 roll = 10 meter → isi 10`), **Dampak jual ke stok** (nilai teknis, tersinkron dua arah otomatis), Minimum stok (hanya bila dipantau) |
| F-02.5 | Seksi transaksi (non-fisik) | Satu field satuan (label per tipe: `"Satuan layanan"` / `"Satuan bundle"` / `"Satuan transaksi"`) + catatan operasional (`"Jasa tidak dipantau sebagai stok gudang."` / `"Bundle tidak dipantau sebagai stok gudang pada tahap ini."` / `"Produk ini dipakai untuk transaksi, tetapi tidak memengaruhi saldo stok."`) |
| F-02.6 | Seksi "Varian Stok" (hanya bila dipantau) | Checkbox `"Produk ini memiliki varian stok"` + hint `"Aktifkan hanya jika warna, ukuran, atau pilihan lain memiliki saldo stok sendiri."`; daftar draft varian (Nama* + Kode opsional/auto + Barcode opsional + Status Aktif/Nonaktif + Hapus); tombol **Tambah Varian**; bila mati: teks `"Produk disimpan dengan varian default internal yang tidak tampil ke user."` |
| F-02.7 | Seksi "Harga & Ringkasan" | Harga beli (placeholder dinamis: `"Wajib untuk produk berstok — dasar perhitungan HPP"` bila dipantau, else `"Harga referensi dari supplier atau vendor"`), Harga jual normal (`"Harga yang ditawarkan ke customer"`), Harga jual minimum (`"Batas bawah harga nego"`), Ringkasan operasional (textarea, `"Tulis konteks bisnis singkat produk ini."`) |
| F-02.8 | Seksi "Atribut Produk" | Pasangan Nama (dropdown saran dari kunci yang sudah dipakai produk lain + boleh ketik baru) + Keterangan + **Hapus** (merah); tombol **"+ Tambah Atribut Baru"**; baris kosong-nama tidak dikirim |
| F-02.9 | Validasi pra-kirim di FE | Satuan stok wajib (fisik); faktor konversi > 0; harga beli > 0 untuk produk **baru** berstok; satuan transaksi wajib (non-fisik); ≥1 varian bernama bila varian aktif — masing-masing dengan toast `warning` spesifik (lihat §4) |
| F-02.10 | Tombol Batal / Simpan | Batal ke detail (edit) atau daftar (tambah); submit: `"Menyimpan..."` → `"Simpan Produk"` / `"Simpan Perubahan"`; sukses → pindah ke `/products/:idBaru`; error → toast, tetap di form |
| F-02.11 | Edit produk di luar 100 pertama | Form mengambil `products/detail` dari server bila produk tidak ada di store (tidak jatuh ke mode tambah — kontras dengan modul Users KI-14) |

### F-03 — Detail Produk (`/products/:productId`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Header | Breadcrumb `Dashboard > Produk > {kode}`; judul nama; deskripsi = ringkasan; tombol **← Kembali**, **Lihat Stok** (hanya berstok), **Penyesuaian Stok** (hanya berstok + `stock.adjust`, membawa `?item_id=&returnTo=`), **Edit** (`product.update`), **Arsipkan** (`product.archive`, merah) |
| F-03.2 | Kartu Informasi Dasar | Kode, Kategori (`"-"` bila tanpa), Tipe, Harga Beli / {purchaseUom} (`"-"` bila null), Harga Jual Normal, Harga Jual Minimum, Satuan Stok + Satuan Beli (`1 box = 12 pcs`) + Satuan Jual (`1 roll = 10 meter` + sub-baris `"Jual 1 meter mengurangi 0.1 roll"` bila faktor ≠ 1) untuk fisik — atau satu baris satuan transaksi + Catatan Operasional untuk non-fisik; Status Aktif/Nonaktif |
| F-03.3 | Kartu Foto Produk | `ProductMediaPanel` (lihat F-05) |
| F-03.4 | Kartu Atribut Produk | Daftar pasangan kunci-nilai; kosong → notice info `"Produk ini belum memiliki atribut tambahan."` |
| F-03.5 | Kartu Kondisi Stok (hanya berstok) | 4 metrik: On hand, Reserved, Available, Minimum — dual-UOM bila satuan beda (`5 meter (0.5 roll)`) |
| F-03.6 | Dialog arsip | Judul **"Arsipkan produk"**, deskripsi `"Produk {nama} akan dipindahkan ke status nonaktif dan tidak muncul lagi di daftar aktif."`, tombol **"Tahan dulu"** (batal) / **"Ya, arsipkan"** (merah); sukses → kembali ke `/products` **tanpa toast sukses** |
| F-03.7 | Produk tidak ditemukan | Notice merah `"Produk tidak ditemukan"` / `"Produk yang Anda cari tidak tersedia pada context perusahaan saat ini."`; saat memuat: notice biru `"Memuat produk"` |

### F-04 — Kategori Produk (`/product-categories`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Pohon hierarki | Urut `sortOrder` lalu nama; root bergaris kiri tebal bermerek, anak menjorok 20px/level + label `Level {n}`; tombol lipat `v` / `>` per induk; tiap node: nama + badge kode + tombol **Edit** (kecil, bila `product.update`) |
| F-04.2 | Form tambah/edit | Kategori Induk (dropdown hierarki; saat edit, diri sendiri + seluruh keturunannya **dikecualikan** dari pilihan; kosong = `"-- Kategori Utama (tanpa induk) --"` + helper `"Kosongkan untuk menjadikan kategori utama pada level 0."`), Nama kategori*, Kode kategori* |
| F-04.3 | Tanpa arsip/hapus di UI | Tidak ada tombol arsip maupun hapus kategori di halaman ini (arsip hanya via API langsung) |
| F-04.4 | Mode baca-saja | Tanpa `product.create`/`product.update`: form diganti teks `"Anda hanya memiliki akses lihat untuk kategori produk."` |
| F-04.5 | Keadaan kosong | `"Belum ada kategori"` / `"Tambahkan kategori pertama menggunakan form di sebelah kanan."` |
| F-04.6 | Kode editable tapi diabaikan server | Field Kode terisi saat edit dan bisa diubah, tetapi endpoint update **tidak menerima kode** — perubahan kode hilang diam-diam (→ KI-51) |

### F-05 — Foto Produk (media)

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Upload | Input file `accept="image/jpeg,image/jpg,image/png,image/webp"`; kompresi di FE dulu, lalu optimasi backend ke WebP; batas 5MB (multer); tanpa file → `400 'Foto produk wajib diunggah'`; tipe tak didukung → `400 'Hanya file gambar JPG, PNG, atau WebP yang diizinkan'` |
| F-05.2 | Foto pertama otomatis utama | `is_primary` tak dikirim → foto pertama produk jadi utama; berikutnya jadi galeri kecuali diminta utama |
| F-05.3 | Urutan | `sort_order` tak dikirim → maks saat ini + 1; **tidak ada endpoint ubah urutan** (urutan beku sejak upload → KI-59) |
| F-05.4 | Jadikan Utama | Satu klik; dalam satu transaksi: semua foto lain diturunkan, yang dipilih naik; audit `product_media.set_primary` |
| F-05.5 | Arsipkan foto | Soft-delete; bila yang diarsipkan foto utama, foto tersisa paling awal otomatis naik jadi utama |
| F-05.6 | Gambar default | Produk tanpa foto utama menampilkan gambar default perusahaan (`buildProductDefaultImageKey`) ber-badge **"Default"** di panel + thumbnail daftar; list/detail selalu mengembalikan `primaryImageKey/Url` (tidak pernah kosong) |
| F-05.7 | URL bertanda tangan | Semua URL foto adalah signed URL berumur 300 detik (lihat test `expiresIn: 300`); tidak ada URL publik permanen |
| F-05.8 | Foto produk arsip tak tersentuh | Seluruh endpoint media menolak bila produk sudah diarsip (`404 'Produk tidak ditemukan'`) |

### F-06 — Upload Bulk Excel (`ProductBulkUploadModal` dari halaman daftar)

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Unduh template | File `template-upload-produk.xlsx`, 5 sheet: Kategori Produk, Daftar Produk, Varian Produk, Info Tambahan Produk, **Panduan** (diabaikan saat upload); header wajib biru, contoh 1 baris, lebar kolom + autofilter + format angka |
| F-06.2 | Pilih/Ganti file + Cek File | Input `.xlsx,.xls` tersembunyi; status badge: **Belum ada file** → **File dipilih** → **Siap diproses** / **Perlu diperbaiki** / **Import selesai**; tombol tutup/klik-luar/Escape diblokir saat sibuk |
| F-06.3 | Preview (tidak menyimpan) | Ringkasan `"{n} produk ditambahkan, {n} produk diperbarui, {n} varian ditambahkan, {n} varian diperbarui, {n} kategori ditambahkan, {n} kategori diperbarui, {n} error"` + 8 kartu angka + tabel 12 isu pertama (Status/Bagian/Baris/Kolom/Keterangan) atau tabel 8 produk + 8 varian pertama bila bersih |
| F-06.4 | Proses Import (terkunci bila `!ok` atau sudah selesai) | Commit idempoten per kode; hasil: banner hijau `"Import selesai. {rekap}"`; baris gagal: banner merah `"{n} baris gagal diimpor. Baris lain tetap masuk. Perbaiki baris di bawah lalu upload ulang file — data yang sudah masuk tidak akan dobel (dicocokkan per kode)."` + tabel 50 kegagalan pertama |
| F-06.5 | Lanjut Isi Stok Awal | Setelah import selesai, tombol ke `/stock/opening` |
| F-06.6 | Aturan main file | Kategori+Produk wajib ada sebagai sheet; alias nama sheet Inggris/Indonesia; header fleksibel (banyak alias per kolom); angka fleksibel (`Rp 12.500`, `12,500`, `12.5`); kode dinormalkan (huruf kecil + spasi tunggal) untuk deteksi duplikat; baris kosong dilewati; nomor baris = nomor baris Excel (header = baris 1) |

### F-07 — Varian Stok

| # | Sub-fitur | Detail |
|---|---|---|
| F-07.1 | Varian default tersembunyi | Setiap produk tanpa varian terlihat punya 1 varian `Standar` (`is_default=true`, `is_hidden=true`, kode=nama=barcode=kode produk, status mengikuti produk); dibuat otomatis saat create produk maupun saat import; diarsipkan otomatis saat varian terlihat pertama diaktifkan |
| F-07.2 | Kode varian | Manual (disimpan huruf besar) atau otomatis `{KODEPRODUK}-{SLUG(NAMA)}`; unik per perusahaan (tidak boleh sama dengan kode produk lain, kode/barcode varian lain; cek case-insensitive); duplikat dalam satu simpan ditolak |
| F-07.3 | Barcode varian | Opsional; tidak boleh sama dengan kode produk/varian/barcode lain; kunci scan operasional (lihat F-08) |
| F-07.4 | Status + urutan | Aktif/Nonaktif per varian; menonaktifkan atau menghapus varian berstok ditolak; `sort_order` default = urutan baris |
| F-07.5 | Batas produk varian | Hanya produk fisik berstok; produk varian tanpa ≥1 varian aktif-terlihat ditolak; mengaktifkan varian saat stok masih di varian default ditolak; menonaktifkan varian terakhir saat stok masih ada ditolak |
| F-07.6 | Import varian | Sheet Varian Produk: Kode Produk (wajib ada di sheet Daftar Produk **file yang sama**), Kode Varian, Nama Varian, Status, Urutan; produk file otomatis ditandai `hasVariants` |

### F-08 — QR Produk

| # | Sub-fitur | Detail |
|---|---|---|
| F-08.1 | Target operasional | Produk bervarian aktif-terlihat → **satu QR per varian** (`kode = variantCode`, `nama = "{produk} - {varian}"`); selain itu → QR kode produk. QR induk **tidak** dibuat untuk produk bervarian |
| F-08.2 | Isi QR | Payload = kode mentah (kode produk atau kode varian); hasil scan dipakai untuk mencari (daftar: mengisi pencarian; POS: mengisi input barcode) |
| F-08.3 | Gambar QR | Kanvas 600×800: QR 480px margin 2 di atas, nama (tebal 34px, wrap, maks 520px) di tengah, kode (monospace 28px abu) di bawah; PNG |
| F-08.4 | Unduhan massal | ZIP berisi PNG per target dalam folder nama kategori (`"Tanpa Kategori"` bila tanpa kategori); nama file `{kode}_{nama}.png` disanitasi (karakter ilegal → `-`, maks 50/70/80, duplikat → sufiks `_2`, ...); nama default `Produk-QR-Codes.zip`; kosong-valid → error `"Tidak ada produk dengan kode yang valid untuk diekspor."` |
| F-08.5 | Modal QR tunggal | Pratinjau + unduh satu PNG (nama `{kode}_{nama 30 char}.png`) |

### F-09 — Picker Produk lintas modul (`ProductSearchSelect` + `products/search-options`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-09.1 | Dropdown async | Mengetik mencari ke server (`products/search-options`); label opsi `"{kode} - {nama}"`, deskripsi `"Satuan stok: {baseUom}"` (berstok) / `"Satuan: {baseUom}"`; default placeholder `"Cari kode atau nama produk..."` |
| F-09.2 | Peringkat relevansi | Kode persis (0) → awalan kode (1) → awalan nama (2) → lainnya (3); lalu stok-tersedia; lalu nama, kode |
| F-09.3 | Filter bawaan | Default `status=active`, `prefer_stock_available=true`, `limit=20` (maks 30); opsi `productType`, `stockTrackedOnly` per pemakai |
| F-09.4 | Resolve nilai tersimpan | Nilai id tersimpan di-resolve via `id_product` + `limit: 1` tanpa media/URL |
| F-09.5 | Pemakai terverifikasi | Baris order (E2E 15), penyesuaian stok (E2E 15), dan seluruh `ProductSearchSelect` di modul lain; dipakai juga untuk grid POS (E2E 17 memverifikasi urutan stok-tersedia-dulu di `products/list`) |

---

## 3. Edge Case (52)

| # | Kondisi | Perilaku (dari kode) |
|---|---|---|
| E-01 | Kode produk duplikat (create) | `409 "Kode produk '{kode}' sudah digunakan"` |
| E-02 | Kode produk diganti ke milik produk lain (update) | `409` sama; ganti ke kode sendiri (setelah trim) → lolos tanpa cek |
| E-03 | Kode produk diganti ke kode/barcode varian | `409 "Kode produk '{kode}' sudah digunakan sebagai kode atau barcode varian"` — kecuali varian default tersembunyi milik produk itu sendiri |
| E-04 | Kode varian diganti/tambah menabrak kode produk | `409 "...sudah digunakan sebagai kode produk"` / `"...sebagai Kode Produk"` (import) |
| E-05 | Kode kategori duplikat (create) | `409 "Kode kategori '{kode}' sudah digunakan"` (hanya vs non-arsip; arsip → lolos lalu berisiko 500 di unique key → bagian dari KI-50) |
| E-06 | Kode produk/kategori berspasi di create | **Tidak di-trim** (hanya update produk yang trim) → `" PRD-1"` tersimpan apa adanya dan lolos cek duplikat → KI-50 |
| E-07 | Nama produk berspasi ganda | Diciutkan (`normalizeWhitespace`) di create, update, dan import |
| E-08 | Tipe tak dikenal (API) | `400 'Tipe produk tidak valid'` |
| E-09 | Produk non-fisik tanpa satuan | `400 'Satuan transaksi wajib diisi'` |
| E-10 | Produk fisik tanpa `base_uom` | `400 'Satuan stok (base_uom) wajib diisi'` (FE: toast warning sebelum kirim) |
| E-11 | Faktor konversi 0/negatif/bukan angka | `400 '{field} harus lebih besar dari 0'` (FE: toast `"Konversi satuan belum valid"`) |
| E-12 | Produk berstok baru tanpa harga beli | `400 'Harga beli wajib diisi (lebih dari 0)...'` (FE: toast `"Harga beli belum diisi"`; import: isu error per baris) |
| E-13 | Produk berstok diedit harga belinya jadi 0/null | **Lolos** — guard hanya di create & import (→ KI-52) |
| E-14 | Ubah base_uom/faktor setelah ada movement | `400 'UoM atau faktor konversi tidak dapat diubah...'`; ubah **label** satuan beli/jual saja → lolos |
| E-15 | Ubah produk berstok jadi non-fisik | `400 'Produk yang sedang dilacak stoknya tidak bisa diubah menjadi tipe non-stok...'` |
| E-16 | Ubah produk fisik non-stok jadi non_stock | Lolos; satuan diseragamkan, `minStockQty` di-null-kan |
| E-17 | Aktifkan varian saat stok > 0 di default | `400 'Produk masih punya stok pada varian default...'` (batas 0.0001) |
| E-18 | Nonaktifkan varian terakhir saat stok varian > 0 | `400 'Produk masih punya stok di varian aktif...'` |
| E-19 | Simpan varian tanpa baris bernama | `400 'Minimal satu varian wajib diisi'` (baris nama-kosong dibuang dulu) |
| E-20 | Hapus/nonaktifkan varian berstok | `400 'Varian masih punya stok...'` |
| E-21 | Arsip produk berstok | `400 'Produk masih punya saldo stok {qty}...'` (batas 0.0001; memakai `on_hand`, **bukan** available → stok ter-reservasi penuh tetap bisa diarsip — [PERLU KONFIRMASI]) |
| E-22 | Arsip produk bersejarah movement tapi stok nol | Lolos (riwayat tetap; soft-delete) |
| E-23 | Arsip kategori yang masih dipakai produk | **Lolos tanpa cek** — produk menunjuk kategori arsip; daftar kategori tak lagi menampilkannya |
| E-24 | Buat kategori dengan induk tak dikenal | **Lolos** — `id_parent_category` disimpan mentah tanpa validasi (→ KI-54) |
| E-25 | Update kategori: parent tak dikenal / siklus / kode diubah | Parent tak divalidasi, siklus tak dicek, **kode diabaikan** (→ KI-51, KI-54) |
| E-26 | Buka edit produk tak dikenal / arsip | Form: notice `"Produk tidak ditemukan"`; API: `404 'Produk tidak ditemukan'` / `'Kategori tidak ditemukan'` |
| E-27 | Produk arsip dibuka / difoto / diimport-ulang kodenya | Detail/list/search tak menemukannya; seluruh endpoint media `404`; import kode-arsip → isu error `"ada di data arsip. Pulihkan ... terlebih dahulu"` — padahal **tidak ada endpoint pulihkan** (→ KI-56) |
| E-28 | `limit` list 0/negatif/bukan angka | Diteruskan ke query (hanya batas atas 100 dijaga) → daftar kosong atau 500 (→ KI-55, seakar KI-26) |
| E-29 | `limit` search-options > 30 / < 1 | Dijepit ke [1, 30]; fetch `limit+1` untuk `has_more` |
| E-30 | `prefer_stock_available` tanpa cabang aktif | Diabaikan (perlu `idBranch` sesi) |
| E-31 | Cari multi-kata / spasi ganda | Dipecah token AND, tahan urutan & spasi (ada test `SPECIAL  1 KG`) |
| E-32 | Upload tanpa file | `400 'Foto produk wajib diunggah'` (produk) / `'File Excel wajib diunggah'` (import) |
| E-33 | Upload > 5MB | Ditolak multer (limit `5 * 1024 * 1024`) |
| E-34 | Upload bukan gambar | `400 'Hanya file gambar JPG, PNG, atau WebP yang diizinkan'`; FE fallback `"Pastikan file JPG, PNG, atau WebP maksimal 5MB"` |
| E-35 | `is_primary`=`"1"` / `"true"` / tak dikirim | `"1"`/`"true"` → utama; tak dikirim → utama hanya bila foto pertama |
| E-36 | Arsip foto utama | Foto tersisa paling awal otomatis jadi utama |
| E-37 | Arsip/set-utama foto tak dikenal atau milik produk lain | `404 'Foto produk tidak ditemukan'` (dicek dalam lingkup produk + perusahaan) |
| E-38 | Import: sheet wajib hilang | Isu error `"Sheet "Kategori Produk" wajib ada."` / `"Sheet "Daftar Produk" wajib ada."`; keduanya dicek, lalu validasi berhenti |
| E-39 | Import: file bukan `.xlsx/.xls` | `400 'File harus berformat Excel .xlsx atau .xls'` |
| E-40 | Import: produk tanpa harga beli (berstok) | Isu error per baris (pesan sama dengan create) → commit diblokir |
| E-41 | Import: kolom inline `Info tambahan:`/`Atribut:` | Isu **error** per sel (harus pindah ke sheet Info Tambahan) → commit diblokir (→ KI-60) |
| E-42 | Import: varian menunjuk produk yang hanya ada di DB (tidak di file) | Isu error — varian **wajib** merujuk baris sheet Daftar Produk file yang sama |
| E-43 | Import: commit dengan error apa pun | `400 'File belum bisa diproses karena masih ada {n} data yang perlu diperbaiki...'` |
| E-44 | Import: gagal di tengah jalan | Kategori gagal → seluruh kategori gagal (1 transaksi); produk gagal per chunk → fallback per baris, baris baik tetap masuk + tiap baris gagal tercatat (sheet+baris+alasan); varian gagal → seluruh varian gagal |
| E-45 | Import: file yang sama diupload ulang | Idempoten per kode ternormalkan → update, tidak dobel; `hasVariants` produk tidak pernah dimatikan oleh import |
| E-46 | Import: angka `Rp 12.500` / `12,500` / `1.000.000` | Diparsing fleksibel (id/en); tak terparsing → isu `"Kolom "{kolom}" harus berisi angka."` + fallback (1 untuk faktor, null untuk opsional) |
| E-47 | Import: tipe/status/boolean dengan ejaan lain | Sinonim luas (lihat business-rules BR-31); tak dikenal → isu + fallback (physical / active / `true` untuk pantau stok) |
| E-48 | Varian: nama kosong | Baris dibuang diam-diam (tidak dihitung, tidak error) |
| E-49 | Varian: kode kosong | Otomatis `{KODEPRODUK}-{SLUG(NAMA)}` huruf besar |
| E-50 | Produk tanpa varian dihapus variannya lewat update | Varian non-default diarsipkan (dengan cek stok); varian default dipertahankan/dibuat ulang |
| E-51 | Kategori file merujuk induk yang juga baru di file | Dibuat rekursif induk-dulu dalam 1 transaksi; siklus → isu `"Hierarki kategori membentuk putaran: A > B > A."` |
| E-52 | Status produk `inactive` | Tetap muncul di list bila filter Status = Nonaktif/Semua; picker default (`status=active`) menyembunyikannya; varian default mengikuti status produk |

---

## 4. Katalog Pesan & Toast (teks apa adanya)

**Error API (Indonesia):** `'Tipe produk tidak valid'` · `'Satuan transaksi wajib diisi'` ·
`'Satuan stok (base_uom) wajib diisi'` · `'{purchase_to_base_factor|sales_to_base_factor} harus lebih besar dari 0'` ·
`'Harga beli wajib diisi (lebih dari 0) untuk produk yang dilacak stoknya — dibutuhkan untuk menghitung HPP saat barang terjual.'` ·
`'Produk yang sedang dilacak stoknya tidak bisa diubah menjadi tipe non-stok. Arsipkan lalu buat produk baru.'` ·
`'UoM atau faktor konversi tidak dapat diubah karena produk sudah punya riwayat pergerakan stok. Buat produk baru jika konversi berbeda.'` ·
`'Produk masih punya stok pada varian default. Kosongkan atau split stok terlebih dahulu sebelum mengaktifkan varian.'` ·
`'Produk masih punya stok di varian aktif. Kosongkan stok varian sebelum menonaktifkan varian.'` ·
`'Varian masih punya stok. Kosongkan stok varian sebelum dihapus atau disembunyikan.'` (form) /
`'Varian masih punya stok. Kosongkan stok varian sebelum dinonaktifkan atau diganti.'` (import) ·
`"Kode produk '{k}' sudah digunakan"` · `"Kode produk '{k}' sudah digunakan sebagai kode atau barcode varian"` ·
`"Kode varian '{k}' duplikat"` / `"Kode varian '{k}' sudah digunakan"` / `"...sudah digunakan sebagai kode produk"` / `"Barcode varian '{b}' sudah digunakan..."` /
`"Kode/barcode varian '{k}' duplikat"` · `"Kode kategori '{k}' sudah digunakan"` ·
`'Kategori tidak ditemukan'` · `'Produk tidak ditemukan'` · `'Foto produk tidak ditemukan'` ·
`"Produk masih punya saldo stok {qty}. Habiskan atau adjustment ke 0 sebelum arsip."` ·
`'File Excel wajib diunggah'` · `'File harus berformat Excel .xlsx atau .xls'` · `'Foto produk wajib diunggah'` ·
`'Hanya file gambar JPG, PNG, atau WebP yang diizinkan'` ·
`"File belum bisa diproses karena masih ada {n} data yang perlu diperbaiki. Cek kembali hasil pemeriksaan file."` ·
`'Kode sudah dipakai data lain (duplikat).'` / `'Database sibuk (lock timeout). Coba ulang.'` / `'Bentrokan transaksi (deadlock). Coba ulang.'` (kegagalan per baris import)

**Toast sukses:** `"Produk berhasil ditambahkan"` / `"Produk berhasil diperbarui"` ·
`"Foto produk berhasil diunggah"` · `"Foto produk diarsipkan"` · `"Foto utama diperbarui"` ·
`"Import produk selesai"` + rekap (`"3 produk ditambahkan, 1 produk diperbarui, ..."` / `"Tidak ada perubahan data"`;
slice memakai `"Tidak ada perubahan data"` tanpa titik, modal memakai `"Tidak ada perubahan data."` — beda titik, [PERLU KONFIRMASI] diseragamkan atau tidak) —
**arsip produk dan simpan kategori TIDAK punya toast sukses.**

**Toast gagal:** `"Gagal menyimpan produk"` + pesan server · `"Gagal mengarsipkan produk"` (tanpa pesan) ·
`"Gagal menyimpan kategori"` (tanpa pesan — pesan server seperti duplikat kode tidak pernah tampil, seakar KI-17) ·
`"Gagal mengunggah foto produk"` + pesan · `"Gagal mengarsipkan foto produk"` · `"Gagal memperbarui foto utama"` ·
`"Gagal membaca file"` · `"Gagal import produk"`.

**Toast warning validasi form:** `"Satuan stok belum diisi"` / `"Pilih atau ketik satuan stok, misalnya pcs, batang, atau sak."` ·
`"Konversi satuan belum valid"` / `"Pastikan konversi beli dan jual lebih besar dari 0."` ·
`"Harga beli belum diisi"` / `"Produk yang dilacak stoknya butuh harga beli (lebih dari 0) supaya HPP-nya bisa dihitung saat barang terjual."` ·
`"Satuan transaksi belum diisi"` / `"Pilih atau ketik satuan yang biasa dipakai saat transaksi."` ·
`"Varian belum diisi"` / `"Tambahkan minimal satu varian stok atau matikan opsi varian."`.

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Restore produk/kategori/varian | Controller+service tidak punya endpoint restore; satu-satunya jalan adalah SQL manual. Kontras dengan Business Party yang punya `archive/restore` |
| NF-02 | Hapus permanen | Semua arsip = `archived_at` (soft). Role memang hard-delete (KI-21), produk tidak |
| NF-03 | Ubah kode kategori | `updateCategory` tidak menerima field kode |
| NF-04 | Ubah urutan foto | Tidak ada endpoint/media UI untuk `sort_order` setelah upload |
| NF-05 | Stok minimum untuk non-stok | Selalu di-null-kan; field disembunyikan di FE |
| NF-06 | Harga beli wajib untuk non-stok | Guard hanya untuk `stockTracked` |
| NF-07 | Pencarian kategori di server | `listCategories` tanpa filter/search — seluruh pohon dibangun di FE |
| NF-08 | Paginasi kategori | Seluruh kategori aktif dimuat sekaligus (tanpa `limit`) |
| NF-09 | Gambar per varian | `media_files` hanya ber-owner produk; varian tampil sebagai teks + QR |
| NF-10 | Riwayat perubahan harga | Harga ditimpa langsung; satu-satunya jejak adalah `audit_logs` (`product.create/update` hanya mencatat id+nama — harga lama tidak tercatat) |
