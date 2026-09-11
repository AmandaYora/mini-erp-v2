# UI/UX Spec — Modul 08 Order Purchasing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Aspek pembelian dari 4 screen
bersama (`/orders`, `/orders/create`, `/orders/:orderId`, `/orders/:orderId/edit`) + modal export.
Rute: `/orders` (`order.view`, menu Operasional → Order), `/orders/create` (`order.create`),
`/orders/:orderId` (`order.view`), `/orders/:orderId/edit` (`order.update`). Bagian ambigu
ditandai **[PERLU KONFIRMASI]**.

---

## 1. Screen `/orders` — Daftar (tab Pembelian)

**Header**: breadcrumb `Dashboard > Order`; judul `Daftar Order`; deskripsi `Kelola seluruh pesanan
penjualan dan pembelian.`; aksi `[Export]` (secondary, `order.export`) + `[Buat Order Baru]`
(`order.create`).

**FilterBar**: `Cari Order / Pihak` (`order-search`, `"Nomor order, nama, no HP, email..."`,
debounce 400ms) · `Status Operasional` (Semua/Pending/Aktif/Selesai/Dibatalkan) · `Termin
Pembayaran` (Semua/Prabayar/Bayar Saat Serah/Tempo (Utang/Piutang)) · `Rentang Tanggal` (dua input
`date`: `order-start-date`, `order-end-date`, pemisah `-`) · `[Hapus Filter]` (ghost merah, bila ada
filter aktif; menyisakan tab).

**Kartu** `Order pada Filter Aktif` (`{n} order pada filter aktif.` + spinner `Memuat...`):
tab `SegmentedControl` Penjualan/Pembelian/Semua + hitungan `{n} order`. Tabel tab Pembelian:
Nomor (+tanggal `formatDateTime`; tanpa badge) · Supplier (nama + telepon??email??kode; `-`) ·
Dibuat Oleh (`-` bila null) · Termin (`Prabayar`/`Tempo`/`Bayar Saat Terima`) · Dibayar (kanan) ·
Total (kanan, tebal) · Status (badge status + badge Lunas hijau/Bayar Sebagian kuning/Jatuh Tempo
merah/Belum Bayar merah + badge Diretur — yang tak pernah tampil, E-30) · `[Detail]` (secondary).
Kosong: `Pesanan Kosong` / `Belum ada pesanan pembelian untuk filter ini.` + `[Buat Order]`.
`Pagination` 20.

**Modal Export** (`Export Data Order`, badge hijau `Report`, deskripsi `Unduh report order yang sudah
diformat agar mudah dibaca, bukan data mentah.`; tutup dikunci saat menyiapkan): Jenis order
(Semua/Penjualan/Pembelian — terisi dari tab aktif) · Dari/Sampai tanggal order (bulan berjalan
default) · Format (Excel `Ringkasan dan detail item` / PDF `Ringkasan siap print`) · `[Batal]` +
`[Export]`/`Menyiapkan...`.

## 2. Screen form — Buat/Edit (aspek pembelian)

**Header**: breadcrumb `Dashboard > Order > Buat Order|{nomor} > Edit`; judul `Buat/Edit Order`;
aksi `[← Kembali]` (ghost, `navigate(-1)`). Edit tak dikenal: `Order tidak ditemukan`/`Memuat
Order` + notice (`Order tidak bisa diedit` / `Data order tidak ditemukan atau Anda tidak memiliki
akses ke order ini.`).

**Detail Utama**: `Supplier *` (picker supplier; tanpa tombol tambah cepat) · (tanpa alamat kirim)
· `Tanggal order` (`datetime-local`, default kini).

**Baris**: `[Tambah Baris]`; per baris: produk (picker + saran harga beli + helper
`Harga beli referensi produk: Rp X / {uomBeli}`) · varian (bila perlu) · qty · harga satuan ·
konversi (`1 {beli} = {n} {stok}` bila faktor ≠1) + ringkasan base (`{n} {stok} stok`) · Catatan —
**tanpa** kolom Diskon · footer `Baris {n} [| {base}]` + `[Hapus baris]` (ghost, bila >1).
Ringkasan: Total (+ `Diskon -Rp X` hanya bila >0 — purchase selalu 0).

**Pengaturan Finansial** (`Jenis transaksi dan termin.`): `Jenis order` (Penjualan/Pembelian;
ganti → harga saran sisi baru + termin default bila sebelumnya default + pihak di-reset) · `Termin
pembayaran` (Prabayar/Bayar Saat Serah/Tempo - Utang-Piutang; kosong → default jenis) ·
`Jatuh tempo pembayaran` (`datetime-local`, required, hanya net) · purchase saja:
`Nomor invoice supplier` (`INV-SUP-001`) · `Tanggal invoice supplier` (mengisi → jatuh tempo =
invoice + hari bila net) · `Termin supplier (hari)` (default 30, `min=0`; mengubah → hitung ulang
jatuh tempo) · pajak: checkbox `Transaksi ini kena pajak` + (`Isi hanya untuk transaksi yang memang
perlu masuk rekap pajak.`) → tarif % + `Harga sudah termasuk pajak` + nomor/tanggal faktur
(opsional).

**Submit**: `[Batal]`? — (tidak diverifikasi terpisah; tombol submit `[Simpan]`/`Menyimpan...`,
disabled bila produk belum siap/baris tak valid/sedang menyimpan). Sukses → detail + toast; gagal →
toast + tetap di form.

## 3. Screen `/orders/:orderId` — Detail PO

**Aksi header (purchase)**: `[Terima Barang]` (syarat §F-03.2; label konfirmasi dinamis di dialog)
· `Diterima {datetime}` (pengganti tombol) · `[Retur ke Supplier]` (`purchase_return.create` +
sudah diterima) · pindah status (tombol per label transisi; selesai disembunyikan pre-terima;
pasca-terima hanya selesai) · bayar/arsip (modul 14 + guard).

**Kartu info**: nomor/status/supplier/tanggal/termin/jatuh-tempo · `Invoice Supplier` + `Tanggal
Invoice Supplier` + `Termin Supplier ({n} hari)` (masing-masing bila ada) · per baris
`Diterima X / Y {uom}` · notice prabayar-belum-lunas (§F-03.4).

**Riwayat Penerimaan Barang** (`Batch penerimaan supplier dan lokasi ...`): kosong →
`Belum ada penerimaan` / `Purchase order ini belum memiliki batch penerimaan barang.`; tabel Waktu
(+`oleh {nama pencatat}`) · Dokumen Supplier (`-`) · Lokasi · Item Diterima (`{nama}: {qty} {uom}`
per baris) · Catatan (`-`).

**Dialog Terima Barang** (badge kuning `Terima Barang`, tutup `×` aria `Tutup dialog`): deskripsi
dinamis (§F-04.1) · `No. Surat Jalan Supplier` (`Opsional`) · `Tanggal Terima` (`datetime-local`) ·
`Lokasi cepat untuk semua item` (required + `Pilih ini untuk mengisi lokasi semua item, lalu ubah
per item jika barang dipilah ke lokasi berbeda.`) · `Barang diterima dan lokasi` (label
`{nama} sisa {n} {uom}`, input `max/min/step`, dijepit 0…sisa; `Lokasi simpan` per baris) · kotak
biru `Total item purchase order` (`• {nama} +{qty} {uom} ({base} stok)`, tracked saja) ·
`Catatan penerimaan` · keputusan COD (`Keputusan pembayaran saat terima`: `[Bayar Sekarang]` /
`[Ubah ke Tempo]` + teks tanpa-izin; bayar → Total readonly + Tanggal Bayar + Metode
(Tunai/Transfer Bank/Cek - Giro, cari `Cari metode...`) + No. Referensi + Catatan Pembayaran +
Bukti (`(opsional, JPG/PNG, maks 5MB)`, `{nama} — akan dikompres sebelum disimpan`); tempo →
`Jatuh Tempo Baru` + Total Utang) · tombol konfirmasi dinamis (§F-04.4, terkunci §F-04.4).

## 4. Format yang mengikat

- Nomor PO selalu `ORD-{KODECABANG}/PB/YYYY/MM/00001` (tampil penuh di daftar/detail/cetak).
- Uang `formatCurrency` (id-ID IDR); tanggal `formatDateTime`; qty apa adanya (desimal 4 di input).
- Termin purchase: `Prabayar` / `Bayar Saat Terima` / `Tempo`; status keuangan: `Lunas` / `Bayar
  Sebagian` / `Jatuh Tempo` / `Belum Bayar`; retur: `Diretur penuh` / `Diretur sebagian` (mati di
  daftar — E-30).
- Lokasi default dialog: daun bernama `*penyimpanan sementara*` → daun default → daun pertama
  ([PERLU KONFIRMASI] prioritas nama ini vs default eksplisit).
