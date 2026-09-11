# Business Rules — Modul 01 Auth & Session

**Kelompok A — SEMUA validasi, kondisi, dan aturan keputusan.** Setiap aturan disertai lokasi
kode agar bisa diverifikasi ulang. Tidak ada asumsi; bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [user-flows.md](user-flows.md) ·
[test-cases.md](test-cases.md) · [known-issues.md](../known-issues.md)

---

## 1. Validasi Input (Payload)

Modul auth adalah **satu-satunya** modul di seluruh API yang memakai validasi DTO runtime
(`class-validator`). Semua modul lain memvalidasi manual di service.

### BR-01 — Validasi payload `auth/login`

`auth.controller.ts` → `LoginDto`

| Field | Aturan | Wajib |
|---|---|---|
| `username` | harus string bila dikirim | tidak |
| `email` | harus string bila dikirim | tidak |
| `password` | harus string, **panjang minimal 1** | **ya** |

- Field di luar ketiga nama itu **dibuang tanpa peringatan** (`whitelist: true`,
  `forbidNonWhitelisted: false`)
- `password` yang tidak dikirim atau kosong → **400** dengan envelope
  `{ code: 200, info: "validation_failed", errors: [...] }`
- `username` **dan** `email` keduanya boleh kosong — tidak ada validasi "salah satu wajib"

### BR-02 — Prioritas identifier

`identifier = dto.username ?? dto.email ?? ''`

- Bila keduanya dikirim, **`username` menang**
- Bila keduanya kosong, identifier menjadi string kosong → dicari ke DB → tidak ditemukan →
  ditolak sebagai kredensial tidak valid (bukan error validasi)
- Frontend **selalu** mengirim field `username`, bahkan ketika user mengetik alamat email

### BR-03 — Validasi payload endpoint lain

| Endpoint | Field | Aturan |
|---|---|---|
| `auth/refresh` | `refresh_token` | harus string, wajib |
| `auth/switch-role` | `role_code` | harus string, wajib |
| `auth/switch-branch` | `id_branch` | harus **angka** dan **positif** (`@IsNumber` + `@IsPositive`) |
| `auth/logout` | — | tanpa payload |
| `auth/me` | — | tanpa payload |

Catatan: `id_branch` dilewatkan `Number()` sekali lagi di controller sebelum masuk service —
lapisan aman ganda terhadap konversi implisit.

### BR-04 — Validasi di sisi frontend

| Lokasi | Aturan |
|---|---|
| Form login | Kedua field ber-atribut `required` → browser memblokir submit kosong sebelum request terkirim |
| Ganti cabang | ID cabang harus lolos `Number.isFinite` **dan** `> 0`; bila tidak, request **tidak dikirim** dan muncul toast `ID cabang tidak valid: "{nilai}"` |
| Sanitasi redirect | Tujuan harus diawali `/` dan **tidak** diawali `//`; bukan `/`, `/login`, atau `/select-branch` |

Tidak ada aturan panjang/kompleksitas password di sisi login (aturan itu, bila ada, berada di modul
Users saat membuat/mengubah password — di luar cakupan dokumen ini).

---

## 2. Aturan Autentikasi

### BR-05 — Empat syarat login berhasil

Diperiksa **berurutan**; gagal di satu titik langsung menghentikan proses.

| # | Syarat | Bila gagal |
|---|---|---|
| 1 | Baris user ditemukan (cocok `username` **atau** `email`) | `401` "Kredensial tidak valid" |
| 2 | `users.status === 'active'` | `401` "Kredensial tidak valid" |
| 3 | `bcrypt.compare(password, users.password_hash)` bernilai true | `401` "Kredensial tidak valid" |
| 4 | User punya **minimal satu** role di `user_roles` | `403` "User tidak memiliki role" |

**Aturan penting — syarat 1 dan 2 disatukan** dalam satu ekspresi
(`if (user?.status !== 'active')`). Konsekuensi yang disengaja: user tidak ada, user `inactive`,
dan user `locked` **menghasilkan pesan yang persis sama**, sehingga tidak membocorkan apakah suatu
username terdaftar. Ini properti keamanan yang harus dipertahankan.

Syarat 4 **berbeda sifatnya**: pesannya spesifik dan statusnya 403, sehingga keberadaan user
tanpa role bisa dibedakan dari password salah. **[PERLU KONFIRMASI]** apakah perbedaan ini
disengaja (membantu admin mendiagnosis) atau sebaiknya diseragamkan.

### BR-06 — Penentuan role aktif saat login

`primaryRole = userRoles[0].role`

- Diambil dari **elemen pertama** hasil `find({ idUser })`
- Query **tidak memiliki `ORDER BY`**, sehingga urutannya ditentukan MySQL (praktiknya mengikuti
  urutan primary key `(id_user, id_role)`, tapi tidak dijamin)
- Untuk user dengan satu role (seluruh akun bawaan) aturan ini tidak berdampak
- Untuk user dengan beberapa role, **role aktif saat login tidak dapat diprediksi dari kode** —
  lihat [known-issues.md](../known-issues.md) KI-05

Tidak ada konsep "role utama" tersimpan di data — tidak ada kolom `is_primary` di `user_roles`.

### BR-07 — Aturan penentuan cabang aktif saat login

Hanya cabang dengan `branches.status = 'active'` diperhitungkan. Cabang non-aktif dibuang lebih
dulu.

| Jumlah cabang aktif | `active_branch` | `requires_branch_selection` |
|---|---|---|
| Tepat 1 | cabang itu | `false` |
| > 1, ada tepat satu ber-`is_default_branch = 1` | cabang default | `false` |
| > 1, ada **beberapa** ber-`is_default_branch = 1` | **yang pertama ditemukan** | `false` |
| > 1, **tidak ada** default | `null` | `true` |
| **0** | `null` | **`false`** ← lihat KI-01 |

Tidak ada aturan yang mencegah lebih dari satu cabang bertanda default untuk satu user — DB hanya
menjamin keunikan pasangan `(id_user, id_branch)`. **[PERLU KONFIRMASI]** apakah modul Users
menegakkan "satu default per user"; ini ditelusuri saat analisis modul 02.

### BR-08 — Aturan verifikasi sesi di setiap request

`jwt.strategy.ts` — dijalankan pada **setiap** request yang membawa Bearer token.

| # | Syarat | Bila gagal |
|---|---|---|
| 1 | JWT sah dan belum kedaluwarsa (`ignoreExpiration: false`) | `401` |
| 2 | Baris sesi ada dengan **`id` DAN `id_user` cocok** dari klaim token | `401` |
| 3 | `user_sessions.revoked_at` masih `NULL` | `401` |
| 4 | `user_sessions.expires_at >= sekarang` | `401` |
| 5 | `users.status === 'active'` | `401` |

Syarat 2 memeriksa **dua** kolom sekaligus — token dengan `sessionId` valid tapi `sub` (id user)
milik orang lain akan ditolak.

Setelah lolos, sistem menyusun payload sesi:

- `permissions` — **selalu di-query ulang** dari `role_permissions` berdasarkan
  `user_sessions.id_active_role`, **bukan** dari klaim token
- `activeRoleCode` — dari tabel `roles`; bila baris role hilang, nilainya jadi **string kosong**
- `idActiveBranch` — bila terisi, dicek ulang ke `user_branch_accesses`; **bila akses sudah tidak
  ada, cabang dijadikan `null`** dan sesi tetap dianggap sah

Biaya: **4 query DB per request terautentikasi** (sesi+user, role-permissions, role, akses cabang).

### BR-09 — Konsekuensi resolusi permission per request

Karena BR-08 me-resolve ulang permission setiap request:

- Perubahan permission role **langsung berlaku** tanpa logout atau refresh token
- Pencabutan akses cabang **langsung berlaku** pada request berikutnya
- Perubahan status user menjadi non-aktif **langsung memutus** akses
- Namun daftar permission yang dipakai **frontend** (untuk menyembunyikan menu) berasal dari
  ringkasan sesi di `localStorage` yang hanya diperbarui saat login, `auth/me`, atau ganti role —
  sehingga UI bisa **sesaat lebih permisif** daripada server. Server tetap menolak, jadi ini
  masalah tampilan, bukan celah akses.

### BR-10 — Aturan guard cabang

`branch.guard.ts`: bila `session.idActiveBranch` kosong → `403` **"Branch aktif belum dipilih"**.

Guard ini **hanya memeriksa keberadaan** cabang aktif, bukan kelayakannya. Verifikasi "user boleh
mengakses cabang ini" terjadi di BR-08.

### BR-11 — Aturan guard permission

`permission.guard.ts`:

1. Metadata permission dibaca dari **handler dulu**, baru dari **class** sebagai fallback
2. Bila tidak ada metadata → **akses diizinkan** (gagal-terbuka)
3. Bila ada metadata tapi payload sesi kosong → `403`
4. Bila permission yang dibutuhkan tidak ada di daftar sesi → `403`

**Seluruh 6 endpoint modul auth tidak memakai `@RequirePermission`** — hanya `JwtAuthGuard`. Jadi
setiap user yang berhasil login boleh memanggil `me`, `logout`, `switch-role`, dan `switch-branch`
tanpa permission apa pun. Ini benar secara desain (mengelola sesi sendiri), tapi berarti guard
permission tidak berperan di modul ini.

---

## 3. Aturan Token & Masa Berlaku

### BR-12 — Parameter token

| Parameter | Sumber | Nilai bawaan |
|---|---|---|
| Secret penandatanganan | env `JWT_SECRET` | `'fallback-secret'` (hanya dev — dikonfirmasi selalu di-set di production) |
| Masa access token | env `JWT_ACCESS_EXPIRES` | `15m` |
| Masa refresh token | env `JWT_REFRESH_EXPIRES` | `7d` |
| Cost bcrypt hash refresh token | tetap di kode | **12** |
| Cost bcrypt placeholder sesi | tetap di kode | **1** |
| Cost bcrypt password seed | tetap di kode | **12** |

**Isi klaim token:**

| Token | Klaim |
|---|---|
| Access | `{ sub: id_user, sessionId: id_user_session, companyId: id_company }` |
| Refresh | `{ sub: id_user, sessionId: id_user_session, type: 'refresh' }` |

Access token **tidak** memuat role maupun permission — keduanya selalu dari DB (BR-08).

### BR-13 — Format nilai masa berlaku

`parseExpiryToMs` hanya menerima pola `^(\d+)(ms|s|m|h|d)$` (huruf besar/kecil bebas):

| Satuan | Pengali |
|---|---|
| `ms` | 1 |
| `s` | 1.000 |
| `m` | 60.000 |
| `h` | 3.600.000 |
| `d` | 86.400.000 |

Format lain (mis. `1w`, `30 m`, `2d12h`) → `null` → masa **baris sesi** jatuh ke **7 hari**,
sementara masa **token JWT** tetap mengikuti nilai env (pustaka `jsonwebtoken` menerima lebih
banyak format). Dua masa berbeda; lihat [known-issues.md](../known-issues.md) KI-06.

### BR-14 — Aturan refresh token

Diperiksa berurutan:

| # | Syarat | Bila gagal |
|---|---|---|
| 1 | JWT sah | `401` "Refresh token tidak valid" |
| 2 | `payload.type === 'refresh'` | `401` — **ditolak sebelum menyentuh tabel sesi** |
| 3 | `payload.sessionId` dan `payload.sub` ada | `401` |
| 4 | Baris sesi ada dengan `id` + `id_user` cocok | `401` |
| 5 | Sesi belum dicabut **dan** belum kedaluwarsa | `401` |
| 6 | `bcrypt.compare(token, refresh_token_hash)` cocok | `401` |
| 7 | `users.status === 'active'` | `401` **+ sesi langsung dicabut** |

Syarat 2 mencegah access token dipakai sebagai refresh token.

Syarat 6 memberi properti penting: **refresh token bersifat sekali pakai**. Karena setiap refresh
menimpa hash, token lama otomatis tidak cocok lagi. Percobaan replay ditolak.

Syarat 7 adalah satu-satunya jalur di seluruh sistem yang **mencabut sesi secara otomatis**.

### BR-15 — Rotasi & perpanjangan masa sesi (sliding session)

Setiap refresh berhasil:

1. Access token baru diterbitkan
2. Refresh token baru diterbitkan
3. `refresh_token_hash` ditimpa hash token baru
4. `user_sessions.expires_at` **direset ke `sekarang + JWT_REFRESH_EXPIRES`**

Konsekuensi: **tidak ada batas umur absolut sesi.** Sesi yang dipakai setidaknya sekali setiap 7
hari dapat hidup selamanya. Karena access token hanya berumur 15 menit, refresh terjadi rutin —
praktisnya sesi user aktif tidak pernah kedaluwarsa.

**[PERLU KONFIRMASI]** apakah perilaku ini disengaja (kenyamanan: kasir tidak perlu login ulang)
atau perlu batas umur absolut di sistem baru.

### BR-16 — Aturan pencabutan sesi

| Pemicu | Cakupan |
|---|---|
| Logout | **Hanya sesi perangkat itu** |
| Refresh saat user non-aktif | Sesi yang dipakai refresh |
| Seed / reset DB dijalankan | **Semua** sesi aktif user 1–5 |

Tidak ada: "logout dari semua perangkat", batas jumlah sesi bersamaan, pencabutan otomatis saat
password diubah, atau pencabutan saat role/akses cabang berubah.

Login berulang **selalu membuat baris sesi baru** — tidak ada pemakaian ulang sesi.

### BR-17 — Dua tahap penulisan hash sesi

Saat login, baris sesi dibuat dengan `refresh_token_hash` berisi hash dari string
literal `'placeholder'` (cost 1), lalu **langsung ditimpa** hash refresh token sebenarnya
(cost 12) setelah token diterbitkan.

Alasannya struktural: `sessionId` harus sudah ada untuk masuk ke dalam klaim token, sementara
token harus sudah ada untuk di-hash. Dampak yang perlu diketahui: ada jendela waktu sangat singkat
di mana baris sesi berisi hash placeholder. Karena tidak ada token yang pernah cocok dengan hash
itu, jendela ini tidak bisa dieksploitasi untuk refresh.

---

## 4. Aturan Ganti Role & Ganti Cabang

### BR-18 — Aturan ganti role

| # | Syarat | Bila gagal |
|---|---|---|
| 1 | `role_code` ada di daftar role user | `403` "Role tidak tersedia untuk user ini" |

Yang **tidak** diperiksa:

- Tidak ada pemeriksaan role masih aktif / belum dihapus (tidak ada kolom status di `roles`)
- Tidak ada larangan berpindah ke role `superadmin`. Aturan `superadmin` ditegakkan di **lapisan
  data**: seed dan migrasi `042` menghapus penetapan `superadmin` dari semua user selain
  `id_user = 1`, sehingga user lain tidak punya role itu untuk dipindahi
- Tidak ada audit log (modul auth termasuk EXEMPT dari audit; jejaknya adalah `user_sessions`)

Setelah berhasil, `user_sessions.id_active_role` diperbarui dan daftar permission role baru
dikembalikan.

### BR-19 — Aturan ganti cabang

| # | Syarat | Bila gagal |
|---|---|---|
| 1 | Ada baris `user_branch_accesses` untuk `(id_user, id_branch)` | `403` "Tidak memiliki akses ke cabang ini" |
| 2 | Cabang ada **dan** `branches.status = 'active'` | **`500 internal_error`** ← lihat KI-02 |

Syarat 2 memakai `findOneOrFail`, yang melempar `EntityNotFoundError` — bukan `HttpException` —
sehingga tertangkap sebagai kesalahan internal, bukan pesan bisnis. User melihat kegagalan generik
padahal penyebabnya jelas dan bisa dijelaskan.

Setelah berhasil, `user_sessions.id_active_branch` diperbarui.

### BR-20 — Aturan bertahan di halaman setelah ganti role

Dijalankan di frontend setelah ganti role berhasil:

```
permissionHalaman = permission yang dibutuhkan rute saat ini
bolehTetap = (tidak ada permissionHalaman) ATAU (permissionHalaman ada di permission role baru)
jika TIDAK bolehTetap → pindah ke /dashboard
```

Daftar permission role baru dibaca dari **data perusahaan** (`rolePermissions[role]`), bukan dari
ringkasan sesi — karena sesi belum tentu sudah ter-update pada saat pemeriksaan.

---

## 5. Aturan Envelope & Pesan Error

### BR-21 — Envelope respons

Seluruh endpoint auth mengembalikan **HTTP 200** saat sukses (`@HttpCode(200)`), dibungkus:

```
{ "code": 0, "info": "success", "data": <payload> }
```

`auth/logout` mengembalikan `data: null` (service tidak mengembalikan apa pun).

### BR-22 — Pemetaan error ke envelope

| Kondisi | HTTP | `code` | `info` | `errors` |
|---|---|---|---|---|
| Kredensial salah / user non-aktif | 401 | 100 | `unauthorized` | "Kredensial tidak valid" |
| Refresh token tidak sah | 401 | 100 | `unauthorized` | "Refresh token tidak valid" |
| User tanpa role | 403 | 400 | `forbidden` | "User tidak memiliki role" |
| Role tidak dimiliki | 403 | 400 | `forbidden` | "Role tidak tersedia untuk user ini" |
| Tidak punya akses cabang | 403 | 400 | `forbidden` | "Tidak memiliki akses ke cabang ini" |
| Belum pilih cabang (guard) | 403 | 400 | `forbidden` | "Branch aktif belum dipilih" |
| Permission kurang (guard) | 403 | 400 | `forbidden` | (pesan default Nest) |
| Payload tidak valid | 400 | 200 | `validation_failed` | daftar pesan validator |
| Cabang non-aktif saat switch | 500 | 1 | `internal_error` | **tidak ada** |

Perhatikan `code: 400` berarti **FORBIDDEN** dalam kode envelope — bukan HTTP 400. Tabrakan angka
ini berlaku di seluruh sistem.

### BR-23 — Pesan yang benar-benar dilihat user saat login gagal

Frontend memilih pesan dengan urutan `info` → `message` → fallback:

```
error: apiErr?.info ?? (err as Error)?.message ?? "Login gagal. Periksa username dan password."
```

Karena `info` **selalu terisi** untuk error HTTP, pesan ramah di `errors` **tidak pernah tampil**
pada kegagalan login. Yang user lihat:

| Penyebab | Teks di Notice |
|---|---|
| Password salah / user tidak ada / non-aktif | `unauthorized` |
| User tanpa role | `forbidden` |
| Server tidak mengembalikan JSON | pesan Indonesia dari lapisan API (mis. "Server sedang sibuk/tidak tersedia. Coba beberapa saat lagi.") |

Fallback "Login gagal. Periksa username dan password." praktis **tidak pernah terpakai**.
Lihat [known-issues.md](../known-issues.md) KI-04.

---

## 6. Aturan Penyimpanan Sesi di Klien

### BR-24 — Kunci penyimpanan

| Kunci `localStorage` | Isi |
|---|---|
| `mini-erp-access` | access token |
| `mini-erp-refresh` | refresh token |
| `mini-erp-session` | ringkasan sesi (JSON) |

### BR-25 — Syarat sesi dianggap ada

Saat aplikasi dimuat, sesi dianggap ada **hanya bila `mini-erp-access` DAN `mini-erp-session`
keduanya terisi**. Bila salah satu hilang, aplikasi langsung menganggap user belum login tanpa
menghubungi server dan tanpa pesan error.

`isAuthenticated` di frontend diturunkan **semata-mata** dari keberadaan ringkasan sesi lokal —
bukan dari validitas token. Validasi sebenarnya terjadi saat request pertama ke server.

### BR-26 — Ketahanan pembacaan penyimpanan

| Pembaca | Penanganan gagal |
|---|---|
| Ringkasan sesi (`loadSession`) | Dibungkus try/catch → dianggap tidak ada bila JSON rusak |
| Preferensi sidebar | Dibungkus try/catch → dianggap `false` |
| **Token** (`getStoredTokens`/`storeTokens`) | **Tidak dibungkus** → dapat menggagalkan aplikasi total (KI-03) |

### BR-27 — Nilai bawaan saat memulihkan sesi

`auth/me` **tidak** mengembalikan data perusahaan, sehingga nilai berikut diambil dari sesi lokal
lama, dengan bawaan bila kosong:

| Field | Bawaan |
|---|---|
| `companyTimezone` | `Asia/Jakarta` |
| `companyCurrencyCode` | `IDR` |
| `companyLocale` | `id-ID` |
| `defaultStockLocationLabel` per cabang | `Default` |
| `activeBranchCode`, kode & kota cabang | string kosong |

Nilai bawaan yang sama dipakai server saat login bila relasi `company` kosong.

---

## 7. Aturan Kebijakan Role `superadmin`

### BR-28 — `superadmin` adalah role developer cadangan

Ditegakkan di lapisan data, bukan di modul auth:

| Mekanisme | Aturan |
|---|---|
| Migrasi `042` + seed | Menghapus penetapan role `superadmin` dari **semua** user selain `id_user = 1`, lalu memastikan `id_user = 1` memilikinya |
| Konstanta kode | `RESERVED_SUPERADMIN_ROLE_CODE = 'superadmin'`, `DEFAULT_SUPERADMIN_USER_ID = 1` |
| Perbandingan | Memakai `trim().toLowerCase()` sehingga `"SuperAdmin "` juga tertangkap |
| Role sistem | Baris `roles` ber-`id_company = NULL` dan `is_system_role = 1` |

Modul auth sendiri **tidak** memeriksa apa pun soal `superadmin`: login dan ganti role
memperlakukannya seperti role lain. Yang mencegah penyalahgunaan adalah tidak adanya penetapan
role itu ke user lain.

### BR-29 — `kasir` bukan role sistem

`kasir` di-seed sebagai role **kustom per-perusahaan** (`id_role = 5`, `id_company = 1`,
`is_system_role = 0`) dengan 9 permission: `dashboard.view`, `product.view`, `order.view`,
`order.create`, `order.update`, `sales_return.view`, `sales_return.create`, `stock.view`,
`stock.adjust`.

Role sistem hanya empat: `superadmin`, `owner`, `admin`, `staff`.

---

## 8. Formula & Perhitungan

Modul ini **tidak memiliki formula bisnis** — tanpa perhitungan uang, kuantitas, pajak, atau
pembulatan. Satu-satunya perhitungan numerik:

| Perhitungan | Rumus |
|---|---|
| Masa berlaku baris sesi | `sekarang + parseExpiryToMs(JWT_REFRESH_EXPIRES)`, bawaan 7 hari |
| Konversi satuan masa | lihat BR-13 |
| Inisial avatar | 2 karakter pertama nama lengkap, dikapitalkan |

---

## 9. Aturan Approval

**Tidak ada aturan approval di modul ini.** Tidak ada persetujuan atasan, tidak ada verifikasi
kredensial pihak kedua, tidak ada alur permintaan akses.

Sebagai pembanding: mekanisme approval bergaya "atasan mengetik password di layar operator" ada di
modul Stock (penyesuaian stok mengirim `approval: { username, password }` dalam payload). Modul
auth tidak dipakai untuk itu — verifikasinya terjadi di dalam modul Stock sendiri, **[PERLU
KONFIRMASI]** apakah ia memanggil ulang logika verifikasi password auth atau punya salinan
sendiri; ditelusuri saat analisis modul 16.

---

## 10. Aturan Audit

### BR-30 — Modul auth EXEMPT dari audit log

Modul auth **tidak** memanggil `AuditLogService`. Ini keputusan sadar yang tercatat di
`.claude/rules/backend-nestjs.md`: **`user_sessions` adalah jejak auditnya**.

Yang terekam sebagai jejak:

| Data | Kolom | Merekam |
|---|---|---|
| Waktu login | `user_sessions.created_at` | Setiap login berhasil |
| Login terakhir | `users.last_login_at` | Ditimpa setiap login |
| Perangkat | `user_sessions.user_agent` | User-Agent saat login |
| Alamat IP | `user_sessions.ip_address` | IP saat login |
| Waktu logout / pencabutan | `user_sessions.revoked_at` | Logout, atau pencabutan otomatis |
| Cabang & role aktif terakhir | `id_active_branch`, `id_active_role` | **Nilai terkini saja** |

Yang **tidak** terekam sama sekali:

- **Percobaan login yang gagal** — tidak ada baris apa pun dibuat. Tidak mungkin mengetahui
  berapa kali sebuah akun gagal login, dari IP mana, atau kapan
- **Riwayat ganti role dan ganti cabang** — kolom di sesi ditimpa, nilai lama hilang. Tidak
  mungkin menjawab "cabang mana yang aktif saat order ini dibuat" dari data sesi
- Waktu refresh token terakhir (hanya `expires_at` yang bergeser)

**[PERLU KONFIRMASI]** apakah ketiadaan jejak percobaan login gagal dan riwayat ganti role/cabang
dapat diterima di sistem baru, atau perlu dicatat. Ini berdampak pada kemampuan menyelidiki
insiden.

---

## 11. Ringkasan Kondisi Khusus

| # | Kondisi | Aturan |
|---|---|---|
| SK-01 | User tanpa cabang aktif | Login **berhasil** tapi tanpa cabang; `requires_branch_selection` tetap `false` (KI-01) |
| SK-02 | Semua cabang user berstatus non-aktif | Sama seperti SK-01 |
| SK-03 | Beberapa cabang bertanda default | Yang pertama ditemukan dipakai; tidak ada error |
| SK-04 | `branches/my-access` gagal saat login | Login lanjut; kode & kota cabang jadi kosong |
| SK-05 | Baris `roles` untuk role aktif hilang | `activeRoleCode` jadi string kosong; sesi tetap sah; permission tetap di-query (kemungkinan kosong) |
| SK-06 | Baris user hilang saat `auth/me` | `findOneOrFail` melempar → **500** (tidak mungkin terjadi bila sesi sah, karena relasi FK) |
| SK-07 | Refresh token dipakai dua kali | Pemakaian kedua ditolak — hash sudah dirotasi |
| SK-08 | Access token dipakai sebagai refresh token | Ditolak oleh pemeriksaan `type !== 'refresh'` |
| SK-09 | Token milik user lain dengan `sessionId` valid | Ditolak — `id` dan `id_user` harus cocok bersamaan |
| SK-10 | `localStorage` diblokir browser | Aplikasi dapat gagal total (KI-03) |
| SK-11 | Login berulang tanpa logout | Sesi menumpuk tanpa batas |
| SK-12 | Seed dijalankan ulang | Semua sesi user 1–5 dicabut **dan** password kembali ke bawaan |
