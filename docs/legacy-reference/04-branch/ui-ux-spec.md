# UI/UX Spec — Modul 04 Branch (Multi-Cabang)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Deskripsi tiap layar dan
interaksinya, presisi sampai teks tombol dan urutan field. Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

Komponen bersama (`PageHeader`, `SectionCard`, `DataTable`, `EmptyState`, `Badge`, `Button`,
`ActionRow`, toast) dijelaskan di [../shared/shared-services.md](../shared/shared-services.md) —
di sini hanya penggunaannya di modul ini.

---

## 1. Peta Layar

| Rute | Judul topbar | Komponen | Permission rute |
|---|---|---|---|
| `/branches` | Cabang & Lokasi | Daftar Cabang | `branch.view` |
| `/branches/create` | Cabang & Lokasi | Form Cabang (mode tambah) | `branch.manage` |
| `/branches/:branchId/edit` | Cabang & Lokasi | Form Cabang (mode ubah) | `branch.manage` |

Ketiganya memakai kerangka aplikasi penuh (sidebar + topbar). Tanpa permission yang dibutuhkan,
penjaga rute mengalihkan ke halaman **403** — bukan menampilkan pesan di tempat.

**Catatan topbar:** teks besar di kiri topbar berlabel kecil "Cabang Aktif" tetapi isinya adalah
**judul halaman** ("Cabang & Lokasi"), bukan nama cabang. Ini perilaku global, sudah tercatat
sebagai KI-08 di modul 01.

---

## 2. Layar: Daftar Cabang (`/branches`)

### 2.1 Kepala halaman

| Bagian | Isi persis |
|---|---|
| Remah roti | `Dashboard > Cabang & Lokasi` |
| Judul | **Cabang & Lokasi** |
| Deskripsi | *Kelola daftar cabang dan lokasi operasional perusahaan Anda.* |
| Aksi kanan | Tombol utama **`+ Tambah Cabang`** — **hanya bila punya `branch.manage`** |

### 2.2 Kartu isi

Satu `SectionCard`:

| Bagian | Isi persis |
|---|---|
| Judul kartu | **Semua Cabang** |
| Deskripsi kartu | *Daftar cabang yang terdaftar di perusahaan ini.* |

### 2.3 Tabel — urutan kolom (kiri ke kanan)

| # | Judul kolom | Isi sel |
|---|---|---|
| 1 | **Nama Cabang** | Tiga baris bertumpuk: **nama** (tebal), **kode** (kecil, redup), lalu badge hijau **"Aktif Saat Ini"** — badge hanya muncul pada cabang yang sedang aktif di sesi ini |
| 2 | **Kota** | Teks kota apa adanya |
| 3 | **Alamat** | Teks alamat apa adanya; kosong bila alamat tidak diisi |
| 4 | **Lokasi Stok Awal** | Label lokasi stok default cabang |
| 5 | **Status / Akses** | Bertumpuk: badge biru **"Pusat"** (hanya bila cabang bertanda pusat), lalu badge hijau **"Anda Memiliki Akses"** atau badge abu **"Tidak Ada Akses"** |
| 6 | **Aksi** | Tombol sekunder **`Edit`** — **seluruh kolom ini hilang** bila tidak punya `branch.manage` |

Baris tabel **tidak bisa diklik** — satu-satunya jalan ke form ubah adalah tombol Edit.
Tabel tidak punya kotak pencarian, filter, penyortiran kolom, maupun paginasi. Seluruh cabang
ditampilkan sekaligus, terurut nama A→Z dari server.

**Yang perlu diperhatikan saat rebuild:** kolom 5 berjudul "Status / Akses" tetapi **tidak pernah
menampilkan status cabang** (aktif/non-aktif). Isinya hanya penanda pusat dan akses pengguna.
Lihat [known-issues.md](../known-issues.md) KI-39.

### 2.4 Keadaan kosong

Bila perusahaan belum punya cabang, tabel diganti kartu kosong bergaris putus-putus:

| Bagian | Isi persis |
|---|---|
| Judul | **Belum ada cabang** |
| Deskripsi | *Tambahkan cabang pertama agar operasional dan stok punya lokasi kerja.* |
| Aksi | Tombol **`Tambah Cabang`** (tanpa tanda `+`) — hanya bila punya `branch.manage` |

Perhatikan perbedaan label yang disengaja: tombol di kepala halaman berbunyi **`+ Tambah Cabang`**,
tombol di keadaan kosong berbunyi **`Tambah Cabang`**.

### 2.5 Keadaan lain

| Keadaan | Yang terlihat |
|---|---|
| Sedang memuat | **Tidak ada indikator memuat.** Data cabang sudah dimuat saat aplikasi dijalankan (sebelum layar aplikasi muncul), jadi halaman langsung tampil terisi |
| Perusahaan aktif belum ada | Halaman merender **kosong sama sekali** (tanpa kepala halaman) |
| Gagal memuat daftar cabang | **Tidak ada pesan galat.** Kegagalan ditelan diam-diam dan halaman menampilkan keadaan kosong "Belum ada cabang" — lihat KI-47 |

---

## 3. Layar: Form Cabang (`/branches/create` dan `/branches/:branchId/edit`)

Satu komponen dipakai untuk dua mode. Perbedaannya hanya pada teks.

### 3.1 Kepala halaman

| Bagian | Mode Tambah | Mode Ubah |
|---|---|---|
| Remah roti | `Dashboard > Cabang & Lokasi > Tambah Cabang` | `Dashboard > Cabang & Lokasi > <Nama Cabang>` |
| Judul | **Tambah Cabang** | **Edit Cabang** |
| Deskripsi | *Atur identitas cabang dan lokasi stok awal agar cabang baru bisa langsung dipakai.* | (sama) |
| Aksi kanan | Tombol samar **`← Kembali`** → `/branches` | (sama) |

### 3.2 Urutan bagian dan field (atas ke bawah)

Form terdiri dari **tiga kartu** berurutan, lalu baris aksi.

#### Kartu 1 — Identitas Cabang

> Deskripsi kartu: *Nama dan kode unik yang merepresentasikan cabang ini di seluruh sistem.*

Tata letak: kisi responsif, kolom minimal 280px, jadi dua kolom di layar lebar dan satu kolom di
layar sempit.

| # | Label | Jenis | Wajib | Contoh isian (placeholder) | Catatan |
|---|---|---|---|---|---|
| 1 | **Nama cabang** | Teks satu baris | ✅ | `Cabang Surabaya` | — |
| 2 | **Kode cabang** | Teks satu baris | ✅ | `SBY` | Tampil **HURUF BESAR**; maksimal **10 karakter** (dijaga browser) |

Teks bantu di bawah kode cabang:

> *Kode singkat unik, dipakai sebagai prefix nomor order, pembayaran, dan surat jalan. Contoh: JKT,
> BDG, SBY.*

**[PERLU KONFIRMASI]** Teks bantu menyebut tiga jenis dokumen, padahal kode juga jadi prefix
**retur penjualan** (`RTR-`) dan **retur pembelian** (`RTB-`). Apakah teks ini perlu diperbarui di
sistem baru, atau sengaja disederhanakan?

#### Kartu 2 — Lokasi

> Deskripsi kartu: *Informasi geografis cabang untuk keperluan operasional dan pengiriman.*

| # | Label | Jenis | Wajib | Contoh isian | Catatan |
|---|---|---|---|---|---|
| 3 | **Kota** | Teks satu baris | ✅ | `Surabaya` | — |
| 4 | **Alamat lengkap** | Kotak teks banyak baris | ❌ | `Jl. Raya Darmo No. 1, Surabaya` | Melebar penuh; tinggi minimal ±80px; dapat diperbesar vertikal oleh pengguna |

#### Kartu 3 — Pengaturan Stok

> Deskripsi kartu: *Buat lokasi stok awal untuk cabang ini. Gudang dan mapping detail tetap bisa
> ditambah lagi dari menu stok.*

| # | Label | Jenis | Wajib | Contoh isian | Catatan |
|---|---|---|---|---|---|
| 5 | **Label lokasi stok awal** | Teks satu baris | ✅ | `Gudang Utama Surabaya` | — |

Teks bantu di bawahnya:

> *Pakai nama lokasi fisik pertama, misalnya Gudang Depan, Area Toko, atau Gudang Utama Jakarta.*

#### Baris aksi (paling bawah)

| Posisi | Tombol | Gaya | Perilaku |
|---|---|---|---|
| Kiri | **`Batal`** | Sekunder | Kembali ke `/branches` **tanpa konfirmasi**, perubahan dibuang |
| Kanan | **`Simpan Cabang`** (mode tambah) / **`Simpan Perubahan`** (mode ubah) | Utama | Kirim form |

### 3.3 Nilai awal field

| Mode | Nilai awal |
|---|---|
| Tambah | Semua kosong |
| Ubah | Terisi dari data cabang yang sedang dibuka; label lokasi stok jatuh ke `"Default"` bila tidak ada |

Field diisi sebagai **nilai awal**, bukan nilai terkendali — mengetik tidak memicu render ulang,
dan form tidak punya keadaan "belum tersimpan".

### 3.4 Field yang ADA di API tapi TIDAK ADA di form

| Field | Akibat |
|---|---|
| Nomor telepon cabang | Tidak pernah bisa diisi/diubah lewat aplikasi (KI-37) |
| Status cabang | Form **selalu** mengirim `active`; cabang tidak bisa ditutup lewat aplikasi, dan cabang non-aktif yang disimpan ulang akan aktif kembali (KI-36) |
| Penanda cabang pusat | Nilai lama dipertahankan di sisi web tapi tidak pernah dikirim ke server; badge "Pusat" praktis hanya milik cabang bawaan (KI-38) |

### 3.5 Validasi yang dirasakan pengguna

| Kejadian | Yang terlihat |
|---|---|
| Nama / kode / kota / label lokasi kosong | Gelembung validasi bawaan browser pada field pertama yang kosong; form tidak terkirim; **tidak ada pesan buatan aplikasi** |
| Kode lebih dari 10 karakter | Browser menolak ketikan ke-11 (tanpa pesan) |
| Kode sudah dipakai cabang lain | Toast merah **"Gagal menyimpan cabang"** tanpa deskripsi — pesan asli server (`Kode cabang 'XXX' sudah digunakan`) **tidak ditampilkan** |
| Galat lain apa pun dari server | Toast merah yang sama persis: **"Gagal menyimpan cabang"** |
| Berhasil menyimpan | **Tidak ada toast sukses.** Halaman langsung berpindah ke `/branches` dan daftar sudah berisi data baru (KI-43) |

Toast bertahan **3,4 detik** lalu hilang sendiri.

**Yang perlu diperhatikan saat rebuild:** ini akar masalah yang sama dengan KI-04 (modul 01) dan
KI-17 (modul 02) — pesan galat spesifik dari server dibuang di lapisan antarmuka dan diganti
kalimat umum. Pengguna yang mengetik kode ganda tidak diberi tahu bahwa masalahnya ada di kode.

---

## 4. Tampilan Cabang di Luar Halaman Modul Ini

Bagian ini bukan milik modul 04, tapi wajib dipertahankan karena pengguna melihatnya sebagai
"cabang".

### 4.1 Chip cabang di topbar

| Kondisi | Tampilan |
|---|---|
| Pengguna punya akses **>1** cabang | Chip **dapat diklik**, label kecil **"Ganti Cabang"**, isi = nama cabang aktif. Diklik → `/select-branch` sambil mengingat halaman asal |
| Pengguna punya akses **1** cabang | Chip **tidak dapat diklik**, label kecil **"Cabang"**, isi = nama cabang aktif |
| Belum ada cabang aktif | Chip tidak dirender sama sekali |

Chip dibatasi lebarnya (maksimal ±280px di desktop, ±220px di layar menengah); nama yang terlalu
panjang dipotong dengan elipsis.

### 4.2 Halaman pemilihan cabang (`/select-branch`, milik modul 01)

Kartu per cabang menampilkan: **nama** (judul), **kota** (di bawah nama), badge biru berisi
**kode**, dan badge aksen **"Default"** pada cabang default pengguna. Bagian bawah kartu berisi
"Access Profile" dan badge peran.

Isinya berasal dari `branches/my-access` (hanya cabang aktif) saat login, tetapi berasal dari
`auth/me` (semua status) saat menyegarkan halaman — perbedaan yang sudah tercatat sebagai KI-10.

### 4.3 Kop dokumen cetak

| Dokumen | Baris cabang |
|---|---|
| Nota penjualan & sejenisnya | `<Nama Cabang> - <Alamat Cabang>` (tanda hubung hanya muncul bila alamat terisi) |
| Identitas dokumen umum | Alamat dan kota cabang aktif dipakai sebagai **cadangan** bila profil dokumen di Pengaturan belum diisi |

Untuk pengguna tanpa `branch.view` (staff, kasir), alamat cabang **kosong** di sini karena daftar
cabang gagal dimuat — lihat KI-47.

---

## 5. Gaya Visual yang Mengikat

| Elemen | Nilai |
|---|---|
| Badge "Aktif Saat Ini", "Anda Memiliki Akses" | Nada **sukses** (hijau lembut) |
| Badge "Pusat" | Nada **info** (biru merek) |
| Badge "Tidak Ada Akses" | Nada **netral** (abu) |
| Badge "Default" (halaman pemilihan cabang) | Nada **aksen** |
| Tombol utama | "+ Tambah Cabang", "Simpan Cabang", "Simpan Perubahan" |
| Tombol sekunder | "Edit", "Batal" |
| Tombol samar | "← Kembali" |
| Lebar minimal tabel | 720px — di layar sempit tabel bergulir horizontal di dalam wadahnya |

---

## 6. Aksesibilitas & Perilaku Kecil

| Hal | Keadaan sekarang |
|---|---|
| Label field | Semua field punya `label` yang tertaut ke input |
| Fokus otomatis | Tidak ada — pengguna harus mengklik field pertama |
| Konfirmasi saat membatalkan | Tidak ada; menekan "Batal" langsung membuang isian |
| Pencegahan klik ganda saat menyimpan | Tidak ada; tombol simpan tidak dinonaktifkan selama permintaan berjalan |
| Indikator sedang menyimpan | Tidak ada |
| Navigasi keyboard | Enter di dalam field teks mengirim form (perilaku bawaan browser) |
