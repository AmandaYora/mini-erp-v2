# UI/UX Spec — Modul 05 Product / Catalog

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Mendeskripsikan tiap screen &
interaksi apa adanya dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Rute terdaftar (`module-registry.tsx`, semua `access: "protected"`, `shell: true`):

| Rute | Izin | Elemen |
|---|---|---|
| `/products` | `product.view` | `ProductsListPage` (+ menu sidebar grup **"Produk & Mitra"**, label **"Produk"**) |
| `/products/create` | `product.create` | `ProductFormPage` |
| `/products/:productId` | `product.view` | `ProductDetailPage` |
| `/products/:productId/edit` | `product.update` | `ProductFormPage` |
| `/product-categories` | `product.view` | `ProductCategoriesPage` |

---

## 1. Screen `/products` — Daftar Produk

**Header** (`PageHeader`): breadcrumb `Dashboard > Produk`; judul **"Daftar Produk"**;
deskripsi **"Kelola daftar produk."**; aksi kanan: **[Kelola Kategori]** (secondary, selalu ada) ·
**[Upload Bulk]** (secondary, bila `product.create`) · **[Tambah Produk]** (primer, bila `product.create`).

**FilterBar** (5 field, semua menyimpan ke URL; ganti filter me-reset `page`):

| # | Label (`htmlFor`) | Kontrol | Opsi / placeholder |
|---|---|---|---|
| 1 | Cari produk (`item-search`) | input teks + tombol ikon scan (aria `"Scan Barcode / QR"`) | placeholder `"Cari kode atau nama produk"`; ketik → debounce 400ms → URL `search`; tombol scan → `QRScannerModal`, hasilnya mengisi pencarian |
| 2 | Kategori (`item-category`) | `ProductCategorySelect` mode filter | `"Semua Kategori"` = semua; hierarki + cari kode/jalur |
| 3 | Tipe Produk (`item-type`) | `SearchableSelect` | Semua tipe / Barang fisik (`physical`) / Jasa (`service`) / Bundle (`bundle`) / Non-stock (`non_stock`) |
| 4 | Status (`item-status`) | `SearchableSelect` | **Default `active`** (Aktif) / Nonaktif (`inactive`) / Semua (`all`) |
| 5 | Pantau stok (`tracked`) | `SearchableSelect` | Semua (`""`) / Dipantau (`true`) / Tanpa stok (`false`) |

**Kartu "Daftar Produk"** (`SectionCard`): deskripsi dinamis `"Memuat…"` / `"{total} produk ditemukan."`;
aksi kanan **[Download QR (ZIP)]** (secondary; saat bekerja label jadi `"Menyiapkan semua QR..."` dan
disabled). Tabel (`DataTable`) kolom: **Produk** (thumbnail 44px rounded + nama tebal + kode redup) ·
**Kategori** (nama / `"—"`) · **Tipe** (badge netral) · **Harga Beli** (kanan: nominal + `/ {purchaseUom}`
redup) · **Harga Jual** (kanan: nominal + `Min {min} / {salesUom}` redup) · **Stok** (berstok: tebal
`"5 meter (0.5 roll)"` bila satuan beda else `"12 pcs"`, sub-baris `Min {n}`; non-stok: redup
`"Tidak dipantau"`; belum siap: `"Memuat stok..."`) · **Status** (badge hijau Aktif / kuning Nonaktif) ·
**Aksi** (tombol ikon QR + **[Detail]** secondary). Kosong (tidak loading): `EmptyState`
**"Daftar produk kosong"** / `"Belum ada produk yang cocok dengan filter ini."` + tombol Tambah (bila
berhak). Bawah: `Pagination` (20/halaman).

**Modal & dialog dari screen ini:** `QRGeneratorModal` (1 target) · unduh ZIP langsung (>1 target) ·
`QRScannerModal` · `ProductBulkUploadModal` (lihat §5) · error unduh QR via `alert()` teks error mentah
(satu-satunya `alert()` di modul ini — [PERLU KONFIRMASI] diganti toast atau tidak).

## 2. Screen `/products/create` & `/products/:id/edit` — Form Produk

Satu komponen, mode dari ada/tidaknya produk. **Header**: breadcrumb
`Dashboard > Produk > Tambah Produk` / `> {kode} > Edit`; judul **"Tambah Produk"** / **"Edit Produk"**;
deskripsi **"Data identitas produk."**; aksi **[← Kembali]** (ghost, `navigate(-1)`). Produk edit tak
dikenal: notice merah **"Produk tidak ditemukan"** / `"Produk yang Anda edit tidak tersedia pada
context perusahaan saat ini."` (saat fetch: notice biru **"Memuat produk"**).

**Kartu Informasi Dasar** (deskripsi `"Pilih tipe produk lebih dulu agar form menyesuaikan kebutuhan
operasional."`): Kode produk* (`itemCode`, required) · Nama produk* (`itemName`, required) · Kategori
(`ProductCategorySelect` mode assignment + `"Tanpa kategori"`) · Tipe Produk (`itemType`: Barang fisik /
Jasa / **Bundle ringan** / Non-stock) · Status (`status`: Aktif default / Nonaktif). Urutan field:
Kode → Nama → Kategori → Tipe → Status.

**Kartu Foto Produk**: tambah (bila `product.create`) → deskripsi `"Foto awal produk akan menjadi foto
utama setelah produk tersimpan."`, field tunggal pilih-file + pratinjau lokal; edit → deskripsi
`"Media visual untuk identifikasi produk."`, `ProductMediaPanel` penuh.

**Kartu operasional** (judul+deskripsi per tipe — Jasa: **"Transaksi Layanan"** /
`"Untuk jasa, Anda cukup menentukan satuan transaksi tanpa pengaturan stok gudang."`; Bundle:
**"Transaksi Bundle"** / `"Bundle saat ini diperlakukan sebagai paket transaksi sederhana, bukan stok
komponen."`; Fisik: **"Stok & Satuan"** / `"Atur satuan operasional produk dan tentukan apakah produk
ini dipantau sebagai stok gudang."`**; Non-stock: **"Transaksi Produk"** / deskripsinya): fisik →
checkbox **"Pantau stok produk ini"** + hint, lalu Satuan stok* (placeholder `"pcs, sak, liter"`) ·
Satuan beli (`"box, sak, karton"`) · Konversi beli ke stok (hint `"Contoh: 1 box = 12 pcs, maka isi 12."`,
preview `"1 {beli} masuk stok sebagai {n} {stok}."`) · Satuan jual (`"pcs, meter, sesi"`) · **Isi jual
per stok** (hint `"Isi dengan bahasa operasional. Contoh: 1 roll = 10 meter, maka isi 10."`, preview
`"1 {stok} = {n} {jual}."`) · **Dampak jual ke stok** (hint `"Nilai ini dihitung otomatis dari field di
sebelahnya. Ubah hanya jika Anda lebih nyaman memakai rumus teknis."`, preview `"Jual {n} {jual}
mengurangi {m} {stok}."` — preview memakai 5 bila angka ≥ 5 else 1) · Minimum stok (bila dipantau);
non-fisik → satu field satuan (label/placeholder per tipe + catatan operasional sebagai teks redup, bukan
notice). Semua dropdown satuan: 19 opsi umum (pcs, batang, lembar, meter, kg, gram, liter, sak, box,
karton, roll, set, unit, paket, layanan, km, jam, hari, sesi) + **boleh ketik baru** (`allowCreate`).
Label field satuan memakai komponen `FieldLabelWithHint` (label + ikon hint berdampingan).

**Kartu Varian Stok** (hanya bila dipantau; judul **"Varian Stok"**, deskripsi **"Aktifkan hanya jika
warna, ukuran, atau pilihan lain memiliki saldo stok sendiri."**): checkbox **"Produk ini memiliki
varian stok"**; aktif → daftar baris varian (Nama varian [`"Merah, Biru, XL"`] · Kode varian
[`"Opsional, otomatis jika kosong"`] · Barcode varian [`"Opsional"`] · Status Aktif/Nonaktif ·
**"Hapus varian {n}"** ghost bila >1 baris) + **[Tambah Varian]**; mati → teks redup `"Produk disimpan
dengan varian default internal yang tidak tampil ke user."`.

**Kartu Harga & Ringkasan** (deskripsi `"Harga tetap fleksibel untuk semua tipe produk."`): Harga beli
(number, `min=0`; placeholder dinamis — berstok: `"Wajib untuk produk berstok — dasar perhitungan HPP"`,
else `"Harga referensi dari supplier atau vendor"`) · Harga jual normal (`"Harga yang ditawarkan ke
customer"`) · Harga jual minimum (`"Batas bawah harga nego"`) · Ringkasan operasional (textarea 80px,
`"Tulis konteks bisnis singkat produk ini."`).

**Kartu Atribut Produk** (deskripsi `"Catat data spesifik produk seperti brand, segmen pasar, bahan
baku, dsb."`): tiap baris Nama Atribut (`"Nama Atribut (Contoh: Brand)"`, dropdown saran + boleh buat
baru, placeholder `"Pilih atau ketik..."`) + Keterangan (`"Isi nilai atribut"`) + **[Hapus]** merah;
**[+ Tambah Atribut Baru]**.

**ActionRow**: **[Batal]** (secondary, ke detail/daftar) · **[Simpan Produk]** / **[Simpan Perubahan]**
(`"Menyimpan..."` saat sibuk, disabled saat menyimpan).

Interaksi penting: ganti tipe fisik→non-fisik **menghapus field stok dari DOM** (test: `queryByLabelText`
null) tetapi **draft satuan fisik dipertahankan** di state (test: kembali ke fisik → nilai kembali);
checkbox pantau/varian adalah checkbox native tanpa switch kustom; foto awal hanya 1 file (ganti =
timpa state, tanpa validasi tipe di FE selain `accept`).

## 3. Screen `/products/:productId` — Detail Produk

**Header**: breadcrumb `Dashboard > Produk > {kode}`; judul nama produk; deskripsi = ringkasan (bisa
kosong); aksi: **[← Kembali]** ghost · **[Lihat Stok]** secondary (hanya `stockTracked`, ke
`/stock/{id}`) · **[Penyesuaian Stok]** secondary (hanya berstok + `stock.adjust`, ke
`/stock/adjustments/create?item_id={id}&returnTo={detail}`) · **[Edit]** secondary (`product.update`,
ke edit) · **[Arsipkan]** danger (`product.archive`, buka dialog).

**Kartu Informasi Dasar** (`"Detail umum produk."`, grid responsif ~180px): Kode Produk · Kategori ·
Tipe · Harga Beli / {purchaseUom} · Harga Jual Normal / {salesUom} · Harga Jual Minimum / {salesUom} ·
(fisik:) Satuan Stok, Satuan Beli (`1 {beli} = {n} {stok}`), Satuan Jual (`1 {stok} = {n} {jual}` +
sub-baris `"Jual 1 {jual} mengurangi {n} {stok}"` bila faktor ≠ 1) — (non-fisik:) satu baris satuan +
Catatan Operasional · Status (Aktif/Nonaktif teks). Nilai kosong harga tampil `"-"`. Gaya key-value:
label kecil uppercase redup + nilai medium (kelas `KV_ITEM`).

**Kartu Foto Produk** (`"Media visual untuk identifikasi produk."`): `ProductMediaPanel`.

**Kartu Atribut Produk** (`"Informasi spesifik tambahan yang melengkapi detail produk."`): pasangan
kunci-nilai gaya key-value; kosong → notice info `"Produk ini belum memiliki atribut tambahan."`.

**Kartu Kondisi Stok** (hanya berstok; `"Informasi real-time ketersediaan produk ini."`; 4 kotak metrik
angka 1.5rem: On hand · Reserved · Available · Minimum `{min} {baseUom}`; dual-UOM untuk on hand &
reserved; Available memakai `displayStockLabel` dari workspace).

**Dialog arsip** (`ConfirmDialog` danger): judul **"Arsipkan produk"**; deskripsi
`"Produk {nama} akan dipindahkan ke status nonaktif dan tidak muncul lagi di daftar aktif."`
(**catatan: teks menyebut "status nonaktif" padahal yang terjadi adalah arsip permanen tak-terpulihkan —
→ KI-56**); tombol **"Tahan dulu"** / **"Ya, arsipkan"**.

## 4. Screen `/product-categories` — Kategori Produk

**Header**: breadcrumb `Dashboard > Produk > Kategori`; judul **"Kategori Produk"**; deskripsi
**"Kelompokkan produk ke dalam hierarki kategori tanpa batas kedalaman."** Dua kolom responsif (~420px):

Kiri — **"Hierarki Kategori"** (`"Klik tombol expand untuk membuka turunan kategori, lalu pilih Edit
bila perlu mengubahnya."`): node kartu (border + garis kiri 3px tebal bermerek untuk root; menjorok
20px/level; tombol lipat teks **`v` / `>`** 0.75rem — bukan ikon; badge kode netral-subtle; tombol
**Edit** kecil bila `product.update`; label `Level {n}` redup untuk anak); kosong → `EmptyState`
**"Belum ada kategori"** / `"Tambahkan kategori pertama menggunakan form di sebelah kanan."`.

Kanan — **"Tambah Kategori"** / **"Edit Kategori"** (`"Tambah kategori baru pada level mana pun dalam
hierarki."` / `"Perbarui nama, kode, atau posisi induk kategori ini."`): Kategori Induk (dropdown
hierarki + helper `"Kosongkan untuk menjadikan kategori utama pada level 0."`, opsi kosong
`"-- Kategori Utama (tanpa induk) --"`) · Nama kategori* (`category-name`) · Kode kategori*
(`category-code`); aksi **[Batal Edit]** (saat edit) / link **[Kembali ke Produk]** (saat tambah) +
**[Tambah Kategori]** / **[Simpan Perubahan]**; submit langsung memanggil save + reset form **tanpa
menunggu hasil dan tanpa toast** (kegagalan hanya toast `"Gagal menyimpan kategori"`). Tanpa
`product.create`+`product.update`: teks `"Anda hanya memiliki akses lihat untuk kategori produk."`.

## 5. Modal Upload Bulk (`ProductBulkUploadModal`, portal, 960px)

Header: badge **Excel** + judul **"Upload Bulk Produk"** + deskripsi **"Import kategori dan produk dari
file Excel."** + tombol tutup ✕ (aria `"Tutup modal"`; disabled + klik-luar/Escape diblokir saat sibuk).
Zona file: ikon + nama file / `"Pilih file Excel"` + ukuran/`".xlsx / .xls"` /
`"Template berisi kategori, produk, varian, dan info tambahan produk."`; tombol **[Download Template]**
(`"Menyiapkan..."` saat membuat) · **[Pilih File]**/**[Ganti File]** · **[Cek File]** (`"Mengecek..."`).
Status: badge **Belum ada file** (netral) / **File dipilih** (info) / **Siap diproses** (sukses) /
**Perlu diperbaiki** (danger) / **Import selesai** (sukses) + ringkasan satu baris atau `"Belum ada
preview."`. Error: kotak merah kiri-tebal **`Gagal:`** + pesan. Preview: 8 kartu angka (Kategori
ditambahkan/diperbarui, Produk ditambahkan/diperbarui, Varian ditambahkan/diperbarui, Error, Peringatan —
kartu error/warning berubah merah/kuning bila > 0); banner hijau **`Import selesai.`** + rekap; banner
merah **`{n} baris gagal diimpor.`** + penjelasan idempoten; tabel isu (Status badge Error/Peringatan ·
Bagian · Baris · Kolom · Keterangan; 12 pertama + `"Masih ada {n} pesan lain di file ini."`) atau tabel
produk (8 pertama: Baris · Kode · Nama · Jenis · Kategori · Aksi **Ditambahkan/Diperbarui**) + tabel
varian (8 pertama). Footer: **[Lanjut Isi Stok Awal]** (secondary, hanya setelah import + ada handler) ·
**[Tutup]** · **[Proses Import]** (`"Memproses..."`; disabled tanpa file / `!ok` / sudah import / sibuk).
Tutup me-reset seluruh state modal.

## 6. Komponen dipakai-ulang lintas modul

| Komponen | Lokasi | Peran & kontrak tampil |
|---|---|---|
| `ProductSearchSelect` | `components/domain/product-search-select.tsx` | Async picker (dipakai Order, Stock, modul lain); opsi `"{kode} - {nama}"` + deskripsi satuan; placeholder `"Cari kode atau nama produk..."` / cari `"Ketik kode atau nama produk..."`; default `status=active`, `preferStockAvailable=true`, `limit=20` |
| `ProductCategorySelect` | `modules/products/components/product-category-select.tsx` | Dropdown hierarki (menjorok per level, tampilkan jalur `A > B > C`); cari mencakup kode + jalur; teks kosong `"Belum ada kategori"`, placeholder `"-- Pilih Kategori --"`; mode assignment menonaktifkan induk (`"Pilih sub-kategori paling bawah"`) |
| `ProductMediaPanel` | `modules/products/components/product-media-panel.tsx` | Label `"Upload foto produk"` (`product-image-upload`); `"Memuat foto..."`; kosong → gambar default ber-badge **Default** + notice `"Produk ini belum memiliki foto."`; kartu 156px: badge **Foto utama** (hijau) / **Galeri** (netral) + **[Jadikan Utama]** (non-utama) + **[Arsipkan]** ghost; tanpa `canUpdate`: tidak ada input maupun tombol |
| `QRGeneratorModal` / `QRScannerModal` | `components/domain/` | Pratinjau + unduh 1 QR; pindai kamera/barcode → teks kode |
| `InitialProductImageField` | `modules/products/components/` | Label `"Foto produk"`; tombol **Pilih Foto** + nama file / `"Belum ada file dipilih"`; pratinjau 220px bila ada |
| `FieldLabelWithHint` | `modules/products/components/` | Label + ikon hint sebaris (dipakai semua field satuan) |

## 7. Perilaku visual & format yang mengikat

- Uang: `formatCurrency` (id-ID, IDR) di daftar + detail; stok: `formatQuantity` (id-ID, maks 4 desimal)
  + label konversi `1 {a} = {n} {b}` / `Jual {n} {jual} mengurangi {m} {stok}`.
- Status produk selalu kata **Aktif/Nonaktif** (badge di daftar, teks di detail, opsi di form); tipe
  selalu **Barang fisik/Jasa/Bundle/Non-stock** (form tambah memakai label **"Bundle ringan"** sedang
  daftar memakai **"Bundle"** — inkonsistensi kecil, [PERLU KONFIRMASI] diseragamkan atau tidak).
- Struktur layout kartu: `PageHeader` → `FilterBar`/`SectionCard` → `DataTable`/`Pagination`;
  grid form `repeat(auto-fit,minmax(280px,1fr))`; grid detail `minmax(180px,1fr)`; tidak ada tabel
  varian di detail (varian hanya memengaruhi QR + dropdown transaksi modul lain).
- Daftar tidak menampilkan: ringkasan, atribut, barcode varian, tanggal dibuat/diubah, stok minimum
  sebagai kolom (hanya sub-baris), maupun penanda `hasVariants`.
