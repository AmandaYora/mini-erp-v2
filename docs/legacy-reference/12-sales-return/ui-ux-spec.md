# UI/UX Spec — Modul 12 Sales Return (Retur Penjualan)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Rute: `/sales-returns`
(`sales_return.view`, menu Operasional → Retur / Tukar), `/sales-returns/create`
(`sales_return.create`), `/sales-returns/:returnId` (`sales_return.view`). Bagian ambigu
ditandai **[PERLU KONFIRMASI]**.

---

## 1. Screen `/sales-returns` — Daftar

**Header**: breadcrumb `Dashboard > Operasional > Retur / Tukar`; judul `Retur / Tukar`;
deskripsi `Dokumen penyesuaian setelah barang sudah diterima customer.`; aksi
`[Buat Retur / Tukar]` (bila create).

**FilterBar**: `Cari` (`sales-return-search`, `Cari nomor retur, order, atau customer`, tanpa
debounce/URL) + `Status` (Semua/Selesai/Dibatalkan) + tombol `[Refresh]` (secondary).
Kegagalan: toast (tanpa notice).

**Kartu** `Daftar Dokumen`: loading `Memuat retur/tukar...`; kosong `Belum ada retur/tukar` /
`Buat dokumen saat customer mengembalikan barang atau menukar dengan barang lain.` (+ tombol).
Tabel (klik-baris → detail, tanpa kolom aksi): Dokumen (nomor + order) · Tanggal · Customer
(`Walk-in`) · Jenis (badge `Tukar` info / `Retur` netral) · Barang Retur (kanan) · Barang
Pengganti (kanan) · Selisih (kanan tebal) · Penyelesaian (label Indonesia). Paginasi 20.

## 2. Screen `/sales-returns/create` — Buat

**Header**: breadcrumb `... > Buat`; judul `Buat Retur / Tukar`; deskripsi `Pilih order asal,
barang yang kembali, barang pengganti, lalu cek selisih sebelum simpan.`; aksi `[Kembali]`
(ghost, `navigate(-1)`).

**Pilih Order Asal** (tanpa konteks): `Cari order` (`sales-return-order-search`, `Cari nomor
order`) + `[Cari]` → tabel order-completed-sales-20 (Order/Tanggal/Customer/`Walk-in`/Total;
klik baris). Via `?order_id=`: langsung konteks (dari detail order).

**Bar lengket** (`Aksi preview retur`, sticky): badge kesiapan + `Cek sebelum simpan` + meta +
live 3 angka (Nilai retur → Barang pengganti → Selisih berwarna + tooltip kalimat) + tombol
`Preview & Cek` (disabled).

**Order Asal** (kartu + `[Ganti Order]`): Nomor/Tanggal/Customer/Total.

**Barang yang Dikembalikan** (`Centang barang yang customer kembalikan, lalu atur jumlah dan
kondisinya.`): kartu per item (centang 18px; terpilih garis-merek + nilai kanan; sub
`kode · sisa bisa diretur {s} {uom}`) → buka Jumlah (`max`, dijepit) + Kondisi (segmented kecil:
`Masuk stok lagi` / `Rusak` / `Tidak kembali`) + Simpan-ke-lokasi (kondisional; `Pilih lokasi`).

**Barang Pengganti** (`Opsional. Tambahkan barang yang diberikan sebagai ganti — boleh lebih
dari satu. Lewati bagian ini jika hanya retur tanpa tukar.`; badge `{n} barang · {nilai}` bila
ada): `Cari produk untuk ditambahkan` (`Setiap produk yang dipilih langsung masuk ke daftar di
bawah. Pilih lagi untuk menambah barang berikutnya.`; placeholder `Ketik nama atau kode
produk…`) → kosong `Belum ada barang pengganti.` / daftar (nama + kode[·varian] + Subtotal +
`[Hapus]` aria `Hapus {nama}`; Varian (bila ada) · Jumlah · Harga satuan (`step=100`) ·
Ambil-dari-lokasi (tracked; `Otomatis`)).

**Penyelesaian & Catatan** (`Atur cara menyelesaikan selisih nilai dan beri alasan
retur/tukar.`): notice Selisih (info impas / warning else; kalimat verdict) · Alasan* (contoh
placeholder) · Cara selisih (`Otomatis (sesuai selisih)` + helper `Pilih 'Otomatis' agar sistem
menentukan sesuai selisih.`; refund bersyarat izin) · Cara bayar (segmented Tunai/Transfer/
Cek) · Tanggal retur · No. referensi + Catatan (`(opsional)`).

**Modal Preview** (xl, kunci-tutup): deskripsi `Periksa data retur, barang pengganti, nilai
selisih, dan penyelesaian sebelum transaksi diproses.` → 4 kartu (Selisih tone + Penyelesaian
hint-angka) → notice (4 varian kalimat) → 2 tabel → `[Ubah Data]` + `[Simpan Retur / Tukar]`
(`Menyimpan...`).

## 3. Screen `/sales-returns/:returnId` — Detail

Header + 4 kartu + Informasi (Tanggal/Customer/Status badge/Jenis + alasan) + 2 tabel +
Pengiriman (§F-04 feature) + Penyelesaian (`Tidak ada kas atau saldo customer.` / tabel
Tanggal/Jenis/Metode/Referensi/Jumlah). Loading `Memuat retur/tukar...`; kosong notice
`Retur/tukar tidak ditemukan` / `Dokumen tidak tersedia pada cabang aktif.`

## 4. Format yang mengikat

- Uang `formatCurrency` (+ kode perusahaan); tanggal `formatDateTime`, order `formatDateOnly`,
  kirim-pengganti `formatDateOnly`.
- Warna selisih: 0 hijau-ok / + kuning-warn / − merah-bad (bar + kartu + tooltip).
- Nama-baris `nama - varian` (bila ada); lokasi `kode - nama`; default lokasi =
  default → primer → pertama.
- `Walk-in` untuk tanpa-pihak (daftar + detail + pilih-order).
- Status detail badge: completed-hijau else merah (hanya dua nilai bertipe).
