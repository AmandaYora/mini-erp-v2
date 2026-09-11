# Numbering & Sequence — Modul 02 Users, Roles & Permissions

**Kelompok A.** Dokumen ini mencatat format penomoran dan pembuatan kode yang dipakai modul ini.

---

## 1. Kesimpulan: Tidak Ada Penomoran Dokumen

Modul ini tidak menghasilkan dokumen bisnis, sehingga tidak ada nomor dokumen yang perlu
direplikasi. Diverifikasi:

| Yang diperiksa | Hasil |
|---|---|
| Pemakaian `BranchDocumentSequence` | Tidak ada di modul ini |
| Pemakaian `FinanceDocumentSequence` | Tidak ada |
| Kunci sequence terkait user/role | Tidak ada |
| Kolom bernomor yang tampil ke user | Tidak ada |

**Namun modul ini punya satu generator kode yang berperilaku seperti penomoran** dan hasilnya
**terlihat user** — yaitu kode role. Itu dibahas di §2 dan **wajib** direplikasi persis.

---

## 2. Generator Kode Role (WAJIB direplikasi persis)

Setiap role punya `code` yang menjadi identitas fungsionalnya: dipakai di seluruh sistem untuk
menetapkan role ke user, memeriksa kebijakan `superadmin`, menampilkan label, dan sebagai kunci
API. **Kode ini tidak pernah diisi manual oleh user** — form Role & Akses hanya punya field
"Nama role" dan "Deskripsi".

### 2.1 Aturan transformasi

Diterapkan pada `code` bila dikirim, atau pada `name` bila `code` tidak dikirim (jalur yang
dipakai UI):

| # | Langkah | Efek |
|---|---|---|
| 1 | Buang spasi di awal & akhir | `"  Kepala Gudang  "` → `"Kepala Gudang"` |
| 2 | Ubah ke huruf kecil semua | `"Kepala Gudang"` → `"kepala gudang"` |
| 3 | Ganti **setiap rentetan** karakter non-`a-z0-9` menjadi satu garis bawah | `"kepala gudang"` → `"kepala_gudang"` |
| 4 | Buang garis bawah di awal & akhir | `"_kepala_gudang_"` → `"kepala_gudang"` |

Langkah 3 memakai pola `[^a-z0-9]+` dengan pengganti `_` — perhatikan **`+`**: beberapa karakter
non-alfanumerik berurutan menjadi **satu** garis bawah, bukan beberapa.

### 2.2 Contoh hasil (dapat diuji langsung)

| Masukan nama role | Kode yang dihasilkan |
|---|---|
| `Kepala Gudang` | `kepala_gudang` |
| `Keuangan` | `keuangan` |
| `Kasir` | `kasir` |
| `Admin Operasional` | `admin_operasional` |
| `Kepala   Gudang` (spasi ganda) | `kepala_gudang` |
| `Kepala-Gudang` | `kepala_gudang` |
| `Kepala Gudang!` | `kepala_gudang` |
| `  Staf Toko  ` | `staf_toko` |
| `Sales & Marketing` | `sales_marketing` |
| `Gudang 2` | `gudang_2` |
| `Área Manager` (huruf beraksen) | `rea_manager` — **huruf `Á` hilang**, bukan diubah jadi `a` |
| `Manajer Área` | `manajer_rea` |
| `!!!` | `` (kosong) → **ditolak** |
| `   ` (hanya spasi) | `` (kosong) → **ditolak** |
| `管理者` (non-Latin) | `` (kosong) → **ditolak** |

Tiga baris terakhir menghasilkan kode kosong dan ditolak dengan `409` **"Kode role tidak valid"**.

Dua baris beraksen layak diperhatikan: karakter di luar `a-z0-9` **dibuang** (diganti garis bawah
lalu ikut terpangkas), bukan ditransliterasi. Untuk nama role berbahasa Indonesia hal ini praktis
tidak muncul, tetapi bila sistem baru perlu mendukung nama beraksen atau non-Latin, aturan ini
harus diputuskan ulang — **[PERLU KONFIRMASI]**, karena mengubahnya mengubah kode yang dihasilkan.

### 2.3 Konsekuensi: dua nama berbeda bisa bentrok

Karena transformasi bersifat "banyak-ke-satu", nama yang berbeda dapat menghasilkan kode sama:

| Nama pertama | Nama kedua | Kode | Hasil |
|---|---|---|---|
| `Kepala Gudang` | `kepala-gudang` | `kepala_gudang` | Yang kedua **ditolak** |
| `Kepala Gudang` | `KEPALA GUDANG` | `kepala_gudang` | Yang kedua **ditolak** |
| `Sales & Marketing` | `Sales Marketing` | `sales_marketing` | Yang kedua **ditolak** |

Pesan penolakan: `Kode role '{kode}' sudah digunakan`.

Yang terlihat user adalah nama yang **tampak berbeda** ditolak sebagai duplikat, tanpa penjelasan
bahwa penyebabnya adalah kode yang sama di belakang layar. Pesan menyebut kodenya, tapi user
tidak pernah melihat field kode di form sehingga hubungannya tidak jelas. Lihat
[known-issues.md](../known-issues.md) KI-23.

### 2.4 Aturan keunikan

Kode diperiksa terhadap **dua ruang nama sekaligus**:

```
role perusahaan ini (id_company = perusahaan aktif)
    ATAU
role sistem global (id_company kosong)
```

Artinya perusahaan **tidak boleh** membuat role kustom berkode `admin`, `owner`, `staff`, atau
`superadmin` — keempatnya sudah dipakai role sistem. Nama seperti "Admin Gudang" tetap boleh
(kodenya `admin_gudang`).

### 2.5 Kode tidak pernah berubah setelah dibuat

`roles/update` hanya menerima `name` dan `description`; `code` dipakai sebagai **kunci pencarian**,
bukan nilai yang bisa diubah. Jadi:

- Mengganti nama role dari "Kepala Gudang" menjadi "Manajer Gudang" **tidak** mengubah kodenya
  (`kepala_gudang`)
- Kode yang tampil di kolom kedua tabel role (lewat `formatRoleLabel`) akan **tetap** membaca
  "Kepala Gudang" meski namanya sudah "Manajer Gudang"

Ini perilaku yang bisa membingungkan, tetapi juga yang menjaga stabilitas: penetapan role di
`user_roles` merujuk `id_role`, dan pemeriksaan kebijakan merujuk `code` — keduanya tidak terganggu
oleh perubahan nama.

---

## 3. Identitas Teknis Lain

Ketiganya internal dan tidak pernah tampil sebagai nomor dokumen ke user.

| Identitas | Bentuk | Catatan |
|---|---|---|
| `users.id_user` | `INT AUTO_INCREMENT` | Muncul di URL form edit (`/users/{id}/edit`) — jadi **terlihat user** di address bar |
| `roles.id_role` | `INT AUTO_INCREMENT` | Role sistem memakai id tetap **1–4**, role `kasir` memakai id tetap **5** (ditanam seed) |
| `permissions.id_permission` | `INT AUTO_INCREMENT` | Tidak pernah dipakai di API; API selalu memakai `permission_code` |

Dua hal yang perlu diketahui:

**Id role bawaan bersifat tetap dan ditanam eksplisit.** Seed menyisipkan
`(1, superadmin)`, `(2, owner)`, `(3, admin)`, `(4, staff)`, `(5, kasir)` dengan id keras. Beberapa
migrasi juga merujuk id ini secara langsung. Jadi id role bawaan adalah bagian kontrak data, bukan
nilai yang dihasilkan sistem.

**Kode permission adalah kunci publik API.** Berbeda dari role yang dirujuk lewat `code` di API
tetapi `id_role` di tabel relasi, permission dirujuk lewat `permission_code` di API
(`roles/permissions/update` mengirim array kode). Format kodenya:

```
<module_key>.<action_key>
```

dengan `module_key` yang dapat memuat titik untuk sub-area, mis. `finance.posting` + `post` →
`finance.posting.post`. Ini menjelaskan mengapa beberapa permission punya tiga segmen. Rincian
seluruh 60 kode ada di
[shared-data-model.md](../shared/shared-data-model.md) §5.2.

---

## 4. Format Nomor WhatsApp

Satu-satunya format nilai lain yang layak dicatat karena tersimpan sebagai teks bebas dan
**terlihat user**.

| Aspek | Kondisi |
|---|---|
| Nama kolom | `phone_e164` — menyiratkan format E.164 |
| Label di form | **"No. WhatsApp"** |
| Validasi | **Tidak ada** — tidak di klien, tidak di server |
| Format seed | `6281100000001` (tanpa `+`, tanpa spasi, tanpa tanda hubung) |
| Nilai kosong | Disimpan sebagai `NULL` |

Nama kolom menjanjikan E.164 tetapi tidak ada satu pun pemeriksaan yang menegakkannya. Pengguna
dapat menyimpan `0812-3456-7890`, `+62 812 3456 7890`, atau teks bebas apa pun.

Ini berdampak ke luar modul: nomor ini kemungkinan dipakai integrasi WhatsApp (modul 21), yang
menuntut format tertentu. **[PERLU KONFIRMASI]** apakah nomor pengguna di sini benar-benar dipakai
untuk pengiriman WhatsApp, atau hanya catatan kontak — bila dipakai, ketiadaan validasi format
adalah sumber kegagalan kirim yang sulit didiagnosis. Ditelusuri saat analisis modul 21.
