# Feature Inventory — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode (`purchase-return.controller.ts`, `purchase-return.service.ts` 867 baris, 3 entity,
migrasi 050, 3 halaman + 1 komponen web, spec 396 baris). Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = retur ke supplier (tanpa tukar — setiap baris mengeluarkan stok).
Cerminan `SalesReturnService` yang disederhanakan (komentar kode). Yang **bukan** bagian modul
ini: penerimaan (modul 10 — di sini hanya dibaca), retur penjualan (modul 12 — tanpa
tukar/pengganti/SJ), CRUD pembayaran (modul 14 — hanya baris settlement), posting finance
(modul 17 — hanya movement + larangan metadata).

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 5 — `purchase-returns/{list,detail,context,preview,create}` (tanpa dispatch/confirm — tanpa kirim barang!) |
| Halaman | 3 rute: `/purchase-returns`, `/purchase-returns/create` (`?order_id=`), `/purchase-returns/:returnId` |
| Menu sidebar | Grup **"Operasional"**, label **"Retur Pembelian"** (`/purchase-returns`, izin `purchase_return.view`) |
| Permission | `purchase_return.view` (daftar/detail/konteks), `purchase_return.create` (preview/buat), `purchase_return.refund` (khusus refund — 403 bila tidak ada) |
| Tabel yang dimiliki | `purchase_returns`, `purchase_return_items`, `purchase_return_settlements` |
| Tabel yang ditulis | `inventory_balances` (−), `inventory_movements` (`out`/`purchase_return`, TANPA `idOrderItem` di metadata!), `orders` (batal bila penuh) + `order_status_history`, `branch_document_sequences` |
| Penomoran | `RTB-{KODECABANG}/{TAHUN}/{5 digit}` — tanpa reset. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | `purchase_return.create` (termasuk flag `cancelsOrder`) |
| Aksi yang TIDAK ada | Edit/hapus/batal retur; tukar/pengganti; SJ/kirim; cicilan; baris `payments`; retur tanpa terima; retur non-purchase |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar (`/purchase-returns`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Cari + status + Refresh | Pola sama modul 12: tanpa debounce/URL; placeholder `Cari nomor retur, order, atau supplier`; Semua/Selesai/Dibatalkan; tombol Refresh |
| F-01.2 | Kolom | Dokumen (nomor + order) · Tanggal · Supplier (`-` bila null) · Nilai Retur (kanan tebal) · Penyelesaian (label) · Status (badge Selesai-hijau/Dibatalkan-merah + `Order dibatalkan` kuning bila `cancels_order`) |
| F-01.3 | Baris klik → detail; paginasi 20; kosong (`Belum ada retur pembelian` / `Buat dokumen saat barang dari supplier dikembalikan sebagian atau seluruhnya.`) + tombol Buat; gagal → toast `Gagal memuat retur pembelian` |

### F-02 — Buat (`/purchase-returns/create`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Pilih PO asal | Tabel purchase (limit 20, cari nomor + Cari, klik baris) — **tanpa filter status_group** (komentar kode: sebagian-terima berstatus Diproses tetap boleh; server menegakkan `goodsReceivedAt`). Via `?order_id=` dari detail PO (`Retur ke Supplier`) |
| F-02.2 | Bar lengket | Badge (`Pilih barang retur` / `Siap dicek`) + `Cek sebelum simpan` + `{n} barang retur dipilih` + live Nilai retur (`unit_price × qty`, bulat) + `Preview & Cek` |
| F-02.3 | Baris | Kartu per item (centang; redup bila sisa 0 — checkbox disabled tetapi kartu tetap tampil!): nama + `kode · sisa bisa diretur {s} {uom}` + nilai kanan; buka: Jumlah (`max`, dijepit) + Lokasi asal barang (`Pilih lokasi`; default = lokasi terima-terakhir else default-cabang) |
| F-02.4 | Penyelesaian | Alasan* (contoh) · Cara (`Otomatis (kurangi hutang, sisanya ditentukan)` + `Kurangi hutang supplier` + `Terima refund...` (izin) + `Simpan jadi saldo supplier`; helper `Pilih 'Otomatis' agar sistem menentukan sesuai sisa hutang.`) · Cara terima refund (segmented; hanya bermakna untuk refund) · Tanggal · referensi + catatan opsional |
| F-02.5 | Preview wajib | Modal `Preview Retur Pembelian` (2 kartu Nilai + Penyelesaian-hint + notice info 2 varian + tabel) → `Simpan Retur Pembelian` (`Menyimpan...`); tanpa preview → warning `Preview belum dikonfirmasi`; tiap ubah → invalidasi; gagal → toast (`Preview gagal` / `Retur pembelian gagal dibuat`); sukses → detail baru |

### F-03 — Detail (`/purchase-returns/:returnId`)

Header (breadcrumb + nomor; `Order asal`; `[Buka Order]` + `[Kembali]` ghost — tanpa Cetak!) →
notice kuning bila `cancels_order` (`Order pembelian dibatalkan` / `Retur ini menghabiskan
seluruh barang yang diterima pada order asal -- statusnya dipindahkan ke Dibatalkan.`) → 3
kartu (Nilai Retur/Penyelesaian/Status Dokumen `Selesai`/`Dibatalkan` teks!) → Informasi
(Tanggal/Supplier/Status badge + alasan) → Barang Retur (Produk/Lokasi/Qty/Harga/Nilai) →
Penyelesaian (`Tidak ada kas atau saldo supplier.` / tabel Tanggal/Jenis/Metode/Referensi/
Jumlah). Gagal: toast `Gagal memuat detail retur`; kosong: `Retur pembelian tidak ditemukan` /
`Dokumen tidak tersedia pada cabang aktif.`

### F-04 — Penyelesaian 3-arah (tanpa tukar)

`reduce_payable` (potong hutang; tanpa baris kas) vs `refund` (terima uang; butuh izin; metode
tunai-default) vs `supplier_credit` (saldo supplier = `supplier_advance` untuk beli
berikutnya). Default otomatis: tanpa-sisa → potong; sisa → refund (eksplisit kredit bila
dipilih). Paksa salah → `400 'Retur ini sepenuhnya mengurangi hutang, belum ada kelebihan
untuk direfund atau dijadikan saldo supplier'`.

### F-05 — Batal-otomatis order-penuh

Retur yang kumulatifnya menghabiskan seluruh yang diterima → `cancels_order: true` + order ke
status grup-cancelled (kind-persis lalu `all`; tanpa status cocok → retur tetap, order
dibiarkan — bukan gagal!). Badge kuning di daftar + notice di detail + badge `Diretur penuh`
di daftar order (modul 08). Dicek SETELAH baris tersimpan (termasuk retur ini).

---

## 3. Edge Case (24)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Alasan kosong / tanpa baris | `400 'Alasan retur wajib diisi'` / `'Minimal satu barang yang dikembalikan harus dipilih'` (FE: tombol disabled) |
| E-02 | Bukan-purchase / arsip / belum-terima / tanpa item | `404 'Order asal tidak ditemukan'` / `400 'Retur ini hanya berlaku untuk order pembelian'` / `'Order pembelian harus sudah menerima barang sebelum bisa diretur'` / `'Order asal tidak memiliki item'` (FE pilih-tanpa-filter-status; konteks menolak) |
| E-03 | Duplikat / luar-order / qty ≤ 0 / over-sisa-terima | `Item retur tidak boleh duplikat` / `...tidak ditemukan pada order asal` / `{label} harus lebih dari 0` / `Qty retur '{n}' melebihi sisa yang bisa diretur. Sisa: {s} {u}` (sisa = terima − retur-completed; FE jepit + redup-nol) |
| E-04 | Tanggal < terima / > kini+1mnt | `Tanggal retur tidak boleh lebih awal dari tanggal penerimaan barang` / `...tidak boleh di masa depan` (ada test keduanya!) |
| E-05 | Lokasi: tanpa-default + tanpa-pilih / non-daun / non-aktif / hilang | `Lokasi asal barang untuk retur '{n}' tidak ditemukan, pilih lokasi secara manual` / `Pilih lokasi stok paling bawah...` / `Lokasi stok harus aktif` / `Lokasi stok tidak ditemukan` (404!) |
| E-06 | Stok lokasi kurang (available! bukan on-hand) | `Stok '{n}' tidak cukup di lokasi {nama}` (ada test: onHand cukup + available kurang → tolak) |
| E-07 | Non-tracked | Tanpa lokasi + tanpa movement (tercatat + nilai) |
| E-08 | Refund tanpa izin | `403 'Anda tidak memiliki izin untuk menerima refund dari supplier'` (ada test ±) |
| E-09 | Penuh vs sebagian | Sebagian: order tetap; penuh: `cancels_order` + pindah-cancelled + history `Retur pembelian menghabiskan seluruh barang yang diterima` (ada test) |
| E-10 | Penuh tanpa status-cancelled | Retur tetap + order dibiarkan (ada test; bukan gagal) |
| E-11 | Nomor dari sequence + naik | Ada test; fallback `RTB-{id}-{ts}` tanpa baris |
| E-12 | Saldo + movement satu transaksi | Ada test (kurang + `out`/`purchase_return`) |
| E-13 | Metadata TANPA `idOrderItem` | Disengaja (lindungi margin sales — komentar + modul 17); baris-id asli di `idOriginalOrderItem` (kolom, bukan metadata) |
| E-14 | Lock baris order (`pessimistic_write`) sebelum rencana | Klik-ganda aman (komentar: tidak mewarisi celah sales-return; sekelas KI-92/97 terpecahkan di sini!) |
| E-15 | Nilai = rasio-qty × harga/pajak baris-asal (rupiah); tanpa diskon terpisah (harga sudah-bersih) |
| E-16 | Konteks: terima + sudah-retur + sisa + lokasi-terakhir + daun (default→sort→nama) |
| E-17 | List: limit jepit 1–50 (seperti sales!); page ≥ 1; cari 3 kolom; urut tanggal + id DESC |
| E-18 | Settlement: hanya non-`reduce_payable` + jumlah > 0 (tanggal = tanggal-retur; metode hanya refund) |
| E-19 | Audit `purchase_return.create` (+ `cancelsOrder` boolean!) — satu-satunya audit modul |
| E-20 | Respons create = preview + `id_purchase_return` + `return_number` + `settlement_id` + `cancels_order` |
| E-21 | Detail badge: status teks (bukan badge!) untuk Status Dokumen vs badge untuk Status; `cancels_order` notice + badge |
| E-22 | Tanpa Cetak di detail (beda sales!); tanpa Jenis (tanpa tukar); Supplier `-` (bukan Walk-in) |
| E-23 | Pilih-PO tanpa filter status (Diproses bisa) — komentar kode eksplisit |
| E-24 | Kartu sisa-nol tetap tampil redup + checkbox disabled (bukan disembunyikan); qty awal = sisa (0) |

---

## 4. Katalog Pesan (teks apa adanya)

**API:** `'Order asal tidak ditemukan'` (404) · `'Retur ini hanya berlaku untuk order pembelian'` · `'Cabang tidak sesuai company aktif'` (404!) · `'Order pembelian harus sudah menerima barang sebelum bisa diretur'` · `'Order asal tidak memiliki item'` · `'Alasan retur wajib diisi'` · `'Minimal satu barang yang dikembalikan harus dipilih'` · `'Item retur tidak boleh duplikat'` · `'Item yang dikembalikan tidak ditemukan pada order asal'` · `'{label} harus lebih dari 0'` · `"Qty retur '{n}' melebihi sisa yang bisa diretur. Sisa: {s} {u}"` · `'Tanggal retur tidak boleh lebih awal dari tanggal penerimaan barang'` · `'Tanggal retur tidak boleh di masa depan'` · `"Lokasi asal barang untuk retur '{n}' tidak ditemukan, pilih lokasi secara manual"` · `'Lokasi stok tidak ditemukan'` (404!) · `'Lokasi stok harus aktif'` · `'Pilih lokasi stok paling bawah, bukan grup gudang'` · `"Stok '{n}' tidak cukup di lokasi {nama}"` · `'Retur ini sepenuhnya mengurangi hutang, belum ada kelebihan untuk direfund atau dijadikan saldo supplier'` · `'Anda tidak memiliki izin untuk menerima refund dari supplier'` (403) · `'Dokumen retur pembelian tidak ditemukan'` (404).

**UI:** `Order tidak bisa diretur` · `Gagal mencari order` · `Gagal memuat retur pembelian` · `Gagal memuat detail retur` · `Preview gagal` · `Preview belum dikonfirmasi` / `Klik Preview & Cek lalu periksa data di modal sebelum menyimpan.` · `Retur pembelian gagal dibuat` · `Pilih barang retur` / `Siap dicek` / `Isi alasan retur` · `Retur pembelian tidak ditemukan` / `Dokumen tidak tersedia pada cabang aktif.` · `Order pembelian dibatalkan` / `Retur ini menghabiskan seluruh barang yang diterima pada order asal -- statusnya dipindahkan ke Dibatalkan.` · label (`Tidak ada selisih` / `Kurangi hutang supplier` / `Refund uang dari supplier` / `Simpan sebagai saldo supplier`) + contoh placeholder alasan.

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Tukar/pengganti/SJ | Tanpa konsep (komentar kode); tiap baris keluar |
| NF-02 | Edit/hapus/batal retur | Tanpa endpoint |
| NF-03 | Cicilan / collect_payment | Selalu kurangi-hutang-dulu (beda sales) |
| NF-04 | Baris `payments` | Hanya settlement (seperti sales E-16!) |
| NF-05 | Retur tanpa terima / non-purchase | Guard tahap + jenis |
| NF-06 | Diskon terpisah | Harga-bersih baris-asal |
| NF-07 | Kondisi barang | Tanpa normal/rusak (semua keluar) |
| NF-08 | Cetak khusus | Tanpa tombol (beda sales) |
| NF-09 | Lokasi untuk non-tracked | Tanpa field (beda sales-normal) |
| NF-10 | `collect`/`none` di opsi FE | Opsi: Otomatis/Kurangi/Refund/Kredit (tanpa `none` eksplisit — `none` = nilai awal Otomatis) |
