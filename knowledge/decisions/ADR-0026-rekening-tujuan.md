# ADR-0026: Rekening perusahaan, rekening tujuan tagihan, dan rekening pembayaran

## Status
Accepted (2026-10-26). Disetujui pemilik sebagai rekomendasi final. Analisis dan riset:
[analysis/002-rekening-tujuan-dan-retur-sebagian](../../analysis/002-rekening-tujuan-dan-retur-sebagian/analysis.md).
**Revisi 2026-10-26** ([analysis/004](../../analysis/004-satu-sumber-rekening/analysis.md)): master adalah satu-satunya
sumber rekening. Editor rekening di Pengaturan › Perusahaan dihapus; rekening teks lama dipindahkan otomatis sekali
saat start (butir 9). Butir 6–9 dan Consequences di bawah sudah memuat revisi ini.
Merevisi catatan `finance_cash_accounts` di `DB_SCHEMA.md` §6 dan menambah satu pengecualian pada
ADR-0018 D12 (dokumen cetak). **Angka saldo per rekening untuk jurnal reklasifikasi: akuntan.**

## Context
- Perusahaan punya beberapa rekening (BCA, Mandiri, …). Kasir dan pembuat order tidak bisa
  menentukan order atau tagihan dibayar ke rekening mana.
- Rekening hanya teks `bankAccounts` di `document_profile` (cetak). Pembayaran hanya menyimpan
  metode. Jurnal transfer/cek selalu ke satu kunci pemetaan `bank` (akun 1010), jadi saldo per
  rekening tidak terlihat dan tidak bisa dicocokkan dengan rekening koran.
- Sistem lama juga menentukan akun kas dari metode saja (legacy BR-14). Ini fitur baru, bukan
  paritas yang hilang.
- Riset (analisis 002 §3): Odoo dan ERPNext memodelkan rekening sebagai master 1:1 ke akun GL kas/bank,
  menyimpan rekening tujuan di dokumen tagihan (bisa diubah, dilacak), mengisi otomatis rekening
  pembayaran dari tagihan, dan mencetak rekening itu. Jurnal, Accurate, dan Kledo memilih akun
  "Setor ke" per penerimaan.

## Decision
1. **Master rekening di modul finance.** Tabel `finance_bank_accounts`: bank, nomor, atas nama,
   tampil di dokumen, default (satu saja), status. Setiap rekening tertaut 1:1 ke akun aset kas
   (`finance_accounts.is_cash`) miliknya sendiri. Akun pemetaan `cash` (kas tunai) tidak boleh
   menjadi rekening. Saldo tetap milik akun buku besar, tabel ini tidak menyimpan saldo.
   Modul lain menyimpan **id akun buku besar** rekening sebagai acuan (ID primitif).
2. **Rekening tujuan di order jual** (`sales_orders.receipt_account_id`). SO adalah tagihan (nota
   mencetak SO).
   - Order baru: rekening default, kecuali pengguna memilih yang lain.
   - Bisa diubah sebagai kolom ringan pada order terkonfirmasi.
   - Untuk order apa pun yang tidak batal, termasuk yang sudah selesai, bisa diubah lewat
     `PUT /sales-orders/{id}/receipt-account` dengan audit sebelum/sesudah.
   - Update yang tidak mengirim kolom ini mempertahankan nilainya.
3. **Rekening pembayaran** (`payments.cash_account_id`), hanya untuk transfer/cek. Urutan penentuan:
   1. pilihan pengguna;
   2. rekening tujuan tagihan yang dialokasi, bila masih aktif;
   3. rekening default.

   Aturan tambahan:
   - Begitu ada rekening aktif, transfer/cek wajib berakhir dengan satu rekening (galat validasi 400 bila tidak ada
     default).
   - Tunai tidak membawa rekening.
   - Tidak bisa diubah setelah dicatat. Salah pilih diperbaiki lewat koreksi pembayaran (ADR-0023):
     jurnal lama dibalik, lalu yang baru dijurnal ke rekening yang benar.
   - Berlaku juga untuk uang keluar (bayar pemasok, refund) dan semua jalur pelunasan: COD saat SJ,
     SO dari SJ Langsung, bayar-sekarang penerimaan barang, tambah bayar tukar, dan POS.
4. **Jurnal.**
   - `buildPayment` memakai akun rekening bila `cash_account_id` terisi (harus aset kas aktif).
   - Bila kosong, tetap memakai pemetaan `cash`/`bank`. Pembayaran historis dan hasil ETL tidak berubah.
5. **"Kas & Bank" di dasbor** = jumlah semua akun aset kas (`CashOnHand`), bukan kunci `cash`+`bank`.
   Tanpa perubahan ini, rekening baru tidak ikut terhitung.
6. **Cetak (pengecualian ADR-0018 D12).**
   - Nota/tagihan yang masih bersisa mencetak satu baris "Transfer ke: …" di kaki, dan baris ini
     tetap tercetak pada kertas kontinu yang preprinted.
   - Bukti pembayaran mencetak "Ke/Dari rekening".
   - Struk POS transfer atau piutang mencetak rekening itu saja. Struk lain mencetak daftar rekening
     dokumen seperti sistem lama.
   - Daftar rekening dokumen diambil dari master (aktif + tampil di dokumen). Daftar teks lama di
     profil dokumen (`document_profile.bankAccounts`, umumnya hasil migrasi) hanya dicetak selama master
     **tidak berisi rekening sama sekali**. Begitu master berisi, dokumen tidak pernah kembali ke teks lama,
     walau semua rekening diarsipkan atau disembunyikan.
7. **Akses.**
   - Kelola: `finance.manage`, di halaman Akun → tab Rekening bank, satu-satunya tempat rekening diubah.
   - Pengaturan › Perusahaan › "Kontak dan rekening" hanya menampilkan rekening di dokumen (baca saja) dengan
     tautan **Kelola rekening**. Teks lama tetap tersimpan apa adanya saat profil disimpan.
   - Baca untuk pemilih dan cetak: `GET /finance/bank-accounts/options`, cukup login dan cabang
     (pola `company/print-profile`).
8. **Tidak ada perubahan perilaku sebelum rekening dibuat.** Master kosong berarti transfer tetap ke
   1010, tidak ada pemilih yang tampil, dan nota tidak berubah.
9. **Pemindahan otomatis rekening teks lama** (`bootstrap.ImportLegacyBankAccounts`, setiap start server).
   - Berjalan hanya bila master **tidak berisi rekening sama sekali** dan profil dokumen punya
     `bankAccounts`; sesudah itu tanpa efek. Setelah data legacy dimuat ulang (cutover) ia berjalan lagi pada
     start berikutnya.
   - Urutan teks dipertahankan (`sortOrder`), semua tampil di dokumen, baris kosong dilewati, nomor ganda
     (tanpa spasi/titik/strip) dilewati. Atas nama kosong diisi nama resmi/nama perusahaan.
   - Rekening **pertama** = default, di atas akun pemetaan `bank` (**1010**), yang sudah menampung semua
     transfer selama ini. Rekening berikutnya mendapat akun kas/bank baru 1011, 1012, ….
   - Semua baris divalidasi sebelum menulis; hasil (dibuat/dilewati) atau galat tercatat di log server.
     Galat tidak menghentikan start. Teks di profil dokumen tidak diubah.

## Consequences
- **Aktivasi terjadi saat start pertama setelah rilis** yang memuat butir 9 (dan lagi sesudah cutover final
  memuat ulang data). Sejak itu transfer/cek wajib memilih rekening. Langkah akuntan:
  1. bila ada lebih dari satu rekening: jurnal manual reklasifikasi per cabang, Dr akun rekening 1011, 1012, … /
     Cr 1010, sebesar saldo rekening koran masing-masing per tanggal aktivasi. Sisa 1010 = saldo rekening pertama;
  2. periksa default dan rekening yang tampil di dokumen, ubah bila perlu di Rekening bank;
  3. umumkan ke kasir.

  Dengan satu rekening, tidak ada reklasifikasi: 1010 langsung menjadi akun rekening itu. Pemetaan `bank` tetap 1010
  (sudah berjurnal) dan juga menampung pembayaran lama tanpa rekening.
- ETL tidak berubah. Verifikasi harian ETL (`etl/verify/daily.go`) tetap membandingkan kunci
  `cash`/`bank`, dan itu benar untuk hari migrasi.
- Laporan Kas & Rekening dan tutup harian sudah per akun `is_cash`, jadi langsung per rekening.
- Tidak termasuk: rekonsiliasi bank otomatis, QRIS/EDC sebagai metode, rekening default per pelanggan
  atau per cabang, pembatasan rekening per cabang. Semuanya bisa ditambah tanpa mengubah model ini.
