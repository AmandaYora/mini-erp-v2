# Feature Inventory — Modul 01 Auth & Session

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi dokumen ini
diturunkan langsung dari kode; tidak ada asumsi. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [test-cases.md](test-cases.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 6 (`auth/login`, `auth/refresh`, `auth/logout`, `auth/me`, `auth/switch-role`, `auth/switch-branch`) |
| Halaman | 2 (`/login`, `/select-branch`) + 1 halaman pendukung (`/403`) |
| Komponen sesi lintas halaman | Avatar menu (topbar), chip "Ganti Cabang", tombol Logout mobile, splash boot |
| Permission yang dibutuhkan | **Tidak ada** — semua endpoint auth hanya memakai `JwtAuthGuard`, tanpa `@RequirePermission` |
| Laporan | Tidak ada — lihat [reports-list.md](reports-list.md) |
| Penomoran dokumen | Tidak ada — lihat [numbering-sequence.md](numbering-sequence.md) |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Login

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Login dengan **username atau email** | Satu field input. Backend mencari `username = input` **OR** `email = input` |
| F-01.2 | Verifikasi password | `bcrypt.compare` terhadap `users.password_hash` |
| F-01.3 | Penerbitan token ganda | `access_token` (default 15 menit) + `refresh_token` (default 7 hari) |
| F-01.4 | Pembuatan baris sesi | Satu baris `user_sessions` per login — login berulang membuat sesi baru, sesi lama **tidak** dicabut |
| F-01.5 | Penentuan role aktif otomatis | Role pertama dari daftar role user (lihat catatan urutan di [business-rules.md](business-rules.md) BR-06) |
| F-01.6 | Penentuan cabang aktif otomatis | 1 cabang aktif → langsung dipilih; >1 dengan default → default dipilih; >1 tanpa default → minta pilih |
| F-01.7 | Pencatatan `last_login_at` | Di-update setiap login berhasil |
| F-01.8 | Pencatatan jejak perangkat | `user_agent` dan `ip_address` disimpan di baris sesi |
| F-01.9 | Pengayaan data cabang | Setelah login, frontend memanggil `branches/my-access` untuk mengambil kode/kota/label lokasi default cabang |
| F-01.10 | Toggle tampil/sembunyi password | Ikon mata di field password (`aria-label` "Tampilkan password" / "Sembunyikan password") |
| F-01.11 | Redirect kembali ke halaman tujuan | Setelah login, kembali ke halaman yang tadinya diminta (bila ada), bukan selalu dashboard |

**Yang TIDAK ada** (dipastikan dari kode — jangan ditambahkan di sistem baru tanpa permintaan):
lupa password / reset password, "ingat saya", registrasi mandiri, verifikasi email, login sosial,
OTP / 2FA, captcha, penguncian akun setelah N kali gagal, batas jumlah sesi aktif.

### F-02 — Pemilihan Cabang (Branch Selection)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Halaman pemilihan cabang | `/select-branch`, ditampilkan sebagai kartu per cabang |
| F-02.2 | Info per kartu | Nama cabang, kota, badge kode cabang, badge "Default" bila cabang default |
| F-02.3 | Info profil akses per kartu | Label "Access Profile" + nilai, plus badge daftar role |
| F-02.4 | Pilih cabang → set sesi | Memanggil `auth/switch-branch`, memperbarui `user_sessions.id_active_branch` |
| F-02.5 | Muat ulang data workspace | Setelah cabang berubah, data bootstrap dimuat ulang dan modul lain dimuat saat dikunjungi |
| F-02.6 | Notifikasi hasil | Toast "Cabang aktif diperbarui" (sukses) / "Gagal mengganti cabang" (gagal) |

### F-03 — Ganti Cabang Saat Bekerja (Switch Branch)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Chip "Ganti Cabang" di topbar | Muncul **hanya bila** `accessibleBranches.length > 1` |
| F-03.2 | Chip statis "Cabang" | Bila user hanya punya 1 cabang — menampilkan nama cabang, tidak bisa diklik |
| F-03.3 | Kembali ke halaman asal | Chip mengirim `state.from = location.pathname`, sehingga setelah memilih cabang user kembali ke halaman yang sama |
| F-03.4 | Validasi ID cabang di frontend | ID non-numerik / ≤ 0 ditolak sebelum request dikirim, dengan toast berisi nilai yang salah |

### F-04 — Ganti Role Aktif (Switch Role)

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Submenu "Ganti Role" di avatar menu | Muncul **hanya bila** user punya lebih dari 1 role |
| F-04.2 | Penanda role aktif | Role yang sedang aktif disorot di daftar |
| F-04.3 | Perbarui permission seketika | `auth/switch-role` mengembalikan daftar permission role baru; menu & akses langsung mengikuti |
| F-04.4 | Auto-pindah bila halaman tak lagi diizinkan | Bila halaman saat ini butuh permission yang tidak dimiliki role baru → otomatis ke `/dashboard` |
| F-04.5 | Notifikasi hasil | Toast "Role aktif diperbarui" / "Gagal mengganti role" |

### F-05 — Refresh Token Otomatis

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Deteksi 401 dan refresh otomatis | Setiap `apiPost`/`apiUpload` yang kena 401 memicu refresh satu kali, lalu request diulang |
| F-05.2 | Rotasi token | Refresh menerbitkan access **dan** refresh token baru; hash refresh lama ditimpa |
| F-05.3 | Perpanjangan masa sesi (sliding) | `user_sessions.expires_at` diperpanjang setiap refresh — sesi bisa hidup tanpa batas selama dipakai dalam 7 hari |
| F-05.4 | Antrean refresh tunggal | Banyak request 401 bersamaan hanya memicu **satu** panggilan `auth/refresh` |
| F-05.5 | Refresh tidak memicu loader global | Dianggap operasi infrastruktur, bukan aksi user |
| F-05.6 | Gagal refresh → sesi dianggap habis | Token dibersihkan dan error `session_expired` dilempar |
| F-05.7 | Pencabutan otomatis bila user tidak aktif | Bila status user bukan `active` saat refresh, baris sesi langsung di-`revoked_at` |

### F-06 — Pemulihan Sesi Saat Reload / Buka Ulang Browser

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Sesi bertahan setelah refresh halaman | Token + ringkasan sesi disimpan di `localStorage` |
| F-06.2 | Validasi ulang ke server | Saat mount, `auth/me` dipanggil untuk memastikan sesi masih sah |
| F-06.3 | Splash saat memulihkan sesi | Layar penuh gelap dengan teks "Memuat sesi..." |
| F-06.4 | Gagal validasi → bersihkan sesi | Token dan sesi lokal dihapus, user kembali ke login |
| F-06.5 | Penggabungan data lokal + server | Data perusahaan (nama/kode/timezone/mata uang/locale) diambil dari sesi lokal karena `auth/me` **tidak** mengembalikannya |

### F-07 — Logout

| # | Sub-fitur | Detail |
|---|---|---|
| F-07.1 | Logout dari avatar menu | Item "Logout" berwarna merah, paling bawah |
| F-07.2 | Logout dari sidebar mobile | Tombol "Logout" di blok bawah sidebar (viewport ≤ 900px) |
| F-07.3 | Pencabutan sesi di server | `auth/logout` menyetel `revoked_at` pada sesi **saat ini saja** |
| F-07.4 | Pembersihan state lokal | Token, sesi, data perusahaan/cabang, daftar user & cabang direset |
| F-07.5 | Notifikasi | Toast sukses "Logout berhasil" / "Sesi Anda telah diakhiri." |
| F-07.6 | Bersifat fire-and-forget | Kegagalan `auth/logout` di server **tidak** menghalangi logout di sisi klien |

### F-08 — Penjagaan Akses Halaman (Route Guard)

| # | Sub-fitur | Detail |
|---|---|---|
| F-08.1 | Tiga tingkat akses rute | `public` (login), `authenticated` (pilih cabang), `protected` (seluruh aplikasi) |
| F-08.2 | Urutan pemeriksaan tetap | Autentikasi → cabang aktif → permission |
| F-08.3 | Simpan halaman tujuan | Redirect ke login membawa `state.from` untuk dikembalikan setelah login |
| F-08.4 | Halaman 403 khusus | Judul "Akses Ditolak" + tombol "Kembali ke Dashboard" |
| F-08.5 | Sanitasi target redirect | Path eksternal / protocol-relative (`//host`) ditolak, jatuh ke `/dashboard` |
| F-08.6 | Blokir halaman login saat sudah login | Sudah login + ada cabang → `/dashboard`; sudah login tanpa cabang → `/select-branch` |

### F-09 — Penegakan Sesi di Setiap Request (Backend)

| # | Sub-fitur | Detail |
|---|---|---|
| F-09.1 | Verifikasi sesi per request | Baris sesi dicek ulang di DB pada **setiap** request — bukan hanya dari klaim token |
| F-09.2 | Permission di-resolve per request | Perubahan permission langsung berlaku tanpa logout / refresh token |
| F-09.3 | Verifikasi ulang akses cabang | Bila akses cabang dicabut, cabang di-drop dari sesi (sesi tetap hidup) |
| F-09.4 | Empat kondisi penolakan | Sesi tidak ada, sesi dicabut, sesi kedaluwarsa, status user bukan `active` |
| F-09.5 | Guard cabang terpisah | Operasi branch-scoped ditolak bila belum ada cabang aktif |
| F-09.6 | Sumber tunggal scope | `idCompany`/`idBranch`/`idUser`/role/permission selalu dari sesi, tidak pernah dari body request |

---

## 3. Edge Case yang Terbukti Ada di Kode

Semua baris di bawah diverifikasi dari kode. Yang berperilaku janggal dicatat juga di
[known-issues.md](../known-issues.md).

| # | Kondisi | Perilaku aktual |
|---|---|---|
| EC-01 | Username **dan** email dikirim bersamaan | `username` menang (`dto.username ?? dto.email`) |
| EC-02 | Keduanya kosong / tidak dikirim | Identifier jadi `''` → user tidak ditemukan → "Kredensial tidak valid" |
| EC-03 | Password kosong | Ditolak **sebelum** menyentuh DB oleh validasi DTO (`@MinLength(1)`) → error validasi, bukan "kredensial tidak valid" |
| EC-04 | User tidak ditemukan | Pesan sama dengan password salah — "Kredensial tidak valid" (tidak membocorkan mana yang salah) |
| EC-05 | Status user `inactive` / `locked` | "Kredensial tidak valid" — sama seperti user tidak ada |
| EC-06 | User tanpa role sama sekali | `403` "User tidak memiliki role" — **login gagal**, sesi tidak dibuat |
| EC-07 | User punya banyak role | Role aktif = role **pertama** yang dikembalikan DB, tanpa `ORDER BY` |
| EC-08 | User punya 1 cabang aktif | Cabang langsung dipilih, `requires_branch_selection = false` |
| EC-09 | User punya >1 cabang, salah satunya default | Default dipilih otomatis |
| EC-10 | User punya >1 cabang, **tanpa** default | `requires_branch_selection = true`, `active_branch = null` |
| EC-11 | **User tanpa cabang aktif sama sekali** | Login **berhasil**, `active_branch = null`, tapi `requires_branch_selection = false` → user terjebak (lihat known-issues KI-01) |
| EC-12 | User punya cabang, semuanya berstatus `inactive` | Sama dengan EC-11 — cabang non-aktif difilter habis |
| EC-13 | `branches/my-access` gagal saat login | Login tetap lanjut; kode & kota cabang jadi string kosong, label lokasi default jadi "Default" |
| EC-14 | Login berulang dari perangkat berbeda | Sesi lama tetap hidup; tidak ada batas jumlah sesi |
| EC-15 | Logout | Hanya sesi perangkat itu yang dicabut; perangkat lain tetap login |
| EC-16 | Refresh token dipakai dua kali (replay) | Pemakaian kedua gagal — hash sudah dirotasi, `bcrypt.compare` tidak cocok |
| EC-17 | Refresh dengan token yang bukan tipe refresh (mis. access token) | Ditolak sebelum menyentuh tabel sesi (`type !== 'refresh'`) |
| EC-18 | Refresh saat sesi sudah dicabut / kedaluwarsa | "Refresh token tidak valid" |
| EC-19 | Refresh saat user sudah di-nonaktifkan | Sesi **dicabut** lalu ditolak |
| EC-20 | Sesi dipakai terus-menerus | Masa sesi diperpanjang setiap refresh → tidak ada batas umur absolut |
| EC-21 | Ganti role ke role yang tidak dimiliki | `403` "Role tidak tersedia untuk user ini" |
| EC-22 | Role user dicabut sementara sesi masih aktif | Sesi tetap hidup dengan permission role lama sampai request berikutnya me-resolve ulang; `auth/me` melaporkan role pertama, bisa berbeda dari role yang dipakai permission (lihat KI-05) |
| EC-23 | Ganti cabang ke cabang tanpa akses | `403` "Tidak memiliki akses ke cabang ini" |
| EC-24 | Ganti cabang ke cabang berstatus `inactive` | **`500 internal_error`**, bukan pesan ramah (lihat KI-02) |
| EC-25 | Akses cabang dicabut saat user sedang bekerja | Cabang di-drop dari sesi → operasi branch-scoped ditolak "Branch aktif belum dipilih" → user diarahkan ke `/select-branch` |
| EC-26 | Buka `/login` saat masih login | Dialihkan ke `/dashboard` (atau `/select-branch` bila belum ada cabang) |
| EC-27 | Buka URL halaman terlindungi tanpa login | Dialihkan ke `/login`, halaman tujuan disimpan dan dibuka setelah login |
| EC-28 | `state.from` berisi URL eksternal | Diabaikan, jatuh ke `/dashboard` |
| EC-29 | `state.from` berisi `/login` atau `/select-branch` | Diabaikan, jatuh ke `/dashboard` (mencegah lingkaran redirect) |
| EC-30 | Buka halaman tanpa permission via URL langsung | Halaman `/403` "Akses Ditolak" |
| EC-31 | Ganti role sambil berada di halaman yang role baru tidak boleh akses | Otomatis dipindah ke `/dashboard` |
| EC-32 | `localStorage` diblokir browser | Aplikasi bisa gagal total (layar putih) — lihat KI-03 |
| EC-33 | Sesi lokal ada tapi token hilang | Dianggap belum login, langsung ke halaman login tanpa error |
| EC-34 | Token ada tapi sesi lokal hilang | Sama seperti EC-33 — keduanya harus ada |
| EC-35 | Seed dijalankan ulang | Semua sesi aktif untuk user 1–5 dicabut → semua perangkat ter-logout |
| EC-36 | `JWT_REFRESH_EXPIRES` diisi format tak dikenal (mis. `1w`) | Masa baris sesi jatuh ke 7 hari, sementara masa token JWT mengikuti nilai env → dua masa berbeda (lihat KI-06) |

---

## 4. Fitur yang Terlihat Ada Tapi Tidak Berfungsi

Dicatat di sini karena **terlihat oleh user** — perlu keputusan apakah dibuat berfungsi atau
dihapus di sistem baru.

| # | Elemen | Kondisi aktual |
|---|---|---|
| NF-01 | Menu **"Profil Saya"** di avatar menu | Hanya menutup dropdown. Tidak ada navigasi, tidak ada halaman profil user di seluruh aplikasi |
| NF-02 | Label **"Cabang Aktif"** di topbar | Nilai di bawahnya adalah **judul halaman**, bukan nama cabang |
| NF-03 | Badge role di kartu pemilihan cabang | Menampilkan seluruh role user, sama di setiap kartu — bukan role per cabang |
| NF-04 | Label **"Access Profile"** di kartu cabang | Nilainya adalah nama lengkap user, bukan sebuah profil akses |

---

## 5. Akun Bawaan (Seed)

Dipakai oleh test E2E dan demo. Password di-hash bcrypt cost 12.

| id | Username | Password | Role | Nama Lengkap | Email | Telepon |
|---|---|---|---|---|---|---|
| 1 | `superadmin` | `superadmin123` | superadmin | Super Admin | superadmin@mini-erp.local | 6281100000001 |
| 2 | `owner` | `owner123` | owner | Owner | owner@mini-erp.local | 6281100000002 |
| 3 | `admin` | `admin123` | admin | Admin | admin@mini-erp.local | 6281100000003 |
| 4 | `staff` | `staff123` | staff | Staff | staff@mini-erp.local | 6281100000004 |
| 5 | `kasir` | `kasir 123` | kasir | Kasir | kasir@mini-erp.local | 6281100000005 |

Catatan penting:

- **Password `kasir` mengandung spasi** (`kasir 123`) — bukan salah tulis di dokumen ini; test E2E
  memakainya persis begitu.
- Kelima akun terikat ke `id_company = 1` dan punya akses ke `id_branch = 1` dengan
  `is_default_branch = 1` → semuanya melewati pemilihan cabang saat login.
- Role `superadmin` dijaga hanya untuk `id_user = 1`: seed dan migrasi `042` menghapus penetapan
  `superadmin` dari user lain setiap kali dijalankan.
