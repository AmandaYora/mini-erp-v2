# User Flows — Modul 02 Users, Roles & Permissions

**Kelompok A — alur end-to-end yang harus identik di sistem baru.** Nama endpoint, teks UI, dan
urutan langkah ditulis apa adanya dari kode.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [ui-ux-spec.md](ui-ux-spec.md) ·
[business-rules.md](business-rules.md) · [test-cases.md](test-cases.md)

Notasi: `→` langkah berikutnya · **[server]** panggilan API · **[lokal]** hanya di browser

---

## UF-01 — Membuka daftar pengguna

1. User membuka menu **Pengaturan → "Pengguna"** (atau URL `/users`)
2. Penjaga rute memeriksa permission `user.view`; bila tidak punya → `/403` "Akses Ditolak"
3. **[server]** Saat modul pertama kali dikunjungi, hook modul memuat **dua** hal paralel:
   `users/list` (limit 100, untuk store) dan `roles/list` (untuk daftar role di form)
4. **[server]** Halaman **juga** memanggil `users/list` sendiri dengan `page: 1, limit: 20`
   — jadi ada **dua permintaan daftar pengguna** dengan batas berbeda saat halaman dibuka
5. Tabel tampil terurut **nama A→Z**
6. Bila sesi bukan `superadmin`, baris user yang memegang role `superadmin` **tidak muncul** dan
   tidak ikut dihitung di total

Deskripsi kartu menampilkan `{total} user perusahaan beserta role dan akses cabangnya.`

---

## UF-02 — Mencari pengguna

1. User mengetik di kotak **"Cari pengguna"**
2. **[lokal]** Setiap karakter langsung: mereset halaman ke 1 **dan** memicu permintaan baru
3. **[server]** `users/list` dengan `search` (sudah di-trim), `page: 1`, `limit: 20`
4. Server mencari **satu frasa utuh** di nama lengkap, username, atau email
5. Tabel dan paginasi diperbarui

Konsekuensi yang perlu diketahui: mengetik "budi" menghasilkan 4 permintaan berurutan (`b`, `bu`,
`bud`, `budi`). Tidak ada penundaan pengetikan. Lihat [known-issues.md](../known-issues.md) KI-24.

Pencarian **beberapa kata** diperlakukan sebagai satu frasa — "budi santoso" tidak akan menemukan
"Budi Ahmad Santoso". Ini berbeda dari modul lain yang memakai pencarian per-kata.

---

## UF-03 — Menambah pengguna baru

1. Dari daftar, user menekan **"Tambah Pengguna"** (butuh `user.create`) → `/users/create`
2. Form terbuka dalam mode tambah:
   - Judul **"Tambah Pengguna"**, breadcrumb "Dashboard > Pengguna > Tambah"
   - Role **`staff`** sudah tercentang
   - Cabang default perusahaan sudah tercentang
   - Dua field password tampil dan wajib
3. User mengisi: Nama lengkap → Jabatan → Email → Username → No. WhatsApp (opsional) →
   Password awal → Konfirmasi password
4. User mencentang role dan cabang yang diperlukan
5. User menekan **"Tambah Pengguna"**
6. **[lokal]** Validasi klien berurutan:
   - Field wajib kosong / format email salah → balon validasi bawaan browser
   - Password < 8 karakter → balon **"Password awal minimal 8 karakter."**
   - Password ≠ konfirmasi → balon **"Konfirmasi password tidak sama."**
7. **[server]** `users/create` dengan `full_name`, `email`, `username`, `password`, `phone`,
   `role_codes`, `branch_ids`, `display_title`
   - Payload **tidak** mengirim `status` → server menetapkan `active`
8. Server dalam satu transaksi: cek username unik → cek email unik → hash password (bcrypt 12) →
   simpan user → tetapkan role → tetapkan cabang
   - **Cabang pertama pada daftar menjadi cabang default user**
9. Server mencatat audit `user.create`
10. **[server]** Daftar pengguna di store dimuat ulang (`users/list`, limit 100)
11. **[lokal]** Pindah ke `/users` — **tanpa toast sukses**

Bila langkah 7–8 gagal: tetap di form, toast danger **"Gagal menyimpan pengguna"** tanpa alasan.
Penyebab paling umum: username atau email sudah dipakai — tapi pesannya tidak sampai ke user
(KI-17).

---

## UF-04 — Mengedit profil pengguna

1. Dari daftar, user menekan **"Detail"** pada satu baris (butuh `user.update`) →
   `/users/{id}/edit`
2. **[lokal]** Form mencari data user di **daftar yang sudah dimuat di store** (bukan permintaan
   detail baru)
   - Bila user tidak ada di 100 data pertama → form terbuka dalam **mode tambah** tanpa peringatan
     (KI-14)
3. Form terbuka dalam mode edit:
   - Judul **"Edit Pengguna"**, breadcrumb "... > Edit"
   - Field profil terisi nilai saat ini
   - Kartu terpisah **"Ubah Password"** tampil, kosong, dengan placeholder "Kosongkan jika tidak
     diubah"
   - Centang role dan cabang mengikuti kondisi saat ini
4. User mengubah field yang perlu, **membiarkan password kosong**
5. User menekan **"Simpan Perubahan"**
6. **[server]** `users/update` dengan `id_user` dan field profil + `role_codes` + `branch_ids`
   - Payload **tidak** mengirim `status` — bila dikirim, server menolaknya
7. Server: cari user dalam perusahaan → periksa kelayakan target → cek unik email/username bila
   berubah → simpan → tetapkan ulang role → tetapkan ulang cabang
8. Server mencatat audit `user.update`
9. **[server]** Daftar dimuat ulang → **[lokal]** pindah ke `/users`

**Penting:** karena role dan cabang **selalu ditetapkan ulang dari nol** (hapus semua lalu isi
ulang), mengirim daftar kosong akan mengosongkan role atau akses cabang user — dan itu punya
akibat serius (lihat UF-09).

---

## UF-05 — Mengganti password pengguna (oleh admin)

1. Langkah 1–3 sama seperti UF-04
2. User mengisi **"Password baru"** dan **"Konfirmasi password baru"**
3. User menekan **"Simpan Perubahan"**
4. **[lokal]** Validasi: panjang minimal 8 (**"Password baru minimal 8 karakter."**) dan
   kecocokan konfirmasi
5. **[server]** **Dua permintaan berurutan:**
   1. `users/update` — menyimpan profil
   2. `users/change-password` — `{ id_user, new_password }`
6. Server pada permintaan kedua: hash password (bcrypt 12) → simpan → **cabut semua sesi aktif
   user itu**
   - Bila admin mengganti **password dirinya sendiri**, sesi yang sedang dipakai
     **dipertahankan** — admin tidak ter-logout
7. Server mencatat audit `user.change_password` beserta jumlah sesi yang dicabut
8. Respons memuat `revoked_sessions`, tetapi **UI tidak menampilkannya**
9. **[lokal]** Pindah ke `/users`

Akibat yang dirasakan pengguna target: ia langsung terputus di semua perangkat dan harus login
dengan password baru. **Tidak ada pemberitahuan** kepadanya (tidak ada email/WA), dan admin juga
tidak melihat konfirmasi berapa sesi yang terputus.

Risiko yang perlu diketahui: bila permintaan pertama berhasil tapi kedua gagal, **profil
tersimpan tetapi password tidak** — tanpa pembatalan dan tanpa pesan yang menjelaskan bagian mana
yang gagal (KI-16).

---

## UF-06 — Menonaktifkan pengguna

1. Dari daftar, user menekan tombol **"Nonaktifkan"** pada baris target (butuh `user.archive`)
2. **Tidak ada dialog konfirmasi** — aksi langsung dijalankan
3. **[server]** `users/update-status` dengan `{ id_user, status: 'inactive' }`
4. Server: cari user → periksa kelayakan target → set status → **cabut semua sesi aktifnya**
5. Server mencatat audit `user.update_status` dengan metadata jumlah sesi tercabut
6. **[server]** Daftar dimuat ulang → badge berubah menjadi kuning `inactive`, tombol berganti
   menjadi **"Aktifkan"**

Yang dirasakan pengguna target: pada permintaan berikutnya ia mendapat `401`, klien mencoba
memperbarui token satu kali, gagal, lalu ia berada di halaman login tanpa penjelasan (lihat
[01-auth-session/user-flows.md](../01-auth-session/user-flows.md) UF-14).

Bila gagal: toast danger **"Gagal mengubah status pengguna"** tanpa alasan.

---

## UF-07 — Mengaktifkan kembali pengguna

1. Dari daftar, user menekan **"Aktifkan"** pada baris berstatus `inactive`
2. **[server]** `users/update-status` dengan `{ id_user, status: 'active' }`
3. Server set status; **sesi tidak disentuh** — sesi yang sudah dicabut tetap mati
4. Badge kembali hijau `active`

Pengguna target harus **login ulang** — reaktivasi tidak menghidupkan sesi lamanya.

---

## UF-08 — Mencoba mengubah status dari form edit

Alur ini tidak bisa dicapai lewat UI (form tidak punya field status), tapi kontraknya perlu
dipertahankan bila ada integrasi lain:

1. **[server]** `users/update` dengan `status` di dalam payload
2. Server menolak dengan `400` dan pesan
   **"Status user hanya dapat diubah lewat aksi aktifkan/nonaktifkan."**
3. Tidak ada perubahan tersimpan — penolakan terjadi sebelum menyentuh basis data

---

## UF-09 — Pengguna kehilangan seluruh role atau akses cabang

Alur ini **bukan fitur**, tapi konsekuensi nyata dari UF-04 yang perlu dipahami sebelum rebuild.

**Skenario A — semua role dihapus:**

1. Admin membuka edit pengguna dan **menghapus semua centang role**
2. **[server]** `users/update` dengan `role_codes: []`
3. Server menghapus semua baris role user, tidak menyisipkan apa pun — **tanpa penolakan**
4. Pengguna target masih bisa memakai sesi yang sedang berjalan (permission dari role lamanya
   sudah tidak ada, jadi praktis ia kehilangan akses menu)
5. Begitu ia logout atau sesinya berakhir, **ia tidak bisa login lagi**: login menolak user tanpa
   role dengan `403` "User tidak memiliki role"
6. Pemulihan hanya bisa dilakukan admin lain lewat UI, atau langsung di basis data

**Skenario B — semua cabang dihapus:**

1. Admin menghapus semua centang cabang → `branch_ids: []`
2. Pengguna target tetap bisa login, tapi **tanpa cabang aktif**
3. Ia terjebak di halaman pemilihan cabang yang kosong dan tanpa tombol logout (KI-01)

Keduanya tidak dijaga sama sekali di klien maupun server. Lihat KI-13.

---

## UF-10 — Membuka halaman Role & Akses

1. User membuka menu **Pengaturan → "Role & Akses"** (atau URL `/settings/roles`)
2. Penjaga rute memeriksa `role.manage`; bila tidak punya → `/403`
3. **[server]** `roles/list` — mengembalikan **seluruh** role tanpa paginasi
4. Tabel role tampil: **role sistem lebih dulu**, lalu urut nama A→Z
5. Bila sesi bukan `superadmin`, role `superadmin` **tidak muncul**
6. Matriks Hak Akses menampilkan notice **"Pilih salah satu role untuk mulai mengatur
   permission."**

---

## UF-11 — Menambah role kustom

1. Di kartu **"Tambah / Edit Role"**, user mengisi **"Nama role"** (mis. "Kepala Gudang") dan
   opsional **"Deskripsi"**
2. User menekan **"Tambah Role"**
3. **[server]** `roles/create` dengan `{ name, description }` — **tanpa `code`**
4. Server membuat kode dari nama: "Kepala Gudang" → `kepala_gudang`
5. Server memeriksa kode belum dipakai (termasuk oleh role sistem) → menyimpan dengan
   `is_system_role = false`
6. Server mencatat audit `role.create`
7. **[server]** Daftar role dimuat ulang
8. Toast success **"Role berhasil ditambahkan"**
9. **[lokal]** Form dikosongkan dan pilihan role dibatalkan

Role baru muncul di tabel dengan badge **"Kustom"** dan **tanpa permission apa pun** — harus
diatur terpisah lewat UF-13.

Bila kode bentrok: toast danger **"Gagal menyimpan role"** dengan deskripsi berisi kode mesin
(`error`), bukan pesan `Kode role '{kode}' sudah digunakan` yang sudah disiapkan server (KI-17).

---

## UF-12 — Mengubah nama / deskripsi role

1. Di tabel role, user menekan **"Atur"** pada baris target
2. **[lokal]** Dua hal terjadi bersamaan:
   - Form kanan terisi nama & deskripsi role itu
   - Role itu menjadi **role terpilih untuk matriks permission** → matriks terbuka di bawah
3. User mengubah nama/deskripsi lalu menekan **"Simpan Role"**
4. **[lokal]** Bila role itu **role sistem**: permintaan **tidak dikirim**; muncul toast warning
   **"Role sistem tidak dapat diubah"** / "Gunakan role kustom untuk kebutuhan tambahan."
5. **[server]** Untuk role kustom: `roles/update` dengan `{ code, name, description }`
6. Server mencari role **milik perusahaan** dengan kode itu → menyimpan → audit `role.update`
7. Toast success **"Role berhasil diperbarui"**
8. **[lokal]** Form dikosongkan **dan pilihan role dibatalkan** → **matriks permission menutup
   kembali** ke notice

Langkah 8 punya efek yang mengganggu alur kerja: bila user sedang mengatur permission lalu
menyimpan perubahan nama, ia harus menekan "Atur" lagi untuk melanjutkan pengaturan permission.

---

## UF-13 — Mengatur permission sebuah role

1. Di tabel role, user menekan **"Atur"** → matriks terbuka menampilkan 22 kelompok modul
2. Pill permission yang sudah dimiliki role tampil **tercentang**
3. User mengklik satu pill (mis. "Produk & Item: Tambah")
4. **[lokal]** Tampilan pill langsung berubah (optimistis), dan daftar permission baru dihitung:
   permission lama **ditambah/dikurangi** satu itu, lalu dideduplikasi
5. **[server]** `roles/permissions/update` dengan `{ code, permissions: [seluruh daftar baru] }`
   — **bukan hanya yang berubah**
6. Server: validasi semua kode permission dikenal → hapus semua permission role → sisipkan daftar
   baru → audit `role.permissions.update`
7. **[server]** Daftar role dimuat ulang
8. Setiap klik pill berikutnya mengulang langkah 3–7 — **satu permintaan per klik**

Bila gagal:
1. Pill dikembalikan ke kondisi sebelumnya
2. Daftar role dimuat ulang
3. Toast danger **"Gagal memperbarui permission role"** dengan deskripsi kode mesin

**Tidak ada tombol simpan** untuk matriks. Tidak ada indikator "menyimpan…" per pill selain
loader global.

---

## UF-14 — Mengatur permission role SISTEM

Alur ini identik UF-13 dan **diizinkan** — inilah yang membedakannya dari mengubah nama role
sistem yang diblokir.

1. User menekan "Atur" pada role sistem (mis. **Admin**)
2. Matriks terbuka; permission role sistem bisa dicentang/dihapus centang seperti role kustom
3. Perubahan berlaku untuk **semua** pengguna yang memegang role itu, seketika (permission
   di-resolve ulang setiap request — lihat modul 01 BR-09)

Yang perlu diketahui sebagai konsekuensi keamanan: pemegang `role.manage` (termasuk role **admin**
bawaan) dapat menambahkan permission apa pun ke role yang ia sendiri pegang. Lihat KI-15.

Satu-satunya role yang terlindungi adalah `superadmin`: sesi non-superadmin yang mencoba mengubah
permissionnya mendapat `404` **"Role tidak ditemukan"**.

---

## UF-15 — Menghapus role kustom

1. Di tabel role, user menekan **"Hapus"** (tombol danger, hanya ada pada role kustom)
2. **Tidak ada dialog konfirmasi** — penghapusan langsung dijalankan
3. **[server]** `roles/delete` dengan `{ code }`
4. Server mencari role **milik perusahaan** → menghitung jumlah user yang memegangnya
5. Bila **masih dipakai**: `409` dan penghapusan dibatalkan → toast danger
   **"Gagal menghapus role"** dengan deskripsi kode mesin (pesan sebenarnya:
   "Role masih dipakai oleh user dan tidak dapat dihapus")
6. Bila tidak dipakai: baris `role_permissions` dihapus, lalu baris role **dihapus permanen**
   (hard delete)
7. Server mencatat audit `role.delete` menyimpan kode & nama role di kolom "sebelum"
8. **[server]** Daftar dimuat ulang → toast success **"Role berhasil dihapus"**

Karena hard delete, satu-satunya jejak keberadaan role itu setelahnya adalah entri audit log.

---

## UF-16 — Alur penyembunyian akun & role Super Admin

Bukan aksi user, tapi perilaku yang konsisten muncul di seluruh modul dan harus direplikasi utuh.

Untuk **sesi non-superadmin**:

| Aksi | Hasil |
|---|---|
| Membuka daftar pengguna | Baris user pemegang `superadmin` **tidak muncul**, total ikut berkurang |
| Membuka daftar role | Role `superadmin` **tidak muncul** |
| Mengedit user superadmin lewat API | `404` **"User tidak ditemukan"** |
| Mengganti password user superadmin | `404` **"User tidak ditemukan"** |
| Mengubah status user superadmin | `404` **"User tidak ditemukan"** |
| Mengubah permission role `superadmin` | `404` **"Role tidak ditemukan"** |
| Menetapkan role `superadmin` ke siapa pun | `400` **"Role Super Admin hanya untuk akun developer default."** |
| Melihat pilihan role di form pengguna | `superadmin` disaring dari daftar checkbox |

Perhatikan pola pesannya: penolakan disamarkan sebagai **"tidak ditemukan"**, bukan "tidak
boleh" — sehingga keberadaan akun/role itu tidak terkonfirmasi. Satu pengecualian: penetapan role
`superadmin` ditolak dengan pesan yang **menyebutkan** role itu secara eksplisit.

Untuk **sesi superadmin**: seluruh penyaringan di atas tidak berlaku.

---

## UF-17 — Menyimpan role pengguna `id_user = 1` (akun developer)

1. Admin (dengan sesi superadmin) mengedit user `id_user = 1` dan menghapus centang semua role
2. **[server]** `users/update` dengan `role_codes: []`
3. Server menghapus semua role user itu, lalu — karena `id_user = 1` — **menambahkan kembali role
   `superadmin` secara otomatis**
4. Hasil: user 1 selalu memegang minimal role `superadmin`, apa pun yang dipilih di form

Ini pengaman agar akun developer tidak pernah terkunci. Efek sampingnya: form pengguna
**menampilkan hasil yang berbeda dari yang dipilih** — user 1 akan tampil memegang `superadmin`
meski checkbox itu bahkan tidak ada di form.

---

## 18. Matriks Permission per Aksi

| Aksi | Permission | Endpoint |
|---|---|---|
| Melihat daftar pengguna | `user.view` | `users/list` |
| Menambah pengguna | `user.create` | `users/create` |
| Mengedit profil pengguna | `user.update` | `users/update` |
| Mengganti password pengguna | **`user.update`** | `users/change-password` |
| Mengaktifkan/menonaktifkan pengguna | **`user.archive`** | `users/update-status` |
| Melihat daftar role | `role.manage` | `roles/list` |
| Menambah role | `role.manage` | `roles/create` |
| Mengubah role | `role.manage` | `roles/update` |
| Menghapus role | `role.manage` | `roles/delete` |
| Mengatur permission role | `role.manage` | `roles/permissions/update` |

Dua hal yang perlu dicatat:

- **Tidak ada permission khusus untuk ganti password.** Siapa pun yang boleh mengedit profil juga
  boleh mengganti password siapa pun (kecuali superadmin).
- **Seluruh operasi role memakai satu permission (`role.manage`).** Tidak ada pemisahan
  lihat/ubah/hapus. Artinya memberi seseorang akses melihat daftar role otomatis memberi hak
  mengubah dan menghapusnya.

### Kemampuan per role bawaan

| Role | `user.view` | `user.create` | `user.update` | `user.archive` | `role.manage` |
|---|---|---|---|---|---|
| `superadmin` | ✔ | ✔ | ✔ | ✔ | ✔ |
| `owner` | ✔ | ✔ | ✔ | ✔ | ✔ |
| `admin` | ✔ | ✘ | ✘ | ✘ | **✔** |
| `staff` | ✘ | ✘ | ✘ | ✘ | ✘ |
| `kasir` | ✘ | ✘ | ✘ | ✘ | ✘ |

Baris `admin` adalah yang paling perlu diperhatikan: ia **boleh mengelola role dan permission**
tetapi **tidak boleh mengelola akun**. Pembagian tugas ini sudah dikonfirmasi disengaja (admin
mengatur struktur akses, owner mengelola akun orang), namun ditegakkan hanya oleh konvensi —
lihat KI-15.
