# API Contract — Mini ERP (Revamp)

Status: **konvensi ditetapkan, belum ada endpoint nyata.** Endpoint per modul didaftarkan di sini
saat modul itu diimplementasikan — jangan menuliskan endpoint yang belum ada.

## 1. Versioning

Semua endpoint bisnis di bawah `/api/v1`.

```txt
POST /api/v1/auth/login
GET  /api/v1/products
POST /api/v1/sales-orders
```

Metode HTTP mengikuti semantik REST standar (`GET` untuk baca, `POST`/`PUT`/`PATCH`/`DELETE` untuk
tulis) — **berbeda dari sistem lama** yang menyeragamkan semua endpoint bisnis jadi `POST` dengan
body `{ data }`. Perubahan ini disengaja: memakai semantik HTTP yang sesungguhnya membuat cache,
idempotency, dan tooling standar (OpenAPI, HTTP client) bekerja tanpa penyesuaian khusus.

## 2. Bentuk Response

### Sukses

```json
{
  "success": true,
  "message": "Order created successfully",
  "data": { "id": 123, "orderNumber": "SO-JKT-00001" }
}
```

### Gagal

```json
{
  "success": false,
  "message": "Validation failed",
  "errors": [
    { "field": "partyId", "message": "Customer wajib dipilih" }
  ]
}
```

`message` **selalu** berisi pesan yang bisa ditampilkan langsung ke pengguna (Bahasa Indonesia).
Frontend **tidak boleh** membuang `message` dan menggantinya dengan teks generik — ini memperbaiki
KI-04/KI-17 sistem lama, di mana pesan spesifik dari server dibuang di lapisan frontend dan
pengguna hanya melihat kode mesin atau pesan generik.

### Terpaginasi

```json
{
  "success": true,
  "message": "Data retrieved successfully",
  "data": [ { "id": 1 }, { "id": 2 } ],
  "meta": { "page": 1, "limit": 20, "total": 134, "totalPages": 7 }
}
```

- `limit` dipangkas ke batas maksimum di server (mis. 100) dan **dijaga terhadap nilai tidak
  wajar** (`0`, negatif, non-angka) di lapisan `shared/pagination` — bukan ditulis ulang per modul
  seperti sistem lama (yang punya bug di jalur ini, lihat KI-26).

## 3. Autentikasi & Scope

- Header: `Authorization: Bearer <access_token>`.
- Access token berumur pendek; refresh lewat `POST /api/v1/auth/refresh` dengan refresh token
  (hash dirotasi tiap dipakai — replay terdeteksi).
- Scope request (`branchId`/`userId`/role/permission) **selalu** diturunkan dari token
  + lookup sesi di server. Endpoint **tidak boleh** menerima scope ini dari body/query request.
- Ganti cabang/role aktif: `POST /api/v1/auth/switch-branch`, `POST /api/v1/auth/switch-role`.

## 4. Validasi

Setiap endpoint tulis wajib mendeklarasikan skema validasi payload eksplisit sebelum masuk ke
application layer (lihat `.claude/rules/api-standard.md`). Ini **wajib sejak endpoint pertama** —
sistem lama hanya memvalidasi 1 dari ~220 endpoint, sisanya bergantung pada asumsi tipe data yang
sering meleset (lihat contoh nyata: email kosong → error 500, `limit=abc` → error 500).

## 5. Permission per Endpoint

Setiap endpoint bisnis mendeklarasikan permission yang dibutuhkan secara eksplisit. Endpoint tanpa
deklarasi permission **ditolak secara default** (fail-closed) — bukan otomatis lolos seperti guard
permission sistem lama.

## 6. Peta Prefix Rute per Modul

Didaftarkan sebagai rencana routing; endpoint konkret ditambahkan ke tabel di bawah **saat modul
itu diimplementasikan** (jangan mendokumentasikan endpoint yang belum ditulis kodenya).

| Modul | Prefix rute | Status |
|---|---|---|
| `auth` | `/api/v1/auth/*` | Terimplementasi (L1) |
| `user` | `/api/v1/users/*`, `/api/v1/roles/*` | Terimplementasi (L1) |
| `company` | `/api/v1/company/*` | Terimplementasi (L1) |
| `branch` | `/api/v1/branches/*` | Terimplementasi (L1) |
| `product` | `/api/v1/products/*`, `/api/v1/product-categories/*`, `/api/v1/product-imports/*` | Terimplementasi (L2) |
| `party` | `/api/v1/customers/*`, `/api/v1/suppliers/*`, `/api/v1/member-types/*`, `/api/v1/pricing/*` | Terimplementasi (L2) |
| `stock` | `/api/v1/stock/*` | Terimplementasi (L3) |
| `purchasing` | `/api/v1/purchase-orders/*` | Terimplementasi (L4) |
| `sales` | `/api/v1/sales-orders/*` | Terimplementasi (L4, kanal regular+pos) |
| `goodsreceipt` | `/api/v1/goods-receipts/*` | Terimplementasi (L5) |
| `delivery` | `/api/v1/deliveries/*` | Terimplementasi (L5) |
| `payment` | `/api/v1/payments/*` | Terimplementasi (L5) |
| `salesreturn` | `/api/v1/sales-returns/*` | Terimplementasi (L6) |
| `purchasereturn` | `/api/v1/purchase-returns/*` | Terimplementasi (L6) |
| `finance` | `/api/v1/finance/*` | Terimplementasi (L7) |
| `dashboard` | `/api/v1/dashboard/*` | Terimplementasi (L8) |
| `reporting` | `/api/v1/reporting/*` | Terimplementasi (L8) |
| `audit` | `/api/v1/audit-logs/*` | Terimplementasi (L8) |
| `assistant` | `/api/v1/assistant/*` | Terimplementasi (L9: bot WA + tooling, tanpa RAG/AI) |

Endpoint yang sudah ada hari ini:

| Method | Path | Keterangan |
|---|---|---|
| `POST` | `/api/v1/auth/login` | Login (public, rate-limit) → access+refresh JWT, cabang otomatis/default/pilih |
| `POST` | `/api/v1/auth/refresh` | Rotasi refresh token (replay = seluruh sesi dicabut) |
| `POST` | `/api/v1/auth/logout` | Cabut sesi pemanggil |
| `GET` | `/api/v1/auth/me` | Identitas + cabang aktif + permission segar + profil perusahaan |
| `POST` | `/api/v1/auth/switch-branch` | Pindah cabang aktif (cabang nonaktif = 403 berpesan) |
| `POST` | `/api/v1/auth/switch-role` | Pindah role aktif milik sendiri |
| `GET` | `/api/v1/users` | Daftar pengguna (`page`, `limit` terjaga, `search` per kata, `status`) |
| `POST` | `/api/v1/users` | Tambah pengguna (transaksi: akun+role+cabang) |
| `GET` | `/api/v1/users/{id}` | Detail pengguna |
| `PUT` | `/api/v1/users/{id}` | Ubah pengguna (transaksi) |
| `PATCH` | `/api/v1/users/{id}/status` | Aktif/nonaktif (tak bisa menonaktifkan diri sendiri) |
| `POST` | `/api/v1/users/{id}/password` | Ganti password (min. 8) |
| `GET` | `/api/v1/roles` | Daftar role |
| `POST` | `/api/v1/roles` | Tambah role kustom (kode unik, tak bisa diubah setelahnya) |
| `PUT` | `/api/v1/roles/{id}` | Ubah nama/deskripsi role |
| `DELETE` | `/api/v1/roles/{id}` | Hapus role kustom yang tak dipakai (role sistem dilindungi) |
| `PUT` | `/api/v1/roles/{id}/permissions` | Ganti total izin role (kode tak dikenal = 400 sebelum apa pun dihapus) |
| `GET` | `/api/v1/permissions` | Katalog permission + nama tampilan (untuk UI kelola role) |
| `GET` | `/api/v1/branches` | Daftar cabang |
| `POST` | `/api/v1/branches` | Tambah cabang (transaksi: cabang + seluruh sequence nomor) |
| `GET` | `/api/v1/branches/{id}` | Detail cabang |
| `PUT` | `/api/v1/branches/{id}` | Ubah/tutup/buka cabang (kode dikunci bila sudah ada dokumen terbit) |
| `GET` | `/api/v1/company/profile` | Profil perusahaan (null bila belum diisi) |
| `PUT` | `/api/v1/company/profile` | Simpan profil (upsert baris tunggal) |
| `GET` | `/api/v1/company/settings` | Seluruh pengaturan operasional |
| `PUT` | `/api/v1/company/settings` | Simpan pengaturan (gabung per kunci, tak pernah ganti-total) |
| `GET` | `/api/v1/products` | Daftar produk (search per kata, filter kategori/status) |
| `POST` | `/api/v1/products` | Tambah produk (transaksi: produk + varian; varian default otomatis) |
| `GET` | `/api/v1/products/search-options` | Opsi ringkas untuk dropdown/POS |
| `GET` | `/api/v1/products/{id}` | Detail produk + varian + kategori + media |
| `PUT` | `/api/v1/products/{id}` | Ubah produk (transaksi; UOM/tipe terkunci bila ada riwayat stok) |
| `POST` | `/api/v1/products/{id}/archive` | Arsipkan produk |
| `GET` | `/api/v1/product-categories` | Daftar kategori |
| `POST` | `/api/v1/product-categories` | Tambah kategori |
| `PUT` | `/api/v1/product-categories/{id}` | Ubah kategori (kode tak bisa diubah) |
| `POST` | `/api/v1/product-categories/{id}/archive` | Arsipkan kategori kosong |
| `GET` | `/api/v1/products/{id}/media` | Daftar foto (URL lokal `/uploads/*` atau presigned S3 sesuai driver) |
| `POST` | `/api/v1/products/{id}/media` | Unggah foto (multipart, gambar ≤5MB) |
| `POST` | `/api/v1/products/{id}/media/{mediaId}/primary` | Jadikan foto utama |
| `DELETE` | `/api/v1/products/{id}/media/{mediaId}` | Hapus foto (baris + berkas) |
| `GET` | `/api/v1/product-imports/template` | Unduh template Excel |
| `POST` | `/api/v1/product-imports/preview` | Pratinjau validasi (tanpa tulis) + galat per baris |
| `POST` | `/api/v1/product-imports/commit` | Impor resumable (sisip valid, lewati sudah-ada, laporkan gagal) |
| `GET` | `/api/v1/customers` | Daftar customer (search per kata) |
| `POST` | `/api/v1/customers` | Tambah customer (kode opsional → otomatis anti-kembar) |
| `GET` | `/api/v1/customers/{id}` | Detail customer + alamat |
| `PUT` | `/api/v1/customers/{id}` | Ubah customer (kode tak bisa diubah; member tak hilang diam-diam) |
| `POST` | `/api/v1/customers/{id}/archive` | Arsipkan customer (buku alamat ikut terkunci) |
| `POST` | `/api/v1/customers/{id}/restore` | Pulihkan customer |
| `GET` | `/api/v1/suppliers` | Daftar supplier |
| `POST` | `/api/v1/suppliers` | Tambah supplier |
| `GET` | `/api/v1/suppliers/{id}` | Detail supplier |
| `PUT` | `/api/v1/suppliers/{id}` | Ubah supplier |
| `POST` | `/api/v1/suppliers/{id}/archive` | Arsipkan supplier |
| `POST` | `/api/v1/suppliers/{id}/restore` | Pulihkan supplier |
| `GET` | `/api/v1/customers/{id}/addresses` | Buku alamat (utama dulu) |
| `POST` | `/api/v1/customers/{id}/addresses` | Tambah alamat (tepat satu utama) |
| `PUT` | `/api/v1/customers/{id}/addresses/{addrId}` | Ubah alamat |
| `POST` | `/api/v1/customers/{id}/addresses/{addrId}/archive` | Arsipkan alamat (utama dialihkan otomatis) |
| `GET` | `/api/v1/member-types` | Daftar tipe member |
| `POST` | `/api/v1/member-types` | Tambah tipe member (aturan persen/nominal + pembulatan berpasangan) |
| `PUT` | `/api/v1/member-types/{id}` | Ubah tipe member (kode tak bisa diubah) |
| `POST` | `/api/v1/member-types/{id}/archive` | Arsipkan tipe member |
| `POST` | `/api/v1/member-types/{id}/restore` | Pulihkan tipe member |
| `POST` | `/api/v1/pricing/quote` | Quote harga member per keranjang (gagal eksplisit, degradasi berpenanda) |
| `GET` | `/api/v1/stock/locations` | Daftar lokasi cabang (sistem + manual) |
| `POST` | `/api/v1/stock/locations` | Tambah lokasi |
| `PUT` | `/api/v1/stock/locations/{id}` | Ubah lokasi (anti-siklus) |
| `POST` | `/api/v1/stock/locations/{id}/archive` | Arsipkan lokasi kosong tanpa anak |
| `GET` | `/api/v1/stock/balances` | Saldo per produk |
| `GET` | `/api/v1/stock/movements` | Riwayat mutasi (filter + tanggal inklusif) |
| `POST` | `/api/v1/stock/scan-product` | Pindai barcode → produk + saldo |
| `POST` | `/api/v1/stock/allocations/suggest` | Saran alokasi + shortfall |
| `POST` | `/api/v1/stock/reservations/hold` | Tahan stok (idempoten per kunci, TTL) |
| `POST` | `/api/v1/stock/reservations/release` | Lepas tahanan (idempoten) |
| `POST` | `/api/v1/stock/adjustments` | Koreksi (di atas ambang perlu approver stock.approve) |
| `GET` | `/api/v1/stock/transfers` | Daftar dokumen transfer |
| `POST` | `/api/v1/stock/transfers` | Buat draf transfer (nomor TRF-…) |
| `GET` | `/api/v1/stock/transfers/{id}` | Detail transfer |
| `POST` | `/api/v1/stock/transfers/{id}/dispatch` | Kirim (potong asal, atomik) |
| `POST` | `/api/v1/stock/transfers/{id}/receive` | Terima (tambah tujuan) |
| `POST` | `/api/v1/stock/transfers/{id}/cancel` | Batalkan (draf langsung, terkirim via reversal) |
| `GET` | `/api/v1/stock/damaged` | Saldo lokasi rusak |
| `POST` | `/api/v1/stock/damaged/move-in` | Catat rusak |
| `POST` | `/api/v1/stock/damaged/restore` | Pulihkan |
| `POST` | `/api/v1/stock/damaged/write-off` | Hapusbukukan (alasan wajib) |
| `GET` | `/api/v1/stock/opening/template` | Template saldo awal |
| `POST` | `/api/v1/stock/opening/preview` | Pratinjau (tanpa tulis) |
| `POST` | `/api/v1/stock/opening/commit` | Bukukan (resumable) |
| `GET` | `/api/v1/purchase-orders` | Daftar PO cabang |
| `POST` | `/api/v1/purchase-orders` | Buat draf PO (nomor ORD-…/PB/…, snapshot harga) |
| `GET` | `/api/v1/purchase-orders/{id}` | Detail PO |
| `PUT` | `/api/v1/purchase-orders/{id}` | Ubah draf PO |
| `POST` | `/api/v1/purchase-orders/{id}/confirm` | Konfirmasi PO |
| `POST` | `/api/v1/purchase-orders/{id}/cancel` | Batalkan PO |
| `GET` | `/api/v1/sales-orders` | Daftar SO cabang (filter status/kanal) |
| `POST` | `/api/v1/sales-orders` | Buat draf SO (harga dari server: member/ katalog) |
| `GET` | `/api/v1/sales-orders/{id}` | Detail SO |
| `PUT` | `/api/v1/sales-orders/{id}` | Ubah draf SO (hitung ulang) |
| `POST` | `/api/v1/sales-orders/{id}/confirm` | Konfirmasi SO (stok bergerak saat delivery) |
| `POST` | `/api/v1/sales-orders/{id}/cancel` | Batalkan SO |
| `POST` | `/api/v1/sales-orders/{id}/approve-credit` | COD → net + tempo |
| `GET` | `/api/v1/goods-receipts` | Daftar penerimaan cabang (filter PO) |
| `POST` | `/api/v1/goods-receipts` | Terima PO (tambah stok, lunasi PO bila penuh) |
| `GET` | `/api/v1/goods-receipts/{id}` | Detail penerimaan |
| `GET` | `/api/v1/deliveries` | Daftar SJ cabang (filter SO/status) |
| `POST` | `/api/v1/deliveries` | Buat draf SJ (stok belum bergerak) |
| `GET` | `/api/v1/deliveries/{id}` | Detail SJ |
| `POST` | `/api/v1/deliveries/{id}/confirm` | Konfirmasi (kurangi stok, lunasi SO bila penuh) |
| `POST` | `/api/v1/deliveries/{id}/cancel` | Batalkan draf (terkonfirmasi via retur L6) |
| `POST` | `/api/v1/deliveries/{id}/proof` | Unggah bukti kirim (gambar ≤5MB) |
| `GET` | `/api/v1/deliveries/{id}/proofs` | Daftar bukti kirim |
| `GET` | `/api/v1/payments` | Daftar pembayaran cabang |
| `POST` | `/api/v1/payments` | Catat pembayaran (alokasi eksplisit atau FIFO otomatis; refund via tipe `sales_return`/`purchase_return` — arah berbalik, satu pembayaran satu arah; partyId 0 = walk-in tunai POS, alokasi wajib eksplisit) |
| `GET` | `/api/v1/payments/{id}` | Detail pembayaran |
| `POST` | `/api/v1/payments/{id}/cancel` | Batalkan pembayaran (saldo terbuka lagi) |
| `GET` | `/api/v1/payments/party-balances` | Saldo tagihan satu pihak (live) |
| `GET` | `/api/v1/payments/party-ledger` | Buku pembayaran satu pihak |
| `POST` | `/api/v1/payments/{id}/proof` | Unggah bukti bayar (gambar ≤5MB) |
| `GET` | `/api/v1/payments/{id}/proofs` | Daftar bukti bayar |
| `POST` | `/api/v1/payments/settle-credit` | Offset retur ke tagihan tanpa kas (dua kaki kontra, method "offset") |
| `GET` | `/api/v1/sales-returns` | Daftar retur cabang |
| `POST` | `/api/v1/sales-returns` | Buat draf retur (batas = terkirim−sudah diretur; mode tukar via `returnMode` + `replacementItems`) |
| `GET` | `/api/v1/sales-returns/context` | Sisa qty per baris SO + daftar lokasi (masukan form retur) |
| `POST` | `/api/v1/sales-returns/preview` | Pratinjau angka retur (tanpa tulis, builder sama dengan create) |
| `GET` | `/api/v1/sales-returns/{id}` | Detail retur (+ mode, barang pengganti, penyelesaian) |
| `POST` | `/api/v1/sales-returns/{id}/confirm` | Konfirmasi (stok kembali) |
| `POST` | `/api/v1/sales-returns/{id}/cancel` | Batalkan (draf langsung, terkonfirmasi via reversal) |
| `POST` | `/api/v1/sales-returns/{id}/replacement-deliveries` | Kirim barang pengganti (draf SJ pengganti; stok keluar saat konfirmasi) |
| `POST` | `/api/v1/sales-returns/{id}/replacement-deliveries/{deliveryId}/confirm` | Konfirmasi SJ pengganti (stok keluar) |
| `POST` | `/api/v1/sales-returns/{id}/settlements` | Catat penyelesaian (memo; total ≤ nilai retur) |
| `GET` | `/api/v1/purchase-returns` | Daftar retur cabang |
| `POST` | `/api/v1/purchase-returns` | Buat draf retur (batas = diterima−sudah diretur) |
| `GET` | `/api/v1/purchase-returns/context` | Sisa qty per baris PO + daftar lokasi |
| `POST` | `/api/v1/purchase-returns/preview` | Pratinjau angka retur (tanpa tulis) |
| `GET` | `/api/v1/purchase-returns/{id}` | Detail retur (+ penyelesaian) |
| `POST` | `/api/v1/purchase-returns/{id}/confirm` | Konfirmasi (stok keluar kembali) |
| `POST` | `/api/v1/purchase-returns/{id}/cancel` | Batalkan (draf langsung, terkonfirmasi via reversal) |
| `POST` | `/api/v1/purchase-returns/{id}/settlements` | Catat penyelesaian (memo; total ≤ nilai retur) |
| `GET` | `/api/v1/finance/accounts` | Daftar akun |
| `POST` | `/api/v1/finance/accounts` | Tambah akun (kode tak bisa diubah) |
| `PUT` | `/api/v1/finance/accounts/{id}` | Ubah akun |
| `POST` | `/api/v1/finance/accounts/{id}/archive` | Arsipkan akun kosong tak terpetakan |
| `GET` | `/api/v1/finance/account-mappings` | Pemetaan akun otomatis |
| `PUT` | `/api/v1/finance/account-mappings` | Ubah pemetaan (kunci dikenal saja) |
| `GET` | `/api/v1/finance/periods` | Daftar periode |
| `POST` | `/api/v1/finance/periods/close` | Tutup periode (kunci tanggal) |
| `POST` | `/api/v1/finance/periods/reopen` | Buka periode |
| `GET` | `/api/v1/finance/posting/preview` | Pratinjau jurnal dokumen (tanpa tulis) |
| `POST` | `/api/v1/finance/posting/post` | Posting dokumen (idempoten) |
| `GET` | `/api/v1/finance/journals` | Daftar jurnal |
| `POST` | `/api/v1/finance/journals/manual` | Jurnal manual (imbang wajib) |
| `GET` | `/api/v1/finance/journals/{id}` | Detail jurnal |
| `POST` | `/api/v1/finance/journals/{id}/reverse` | Balik jurnal (alasan wajib) |
| `GET` | `/api/v1/finance/expenses` | Daftar biaya |
| `POST` | `/api/v1/finance/expenses` | Catat biaya (auto-jurnal) |
| `GET` | `/api/v1/finance/expenses/{id}` | Detail biaya |
| `POST` | `/api/v1/finance/expenses/{id}/cancel` | Batalkan biaya (jurnal dibalik) |
| `POST` | `/api/v1/finance/costing/sync` | Sinkronisasi HPP dari mutasi stok |
| `GET` | `/api/v1/finance/costing/uncosted` | Daftar posisi estimasi |
| `GET` | `/api/v1/finance/tax-periods` | Daftar periode pajak |
| `POST` | `/api/v1/finance/tax-periods/close` | Tutup + snapshot SPT |
| `POST` | `/api/v1/finance/tax-periods/reopen` | Buka periode pajak |
| `GET` | `/api/v1/finance/reports/trial-balance` | Neraca saldo per bulan |
| `GET` | `/api/v1/finance/reports/profit-loss` | Laba rugi per rentang |
| `GET` | `/api/v1/finance/reports/balance-sheet` | Neraca per tanggal |
| `GET` | `/api/v1/finance/reports/general-ledger` | Buku besar per akun |
| `GET` | `/api/v1/finance/reports/tax-summary` | Ringkasan PPN per bulan |
| `GET` | `/api/v1/finance/reports/tax-detail` | Rincian PPN (JSON/CSV) |
| `GET` | `/api/v1/finance/reports/inventory-value` | Nilai persediaan (rata-rata) |
| `GET` | `/api/v1/finance/reports/margin` | Marjin kotor per rentang |
| `GET` | `/api/v1/dashboard/summary` | Snapshot operasional cabang: P&L hari ini + bulan berjalan, saldo kas/piutang/hutang/persediaan, nilai persediaan (semua dari buku finance) |
| `GET` | `/api/v1/reporting/sales-trend` | Tren harian revenue/expense/profit (`from`/`to` YYYY-MM-DD, maks 366 hari) |
| `GET` | `/api/v1/reporting/inventory` | Posisi + total nilai persediaan (rata-rata berjalan) |
| `GET` | `/api/v1/audit-logs` | Jejak audit cabang (termasuk event global) — filter `action`, `entity`, `page`/`limit` terjaga |
| `POST` | `/api/v1/assistant/chat` | Konsol asisten (tanpa kanal): `{message, branchId?}` → `{runId, intent, mode, answer}` |
| `POST` | `/api/v1/assistant/simulate` | Jalur inbound penuh tanpa kirim (nomor terotorisasi + kanal wajib) |
| `GET` | `/api/v1/assistant/runs/stats` | Agregat 7 hari per hari×intent×mode |
| `GET` | `/api/v1/assistant/channel` | Status gateway (connected, phone, qrDataUrl, lastError) |
| `POST` | `/api/v1/assistant/channel/connect` | Sambung (QR bila belum tertaut, login diam-diam bila sudah) |
| `POST` | `/api/v1/assistant/channel/disconnect` | Putus (sesi disimpan, sambung ulang tanpa QR) |
| `POST` | `/api/v1/assistant/channel/reset` | Hapus sesi (wajib pindai ulang) |
| `GET` | `/api/v1/assistant/authorizations` | Whitelist nomor (`includeRevoked`) |
| `POST` | `/api/v1/assistant/authorizations` | Tambah nomor (normalisasi digit, duplikat aktif 409, revoked reaktif) |
| `PUT` | `/api/v1/assistant/authorizations/{id}` | Ubah nomor aktif |
| `POST` | `/api/v1/assistant/authorizations/{id}/revoke` | Cabut (soft, riwayat utuh) |
| `GET` | `/api/v1/assistant/config` | Tuning + mode efektif |
| `PUT` | `/api/v1/assistant/config` | Simpan tuning (mode hanya rule_based) |
| `GET` | `/api/v1/health` | Health check (`apps/api/cmd/server/main.go`) — bukan bagian modul bisnis |
| *static* | `/uploads/*` | Berkas media (foto/bukti) — diserve container yang sama, bukan API modul |

## 7. Konfigurasi Environment

Lihat `.env.example` di root. Minimum yang wajib ada, tanpa nilai rahasia asli:

```env
APP_ENV=development
APP_PORT=8080
APP_NAME=mini-erp
APP_TIMEZONE=Asia/Jakarta
APP_LOCALE=id-ID

DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=
DB_NAME=mini_erp_db
DB_DSN=root:@tcp(localhost:3306)/mini_erp_db

JWT_SECRET=change-me
JWT_ACCESS_EXPIRES_IN=15m
JWT_REFRESH_EXPIRES_IN=7d

CORS_ALLOWED_ORIGINS=http://localhost:5173

PUBLIC_DIR=

STORAGE_DIR=./storage

STORAGE_DRIVER=local
STORAGE_ENDPOINT=https://is3.cloudhost.id
STORAGE_BUCKET=mini-erp
STORAGE_REGION=auto
STORAGE_FORCE_PATH_STYLE=true
STORAGE_ACCESS_KEY_ID=
STORAGE_SECRET_ACCESS_KEY=
STORAGE_ROOT_PREFIX=vioni
STORAGE_UPLOAD_ROOT_PREFIX=
STORAGE_PRODUCT_PREFIX=
STORAGE_PAYMENT_TRANSFER_PREFIX=
STORAGE_DELIVERY_PROOF_PREFIX=
STORAGE_SIGNED_URL_TTL_SECONDS=300
STORAGE_PUBLIC_BASE_URL=

VITE_API_BASE_URL=
```

Penyimpanan objek mengikuti alamat legacy (IDCloudHost S3, bucket `mini-erp`):
`STORAGE_DRIVER=s3` memakai endpoint/bucket/kunci di atas; kunci objek mengikuti
tata letak legacy `{prefix}/{uuid}.{ext}` (produk) dan `{prefix}/{YYYY-MM-DD}/{uuid}.{ext}`
(bukti/transfer) dengan prefix `{root}/{env}/upload/…`. Bucket bersifat privat —
baca file lewat URL presigned (TTL `STORAGE_SIGNED_URL_TTL_SECONDS`), kecuali
`STORAGE_PUBLIC_BASE_URL` diisi. Driver `local` menyimpan di `STORAGE_DIR` dan
melayani `/uploads/*` (cocok untuk dev tanpa kredensial).

`CORS_ALLOWED_ORIGINS` wajib diisi eksplisit (daftar origin, bukan `*`) — lihat
[SYSTEM_DESIGN.md §9](SYSTEM_DESIGN.md#9-keamanan--isolasi-data) dan
[PRD.md §5](PRD.md#5-keputusan-bisnis-yang-perlu-dikonfirmasi-sebelumselagi-desain-modul) poin 1.

## 8. Referensi

- [SYSTEM_DESIGN.md §6](SYSTEM_DESIGN.md#6-response-envelope--konvensi-api) — alasan arsitektur di
  balik konvensi ini
- `.claude/rules/api-standard.md` — aturan yang ditegakkan Claude Code saat menulis handler baru
- [legacy-reference/shared/shared-business-rules.md](legacy-reference/shared/shared-business-rules.md) —
  konvensi lama (envelope `{code, info, data}`, dll.) sebagai pembanding, **bukan** standar yang
  dipakai di sistem baru
