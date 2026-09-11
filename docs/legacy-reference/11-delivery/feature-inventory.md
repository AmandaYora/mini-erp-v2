# Feature Inventory — Modul 11 Delivery / Pengiriman

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode (`delivery.controller.ts`, `delivery.service.ts`, `delivery-proof.service.ts`, entity,
migrasi 013/014/037/046, halaman + komponen + hook web, E2E 06/18). Bagian ambigu ditandai
**[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = surat jalan order (`deliveries/*`: terbit, konfirmasi-kembali,
daftar, antrean, batal, bukti, URL bukti, cetak SJ). Serah langsung (`orders/deliver-goods`)
milik modul 09; SJ pengganti retur (`document_kind='replacement'`, endpoint
`sales-returns/replacement-deliveries/*`) milik modul 12 — di sini hanya sebagai batas
(konfirmasi SJ pengganti lewat endpoint khusus ditolak di sini). Mekanika order umum milik 08.

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 7 — `deliveries/{create,confirm,list,work-queue,archive,proof/upload,proof-url}` |
| Halaman | 2 rute: `/delivery-work-queue` (`order.view`, menu Operasional → Pantauan Surat Jalan), `/orders/:orderId/delivery/:sjId/print` (`authenticated` — tanpa permission!) + seksi Surat Jalan di detail order |
| Permission | `order.view` (daftar, antrean, URL bukti, cetak-daftar) + `order.update` (terbit, konfirmasi, batal, unggah bukti). COD `pay_now` saat konfirmasi menuntut `payment.create` (403 khusus). Cetak SJ: login saja |
| Tabel yang dimiliki | `delivery_notes`, `delivery_note_items`, `stock_issue_allocations` |
| Tabel yang ditulis | `inventory_balances` (−), `inventory_movements` (`out`, ref SJ), `payments` (COD konfirmasi), `orders` (tanggal + selesai auto), `order_status_history`, `branch_document_sequences` (nomor SJ), `media_files` (bukti) |
| Penomoran | `SJ-{KODECABANG}/{TAHUN}/{5 digit}` — tanpa reset. Lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | `delivery.create/confirm/archive/proof_uploaded`, `payment.create` (sumber `delivery_confirm_cod`), `order.approve_credit` (sumber sama, bila alih-tempo) |
| Aksi yang TIDAK ada | Edit SJ; SJ untuk purchase; konfirmasi SJ pengganti di sini (ditolak, lewat retur); hapus permanen; cetak massal; SJ tanpa jalur tahap (non-pos); serah tanpa SJ selain jalur warisan 09 |

**Catatan lingkup.** Alur kertas: admin menerbitkan + mencetak SJ → sopir membawa fisik →
fisik kembali → admin mengunggah arsip + konfirmasi. **Stok berkurang saat terbit, bukan saat
konfirmasi** (dikunci E2E 06: confirm tidak double-deduct). Konfirmasi menutup order hanya bila
semua item terpenuhi TERKONFIRMASI dan tak ada SJ aktif tersisa.

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Terbitkan SJ (`deliveries/create`, dialog di detail order)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Syarat tombol | `order.update` + sales + belum-serah + status active + tanpa transisi proses ke depan + bukan-prepaid-berutang + masih ada sisa (tombol di detail + seksi; shortcut `?action=create-sj` dengan syarat sama) |
| F-01.2 | Dialog | `Nama Sopir *` (`Nama sopir pengantar`) · `No. Plat Kendaraan` (`Contoh: B25PLT`, opsional) · `Nama Petugas Gudang *` (`Nama petugas gudang yang tanda tangan`) · `Tanggal Kirim *` (`date`, default hari ini) · `Catatan` (`Opsional`) · `Pilih Item yang Dibawa` (checkbox + qty `min=0.01 step=0.01 max=sisa`, dijepit UI; tanpa-sisa → `Semua item sudah dikirim sepenuhnya.`) · `[Batal]` + `[Terbitkan & Cetak]` (`Memproses...`; disabled tanpa item/tanpa-sisa/saat kirim) |
| F-01.3 | Validasi FE | Sopir kosong → `Nama sopir wajib diisi`; gudang kosong → `Nama petugas gudang wajib diisi`; tanpa item → `Pilih minimal satu item` / `Harus ada item yang dibawa di surat jalan ini.` (warning) |
| F-01.4 | Hasil | Audit `delivery.create`; dialog tutup + form reset + seksi refresh + **cetak otomatis tab baru** (`/orders/:id/delivery/:sjId/print`); gagal → toast `Gagal menerbitkan surat jalan` + pesan, dialog tetap terbuka |
| F-01.5 | Aturan server | Sales saja; non-terminal; ≥1 item; prabayar lunas-dulu; per baris: milik order, qty > 0, ≤ sisa-kirim + 0.0001 (`Item '{n}' melebihi sisa yang belum dikirim. Sisa: {s} {uom}`, `Qty item '{n}' harus lebih dari 0`, `Item dengan id {id} tidak ditemukan di order ini`, `Minimal satu item harus disertakan dalam surat jalan`); nomor SJ; stok keluar per alokasi (otomatis bila tak dikirim); audit |

### F-02 — Alokasi stok keluar (otomatis vs manual)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Otomatis (tanpa `allocations`) | Ambil dari saldo tersedia cabang per varian, lokasi aktif-daun saja, urut: area-petik → primer → default → tersedia-terbanyak → sortOrder → id; ambil berurutan sampai cukup; kurang → `Stok '{n}' tidak cukup di seluruh lokasi. Tersedia {t} {uom}.` |
| F-02.2 | Manual (`allocations` per baris) | Tiap lokasi: daun-aktif + tersedia & fisik cukup (`Stok '{n}' di {lokasi} tidak cukup. Tersedia {t} {uom}.`), tanpa duplikat lokasi (`Lokasi alokasi untuk '{n}' tidak boleh duplikat`), qty > 0 (`Quantity alokasi '{n}' harus lebih dari 0`); total wajib = qty kirim (`Total alokasi '{n}' harus sama dengan quantity yang dikirim`) |
| F-02.3 | Reservasi (`reservation_key`) | Bila dikirim: pakai reservasi aktif-tak-kedaluwarsa milik user itu; kurang/kedaluwarsa → `Reservation stok '{n}' sudah tidak cukup atau sudah expired`; sukses → reservasi `consumed` + id order. Tanpa kunci: abaikan |
| F-02.4 | Konsumsi | Per alokasi: saldo −= base (`available = on_hand − reserved`), movement `out` (ref SJ, alasan `Pengiriman dari SO {nomor}`, metadata SJ+baris), baris `stock_issue_allocations` tersimpan; non-tracked: baris SJ saja |

### F-03 — Konfirmasi SJ kembali (`deliveries/confirm`, modal di seksi)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Prasyarat | SJ ada + cabang + non-arsip; belum-pernah-confirm (`Surat jalan ini sudah dikonfirmasi sebelumnya`); status TTD ∈ signed/not_signed (`Status tanda tangan penerima wajib dipilih`); arsip SJ fisik sudah ada ≥1 (`Foto atau scan surat jalan fisik wajib diunggah sebelum konfirmasi`); not_signed wajib alasan (`Alasan tanda tangan penerima kosong wajib diisi`); pengganti → ditolak (`Surat jalan pengganti dikonfirmasi lewat menu Retur/Tukar`) |
| F-03.2 | Modal (badge `SJ Kembali`, kunci-tutup saat proses) | Notice info `{nomor}` / `Upload foto atau scan surat jalan fisik yang kembali dari sopir.` · Arsip SJ Fisik (wajib kecuali sudah tersimpan: `Arsip SJ sudah tersimpan. Upload file baru hanya jika perlu menambah bukti.`; JPG/PNG/WebP 5MB; `{nama} - akan dikompres sebelum disimpan`) · Status TTD* (radio `Ada TTD penerima` / `Tidak ada TTD penerima`) · Nama Penerima (`Opsional`) vs Alasan* (`Contoh: penerima tidak ada, barang ditaruh di teras sesuai instruksi customer.`) · Foto Lokasi opsional · Catatan Lokasi (`Opsional`) · `[Batal]` + `[Konfirmasi Selesai]` (`Memproses...`) |
| F-03.3 | Validasi FE | Tanpa arsip-baru & tanpa arsip-lama → `Arsip SJ wajib diunggah` / `Foto atau scan surat jalan fisik yang kembali wajib dilampirkan.`; not_signed tanpa alasan → `Alasan wajib diisi` / `Isi alasan kenapa kolom tanda tangan penerima kosong.` (warning) |
| F-03.4 | Hasil | File diunggah dulu (tandatangan lalu lokasi), konfirmasi, seksi refresh; toast `Surat jalan dikonfirmasi` + (`Semua pengiriman selesai — order otomatis ditutup.` bila auto-close else `Surat jalan berhasil dikonfirmasi.`); gagal → hook toast + reload |
| F-03.5 | Auto-close | Bila seluruh item terpenuhi TERKONFIRMASI + tanpa SJ aktif: butuh status-selesai + transisi aktif (`Order belum berada pada tahap yang bisa diselesaikan.`) → finansial COD/prepaid (prepaid berutang ditolak: `...harus lunas sebelum barang dapat diserahkan sepenuhnya.`; COD berutang wajib mode) → `pay_now` (403 khusus konfirmasi) / `switch_to_net` (tempo + approve + audit sumber-konfirmasi) → tanggal = tanggal-kirim-SJ else kini + selesai + history `Order otomatis diselesaikan setelah semua surat jalan dikonfirmasi` + audit confirm (kaya: autoClose, settlement, status TTD, hitungan bukti, flag nama/alasan/catatan) |
| F-03.6 | Shortcut | `?action=confirm-sj&delivery_note_id=` membuka modal SJ aktif itu (sekali, lalu param dibersihkan `replace`) |

### F-04 — Batalkan SJ (`deliveries/archive`)

Tombol `Batalkan` merah (SJ aktif + `order.update`) → dialog `Batalkan Surat Jalan?` / `Surat
jalan ini akan dibatalkan dan stok barang yang sudah dikurangi akan dikembalikan ke gudang.` /
`Ya, Batalkan SJ` → stok kembali per alokasi (+ movement `in` alasan `Pembatalan SJ {sj} dari SO
{order}`, metadata `cancellation`); tanpa alokasi → kembalikan ke lokasi default (bila ada;
tanpa saldo → lewati diam-diam); SJ terkonfirmasi ditolak (`...sudah dikonfirmasi tidak bisa
dibatalkan`); audit `delivery.archive`; toast `Surat jalan dibatalkan` / `SJ berhasil dibatalkan
dan stok dikembalikan.` (info) ; gagal → toast + dialog tetap (retry). E2E 06 mengunci
kembali-ke-semula.

### F-05 — Bukti SJ (`proof/upload`, `proof-url`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-05.1 | Unggah (multipart `order.update`) | Field `id_delivery_note` (tak-valid → `Surat jalan tidak valid`) + `purpose` (wajib salah satu: `Jenis bukti surat jalan tidak valid`) + `file` (wajib: `File bukti surat jalan wajib diunggah`; 5MB; optimasi profil delivery) → `media_files` (`delivery_note`, private, pertama primer + `sortOrder` menaik, metadata kompresi) + audit `delivery.proof_uploaded` |
| F-05.2 | Dua purpose | `delivery_signed_document` (Arsip SJ — wajib sebelum konfirmasi) vs `delivery_location_photo` (Foto Lokasi — opsional kapan pun) |
| F-05.3 | URL (`order.view`) | By id media + perusahaan + purpose-di-dua-itu + SJ ada + cabang + non-arsip → `{url, expires_in}`; else `404 'Bukti surat jalan tidak ditemukan'` |
| F-05.4 | Lihat di UI | Tombol `Bukti` (bila ada) → modal `Bukti Surat Jalan` (badge hijau; nomor; notice kuning alasan-tanpa-TTD; catatan drop; tombol per bukti `Arsip SJ`/`Foto Lokasi`; pratinjau `Memuat bukti...` → gambar `Bukti surat jalan`) |

### F-06 — Daftar & antrean

| # | Sub-fitur | Detail |
|---|---|---|
| F-06.1 | `deliveries/list` (per order) | Array mentah (bukan `{items}`!): SJ + item (nama/kode/varian dari baris order, qty, UOM) + alokasi (lokasi, qty) + bukti (tanpa URL!) + pengirim + `status: active|confirmed`; urut dibuat-ASC. Dipakai seksi + cetak SJ |
| F-06.2 | Work queue (`order.view`, tanpa audit) | `{create_sj, waiting_return, limit}`; limit 1–200 default 100; cari (SJ/order/customer); tanggal (dari + sampai-akhir-hari — beda dengan `orders/list`!); umur (hari-ini/>2/>7 dari tanggal-kirim); `create_sj`: sales non-pos + belum-serah + active-non-terminal + (non-prepaid / lunas ±0.01) + tanpa-transisi-proses-ke-depan + sisa > 0 → urut order-terlama (payment-due hanya info); `waiting_return`: SJ aktif non-arsip (sales non-pos) → urut kirim-terlama |
| F-06.3 | Halaman Pantauan | Header + 2 tab berhitung (Buat SJ / Menunggu SJ Kembali) + filter Cari (`No order, no SJ, atau customer`, debounce 350ms) / Dari / Sampai / Umur (Semua/Hari ini/>2/>7) + `Reset filter` (menyisakan tab) + error `Gagal memuat pantauan surat jalan. Coba ulangi atau periksa koneksi.` (notice `Pantauan tidak dapat dimuat`) + `Memuat pantauan surat jalan...`; tabel Buat (Order+customer / Tanggal + `Jatuh tempo pembayaran` / Status kuning / Progress `{kirim}/{total} item dikirim` + `Sisa {n} item / {q} qty` + `Ada SJ menunggu kembali` / Termin / `[Buat SJ]`(kecil, `order.update` → `?action=create-sj`) + `[Lihat Order]`) · tabel Tunggu (SJ+order / Customer / Kirim + dispatch / Umur (merah >7 else kuning: `Hari ini`/`{n} hari`) / Sopir (+plat) / Arsip (`Arsip tersimpan` hijau / `Belum ada arsip SJ` kuning) / `[Cetak SJ]` + `[Konfirmasi]` + `[Lihat Order]`); kosong masing-masing (`Tidak ada order yang perlu dibuatkan SJ` / `...sudah dibuatkan SJ atau belum siap dikirim.` / `Tidak ada SJ yang menunggu kembali` / `...sudah dikonfirmasi atau belum diterbitkan.`) |

### F-07 — Seksi Surat Jalan (di detail order sales active/completed)

Kartu `Surat Jalan` / `Daftar pengiriman dan konfirmasi serah barang ke customer.` + tombol
`Terbitkan Surat Jalan` (secondary; non-terminal + kelola + sisa; tanpa sisa → hilang) →
notice kuning `Semua barang sedang dalam pengiriman` / `Tunggu sopir membawa kembali surat jalan
fisik, lalu unggah arsip SJ sebelum konfirmasi.` (bila habis-tapi-aktif) → `Memuat...` / kosong
(`Belum ada pengiriman` / `Terbitkan surat jalan pertama untuk memulai proses pengiriman ke
customer.` + tombol) / tabel (No. SJ + tanggal-kirim · Sopir (+plat) · Petugas Gudang · Item
Dibawa (`{nama[-varian]}: {qty} {uom}` + alokasi `(lokasi: qty, ...)` bila ada) · Dikirim
(datetime) · Status (`Aktif` kuning / `Selesai - Tanpa TTD` kuning / `Selesai - TTD Penerima`
hijau / `Selesai` hijau + `{n} bukti tersimpan`) · Aksi (`[Konfirmasi Kembali]` primer +
`[Bukti]` + `[Cetak]` tab-baru + `[Batalkan]` merah; aktif + kelola)) + modal konfirmasi +
modal bukti + dialog batal (§F-03/04/05).

### F-08 — Cetak SJ (`/orders/:orderId/delivery/:sjId/print`, login saja)

Toolbar: A4 / Kontinu 3-Ply + preprinted (kontinu) + Notes-hanya-cetak + Cetak/Tutup. Isi:
kop + `SURAT JALAN` + nomor; pihak (Nama, Penerima bila beda, No HP, Alamat — snapshot
diutamakan); Tanggal Kirim / No Nota / Tanggal Nota / Staff (pembuat) / Kendaraan (bila ada);
tabel No/Qty/Satuan/Produk/Keterangan (kosong!); `STATUS : {Lunas|Bayar Sebagian|Bayar Saat
Serah (COD)|Tempo|Belum Lunas}` (dari status+termin order, bukan statis); Catatan SJ (bila ada);
kota + 3 kotak tanda (Petugas Gudang {nama} / Sopir {nama} / Pembeli {nama}); footbar
`Dicetak: {waktu}` + `PUTIH : SOPIR   MERAH : PELANGGAN` (keputusan bisnis terkunci, bukan bug)
+ `USER : {pencetak}`. A4 landscape + ganjal 5 baris; kontinu tanpa ganjal. Tanpa SJ →
`Surat jalan tidak ditemukan.`; loading `Memuat...`.

---

## 3. Edge Case (26)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Bukan sales / terminal / tanpa item / prabayar-berutang | 4 pesan terbit (§F-01.5) |
| E-02 | Baris luar-order / qty ≤ 0 / over-sisa | 3 pesan baris (§F-01.5; sisa = order − dispatch-non-arsip, toleransi) |
| E-03 | Stok kurang (otomatis maupun manual) | 2 pesan + angka id-ID (§F-02) |
| E-04 | Lokasi duplikat / alokasi-0 / total ≠ kirim | 3 pesan manual (§F-02.2) |
| E-05 | Reservasi cukup → pakai + consumed; kurang/kedaluwarsa/salah-user | Konsumsi vs `...sudah tidak cukup atau sudah expired` (+ `...tidak valid` tanpa saldo) |
| E-06 | Non-tracked di SJ | Baris SJ tanpa alokasi/movement (jasa ikut terkirim di kertas) |
| E-07 | Confirm ganda / SJ hilang / arsip-tanpa-SJ / TTD-tak-dipilih / tanpa-alasan | 5 pesan (§F-03.1) |
| E-08 | Pengganti via endpoint ini | `400 Surat jalan pengganti dikonfirmasi lewat menu Retur/Tukar` |
| E-09 | Confirm tanpa SJ-aktif-lain + semua-terpenuhi → auto-close (+ finansial COD/prepaid, transisi, 403-bayar) | Rantai §F-03.5; E2E 06 mengunci `auto_closed_order` true/false |
| E-10 | Confirm dengan SJ-aktif-lain tersisa / item-belum-penuh | Tanpa auto-close (bagian terpenuhi parsial; E2E 06 `toBeFalsy`) |
| E-11 | Batal terkonfirmasi / hilang | 2 pesan; batal → stok kembali (+ movement `in` alasan-pembatalan; tanpa-alokasi → default bila ada + bersaldo, else lewati) |
| E-12 | Bukti: tanpa file / SJ-id tak-valid / purpose salah / SJ hilang / URL untuk purpose-lain | 5 pesan (§F-05); unggah ganda = baris baru (sortOrder naik; primer tetap pertama) |
| E-13 | Antrean: POS dikecualikan dua-duanya; retur-pengganti tak masuk (JOIN order); prepaid lunas ±0.01; tanpa-transisi-proses; sisa > 0 (HAVING); limit 1–200 | Filter §F-06.2 |
| E-14 | Tanggal antrean: `date_to` panjang-10 → akhir-hari (beda `orders/list`!); umur dari tanggal-kirim (fallback dispatch); `today/gt_2/gt_7` hanya waiting | Inkonsistensi terdokumentasi (→ KI baru) |
| E-15 | Cetak SJ id tak ada di daftar / order gagal | `Surat jalan tidak ditemukan.` (SJ dari daftar SJ, info dari detail/store) |
| E-16 | Shortcut `action=` tak-syarat / sudah-dipakai / SJ tak-aktif | Diam (tanpa error, param dibersihkan hanya bila dibuka) |
| E-17 | Tombol Terbitkan hilang tanpa sisa (seksi) vs dialog butuh item (detail) | Dua lapis konsisten (sisa > 0) |
| E-18 | Sopir/gudang teks bebas (bukan user) | Disengaja (pekerja kertas; knowledge COMMERCE) |
| E-19 | `delivery_date` = tanggal (tanpa jam); `dispatched_at` = kini; cetak pakai kirim; umur pakai kirim | Dua waktu beda peran |
| E-20 | SJ parsial 1/10 lalu batal → sisa kembali 10/10 (arsip non-arsip dikecualikan dispatch) | E2E 06 mengunci |
| E-21 | Duplikat lokasi manual, alokasi total≠kirim, qty-0 | Pesan §F-02.2 (tanpa FE khusus — API) |
| E-22 | `vehicle_plate`/`notes` kosong → null; penerima kosong + signed → null (nama opsional saat TTD ada!) |
| E-23 | `drop_location_note` selalu opsional; tampil di modal bukti, tidak di cetak SJ (terverifikasi: render cetak tanpa field ini) |
| E-24 | Status list `active|confirmed` dihitung (`confirmedAt?`), bukan kolom |
| E-25 | Cetak tanpa `order.view` (login saja) — beda dengan nota/bukti-bayar (→ KI baru) |
| E-26 | Pengganti via modul 12 (E2E 18: pending → dispatched → delivered, bernomor SJ normal). Karena `id_order` NULL, SJ pengganti **tidak tampil** di `deliveries/list` maupun cetak SJ order (filter `id_order` tak cocok NULL) — dikelola dari halaman retur |

---

## 4. Katalog Pesan (teks apa adanya)

**Terbit:** `'Surat jalan hanya berlaku untuk sales order'` · `'Order sudah selesai, tidak bisa menerbitkan surat jalan baru'` · `'Minimal satu item harus disertakan dalam surat jalan'` · `'Sales order prabayar harus lunas sebelum surat jalan diterbitkan.'` · `` `Item dengan id ${id} tidak ditemukan di order ini` `` · `` `Qty item '${n}' harus lebih dari 0` `` · `` `Item '${n}' melebihi sisa yang belum dikirim. Sisa: ${s} ${u}` `` · `'Nama sopir wajib diisi'` · `'Nama petugas gudang wajib diisi'` · `'Pilih minimal satu item'` / `'Harus ada item yang dibawa di surat jalan ini.'` · `'Gagal menerbitkan surat jalan'` + pesan.

**Alokasi:** `` `Lokasi alokasi untuk '${n}' tidak boleh duplikat` `` · `` `Quantity alokasi '${n}' harus lebih dari 0` `` · `` `Stok '${n}' di ${lokasi} tidak cukup. Tersedia ${t} ${u}.` `` · `` `Total alokasi '${n}' harus sama dengan quantity yang dikirim` `` · `` `Stok '${n}' tidak cukup di seluruh lokasi. Tersedia ${t} ${u}.` `` · `` `Reservation stok '${n}' sudah tidak cukup atau sudah expired` `` · `` `Reservation stok '${n}' tidak valid` ``.

**Konfirmasi:** `'Surat jalan ini sudah dikonfirmasi sebelumnya'` · `'Surat jalan tidak ditemukan'` · `'Status tanda tangan penerima wajib dipilih'` · `'Foto atau scan surat jalan fisik wajib diunggah sebelum konfirmasi'` · `'Alasan tanda tangan penerima kosong wajib diisi'` · `'Surat jalan pengganti dikonfirmasi lewat menu Retur/Tukar'` · `'Order terkait tidak ditemukan'` · `'Order belum berada pada tahap yang bisa diselesaikan.'` · `'Status selesai untuk sales order belum dikonfigurasi'` · `'Sales order prabayar harus lunas sebelum barang dapat diserahkan sepenuhnya.'` · `'Sales order COD harus memilih bayar saat serah atau diubah ke tempo sebelum selesai.'` · `'Anda tidak memiliki izin untuk mencatat pembayaran customer saat konfirmasi SJ.'` (403) · `'Arsip SJ wajib diunggah'` / `'Foto atau scan surat jalan fisik yang kembali wajib dilampirkan.'` · `'Alasan wajib diisi'` / `'Isi alasan kenapa kolom tanda tangan penerima kosong.'` · `'Surat jalan dikonfirmasi'` + (`Semua pengiriman selesai — order otomatis ditutup.` / `Surat jalan berhasil dikonfirmasi.`) · `'Gagal mengkonfirmasi surat jalan'` + pesan.

**Batal/bukti/antrean:** `'Surat jalan yang sudah dikonfirmasi tidak bisa dibatalkan'` · `'Batalkan Surat Jalan?'` / `'Surat jalan ini akan dibatalkan dan stok barang yang sudah dikurangi akan dikembalikan ke gudang.'` / `'Ya, Batalkan SJ'` · `'Surat jalan dibatalkan'` / `'SJ berhasil dibatalkan dan stok dikembalikan.'` (info) · `'Gagal membatalkan surat jalan'` + pesan · `'File bukti surat jalan wajib diunggah'` · `'Surat jalan tidak valid'` · `'Jenis bukti surat jalan tidak valid'` · `'Bukti surat jalan tidak ditemukan'` · `'Gagal memuat pantauan surat jalan. Coba ulangi atau periksa koneksi.'` / `'Pantauan tidak dapat dimuat'`.

**Cetak:** `SURAT JALAN` · `PUTIH : SOPIR   MERAH : PELANGGAN` · `Memuat...` / `Surat jalan tidak ditemukan.` · `Kertas A4 (biasa)` / `Kontinu 3-Ply (Dot-Matrix)` / `Kop/rekening sudah preprinted (sembunyikan)` · `Notes (opsional, hanya tampil di hasil cetak — tidak disimpan):` · `Cetak / Print` / `Tutup`.

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Edit SJ | Tanpa endpoint (batal + terbit ulang) |
| NF-02 | SJ purchase | `Surat jalan hanya berlaku untuk sales order` |
| NF-03 | Konfirmasi pengganti di sini | Ditolak eksplisit → modul 12 |
| NF-04 | Hapus permanen SJ | Arsip + stok-kembali |
| NF-05 | URL bukti di list | Tanpa URL (ambil per-file via proof-url) |
| NF-06 | Audit antrean | Read-only tanpa log |
| NF-07 | Cetak massal SJ | Per SJ per tab |
| NF-08 | Cek transisi proses di `create` | TIDAK ADA — create hanya menolak terminal + prabayar-berutang. Syarat "tanpa transisi proses ke depan" hanya di tombol UI + antrean, bukan API (→ KI baru: API lebih longgar) |
| NF-09 | Blokir terbit prepaid-lunas-sebagian? | Lunas-penuh (±0, E2E) — bukan sebagian |
| NF-10 | `deliveries/list` envelope `{items}` | Array mentah (hook `res ?? []`) |
