# Feature Inventory — Modul 08 Order Purchasing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = PO pembelian (buat/ubah/daftar/detail/arsip/pindah status) +
penerimaan barang (`orders/receive-goods`, `goods-receipts/*`) + konfigurasi status order +
export order (filter pembelian). Yang **bukan** bagian modul ini: penjualan/POS
(modul 09/15), pengiriman/SJ (modul 11), retur (modul 12/13), pembayaran manual & saldo
(modul 14), posting finance (modul 17). Perilaku sales disebut hanya sebagai pembanding
batas. `orders/approve-credit` menolak purchase secara eksplisit — milik modul 09.

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint dalam lingkup | 14 — `orders/{list,detail,create,update,update-status,archive,status-history}` (dipakai purchase), `orders/receive-goods`, `goods-receipts/{list,detail,create}`, `orders/export` (filter purchase), `order-status-definitions/{list,create,update}`, `order-status-transitions/{list,create,update}` |
| Di luar lingkup (satu controller) | `orders/deliver-goods`, `orders/approve-credit` (sales) → modul 09/11 |
| Halaman | Dipakai bersama sales: `/orders` (tab Pembelian), `/orders/create`, `/orders/:orderId`, `/orders/:orderId/edit` — aspek pembelian di §2 |
| Menu sidebar | Grup **"Operasional"**, label **"Order"** (`/orders`, izin `order.view`); tab default = Penjualan |
| Permission | `order.view` (daftar/detail/riwayat/penerimaan-list), `order.create` (buat PO), `order.update` (ubah/pindah status/terima barang/buat penerimaan), `order.archive` (arsip PO), `order.export` (export), `company_config.view/manage` (konfigurasi status). Kasus COD `pay_now` menuntut tambahan `payment.create` di sesi (403 bila tidak ada) |
| Tabel yang dimiliki | `orders`, `order_items`, `order_status_definitions`, `order_status_history`, `order_status_transitions`, `goods_receipts`, `goods_receipt_items` |
| Tabel yang ditulis lintas modul | `inventory_balances` (+), `inventory_movements` (`in`/`goods_receipt`), `payments` (COD `pay_now`), `branch_document_sequences` (nomor PO + nomor bayar) |
| Penomoran | `ORD-{KODECABANG}/PB/YYYY/MM/00001` — reset bulanan. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | `order.create/update/status_change/archive/goods_received/approve_credit` (approve_credit juga dipakai switch-to-net penerimaan), `payment.create` (sumber `receive_goods_cod`) |
| Aksi yang TIDAK ada | Hapus permanen PO; edit penerimaan yang sudah tersimpan; hapus/batal penerimaan; retur langsung dari PO (wajib via modul 13); terima barang tanpa status selesai terkonfigurasi; ubah baris PO setelah stok bergerak/ada bayar/masuk finance |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar Order, tab Pembelian (`/orders`, `order_kind=purchase`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Tiga tab | Penjualan (default bila param tak dikenal) / Pembelian / Semua (`order_kind` di URL, reset `page`) |
| F-01.2 | Cari Order/Pihak | Satu frasa LIKE: nomor order + kode/nama/telepon/email pihak. Placeholder `"Nomor order, nama, no HP, email..."`, debounce 400ms |
| F-01.3 | Filter status operasional | Semua/Pending/Aktif/Selesai/Dibatalkan (`status_group`) |
| F-01.4 | Filter termin | Semua/Prabayar/Bayar Saat Serah/Tempo (Utang/Piutang) (`payment_terms`) |
| F-01.5 | Filter tanggal | `start_date` + `end_date` (tanggal order; akhir dikirim `T23:59:59`); tombol `Hapus Filter` (merah, ghost) bila ada filter aktif |
| F-01.6 | Kolom (20/baris) | Nomor (+tanggal, +badge Pembelian kuning bila tab Semua) · Supplier (nama + telepon??email??kode; `-`) · Dibuat Oleh · Termin (`Prabayar`/`Tempo`/`Bayar Saat Terima` untuk purchase) · Dibayar · Total · Status (badge status + badge Lunas/Bayar Sebagian/Jatuh Tempo/Belum Bayar + `Diretur penuh`/`Diretur sebagian` bila ada retur completed) · Detail |
| F-01.7 | Ringan + kosong | Request memakai `summary_only` (tanpa item/riwayat; tanpa peta receipt/return); kosong: `Pesanan Kosong` / `Belum ada pesanan pembelian untuk filter ini.` + `Buat Order` (bila `order.create`) |
| F-01.8 | Tombol Export + Buat Order Baru | Header (masing-masing `order.export` / `order.create`) |

### F-02 — Buat / Edit PO (`/orders/create`, `/orders/:id/edit`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Jenis order | Dropdown Penjualan/Pembelian (default Penjualan). Ganti jenis: baris ikut harga saran jenis baru (kecuali yang diedit manual), termin mengikuti default bila sebelumnya default (`net` untuk purchase), pihak di-reset, quote dibatalkan |
| F-02.2 | Supplier wajib | Label `Supplier *`; picker supplier server-side; tanpa shortcut tambah (tombol Tambah Pelanggan hanya sales) |
| F-02.3 | Harga saran beli | Otomatis `purchasePrice ?? sellingPrice ?? minSellingPrice ?? 0` + teks bantu `Harga beli referensi produk: Rp X / {purchaseUom}`; satuan & faktor memakai sisi BELI (purchaseUom/faktor) |
| F-02.4 | Tanpa diskon baris | Kolom `Diskon (Rp)` hanya sales; submit purchase memaksa diskon undefined (total Diskon selalu 0 di ringkasan) |
| F-02.5 | Finansial purchase | Termin (default Tempo/net): Prabayar / Bayar Saat Serah / Tempo; Jatuh tempo (`datetime-local`, required, hanya bila net); Nomor invoice supplier (placeholder `INV-SUP-001`); Tanggal invoice supplier (mengisi → jatuh tempo otomatis = invoice + hari termin bila net); Termin supplier hari (default 30, `min=0`; mengubahnya menghitung ulang jatuh tempo) |
| F-02.6 | Pajak opsional | Checkbox `Transaksi ini kena pajak` + teks `Isi hanya untuk transaksi yang memang perlu masuk rekap pajak.`; bila aktif: tarif % (`min=0 step=0.01`, default 11), `Harga sudah termasuk pajak`, nomor + tanggal faktur pajak (opsional) |
| F-02.7 | Baris + validasi kirim | Tambah Baris / Hapus baris (bila >1); per baris: produk (+varian bila perlu), qty, harga, konversi (`1 {beli} = {n} {stok}` bila faktor ≠1), ringkasan base, catatan. Submit terkunci bila: produk belum siap, ada baris tak valid (tanpa produk/qty ≤0/varian wajib kosong), atau sedang menyimpan. Tanpa blokir quote (khusus sales) |
| F-02.8 | Simpan | `Menyimpan...` → toast `Order berhasil dibuat/diperbarui` → ke detail; gagal → toast `Gagal menyimpan order` + pesan, form tetap terbuka. Nomor order backend (FE mengirim nomor dummy `ORD-YYYYMMDD-XXX` yang diabaikan server) |
| F-02.9 | Edit by-id | Id di luar store/direct/refresh → fetch `orders/detail` + layar muat/tidak-ditemukan (`Order tidak bisa diedit` / `Data order tidak ditemukan atau Anda tidak memiliki akses ke order ini.`) |

### F-03 — Detail PO (`/orders/:orderId`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Kartu info purchase | Nomor, status, supplier, tanggal; `Invoice Supplier` + `Tanggal Invoice Supplier` (bila ada); `Termin Supplier` (`{n} hari`, bila net + terisi); per baris `Diterima X / Y {uom}` |
| F-03.2 | Tombol Terima Barang | Syarat: `order.update` + purchase + belum `goodsReceivedAt` + ada transisi selesai + bukan (prabayar & belum lunas). Sesudah selesai: teks `Diterima {datetime}` (tanpa tombol) |
| F-03.3 | Tombol Retur ke Supplier | Syarat: `purchase_return.create` + sudah `goodsReceivedAt` → `/purchase-returns/create?order_id=` |
| F-03.4 | Peringatan prabayar | Bila prabayar belum lunas: notice `Pelunasan diperlukan sebelum menerima barang` / `Termin prabayar harus lunas sebelum barang dapat diterima. Catat pembayaran ke supplier melalui seksi Pembayaran di bawah.` |
| F-03.5 | Riwayat Penerimaan Barang | Kosong: `Belum ada penerimaan` / `Purchase order ini belum memiliki batch penerimaan barang.`; tabel: Waktu (+`oleh {nama}`) · Dokumen Supplier (`-` bila tanpa) · Lokasi (header; fallback item pertama; `-` bila tak ada — diverifikasi) · Item Diterima (`{nama}: {qty} {uom}` per item) · Catatan |
| F-03.6 | Pindah status manual | Tombol per transisi aktif dari status kini (label transisi); yang ke selesai disembunyikan sebelum barang diterima; setelah diterima hanya ke selesai; ke batal diblokir bila sudah ada movement/bayar/finance |
| F-03.7 | Seksi pembayaran + arsip | Bayar/ledger milik modul 14; arsip (`order.archive`) diblokir dengan pesan bila sudah bergerak/berbayar/masuk finance |

### F-04 — Terima Barang (dialog di detail PO → `orders/receive-goods`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-04.1 | Deskripsi dinamis | COD berlunasan-akhir: `Menerima barang dari PO {nomor} akan menambah stok berikut. Setelah stok masuk, termin Bayar Saat Terima mengharuskan keputusan langsung: bayar sekarang atau ubah ke tempo.` (+` dan langsung menutup order ini` bila net). Selain itu: `Menerima barang dari PO {nomor} akan menambah stok berikut[.]` |
| F-04.2 | Field | `No. Surat Jalan Supplier` (`Opsional`) · `Tanggal Terima` (`datetime-local`, default kini; boleh masa lalu — movement memakai tanggal ini) · `Lokasi cepat untuk semua item` (required + helper pilah) · per item: qty (`max=sisa`, `min=0 step=0.0001`, dijepit UI 0…sisa; label `sisa {n} {uom}`) + `Lokasi simpan` (required per baris) · kotak ringkas `Total item purchase order` (tracked saja: `• {nama} +{qty} {uom} ({base} stok)` bila faktor ≠1) · `Catatan penerimaan` |
| F-04.3 | Keputusan COD (hanya penerimaan terakhir + sisa > 0) | `Keputusan pembayaran saat terima`: `[Bayar Sekarang]` (hanya `payment.create`; tanpa izin: teks `Anda tidak memiliki izin mencatat pembayaran dari dialog ini. Order COD hanya bisa diproses dengan mengubahnya ke tempo.`) / `[Ubah ke Tempo]`. Bayar: Total pembayaran supplier (readonly) + Tanggal Bayar + Metode (Tunai/Transfer Bank/Cek-Giro) + No. Referensi + Catatan + Bukti (opsional JPG/PNG 5MB, dikompres; gagal upload tidak membatalkan penerimaan). Tempo: `Jatuh Tempo Baru` + Total Utang ke Supplier |
| F-04.4 | Label konfirmasi dinamis | Bukan final: `Simpan Penerimaan Parsial`; final non-COD: `Konfirmasi & Tambah Stok`; final COD: `Terima, Bayar & Selesaikan` / `Ubah ke Tempo & Selesaikan`. Terkunci bila: tak ada qty > 0, ada baris tanpa lokasi, atau (COD final) tanggal/jatuh-tempo kosong |
| F-04.5 | Hasil | Toast `Barang berhasil diterima` + 1 dari 5 deskripsi (bayar/tempo/selesai-otomatis/parsial/lunas-dulu/fallback). Daftar + stok me-reload; bukti diunggah setelahnya bila ada. Item kosong (`Semua item ... sudah diterima.`) = dialog tanpa baris — konfirmasi tetap memanggil API yang menolak `Tidak ada sisa item yang dapat diterima` (→ KI baru) |

### F-05 — Status order: pindah + konfigurasi

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Pindah status (`orders/update-status`) | Target harus transisi aktif dari status kini (+ alasan opsional → history). Aturan purchase: selesai wajib sudah diterima; sudah-diterima hanya boleh ke selesai; batal diblokir pasca-movement/bayar/finance; selesai-terminal menuntut lunas untuk prabayar/COD (`Order dengan termin prabayar harus lunas sebelum diselesaikan.` / `...bayar saat serah...`) |
| F-05.2 | Riwayat (`orders/status-history`) | Urut menaik: dari → ke + pengubah + alasan + waktu |
| F-05.3 | Kelola definisi (`company_config.manage`) | Seed bawaan 5 status `all` (Draft-pending-awal → Dikonfirmasi/Diproses-active → Selesai-completed-terminal → Dibatalkan-cancelled-terminal) + 6 transisi (Konfirmasi, Mulai Proses, Selesaikan, Batalkan ×3). Halaman `/settings/order-status`: tambah status (selalu Menunggu/non-terminal, lihat KI-35) + tambah aturan (tanpa validasi — KI-31); kode unik per perusahaan; `applicable_order_kind` ∈ sales/purchase/all |
| F-05.4 | Arsip PO | Guard movement/bayar/finance + soft + audit. Tanpa restore |

### F-06 — Export order (filter Pembelian)

Modal di daftar (`order.export`): jenis terisi dari tab; bulan berjalan default; tanggal wajib
(peringatan `Tanggal export belum lengkap` / `Range tanggal tidak valid` bila terbalik); sukses →
file terunduh + toast `Export selesai` (`{file} siap diunduh ({n} order, {KB} KB).`); gagal → toast.
Server: `order_kind` all/sales/purchase, `format` xlsx/pdf, tanggal `YYYY-MM-DD` wajib, maks 366
hari, maks 5000 order; file `report-order_{jenis}_{dari}_{sampai}.{ext}`; sheet Ringkasan, Daftar
Order (22 kolom + autofilter; termasuk Invoice Supplier), Detail Item (snapshot + UOM ganda).
Detail logika di [reports-list.md](reports-list.md).

---

## 3. Edge Case (30)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Jenis selain sales/purchase | `400 'order_kind hanya mendukung sales atau purchase.'` (create, update, filter list) |
| E-02 | PO tanpa supplier | `400 'Purchase order wajib memilih supplier.'` (label FE `Supplier *`) |
| E-03 | Pihak customer/partner untuk PO | `400 'Pihak terkait untuk purchase order harus bertipe supplier, bukan {t}.'` |
| E-04 | Supplier arsip/tak dikenal | `400 'Supplier tidak ditemukan atau sudah diarsipkan.'` |
| E-05 | Tanpa status awal terkonfigurasi | `400 'Status awal order belum dikonfigurasi'` (cocok kind lalu `all`) |
| E-06 | Produk baris tak dikenal | `404 'Produk ID {id} tidak ditemukan'` |
| E-07 | Varian wajib tak dipilih / nonaktif / tak dikenal / default hilang | `400 'Varian produk '{nama}' wajib dipilih'` / `'Varian produk tidak aktif'` / `404 'Varian produk tidak ditemukan'` / `400 'Varian default produk '{nama}' belum tersedia'` |
| E-08 | Qty ≤ 0 / diskon > harga / tarif pajak di luar 0–100 / uang negatif | `Qty produk '{nama}' harus lebih dari 0.` / `Diskon produk '{nama}' tidak boleh melebihi harga satuan.` / `Tarif pajak harus bernilai 0 sampai 100.` / `'{label}' harus bernilai 0 atau lebih.` |
| E-09 | Termin hari bukan integer 0–3650 (purchase net) | `400 'Termin hari pembayaran harus berupa angka 0 sampai 3650.'`; non-net/non-purchase → disimpan null (input diabaikan) |
| E-10 | Net tanpa jatuh tempo & tanpa (invoice+trial-hari) | `400 'Jatuh tempo wajib diisi untuk termin tempo.'`; non-net → jatuh tempo selalu null (dibersihkan) |
| E-11 | Edit order terminal | `400 'Order terminal tidak bisa diubah'` (+ varian tak-ditemukan untuk id/status) |
| E-12 | Ganti baris setelah movement/bayar/finance | `400 'Order tidak dapat diubah barangnya karena {stok sudah bergerak... Gunakan retur atau adjustment. \| sudah memiliki pembayaran... refund/adjustment... \| sudah masuk catatan keuangan (Finance)... reversal jurnal...}'`; header saja (tanpa items) tetap boleh |
| E-13 | Arsip / batal pasca-movement/bayar/finance | Pesan sepasang dengan kata `diarsipkan`/`dibatalkan` |
| E-14 | Selesai sebelum terima / sudah-terima ke non-selesai / transisi tak ada / dari terminal | `Purchase order tidak dapat diselesaikan sebelum barang diterima` / `...hanya dapat dilanjutkan ke status selesai.` / `Transisi status tidak diizinkan` / `Order sudah berada di status terminal` (+ varian update-status `Order tidak ditemukan`) |
| E-15 | Terima: bukan purchase / terminal / tanpa item / tanpa status-selesai / tanpa transisi tahap | `Hanya purchase order (jenis: purchase) yang dapat diterima barangnya` / `Order sudah berada di status terminal dan tidak dapat diproses` / `Order tidak memiliki item` / `Status selesai untuk purchase order belum dikonfigurasi` / `Purchase order belum berada pada tahap penerimaan barang.` |
| E-16 | Terima: qty 0/negatif/bukan-angka / item duplikat / item luar order / melebihi sisa | `Quantity receipt harus lebih dari 0` / `Item receipt tidak boleh duplikat` / `Item order {id} tidak ditemukan pada order ini` / `Quantity receipt untuk {nama} melebihi sisa yang belum diterima` (toleransi 0.0001) |
| E-17 | Terima: tanpa sisa | `Tidak ada sisa item yang dapat diterima` (items kosong → isi-otomatis sisa; tetap kosong → error ini) |
| E-18 | Terima: prabayar belum lunas | `Purchase order prabayar harus lunas sebelum barang diterima.` (tombol Terima disembunyikan + notice; E2E 05 mengunci) |
| E-19 | Terima: settlement di luar COD / di luar penerimaan terakhir / final COD tanpa pilihan | `Pilihan penyelesaian COD hanya berlaku untuk termin bayar saat serah.` / `Penyelesaian pembayaran COD hanya diproses pada penerimaan terakhir.` / `Purchase order COD harus memilih bayar saat terima atau diubah ke tempo sebelum selesai.` |
| E-20 | Terima COD bayar tanpa `payment.create` | `403 'Anda tidak memiliki izin untuk mencatat pembayaran supplier saat terima barang.'` (UI menyembunyikan opsi + teks) |
| E-21 | Terima: lokasi induk/nonaktif/tak dikenal; tanpa default cabang | `Lokasi stok harus berupa lokasi akhir (tanpa sub-lokasi)` / `Lokasi stok tidak aktif` / `Lokasi stok tidak ditemukan` / `Lokasi stok default belum dikonfigurasi untuk cabang ini` (+ varian default-beranak) |
| E-22 | Terima: tanggal tak valid | Label `Tanggal penerimaan`/`Tanggal pembayaran` via `parseDateInput` (format tak valid → 400; kosong → kini) |
| E-23 | Terima: item non-stok | Tercatat tanpa movement/saldo (hanya baris penerimaan) |
| E-24 | Terima final: `goodsReceivedAt` = tanggal terima (bisa masa lalu) + status selesai + history `Order otomatis diselesaikan saat seluruh barang sudah diterima`; parsial: status tetap, tanggal tetap null (E2E 05 mengunci null parsial) |
| E-25 | Lokasi header penerimaan = lokasi bila seluruh baris seragam, else null |
| E-26 | Saldo: buat bila belum ada (0/0/0); `on_hand += base`, `available = on_hand − reserved`; movement `in`/`goods_receipt` + metadata order/item/penerimaan + konversi |
| E-27 | Bukti bayar COD gagal diunggah | Penerimaan tetap sah (komentar kode; toast sukses penerimaan tetap) |
| E-28 | `limit` list >100 → 100; 0/negatif/string → terus (sekelas KI-26/55/70 → KI baru) |
| E-29 | `date_to` tanpa waktu = `<= YYYY-MM-DD` mentah (tanpa akhir-hari) di API; FE menambah `T23:59:59` — request langsung kehilangan order di hari akhir setelah 00:00 ([PERLU KONFIRMASI]/KI baru) |
| E-30 | `summary_only` mematikan peta receipt/return (badge Diretur hilang di daftar — daftar selalu summary; badge dihitung dari query batch terpisah yang tetap jalan untuk purchase non-summary? — daftar memakai summary_only:true sehingga `returnSummary` selalu default `{false,false}`: badge Diretur **tidak pernah tampil di daftar** — → KI baru) |

---

## 4. Katalog Pesan (teks apa adanya)

**Order:** `'Order tidak ditemukan'` · `'order_kind hanya mendukung sales atau purchase.'` ·
`'Status awal order belum dikonfigurasi'` · `'Purchase order wajib memilih supplier.'` ·
`'Pihak terkait untuk purchase order harus bertipe supplier, bukan {t}.'` ·
`'Supplier tidak ditemukan atau sudah diarsipkan.'` · `'Produk ID {id} tidak ditemukan'` ·
`"Varian produk '{n}' wajib dipilih"` / `'Varian produk tidak aktif'` / `'Varian produk tidak ditemukan'` /
`"Varian default produk '{n}' belum tersedia"` · `"Qty produk '{n}' harus lebih dari 0."` ·
`'Tarif pajak harus bernilai 0 sampai 100.'` · `'{label} harus bernilai 0 atau lebih.'` ·
`'Termin hari pembayaran harus berupa angka 0 sampai 3650.'` ·
`'Jatuh tempo wajib diisi untuk termin tempo.'` · `'Order terminal tidak bisa diubah'` ·
`'Order tidak dapat {diubah barangnya\|dibatalkan\|diarsipkan} karena {stok sudah bergerak. Gunakan retur atau adjustment. \| sudah memiliki pembayaran. Selesaikan refund atau adjustment terlebih dahulu. \| sudah masuk catatan keuangan (Finance). Lakukan reversal jurnal terlebih dahulu.}'` ·
`'Order sudah berada di status terminal'` · `'Transisi status tidak diizinkan'` ·
`'Purchase order tidak dapat diselesaikan sebelum barang diterima'` ·
`'Purchase order yang barangnya sudah diterima hanya dapat dilanjutkan ke status selesai.'` ·
`'Order dengan termin prabayar harus lunas sebelum diselesaikan.'` ·
`'Purchase order tidak memerlukan persetujuan kredit'` (approve-credit, batas modul) ·
`'Tanggal order' / 'Tanggal invoice supplier' / 'Tanggal faktur pajak' / 'Jatuh tempo'` (label parse tanggal).

**Terima:** `'Hanya purchase order (jenis: purchase) yang dapat diterima barangnya'` ·
`'Order sudah berada di status terminal dan tidak dapat diproses'` · `'Order tidak memiliki item'` ·
`'Status selesai untuk purchase order belum dikonfigurasi'` ·
`'Purchase order belum berada pada tahap penerimaan barang.'` · `'Quantity receipt harus lebih dari 0'` ·
`'Item receipt tidak boleh duplikat'` · `'Item order {id} tidak ditemukan pada order ini'` ·
`"Quantity receipt untuk {n} melebihi sisa yang belum diterima"` · `'Tidak ada sisa item yang dapat diterima'` ·
`'Purchase order prabayar harus lunas sebelum barang diterima.'` ·
`'Pilihan penyelesaian COD hanya berlaku untuk termin bayar saat serah.'` ·
`'Penyelesaian pembayaran COD hanya diproses pada penerimaan terakhir.'` ·
`'Purchase order COD harus memilih bayar saat terima atau diubah ke tempo sebelum selesai.'` ·
`'Anda tidak memiliki izin untuk mencatat pembayaran supplier saat terima barang.'` (403) ·
`'Lokasi stok tidak ditemukan'` / `'Lokasi stok tidak aktif'` /
`'Lokasi stok harus berupa lokasi akhir (tanpa sub-lokasi)'` ·
`'Lokasi stok default belum dikonfigurasi untuk cabang ini'` ·
`'Order penerimaan wajib diisi'` · `'Item order penerimaan wajib diisi'` ·
`'Penerimaan barang tidak ditemukan'` · `'Alamat kirim tidak ditemukan.'` /
`'Alamat kirim bukan milik pelanggan ini.'` (jalur sales, batas).

**Status config:** `"Kode status '{k}' sudah ada"` · `'Status tidak ditemukan'` ·
`'Transisi status tidak ditemukan'` · `'applicable_order_kind hanya mendukung sales, purchase, atau all.'`

**Toast:** `Order berhasil dibuat/diperbarui` · `Gagal menyimpan order` + pesan ·
`Barang berhasil diterima` + 5 varian deskripsi (bayar/tempo/selesai-otomatis/parsial/lunas-dulu/fallback) ·
`Gagal menerima barang` + pesan · `Export selesai` (`{file} siap diunduh ({n} order, {KB} KB).`) ·
`Gagal export order` + pesan · `Tanggal export belum lengkap` / `Range tanggal tidak valid` (warning pra-kirim).

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Hapus permanen PO/penerimaan | Soft arsip PO; penerimaan tanpa arsip/batal sama sekali |
| NF-02 | Edit/hapus penerimaan tersimpan | Tanpa endpoint; salah catat = adjustment stok manual (modul 16) |
| NF-03 | Restore PO | Tanpa endpoint (beda dengan pihak yang punya) |
| NF-04 | Diskon baris PO | FE menyembunyikan; submit memaksa undefined |
| NF-05 | Guard harga minimum PO | Hanya sales; purchase bebas (harga = kesepakatan supplier) |
| NF-06 | Member pricing PO | `resolveLinePricing` non-sales selalu manual |
| NF-07 | Terima parsial COD-bayar | Settlement hanya penerimaan terakhir |
| NF-08 | Terima tanpa transisi ke selesai | Wajib ada jalur aktif kini→selesai walau parsial |
| NF-09 | Nomor penerimaan sendiri | Identitas = id + nomor SJ supplier (opsional) + tanggal |
| NF-10 | Pindah cabang PO | `idBranch` dari sesi + guard; tanpa endpoint pindah |
