# UI/UX Spec — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Rute: `/purchase-returns`
(`purchase_return.view`, menu Operasional → Retur Pembelian), `/purchase-returns/create`
(`purchase_return.create`), `/purchase-returns/:returnId` (`purchase_return.view`). Cermin
modul 12 yang disederhanakan (tanpa tukar). Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. Screen `/purchase-returns` — Daftar

**Header**: breadcrumb `Dashboard > Operasional > Retur Pembelian`; judul sama; deskripsi
`Dokumen pengembalian barang ke supplier untuk pembelian yang barangnya sudah diterima.`;
aksi `[Buat Retur Pembelian]` (bila create).

**FilterBar**: `Cari` (`purchase-return-search`, `Cari nomor retur, order, atau supplier`,
tanpa debounce/URL) + `Status` (Semua/Selesai/Dibatalkan) + `[Refresh]` (secondary). Gagal:
toast (tanpa notice).

**Kartu** `Daftar Dokumen`: loading `Memuat retur pembelian...`; kosong
`Belum ada retur pembelian` / `Buat dokumen saat barang dari supplier dikembalikan sebagian
atau seluruhnya.` (+ tombol). Tabel klik-baris (tanpa kolom aksi): Dokumen (nomor + order) ·
Tanggal · Supplier (`-`) · Nilai Retur (kanan tebal) · Penyelesaian (label) · Status (badge
Selesai-hijau/Dibatalkan-merah + `Order dibatalkan` kuning bila `cancels_order`). Paginasi 20.

## 2. Screen `/purchase-returns/create` — Buat

**Header**: breadcrumb `... > Buat`; judul `Buat Retur Pembelian`; deskripsi `Pilih order
pembelian asal, barang yang dikembalikan ke supplier, lalu cek nilainya sebelum simpan.`;
aksi `[Kembali]` (ghost, `navigate(-1)`).

**Pilih Order** (tanpa konteks): `Cari order` (`purchase-return-order-search`, `Cari nomor
order`) + `[Cari]` → tabel purchase-20 (Order/Tanggal/Supplier/`-`/Total; klik baris; tanpa
filter status — Diproses boleh). Via `?order_id=`: langsung konteks.

**Bar lengket** (`Aksi preview retur pembelian`): badge (`Pilih barang retur`/
`Isi alasan retur`/`Siap dicek`) + `Cek sebelum simpan` + `{n} barang retur dipilih` + live
Nilai retur (`unit_price × qty`, bulat; tanpa verdict!) + `Preview & Cek`.

**Order Asal** + `[Ganti Order]`: Nomor/Tanggal/Supplier/Total.

**Barang yang Dikembalikan** (`Centang barang yang dikembalikan ke supplier, lalu atur jumlah
dan lokasi asalnya.`; kosong-semua: `Tidak ada barang yang tersisa untuk diretur pada order
ini.`): kartu per item (centang 18px; redup + disabled bila sisa 0; sub `kode · sisa bisa
diretur {s} {uom}` + nilai kanan) → buka Jumlah (`max`, dijepit) + Lokasi asal barang
(`Pilih lokasi`; tracked saja; default terima-terakhir).

**Penyelesaian & Catatan** (`Atur cara menyelesaikan nilai retur dan beri alasan.`): Alasan*
(contoh) · Cara (`Otomatis (kurangi hutang, sisanya ditentukan)` + helper `Pilih 'Otomatis'
agar sistem menentukan sesuai sisa hutang.` + Kurangi + Refund (izin) + Kredit) · Cara terima
refund (segmented; hanya-refund) · Tanggal · referensi + catatan (opsional).

**Modal Preview** (xl, kunci-tutup): deskripsi `Periksa barang, nilai retur, dan penyelesaian
sebelum transaksi diproses.` → 2 kartu (Nilai + Penyelesaian-hint) → notice info (2 varian:
penuh-potong / sisa-refund-kredit) → tabel → `[Ubah Data]` + `[Simpan Retur Pembelian]`
(`Menyimpan...`).

## 3. Screen `/purchase-returns/:returnId` — Detail

Header + notice-batal + 3 kartu (Nilai/Penyelesaian/Status-teks) + Informasi
(Tanggal/Supplier/Status-badge + alasan) + Barang Retur (Produk/Lokasi/Qty/Harga/Nilai) +
Penyelesaian (`Tidak ada kas atau saldo supplier.` / tabel). Loading `Memuat retur
pembelian...`; kosong `Retur pembelian tidak ditemukan` / `Dokumen tidak tersedia pada cabang
aktif.`

## 4. Format yang mengikat

- Tanpa badge Jenis (tanpa tukar); Supplier `-` (bukan Walk-in); Status Dokumen teks (bukan
  badge!) vs Status badge; `cancels_order` kuning di dua tempat + notice.
- Live = `unit_price × qty` (tanpa pajak di bar!) vs preview server (dengan pajak).
- Lokasi `kode - nama`; default = terima-terakhir → default → primer → pertama.
- Tanpa Cetak; tanpa Jenis; tanpa kolom Pengganti/Selisih.
