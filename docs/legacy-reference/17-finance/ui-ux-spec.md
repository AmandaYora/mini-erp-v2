# UI/UX Spec — Modul 17 Finance / Accounting & Pajak

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Enam belas rute dalam empat
seksi menu. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Komponen bersama dijelaskan di [../shared/shared-services.md](../shared/shared-services.md).

---

## 1. Peta Menu — Empat Seksi

Grup sidebar **"Keuangan"**, terbagi empat seksi. Seksi **Harian** terbuka secara bawaan.

### Seksi 1 — Harian

| Label menu | Rute | Judul halaman | Permission |
|---|---|---|---|
| Tutup Buku | `/finance/back-office` | Tutup Buku | `finance.posting.view` |
| Kas & Rekening | `/finance/cash` | Kas & Rekening | `finance.report.view` |
| Biaya Usaha | `/finance/expenses` | Biaya Usaha | `finance.expense.view` |

### Seksi 2 — Cek Bisnis

| Label menu | Rute | Judul halaman | Permission |
|---|---|---|---|
| Hutang & Tagihan | `/finance/receivables-payables` | Hutang & Tagihan | `finance.report.view` |
| Untung Rugi (Cepat) | `/finance/margin` | Untung Rugi (Cepat) | `finance.report.view` |
| Untung Rugi | `/finance/profit-loss` | Untung Rugi | `finance.report.view` |
| Posisi Harta & Hutang | `/finance/balance-sheet` | Posisi Harta & Hutang | `finance.report.view` |

### Seksi 3 — Pajak & Akhir Bulan

| Label menu | Rute | Judul halaman | Permission |
|---|---|---|---|
| Pajak | `/finance/tax` | Pajak | `finance.tax_report.view` |
| Detail Pajak Per Faktur | `/finance/tax-detail` | Detail Pajak Per Faktur | `finance.tax_report.view` |
| Penyesuaian Pajak (Worksheet) | `/finance/tax-adjustments` | Penyesuaian Pajak | `finance.tax_adjustment.view` |
| Kunci Bulan & Berkas Pajak | `/finance/period-close` | Kunci Bulan & Berkas Pajak | `finance.view` |

### Seksi 4 — Detail

| Label menu | Rute | Judul halaman | Permission |
|---|---|---|---|
| Catatan Keuangan | `/finance/journals` | Catatan Keuangan | `finance.journal.view` |
| Rincian Semua Catatan | `/finance/general-ledger` | Rincian Semua Catatan | `finance.report.view` |
| Cek Saldo Akun | `/finance/trial-balance` | Cek Saldo Akun | `finance.report.view` |
| Saldo Awal | `/finance/opening` | Saldo Awal | `finance.view` |
| Pengaturan Keuangan | `/finance/settings` | Pengaturan Keuangan | `finance.view` |

**Kontrak wording yang wajib dipertahankan.** Seluruh istilah akuntansi diterjemahkan ke bahasa
pemilik:

| Istilah akuntansi | Yang dilihat pengguna |
|---|---|
| Jurnal | **Catatan Keuangan** |
| Buku besar | **Rincian Semua Catatan** |
| Neraca saldo | **Cek Saldo Akun** |
| Neraca | **Posisi Harta & Hutang** |
| Laba rugi | **Untung Rugi** |
| Tutup periode | **Kunci Bulan** |
| Antrian posting | **Tutup Buku** |
| Jurnal pembalik | **Catatan pembalik** |

Ini penerapan langsung aturan *"UI finance wajib tetap owner-friendly"*.

---

## 2. Halaman: Tutup Buku (`/finance/back-office`)

Halaman harian utama. Deskripsi:
*"Pastikan semua transaksi hari ini sudah masuk pembukuan sebelum menutup toko."*

### 2.1 Yang dilihat pengguna

| Bagian | Isi |
|---|---|
| Status hari ini | Salah satu dari empat: **kosong** · **belum** · **selesai_catatan** · **lengkap** |
| Ringkasan angka | Jumlah transaksi hari ini per kelompok |
| Daftar antrian | Berhalaman, dengan penanda status dan jejak sumber |
| Tugas yang butuh data | **Hanya Tipe B** — dikelompokkan per akar masalah |
| Aksi | Tarik transaksi · Pratinjau · Posting · Posting massal · Tutup hari · Abaikan · Pulihkan · Batal posting |

### 2.2 Kontrak yang paling penting

**Antrian Tipe A tidak pernah ditampilkan sebagai tugas.** Transaksi yang sekadar menunggu
urutan posting (menunggu induk penjualan, menunggu retur diposting) disembunyikan — ia akan selesai
sendiri saat tombol "tutup hari" ditekan.

Yang muncul sebagai tugas hanyalah **Tipe B**: butuh harga modal, butuh alokasi pembayaran, butuh
order asal. Tanpa pemisahan ini pemilik akan melihat puluhan "tugas" yang sebenarnya plumbing.

### 2.3 Empat status hari ini

| Status | Arti bagi pengguna |
|---|---|
| **kosong** | Belum ada transaksi hari ini — bukan "sudah bersih" |
| **belum** | Masih ada yang harus diposting |
| **selesai_catatan** | Posting rutin beres, ada catatan yang butuh data Anda |
| **lengkap** | Semua transaksi hari ini sudah masuk buku |

Pembedaan **kosong** dari **lengkap** disengaja dan wajib dipertahankan.

---

## 3. Halaman: Saldo Awal (`/finance/opening`)

Halaman dengan **panduan empat langkah bernomor** — satu-satunya di sistem yang berbentuk wizard
naratif.

| Langkah | Judul kartu |
|---|---|
| 1 | **1. Kapan pembukuan dimulai?** |
| 2 | **2. Saldo uang & utang-piutang di tanggal itu** |
| 3 | **3. Stok barang & harga modal awal** |
| 4 | **4. Tinjau & kunci saldo awal** |

Label sisi kiri: *"Langkah pengisian saldo awal"*.

### 3.1 Keadaan yang mungkin muncul

| Judul pemberitahuan | Kapan |
|---|---|
| **Tanggal mulai belum diisi** | Cutover kosong |
| **Masih perlu dilengkapi** | Validasi menemukan masalah |
| **Saldo awal siap dikunci** | Validasi bersih |
| **Saldo awal sudah dikunci** | Sudah diposting |
| **Saldo awal aktif** | Ringkasan pasca-kunci |
| **Sekali dikunci, tidak bisa diubah** | Peringatan sebelum mengunci |
| **Lengkapi stok awal yang terlewat** | Ada produk tertahan `missing_cost_basis` |
| **Hanya bisa lihat** | Tanpa `finance.manage` |

### 3.2 Kontrak yang wajib dipertahankan

**Kolom harga modal tidak ada di layar.** Pengguna hanya mengisi cabang, produk, dan jumlah —
harga modal diambil sistem dari harga beli master produk. Ini hasil langsung insiden Agustus 2026
(241 produk salah 10–1000×). Menampilkan kembali field itu berarti mengulang insiden.

**Jalur "Lengkapi Stok Awal"** muncul otomatis ketika ada produk yang membuat posting tertahan,
lengkap dengan daftar produknya dan harga modal yang disarankan.

---

## 4. Halaman: Biaya Usaha (`/finance/expenses`)

| Bagian | Isi persis |
|---|---|
| Judul | **Biaya Usaha** |
| Keadaan kosong | **Belum ada biaya** — *"Klik Tambah Biaya untuk mencatat pengeluaran usaha pertama Anda."* |
| Form tambah | **Catat Biaya Baru** — *"Isi tanggal, jenis biaya, sumber pembayaran, dan nominal."* |
| Ringkasan | **Total Biaya (filter aktif)** · **Jumlah Catatan** |
| Dialog batal | **Batalkan Biaya** — *"Catatan keuangannya akan otomatis dibalik. Aksi tidak menghapus data; jejak audit tersimpan."* |

Deskripsi form sengaja tidak menyebut "akun debit/kredit" — hanya *"jenis biaya"* dan
*"sumber pembayaran"*.

Kalimat dialog pembatalan adalah contoh terbaik wording modul ini: menjelaskan **apa yang terjadi**
("dibalik"), **apa yang tidak terjadi** ("tidak menghapus data"), dan **jaminannya** ("jejak audit
tersimpan").

---

## 5. Halaman: Kunci Bulan & Berkas Pajak (`/finance/period-close`)

Halaman paling konsekuensial di modul ini.

| Bagian | Isi persis |
|---|---|
| Judul | **Kunci Bulan & Berkas Pajak** |
| Remah roti | `Keuangan > Kunci Bulan` |
| Deskripsi | *"Periksa kesiapan, kunci periode, dan unduh paket berkas untuk konsultan pajak."* |
| Pemilih periode | Dropdown dengan kode + rentang tanggal + status |
| Tombol | **Refresh Cek** (*"Memuat..."* saat berjalan) |
| Keadaan kosong | **Pilih periode** — *"Pilih periode finance untuk melihat checklist kunci bulan."* |

### 5.1 Checklist

Kartu **Kunci Bulan**, deskripsi:
*"Kunci periode setelah semua item checklist beres agar angka periode ini tidak berubah lagi."*

Legenda warna yang ditampilkan eksplisit:

> *"Hijau = OK; Kuning = peringatan (boleh lanjut); Merah = harus diperbaiki dulu."*

Enam pemeriksaan dengan label owner-friendly — lihat [reports-list.md](reports-list.md) §7.

### 5.2 Dialog konfirmasi

| Dialog | Kapan | Isi |
|---|---|---|
| **Kunci Periode?** | Semua bersih | Konfirmasi biasa |
| **Periode belum aman dikunci** | Ada yang gagal | Menahan aksi |
| **Kunci Paksa?** | Menekan paksa | *"Anda akan mengunci periode meski masih ada checklist gagal. Aksi ini tercatat di audit log. Lanjutkan?"* |
| **Buka Kunci Periode Keuangan?** | Membuka kembali | Wajib mengisi alasan |
| **Buka Kunci Periode Pajak?** | Membuka periode pajak | Wajib mengisi alasan |

### 5.3 ⚠️ Kartu "Unduh Berkas Pajak" — dua tombol berdampingan

| Bagian | Isi persis |
|---|---|
| Judul kartu | **Unduh Berkas Pajak** |
| Deskripsi | *"Paket Excel berisi laba rugi, neraca, buku besar, dan rincian PPN — untuk diberikan ke konsultan pajak."* |

| Posisi | Label | Sub-label |
|---|---|---|
| Kiri | **Data riil (apa adanya)** | *(Layer 1)* |
| Kanan | **Versi dibatasi Rp4,8 M** | *(Layer 2)* |

Kedua tombol berbunyi **Unduh**, berubah jadi *"Menyiapkan..."* saat berjalan. Berhasil →
toast **"Berkas siap"** dengan nama berkas dan ukurannya.

**Labelnya jujur** — pengguna tahu mana yang riil dan mana yang dibatasi. Yang perlu diputuskan
sebelum rebuild adalah apakah tombol kedua tetap ada. Lihat
[open-questions.md](../open-questions.md) **OQ-A42**.

**Tidak ada jejak audit** saat tombol mana pun ditekan.

---

## 6. Halaman: Pengaturan Keuangan (`/finance/settings`)

| Bagian | Isi persis |
|---|---|
| Judul | **Pengaturan Keuangan** |
| Deskripsi | *"Daftar akun, arah pencatatan, kas/rekening, dan periode keuangan. Untuk admin/konsultan."* |
| Kartu | **Tambah Akun** · **Arah Posting Akun** · **Kas & Rekening** · **Periode Keuangan** · **Periode Pajak** |

Deskripsi halaman **secara eksplisit menandai dirinya "untuk admin/konsultan"** — pengakuan bahwa
halaman ini memang berbahasa akuntansi, berbeda dari halaman lain.

**"Arah Posting Akun"** adalah nama owner-friendly untuk mapping akun.

---

## 7. Halaman: Catatan Keuangan (`/finance/journals`)

| Bagian | Isi persis |
|---|---|
| Judul | **Catatan Keuangan** |
| Deskripsi | *"Catatan keuangan resmi yang dipakai laporan. Termasuk catatan pembalik bila ada koreksi."* |
| Tabel | **Daftar Jurnal** |

Deskripsi menyiapkan pengguna untuk melihat **pasangan jurnal asli + pembalik** — mencegah kesan
"kok datanya dobel".

---

## 8. Halaman: Pajak (`/finance/tax`)

| Bagian | Isi persis |
|---|---|
| Judul | **Pajak** |
| Deskripsi | *"Ringkasan DPP dan PPN dari order. Dipakai sebagai bahan cek pajak bersama konsultan."* |
| Kartu | **Periode Pajak** · **Transaksi Perlu Faktur** |

Kartu **"Transaksi Perlu Faktur"** menampilkan order kena pajak yang nomor fakturnya masih kosong
— pekerjaan yang harus dibereskan sebelum lapor.

---

## 9. Pola Lintas Halaman

| Pola | Perilaku |
|---|---|
| Filter tanggal | Selalu `date_from` + `date_to`, ditafsirkan sebagai hari **WIB** |
| Paginasi | Seluruh daftar berhalaman |
| Angka rupiah | Format ribuan Indonesia, negatif berwarna merah |
| Aksi merusak | **Selalu** lewat dialog konfirmasi |
| Buka kunci | **Selalu** wajib alasan |
| Kegagalan | Toast merah dengan **pesan asli server** |
| Keberhasilan | Toast hijau |

**Modul ini menampilkan pesan server pada kegagalan** — sama seperti modul 05 Product, berbeda dari
modul 01–04 yang membuangnya. Ini penting karena pesan finance memuat instruksi yang bisa
ditindaklanjuti (*"Lengkapi harga beli produk..."*).

---

## 10. Yang Perlu Dikonfirmasi

**[PERLU KONFIRMASI]** Halaman **Nilai Persediaan** punya endpoint (`finance/reports/inventory-value`)
tetapi **tidak punya rute menu**. Ia hanya dipanggil dari dalam halaman lain. Apakah pemilik pernah
membutuhkan layar nilai persediaan tersendiri?

**[PERLU KONFIRMASI]** Aturan responsif dan CSS cetak belum ditelusuri per halaman. Seluruh halaman
finance memakai kelas utilitas standar sistem, tetapi tidak ada halaman finance yang punya tata
letak cetak khusus — laporan diekspor ke Excel, bukan dicetak dari layar. Perlu dipastikan tidak
ada pemilik yang mencetak layar laporan langsung dari browser.
