# ADR-0027: Akun kas pada pembayaran tunai, dan refund retur sebagai pembayaran

## Status
Accepted (2026-11-02). Permintaan pengguna 2026-11-02: "retur belum masuk ke laporan kas" dan "pembayaran ke
pemasok bisa dipilih pakai kas besar / kas tunai". Merevisi sebagian [ADR-0026](ADR-0026-rekening-tujuan.md) §3
("Tunai tidak membawa rekening").

## Context
- **Retur.** "Catat penyelesaian → Refund" di retur jual/beli hanya menyimpan baris memo
  (`*_return_settlements`) dengan metode teks bebas. Tidak ada pembayaran, jadi tidak ada jurnal kas/bank, dan
  laporan Kas (yang dibaca dari buku besar akun `is_cash`) tidak memuat refund itu. Uangnya harus dicatat lagi
  secara terpisah di menu Pembayaran. Retur penjualan walk-in (POS tanpa pelanggan) tidak punya jalan sama sekali,
  karena halaman Pembayaran memerlukan pihak. "Tambah bayar" retur tukar lewat memo bernasib sama.
- **Kas.** Pengguna membuat akun kas baru "Kas Besar" dan membayar semua pemasok dari sana. Pembayaran tunai
  selalu dijurnal ke pemetaan `cash` (Kas Tunai), karena hanya transfer/cek yang boleh membawa
  `payments.cash_account_id` (rekening, ADR-0026).
- `payments.cash_account_id` (migrasi `000048`) adalah id akun buku besar generik. `buildPayment` sudah
  mendebit/mengkredit akun itu untuk kunci `cash` maupun `bank`. Biaya operasional sudah memilih akun kas
  sembarang ("Dibayar dari").

## Decision
1. **Pembayaran tunai boleh menunjuk akun kas** (`cashAccountId`): akun aset `is_cash` aktif yang **bukan
   rekening** (tidak ada di `finance_bank_accounts`) dan **bukan pemetaan `bank`**. Kosong berarti pemetaan `cash`
   (Kas Tunai), sama seperti sebelumnya. Rekening tetap hanya untuk transfer/cek.
   - Pilihan kas: `GET /finance/cash-accounts/options` (cukup login + cabang, pola `bank-accounts/options`),
     Kas Tunai pertama sebagai default.
   - Pemilih "Dari kas" / "Ke kas" hanya tampil bila ada **lebih dari satu** kas aktif. Dengan satu kas, tidak ada
     yang berubah. Isian awalnya kas terakhir yang dipakai untuk arah uang yang sama di perangkat itu (jadi semua
     bayar pemasok dari Kas Besar tidak perlu dipilih ulang), lalu Kas Tunai.
   - Berlaku di semua form pembayaran web: catat pembayaran, bayar dari detail order, koreksi pembayaran (awalnya
     kas pembayaran asli), bayar-sekarang penerimaan barang, dan penyelesaian retur. POS dan pelunasan SJ tidak
     berubah (tunai ke Kas Tunai).
   - Tanpa migrasi. Salah pilih diperbaiki lewat koreksi pembayaran (ADR-0023).
2. **Penyelesaian retur yang menggerakkan uang dicatat sebagai pembayaran.** `refund` (retur jual: uang keluar;
   retur beli: uang masuk) dan `collect_payment` (tambah bayar retur tukar: uang masuk) membuat pembayaran yang
   dialokasikan ke retur itu (`RecordForOrder`, tipe `sales_return` / `purchase_return`), lalu menulis memo dengan
   nomor pembayaran sebagai referensi. Jurnalnya jurnal pembayaran biasa (Dr Piutang / Cr Kas untuk refund
   pelanggan, Dr Kas / Cr Hutang untuk refund pemasok), sehingga masuk laporan Kas.
   - Metode wajib `cash` / `transfer` / `check` (bukan teks bebas), dengan kas atau rekening seperti butir 1.
   - Idempoten per penyelesaian: `clientRef` dari form (kunci baru setiap form dibuka), atau, bila kosong, urutan
     memo uang yang sudah ada. Kirim ulang tidak membayar dua kali. Bila pembayaran berhasil tetapi memo gagal,
     percobaan berikutnya memakai ulang pembayaran itu (ADR-0016).
   - Ref klien pembayarannya `settle:sr|pr:<id retur>:<kunci>`. **Membatalkan pembayaran itu menghapus memo
     cerminannya** (`ReleaseSettlementPayment`), jadi sisanya bisa diselesaikan lagi. Pola ini sama dengan offset
     otomatis (`offset:sr:`).
   - `collect_payment` dari halaman retur kini juga memerlukan `payment.create` (sama dengan `pay_now`).
   - `customer_credit` / `supplier_credit` tidak berubah (memo + jurnal reklasifikasi, K-04a).
3. **Pembeli walk-in bernama "Umum"**, bukan "Tunai" (`salescontracts.WalkInPartyName`), di semua tempat yang
   menampilkan nama pihak (daftar/detail order, daftar pembayaran, umur piutang, asisten). Label lama terbaca
   seperti metode bayar, sehingga penjualan walk-in yang dibayar transfer tampak "Tunai".

## Consequences
- Memo refund/tambah bayar lama (sebelum keputusan ini, termasuk hasil migrasi) tetap memo saja. Bila uangnya
  memang belum dicatat, catat lewat menu Pembayaran seperti sebelumnya.
- Koreksi pembayaran refund (ADR-0023) membuat pembayaran pengganti tanpa mengubah memo: memo tetap menunjuk nomor
  dan nominal pembayaran asli. Koreksi metode/kas/rekening aman. Bila **nominal** refund salah, batalkan
  pembayarannya (memo ikut terhapus) lalu catat ulang penyelesaiannya.
- Akun kas yang tampil di pilihan mengikuti tanda "Kas" (`is_cash`) pada bagan akun. Kas Besar yang dibuat tanpa
  tanda itu tidak muncul di pilihan maupun di laporan Kas.
- Saldo awal Kas Besar (bila uangnya dipindah dari Kas Tunai) dicatat dengan jurnal manual oleh admin keuangan.
