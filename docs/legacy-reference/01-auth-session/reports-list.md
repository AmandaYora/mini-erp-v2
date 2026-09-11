# Reports List — Modul 01 Auth & Session

**Kelompok A.** Dokumen ini mencatat laporan yang dihasilkan modul ini beserta logika
perhitungannya.

---

## 1. Kesimpulan: Modul Ini Tidak Menghasilkan Laporan

**Modul Auth & Session tidak memiliki laporan sama sekali.** Ini diverifikasi, bukan diasumsikan:

| Yang diperiksa | Hasil |
|---|---|
| Endpoint modul auth | 6, semuanya operasional (login/refresh/logout/me/switch-role/switch-branch) — tidak ada yang mengembalikan agregat, rekap, atau daftar berpaginasi |
| Halaman frontend | 2 (`/login`, `/select-branch`) — keduanya form/pemilihan, bukan tampilan laporan |
| Permission bertipe laporan | Tidak ada. Modul auth tidak memakai `@RequirePermission` sama sekali |
| Menu di `module-registry.tsx` | Kedua rute auth tidak punya entri `menu` — tidak muncul di sidebar, termasuk grup "Pantauan" |
| Endpoint ekspor | Tidak ada (bandingkan `orders/export`, `finance/export/tax-package` di modul lain) |
| Halaman cetak | Tidak ada |

Tidak ada perhitungan untuk didokumentasikan — modul ini tidak memiliki formula bisnis apa pun
(lihat [business-rules.md](business-rules.md) §8).

---

## 2. Data Sesi Tidak Terekspos ke Mana Pun

Temuan yang perlu diketahui sebelum merancang sistem baru: **tabel `user_sessions` tidak pernah
dibaca untuk ditampilkan kepada siapa pun.**

Penelusuran seluruh pembaca tabel itu:

| Pembaca | Tujuan |
|---|---|
| `AuthService.login` | Membuat baris sesi |
| `AuthService.refresh` | Memvalidasi & merotasi |
| `AuthService.logout` | Menyetel `revoked_at` |
| `AuthService.switchRole` / `switchBranch` | Memperbarui kolom sesi |
| `JwtStrategy.validate` | Memverifikasi sesi per request |
| `seed-runner.ts` | Mencabut sesi saat seed dijalankan |

Semuanya operasional. **Tidak ada** endpoint atau halaman yang menampilkan daftar sesi, riwayat
login, atau perangkat aktif.

Konsekuensinya, pertanyaan operasional berikut **tidak bisa dijawab** oleh sistem saat ini:

- Siapa yang sedang login sekarang, dari perangkat/IP mana?
- Kapan seorang user terakhir login? (`users.last_login_at` tersimpan tapi tidak ditampilkan di
  UI mana pun — **[PERLU KONFIRMASI]** apakah modul Users menampilkannya; ditelusuri saat analisis
  modul 02)
- Berapa kali sebuah akun gagal login? → **mustahil**, percobaan gagal tidak dicatat sama sekali
- Cabang atau role apa yang aktif saat sebuah transaksi dibuat? → tidak dari data sesi; kolomnya
  ditimpa setiap kali berubah

---

## 3. Bahan Mentah yang Tersedia (bila laporan sesi ingin dibuat)

Dicatat sebagai inventaris data, **bukan** sebagai usulan fitur. Jangan diimplementasikan tanpa
permintaan eksplisit.

| Data yang tersedia | Kolom | Batasan |
|---|---|---|
| Waktu login | `user_sessions.created_at` | Presisi `DATETIME(3)` |
| Perangkat | `user_sessions.user_agent` | String User-Agent mentah, tanpa parsing |
| Alamat IP | `user_sessions.ip_address` | `VARCHAR(64)`, dari `req.ip` (nilainya bergantung konfigurasi proxy — **[PERLU KONFIRMASI]** apakah `trust proxy` di-set, sebab tanpa itu IP yang tercatat adalah IP proxy, bukan IP pengguna) |
| Waktu berakhir/dicabut | `user_sessions.revoked_at`, `expires_at` | `revoked_at` terisi baik oleh logout maupun pencabutan otomatis — **keduanya tidak bisa dibedakan** |
| Login terakhir per user | `users.last_login_at` | Hanya nilai terakhir; riwayat tidak disimpan |
| Cabang & role aktif terakhir | `user_sessions.id_active_branch`, `id_active_role` | Nilai terkini saja; nilai lama hilang saat diganti |

Yang **tidak** tersedia dan tidak bisa direkonstruksi dari data yang ada: percobaan login gagal,
riwayat ganti role, riwayat ganti cabang, waktu refresh token, alasan pencabutan sesi.

---

## 4. Modul Lain yang Menampilkan Data Bernuansa Sesi

Agar tidak salah kira bahwa fitur berikut milik modul auth:

| Fitur | Modul pemilik |
|---|---|
| Halaman **Audit Log** (`/audit-logs`) | Modul 20 Observability — mencatat aksi bisnis (`order.create`, dst), **bukan** peristiwa login/logout |
| Daftar & pengelolaan user | Modul 02 Users, Roles & Permissions |
| Halaman **Role & Akses** (`/settings/roles`) | Modul 02 |
| Pengaturan akses cabang per user | Modul 02 (`user_branch_accesses` ditulis di sana) |

Halaman Audit Log **tidak** memuat peristiwa autentikasi, karena modul auth memang EXEMPT dari
audit log (lihat [business-rules.md](business-rules.md) BR-30).
