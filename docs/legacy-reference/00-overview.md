# 00 — Overview Analisis Legacy Mini ERP

Ringkasan seluruh hasil analisis: apa yang sudah dipetakan, seberapa dalam, dan apa yang masih
menggantung. **Baca ini lebih dulu** sebelum masuk ke dokumen modul mana pun.

| Berkas | Isi |
|---|---|
| [00-module-map.md](00-module-map.md) | Peta wilayah — struktur folder, nama file, rute (tanpa analisis logic) |
| **00-overview.md** *(ini)* | Ringkasan hasil, status kelengkapan, kualitas per modul |
| [module-dependency-map.md](module-dependency-map.md) | Ketergantungan antar-modul + urutan rebuild |
| [open-questions.md](open-questions.md) | 137 tanda [PERLU KONFIRMASI], dibagi A/B/C |
| [known-issues.md](known-issues.md) | 197 perilaku aneh yang menunggu keputusan perbaiki/replikasi |
| [PROGRESS.md](PROGRESS.md) | Checklist per modul + temuan menonjol per modul |
| [shared/](shared/) | Lapisan lintas modul (services, data model, business rules) |

---

## 1. Angka Ringkas

| Aspek | Nilai |
|---|---|
| Modul dianalisis | **21 dari 21** (100%) |
| Dokumen dihasilkan | **189 berkas** (21 modul × 9) + 3 shared + 5 lintas-modul |
| Total baris dokumen | ~34.400 (terukur; baseline awal dilaporkan ~26.100) |
| Known issue terkumpul | **197** (KI-01…KI-197; KI-198 tak terpakai; 1 sudah tertutup: KI-120) |
| Open question terkumpul | **137** → 48 untuk Anda, 32 gap analisis (19 tertutup), 38 kosmetik |
| Endpoint tercakup | ~220 (seluruh API) |
| Rute web tercakup | 79 |
| Tabel DB tercakup | ~70 entity |

---

## 2. Status Kelengkapan per Modul

Legenda: ✅ selesai & terverifikasi · 🔵 selesai, menunggu review · ⚠️ ada gap yang saya akui

| # | Modul | Status | Dok | Baris dok | Kode API | Kedalaman | KI | OQ (A/B/C) |
|---|---|---|---|---|---|---|---|---|
| 01 | Auth & Session | ✅ | 9/9 | 2.660 | 519 | **5,1×** | 12 | 6/3/8 |
| 02 | Users, Roles & Permissions | ✅ | 9/9 | 3.560 | 886 | **4,0×** | 15 | 5/2/2 |
| 03 | Company & Settings | ✅ | 9/9 | 3.146 | 365 | **8,6×** | 8 | 5/2/3 |
| 04 | Branch (Multi-Cabang) | ✅ | 9/9 | 2.112 | 397 | **5,3×** | 14 | 5/0/4 |
| 05 | Product / Catalog | ✅ | 9/9 | 1.174 | 2.891 | 0,41× | 11 | 6/4/3 |
| 06 | Business Party | ✅ | 9/9 | 829 | 1.271 | 0,65× | 10 | 2/1/1 |
| 07 | Member Type & Pricing | ✅ | 9/9 | 605 | *(sub 06)* | — | 6 | 2/0/3 |
| 08 | Order — Purchasing | ✅ | 9/9 | 824 | *(order 9.057)* | 0,41× | 8 | 2/2/1 |
| 09 | Order — Sales | ✅ | 9/9 | 634 | *(idem)* | — | 3 | 0/0/1 |
| 10 | Goods Receipt | ✅ | 9/9 | 479 | *(idem)* | — | 5 | 1/0/1 |
| 11 | Delivery | ✅ | 9/9 | 633 | *(idem)* | — | 5 | 1/0/1 |
| 12 | Sales Return | ✅ | 9/9 | 590 | *(idem)* | — | 3 | 1/1/1 |
| 13 | Purchase Return | ✅ | 9/9 | 521 | *(idem)* | — | 3 | 0/0/2 |
| 14 | Payment | ✅ | 9/9 | 520 | 1.056 | 0,49× | 3 | 1/0/2 |
| 15 | POS / Kasir | ✅ | 9/9 | 533 | *(tanpa backend)* | — | 4 | 2/0/0 |
| 16 | Stock & Gudang | ✅ | 9/9 | 970 | 3.836 | 0,25× | **19** | 0/1/1 |
| 17 | Finance & Pajak | ✅ | 9/9 | **2.674** | 9.066 | **0,29×** | **14** | 9/0/2 |
| 18 | Dashboard | ✅ | 9/9 | 825 | 1.118 | 0,74× | **11** | 0/1/0 |
| 19 | Reporting | ✅ | 9/9 | 614 | 283 | 2,2× | **10** | 0/3/0 |
| 20 | Audit Log | ✅ | 9/9 | 577 | 134 | 4,3× | **11** | 1/2/1 |
| 21 | Assistant + WA + Knowledge | ✅ | 9/9 | 1.300 | 3.241 | 0,40× | **22** | 0/1/0 |
| — | shared/ | ✅ | 3/3 | 2.539 | — | — | — | 2/4/1 |

**Kolom "Kedalaman"** = baris dokumen ÷ baris kode API. Bukan ukuran mutlak kualitas, tapi
penanda kasar seberapa detail sebuah modul ditulis.

---

## 3. Yang Perlu Anda Ketahui: Kedalaman Analisis Tidak Merata

Ini temuan paling penting dari peninjauan ulang, dan saya sampaikan apa adanya.

**Empat modul pertama (01–04) ditulis jauh lebih dalam daripada tujuh belas sisanya.**

| Kelompok | Rata-rata baris dok | Rata-rata kedalaman |
|---|---|---|
| Modul 01–04 (fondasi) | 2.870 | **5,8×** |
| **17 Finance** *(sudah diperdalam)* | **2.674** | **0,29×** |
| **16, 18, 19, 20, 21** *(diperdalam dari kode)* | 857 | 1,6× |
| Modul 05–15 | 600 | **0,4×** |

Dokumen 01–04 dan 17 memuat teks tombol, urutan field, isi placeholder, dan kalimat toast persis.
Modul 05–15 sebagian besar berbentuk tabel padat bersingkatan (`?`, `—`, `cek!`) — cukup untuk memahami
*apa* yang ada, tetapi **belum tentu cukup untuk membangun ulang tampilan yang sama persis**.
Modul 16/18/19/20/21 sudah diperdalam dari kode (kontrak endpoint penuh, rumus, teks UI, TC).

### ✅ Modul 17 Finance — pendalaman SELESAI

Dibaca menyeluruh: 6 controller, 8 service (14.068 baris termasuk spec), 16 entity, 8 migrasi,
16 halaman web, 164 test unit.

| Sebelum | Sesudah |
|---|---|
| 973 baris dokumen | **2.674 baris** |
| 19 gap "belum dibaca" | **0 — semua tertutup** |
| 8 known-issue | **14** (KI-144…KI-149 baru) |
| 2 pertanyaan pemilik | **9** (OQ-A42…OQ-A48 baru) |
| "63 endpoint" *(perkiraan)* | **64 endpoint** *(terhitung)* |

**Yang paling penting ditemukan:** paket berkas pajak punya **dua versi** — *"Data riil (apa
adanya)"* dan *"Versi dibatasi Rp4,8 M"*. Versi kedua membuang order penjualan sampai omzet
kumulatif berada di bawah ambang PP-23. Ini keputusan bisnis yang harus Anda konfirmasi sebelum
fase desain → **OQ-A42**.

Temuan lain: dua laporan PPN memakai rumus DPP berbeda (KI-145), ekspor tanpa batas rentang tanggal
pada VPS 2 GB (KI-144), tipe akun bebas diubah meski sudah berjurnal (KI-147), dan unduhan berkas
pajak tanpa jejak audit (KI-149 — enam baris sensitif lain yang dulu dilaporkan ternyata SUDAH
teraudit, terverifikasi dari kode).

### Modul 16, 18, 19, 20, 21 — pendalaman SELESAI

Diperdalam langsung dari kode (controller, service, entity, halaman web, spec, E2E):

| Modul | Sebelum | Sesudah |
|---|---|---|
| 16 Stock | 500 baris, 4 KI | **970 baris**, 25/25 endpoint terkontrak, **KI-150…KI-164 baru** (15) |
| 18 Dashboard | 411 baris, 4 KI | **825 baris**, tiap metrik + rumus + teks UI, **KI-179…KI-185 baru** (7) |
| 19 Reporting | 388 baris, 4 KI | **614 baris**, cron + agregat + presisi, **KI-186…KI-191 baru** (6) |
| 20 Audit Log | 355 baris, 5 KI | **577 baris**, enumerasi 102 situs / 103 actionKey runtime, **KI-192…KI-197 baru** (6) |
| 21 Assistant/WA/Knowledge | 473 baris, 8 KI | **1.300 baris**, 18/18 endpoint + 10 tool + pipeline RAG, **KI-165…KI-178 baru** (14) |

Koreksi temuan lama dari verifikasi kode: headline "26 endpoint" Stock → **25** (enumerasinya
memang 25); KI-149 dipersempit (hanya unduh paket pajak yang tak teraudit).

### Tidak ada modul tipis tersisa

Tabel "dua modul yang perlu diperdalam" di versi sebelumnya sudah kedaluwarsa — 16 dan 21
termasuk yang diselesaikan di atas. Urutan desain tetap: modul 16 menjadi acuan sebelum
merancang Stok/Finance di sistem baru; modul 21 boleh paling akhir karena tidak ada modul
lain yang bergantung padanya.

---

## 4. Verifikasi Cakupan — Tidak Ada yang Terlewat

Diperiksa ulang terhadap [00-module-map.md](00-module-map.md):

| Yang dicek | Hasil |
|---|---|
| 18 folder module API | ✅ Semua tercakup. `storage/` masuk lapisan shared (S1); `tools/` masuk modul 21 (F-02) |
| 17 folder module web | ✅ Semua tercakup. `core/` & `shared/` masuk lapisan shared |
| 21 nomor modul di peta | ✅ Semua punya folder dokumen lengkap 9/9 |
| `android-pos-shell` (Kotlin) | ✅ Tercakup di modul 15 (`feature-inventory.md` §Perangkat) |
| `packages/shared-*` | ✅ Tercakup di `shared/shared-data-model.md` §4–§5 |
| 27 spec E2E | ✅ Dirujuk lintas modul |
| 13 area shared (S1–S13) | 11 ✅ · 2 sebagian (lihat bawah) |

### Dua sisa kecil di lapisan shared

| Area | Keadaan |
|---|---|
| **S10 Cetak dokumen** | ✅ **Sudah tertutup** — komponen cetak per-dokumen akhirnya terdokumentasi di modul 09, 11, 14, dan 15 |
| **S11 PWA & perangkat** | 🔵 **Sebagian.** Bagian printer sudah dalam di modul 15 (ESC/POS, laci, shell Android). Yang belum dibaca detail: isi `pwa-registration.ts` dan `chunk-load-recovery.ts` — keduanya menyentuh perilaku saat deploy baru |

Tidak ada modul atau berkas dari mapping awal yang terlewat.

---

## 5. Known Issues — Sebaran

197 temuan, **sudah diputuskan analis** (delegasi pemilik): **165 perbaiki** / **3 replikasi** /
**29 sebagian** + KI-120 yang sudah tertutup. Nol ⬜ tersisa — override per item bila tidak setuju.

| Modul | KI | Modul | KI |
|---|---|---|---|
| 01 Auth | KI-01…12 (12) | 12 Sales Return | KI-98…100 (3) |
| 02 Users/Roles | KI-13…27 (15) | 13 Purchase Return | KI-101…103 (3) |
| 03 Company | KI-28…35 (8) | 14 Payment | KI-104…106 (3) |
| 04 Branch | KI-36…49 (14) | 15 POS | KI-107…110 (4) |
| 05 Product | KI-50…60 (11) | 16 Stock | KI-111…114 + KI-150…164 (**19**) |
| 06 Business Party | KI-61…70 (10) | 17 Finance | **KI-115…122 + KI-144…149 (14)** |
| 07 Member Pricing | KI-71…76 (6) | 19 Reporting | KI-123…126 + KI-186…191 (**10**) |
| 08 Order-Purchasing | KI-77…84 (8) | 20 Audit Log | KI-127…131 + KI-192…197 (**11**) |
| 09 Order-Sales | KI-85…87 (3) | 18 Dashboard | KI-132…135 + KI-179…185 (**11**) |
| 10 Goods Receipt | KI-88…92 (5) | 21 Assistant/WA | KI-136…143 + KI-165…178 (**22**) |
| 11 Delivery | KI-93…97 (5) | | |

**Empat modul menyumbang ~37% temuan**: 21 (Assistant/WA, 22), 16 (Stock, 19), 02 (Users/Roles, 15),
04 (Branch, 14) — sebagian karena memang banyak perilaku setengah jadi, sebagian karena
ditulis paling dalam.

---

## 6. Temuan Lintas Modul yang Memengaruhi Arsitektur

Diangkat dari lapisan shared dan berulang di banyak modul:

| # | Temuan | Dampak untuk rebuild |
|---|---|---|
| 1 | **Validasi payload praktis absen** — `class-validator` dipakai di 1 file saja; ~214 endpoint tanpa validasi runtime | Sistem baru butuh lapisan validasi menyeluruh sejak awal |
| 2 | **Tidak ada base entity** — konvensi kolom diulang manual di 70 entity | Standarkan sekali di lapisan dasar |
| 3 | **Migration runner tanpa tabel pelacak** — 50 migrasi dijalankan ulang tiap start; aman hanya karena 6 errno diabaikan | Sistem baru butuh pelacak migrasi yang benar |
| 4 | **Dua standar pembulatan uang hidup bersamaan** — rupiah utuh vs 2 desimal | Pilih satu, tegakkan di lapisan dasar |
| 5 | **`PermissionGuard` gagal-terbuka** — endpoint tanpa `@RequirePermission` lolos otomatis | Balik jadi gagal-tertutup |
| 6 | **Audit log di luar transaksi pemanggil** — baris audit tetap tertulis meski transaksi dibatalkan | Keputusan sadar: masuk transaksi, atau tetap di luar? |
| 7 | **Zona waktu belum konsisten** — finance sudah WIB-aware, penomoran order masih waktu lokal server | Seragamkan ke Asia/Jakarta |
| 8 | **Isolasi tenant bergantung disiplin per-service** — tanpa row-level security | Pertimbangkan penyaring global |
| 9 | **Enam aturan paling kritis ditegakkan manual**; `arch:check` tidak memeriksa satu pun | Otomatiskan di sistem baru |
| 10 | **Pesan galat server dibuang di antarmuka** — berulang di modul 01, 02, 03, 04 | Satu pola penanganan galat untuk semua |

---

## 7. Pola Berulang yang Layak Ditiru (bukan diperbaiki)

Tidak semua di sistem lama perlu diganti. Yang ini sudah benar dan sebaiknya dipertahankan:

| Pola | Asal | Kenapa |
|---|---|---|
| **Provisioning gagal-bersama** — cabang lahir lengkap dengan gudang + 5 penghitung nomor dalam satu transaksi | 04 Branch | Tidak ada entitas setengah jadi |
| **"Pastikan ada", bukan "buat ulang"** — idempoten, melengkapi yang hilang tanpa mereset | 04 Branch | Fitur baru otomatis menyusul ke data lama |
| **Impor batch resumable** — baris gagal tidak membatalkan baris lain, tertelusur per sheet/baris/kolom | 05 Product | Migrasi data besar jadi manusiawi |
| **Pesan galat berbahasa Indonesia yang menyebut nilai aslinya** | 05 Product (impor) | Pengguna bisa memperbaiki sendiri |
| **Dua sisi satu angka** — "isi jual per stok" ↔ faktor teknis, tersinkron otomatis | 05 Product | Pengguna awam tidak perlu paham rumus |
| **Audit lengkap sebelum & sesudah** | 03 Company | Jadikan standar; bandingkan `product.update` yang hanya mencatat nama |
| **Mode baca-saja tiga lapis** — kendali nonaktif + teks penjelas + submit diabaikan | 03 Company | Pola terbaik di sistem |
| **Snapshot dokumen** — nota historis tidak bergeser saat master data berubah | 08/09 Order | Wajib dipertahankan |
| **Guard integritas lintas modul** — kunci satuan setelah ada mutasi, stok wajib nol sebelum arsip | 05 Product | Penjaga angka historis |

---

## 8. Kesiapan Masuk Fase Desain

| Prasyarat | Status |
|---|---|
| Seluruh modul terpetakan | ✅ 21/21 |
| Kontrak lintas modul teridentifikasi | ✅ 12 kontrak, lihat [module-dependency-map.md](module-dependency-map.md) §6 |
| Urutan rebuild ditentukan | ✅ 11 tahap, lihat dependency map §9 |
| Known issue terkumpul | ✅ 197, menunggu keputusan Anda |
| Open question terkumpul | ✅ 130, **12 di antaranya memblokir** |
| Kedalaman analisis merata | ✅ — modul 16/17/18/19/20/21 sudah diperdalam dari kode; 05–15 padat-tabel |
| Presisi kolom DB terverifikasi | ⚠️ **Belum** — butuh `SHOW COLUMNS` di production (7 gap sekaligus) |

**Dua hal yang saya sarankan dituntaskan sebelum desain dimulai:**

1. **Jawab 15 open question P0** di [open-questions.md](open-questions.md) §"Yang Saya Sarankan
   Anda Jawab Lebih Dulu" — termasuk tiga yang baru dari pendalaman Finance, dan yang paling
   mendesak: **OQ-A42** (dua versi paket berkas pajak).
2. ~~**Perdalam modul 16 Stock.**~~ ✅ SELESAI (970 baris + KI-150…KI-164). Modul 21 boleh paling akhir.

Sementara itu, keputusan atas 197 known issue sudah diputuskan analis — tidak perlu selesai
sebelum desain modul 01–04 dimulai; override per item bila tidak setuju.
