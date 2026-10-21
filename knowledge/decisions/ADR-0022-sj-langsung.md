# ADR-0022: SJ Langsung — surat jalan dulu, order dibuat dari surat jalan

## Status
Accepted (2026-10-21) dengan **default K1–K7 dipakai**; konfirmasi pemilik dan akuntan masih ditunggu
(tercatat di `docs/STATUS.md` §3). Diimplementasikan sesuai
[`docs/plan/sj-langsung/PLAN.md`](../../docs/plan/sj-langsung/PLAN.md) (penyesuaian desain saat
implementasi ada di PLAN §0a). Analisis dan riset: [`analysis/001-sj-langsung/`](../../analysis/001-sj-langsung/analysis.md).
Melengkapi ADR-0004 (pemisahan `sales`/`delivery`), ADR-0016 (atomisitas lewat idempoten + CAS), ADR-0018 D2
(stok keluar saat konfirmasi SJ), ADR-0020 (invarian persediaan), ADR-0021 (tanggal ≤ hari ini), dan
ADR-0023 (pembatalan beralasan).

## Context
Pengguna meminta jalur tambahan. Gudang mengirim barang lebih dulu dengan surat jalan (SJ), lalu admin
penjualan membuat order dari SJ itu, biasanya karena harga baru disepakati setelah barang keluar. Jalur
SO → SJ yang sekarang tidak boleh berubah.

Hari ini SJ terikat penuh ke order jual:

- `delivery_notes.sales_order_id` wajib diisi, dan SJ tidak menyimpan pelanggan sendiri.
- Membuat dan mengonfirmasi SJ mensyaratkan SO terkonfirmasi. Pemeriksaan sisa kirim, pelunasan
  COD/prabayar, dan penyelesaian SO otomatis semuanya dihitung dari SO.
- Jurnal SJ (`buildDelivery`) menghitung piutang, penjualan, dan PPN dari harga baris SO, serta HPP dari
  nilai keluar stok SJ.
- Retur jual, cetak SJ, dan antrean kirim membaca SO.

Sistem lama tidak punya alur ini. ETL bahkan memperlakukan "SJ tanpa SO" sebagai data yatim (V-19).

ERP lain mengenal pola ini dengan pengaman yang konsisten:

- ERPNext membolehkan Delivery Note tanpa SO dengan status "To Bill", dan menyediakan akun antara
  *Stock Delivered But Not Billed*.
- Business Central mengambil baris pengiriman yang belum ditagih ke faktur baru ("Get Shipment Lines")
  dan bisa menggabungkan beberapa pengiriman.
- SAP Business One memperingatkan bahwa faktur yang tidak merujuk pengiriman yang ada akan mengeluarkan
  stok dua kali.

Menurut aturan faktur pajak (PER-03/PJ/2022), PPN terutang dan faktur pajak dibuat saat penyerahan barang.

## Decision
1. **Jalur tambahan, bukan pengganti.** SO → SJ, POS, dan SJ pengganti retur tidak berubah. Jalur baru
   bernama **SJ Langsung**: SJ yang dibuat tanpa order untuk pelanggan terdaftar (bukan pembeli umum).
2. **Model data.**
   - SJ Langsung tetap baris `delivery_notes` dengan `document_kind = 'order'`. Kolom baru
     `origin = 'direct'` (bawaan `'order'`) membedakannya.
   - Selama belum diorder, `sales_order_id = 0`. Pelanggan dan alamat kirim disimpan sebagai snapshot di
     SJ (`party_id`, `ship_to_*`).
   - Setelah ditautkan, SJ Langsung memakai seluruh query per-SO yang ada (`DeliveredQty`,
     `ConfirmedQtyBefore`, `ActiveNoteCount`, …) tanpa perubahan.
   - `sales_orders.origin` (`'standard'` | `'delivery'`) menandai order yang lahir dari SJ.
   - Tidak ada FK atau join lintas modul. Satu migrasi per modul.
3. **Siklus SJ Langsung.**
   - Alurnya `draft → (dikirim) → confirmed`. Stok keluar saat konfirmasi lewat `stock.Post` yang sama,
     tanpa pelunasan apa pun, lalu SJ menjadi **diorder** setelah tertaut.
   - Konversi satuan dan status lacak-stok diambil dari master produk, bukan dari SO.
   - Bukti terima (DL-01) tetap berlaku. SJ yang belum diorder boleh dibatalkan, dan stoknya dibalik.
   - Setelah tertaut, SJ mengikuti aturan SJ order yang ada. Retur hanya lewat Retur Jual dan hanya
     setelah tertaut.
4. **Order dari SJ** (endpoint di modul `sales`, pemilik harga).
   - Masukan: satu atau beberapa SJ Langsung terkonfirmasi yang belum diorder, dari cabang dan pelanggan
     yang sama.
   - Baris stok dibangun **di server** dari baris SJ (per produk/varian/satuan) dengan qty terkunci.
     Klien hanya mengirim harga, diskon, pajak, termin, dan baris jasa non-stok tambahan.
   - Satu langkah berorkestrasi (ADR-0016) yang idempoten lewat `client_ref`:
     1. buat dan konfirmasi SO;
     2. `delivery.LinkDirect` dengan CAS `sales_order_id 0 → id` dan pemeriksaan bahwa Σ SJ = qty order;
     3. SO selesai (`completeOrderIfDone`);
     4. bila tunai, catat pembayaran dengan `client_ref` deterministik.
   - Ulang setelah gagal di tengah melanjutkan, bukan menggandakan.
5. **Akuntansi masa tunggu (default K1).**
   - Jurnal SJ Langsung **ditunda sampai tertaut**. `buildDelivery` tidak berubah dan memakai tanggal SJ.
   - Selama menunggu, SJ tampil di antrean posting dengan alasan "Menunggu order".
   - Rekonsiliasi persediaan sudah menetralkan nilai ledger dokumen yang antre, dan `SafeClose` sudah
     menolak periode dengan dokumen belum diposting. Pemeriksaan kesiapan menambah butir khusus
     "SJ Langsung belum diorder".
   - Alternatif yang perlu persetujuan akuntan: akun antara *Barang Terkirim Belum Ditagih*. Saat SJ:
     Dr akun antara / Cr Persediaan. Saat order: pendapatan, PPN, piutang, dan Dr HPP / Cr akun antara.
6. **Pajak.** Pendapatan dan PPN bertanggal SJ. Form order menyarankan tanggal faktur pajak = tanggal SJ
   dan mengingatkan bila masa pajaknya berbeda.
7. **Pengaman.**
   - Izin baru `delivery.create_direct` untuk membuat SJ Langsung. Order dari SJ memakai `sales.create`.
   - Form order biasa menampilkan peringatan bila pelanggan masih punya SJ Langsung yang belum diorder,
     untuk mencegah barang keluar dua kali.
   - Dashboard dan daftar order menampilkan jumlah dan umur SJ yang belum diorder.
8. **Di luar cakupan.** Harga di SJ, SJ Langsung untuk pembeli umum, satu SJ dipecah ke beberapa order,
   dan menambah baris barang stok di order dari SJ.
9. **Penajaman saat implementasi** (PLAN §0a): SJ Langsung hanya memuat **barang berstok**; penyelesaian order
   adalah milik `sales` (`LinkDirect` hanya menautkan dan memeriksa Σ qty; `sales` mengonfirmasi, menautkan,
   melunasi, lalu `completed`; tautan gagal → SO baru dibatalkan); SJ Langsung yang belum diorder boleh
   dibatalkan **juga setelah terkonfirmasi** (stok dibalik, alasan wajib di web); SJ yang menunggu order
   **tidak bisa diabaikan** dari antrean posting; dasbor membaca `GET /deliveries/open-direct` tanpa tepi modul
   baru; kunci `sjl:<clientRef>` (sales), `so:<id>` + `sj:<id>` (delivery).

### Keputusan pemilik/akuntan

| ID | Pertanyaan | Default |
|---|---|---|
| K1 | Akuntansi selama SJ belum diorder | Tunda jurnal sampai tertaut (§5); akun antara hanya bila akuntan memintanya |
| K2 | Satu order boleh menggabungkan beberapa SJ? | Ya, untuk pelanggan dan cabang yang sama |
| K3 | Termin order dari SJ | Tempo (`net`) atau tunai dibayar saat order disimpan (`cod` + bayar sekarang/jadikan tempo, sama seperti SJ COD terakhir); prabayar ditolak |
| K4 | Batas umur SJ belum diorder | Tanpa batas keras; peringatan bila > 3 hari; tutup bulan normal terhalang |
| K5 | Siapa yang mendapat `delivery.create_direct` | Tidak diberikan otomatis ke role mana pun selain superadmin; pemilik memberikan per role |
| K6 | Baris jasa (ongkir dsb.) saat order dari SJ | Boleh, hanya produk non-stok |
| K7 | Waktu rilis | Setelah cutover produksi selesai dan stabil |

## Consequences
- Modul `delivery` mendapat kolom, kontrak (`ListOpenDirect`, `LinkDirect`, `OpenDirectSummary`), dan
  cabang logika untuk SJ yang belum punya SO. Logika SJ order yang ada tetap dan diuji ulang.
- `sales` memanggil `delivery` lewat kontrak untuk order dari SJ, lewat dependensi yang sudah di-*wire*
  terlambat (`SetDependents`).
- `finance` mendapat alasan antre baru dan butir kesiapan baru. Bila K1 memilih akun antara, `buildDelivery`
  dipecah dan butuh pemetaan akun baru.
- Ada jeda ketika stok sudah keluar tetapi buku besar belum mencatatnya. Pengendaliannya: rekonsiliasi yang
  menjelaskan selisih (ADR-0020), `SafeClose`, dan pengingat umur. Menutup paksa periode dengan SJ belum
  diorder akan menjatuhkan dokumen ke periode tertutup (peringatan `late_period_docs`).
- ETL tidak terdampak: semua baris migrasi memakai nilai bawaan `origin = 'order'`.
- Dokumen yang ikut diperbarui saat implementasi:
  - `docs/API_CONTRACT.md` §6;
  - `docs/DB_SCHEMA.md` (regenerasi + §8);
  - `knowledge/DOMAIN_GLOSSARY.md` (istilah "SJ Langsung");
  - `docs/SYSTEM_DESIGN.md` (kontrak lintas modul);
  - `apps/web/src/modules/audit/lib/audit-labels.ts`.
- Bukti: `apps/api/internal/integration/direct_delivery_test.go` (siklus SJ Langsung tanpa order, order dari
  SJ, bayar sekarang, penjaga `LinkDirect`, balapan dua admin, antrean kerja).
