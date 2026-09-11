# Data Model (Legacy) — Modul 17 Finance / Accounting & Pajak

**Kelompok B — cukup dipahami maksudnya, BUKAN untuk ditiru strukturnya.** Menjelaskan *apa yang
disimpan dan mengapa*. Peta foreign key di §2 diambil dari migrasi 022–032 (bukan perkiraan).

---

## 1. Enam Belas Tabel Milik Modul Ini

Dikelompokkan menurut perannya.

### 1.1 Kerangka akuntansi (4 tabel)

| Tabel | Maksud |
|---|---|
| `finance_accounts` | Bagan akun. Punya `system_key` untuk akun bawaan, kolom induk (tak terpakai), dan arsip |
| `finance_account_mappings` | **Jembatan tunggal** peristiwa bisnis → akun. 15 kunci tetap. Satu baris = satu kunci per perusahaan |
| `finance_cash_accounts` | Kas/rekening yang bisa dipilih pengguna; menunjuk satu akun buku besar |
| `finance_document_sequences` | Penghitung nomor dokumen finance (per perusahaan, bukan per cabang) |

**Yang perlu dipahami:** template jurnal **tidak pernah** menyebut id akun — selalu *mapping key*.
Ini yang membuat bagan akun bisa berbeda antar perusahaan tanpa mengubah kode posting. Harganya:
laporan yang membaca "akun saat ini" lewat mapping akan kehilangan riwayat bila mapping diganti —
karena itu ada penjagaan yang melarang melepas akun yang sudah punya jurnal.

### 1.2 Buku besar (2 tabel)

| Tabel | Maksud |
|---|---|
| `finance_journal_entries` | Kepala jurnal: nomor, tanggal, tipe posting, sumber, status, jejak balik |
| `finance_journal_lines` | Baris jurnal: akun, debit, kredit, + **7 kolom dimensi** |

**Tujuh kolom dimensi** pada baris jurnal (`id_product`, `id_product_variant`, `id_business_party`,
`id_order`, `id_payment`, `id_inventory_movement`, `id_branch`) adalah keputusan penting: mereka
membuat satu baris jurnal bisa ditelusuri balik ke dokumen asalnya **tanpa tabel perantara**.

Inilah yang memungkinkan mekanisme Layer 2 (§ [algorithms-legacy.md](algorithms-legacy.md) §7)
mengumpulkan "semua jurnal milik satu order" hanya lewat `id_order`.

### 1.3 Antrian posting (1 tabel)

| Tabel | Maksud |
|---|---|
| `finance_posting_sources` | Satu baris = satu peristiwa bisnis yang **harus** jadi jurnal |

Ini tabel paling khas modul ini. Kunci identitasnya adalah
**(perusahaan, jenis sumber, id sumber, event key)** — satu dokumen bisa memunculkan beberapa baris
antrian dengan event berbeda (mis. retur penjualan menghasilkan `sales_return_completed`,
`sales_return_stock_in`, dan `sales_return_refund`).

Kolom yang membuatnya bekerja:

| Kolom | Peran |
|---|---|
| `posting_status` | 6 keadaan; menentukan apa yang muncul di antrian |
| `pending_reason_code` | **Kode bertipe**, bukan teks bebas — agar antarmuka memisahkan Tipe A/B tanpa mencocokkan string |
| `source_hash` | Sidik jari isi sumber; dipakai mendeteksi dokumen yang berubah setelah disinkron |
| `metadata_json` | Salinan angka yang dibutuhkan template jurnal, agar posting tak perlu membaca ulang tabel asal |
| `id_journal_entry` | Tautan hasil — **tanpa foreign key** (lihat §2.2) |

**Yang perlu dipahami:** `metadata_json` adalah **snapshot sengaja**. Nilai retur, pajak, dan pihak
disalin saat sinkronisasi supaya jurnal yang terbentuk mencerminkan keadaan saat peristiwa terjadi,
bukan saat tombol posting ditekan.

### 1.4 Buku besar harga modal (2 tabel)

| Tabel | Maksud |
|---|---|
| `finance_inventory_cost_states` | Keadaan **berjalan** per (cabang, produk): jumlah, harga modal rata-rata, nilai |
| `finance_inventory_cost_movements` | Riwayat append-only tiap perubahan, dengan nilai **sebelum & sesudah** |

Dua hal penting:

1. **Per (cabang, produk) — bukan per varian.** Kolom varian ada di tabel mutasi tetapi **tidak
   ikut membentuk kunci keadaan**. Jadi seluruh varian satu produk berbagi satu harga modal.
2. **Presisi 12 desimal** untuk jumlah dan harga modal, 2 desimal untuk nilai rupiah. Sama seperti
   faktor konversi produk — sengaja, agar pecahan seperti 1/24 tidak menimbulkan penyimpangan.

Menyimpan nilai sebelum & sesudah di tiap baris mutasi membuat buku besar ini **dapat diaudit
mundur** tanpa menghitung ulang — properti yang layak dipertahankan.

### 1.5 Saldo awal (2 tabel)

| Tabel | Maksud |
|---|---|
| `finance_opening_balances` | Satu baris per perusahaan: tanggal cutover, 4 saldo uang, status draft/posted |
| `finance_opening_inventory_items` | Stok awal per (cabang, produk) dengan harga modal & nilainya |

**Yang perlu dipahami:** kolom `average_cost` di sini **tidak pernah** berasal dari input pengguna.
Server selalu mengisinya dari harga beli master produk. Kolomnya tetap ada karena nilainya adalah
*snapshot* — harga beli master boleh berubah kemudian tanpa mengubah saldo awal yang sudah dikunci.

### 1.6 Pajak (3 tabel)

| Tabel | Maksud |
|---|---|
| `finance_tax_periods` | Periode pajak (terpisah dari periode akuntansi!) dengan status buka/tutup |
| `finance_tax_report_snapshots` | Hasil ringkasan PPN yang **dibekukan** saat periode pajak ditutup |
| `finance_tax_adjustments` | Worksheet rekonsiliasi fiskal: laba komersil → laba fiskal |

**Isi snapshot** (pertanyaan yang sebelumnya terbuka): tiga kolom angka utama (PPN keluaran,
PPN masukan, PPN kurang bayar) **plus** kolom JSON berisi **seluruh hasil ringkasan PPN apa
adanya** — DPP penjualan, DPP pembelian, dan rentang tanggalnya.

**Relasi tax-period → snapshot → adjustment** (pertanyaan yang sebelumnya terbuka):

```
finance_tax_periods
   ├── 1:1 → finance_tax_report_snapshots   (FK, dibuat saat periode ditutup)
   └── 1:N → finance_tax_adjustments        (FK opsional — worksheet boleh berdiri sendiri)
```

Kolom periode pada worksheet penyesuaian **boleh kosong**: worksheet menyimpan kode periode dan
rentang tanggalnya sendiri, jadi ia bisa dibuat tanpa periode pajak formal.

### 1.7 Biaya usaha (1 tabel)

| Tabel | Maksud |
|---|---|
| `business_expenses` | Pengeluaran operasional: nomor, tanggal, akun biaya, akun bayar, nominal, status, tautan jurnal + jurnal pembalik |

Satu-satunya tabel di modul ini yang mewakili **dokumen yang dibuat pengguna**, bukan turunan
peristiwa modul lain.

---

## 2. Peta Foreign Key

Diambil dari migrasi 022, 023, 024, 025, 026, 030, 031, 032.

### 2.1 Tautan yang punya foreign key

| Dari | Ke | Kolom |
|---|---|---|
| Semua 16 tabel | `companies` | `id_company` |
| `finance_accounts` | `finance_accounts` (induk) | `parent_account_id` |
| `finance_account_mappings` | `finance_accounts` | `id_finance_account` |
| `finance_cash_accounts` | `finance_accounts` | `id_finance_account` |
| `finance_periods` | `users` | `closed_by` |
| `finance_posting_sources` | `branches`, `users` | `id_branch`, `posted_by` |
| `finance_journal_entries` | `branches`, `users` ×2, **dirinya sendiri** | `id_branch`, `posted_by`, `reversed_by`, `id_reversal_of` |
| `finance_journal_lines` | `finance_journal_entries`, `finance_accounts`, `branches`, `products`, `business_parties`, `orders`, `payments`, `inventory_movements` | 8 tautan |
| `finance_inventory_cost_states` | `branches`, `products` | |
| `finance_inventory_cost_movements` | `branches`, `products`, `inventory_movements` | |
| `finance_opening_balances` | `users` | `posted_by` |
| `finance_opening_inventory_items` | `finance_opening_balances`, `branches`, `products` | |
| `finance_tax_periods` | `users` | `closed_by` |
| `finance_tax_report_snapshots` | `finance_tax_periods`, `users` | |
| `finance_tax_adjustments` | `finance_tax_periods` *(opsional)* | |
| `business_expenses` | `branches`, `finance_accounts` ×2, `finance_journal_entries` ×2 | |

**Baris jurnal adalah simpul terpadat di seluruh basis data** — 8 foreign key, menyentuh 4 modul
lain. Ini konsekuensi langsung dari keputusan "dimensi di baris jurnal" (§1.2).

### 2.2 Tautan tanpa foreign key (disengaja atau kelalaian)

| Dari | Ke | Catatan |
|---|---|---|
| `finance_posting_sources.id_journal_entry` | `finance_journal_entries` | **Tanpa FK.** Antrian bisa menunjuk jurnal yang sudah hilang |
| `finance_posting_sources.id_source` | tabel asal (bervariasi) | Polimorfik — memang tidak bisa di-FK-kan |
| `finance_journal_entries.id_source` | tabel asal (bervariasi) | Idem |
| `finance_journal_lines.id_product_variant` | `product_variants` | **Tanpa FK** meski kolomnya ada |

Empat tautan polimorfik itu adalah harga dari desain "satu antrian untuk 11 jenis peristiwa".
Sistem baru perlu memutuskan: pertahankan polimorfisme, atau pecah per jenis.

---

## 3. Dua Belas Tabel Modul Lain yang Dibaca

Finance **tidak pernah menulis** ke tabel ini — hanya membaca lewat SQL mentah.

| Tabel | Milik modul | Dibaca untuk |
|---|---|---|
| `orders` | 08/09 Order | Pendapatan, PPN, tarif pajak, nomor faktur |
| `order_items` | 08/09 Order | Pendapatan per produk, basis biaya penerimaan |
| `payments` | 14 Payment | Kas masuk/keluar |
| `payment_allocations` | 14 Payment | Pemecahan pelunasan per order |
| `goods_receipts`, `goods_receipt_items` | 10 Goods Receipt | Nilai persediaan masuk |
| `inventory_movements` | 16 Stock | Basis HPP, koreksi stok |
| `products` | 05 Product | Harga beli sebagai basis biaya, faktor konversi |
| `sales_returns`, `sales_return_items`, `sales_return_settlements` | 12 Sales Return | Koreksi pendapatan & kas |
| `purchase_returns`, `purchase_return_settlements` | 13 Purchase Return | Koreksi pembelian & kas |
| `branches` | 04 Branch | Dimensi cabang, validasi |

**Ini titik rapuh terbesar modul.** Tidak ada satu pun kontrak formal — hanya SQL yang mengasumsikan
nama kolom tertentu. Sistem baru sebaiknya memberi Finance **antarmuka baca yang eksplisit** dari
tiap modul, bukan akses langsung ke tabelnya.

---

## 4. Presisi Kolom — Perlu Diverifikasi di Production

| Jenis nilai | Presisi di entity | Catatan |
|---|---|---|
| Uang (debit/kredit/nilai/nominal) | `DECIMAL(18,2)` | Konsisten di seluruh modul finance |
| Jumlah & harga modal | `DECIMAL(24,12)` | Sama dengan faktor konversi produk |
| Waktu | `DATETIME(6)` | Migrasi lama sebagian menulis `(3)` |
| `current_value` sequence | `BIGINT` | |

Ketidakcocokan `DATETIME(3)` vs `(6)` adalah pola yang sama di seluruh sistem — satu perintah
`SHOW COLUMNS` di production menutup pertanyaan ini untuk semua modul sekaligus. Lihat
[open-questions.md](../open-questions.md) §B.2.

---

## 5. Dua Konsep Periode yang Berbeda

Mudah tertukar dan **wajib tetap terpisah** di sistem baru:

| | Periode akuntansi | Periode pajak |
|---|---|---|
| Tabel | `finance_periods` | `finance_tax_periods` |
| Menjaga | Penulisan jurnal (posting, biaya, pembalikan) | **Tidak menjaga apa pun** |
| Efek menutup | Semua tulis pada rentang itu ditolak | Hanya membekukan snapshot PPN |
| Buka kembali | Wajib alasan | Wajib alasan |
| Tumpang tindih | **Ditolak** saat dibuat | **Tidak diperiksa** |

Menutup periode pajak **tidak** mengunci apa pun — ia hanya mengambil foto angka PPN. Perbedaan
ini tidak terlihat di antarmuka dan mudah disalahpahami pengguna.

---

## 6. Ringkasan Niat Model Ini

Bila seluruh detail dilupakan, empat niat inilah yang harus bertahan:

1. **Peristiwa bisnis tidak langsung jadi jurnal.** Ada antrian di antaranya, yang bisa dilihat,
   dipratinjau, ditunda, diabaikan, dan dibatalkan. Inilah yang membuat keuangan tetap
   *reviewable* dan *reversible*.
2. **Jurnal tidak pernah menyebut akun secara langsung** — selalu lewat mapping key, sehingga bagan
   akun bisa berbeda tanpa menyentuh kode.
3. **Harga modal adalah buku besar tersendiri** dengan riwayat sebelum/sesudah, bukan angka
   turunan yang dihitung ulang saat dibutuhkan.
4. **Tidak ada penghapusan.** Koreksi selalu berupa jurnal pembalik, sehingga jejaknya utuh.
