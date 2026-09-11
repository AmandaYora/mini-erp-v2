# Test Cases — Modul 01 Auth & Session

**Kelompok A — skenario input→output nyata, diturunkan dari kode dan test yang sudah ada.**
Semua ekspektasi di bawah dapat diverifikasi terhadap sistem lama sebelum sistem baru dianggap
setara.

Sumber: `auth.service.spec.ts` (17 kasus), `jwt.strategy.spec.ts` (4 kasus),
`permission.guard.spec.ts` (5 kasus), `auth-redirect.test.ts` (4 kasus),
`use-auth-module.test.ts` (5 kasus), `00-auth-shell-access.spec.ts` (4 kasus E2E), plus kasus
turunan dari pembacaan kode (ditandai **[dari kode]**).

Kredensial yang dipakai: lihat [feature-inventory.md](feature-inventory.md) §5.

---

## 1. Login — Backend (`auth/login`)

Format: request → respons yang diharapkan.

### TC-L01 — Login berhasil, satu cabang aktif ✅ *ada di spec*

```
Request : { data: { username: "superadmin", password: "superadmin123" } }
```
Diharapkan:
- HTTP 200, `code: 0`, `info: "success"`
- `data.access_token` dan `data.refresh_token` terisi
- `data.requires_branch_selection` = `false`
- `data.user.active_branch` **tidak null**, `id_branch` = 1
- `data.user.email` = `"superadmin@mini-erp.local"`
- `data.user.company.name` terisi
- `data.user.active_role.code` = `"superadmin"`
- Satu baris baru di `user_sessions`; `users.last_login_at` ter-update

### TC-L02 — User tidak ditemukan ✅ *ada di spec*

```
Request : { data: { username: "nobody", password: "pw" } }
```
Diharapkan: **401**, `info: "unauthorized"`, `errors: "Kredensial tidak valid"`.
Tidak ada baris sesi dibuat.

### TC-L03 — Status user `inactive` ✅ *ada di spec*

Diharapkan: **401** dengan pesan **persis sama** seperti TC-L02 — tidak boleh membocorkan bahwa
username itu terdaftar.

### TC-L04 — Password salah ✅ *ada di spec*

```
Request : { data: { username: "superadmin", password: "wrongpw" } }
```
Diharapkan: **401**, pesan sama seperti TC-L02.

### TC-L05 — User tanpa role ✅ *ada di spec*

Diharapkan: **403**, `info: "forbidden"`, `errors: "User tidak memiliki role"`.
**Sesi tidak dibuat** meski password benar.

### TC-L06 — Auto-pilih cabang saat tepat 1 cabang aktif ✅ *ada di spec*

Diharapkan: `requires_branch_selection` = `false`, `active_branch.id_branch` = id cabang itu.

### TC-L07 — Banyak cabang, tanpa default ✅ *ada di spec*

Data: 2 cabang aktif, keduanya `is_default_branch = 0`.
Diharapkan: `requires_branch_selection` = **`true`**, `active_branch` = **`null`**.
Token **tetap** diterbitkan dan sesi **tetap** dibuat.

### TC-L08 — Banyak cabang, satu default ✅ *ada di spec*

Data: cabang id 2 (bukan default) dan id 3 (default).
Diharapkan: `requires_branch_selection` = `false`, `active_branch.id_branch` = **3**.

### TC-L09 — Login dengan email sebagai identifier **[dari kode]**

```
Request : { data: { username: "superadmin@mini-erp.local", password: "superadmin123" } }
```
Diharapkan: **berhasil sama seperti TC-L01** — backend mencari `username` OR `email`.
Catatan: frontend mengirim nilai apa pun di field `username`, jadi ini jalur yang benar-benar
dipakai user yang mengetik email.

### TC-L10 — `username` dan `email` dikirim bersamaan **[dari kode]**

```
Request : { data: { username: "admin", email: "owner@mini-erp.local", password: "admin123" } }
```
Diharapkan: **`username` menang** → login sebagai `admin`, bukan `owner`.

### TC-L11 — Password tidak dikirim **[dari kode]**

```
Request : { data: { username: "superadmin" } }
```
Diharapkan: **400**, `code: 200`, `info: "validation_failed"`, `errors` berisi pesan validator
untuk `password`. **Bukan** "Kredensial tidak valid" — ditolak sebelum menyentuh DB.

### TC-L12 — Password string kosong **[dari kode]**

```
Request : { data: { username: "superadmin", password: "" } }
```
Diharapkan: **400 validation_failed** (gagal `@MinLength(1)`).

### TC-L13 — Identifier kosong, password ada **[dari kode]**

```
Request : { data: { password: "superadmin123" } }
```
Diharapkan: **401** "Kredensial tidak valid" (identifier menjadi `''`, tidak ada user cocok).

### TC-L14 — Field asing di payload **[dari kode]**

```
Request : { data: { username: "superadmin", password: "superadmin123", role_code: "owner" } }
```
Diharapkan: `role_code` **dibuang tanpa peringatan**, login berhasil sebagai `superadmin`.
Menguji bahwa payload tidak bisa dipakai memaksa role.

### TC-L15 — Password `kasir` mengandung spasi **[dari kode + E2E]**

```
Request : { data: { username: "kasir", password: "kasir 123" } }
```
Diharapkan: **berhasil**. Dengan `"kasir123"` (tanpa spasi) → **401**.
Penting saat migrasi: password bawaan ini memang berisi spasi.

### TC-L16 — Login berulang tanpa logout **[dari kode]**

Login 3× berturut-turut dengan akun sama.
Diharapkan: **3 baris `user_sessions` aktif**, ketiga refresh token tetap berfungsi. Tidak ada
sesi yang dicabut otomatis.

---

## 2. Refresh Token (`auth/refresh`)

### TC-R01 — Refresh berhasil, token dirotasi ✅ *ada di spec*

Diharapkan:
- `access_token` dan `refresh_token` baru terisi
- Pencarian sesi memakai **`{ id, idUser }` keduanya**
- `user_sessions.expires_at` **diperpanjang**
- `refresh_token_hash` ditimpa

### TC-R02 — Sesi tidak ditemukan ✅ *ada di spec*

Diharapkan: **401** "Refresh token tidak valid".

### TC-R03 — Sesi sudah dicabut ✅ *ada di spec*

Data: `revoked_at` terisi. Diharapkan: **401**.

### TC-R04 — Sesi kedaluwarsa ✅ *ada di spec*

Data: `expires_at` di masa lalu. Diharapkan: **401**.

### TC-R05 — JWT tidak sah ditolak sebelum menyentuh DB ✅ *ada di spec*

Diharapkan: **401**, dan **`sessionRepo.findOne` tidak pernah dipanggil**. Ini menguji urutan
pemeriksaan — verifikasi tanda tangan lebih dulu, baru query.

### TC-R06 — User tidak lagi aktif → sesi dicabut ✅ *ada di spec*

Data: `users.status = 'locked'`, token sah.
Diharapkan: **401** **dan** `user_sessions.revoked_at` **disetel**. Satu-satunya jalur pencabutan
otomatis di sistem.

### TC-R07 — Access token dipakai sebagai refresh token **[dari kode]**

```
Request : { data: { refresh_token: <access_token yang sah> } }
```
Diharapkan: **401** — ditolak karena klaim `type !== 'refresh'`, dan tabel sesi tidak disentuh.

### TC-R08 — Replay refresh token (dipakai dua kali) **[dari kode]**

1. Refresh dengan token A → berhasil, menerbitkan token B
2. Refresh **lagi** dengan token A

Diharapkan langkah 2: **401** — hash sudah dirotasi ke token B, `bcrypt.compare` gagal.
**Ini properti keamanan penting yang harus dipertahankan.**

### TC-R09 — Sliding expiry **[dari kode]**

1. Catat `expires_at` sesi
2. Tunggu, lalu refresh
3. Baca ulang `expires_at`

Diharapkan: nilai baru = `waktu refresh + JWT_REFRESH_EXPIRES`. **Tidak ada batas umur absolut**
— sesi bisa hidup selamanya bila dipakai rutin.

### TC-R10 — Token milik user lain dengan `sessionId` valid **[dari kode]**

Susun refresh token dengan `sessionId` sesi user A tapi `sub` = id user B.
Diharapkan: **401** — pencarian mensyaratkan `id` **dan** `id_user` cocok bersamaan.

---

## 3. Logout (`auth/logout`)

### TC-O01 — Logout menyetel `revoked_at` ✅ *ada di spec*

Diharapkan: `user_sessions.update(sessionId, { revokedAt: <Date> })` terpanggil.
Respons: HTTP 200, `data: null`.

### TC-O02 — Logout hanya mencabut sesi sendiri **[dari kode]**

1. Login di perangkat A dan perangkat B (akun sama)
2. Logout dari perangkat A
3. Panggil `auth/me` dengan token perangkat B

Diharapkan langkah 3: **tetap berhasil**. Tidak ada "logout semua perangkat".

### TC-O03 — Token setelah logout ditolak **[dari kode]**

Setelah logout, panggil endpoint apa pun dengan access token lama.
Diharapkan: **401** (sesi `revoked_at` terisi → gagal verifikasi per request).

### TC-O04 — Logout tanpa token **[dari kode]**

Diharapkan: **401** — endpoint dilindungi `JwtAuthGuard`.

---

## 4. `auth/me`

### TC-M01 — Mengembalikan identitas, role aktif, dan cabang ✅ *ada di spec*

Diharapkan: `id` sesuai user; `active_role.code` sesuai role sesi; `branches` berisi 1 entri.

### TC-M02 — `me` tidak mengembalikan data perusahaan **[dari kode]**

Diharapkan: respons **tidak memuat** objek `company` (berbeda dari `auth/login`). Ini yang
memaksa frontend menyimpan data perusahaan di `localStorage` dan menggabungkannya saat memulihkan
sesi.

### TC-M03 — `me` mengembalikan cabang non-aktif **[dari kode]**

Data: user punya akses ke cabang aktif **dan** cabang berstatus `inactive`.
Diharapkan: `branches` memuat **keduanya** — `auth/me` tidak memfilter status, berbeda dari
`auth/login` yang hanya mengembalikan cabang aktif. Ketidakkonsistenan ini nyata; lihat
[known-issues.md](../known-issues.md) KI-10.

### TC-M04 — `me` tidak mengembalikan `username` **[dari kode]**

Diharapkan: respons memuat `id`, `full_name`, `email` tapi **tidak** `username`. Akibatnya
`activeUser.username` di frontend selalu string kosong setelah reload.

### TC-M05 — Role user dicabut, sesi masih hidup **[dari kode]**

Data: sesi ber-`id_active_role = X`, tapi baris `user_roles` untuk X sudah dihapus dan user punya
role Y.
Diharapkan: `active_role` melaporkan **Y** (fallback ke role pertama), sementara `permissions`
tetap berasal dari **X** (di-resolve oleh verifikasi per request). Laporan role dan permission
efektif **tidak sinkron**; lihat KI-05.

---

## 5. Ganti Role (`auth/switch-role`)

### TC-SR01 — Berhasil untuk role yang dimiliki ✅ *ada di spec*

```
Request : { data: { role_code: "superadmin" } }
```
Diharapkan: `active_role.code` = `"superadmin"`; `user_sessions.id_active_role` di-update;
daftar `permissions` role itu dikembalikan.

### TC-SR02 — Role tidak dimiliki user ✅ *ada di spec*

Data: user tanpa role sama sekali → `role_code: "admin"`.
Diharapkan: **403** "Role tidak tersedia untuk user ini".

### TC-SR03 — Kode role berbeda dari yang dimiliki ✅ *ada di spec*

Data: user hanya punya role `cashier` → minta `superadmin`.
Diharapkan: **403**.

### TC-SR04 — Ganti role memperbarui permission efektif **[dari kode]**

1. Login sebagai user dengan role `owner` dan `staff`
2. `switch-role` ke `staff`
3. Panggil endpoint yang butuh `finance.manage`

Diharapkan langkah 3: **403** — permission di-resolve ulang dari role sesi yang baru pada request
berikutnya.

---

## 6. Ganti Cabang (`auth/switch-branch`)

### TC-SB01 — Berhasil bila punya akses ✅ *ada di spec*

```
Request : { data: { id_branch: 2 } }
```
Diharapkan: `active_branch.id_branch` = 2; `user_sessions.id_active_branch` di-update.

### TC-SB02 — Tidak punya akses cabang ✅ *ada di spec*

Diharapkan: **403** "Tidak memiliki akses ke cabang ini".

### TC-SB03 — Cabang berstatus `inactive` **[dari kode]**

Data: user punya baris akses ke cabang, tapi `branches.status = 'inactive'`.
Diharapkan (perilaku **aktual** sistem lama): **500** `internal_error` tanpa field `errors`.
Bukan 403 dan bukan pesan bisnis. Lihat KI-02 — **[PERLU KONFIRMASI]** apakah sistem baru harus
mempertahankan 500 ini atau boleh diperbaiki menjadi 403 berpesan jelas.

### TC-SB04 — `id_branch` bukan angka positif **[dari kode]**

```
Request : { data: { id_branch: 0 } }      → 400 validation_failed (@IsPositive)
Request : { data: { id_branch: -1 } }     → 400 validation_failed
Request : { data: { id_branch: "abc" } }  → 400 validation_failed (@IsNumber)
```

### TC-SB05 — Frontend menolak ID tidak valid sebelum request **[dari kode]**

Pemanggilan `selectBranch("abc")` di frontend.
Diharapkan: **tidak ada request terkirim**; toast danger berjudul "Gagal mengganti cabang" dengan
deskripsi `ID cabang tidak valid: "abc"`.

---

## 7. Verifikasi Sesi per Request (JwtStrategy)

### TC-J01 — Akses cabang dicabut → cabang di-drop ✅ *ada di spec*

Data: sesi ber-`idActiveBranch = 12`, tapi tidak ada baris `user_branch_accesses` untuk
`(user 7, branch 12)`.
Diharapkan payload sesi:
- `idActiveBranch` = **`null`** (sesi **tetap sah**, tidak dilempar 401)
- `idUser` = 7, `idCompany` = 1, `idSession` = 99
- `activeRoleCode` = `"owner"`
- `permissions` = `['order.view', 'finance.view']`
- Payload di-inject ke `request.session_payload`
- Pencarian akses memakai `{ where: { idUser: 7, idBranch: 12 } }`

### TC-J02 — Cabang dipertahankan bila akses masih ada ✅ *ada di spec*

Diharapkan: `idActiveBranch` = 12.

### TC-J03 — Sesi dicabut ✅ *ada di spec*

Data: `revokedAt` terisi. Diharapkan: **401**.

### TC-J04 — User tidak lagi aktif ✅ *ada di spec*

Data: `user.status = 'locked'`. Diharapkan: **401**.

### TC-J05 — Sesi kedaluwarsa **[dari kode]**

Data: `expiresAt` di masa lalu. Diharapkan: **401**.

### TC-J06 — Permission diambil dari DB, bukan dari token **[dari kode]**

1. Login → catat access token
2. Ubah `role_permissions` role itu (tambah/hapus permission)
3. Panggil endpoint dengan token **yang sama** dari langkah 1

Diharapkan: permission baru **langsung berlaku** tanpa refresh token atau login ulang.

### TC-J07 — Baris `roles` hilang **[dari kode]**

Data: `roleRepo.findOne` mengembalikan `null`.
Diharapkan: sesi **tetap sah**, `activeRoleCode` = **string kosong** (`''`).

---

## 8. Permission Guard

### TC-P01 — Permission di handler cocok ✅ *ada di spec*

Metadata handler `finance.view`, sesi punya `['finance.view']` → **diizinkan** (`true`).

### TC-P02 — Fallback ke metadata class ✅ *ada di spec*

Metadata hanya di class (`role.manage`), sesi punya `['role.manage']` → **diizinkan**.
Diharapkan: `reflector.get` dipanggil untuk **handler dulu**, lalu class.

### TC-P03 — Tanpa metadata permission → diizinkan ✅ *ada di spec*

Diharapkan: `true`. **Gagal-terbuka** — endpoint tanpa `@RequirePermission` dapat diakses siapa
pun yang punya token.

### TC-P04 — Payload sesi kosong ✅ *ada di spec*

Metadata `finance.view`, tanpa `session_payload` → **`ForbiddenException`**.

### TC-P05 — Permission kurang ✅ *ada di spec*

Metadata `finance.manage`, sesi punya `['finance.view']` → **`ForbiddenException`**.

---

## 9. Sanitasi Redirect (`resolvePostAuthRedirectPath`)

### TC-RD01 — Nilai kosong / root / halaman auth ✅ *ada di test*

| Input | Output |
|---|---|
| *(tidak ada)* | `/dashboard` |
| `"/"` | `/dashboard` |
| `"/login"` | `/dashboard` |
| `"/select-branch"` | `/dashboard` |

### TC-RD02 — Path aplikasi internal dipertahankan ✅ *ada di test*

`"/orders/123"` → `/orders/123`

### TC-RD03 — Objek lokasi React Router ✅ *ada di test*

`{ pathname: "/orders/123", search: "?tab=open", hash: "#items" }` →
`/orders/123?tab=open#items`

### TC-RD04 — Path eksternal / protocol-relative ditolak ✅ *ada di test*

| Input | Output |
|---|---|
| `"https://example.com"` | `/dashboard` |
| `"//example.com"` | `/dashboard` |
| `{ pathname: "//example.com" }` | `/dashboard` |

### TC-RD05 — `search`/`hash` tanpa awalan yang benar diabaikan **[dari kode]**

`{ pathname: "/orders/1", search: "tab=open", hash: "items" }` → `/orders/1`
(`search` harus diawali `?`, `hash` harus diawali `#`).

---

## 10. End-to-End (UI nyata)

Keempat kasus ini **sudah ada** di `apps/e2e/tests/business/00-auth-shell-access.spec.ts` dan
berjalan tanpa sesi tersimpan (`storageState` kosong).

### TC-E01 — Login gagal menampilkan error tanpa membuat sesi ✅

```
1. Buka /login
2. Isi #identifier = "superadmin", #password = "password-salah"
3. Klik tombol bernama "Sign In"
```
Diharapkan:
- Teks **"Authentication Error"** terlihat
- URL tetap cocok `/login`
- `localStorage.getItem('mini-erp-access')` = **`null`**

### TC-E02 — Login, reload halaman terlindungi, lalu logout ✅

```
1. Login sebagai superadmin / superadmin123
2. Tunggu URL /dashboard
3. Reload halaman
4. Buka .avatar-trigger → klik tombol "Logout"
```
Diharapkan:
- Setelah login: `.sidebar-panel` terlihat, `mini-erp-access` **tidak null**
- Setelah reload: `.sidebar-panel` **tetap** terlihat, **tanpa runtime error**
- Setelah logout: URL `/login`, `mini-erp-access` = `null`, `mini-erp-session` = `null`

### TC-E03 — Logout tersedia dari sidebar mobile ✅

```
1. Set viewport 390×844
2. Login sebagai superadmin
3. Klik tombol "Buka menu"
4. Klik "Logout" di dalam .app-sidebar-open
```
Diharapkan: `.sidebar-mobile-logout` terlihat; setelah logout URL `/login` dan
`mini-erp-access` = `null`.

### TC-E04 — Role `kasir` fokus transaksi, tidak bisa buka finance ✅

```
1. Login sebagai kasir / "kasir 123"
2. Periksa sidebar
3. Buka /finance/back-office lewat URL langsung
```
Diharapkan:
- Tautan **"POS"** terlihat
- Tautan **"Tutup Buku"** **tidak ada** (`toHaveCount(0)`)
- Heading **"Akses Ditolak"** terlihat
- **Tidak ada** teks error runtime (`Cannot read properties`, `is not a function`,
  `Unexpected Application Error`)

---

## 11. Kasus yang Belum Punya Test dan Layak Ditambahkan

Diturunkan dari edge case yang terbukti ada di kode namun tidak tercakup test mana pun. Daftar ini
**bukan** usulan fitur baru — ia menandai risiko regresi saat rebuild.

| # | Kasus | Kenapa penting |
|---|---|---|
| GAP-01 | Login user **tanpa cabang aktif** | Menghasilkan jalan buntu di UI (KI-01) — tidak ada satu pun test yang menyentuhnya |
| GAP-02 | Ganti cabang ke cabang non-aktif | Menghasilkan 500, bukan pesan bisnis (KI-02) |
| GAP-03 | Replay refresh token | Properti keamanan inti, belum ada test |
| GAP-04 | Access token dipakai sebagai refresh token | Properti keamanan, belum ada test |
| GAP-05 | Token dengan `sessionId` dan `sub` tidak cocok | Properti keamanan, belum ada test |
| GAP-06 | Sliding expiry tidak punya batas absolut | Perilaku yang perlu diputuskan sadar (BR-15) |
| GAP-07 | `auth/me` mengembalikan cabang non-aktif | Ketidakkonsistenan dengan login (KI-10) |
| GAP-08 | Isi pesan error login yang benar-benar tampil | E2E hanya memeriksa judul "Authentication Error", tidak isinya — inilah sebabnya KI-04 tidak pernah ketahuan |
| GAP-09 | Ganti role sambil di halaman yang tak diizinkan | Alur UF-09 langkah 9 belum diuji |
| GAP-10 | `localStorage` diblokir browser | Berpotensi layar putih (KI-03) |
