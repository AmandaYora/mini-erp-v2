# PRD — Mini ERP (Revamp)

Status: **terimplementasi penuh** — 20 modul backend + frontend + POS + bot WA
(skop L9: tanpa Knowledge/RAG/AI). Dokumen ini adalah catatan tujuan, cakupan,
dan peta modul produk awal.

## 1. Latar Belakang & Tujuan Revamp

Mini ERP versi lama (`../mini-erp/`, NestJS + TypeORM + MySQL) sudah menjalankan operasional
nyata satu perusahaan multi-cabang: pembelian, stok/gudang, penjualan & POS, pengiriman,
pembayaran, keuangan/pajak, dan laporan. Setelah dianalisis menyeluruh
([docs/legacy-reference/](legacy-reference/) — 21 modul, ~34.400 baris dokumentasi, 197 known
issue, 130 pertanyaan terbuka), tiga alasan konkret mendasari revamp ini:

1. **Satu modul backend menaungi enam domain sekaligus** (`order/`, ~9.057 baris) — pembelian,
   penjualan, penerimaan barang, pengiriman, retur jual, retur beli semua bercampur tanpa batas
   yang jelas.
2. **Ketergantungan data tak terlihat dari kode** — modul Finance secara nyata membaca 12 tabel
   milik 8 modul lain lewat SQL mentah, padahal dari `import` ia tampak mandiri. Perubahan bentuk
   tabel di modul manapun bisa merusak laporan keuangan secara diam-diam.
3. **Konvensi dasar tidak ditegakkan sistem** — validasi payload nyaris tidak ada (~214 dari 220
   endpoint), tidak ada base entity, migration runner tanpa tabel pelacak, dua standar pembulatan
   uang hidup bersamaan, dan guard permission gagal-terbuka pada endpoint tanpa anotasi.

Tujuan revamp: membangun ulang **fitur inti yang sama** di atas **arsitektur yang bisa dirawat** —
Go modular monolith dengan batas antar-modul yang ditegakkan compiler (`contracts/`-only), MySQL
dengan migrasi terlacak, dan konvensi yang seragam sejak modul pertama — bukan menambah fitur baru
di luar yang sudah terbukti dibutuhkan.

## 2. Pengguna & Konteks Operasional

- **Model tenant:** standalone single-tenant — satu instalasi dimiliki satu perusahaan,
  multi-cabang. Seluruh mesin multi-tenant dikeluarkan (lihat ADR-0009).
- Setiap sesi terikat pada `branch` aktif (+ `user`/role).
- **Peran pengguna:** owner, admin, kasir, staf gudang/gudang, staf keuangan — persis peran yang
  sudah berjalan di sistem lama (lihat `knowledge/DOMAIN_GLOSSARY.md` dan
  [legacy-reference/02-users-roles-permissions/](legacy-reference/02-users-roles-permissions/)).
- **Bahasa & locale:** Indonesia, `Asia/Jakarta`, Rupiah — dikunci sebagai konstanta aplikasi,
  **tidak** dijadikan setelan yang bisa diubah pengguna (lihat
  [ADR-0006](../knowledge/decisions/ADR-0006-locale-timezone-locked.md) — keputusan ini sudah
  diambil sebelumnya di analisis legacy, KI-28).

## 3. Cakupan Produk (Peta Modul Bisnis)

Cakupan fitur mengikuti 21 modul bisnis yang sudah dipetakan di sistem lama, dikelompokkan ulang
menjadi **20 modul backend** dengan batas yang lebih jelas (detail teknis di
[SYSTEM_DESIGN.md §3](SYSTEM_DESIGN.md#3-peta-modul-backend)):

| Kelompok | Modul produk | Modul lama yang dipetakan |
|---|---|---|
| Identitas & Organisasi | Auth, User (Roles/Permissions), Company, Branch | 01, 02, 03, 04 |
| Master Data | Product (Catalog), Party (Customer/Supplier + Member Pricing) | 05, 06, 07 |
| Inventaris | Stock (lokasi, saldo, mutasi, transfer, opname) | 16 |
| Transaksi Inti | Purchasing (PO), Sales (SO, termasuk kanal POS) | 08, 09, 15 |
| Turunan Transaksi | Goods Receipt, Delivery, Payment | 10, 11, 14 |
| Retur | Sales Return, Purchase Return | 12, 13 |
| Keuangan | Finance (COA, jurnal, HPP, laporan, pajak, tutup periode) | 17 |
| Observabilitas | Dashboard, Reporting, Audit Log | 18, 19, 20 |
| *(fase lanjut, opsional)* | Assistant AI + WhatsApp + Knowledge/RAG | 21 |

**POS** tidak menjadi modul backend tersendiri (mengikuti pola sistem lama) — ia adalah kanal
frontend yang merakit Product, Party, Stock, Sales, dan Payment lewat kontrak modul yang sama.

**Modul 21 (Assistant/WhatsApp/Knowledge)** sengaja **tidak** masuk cakupan tahap awal. Modul ini
paling kompleks secara integrasi (gateway WhatsApp/Baileys, pipeline RAG) dan di analisis legacy
sendiri paling tipis didokumentasikan. Keputusan membawanya ke sistem baru — dan kapan — adalah
keputusan bisnis terpisah, bukan bagian dari revamp inti operasional.

## 4. Prinsip Cakupan: Replikasi Perilaku, Bukan Replikasi Cacat

Default untuk setiap modul: **replikasi fitur + alur bisnis + tampilan** dari sistem lama, karena
itulah yang sudah dipahami dan dipakai pengguna setiap hari. **Bukan default**: mewarisi bug,
jalan buntu, atau validasi yang hilang hanya karena "begitu sistem lama bekerja".

Aturan keputusan per modul, saat modul itu didesain:

1. Jika perilaku tercatat sebagai **pola yang layak ditiru** (lihat
   [legacy-reference/00-overview.md §7](legacy-reference/00-overview.md)) → pertahankan.
2. Jika tercatat sebagai **known issue** ([legacy-reference/known-issues.md](legacy-reference/known-issues.md))
   → butuh keputusan eksplisit: perbaiki atau replikasi. Jangan diam-diam mewarisi.
3. Jika tercatat sebagai **open question kelompok A** ([legacy-reference/open-questions.md](legacy-reference/open-questions.md))
   → ini pertanyaan bisnis yang hanya pemilik sistem bisa jawab. Jangan ditebak saat desain modul.

## 5. Keputusan Bisnis yang Perlu Dikonfirmasi Sebelum/Selagi Desain Modul

Kelompok A dari analisis legacy berisi 48 pertanyaan; berikut **15 yang memblokir desain** (P0),
disarikan dari [legacy-reference/open-questions.md](legacy-reference/open-questions.md). Ini
**bukan** untuk dijawab sekarang di tahap setup — dicatat di sini supaya tidak terlewat saat modul
terkait mulai didesain.

| # | Modul terdampak | Pertanyaan ringkas |
|---|---|---|
| 1 | Auth | CORS dibatasi di reverse proxy? Sesi perlu batas umur mutlak? |
| 2 | Company | Sistem feature flag dibawa atau dibuang? (disarankan: **dibuang** — mati di 3 lapisan di sistem lama) |
| 3 | Branch | Nomor dokumen (SJ/retur/pembayaran) reset tiap tahun? Kode cabang dikunci setelah ada dokumen terbit? |
| 4 | Product | Harga beli wajib diisi juga saat *update* (bukan cuma saat create)? Upload bulk boleh melewati validasi form? |
| 5 | Purchasing/Sales | Filter tanggal akhir-hari (`date_to`) diperbaiki agar tidak memotong data hari terakhir? |
| 6 | Sales/POS | Keranjang POS perlu bertahan (draft) saat halaman refresh? |
| 7 | Finance | Paket ekspor pajak versi "dibatasi Rp4,8 M" dibawa ke sistem baru? (**konsultasikan ke konsultan pajak** — bukan keputusan teknis) |
| 8 | Finance | DPP mana yang benar saat tarif pajak order kosong (nol vs subtotal)? |
| 9 | Finance | Berapa rentang tanggal terpanjang yang benar-benar dibutuhkan untuk sekali unduh berkas pajak? |
| 10 | Lintas UI | Bahasa antarmuka: Indonesia sepenuhnya (disarankan), atau campuran seperti sekarang? |
| 11 | Lintas UI | Umpan balik toast/pesan galat diseragamkan (selalu tampilkan pesan asli server)? |
| 12 | Lintas UI | Label & format tanggal diseragamkan? |

Daftar lengkap 48 pertanyaan Kelompok A (P0–P2) ada di
[legacy-reference/open-questions.md](legacy-reference/open-questions.md).

## 6. Perbaikan Arsitektur yang Wajib Diadopsi (bukan opsional)

Sepuluh temuan lintas-modul dari analisis legacy (
[legacy-reference/00-overview.md §6](legacy-reference/00-overview.md)) langsung membentuk
persyaratan non-fungsional sistem baru:

1. **Validasi payload di setiap endpoint** sejak modul pertama (bukan ditambahkan belakangan).
2. **Base model/kolom seragam** (id, created_at, updated_at, created_by, dst.) — sekali distandarkan
   di `shared/`, dipakai semua modul.
3. **Migration runner dengan tabel pelacak** (`golang-migrate` sudah menjamin ini — lihat
   [SYSTEM_DESIGN.md](SYSTEM_DESIGN.md)).
4. **Satu standar pembulatan uang**: rupiah bulat (integer), tanpa desimal — lihat
   [ADR-0005](../knowledge/decisions/ADR-0005-money-as-integer-rupiah.md).
5. **Permission guard gagal-tertutup**: endpoint tanpa anotasi permission eksplisit **ditolak**,
   bukan lolos otomatis.
6. **Audit log** — keputusan sadar tiap modul: masuk transaksi DB pemanggil, atau tetap terpisah
   (didokumentasikan per modul, bukan dibiarkan implisit seperti sistem lama).
7. **Zona waktu konsisten `Asia/Jakarta`** di seluruh modul (bukan hanya Finance seperti sistem
   lama).
8. **Isolasi data per branch ditegakkan di lapisan aplikasi** secara konsisten (scope
   selalu dari sesi terautentikasi, tidak pernah dari body request).
9. **Aturan arsitektur kritis diperiksa otomatis** (lint/arch-check), bukan hanya konvensi manual.
10. **Pesan galat server tidak pernah dibuang di frontend** — pesan asli dari API selalu sampai ke
    pengguna (lihat [API_CONTRACT.md](API_CONTRACT.md)).

## 7. Pola yang Terbukti Baik — Dipertahankan

Dari [legacy-reference/00-overview.md §7](legacy-reference/00-overview.md), pola berikut **bukan**
bug dan sebaiknya direplikasi:

- Provisioning cabang gagal-bersama (satu transaksi: cabang + gudang default + seluruh penomoran
  dokumen).
- Pola "pastikan ada" (idempoten) alih-alih "buat ulang" saat melengkapi data yang hilang.
- Import batch resumable dengan pesan galat yang menyebut nilai asli & lokasi baris/kolom.
- Snapshot dokumen historis (nota tidak bergeser saat master data berubah).
- Guard integritas lintas modul (kunci satuan produk setelah ada riwayat stok; stok wajib nol
  sebelum arsip produk).
- Mode baca-saja tiga lapis (kendali nonaktif + teks penjelas + submit diabaikan) pada pengaturan
  sensitif.

## 8. Urutan Pengerjaan Modul (Fase Produk)

Urutan ini murni berbasis ketergantungan data/kode (detail lengkap di
[SYSTEM_DESIGN.md §4](SYSTEM_DESIGN.md#4-urutan-pembangunan-modul), diturunkan dari
[legacy-reference/module-dependency-map.md](legacy-reference/module-dependency-map.md)):

1. Fondasi (audit, media, envelope, auth) — L0–L1
2. Identitas & Organisasi (user, company, branch) — L1
3. Master data (product, party) — L2
4. Stok (stock) — L3
5. Transaksi inti (purchasing, sales) — L4
6. Turunan transaksi (goods receipt, delivery, payment) — L5
7. Retur (sales return, purchase return) — L6
8. Keuangan (finance) — L7
9. Observabilitas (dashboard, reporting, audit UI) — L8
10. Assistant/WhatsApp (bot + tooling, tanpa Knowledge/RAG/AI) — L9, dijadwalkan dan dibangun atas permintaan pemilik

**Titik uji paling berharga** (diwarisi dari legacy): akhir tahap 5–7 — bila alur POS lengkap
(jual produk bervarian, harga member, cetak struk, potong stok, catat pembayaran) berjalan benar,
seluruh kontrak lintas modul di §9 SYSTEM_DESIGN sudah benar sebelum Finance dibangun.

## 9. Di Luar Cakupan (Untuk Saat Ini)

- Multi-tenant / multi-perusahaan dalam satu instalasi.
- Microservices, Kubernetes, message broker, cache terdistribusi (lihat aturan
  anti-overengineering di `CLAUDE.md`).
- Assistant AI dan knowledge base/RAG (bot WhatsApp + tooling operasional SUDAH dibangun
  sebagai L9; yang di luar cakupan hanya AI generatif + RAG — lihat §3).
- Localization selain Indonesia/Rupiah/WIB.

## 10. Referensi

- Peta modul lengkap (struktur file sistem lama): [legacy-reference/00-module-map.md](legacy-reference/00-module-map.md)
- Ringkasan kelengkapan analisis per modul: [legacy-reference/00-overview.md](legacy-reference/00-overview.md)
- Ketergantungan antar-modul & urutan rebuild: [legacy-reference/module-dependency-map.md](legacy-reference/module-dependency-map.md)
- 197 known issue: [legacy-reference/known-issues.md](legacy-reference/known-issues.md)
- 130 pertanyaan terbuka: [legacy-reference/open-questions.md](legacy-reference/open-questions.md)
- Dokumen mendalam per modul: `legacy-reference/<nomor>-<nama-modul>/`
