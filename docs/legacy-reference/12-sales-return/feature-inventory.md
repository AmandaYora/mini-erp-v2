# Feature Inventory — Modul 12 Sales Return (Retur Penjualan)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode (`sales-return.controller.ts`, `sales-return.service.ts` 1400 baris, 3 entity, migrasi
038/046, 4 halaman + 2 komponen web, E2E 18). Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = retur/tukar pasca-jual + SJ pengganti Order. Yang **bukan** bagian
modul ini: SJ order biasa (modul 11 — di sini hanya SJ `replacement` yang ditulis modul ini),
retur pembelian (modul 13), CRUD pembayaran (modul 14 — di sini hanya baris settlement, bukan
baris `payments`), posting finance (modul 17 — di sini hanya basis biaya + movement).

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 7 — `sales-returns/{list,detail,context,preview,create}` (5), `sales-returns/replacement-deliveries/{dispatch,confirm}` (2) |
| Halaman | 3 rute: `/sales-returns`, `/sales-returns/create` (`?order_id=` opsional), `/sales-returns/:returnId` |
| Menu sidebar | Grup **"Operasional"**, label **"Retur / Tukar"** (`/sales-returns`, izin `sales_return.view`) |
| Permission | `sales_return.view` (daftar/detail/konteks), `sales_return.create` (preview/buat/dispatch/confirm pengganti), `sales_return.refund` (khusus refund tunai — 403 bila tidak ada) |
| Tabel yang dimiliki | `sales_returns`, `sales_return_items`, `sales_return_settlements` |
| Tabel yang ditulis | `inventory_balances` (±), `inventory_movements` (`in`/`sales_return`, `out`/`sales_return_exchange`), `delivery_notes` + `delivery_note_items` (pengganti Order), `branch_document_sequences` (nomor retur + nomor SJ) |
| Penomoran | `RTR-{KODECABANG}/{TAHUN}/{5 digit}` — tanpa reset. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | `sales_return.create/dispatch_replacement/confirm_replacement` |
| Aksi yang TIDAK ada | Edit/hapus/batal retur tersimpan; retur untuk purchase; retur order belum-serah; cicilan selisih (satu keputusan); baris `payments` untuk collect/refund (hanya baris settlement!) |

**Catatan lingkup.** Dua mode satu dokumen: `return_only` (tanpa pengganti) vs `exchange` (dengan
pengganti). Tukar POS (`source='pos'`) keluar-kounter langsung; tukar Order (`source ≠ 'pos'`)
dengan pengganti fisik DITUNDA ke SJ Pengganti (`pending` → `dispatched` → `delivered`); pengganti
jasa tak perlu kirim. Selisih = pengganti − retur: positif = tambah-bayar; negatif = potong
piutang dulu, sisa jadi refund/kredit; nol = tanpa penyelesaian.

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar (`/sales-returns`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Cari (tanpa debounce/URL) | Ketik langsung request; cari nomor retur / nomor order / nama customer. Placeholder `Cari nomor retur, order, atau customer` |
| F-01.2 | Filter status | Semua/Selesai/Dibatalkan (`completed`/`cancelled`); reset halaman |
| F-01.3 | Tombol Refresh | Muat ulang manual (secondary) — satu-satunya daftar dengan tombol ini |
| F-01.4 | Kolom | Dokumen (nomor + order asal) · Tanggal · Customer (`Walk-in` bila null) · Jenis (badge `Tukar` info / `Retur` netral) · Barang Retur (kanan) · Barang Pengganti (kanan) · Selisih (kanan tebal) · Penyelesaian (label Indonesia) |
| F-01.5 | Baris klik → detail | `onRowClick` ke `/sales-returns/:id` (tanpa tombol aksi) |
| F-01.6 | Paginasi 20 + kosong | `Belum ada retur/tukar` / `Buat dokumen saat customer mengembalikan barang atau menukar dengan barang lain.` + tombol Buat (bila `sales_return.create`) |
| F-01.7 | Gagal muat | Toast `Gagal memuat retur/tukar` + pesan (tanpa notice inline) |

### F-02 — Buat (`/sales-returns/create`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Pilih order asal | Tabel order sales-`completed` (limit 20, cari nomor + tombol Cari, klik baris) — ATAU langsung via `?order_id=` (dari detail order). Gagal konteks → toast `Order tidak bisa diretur` + pesan. Ganti Order mengulang dari awal |
| F-02.2 | Bar status lengket | Badge kesiapan (`Pilih barang retur` kuning / `Isi alasan retur` / `Siap dicek` hijau) + meta (`{n} barang retur dipilih[, {m} barang pengganti]`) + live Nilai retur → Pengganti → Selisih (hijau 0 / kuning + / merah −, tooltip kalimat) + tombol `Preview & Cek` (disabled bila belum siap/saat kirim) |
| F-02.3 | Barang dikembalikan | Kartu per item (centang; terpilih = garis merek kiri): nama + `kode · sisa bisa diretur {s} {uom}` + nilai proporsional live; buka: Jumlah (`max=sisa`, dijepit) + Kondisi (`Masuk stok lagi` / `Rusak` / `Tidak kembali`, default normal) + Simpan-ke-lokasi (hanya tracked + normal; default lokasi default) |
| F-02.4 | Barang pengganti (opsional) | Cari produk (pilih → langsung masuk daftar; pilih lagi = baris baru); per baris: Subtotal live + Hapus; Varian (bila ada) · Jumlah · Harga satuan (`step=100`) · Ambil-dari-lokasi (tracked saja; `Otomatis` = kosong → server memilih); kosong → `Belum ada barang pengganti.` |
| F-02.5 | Penyelesaian & catatan | Notice Selisih live (`Impas — tidak ada kas tambahan.` / `Customer perlu menambah {n}.` / `Customer menerima kembali {n}.`); Alasan* (placeholder contoh) · Cara selisih (`Otomatis (sesuai selisih)` + 4 opsi; `refund` hanya bila `sales_return.refund`) · Cara bayar (Tunai/Transfer/Cek) · Tanggal retur (`datetime-local`, default kini) · No. referensi + Catatan (opsional) |
| F-02.6 | Preview wajib | `Preview & Cek` → modal `Preview Retur / Tukar` (4 kartu + notice + 2 tabel) → `Simpan Retur / Tukar` (`Menyimpan...`; tutup dikunci). Tanpa preview → warning `Preview belum dikonfirmasi` / `Klik Preview & Cek lalu periksa data di modal sebelum menyimpan.` Gagal preview/buat → toast + pesan; sukses → ke detail baru |
| F-02.7 | Modal preview | Kartu Nilai Retur/Pengganti/Selisih (tone hijau/kuning/info) + Penyelesaian (label + hint angka: potongan-pengurang atau jumlah) · notice (tambah-bayar / potong-piutang / refund-atau-saldo / impas) · tabel retur (Produk/Qty/Kondisi/Lokasi/Nilai) + pengganti (Produk/Qty/Harga/Lokasi/Nilai) · `[Ubah Data]` + `[Simpan]` |

### F-03 — Detail (`/sales-returns/:returnId`)

Header (breadcrumb + nomor; deskripsi `Order asal {nomor}`; `[Buka Order]` + `[Cetak]`
(`window.print()`) + `[Kembali]` ghost) → 4 kartu ringkas (Nilai Retur/Pengganti/Selisih/
Penyelesaian) → Informasi (Tanggal/Customer/`Walk-in`/Status badge completed-hijau else merah/
Jenis `Tukar barang`/`Retur barang` + alasan) → Barang Retur (Produk/Kondisi/Lokasi/Qty/Nilai)
→ Barang Pengganti (`Tidak ada barang pengganti.` bila kosong; Produk/Lokasi/Qty/Harga/Nilai)
→ Pengiriman Barang Pengganti (§F-04; hilang bila bukan-exchange) → Penyelesaian
(`Tidak ada kas atau saldo customer.` bila kosong; Tanggal/Jenis/Metode/Referensi/Jumlah).
Gagal: toast `Gagal memuat detail retur`; kosong: `Retur/tukar tidak ditemukan` /
`Dokumen tidak tersedia pada cabang aktif.`

### F-04 — SJ Pengganti (seksi di detail; endpoint milik modul ini)

Hanya bila `return_mode=exchange` + status ≠ `not_required`. Status badge: `Tidak perlu`
(netral) / `Menunggu dikirim` (kuning) / `Dalam pengiriman` (info) / `Sudah diterima` (hijau).
`pending` → notice kuning (`Barang pengganti belum dikirim` / `Buat Surat Jalan Pengganti agar
barang dikirim ke pelanggan lewat supir. Stok baru berkurang saat surat jalan dibuat.`) + form
(sopir `mis. Budi` · plat `mis. B 1234 XX` · petugas · tanggal-kirim `date` default hari ini ·
catatan · tabel per pengganti-tracked: lokasi `Pilih lokasi`) → `[Buat Surat Jalan Pengganti]`
→ toast sukses (`Surat jalan pengganti dibuat` / `Stok barang pengganti sudah keluar dari
gudang.`) + reload. `dispatched/delivered` → kartu SJ (nomor/supir/kendaraan/tanggal/
dikirim/diterima/`Belum`/penerima) + (bila dispatched) konfirmasi (TTD select
Ditandatangani/Tidak + nama/alasan `mis. dititip satpam` → `[Konfirmasi Terima]`). Warning FE:
sopir/petugas/lokasi-per-item/nama/alasan kosong. Sukses confirm → toast
`Barang pengganti dikonfirmasi diterima pelanggan`.

### F-05 — Konteks & preview (API bantu)

`context` (order + `returnable_items` per baris: sudah-diretur, sisa, harga, tracked + lokasi
daun) dan `preview` (rencana penuh + settlement ter-resolve; dipakai modal; create memakai
hasil preview yang sama — tipe disimpan dari hasil preview, bukan pilihan mentah).

---

## 3. Edge Case (28)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Alasan kosong / tanpa baris | `400 'Alasan retur/tukar wajib diisi'` / `'Minimal satu barang yang dikembalikan harus dipilih'` (FE: tombol disabled + label kesiapan) |
| E-02 | Order tak dikenal / purchase / cabang-lain / belum-serah-belum-selesai / tanpa item | `404 'Order asal tidak ditemukan'` / `400 'Retur/tukar hanya berlaku untuk order penjualan'` / `404 'Cabang tidak sesuai company aktif'` / `400 'Order penjualan harus sudah selesai/diserahkan...'` / `'Order asal tidak memiliki item'` (FE: toast `Order tidak bisa diretur`) |
| E-03 | Baris duplikat / luar-order / qty ≤ 0 / over-sisa | `Item retur tidak boleh duplikat` / `Item yang dikembalikan tidak ditemukan pada order asal` / `{label} harus lebih dari 0` / `Qty retur '{n}' melebihi sisa yang bisa diretur. Sisa: {s} {u}` (sisa = order − retur-completed, toleransi; FE jepit 0…sisa) |
| E-04 | Kondisi tak dikenal | `400 'Kondisi barang retur wajib dipilih'` |
| E-05 | Tracked + normal tanpa lokasi | `400 'Lokasi masuk stok wajib dipilih untuk '{n}''` (FE: field lokasi kondisional; damaged/not_returned tanpa field) |
| E-06 | Produk pengganti tak dikenal / nonaktif / varian wajib-hilang / nonaktif / default-hilang | `404 'Produk pengganti tidak ditemukan'` / `400 '{n}' tidak aktif` / 3 varian (pola order) |
| E-07 | Harga pengganti < minimum | `400 'Harga jual pengganti '{n}' tidak boleh di bawah harga minimum'` (tanpa pengecualian member!) |
| E-08 | Harga pengganti negatif/bukan-angka | `{label} tidak valid` (FE `min=0`; `step=100`) |
| E-09 | Pengganti tracked tanpa stok (saat buat, non-tunda) | Dipilih otomatis (atau manual): kurang → `Stok pengganti tidak cukup di ...` (2 pesan); saat buat non-tunda lokasi wajib dipastikan |
| E-10 | Pengganti tanpa modal (avg 0 + beli 0) | `400 'Barang pengganti "{n}" belum punya harga modal. Isi Harga Beli produk, atau posting pembelian/saldo awal produk ini dulu, sebelum menyerahkan/mengirim barang pengganti.'` (saat buat non-tunda + saat dispatch tunda) |
| E-11 | Selisih + (mahal) bukan-collect | `400 'Selisih lebih mahal harus diselesaikan sebagai tambah bayar customer'` (+ UI notice tambah-bayar-sebelum-simpan) |
| E-12 | Selisih − (murah): potong piutang dulu sebesar sisa terutang, sisa-eksternal → refund/kredit otomatis (berpihak → kredit, walk-in → refund); tanpa sisa-eksternal → `reduce_receivable` | Paksa refund tanpa izin → `403 'Anda tidak memiliki izin untuk refund uang customer'`; paksa tipe non-otomatis padahal tak ada sisa-eksternal → `400 'Selisih retur ini mengurangi sisa piutang order, belum ada nilai yang bisa direfund atau dijadikan saldo customer'` |
| E-13 | Selisih ∓0.009 | `none` (tanpa baris settlement) |
| E-14 | `collect/refund` tanpa metode → tunai; referensi/catatan trim-or-null |
| E-15 | `reduce_receivable` tanpa baris settlement (implisit via selisih!) — tidak ada baris kas; efek via query finansial |
| E-16 | Collect/refund tanpa baris `payments` — kas dihitung di luar sistem lalu dicatat (preview: "tambah bayar sebelum disimpan"); piutang-efektif order langsung berkurang/tambah via efek (→ KI baru: kas tanpa baris bayar) |
| E-17 | Tukar POS fisik langsung keluar (+ varian wajib, lokasi, stok, modal-guard); tukar Order fisik ditunda (tanpa gerak, tanpa guard, lokasi null) |
| E-18 | Dispatch: bukan-completed (`Retur ini sudah dibatalkan`) / bukan-pending (`...tidak memiliki barang pengganti yang menunggu dikirim`) / lokasi-per-item wajib (`Lokasi pengambilan barang pengganti '{n}' wajib dipilih`) / stok-kurang / modal-nol |
| E-19 | Dispatch menulis lokasi + movement ke baris pengganti; status `dispatched`; SJ `replacement` + nomor-SJ; audit `dispatch_replacement` |
| E-20 | Confirm: status tak dikenal (`Status tanda tangan penerima wajib dipilih`); signed tanpa nama (`Nama penerima wajib diisi` — KETAT vs SJ order yang opsional!); not_signed tanpa alasan; ganda (`...sudah dikonfirmasi`) |
| E-21 | Confirm pengganti via endpoint SJ order | Ditolak di sana (modul 11 E-08); di sini tanpa TTD-arsip-wajib (beda SJ order!) |
| E-22 | Retur arsip/batal: tanpa endpoint (status `cancelled` ada di tipe + filter, tanpa penulis — kolom `cancelled_*` mati; → KI baru) |
| E-23 | Pajak pengganti mengikuti order (kena + tarif + termasuk) — tanpa pajak-baris input; retur proporsional rasio qty |
| E-24 | `return_date` tak-valid → label `Tanggal retur` (parse); kosong → kini |
| E-25 | Lokasi arsip/non-daun/non-aktif | `Lokasi stok tidak ditemukan` (404!) / `Lokasi stok harus aktif` / `Pilih lokasi stok paling bawah, bukan grup gudang` |
| E-26 | Lokasi RUSAK auto-buat (`RUSAK`/`Barang Rusak`/9999/`damaged`; paksa status bila ada) — tanpa UI pilih (otomatis saat kondisi rusak) |
| E-27 | Baris settlement hanya bila tipe riil + jumlah > 0 (tanggal = tanggal-retur; metode hanya collect/refund) |
| E-28 | Live FE vs server: proporsional-bulat vs rasio-uang-penuh (toleransi tampilan; final dari preview server) |

---

## 4. Katalog Pesan (teks apa adanya)

**Rencana:** `'Alasan retur/tukar wajib diisi'` · `'Minimal satu barang yang dikembalikan harus dipilih'` · `'Order asal tidak ditemukan'` · `'Retur/tukar hanya berlaku untuk order penjualan'` · `'Cabang tidak sesuai company aktif'` · `'Order penjualan harus sudah selesai/diserahkan sebelum bisa diretur atau ditukar'` · `'Order asal tidak memiliki item'` · `'Item retur tidak boleh duplikat'` · `'Item yang dikembalikan tidak ditemukan pada order asal'` · `'{label} harus lebih dari 0'` (qty) · `'{label} tidak valid'` (uang) · `"Qty retur '{n}' melebihi sisa yang bisa diretur. Sisa: {s} {u}"` · `'Kondisi barang retur wajib dipilih'` · `"Lokasi masuk stok wajib dipilih untuk '{n}'"` · `'Produk pengganti tidak ditemukan'` · `"'{n}' tidak aktif"` (produk) · `'Varian produk tidak ditemukan'` / `'Varian produk tidak aktif'` / `"Varian produk '{n}' wajib dipilih"` / `"Varian default produk '{n}' belum tersedia"` · `"Harga jual pengganti '{n}' tidak boleh di bawah harga minimum"` · `'Stok pengganti tidak cukup di ...'` (2 varian lokasi/cabang) · `'Stok pengganti '{n}' tidak cukup di lokasi {nama}'` (dispatch) · `"Barang pengganti \"{n}\" belum punya harga modal. ..."` · `'Selisih lebih mahal harus diselesaikan sebagai tambah bayar customer'` · `'Selisih lebih murah harus mengurangi piutang, refund, atau saldo customer'` · `'Saldo customer hanya bisa dipakai jika order memiliki data customer'` · `'Anda tidak memiliki izin untuk refund uang customer'` (403) · `'Selisih retur ini mengurangi sisa piutang order, belum ada nilai yang bisa direfund atau dijadikan saldo customer'` · `'Lokasi stok tidak ditemukan'` (404!) · `'Lokasi stok harus aktif'` · `'Pilih lokasi stok paling bawah, bukan grup gudang'` · `'Tanggal retur'` (label parse).

**Pengganti:** `'Nama supir wajib diisi'` · `'Nama petugas gudang wajib diisi'` · `'Retur ini sudah dibatalkan'` · `'Retur ini tidak memiliki barang pengganti yang menunggu dikirim'` · `"Lokasi pengambilan barang pengganti '{n}' wajib dipilih"` · `'Status tanda tangan penerima wajib dipilih'` · `'Nama penerima wajib diisi'` · `'Alasan tanda tangan penerima kosong wajib diisi'` · `'Surat jalan pengganti tidak ditemukan'` · `'Surat jalan pengganti ini sudah dikonfirmasi'`.

**UI:** `Order tidak bisa diretur` · `Gagal mencari order` · `Gagal memuat retur/tukar` · `Gagal memuat detail retur` · `Preview gagal` · `Preview belum dikonfirmasi` / `Klik Preview & Cek lalu periksa data di modal sebelum menyimpan.` · `Retur/tukar gagal dibuat` · `Pilih barang retur` / `Isi alasan retur` / `Siap dicek` · `Impas — tidak ada kas tambahan.` / `Customer perlu menambah {n}.` / `Customer menerima kembali {n}.` · `Nama supir wajib diisi` · `Nama petugas gudang wajib diisi` · `Pilih lokasi pengambilan untuk semua barang pengganti` · `Nama penerima wajib diisi` · `Alasan tanda tangan kosong wajib diisi` · `Surat jalan pengganti dibuat` / `Stok barang pengganti sudah keluar dari gudang.` · `Gagal membuat surat jalan pengganti` · `Barang pengganti dikonfirmasi diterima pelanggan` · `Gagal konfirmasi terima` · `Retur/tukar tidak ditemukan` / `Dokumen tidak tersedia pada cabang aktif.` · label kondisi (`Kembali ke stok` / `Barang rusak` / `Tidak kembali fisik`) + settlement (`Tidak ada selisih` / `Customer tambah bayar` / `Potong piutang order` / `Refund uang customer` / `Simpan sebagai saldo customer`) + status kirim (`Tidak perlu` / `Menunggu dikirim` / `Dalam pengiriman` / `Sudah diterima`).

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Edit/hapus/batal retur | Tanpa endpoint (kolom `cancelled_*` mati — E-22) |
| NF-02 | Baris `payments` untuk collect/refund | Hanya settlement (E-16 → KI baru) |
| NF-03 | Retur purchase / order-belum-serah | Guard jenis + tahap |
| NF-04 | Cicilan selisih | Satu keputusan per dokumen |
| NF-05 | Pajak-baris input pengganti | Mengikuti order |
| NF-06 | Arsip TTD wajib pengganti | Beda SJ order (E-21) |
| NF-07 | Lokasi untuk damaged/not_returned | Otomatis RUSAK / tanpa gerak |
| NF-08 | Restore/pulihkan | Tanpa endpoint (beda pihak) |
| NF-09 | Cetak khusus retur | `[Cetak]` = `window.print()` halaman |
| NF-10 | Validasi stok pengganti-tunda saat buat | Ditunda ke dispatch (disengaja; E2E 18 mengunci stok-tetap-5) |
