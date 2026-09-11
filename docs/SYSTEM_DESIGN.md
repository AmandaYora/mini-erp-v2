# System Design — Mini ERP (Revamp)

Status: **setup selesai, modul belum diimplementasikan**. Dokumen ini adalah rencana arsitektur
yang mengikat untuk seluruh pekerjaan modul berikutnya. Lihat [PRD.md](PRD.md) untuk latar
belakang produk dan [DB_SCHEMA.md](DB_SCHEMA.md) / [API_CONTRACT.md](API_CONTRACT.md) untuk detail
turunannya.

## 1. Gambaran Umum

```txt
apps/web  (React 19 + Tailwind 4, SPA)
    │  HTTP JSON  /api/v1/*
    ▼
apps/api  (Go modular monolith)
    │  database/sql + sqlc
    ▼
MySQL 8 (host-level, satu database)
```

- Satu perusahaan, multi-cabang. Setiap request terautentikasi membawa scope
  `idBranch` / `idUser` / role / permission — **selalu** diturunkan dari sesi
  (JWT + lookup DB), **tidak pernah** dari body request. Tidak ada scope perusahaan:
  instalasi ini standalone single-tenant (lihat ADR-0009).
- Tidak ada microservices, tidak ada message broker. Modul saling terisolasi lewat **package
  boundary di dalam satu binary Go**, ditegakkan compiler (import ke luar `contracts/` gagal
  build), bukan lewat jaringan.

## 2. Stack Terkunci

| Layer | Pilihan | Alasan |
|---|---|---|
| Backend | Go 1.21+, `net/http` + router minimal | Modular monolith standar Dimas |
| DB access | `database/sql` + `sqlc` (bukan GORM) | Query eksplisit, type-safe, tanpa magic ORM |
| Migrasi | `golang-migrate`, migrasi SQL bernomor | Ada tabel pelacak versi — memperbaiki KI migration-runner sistem lama |
| Watcher dev | `air` | `npm run dev:api` |
| Database | MySQL 8, host-level (bukan container) | Sesuai standar Docker Dimas |
| Frontend | React 19 + Tailwind 4 + react-router-dom + Zustand + Zod + Axios | Standar Dimas, dikunci |
| Auth | JWT access + refresh token, hash refresh token dirotasi tiap pakai | Properti keamanan yang **sudah benar** di sistem lama — dipertahankan |
| Locale | `id-ID`, `Asia/Jakarta`, IDR — konstanta aplikasi | Lihat ADR-0006; bukan setelan yang bisa diubah pengguna |

## 3. Peta Modul Backend

20 modul bisnis + `shared/` teknis, mengganti 1 modul raksasa (`order/`, 6 domain tercampur) di
sistem lama dengan batas yang eksplisit. Struktur folder tiap modul mengikuti
`.claude/rules/backend-modular-monolith.md` (`contracts/ · application/ · domain/ · infrastructure/ · presentation/`).

| Modul (`internal/modules/<x>`) | Tanggung jawab | Kontrak publik utama (`contracts/`) | Modul lama |
|---|---|---|---|
| `audit` | Mencatat siapa mengubah apa, kapan, di semua modul | `AuditClient.Log(entry)` | 20 |
| `media` | Penyimpanan file (driver lokal/S3) untuk foto produk, bukti bayar, bukti kirim | `MediaClient.Upload/GetURL` | storage (S1) |
| `auth` | Login, sesi (JWT access/refresh), ganti role/cabang aktif, resolusi permission | `SessionClient.Verify`, `PermissionClient.Check` | 01 |
| `user` | Akun pengguna, role, permission (RBAC) | `UserClient.GetByID`, `RoleClient.GetPermissions` | 02 |
| `company` | Profil & pengaturan perusahaan (tunggal) | `CompanyClient.GetSettings` | 03 |
| `branch` | Direktori cabang + penomoran dokumen per cabang | `BranchClient.GetByID`, `BranchClient.NextDocumentNumber(kind)` | 04 |
| `product` | Katalog produk, kategori, varian, UOM & faktor konversi | `ProductClient.GetByID`, `ProductClient.ResolveUOM` | 05 |
| `party` | Customer, supplier, alamat kirim, member type & harga member | `PartyClient.GetByID`, `PricingClient.Quote` | 06, 07 |
| `stock` | Lokasi stok, saldo & mutasi, transfer, penyesuaian, saldo awal | `StockClient.GetBalance`, `StockClient.Reserve/Move` | 16 |
| `purchasing` | Order pembelian ke supplier | `PurchaseOrderClient.GetByID` | 08 |
| `sales` | Order penjualan ke customer (kanal reguler & POS) | `SalesOrderClient.GetByID` | 09, 15 (kanal) |
| `goodsreceipt` | Penerimaan barang atas PO | `GoodsReceiptClient.GetByID` | 10 |
| `delivery` | Pengiriman atas SO, surat jalan, bukti kirim | `DeliveryClient.GetByID` | 11 |
| `payment` | Pembayaran & alokasi, saldo/ledger pihak | `PaymentClient.GetByID`, `PaymentClient.GetPartyBalance` | 14 |
| `salesreturn` | Retur penjualan atas SO/SJ | `SalesReturnClient.GetByID` | 12 |
| `purchasereturn` | Retur pembelian atas PO/penerimaan | `PurchaseReturnClient.GetByID` | 13 |
| `finance` | COA, jurnal, HPP, laporan keuangan, tutup periode, ekspor pajak | *(konsumen terbesar — lihat §7)* | 17 |
| `dashboard` | Ringkasan operasional cabang aktif (baca finance live, tanpa tabel) | `DashboardClient.Summary` | 18 |
| `reporting` | Tren penjualan + nilai persediaan (baca finance live, tanpa agregasi terjadwal — KI-126) | `ReportingClient.SalesTrend/Inventory` | 19 |
| `assistant` | Bot WA (whatsmeow) + 8 intent tooling read-only, whitelist nomor, tanpa RAG/AI | `AssistantClient.Ask` | 21 |

**L9 terjadwal belakangan atas permintaan pemilik:** `assistant` (bot WhatsApp +
tooling operasional, TANPA Knowledge/RAG/AI — keputusan skop L9). Lihat
[PRD.md §3](PRD.md#3-cakupan-produk-peta-modul-bisnis).

Semua 20 modul + `auth` sudah terisi penuh di `apps/api/internal/modules/`
(masing-masing `contracts/` → `application/` → `domain/` → `infrastructure/` → `presentation/`).

**POS** sengaja tidak punya modul backend sendiri — ia adalah kanal frontend yang merakit
`product`, `party`, `stock`, `sales`, `payment` lewat kontrak yang sama dipakai alur penjualan
reguler (`sales.channel = 'pos' | 'regular'`). Ini meniru pola sistem lama yang sudah benar
(modul 15 tidak pernah punya backend sendiri).

## 4. Urutan Pembangunan Modul

Diturunkan dari [legacy-reference/module-dependency-map.md §4, §9](legacy-reference/module-dependency-map.md)
— ketergantungan **data** (query lintas tabel), bukan cuma `import`, sudah diperhitungkan.

| Lapisan | Modul | Boleh mulai setelah |
|---|---|---|
| L0 — Fondasi | `shared/` (envelope, error, validator, pagination), `audit`, `media` | — |
| L1 — Identitas & Organisasi | `auth`, `user`, `company`, `branch` | L0. Dibangun sebagai **satu paket** — saling bergantung erat (siklus auth⇄branch⇄user), diselesaikan lewat `contracts/` masing-masing, bukan import langsung |
| L2 — Master Data | `product`, `party` | L1. `party` butuh `product` (member pricing menghitung dari harga produk) |
| L3 — Inventaris | `stock` | L1 + `product` |
| L4 — Transaksi Inti | `purchasing`, `sales` | L1 + L2 + L3 |
| L5 — Turunan Transaksi | `goodsreceipt` (butuh `purchasing`), `delivery` (butuh `sales`+`stock`), `payment` (butuh `purchasing`+`sales`) | L4. Ketiganya bisa paralel |
| L6 — Retur | `salesreturn` (butuh `sales`+`delivery`), `purchasereturn` (butuh `purchasing`+`goodsreceipt`) | L5 |
| L7 — Keuangan | `finance` | **Semua modul di atas** — konsumen data terbesar (§7). Selalu paling akhir dari sisi transaksional |
| L8 — Observabilitas | `dashboard`, `reporting`, UI `audit` | L4 (dashboard/reporting cukup baca order+stok) |
| L9 — Asisten | `assistant` | `branch`, `product`, `sales`, `purchasing`, `stock`, `finance` (read-only via kontrak) |

**Titik uji tervalidasi**: setelah L4–L6 selesai, alur POS end-to-end (jual produk bervarian +
harga member → potong stok → catat pembayaran → cetak struk) harus berjalan benar sebelum
`finance` mulai dibangun — ini bukti bahwa kontrak lintas modul di §9 sudah stabil.

## 5. Aturan Batas Modul (ditegakkan, bukan konvensi)

- Modul hanya boleh mengakses tabel miliknya sendiri. Relasi lintas modul disimpan sebagai
  **primitive ID** (`branch_id`, `product_id`, dst.), **tanpa foreign key fisik lintas modul**.
- Modul lain hanya boleh diakses lewat `contracts/` milik modul penyedia — dilarang mengimpor
  `application/`, `domain/`, atau `infrastructure/` modul lain.
- Alur lintas modul (mis. buat SO → cek stok → kurangi stok → buat pembayaran) diorkestrasi oleh
  application service modul pemicu, memanggil kontrak modul lain secara eksplisit — bukan lewat
  join database atau service call tersembunyi.
- Transaksi database (`BEGIN...COMMIT`) selalu dalam lingkup satu modul. Konsistensi lintas modul
  ditangani lewat orkestrasi eksplisit di application layer, bukan transaksi terdistribusi.

## 6. Response Envelope & Konvensi API

Ditetapkan sekali di `shared/response`, dipakai semua modul — lihat detail lengkap di
[API_CONTRACT.md](API_CONTRACT.md). Ringkas:

- Semua endpoint bisnis berbasis `/api/v1/*`.
- Sukses: `{ "success": true, "message": "...", "data": {...} }`.
- Gagal: `{ "success": false, "message": "...", "errors": [...] }` — **pesan asli selalu ada**,
  tidak pernah dibuang di frontend (memperbaiki KI-04/KI-17 sistem lama).
- Validasi payload wajib di setiap endpoint tulis, sejak modul pertama — bukan ditambahkan
  belakangan (memperbaiki temuan "~214 dari 220 endpoint tanpa validasi runtime").
- Scope (`idBranch`/`idUser`/role/permission) selalu diturunkan dari sesi
  terautentikasi lewat middleware, tidak pernah dari body/query request.
- Permission guard **gagal-tertutup**: endpoint tanpa deklarasi permission eksplisit ditolak,
  bukan lolos otomatis (memperbaiki KI permission guard sistem lama).

## 7. Kontrak Lintas Modul yang Tidak Boleh Berubah Sepihak

Diadaptasi dari 12 kontrak yang teridentifikasi di
[legacy-reference/module-dependency-map.md §6](legacy-reference/module-dependency-map.md). Setiap
baris di sini melibatkan lebih dari satu modul — mengubahnya butuh koordinasi lintas modul, bukan
keputusan modul tunggal.

| # | Kontrak | Pemilik | Konsumen |
|---|---|---|---|
| 1 | Scope sesi (`idBranch`/`idUser`/role/permission) selalu dari token, tidak pernah dari body | `auth` | Semua modul |
| 2 | Response envelope `{success, message, data}` / error `{success:false, message, errors}` | `shared/response` | Semua modul |
| 3 | Satuan stok + faktor konversi — semua saldo disimpan dalam satuan stok dasar | `product` | `stock`, `purchasing`, `sales`, `finance` |
| 4 | Snapshot item dokumen — nota historis menyimpan salinan data, bukan join ke master data hidup | `purchasing`/`sales` | `goodsreceipt`, `delivery`, `payment`, `salesreturn`, `purchasereturn`, `finance` |
| 5 | Setiap perubahan saldo stok (`StockBalance`) selalu disertai baris mutasi (`StockMovement`), resolve ke lokasi daun | `stock` | `purchasing`, `sales`, `goodsreceipt`, `delivery`, `salesreturn`, `purchasereturn`, `finance` |
| 6 | Transfer stok = dua mutasi (`out` + `in`), bukan satu baris bertipe `transfer` | `stock` | `finance` |
| 7 | Prefix nomor dokumen diturunkan dari kode cabang, dialokasikan lewat `BranchClient.NextDocumentNumber` | `branch` | `purchasing`, `sales`, `goodsreceipt`, `delivery`, `payment`, `salesreturn`, `purchasereturn` |
| 8 | Harga beli produk (harga pokok) adalah basis HPP | `product` | `finance` |
| 9 | Seluruh rentang tanggal & penomoran periode memakai zona `Asia/Jakarta` | `shared` (util tanggal) | Semua modul, terutama `finance` |
| 10 | `AuditClient.Log` dipanggil di setiap operasi tulis dengan `action_key` `snake_case` `resource.action` | `audit` | Semua modul |
| 11 | Setiap produk selalu punya minimal satu varian (varian default tersembunyi bila produk tidak dikonfigurasi bervarian) | `product` | `stock`, `sales` (kanal POS) |
| 12 | Momen stok berkurang untuk penjualan (saat `delivery` dibuat vs saat SO dikonfirmasi) — **keputusan terbuka**, harus ditetapkan eksplisit saat mendesain modul `sales`/`delivery`, tidak boleh diasumsikan dari kebiasaan sistem lama | `sales`/`delivery` | `stock`, `finance` |

## 8. Modul Paling Berisiko Diubah

Diwarisi dari analisis dependensi legacy — informasi ini menentukan **berapa hati-hati** dan
seberapa lengkap test yang dibutuhkan saat modul itu didesain:

| Modul | Kenapa berisiko tinggi |
|---|---|
| `branch` | ~26 tabel lama menaut `branch_id`; seluruh transaksi & jurnal terikat cabang |
| `product` | Satuan & faktor konversi adalah kontrak lintas modul (stock, order, finance, POS) |
| `stock` | Saldo & mutasi jadi basis HPP — kesalahan di sini bocor ke laporan keuangan |
| `purchasing`/`sales` | Modul transaksi terbesar; paling banyak dikonsumsi modul turunan |
| `auth` | Bentuk sesi adalah kontrak universal seluruh sistem |
| `finance` | Tidak berisiko *sebagai dependensi* (hampir tidak ada yang bergantung padanya), tapi **paling rapuh sebagai konsumen** — baca ulang §7 sebelum mengubah bentuk tabel modul manapun |

## 9. Keamanan & Isolasi Data

- Autentikasi: JWT access token (masa pakai pendek) + refresh token (hash dirotasi tiap dipakai,
  replay terdeteksi) — properti yang sudah benar di sistem lama, dipertahankan. Batas umur sesi
  mutlak 30 hari di atas refresh sliding 7 hari (OQ-A02). Kegagalan login dicatat dan endpoint
  login di-rate-limit (OQ-A03/A04, keputusan analis).
- Otorisasi: permission di-resolve ulang dari DB tiap request (bukan di-cache di token) — perubahan
  hak akses berlaku seketika tanpa perlu logout.
- Isolasi cabang: setiap query atas data terikat-cabang wajib difilter `branch_id` dari scope
  sesi. Tidak ada filter perusahaan — instalasi ini standalone single-tenant (lihat ADR-0009).
  Isolasi antar-cabang wajib ditegakkan konsisten di setiap query modul (bukan hanya disiplin
  per-service seperti sistem lama).
- CORS: dibatasi ke origin frontend yang dikenal secara eksplisit di konfigurasi server — **tidak**
  mengizinkan seluruh origin (`*`) seperti sistem lama (lihat PRD §5 poin 1).

## 10. Observabilitas

- Audit log (`audit` module) adalah dependensi L0 — dipanggil semua modul penulis data sejak awal,
  bukan ditambahkan belakangan.
- Migrasi terlacak lewat `golang-migrate` (tabel versi bawaan) — dijalankan eksplisit via
  `npm run migrate:up`, tidak otomatis re-run di setiap start seperti sistem lama.

## 11. Referensi

- [PRD.md](PRD.md) — tujuan produk & cakupan
- [DB_SCHEMA.md](DB_SCHEMA.md) — kepemilikan tabel per modul
- [API_CONTRACT.md](API_CONTRACT.md) — konvensi endpoint
- [DEPLOYMENT.md](DEPLOYMENT.md) — build & deploy
- `.claude/rules/backend-modular-monolith.md` — aturan boundary yang ditegakkan saat menyunting kode
- [legacy-reference/module-dependency-map.md](legacy-reference/module-dependency-map.md) — sumber
  urutan lapisan & kontrak di §4 dan §7
