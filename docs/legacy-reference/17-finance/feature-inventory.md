# Feature Inventory — Modul 17 Finance / Accounting & Pajak

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode: 6 controller, 8 service, 16 entity, 8 migrasi, 16 halaman web. Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md) ·
[data-model-legacy.md](data-model-legacy.md) · [algorithms-legacy.md](algorithms-legacy.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | **64** (21 inti + 15 posting + 15 pelaporan + 6 biaya + 4 penyesuaian pajak + 3 tutup/ekspor) |
| Controller | 6 |
| Service | 8 (posting 2.987 baris, pelaporan 1.220, inti 1.164, biaya-persediaan 770, biaya-usaha 510, ekspor 491, tutup 242, penyesuaian-pajak 235) |
| Halaman | **16 rute**, 4 seksi menu: Harian · Cek Bisnis · Pajak & Akhir Bulan · Detail |
| Permission | **11** — lihat §6 |
| Tabel yang dimiliki | **16** |
| Tabel modul lain yang dibaca | **12** (order, stok, payment, retur, produk, cabang) |
| Laporan | **12** — lihat [reports-list.md](reports-list.md) |
| Penomoran dokumen | 3 sequence (1 tidak terpakai) — lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | **17 `actionKey`** — lihat §7 |
| Jenis jurnal otomatis | **15 event** dengan 12 template baris jurnal |

**Karakter modul ini.** Finance **tidak dipanggil modul lain** — ia **membaca** hasil kerja modul
lain lalu menerjemahkannya jadi jurnal. Arah ketergantungannya satu arah dan tersembunyi: dari
sisi impor kode, Finance hanya butuh `audit-log`; dari sisi data, ia bergantung pada 12 tabel milik
6 modul. Lihat [module-dependency-map.md](../module-dependency-map.md) §3.1.

**Konsekuensi untuk rebuild:** perubahan bentuk tabel di modul Order, Stok, Payment, atau Retur
akan memecahkan Finance **secara diam-diam** — tidak terdeteksi kompilasi, hanya terlihat dari
angka laporan yang salah.

---

## 2. Peta Sub-Area

| Kode | Sub-area | Endpoint | Service | Halaman utama |
|---|---|---|---|---|
| 17a | Bagan Akun & Mapping | 10 | `finance.service` | Pengaturan Keuangan |
| 17b | Antrian Posting & Jurnal | 15 | `finance-posting.service` | Tutup Buku, Catatan Keuangan |
| 17c | Harga Modal / HPP | — *(internal)* | `finance-inventory-cost.service` | *(tanpa halaman)* |
| 17d | Laporan | 13 | `finance-reporting.service` | 8 halaman laporan |
| 17e | Pajak & SPT | 8 | `finance-reporting` + `finance-tax-adjustment` + `finance-export` | Pajak, Detail Pajak, Penyesuaian Pajak |
| 17f | Tutup Periode | 5 | `finance-close.service` | Kunci Bulan |
| 17g | Biaya Usaha | 6 | `business-expense.service` | Biaya Usaha |
| 17h | Saldo Awal & Ekuitas | 7 | `finance.service` | Saldo Awal |

---

## 3. Daftar Fitur & Sub-Fitur

### F-01 — Bagan Akun (17a)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Daftar akun | Urut urutan-tampil lalu kode akun; saring pencarian/tipe/status |
| F-01.2 | Tambah akun | Kode, nama, tipe (harta/hutang/modal/pendapatan/biaya), saldo normal (debit/kredit), induk, urutan |
| F-01.3 | Ubah akun | Semua field bisa diubah, **termasuk tipe dan saldo normal** — tanpa penjagaan meski akun sudah punya jurnal |
| F-01.4 | Arsip akun | Ditolak bila akun dipakai mapping, dipakai kas/rekening, **atau sudah pernah muncul di jurnal** |
| F-01.5 | Akun bawaan sistem | Punya `system_key`; dibuat oleh seed, tidak pernah diberi `system_key` oleh alur tambah manual |
| F-01.6 | Hierarki akun | Kolom induk ada; **tidak ada penjagaan putaran** dan tidak dipakai laporan mana pun |

### F-02 — Mapping Akun (17a)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Lihat 15 mapping | Kunci tetap, lihat §4 |
| F-02.2 | Ubah mapping | Massal; kunci di luar daftar ditolak |
| F-02.3 | Kunci mapping yang sudah berjurnal | Akun lama yang **sudah pernah dipakai jurnal tidak boleh dilepas** dari mapping-nya |

Ini adalah **jembatan tunggal** antara peristiwa bisnis dan bagan akun: seluruh template jurnal
menyebut *mapping key*, tidak pernah id akun.

### F-03 — Kas & Rekening (17a)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Daftar kas/rekening | Urut jenis lalu kode |
| F-03.2 | Tambah/ubah | Kode, nama, jenis (`cash`/`bank`), akun buku besar, penanda utama |
| F-03.3 | Arsip | **Tanpa penjagaan apa pun** — berbeda dari arsip akun |
| F-03.4 | Penanda "utama" | Kolomnya ada, **tidak ada penjagaan "hanya satu"** dan tidak dibaca alur posting |

### F-04 — Antrian Posting (17b) — jantung modul

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Sinkronisasi sumber | Memindai 11 jenis peristiwa bisnis → membuat/memperbarui baris antrian |
| F-04.2 | Watermark otomatis | Rentang pindai dibatasi sendiri agar tidak memindai ulang seluruh sejarah |
| F-04.3 | Enam status | `pending` · `ready` · `posted` · `failed` · `ignored` · `reversed` |
| F-04.4 | Alasan tertahan bertipe | 7 kode alasan, dibagi 2 kategori — lihat §5 |
| F-04.5 | Daftar antrian | Saring status/cabang/tanggal; berhalaman; disertai ringkasan & jejak sumber |
| F-04.6 | Detail & pratinjau | Pratinjau menampilkan baris jurnal **tanpa menyimpan apa pun** |
| F-04.7 | Posting satu | Membuat jurnal + menerapkan harga modal |
| F-04.8 | Posting massal | Beberapa id sekaligus |
| F-04.9 | Tutup hari | Memposting seluruh gelombang `ready` **dalam urutan dependensi** |
| F-04.10 | Ringkasan tutup hari | Empat status hari ini: `kosong` · `belum` · `selesai_catatan` · `lengkap` |
| F-04.11 | Abaikan / pulihkan | Menandai sumber tidak perlu diposting, dan membatalkannya |
| F-04.12 | Batalkan posting | Membalik jurnal + mengembalikan status sumber |
| F-04.13 | Rekonsiliasi sumber terarsip | Dokumen sumber yang diarsipkan otomatis ditandai di antrian |

### F-05 — Jurnal (17b)

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Daftar jurnal | Saring cabang/tanggal, berhalaman |
| F-05.2 | Detail jurnal | Header + seluruh baris |
| F-05.3 | Balik jurnal | Membuat jurnal lawan bertipe `reversal`; asli jadi `reversed` |
| F-05.4 | Tiga tipe posting | `system` (otomatis) · `manual` (biaya usaha) · `reversal` |
| F-05.5 | Tiga status | `draft` (tidak pernah dipakai) · `posted` · `reversed` |

### F-06 — Harga Modal / HPP (17c)

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | Metode rata-rata bergerak | Harga modal rata-rata per **(cabang, produk)** |
| F-06.2 | Buku besar biaya | Setiap perubahan mencatat nilai sebelum & sesudah |
| F-06.3 | Idempoten per mutasi | Satu mutasi stok hanya boleh menghasilkan satu baris biaya |
| F-06.4 | Cadangan harga modal | Urutan: harga retur → harga beli master → harga modal rata-rata |
| F-06.5 | Penjagaan kewajaran | Rasio terhadap harga beli master wajib **0,1×–10×** |
| F-06.6 | Penjagaan negatif | Harga modal rata-rata **tidak boleh negatif** — barang masuk ditolak |
| F-06.7 | Tanpa harga modal → tertahan | Sumber posting jadi `pending` beralasan `missing_cost_basis` |

**Tidak punya halaman sendiri.** Seluruhnya berjalan di balik posting dan hanya terlihat lewat
laporan Nilai Persediaan dan status "Menunggu data".

### F-07 — Saldo Awal (17h)

| # | Sub-fitur | Detail |
|---|---|---|
| F-07.1 | Isi saldo awal | Tanggal cutover, kas, bank, piutang, hutang, catatan |
| F-07.2 | Daftar stok awal per produk | Per cabang + produk + jumlah |
| F-07.3 | **Harga modal tidak bisa diketik** | Selalu diambil server dari harga beli master produk — lihat §8 |
| F-07.4 | Impor CSV | Header wajib `id_branch,id_product,quantity_on_hand` |
| F-07.5 | Validasi kesiapan | Cutover terisi, tiap produk punya harga beli, 6 mapping wajib lengkap |
| F-07.6 | Kunci saldo awal | Membuat jurnal pembuka + seluruh cost state; ekuitas jadi penyeimbang |
| F-07.7 | Daftar produk tanpa harga modal | Produk yang membuat posting tertahan |
| F-07.8 | Lengkapi stok awal | Menambah produk **setelah** saldo awal dikunci — aditif, tidak mereset |

### F-08 — Biaya Usaha (17g)

| # | Sub-fitur | Detail |
|---|---|---|
| F-08.1 | Daftar biaya | Saring pencarian/tanggal/cabang/status/akun; berhalaman |
| F-08.2 | Pilihan akun biaya & akun pembayaran | Dua endpoint terpisah |
| F-08.3 | Catat biaya | Langsung memposting jurnal (debit biaya, kredit kas/bank) — **tanpa antrian** |
| F-08.4 | Batalkan biaya | Wajib alasan; membuat jurnal pembalik |
| F-08.5 | Penjagaan jenis akun | Akun biaya wajib bertipe biaya; akun bayar wajib bertipe harta **dan** terdaftar sebagai kas/rekening |

**Satu-satunya jalur jurnal manual** di seluruh sistem.

### F-09 — Tutup Periode (17f)

| # | Sub-fitur | Detail |
|---|---|---|
| F-09.1 | Buat periode | Kode + rentang tanggal; **tumpang tindih ditolak** |
| F-09.2 | Cek kesiapan | 6 pemeriksaan — lihat [reports-list.md](reports-list.md) §7 |
| F-09.3 | Kunci aman | Ditolak bila ada pemeriksaan gagal |
| F-09.4 | Kunci paksa | Butuh permission terpisah `finance.close.force` |
| F-09.5 | Buka kembali | **Wajib alasan** |
| F-09.6 | Kunci periode memblokir tulis | Posting, biaya, dan pembatalan pada periode terkunci ditolak |

### F-10 — Pajak (17e)

| # | Sub-fitur | Detail |
|---|---|---|
| F-10.1 | Ringkasan PPN | DPP & PPN keluaran/masukan, PPN kurang bayar |
| F-10.2 | Rincian PPN per faktur | Penjualan atau pembelian; menandai faktur kosong |
| F-10.3 | Periode pajak | Daftar, tutup (+snapshot), buka kembali (wajib alasan) |
| F-10.4 | Snapshot periode pajak | Menyimpan **seluruh hasil ringkasan PPN** sebagai JSON + 3 angka utama |
| F-10.5 | Worksheet penyesuaian fiskal | Laba komersil → laba fiskal |
| F-10.6 | Paket berkas pajak (Excel) | 8 sheet — lihat F-11 |

### F-11 — Paket Berkas Pajak — **dua versi**

| # | Sub-fitur | Detail |
|---|---|---|
| F-11.1 | 8 sheet Excel | Sampul · Untung Rugi · Neraca · Cek Saldo Akun · Buku Besar · PPN Keluaran · PPN Masukan · Penyesuaian Pajak |
| F-11.2 | **Layer 1 — "Data riil (apa adanya)"** | Seluruh data periode |
| F-11.3 | **Layer 2 — "Versi dibatasi Rp4,8 M"** | Order penjualan dibuang dari laporan sampai omzet kumulatif ≤ Rp 4.800.000.000 |

**Layer 2 wajib dibaca sebelum rebuild.** Mekanismenya membuang **seluruh jurnal milik order yang
dipilih** (penjualan + HPP + PPN + pembayaran), memakai urutan acak-stabil, sehingga seluruh
laporan tetap seimbang tetapi menampilkan omzet di bawah plafon Rp 4,8 M. Kedua tombol muncul
berdampingan di halaman Kunci Bulan dengan deskripsi *"untuk diberikan ke konsultan pajak"*.

Detail teknisnya di [algorithms-legacy.md](algorithms-legacy.md) §7. **Ini keputusan bisnis, bukan
temuan teknis** — lihat [open-questions.md](../open-questions.md) OQ-A42.

---

## 4. Lima Belas Mapping Key

Seluruh template jurnal menyebut kunci ini, bukan id akun.

| Kunci | Peran | Dipakai event |
|---|---|---|
| `cash_cash` | Kas tunai | pembayaran tunai, refund, biaya, saldo awal |
| `cash_bank` | Rekening bank | pembayaran non-tunai, refund, biaya, saldo awal |
| `accounts_receivable` | Piutang customer | penjualan, pembayaran, retur jual, saldo awal |
| `accounts_payable` | Hutang supplier | penerimaan barang, pembayaran, retur beli, saldo awal |
| `inventory` | Persediaan | penerimaan, HPP, retur, koreksi stok, saldo awal |
| `input_tax` | PPN masukan | penerimaan barang, retur beli |
| `output_tax` | PPN keluaran | penjualan, retur jual |
| `sales_revenue` | Pendapatan penjualan | penjualan, retur jual |
| `cogs` | HPP | pengeluaran stok, stok masuk retur |
| `customer_advance` | Uang muka customer | penyelesaian retur jual |
| `supplier_advance` | Uang muka supplier | penyelesaian retur beli |
| `stock_difference` | Selisih stok | koreksi stok |
| `purchase_return_difference` | Selisih retur pembelian | retur beli (sisi uang & sisi harga modal) |
| `rounding_difference` | Selisih pembulatan | **tidak dipakai satu pun template** |
| `opening_equity` | Modal awal | saldo awal + lengkapi stok awal |

**Catatan:** `rounding_difference` terdaftar sebagai kunci sah tetapi tidak muncul di template mana
pun — sisa rancangan.

---

## 5. Sebelas Event Posting & Tujuh Alasan Tertahan

### 5.1 Event yang dipindai (urutan dependensi)

| Urutan | Event | Sumber | Arti bisnis |
|---|---|---|---|
| 10 | `purchase_received` | Penerimaan barang | Persediaan + PPN masukan vs hutang |
| 20 | `sales_completed` | Order penjualan selesai | Piutang vs pendapatan + PPN keluaran |
| 25 | `sales_return_completed` | Retur penjualan | Pembatalan pendapatan + barang pengganti |
| 26 | `purchase_return_completed` | Retur pembelian (sisi uang) | Pengurangan hutang |
| 30 | `stock_issue_cogs` | Mutasi stok keluar | HPP vs persediaan |
| 35 | `sales_return_stock_in` | Stok masuk dari retur | Persediaan vs koreksi HPP |
| 36 | `purchase_return_stock_out` | Stok keluar retur beli | Selisih retur vs persediaan |
| 40 | `customer_payment` / `supplier_payment` | Pembayaran | Kas vs piutang/hutang |
| 45 | `sales_return_payment` / `sales_return_refund` / `sales_return_customer_credit` | Penyelesaian retur jual | 3 varian |
| 46 | `purchase_return_refund` / `purchase_return_supplier_credit` | Penyelesaian retur beli | 2 varian |
| 50 | `stock_adjustment` | Koreksi stok | Selisih stok |

### 5.2 Alasan tertahan — dua kategori yang sengaja dibedakan

**Tipe A — `ordering`** (selesai sendiri, **tidak ditampilkan** ke pemilik):

`awaiting_parent_sale` · `awaiting_return_posting` · `awaiting_related_sale` ·
`awaiting_receipt_posting`

**Tipe B — `needs_input`** (butuh data dari pemilik, **ditampilkan sebagai tugas** dan dibawa ke
hari berikutnya):

`missing_cost_basis` · `missing_payment_allocation` · `missing_source_order`

Pembedaan ini disimpan sebagai kolom tersendiri, bukan diurai dari teks pesan galat — supaya
antarmuka bisa memisahkan "plumbing" dari "pekerjaan pemilik". **Ini keputusan desain yang layak
dipertahankan.**

---

## 6. Sebelas Permission

| Permission | Menjaga |
|---|---|
| `finance.view` | Bagan akun, mapping, kas, periode, saldo awal, cek kesiapan |
| `finance.manage` | Ubah akun/mapping/kas, buat & kunci & buka periode, kunci saldo awal, tutup & buka periode pajak |
| `finance.close.force` | **Hanya** kunci paksa periode |
| `finance.posting.view` | Lihat antrian & ringkasan tutup hari |
| `finance.posting.post` | Sinkron, posting, abaikan, pulihkan, batal posting, tutup hari |
| `finance.journal.view` | Daftar & detail jurnal |
| `finance.journal.reverse` | Balik jurnal |
| `finance.report.view` | 10 laporan + **paket berkas pajak** |
| `finance.tax_report.view` | Ringkasan PPN, rincian PPN, daftar periode pajak |
| `finance.expense.view` / `.create` / `.cancel` | Biaya usaha |
| `finance.tax_adjustment.view` / `.manage` | Worksheet penyesuaian |

**Keganjilan yang perlu diputuskan:**

1. **Paket berkas pajak (termasuk Layer 2) dijaga `finance.report.view`** — permission yang sama
   dengan melihat laporan biasa, bukan permission pajak.
2. **Menutup periode pajak dijaga `finance.manage`**, sementara melihatnya `finance.tax_report.view`.
3. Peran `admin` bawaan punya seluruh permission *view* tetapi **tidak** punya `finance.manage`,
   `finance.close.force`, `finance.posting.post`, maupun `finance.journal.reverse`.

---

## 7. Tujuh Belas Audit Action Key

| Kelompok | `actionKey` |
|---|---|
| Akun | `finance.account.create` · `finance.account.update` · `finance.account.archive` |
| Mapping | `finance.account_mapping.update` |
| Kas | `finance.cash_account.create` · `finance.cash_account.update` · `finance.cash_account.archive` |
| Periode | `finance.period.create` · `finance.period.reopen` · `finance.period.safe_close` |
| Saldo awal | `finance.opening.update` · `finance.opening.post` · `finance.opening.supplement` |
| Pajak | `finance.tax_period.close` · `finance.tax_period.reopen` |
| Biaya usaha | `business_expense.create` · `business_expense.cancel` |

**Yang TIDAK teraudit** — dan ini kesenjangan besar:

| Aksi | Konsekuensi |
|---|---|
| Posting sumber (satu/massal/tutup hari) | Tidak ada jejak siapa memposting apa — hanya kolom `posted_by` di barisnya |
| Balik jurnal | Tidak ada jejak audit |
| Batal posting | Tidak ada jejak audit |
| Abaikan / pulihkan sumber | Tidak ada jejak audit |
| Sinkronisasi sumber | Tidak ada jejak audit |
| **Unduh paket berkas pajak (Layer 1 maupun Layer 2)** | **Tidak ada jejak sama sekali** |
| Simpan / arsip penyesuaian pajak | Tidak ada jejak audit |

Modul yang paling sensitif secara finansial justru **paling tidak lengkap jejak auditnya**.
Bandingkan modul 03 Company yang mencatat seluruh field sebelum & sesudah.

---

## 8. Pelajaran dari Insiden Nyata (tertulis di kode)

Kode memuat catatan insiden produksi yang membentuk beberapa penjagaan. Ini konteks penting untuk
rebuild:

> **Agustus 2026** — kolom harga modal manual di Saldo Awal dientri **10–1000× lipat salah** untuk
> **241 produk**. Tidak ada validasi yang menangkapnya selain "harus > 0". HPP tercemar berminggu-
> minggu, sekitar **Rp 3,2 miliar** jurnal terlanjur terposting.

Penjagaan yang lahir dari insiden itu, semuanya **wajib dipertahankan**:

| Penjagaan | Isi |
|---|---|
| Harga modal saldo awal tidak bisa diketik | Server **mengabaikan** nilai kiriman klien; selalu dari harga beli master |
| Rasio kewajaran | Harga modal wajib berada di rentang **0,1×–10×** harga beli master |
| Larangan harga modal negatif | Barang masuk yang menghasilkan rata-rata negatif ditolak dengan penjelasan |
| Pemeriksaan nilai persediaan negatif | Masuk checklist tutup bulan, dicek **seluruh perusahaan**, bukan per periode |
| Jalur "Lengkapi Stok Awal" | Menambah produk terlewat secara aditif — pengganti "buka & posting ulang" yang merusak |

---

## 9. Yang TIDAK Ada di Modul Ini

| Hal | Keterangan |
|---|---|
| Jurnal manual bebas | Hanya lewat Biaya Usaha (2 baris tetap) |
| Ubah / hapus jurnal | Hanya balik jurnal |
| Anggaran (budget) | Tidak ada |
| Mata uang asing | Tidak ada |
| Aset tetap & penyusutan | Tidak ada |
| Rekonsiliasi bank | Tidak ada |
| Arus kas metode tidak langsung | Tidak ada — hanya ringkasan kas |
| Tutup tahun / laba ditahan otomatis | **Tidak ada** — tidak ada penutupan akun nominal |
| PPh 21/23/final | Tidak ada — hanya PPN dan worksheet fiskal |
| Validasi rentang tanggal ekspor | **Tidak ada** — lihat KI-144 |
| Status jurnal `draft` | Terdefinisi, tidak pernah dipakai |
| Mapping `rounding_difference` | Terdaftar, tidak dipakai |
| Sequence `opening_balance` | Ter-seed, tidak pernah dibaca |
| Penjagaan hierarki akun | Kolom induk ada tanpa pemeriksaan putaran, tanpa pembaca |
