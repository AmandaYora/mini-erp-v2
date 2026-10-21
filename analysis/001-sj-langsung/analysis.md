# Analisis 001 — SJ Langsung (surat jalan dulu, order belakangan)

**Tanggal:** 2026-10-21 · **Keputusan:** [ADR-0022](../../knowledge/decisions/ADR-0022-sj-langsung.md) ·
**Rencana:** [docs/plan/sj-langsung/PLAN.md](../../docs/plan/sj-langsung/PLAN.md) ·
**Diagram:** [flow.mmd](flow.mmd), [sequence.mmd](sequence.mmd), [erd.mmd](erd.mmd)

## 1. Kebutuhan

Pengguna meminta jalur baru. Gudang membuat surat jalan (SJ) dan mengirim barang lebih dulu, lalu order
jual dibuat berdasarkan SJ tersebut. Jalur ini **tambahan**: jalur SO → SJ yang sekarang tidak berubah.

## 2. Kondisi saat ini (2026-10-21)

| Titik | Fakta di kode |
|---|---|
| Skema | `delivery_notes.sales_order_id NOT NULL`; SJ tidak punya pelanggan sendiri (`apps/api/migrations/000012_delivery.up.sql`) |
| Membuat SJ | Mensyaratkan SO `confirmed`; baris harus ada di SO, satuan sama dengan SO, qty ≤ sisa SO; SO prabayar harus lunas (`delivery/application/service.go` `createLocked`) |
| Konfirmasi SJ | SO harus `confirmed`; kuota dicek ulang; bukti terima; pelunasan COD/prabayar; `stock.Post`; SO otomatis selesai (`confirmLocked`, `planSettlement`, `completeOrderIfDone`) |
| Jurnal | `finance/application/builders.go` `buildDelivery`: piutang, penjualan, dan PPN dari harga dan pajak baris SO (prorata per SJ); HPP dari nilai keluar stok SJ; tanggal = tanggal SJ |
| Antrean posting | SJ terkonfirmasi masuk antrean dari `ListConfirmedAfter`; dokumen yang gagal dibangun diberi alasan (`dailyclose.go` `classifyBuildFailure`) |
| Tutup periode | `SafeClose` menolak bila ada dokumen belum diposting (`readiness.go`); rekonsiliasi persediaan menetralkan nilai dokumen yang antre. `ClosePeriod` paksa tidak memeriksa apa pun |
| Ubah/batal SO | Harga dan qty terkunci bila ada SJ (`assertNoDependents`); batal ditolak bila ada SJ aktif |
| Retur | Batas retur dari `DeliveredQty` per SO |
| Cetak SJ | Pelanggan dan alamat kirim dari SO (`DeliveryNotePrintPage.tsx`) |
| POS | Frontend merangkai buat SO → konfirmasi → buat SJ → konfirmasi + bayar, idempoten lewat `clientRef` (`pos/lib/checkout.ts`), dan bisa dilanjutkan dari server (`sales/application/pos_state.go`) |
| Sistem lama | Tidak punya alur ini; SJ selalu dari order. ETL menganggap SJ tanpa SO sebagai yatim (V-19) |

## 3. Riset praktik ERP lain

| Sumber | Temuan | Pelajaran |
|---|---|---|
| ERPNext — [Delivery Note](https://docs.frappe.io/erpnext/user/manual/en/stock/delivery-note) | Delivery Note boleh dibuat tanpa SO ("tidak disarankan"); status **To Bill** untuk yang belum ditagih | Status jelas "Belum diorder" |
| ERPNext — [Stock Delivered But Not Billed](https://docs.frappe.io/erpnext/purpose-of-stock-delivered-but-not-billed) | Akun antara: Dr akun antara / Cr Persediaan saat kirim; Dr HPP / Cr akun antara saat faktur | Opsi akuntansi K1 alternatif |
| Business Central — [Combine shipments](https://learn.microsoft.com/en-us/dynamics365/business-central/sales-how-to-combine-shipments-on-a-single-invoice) | Faktur baru → pilih pelanggan → **Get Shipment Lines**; beberapa pengiriman bisa digabung | Pola "Ambil dari SJ" berawal dari form order |
| SAP Business One — [help](https://help.sap.com/doc/saphelp_sbo900/9.0/en-US/44/fd93e793202463e10000000a1553f7/content.htm) | Faktur tanpa merujuk pengiriman yang ada mengeluarkan stok dua kali | Order untuk barang yang sudah dikirim wajib tertaut; ada peringatan di form order biasa |
| Praktik tutup bulan ([diskusi Frappe](https://discuss.frappe.io/t/stock-issued-but-not-billed-recognizing-cost-of-goods-sold-upon-invoicing/101326), [Racklify](https://racklify.com/encyclopedia/query/unbilled-charges/)) | Laporan *shipped not invoiced* dibereskan sebelum tutup; bila hanya manual, sering ada yang terlewat | Umur, penanda di dashboard, butir wajib di kesiapan tutup |
| Pajak — [DDTCNews: kapan faktur pajak dibuat](https://news.ddtc.co.id/kapan-faktur-pajak-harus-dibuat-simak-di-sini-38282) | Faktur pajak dibuat saat penyerahan barang (PER-03/PJ/2022; periksa PER-1/PJ/2025) | Pendapatan dan PPN bertanggal SJ; order di masa pajak yang sama |

## 4. Opsi

- **A — "SJ cepat"** (hanya tampilan). Form SJ yang di belakangnya membuat SO berharga katalog + SJ dalam
  satu langkah, seperti POS. Tanpa perubahan skema dan akuntansi. **Tidak cocok** bila harga baru diketahui
  setelah barang keluar, karena harga SO terkunci begitu ada SJ.
- **B — SJ Langsung** (dipilih). SJ tanpa order dan tanpa harga; order dibuat belakangan dari satu atau
  beberapa SJ. Butuh migrasi `delivery` + `sales`, cabang logika SJ tanpa SO, endpoint order dari SJ,
  alasan antre dan butir kesiapan di `finance`, serta layar baru.

Kenapa B memakai `origin` dan bukan `document_kind` baru: semua query per-SO menyaring
`document_kind = 'order'`. Dengan SJ Langsung tetap `order` dan hanya `sales_order_id`-nya 0 sampai
tertaut, `DeliveredQty`, `ConfirmedQtyBefore`, `ActiveNoteCount`, retur, dan `buildDelivery` bekerja tanpa
perubahan setelah tertaut.

Kenapa default akuntansi menunda jurnal: antrean posting, alasan antre, rekonsiliasi yang menetralkan
dokumen antre, dan penolakan `SafeClose` sudah ada. Jeda "stok keluar belum dijurnal" dengan begitu sudah
terjelaskan dan terkendali tanpa akun baru. Akun antara tetap opsi bila akuntan memintanya (ADR-0022 K1).

## 5. Flow pengguna ideal

Tidak ada menu sidebar baru. Fitur ini menjadi pilihan di menu **Pengiriman** dan **Order Jual**,
dibatasi izin.

**Gudang** — Pengiriman → *Buat surat jalan ▾* → **Tanpa order (SJ Langsung)**

1. Isi form:
   - pelanggan (wajib);
   - alamat kirim: pilih dari alamat pelanggan atau ketik;
   - tanggal kirim (≤ hari ini);
   - sopir, kendaraan, petugas gudang;
   - baris: produk, satuan, qty, lokasi (kosong = alokasi otomatis). **Tanpa harga.**
2. Peringatan lunak: stok mungkin kurang, dan "pelanggan masih punya N SJ belum diorder (tertua X hari)".
3. Simpan (draf) → cetak SJ (tata letak sama, bertanda SJ Langsung, tanpa nomor order) → dikirim → konfirmasi
   terima (penerima, tanda tangan/alasan, foto SJ bertanda tangan) → stok keluar. Tanpa pertanyaan
   pembayaran.
4. Rel Dokumen: `Draf → Dikirim → Diterima → Diorder`. Sampai tertaut, SJ berlabel "Belum diorder".

**Admin penjualan** — panel **"SJ menunggu order (N) · tertua X hari"** di Order Jual dan di dashboard.

1. Ada dua pintu masuk ke form yang sama:
   - *Tambah order ▾* → **Dari surat jalan** → pilih pelanggan → centang SJ → **Ambil**;
   - di detail SJ, klik **Buat order dari SJ ini**.
2. Form order mode "dari SJ":
   - pelanggan dan alamat kirim terkunci;
   - baris barang dijumlahkan per produk/satuan dengan qty terkunci;
   - barang stok tidak bisa ditambah; baris jasa non-stok boleh;
   - harga otomatis (katalog/member), bisa diubah menurut aturan batas harga;
   - termin Tempo, atau Tunai + bayar sekarang/jadikan tempo; prabayar tidak ada;
   - tanggal faktur pajak disarankan = tanggal SJ.
3. **Simpan & selesaikan** → order berstatus *Selesai*, SJ berstatus *Diorder* → pilihan berikutnya:
   *Cetak invoice*, *Catat pembayaran*.
4. Form order **biasa** untuk pelanggan yang masih punya SJ Langsung terbuka menampilkan banner
   "Pelanggan ini punya N SJ belum diorder — ambil dari SJ?".

**Kasir / finance:** pembayaran lewat menu Pembayaran seperti biasa. Jurnal otomatis setelah tertaut,
bertanggal SJ.

**Akhir bulan:** kesiapan tutup periode punya butir "SJ Langsung belum diorder: 0". Bila belum nol,
ditampilkan daftar dan tautannya, dan `SafeClose` tertahan.

**Pengecualian:**

| Situasi | Penanganan |
|---|---|
| SJ salah (pelanggan atau barang), belum diorder | Batalkan SJ (stok kembali), buat ulang |
| Sebagian barang ditolak, belum diorder | Batalkan, buat ulang sesuai yang benar-benar diterima |
| Barang kembali setelah diorder | Retur Jual seperti biasa |
| Harga belum disepakati di akhir bulan | Tutup normal tertahan; keputusan pemilik |

## 6. Risiko

- **Jeda stok keluar tanpa jurnal:** dikendalikan oleh antrean, rekonsiliasi, `SafeClose`, dan umur.
  Menutup paksa menjatuhkan dokumen ke periode tertutup.
- **Barang keluar sebelum ada persetujuan harga dan kredit:** hanya diberi peringatan, karena belum ada
  fitur limit kredit.
- **Stok ganda bila admin membuat order biasa untuk barang yang sudah dikirim:** dicegah dengan banner
  dan dengan disiplin pemakaian.
- **Order setengah jadi** (SO terkonfirmasi, SJ belum tertaut): ditangani dengan orkestrasi idempoten +
  `origin = 'delivery'`, sehingga SO itu tidak tampil di Antrian Kirim dan bisa dilanjutkan.
- **Waktu rilis:** menyentuh `delivery` dan `finance` yang baru diaudit untuk ETL. Rilis setelah cutover
  (K7).
