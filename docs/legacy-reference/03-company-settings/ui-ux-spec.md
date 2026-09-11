# UI/UX Spec — Modul 03 Company & Settings

**Kelompok A — kontrak tampilan yang harus SAMA PERSIS di sistem baru.** Semua teks, urutan
field, dan placeholder dikutip apa adanya dari kode.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md)

Seluruh UI modul ini **berbahasa Indonesia** tanpa pengecualian. Token warna dan komponen bersama
dijelaskan di [shared-services.md](../shared/shared-services.md) §5 dan
[01-auth-session/ui-ux-spec.md](../01-auth-session/ui-ux-spec.md) §1.

---

## 1. Peta Screen

| Rute | Judul halaman | Permission | Menu sidebar |
|---|---|---|---|
| `/settings` | Profil Perusahaan | **`company_config.view`** | Pengaturan → "Profil Perusahaan" (ikon `company_profile`, `end: true`) |
| `/settings/order-status` | Status Order | **`company_config.manage`** | Pengaturan → "Status Order" (ikon `order_status`) |

Dua hal yang perlu dicatat:

- Menu "Profil Perusahaan" memakai **`end: true`** (pencocokan persis), sehingga ia **tidak**
  tersorot ketika pengguna berada di `/settings/order-status` atau `/settings/roles`.
- Halaman Status Order menuntut **`company_config.manage`**, bukan `.view` — jadi pemegang izin
  baca-saja **tidak bisa membukanya sama sekali**, meski halaman itu hanya menampilkan daftar di
  separuh layarnya.

---

## 2. Screen: `/settings` — Profil Perusahaan

Struktur vertikal `gap-6`: PageHeader → satu `<form>` berisi 5 blok → ActionRow.

### 2.1 PageHeader

| Elemen | Isi |
|---|---|
| Breadcrumb | **"Dashboard > Profil Perusahaan"** |
| Judul | **"Profil Perusahaan"** |
| Deskripsi | **"Pengaturan perusahaan yang berlaku lintas cabang."** |
| Aksi | *(tidak ada)* |

### 2.2 Kondisi render kosong

Halaman merender **kosong sama sekali** (bukan indikator memuat, bukan pesan) bila salah satu dari
data perusahaan, data pendamping perusahaan, atau pengaturan belum tersedia. Pengguna melihat
layar kosong di bawah topbar sampai data selesai dimuat.

### 2.3 Baris 1 — dua kartu berdampingan

Grid `repeat(auto-fit, minmax(min(100%, 420px), 1fr))`, `gap-6` — menumpuk vertikal di layar
sempit.

#### Kartu kiri: "Profil Perusahaan"

| Elemen | Isi |
|---|---|
| Judul kartu | **"Profil Perusahaan"** |
| Deskripsi | **"Identitas bisnis utama."** |
| Grid isi | `repeat(auto-fit, minmax(min(100%, 320px), 1fr))`, `gap-5` |

**Urutan field (WAJIB dipertahankan):**

| # | Label | Wajib | Placeholder | Catatan |
|---|---|---|---|---|
| 1 | **"Kode perusahaan"** | ✔ | — | |
| 2 | **"Nama perusahaan"** | ✔ | — | |
| 3 | **"Nama legal"** | ✘ | **"Opsional"** | Selebar penuh (`col-span-full`) |
| 4 | **"Zona waktu"** | ✔ | — | Input teks bebas, bukan pilihan |
| 5 | **"Mata uang"** | ✔ | — | `maxLength=10`; **diubah ke huruf besar saat mengetik** |
| 6 | **"Bahasa sistem"** | ✔ | — | Input teks bebas, bukan pilihan |

Label ditulis sebagai isi elemen `<label>` yang membungkus input (bukan `htmlFor` terpisah), gaya
0.85rem `font-medium` warna `heading`, jarak `gap-1.5` ke input.

Field 4 dan 6 adalah **input teks bebas** — bukan dropdown daftar zona waktu atau locale. Pengguna
harus mengetik `Asia/Jakarta` dan `id-ID` dengan benar sendiri, dan tidak ada validasi bila salah.

#### Kartu kanan: "Cabang" (baca-saja)

| Elemen | Isi |
|---|---|
| Judul kartu | **"Cabang"** |
| Deskripsi | **"Cabang yang terdaftar dan dapat diakses akun ini."** |

Dua baris statistik, masing-masing label kecil + nilai tebal:

| Label (0.8rem, uppercase, `tracking-[0.05em]`, `muted`) | Nilai (1rem, `font-medium`, `ink`) |
|---|---|
| **"Cabang aktif sekarang"** | Nama cabang aktif; **`-`** bila tidak ada |
| **"Jumlah cabang yang dapat diakses"** | Angka |

Tidak ada tautan ke halaman pengelolaan cabang.

### 2.4 Baris 2 — dua kartu berdampingan

#### Kartu kiri: "Aturan Koreksi Stok"

| Elemen | Isi |
|---|---|
| Judul | **"Aturan Koreksi Stok"** |
| Deskripsi | **"Mengatur cara koreksi stok disetujui."** |
| Label field | **"Siapa yang boleh menyetujui koreksi stok?"** (`id="stock-approval-mode"`) |
| Bentuk | `SelectField` (dropdown dengan pencarian) |
| Placeholder pencarian | **"Cari mode persetujuan..."** |

Dua pilihan, teks persis:

| Nilai | Label yang tampil |
|---|---|
| `simple` | **"Siapapun yang punya izin persetujuan"** |
| `strict` | **"Harus orang berbeda dari yang membuat koreksi"** |

#### Kartu kanan: "Kuota AI Harian"

| Elemen | Isi |
|---|---|
| Judul | **"Kuota AI Harian"** |
| Deskripsi | **"Batas maksimal jawaban berbasis AI yang dapat dikirim bot per hari."** |
| Label field | **"Jumlah maksimal jawaban AI per hari"** (`id="ai-daily-limit"`) |
| Bentuk | `<input type="number">` dengan `min=1`, `max=500` |

### 2.5 Baris 3 — kartu "Identitas Dokumen Cetak" (lebar penuh)

| Elemen | Isi |
|---|---|
| Judul | **"Identitas Dokumen Cetak"** |
| Deskripsi | **"Data ini muncul pada nota, struk, dan surat jalan yang dicetak. Kosongkan bila tidak dipakai."** |

#### Sub-blok 1 — empat field teks (grid `minmax(min(100%,320px),1fr)`)

| # | Label | Placeholder |
|---|---|---|
| 1 | **"Nama di kepala dokumen"** | `Kosongkan untuk memakai "{nama perusahaan}"` — **placeholder dinamis** |
| 2 | **"Alamat di kepala dokumen"** | **"Kosongkan untuk memakai alamat cabang"** |
| 3 | **"Baris lokasi singkat (struk)"** | **"contoh: PEREMPATAN KAMOLAN"** |
| 4 | **"Kota untuk tanda tangan surat jalan"** | **"contoh: BLORA"** |

#### Sub-blok 2 — "Nomor telepon" (daftar dinamis)

Judul sub-blok: `<span>` **"Nomor telepon"** tebal, `mt-4`.

Setiap baris berisi dua input berdampingan (grid `minmax(min(100%,320px),1fr)`):

| Input | Placeholder |
|---|---|
| Label | **"Label, mis. TOKO / HANDOKO"** |
| Nomor | **"Nomor telepon"** |

Di samping input nomor: tombol **"Hapus"** (variant `ghost`, size `sm`) — hanya bila punya izin
kelola.

Di bawah daftar: tombol **"Tambah telepon"** (variant `secondary`, size `sm`).

Baris baru selalu ditambahkan dengan label dan nomor kosong.

#### Sub-blok 3 — "Rekening transfer" (daftar dinamis)

Judul sub-blok: **"Rekening transfer"** tebal, `mt-4`.

Setiap baris berisi tiga input (grid `minmax(280px,1fr)` — perhatikan **tanpa** `min(100%,…)`
seperti sub-blok lain):

| Input | Placeholder |
|---|---|
| Bank | **"Bank, mis. BCA"** |
| Atas nama | **"Atas nama"** |
| Nomor rekening | **"Nomor rekening"** |

Tombol **"Hapus"** di samping input nomor; tombol **"Tambah rekening"** di bawah daftar.

#### Sub-blok 4 — tiga field penutup (`mt-4`)

| # | Label | Bentuk | Placeholder |
|---|---|---|---|
| 1 | **"Daftar barang yang dijual (untuk nota)"** | `<textarea>` `min-h-[80px]`, `resize-y`, `rows=4`, selebar penuh | Multi-baris: `Satu baris satu item, mis.` lalu `- BAHAN BANGUNAN` lalu `- KERAMIK, GRANIT & GRANIT ALAM` |
| 2 | **"Catatan pada nota & struk"** | input teks | **"mis. barang yang sudah dibeli tidak dapat ditukar kembali"** |
| 3 | **"Ucapan penutup struk"** | input teks | **"mis. TERIMA KASIH"** |

### 2.6 Baris 4 — kartu "Kertas Kontinu Nota (Dot-Matrix)" (lebar penuh)

| Elemen | Isi |
|---|---|
| Judul | **"Kertas Kontinu Nota (Dot-Matrix)"** |
| Deskripsi | **Dinamis**, menyertakan angka default: `Kalibrasi ukuran & margin cetak Nota untuk printer dot-matrix (mis. Epson LQ-310) di kertas continuous form 3-ply 9.5" x 11"/2 (1/2 HVS). Default: 241.3mm x 139.7mm — sesuaikan lewat tes cetak fisik bila hasil masih bergeser.` |

Enam input angka, grid `repeat(auto-fit, minmax(min(100%, 200px), 1fr))`, semuanya
`type="number"`, `step="0.1"`, `min={0}`, dan **placeholder berisi angka default**:

| # | Label | Placeholder (default) |
|---|---|---|
| 1 | **"Lebar form (mm)"** | `241.3` |
| 2 | **"Tinggi per lembar (mm)"** | `139.7` |
| 3 | **"Margin atas (mm)"** | `4` |
| 4 | **"Margin bawah (mm)"** | `7` |
| 5 | **"Margin kiri (mm)"** | `6` |
| 6 | **"Margin kanan (mm)"** | `37.91` |

Urutan margin di UI adalah **atas → bawah → kiri → kanan**, bukan urutan CSS biasa
(atas-kanan-bawah-kiri).

Dua checkbox di bawahnya (`mt-4`, `gap-2`):

| # | Teks label | Perilaku |
|---|---|---|
| 1 | **"Jadikan kertas kontinu sebagai default saat cetak Nota"** | Tercentang → tersimpan sebagai default aktif |
| 2 | **"Kertas sudah preprinted (kop, rekening, daftar jual) — jangan cetak ulang"** | **Logika terbalik**: tercentang → kop **tidak** dicetak |

Checkbox kedua menampilkan status tercentang bila nilai tersimpan adalah "jangan tampilkan kop".
Ini pembalikan yang benar secara logika bisnis (label berbunyi negatif), tetapi patut diketahui
saat mereplikasi.

### 2.7 ActionRow

| Kondisi | Isi |
|---|---|
| Punya `company_config.manage` | Teks kiri: **"Perubahan akan langsung berlaku untuk seluruh cabang."** + tombol primary **"Simpan Pengaturan"** |
| Tanpa izin kelola | Teks kiri: **"Anda sedang melihat pengaturan dalam mode baca saja."** — **tanpa tombol** |

Teks kiri berukuran 0.85rem warna `muted`.

### 2.8 Mode baca-saja

Tanpa `company_config.manage`:

- **Semua** input, textarea, select, dan checkbox dinonaktifkan
- Tombol "Hapus", "Tambah telepon", "Tambah rekening" **tidak dirender**
- Tombol "Simpan Pengaturan" **tidak dirender**
- Submit form diabaikan bahkan bila dipicu lewat cara lain

Halaman tetap bisa dibuka karena rutenya hanya menuntut `company_config.view`.

---

## 3. Screen: `/settings/order-status` — Status Order

Struktur: PageHeader → dua kartu berdampingan (daftar + tambah status) → dua kartu berdampingan
(daftar + tambah aturan perpindahan).

### 3.1 PageHeader

| Elemen | Isi |
|---|---|
| Breadcrumb | **"Dashboard > Pengaturan > Status Order"** |
| Judul | **"Status Order"** |
| Deskripsi | **"Status order ini berlaku untuk seluruh cabang."** |

### 3.2 Kartu "Daftar Status"

| Elemen | Isi |
|---|---|
| Judul | **"Daftar Status"** |
| Deskripsi | **"Semua status yang dikenali dalam alur order."** |
| Urutan | Diurutkan berdasarkan **urutan tampil** (`sortOrder`) naik |

Setiap status ditampilkan sebagai kotak (`rounded-md`, border `hairline`, latar `surface`,
padding 4, kelas legacy `list-item`) berisi satu baris `flex-wrap justify-between`:

- **Kiri:** label status (`<strong>`, warna `ink`)
- **Kanan:** rangkaian badge:

| Badge | Kondisi | Tone |
|---|---|---|
| Nama kelompok status | selalu | mengikuti kelompok (lihat tabel di bawah) |
| **"Titik Awal"** | bila status adalah titik awal | `info` |
| **"Titik Akhir"** | bila status adalah titik akhir | `neutral` |

**Terjemahan kelompok status (WAJIB dipertahankan):**

| Nilai | Label yang tampil | Tone badge |
|---|---|---|
| `pending` | **"Menunggu"** | warning |
| `active` | **"Aktif"** | info |
| `completed` | **"Selesai"** | success |
| `cancelled` | **"Dibatalkan"** | danger |
| nilai lain | nilai mentah apa adanya | info |

Empty state:

| Elemen | Isi |
|---|---|
| Judul | **"Belum ada status"** |
| Deskripsi | **"Tambahkan status pertama agar alur order bisa dipakai."** |

### 3.3 Kartu "Tambah Status"

| Elemen | Isi |
|---|---|
| Judul | **"Tambah Status"** |
| Deskripsi | **"Tambahkan titik status baru dalam alur order."** |

| # | Label | `id` | Placeholder | Wajib |
|---|---|---|---|---|
| 1 | **"Nama status"** | `status-label-input` | **"Contoh: Menunggu Konfirmasi"** | ✔ |
| 2 | **"Kode unik"** | `status-code-input` | **"Contoh: waiting_confirmation"** | ✔ |

**Peringatan duplikat langsung:** bila kode yang diketik sudah dipakai status lain, muncul teks di
bawah field kode — 0.82rem, warna `bad`:

> **"Kode ini sudah digunakan. Gunakan kode lain."**

Tombol submit **"Tambah Status"** dinonaktifkan bila: nama kosong (setelah trim), kode kosong
(setelah trim), **atau** kode sudah dipakai.

Setelah submit, kedua field dikosongkan.

### 3.4 Kartu "Aturan Perpindahan"

| Elemen | Isi |
|---|---|
| Judul | **"Aturan Perpindahan"** |
| Deskripsi | **"Urutan perpindahan status yang diizinkan."** |

Setiap aturan sebagai kotak berisi dua baris:

- **Baris 1:** label tombol aksi (`<strong>`, `ink`)
- **Baris 2:** `{label status asal} → {label status tujuan}` (warna `muted`, memakai entitas
  `&rarr;`)

Bila status asal/tujuan tidak ditemukan di daftar, bagian itu tampil **kosong** (tanpa placeholder
atau tanda `-`).

Urutan tampil: **apa adanya dari server**, tanpa pengurutan di klien.

Empty state:

| Elemen | Isi |
|---|---|
| Judul | **"Belum ada aturan"** |
| Deskripsi | **"Tambahkan aturan perpindahan agar pengguna tahu langkah berikutnya pada order."** |

### 3.5 Kartu "Tambah Aturan Perpindahan"

| Elemen | Isi |
|---|---|
| Judul | **"Tambah Aturan Perpindahan"** |
| Deskripsi | **"Tentukan dari status mana ke status mana order boleh berpindah."** |

| # | Label | `id` | Bentuk | Opsi pertama |
|---|---|---|---|---|
| 1 | **"Dari status"** | `transition-from-status` | `SearchableSelect` | **"Pilih status asal"** (nilai kosong) |
| 2 | **"Ke status"** | `transition-to-status` | `SearchableSelect` | **"Pilih status tujuan"** (nilai kosong) |
| 3 | **"Label tombol aksi"** | `transition-label-input` | input teks, `required` | Placeholder **"Contoh: Konfirmasi"** |

Tombol submit: **"Tambah Aturan"** — **tidak** dinonaktifkan meski status belum dipilih.

Setelah submit, **hanya** label yang dikosongkan; pilihan "dari" dan "ke" tetap terisi.

### 3.6 Kondisi render kosong

Sama seperti halaman Profil Perusahaan: merender **kosong** bila data pendamping perusahaan belum
tersedia.

---

## 4. Katalog Notifikasi Toast Modul Ini

| Pemicu | Judul | Deskripsi | Tone |
|---|---|---|---|
| Profil perusahaan tersimpan | **"Profil perusahaan disimpan"** | **"Identitas perusahaan berhasil diperbarui."** | success |
| Gagal menyimpan profil | **"Gagal menyimpan profil perusahaan"** | *(tidak ada)* | danger |
| Gagal menyimpan pengaturan | **"Gagal menyimpan pengaturan"** | *(tidak ada)* | danger |
| Gagal menyimpan cabang | **"Gagal menyimpan cabang"** | *(tidak ada)* | danger |
| Gagal menyimpan definisi status | **"Gagal menyimpan definisi status"** | *(tidak ada)* | danger |
| Gagal menyimpan transisi status | **"Gagal menyimpan transisi status"** | *(tidak ada)* | danger |

Dua asimetri yang perlu diketahui:

1. **Hanya profil perusahaan punya toast sukses.** Menyimpan pengaturan (mode koreksi stok, kuota
   AI, identitas dokumen, kalibrasi kertas) **tidak** memunculkan notifikasi apa pun — padahal
   itulah bagian terbesar halaman.
2. **Menambah status dan aturan perpindahan juga tanpa toast sukses.** Umpan baliknya hanya
   munculnya baris baru di daftar setelah data dimuat ulang.

Semua pesan gagal **tanpa deskripsi** — pesan spesifik dari server dibuang. Pola yang sama dengan
KI-04 dan KI-17.

---

## 5. Format & Konvensi Tampilan

| Aspek | Aturan di modul ini |
|---|---|
| Format tanggal | **Modul ini tidak menampilkan tanggal apa pun** |
| Format angka | Kuota AI dan kalibrasi kertas ditampilkan apa adanya di `input type=number` (tanpa pemisah ribuan) |
| Angka desimal kalibrasi | `step="0.1"`; nilai default ditampilkan sebagai placeholder, bukan nilai terisi |
| Mata uang | Diubah ke huruf besar **saat mengetik**, bukan saat simpan |
| Zona waktu & locale | Teks bebas, tanpa validasi, tanpa daftar pilihan |
| Kelompok status order | Diterjemahkan ke Indonesia lewat peta tetap (§3.2) |
| Kelas CSS legacy | `field`, `list-item` — hook selector test |
| Aksi destruktif | Tombol "Hapus" pada telepon/rekening **tanpa konfirmasi** (hanya menghapus baris di form, belum tersimpan) |
| Mode baca-saja | Ditandai dengan input yang dinonaktifkan + teks penjelas, bukan dengan menyembunyikan kartu |
