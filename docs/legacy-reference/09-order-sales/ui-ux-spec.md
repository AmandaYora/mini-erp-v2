# UI/UX Spec — Modul 09 Order Sales

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Aspek sales dari screen bersama +
dokumen cetak. Rute: `/orders` (tab Penjualan default), `/orders/create`, `/orders/:orderId`,
`/orders/:orderId/edit` (izin `order.*`, menu Operasional → Order) + cetak (tab baru):
`/orders/:orderId/sales-document/print`, `/orders/:orderId/pos-receipt/print` (modul 15),
`/orders/:orderId/payments/:paymentId/print` (semua `order.view`).

---

## 1. Daftar — tab Penjualan

Default bila `order_kind` tak dikenal. Kolom Pihak berlabel **Customer**; Termin
`Prabayar`/`Bayar Saat Serah`/`Tempo`; kosong: `Belum ada pesanan penjualan untuk filter ini.`.
Selebihnya = modul 08 (filter, tab, export, badge, paginasi).

## 2. Form — aspek sales

**Detail Utama**: `Customer` (tanpa `*`; picker customer + tombol kecil `Tambah Pelanggan` bila
sales + `order.create`) · `Alamat kirim` (`AddressPicker`, sales saja) · `Tanggal order`.
**Baris**: produk (saran jual + helper `Harga jual referensi produk: {min} - {ref} / {uom}`) ·
qty · harga · `Diskon (Rp)` (total-baris; klem otomatis + nilai tampil `roundRupiah(total)`) ·
Catatan · konversi + base · `Baris {n}` + `Hapus baris` (>1). Banner quote error (bila ada) +
blokir submit saat quote loading/error (sales + customer). Ringkasan: Total + `Diskon -Rp X`
(bila >0) + pajak + Total Akhir.

**Modal Tambah Pelanggan** (dari order): Kode (opsional) · Nama* · Member · Telepon · Email ·
Alamat · Catatan → error inline (`Nama pelanggan wajib diisi.` / `Pelanggan gagal disimpan.`)
→ `[Batal]` / `[Simpan & Pilih Pelanggan]` (`Menyimpan...`; tutup dikunci saat simpan) →
terpilih + requote.

**Finansial**: Termin (default Bayar Saat Serah untuk sales baru); Jatuh tempo (net saja);
tanpa field invoice supplier; pajak sama dengan purchase.

## 3. Detail SO

**Header**: `[← Kembali]` · `[Edit Order]` (non-terminal) · `[Cetak Tagihan]`/`[Cetak Nota]`
(tab baru) · `[Retur / Tukar]` (syarat modul 12) · `[Terbitkan Surat Jalan]` (primer; syarat
modul 11) · pindah status (label transisi) · `Diserahkan {datetime}` (pengganti aksi, teks redup 0.85em — pola sama dengan
`Diterima ...` purchase; diverifikasi baris 463–467).

**Seksi Pembayaran** (§F-03.2–F-03.4 feature): kartu + badge + tombol + 4 kotak + form + tabel +
viewer + modal bukti + dialog tempo + dialog hapus. Label sisi-customer: `Total Tagihan` /
`Sudah Dibayar Customer` / `Sisa Piutang` / `Belum ada pembayaran dari customer untuk order ini.`

**Dialog Ubah ke Tempo** (modal 520px, badge kuning `Tempo`, kunci-tutup saat simpan):
`Jatuh Tempo *` (`datetime-local`, default +30 hari) → `[Batal]` / `[Ubah ke Tempo]`
(`Menyimpan...`; disabled tanpa tanggal).

**Riwayat Status**: timeline + `dari {asal}` + `waktu · oleh {nama} (@{username})` + alasan.

## 4. Dokumen cetak

**Nota/Tagihan** (`sales-document/print`): loading `Memuat dokumen...` (padding 2rem; sama untuk
error `Order tidak ditemukan.` / pesan khusus non-sales). Toolbar non-cetak (kelas `no-print`):
radio `Kertas A4 (biasa)` / `Kontinu 3-Ply (Dot-Matrix)` · checkbox `Kop/rekening sudah
preprinted (sembunyikan)` (kontinu saja; default dari Pengaturan) · checkbox `Sembunyikan diskon
(harga penuh)` (bila ada diskon) · Notes (`Notes (opsional, hanya tampil di hasil cetak —
tidak disimpan):`, placeholder `Tulis catatan tambahan untuk nota ini...`) · `[Cetak / Print]`
(`window.print()`) + `[Tutup]` (`window.close()`). Semua opsi via URL (cetakan bisa
dibagikan sebagai link).

**Isi nota**: kop (atau kosong bila preprinted) + judul dinamis; pihak (Nama/No HP/Alamat;
`Pelanggan umum`); No Nota/Tanggal/`Tgl. Jtp`/Staff/Faktur Pajak (kondisional); tabel
No/Qty/Satuan/Produk (+catatan 8pt abu)/Harga Satuan/[Disc]/Harga Jual; ringkasan terbilang +
Jumlah/Diskon/Pajak/Total/Sudah Dibayar/Sisa Tagihan; footer notes + Pembeli
`(................)` + MENJUAL + `Dicetak: {waktu} USER : {pencetak}`.

**Bukti Bayar** (`payments/:paymentId/print`): satu pembayaran + metode/referensi + order terkait
+ sisa sesudahnya; tanpa tanda tangan.

## 5. Format yang mengikat

- Judul nota = status bayar (`Nota Penjualan Lunas` / `Tagihan - Bayar Sebagian` / `Tagihan
  Penjualan`); tombol detail = sebaliknya (`Cetak Tagihan` bila sisa > 0 else `Cetak Nota`).
- Harga cetak = penuh (sebelum diskon); `Sisa Tagihan = max(0, Total − Dibayar)`; terbilang
  mengikuti total tampil.
- Qty cetak `id-ID`; uang `formatCurrency`; tanggal panjang `formatLongDate`.
- `USER` = pencetak, `Staff` = pembuat (aturan kasir-aman); A4 ganjal 6 baris, kontinu tanpa.
