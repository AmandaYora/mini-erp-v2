# Reports — Modul 17 Finance / Accounting & Pajak

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Dua belas laporan + satu paket
ekspor + satu checklist kesiapan. Setiap logika perhitungan diturunkan dari kode.

---

## 1. Daftar Lengkap Laporan

| # | Laporan | Endpoint | Halaman | Permission |
|---|---|---|---|---|
| 1 | Ringkasan Kas & Rekening | `finance/reports/cash-summary` | Kas & Rekening | `finance.report.view` |
| 2 | Piutang per Pelanggan | `finance/reports/receivables` | Hutang & Tagihan | `finance.report.view` |
| 3 | Hutang per Supplier | `finance/reports/payables` | Hutang & Tagihan | `finance.report.view` |
| 4 | Nilai Persediaan | `finance/reports/inventory-value` | *(dipakai internal)* | `finance.report.view` |
| 5 | Margin (cepat) | `finance/reports/margin` | Untung Rugi (Cepat) | `finance.report.view` |
| 6 | Laba Kotor | `finance/reports/gross-profit` | Untung Rugi (Cepat) | `finance.report.view` |
| 7 | Buku Besar | `finance/reports/general-ledger` | Rincian Semua Catatan | `finance.report.view` |
| 8 | Neraca Saldo | `finance/reports/trial-balance` | Cek Saldo Akun | `finance.report.view` |
| 9 | Laba Rugi | `finance/reports/profit-loss` | Untung Rugi | `finance.report.view` |
| 10 | Neraca | `finance/reports/balance-sheet` | Posisi Harta & Hutang | `finance.report.view` |
| 11 | Ringkasan PPN | `finance/reports/tax-summary` | Pajak | `finance.tax_report.view` |
| 12 | Rincian PPN per Faktur | `finance/reports/tax-detail` | Detail Pajak Per Faktur | `finance.tax_report.view` |
| — | **Paket Berkas Pajak (Excel)** | `finance/export/tax-package` | Kunci Bulan | `finance.report.view` |
| — | **Checklist Kesiapan Tutup** | `finance/close/readiness` | Kunci Bulan | `finance.view` |

**Nama yang dilihat pengguna sengaja dibuat awam** — "Cek Saldo Akun" bukan "Neraca Saldo",
"Rincian Semua Catatan" bukan "Buku Besar", "Posisi Harta & Hutang" bukan "Neraca". Ini konsisten
dengan aturan wording owner-friendly dan **wajib dipertahankan**.

---

## 2. Sumber Angka: Satu Kebenaran

**Seluruh laporan 1–10 dihitung dari `finance_journal_lines`** — tidak ada satu pun yang membaca
tabel order/payment/stok secara langsung.

Konsekuensinya, yang harus dipahami sebelum rebuild:

| Sifat | Akibat |
|---|---|
| Transaksi yang belum diposting **tidak muncul** di laporan | Angka laporan = angka yang sudah masuk buku, bukan seluruh aktivitas bisnis |
| Laporan tidak bisa "salah" terhadap jurnal | Selisih hanya mungkin muncul dari antrian yang belum diposting |
| Menutup hari = membuat laporan lengkap | Inilah alasan halaman "Tutup Buku" ada |

Pengecualian: laporan **Nilai Persediaan** (#4) membaca buku besar harga modal, dan **Rincian PPN**
(#12) menggabungkan jurnal dengan tabel `orders` untuk mengambil nomor faktur dan tarif pajak.

---

## 3. Aturan Tanggal — Zona Waktu WIB

Seluruh laporan berbasis rentang tanggal memakai konversi zona **Asia/Jakarta**, bukan
perbandingan `YYYY-MM-DD` mentah.

Alasannya: `journal_date` tersimpan sebagai instant UTC, sementara pemilik memilih hari WIB. Tanpa
konversi, transaksi 1 Agustus 01:00 WIB akan jatuh ke 31 Juli.

| Jenis filter | Perlakuan |
|---|---|
| `date_from` | Awal hari WIB |
| `date_to` | **Akhir hari WIB, inklusif** |
| `as_of_date` (neraca, neraca saldo) | Akumulasi seluruh jurnal **sampai akhir hari itu** |
| Saldo awal periode (ringkasan kas) | Akumulasi jurnal **sebelum** `date_from` |

Format tanggal wajib `YYYY-MM-DD`; nilai lain ditolak.

**Ini kontras dengan modul Order** yang filter `date_to`-nya tidak mencakup akhir hari (OQ-A32).
Finance sudah benar; modul Order yang perlu diperbaiki.

---

## 4. Logika Perhitungan Per Laporan

### 4.1 Ringkasan Kas & Rekening

| Kolom | Perhitungan |
|---|---|
| Saldo awal | Σ(debit − kredit) seluruh jurnal **sebelum** `date_from` pada akun kas itu |
| Kas masuk | Σ debit dalam rentang |
| Kas keluar | Σ kredit dalam rentang |
| Saldo akhir | Saldo awal + masuk − keluar |

Satu baris per kas/rekening terdaftar. Saldo awal **tidak** ditambah kolom cutover terpisah —
saldo awal sudah ikut terjurnal saat dikunci, jadi menambahkannya lagi akan menghitung ganda.

### 4.2 Piutang & Hutang per Pihak

Keduanya laporan yang sama dengan mapping key berbeda (`accounts_receivable` / `accounts_payable`),
dikelompokkan per **pihak bisnis** dari kolom dimensi baris jurnal.

Saldo = Σ(debit − kredit) untuk piutang; kebalikannya untuk hutang.

**Yang perlu diketahui:** baris jurnal tanpa pihak (mis. pembayaran tanpa alokasi) tidak masuk
kelompok mana pun. Total laporan ini karena itu bisa berbeda dari saldo akun di Neraca.

### 4.3 Nilai Persediaan

Dibaca dari **buku besar harga modal** (bukan jurnal): jumlah, harga modal rata-rata, dan nilai per
(cabang, produk). Bisa disaring per cabang.

**Ini satu-satunya laporan yang bisa berbeda dari Neraca** — bila ada jurnal persediaan yang
diposting tanpa mutasi biaya, atau sebaliknya.

### 4.4 Margin & Laba Kotor

Dua laporan "cepat" untuk pemilik: pendapatan − HPP, tanpa biaya operasional. Berbasis akun
bermapping `sales_revenue` dan `cogs`.

### 4.5 Buku Besar

Seluruh baris jurnal dalam rentang, dikelompokkan per akun, dengan saldo berjalan. Menyertakan
saldo awal per akun (akumulasi sebelum rentang).

### 4.6 Neraca Saldo

Saldo tiap akun per tanggal tertentu, dipisah kolom debit/kredit menurut arah saldo bersihnya.
Menyertakan penanda **seimbang / tidak seimbang** — dipakai checklist tutup bulan.

### 4.7 Laba Rugi

Akun bertipe pendapatan dan biaya dalam rentang. Laba bersih = pendapatan − biaya.

### 4.8 Neraca

Akun harta, hutang, dan modal per tanggal tertentu. Menyertakan penanda
**Harta = Hutang + Modal**.

**Catatan penting:** tidak ada penutupan akun nominal (tutup tahun). Laba berjalan karena itu
**tidak otomatis masuk ke ekuitas** — bila neraca tetap seimbang, itu karena laba berjalan ikut
dihitung sebagai bagian dari sisi modal oleh perhitungan laporan, bukan karena ada jurnal penutup.

### 4.9 Ringkasan PPN

| Kolom | Perhitungan |
|---|---|
| PPN keluaran | Σ kredit akun `output_tax` dari order penjualan |
| PPN masukan | Σ debit akun `input_tax` dari order pembelian |
| DPP penjualan | Σ (PPN ÷ tarif × 100) per order |
| DPP pembelian | Idem |
| PPN kurang bayar | Keluaran − masukan |

**Tarif pajak tidak pernah berasal dari pengaturan sistem** — ia diambil dari `tax_rate_snapshot`
pada order, yang **diketik per order** di modul Order (bawaan 0 bila tidak diisi). Tidak ada tarif
11% yang di-hardcode di mana pun. Ini menutup pertanyaan lama "di mana tarif PPN & DPP-fallback
berada": **di modul Order, per dokumen.**

### 4.10 Rincian PPN per Faktur

Satu baris per order kena pajak: nomor order, tanggal, nomor faktur, pihak, tarif, DPP, PPN, total,
dan penanda **faktur kosong**. Berhalaman.

---

## 5. ⚠️ Dua Laporan PPN Memakai Rumus DPP yang Berbeda

Bila tarif pajak pada order bernilai 0 atau kosong sementara nominal PPN-nya tidak nol:

| Laporan | DPP yang ditampilkan |
|---|---|
| Ringkasan PPN | **0** |
| Rincian PPN per Faktur | **Subtotal sebelum pajak** (nilai sebenarnya) |

Akibatnya kedua laporan **tidak akan rekonsiliasi** untuk order semacam itu — dan keduanya masuk
paket berkas pajak yang sama. Lihat KI-145.

---

## 6. Paket Berkas Pajak — Delapan Sheet, Dua Versi

Satu berkas Excel, dikirim sebagai base64 untuk diunduh browser.

| # | Sheet | Isi |
|---|---|---|
| 1 | Sampul | Judul, periode, daftar isi |
| 2 | Laporan Untung Rugi | Laba rugi periode |
| 3 | Neraca | Posisi per tanggal akhir |
| 4 | Cek Saldo Akun | Neraca saldo |
| 5 | Rincian Semua Catatan | Buku besar penuh |
| 6 | PPN Keluaran | Rincian per faktur penjualan |
| 7 | PPN Masukan | Rincian per faktur pembelian |
| 8 | Penyesuaian Pajak | Worksheet rekonsiliasi fiskal |

Nama berkas: `berkas-pajak_{tanggal awal}_{tanggal akhir}.xlsx`

### 6.1 Layer 1 vs Layer 2

Halaman Kunci Bulan menampilkan **dua tombol unduh berdampingan**, di bawah kartu berjudul
*"Unduh Berkas Pajak"* dengan deskripsi *"Paket Excel berisi laba rugi, neraca, buku besar, dan
rincian PPN — untuk diberikan ke konsultan pajak."*

| Tombol | Label persis | Isi |
|---|---|---|
| Kiri | **"Data riil (apa adanya)"** *(Layer 1)* | Seluruh data periode |
| Kanan | **"Versi dibatasi Rp4,8 M"** *(Layer 2)* | Order penjualan dibuang sampai omzet kumulatif ≤ Rp 4.800.000.000 |

Layer 2 membuang **seluruh jurnal milik order terpilih** (penjualan + HPP + PPN + pembayaran),
sehingga kedelapan sheet tetap seimbang dan konsisten satu sama lain. Angka Rp 4,8 miliar adalah
ambang peredaran bruto PP-23.

**Ini keputusan bisnis yang harus dikonfirmasi pemilik sebelum dibawa ke sistem baru**, bukan
temuan teknis. Mekanismenya di [algorithms-legacy.md](algorithms-legacy.md) §7; pertanyaannya di
[open-questions.md](../open-questions.md) **OQ-A42**.

### 6.2 Tidak ada validasi rentang tanggal

Endpoint ekspor **tidak memeriksa apa pun**: tanggal kosong, tanggal terbalik, maupun rentang
bertahun-tahun semuanya diterima, lalu delapan sheet dibangun seluruhnya di memori.

Tidak ada batas 366 hari maupun 5.000 baris — angka itu milik ekspor modul Order, bukan Finance.
Pada VPS 2 GB ini risiko nyata. Lihat KI-144.

---

## 7. Checklist Kesiapan Tutup Bulan — Enam Pemeriksaan

| # | Kunci | Label yang dilihat pengguna | Gagal bila |
|---|---|---|---|
| 1 | `source_unposted` | *Semua transaksi periode ini sudah masuk laporan keuangan* | Ada sumber `ready` atau `pending` |
| 2 | `source_failed` | *Tidak ada source yang gagal posting* | Ada sumber `failed` |
| 3 | `source_ignored` | *Transaksi yang diabaikan sudah dicek* | — (hanya **peringatan** bila ada) |
| 4 | `trial_balance` | *Catatan keuangan (Cek Saldo Akun) seimbang* | Debit ≠ Kredit |
| 5 | `balance_sheet` | *Posisi Harta = Hutang + Modal* | Tidak seimbang |
| 6 | `tax_missing_invoice` | *Faktur pajak sudah diisi untuk transaksi kena pajak* | — (hanya **peringatan**) |
| 7 | `inventory_value_negative` | *Tidak ada produk dengan nilai/harga modal persediaan negatif* | Ada jumlah/nilai/harga modal negatif |
| 8 | `period_status` | *Periode finance siap dikunci* | — (peringatan bila sudah tertutup) |

Ringkasannya menghitung jumlah ok/peringatan/gagal dan penanda **aman dikunci** (= nol gagal).

**Dua sifat yang layak dipertahankan:**

1. Pemeriksaan #7 dilakukan **seluruh perusahaan**, bukan hanya periode itu — karena kerusakan
   basis biaya bisa berasal dari periode kapan pun. Ini penjagaan hasil insiden Agustus 2026.
2. Peringatan tidak memblokir; hanya `failed` yang memblokir. Kunci paksa tetap mungkin, tapi
   butuh permission tersendiri.

---

## 8. Yang TIDAK Ada

| Laporan yang mungkin diharapkan | Kenyataan |
|---|---|
| Arus kas (metode langsung/tidak langsung) | Tidak ada — hanya ringkasan kas per rekening |
| Umur piutang / hutang (aging) | **Tidak ada di modul ini**; aging ada di modul Payment |
| Perbandingan antar periode | Tidak ada |
| Laporan per cabang | Kolom cabang ada di jurnal, tapi **tidak ada laporan yang mengelompokkan per cabang** |
| Laporan konsolidasi antar perusahaan | Tidak relevan (satu perusahaan) |
| Ekspor PDF | Tidak ada — hanya Excel |
| Ekspor per laporan | Tidak ada — hanya paket 8 sheet sekaligus |
| Penjadwalan laporan otomatis | Tidak ada |
| Laba ditahan / tutup tahun | Tidak ada |

**Kesenjangan yang paling terasa:** meski setiap baris jurnal menyimpan cabangnya, tidak satu pun
laporan yang menyajikan angka per cabang. Untuk perusahaan multi-cabang ini kesenjangan produk
yang nyata — lihat OQ-A20 (pertanyaan serupa di modul Branch).
