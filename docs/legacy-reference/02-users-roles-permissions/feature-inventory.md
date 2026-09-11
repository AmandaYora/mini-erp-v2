# Feature Inventory — Modul 02 Users, Roles & Permissions

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [test-cases.md](test-cases.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 10 — 5 `users/*` + 5 `roles/*` |
| Halaman | 4 rute: `/users`, `/users/create`, `/users/:userId/edit`, `/settings/roles` |
| Menu sidebar | 2, keduanya grup **"Pengaturan"**: "Pengguna" (ikon `users`), "Role & Akses" (ikon `roles`) |
| Permission yang dibutuhkan | `user.view`, `user.create`, `user.update`, `user.archive`, `role.manage` |
| Tabel yang dimiliki | `users`, `roles`, `permissions`, `role_permissions`, `user_roles`, `user_branch_accesses` |
| Laporan | Tidak ada — lihat [reports-list.md](reports-list.md) |
| Penomoran dokumen | Tidak ada; tapi ada **generator kode role** — lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | **7 `actionKey`** ditulis modul ini (berbeda dari modul auth yang EXEMPT) |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar Pengguna

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Tabel pengguna perusahaan | 6 kolom: Nama, Jabatan, Role, Cabang, Status, Aksi |
| F-01.2 | Pencarian | Satu kotak, mencari di **nama lengkap ATAU username ATAU email** sekaligus |
| F-01.3 | Paginasi | 20 baris per halaman, tombol Prev/Next + teks "`{dari}`–`{sampai}` dari `{total}` data" |
| F-01.4 | Badge role per user | Semua role yang dimiliki, label diterjemahkan (`kasir` → "Kasir") |
| F-01.5 | Badge cabang per user | Menampilkan **kode** cabang (mis. `BR-001`), bukan namanya |
| F-01.6 | Badge status | Hijau bila aktif, kuning bila tidak — **teksnya nilai mentah** `active`/`inactive` |
| F-01.7 | Aktifkan / Nonaktifkan langsung dari tabel | Tombol ghost yang labelnya berganti sesuai status saat ini |
| F-01.8 | Tombol "Detail" ke form edit | Muncul hanya bila punya `user.update` |
| F-01.9 | Penyembunyian akun Super Admin | Sesi non-superadmin **tidak melihat** user yang memegang role `superadmin` — baris itu hilang dari daftar **dan** dari hitungan total |
| F-01.10 | Tombol aksi kondisional per permission | "Tambah Pengguna" butuh `user.create`, toggle status butuh `user.archive`, "Detail" butuh `user.update` |

### F-02 — Tambah Pengguna

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Form profil | Nama lengkap, Jabatan, Email, Username, No. WhatsApp |
| F-02.2 | Password awal + konfirmasi | Dua field, keduanya wajib, minimal 8 karakter, harus sama |
| F-02.3 | Toggle tampil/sembunyi password | Ikon mata per field |
| F-02.4 | Pemilihan role (multi) | Checkbox pill; **role `superadmin` tidak pernah muncul sebagai pilihan** |
| F-02.5 | Role bawaan tercentang | Untuk user baru, role **`staff`** otomatis tercentang |
| F-02.6 | Pemilihan cabang (multi) | Checkbox pill berisi **nama** cabang |
| F-02.7 | Cabang bawaan tercentang | Cabang yang bertanda default perusahaan otomatis tercentang |
| F-02.8 | Penetapan cabang default otomatis | **Cabang pertama pada daftar terpilih menjadi cabang default user** (implisit, tidak terlihat di UI) |
| F-02.9 | Hash password | bcrypt cost 12 |
| F-02.10 | Status awal | Selalu `active` (form tidak mengirim status) |

### F-03 — Edit Pengguna

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Form profil terisi | Nilai awal dari data yang sudah dimuat di daftar |
| F-03.2 | Ubah role & cabang | Sama seperti tambah; centang mengikuti kondisi saat ini |
| F-03.3 | Kartu "Ubah Password" terpisah | Hanya muncul di mode edit, dengan placeholder "Kosongkan jika tidak diubah" |
| F-03.4 | Password opsional | Dikosongkan = password tidak diubah |
| F-03.5 | Pencabutan sesi saat password diganti | Semua sesi aktif user itu dicabut; **bila mengganti password sendiri, sesi yang sedang dipakai dipertahankan** |
| F-03.6 | Status **tidak** bisa diubah dari form ini | Ditolak backend dengan pesan khusus; perubahan status hanya lewat tombol di daftar |
| F-03.7 | Penyembunyian akun Super Admin | Sesi non-superadmin yang mencoba mengedit user superadmin mendapat "User tidak ditemukan" |
| F-03.8 | Dua permintaan terpisah | Profil dan password disimpan lewat dua panggilan berurutan, bukan satu transaksi |

### F-04 — Aktifkan / Nonaktifkan Pengguna

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Toggle satu klik dari daftar | Tanpa dialog konfirmasi |
| F-04.2 | Pencabutan sesi saat dinonaktifkan | Semua sesi aktif user itu langsung dicabut → user terputus dari semua perangkat |
| F-04.3 | Reaktivasi | Mengubah kembali ke `active` **tidak** menyentuh sesi (yang sudah dicabut tetap mati) |
| F-04.4 | Jumlah sesi tercabut dicatat | Masuk ke metadata audit log, **tidak** ditampilkan ke user |
| F-04.5 | Permission terpisah | Memakai `user.archive`, bukan `user.update` |

### F-05 — Ganti Password Pengguna (oleh admin)

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Diatur dari form edit pengguna | Bukan halaman terpisah |
| F-05.2 | Tanpa verifikasi password lama | Admin tidak perlu tahu password sebelumnya |
| F-05.3 | Minimal 8 karakter | Divalidasi di klien **dan** server |
| F-05.4 | Konfirmasi harus sama | Divalidasi di klien saja |
| F-05.5 | Pencabutan sesi | Lihat F-03.5 |
| F-05.6 | Permission | Memakai `user.update` (tidak ada permission khusus password) |

### F-06 — Daftar Role

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Tabel role | 3 kolom: Role (nama + kode terformat), Tipe, Aksi |
| F-06.2 | Pembeda role sistem vs kustom | Badge "Sistem" (netral) / "Kustom" (info) |
| F-06.3 | Urutan | **Role sistem lebih dulu**, lalu berdasarkan nama A→Z |
| F-06.4 | Gabungan role global + perusahaan | Role sistem (`id_company` kosong) dan role kustom perusahaan tampil dalam satu daftar |
| F-06.5 | Penyembunyian role Super Admin | Sesi non-superadmin tidak melihat role `superadmin` |
| F-06.6 | Deskripsi otomatis | Bila kosong, diisi "Role sistem `{nama}`" atau "Role kustom `{nama}`" |
| F-06.7 | Tanpa paginasi | Seluruh role dikembalikan sekaligus |
| F-06.8 | Tombol "Hapus" hanya untuk role kustom | Role sistem tidak punya tombol hapus |

### F-07 — Tambah / Edit Role

| # | Sub-fitur | Detail |
|---|---|---|
| F-07.1 | Form dua field | Nama role (wajib) + Deskripsi (textarea, opsional) |
| F-07.2 | Kode role dibuat otomatis dari nama | "Kepala Gudang" → `kepala_gudang` |
| F-07.3 | Kode bisa ditentukan manual | Bila dikirim, tetap dinormalisasi dengan aturan sama |
| F-07.4 | Tombol "Atur" mengisi form untuk edit | Sekaligus memilih role untuk matriks permission |
| F-07.5 | Tombol "Reset" mengosongkan form | Sekaligus membatalkan pemilihan role |
| F-07.6 | Label tombol simpan berganti | "Tambah Role" / "Simpan Role" |
| F-07.7 | Role sistem tidak bisa diubah nama/deskripsi | Ditolak di klien (toast peringatan, tanpa request) **dan** di server |
| F-07.8 | Role baru selalu milik perusahaan | `is_system_role` selalu `false` |

### F-08 — Hapus Role

| # | Sub-fitur | Detail |
|---|---|---|
| F-08.1 | Hanya role kustom | Role sistem tidak punya tombol, dan server menolaknya |
| F-08.2 | Penjagaan "sedang dipakai" | Role yang masih ditetapkan ke minimal satu user tidak bisa dihapus |
| F-08.3 | **Hard delete** | Baris role benar-benar dihapus dari database — satu-satunya pengecualian larangan hard-delete di seluruh sistem |
| F-08.4 | Permission role ikut dihapus | Baris `role_permissions` dibersihkan lebih dulu |
| F-08.5 | Tanpa dialog konfirmasi | Klik "Hapus" langsung menghapus |

### F-09 — Matriks Hak Akses (Permission per Role)

| # | Sub-fitur | Detail |
|---|---|---|
| F-09.1 | Matriks 22 kelompok modul | Setiap kelompok satu kotak berisi pill permission |
| F-09.2 | 59 permission dapat diatur | Dari 60 permission yang ada di sistem |
| F-09.3 | Label bisnis, bukan kode teknis | `finance.close.force` tampil sebagai "Tutup Buku Paksa (Force, Lewati Checklist)" |
| F-09.4 | Simpan otomatis per klik | Tidak ada tombol simpan — setiap centang/hapus centang langsung dikirim |
| F-09.5 | Pembaruan optimistis | Tampilan berubah lebih dulu, dikembalikan bila permintaan gagal |
| F-09.6 | Permission role **sistem** bisa diubah | Berbeda dari nama/hapus yang diblokir untuk role sistem |
| F-09.7 | Penyembunyian role Super Admin | Sesi non-superadmin ditolak "Role tidak ditemukan" bila mencoba mengubah permission `superadmin` |
| F-09.8 | Validasi permission tak dikenal | Ditolak **sebelum** permission lama dihapus, jadi state lama aman |
| F-09.9 | Deduplikasi | Kode permission ganda disaring di klien dan server |
| F-09.10 | Notice bila belum memilih role | "Pilih salah satu role untuk mulai mengatur permission." |

### F-10 — Kebijakan Role `superadmin`

| # | Sub-fitur | Detail |
|---|---|---|
| F-10.1 | Tidak muncul sebagai pilihan role di form user | Disaring di frontend |
| F-10.2 | Penetapan lewat API ditolak | "Role Super Admin hanya untuk akun developer default." |
| F-10.3 | Disaring dari daftar user | Sesi non-superadmin tidak melihat pemegangnya |
| F-10.4 | Disaring dari daftar role | Sesi non-superadmin tidak melihat rolenya |
| F-10.5 | Aksi terhadap user superadmin disamarkan | Bukan "tidak boleh", melainkan **"User tidak ditemukan"** |
| F-10.6 | Selalu dipulihkan untuk akun developer | Setiap kali role user `id_user = 1` disimpan, role `superadmin` otomatis ditambahkan kembali |

---

## 3. Edge Case yang Terbukti Ada di Kode

Semua diverifikasi dari kode. Yang berperilaku janggal juga dicatat di
[known-issues.md](../known-issues.md).

### Daftar & pencarian

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-01 | Pencarian dengan spasi di awal/akhir | Di-trim sebelum dikirim; bila hasilnya kosong, parameter pencarian tidak dikirim sama sekali |
| EC-02 | Pencarian **beberapa kata** | Diperlakukan sebagai **satu frasa utuh** — "budi santoso" tidak cocok dengan "Budi Ahmad Santoso". Berbeda dari modul lain yang memakai pencarian per-kata |
| EC-03 | Setiap ketikan di kotak cari | **Satu permintaan API per karakter** — tidak ada penundaan (debounce) |
| EC-04 | `limit` dikirim > 100 | Dipangkas ke 100 |
| EC-05 | `limit` dikirim `0`, negatif, atau bukan angka | **Tidak ditangani** — `Math.min(0, 100)` = 0 → `LIMIT 0` → daftar kosong. Berbeda dari modul lain yang memakai helper clamp bersama |
| EC-06 | Daftar gagal dimuat | Tabel dikosongkan **tanpa pesan error apa pun** |
| EC-07 | Sesi non-superadmin | Total di deskripsi kartu ikut berkurang karena penyaringan terjadi di query |

### Tambah / edit pengguna

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-08 | Username sudah dipakai | `409` "Username sudah digunakan" |
| EC-09 | Email sudah dipakai | `409` "Email sudah digunakan" |
| EC-10 | Email dikirim string kosong saat **tambah** | Disimpan sebagai string kosong, **bukan** null. User kedua dengan email kosong → **error 500** (bentrok indeks unik) |
| EC-11 | Email dikosongkan saat **edit** | Disimpan sebagai null (perilaku berbeda dari tambah) |
| EC-12 | Nomor WhatsApp dikosongkan | Disimpan sebagai null |
| EC-13 | Jabatan dikosongkan lewat API | Disimpan sebagai **string kosong**, bukan null (tidak konsisten dengan email & telepon) |
| EC-14 | Password < 8 karakter | Ditolak klien (pesan native browser) dan server (`400`) |
| EC-15 | Password ≠ konfirmasi | Ditolak klien saja: "Konfirmasi password tidak sama." |
| EC-16 | Edit tanpa mengisi password | Password tidak diubah, permintaan ganti password tidak dikirim |
| EC-17 | Edit dengan mengisi **hanya** konfirmasi | Validasi tetap berjalan — dianggap ingin mengubah password, lalu gagal di panjang minimum |
| EC-18 | Mengirim `status` lewat `users/update` | `400` "Status user hanya dapat diubah lewat aksi aktifkan/nonaktifkan." |
| EC-19 | Menetapkan role `superadmin` | `400` "Role Super Admin hanya untuk akun developer default." |
| EC-20 | **Kode role tidak dikenal** dikirim | **Dilewati tanpa error** — tidak ada peringatan bahwa role gagal ditetapkan |
| EC-21 | **`role_codes` dikirim array kosong** | Semua role user dihapus → user **tidak bisa login lagi** ("User tidak memiliki role") |
| EC-22 | Menyimpan role untuk `id_user = 1` | Role `superadmin` otomatis ditambahkan kembali, apa pun isi pilihan |
| EC-23 | Urutan cabang yang dicentang | **Cabang pertama menjadi default.** Mengubah kombinasi centang bisa memindahkan cabang default tanpa terlihat |
| EC-24 | `branch_ids` dikirim array kosong | Semua akses cabang dihapus → user login tapi **tanpa cabang** → jalan buntu (lihat KI-01 modul 01) |
| EC-25 | `branch_ids` berisi id cabang tak dikenal | Gagal di tingkat basis data → **500**, bukan pesan bisnis |
| EC-26 | Edit user yang **tidak ada di 100 data pertama** | Form terbuka dalam **mode tambah** tanpa peringatan — data user tidak ditemukan di store |
| EC-27 | Permintaan profil berhasil, ganti password gagal | Profil **tersimpan**, password **tidak** — tidak ada pembatalan |
| EC-28 | Username diubah ke nilai yang sama | Tidak dianggap bentrok (pemeriksaan mengecualikan diri sendiri) |
| EC-29 | Username sudah dipakai user di **perusahaan lain** | Tetap ditolak — keunikan username bersifat global, bukan per-perusahaan |

### Status pengguna

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-30 | Menonaktifkan user | Semua sesi aktifnya dicabut; user langsung terputus di semua perangkat |
| EC-31 | Menonaktifkan diri sendiri | **Tidak ada penjagaan** — admin bisa menonaktifkan akunnya sendiri dan langsung terputus |
| EC-32 | Mengaktifkan kembali user | Sesi lama **tidak** dihidupkan; user harus login ulang |
| EC-33 | Menonaktifkan user superadmin dari sesi non-superadmin | "User tidak ditemukan" (`404`) |
| EC-34 | Status `locked` | **Tidak bisa diset lewat modul ini** — hanya `active`/`inactive`. Nilai `locked` ada di data tapi tak punya jalur UI |

### Role & permission

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-35 | Nama role berisi karakter non-alfanumerik | Dinormalisasi: "Kepala Gudang!" → `kepala_gudang` |
| EC-36 | Nama role hanya simbol (mis. "!!!") | Kode jadi kosong → `409` "Kode role tidak valid" |
| EC-37 | Kode role sudah dipakai role sistem | Ditolak — pemeriksaan mencakup role global |
| EC-38 | Nama dua role berbeda menghasilkan kode sama | Mis. "Kepala Gudang" dan "kepala-gudang" → keduanya `kepala_gudang` → yang kedua ditolak |
| EC-39 | Mengubah nama role sistem lewat UI | Diblokir di klien: toast "Role sistem tidak dapat diubah" — request tidak dikirim |
| EC-40 | Mengubah nama role sistem lewat API | `404` "Role kustom tidak ditemukan" |
| EC-41 | Menghapus role yang masih dipakai | `409` "Role masih dipakai oleh user dan tidak dapat dihapus" |
| EC-42 | Menghapus role sistem lewat API | `404` "Role kustom tidak ditemukan" |
| EC-43 | Menyimpan permission dengan kode tak dikenal | `400` "Permission tidak dikenal: `{kode}`" — **permission lama tidak terhapus** |
| EC-44 | Menyimpan permission array kosong | Semua permission role dihapus; tidak ada penjagaan "role harus punya minimal 1 permission" |
| EC-45 | Mengubah permission role **sistem** | **Diizinkan** — termasuk oleh sesi `owner` dan `admin` |
| EC-46 | Mengubah permission role `superadmin` dari sesi non-superadmin | `404` "Role tidak ditemukan" |
| EC-47 | Permission `whatsapp.simulate` | **Tidak ada di matriks UI** (sengaja, agar tidak bisa diberikan ke role kustom) — tapi **tetap bisa diberikan lewat API langsung** |
| EC-48 | Menyimpan role dari form | Setelah simpan, pilihan role **dikosongkan** → matriks permission menutup kembali ke notice |
| EC-49 | Klik "Hapus" pada role | Langsung terhapus, **tanpa dialog konfirmasi** |
| EC-50 | Seed dijalankan ulang | Permission yang sudah dihapus dari role sistem lewat UI **muncul kembali** (seed bersifat menambah, tidak pernah menghapus) |

---

## 4. Hal yang Tampak Tersedia Tapi Tidak Berfungsi Seperti Dugaan

| # | Elemen | Kondisi aktual |
|---|---|---|
| NF-01 | Badge status di daftar pengguna | Menampilkan `active` / `inactive` dalam bahasa Inggris mentah, di tengah UI berbahasa Indonesia |
| NF-02 | Pesan error saat gagal menyimpan pengguna | Selalu "Gagal menyimpan pengguna" tanpa alasan — pesan spesifik dari server (mis. "Username sudah digunakan") **dibuang** |
| NF-03 | Pesan error saat gagal menyimpan/menghapus role | Menampilkan kode mesin (`error`, `validation_failed`) sebagai deskripsi toast, bukan pesan Indonesia yang sudah disiapkan server |
| NF-04 | Kolom "Jabatan" di daftar pengguna | Berisi `display_title`; bila kosong, adapter mengisinya dengan **nama lengkap** sehingga kolom Nama dan Jabatan tampak sama |
| NF-05 | Penyembunyian `whatsapp.simulate` dari matriks | Hanya menyembunyikan di UI; API tetap menerimanya |

---

## 5. Data Bawaan (Seed) Terkait Modul Ini

### Role sistem (4) — `id_company` kosong, `is_system_role = 1`

| id | Kode | Nama |
|---|---|---|
| 1 | `superadmin` | Super Admin |
| 2 | `owner` | Owner |
| 3 | `admin` | Admin |
| 4 | `staff` | Staff |

### Role kustom (1) — milik perusahaan, `is_system_role = 0`

| id | Kode | Nama | Permission bawaan |
|---|---|---|---|
| 5 | `kasir` | Kasir | 9: `dashboard.view`, `product.view`, `order.view`, `order.create`, `order.update`, `sales_return.view`, `sales_return.create`, `stock.view`, `stock.adjust` |

### Permission (60 total)

| Sumber | Jumlah |
|---|---|
| Daftar di seed-runner | 56 |
| Migrasi `050_purchase_returns` | 4 (`purchase_return.*`) |

Beberapa migrasi lain (`009`, `017`, `022`, `031`, `032`, `038`, `040`, `041`, `048`) juga
menyisipkan permission dan penetapannya ke role — sehingga **sumber kebenaran permission tersebar
di seed + 10 migrasi**, bukan satu tempat.

### Penetapan role & cabang bawaan

Kelima akun bawaan (`superadmin`, `owner`, `admin`, `staff`, `kasir`) masing-masing memegang
**tepat satu** role dengan nama sama, dan semuanya punya akses ke `id_branch = 1` dengan
`is_default_branch = 1`.

Konsekuensi: fitur multi-role (F-04 modul 01 "Ganti Role") **tidak pernah aktif** untuk akun
bawaan. **[PERLU KONFIRMASI]** apakah ada akun production yang memegang lebih dari satu role.

### Kebijakan yang ditegakkan ulang setiap seed

| Aksi | Efek |
|---|---|
| Hapus role `superadmin` dari semua user selain `id_user = 1` | Kebijakan role cadangan |
| Pastikan `id_user = 1` memegang `superadmin` | Akun developer selalu bisa masuk |
| Tambahkan kembali seluruh permission bawaan ke role sistem | **Menimpa** penghapusan permission yang dilakukan lewat UI |
| Cabut semua sesi aktif user 1–5 | Semua perangkat ter-logout |
| Timpa `password_hash` kelima akun ke nilai bawaan | Password yang diubah manual kembali ke bawaan |
