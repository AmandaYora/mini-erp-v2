# Feature Inventory — Modul 07 Member Type & Member Pricing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = master jenis member (CRUD) + mesin hitung harga member (`resolvePrice`,
`pricing/quote`, snapshot). Yang **bukan** bagian modul ini: penetapan member ke pelanggan
(milik modul 06 — di sini hanya dibaca via `id_member_type`), pemakaian harga di order/POS
(milik modul 08/09/15 — di sini hanya kontrak `pricing_source` + snapshot + bypass guard),
cetak nota (membaca snapshot, milik modul cetak).

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 5 — `member-types/{list,create,update,archive}` (4), `pricing/quote` (1) |
| Halaman | 3 rute, 1 komponen: `/member-types`, `/member-types/create`, `/member-types/:memberTypeId/edit` (`MemberTypesPage` tiga mode) |
| Menu sidebar | 1, grup **"Produk & Mitra"**, label **"Jenis Member"** (`/member-types`) |
| Permission | `member_type.view` (buka daftar + field member di form pelanggan/shortcut) vs `member_type.manage` (tambah/ubah/arsip). `pricing/quote` memakai `order.create`. Seed 040: keduanya untuk superadmin/owner/admin |
| Tabel yang dimiliki | `member_types` |
| Tabel yang dibaca | `business_parties` (hitung pemakai; relasi `memberType` di quote), `products` (basis harga di quote) |
| Kolom yang ditulis di tabel modul lain | 7 snapshot di `order_items` (ditulis modul Order — kontrak konsumen, lihat F-03) |
| Laporan | Tidak ada — lihat [reports-list.md](reports-list.md) |
| Penomoran | Tidak ada (kode manual) — lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | 3 `actionKey`: `member_type.create/update/archive` (snapshot rule penuh; update before+after) |
| Aksi yang TIDAK ada | Restore member; hapus permanen; duplikat/copy rule; riwayat perubahan rule; simulasi massal (quote hanya per daftar id produk yang dikirim); member per produk/kategori (rule selalu global) |

**Catatan lingkup.** Satu jenis member = **satu rule global untuk semua produk**: basis + aksi +
tipe + besaran + pembulatan opsional. Tidak ada pengecualian per produk, kategori, cabang, atau
periode. Harga member dihitung dalam **satuan jual (UOM transaksi)** dan ditulis ke order sebagai
**snapshot** — order lama tidak berubah saat rule berubah (future-only; dijamin oleh snapshot,
tetapi belum ada test yang mengubah rule lalu memeriksa order lama — [GAP], lihat G-02).

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar Jenis Member (`/member-types`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Cari lokal (tanpa debounce, tanpa URL) | Ketik → `listSearch` state → request ulang `page=1`; placeholder `"Cari nama atau kode"`; cari di nama ATAU kode (LIKE, server). Refresh/reset state menghilangkan pencarian (beda dengan pelanggan/produk — → KI-74) |
| F-01.2 | Paginasi 20 (state lokal, bukan URL) | `listPage` state; `Pagination` standar; request selalu `status: "all"` (aktif + nonaktif tampil campur; arsip tak pernah tampil) |
| F-01.3 | Kolom Nama | Nama tebal + kode redup |
| F-01.4 | Kolom Rule | `"{Basis} {+ polynomial}"`: `Harga beli + Rp 1.000` / `Harga jual normal - 10%` / `Harga jual minimum + Rp 500` (tanda `-` untuk Kurang, `+` untuk Tambah; `%` vs `Rp` mengikuti tipe; angka format `id-ID` maks 2 desimal) |
| F-01.5 | Kolom Pembulatan | `Tanpa pembulatan` (mode none) atau `"{Mode} Rp {kelipatan}"` (`Ke atas`/`Terdekat`/`Ke bawah`; kelipatan kosong → label mode + spasi kosong) |
| F-01.6 | Kolom Status | Badge hijau `Aktif` / kuning `Tidak aktif` |
| F-01.7 | Aksi baris | `[Edit]` secondary + `[Arsipkan]` ghost — hanya bila `member_type.manage`; tanpa izin: sel kosong |
| F-01.8 | Tombol Tambah | Header + empty-state, hanya `member_type.manage` (`Tambah Jenis Member`) |
| F-01.9 | Kartu + kosong | `Daftar Jenis Member` / `"{n} jenis member tersedia."` / `"Memuat…"`; kosong: `Belum ada jenis member` / `Buat jenis member pertama untuk mulai mengaitkannya ke pelanggan.` |
| F-01.10 | Dialog arsip | `Arsipkan jenis member` (warning): `"Jenis member {nama} tidak akan bisa dipilih untuk pelanggan baru."` → `Arsipkan`. Gagal (masih dipakai) → dialog **tetap terbuka** + toast gagal berpesan; sukses → tutup + reload + toast sukses |

### F-02 — Form Jenis Member (`/create`, `/:id/edit`, satu komponen dua mode)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Mode dari rute + store | Tambah vs edit dari URL; data edit dari store (limit 1000, semua status) — tanpa fetch by-id (beda dengan pelanggan/produk; >1000 rule = layar tidak-ditemukan — → KI-75) |
| F-02.2 | Layar muat / tidak-ditemukan | Belum siap: `Memuat Jenis Member` / `Data jenis member sedang disiapkan.` + kartu `Memuat data` / `Mohon tunggu sebentar.` / `Mengambil data jenis member...`; siap tapi tak ada: `Jenis Member Tidak Ditemukan` + `EmptyState` + `[Lihat Daftar]` |
| F-02.3 | Kartu Identitas | `Kode`* (placeholder `"A"`, required; disimpan **huruf besar**) · `Nama jenis member`* (placeholder `"Member A"`, required) · `Status` (Aktif default / Tidak aktif) · `Catatan` (textarea 80px, opsional) |
| F-02.4 | Kartu Aturan Harga | `Basis harga` (default Harga beli) · `Aksi` (default Tambah) · `Tipe nilai` (default Nominal) · `Besaran`* (number `min=0 step=0.01`, default 0, required) · `Pembulatan` (default Tanpa pembulatan) · `Kelipatan pembulatan` (number `min=0 step=0.01`, placeholder `"100"`, opsional). Deskripsi kartu: `"Basis harga dipilih sekali untuk jenis member ini."` |
| F-02.5 | Header form | Breadcrumb `Dashboard > Jenis Member > Tambah|Edit`; judul `Tambah/Edit Jenis Member`; deskripsi **`"Aturan harga berlaku global untuk semua produk dan dihitung pada UOM jual."`**; `[← Kembali]` ghost → `/member-types` |
| F-02.6 | Submit | `[Batal]` (secondary → daftar) + `[Simpan Jenis Member]` (`"Menyimpan..."` + disabled saat simpan — anti-double-submit ADA di sini, beda dengan form pihak); sukses → toast + ke daftar; gagal → toast gagal + pesan server, tetap di form |

### F-03 — Mesin harga member (dipakai Order & POS, kontrak modul ini)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | `pricing/quote` (izin `order.create`) | Input: `id_member_type` ATAU `id_business_party` + `items: [{id_product}]` (id didedup). Output: `member_type` (detail rule / null) + per item `{id_product, product_name, product_code, sales_uom, pricing_source, unit_price, basis_price, member_type, error}`. **Per-item gagal tidak menggagalkan request** (item itu `unit_price: null` + `error`) |
| F-03.2 | Rumus (urutan tetap) | Basis (dalam UOM jual) → tambah/kurang besaran (nominal langsung; persen = basis × nilai/100) → tolak bila negatif → bulatkan → bulatkan 2 desimal. `basis_price` snapshot = basis 2-desimal |
| F-03.3 | Tiga basis | `selling_price` = harga jual normal; `min_selling_price` = harga jual minimum; `purchase_price` = `(harga_beli ÷ faktor_beli) × faktor_jual` (konversi ke UOM jual). Basis kosong/bukan-angka → error `"'{nama}' belum memiliki {label}"`; negatif → `"{label} ... tidak valid"`; faktor konversi 0/invalid (basis beli) → error konversi |
| F-03.4 | Tanpa member | Harga = manual (bila diketik) ?? jual normal ?? 0; `pricing_source` = `standard` bila tanpa ketikan ATAU ketikan == jual normal (2-desimal), else `manual`; tanpa harga pula → 0 (`standard`) |
| F-03.5 | Snapshot order (7 kolom) | `pricing_source`, `id_member_type`, `member_type_name`, `price_basis`, `price_adjustment_direction`, `price_adjustment_type`, `price_adjustment_value`, `basis_price` — ditulis modul Order saat create/update; cetak & diskon memakai snapshot ini |
| F-03.6 | Bypass guard minimum | Harga `member_rule` **boleh di bawah `min_selling_price`** (pengecualian disengaja di 2 jalur order create/update, sales saja). Manual/non-member tetap dijaga. Harga member sendiri adalah floor (diskon manual di atasnya di-klem ke harga member, bukan ke minimum) |
| F-03.7 | Requote di UI | Ganti pelanggan di form order/POS → quote ulang → harga baris non-manual ditimpa; baris yang sudah diedit manual (`priceEdited`) tidak ditimpa (order form); diskon di-klem ulang ke harga baru; error per produk diagregasi jadi banner (bukan toast) |

### F-04 — Guard penetapan (relasi ke pelanggan)

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Arsip ditolak bila dipakai | `member-types/archive` + update→`inactive` menolak dengan hitungan: `"Tidak bisa {mengarsipkan\|menonaktifkan} jenis member yang masih digunakan oleh {n} pelanggan aktif. Lepaskan atau ganti jenis member pelanggan terlebih dahulu."` (hanya pelanggan aktif dihitung; arsip/SUPPLIER tak dihitung — supplier tak bisa punya member) |
| F-04.2 | Penetapan di form pelanggan | Dropdown `Non-member` + member aktif (+ milik saat ini); tanpa `member_type.view` field hilang dan nilai lestari (KI-67 modul 06) |
| F-04.3 | Tanpa restore | Arsip final; kode arsip tak bisa dipakai ulang (unique mencakup arsip — → KI-71) |

---

## 3. Edge Case (26)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Kode duplikat aktif | `409 "Kode jenis member '{k}' sudah digunakan"` (cek: trim + UPPERCASE + non-arsip) |
| E-02 | Kode milik arsip dipakai ulang | Lolos cek → **500** unique-key (tanpa pesan ramah — → KI-71) |
| E-03 | Kode/nama kosong (create maupun update-menyentuh) | `400 'Kode jenis member wajib diisi'` / `'Nama jenis member wajib diisi'` (setelah trim; update parsial: hanya bila field dikirim) |
| E-04 | Kode `grosir` vs `GROSIR` | Sama (selalu uppercase; test: `a` → `A`) |
| E-05 | Enum tak dikenal (basis/aksi/tipe/mode/status) | `400 '{Basis harga\|Aksi harga\|Tipe nilai\|Pembulatan\|Status jenis member} tidak valid'` |
| E-06 | Besaran negatif/bukan-angka | `400 'Besaran rule member harus bernilai 0 atau lebih'` (create & update-menyentuh) |
| E-07 | Kelipatan negatif/bukan-angka | `400 'Nilai pembulatan harus bernilai 0 atau lebih'`; `null`/undefined/`0` → null (= tanpa pembulatan) |
| E-08 | Kelipatan tanpa mode / mode tanpa kelipatan | Keduanya independen: mode ≠ none + kelipatan null → tanpa pembulatan diam-diam (→ KI-73) |
| E-09 | Persen raksasa (mis. 1000%) | Lolos (hanya ≥0 dicek) — negatif hasilnya ditolak saat hitung, bukan saat simpan (→ KI-76) |
| E-10 | Update tanpa id / id 0/negatif/pecahan | `400 'Jenis member tidak valid'` sebelum query (ada test) |
| E-11 | Update/archive id tak dikenal atau terarsip | `404 'Jenis member tidak ditemukan'` |
| E-12 | Update kode ke milik rule lain (non-arsip) | `409`; ke kode sendiri → lolos (dikecualikan by id) — kode BISA diubah di sini (beda dengan pihak/kategori) |
| E-13 | Nonaktifkan dipakai 1 pelanggan aktif | `400 '...masih digunakan oleh 1 pelanggan aktif...'` (ada test; save+audit batal) |
| E-14 | Arsip dipakai 2 pelanggan aktif | `400 '...masih digunakan oleh 2 pelanggan aktif...'` (ada test) |
| E-15 | Nonaktifkan yang sudah nonaktif | Lolos (guard hanya saat transisi ke inactive) |
| E-16 | Ubah rule yang dipakai (harga/basis/pembulatan) | **Lolos selalu** — order lama tak tersentuh (snapshot), order baru memakai rule baru. Tanpa peringatan, tanpa re-price draft |
| E-17 | Quote `id_member_type: -1` / `"abc"` / `0` | `0`/`""`/null/undefined = diabaikan (lanjut ke pihak); selain itu tak valid → `400 '{Jenis member\|Customer} tidak valid'` (ada test untuk -1) |
| E-18 | Quote dua-duanya dikirim | `id_member_type` menang; pihak diabaikan (→ KI-72) |
| E-19 | Quote member id tak dikenal / nonaktif / arsip / lain-perusahaan | `404 'Jenis member tidak ditemukan'` / `400 'Jenis member tidak aktif'` |
| E-20 | Quote pihak tak dikenal / bukan customer / terarsip | `404 'Customer tidak ditemukan'` |
| E-21 | Quote pihak tanpa member / member nonaktif/arsip | **Sukses** dengan `member_type: null` + harga standard per item (degradasi diam-diam — → KI-72) |
| E-22 | Quote produk tak dikenal / lain-perusahaan / terarsip | Item `pricing_source: 'manual'`, `unit_price: null`, `error: 'Produk tidak ditemukan'` |
| E-23 | Quote hitung gagal (basis kosong, negatif, konversi) | Item `pricing_source: 'member_rule'`, `unit_price: null`, `error: <pesan>`; item lain tetap dihitung |
| E-24 | Quote tanpa item / tanpa keduanya | `items: []`, `member_type` sesuai resolusi (tetap 200) |
| E-25 | Basis beli dengan faktor 0 | `400 "Konversi satuan {beli\|jual} produk '{nama}' tidak valid"` (ada test untuk beli-0) |
| E-26 | Hasil negatif (diskon > basis) | `400 "Harga member untuk produk '{nama}' menjadi negatif"` (ada test: 1000 − 2000) |

---

## 4. Katalog Pesan (teks apa adanya)

**Error API:** `'Kode jenis member wajib diisi'` · `'Nama jenis member wajib diisi'` ·
`'Basis harga tidak valid'` · `'Aksi harga tidak valid'` · `'Tipe nilai tidak valid'` ·
`'Pembulatan tidak valid'` · `'Status jenis member tidak valid'` ·
`'Besaran rule member harus bernilai 0 atau lebih'` · `'Nilai pembulatan harus bernilai 0 atau lebih'` ·
`'Jenis member tidak valid'` (id) · `'Jenis member tidak ditemukan'` · `'Jenis member tidak aktif'` ·
`"Kode jenis member '{k}' sudah digunakan"` ·
`"Tidak bisa {mengarsipkan\|menonaktifkan} jenis member yang masih digunakan oleh {n} pelanggan aktif. Lepaskan atau ganti jenis member pelanggan terlebih dahulu."` ·
`'Customer tidak valid'` · `'Customer tidak ditemukan'` · `'Produk tidak ditemukan'` (per-item quote) ·
`"Produk '{nama}' belum memiliki {harga jual normal\|harga jual minimum\|harga beli}"` ·
`"{label} produk '{nama}' tidak valid"` (negatif) ·
`"Konversi satuan {beli\|jual} produk '{nama}' tidak valid"` ·
`"Harga member untuk produk '{nama}' menjadi negatif"` · `'Harga member tidak dapat dihitung'` (fallback).

**Toast:** `"Jenis member dibuat"` / `"Jenis member diperbarui"` / `"Jenis member diarsipkan"` ·
`"Gagal menyimpan jenis member"` + pesan (fallback `"Periksa aturan harga member dan coba lagi"`) ·
`"Gagal mengarsipkan jenis member"` + pesan (fallback `"Jenis member belum dapat diarsipkan"`).

**Quote gagal total (jaringan/server):** POS `"Harga member belum bisa dihitung. Periksa koneksi atau
data harga master produk."`; error per produk: `"{nama}: {pesan}"` diagregasi jadi banner
(`formatPricingQuoteError`/`formatPriceQuoteError`).

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Restore / hapus permanen member | Tanpa endpoint; arsip final |
| NF-02 | Member per produk/kategori/cabang/periode | Rule global; tanpa tabel pengecualian |
| NF-03 | Batas atas besaran/persen | Hanya `≥ 0`; 1000% tersimpan |
| NF-04 | Re-price order lama/draft saat rule berubah | Tanpa job/trigger; snapshot final |
| NF-05 | Quote tanpa `order.create` | Guard di controller (kasir tanpa izin ini tak dapat harga member otomatis) |
| NF-06 | Paginasi/filter buku? — n/a | List: cari + status; tanpa filter basis/aksi |
| NF-07 | Tab Sampah member | Arsip hilang dari `status: all` sekalipun (filter arsip selalu `IS NULL`) |
| NF-08 | Riwayat perubahan rule | Audit ada (before+after penuh) tetapi tanpa halaman riwayat di modul ini |
| NF-09 | Validasi `rounding_increment > 0` bermakna | `0` = null (tanpa pembulatan); negatif ditolak; tanpa batas atas |
| NF-10 | Guard member untuk purchase order | `resolveLinePricing` non-sales selalu manual (modul Order) |
