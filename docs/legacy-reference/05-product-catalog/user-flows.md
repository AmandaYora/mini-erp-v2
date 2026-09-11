# User Flows — Modul 05 Product / Catalog

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Alur end-to-end per skenario,
termasuk matriks izin. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Endpoint selalu `@Post` + `@HttpCode(200)` + envelope `{ data }`, kecuali dua endpoint multipart
(`products/media/upload`, `product-imports/preview|commit`) yang memakai `FormData` (`file` + field
string). Scope perusahaan selalu dari sesi (`idCompany`); `idBranch` sesi hanya dipakai untuk
pengurutan stok-tersedia-dulu.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Buka `/products`, `/products/:id`, `/product-categories`, cari, unduh QR | `product.view` | Rute diblokir penjaga (lihat modul 02) |
| Buka `/products/create`, tombol Upload Bulk/Tambah | `product.create` | Tombol hilang; rute diblokir |
| Buka `/products/:id/edit`, tombol Edit, kelola foto | `product.update` | Tombol hilang; rute diblokir |
| Tombol Arsipkan produk | `product.archive` | Tombol hilang |
| Form kategori tambah vs edit | tambah butuh `product.create`, edit butuh `product.update` (salah satu cukup untuk melihat form) | Teks baca-saja |
| Tombol Lihat Stok / Penyesuaian Stok di detail | tampil bila berstok; penyesuaian + butuh `stock.adjust` | Tombol hilang |
| Preview + commit import | Keduanya `product.create` | — |
| Upload/jadikan-utama/arsip foto | `product.update` | Panel baca-saja |

## UF-01 — Tambah produk fisik berstok (alur utama)

1. Pengguna (`product.create`) membuka `/products` → **[Tambah Produk]** → `/products/create`
   (tipe default **Barang fisik**, pantau stok **centang**).
2. Isi Kode* + Nama* → Kategori (pilih leaf; induk disabled) → Status (default Aktif).
3. (Opsional) Foto awal: **[Pilih Foto]** → pratinjau lokal.
4. Seksi Stok & Satuan: Satuan stok* (mis. `roll`) → Satuan beli (`box`) + Konversi (`12`) →
   Satuan jual (`meter`) + Isi jual per stok (`10`; Dampak terisi otomatis `0.1`) → Minimum stok.
   Preview di bawah tiap field mengonfirmasi (`1 box masuk stok sebagai 12 roll.`).
5. Harga: Harga beli* (> 0, wajib) → Jual normal → Jual minimum → Ringkasan → Atribut (opsional).
6. **[Simpan Produk]** → validasi FE (lihat F-02.9) → `POST products/create` → toast
   `"Produk berhasil ditambahkan"` → pindah `/products/:idBaru` → (bila ada foto awal) upload sebagai
   foto utama → foto tampil di detail. **Hasil akhir:** produk + 1 varian default tersembunyi
   `Standar` (kode=barcode=kode produk).

## UF-02 — Tambah jasa / bundle / non-stock

1–2. Sama, lalu Tipe = **Jasa/Bundle ringan/Non-stock** → seksi stok hilang dari DOM, muncul satu
field satuan + catatan operasional.
3. Isi satuan transaksi (mis. `jam`) → harga (bebas, boleh kosong) → **[Simpan]**.
4. Server memaksa: `stock_tracked=false`, ketiga satuan = satuan transaksi, kedua faktor = 1,
   `min_stock_qty=null` — apa pun yang dikirim. Produk tidak muncul di modul stok dan tidak
   memengaruhi saldo.

## UF-03 — Edit produk

1. Dari daftar → **Detail** → **[Edit]** (`product.update`) → `/products/:id/edit` terisi penuh
   (bila produk di luar 100 store: fetch `products/detail` + notice biru `"Memuat produk"`).
2. Ubah field → **[Simpan Perubahan]** → `POST products/update` → toast sukses → kembali ke detail.
3. Batasan yang menahan simpan (server, pesan spesifik): ganti kode menabrak kode/barcode lain (409);
   ubah base_uom/faktor setelah ada movement (400); ubah tipe berstok → non-fisik (400); aktifkan
   varian saat stok default > 0 (400). **Bebas:** ganti label satuan beli/jual, turunkan/Naikkan harga,
   ganti kategori/status/ringkasan/atribut kapan pun.

## UF-04 — Arsip produk

1. Detail → **[Arsipkan]** → dialog **"Arsipkan produk"** → **[Ya, arsipkan]** →
   `POST products/archive` → kembali `/products` **tanpa toast**.
2. Stok > 0 → toast `"Gagal mengarsipkan produk"` (tanpa rincian — pesan server berisi angka stok
   tidak diteruskan) dan tetap di detail.
3. Setelah arsip: hilang dari **semua** filter daftar (termasuk "Semua"), picker, POS, dan endpoint
   media; historinya (order/stok/keuangan lama) tetap. **Tidak ada jalan kembali** (→ KI-56).

## UF-05 — Kelola kategori

1. `/products` → **[Kelola Kategori]** → pohon + form.
2. Tambah: pilih Induk (opsional) → Nama* → Kode* → **[Tambah Kategori]** → pohon bertambah
   (tanpa toast; gagal → toast `"Gagal menyimpan kategori"` tanpa alasan).
3. Edit: **[Edit]** di node → form terisi (Induk = induk saat ini; diri + keturunan hilang dari
   pilihan) → ubah Nama/Induk → **[Simpan Perubahan]**. **Kode bisa diketik ulang tetapi diabaikan
   server** (→ KI-51).
4. Arsip kategori: **tidak ada di UI** (hanya API langsung; tanpa cek pemakaian — produk yang
   menunjuknya tetap menunjuk, namanya hilang dari daftar menjadi `"—"` bila data workspace
   di-reload... tepatnya kolom daftar memakai data workspace sehingga nama lama tetap tampil sampai
   reload; [PERLU KONFIRMASI] perilaku cache ini).

## UF-06 — Kelola foto produk

1. Detail/edit → kartu Foto → **[Upload]** (input `product-image-upload`) → kompresi FE → optimasi
   WebP backend → toast `"Foto produk berhasil diunggah"` → grid bertambah.
2. Foto non-utama → **[Jadikan Utama]** → badge hijau pindah + toast `"Foto utama diperbarui"`.
3. Foto mana pun → **[Arsipkan]** → hilang + toast `"Foto produk diarsipkan"`; bila itu foto utama,
   foto tersisa paling awal naik otomatis.
4. Produk tanpa foto: gambar default ber-badge **Default** + notice `"Produk ini belum memiliki foto."`.
5. Tanpa `product.update`: hanya grid + badge, tanpa input maupun tombol.

## UF-07 — Import bulk Excel (tambah massal + update massal)

1. `/products` → **[Upload Bulk]** → **[Download Template]** → `template-upload-produk.xlsx`.
2. Isi sheet (urutan disarankan: Kategori → Daftar Produk → Varian → Info Tambahan; baca Panduan) →
   **[Pilih File]** → **[Cek File]** (`product-imports/preview`).
3. Bersih (`Siap diproses`): tabel 8 produk + 8 varian pertama (Aksi Ditambahkan/Diperbarui) →
   **[Proses Import]** (`product-imports/commit`) → banner hijau + rekap → **[Lanjut Isi Stok Awal]**
   (opsional, ke `/stock/opening`) → **[Tutup]** (daftar me-reload).
4. Kotor (`Perlu diperbaiki`): tabel 12 isu pertama (Status/Bagian/Baris/Kolom/Keterangan) → perbaiki
   file → ulangi dari langkah 2. Tombol Proses **disabled** selama `!ok`.
5. Sebagian gagal: banner merah + tabel 50 kegagalan → perbaiki baris itu saja → upload ulang file
   yang sama (idempoten, tidak dobel).

## UF-08 — Ekspor QR

1. Daftar → **[Download QR (ZIP)]** → seluruh produk (100/halaman) → ZIP per folder kategori →
   `Produk-QR-Codes.zip`. Produk bervarian menyumbang **satu file per varian**, tanpa file induk.
2. Baris produk → tombol QR: 1 target → modal pratinjau + unduh 1 PNG; >1 target → langsung ZIP
   `{kode}-QR-Varian.zip`.
3. QR ditempel di rak/kemasan; hasil pindai = kode mentah yang cocok dengan pencarian daftar,
   input barcode POS, dan picker async modul lain.

## UF-09 — Cari & pakai produk dari modul lain (picker)

1. Order/stok/POS memanggil `ProductSearchSelect` → ketik ≥1 huruf → dropdown server (relevansi +
   stok-tersedia-dulu) → pilih → id tersimpan; label tersimpan di-resolve ulang via `id_product`.
2. Daftar + POS memakai `products/list` dengan `prefer_stock_available` — produk habis turun ke
   bawah (E2E 17 mengunci urutan: berstok → non-stok/jasa → habis).
3. Produk `inactive`/arsip tidak muncul di picker default.

## UF-10 — Aktifkan varian pada produk berjalan

1. Edit produk berstok tanpa varian → centang **"Produk ini memiliki varian stok"** → **[Tambah
   Varian]** → isi Nama (+ Kode/Barcode/Status opsional) → simpan.
2. Stok masih > 0 di varian default → 400 + toast gagal (harus kosongkan/split stok dulu via modul
   Stock). Sukses → varian default lama diarsipkan; saldo berikutnya tercatat per varian; QR induk
   digantikan QR per varian.
3. Menonaktifkan varian terakhir / menghapus varian berstok → ditolak dengan pesan yang sama.
   Menonaktifkan produk menjadi non-fisik saat varian aktif → ditolak (aturan tipe).

## UF-11 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons pengguna |
|---|---|
| Simpan produk invalid (server) | Toast `"Gagal menyimpan produk"` + pesan server spesifik (kode duplikat, UOM terkunci, tipe dilarang, dst.) |
| Simpan kategori gagal | Toast `"Gagal menyimpan kategori"` **tanpa alasan** (duplikat kode tak terbedakan) |
| Arsip produk berstok | Toast `"Gagal mengarsipkan produk"` tanpa angka; tetap di detail |
| Upload foto gagal | Toast + pesan (tipe tak didukung / >5MB / tanpa file) |
| Import kotor | Tombol Proses disabled + tabel isu; commit langsung via API → 400 dengan hitungan error |
| Commit sebagian gagal | Banner merah + tabel per baris; upload ulang file sama aman |
| Produk tak dikenal (URL lama/manual) | Notice `"Produk tidak ditemukan"` (tidak jatuh ke form tambah) |
| Tanpa foto | Gambar default + badge + notice (bukan error) |
