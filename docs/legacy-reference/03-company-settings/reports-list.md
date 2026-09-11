# Reports List — Modul 03 Company & Settings

**Kelompok A.** Dokumen ini mencatat laporan yang dihasilkan modul ini beserta logika
perhitungannya.

---

## 1. Kesimpulan: Modul Ini Tidak Menghasilkan Laporan

**Tidak ada laporan di modul Company & Settings.** Diverifikasi:

| Yang diperiksa | Hasil |
|---|---|
| 6 endpoint modul | Semuanya baca/tulis konfigurasi tunggal — tidak ada agregat, periode, atau perhitungan |
| Halaman frontend | 2 halaman pengaturan; keduanya form, bukan tampilan laporan |
| Permission bertipe laporan | Tidak ada. Modul memakai `company_config.view` / `.manage` |
| Menu sidebar | Kedua entri masuk grup **"Pengaturan"**, bukan "Pantauan" |
| Endpoint ekspor | Tidak ada |
| Halaman cetak | Tidak ada — modul ini **mengonfigurasi** dokumen cetak, tetapi pencetakannya dilakukan modul Order |
| Paginasi | Tidak ada di satu pun endpoint |

---

## 2. Dua Angka yang Ditampilkan

Untuk kelengkapan, inilah seluruh angka yang muncul di modul ini:

| Angka | Rumus | Tampil di |
|---|---|---|
| Jumlah cabang yang dapat diakses | Jumlah entri akses cabang pengguna yang sedang login | Kartu "Cabang" di `/settings` |
| Kuota AI harian | Nilai tersimpan, sudah dinormalisasi ke rentang 1–500 | Kartu "Kuota AI Harian" |

Angka pertama layak dicatat: ia menghitung cabang yang dapat diakses **pengguna yang sedang
login**, bukan seluruh cabang perusahaan. Sehingga dua pengguna berbeda melihat angka berbeda di
halaman yang sama, dan angka itu **bukan** statistik perusahaan meski letaknya di kartu berjudul
"Cabang". **[PERLU KONFIRMASI]** apakah maksudnya memang jumlah cabang milik pengguna, atau
seharusnya total cabang perusahaan — label "Jumlah cabang yang dapat diakses" cenderung
menyiratkan yang pertama, jadi kemungkinan besar sesuai maksud.

---

## 3. Endpoint yang Ada Tetapi Tidak Dipakai UI

Temuan yang perlu diketahui sebelum rebuild: **empat dari enam endpoint modul ini tidak pernah
dipanggil oleh antarmuka.**

| Endpoint | Dipanggil UI? | Pemakaian nyata yang ditemukan |
|---|---|---|
| `company/profile/update` | ✔ | Halaman Profil Perusahaan |
| `company/settings/update` | ✔ | Halaman Profil Perusahaan |
| `company/profile/get` | **✘** | Tidak ada pemanggil sama sekali |
| `company/settings/get` | **✘** | Hanya **test E2E** modul Stock (untuk membaca-lalu-memulihkan aturan operasional) |
| `company/features/list` | **✘** | Tidak ada pemanggil sama sekali |
| `company/features/update` | **✘** | Tidak ada pemanggil sama sekali |

Penyebab dua yang pertama: halaman Profil Perusahaan mengisi formnya dari **data yang sudah ada di
browser** — profil dari ringkasan sesi, pengaturan dari bootstrap aplikasi. Jadi endpoint baca
tidak dibutuhkan.

Konsekuensi yang perlu diketahui: **nilai di form bisa basi.** Bila profil perusahaan diubah dari
sesi lain (atau langsung di basis data), pengguna yang sudah login akan melihat nilai lama sampai
ia memuat ulang aplikasi — karena ringkasan sesi hanya diperbarui saat login, pemulihan sesi, atau
setelah ia sendiri menyimpan.

Untuk `company/features/*`, penyebabnya lebih dalam: adapter frontend **selalu memaksa** daftar
feature flag menjadi kosong, sehingga memanggil endpointnya pun tidak akan berpengaruh. Sistem
feature flag mati di tiga lapisan sekaligus — tanpa UI, tanpa pembaca, dan tanpa jalur data.

---

## 4. Modul Ini Memasok Konfigurasi untuk Modul Lain

Meski tidak menghasilkan laporan, modul ini menentukan **isi dokumen cetak** yang dihasilkan modul
lain. Ini yang perlu diketahui agar tidak salah kira letak fiturnya.

### 4.1 Identitas dokumen cetak → modul Order

Seluruh isi kartu "Identitas Dokumen Cetak" dan "Kertas Kontinu Nota" dibaca modul Order saat
merender halaman cetak:

| Yang diatur di sini | Muncul di |
|---|---|
| Nama di kepala dokumen | Kop nota & surat jalan (fallback: nama perusahaan) |
| Alamat di kepala dokumen | Kop nota & surat jalan (fallback: alamat cabang) |
| Baris lokasi singkat | Struk POS |
| Kota tanda tangan | Baris tanda tangan surat jalan (fallback: kota cabang) |
| Daftar nomor telepon | Kop nota |
| Daftar rekening transfer | Blok rekening di nota |
| Daftar barang yang dijual | Blok "MENJUAL" di nota |
| Catatan nota | Catatan di kaki nota & struk |
| Ucapan penutup | Penutup struk |
| Kalibrasi kertas kontinu | Ukuran `@page` dan margin saat mencetak Nota |

Halaman cetak yang mengonsumsinya: nota penjualan, struk POS termal, surat jalan, dan kwitansi
pembayaran — semuanya milik modul Order (analisis modul 08–11 dan 15).

### 4.2 Kebijakan approval koreksi stok → modul Stock

Setelan mode persetujuan dibaca **ulang setiap kali** koreksi stok divalidasi. Efeknya
terverifikasi lewat test E2E: dengan mode `strict`, persetujuan oleh orang yang sama dengan
pembuat koreksi **ditolak**.

### 4.3 Kuota & mode asisten → modul WhatsApp

Tiga nilai di blok preferensi asisten dibaca modul WhatsApp: batas jawaban AI harian, mode
jawaban (berbasis aturan atau AI), dan preferensi perilaku bot.

### 4.4 Audit → modul Observability

Tiga `actionKey` yang ditulis modul ini tampil di halaman **Riwayat Aktivitas**:

| `actionKey` | Kelengkapan catatan |
|---|---|
| `company.profile.update` | **Lengkap** — keenam field identitas di `before` dan `after` |
| `company.settings.update` | **Lengkap** — kelima blok di `before` dan `after` |
| `company.feature.update` | Isi lengkap, tetapi **`idEntity` menunjuk perusahaan, bukan baris flag** |

Dua yang pertama adalah contoh audit paling lengkap di seluruh modul yang sudah dianalisis — pola
yang seharusnya dijadikan acuan (bandingkan `user.update` di modul 02 yang hanya mencatat satu
field di `after`).

Untuk `company.settings.update`, kelengkapan itu berarti riwayat aktivitas **dapat** menjawab
"siapa mengubah mode persetujuan koreksi stok, dari apa ke apa" — pertanyaan pengawasan yang nyata,
mengingat setelan itu melemahkan kebijakan approval seluruh perusahaan.

---

## 5. Bahan Mentah bila Laporan Ingin Dibuat

Dicatat sebagai inventaris data, **bukan** usulan fitur.

| Pertanyaan operasional | Bisa dijawab sekarang? |
|---|---|
| Siapa mengubah profil perusahaan, kapan, dari apa ke apa? | **Ya** — audit `company.profile.update` mencatat lengkap |
| Siapa mengubah mode persetujuan koreksi stok? | **Ya** — audit `company.settings.update` mencatat kelima blok |
| Kapan identitas dokumen cetak terakhir diubah? | **Ya** — dari audit, atau dari waktu perubahan baris pengaturan |
| Feature flag mana yang aktif? | **Secara data ya, secara arti tidak** — nilainya tersimpan tetapi tidak memengaruhi apa pun |
| Riwayat perubahan kalibrasi kertas | **Ya** — ikut terekam di dalam blok preferensi UI pada audit |

Kolom waktu yang tersedia: `companies.created_at`/`updated_at`, `company_settings.updated_at`
(tanpa `created_at`), `company_features.created_at`/`updated_at`. Tidak satu pun ditampilkan di UI.

Yang **tidak** tersedia: riwayat versi pengaturan di luar audit log (tidak ada tabel versi), dan
tidak ada cara mengembalikan pengaturan ke kondisi sebelumnya selain mengetiknya ulang dari isi
audit.

---

## 6. Modul Lain yang Menampilkan Data Bernuansa Perusahaan

Agar tidak salah kira letak fiturnya:

| Fitur | Modul pemilik |
|---|---|
| Chip **"Perusahaan"** di topbar | Lapisan shell aplikasi — membaca ringkasan sesi, bukan endpoint modul ini |
| Daftar & pengelolaan cabang (`/branches`) | Modul 04 Branch |
| Halaman **Role & Akses** (`/settings/roles`) | Modul 02 — meski berkasnya ada di folder web modul ini |
| Halaman cetak nota/struk/surat jalan | Modul Order (08–11) & POS (15) |
| Halaman **Pengaturan Keuangan** (`/finance/settings`) | Modul 17 Finance — pengaturan terpisah, bukan bagian modul ini |
| Halaman **Riwayat Aktivitas** (`/audit-logs`) | Modul 20 Observability |

Perhatikan baris terakhir sebelum audit: `/finance/settings` adalah halaman pengaturan **lain**
yang tidak berada di bawah `/settings`. Jadi pengaturan sistem tersebar di dua tempat dengan pola
URL berbeda. **[PERLU KONFIRMASI]** apakah pemisahan itu disengaja (keuangan dianggap domain
sendiri) — bila ya, tidak perlu diubah, tetapi perlu dicatat agar navigasi sistem baru tidak
membingungkan.
