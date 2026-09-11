# Numbering & Sequence — Modul 03 Company & Settings

**Kelompok A.** Dokumen ini mencatat format penomoran dan pengodean yang dipakai modul ini.

---

## 1. Kesimpulan: Tidak Ada Penomoran Dokumen

Modul ini tidak menghasilkan dokumen bisnis. Diverifikasi:

| Yang diperiksa | Hasil |
|---|---|
| Pemakaian sequence dokumen cabang | Tidak ada di modul ini |
| Pemakaian sequence dokumen finance | Tidak ada |
| Kunci sequence terkait perusahaan/pengaturan | Tidak ada |
| Nomor yang dihasilkan otomatis | Tidak ada |

**Namun modul ini memegang dua identitas berkode yang terlihat pengguna** — kode perusahaan dan
kode status order — dan keduanya berperilaku **berbeda** dari kode role di modul 02. Perbedaan itu
yang perlu dipertahankan.

---

## 2. Kode Perusahaan — Diisi Manual, Tanpa Normalisasi

### 2.1 Perbandingan dengan kode role (modul 02)

Ini kontras yang paling penting dicatat, karena keduanya adalah "kode identitas" tetapi
diperlakukan berlawanan:

| Aspek | Kode **role** (modul 02) | Kode **perusahaan** (modul ini) |
|---|---|---|
| Cara mengisi | **Dihasilkan otomatis** dari nama | **Diketik manual** oleh pengguna |
| Field di form | **Tidak ada** | **Ada**, wajib diisi |
| Normalisasi | Huruf kecil, non-alfanumerik → `_` | **Hanya di-trim** — huruf besar/kecil, spasi tengah, simbol semuanya dipertahankan |
| Boleh diubah setelah dibuat | **Tidak** (hanya kunci pencarian) | **Ya**, bebas diubah |
| Keunikan | Dicek terhadap role perusahaan + role global | Dicek global |

Jadi kode perusahaan adalah **teks bebas dengan syarat unik**, bukan slug yang dihasilkan sistem.

### 2.2 Aturan yang berlaku

| # | Aturan | Detail |
|---|---|---|
| 1 | Wajib diisi | Kosong atau spasi saja → `400` **"Kode perusahaan wajib diisi"** |
| 2 | Di-trim | Spasi di awal & akhir dibuang |
| 3 | Unik secara global | Bentrok → `409` **"Kode perusahaan sudah digunakan"** |
| 4 | Maksimal 50 karakter | Ditegakkan **hanya oleh kolom basis data** — melebihinya menghasilkan `500`, bukan pesan bisnis |
| 5 | Boleh diubah kapan saja | Tidak ada penjagaan "kode tidak boleh berubah" |

### 2.3 Contoh hasil (dapat diuji langsung)

| Masukan | Tersimpan sebagai | Catatan |
|---|---|---|
| `TSAK` | `TSAK` | Nilai bawaan seed |
| `  TSAK  ` | `TSAK` | Di-trim |
| `tsak` | `tsak` | **Huruf kecil dipertahankan** — berbeda dari kode role |
| `TB SUMBER ABADI` | `TB SUMBER ABADI` | **Spasi tengah dipertahankan** |
| `TSAK-01` | `TSAK-01` | Simbol dipertahankan |
| `TB. SUMBER!` | `TB. SUMBER!` | Titik dan tanda seru dipertahankan |
| `   ` | *(ditolak)* | `400` "Kode perusahaan wajib diisi" |
| 51 karakter atau lebih | *(gagal teknis)* | `500` dari basis data |

Konsekuensi yang perlu diketahui: karena tidak ada normalisasi huruf besar/kecil, **`TSAK` dan
`tsak` dianggap dua kode berbeda** oleh aplikasi. Apakah basis data menganggapnya bentrok
bergantung collation kolom — kolom memakai `utf8mb4_unicode_ci` yang **tidak peka huruf besar/
kecil**, sehingga indeks unik akan menolaknya sebagai duplikat sementara pemeriksaan aplikasi
(yang membandingkan string apa adanya) meloloskannya. Untuk sistem satu-perusahaan kondisi ini
tidak pernah tercapai, tetapi ia adalah ketidaksesuaian nyata antara penjagaan aplikasi dan
penjagaan basis data.

### 2.4 Kode perusahaan dipakai di mana

| Konsumen | Cara dipakai |
|---|---|
| Ringkasan sesi di browser | Disimpan sebagai bagian identitas perusahaan |
| Prefix nomor dokumen | **Tidak** — prefix dokumen memakai kode **cabang**, bukan kode perusahaan (lihat modul 04) |
| Tampilan UI | Tidak ditampilkan di mana pun selain form pengaturan itu sendiri |

Temuan yang perlu diketahui: **kode perusahaan tidak dipakai untuk apa pun yang terlihat.** Ia
wajib diisi dan dijaga keunikannya, tetapi tidak muncul di dokumen, tidak menjadi prefix nomor,
dan tidak ditampilkan di antarmuka. Nomor dokumen memakai kode **cabang** (mis. `ORD-BLR/2026/00001`
dari cabang berkode `BLR`).

**[PERLU KONFIRMASI]** apakah kode perusahaan punya kegunaan di luar sistem (mis. dipakai di
dokumen manual, atau direncanakan untuk integrasi). Bila tidak, ia kandidat untuk disederhanakan
di sistem baru.

---

## 3. Kode Status Order — Diisi Manual, Tanpa Normalisasi

Halaman `/settings/order-status` menuntut pengguna mengetik **kode unik** sendiri, dengan
placeholder yang mengajarkan formatnya:

| Aspek | Detail |
|---|---|
| Label field | **"Kode unik"** |
| Placeholder | **"Contoh: waiting_confirmation"** — menyarankan `snake_case` |
| Normalisasi | **Tidak ada** — nilai dikirim apa adanya, bahkan tanpa trim di klien |
| Pemeriksaan duplikat | **Di klien**, saat mengetik, terhadap daftar yang sudah dimuat |
| Pesan duplikat | **"Kode ini sudah digunakan. Gunakan kode lain."** (teks merah di bawah field) |
| Tombol submit | Dinonaktifkan bila kode kosong (setelah trim), nama kosong, atau kode duplikat |

Perhatikan asimetri: tombol dinonaktifkan bila kode **kosong setelah trim**, tetapi nilai yang
dikirim **tidak di-trim**. Sehingga mengetik `" draft "` (dengan spasi) melewati penjagaan dan
tersimpan dengan spasi di dalamnya.

### 3.1 Kode status bawaan (seed)

Lima status di-seed dengan kode `snake_case`:

| Kode | Label | Kelompok | Titik awal | Titik akhir | Urutan | Warna |
|---|---|---|---|---|---|---|
| `draft` | Draft | pending | ✔ | ✘ | 1 | `#9CA3AF` |
| `confirmed` | Dikonfirmasi | active | ✘ | ✘ | 2 | `#3B82F6` |
| `in_progress` | Diproses | active | ✘ | ✘ | 3 | `#F59E0B` |
| `completed` | Selesai | completed | ✘ | ✔ | 4 | `#10B981` |
| `cancelled` | Dibatalkan | cancelled | ✘ | ✔ | 5 | `#EF4444` |

Kelimanya berlaku untuk **semua** jenis order.

### 3.2 Status yang dibuat lewat UI berbeda dari yang di-seed

Status baru **selalu** memakai nilai tetap yang tidak bisa dipilih pengguna:

| Properti | Status seed | Status baru dari UI |
|---|---|---|
| Kelompok | bervariasi | **selalu `pending`** |
| Titik awal / akhir | ditentukan | **selalu bukan keduanya** |
| Warna | palet 5 warna | **selalu `#12343B`** |
| Urutan tampil | 1–5 berurutan | **jumlah status + 1** |

Warna `#12343B` **tidak ada** di palet seed — sehingga status buatan pengguna langsung terlihat
berbeda (biru-hijau gelap) dari kelima status bawaan. Ini kemungkinan tidak disengaja.

### 3.3 Urutan tampil dihitung dari jumlah, bukan nilai maksimum

`urutan = jumlah status saat ini + 1`.

| Kondisi | Hasil |
|---|---|
| 5 status (urutan 1–5), tambah satu | Urutan **6** — benar |
| Status urutan 3 dihapus di basis data (sisa 4 status: 1,2,4,5), tambah satu | Urutan **5** — **bentrok** dengan status yang sudah ada |

Tidak ada penjagaan terhadap urutan duplikat, dan daftar diurutkan berdasarkan kolom itu —
sehingga urutan dua status yang bentrok menjadi tidak dapat diprediksi.

Karena halaman ini **tidak punya fungsi hapus**, kondisi itu hanya tercapai bila status dihapus
langsung di basis data. Tetap dicatat karena rumusnya rapuh.

---

## 4. Identitas Teknis Lain

| Identitas | Bentuk | Catatan |
|---|---|---|
| `companies.id_company` | `INT AUTO_INCREMENT` | Seed menanam **`1`** secara eksplisit; sistem berjalan single-company |
| `company_settings.id_company` | **Kunci utama = id perusahaan** | Relasi satu-ke-satu; tidak punya id sendiri |
| `company_features.id_company_feature` | `INT AUTO_INCREMENT` | Punya id sendiri, tetapi **tidak pernah dipakai** — API mengalamatkan flag lewat `feature_key`, dan audit malah mencatat id perusahaan (lihat KI-32) |
| `company_features.feature_key` | Teks bebas, maks 100 karakter | **Kunci publik API.** Unik per pasangan (perusahaan, kunci). Tidak ada daftar kunci yang sah — kunci apa pun bisa dibuat |

Tiga kunci flag yang di-seed memakai `snake_case`: `whatsapp_assistant`, `knowledge_rag`,
`multi_location_stock`. Karena tidak ada validasi, kunci dengan format lain juga akan diterima —
tidak ada yang menegakkan konvensi itu.

---

## 5. Format Nilai Lain yang Terlihat Pengguna

Dicatat karena tersimpan sebagai teks bebas tanpa validasi, dan salah format tidak menimbulkan
gejala apa pun (karena nilainya tidak dibaca — lihat
[business-rules.md](business-rules.md) BR-19).

| Field | Format yang diharapkan | Divalidasi? | Nilai seed |
|---|---|---|---|
| Zona waktu | Zona IANA, mis. `Asia/Jakarta` | **Tidak** | `Asia/Jakarta` |
| Mata uang | Kode ISO 4217, mis. `IDR` | **Tidak** — hanya diubah ke huruf besar | `IDR` |
| Bahasa sistem | Tag BCP 47, mis. `id-ID` | **Tidak** | `id-ID` |

Ketiganya adalah field **teks bebas** di form — bukan dropdown. Pengguna harus mengetik formatnya
dengan benar sendiri, tanpa contoh, tanpa placeholder, dan tanpa umpan balik bila salah.

Untuk kalibrasi kertas kontinu, format nilainya adalah **milimeter dengan satu desimal**
(`step="0.1"`), dan nilai default ditampilkan sebagai placeholder di setiap kotak. Rincian angka
default beserta alasan tiap angkanya ada di
[shared-data-model.md](../shared/shared-data-model.md) §7.3.
