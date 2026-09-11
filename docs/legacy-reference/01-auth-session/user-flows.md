# User Flows — Modul 01 Auth & Session

**Kelompok A — alur end-to-end yang harus identik di sistem baru.** Setiap langkah diturunkan
dari kode; nama endpoint, teks UI, dan urutan redirect ditulis apa adanya.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [ui-ux-spec.md](ui-ux-spec.md) ·
[business-rules.md](business-rules.md) · [test-cases.md](test-cases.md)

Notasi: `→` langkah berikutnya · **[server]** panggilan API · **[lokal]** hanya di browser

---

## UF-01 — Login, satu cabang (alur paling umum)

Berlaku untuk **seluruh 5 akun bawaan**, karena semuanya hanya punya akses ke `id_branch = 1`.

1. User membuka URL aplikasi apa pun tanpa sesi → `RouteGuard` mengalihkan ke `/login`, menyimpan
   halaman tujuan di `state.from`
2. Halaman login tampil: hero kiri (desktop), kartu form kanan
3. User mengisi **"Username or Email"** lalu **"Password"** → menekan **"Sign In"** (atau Enter)
4. **[server]** `POST auth/login` dengan `{ data: { username: <input>, password: <input> } }`
   - Frontend selalu mengirim field bernama `username`, bahkan bila user mengetik email — backend
     mencari di kedua kolom
5. Server: user ditemukan & aktif → password cocok → punya role → punya **1 cabang aktif** →
   cabang langsung dipilih
6. Server membuat baris `user_sessions`, menerbitkan `access_token` + `refresh_token`, mencatat
   `last_login_at`, dan mengembalikan `requires_branch_selection: false`
7. **[lokal]** Token disimpan (`mini-erp-access`, `mini-erp-refresh`)
8. **[server]** `POST branches/my-access` untuk melengkapi kode/kota/label lokasi default cabang
   - Bila gagal: alur **tetap lanjut**, kode & kota cabang jadi string kosong
9. **[lokal]** Ringkasan sesi disimpan (`mini-erp-session`)
10. **[server]** Bootstrap dimuat (definisi status order + pengaturan perusahaan)
11. **[lokal]** Halaman berpindah ke `state.from` bila ada dan valid; kalau tidak → `/dashboard`
12. Shell aplikasi tampil: sidebar, topbar (chip Perusahaan + chip Cabang statis), avatar menu

**Tidak ada toast sukses.** Umpan balik keberhasilan adalah perpindahan halaman itu sendiri.

---

## UF-02 — Login, banyak cabang tanpa cabang default

1. Langkah 1–4 sama seperti UF-01
2. Server menemukan **lebih dari 1 cabang aktif** dan **tidak ada** yang bertanda default →
   `active_branch: null`, `requires_branch_selection: true`
3. Sesi **tetap dibuat** dan token **tetap diterbitkan** — user sudah terautentikasi, hanya belum
   punya cabang aktif
4. Bootstrap **tidak** dimuat pada tahap ini (menunggu cabang dipilih)
5. **[lokal]** Halaman berpindah ke `/select-branch`, membawa `state.from` hasil sanitasi
6. Halaman "Select Branch" tampil dengan deskripsi
   `{Nama Perusahaan} has multiple branches available for your account. Please select one to proceed.`
7. User mengklik salah satu kartu cabang
8. **[server]** `POST auth/switch-branch` dengan `{ data: { id_branch: <angka> } }`
9. Server memverifikasi akses cabang → memverifikasi cabang berstatus aktif → memperbarui
   `user_sessions.id_active_branch`
10. **[lokal]** Ringkasan sesi diperbarui (id/nama/kode cabang aktif)
11. **[server]** Bootstrap dimuat
12. Toast **"Cabang aktif diperbarui"** / "Data tiap modul akan dimuat ulang saat Anda mengunjunginya."
13. Halaman berpindah ke `state.from` bila valid; kalau tidak → `/dashboard`

---

## UF-03 — Login, banyak cabang dengan satu cabang default

1. Langkah 1–4 sama seperti UF-01
2. Server memilih cabang bertanda `is_default_branch = 1` → `requires_branch_selection: false`
3. Alur selanjutnya identik UF-01 langkah 6–12 — **user tidak melihat halaman pemilihan cabang**
4. Karena user punya > 1 cabang, topbar menampilkan chip **"Ganti Cabang"** yang bisa diklik

---

## UF-04 — Login gagal

1. Langkah 1–4 sama seperti UF-01
2. Server menolak. Tiga kemungkinan penyebab dengan hasil berbeda:

| Penyebab | Status | Yang tampil di Notice |
|---|---|---|
| Password salah, user tidak ada, atau status user bukan `active` | 401 | `unauthorized` |
| User tidak punya role sama sekali | 403 | `forbidden` |
| Password kosong (lolos `required` HTML, mis. via autofill) | 400 | pesan validasi dari server |

3. **[lokal]** Token **tidak** disimpan, sesi lokal **tidak** dibuat
4. Blok `Notice` merah berjudul **"Authentication Error"** muncul di atas form
5. URL tetap di `/login`. Field tidak dikosongkan — user bisa langsung memperbaiki input
6. Error tetap tampil sampai submit berikutnya

Catatan: untuk penyebab 401 dan 403, server sebenarnya mengirim pesan Indonesia yang ramah
("Kredensial tidak valid", "User tidak memiliki role") tetapi frontend menampilkan kode mesinnya.
Lihat [known-issues.md](../known-issues.md) KI-04.

---

## UF-05 — Membuka ulang aplikasi (sesi masih hidup)

1. User membuka aplikasi (tab baru / reload / buka browser lagi)
2. **[lokal]** Provider memeriksa `mini-erp-access` **dan** `mini-erp-session`
   - Bila salah satu tidak ada → langsung dianggap belum login, splash dilewati, user ke `/login`
3. Splash layar penuh **"Memuat sesi..."** tampil
4. **[server]** `POST auth/me`
5. Server memverifikasi sesi (ada, tidak dicabut, belum kedaluwarsa, user aktif) dan mengembalikan
   identitas, role aktif, daftar role, daftar cabang, dan permission
6. **[lokal]** Ringkasan sesi dibangun ulang dengan menggabungkan:
   - dari server: id/nama/email user, cabang aktif, role aktif, daftar role, permission, daftar cabang
   - dari sesi lokal lama: seluruh data perusahaan, kode cabang aktif, kota & label lokasi default
     tiap cabang (karena `auth/me` tidak mengembalikannya)
7. Bila ada cabang aktif → **[server]** bootstrap dimuat
8. Splash hilang, aplikasi tampil di halaman yang diminta

---

## UF-06 — Membuka ulang aplikasi (sesi sudah tidak sah)

1. Langkah 1–4 sama seperti UF-05
2. `auth/me` gagal — sesi dicabut (logout dari perangkat lain / seed dijalankan ulang), sesi
   kedaluwarsa, atau user dinonaktifkan
3. Karena request memakai `apiPost`, kegagalan 401 lebih dulu memicu **satu kali** percobaan
   `auth/refresh`:
   - Bila refresh berhasil → `auth/me` diulang dan alur kembali ke UF-05
   - Bila refresh gagal → token dibersihkan, error `session_expired` dilempar
4. **[lokal]** Token dan sesi lokal dihapus; state perusahaan/cabang/user direset
5. Splash hilang → user berada di `/login` **tanpa pesan error apa pun**

Tidak ada notifikasi "sesi Anda telah berakhir" — dari sisi user, aplikasi sekadar kembali ke
halaman login.

---

## UF-07 — Token access kedaluwarsa saat sedang bekerja

Alur ini **tidak terlihat oleh user** bila berhasil. Access token default hanya berumur 15 menit,
jadi alur ini terjadi rutin.

1. User melakukan aksi apa pun yang memanggil API
2. **[server]** Server menolak dengan 401 (token access kedaluwarsa)
3. **[lokal]** Klien menahan request, memanggil **satu** `auth/refresh` (bahkan bila puluhan
   request 401 datang bersamaan — semuanya menunggu antrean refresh yang sama)
4. **[server]** `POST auth/refresh` dengan refresh token tersimpan
5. Server memverifikasi token & sesi, lalu **merotasi keduanya**: access + refresh baru diterbitkan,
   hash refresh lama ditimpa, dan **masa sesi diperpanjang**
6. **[lokal]** Token baru disimpan
7. Request awal **diulang otomatis** dengan token baru → user hanya merasakan jeda singkat
8. Loader global **tidak** menampilkan langkah refresh (dianggap operasi infrastruktur)

Bila langkah 4–5 gagal: token dibersihkan, request awal gagal dengan `session_expired`, dan aksi
user berikutnya akan mengantarnya ke halaman login.

---

## UF-08 — Ganti cabang saat sedang bekerja

1. User berada di halaman mana pun (mis. `/orders/123`) dan **punya lebih dari 1 cabang**
2. User mengklik chip **"Ganti Cabang"** di topbar
3. **[lokal]** Berpindah ke `/select-branch` dengan `state.from = /orders/123`
4. Halaman "Select Branch" tampil — semua cabang ditampilkan, **termasuk cabang yang sedang
   aktif**, tanpa penanda mana yang sedang dipakai
5. User memilih cabang
6. Langkah 8–13 identik UF-02
7. User kembali ke `/orders/123`, kini dalam konteks cabang baru

Bila user hanya punya 1 cabang, chip tidak bisa diklik (berlabel "Cabang") dan alur ini tidak
tersedia dari UI.

---

## UF-09 — Ganti role aktif

Hanya tersedia bila user punya **lebih dari 1** role. Akun bawaan masing-masing hanya punya 1
role, jadi submenu ini **tidak tampil** untuk mereka. **[PERLU KONFIRMASI]** apakah ada akun nyata
di production yang memegang lebih dari satu role — bila tidak, alur ini praktis tidak terpakai.

1. User mengklik avatar di topbar → dropdown terbuka
2. User mengklik **"Ganti Role"** → chevron berputar 180°, submenu daftar role terbuka
3. Role yang sedang aktif tampil tersorot
4. User mengklik role lain
5. **[server]** `POST auth/switch-role` dengan `{ data: { role_code: "<kode>" } }`
6. Server memverifikasi user memang punya role itu → memperbarui `user_sessions.id_active_role` →
   mengembalikan daftar permission role baru
7. **[lokal]** Kode role aktif dan daftar permission di sesi lokal diperbarui
8. Toast **"Role aktif diperbarui"** / `Tampilan sekarang mengikuti akses role {kode_role}.`
9. **[lokal]** Sistem memeriksa apakah halaman saat ini masih boleh diakses role baru:
   - Bila halaman tidak butuh permission → tetap di halaman
   - Bila permission halaman ada di daftar role baru → tetap di halaman
   - Bila tidak → **otomatis pindah ke `/dashboard`**
10. Dropdown tertutup; menu sidebar langsung menyesuaikan permission role baru

Bila langkah 5 gagal: toast **"Gagal mengganti role"** / "Role ini tidak tersedia untuk akun
Anda.", dropdown **tetap terbuka**, dan tidak ada perpindahan halaman.

---

## UF-10 — Logout (desktop)

1. User mengklik avatar di topbar → dropdown terbuka
2. User mengklik **"Logout"** (item merah paling bawah)
3. **[server]** `POST auth/logout` dikirim **tanpa ditunggu** (fire-and-forget) — kegagalannya
   diabaikan
4. Server menyetel `revoked_at` pada baris sesi **perangkat ini saja**
5. **[lokal]** Token dihapus, sesi lokal dihapus, state perusahaan/cabang/user/daftar direset
6. Toast **"Logout berhasil"** / "Sesi Anda telah diakhiri."
7. **[lokal]** Halaman berpindah ke `/login`

Sesi di perangkat lain **tetap hidup**. Tidak ada opsi "logout dari semua perangkat".

---

## UF-11 — Logout (mobile, viewport ≤ 900px)

1. User mengklik tombol hamburger (`aria-label` **"Buka menu"**) di topbar
2. Sidebar mobile terbuka sebagai drawer
3. User menggulir ke bawah → blok identitas (nama + label role) dan tombol **"Logout"** merah
4. User mengklik "Logout"
5. Langkah 3–6 identik UF-10, lalu menu mobile ditutup, lalu berpindah ke `/login`

Sidebar mobile juga bisa ditutup dengan tombol **"Tutup menu"** atau tombol **Escape**.

---

## UF-12 — Mencoba membuka halaman tanpa permission

1. User (mis. role `kasir`) mengetik URL langsung, mis. `/finance/back-office`
2. **[lokal]** `RouteGuard` memeriksa berurutan: sudah login ✓ → ada cabang aktif ✓ → permission ✗
3. Berpindah ke `/403` (`replace`, tidak menumpuk riwayat)
4. Halaman **"Akses Ditolak"** tampil **di dalam shell** — sidebar & topbar tetap ada
5. User bisa: menekan **"Kembali ke Dashboard"**, memilih menu lain, atau mengganti role dari
   avatar menu

Menu yang tidak diizinkan **sudah disembunyikan** dari sidebar, jadi alur ini hanya terjadi lewat
URL langsung, bookmark, atau tautan lama.

---

## UF-13 — Akses cabang dicabut saat user sedang bekerja

1. Admin mencabut akses cabang user X (di modul Users) sementara X sedang bekerja
2. Pada request berikutnya dari X, server memverifikasi ulang akses cabang → tidak ditemukan →
   **cabang di-drop dari sesi** (`id_active_branch` dianggap `null` untuk request itu)
3. Sesi X **tidak** dimatikan — X masih terautentikasi
4. Operasi apa pun yang butuh cabang ditolak dengan **"Branch aktif belum dipilih"** (403)
5. Pada navigasi berikutnya, `RouteGuard` melihat tidak ada cabang aktif → mengalihkan X ke
   `/select-branch`
6. X memilih cabang lain yang masih boleh diakses → kembali bekerja

Bila akses **semua** cabang X dicabut, langkah 6 tidak bisa diselesaikan: halaman pemilihan cabang
kosong dan tidak ada tombol logout. Lihat [known-issues.md](../known-issues.md) KI-01.

---

## UF-14 — User dinonaktifkan saat sedang bekerja

1. Admin mengubah status user X menjadi `inactive` atau `locked`
2. Request berikutnya dari X ditolak **401** oleh verifikasi sesi
3. Klien mencoba `auth/refresh` satu kali
4. Server menolak refresh **dan** langsung menyetel `revoked_at` pada sesi X
5. Token lokal dibersihkan; aksi X gagal dengan `session_expired`
6. Navigasi berikutnya membawa X ke `/login` tanpa pesan penjelasan

---

## UF-15 — Seed / reset database dijalankan ulang

Relevan untuk lingkungan dev dan test.

1. `npm run seed` dijalankan (atau `db:reset`, atau global-setup E2E)
2. Seed menyetel `revoked_at = NOW(6)` untuk **semua sesi aktif** user 1–5
3. Semua perangkat yang sedang login sebagai akun bawaan otomatis kehilangan sesi
4. Perilaku yang dirasakan user identik UF-06: aplikasi kembali ke `/login` tanpa pesan

Seed juga menimpa `password_hash` kelima akun ke nilai bawaan (`ON DUPLICATE KEY UPDATE`) —
password yang pernah diubah manual akan kembali ke bawaan.

---

## 16. Ringkasan Matriks Redirect

Semua redirect memakai `replace` (tidak menumpuk riwayat browser).

| Kondisi user | Membuka `/login` | Membuka `/select-branch` | Membuka halaman terlindungi |
|---|---|---|---|
| Belum login | Halaman login tampil | → `/login` | → `/login` (+ simpan tujuan) |
| Login, **tanpa** cabang aktif | → `/select-branch` | Halaman pilih cabang tampil | → `/select-branch` |
| Login, ada cabang, permission cukup | → `/dashboard` | Halaman pilih cabang tampil | Halaman tampil |
| Login, ada cabang, permission kurang | → `/dashboard` | Halaman pilih cabang tampil | → `/403` |

Perhatikan: `/select-branch` **selalu** bisa diakses oleh user yang sudah login — inilah yang
memungkinkan alur "ganti cabang" (UF-08) bekerja lewat chip topbar.

### Sanitasi tujuan redirect

Nilai `state.from` dibersihkan sebelum dipakai:

| Nilai `from` | Hasil |
|---|---|
| kosong / tidak ada | `/dashboard` |
| `/` | `/dashboard` |
| `/login` | `/dashboard` |
| `/select-branch` | `/dashboard` |
| `/orders/123` | `/orders/123` |
| objek lokasi `{pathname:"/orders/123", search:"?tab=open", hash:"#items"}` | `/orders/123?tab=open#items` |
| `https://example.com` | `/dashboard` |
| `//example.com` | `/dashboard` |

`search` hanya dipakai bila diawali `?`; `hash` hanya bila diawali `#`.
