# Data Model Legacy — Modul 01 Auth & Session

**Kelompok B — cukup dipahami maksudnya, JANGAN ditiru strukturnya.** Dokumen ini menjelaskan
*data apa yang perlu ada* dan *kenapa*, bukan bentuk tabel yang harus disalin.

Berkas terkait: [algorithms-legacy.md](algorithms-legacy.md) ·
[business-rules.md](business-rules.md) (di sana aturannya presisi dan wajib dipertahankan)

---

## 1. Tabel yang Disentuh Modul Ini

| Tabel | Peran modul auth terhadapnya | Pemilik |
|---|---|---|
| `user_sessions` | **Tulis & baca** — satu-satunya penulis | Modul auth |
| `users` | Baca (verifikasi) + tulis `last_login_at` | Modul 02 Users |
| `user_roles` | Baca saja | Modul 02 |
| `roles` | Baca saja | Modul 02 |
| `role_permissions` | Baca saja | Modul 02 |
| `permissions` | Baca via relasi | Modul 02 |
| `user_branch_accesses` | Baca saja | Modul 02 |
| `branches` | Baca saja (status & nama) | Modul 04 Branch |
| `companies` | Baca via relasi (nama, timezone, mata uang, locale) | Modul 03 Company |

Modul auth **hanya memiliki satu tabel** (`user_sessions`); sisanya milik modul lain dan dibaca
untuk verifikasi. Ini pola yang sehat dan layak dipertahankan: autentikasi tidak memiliki data
identitas, ia hanya memverifikasinya.

---

## 2. Relasi Antar Tabel

```
companies
    │ 1
    │
    │ n
  users ──────n──── user_roles ──────n──── roles ──n── role_permissions ──n── permissions
    │ 1                                      │
    │                                        │ (id_active_role)
    │ n                                      │
  user_sessions ─────────────────────────────┘
    │
    │ (id_active_branch)
    │
  branches ──────n──── user_branch_accesses ──────n──── users
```

Tiga hubungan many-to-many, semuanya lewat tabel penghubung:

| Hubungan | Tabel penghubung | Atribut tambahan |
|---|---|---|
| User ↔ Role | `user_roles` | tidak ada |
| Role ↔ Permission | `role_permissions` | tidak ada |
| User ↔ Cabang | `user_branch_accesses` | `is_default_branch` |

Yang perlu dipahami maksudnya: **satu user bisa punya beberapa role dan beberapa cabang**, dan
sesi menyimpan *pilihan aktif* di antara keduanya. Jadi "role aktif" dan "cabang aktif" adalah
properti **sesi**, bukan properti user — dua perangkat yang login dengan akun sama bisa berada di
cabang berbeda secara bersamaan.

Konsekuensi yang tidak langsung terlihat: karena pilihan itu hidup di sesi dan **ditimpa** saat
berganti, tidak ada jejak historis "cabang apa yang aktif saat transaksi X dibuat". Transaksi
menyimpan `id_branch`-nya sendiri, jadi informasinya tidak hilang — tapi ia tidak bisa
direkonstruksi dari sisi sesi.

---

## 3. Data yang Perlu Ada — per Entitas

### 3.1 Sesi (`user_sessions`)

Yang benar-benar dibutuhkan konsepnya:

| Kebutuhan data | Kenapa ada |
|---|---|
| Identitas sesi | Dirujuk dari dalam token, agar token bisa dicabut |
| Pemilik sesi (user) | Diverifikasi bersamaan dengan identitas sesi, mencegah token silang |
| Perusahaan | Sumber scope tenant untuk seluruh request |
| Pilihan cabang aktif | **Boleh kosong** — user bisa terautentikasi tanpa cabang |
| Pilihan role aktif | Dasar resolusi permission tiap request |
| Sidik refresh token | Disimpan sebagai hash, bukan token mentah |
| Masa berlaku | Batas hidup sesi, diperpanjang tiap refresh |
| Waktu pencabutan | **Boleh kosong**; terisi = sesi mati |
| Jejak perangkat & IP | Diagnostik; tidak pernah ditampilkan (lihat [reports-list.md](reports-list.md)) |
| Waktu dibuat | Jejak waktu login |

Catatan struktur yang **tidak** perlu ditiru:

- Refresh token disimpan sebagai **hash bcrypt cost 12**. Maksudnya benar (token bocor dari DB
  tidak langsung bisa dipakai), tapi biayanya mahal: setiap refresh melakukan satu operasi bcrypt
  cost 12 untuk perbandingan. Di sistem baru, hash cepat berbasis HMAC/SHA-256 memberi jaminan
  setara untuk token acak berentropi tinggi — bcrypt hanya perlu untuk rahasia berentropi rendah
  seperti password manusia.
- Ada jendela singkat saat login di mana kolom hash berisi nilai placeholder sebelum ditimpa. Ini
  konsekuensi urutan "buat sesi dulu supaya dapat id, baru buat token". Di sistem baru, hasilkan
  identitas sesi lebih dulu (mis. UUID) supaya token dan hash bisa ditulis sekali jalan.
- Tidak ada indeks pada `id_user`, `revoked_at`, atau `expires_at` — hanya primary key dan foreign
  key. Untuk pola query yang dipakai (selalu by primary key + `id_user`) ini cukup, tapi tidak ada
  jalan efisien untuk "cari semua sesi aktif user X" bila fitur itu nanti dibutuhkan.
- **Tidak ada foreign key** ke `branches` maupun `roles`, hanya ke `users`. Jadi menghapus cabang
  atau role tidak dicegah oleh DB, dan sesi bisa merujuk baris yang sudah hilang — inilah sebabnya
  kode menangani "baris role hilang" dengan `activeRoleCode` string kosong.

### 3.2 User (`users`)

| Kebutuhan data | Catatan |
|---|---|
| Perusahaan | Scope tenant |
| Nama lengkap | Ditampilkan di avatar & sidebar; inisialnya dipakai sebagai avatar |
| Email | **Unik, boleh kosong** — sekaligus identifier login alternatif |
| Username | **Unik, wajib** — identifier login utama |
| Hash password | bcrypt |
| Telepon, gelar tampilan | Tidak dipakai modul auth |
| Status | Nilai yang terpakai: `active`, `inactive`, `locked` |
| Waktu login terakhir | Ditulis modul auth, ditimpa setiap login |

Yang perlu dipahami maksudnya:

- **Status disimpan sebagai teks bebas** (`VARCHAR(30)` dengan bawaan `'active'`), bukan enum DB.
  Yang menjaga nilainya hanya TypeScript. Praktisnya seluruh kode hanya membandingkan
  `status === 'active'`, sehingga **`inactive` dan `locked` berperilaku identik** — tidak ada satu
  pun cabang logika yang membedakan keduanya. **[PERLU KONFIRMASI]** apakah `locked` dimaksudkan
  punya arti berbeda (mis. terkunci karena percobaan gagal, perlu dibuka admin) atau memang hanya
  sinonim.
- Email unik tapi nullable: dua user tanpa email diizinkan, tapi dua user dengan email sama tidak.
  Karena email juga jadi identifier login, user tanpa email hanya bisa login dengan username.
- `password_hash` bertipe `TEXT`, bukan panjang tetap — sisa fleksibilitas yang tidak dibutuhkan
  bcrypt (selalu 60 karakter).

### 3.3 Role & Permission

| Entitas | Kebutuhan data | Catatan penting |
|---|---|---|
| `roles` | perusahaan (**boleh kosong**), kode, nama, deskripsi, penanda role sistem | `id_company = NULL` menandai **role sistem global**; terisi = role kustom milik satu perusahaan |
| `permissions` | kunci modul, kunci aksi, **kode gabungan (unik)** | Kode gabungan (`module.action`) adalah yang dipakai kode; dua kolom pecahannya tampak redundan |
| `role_permissions` | pasangan role–permission | Tanpa atribut tambahan |
| `user_roles` | pasangan user–role | **Tanpa penanda role utama** |

Yang perlu dipahami maksudnya:

- **Tidak ada kolom yang menandai role utama** di `user_roles`. Inilah akar masalah "role aktif
  saat login tidak dapat diprediksi" (lihat [business-rules.md](business-rules.md) BR-06). Di
  sistem baru, bila user boleh punya beberapa role, kebutuhan datanya jelas: **satu penanda role
  default per user**, atau aturan urutan yang eksplisit dan deterministik.
- `permissions` menyimpan `module_key`, `action_key`, **dan** `permission_code` yang merupakan
  gabungan keduanya. Hanya `permission_code` yang dibaca kode. Dua kolom pecahan itu tidak punya
  pembaca — kandidat penyederhanaan.
- `roles` **tidak punya kolom status** — role tidak bisa dinonaktifkan, hanya dihapus (dan modul
  Users menghapusnya secara hard-delete dengan pemeriksaan "sedang dipakai"). Ini satu-satunya
  pengecualian larangan hard-delete di seluruh sistem.
- Perbedaan role sistem vs kustom hanya dari `id_company` kosong/terisi plus penanda boolean.
  Keduanya menyimpan informasi yang tumpang tindih.

### 3.4 Akses Cabang (`user_branch_accesses`)

| Kebutuhan data | Catatan |
|---|---|
| Pasangan user–cabang | Menyatakan "user boleh bekerja di cabang ini" |
| Penanda cabang default | Menentukan cabang mana yang dipilih otomatis saat login |
| Waktu dibuat | Tidak dipakai modul auth |

Yang perlu dipahami maksudnya: ini **pivot beratribut** — hubungan many-to-many yang membawa satu
informasi tambahan. Kunci gabungan `(user, cabang)` sekaligus mencegah baris duplikat.

Yang **tidak** dijamin data: hanya boleh ada **satu** cabang default per user. DB hanya menjamin
keunikan pasangan, bukan keunikan penanda default. Bila ada dua cabang bertanda default, kode
mengambil yang pertama ditemukan tanpa error. Di sistem baru, batasan "satu default per user"
layak ditegakkan di lapisan data.

---

## 4. Ketidaksesuaian antara Definisi Entity dan Skema Nyata

Temuan yang perlu diketahui sebelum migrasi data — bukan bug perilaku, tapi sumber kebingungan.

**Semua tabel auth memakai presisi waktu `DATETIME(3)` (milidetik) di database, sementara
definisi entity di kode mendeklarasikan presisi 6 (mikrodetik).**

| Tabel | Presisi di DB | Presisi di entity |
|---|---|---|
| `users` | `DATETIME(3)` | 6 |
| `roles` | `DATETIME(3)` | 6 |
| `user_sessions` | `DATETIME(3)` | 6 |
| `user_branch_accesses` | `DATETIME(3)` | 6 |

Penyebabnya sejarah: tabel-tabel ini lahir di migrasi baseline yang memakai `DATETIME(3)`,
sedangkan tabel yang lahir kemudian (mis. seluruh tabel finance) memakai `DATETIME(6)`. Karena
sinkronisasi skema otomatis dimatikan permanen, DB tidak pernah menyusul deklarasi entity.

Dampak nyatanya kecil (presisi mikrodetik tidak dipakai untuk apa pun di modul auth), tapi ada dua
konsekuensi yang perlu disadari:

1. Klaim "semua kolom waktu berpresisi mikrodetik" **tidak berlaku** untuk tabel auth — koreksi
   terhadap ringkasan di [shared-data-model.md](../shared/shared-data-model.md) §2.2 yang
   menggambarkan konvensi entity, bukan kondisi DB baseline.
2. Bila sistem baru menyeragamkan presisi, nilai waktu lama tetap valid tapi tidak akan pernah
   punya digit mikrodetik yang bermakna.

---

## 5. Bentuk Data Sesi di Sisi Klien

Bukan tabel, tapi bagian dari model data yang perlu dipahami: browser menyimpan **ringkasan sesi**
yang menduplikasi sebagian data server.

Isinya secara konsep: identitas user, seluruh data perusahaan, pilihan cabang & role aktif, daftar
role yang dimiliki, daftar permission, dan daftar cabang yang bisa diakses (masing-masing dengan
nama, kode, kota, dan label lokasi default).

Kenapa duplikasi ini ada: menu sidebar perlu tahu permission untuk menyembunyikan item, dan
halaman pemilihan cabang perlu detail cabang — keduanya sebelum request apa pun selesai. Tanpa
ringkasan lokal, setiap muat halaman akan berkedip.

Yang perlu dipahami maksudnya, dan **tidak** perlu ditiru bentuknya:

- Ringkasan ini hanya diperbarui saat login, pemulihan sesi, dan ganti role/cabang. Di antara itu
  ia bisa **basi** — mis. permission yang baru dicabut masih tampak di menu sampai muat ulang
  berikutnya. Server tetap menolak, jadi ini masalah tampilan, bukan celah akses.
- `auth/me` **tidak** mengembalikan data perusahaan, sehingga pemulihan sesi harus menggabungkan
  respons server dengan ringkasan lokal lama. Ini menciptakan ketergantungan yang tidak jelas:
  data perusahaan bertahan **hanya** karena ada di `localStorage`. Di sistem baru, satu endpoint
  bootstrap yang mengembalikan seluruh konteks sesi menghapus kebutuhan penggabungan ini.
- Seluruh identitas di sisi klien bertipe **string**, sementara di server bertipe angka —
  konversinya terjadi bolak-balik di lapisan adapter, dengan validasi ulang di beberapa titik.
- Daftar permission di ringkasan lokal kehilangan pengetikan ketat (hanya daftar teks), justru di
  tempat ia dipakai untuk menentukan tampilan.

---

## 6. Ringkasan untuk Perancangan Ulang

Kebutuhan data yang **wajib** ada di sistem baru, apa pun bentuknya:

| # | Kebutuhan | Kenapa |
|---|---|---|
| 1 | Sesi sebagai entitas tersendiri yang bisa dicabut | Logout dan pencabutan otomatis mustahil bila status hanya ada di dalam token |
| 2 | Pilihan cabang & role aktif tersimpan **per sesi**, bukan per user | Dua perangkat bisa berada di cabang berbeda bersamaan |
| 3 | Cabang aktif boleh kosong | User bisa terautentikasi tanpa cabang; alur pemilihan cabang bergantung padanya |
| 4 | Sidik refresh token tersimpan, bukan tokennya | Memungkinkan deteksi replay dan pencabutan |
| 5 | Masa berlaku sesi yang bisa diperpanjang | Dasar sliding session (BR-15) |
| 6 | Penanda cabang default per user | Menentukan auto-pilih saat login |
| 7 | Permission dapat di-resolve dari role aktif saat runtime | Perubahan permission harus langsung berlaku |
| 8 | Role boleh bersifat global atau milik satu perusahaan | Role sistem vs role kustom (`kasir`) |

Kebutuhan data yang **hilang** di sistem lama dan layak ditambahkan (masing-masing punya
konsekuensi nyata yang sudah terdokumentasi):

| # | Yang hilang | Konsekuensi di sistem lama |
|---|---|---|
| A | Penanda role utama per user | Role aktif saat login tidak deterministik (BR-06) |
| B | Batasan satu cabang default per user | Dua default mungkin, dipilih sembarang (SK-03) |
| C | Jejak percobaan login gagal | Tidak mungkin menyelidiki serangan atau akun bermasalah (BR-30) |
| D | Riwayat ganti role & cabang | Kolom ditimpa, nilai lama hilang |
| E | Pembeda alasan pencabutan sesi | Logout dan pencabutan otomatis tak bisa dibedakan |
| F | Foreign key sesi → role/cabang | Sesi bisa merujuk baris yang sudah hilang |
