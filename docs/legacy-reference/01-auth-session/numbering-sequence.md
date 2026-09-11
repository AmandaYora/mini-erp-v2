# Numbering & Sequence — Modul 01 Auth & Session

**Kelompok A.** Dokumen ini mencatat format penomoran dokumen yang dipakai modul ini.

---

## 1. Kesimpulan: Modul Ini Tidak Memiliki Penomoran Dokumen

**Modul Auth & Session tidak menghasilkan dokumen bisnis, sehingga tidak ada format penomoran
yang perlu direplikasi.** Diverifikasi:

| Yang diperiksa | Hasil |
|---|---|
| Pemakaian `BranchDocumentSequence` | Tidak ada di modul auth (dipakai `order`, `delivery`, `payment`, `sales-return`, `purchase-return`, `stock-transfer`) |
| Pemakaian `FinanceDocumentSequence` | Tidak ada (khusus modul finance) |
| Kunci sequence terkait auth | Tidak ada. Kunci yang ada: `order`, `order_<kind>_<YYYY>-<MM>`, `payment`, `sj`, `sales_return`, `purchase_return`, `stock_transfer` |
| Kolom bernomor yang tampil ke user | Tidak ada |

Tidak ada nomor sesi, nomor login, kode referensi, atau identitas apa pun dari modul ini yang
pernah ditampilkan kepada user.

---

## 2. Identitas Teknis yang Dipakai Modul Ini

Ketiga hal berikut adalah identitas **internal** — tidak pernah tampil di UI, tidak dipakai
sebagai referensi bisnis, dan tidak perlu mempertahankan formatnya di sistem baru.

### 2.1 `user_sessions.id_user_session`

`INT AUTO_INCREMENT`. Tanpa prefix, tanpa reset periodik, tanpa format.

Dipakai sebagai klaim `sessionId` di dalam token, sehingga nilainya **terlihat oleh pemegang
token** (payload JWT hanya di-encode base64, bukan dienkripsi). Konsekuensi yang perlu
diperhatikan saat merancang sistem baru: nomor berurutan seperti ini membocorkan estimasi jumlah
login yang pernah terjadi di sistem, dan memudahkan menebak `sessionId` milik orang lain. Yang
menahan penyalahgunaannya adalah verifikasi ganda `id` + `id_user` (lihat
[business-rules.md](business-rules.md) BR-08 syarat 2) dan tanda tangan JWT.

**[PERLU KONFIRMASI]** apakah identitas sesi di sistem baru sebaiknya memakai nilai acak
(UUID/ULID) alih-alih auto-increment. Ini bukan perubahan perilaku yang terlihat user.

### 2.2 Struktur token

Bukan penomoran, tapi dicatat di sini karena satu-satunya "format" yang dihasilkan modul ini.

| Token | Isi klaim | Masa bawaan |
|---|---|---|
| Access | `{ sub, sessionId, companyId }` | 15 menit |
| Refresh | `{ sub, sessionId, type: 'refresh' }` | 7 hari |

Keduanya JWT HS256 bertanda tangan `JWT_SECRET`. Tidak ada prefix, versi, atau penanda lingkungan
di dalam token.

### 2.3 Kunci penyimpanan di browser

Bukan penomoran, tapi bagian dari kontrak teknis yang perlu diketahui saat migrasi:

| Kunci | Isi |
|---|---|
| `mini-erp-access` | access token |
| `mini-erp-refresh` | refresh token |
| `mini-erp-session` | ringkasan sesi (JSON) |
| `mini-erp.sidebar.collapsed` | preferensi UI (bukan milik modul auth, tapi ikut terdampak logout) |

Perhatikan konvensi penamaannya **tidak seragam**: tiga kunci pertama memakai tanda hubung,
yang keempat memakai titik. Bila sistem baru memakai kunci berbeda, user yang sedang login akan
otomatis ter-logout satu kali saat rilis (tidak ada migrasi kunci `localStorage`) — perlu
diantisipasi di rencana rilis, bukan diperbaiki di kode.
