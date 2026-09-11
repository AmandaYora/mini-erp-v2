# Feature Inventory — Modul 09 Order Sales

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = SO penjualan (buat/ubah/daftar/detail/arsip/pindah status) +
serah-terima langsung (`orders/deliver-goods`) + persetujuan tempo (`orders/approve-credit`) +
seksi pembayaran di detail order + dokumen cetak penjualan (nota/tagihan, bukti bayar).
Mekanika bersama (list/detail/update/status-config/guard tiga-lapis/rumus baris) milik modul 08
dan hanya dirujuk; yang didokumentasikan di sini adalah **perilaku sisi sales**. Yang **bukan**
bagian modul ini: SJ/surat-jalan flow (modul 11), POS (modul 15), retur (modul 12), CRUD
pembayaran & saldo pihak (modul 14 — di sini hanya tampilan seksi + dialog tempo).

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint dalam lingkup | `orders/{list,detail,create,update,update-status,archive,status-history}` (pakai sales), `orders/deliver-goods`, `orders/approve-credit` (+ baca `payments/{list}` di seksi bayar — tulis milik 14) |
| Halaman | Bersama purchase: `/orders` (tab Penjualan = default), `/orders/create`, `/orders/:orderId`, `/orders/:orderId/edit` + cetak `/orders/:orderId/{sales-document,pos-receipt}/print`, `/orders/:orderId/payments/:paymentId/print` |
| Menu | Sama: Operasional → Order |
| Permission | `order.*` (lihat/buat/ubah/arsip/export — modul 08) + `payment.create` (catat bayar; 403 khusus di COD-serah) + `payment.archive` (hapus bayar; disembunyikan bila terminal + non-net) |
| Tabel yang ditulis | `orders`/`order_items` (sisi sales + snapshot member + ship-to), `inventory_*` (keluar saat serah), `payments` (COD `pay_now` + `amount_tendered`), status/history/audit |
| Penomoran | `ORD-{KODECABANG}/PJ/YYYY/MM/00001` — lihat [numbering-sequence.md](numbering-sequence.md) |
| Audit log | `order.create/update/status_change/archive/goods_delivered/approve_credit`, `payment.create` (sumber `deliver_goods_cod`) |
| Aksi yang TIDAK ada | Hapus permanen; serah parsial (sekali tembak); edit perlindungan sama dengan purchase (guard tiga-lapis); tempo tanpa jatuh tempo; member untuk non-sales |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Daftar, tab Penjualan (default)

Tab `Penjualan` bila param tak dikenal; kolom Pihak = **Customer**; Termin = `Prabayar`/`Bayar Saat
Serah`/`Tempo`; badge sama dengan purchase. Buat Order Baru → form default Penjualan. Selebihnya
= modul 08 (filter, export, paginasi, badge Diretur mati — E-30 modul 08).

### F-02 — Buat / Edit SO (aspek sales)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Customer opsional vs wajib | Non-tempo: boleh kosong (walk-in/tunai); tempo: wajib customer + wajib No. HP (`Penjualan tempo wajib memilih customer.` / `...wajib mencatat nomor HP customer.`) |
| F-02.2 | Tipe pihak | Bila diisi harus customer (`Pihak terkait untuk penjualan tempo harus bertipe customer, bukan {t}.` — teks menyebut tempo walau berlaku umum, → KI baru); arsip/tak dikenal → `Customer tidak ditemukan atau sudah diarsipkan.` |
| F-02.3 | Shortcut Tambah Pelanggan | Tombol di samping label (sales + `order.create`): modal kode/nama*/telepon/email/alamat/catatan/member → validasi `Nama pelanggan wajib diisi.` → simpan → langsung terpilih + requote (error → pesan inline `Pelanggan gagal disimpan.`) |
| F-02.4 | Alamat kirim | `AddressPicker` (milik 06): utama otomatis, ad-hoc, tambah inline, walk-in opsional; submit hanya untuk sales |
| F-02.5 | Harga saran jual | `sellingPrice ?? purchasePrice ?? minSellingPrice ?? 0` + helper `Harga jual referensi produk: {min} - {ref} / {uom}` (atau satu nilai); satuan & faktor sisi JUAL |
| F-02.6 | Diskon baris (sales saja) | Kolom `Diskon (Rp)` = TOTAL per baris (bukan per unit); diklem `clampLineDiscountTotal` (batas `(penuh − floor) × qty`, floor = 0 untuk member else minimum; total tak-habis-bagi dibulatkan per unit — mis. 1000/3 → 333 → efektif 999); submit mengirim per-unit; ringkasan menampilkan `Diskon -Rp X` bila > 0 |
| F-02.7 | Quote member reaktif | Ganti customer → requote (`pricing/quote`): baris non-manual ditimpa + tandai `member_rule`; baris `priceEdited` lestari; diskon diklem ulang; error per produk → banner (maks 2 + `dan {n} produk lain. Periksa harga master produk atau ubah customer.`); quote loading/error memblokir submit (khusus sales + ada customer) |
| F-02.8 | Guard harga minimum | Server menolak harga bersih < minimum (non-member) dengan kedua nominal id-ID; FE: saran + klem + tombol POS `Harga di bawah minimum` |
| F-02.9 | Edit SO | Baris lama dimuat harga-penuh + `priceEdited` (anti-diskon-ganda); ganti jenis/termin/pihak divalidasi ulang; guard gerak/bayar/finance sama dengan purchase |

### F-03 — Detail SO

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Aksi header | `[← Kembali]` · `[Edit Order]` (`order.update`, non-terminal) · `[Cetak Tagihan]` (sisa > 0) / `[Cetak Nota]` (lunas; tab baru) · `[Retur / Tukar]` (`sales_return.create` + sudah-serah/selesai → `/sales-returns/create?order_id=`) · `[Terbitkan Surat Jalan]` (primer; syarat modul 11) · pindah status (label transisi; selesai disembunyikan pre-serah) |
| F-03.2 | Seksi Pembayaran (komponen, data milik 14) | Kartu `Pembayaran` / `Riwayat pembayaran dan posisi finansial order ini.`: badge Termin · `[Ubah ke Tempo]` (ghost; syarat §F-05) · `[+ Catat Pembayaran]` (secondary; `payment.create` + form tutup + sisa > 0); 4 kotak: Total Tagihan / Sudah Dibayar Customer / Sisa Piutang (merah/hijau) / Status Finansial; kosong → `Belum ada transaksi pembayaran` / `Belum ada pembayaran dari customer untuk order ini.` |
| F-03.3 | Form Catat Pembayaran | `Catat Pembayaran Baru`: Tanggal Bayar* (default kini) · Jumlah (Rp)* (bulat saja — pecahan dibuang: `12017,40`→`12017`; placeholder = sisa) · Metode (Tunai/Transfer/Cek-Giro) · No. Referensi (auto `REF-YYYYMMDD-XXXX`, bisa diubah) · Catatan · Bukti (JPG/PNG 5MB, kompres; gagal tak membatalkan) · `[Batal]`/`[Simpan Pembayaran]`(`Menyimpan...`); sukses → toast `Pembayaran berhasil dicatat` |
| F-03.4 | Tabel bayar | No. Bayar · Tanggal · Metode (badge) · No. Referensi (`-`) · Jumlah (kanan tebal) · Bukti (`Lihat` / `—`) · Aksi (`Cetak` tab baru · `Edit` = ganti bukti (`payment.create`) · `Hapus` merah (`payment.archive`, disembunyikan bila terminal + non-net)). Lihat bukti: overlay gelap + `✕`/`Tutup` + Escape. Hapus: `Hapus pembayaran?` / `Catatan pembayaran ini akan dihapus. Tindakan ini tidak bisa dibatalkan.` / `Hapus Pembayaran` → toast `Pembayaran dihapus` |
| F-03.5 | Riwayat Status | Timeline: label tujuan (+`dari {asal}`) · waktu + `oleh {nama} ( @{username})` · alasan |
| F-03.6 | Terima-tombol-tak-ada | SO tidak punya Terima Barang; penyelesaiannya via serah (langsung) atau SJ (modul 11) |

### F-04 — Serah langsung (`orders/deliver-goods`, warisan cepat)

Sekali tembak (tanpa parsial): syarat sales + belum-pernah-serah + non-terminal + punya item +
status-selesai terkonfigurasi (+ transisi aktif, kecuali source `pos`) → kurangi stok tracked per
baris (alokasi + reservasi bila dikirim) → `goodsDeliveredAt` = eksplisit ?? tanggal-bayar-COD ??
kini → selesai + history `Order otomatis diselesaikan saat serah barang` + audit
`order.goods_delivered`. Prabayar wajib lunas (`Sales order prabayar harus lunas sebelum barang
diserahkan.`); COD + sisa + tanpa mode → wajib pilih (`...harus memilih bayar saat serah atau
diubah ke tempo sebelum selesai.`); `pay_now` butuh `payment.create` (403 khusus customer),
tunai tercatat + `amount_tendered` (bulat, wajib ≥ tagihan: `Uang tunai yang diterima tidak boleh
kurang dari total tagihan.`); `switch_to_net` = termin + tempo + approve (audit
`order.approve_credit`). Respons cermin receive (tanpa `fully_received`; `items_delivered`).

### F-05 — Ubah ke Tempo (`orders/approve-credit` + dialog)

Tombol ghost di seksi bayar (syarat: `order.update` + non-purchase + non-terminal + non-net) →
modal badge Tempo `Ubah ke Tempo`: `Jatuh Tempo *` (`datetime-local`, default +30 hari) →
`[Batal]`/`[Ubah ke Tempo]` (disabled tanpa tanggal/saat simpan) → termin net + tempo + approve +
audit → toast `Termin diubah ke tempo` / `Order dapat diselesaikan dengan jatuh tempo yang
tercatat.`; gagal → toast `Gagal` + pesan. Server: purchase ditolak (`Purchase order tidak
memerlukan persetujuan kredit`); sudah-net ditolak (`...sudah menggunakan termin pembayaran
tempo`); terminal ditolak; tempo wajib (`Jatuh tempo wajib diisi untuk termin tempo.`).

### F-06 — Dokumen cetak penjualan

| Dokumen | Rute (tab baru, `order.view`) | Isi |
|---|---|---|
| Nota/Tagihan | `sales-document/print` (+`?discount_as_price=1`, `?paper=`, `?hide_letterhead=`) | Judul dinamis (`Nota Penjualan Lunas` / `Tagihan - Bayar Sebagian` / `Tagihan Penjualan`); kop; pihak (`Pelanggan umum` bila kosong; alamat kirim snapshot else kontak); No Nota/Tanggal/`Tgl. Jtp` (`-` bila lunas/tanpa)/Staff (pembuat)/Faktur Pajak (bila ada); tabel No/Qty/Satuan/Produk (+catatan)/Harga Satuan (penuh)/[Disc]/Harga Jual; ringkasan terbilang + Jumlah (kotor) + Diskon (bila ada & tampil) + Pajak (bila >0) + Total + Sudah Dibayar (bila >0) + Sisa Tagihan; footer notes-cetak + Pembeli `(................)` + MENJUAL + `Dicetak: {waktu} USER : {pencetak}`. Toolbar: kertas A4/Kontinu 3-Ply, preprinted (kontinu saja), sembunyikan-diskon (bila ada diskon), Notes прове-cetak (tidak disimpan), Cetak/Tutup. A4: ganjal 6 baris; kontinu: tanpa ganjal |
| Mode sembunyi-diskon | `?discount_as_price=1` | Harga penuh, kolom Disc `-`, jumlah/bayar naik ke bruto, sisa tetap riil; hemat member dilipat (basis > eksplisit dipakai sebagai harga penuh) |
| Bukti Bayar | `payments/:paymentId/print` | Satu bayar + metode/referensi + order + sisa sesudah bayar itu; tanpa kolom tanda tangan (validasi = nomor + referensi/bukti + audit) |
| Struk POS | `pos-receipt/print` | Thermal 80mm browser-print (milik 15; baca data sama) |

---

## 3. Edge Case (24)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Sales non-net tanpa customer | Lolos (walk-in); net wajib customer + HP (2 pesan tempo) |
| E-02 | Pihak supplier di sales | `400` (teks menyebut "tempo" walau berlaku umum — → KI baru) |
| E-03 | Harga bersih < minimum (non-member) | `400` dua-nominal id-ID (create + update) |
| E-04 | Member di bawah minimum | Lolos (disengaja); floor diskon = harga member |
| E-05 | Diskon > harga / persen >100 | `400` (`Diskon produk ... tidak boleh melebihi harga satuan.` / persen 0–100) |
| E-06 | Quote error per produk | Banner agregat; submit diblokir (sales + customer + loading/error) |
| E-07 | Selesai pre-serah (manual) | Ditolak dua lapis (`...tidak boleh langsung selesai tanpa proses serah barang.` + `...harus diserahkan terlebih dahulu...`); POS dikecualikan lapis-1 tetapi TETAP tertahan lapis-2 tanpa serah (→ KI baru: pengecualian tampak mati) |
| E-08 | Sudah-serah ke non-selesai | `...hanya dapat dilanjutkan ke status selesai.` |
| E-09 | Aktifkan prabayar belum lunas | `Sales order prabayar harus lunas sebelum dapat diproses.` (detail: seksi bayar tampil sejak Draft khusus prepaid prabayar) |
| E-10 | Selesai-terminal prabayar/COD berutang | `...prabayar harus lunas sebelum diselesaikan.` / `...bayar saat serah...` (net bebas) |
| E-11 | Serah: bukan-sales / sudah-pernah / terminal / tanpa item / tanpa status-selesai / tanpa tahap (non-pos) | 6 pesan §F-04 |
| E-12 | Serah: tunai kurang | `400 Uang tunai yang diterima tidak boleh kurang dari total tagihan.` (non-tunai/kosong = null, tanpa kembalian) |
| E-13 | Serah: tanggal eksplisit vs bayar-COD vs kini | Prioritas eksplisit → tanggal-bayar → kini (dashboard net-sales memakai tanggal ini) |
| E-14 | Serah memakai reservasi | Konsumsi kunci reservasi aktif milik user → `consumed` + id order |
| E-15 | Approve: purchase / sudah-net / terminal / tanpa tempo | 4 pesan §F-05 |
| E-16 | Hapus bayar terminal-non-net | Tombol disembunyikan (backend juga menolak — milik 14) |
| E-17 | Jumlah bayar pecahan (`12017,40`) | Dibuang jadi `12017` di input (bukan 1201740) — komentar kode eksplisit |
| E-18 | Referensi auto `REF-...` | Dibuat per buka form; bisa diubah; tanpa unik-cek di FE |
| E-19 | Bukti bayar gagal | Pembayaran tetap sah (3 tempat: form, edit-bukti, terima-COD) |
| E-20 | Cetak non-sales | `Dokumen ini hanya untuk order penjualan.`; loading `Memuat dokumen...`; gagal `Order tidak ditemukan.` |
| E-21 | Kolom Disc hanya bila ada diskon-baris (diskon level-order saja → kolom hilang walau agregat > 0) |
| E-22 | `Tgl. Jtp` = `-` bila lunas/tanpa tempo; Staff = pembuat mentah (kasir-aman); USER = pencetak |
| E-23 | Notes cetak hanya-cetak (reset saat reload/order-lain); filler A4 6 baris; kontinu tanpa filler |
| E-24 | Shortcut: nama kosong → inline `Nama pelanggan wajib diisi.`; gagal simpan → `Pelanggan gagal disimpan.`; baru langsung terpilih + requote |

---

## 4. Katalog Pesan (teks apa adanya)

**Sales:** `'Penjualan tempo wajib memilih customer.'` · `'Penjualan tempo wajib mencatat nomor HP customer.'` ·
`'Pihak terkait untuk penjualan tempo harus bertipe customer, bukan {t}.'` ·
`'Customer tidak ditemukan atau sudah diarsipkan.'` ·
`"Harga jual untuk produk '{n}' (Rp {a}) tidak boleh di bawah harga jual minimum (Rp {m})"` ·
`"Diskon produk '{n}' tidak boleh melebihi harga satuan."` ·
`"Persentase diskon produk '{n}' harus bernilai 0 sampai 100."` ·
`'Order manual tidak boleh langsung selesai tanpa proses serah barang.'` ·
`'Sales order harus diserahkan terlebih dahulu sebelum selesai.'` ·
`'Sales order yang barangnya sudah diserahkan hanya dapat dilanjutkan ke status selesai.'` ·
`'Sales order prabayar harus lunas sebelum dapat diproses.'` ·
`'Hanya sales order (jenis: sales) yang dapat diserahkan barangnya'` ·
`'Barang pada order ini sudah pernah diserahkan sebelumnya'` ·
`'Status selesai untuk sales order belum dikonfigurasi'` ·
`'Sales order belum berada pada tahap serah barang.'` ·
`'Sales order prabayar harus lunas sebelum barang diserahkan.'` ·
`'Sales order COD harus memilih bayar saat serah atau diubah ke tempo sebelum selesai.'` ·
`'Anda tidak memiliki izin untuk mencatat pembayaran customer saat serah barang.'` (403) ·
`'Uang tunai yang diterima tidak boleh kurang dari total tagihan.'` ·
`'Purchase order tidak memerlukan persetujuan kredit'` ·
`'Order sudah menggunakan termin pembayaran tempo'` · label tanggal `Tanggal serah barang`.

**UI:** `Nama pelanggan wajib diisi.` · `Pelanggan gagal disimpan.` ·
`{2 pesan}; dan {n} produk lain. Periksa harga master produk atau ubah customer.` ·
`Harga di bawah minimum` / `Harga minimum` (tombol POS) · `Termin diubah ke tempo` /
`Order dapat diselesaikan dengan jatuh tempo yang tercatat.` · `Gagal` + pesan ·
`Pembayaran berhasil dicatat` · `Pembayaran dihapus` · `Gagal memuat data pembayaran` ·
`Hapus pembayaran?` / `Catatan pembayaran ini akan dihapus. Tindakan ini tidak bisa dibatalkan.` /
`Hapus Pembayaran` · `Ubah ke Tempo` · `Belum ada transaksi pembayaran` (+ varian supplier/customer) ·
`Cetak Tagihan` / `Cetak Nota` · `Nota Penjualan Lunas` / `Tagihan - Bayar Sebagian` / `Tagihan Penjualan` ·
`Pelanggan umum` · `Memuat dokumen...` / `Order tidak ditemukan.` / `Dokumen ini hanya untuk order penjualan.` ·
`Sembunyikan diskon (harga penuh)` · `Kertas A4 (biasa)` / `Kontinu 3-Ply (Dot-Matrix)` /
`Kop/rekening sudah preprinted (sembunyikan)` · `Notes (opsional, hanya tampil di hasil cetak — tidak disimpan):` /
`Tulis catatan tambahan untuk nota ini...` · `Cetak / Print` / `Tutup`.

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Serah parsial langsung | Sekali tembak + tolak-ulang (parsial via SJ modul 11) |
| NF-02 | Diskon di purchase | Kolom + submit sales-saja |
| NF-03 | Member di purchase | Selalu manual |
| NF-04 | Tempo tanpa jatuh tempo | Server + dialog + FE required |
| NF-05 | Tanda tangan di nota/bayar | Disengaja (validasi = nomor + bukti + audit) |
| NF-06 | Kolom Disc tanpa diskon-baris | Syarat `hasLineDiscount` |
| NF-07 | Hapus bayar terminal-non-net | Disembunyikan + ditolak |
| NF-08 | Serah tanpa jalur selesai (non-pos) | Wajib transisi aktif |
| NF-09 | Kembalian tersimpan | Dihitung tampil dari tender − tagihan |
| NF-10 | Pindah cabang / ubah ship-to purchase | Ship purchase selalu kosong |
