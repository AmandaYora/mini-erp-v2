# Test Cases — Modul 02 Users, Roles & Permissions

**Kelompok A — skenario input→output nyata, diturunkan dari kode dan test yang sudah ada.**
Semua ekspektasi dapat diverifikasi terhadap sistem lama sebelum sistem baru dianggap setara.

Sumber: `user.service.spec.ts` (**27 kasus**), `role.service.spec.ts` (**22 kasus**),
`users-pages.test.tsx` (4), `roles-settings-page.test.tsx` (4), `role-access-config.test.ts` (3),
`roles.slice.test.tsx` (1), `use-users-module.test.ts`, `18-role-access-matrix.spec.ts` (2 E2E),
plus kasus turunan pembacaan kode (ditandai **[dari kode]**).

Kredensial & data bawaan: lihat [feature-inventory.md](feature-inventory.md) §5.

---

## 1. `users/list`

### TC-UL01 — Menyembunyikan pemegang role superadmin dari sesi non-superadmin ✅ *ada di spec*

Sesi: `owner`.
Diharapkan: query memuat klausa `NOT EXISTS` dengan parameter
`{ reservedRoleCode: 'superadmin' }`.

### TC-UL02 — Tidak menyaring untuk sesi superadmin ✅ *ada di spec*

Sesi: `superadmin`.
Diharapkan: klausa `NOT EXISTS` **tidak** ditambahkan.

### TC-UL03 — Paginasi bawaan **[dari kode]**

```
Request : { data: {} }
```
Diharapkan: `page = 1`, `limit = 20`, `meta = { page: 1, limit: 20, total: <n> }`, urut nama A→Z.

### TC-UL04 — Limit dipangkas ke 100 **[dari kode]**

```
Request : { data: { limit: 500 } }
```
Diharapkan: `meta.limit = 100`, maksimal 100 baris.

### TC-UL05 — Limit 0 menghasilkan daftar kosong **[dari kode]**

```
Request : { data: { limit: 0 } }
```
Diharapkan (perilaku **aktual** sistem lama): `Math.min(0, 100) = 0` → `LIMIT 0` → `items: []`
dengan `meta.total` tetap benar. **Bukan** fallback ke 20.
Menguji ketiadaan clamp bawah — lihat [known-issues.md](../known-issues.md) KI-26.

### TC-UL06 — Limit bukan angka **[dari kode]**

```
Request : { data: { limit: "abc" } }
```
Diharapkan: `Math.min(NaN, 100) = NaN` → query gagal → **500**, bukan pesan bisnis.

### TC-UL07 — Pencarian satu kata **[dari kode]**

```
Request : { data: { search: "budi" } }
```
Diharapkan: cocok bila `budi` muncul di nama lengkap, username, **atau** email (LIKE `%budi%`,
tidak peka huruf besar/kecil).

### TC-UL08 — Pencarian dua kata diperlakukan sebagai satu frasa **[dari kode]**

Data: user bernama `Budi Ahmad Santoso`.

```
Request : { data: { search: "budi santoso" } }
```
Diharapkan: **TIDAK ditemukan** — pencarian mencari frasa utuh `%budi santoso%`.
Ini berbeda dari modul lain yang memakai pencarian per-kata. Menguji KI-25.

### TC-UL09 — `passwordHash` tidak pernah dikembalikan **[dari kode]**

Diharapkan: tidak ada key `passwordHash` di setiap item respons.

### TC-UL10 — Bentuk item respons **[dari kode]**

Diharapkan setiap item memuat: field user (tanpa hash) + `roles: [{ code, name }]` +
`branchIds: number[]`.

### TC-UL11 — `last_login_at` ikut dikembalikan tapi tak dipakai **[dari kode]**

Diharapkan: field itu **ada** di respons (karena entity di-spread apa adanya) tetapi tidak
ditampilkan di UI mana pun. Menguji temuan di [reports-list.md](reports-list.md) §4.

---

## 2. `users/create`

### TC-UC01 — Membuat user dengan password ter-hash ✅ *ada di spec*

```
Request : { data: { full_name: "Budi Santoso", email: "budi@test.com", username: "budi",
                    password: "secret123", role_codes: ["cashier"], branch_ids: [2] } }
```
Diharapkan:
- User dibuat dengan `username: "budi"` dan `passwordHash` hasil bcrypt (cost **12**)
- Audit `user.create` tercatat
- `result.passwordHash` **undefined**

### TC-UC02 — Username sudah dipakai ✅ *ada di spec*

Diharapkan: **409** `ConflictException` **"Username sudah digunakan"**.

### TC-UC03 — Password terlalu pendek ✅ *ada di spec*

```
Request : { data: { ..., password: "short" } }
```
Diharapkan: **400** **"Password awal minimal 8 karakter"**, dan `userRepo.save` **tidak pernah
dipanggil** (ditolak sebelum transaksi).

### TC-UC04 — Menetapkan role superadmin ditolak ✅ *ada di spec*

```
Request : { data: { ..., role_codes: ["superadmin"] } }
```
Diharapkan: **400** **"Role Super Admin hanya untuk akun developer default."**, dan `save`
**tidak** dipanggil.

### TC-UC05 — Role ditetapkan setelah user disimpan ✅ *ada di spec*

Diharapkan: `userRoleRepo.delete({ idUser })` dipanggil lebih dulu, lalu `save`.

### TC-UC06 — Cabang ditetapkan setelah user disimpan ✅ *ada di spec*

```
Request : { data: { ..., branch_ids: [2, 3] } }
```
Diharapkan: `ubaRepo.delete({ idUser })` sekali, lalu `ubaRepo.save` **dua kali**.

### TC-UC07 — Cabang pertama menjadi default ✅ *ada di spec*

```
Request : { data: { ..., branch_ids: [2, 3] } }
```
Diharapkan: baris pertama `isDefaultBranch = true`, baris kedua `false`.
**Aturan ini tidak terlihat di UI** — menguji BR-14.

### TC-UC08 — Email sudah dipakai **[dari kode]**

Diharapkan: **409** **"Email sudah digunakan"**.

### TC-UC09 — Email string kosong pada user pertama **[dari kode]**

```
Request : { data: { ..., email: "" } }
```
Diharapkan: berhasil; email tersimpan sebagai **string kosong**, bukan `NULL` (pemeriksaan unik
dilewati karena nilainya falsy).

### TC-UC10 — Email string kosong pada user KEDUA **[dari kode]**

Langkah: buat user A dengan `email: ""`, lalu user B dengan `email: ""`.
Diharapkan langkah 2 (perilaku **aktual**): bentrok indeks unik MySQL → **500 `internal_error`**,
bukan `409` berpesan jelas. Menguji KI-22.

### TC-UC11 — Status bawaan **[dari kode]**

```
Request : { data: { ... } }   // tanpa status
```
Diharapkan: `status = "active"`.

### TC-UC12 — Kode role tidak dikenal dilewati tanpa error **[dari kode]**

```
Request : { data: { ..., role_codes: ["role_yang_tidak_ada"] } }
```
Diharapkan (perilaku **aktual**): user **berhasil dibuat** dengan **nol role** — tanpa error,
tanpa peringatan. User itu kemudian **tidak bisa login** (`403` "User tidak memiliki role").
Menguji KI-13. **Belum ada test.**

### TC-UC13 — Id cabang tidak dikenal **[dari kode]**

```
Request : { data: { ..., branch_ids: [999999] } }
```
Diharapkan: gagal di foreign key → **500**, bukan pesan bisnis.

### TC-UC14 — Username unik bersifat global, bukan per-perusahaan **[dari kode]**

Diharapkan: pemeriksaan `findOne({ where: { username } })` **tanpa** `idCompany` → username yang
dipakai perusahaan lain tetap ditolak.

---

## 3. `users/update`

### TC-UU01 — Mengubah nama lengkap ✅ *ada di spec*

Diharapkan: user tersimpan dengan nama baru; audit `user.update` tercatat.

### TC-UU02 — User tidak ada ✅ *ada di spec*

Diharapkan: **404** **"User tidak ditemukan"**.

### TC-UU03 — Username baru sudah dipakai user lain ✅ *ada di spec*

Diharapkan: **409** **"Username sudah digunakan"**.

### TC-UU04 — Mempertahankan username yang sama tidak dianggap bentrok ✅ *ada di spec*

Diharapkan: berhasil — pemeriksaan mengecualikan diri sendiri.

### TC-UU05 — Perubahan status ditolak ✅ *ada di spec*

```
Request : { data: { id_user: 10, status: "inactive" } }
```
Diharapkan: **400** **"Status user hanya dapat diubah lewat aksi aktifkan/nonaktifkan."**

### TC-UU06 — Menetapkan role superadmin lewat update ditolak ✅ *ada di spec*

Diharapkan: **400** (pesan sama TC-UC04).

### TC-UU07 — Field yang tidak dikirim tidak berubah **[dari kode]**

```
Request : { data: { id_user: 10, full_name: "Nama Baru" } }
```
Diharapkan: email, username, telepon, jabatan, role, dan cabang **tidak tersentuh** (role & cabang
hanya ditetapkan ulang bila `role_codes`/`branch_ids` dikirim).

### TC-UU08 — Mengosongkan email menyimpan NULL **[dari kode]**

```
Request : { data: { id_user: 10, email: "" } }
```
Diharapkan: `email = NULL` — **berbeda dari create** yang menyimpan string kosong. Menguji BR-03.

### TC-UU09 — Mengosongkan jabatan menyimpan string kosong **[dari kode]**

```
Request : { data: { id_user: 10, display_title: "" } }
```
Diharapkan: `display_title = ""` (bukan `NULL`) — tidak konsisten dengan email & telepon.

### TC-UU10 — `role_codes: []` mengosongkan seluruh role **[dari kode]**

```
Request : { data: { id_user: 10, role_codes: [] } }
```
Diharapkan (perilaku **aktual**): berhasil; user kehilangan semua role dan **tidak bisa login
lagi**. Tidak ada penjagaan. Menguji KI-13. **Belum ada test.**

### TC-UU11 — `branch_ids: []` mengosongkan akses cabang **[dari kode]**

Diharapkan: berhasil; user login tetapi tanpa cabang → jalan buntu (KI-01 modul 01).
**Belum ada test.**

### TC-UU12 — Role superadmin dipulihkan untuk `id_user = 1` **[dari kode]**

```
Request : { data: { id_user: 1, role_codes: [] } }    // sesi superadmin
```
Diharapkan: semua role dihapus, lalu role `superadmin` **ditambahkan kembali otomatis**.
Menguji BR-12. **Belum ada test.**

### TC-UU13 — Edit user superadmin dari sesi non-superadmin **[dari kode]**

Sesi `owner`, target user memegang `superadmin`.
Diharapkan: **404** **"User tidak ditemukan"** (disamarkan, bukan 403).

---

## 4. `users/update-status`

### TC-US01 — Menonaktifkan user ✅ *ada di spec*

```
Request : { data: { id_user: 10, status: "inactive" } }
```
Diharapkan: `status = "inactive"` tersimpan; audit `user.update_status` tercatat.

### TC-US02 — Sesi dicabut saat dinonaktifkan ✅ *ada di spec*

Diharapkan: update ke `UserSession` dengan kriteria `id_user = 10`, `id_company` sesi, dan
`revoked_at IS NULL`; audit memuat `metadata: { revokedSessions: 1 }`.

### TC-US03 — User tidak ada ✅ *ada di spec*

Diharapkan: **404** "User tidak ditemukan".

### TC-US04 — Mengaktifkan kembali **tidak** menyentuh sesi ✅ *ada di spec*

```
Request : { data: { id_user: 10, status: "active" } }
```
Diharapkan: status tersimpan `active`, dan **pencabutan sesi tidak dijalankan**.

### TC-US05 — Menonaktifkan user superadmin dari sesi non-superadmin ✅ *ada di spec*

Sesi `owner`, target memegang `superadmin`.
Diharapkan: **404** "User tidak ditemukan", dan `save` **tidak** dipanggil.

### TC-US06 — Menonaktifkan diri sendiri **[dari kode]**

```
Request : { data: { id_user: <id sesi sendiri>, status: "inactive" } }
```
Diharapkan (perilaku **aktual**): **berhasil**. Semua sesi aktor dicabut, termasuk sesi yang
sedang dipakai → aktor langsung terputus. Tidak ada penjagaan. Menguji KI-19. **Belum ada test.**

### TC-US07 — Respons kosong **[dari kode]**

Diharapkan: HTTP 200 dengan `data: null` (controller `await` tanpa mengembalikan nilai).

---

## 5. `users/change-password`

### TC-CP01 — Password diganti dan sesi target dicabut ✅ *ada di spec*

```
Request : { data: { id_user: 10, new_password: "newsecret123" } }   // aktor id 1
```
Diharapkan:
- `bcrypt.hash("newsecret123", 12)` dipanggil
- Pencabutan sesi dengan kriteria `id_user = 10` + `id_company` + `revoked_at IS NULL`
- Audit `user.change_password` dengan `metadata: { revokedSessions: 2, retainedCurrentSession: false }`
- Respons **tepat**: `{ id: 10, password_updated: true, revoked_sessions: 2 }`

### TC-CP02 — Mengganti password sendiri mempertahankan sesi aktif ✅ *ada di spec*

```
Request : { data: { id_user: 1, new_password: "newsecret123" } }    // aktor id 1
```
Diharapkan:
- Kriteria pencabutan **menambahkan** `id_user_session != <sesi saat ini>`
- Audit memuat `metadata: { revokedSessions: 1, retainedCurrentSession: true }`
- Aktor **tidak** ter-logout

### TC-CP03 — Password baru terlalu pendek ✅ *ada di spec*

Diharapkan: **400** **"Password baru minimal 8 karakter"**, `save` tidak dipanggil.

### TC-CP04 — User tidak ada ✅ *ada di spec*

Diharapkan: **404** "User tidak ditemukan".

### TC-CP05 — Ganti password user superadmin dari sesi non-superadmin ✅ *ada di spec*

Diharapkan: **404** "User tidak ditemukan", `save` tidak dipanggil.

### TC-CP06 — Tanpa verifikasi password lama **[dari kode]**

Diharapkan: permintaan berhasil **tanpa** mengirim password saat ini. Menguji BR-05 — properti
yang perlu diputuskan sadar di sistem baru.

### TC-CP07 — Tanpa validasi konfirmasi di server **[dari kode]**

Diharapkan: server menerima `new_password` tanpa field konfirmasi apa pun. Kecocokan konfirmasi
**hanya** dijaga di klien.

---

## 6. `roles/list`

### TC-RL01 — Mengembalikan role beserta permissionnya ✅ *ada di spec*

Diharapkan: setiap item memuat `id`, `code`, `name`, `description`, `is_system_role`,
`permissions: string[]`.

### TC-RL02 — Role tanpa permission mengembalikan array kosong ✅ *ada di spec*

Diharapkan: `permissions: []`, bukan `null` atau field hilang.

### TC-RL03 — Tidak ada role sama sekali ✅ *ada di spec*

Diharapkan: `{ items: [] }`.

### TC-RL04 — Menyembunyikan role superadmin dari sesi non-superadmin ✅ *ada di spec*

Diharapkan: role berkode `superadmin` tidak ada di `items`.

### TC-RL05 — Urutan hasil **[dari kode]**

Diharapkan: role sistem lebih dulu (`is_system_role` DESC), lalu urut `name` A→Z.

### TC-RL06 — Deskripsi otomatis bila NULL **[dari kode]**

| Kondisi | Deskripsi yang dikembalikan |
|---|---|
| `description = NULL`, role sistem | `Role sistem {name}` |
| `description = NULL`, role kustom | `Role kustom {name}` |
| `description = ""` | **string kosong** (fallback tidak berlaku, karena `??` hanya menangkap null) |

### TC-RL07 — Tanpa paginasi **[dari kode]**

Diharapkan: seluruh role dikembalikan; tidak ada `meta`.

---

## 7. `roles/create`

### TC-RC01 — Kode dibuat otomatis dari nama ✅ *ada di spec*

```
Request : { data: { name: "Kepala Gudang" } }
```
Diharapkan: `code = "kepala_gudang"`; audit `role.create` tercatat.

### TC-RC02 — Kode yang dikirim tetap dinormalisasi ✅ *ada di spec*

```
Request : { data: { code: "Keuangan", name: "Keuangan" } }
```
Diharapkan: role dibuat dengan `code = "keuangan"` (huruf kecil).

### TC-RC03 — Kode sudah dipakai ✅ *ada di spec*

Diharapkan: **409** `Kode role '{kode}' sudah digunakan`.

### TC-RC04 — Nama hanya simbol → kode kosong **[dari kode]**

```
Request : { data: { name: "!!!" } }
```
Diharapkan: **409** **"Kode role tidak valid"**.

### TC-RC05 — Tabel normalisasi kode **[dari kode]**

| Masukan `name` | `code` |
|---|---|
| `Kepala Gudang` | `kepala_gudang` |
| `Kepala   Gudang` | `kepala_gudang` |
| `Kepala-Gudang` | `kepala_gudang` |
| `Kepala Gudang!` | `kepala_gudang` |
| `  Staf Toko  ` | `staf_toko` |
| `Sales & Marketing` | `sales_marketing` |
| `Gudang 2` | `gudang_2` |
| `Área Manager` | `rea_manager` |
| `管理者` | *(kosong → ditolak)* |

### TC-RC06 — Bentrok dengan kode role sistem **[dari kode]**

```
Request : { data: { name: "Admin" } }
```
Diharapkan: **409** — pemeriksaan mencakup role global, jadi `admin` sudah dipakai role sistem.

### TC-RC07 — Role baru selalu kustom **[dari kode]**

Diharapkan: `is_system_role = false`, `id_company` = perusahaan sesi, `permissions: []`.

### TC-RC08 — Deskripsi pada respons memakai input mentah **[dari kode]**

```
Request : { data: { name: "Kasir Toko", description: "  catatan  " } }
```
Diharapkan: yang **tersimpan** `"catatan"` (ter-trim), tetapi yang **dikembalikan** respons adalah
`"  catatan  "` (input mentah). Tidak konsisten dengan `roles/update` yang mengembalikan nilai
tersimpan.

---

## 8. `roles/update`

### TC-RU01 — Mengubah nama role ✅ *ada di spec*

Diharapkan: nama tersimpan; audit `role.update` dengan `before: { name }`.

### TC-RU02 — Role tidak ada ✅ *ada di spec*

Diharapkan: **404** **"Role kustom tidak ditemukan"**.

### TC-RU03 — Role sistem tidak bisa diubah **[dari kode]**

```
Request : { data: { code: "admin", name: "Administrator" } }
```
Diharapkan: **404** "Role kustom tidak ditemukan" — pencarian mensyaratkan `id_company` terisi,
sehingga role global tidak pernah ditemukan.

### TC-RU04 — Kode tidak pernah berubah **[dari kode]**

Diharapkan: mengganti nama tidak mengubah `code`. Setelah "Kepala Gudang" → "Manajer Gudang",
`code` tetap `kepala_gudang`.

### TC-RU05 — Frontend memblokir role sistem sebelum request **[dari kode]**

Aksi: menekan "Simpan Role" pada role sistem.
Diharapkan: **tidak ada permintaan API**; toast warning **"Role sistem tidak dapat diubah"** /
"Gunakan role kustom untuk kebutuhan tambahan."

---

## 9. `roles/delete`

### TC-RD01 — Menghapus role yang tidak dipakai ✅ *ada di spec*

Diharapkan: `role_permissions` dihapus, lalu baris role dihapus (**hard delete**); audit
`role.delete` dengan `before: { code, name }`.

### TC-RD02 — Role tidak ada ✅ *ada di spec*

Diharapkan: **404** "Role kustom tidak ditemukan".

### TC-RD03 — Role masih dipakai user ✅ *ada di spec*

Diharapkan: **409** **"Role masih dipakai oleh user dan tidak dapat dihapus"**; tidak ada
penghapusan.

### TC-RD04 — Role sistem tidak bisa dihapus **[dari kode]**

```
Request : { data: { code: "staff" } }
```
Diharapkan: **404** "Role kustom tidak ditemukan".

### TC-RD05 — Respons kosong **[dari kode]**

Diharapkan: HTTP 200 dengan `data: null`.

### TC-RD06 — Tanpa transaksi **[dari kode]**

Diharapkan: dua penghapusan **berurutan tanpa transaksi**. Bila penghapusan kedua gagal, role
tetap ada tetapi **tanpa permission**. Menguji KI-27. **Belum ada test.**

---

## 10. `roles/permissions/update`

### TC-RP01 — Menetapkan permission ke role ✅ *ada di spec*

Diharapkan: permission lama dihapus, yang baru disisipkan, respons memuat daftar kode.

### TC-RP02 — Array kosong mengosongkan semua permission ✅ *ada di spec*

```
Request : { data: { code: "kasir", permissions: [] } }
```
Diharapkan: `delete` dipanggil, `save` **tidak** dipanggil, `result.permissions = []`.

### TC-RP03 — Kode tidak dikenal ditolak TANPA menghapus yang lama ✅ *ada di spec*

```
Request : { data: { code: "kasir", permissions: ["order.read", "missing.permission"] } }
```
Diharapkan:
- **400** `Permission tidak dikenal: missing.permission`
- `rolePermissionRepo.delete` **tidak pernah dipanggil**
- `rolePermissionRepo.save` **tidak pernah dipanggil**

Ini properti penting: validasi mendahului penghapusan, sehingga state lama aman.

### TC-RP04 — Deduplikasi kode ✅ *ada di spec*

```
Request : { data: { code: "kasir", permissions: ["order.read", "order.read"] } }
```
Diharapkan: pencarian permission dilakukan sekali untuk kode itu; `result.permissions =
["order.read"]`.

### TC-RP05 — Role tidak ada ✅ *ada di spec*

Diharapkan: **404** **"Role tidak ditemukan"** (pesan berbeda dari update/delete yang menyebut
"Role kustom").

### TC-RP06 — Role superadmin ditolak untuk sesi non-superadmin ✅ *ada di spec*

Sesi `owner`, `code: "superadmin"`.
Diharapkan: **404** "Role tidak ditemukan", dan `delete` **tidak** dipanggil (ditolak sebelum
query role).

### TC-RP07 — Permission role SISTEM boleh diubah **[dari kode]**

```
Request : { data: { code: "admin", permissions: ["dashboard.view"] } }   // sesi owner
```
Diharapkan (perilaku **aktual**): **berhasil**. Berbeda dari `roles/update` yang menolak role
sistem. Menguji BR-09 aturan 2 dan KI-15.

### TC-RP08 — `whatsapp.simulate` dapat diberikan lewat API **[dari kode]**

```
Request : { data: { code: "kasir", permissions: ["whatsapp.simulate"] } }
```
Diharapkan (perilaku **aktual**): **berhasil** — permission itu ada di tabel `permissions`,
sehingga validasi lolos. Penyembunyiannya di matriks UI hanya kosmetik. Menguji KI-16.
**Belum ada test.**

### TC-RP09 — Tanpa transaksi **[dari kode]**

Diharapkan: hapus-semua lalu sisipkan, **tanpa transaksi**. Bila penyisipan gagal, role kehilangan
seluruh permission. Menguji KI-27. **Belum ada test.**

---

## 11. Frontend — Form Pengguna

### TC-FU01 — Field ubah password tampil di mode edit ✅ *ada di test*

Diharapkan terlihat: heading **"Edit Pengguna"**, label **"Password baru"**, label
**"Konfirmasi password baru"**, teks **"Isi hanya jika password pengguna perlu diganti."**

### TC-FU02 — Password baru dikirim saat field diisi ✅ *ada di test*

Aksi: mengisi kedua field dengan `newsecret123`, menekan **"Simpan Perubahan"**.
Diharapkan: `saveUserMembership` dipanggil dengan `{ id: "10", userId: "10",
password: "newsecret123" }`.

### TC-FU03 — Password tidak dikirim saat field dikosongkan ✅ *ada di test*

Aksi: langsung menekan "Simpan Perubahan".
Diharapkan: `saveUserMembership` dipanggil dengan `password: undefined`.

### TC-FU04 — Role Super Admin tidak muncul sebagai pilihan ✅ *ada di test*

Data: `roleDefinitions` memuat `superadmin`, `admin`, `staff`.
Diharapkan: label "Super Admin" **tidak ada di DOM**; "Admin" dan "Staff" terlihat.

### TC-FU05 — Dua permintaan berurutan saat password diisi **[dari kode]**

Diharapkan urutan: `users/update` lebih dulu, lalu `users/change-password`. **Bukan** satu
permintaan gabungan. Menguji KI-16.

### TC-FU06 — Pesan gagal generik **[dari kode]**

Data: server menolak dengan `409` "Username sudah digunakan".
Diharapkan: toast danger berjudul **"Gagal menyimpan pengguna"** **tanpa deskripsi** — pesan
server hilang. Menguji KI-17.

### TC-FU07 — Edit user di luar 100 data pertama **[dari kode]**

Data: user dengan id yang tidak ada di `userRecords` store.
Diharapkan (perilaku **aktual**): form terbuka dalam **mode tambah** — judul "Tambah Pengguna",
field kosong, field password wajib — tanpa peringatan apa pun. Menguji KI-14.
**Belum ada test.**

### TC-FU08 — Centang bawaan mode tambah **[dari kode]**

Diharapkan: role **`staff`** tercentang; cabang yang bertanda default perusahaan tercentang.

---

## 12. Frontend — Halaman Role & Akses

### TC-FR01 — Notice sebelum role dipilih ✅ *ada di test*

Diharapkan terlihat: **"Pilih salah satu role untuk mulai mengatur permission."**

### TC-FR02 — Permission existing tampil tercentang ✅ *ada di test*

Data: role `admin` dengan `["dashboard.view", "role.manage"]`.
Aksi: menekan **"Atur"**.
Diharapkan:
- `getByLabelText("Dashboard: Lihat")` → **tercentang**
- `getByLabelText("Role & Akses: Kelola")` → **tercentang**
- `getByLabelText("Produk & Item: Tambah")` → **tidak** tercentang

Format `aria-label` `{Modul}: {Aksi}` adalah kontrak — jangan diubah.

### TC-FR03 — Menambah permission tanpa menghapus yang lain ✅ *ada di test*

Aksi: klik "Produk & Item: Tambah".
Diharapkan: `saveRolePermissions("admin", [...])` memuat ketiganya —
`dashboard.view`, `role.manage`, `product.create`.

### TC-FR04 — Menghapus centang mengeluarkan permission dari payload ✅ *ada di test*

Aksi: klik "Role & Akses: Kelola" (yang sedang tercentang).
Diharapkan: `saveRolePermissions("admin", ["dashboard.view"])` — **tepat** satu elemen.

### TC-FR05 — Pembaruan optimistis + deduplikasi ✅ *ada di test*

Aksi: `saveRolePermissions("admin", ["dashboard.view", "product.create", "product.create"])`.
Diharapkan:
- State lokal langsung menjadi `["dashboard.view", "product.create"]` (sebelum server menjawab)
- Permintaan API mengirim `{ code: "admin", permissions: ["dashboard.view", "product.create"] }`

### TC-FR06 — Role sistem tetap bisa diatur permissionnya ✅ *tersirat di test*

Data test memakai role `admin` dengan `isSystem: true`, dan `saveRolePermissions` **terpanggil**.
Diharapkan: tidak ada pemblokiran di klien untuk permission role sistem (berbeda dari nama role).

### TC-FR07 — Pilihan role dibatalkan setelah menyimpan role **[dari kode]**

Aksi: menekan "Atur" → mengubah nama → menekan "Simpan Role".
Diharapkan: pilihan role dikosongkan → **matriks permission menutup** kembali ke notice.
Menguji UF-12 langkah 8.

### TC-FR08 — Hapus role tanpa konfirmasi **[dari kode]**

Aksi: menekan "Hapus" pada role kustom.
Diharapkan: `deleteRoleDefinition` langsung terpanggil — **tidak ada dialog konfirmasi**.
Menguji KI-21.

### TC-FR09 — Pesan gagal memakai kode mesin **[dari kode]**

Data: server menolak `roles/create` dengan `409` `Kode role 'kasir' sudah digunakan`.
Diharapkan: toast danger **"Gagal menyimpan role"** dengan deskripsi **`error`** (kode mesin),
bukan pesan Indonesia. Menguji KI-17.

---

## 13. Konfigurasi Matriks Permission

### TC-CFG01 — Semua permission bawaan dapat diatur dari UI ✅ *ada di test*

Diharapkan: setiap permission di `DEFAULT_ROLE_PERMISSIONS` ada di matriks, **kecuali** dua yang
sengaja dikecualikan: `whatsapp.simulate` dan `reporting.view`.

Catatan hasil verifikasi: `reporting.view` sebenarnya **ADA** di matriks (kelompok "Laporan
Operasional", label "Lihat Laporan"). Ia hanya dikeluarkan dari daftar pembanding di test.
Sehingga satu-satunya permission yang benar-benar tidak ada di UI adalah **`whatsapp.simulate`** —
komentar di test sudah tidak akurat sebagian.

### TC-CFG02 — Tidak ada permission ganda ✅ *ada di test*

Diharapkan: jumlah kode unik = jumlah total kode (59).

### TC-CFG03 — Seluruh permission Keuangan punya label bisnis ✅ *ada di test*

Diharapkan: 14 permission finance ada di matriks dengan **label persis**:
"Lihat Setup", "Kelola Setup, Saldo Awal & Periode",
"Tutup Buku Paksa (Force, Lewati Checklist)", "Lihat Rekap",
"Posting, Abaikan, Pulihkan & Batalkan", "Lihat Jurnal", "Balikkan Jurnal",
"Kas, Hutang/Piutang, Margin & Stok", "Pajak", "Lihat Biaya", "Catat Biaya",
"Batalkan Biaya", "Lihat Penyesuaian Pajak", "Kelola Penyesuaian Pajak".

### TC-CFG04 — Jumlah total permission di matriks **[dari kode]**

Diharapkan: **22 kelompok modul**, **59 permission**, dari **60** yang ada di sistem.

---

## 14. End-to-End (UI nyata)

Kedua kasus **sudah ada** di `apps/e2e/tests/business/18-role-access-matrix.spec.ts`, dijalankan
**serial**.

### TC-E01 — UI menyimpan toggle permission tanpa menghapus yang lain ✅

```
1. Buat role via API: name = "{runId} Role Matrix UI"
2. Buka /settings/roles → pastikan heading "Matriks Hak Akses" terlihat
3. Cari baris tabel yang memuat nama role → klik "Atur"
4. Pastikan "Dashboard: Lihat" belum tercentang → centang
5. Poll roles/list sampai permissions memuat 'dashboard.view'
6. Centang "Produk & Item: Lihat"
7. Poll sampai permissions memuat KEDUANYA
8. Hapus centang "Dashboard: Lihat"
9. Poll sampai permissions TIDAK memuat 'dashboard.view' TAPI masih memuat 'product.view'
```
Menguji bahwa setiap toggle mengirim daftar lengkap, bukan hanya yang berubah.

### TC-E02 — API menolak permission invalid & menyembunyikan Super Admin ✅

```
1. Buat role via API
2. Set permissions = ['dashboard.view']
3. Kirim permissions = ['dashboard.view', 'unknown.permission']
   → status 400, pesan cocok /Permission tidak dikenal|unknown\.permission/i
4. Baca ulang role → permissions HARUS tetap ['dashboard.view']
5. Login sebagai owner/owner123:
   → role 'owner' memuat 'role.manage'
   → roles/list TIDAK memuat role 'superadmin'
6. Login sebagai admin/admin123:
   → role 'admin' memuat 'role.manage'
   → roles/list TIDAK memuat role 'superadmin'
7. Sebagai owner, kirim roles/permissions/update untuk code='superadmin'
   → status 404
```
Langkah 5–6 mengonfirmasi bahwa **owner dan admin keduanya memegang `role.manage`** di data nyata.

### TC-E03 — Kasir ditolak mengakses laporan finance ✅ *ada di 10-observability*

```
1. Login API sebagai kasir / "kasir 123"
2. POST finance/reports/profit-loss
   → status 403
3. POST dashboard/summary tanpa token
   → status 401
```

---

## 15. Kasus yang Belum Punya Test dan Layak Ditambahkan

Diturunkan dari edge case yang terbukti ada di kode namun tidak tercakup test mana pun. Daftar ini
menandai **risiko regresi saat rebuild**, bukan usulan fitur.

| # | Kasus | Kenapa penting |
|---|---|---|
| GAP-01 | `role_codes: []` mengunci user dari sistem | Akibat paling serius di modul ini (KI-13); tidak ada satu pun test |
| GAP-02 | Kode role tak dikenal dilewati diam-diam | Jalur lain ke kondisi yang sama (KI-13) |
| GAP-03 | `branch_ids: []` membuat user terjebak tanpa cabang | Bergandengan dengan KI-01 modul 01 |
| GAP-04 | Role `superadmin` dipulihkan untuk `id_user = 1` | Pengaman penting yang tak terverifikasi |
| GAP-05 | Permission role sistem bisa diubah pemegang `role.manage` | Properti eskalasi hak (KI-15) |
| GAP-06 | `whatsapp.simulate` bisa diberikan lewat API | Penjagaan UI-saja (KI-16) |
| GAP-07 | Menonaktifkan diri sendiri | Admin bisa mengunci dirinya (KI-19) |
| GAP-08 | Email string kosong pada user kedua → 500 | Kegagalan tak berpesan (KI-22) |
| GAP-09 | `roles/delete` & `permissions/update` tanpa transaksi | Bisa meninggalkan role tanpa permission (KI-27) |
| GAP-10 | Edit user di luar 100 data pertama | Form salah mode tanpa peringatan (KI-14) |
| GAP-11 | Isi pesan error yang benar-benar tampil di toast | Sama seperti GAP-08 modul 01 — inilah sebabnya KI-17 tak pernah ketahuan |
| GAP-12 | `limit: 0` menghasilkan daftar kosong | Ketiadaan clamp bawah (KI-26) |
| GAP-13 | Pencarian dua kata tidak menemukan hasil | Perbedaan perilaku dari modul lain (KI-25) |
| GAP-14 | Perubahan role & cabang tidak tercatat di audit | Kelemahan pengawasan (BR-22) |
