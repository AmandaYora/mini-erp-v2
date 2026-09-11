# UI/UX Spec — Modul 01 Auth & Session

**Kelompok A — kontrak tampilan yang harus SAMA PERSIS di sistem baru.** Semua teks, urutan
field, warna, dan label di bawah dikutip apa adanya dari kode.

Berkas terkait: [feature-inventory.md](feature-inventory.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md)

**Catatan bahasa:** modul ini **campur Indonesia–Inggris**, dan campurannya tidak acak — ia
konsisten per area. Halaman auth publik (`/login`, `/select-branch`) berbahasa **Inggris** untuk
judul & label form, dengan satu blok hero berbahasa **Indonesia**. Seluruh UI setelah login
(avatar menu, toast, halaman 403) berbahasa **Indonesia**. Ini disalin apa adanya, bukan
diseragamkan, kecuali Anda memutuskan sebaliknya — **[PERLU KONFIRMASI]**.

---

## 1. Token Desain yang Dipakai Modul Ini

Diambil dari `apps/web/src/tailwind.css`. Nilai heksadesimal disertakan supaya bisa direplikasi
tanpa membaca kode.

| Token | Nilai | Dipakai untuk |
|---|---|---|
| `brand` | `#1e3a5f` | Logo, tombol Sign In, gradient hero, avatar bulat |
| `brand-hover` | `#16314f` | Hover tombol Sign In |
| `ink` | `#0f172a` | Teks judul & isi input |
| `heading` | `#334155` | Teks isi kartu auth, item menu |
| `muted` | `#64748b` | Sub-teks, label kecil |
| `hairline` | `#cbd5e1` | Border kartu, border input, garis pemisah |
| `surface` | `#ffffff` | Latar kartu |
| `surface-subtle` | `#f8fafc` | Latar halaman auth, hover item menu |
| `bad` | `#b91c1c` | Teks item Logout |
| `bad-soft` | `#fee2e2` | Latar Notice error |
| `accent` | `#c2603a` | Badge "Default", badge role aktif |
| Font | `Inter`, fallback `ui-sans-serif, system-ui, -apple-system, "Segoe UI", sans-serif` | Seluruh aplikasi |

---

## 2. Screen: `/login` — Halaman Login

### 2.1 Tata letak

Dua kolom, layar penuh (`min-h-screen`), latar `surface-subtle`.

```
┌───────────────────────────────┬──────────────────────────────┐
│  HERO (aside)                 │  FORM (kartu tengah)         │
│  lg:flex-[1.2]                │  flex-1                      │
│  gradient brand → #0f172a     │  latar surface-subtle        │
│  hidden di < 1024px           │  kartu max-w 420px           │
│                               │                              │
│  "Operasional Bisnis,         │      ┌──────────────┐        │
│   Kini Lebih Mudah."          │      │ [ME]         │        │
│                               │      │ Welcome back │        │
│  paragraf deskripsi           │      │ ...form...   │        │
│                               │      └──────────────┘        │
└───────────────────────────────┴──────────────────────────────┘
```

**Responsif:** panel hero memakai `hidden … lg:flex` — **hilang total di bawah 1024px**. Di mobile
hanya kartu form yang tampil, terpusat, dengan padding 8 (32px).

### 2.2 Panel hero (kiri, desktop ≥ 1024px)

| Elemen | Isi / spesifikasi |
|---|---|
| Latar | `bg-gradient-to-br from-brand to-[#0f172a]` |
| Dekorasi | Cahaya ambient radial berputar — animasi `spin 30s linear infinite`, ukuran 200%×200%, posisi `-left-1/2 -top-1/2` |
| Aksesibilitas gerak | `motion-reduce:before:animate-none` — animasi berhenti bila user memilih "reduce motion" |
| Padding | 16 (64px) |
| Judul (h1) | **"Operasional Bisnis, Kini Lebih Mudah."** — 3rem, bold, `leading-[1.2]`, putih |
| Paragraf | **"Platform Mini ERP modern yang siap tumbuh bersama skala bisnis Anda. Kelola logistik, inventaris, dan operasional dengan presisi tinggi dan visibilitas penuh."** — 1.1rem, `leading-[1.6]`, `opacity-90` |
| Lebar maksimum konten | 480px |

### 2.3 Kartu form (kanan)

| Elemen | Spesifikasi |
|---|---|
| Kartu | Lebar penuh, `max-w-[420px]`, `rounded-xl`, border `hairline`, latar `surface`, padding 10 (40px), `shadow-lg` |
| Logo | Kotak 48×48px, `rounded-lg`, latar `brand`, teks **"ME"** putih 1.25rem bold, shadow brand. Margin bawah 6 |
| Judul (h1) | **"Welcome back"** — 1.75rem, bold, warna `ink`, margin bawah 2 |
| Sub-judul | **"Please enter your details to sign in."** — 0.95rem, `muted`, margin bawah 8 |

### 2.4 Urutan field form (WAJIB dipertahankan)

**Field 1 — Username or Email**

| Properti | Nilai |
|---|---|
| Label | **"Username or Email"** — 0.85rem, `font-semibold`, warna `ink`, margin bawah 2 |
| `id` / `htmlFor` | `identifier` |
| Tipe | `text` |
| Placeholder | **"Enter your username"** |
| `autoComplete` | `username` |
| `required` | ya (validasi native browser) |
| Style | `rounded-lg`, border `hairline`, padding `px-4 py-3`, 0.95rem; fokus → border `brand` + ring `0 0 0 3px rgba(30,58,95,0.12)` |
| Margin bawah blok | 5 |

**Field 2 — Password**

| Properti | Nilai |
|---|---|
| Label | **"Password"** — style sama seperti field 1 |
| `id` / `htmlFor` | `password` |
| Komponen | `PasswordInput` (input + tombol mata) |
| Placeholder | **"••••••••"** (8 karakter bullet U+2022) |
| `autoComplete` | `current-password` |
| `required` | ya |
| Tombol toggle | Ikon mata di kanan dalam input (`px-3`), warna `muted` → hover `ink` |
| `aria-label` toggle | **"Tampilkan password"** saat tersembunyi, **"Sembunyikan password"** saat tampil (juga sebagai `title`) |
| `aria-pressed` | mengikuti status tampil |
| Ikon | SVG 18×18, mata; garis diagonal tambahan (`M4.5 19.5 19.5 4.5`) muncul saat password **tersembunyi** |
| Padding kanan input | `pr-[46px]` agar teks tidak menabrak ikon |
| Margin bawah blok | 7 |

**Tombol submit**

| Properti | Nilai |
|---|---|
| Teks | **"Sign In"** |
| Lebar | penuh (`w-full`) |
| Style | `rounded-lg`, latar `brand`, `py-3`, 1rem, `font-semibold`, teks putih; hover → `brand-hover` |
| Margin atas | 2 |
| Tipe | `submit` (form men-submit juga lewat tombol Enter) |

### 2.5 Blok error

Muncul **di atas form**, di bawah sub-judul, hanya bila ada error. Margin bawah 6.

Komponen `Notice` dengan `tone="danger"`:

| Properti | Nilai |
|---|---|
| Judul | **"Authentication Error"** (tebal) |
| Isi | Pesan error dari hasil login |
| Style | `rounded-md`, **garis kiri 4px** warna `bad`, latar `bad-soft`, teks `bad`, padding 4, 0.9rem |
| Kelas legacy | `notice notice-danger` — dipertahankan sebagai hook selector E2E |

**Isi pesan yang benar-benar tampil** (lihat [business-rules.md](business-rules.md) BR-19 dan
[known-issues.md](../known-issues.md) KI-04):

| Penyebab | Teks yang tampil di Notice |
|---|---|
| Password salah / user tidak ada / user non-aktif | `unauthorized` |
| User tanpa role | `forbidden` |
| Server tidak merespons JSON (mis. API mati) | Pesan Indonesia sesuai status, mis. "Server sedang sibuk/tidak tersedia. Coba beberapa saat lagi." |

Error **tidak** dibersihkan otomatis saat user mengetik ulang — ia tetap tampil sampai submit
berikutnya berhasil atau menghasilkan error baru.

### 2.6 Elemen yang TIDAK ada di halaman login

Tautan "Lupa password", checkbox "Ingat saya", tautan daftar/registrasi, pemilih bahasa, tombol
login sosial, indikator kekuatan password, tautan bantuan/kontak, versi aplikasi.

---

## 3. Screen: `/select-branch` — Pemilihan Cabang

### 3.1 Tata letak

Satu kolom terpusat, layar penuh, latar `surface-subtle`, padding 10 (40px). **Tidak ada panel
hero.** Kartu memakai `max-w-[700px]` (lebih lebar dari kartu login).

### 3.2 Kepala kartu (terpusat)

| Elemen | Spesifikasi |
|---|---|
| Logo | Kotak 48×48px "ME", **terpusat** (`mx-auto`), margin bawah 4 |
| Judul (h1) | **"Select Branch"** — 1.75rem, bold, `ink`, margin bawah 2 |
| Deskripsi | 0.95rem, `muted`, `max-w-[500px]`, terpusat — teks berubah sesuai kondisi (tabel di bawah) |
| Margin bawah blok | 8 |

**Tiga varian teks deskripsi:**

| Kondisi | Teks |
|---|---|
| Ada perusahaan aktif **dan** jumlah cabang > 1 | `{Nama Perusahaan} has multiple branches available for your account. Please select one to proceed.` |
| Ada perusahaan aktif, jumlah cabang ≤ 1 | `{Nama Perusahaan} has one branch available for your account. Select it to continue.` |
| Tidak ada perusahaan aktif | **"Please select an active branch to continue."** |

Perhatikan varian kedua: teksnya berbunyi "has one branch" **juga ketika jumlah cabang nol** —
kondisinya hanya `activeCompany` tanpa memeriksa panjang daftar. Lihat
[known-issues.md](../known-issues.md) KI-01.

### 3.3 Grid kartu cabang

| Properti | Nilai |
|---|---|
| Layout | `grid`, `gap-4`, `grid-template-columns: repeat(auto-fit, minmax(280px, 1fr))` |
| Margin atas | 6 |
| Perilaku responsif | Kartu otomatis menyusut/melipat; minimum 280px per kartu |

**Struktur satu kartu cabang** (elemen `<button type="button">`, seluruh kartu bisa diklik):

```
┌────────────────────────────────────────────┐
│  Nama Cabang                    [KODE]     │  ← baris atas
│  Kota                          [Default]   │     (badge kanan, bertumpuk)
│ ────────────────────────────────────────── │  ← garis pemisah (border-t)
│  Access Profile                  [Role]    │  ← baris bawah
│  {nilai profil}                  [Role]    │
└────────────────────────────────────────────┘
```

| Elemen | Spesifikasi |
|---|---|
| Kartu | `rounded-lg`, border `hairline`, latar `surface`, padding 5, teks kiri, `gap-3`, `cursor-pointer` |
| Hover | Naik 2px (`-translate-y-0.5`), border jadi `brand`, latar `brand/[0.06]`, `shadow-md`, transisi `all` |
| Nama cabang (h2) | 1.05rem, `font-semibold`, `ink`, margin bawah 1 |
| Kota | 0.85rem, `muted` |
| Badge kode cabang | `Badge tone="info"` — latar `brand/10`, teks `brand` |
| Badge "Default" | `Badge tone="accent"` berteks **"Default"** — hanya bila cabang itu default. Diletakkan di bawah badge kode (kolom, `items-end`, `gap-1`) |
| Pemisah | `border-t border-hairline`, `pt-3`, `mt-3` |
| Label kiri bawah | **"Access Profile"** — 0.75rem, `muted`, tampil sebagai blok |
| Nilai kiri bawah | `<strong>` 0.85rem, `ink` |
| Badge role | Satu `Badge tone="neutral"` per role, teks lewat `formatRoleLabel`, rata kanan, `flex-wrap`, `gap-1` |

**Format label role** (`formatRoleLabel`):

| Kode role | Tampilan |
|---|---|
| `superadmin` | Superadmin |
| `owner` | Owner |
| `admin` | Admin |
| `staff` | Staff |
| lain (mis. `kasir`) | Title Case dari `snake_case` → "Kasir"; `kepala_gudang` → "Kepala Gudang" |
| tidak ada | `-` |

### 3.4 Elemen yang TIDAK ada di halaman pemilihan cabang

**Tidak ada tombol Logout, tidak ada tombol Kembali, tidak ada tautan keluar apa pun.** Bila
daftar cabang kosong, halaman ini menampilkan kepala kartu tanpa kartu cabang dan user tidak punya
jalan keluar dari UI. Lihat [known-issues.md](../known-issues.md) KI-01.

Juga tidak ada: pencarian/filter cabang, indikator cabang yang sedang aktif (saat dipakai untuk
ganti cabang), pengaturan "jadikan default".

---

## 4. Komponen Sesi: Avatar Menu (topbar, semua halaman)

Muncul di kanan topbar. **Tidak dirender sama sekali** bila salah satu dari `activeUser`,
`activeCompany`, atau `activeBranch` kosong.

### 4.1 Tombol pemicu

Kelas penanda: `avatar-trigger` (dipakai test E2E — jangan dihapus).

| Elemen | Spesifikasi |
|---|---|
| Bentuk | `rounded-full`, border transparan, `py-1 pl-1 pr-3`; hover → border `hairline` + latar `surface-subtle` |
| Avatar | Bulat 32×32px, latar `brand`, teks putih 0.85rem `font-semibold` |
| Isi avatar | **2 karakter pertama nama lengkap, huruf kapital** (mis. "Super Admin" → `SU`) |
| Baris 1 | Nama lengkap — 0.85rem, `font-semibold`, `ink`, `truncate` |
| Baris 2 | Label role aktif (`formatRoleLabel`) — 0.78rem, `muted`, `truncate` |

### 4.2 Dropdown

| Properti | Nilai |
|---|---|
| Posisi | Absolut, kanan, `top-[calc(100%+8px)]`, `z-[100]` |
| Lebar | 260px |
| Style | `rounded-[10px]`, border `hairline`, latar `surface`, `py-3`, `shadow-lg` |
| Animasi buka | `slideDown 0.15s ease-out` |
| Mobile (≤ 640px) | Jadi `fixed`, `right-4`, `top-[calc(64px+32px)]`, lebar `min(260px, 100vw-32px)` |
| Menutup | Klik di luar (`mousedown` di luar area menu) atau memilih item |

**Isi, dari atas ke bawah:**

1. **Blok identitas** (padding `px-4 py-2`, `gap-1`)
   - Nama lengkap dalam `<strong>` warna `ink`
   - Baris badge (`flex-wrap`, `gap-2`): `Badge tone="accent"` = label role aktif ·
     `Badge tone="neutral"` = **kode cabang aktif**

2. **Pemisah** — garis 1px `hairline`, margin vertikal 1.5 · *hanya bila blok Ganti Role tampil*

3. **"Ganti Role"** — hanya bila user punya **lebih dari 1** role
   - Tombol selebar menu, teks kiri, ikon chevron di kanan (SVG 16×16, `polyline 6 9 12 15 18 9`)
   - Chevron **berputar 180°** saat submenu terbuka (`transition-transform`)
   - Submenu (saat terbuka): kotak `mx-2`, `rounded-lg`, latar `surface-subtle`, `px-2 py-1`
   - Item role: `px-3 py-1.5`, teks `formatRoleLabel(role)`
   - Role yang sedang aktif: latar `brand/10`, `font-medium`, teks `brand`

4. **Pemisah** — garis 1px `hairline`

5. **"Profil Saya"** — item menu biasa. **Tidak melakukan apa pun selain menutup dropdown**
   (lihat [known-issues.md](../known-issues.md) KI-07)

6. **"Logout"** — item menu dengan teks warna `bad` (merah). Menjalankan logout lalu pindah ke
   `/login`

Style item menu: `px-4 py-2.5`, teks kiri 0.9rem warna `heading`; hover → latar `surface-subtle`,
teks `ink`.

---

## 5. Komponen Sesi: Chip Cabang (topbar)

Dua bentuk, dipilih otomatis:

| Kondisi | Bentuk | Label | Perilaku |
|---|---|---|---|
| Punya > 1 cabang | `<button>` bisa diklik, `cursor-pointer` | **"Ganti Cabang"** + nama cabang (tebal) | Membuka `/select-branch`, membawa halaman saat ini agar bisa dikembalikan |
| Punya 1 cabang | `<div>` statis | **"Cabang"** + nama cabang (tebal) | Tidak bisa diklik |

Style chip (keduanya): `rounded-full`, border `hairline`, latar `surface-subtle`, `px-3 py-1.5`,
`max-w-[min(280px,32vw)]` (turun ke `min(220px,38vw)` di bawah 1024px), nama cabang `truncate`.
Label 0.75rem `muted`, nilai 0.85rem `ink`.

Di sebelahnya (kiri) ada chip serupa berlabel **"Perusahaan"** + nama perusahaan, tampil bila ada
perusahaan aktif.

**Blok kiri topbar** menampilkan label kecil **"Cabang Aktif"** (0.75rem, uppercase,
`tracking-[0.05em]`, `muted`) dengan nilai di bawahnya 1rem `font-semibold` — tapi nilai itu
adalah **judul halaman**, bukan nama cabang (KI-08).

---

## 6. Komponen Sesi: Logout di Sidebar Mobile

Hanya tampil pada viewport **≤ 900px**, di bagian paling bawah sidebar (`mt-auto`), dipisahkan
garis atas `border-white/[0.08]`.

| Elemen | Spesifikasi |
|---|---|
| Nama user | `<strong>` 0.92rem, `sidebar-strong` (`#f8fafc`), `truncate` |
| Label role | 0.78rem, `sidebar-fg` (`#94a3b8`), `truncate` |
| Tombol | Kelas penanda `sidebar-mobile-logout` (hook E2E). Teks **"Logout"**, `min-h-10`, lebar penuh, `rounded-lg`, border `rgba(248,113,113,0.28)`, latar `rgba(248,113,113,0.1)`, teks `#fecaca` bold; hover → latar `rgba(248,113,113,0.16)` |
| Aksi | Logout → tutup menu mobile → pindah ke `/login` |

Tombol pembuka sidebar mobile: ikon hamburger 38×38px dengan `aria-label` **"Buka menu"**; tombol
penutupnya `aria-label` **"Tutup menu"**. Tombol **Escape** juga menutup sidebar mobile.

---

## 7. Screen Pendukung: `/403` — Akses Ditolak

| Elemen | Isi |
|---|---|
| Breadcrumb | **"Dashboard > 403"** |
| Judul | **"Akses Ditolak"** |
| Deskripsi | **"Guard frontend menyembunyikan menu tanpa permission, tetapi direct URL access tetap diarahkan ke halaman ini."** |
| Notice (danger) judul | **"Anda tidak memiliki akses ke halaman ini."** |
| Notice isi | **"Kembali ke dashboard atau ganti role aktif jika Anda memang memiliki role lain yang relevan."** |
| Tombol | **"Kembali ke Dashboard"** — tombol primary, lebar `w-fit`, menuju `/dashboard` |

Halaman ini dirender **di dalam shell aplikasi** (sidebar + topbar tetap tampil), sehingga user
masih bisa berpindah menu atau ganti role dari avatar menu.

Teks deskripsinya memakai istilah developer ("guard frontend", "direct URL access") — di luar gaya
owner-friendly bagian lain. **[PERLU KONFIRMASI]** apakah teks ini mau dipertahankan apa adanya
atau ditulis ulang untuk pengguna harian.

---

## 8. Splash: Memulihkan Sesi

Ditampilkan **menggantikan seluruh aplikasi** selama proses validasi sesi saat mount.

| Properti | Nilai |
|---|---|
| Layout | Flex terpusat, `min-height: 100vh` |
| Latar | `var(--bg-canvas, #0f172a)` — **gelap** (nilai variabel adalah `#f8fafc`, jadi hasilnya terang; fallback gelap hanya dipakai bila variabel hilang) |
| Teks | **"Memuat sesi..."** — putih, 1rem, `opacity: 0.7` |

Ditulis dengan `style` inline, bukan utility Tailwind — satu-satunya tempat di alur auth yang
begitu. Karena `--bg-canvas` bernilai terang sementara teks berwarna putih, hasil nyatanya adalah
**teks putih di latar terang** (hampir tidak terbaca). Lihat [known-issues.md](../known-issues.md)
KI-09.

---

## 9. Loader Global Selama Request

Setiap panggilan API (termasuk `auth/login`, `auth/switch-branch`, `auth/switch-role`) otomatis
menaikkan penghitung request dan menampilkan `GlobalLoader`. Pengecualian: `auth/refresh` yang
dijalankan diam-diam tanpa loader.

Tombol "Sign In" **tidak** dinonaktifkan selama proses login dan tidak berubah menjadi state
loading — user bisa menekannya berulang kali. Umpan balik satu-satunya adalah loader global.

---

## 10. Notifikasi Toast Modul Ini

Toast muncul lewat `ToastViewport`, hilang otomatis setelah **3400 ms**.

| Pemicu | Judul | Deskripsi | Tone |
|---|---|---|---|
| Logout berhasil | **"Logout berhasil"** | **"Sesi Anda telah diakhiri."** | success |
| Ganti cabang berhasil | **"Cabang aktif diperbarui"** | **"Data tiap modul akan dimuat ulang saat Anda mengunjunginya."** | success |
| Ganti cabang gagal (error server) | **"Gagal mengganti cabang"** | **"Terjadi kesalahan saat memilih cabang."** | danger |
| Ganti cabang gagal (ID tidak valid) | **"Gagal mengganti cabang"** | `ID cabang tidak valid: "{nilai}"` | danger |
| Ganti role berhasil | **"Role aktif diperbarui"** | `Tampilan sekarang mengikuti akses role {kode_role}.` | success |
| Ganti role gagal | **"Gagal mengganti role"** | **"Role ini tidak tersedia untuk akun Anda."** | danger |

Perhatikan toast ganti role memakai **kode role mentah** (mis. `kasir`), bukan label terformat
("Kasir") seperti di tempat lain. Login **tidak** memunculkan toast sukses — umpan baliknya adalah
perpindahan halaman.

---

## 11. Format & Konvensi Tampilan

| Aspek | Aturan di modul ini |
|---|---|
| Format tanggal | **Modul auth tidak menampilkan tanggal apa pun di UI.** `last_login_at`, `expires_at`, `revoked_at` tersimpan tapi tidak pernah ditampilkan |
| Locale | Seluruh formatter aplikasi memakai `id-ID` (hardcoded) — tidak relevan di modul ini karena tak ada angka/tanggal yang diformat |
| Inisial avatar | 2 karakter pertama nama lengkap, dikapitalkan |
| Kode cabang | Ditampilkan apa adanya dari data (mis. `BR-001`) |
| Password | Selalu tersembunyi secara default; toggle per-field, tidak persist antar halaman |
| Kelas CSS legacy | `avatar-trigger`, `sidebar-mobile-logout`, `sidebar-panel`, `app-sidebar-open`, `notice`, `notice-danger` — **hook selector test E2E, jangan dihapus** |
