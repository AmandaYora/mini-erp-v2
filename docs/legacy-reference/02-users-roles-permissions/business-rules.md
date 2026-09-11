# Business Rules — Modul 02 Users, Roles & Permissions

**Kelompok A — SEMUA validasi, kondisi, dan aturan keputusan.** Setiap aturan disertai lokasi
agar bisa diverifikasi ulang. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [user-flows.md](user-flows.md) ·
[test-cases.md](test-cases.md) · [known-issues.md](../known-issues.md)

---

## 1. Validasi Input

Modul ini **tidak memakai DTO `class-validator`** — seluruh validasi ditulis manual di service.
(Satu-satunya modul yang memakai DTO adalah `auth`; lihat
[shared-business-rules.md](../shared/shared-business-rules.md) §4.1a.)

Konsekuensi langsung: tipe payload hanya dijamin oleh TypeScript pada waktu kompilasi. Saat
runtime, field yang tidak divalidasi manual **lolos apa adanya** ke basis data.

### BR-01 — Validasi `users/create`

| Field | Divalidasi? | Aturan |
|---|---|---|
| `password` | **ya** | Wajib ada **dan** panjang ≥ 8 → `400` **"Password awal minimal 8 karakter"** |
| `role_codes` | **ya** | Tidak boleh memuat `superadmin` (perbandingan `trim().toLowerCase()`) → `400` **"Role Super Admin hanya untuk akun developer default."** |
| `username` | **ya** | Harus unik secara global → `409` **"Username sudah digunakan"** |
| `email` | **sebagian** | Diperiksa unik **hanya bila nilainya truthy**. String kosong melewati pemeriksaan |
| `full_name` | tidak | Diterima apa adanya, termasuk string kosong |
| `display_title` | tidak | `?? null` |
| `phone` | tidak | `?? null` |
| `status` | tidak | `?? 'active'`; nilai selain `active`/`inactive` **tidak ditolak** |
| `branch_ids` | tidak | Tidak diperiksa keberadaan maupun kepemilikan perusahaan |

Urutan pemeriksaan: password → role superadmin → (mulai transaksi) → username → email.

### BR-02 — Validasi `users/update`

| Field | Aturan |
|---|---|
| `status` | **Bila ada sama sekali** (termasuk `undefined` yang eksplisit dikirim sebagai key? tidak — pemeriksaannya `!== undefined`) → `400` **"Status user hanya dapat diubah lewat aksi aktifkan/nonaktifkan."** |
| `role_codes` | Tidak boleh memuat `superadmin` → `400` (pesan sama BR-01) |
| `id_user` | Harus ada baris user dengan `id` itu **dan** `id_company` sesi → `404` **"User tidak ditemukan"** |
| `email` | Diperiksa unik **hanya bila `!== undefined` dan berbeda dari nilai sekarang**, mengecualikan diri sendiri → `409` **"Email sudah digunakan"** |
| `username` | Sama pola dengan email → `409` **"Username sudah digunakan"** |

Field yang tidak dikirim (`undefined`) **tidak diubah** — pola partial update.

### BR-03 — Normalisasi nilai kosong saat update (tidak konsisten)

| Field | Nilai kosong disimpan sebagai |
|---|---|
| `email` | **`null`** (`data.email \|\| null`) |
| `phone` | **`null`** (`data.phone \|\| null`) |
| `display_title` | **string kosong** (`data.display_title` apa adanya) |

Pada `users/create` perilakunya **berbeda lagi**: `email` disimpan apa adanya, sehingga string
kosong tersimpan sebagai `''`, bukan `null`. Karena kolom email ber-indeks unik dan MySQL
memperlakukan `''` sebagai nilai biasa (bukan seperti `NULL` yang boleh berulang), **user kedua
dengan email kosong gagal dengan error 500**. Lihat [known-issues.md](../known-issues.md) KI-22.

### BR-04 — Validasi `users/update-status`

| Field | Aturan |
|---|---|
| `status` | Tipe membatasi ke `'active' \| 'inactive'`, tapi **tidak ada validasi runtime** — nilai lain akan tersimpan |
| `id_user` | Harus ada dalam perusahaan → `404` "User tidak ditemukan" |

Status **`locked`** tidak bisa diset lewat modul ini, meski nilainya dikenali modul auth.
**[PERLU KONFIRMASI]** apakah `locked` memang tidak dimaksudkan bisa diset manual.

### BR-05 — Validasi `users/change-password`

| Field | Aturan |
|---|---|
| `new_password` | Wajib ada **dan** panjang ≥ 8 → `400` **"Password baru minimal 8 karakter"** |
| `id_user` | Harus ada dalam perusahaan → `404` "User tidak ditemukan" |

**Tidak ada verifikasi password lama.** Tidak ada aturan kompleksitas (huruf besar, angka,
simbol), tidak ada larangan memakai password sebelumnya, dan tidak ada pemeriksaan terhadap daftar
password lemah.

### BR-06 — Validasi klien pada form pengguna

Dijalankan sebelum permintaan dikirim, memakai balon validasi native browser:

| Kondisi | Pesan |
|---|---|
| Password < 8 karakter, mode tambah | **"Password awal minimal 8 karakter."** |
| Password < 8 karakter, mode edit | **"Password baru minimal 8 karakter."** |
| Password ≠ konfirmasi | **"Konfirmasi password tidak sama."** |

Aturan penentu apakah password akan diubah:

```
mode tambah  → selalu diubah
mode edit    → diubah bila (password terisi) ATAU (konfirmasi terisi)
```

Konsekuensi: mengisi **hanya** kolom konfirmasi tetap memicu validasi dan gagal di panjang
minimum — tidak diabaikan.

**Kecocokan konfirmasi hanya divalidasi di klien.** Server tidak menerima field konfirmasi, jadi
permintaan langsung ke API dapat menetapkan password apa pun tanpa konfirmasi.

### BR-07 — Validasi `roles/create`

| # | Aturan | Bila gagal |
|---|---|---|
| 1 | Kode hasil normalisasi tidak boleh kosong | `409` **"Kode role tidak valid"** |
| 2 | Kode belum dipakai role perusahaan **maupun** role global | `409` `Kode role '{kode}' sudah digunakan` |

`name` **tidak** divalidasi tidak-kosong di server (hanya `required` di form). `name.trim()`
dipakai saat menyimpan.

Perhatikan aturan 1 memakai `ConflictException` (HTTP 409) untuk kondisi yang sebenarnya kesalahan
input — seharusnya `400`. Efeknya pada envelope dibahas di BR-22.

### BR-08 — Validasi `roles/update` dan `roles/delete`

| Aturan | Bila gagal |
|---|---|
| Role harus ditemukan dengan `code` **dan** `id_company` sesi | `404` **"Role kustom tidak ditemukan"** |

Karena pencarian mensyaratkan `id_company` terisi, **role sistem (yang ber-`id_company` kosong)
tidak akan pernah ditemukan** — inilah mekanisme yang melindungi role sistem dari perubahan nama
dan penghapusan. Pesannya pun sudah menyebut "Role kustom".

Tambahan khusus `roles/delete`:

| Aturan | Bila gagal |
|---|---|
| Tidak boleh ada satu pun baris `user_roles` yang menunjuk role itu | `409` **"Role masih dipakai oleh user dan tidak dapat dihapus"** |

### BR-09 — Validasi `roles/permissions/update`

| # | Aturan | Bila gagal |
|---|---|---|
| 1 | Bila `code` = `superadmin` dan sesi bukan superadmin | `404` **"Role tidak ditemukan"** |
| 2 | Role ditemukan dengan `code` **dan** (`id_company` sesi **ATAU** `id_company` kosong) | `404` **"Role tidak ditemukan"** |
| 3 | **Semua** kode permission harus ada di tabel `permissions` | `400` `Permission tidak dikenal: {daftar kode}` |

Aturan 2 sengaja **lebih longgar** dari BR-08: ia mencakup role global, sehingga **permission role
sistem dapat diubah**. Ini perbedaan penting dan sepertinya disengaja — nama role sistem tetap,
tapi hak aksesnya bisa disesuaikan per perusahaan.

Aturan 3 diperiksa **sebelum** penghapusan permission lama, sehingga kegagalan validasi
**tidak merusak** state yang sudah ada. Ada test khusus untuk properti ini.

Yang **tidak** divalidasi: tidak ada batas minimum jumlah permission (array kosong diterima dan
mengosongkan seluruh permission role), dan tidak ada pemeriksaan bahwa aktor memiliki permission
yang ia berikan.

---

## 2. Aturan Penetapan Role

### BR-10 — Penetapan role selalu "ganti total", bukan tambah/kurang

Setiap kali `role_codes` dikirim (baik pada create maupun update):

1. **Semua** baris `user_roles` milik user itu **dihapus**
2. Untuk setiap kode dalam daftar, role dicari lalu baris baru disisipkan

Tidak ada operasi tambah/hapus selektif. Konsekuensi:

| Kondisi | Hasil |
|---|---|
| `role_codes` tidak dikirim (`undefined`) pada update | Role user **tidak disentuh** |
| `role_codes: []` dikirim | **Semua role user dihapus** |
| `role_codes` memuat kode yang sudah dimiliki | Dihapus lalu disisipkan kembali (hasil sama) |

### BR-11 — Kode role tidak dikenal dilewati tanpa error

Pencarian role: `{ code, idCompany }` **ATAU** `{ code, idCompany: NULL }`. Bila tidak ditemukan,
kode itu **dilewati** (`if (role) ...`) — **tidak ada pengecualian dilempar dan tidak ada
peringatan**.

Konsekuensi paling serius: mengirim `role_codes: ['tidak_ada']` menghapus semua role user lalu
gagal menyisipkan apa pun → user berakhir **tanpa role** → **tidak bisa login lagi** (login
menolak user tanpa role dengan `403`). Lihat KI-13.

### BR-12 — Role `superadmin` selalu dipulihkan untuk `id_user = 1`

Setelah penetapan role selesai, bila `idUser === 1`:

```
cari role dengan code = 'superadmin' dan id_company kosong
bila ditemukan → sisipkan baris user_roles untuk user 1
```

Ini pengaman agar akun developer tidak pernah terkunci. Berlaku **tanpa syarat** — termasuk saat
`role_codes: []`.

Nomor 1 di sini adalah konstanta `DEFAULT_SUPERADMIN_USER_ID`, bukan nilai yang dibaca dari data.

---

## 3. Aturan Penetapan Akses Cabang

### BR-13 — Penetapan cabang juga "ganti total"

Pola identik BR-10: semua baris `user_branch_accesses` dihapus, lalu disisipkan ulang.

| Kondisi | Hasil |
|---|---|
| `branch_ids` tidak dikirim pada update | Akses cabang **tidak disentuh** |
| `branch_ids: []` dikirim | **Semua akses cabang dihapus** → user login tanpa cabang (jalan buntu, KI-01 modul 01) |

### BR-14 — Cabang pertama pada daftar menjadi cabang default

```
untuk setiap i, branchIds[i]:
    isDefaultBranch = (i === 0)
```

Aturan ini **tidak terlihat di UI** — form hanya menampilkan checkbox tanpa penanda urutan atau
penanda "default". Urutan yang dikirim mengikuti urutan cabang di store (bukan urutan user
mencentang), sehingga cabang default bisa berpindah saat kombinasi centang berubah, tanpa
tanda apa pun.

Cabang default menentukan cabang mana yang dipilih otomatis saat user login (lihat
[01-auth-session/business-rules.md](../01-auth-session/business-rules.md) BR-07).

Karena penetapan selalu ganti-total, **tepat satu** cabang selalu bertanda default — sehingga
kekhawatiran "beberapa default" di modul 01 BR-07 tidak dapat terjadi lewat jalur UI ini. Jawaban
untuk M1-Q3: **ya, dijaga**, tapi secara implisit lewat urutan array, bukan lewat batasan data.

### BR-15 — Id cabang tidak divalidasi

`branch_ids` disisipkan apa adanya. Tidak ada pemeriksaan bahwa cabang itu ada, aktif, atau milik
perusahaan yang sama. Yang menahan hanyalah foreign key basis data — id yang tidak ada menghasilkan
`500`, bukan pesan bisnis.

Untuk sistem satu-perusahaan ini tidak berdampak, tetapi bila multi-perusahaan diaktifkan, ini
lubang isolasi tenant.

---

## 4. Aturan Kebijakan Role `superadmin`

### BR-16 — Empat lapis penjagaan

| Lapis | Mekanisme | Berlaku pada |
|---|---|---|
| 1. Penyaringan daftar | Query `NOT EXISTS` menyaring user pemegang `superadmin` | `users/list`, sesi non-superadmin |
| 2. Penyaringan daftar role | Filter di memori menyaring role `superadmin` | `roles/list`, sesi non-superadmin |
| 3. Penyamaran aksi | Target yang memegang `superadmin` diperlakukan sebagai **tidak ditemukan** | `users/update`, `update-status`, `change-password` |
| 4. Penolakan penetapan | Kode `superadmin` di `role_codes` ditolak eksplisit | `users/create`, `users/update` |

Lapis 3 diimplementasikan sebagai satu pemeriksaan bersama: bila sesi **adalah** superadmin →
lolos tanpa cek; bila bukan dan target memegang `superadmin` → `404` "User tidak ditemukan".

### BR-17 — Pola pesan: disamarkan, kecuali satu

Tiga dari empat lapis memakai pesan **"tidak ditemukan"**, bukan "tidak boleh" — sehingga
keberadaan akun/role tidak terkonfirmasi kepada pihak yang tidak berhak.

Pengecualiannya lapis 4: penolakan penetapan berbunyi
**"Role Super Admin hanya untuk akun developer default."** — menyebut role itu secara eksplisit.
Ini masuk akal (pengirim jelas sudah tahu nama rolenya karena ia yang mengirim), tapi patut
dicatat sebagai ketidakseragaman yang disengaja.

### BR-18 — Penjagaan di lapisan data

Di luar modul ini, migrasi `042_reserved_superadmin_role_policy.sql` dan seed **menghapus**
penetapan `superadmin` dari semua user selain `id_user = 1` setiap kali dijalankan, lalu
memastikan `id_user = 1` memilikinya.

Konstanta yang dipakai (`reserved-role-policy.ts`):

| Nama | Nilai |
|---|---|
| `RESERVED_SUPERADMIN_ROLE_CODE` | `'superadmin'` |
| `DEFAULT_SUPERADMIN_USER_ID` | `1` |

Perbandingan kode role selalu memakai `trim().toLowerCase()`, sehingga `"SuperAdmin "` juga
tertangkap.

---

## 5. Aturan Pencabutan Sesi

Modul ini adalah **satu-satunya** modul selain `auth` yang menulis ke tabel sesi.

### BR-19 — Kapan sesi dicabut

| Aksi | Sesi yang dicabut |
|---|---|
| Menonaktifkan user (`status = 'inactive'`) | **Semua** sesi aktif user itu dalam perusahaan itu |
| Mengaktifkan user (`status = 'active'`) | **Tidak ada** — sesi lama tetap mati |
| Mengganti password user lain | **Semua** sesi aktif user itu |
| Mengganti password **diri sendiri** | Semua sesi aktif **kecuali sesi yang sedang dipakai** |
| Mengubah role user | **Tidak ada** |
| Mengubah akses cabang user | **Tidak ada** |
| Menghapus semua role user | **Tidak ada** |

Kriteria pencabutan: `id_user` cocok **dan** `id_company` cocok **dan** `revoked_at IS NULL`.

Jumlah baris terpengaruh dicatat sebagai `revokedSessions` di metadata audit. Untuk ganti password,
jumlah itu juga dikembalikan ke pemanggil sebagai `revoked_sessions` — tetapi **UI tidak
menampilkannya**.

### BR-20 — Perubahan role & cabang berlaku tanpa mencabut sesi

Karena permission dan kelayakan cabang di-resolve ulang setiap request (modul 01 BR-08/BR-09),
perubahan role atau akses cabang **langsung berlaku** pada permintaan berikutnya tanpa perlu
mencabut sesi. Ini konsisten dan disengaja.

Yang perlu diketahui: daftar permission yang dipakai **frontend** untuk menyembunyikan menu berasal
dari ringkasan sesi di browser, yang hanya diperbarui saat login/`auth/me`/ganti role. Jadi setelah
adminnya mengubah role seseorang, menu di layar orang itu **belum berubah** sampai ia memuat ulang
aplikasi — meski server sudah menolak aksinya.

---

## 6. Aturan Transaksi & Audit

### BR-21 — Cakupan transaksi

| Operasi | Dalam transaksi? | Isi transaksi |
|---|---|---|
| `users/create` | **ya** | cek unik → simpan user → tetapkan role → tetapkan cabang |
| `users/update` | **ya** | cari → cek unik → simpan → tetapkan role → tetapkan cabang |
| `users/update-status` | **ya** | cari → simpan status → cabut sesi |
| `users/change-password` | **ya** | cari → simpan hash → cabut sesi |
| `roles/create` | tidak | satu penyimpanan tunggal |
| `roles/update` | tidak | satu penyimpanan tunggal |
| `roles/delete` | **tidak** | **dua** penghapusan berurutan (`role_permissions`, lalu `roles`) |
| `roles/permissions/update` | **tidak** | **hapus semua** lalu **sisipkan** |

Dua baris terakhir adalah operasi multi-langkah **tanpa** perlindungan transaksi. Bila langkah
kedua gagal:

- `roles/delete` → permission role terhapus, role tetap ada (role menjadi "kosong hak")
- `roles/permissions/update` → role kehilangan **semua** permission dan tidak mendapat yang baru

Ini melanggar aturan proyek "multi-write wajib dalam transaction". Lihat KI-25.

**Catatan penting tentang audit dalam transaksi:** `auditLog.log(...)` selalu dipanggil **setelah**
transaksi selesai (di luar blok), sehingga baris audit tidak ikut batal bila transaksi di-rollback.
Ini perilaku global yang sudah diputuskan untuk diperbaiki di sistem baru — lihat
[shared-services.md](../shared/shared-services.md) §2.1.

### BR-22 — `actionKey` audit yang ditulis modul ini

| `actionKey` | `entityType` | Isi `before` | Isi `after` | Metadata |
|---|---|---|---|---|
| `user.create` | `user` | — | `{ username }` | — |
| `user.update` | `user` | `{ fullName, email, username, phone, displayTitle }` | `{ fullName }` | — |
| `user.update_status` | `user` | `{ status }` | `{ status }` | `{ revokedSessions }` |
| `user.change_password` | `user` | — | `{ passwordChanged: true }` | `{ revokedSessions, retainedCurrentSession }` |
| `role.create` | `role` | — | `{ code, name }` | — |
| `role.update` | `role` | `{ name }` | `{ code, name }` | — |
| `role.delete` | `role` | `{ code, name }` | — | — |
| `role.permissions.update` | `role` | — | `{ code, permissions[] }` | — |

Semuanya memakai `idBranch: null` (aksi level-perusahaan) dan `actorType: 'user'`.

Perhatikan tiga ketidaklengkapan yang berdampak pada kemampuan audit:

1. `user.update` menyimpan **5 field** di `before` tapi hanya `fullName` di `after` — perubahan
   email/username/telepon/jabatan **tidak bisa dibaca hasilnya** dari audit, hanya nilai
   sebelumnya.
2. Perubahan **role** dan **akses cabang** **tidak dicatat sama sekali** — tidak di `before`, tidak
   di `after`. Padahal keduanya perubahan hak akses yang paling penting dilacak.
3. `role.permissions.update` mencatat daftar permission **baru** tapi tidak yang **lama**, sehingga
   tidak bisa diketahui permission apa yang dicabut.

Perhatikan juga penamaan `role.permissions.update` memakai **tiga** segmen dengan titik, sementara
konvensi proyek adalah `resource.action` snake_case dua segmen. Ini satu-satunya `actionKey` di
modul ini yang menyimpang.

**[PERLU KONFIRMASI]** apakah ketiga ketidaklengkapan itu dapat diterima, atau audit perubahan
hak akses perlu dilengkapi di sistem baru.

---

## 7. Aturan Envelope & Pesan Error

### BR-23 — Pemetaan error ke envelope

| Kondisi | Kelas exception | HTTP | `code` | `info` |
|---|---|---|---|---|
| Password terlalu pendek | `BadRequestException` | 400 | 200 | `validation_failed` |
| Role superadmin ditetapkan | `BadRequestException` | 400 | 200 | `validation_failed` |
| Status dikirim ke `users/update` | `BadRequestException` | 400 | 200 | `validation_failed` |
| Permission tidak dikenal | `BadRequestException` | 400 | 200 | `validation_failed` |
| User/role tidak ditemukan | `NotFoundException` | 404 | 300 | `not_found` |
| Username/email sudah dipakai | `ConflictException` | **409** | **1** | **`error`** |
| Kode role sudah dipakai | `ConflictException` | **409** | **1** | **`error`** |
| Kode role tidak valid | `ConflictException` | **409** | **1** | **`error`** |
| Role masih dipakai user | `ConflictException` | **409** | **1** | **`error`** |
| Permission kurang (guard) | `ForbiddenException` | 403 | 400 | `forbidden` |

Baris bertanda tebal adalah masalah nyata: `HttpExceptionFilter` **tidak memetakan HTTP 409**,
sehingga semua konflik jatuh ke kode error generik dengan `info: 'error'`.

### BR-24 — Pesan yang benar-benar dilihat user

Frontend modul ini memakai dua pola berbeda, keduanya membuang pesan ramah dari server:

**Pola A — aksi pengguna** (`users.slice`): pesan server **diabaikan sepenuhnya**.

```
notify("Gagal menyimpan pengguna", undefined, "danger")
```

Yang tampil: judul saja, tanpa deskripsi. Sehingga "Username sudah digunakan", "Email sudah
digunakan", dan "Password awal minimal 8 karakter" semuanya tampil identik.

**Pola B — aksi role** (`roles.slice`): memakai `apiError.info` — **kode mesin**.

| Penyebab nyata | Deskripsi toast yang tampil |
|---|---|
| Kode role sudah dipakai | `error` |
| Kode role tidak valid | `error` |
| Role masih dipakai user | `error` |
| Permission tidak dikenal | `validation_failed` |
| Role tidak ditemukan | `not_found` |

Padahal pesan Indonesia yang tepat tersedia di field `errors` respons. Ini pola yang sama dengan
KI-04 di modul 01 — lihat KI-17.

---

## 8. Aturan Daftar & Paginasi

### BR-25 — Paginasi `users/list`

```
page  = filters.page ?? 1
limit = Math.min(filters.limit ?? 20, 100)
skip  = (page - 1) * limit
```

Modul ini **tidak** memakai helper `normalizePageLimit` bersama, sehingga tidak punya perlindungan
terhadap nilai tidak wajar:

| `limit` dikirim | Hasil |
|---|---|
| tidak dikirim | 20 |
| 50 | 50 |
| 500 | 100 (dipangkas) |
| **0** | **0** → `LIMIT 0` → daftar selalu kosong |
| **-5** | **-5** → query gagal |
| **`"abc"`** | **`NaN`** → query gagal |

Perilaku ini identik dengan `audit-log/list` dan berbeda dari modul lain — dicatat sebagai
inkonsistensi C di [shared-services.md](../shared/shared-services.md) §8.

Urutan hasil: **`fullName` ASC**.

### BR-26 — Pencarian `users/list`

```sql
(full_name LIKE '%{term}%' OR username LIKE '%{term}%' OR email LIKE '%{term}%')
```

Satu term utuh, **bukan** dipecah per kata. Modul ini tidak memakai `applyTokenizedLike` bersama,
sehingga:

- Pencarian tahan huruf besar/kecil (bergantung collation `utf8mb4_unicode_ci` → ya, tidak
  case-sensitive)
- Pencarian **tidak** tahan urutan kata: "santoso budi" tidak menemukan "Budi Santoso"
- Pencarian **tidak** tahan spasi ganda di data

### BR-27 — `roles/list` tanpa paginasi

Seluruh role dikembalikan sekaligus. Urutan: **`isSystemRole` DESC, `name` ASC** — role sistem
lebih dulu, lalu alfabetis.

Deskripsi role diisi otomatis bila kosong:

```
role.description ?? (isSystemRole ? "Role sistem {name}" : "Role kustom {name}")
```

Perhatikan operator `??` — nilai **string kosong** tetap dipakai (bukan diganti fallback). Jadi
role yang deskripsinya sengaja dikosongkan lewat form akan tampil tanpa deskripsi, sementara role
yang `NULL` mendapat teks otomatis.

### BR-28 — Penyembunyian data pada daftar

| Endpoint | Mekanisme | Efek pada total |
|---|---|---|
| `users/list` | Subquery `NOT EXISTS` di dalam query | **Total ikut berkurang** |
| `roles/list` | Filter array setelah query | Tidak ada total yang dilaporkan |

Perbedaan ini berarti daftar user menyembunyikan secara "jujur" (angka total konsisten dengan
baris yang tampil), sementara daftar role menyembunyikan setelah pengambilan data.

### BR-29 — Field yang tidak pernah dikembalikan

`passwordHash` di-set `undefined` sebelum respons dikirim, pada `list`, `create`, dan `update`.
Karena serialisasi JSON menghilangkan key ber-nilai `undefined`, hash **tidak pernah** bocor ke
klien.

---

## 9. Formula & Perhitungan

Modul ini **tidak memiliki formula bisnis** — tanpa uang, kuantitas, pajak, atau pembulatan.
Perhitungan yang ada:

| Perhitungan | Rumus |
|---|---|
| Offset paginasi | `(page - 1) * limit` |
| Batas atas limit | `Math.min(limit ?? 20, 100)` |
| Total halaman (UI) | `Math.ceil(total / limit)` |
| Rentang baris (UI) | `dari = (page-1)*limit + 1`, `sampai = Math.min(page*limit, total)` |
| Penanda cabang default | `isDefaultBranch = (index === 0)` |
| Cost hash password | bcrypt **12** (create dan change-password) |
| Normalisasi kode role | lihat [numbering-sequence.md](numbering-sequence.md) |

---

## 10. Aturan Approval

**Tidak ada aturan approval di modul ini.** Tidak ada persetujuan berjenjang untuk membuat user,
mengubah role, atau mengubah permission — semuanya berlaku langsung begitu disimpan.

Yang paling dekat dengan approval adalah pembagian permission antara `owner` (boleh kelola akun)
dan `admin` (boleh kelola role) — tetapi itu pembagian tugas, bukan alur persetujuan: tidak ada
pihak kedua yang harus menyetujui perubahan pihak pertama.

Sebagai pembanding, mekanisme approval nyata ada di modul Stock (penyesuaian stok mengirim
kredensial approver dalam payload). **[PERLU KONFIRMASI]** apakah verifikasi kredensial di sana
memakai ulang logika modul ini atau punya salinan sendiri — ditelusuri saat analisis modul 16
(pertanyaan M1-Q5 dari modul 01, masih terbuka).

---

## 11. Ringkasan Kondisi Khusus

| # | Kondisi | Aturan |
|---|---|---|
| SK-01 | `role_codes: []` dikirim | Semua role dihapus; **user tidak bisa login lagi**. Tidak ada penjagaan |
| SK-02 | Kode role tak dikenal dikirim | Dilewati tanpa error; bisa berujung user tanpa role |
| SK-03 | `branch_ids: []` dikirim | Semua akses cabang dihapus; user login tapi terjebak tanpa cabang |
| SK-04 | Menyimpan role user `id_user = 1` | Role `superadmin` selalu ditambahkan kembali |
| SK-05 | Email kosong saat create | Tersimpan `''`; user kedua dengan email kosong → **500** |
| SK-06 | Email kosong saat update | Tersimpan `NULL` (berbeda dari create) |
| SK-07 | Jabatan kosong | Tersimpan string kosong (bukan `NULL`) |
| SK-08 | Menonaktifkan diri sendiri | **Diizinkan** — admin langsung terputus dari semua perangkat |
| SK-09 | Mengganti password sendiri | Sesi yang sedang dipakai dipertahankan; perangkat lain terputus |
| SK-10 | Mengaktifkan kembali user | Sesi lama tidak dihidupkan |
| SK-11 | Permission array kosong pada role | Diterima; role kehilangan seluruh hak akses |
| SK-12 | Permission tak dikenal dalam daftar | Ditolak **sebelum** permission lama dihapus |
| SK-13 | Mengubah permission role sistem | **Diizinkan** untuk semua pemegang `role.manage` |
| SK-14 | Mengubah permission role `superadmin` | Ditolak untuk sesi non-superadmin |
| SK-15 | Kode role dua nama berbeda menormalisasi sama | Yang kedua ditolak sebagai duplikat |
| SK-16 | Seed dijalankan ulang | Permission yang dihapus dari role sistem **muncul kembali** |
| SK-17 | Edit user di luar 100 data pertama | Form terbuka dalam mode tambah tanpa peringatan |
