# Analisis 002 — Rekening tujuan per order/tagihan & retur sebagian

**Tanggal:** 2026-10-26 · **Status:** usulan, menunggu keputusan pemilik/akuntan (§7) ·
**Diagram:** [flow.mmd](flow.mmd), [sequence.mmd](sequence.mmd), [erd.mmd](erd.mmd)

Belum ada ADR. Setelah §7 dijawab, terbitkan ADR-0026 (rekening tujuan) dan
`docs/plan/rekening-tujuan/PLAN.md`. Analisis ini tidak ikut disunting lagi (aturan `analysis/README.md`).

## 1. Kebutuhan pengguna

1. **Retur sebagian.** "Order 5, yang diretur 1" dilaporkan belum bisa ditangani sistem.
2. **Rekening tujuan.** Perusahaan punya beberapa rekening. Kasir atau pembuat order ingin menentukan
   order ini dibayar ke rekening mana, dan tagihannya menunjukkan rekening tersebut.

## 2. Kondisi saat ini (kode `main` 2026-10-26, image live `4deff6b` 2026-10-22)

### 2.1 Retur sebagian — kemampuannya sudah ada

| Titik | Fakta |
|---|---|
| Backend retur jual | `computeReturn` (`salesreturn/application/service.go:347`) menerima qty per baris. Batasnya `qty dasar ≤ terkirim − (retur draf + terkonfirmasi)` per produk+varian. Satu order boleh diretur berkali-kali. Nilai diprorata dari ekonomi baris asli |
| Backend retur beli | Pola sama dengan batas `diterima` (`purchasereturn/application/service.go:333`) |
| Bukti tes | `TestReturnConditions` (order 10 → retur 3, lalu 2) dan `TestReturnAutoOffset` (retur 4, lalu 6) di `internal/integration/returns_advanced_test.go` |
| Form web | `ReturnLinesTable` (`shared/components/ui/return-lines-table.tsx`): input qty per baris, `max = sisa`. **Semua baris terisi qty sisa penuh sejak awal** (`SalesReturnCreatePage.tsx:174`, `PurchaseReturnCreatePage.tsx:129`) |
| Sistem lama | Pengguna **mencentang** barang yang dikembalikan, lalu mengisi jumlahnya (`docs/legacy-reference/12-sales-return/ui-ux-spec.md:42`). Default-nya kosong |
| **Versi live** | `4deff6b` **belum** memuat `858f84d` "retur jual & beli bisa memilih satuan produk" (23 Okt, temuan pengguna). Di versi live, baris order ber-DUS menampilkan sisa dalam satuan dasar ("Sisa 72 dari 2 dipesan · DUS"), mengisi qty itu, lalu ditolak "Melebihi yang bisa diretur". Satuannya juga terkunci di DUS, sehingga retur 1 PCS dari baris DUS tidak bisa |
| Konfirmasi SJ | Tidak ada input "qty diterima" (`ConfirmInput`, `delivery/application/service.go:267`). SJ draf juga tidak bisa diubah: tidak ada `PUT /deliveries/{id}` |

**Kesimpulan 2.1.** Logika retur sebagian sudah benar dan teruji. Keluhan pengguna paling mungkin
berasal dari salah satu berikut. Perlu dikonfirmasi dengan nomor order nyata (§7 Q1):

- **(a)** Bug satuan di versi live. Sudah diperbaiki di `main` tetapi belum dirilis.
- **(b)** Form mengisi semua baris penuh, sehingga terlihat seperti "retur semua". Sistem lama memakai pola
  pilih-dulu.
- **(c)** Pelanggan menolak sebagian barang saat diantar. Kasus ini bukan retur (lihat §3.2).

### 2.2 Rekening tujuan — belum ada

| Titik | Fakta |
|---|---|
| Data rekening | Hanya array teks `bankAccounts {bank, holder, number}` di JSON `document_profile` milik company. Tidak punya ID dan tidak tertaut ke akun buku besar |
| Order jual | `sales_orders` tidak punya kolom rekening. Kolom terkait pembayaran hanya `payment_terms` dan `due_date`. **Tidak ada tabel faktur terpisah**: nota/tagihan mencetak SO |
| Pembayaran | `payments` hanya menyimpan `method` (cash/transfer/check/offset/advance) dan `reference_number`. Tidak ada satu pun jalur yang membawa rekening: modal bayar order, form pembayaran, koreksi, checkout POS, pelunasan COD di konfirmasi SJ, SO dari SJ Langsung, bayar-sekarang GR, dan selisih tukar retur |
| Jurnal | `classifyPayment` (`finance/application/payment_kinds.go:40`): transfer/cek → kunci `bank` → satu akun (1010). `finance_account_mappings.mapping_key` adalah PK. `rawLeg` sudah punya jalur akun eksplisit (`builders.go:25-36`, `finish` `:66-98`) |
| Akun | `finance_accounts` punya `is_cash`, tanpa hierarki. Biaya (expenses) sudah memilih `payAccountId` dari akun `is_cash`, jadi preseden memilih akun kas/bank sudah ada |
| Laporan | Laporan kas (`CashFlow`/`CashSummary`) dan tutup harian sudah per akun `is_cash`. **Dasbor "Kas & Bank"** hanya menjumlahkan kunci `cash` + `bank` (`dashboard/application/service.go:106-127`), sehingga akun bank baru tidak akan terhitung |
| Cetak | Struk POS mencetak **semua** rekening. Nota A4/kontinu tidak mencetak rekening sama sekali, padahal `BankAccountsBlock` sudah ada tetapi tidak dipakai. Bukti bayar hanya mencetak metode dan referensi |
| Sistem lama | Punya `finance_cash_accounts` (kas/rekening → satu akun GL), tetapi pemilihan kas saat pembayaran **tetap ditentukan oleh metode** (legacy BR-14). Revamp memutuskan memakai flag `is_cash` (`DB_SCHEMA.md` §6). Jadi ini **fitur baru**, bukan paritas yang hilang |
| ETL | Pembayaran legacy tidak membawa id kas. Verify harian membandingkan kunci `cash`/`bank` (`etl/verify/daily.go:89-96`) |

## 3. Riset praktik ERP lain

### 3.1 Rekening tujuan

| Sumber | Temuan | Pelajaran |
|---|---|---|
| Odoo — [Bank](https://www.odoo.com/documentation/17.0/applications/finance/accounting/bank.html), [`account_move.py`](https://github.com/odoo/odoo/blob/17.0/addons/account/models/account_move.py) | Satu jurnal + satu akun GL per rekening. Faktur punya `partner_bank_id` **"Recipient Bank"**: default rekening pertama perusahaan, bisa diubah, dan perubahannya dilacak | Rekening tujuan menempel di dokumen tagihan, bisa diubah, dan diaudit |
| Odoo — [report_invoice.xml](https://github.com/odoo/odoo/blob/17.0/addons/account/views/report_invoice.xml), [QR EPC](https://www.odoo.com/documentation/17.0/applications/finance/accounting/customer_invoices/epc_qr_code.html) | Cetakan faktur menulis "…on this account: [rekening]". QR pembayaran memakai rekening itu | Cetakan menampilkan rekening yang dipilih, bukan seluruh daftar |
| Odoo — [`account_payment_register.py`](https://github.com/odoo/odoo/blob/17.0/addons/account/wizard/account_payment_register.py) | Saat mencatat bayar, jurnal bank dipilih otomatis sesuai rekening di faktur dan tetap bisa diganti | Pembayaran di-*prefill* dari tagihan. Pembayaran menyimpan rekening tempat uang benar-benar masuk |
| Odoo — [POS payment methods](https://www.odoo.com/documentation/17.0/applications/sales/point_of_sale/payment_methods.html) | Setiap metode bayar POS → satu jurnal kas/bank | POS: metode non-tunai dipetakan ke rekening |
| ERPNext — [Bank Account](https://docs.frappe.io/erpnext/user/manual/en/bank-account), [`bank_account.py`](https://github.com/frappe/erpnext/blob/version-15/erpnext/accounts/doctype/bank_account/bank_account.py) | Doctype Bank Account `is_company_account` → satu ledger. Satu ledger tidak boleh dipakai dua rekening (1:1). Ada `is_default`, dan default per pelanggan `default_bank_account` | Master rekening terpisah, terkait 1:1 ke akun GL. Urutan default: pelanggan → perusahaan |
| ERPNext — [`payment_entry.js`](https://github.com/frappe/erpnext/blob/version-15/erpnext/accounts/doctype/payment_entry/payment_entry.js) | "Company Bank Account" mengisi "Account Paid To" | Rekening menentukan akun jurnal |
| Jurnal (Mekari) — [Terima Pembayaran](https://help-center.jurnal.id/hc/id/articles/11681641022361-Bagaimana-Cara-Terima-Pembayaran-Tagihan-Penjualan) | Kolom **"Setor Ke"** (akun kas/bank) saat menerima pembayaran faktur | Pola lokal: akun tujuan dipilih per penerimaan |
| Accurate — [Penerimaan Penjualan](https://help.accurate.id/product/accurate-online/fitur-aol/penjualan/penerimaan-penjualan/mengenal-fitur-penerimaan-penjualan/) | Kolom **Bank**: akun kas/bank tempat uang masuk | Sama |
| Kledo — [akun penerimaan POS](https://kledo.com/detail-faq/cara-mengatur-akun-untuk-menampung-penerimaan-pembayaran/), [pemetaan non-tunai](https://kledo.com/detail-faq/cara-mengatur-pemetaan-akun-metode-pembayaran-non-tunai/) | Default akun penerimaan POS; metode non-tunai → akun kas/bank | Ada default supaya kasir tidak repot |
| Rekonsiliasi — [Odoo](https://www.odoo.com/documentation/17.0/applications/finance/accounting/bank/reconciliation.html), [ERPNext](https://docs.frappe.io/erpnext/user/manual/en/bank-reconciliation) | Rekonsiliasi dilakukan per jurnal/rekening | Satu akun GL per rekening adalah prasyarat praktis rekonsiliasi dengan rekening koran |

Konsensus dari sumber di atas:

- Rekening adalah master yang tertaut 1:1 ke akun GL kas/bank.
- Dokumen tagihan menyimpan rekening tujuan. Nilainya terisi dari default, bisa diubah, dan perubahannya diaudit.
- Pembayaran di-*prefill* dari tagihan tetapi menyimpan rekening yang benar-benar dipakai.
- Jurnal pembayaran masuk ke akun milik rekening itu.
- Cetakan menampilkan rekening yang dipilih.

Tidak ada sumber yang menaruh rekening di *sales order* sebelum faktur. Di sistem ini **SO adalah tagihan**
(nota mencetak SO), jadi SO tempat yang tepat.

### 3.2 Retur sebagian dan penolakan saat kirim

| Sumber | Temuan | Pelajaran |
|---|---|---|
| Odoo 17 — [`stock_picking_return.py`](https://github.com/odoo/odoo/blob/17.0/addons/stock/wizard/stock_picking_return.py) | Default qty retur = terkirim − sudah diretur. Qty bisa diubah dan baris bisa dihapus | Sama dengan v2 sekarang |
| Odoo 18/19 — [`stock_picking_return.py`](https://github.com/odoo/odoo/blob/18.0/addons/stock/wizard/stock_picking_return.py), [view](https://github.com/odoo/odoo/blob/18.0/addons/stock/wizard/stock_picking_return_views.xml) | **Diubah:** default qty **0**, dengan tombol "Return" dan **"Return All"**. Semua nol ditolak: "Please specify at least one non-zero quantity" | Arah terbaru (dan pola sistem lama) adalah pilih-dulu, dengan "kembalikan semua" sebagai aksi eksplisit |
| ERPNext — [Sales Return](https://docs.frappe.io/erpnext/user/manual/en/sales-return) | Retur dari DN/faktur, qty awal penuh lalu dikurangi | Pola lama |
| Odoo — [Operation type: backorder](https://www.odoo.com/documentation/19.0/applications/inventory_and_mrp/inventory/warehouses_storage/inventory_management/operation_type.html), [invoicing policy](https://www.odoo.com/documentation/17.0/applications/sales/sales/invoicing/invoicing_policy.html) | Penolakan sebagian saat kirim = validasi pengiriman dengan qty lebih kecil. Sisanya jadi *backorder* atau ditutup. Tagihan mengikuti qty terkirim | **Penolakan saat kirim bukan retur** |
| ERPNext — [Delivery Note](https://docs.frappe.io/erpnext/user/manual/en/delivery-note) | Qty DN bisa diubah sebelum submit. "Closed" untuk menutup sisa | Sama |

## 4. Opsi desain rekening tujuan

| Opsi | Inti | Penilaian |
|---|---|---|
| **R1 — Label saja** | SO dan pembayaran menyimpan teks/indeks dari daftar `document_profile`; jurnal tetap ke 1010 | Murah, tetapi saldo per rekening tidak bisa dilihat dan tidak bisa direkonsiliasi. Indeks JSON rapuh, karena urutan daftar berubah saat diedit. **Bertentangan dengan konsensus §3.1** |
| **R2 — Rekening = akun kas/bank + detail rekening** (dipilih) | Tabel `finance_bank_accounts` (identitas rekening), 1:1 ke `finance_accounts` (akun GL `is_cash` sendiri per rekening). SO menyimpan `receipt_account_id`; pembayaran menyimpan `cash_account_id`; jurnal pembayaran memakai akun itu | Mengikuti Odoo/ERPNext. Saldo tetap di akun GL, jadi tidak menduplikasi saldo seperti `finance_cash_accounts` legacy yang dulu dibuang. Laporan kas dan tutup harian sudah per akun `is_cash`, sehingga langsung berfungsi |
| R3 — Kolom bank di `finance_accounts` | Sama dengan R2, tetapi `bank_name/number/holder` dijadikan kolom akun | Lebih sedikit tabel, tetapi bagan akun bercampur data operasional. Perluasan berikutnya (cabang, urutan tampil, tampil di dokumen) membebani tabel akun. R2 lebih bersih dengan biaya satu tabel |

**Rekomendasi: R2.** Alasannya:

- Satu-satunya opsi yang memenuhi kebutuhan pengguna: kasir memilih, uang tercatat di rekening yang benar,
  dan saldo per rekening bisa dicocokkan dengan rekening koran.
- Memakai jalur yang sudah ada: `rawLeg` eksplisit, laporan kas per akun, pola `payAccountId` di biaya, dan
  koreksi pembayaran untuk salah pilih.
- Tidak mengubah perilaku lama. Kolom baru `NULL` berarti jatuh ke pemetaan `bank`, jadi pembayaran
  historis dan hasil ETL tetap identik.

## 5. Rancangan R2

### 5.1 Data (satu migrasi per modul)

- **`000046_finance_bank_accounts`** — tabel `finance_bank_accounts`:
  - `id`; `account_id` UNIQUE FK → `finance_accounts` (modul sama; harus aset `is_cash`);
  - `bank_name`, `account_number`, `account_holder`;
  - `show_on_documents`, `is_default`, `sort_order`, `status` (active/archived);
  - `created/updated_at/by`.
  - Aturan: satu default aktif (CAS di aplikasi); nomor rekening unik per bank.
- **`000047_sales_receipt_account`** — `sales_orders.receipt_account_id BIGINT UNSIGNED NULL`. ID primitif,
  tanpa FK lintas modul.
- **`000048_payment_cash_account`** — `payments.cash_account_id BIGINT UNSIGNED NULL`, plus indeks untuk
  filter/laporan.

Nomor migrasi diambil saat implementasi (saat ini tertinggi `000045`). Lalu `node scripts/db-schema-doc.mjs`
dan baris §8 `DB_SCHEMA.md`. Catatan "Diputuskan" `finance_cash_accounts` di §6 direvisi: tabel baru hanya
memuat identitas rekening, bukan saldo.

### 5.2 Backend

| Modul | Perubahan |
|---|---|
| finance | CRUD rekening (`finance.manage`). Endpoint ringan **`GET /api/v1/finance/bank-accounts/options`** (autentikasi + cabang, tanpa izin, pola `company/print-profile`) untuk kasir/admin penjualan. Kontrak `BankAccountClient` (`Get`, `ValidateReceipt`, `Default`). `buildPayment`: bila `CashAccountID > 0`, leg kas memakai akun itu (harus aset `is_cash` aktif); bila 0, tetap memakai `classifyPayment` lama. Ringkasan antrean posting menyebut rekening. **`CashTotal` dasbor = jumlah semua akun `is_cash`**, bukan kunci `cash`+`bank` |
| sales | `OrderInput.ReceiptAccountID`. Default saat buat = rekening default (Q2). Termasuk perubahan **ringan** (`lightOnly`), jadi boleh pada order terkonfirmasi yang sudah punya SJ/pembayaran. Untuk order `completed` yang masih bersisa, endpoint khusus `PUT /sales-orders/{id}/receipt-account` (`sales.update`) yang diaudit (`sales_orders.receipt_account_change`) |
| payment | `Create`/`RecordForOrder`/`Correct` menerima `cashAccountId`. Aturannya: transfer/cek → bila kosong pakai `receipt_account_id` order (alokasi satu order), lalu default, lalu `NULL`. Tunai/offset/advance → `NULL`. Divalidasi lewat kontrak finance. **Tidak bisa diubah setelah dibuat**; salah pilih diperbaiki lewat koreksi pembayaran (balik jurnal + buat ulang, ADR-0023) |
| delivery / goodsreceipt / salesreturn | Settlement (COD di konfirmasi SJ, bayar-sekarang GR, selisih tukar, POS) meneruskan `cashAccountId` ke payment |

Semua langkah tetap idempoten (clientRef) dan CAS (ADR-0016). Audit ditulis dalam transaksi (ADR-0011).
Lingkup cabang diambil dari sesi.

### 5.3 Frontend

- **Keuangan → Kas & Rekening:** kelola rekening (bank, nomor, atas nama, akun GL, default, tampil di dokumen).
- **Form order jual:** pilih "Rekening tujuan", terisi default. Detail order menampilkan dan mengubahnya.
- **Modal bayar order, form pembayaran, koreksi, konfirmasi SJ (COD), GR:** saat metode transfer/cek, muncul
  pilihan rekening yang terisi dari order.
- **POS:** saat memilih Transfer, muncul rekening tujuan (default cabang/perusahaan). Struk mencetak rekening itu.
- **Cetak:**
  - Nota/tagihan: satu baris "Transfer ke: BCA 123… a.n. …" di area kaki, sehingga tetap tercetak pada
    form kontinu preprinted.
  - Bukti bayar: "Ke rekening".
  - Struk POS transfer: rekening terpilih. Struk lain mencetak semua rekening `show_on_documents`, seperti legacy.
  - Sumber daftar rekening cetak berpindah dari `document_profile.bankAccounts` ke master. Bila master
    masih kosong, daftar lama tetap dipakai, sehingga tidak ada kekosongan saat transisi.
- **Daftar pembayaran:** filter dan kolom rekening.

### 5.4 Akuntansi saat aktivasi (wajib akuntan)

Akun 1010 "Rekening Bank" saat ini menampung saldo **gabungan** semua bank. Saat fitur diaktifkan pada
tanggal X (sesudah cutover final):

1. Buat satu akun `is_cash` per rekening (mis. 1011 BCA, 1012 Mandiri), atau tetapkan 1010 sebagai salah
   satu rekening (Q3).
2. Buat jurnal manual reklasifikasi (`POST /finance/journals/manual`, sudah ada) per cabang:
   Dr 1011/1012… / Cr 1010, sebesar saldo rekening koran per tanggal X. Selisihnya diputuskan akuntan.
3. Mulai X, pembayaran transfer wajib membawa rekening (Q5). Pemetaan `bank` hanya tersisa untuk data historis.

ETL tidak berubah. Kolom `NULL` berarti jalur lama, dan verify migrasi hanya berlaku untuk hari migrasi.
Mengaktifkannya di data uji v2 sekarang tidak ada gunanya, karena data akan dimuat ulang saat cutover final.

### 5.5 Yang sengaja tidak masuk

- Rekonsiliasi bank (impor rekening koran).
- QRIS/EDC sebagai metode terpisah (MDR, settlement).
- Default rekening per pelanggan (ERPNext). Bisa ditambah bila diminta (Q2).

## 6. Rencana retur

| ID | Tindakan | Dasar |
|---|---|---|
| **RT-A** | Konfirmasi kasus nyata (nomor order/retur, pesan galat) | §2.1: tiga kemungkinan sebab dengan solusi berbeda |
| **RT-B** | Rilis perbaikan satuan `858f84d`. Pilihannya: rilis `main` (ikut membawa SJ Langsung yang izinnya tidak diberikan ke role mana pun, alert kesehatan data, dan migrasi `000042`–`000045`), atau hotfix dari `4deff6b` + `858f84d` | Bug terverifikasi di versi live. Keputusan rilis di tangan pemilik (rilis lewat CI → GHCR → `deploy.sh`) |
| **RT-C** | Form retur jual & beli: qty awal **kosong**, dengan tombol **"Retur semua sisa"** dan "maks" per baris. Ringkasan menampilkan baris yang dipilih. Tanpa perubahan server; server sudah menolak retur tanpa baris | Odoo 18+ dan sistem lama (pilih-dulu) mencegah retur penuh yang tidak disengaja. Perubahan ada di `ReturnLinesTable` + dua halaman + vitest + e2e E22 |
| RT-D (opsional) | **Konfirmasi SJ dengan qty diterima** (pelanggan menolak sebagian di tempat): qty terkonfirmasi ≤ qty draf, sisanya kembali menjadi sisa order (lalu tutup sisa bila tidak dikirim ulang). Pelunasan COD dihitung dari qty diterima | Odoo/ERPNext: penolakan saat kirim bukan retur. Butuh analysis + ADR tersendiri karena menyentuh stok, jurnal SJ, COD, dan cetak |

## 7. Keputusan yang dibutuhkan

| # | Pertanyaan | Usulan default |
|---|---|---|
| Q1 | Skenario retur mana yang dikeluhkan (a satuan / b form penuh / c ditolak saat kirim)? | Kirim nomor order contoh |
| Q2 | Default rekening: satu untuk perusahaan, per cabang, atau per pelanggan? | Perusahaan + boleh per cabang. Per pelanggan ditunda |
| Q3 | Saldo 1010: dipecah lewat jurnal reklasifikasi, atau 1010 dijadikan rekening utama? | Reklasifikasi per rekening koran tanggal X (akuntan) |
| Q4 | Nota mencetak rekening terpilih. Ini menyimpang dari D12 (cetak sama dengan legacy) | Ya, satu baris "Transfer ke". Revisi ADR-0018 D12 sebagai pengecualian |
| Q5 | Transfer/cek wajib memilih rekening? | Wajib, dengan nilai awal terisi |
| Q6 | Pembayaran keluar (pemasok, refund) ikut memilih rekening sumber? | Ya, dengan kolom yang sama, di fase kedua |
| Q7 | Rekening dibatasi per cabang? | Tidak. Semua cabang melihat semua rekening aktif |
| Q8 | Rilis RT-B: `main` penuh atau hotfix? | Pemilik memutuskan |

## 8. Urutan kerja yang diusulkan

1. **Retur:** RT-A → RT-B → RT-C (kecil, bisa dikerjakan segera). RT-D hanya bila Q1 = c.
2. **Rekening:** ADR-0026 → migrasi + finance (master, options, posting, dasbor) → payment → sales → jalur
   settlement (SJ/GR/retur/POS) → frontend → cetak → dokumen (`API_CONTRACT` §6, `DB_SCHEMA`, glosarium,
   `audit-labels.ts`, `STATUS.md`).
3. **Uji kunci:**
   - Integrasi: pembayaran transfer ber-rekening menjurnal ke akun rekening.
   - Tanpa rekening tetap ke 1010.
   - Koreksi pembayaran membalik dan menjurnal ulang ke rekening baru.
   - Rekening diarsipkan/bukan `is_cash` ditolak.
   - Dasbor menjumlahkan semua akun `is_cash`.
   - Order terkonfirmasi ber-SJ boleh ganti rekening; order selesai lewat endpoint khusus.
   - ETL `verify --self-test` tetap lulus.
   - E2E: POS transfer → struk rekening; bayar order → laporan kas per rekening.
4. **Aktivasi produksi** sesudah cutover final: buat rekening → jurnal reklasifikasi → set default → umumkan
   ke kasir.
