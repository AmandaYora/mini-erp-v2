# API Contract — Mini ERP (Revamp)

Tabel §6 mendaftar **setiap** rute `/api/v1` yang terdaftar di router (ditambah probe `/healthz`, `/readyz`,
`/api/v1/health*`, `/api/v1/metrics` yang dipasang `server.go`, dan `/media/*`). `node scripts/contract-check.mjs`
mencocokkan setiap panggilan frontend dan kode izin dengan router (`--list` menampilkan rute yang tidak dipanggil web);
rute baru wajib ditambahkan di sini bersama kodenya. Jangan menuliskan endpoint yang belum ada. Jumlah rute terkini:
[STATUS.md](STATUS.md). Terakhir disinkronkan penuh dengan kode 2026-10-19 (DOC-02): selisih rute backend vs tabel ini = nol.

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
  seperti sistem lama (yang punya bug di jalur ini, lihat KI-26). Parameter `?limit=` yang berupa
  batas tampilan top-N (dashboard, marjin per produk) memakai `pagination.ParseDisplayLimit` dengan
  batas atas eksplisit.
- Seluruh body envelope (sukses, galat, 401/403/500/503 tetap) ditulis hanya oleh `shared/response`;
  literal `{"success":` di luar paket itu ditolak arch-check (Rule M).

### Tanggal & waktu

- **Semua waktu di response adalah jam dinding WIB (Asia/Jakarta) tanpa zona**:
  `"YYYY-MM-DD hh:mm:ss"`, atau `"YYYY-MM-DD"` untuk tanggal saja. Klien menampilkan angkanya apa
  adanya (tidak dikonversi menurut zona perangkat). Tanggal dokumen memang tersimpan WIB; stempel
  sistem (`createdAt`, `movedAt`, `dispatchedAt`, `confirmedAt`, `creditApprovedAt`, `archivedAt`,
  `lastLoginAt`, `updatedAt`, `lastSeenAt`, `ignoredAt`) tersimpan UTC dan dikonversi di lapisan
  presentation (`timeutil.FormatWIB` / `WireTimeWIB`). Konvensi kolom: `knowledge/DATABASE_GUIDE.md`.
- Filter tanggal (`dateFrom`/`dateTo`, `from`/`to`) adalah hari kalender WIB, inklusif; format salah
  ditolak 400.

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

Semua modul di bawah sudah terimplementasi; rute konkret ada di tabel setelah peta ini. Tambahkan
baris di sini bersamaan dengan rute baru (jangan mendokumentasikan endpoint yang belum ditulis kodenya).

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
| `GET` | `/api/v1/auth/me` | Identitas + cabang aktif (`branch.address`/`branch.city` untuk kop dokumen cetak) + permission segar + profil perusahaan |
| `POST` | `/api/v1/auth/switch-branch` | Pindah cabang aktif (cabang nonaktif = 403 berpesan) |
| `POST` | `/api/v1/auth/switch-role` | Pindah role aktif milik sendiri |
| `GET` | `/api/v1/users` | Daftar pengguna (`page`, `limit` terjaga, `search` per kata, `status`); tiap baris memuat `roles[{id,name}]`, `branches[{id,name,isDefault}]`, `displayTitle`, `lastLoginAt` (`null` = belum pernah login) dalam jumlah query konstan (U-01) |
| `POST` | `/api/v1/users` | Tambah pengguna (transaksi: akun+role+cabang) |
| `GET` | `/api/v1/users/{id}` | Detail pengguna |
| `PUT` | `/api/v1/users/{id}` | Ubah pengguna (transaksi) |
| `PATCH` | `/api/v1/users/{id}/status` | Aktif/nonaktif (tak bisa menonaktifkan diri sendiri) |
| `POST` | `/api/v1/users/{id}/password` | Ganti password (min. 8) |
| `GET` | `/api/v1/roles` | Daftar role |
| `GET` | `/api/v1/roles/{id}` | Detail role + `permissions[]` (kode izin tersimpan) dan `locked` (owner/superadmin: hanya-baca). Perlu `roles.view`; role superadmin = 404 bagi non-superadmin |
| `POST` | `/api/v1/roles` | Tambah role kustom (kode unik, tak bisa diubah setelahnya) |
| `PUT` | `/api/v1/roles/{id}` | Ubah nama/deskripsi role |
| `DELETE` | `/api/v1/roles/{id}` | Hapus role kustom yang tak dipakai (role sistem dilindungi) |
| `PUT` | `/api/v1/roles/{id}/permissions` | Ganti total izin role (kode tak dikenal = 400 sebelum apa pun dihapus) |
| `GET` | `/api/v1/permissions` | Katalog permission + nama tampilan (untuk UI kelola role) |
| `GET` | `/api/v1/branches` | Daftar cabang |
| `POST` | `/api/v1/branches` | Tambah cabang (transaksi: cabang + seluruh sequence nomor) |
| `GET` | `/api/v1/branches/{id}` | Detail cabang |
| `GET` | `/api/v1/branches/my-access` | Cabang AKTIF yang boleh diakses pemanggil + penanda default (cukup login; tanpa `branches.view`, tak dipaginasi) |
| `PUT` | `/api/v1/branches/{id}` | Ubah/tutup/buka cabang (kode dikunci bila sudah ada dokumen terbit) |
| `GET` | `/api/v1/company/profile` | Profil perusahaan (null bila belum diisi) |
| `PUT` | `/api/v1/company/profile` | Simpan profil (upsert baris tunggal) |
| `GET` | `/api/v1/company/settings` | Seluruh pengaturan operasional |
| `PUT` | `/api/v1/company/settings` | Simpan pengaturan (gabung per kunci, tak pernah ganti-total) |
| `GET` | `/api/v1/company/settings/definitions` | Pengaturan terdaftar (kunci, jenis, default, nilai yang diizinkan) untuk UI |
| `GET` | `/api/v1/company/print-profile` | Allow-list profil untuk cetak; cukup login + cabang (tanpa `company.view`) |
| `GET` | `/api/v1/products` | Daftar produk (search per kata berperingkat relevansi, filter kategori/status; tanpa `status` = semua). Baris membawa `archivedAt` (produk arsip), `stockAvailable`/`stockUom`/`belowMinStock` (satuan jual; hanya sesi dengan `stock.view` + cabang aktif, satu query grup) dan `purchasePrice` (hanya sesi dengan `products.update` atau `purchasing.view`; selain itu kunci tidak dikirim) |
| `POST` | `/api/v1/products` | Tambah produk (transaksi: produk + varian; varian default otomatis) |
| `GET` | `/api/v1/products/search-options` | Opsi ringkas untuk dropdown/POS. Pencarian berperingkat (kode persis, prefix kode, prefix nama, sisanya). `productId=` memulihkan satu opsi terpilih (juga produk nonaktif/arsip). `preferStock=true` menaikkan produk ber-stok tersedia di cabang sesi di dalam peringkat relevansi yang sama |
| `GET` | `/api/v1/products/scan?code=` | Arti satu kode pindai (barcode/kode varian/kode produk) → `{id, code, name, status, via, variantRequired, variantId?}`; izin `products.view` saja; 404 tak dikenal/ambigu, 400 kode kosong |
| `GET` | `/api/v1/products/{id}` | Detail produk + varian + kategori + media |
| `GET` | `/api/v1/products/{id}/qr` | Label QR (PNG); `?variantId=` memilih varian, `?size=` 128–1024 piksel (default 256) |
| `GET` | `/api/v1/products/qr-codes/export` | Seluruh katalog sebagai ZIP PNG QR, berfolder per kategori |
| `PUT` | `/api/v1/products/{id}` | Ubah produk (transaksi; UOM/tipe terkunci bila ada riwayat stok) |
| `POST` | `/api/v1/products/{id}/archive` | Arsipkan produk |
| `GET` | `/api/v1/product-categories` | Daftar kategori |
| `POST` | `/api/v1/product-categories` | Tambah kategori (`sortOrder` opsional 0-9999) |
| `PUT` | `/api/v1/product-categories/{id}` | Ubah kategori (kode tak bisa diubah; `sortOrder` dihilangkan = tetap). Daftar kategori berurut `sortOrder`, lalu nama |
| `POST` | `/api/v1/product-categories/{id}/archive` | Arsipkan kategori kosong |
| `GET` | `/api/v1/products/{id}/media` | Daftar foto (URL lokal bertanda tangan `/media/*?exp&sig`, kedaluwarsa; atau presigned S3 sesuai driver) |
| `POST` | `/api/v1/products/{id}/media` | Unggah foto (multipart, gambar ≤5MB). Field form opsional `sort_order` (0-9999) dan `is_primary` (true/false); nilai tak sah = 400 sebelum berkas disimpan |
| `POST` | `/api/v1/products/{id}/media/{mediaId}/primary` | Jadikan foto utama |
| `DELETE` | `/api/v1/products/{id}/media/{mediaId}` | Hapus foto (baris + berkas) |
| `GET` | `/api/v1/product-imports/template` | Unduh template Excel |
| `POST` | `/api/v1/product-imports/preview` | Pratinjau validasi (tanpa tulis) + galat per baris |
| `POST` | `/api/v1/product-imports/commit` | Impor resumable (sisip valid, lewati sudah-ada, laporkan gagal) |
| `GET` | `/api/v1/customers` | Daftar customer (search per kata; baris membawa `archivedAt`) |
| `POST` | `/api/v1/customers` | Tambah customer (kode opsional → otomatis anti-kembar) |
| `GET` | `/api/v1/customers/{id}` | Detail customer + alamat |
| `PUT` | `/api/v1/customers/{id}` | Ubah customer (kode tak bisa diubah; member tak hilang diam-diam) |
| `POST` | `/api/v1/customers/{id}/archive` | Arsipkan customer (buku alamat ikut terkunci) |
| `POST` | `/api/v1/customers/{id}/restore` | Pulihkan customer |
| `GET` | `/api/v1/suppliers` | Daftar supplier (baris membawa `archivedAt`) |
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
| `GET` | `/api/v1/member-types/{id}/customer-count` | Dampak arsip: `{memberTypeId, activeCustomers}` (pelanggan aktif pada tipe; izin `member_types.view`; 404 bila tipe tak ada) |
| `POST` | `/api/v1/member-types` | Tambah tipe member (aturan persen/nominal + pembulatan berpasangan) |
| `PUT` | `/api/v1/member-types/{id}` | Ubah tipe member (kode tak bisa diubah) |
| `POST` | `/api/v1/member-types/{id}/archive` | Arsipkan tipe member |
| `POST` | `/api/v1/member-types/{id}/restore` | Pulihkan tipe member |
| `POST` | `/api/v1/pricing/quote` | Quote harga member per keranjang (gagal eksplisit, degradasi berpenanda). Body `customerId` ATAU `memberTypeId` (tipe eksplisit tanpa customer; keduanya = 400; tipe tak ada = 404; tipe arsip = harga standar + `memberInactive`) |
| `GET` | `/api/v1/stock/locations` | Daftar lokasi cabang (sistem + manual). Query `status`, `branchId` (K-06: lokasi cabang lain, mis. tujuan transfer; hanya bila user punya akses ke cabang itu, jika tidak `403`; `branchId` tak valid `400`; tanpa `branchId` = cabang sesi) |
| `POST` | `/api/v1/stock/locations` | Tambah lokasi; atribut alokasi opsional `isPickingArea`, `isPrimary`, `sortOrder` (ST-01) |
| `PUT` | `/api/v1/stock/locations/{id}` | Ubah lokasi (anti-siklus). `code` opsional boleh diganti (`409` bila bentrok, diaudit before/after); atribut `isPickingArea`/`isPrimary`/`sortOrder` (nil = tak berubah); lokasi sistem menjaga kode/nama/induk tetapi lokasi default boleh diatur atributnya, lokasi rusak tidak (ST-01) |
| `POST` | `/api/v1/stock/locations/{id}/archive` | Arsipkan lokasi kosong tanpa anak |
| `GET` | `/api/v1/stock/balances` | Saldo per produk |
| `GET` | `/api/v1/stock/balances/{productId}` | Saldo satu produk per varian/lokasi |
| `GET` | `/api/v1/stock/movements` | Riwayat mutasi (filter `type` in/out + tanggal inklusif); tiap baris memuat `balanceBefore`, `balanceAfter` (null pada baris lama), `notes`, `type`, `movedAt` (ST-04) |
| `POST` | `/api/v1/stock/scan-product` | Pindai barcode → produk + saldo |
| `POST` | `/api/v1/stock/allocations/suggest` | Saran alokasi + shortfall |
| `POST` | `/api/v1/stock/reservations/hold` | Tahan stok (idempoten per kunci, TTL) |
| `POST` | `/api/v1/stock/reservations/release` | Lepas tahanan (idempoten) |
| `POST` | `/api/v1/stock/adjustments` | Koreksi (di atas ambang perlu approver stock.approve) |
| `GET` | `/api/v1/stock/transfers` | Daftar dokumen transfer. Query `status`, `direction` (`out` ke cabang lain, `in` dari cabang lain, `internal` dalam cabang; relatif cabang sesi, ST-06); baris memuat `driverName`, `vehiclePlate`, `transferDate` |
| `POST` | `/api/v1/stock/transfers` | Buat draf transfer (nomor TRF-…); `driverName` (wajib bila antar cabang, BR-20), `vehiclePlate`, `transferDate` (YYYY-MM-DD, default hari ini WIB) tersimpan di dokumen dan tercetak di `/print/transfers/{id}` (ST-02); `toBranchId` selain cabang sesi wajib cabang yang dapat diakses user, jika tidak `403` (K-06) |
| `POST` | `/api/v1/stock/transfers/move-location` | Pindah lokasi satu cabang dalam satu panggilan; transfer yang dikembalikan sudah diterima |
| `GET` | `/api/v1/stock/transfers/{id}` | Detail transfer |
| `POST` | `/api/v1/stock/transfers/{id}/dispatch` | Kirim (potong asal, atomik) |
| `POST` | `/api/v1/stock/transfers/{id}/receive` | Terima (tambah tujuan) |
| `POST` | `/api/v1/stock/transfers/{id}/cancel` | Batalkan (draf langsung, terkirim via reversal). Body `{reason}` wajib (≤ 255 karakter, 400 bila kosong); alasan, waktu, dan pelaku tersimpan di dokumen (`cancelReason`, `cancelledAt`, `cancelledBy` di view) dan di audit |
| `GET` | `/api/v1/stock/damaged` | Saldo lokasi rusak, paginasi server (`page`,`limit`, `empty=true` = posisi sudah nol). Respons `data` = objek `{items,total,withStock,empty,totalEstimatedLoss,page,limit,totalPages}` + `meta`; tiap item memuat `estimatedUnitCost` (harga modal rata-rata, fallback harga beli) dan `estimatedLoss` = qty x harga (ST-03) |
| `POST` | `/api/v1/stock/damaged/move-in` | Catat rusak; `locationId` opsional: tanpa lokasi asal, barang dicatat langsung masuk sebagai rusak (datang rusak, ST-06; mutasi `damaged_arrival`, masuk antrian posting dan dijurnal seperti selisih stok lebih, ADR-0020) |
| `POST` | `/api/v1/stock/damaged/restore` | Pulihkan |
| `POST` | `/api/v1/stock/damaged/write-off` | Hapusbukukan (alasan wajib) |
| `GET` | `/api/v1/stock/opening/template` | Template saldo awal (.xlsx, sheet `Stok Awal`, judul Indonesia) berisi daftar barang berstok + lokasi saran (prioritas alokasi); jumlah dikosongkan (ST-05) |
| `POST` | `/api/v1/stock/opening/preview` | Pratinjau (tanpa tulis). Menerima alias sheet (`Stok Awal`/`opening`/`Stock Opening`/`Migrasi Stok`, atau satu-satunya sheet) dan judul Indonesia/Inggris; hanya `.xlsx`. Respons: `validRows`, `skipped`, `errors`, `rows[]` rinci (maks 500), `totalQty`, `totalValue`, `warnings` |
| `POST` | `/api/v1/stock/opening/commit` | Bukukan (resumable) |
| `GET` | `/api/v1/purchase-orders` | Daftar PO cabang. Tiap baris membawa `paidAmount` (uang riil; offset retur tidak termasuk), `returnOffset` (bagian tagihan yang dilunasi retur), `outstandingAmount`, `paymentStatus`, `effectiveDueDate`, `isOverdue`, `returnSummary` (`none`\|`partial`\|`full`, retur terkonfirmasi vs diterima), `createdBy`/`createdByName`, `assignedUserId`/`assignedUserName`. `search` mencocokkan nomor, nomor faktur supplier/pajak, dan nama/kode/telepon/email/alamat supplier (PO-01) |
| `GET` | `/api/v1/purchase-orders/export` | Ekspor daftar PO sesuai filter (`format=xlsx|pdf`). xlsx tiga sheet (PO-02): `Ringkasan` (jumlah per status, total, DPP, pajak, dibayar, sisa), `Daftar Order` (24 kolom: Dibayar/Sisa/DPP/Status Bayar/Status Finansial/Metode Bayar/Status Retur/faktur supplier & pajak/Dibuat Oleh/Penanggung Jawab) dan `Detail Item` |
| `POST` | `/api/v1/purchase-orders` | Buat draf PO (nomor ORD-…/PB/…, snapshot harga). PO-03: `supplierInvoiceNumber`/`supplierInvoiceDate` (dipakai PPN Masukan), `taxInvoiceNumber`/`taxInvoiceDate` (faktur pajak), `assignedUserId` (penanggung jawab, user aktif ber-akses cabang) dan `items[].taxRate` (tarif pajak baris 0–100 yang menimpa tarif order; hanya bila order kena pajak) |
| `GET` | `/api/v1/purchase-orders/{id}` | Detail PO |
| `PUT` | `/api/v1/purchase-orders/{id}` | Ubah draf PO |
| `POST` | `/api/v1/purchase-orders/{id}/confirm` | Konfirmasi PO |
| `POST` | `/api/v1/purchase-orders/{id}/cancel` | Batalkan PO |
| `GET` | `/api/v1/sales-orders` | Daftar SO cabang (filter status/kanal/termin/pihak/tanggal dan `paymentStatus=unpaid\|partial\|paid\|overdue`, disaring server dari data pembayaran; draf/batal tak pernah cocok; rentang terlalu lebar >30.000 order → 400 minta dipersempit). Tiap baris membawa ringkasan bayar bersih-retur: `paidAmount` (kas riil), `outstandingAmount` (≥0), `paymentStatus`, `isOverdue`, `originalTotal`, `returnAdjustment`, `netTotal`, `returnOffset/Refunded/Credit`, `refundDue`, `exchangeChargeDue`, `financialStatus`, plus `createdBy/createdByName`, `assignedUserId/Name`, `creditApprovedBy/Name/At` |
| `GET` | `/api/v1/sales-orders/export` | Ekspor daftar SO sesuai filter (`format=xlsx|pdf`, termasuk `paymentStatus`). xlsx 3 sheet streaming (keyset 500/halaman): **Ringkasan** (jumlah/nilai/retur/total bersih/DPP/pajak/dibayar/sisa/jatuh tempo/penjualan bersih; margin kotor hanya untuk `finance.view` dan bila `from`+`to` terisi), **Daftar Order** (28 kolom: dibayar, sisa, metode, kasir, dibuat oleh, penanggung jawab, sopir, plat, faktur pajak, DPP, status finansial …), **Detail Item**; PDF 12 kolom |
| `GET` | `/api/v1/sales-orders/by-client-ref/{ref}/state` | Posisi keranjang POS (`client_ref`) agar dapat dilanjutkan dari `nextStep` |
| `POST` | `/api/v1/sales-orders` | Buat draf SO (harga dari server: member/ katalog). Payload opsional: `assignedUserId` (penanggung jawab, harus pengguna aktif), alamat kirim sekali pakai `shipToLabel/shipToRecipient/shipToPhone/shipToAddress` (dipakai bila `shipToAddressId`=0; alamat wajib bila ada rincian; walk-in boleh; disalin ke order, tidak ke buku alamat), `notes` (juga dari POS) |
| `GET` | `/api/v1/sales-orders/{id}` | Detail SO (ringkasan bayar bersih-retur seperti daftar, nama pembuat/penanggung jawab/penyetuju kredit, `cashierName` = pencatat pembayaran, cadangan pembuat order POS; `partyPhone`/`partyAddress` = kontak pelanggan untuk dokumen cetak tanpa alamat kirim). Koreksi: `editMode` (`full` = draf atau terkonfirmasi tanpa dokumen turunan; `header` = sudah ada surat jalan/pembayaran/retur, hanya kolom ringan; `none`), `editLockReason` (pesan penjaga server), `canCloseShort`; jejak `cancelReason/cancelledAt/cancelledBy(Name)` dan `shortCloseReason/shortClosedAt/shortClosedBy(Name)` |
| `PUT` | `/api/v1/sales-orders/{id}` | Ubah draf SO (hitung ulang), atau koreksi SO terkonfirmasi: penuh bila tak ada surat jalan/pembayaran/retur aktif; bila ada, hanya kolom ringan (jatuh tempo — wajib untuk termin net —, catatan, alamat kirim, faktur pajak, penanggung jawab; perubahan ekonomi → 409). `headerOnly: true` (dialog "Ubah info order") memaksa jalur kolom ringan walau order bersih — tak pernah menghitung ulang harga, perubahan ekonomi → 409. `reason` opsional (≤255) masuk jejak audit; audit membawa before/after kolom yang berubah (header, total, ringkasan baris) |
| `POST` | `/api/v1/sales-orders/{id}/confirm` | Konfirmasi SO (stok bergerak saat delivery) |
| `POST` | `/api/v1/sales-orders/{id}/cancel` | Batalkan SO. Body opsional `{reason}` (≤255; UI mewajibkan) disimpan ke `cancel_reason` + `cancelled_at/by` dan jejak audit |
| `GET` | `/api/v1/sales-orders/{id}/close-short/preview` | Pratinjau "tutup sisa order" (tanpa tulis, builder sama dengan eksekusi): `lines[]` (`orderedQty`, `deliveredQty`, `oldLineTotal`, `newLineTotal`, `removed`), `oldGrandTotal`, `newSubtotal/newDiscountTotal/newTaxTotal/newGrandTotal`, `paidAmount`, `outstandingAfter`, `refundDueAfter`. Permission `sales.archive` |
| `POST` | `/api/v1/sales-orders/{id}/close-short` | "Tutup sisa order" `{reason}` (wajib, ≤255): SO terkonfirmasi dengan ≥1 surat jalan terkonfirmasi dan tanpa surat jalan draf. Qty tiap baris dipotong ke qty terkirim (baris tanpa kiriman dihapus), dihitung ulang dari harga/diskon/pajak baris sendiri (diskon nominal pro-rata), order `completed` + `short_close_reason/at/by`. Tanpa jurnal (pendapatan & piutang sudah dijurnal per surat jalan); kelebihan bayar tidak ditolak, tampil sebagai `refundDue`. Diulang = replay. Permission `sales.archive` |
| `POST` | `/api/v1/sales-orders/{id}/approve-credit` | COD **atau prabayar** → net + tempo (sama dengan `SwitchToNet`); menyimpan `creditApprovedBy/At`; order bertermin net ditolak 409 |
| `GET` | `/api/v1/goods-receipts` | Daftar penerimaan cabang (filter PO). GR-01: tiap baris membawa `orderNumber`, `lineCount`, `receivedBy`/`receivedByName` (tiga baca grup, tanpa N+1) |
| `POST` | `/api/v1/goods-receipts` | Terima PO (tambah stok, lunasi PO bila penuh). GR-02: `items[].locationId` kosong → lokasi default cabang; `settlement.notes` (maks 255) pada `pay_now`; respons memuat `payment: {id, number}` bila pembayaran dibuat, bukti transfer dilampirkan lewat `POST /payments/{id}/proof` |
| `GET` | `/api/v1/goods-receipts/{id}` | Detail penerimaan |
| `POST` | `/api/v1/goods-receipts/{id}/complete` | Ulangi posting stok penerimaan yang tertunda (idempoten) |
| `POST` | `/api/v1/goods-receipts/{id}/discard` | Batalkan penerimaan tertunda, membalik stok yang sudah bergerak. Body `{reason}` wajib (≤ 255 karakter, 400 bila kosong); alasan, waktu, dan pelaku tersimpan di dokumen (`cancelReason`, `cancelledAt`, `cancelledBy` di view) dan di audit |
| `GET` | `/api/v1/deliveries` | Daftar SJ cabang (filter SO/status) |
| `GET` | `/api/v1/deliveries/work-queue` | Antrean kerja gudang (SO siap kirim + draf menunggu); baca-saja. `search`, `from`, `to` dijalankan di SQL sebelum batas daftar pada kedua tab; order prepaid belum lunas disembunyikan; kartu memuat `itemCount`, tab menunggu memuat `hasSignedDocument` (DL-02) |
| `POST` | `/api/v1/deliveries` | Buat draf SJ (stok belum bergerak) |
| `GET` | `/api/v1/deliveries/{id}` | Detail SJ |
| `POST` | `/api/v1/deliveries/{id}/confirm` | Konfirmasi (kurangi stok, lunasi SO bila penuh) |
| `POST` | `/api/v1/deliveries/{id}/cancel` | Batalkan draf (terkonfirmasi via retur L6; SJ pengganti ditolak, ditarik lewat retur). Body `{reason}` wajib (≤ 255 karakter, 400 bila kosong); alasan, waktu, dan pelaku tersimpan di dokumen (`cancelReason`, `cancelledAt`, `cancelledBy` di view) dan di audit |
| `POST` | `/api/v1/deliveries/{id}/proof` | Unggah bukti kirim (gambar ≤5MB), multipart `file` + `purpose` = `delivery_signed_document` (scan/foto SJ fisik bertanda tangan; wajib untuk konfirmasi bila `delivery_require_evidence` aktif) atau `delivery_location_photo` (opsional, default) (DL-01) |
| `GET` | `/api/v1/deliveries/{id}/proofs` | Daftar bukti kirim; tiap berkas memuat `purpose` |
| `GET` | `/api/v1/payments` | Daftar pembayaran cabang; tiap baris membawa `allocations` (order + nominal), dimuat dalam jumlah query tetap |
| `POST` | `/api/v1/payments` | Catat pembayaran (alokasi eksplisit atau FIFO otomatis; refund via tipe `sales_return`/`purchase_return` — arah berbalik, satu pembayaran satu arah; partyId 0 = walk-in tunai POS, alokasi wajib eksplisit) |
| `GET` | `/api/v1/payments/{id}` | Detail pembayaran (`partyPhone`/`partyAddress` = kontak pihak untuk bukti pembayaran) |
| `POST` | `/api/v1/payments/{id}/cancel` | Batalkan pembayaran (saldo terbuka lagi). Body opsional `{reason}` (maks 255; web selalu mengirim): disimpan sebagai `cancelReason` + `cancelledAt`/`cancelledBy` di semua kaki dan di audit `payments.cancel`. Ditolak 409 bila order tunai/COD yang sudah selesai jadi belum lunas → pakai koreksi |
| `POST` | `/api/v1/payments/{id}/correct` | **Koreksi pembayaran** (salah input): `{method, amount, paidAt?, reference?, amountTendered?, notes?, reason}` (`reason` wajib 5–255). Membuat pembayaran pengganti (pihak, arah, dan dokumen sama; nominal dibagi ke alokasi asli berurutan, dihitung seolah yang asli sudah batal; `correctsPaymentId` = asli; kunci idempoten `correct:<id>`) dan membatalkan yang asli dengan alasan itu dalam satu transaksi; jurnal yang asli dibalik, pengganti masuk antrean posting. 201 = pengganti baru, 200 = permintaan sama diulang. 409: kaki offset/pakai saldo, sudah batal, sudah dikoreksi dengan data lain, nominal melebihi tagihan, atau order tunai/COD selesai jadi belum lunas. Butuh `payment.create` + `payment.archive` (+ izin refund retur bila mengalokasi ke retur); audit `payments.correct` (sebelum/sesudah + alasan). Detail pembayaran membawa `correctsPaymentNumber` dan `correctedById/Number` |
| `GET` | `/api/v1/payments/party-balances` | Saldo tagihan satu pihak (live) |
| `GET` | `/api/v1/payments/open-parties` | Daftar semua pihak berpiutang/berhutang (PY-01). Query `side` (`receivable` default / `payable`), `search` (nama/kode/HP), `page`, `limit`. `data` = `{items:[{partyId,code,name,side,orderCount,oldestDate,outstanding}],total,totalOutstanding,page,limit,totalPages}` + `meta`; tertua dulu; dihitung dari order terbuka dikurangi alokasi (tanpa retur); terisolasi cabang sesi |
| `GET` | `/api/v1/payments/party-ledger` | Buku pembayaran satu pihak |
| `POST` | `/api/v1/payments/{id}/proof` | Unggah bukti bayar (gambar ≤5MB) |
| `GET` | `/api/v1/payments/{id}/proofs` | Daftar bukti bayar |
| `POST` | `/api/v1/payments/settle-credit` | Offset retur ke tagihan tanpa kas (dua kaki kontra, method "offset") |
| `POST` | `/api/v1/payments/use-credit` | **Pakai saldo uang muka** (N-41): `{partyId, returnType, returnId, orderType, orderId, amount, notes?}`. Pasangan kaki dalam satu transaksi (kaki retur method `offset` tak dijurnal; kaki tagihan method `advance` dijurnal Dr Uang Muka Pelanggan/Cr Piutang atau Dr Hutang/Cr Uang Muka Supplier). Kedua kaki memuat `creditSourceType/Id` = retur sumber. `amount` ≤ sisa saldo retur dan ≤ sisa tagihan; membatalkan salah satu kaki membatalkan keduanya dan membalik jurnal; retur yang saldonya terpakai tidak bisa dibatalkan. Butuh `payment.create` |
| `GET` | `/api/v1/payments/credit-balances?partyId=` | Retur milik pihak yang menyimpan saldo: `items[{returnType, returnId, number, credited, used, remaining}]` + `totalRemaining` (`payment.view`) |
| `GET` | `/api/v1/sales-returns` | Daftar retur cabang. RT-01: `orderNumber` terisi di tiap baris; `search` mencocokkan nomor, catatan, alasan, nomor order, dan nama/telepon pelanggan |
| `POST` | `/api/v1/sales-returns` | Buat draf retur (batas = terkirim−sudah diretur; mode tukar via `returnMode` + `replacementItems`) |
| `GET` | `/api/v1/sales-returns/context` | Sisa qty per baris SO + daftar lokasi (masukan form retur) |
| `POST` | `/api/v1/sales-returns/preview` | Pratinjau angka retur (tanpa tulis, builder sama dengan create) + **perkiraan penyelesaian** (K-04d): `difference`, `owedToCustomer`, `chargeToCustomer`, `orderOutstanding`, `receivableReduction`, `externalRemaining`, `allowedSettlementTypes`, `hasParty` — angka sama dengan hasil konfirmasi |
| `GET` | `/api/v1/sales-returns/{id}` | Detail retur (+ mode, barang pengganti, penyelesaian, posisi penyelesaian K-04c: `externalRemaining`, `chargeRemaining`, `allowedSettlementTypes`) |
| `POST` | `/api/v1/sales-returns/{id}/confirm` | Konfirmasi (stok kembali). RT-02: pada order kanal POS, retur tukar langsung menyerahkan barang pengganti di konter dalam langkah yang sama (stok keluar, `replacementDeliveryStatus` = `confirmed`; SJ pengganti dibuat dan dikonfirmasi otomatis tanpa supir). Body opsional `settlement: {mode: "pay_now", …}` menagih selisih tukar lebih mahal saat itu juga (butuh `payment.create`) |
| `POST` | `/api/v1/sales-returns/{id}/cancel` | Batalkan (draf langsung, terkonfirmasi via reversal). Body `{reason}` wajib (≤ 255 karakter, 400 bila kosong); alasan, waktu, dan pelaku tersimpan di dokumen (`cancelReason`, `cancelledAt`, `cancelledBy` di view) dan di audit |
| `POST` | `/api/v1/sales-returns/{id}/replacement-deliveries` | Kirim barang pengganti (draf SJ pengganti; stok keluar saat konfirmasi). RT-03: `driverName` dan `warehouseStaffName` wajib (422), kecuali setting `delivery_require_evidence=false`. RT-04: harga modal pengganti dijaga — rata-rata finance dulu, lalu harga beli katalog |
| `POST` | `/api/v1/sales-returns/{id}/replacement-deliveries/{deliveryId}/confirm` | Konfirmasi SJ pengganti (stok keluar). Body opsional `settlement: {mode: "pay_now", amount?, method?, paymentDate?, reference?, amountTendered?}` menagih selisih tukar yang lebih mahal secara atomik dengan konfirmasi (K-04e; butuh `payment.create`; hanya retur tukar ber-selisih > 0, nominal ≤ selisih terbuka; pembayaran gagal → SJ tetap draf, bisa diulang idempoten `sjr:<id>`) |
| `POST` | `/api/v1/sales-returns/{id}/replacement-delivery/cancel` | Tarik pengiriman pengganti yang salah (`salesreturn.archive`): retur tukar berstatus pengganti `dispatched` yang SJ penggantinya masih **draf** — SJ itu dibatalkan (stok yang terlanjur keluar dibalik) dan status pengganti kembali `pending`, lalu kirim ulang pengganti yang benar atau batalkan retur. SJ pengganti terkonfirmasi ditolak (409). Idempoten (ulang setelah sukses = 200). Body `{reason}` wajib (≤ 255), tercatat di SJ dan audit `sales_returns.cancel_replacement`; respons returnView |
| `POST` | `/api/v1/sales-returns/{id}/settlements` | Catat penyelesaian (K-04c: jenis & nominal menurut arah selisih — retur murni/tukar lebih murah: `refund` \| `customer_credit` ≤ `externalRemaining`; tukar lebih mahal: `collect_payment` ≤ `chargeRemaining`; `reduce_receivable` otomatis, ditolak manual; `customer_credit` butuh order ber-customer dan **dijurnal** Dr Piutang/Cr Uang Muka Pelanggan, K-04a; `refund` butuh `salesreturn.refund`) |
| `GET` | `/api/v1/purchase-returns` | Daftar retur cabang. RT-01: `orderNumber` terisi; `search` mencocokkan nomor, catatan, alasan, nomor PO, dan pemasok |
| `POST` | `/api/v1/purchase-returns` | Buat draf retur (batas = diterima−sudah diretur) |
| `GET` | `/api/v1/purchase-returns/context` | Sisa qty per baris PO + daftar lokasi. RT-04: tiap baris membawa `defaultLocationId` (lokasi penerimaan terbaru) dan konteks `firstReceivedAt`; `POST` retur menolak `returnDate` lebih awal dari tanggal terima paling awal |
| `POST` | `/api/v1/purchase-returns/preview` | Pratinjau angka retur (tanpa tulis) + perkiraan penyelesaian (K-04d): `orderOutstanding`, `payableReduction`, `externalRemaining`, `allowedSettlementTypes` |
| `GET` | `/api/v1/purchase-returns/{id}` | Detail retur (+ penyelesaian, `payableReduction`, posisi K-04c: `externalRemaining`, `allowedSettlementTypes`) |
| `POST` | `/api/v1/purchase-returns/{id}/confirm` | Konfirmasi (stok keluar kembali) |
| `POST` | `/api/v1/purchase-returns/{id}/cancel` | Batalkan (draf langsung, terkonfirmasi via reversal). Body `{reason}` wajib (≤ 255 karakter, 400 bila kosong); alasan, waktu, dan pelaku tersimpan di dokumen (`cancelReason`, `cancelledAt`, `cancelledBy` di view) dan di audit |
| `POST` | `/api/v1/purchase-returns/{id}/settlements` | Catat penyelesaian — kamus **pembelian** (K-04b): `refund` \| `supplier_credit` ≤ `externalRemaining` (kelebihan di atas sisa hutang); `reduce_payable` otomatis, ditolak manual; tipe penjualan ditolak. `supplier_credit` **dijurnal** Dr Uang Muka Supplier/Cr Hutang (K-04a); `refund` butuh `purchasereturn.refund` |
| `GET` | `/api/v1/finance/accounts` | Daftar akun |
| `POST` | `/api/v1/finance/accounts` | Tambah akun (kode tak bisa diubah) |
| `PUT` | `/api/v1/finance/accounts/{id}` | Ubah akun |
| `POST` | `/api/v1/finance/accounts/{id}/archive` | Arsipkan akun kosong tak terpetakan |
| `GET` | `/api/v1/finance/account-mappings` | Pemetaan akun otomatis |
| `PUT` | `/api/v1/finance/account-mappings` | Ubah pemetaan (kunci dikenal saja) |
| `GET` | `/api/v1/finance/periods` | Daftar periode |
| `POST` | `/api/v1/finance/periods/close` | Tutup periode (kunci tanggal) |
| `POST` | `/api/v1/finance/periods/reopen` | Buka periode. Body `{year, month, reason}`: `reason` WAJIB (maks 255) dan tersimpan di diff audit `fiscal_periods.reopen` (FN-05) |
| `GET` | `/api/v1/finance/posting/preview` | Pratinjau jurnal dokumen (tanpa tulis). `docType` kini juga `sales_return_settlement` / `purchase_return_settlement` (K-04a: saldo customer/supplier dari retur; `docId` = id baris penyelesaian; ikut antrean posting, batas cutover, orphan bila retur dibatalkan) |
| `POST` | `/api/v1/finance/posting/post` | Posting dokumen (idempoten) |
| `GET` | `/api/v1/finance/posting-sources` | Dokumen terkonfirmasi yang belum dijurnal, dihitung langsung (tanpa tabel antrean). Tiap item memuat `ignored`/`ignoreReason`/`ignoredAt` (FN-01) serta `amount`, `amountKind` (`document` = nilai dokumen, `order` = total pesanan induk untuk SJ/penerimaan), `partyId`/`partyName`, `summary` (FN-10) |
| `GET` | `/api/v1/finance/posting/orphans` | Jurnal dokumen yang sudah dibatalkan (perlu dibalik) |
| `POST` | `/api/v1/finance/posting/post-batch` | Posting beberapa dokumen sekaligus |
| `POST` | `/api/v1/finance/posting/close-day` | Posting semua antrean sampai tanggal tertentu (dokumen diabaikan/periode tertutup tidak ikut). Berjalan per 50 dokumen sampai antrean hari itu habis atau 500 dokumen tercapai; dokumen gagal tidak diulang dalam satu panggilan. `data` = `{results[{docType,docId,entryId?,entryNumber?,error?}], posted, failed, remaining}`; `remaining` = dokumen layak-tutup yang masih antre (gagal atau melewati batas) — UI menampilkan "N diposting, M tersisa — jalankan lagi" (N-52) |
| `GET` | `/api/v1/finance/posting/overview` | Ikhtisar tutup hari (FN-02, `date` opsional, default hari ini WIB; izin `finance.posting.view`): `status` (`kosong`/`belum`/`selesai_catatan`/`lengkap`), `totals` (penjualan, pembelian, retur: `{amount,count}`; `cashIn`, `cashOut`, dihitung dari jurnal hari itu), `tasks[]` dan `backlog.tasks[]` per akar masalah (`missing_cost_basis`, `missing_source_order`, `missing_account_mapping`, `build_failed`) berisi dokumen + pesan, `ready`, `ignored`, `dataWarning.invalidDateCount`; pengelompokan memeriksa maks. 200 dokumen (`probeCapped`) |
| `POST` | `/api/v1/finance/posting/ignore` | Abaikan dokumen macet dari antrian (FN-01): `{docType, docId, reason}` (alasan wajib maks 255; dokumen harus ada di antrian). Tidak memblokir tutup periode (peringatan `ignored_docs`), dilewati posting massal/tutup hari, audit `finance.posting.ignore`. Izin `finance.posting.post` |
| `POST` | `/api/v1/finance/posting/restore` | Pulihkan dokumen yang diabaikan ke antrian: `{docType, docId}`; audit `finance.posting.restore` |
| `GET` | `/api/v1/finance/opening` | Jurnal saldo awal yang sudah dibukukan, atau null bila cabang belum cutover |
| `GET` | `/api/v1/finance/opening/inventory` | Persediaan saldo awal yang dihitung server dari ledger biaya (total, jumlah posisi, jumlah berharga katalog, produk berstok tanpa harga modal, `openingPosted`) — izin `finance.view` |
| `POST` | `/api/v1/finance/opening/preview` | Pratinjau jurnal saldo awal (tanpa tulis); baris Persediaan + Modal Awal ditambahkan server (Persediaan ditampilkan satu baris), respons memuat `inventory` |
| `POST` | `/api/v1/finance/opening/post` | Bukukan saldo awal (idempoten, termasuk bila jurnal `opening` berasal dari ETL). Baris yang dikirim harus seimbang sendiri dan tidak boleh memuat akun Persediaan; `lines` boleh kosong bila hanya ada persediaan. Aturan `partyId` per baris sama dengan jurnal manual (juga untuk `/opening/preview`) |
| `POST` | `/api/v1/finance/opening/complete` | Setelah saldo awal terkunci: beri harga modal produk berstok yang bernilai nol (`items[{productId, variantId, unitCost?}]`, default harga beli katalog); satu jurnal suplemen Persediaan/Modal Awal (`opening_supplement`) + baris ledger biaya `opening_cost` — izin `finance.manage` |
| `POST` | `/api/v1/finance/cost-restatement/preview` | Hitung ulang HPP, dry run (OP-04, sama dengan `cmd/costrebuild` tanpa `--commit`; tidak menulis apa pun): `{date?}` → `RestateReport` (`positions`, `rowsChanged`, `products[{productId, variantId, productCode, productName, rows, valueBefore, valueAfter, delta, offsets}]`, `skipped[{productId, variantId, productCode, productName, reason}]`, `delta`, `gates[{key, ok, detail}]` = `trial_balance`/`inventory_recon`/`pending_documents`/`period_open`, `reconBefore`, `cutoverMovementId`) — izin `finance.manage` |
| `POST` | `/api/v1/finance/cost-restatement/commit` | Hitung ulang HPP: `{date?, reason}` (alasan wajib, maks 500). Semua gerbang harus lolos (409 menyebut gerbangnya); menulis ulang baris keluar di atas batas cutover + SATU jurnal koreksi (`cost_restatement`), audit `finance.cost_restatement` dengan `reason`; respons memuat `entryNumber`, `reconBefore`/`reconAfter` (tanpa selisih: `applied=false`) — izin `finance.close.force` |
| `GET` | `/api/v1/finance/inventory-revaluations` | Riwayat revaluasi harga modal cabang (terpaginasi, terbaru dulu): `{id, date, reason, totalDelta, entryId, entryNumber, createdBy, createdAt, lines[{productId, variantId, productCode, productName, qty, unitCostBefore, unitCostAfter, valueBefore, valueAfter, delta}]}` — izin `finance.view` |
| `GET` | `/api/v1/finance/inventory-revaluations/positions` | `productId` wajib: posisi ledger biaya produk per varian (`qty`, `value`, `averageCost`, `lastReceiptCost` = harga satuan penerimaan terakhir, `catalogCost` = harga beli master per satuan dasar, `baseUom`, `variantName`) — izin `finance.manage` |
| `POST` | `/api/v1/finance/inventory-revaluations/preview` | Pratinjau revaluasi (tanpa tulis): `{productId, variantId, newUnitCost}` atau `items[{productId, variantId, newUnitCost}]` (maks 200; harga per satuan dasar, rupiah bulat ≥ 0), `date?`, `reason?` → `{totalDelta, lines[…]}`; galat sama dengan commit — izin `finance.manage` |
| `POST` | `/api/v1/finance/inventory-revaluations/commit` | Revaluasi harga modal: body seperti preview + `reason` wajib + `clientRef?` (replay dengan kunci yang sama → 200 `replayed`, tanpa tulis). Di bawah kunci biaya: per posisi `delta = round(qty × newUnitCost) − nilai`, satu baris ledger nilai saja (`inventory_revaluation`, qty 0) + SATU jurnal (`inventory_revaluation`) Persediaan vs Selisih Stok (`adjustment`), dibalik bila turun; audit `finance.inventory_revaluation`. Ditolak: qty ≤ 0, tidak ada perubahan, harga negatif, tanggal masa depan/periode tertutup, saldo awal belum diposting (409). Respons 201: `id`, `entryNumber`, `reconBefore`/`reconAfter` — izin `finance.close.force` |
| `GET` | `/api/v1/finance/cutover` | Batas per jenis dokumen: di bawahnya dianggap sudah dijurnal |
| `PUT` | `/api/v1/finance/cutover` | Tetapkan batas cutover |
| `GET` | `/api/v1/finance/integrity` | Bandingkan agregat harian dengan baris jurnal, dan akun Persediaan buku dengan nilai ledger biaya (`inventory`: book/ledger/diff/tolerance/pending/pendingValue/ignoredValue/unexplained/missingCost/emptyValued/emptyValue; ledger = nilai SELURUH posisi biaya termasuk posisi kosong yang masih bernilai; konsisten bila nilai mutlak `unexplained` ≤ toleransi, `unexplained` = diff + nilai ledger dokumen belum dijurnal + diabaikan, ADR-0020). Kesiapan tutup periode (`close/readiness`) menambah cek `inventory_recon` (blocker bila ada selisih tak terjelaskan; peringatan bila selisih seluruhnya dijelaskan dokumen belum dijurnal/diabaikan) dan `missing_cost` serta `empty_valued` (peringatan) |
| `POST` | `/api/v1/finance/integrity/rebuild` | Bangun ulang agregat harian dari baris jurnal |
| `GET` | `/api/v1/finance/close/readiness` | Gerbang kesiapan tutup buku per bulan (`year`, `month`) |
| `POST` | `/api/v1/finance/close/safe-close` | Tutup bulan hanya bila semua gerbang lolos |
| `GET` | `/api/v1/finance/tax-adjustments` | Daftar penyesuaian fiskal; tiap baris memuat `classification`, `classLabel`, `note` (FN-07) |
| `POST` | `/api/v1/finance/tax-adjustments` | Catat penyesuaian fiskal (jurnal bertipe penyesuaian, tak masuk laba komersial). Body tambahan `classification` WAJIB (`positive`/`negative`/`final_income`/`non_object`) dan `note` opsional (maks 1000). Model jurnal tidak berubah (ADR-0018 D6). `partyId` per baris opsional (jurnal fiskal tak dibaca per pihak); bila dikirim diperiksa seperti jurnal manual |
| `GET` | `/api/v1/finance/tax-adjustments/{id}/attachments` | Lampiran (gambar) koreksi fiskal: `[{id,name,mime,sizeBytes,url}]` (izin `finance.tax_adjustment.view`) |
| `POST` | `/api/v1/finance/tax-adjustments/{id}/attachments` | Unggah lampiran (multipart `file`, JPG/PNG/WebP maks 5 MB; izin `finance.tax_adjustment.manage`) |
| `GET` | `/api/v1/finance/journals` | Daftar jurnal |
| `POST` | `/api/v1/finance/journals/manual` | Jurnal manual (imbang wajib). Baris ke akun yang dipetakan `inventory` ditolak 422: Persediaan hanya bergerak bersama mutasi stok (ADR-0020). Baris `lines[]` = `{accountCode, debit, credit, partyId?}`: baris ke akun yang dipetakan `receivable`/`customer_advance` WAJIB `partyId` pelanggan, `payable`/`supplier_advance` WAJIB `partyId` pemasok (422 per baris "baris N: akun … wajib memilih …"); baris akun lain tidak boleh membawa `partyId` (F5) |
| `GET` | `/api/v1/finance/journals/{id}` | Detail jurnal |
| `POST` | `/api/v1/finance/journals/{id}/reverse` | Balik jurnal (alasan wajib) |
| `GET` | `/api/v1/finance/expenses` | Daftar biaya; filter `partyId`; tiap baris memuat `partyId`/`partyName` (FN-10) |
| `POST` | `/api/v1/finance/expenses` | Catat biaya (auto-jurnal); `partyId` opsional (pihak lawan, tercatat juga pada baris beban jurnal). Akun beban = akun yang dipetakan `cogs` ditolak 422 (HPP hanya dari ledger biaya, ADR-0020) |
| `GET` | `/api/v1/finance/expenses/{id}` | Detail biaya |
| `POST` | `/api/v1/finance/expenses/{id}/cancel` | Batalkan biaya (jurnal dibalik) |
| `POST` | `/api/v1/finance/costing/sync` | Sinkronisasi HPP dari mutasi stok |
| `GET` | `/api/v1/finance/costing/uncosted` | Daftar posisi estimasi |
| `GET` | `/api/v1/finance/tax-periods` | Daftar periode pajak |
| `POST` | `/api/v1/finance/tax-periods/close` | Tutup + snapshot SPT |
| `POST` | `/api/v1/finance/tax-periods/reopen` | Buka periode pajak. Body `{year, month, reason}`, `reason` wajib dan masuk diff audit (FN-05) |
| `GET` | `/api/v1/finance/reports/trial-balance` | Neraca saldo per bulan |
| `GET` | `/api/v1/finance/reports/profit-loss` | Laba rugi per rentang; memuat `revenueLines`, `cogsLines`, `expenseLines` (rincian per akun; jumlah tiap golongan = angka ringkasan, FN-03) |
| `GET` | `/api/v1/finance/reports/balance-sheet` | Neraca per tanggal |
| `GET` | `/api/v1/finance/reports/general-ledger` | Buku besar per akun (`account`); baris memuat `partyName`, `productName`, `description` (FN-08). TANPA `account` = ikhtisar semua akun berpaginasi (`page`, `limit`): `data = {rows[{code,name,type,normal,opening,debit,credit,closing}], totalDebit, totalCredit}` + `meta`; total debit/kredit = neraca saldo rentang yang sama |
| `GET` | `/api/v1/finance/reports/tax-summary` | Ringkasan PPN per bulan; kini memuat `salesDpp`, `purchaseDpp`, `exceptions[]` (transaksi kena pajak tanpa nomor faktur, satu baris per pesanan; paginasi `page`/`limit`) dan `exceptionsMeta` (FN-06) |
| `GET` | `/api/v1/finance/reports/tax-detail` | Rincian PPN (JSON/CSV), berpaginasi; `kind` kini juga `purchase_return`; tiap baris memuat `dpp`, `orderId`, `orderNumber`, `partyName`, `missingInvoice` (FN-04/06) |
| `GET` | `/api/v1/finance/reports/inventory-value` | Nilai persediaan (rata-rata). Satu-satunya implementasi valuasi (`finance.report.view`). `branch=all|<id>` (izin `finance.report.all_branches`) menilai beberapa cabang; respons memuat `byBranch[{branchId,branchName,total,positions}]` (FN-09) |
| `GET` | `/api/v1/finance/reports/margin` | Marjin kotor per rentang; `branch=all` menambah `byBranch[]` yang berjumlah sama dengan total (FN-09) |
| `GET` | `/api/v1/finance/reports/margin-by-product` | Marjin per produk dari buku (`from`/`to`, `limit` = batas tampilan) |
| `GET` | `/api/v1/finance/reports/receivables` | Piutang per pihak (`asOf`; `view=documents` = per dokumen berumur: hanya order bertanggal ≤ `asOf`, dikurangi pembayaran/offset bertanggal ≤ `asOf`; respons juga `accountCode`, `accountName`, `glBalance` = saldo akun kendali per `asOf` (semua baris), `difference` = `glBalance − total`) |
| `GET` | `/api/v1/finance/reports/payables` | Hutang per pihak (`asOf`; `view=documents` = per dokumen berumur, aturan dan rekonsiliasi sama dengan piutang) |
| `GET` | `/api/v1/finance/reports/fiscal-summary` | Ringkasan fiskal per rentang (`from`/`to`) |
| `GET` | `/api/v1/finance/reports/cash-summary` | Ringkasan kas per bulan (`year`, `month`) |
| `GET` | `/api/v1/finance/export/tax-package` | Parameter `branch` (`all` atau id cabang; izin `finance.report.all_branches`, 403 bila tidak/di luar akses; N-53) memuat beberapa cabang dalam satu paket: sheet Rincian PPN, PPN Masukan, Buku Besar mendapat kolom Cabang, sampul menyebut cabang, total sama dengan TaxSummary/TaxDetail lingkup yang sama; tanpa `branch` = cabang sesi (bentuk lama). Paket ekspor pajak (`variant` = `actual`/`capped`) untuk satu bulan (`year`, `month`) ATAU rentang bebas (`from`, `to`; varian `capped` harus dalam satu tahun pajak). Sheet: Ringkasan, Untung Rugi (rinci per akun), Posisi Harta Hutang, Cek Saldo Akun, Rincian PPN (keluaran + retur jual), PPN Masukan (pembelian + retur beli, kolom tanpa faktur), Rekonsiliasi Fiskal (per jenis koreksi), Buku Besar semua akun (maks. 60.000 baris, lebih dari itu ditolak dengan pesan persempit rentang) (FN-04) |
| `GET` | `/api/v1/dashboard/summary` | Snapshot operasional cabang: P&L hari ini + bulan berjalan, saldo kas/piutang/hutang/persediaan per hari ini WIB (jurnal bertanggal masa depan tidak ikut — sama dengan neraca hari ini), nilai persediaan (semua dari buku finance). `operations` (dokumen order): `purchasesToday`/`purchasesMonth` = rupiah nilai order beli selain batal bertanggal periode itu, `purchaseOrdersToday`/`purchaseOrdersMonth` = jumlahnya; `ordersToday`/`ordersMonth` = jumlah order jual |
| `GET` | `/api/v1/dashboard/critical-stock` | Stok kritis cabang (`limit` = batas tampilan, maks. 20/50 di service) |
| `GET` | `/api/v1/dashboard/priority-orders` | Order yang perlu tindakan (`limit`): SO (skor legacy) + PO lewat jatuh tempo belum lunas (100) / draf menunggu keputusan (60); tiap item memuat `kind` (`sales`/`purchase`), `reason`, `ageDays` (DB-01). Ringkasan dashboard kini memuat `operations.missingCostCount` dan `operations.costBasis` (`complete`/`estimated`); kartu WhatsApp memakai `GET /api/v1/assistant/channel` (izin `assistant.view`) |
| `GET` | `/api/v1/dashboard/order-status` | Hitungan order per status |
| `GET` | `/api/v1/dashboard/top-sellers` | Produk terlaris (`limit`) |
| `GET` | `/api/v1/dashboard/top-margin` | Produk marjin tertinggi (`limit`) |
| `GET` | `/api/v1/reporting/sales-trend` | Tren harian revenue/expense/profit (`from`/`to` YYYY-MM-DD, maks 366 hari) |
| `GET` | `/api/v1/reporting/orders` | Laporan order (`type=sales|purchasing`, `status`, `from`, `to`, `page`, `limit`) |
| `GET` | `/api/v1/reporting/stock` | Laporan stok (`criticalOnly`, `page`, `limit`) |
| `GET` | `/api/v1/reporting/party-reconciliation` | Rekonsiliasi buku vs dokumen per pihak (`side` = `receivable`\|`payable`; izin `finance.report.view`): `partiesChecked`, `matched`, `unexplained`, `totalDifference`, `rows[]` (`partyId`, `partyName`, `books` = akun kendali + uang muka, `documents` = saldo pihak modul pembayaran, `difference`, `openOrders`, `timing`). Pihak tanpa order berjalan wajib cocok persis (ADR-0023) |
| `GET` | `/api/v1/reporting/inventory` | Posisi + total nilai persediaan. ALIAS tipis atas `finance.InventoryValue` di bawah izin `reporting.view` (F8-11) — data identik dengan `/finance/reports/inventory-value`; jangan menambah logika valuasi di reporting |
| `GET` | `/api/v1/audit-logs` | Jejak audit (izin `audit.view`). Filter: `branchId` (id atau `all` = lintas cabang; kosong = cabang sesi + event global), `entity`, `entityId`, `action` (prefiks), `actorId`, `from`/`to` (hari WIB), `page`/`limit` terjaga. Aksi `error.unexpected` (entitas `error`) = galat 5xx tak terduga yang dicatat middleware `recoverer` dengan dedupe 60 dtk dan batas 20 baris/menit, tanpa body/header/query (OP-06). UI menampilkan label Indonesia untuk setiap kunci aksi/entitas (`audit-labels.ts`, U-02) |
| `POST` | `/api/v1/assistant/chat` | Konsol asisten (tanpa kanal): `{message, branchId?}` → `{runId, branchId, branchName, intent, mode, answer}`. Cabang dikenali dari teks (nama/kode/kota; ambigu → klarifikasi); operator hanya bisa mencapai cabang aksesnya sendiri (cabang di luar akses → 403) (AS-01) |
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
| `GET` | `/api/v1/ops/integrity` | Konsol operator: keadaan antara dari alur multi-langkah yang gagal (draf SJ dengan stok keluar, penerimaan tertunda, retur yang efek stoknya tak sesuai status); baca-saja (`stock.update`; `bootstrap.RegisterOps`) |
| `GET` | `/api/v1/health` | Liveness murni (`{"status":"ok"}`) — tanpa auth |
| `GET` | `/api/v1/health/ready` | Readiness: DB ping 2 dtk + direktori tulis + versi migrasi; 503 bila belum siap — tanpa auth, dipakai compose `healthcheck` |
| `GET` | `/healthz` | Alias liveness untuk proxy/orchestrator — tanpa auth |
| `GET` | `/readyz` | Alias readiness untuk proxy/orchestrator — tanpa auth |
| `GET` | `/api/v1/metrics` | Metrics Prometheus; hanya bila `METRICS_TOKEN` terisi (`Authorization: Bearer`), selain itu 404 |
| *static* | `/media/*?exp&sig` | Berkas media (foto/bukti) — hanya folder `media/`, wajib tanda tangan HMAC + kedaluwarsa, tanpa listing. `/uploads/*` sudah dihapus (404) |

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
`STORAGE_PUBLIC_BASE_URL` diisi. Driver `local` menyimpan di `STORAGE_DIR/media` dan
melayani `/media/*` hanya lewat URL bertanda tangan ber-expiry (cocok untuk dev tanpa kredensial);
rahasia seperti sesi WhatsApp berada di `SESSION_DIR` (default `STORAGE_DIR/_private`) dan tidak pernah disajikan.

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
