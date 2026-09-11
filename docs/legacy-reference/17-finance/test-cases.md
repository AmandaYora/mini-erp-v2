# Test Cases — Modul 17 Finance / Accounting & Pajak

**Kelompok A — skenario input→output nyata, diturunkan dari kode dan test yang sudah ada.**

Sumber: **164 test unit** di 12 berkas spec, plus kasus turunan pembacaan kode (ditandai
**[dari kode]**).

| Berkas spec | Test |
|---|---|
| `finance-posting.service.spec.ts` | 54 |
| `finance.service.spec.ts` | 30 |
| `finance-reporting.service.spec.ts` | 24 |
| `finance-inventory-cost.service.spec.ts` | 14 |
| `finance-posting-a2-a3.spec.ts` | 12 |
| `finance-posting-batch-equivalence.spec.ts` | 7 |
| `finance-close.service.spec.ts` | 6 |
| `business-expense.service.spec.ts` | 5 |
| `finance-posting.reasons.spec.ts` | 4 |
| `finance-document-sequence.util.spec.ts` | 3 |
| `finance-tax-adjustment.service.spec.ts` | 3 |
| `finance-export.service.spec.ts` | 2 |

**Modul dengan cakupan test terbaik di seluruh sistem** — tetapi sebarannya sangat tidak merata:
posting 54 test, ekspor hanya 2, penyesuaian pajak hanya 3.

---

## 1. Bagan Akun & Mapping

### TC-A01 — Kode akun ganda ditolak ✅ *ada di spec*
Diharapkan: **409** `Kode akun '{kode}' sudah digunakan`.

### TC-A02 — Arsip akun yang dipakai mapping ditolak ✅ *ada di spec*
Diharapkan: **400** `Akun masih dipakai mapping atau kas/rekening`.

### TC-A03 — Arsip akun yang pernah berjurnal ditolak ✅ *ada di spec*
Diharapkan: **400** dengan pesan yang menyarankan melepas dari mapping, bukan mengarsipkan.

### TC-A04 — Mapping key tak dikenal ditolak ✅ *ada di spec*
```
Request : { data: { mappings: [{ mapping_key: "foo", id_finance_account: 1 }] } }
```
Diharapkan: **400** `Mapping key tidak dikenal: foo`.

### TC-A05 — Remap akun yang sudah berjurnal ditolak ✅ *ada di spec*
Diharapkan: **400** dengan saran "Buat akun baru untuk transaksi ke depan".

### TC-A06 — Mengubah tipe akun yang sudah berjurnal **[dari kode]** ⚠️
```
Data    : akun 4000 "Penjualan" bertipe revenue, punya 500 baris jurnal
Request : { data: { id_finance_account: 4000, account_type: "expense" } }
```
Diharapkan **saat ini**: **berhasil tanpa peringatan**. Seluruh laporan laba rugi historis berubah
seketika. Tidak ada test yang menjaga ini (BR-05).

---

## 2. Periode

### TC-P01 — Rentang tumpang tindih ditolak ✅ *ada di spec*
Diharapkan: **400** `Rentang tanggal tumpang tindih dengan periode '{kode}' (...)`.

### TC-P02 — Buka kembali tanpa alasan ditolak ✅ *ada di spec*
Diharapkan: **400** `Alasan koreksi wajib diisi`.

### TC-P03 — Periode dicari memakai hari kalender WIB ✅ *ada di spec*
```
Data    : periode Agustus (2026-08-01 s.d. 2026-08-31) berstatus closed
Jurnal  : 2026-07-31 18:00 UTC  (= 1 Agustus 01:00 WIB)
```
Diharapkan: **ditolak** — hari WIB-nya masuk Agustus yang terkunci, meski hari UTC-nya Juli.

### TC-P04 — Jurnal di hari UTC terkunci tapi hari WIB terbuka ✅ *ada di spec*
Kebalikan TC-P03: diharapkan **diterima**.

### TC-P05 — Tanggal tidak masuk periode mana pun **[dari kode]**
Diharapkan: **lolos**. Penguncian bersifat *opt-in* (BR-27).

### TC-P06 — Tanggal jurnal rusak **[dari kode]**
Diharapkan: **400** `Tanggal transaksi tidak valid; tidak bisa menentukan periode keuangan` —
bukan galat internal 500.

---

## 3. Antrian Posting

### TC-Q01 — Sumber `failed` dievaluasi ulang tiap sinkronisasi ✅ *ada di spec*
Diharapkan: tidak dilewati berdasarkan sidik jari, sehingga **sembuh sendiri** begitu penyebabnya
diperbaiki.

### TC-Q02 — HPP tertahan sampai produk punya basis biaya ✅ *ada di spec*
Diharapkan: status `pending`, alasan **`missing_cost_basis`**.

### TC-Q03 — HPP siap lewat cadangan harga beli master ✅ *ada di spec*
Diharapkan: `ready` meski belum ada cost state.

### TC-Q04 — Koreksi stok **masuk** tanpa basis biaya tertahan ✅ *ada di spec*
Diharapkan: `pending` — bukan `ready` palsu yang gagal saat diposting.

### TC-Q05 — Koreksi stok **keluar** TIDAK tertahan ✅ *ada di spec*
Diharapkan: `ready` — memakai rata-rata/cadangan saat posting.

### TC-Q06 — Koreksi stok bernilai nol bersih ditandai `ignored` ✅ *ada di spec*
Diharapkan: opname yang cocok **tidak** jadi "Perlu dicek"/`failed`.

### TC-Q07 — HPP barang pengganti tertahan meski induk retur sudah diposting ✅ *ada di spec*
Diharapkan: `pending` `missing_cost_basis` — dua syarat harus terpenuhi, bukan satu.

### TC-Q08 — Dokumen sumber diarsipkan → antrian ikut dipensiunkan ✅ *ada di spec*

### TC-Q09 — Sinkron ulang saat hanya nilai turunan berubah ✅ *ada di spec*
Diharapkan: **diam** — tidak menandai "berubah" palsu (serialisasi berurutan kunci).

### TC-Q10 — Sinkron ulang saat metadata benar-benar berubah ✅ *ada di spec*
Diharapkan: ditandai berubah.

### TC-Q11 — Daftar "Semua" diurut prioritas aksi, bukan tanggal ✅ *ada di spec*

### TC-Q12 — `page`/`limit` non-numerik ✅ *ada di spec*
Diharapkan: dipaksa jadi angka — SQL tidak pernah menerima `LIMIT NaN`.

### TC-Q13 — Posting gagal menandai sumber `failed` ✅ *ada di spec*
Kecuali dipanggil dengan opsi tanpa-tandai (dipakai tutup hari).

### TC-Q14 — Sumber `pending` tidak diturunkan jadi `failed` ✅ *ada di spec*
Diharapkan: "Menunggu data" tidak berubah jadi "Perlu dicek" saat ditolak penjagaan prasyarat.

### TC-Q15 — Posting sumber `failed` ditolak sampai dipulihkan ✅ *ada di spec*

### TC-Q16 — Pulihkan sumber `posted` ditolak ✅ *ada di spec*

### TC-Q17 — Batal posting: jurnal dibalik, sumber kembali `ready` ✅ *ada di spec*

### TC-Q18 — Race jinak pada posting massal ✅ *ada di spec*
Diharapkan: dilaporkan **ok** (sudah final), bukan error.

---

## 4. Watermark Sinkronisasi

Lima kasus, semuanya ✅ *ada di spec*:

| Kondisi | Hasil |
|---|---|
| Diminta **di atas** watermark | Pakai yang diminta |
| Diminta **di bawah** watermark | Tetap pakai watermark |
| Ada watermark, tanpa jendela diminta | Watermark penuh, tanpa batas atas |
| Tanpa watermark, ada jendela | Pakai jendela itu |
| Tanpa keduanya | **Tidak menangkap apa pun** |

---

## 5. Tutup Hari

### TC-C01 — Posting urut dependensi ✅ *ada di spec*
Diharapkan: induk sebelum anak (penerimaan → penjualan → HPP → pembayaran).

### TC-C02 — Sumber bertanggal SETELAH tanggal tutup tidak diposting ✅ *ada di spec*
Diharapkan: backlog lama **tetap** diposting; yang lebih baru tidak.

### TC-C03 — Sumber gagal dibiarkan `ready`, tidak diulang selamanya ✅ *ada di spec*

### TC-C04 — Bawaan tanggal = hari ini **WIB** ✅ *ada di spec*

### TC-C05 — Ringkasan menggabungkan total hari ini + mengelompokkan Tipe B ✅ *ada di spec*

### TC-C06 — Empat status hari ini ✅ *ada di spec* (`finance-posting.reasons.spec.ts`)

| Aktivitas hari ini | Pekerjaan | Tipe B | Status |
|---|---|---|---|
| 0 | — | — | `kosong` |
| > 0 | > 0 | — | `belum` |
| > 0 | 0 | > 0 | `selesai_catatan` |
| > 0 | 0 | 0 | `lengkap` |

---

## 6. Template Jurnal

### TC-J01 — Retur pembelian: sisi uang ✅ *ada di spec*
Diharapkan: debit Hutang Supplier = nilai retur + PPN; kredit PPN Masukan + Selisih Retur
Pembelian; **seimbang**.

### TC-J02 — Retur pembelian: sisi harga modal ✅ *ada di spec*
Diharapkan: debit Selisih Retur Pembelian, kredit Persediaan, sebesar harga modal dari mesin biaya.

### TC-J03 — Sisa saldo Selisih Retur Pembelian ✅ *ada di spec*
```
Harga nota beli  : Rp 100.000
Harga modal rata²: Rp  90.000
```
Diharapkan: saldo akun Selisih Retur Pembelian **persis Rp 10.000**.

### TC-J04 — Retur 100% mengembalikan Hutang Supplier ke nol persis ✅ *ada di spec*

### TC-J05 — Dua varian penyelesaian retur pembelian ✅ *ada di spec*
| Event | Jurnal |
|---|---|
| `purchase_return_refund` | debit Kas/Bank, kredit Hutang Supplier |
| `purchase_return_supplier_credit` | debit Uang Muka Supplier, kredit Hutang Supplier |

### TC-J06 — Baris pendapatan retur penjualan membawa id produk ✅ *ada di spec*
Diperlukan laporan margin per produk.

### TC-J07 — Jurnal tidak seimbang ditolak **[dari kode]**
Diharapkan: **400** `Jurnal tidak balance. Debit {d}, credit {c}`. Toleransi 0,009.

### TC-J08 — Baris debit **dan** kredit sekaligus ditolak **[dari kode]**

### TC-J09 — Pembayaran dengan alokasi ganda **[dari kode]**
Diharapkan: **satu baris lawan per alokasi**, masing-masing membawa id order-nya — bukan satu baris
gabungan.

### TC-J10 — Pembayaran tanpa alokasi **[dari kode]**
Diharapkan: satu baris lawan sebesar nominal penuh.

---

## 7. Harga Modal / HPP

### TC-H01 — Rata-rata bergerak barang masuk ✅ *ada di spec*
```
Sebelum : 10 unit @ Rp 1.000  (nilai Rp 10.000)
Masuk   : 10 unit @ Rp 2.000  (nilai Rp 20.000)
```
Diharapkan: 20 unit, rata-rata **Rp 1.500**, nilai **Rp 30.000**.

### TC-H02 — Barang keluar tidak mengubah rata-rata ✅ *ada di spec*

### TC-H03 — Idempoten per mutasi ✅ *ada di spec*
Diharapkan: memanggil dua kali menghasilkan **satu** baris biaya.

### TC-H04 — Cadangan harga beli master saat rata-rata nol ✅ *ada di spec*

### TC-H05 — Koreksi stok masuk **menambah** nilai ✅ *ada di spec*
Diharapkan: bukan mengurangi — arah sesuai penerapannya.

### TC-H06 — Barang masuk tanpa harga modal ditolak **[dari kode]**
Diharapkan: **400** `Harga modal barang masuk belum tersedia...`.

### TC-H07 — Rasio kewajaran dilanggar **[dari kode]** ⚠️
```
Harga beli master : Rp 10.000
Harga modal masuk : Rp 5.000.000  (500×)
```
Diharapkan: **400** menyebut rasio dan menyarankan periksa salah digit.

### TC-H08 — Rata-rata negatif ditolak **[dari kode]** ⚠️
Diharapkan: **400** menjelaskan basis biaya sudah rusak dari transaksi sebelumnya, dan melarang
"ditambal dengan adjustment biasa".

### TC-H09 — Harga modal per **varian** **[dari kode]** ⚠️
```
Data : produk KAOS, varian Merah (beli Rp 50rb) & Biru (beli Rp 80rb)
```
Diharapkan **saat ini**: keduanya berbagi **satu** harga modal rata-rata gabungan — granularitas
cost state adalah (cabang, produk), bukan varian. HPP per varian akan meleset (BR-53).

---

## 8. Saldo Awal

### TC-O01 — Harga modal kiriman klien diabaikan ✅ *ada di spec*
```
Request : { data: { inventory_items: [{ id_product: 1, quantity_on_hand: 10, average_cost: 999999 }] } }
Data    : harga beli master produk 1 = Rp 10.000
```
Diharapkan: tersimpan **Rp 10.000**, bukan Rp 999.999.

### TC-O02 — Produk tanpa harga beli ditolak ✅ *ada di spec*
Diharapkan: **400** `Produk "{nama}" belum punya Harga Beli di Data Produk...`.

### TC-O03 — Saldo awal terkunci tidak bisa diedit ✅ *ada di spec*

### TC-O04 — Posting ulang ditolak ✅ *ada di spec*
Diharapkan: **400** `Catatan pembuka untuk saldo awal sudah ada. Gunakan reversal untuk koreksi.`

### TC-O05 — Ekuitas jadi penyeimbang ✅ *ada di spec*
```
Kas Rp 10jt · Bank Rp 20jt · Piutang Rp 5jt · Persediaan Rp 15jt · Hutang Rp 8jt
```
Diharapkan: Modal Awal dikredit **Rp 42jt** (50jt − 8jt), jurnal seimbang.

### TC-O06 — Semua nol → tidak ada jurnal ✅ *ada di spec*
Diharapkan: saldo awal tetap ditandai terkunci, tanpa jurnal.

### TC-O07 — Nomor jurnal dari tanggal cutover ✅ *ada di spec*
Diharapkan: bulan mengikuti cutover, bukan hari mengunci.

### TC-O08 — Mapping wajib belum lengkap ✅ *ada di spec*
Diharapkan: **400** menyebut kunci yang kurang.

### TC-O09 — Lengkapi stok awal pada saldo draft ditolak **[dari kode]**

### TC-O10 — Lengkapi produk yang sudah ada ditolak **[dari kode]**
Diharapkan: **400** `Produk "{nama}" sudah ada di saldo awal`.

### TC-O11 — CSV tanpa header wajib ditolak **[dari kode]**
Diharapkan: **400** `CSV wajib punya header id_branch,id_product,quantity_on_hand`.

---

## 9. Biaya Usaha

### TC-E01 — Nominal ≤ 0 ditolak ✅ *ada di spec*

### TC-E02 — Akun bukan bertipe biaya ditolak ✅ *ada di spec*
Diharapkan: **400** `Akun yang dipilih bukan akun biaya`.

### TC-E03 — Akun bayar bukan kas/rekening terdaftar ditolak ✅ *ada di spec*

### TC-E04 — Batal tanpa alasan ditolak ✅ *ada di spec*

### TC-E05 — Batal biaya dari periode terkunci ditolak ✅ *ada di spec*
Diharapkan: penjagaan memakai periode **jurnal asli**, bukan hari ini.

### TC-E06 — Tanggal jurnal pembalik **[dari kode]** ⚠️
Diharapkan **saat ini**: bertanggal **hari ini** (bukan tanggal biaya asli), dan "hari ini"
dihitung **UTC** — pembatalan pukul 02:00 WIB menghasilkan pembalik bertanggal kemarin (KI-146).

### TC-E07 — Satu biaya menghabiskan dua nomor **[dari kode]**
Diharapkan: satu `EXP/...` dan satu `JRN/...`.

---

## 10. Tutup Bulan

### TC-K01 — Enam pemeriksaan dihitung ✅ *ada di spec*

### TC-K02 — Kunci ditolak bila ada yang gagal ✅ *ada di spec*

### TC-K03 — Kunci paksa tanpa permission ditolak ✅ *ada di spec*
Diharapkan: **403** `Anda tidak memiliki izin untuk menutup periode secara paksa (force)`.

### TC-K04 — Peringatan tidak memblokir ✅ *ada di spec*

### TC-K05 — Kunci pada periode yang sudah terkunci ✅ *ada di spec*
Diharapkan: dikembalikan apa adanya, **tanpa audit tambahan**.

### TC-K06 — Nilai persediaan negatif memblokir ✅ *ada di spec*
Diharapkan: dicek **seluruh perusahaan**, bukan hanya periode itu.

---

## 11. Laporan

### TC-R01 — Filter tanggal memakai batas WIB ✅ *ada di spec*
Diharapkan: `date_to` inklusif akhir hari WIB.

### TC-R02 — Format tanggal salah ditolak ✅ *ada di spec*

### TC-R03 — Neraca saldo menandai seimbang/tidak ✅ *ada di spec*

### TC-R04 — Saldo awal ringkasan kas dari jurnal sebelum rentang ✅ *ada di spec*
Diharapkan: **tidak** ditambah kolom cutover terpisah (mencegah hitung ganda).

### TC-R05 — DPP saat tarif pajak 0 **[dari kode]** ⚠️
```
Data : order kena pajak, tax_rate_snapshot = 0, tax_amount = Rp 110.000,
       subtotal_before_tax = Rp 1.000.000
```
Diharapkan **saat ini**:

| Laporan | DPP |
|---|---|
| Ringkasan PPN | **Rp 0** |
| Rincian PPN per Faktur | **Rp 1.000.000** |

Kedua laporan **tidak rekonsiliasi** — dan keduanya masuk paket berkas pajak yang sama (KI-145).

---

## 12. Layer 2 / Paket Berkas Pajak

### TC-L01 — Urutan pembuangan bersifat stabil **[dari kode]**
Diharapkan: mengunduh dua kali dengan data yang sama menghasilkan **order yang sama** dibuang —
memakai hash id order, bukan acak nyata.

### TC-L02 — Seluruh jurnal milik order dibuang, bukan hanya jurnal omzet **[dari kode]**
Diharapkan: penjualan + HPP + PPN + pembayaran ikut dibuang, sehingga seluruh laporan tetap
**seimbang**.

### TC-L03 — Kedelapan sheet memakai himpunan pengecualian yang sama **[dari kode]**
Diharapkan: dihitung **sekali**, dipakai bersama — angka antar-sheet konsisten.

### TC-L04 — Rentang tanggal terbalik **[dari kode]** ⚠️
```
Request : { data: { date_from: "2026-12-31", date_to: "2026-01-01" } }
```
Diharapkan **saat ini**: **diterima tanpa penolakan**. Tidak ada validasi apa pun (BR-105, KI-144).

### TC-L05 — Rentang bertahun-tahun **[dari kode]** ⚠️
Diharapkan **saat ini**: diterima; delapan sheet dibangun seluruhnya di memori. Risiko nyata pada
VPS 2 GB.

### TC-L06 — Unduh tidak meninggalkan jejak audit **[dari kode]** ⚠️
Diharapkan **saat ini**: **tidak ada** entri audit — tidak tercatat siapa mengunduh versi mana.

---

## 13. Penomoran

### TC-N01 — Tahun/bulan dari tanggal jurnal, bukan "sekarang" ✅ *ada di spec*

### TC-N02 — Kalender WIB **[dari kode]**
```
Tanggal jurnal : 2026-07-31 18:00 UTC (= 1 Agustus 01:00 WIB)
```
Diharapkan: nomor bermuatan bulan **08**.

### TC-N03 — Sequence hilang → transaksi gagal **[dari kode]**
Diharapkan: **400**, bukan nomor darurat (berbeda dari modul Branch).

### TC-N04 — Reset tahunan **[dari kode]** ⚠️
Diharapkan **saat ini**: **tidak terjadi**. `JRN/2027/01848` mengikuti `JRN/2026/01847`.

---

## 14. Celah Test yang Perlu Ditutup

| # | Perilaku tanpa test | Kenapa berisiko |
|---|---|---|
| GAP-01 | Ubah tipe akun / saldo normal setelah berjurnal (TC-A06) | Membalik seluruh laporan historis |
| GAP-02 | Perbedaan rumus DPP dua laporan PPN (TC-R05) | Angka pajak tidak rekonsiliasi |
| GAP-03 | Layer 2 — hanya 2 test untuk seluruh ekspor | Mekanisme paling sensitif, cakupan terendah |
| GAP-04 | Validasi rentang tanggal ekspor (TC-L04/05) | Risiko kehabisan memori |
| GAP-05 | Harga modal per varian (TC-H09) | HPP meleset untuk produk bervarian |
| GAP-06 | Arsip kas/rekening yang sudah dipakai | Tanpa penjagaan sama sekali |
| GAP-07 | Periode pajak tumpang tindih | Tidak diperiksa |
| GAP-08 | Ketiadaan audit untuk posting/pembalikan/unduh | Tidak ada test yang menuntutnya |
| GAP-09 | Worksheet penyesuaian fiskal — hanya 3 test | Formula 5 komponen nyaris tanpa penjagaan |
| GAP-10 | Zona waktu pembatalan biaya (TC-E06) | Satu-satunya jalur non-WIB |

**Tidak ada satu pun test E2E** yang menyentuh Layer 2 — padahal ini fitur dengan konsekuensi
terbesar di modul ini.
