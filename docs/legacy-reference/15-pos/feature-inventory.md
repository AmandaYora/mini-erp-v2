# Feature Inventory — Modul 15 POS / Kasir (+ Shell Android)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Semua isi diturunkan langsung
dari kode (modul web `pos/` 15 file, struk thermal, `native-printer.ts`, `android-pos-shell`
8 file, migrasi 015, E2E 07/11/15/17). Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

**Batas modul.** Modul ini = kasir ritel (katalog + cart + checkout tunai/piutang + struk +
cetak native). Tanpa backend sendiri: order via `orders/create` (`source='pos'`), bayar +
serah via `orders/deliver-goods`, harga member via `pricing/quote`, stok via
`products/list` + `stock/balances` + `stock/scan-product` + `stock/reservations/*`.
Mekanika order/bayar/member milik 07/09/14; di sini perilaku kasirnya.

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Rute | `/pos` (`order.create`, menu Operasional → POS) + `/orders/:orderId/pos-receipt/print` (`order.view`) |
| Guard halaman | Tanpa perusahaan/cabang aktif → `Pilih cabang terlebih dahulu` / `POS membutuhkan cabang aktif.` (bukan redirect) |
| Backend dipakai | create order, deliver (pay_now tunai/kembalian; tanpa-pelanggan możliwy), quote member, scan, balances (100/batch, 60-detik), reservasi (kunci acak, TTL 600 dtk, rilis) |
| Penomoran | Nomor order `.../PJ/...` + nomor bayar (milik 08/14); tanpa nomor POS sendiri |
| Perangkat | Shell Android (WebView + `window.AndroidPrinter` + ESC/POS LAN + laci + kamera + `/ting.mp3`); browser = `window.print()` |
| Aksi yang TIDAK ada | Edit/hapus transaksi (order POS final); diskon persen (nominal saja); metode selain Tunai/Transfer; cicilan; tempo-tanpa-pelanggan-HP; checkout tanpa `order.create`; cetak tanpa order |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Katalog (`/pos` kiri)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Cari (350ms) | `Cari produk` (`pos-search`, `Cari nama atau kode produk...`); tombol `Scan Barcode / QR`; hitungan `{n} produk ditemukan` |
| F-01.2 | Drilldown kategori | `Semua` + breadcrumb `/` + `Naik ←` + pil (badge jumlah-anak); pilih = filter (+ masuk-anak bila beranak) |
| F-01.3 | Grid kartu (36/halaman, tambah-gulir) | Foto (lazy; fallback 2-huruf)/nama-2-baris/kode/varian-dropdown (`Cari varian...`)/harga-brand + `/ {uom}`/badge stok (Habis-merah/`{jual} ({base})`/kritikal-kuning/sukses; tanpa-badge non-tracked)/stepper-bila-di-cart else `[+ Tambah]` (klik + Enter/Spasi + animasi 100ms; habis = `Stok Habis` disabled + redup + aria). Klik-kartu = tambah (kecuali habis/sudah-di-cart) |
| F-01.4 | Keadaan | Skeleton 8; `Gagal memuat produk` + `Muat ulang`; `Tidak ada produk yang cocok`; bawah: `Memuat produk...` / `Muat ulang` (error + sisa) / `Semua produk tampil` |
| F-01.5 | Stok live | Saldo per produk tampil + cart (100/batch; 60-detik; gagal = snapshot-lama); urut stok-tersedia (request) |

### F-02 — Scan barcode/QR

Modal (`Scan Barcode / QR` dua tempat) + input manual (`Input kode barcode manual` + `[Tambah]`)
+ kamera (shell) → `stock/scan-product` → tambah-langsung + `/ting.mp3` (gagal-bunyi tak
menggagalkan!) + saldo-disimpan. Dialog hasil (`Mengerti`, tutup `Tutup hasil scan`): Kode
kosong (warning, dua varian) / Tidak ditemukan / Pilih varian (kode-induk!) / Nonaktif /
Stok kosong / Stok tidak cukup (`...mencapai stok tersedia: {n} {uom}`) / Koneksi gagal
(danger). E2E 07 mengunci sukses + kosong + bunyi + input-kosong-kembali.

### F-03 — Cart (kanan, `pos-cart-panel`)

Header (`Transaksi Aktif` + `{n} items` + `Kosongkan` merah) + ringkasan pelanggan
(`Pelanggan`: `Umum / Walk-in` + telepon/kode + badge member) + baris:
nama/kode[/varian]/harga-efektif (`dari {penuh}` bila diskon) + badge member + lokasi ≤3
(`Lokasi: {n} ({q} {uom})`) + hapus-`✕` + stepper (−/input/`max=stok`/`step`/+; 0 = hapus;
dijepit-stok) + Diskon nominal-total (`max`, klem-minimum/member-nol) + total-tebal +
`Hemat {n}`. Wide-variant (tak terpakai? — komponen mati, → KI baru). Kosong:
`Pilih produk dari katalog`. Kunci baris `produk:varian`; tambah-ada = +1 (dijepit) +
segarkan harga/stok/label; non-tracked tanpa-batas.

Ringkasan: Subtotal + Diskon + Total-besar-brand; Termin + Metode **TERSEMBUNYI**
(`display:none` — hanya dialog yang berlaku! → KI baru); error merah + member-kuning
(`Menghitung harga member...`); tombol utama (`Pilih produk terlebih dahulu` / `Harga di
bawah minimum` / `Lanjut Pembayaran`; disabled kosong/tanpa-izin/harga-invalid) + bar-mobile
(`{n} item` + total + `Pilih produk`/`Harga minimum`/`Bayar`; ≤900px; safe-area).
Sukses-overlay: `Sukses` + (`Transaksi Berhasil` / `Barang Diserahkan` + `Piutang tercatat —
lunasi via halaman Order`) + nomor + Total + checkbox sembunyi-diskon (bila hemat/diskon) +
`[Cetak Struk]` + `[Cetak Nota/Tagihan]` + `[Bukti Bayar]` (bila ada) + `[Lihat Order]` (tab)
+ `[Transaksi Baru]` (reset-total). Catatan transaksi (opsional, → `order.notes`).

### F-04 — Dialog checkout (inti kasir!)

Judul dinamis (`Pembayaran` / `Serah Barang`); total-besar; mode 2-kotak
(`Bayar Sekarang`-brand / `Bayar Nanti`-kuning); pelanggan (walk-in + autocomplete + tambah
+ member + HP-wajib-hutang + alamat-opt-in); catatan; tunai: metode (Tunai/Transfer) + Uang
diterima (digit-saja, `Kosongkan bila uang pas`) + saran-cepat (pas + bulat-5/10/50/100rb,
maks 4, unik) + kurang-merah (`Uang diterima kurang dari total transaksi.`) / Kembalian-hijau;
tempo: Jatuh-tempo (`date`, min-hari-ini) + teks-piutang + blokir-tanpa-HP; footer `[Batal]` +
`[Proses Pembayaran]`/`[Serahkan & Catat Piutang]` (disabled: tempo-tanpa-HP / quote-loading /
quote-error / tunai-kurang); proses = spinner (`Memproses transaksi...` / `Harap tunggu,
jangan tutup halaman.`, kunci-tutup).

### F-05 — Arus simpan (idempoten-ganda + reservasi-hantu-guard)

Kunci: meterai (`produk:varian:qty:harga:diskon|pelanggan|tempo`) + order-tertunda (beda →
error `Order POS sebelumnya sudah dibuat tetapi belum selesai. Selesaikan order tersebut dari
halaman Order sebelum membuat transaksi baru.`); hold-ulang + serah (`pay_now` tunai/
kembalian / tanpa-pelanggan-boleh; `net`-tempo) → abaikan-`sudah pernah diserahkan` (lanjut
sukses!) → sukses (rotasi-kunci + saldo + lepas-pelanggan/alamat) / error (pesan). Tanpa
pelanggan + tunai lolos (E2E 07); tempo wajib pelanggan + HP (`Pilih pelanggan dan isi no. HP
sebelum mencatat piutang.`); quote-loading/error memblokir (`Harga member masih dihitung...`).
Reservasi ambient 350ms/TTL-600 (bukan saat proses/sukses — anti-hantu) + rilis-saat-kosong;
kunci per-percobaan, konsumsi-saat-serah.

### F-06 — Struk thermal + cetak native

Halaman `/pos-receipt/print` (`order.view`): loading `Memuat struk POS...`; non-sales
`Struk POS hanya untuk transaksi penjualan.`; konten (logo/nama/tagline/HP/`LUNAS|PIUTANG`;
No Order/Bayar/Tanggal/Kasir(pembuat-jatuh-ke-pencetak)/Pelanggan; baris
`{qty} {uom} x {penuh}` + subtotal + Diskon + catatan; Subtotal/Diskon/Pajak/TOTAL/Dibayar/
Tunai/Kembalian/Sisa/Metode(gabung); rekening; `Barang yang sudah dibeli mengikuti kebijakan
toko.`/`Terima kasih`; hide-diskon simetris nota). Toolbar: kertas + hide-diskon (bila ada) +
cetak. Native: `window.AndroidPrinter` (fitur-deteksi; 20-detik-timeout; `NO_BRIDGE/TIMEOUT`);
payload-angka-final (web-hitung, Kotlin-cetak); gagal → `alert Gagal mencetak struk...`.
Kotlin: 48-kolom ESC/POS + logo + `Rp .`-format + `TES CETAK` + potong + laci + pindai-LAN +
pengaturan (IP/port/MAC/potong/laci; 3-jari-tahan; kamera; URL-server; pertama-kosong →
pengaturan-otomatis).

### F-07 — Pelanggan POS

Autocomplete (API `customers/list` cari + 10; `Cari atau pilih pelanggan...`;
`Memuat…`/`Tidak ada hasil`; klik-luar-tutup; pilih = objek) + panel tambah
(`Tambah Pelanggan Baru`; `Nama pelanggan *`; `No. HP[*]`; member (bila ada opsi); error
inline; `Batal`/`Simpan Customer`; sukses → langsung-terpilih + ambil-ulang-server (kode +
member akurat)) + walk-in (boleh tunai; dilarang tempo).

---

## 3. Edge Case (26)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Tanpa cabang | Layar toko + `Pilih cabang terlebih dahulu` (bukan redirect) |
| E-02 | Tanpa `order.create` | Tombol mati (label tetap); guard halaman tetap terbuka! |
| E-03 | Habis (0) | Kartu redup + `Stok Habis` + tanpa-tambah; scan → dialog `Stok kosong` |
| E-04 | Tambah melebihi stok | Dijepit (kartu/stepper/cart/scan sama) |
| E-05 | Qty 0 | Hapus baris (stepper/input) |
| E-06 | Non-tracked | Tanpa-batas + tanpa-badge + tanpa-reservasi |
| E-07 | Varian ganda | Kunci per varian; scan-induk → pilih-manual; 1-varian preselect |
| E-08 | Harga berubah saat di-cart | Segarkan (harga + sumber + member + stok) |
| E-09 | Diskon > (harga − floor) | Diklem (member-floor-nol!); total tak-habis-bagi per-unit |
| E-10 | Harga < minimum non-member | Tombol `Harga di bawah minimum`, submit diam |
| E-11 | Ganti/lepas pelanggan | Reset-alamat + quote-ulang / reset-standard |
| E-12 | Quote gagal | Banner + submit-diblokir (tunai pun tak bisa!) |
| E-13 | Tempo tanpa HP | Error + tombol-dialog-mati |
| E-14 | Tunai kurang | Tombol-dialog-mati + merah (server pun menolak!) |
| E-15 | Klik-ganda / serah-ganda | Guard + abaikan-sudah-serah (sukses!) |
| E-16 | Cart-berubah pasca-order | Error tertundapecahkan (ke Order!) |
| E-17 | Tanpa pelanggan tunai | Lolos (walk-in; tanpa-saldo) |
| E-18 | Kirim-alamat POS | Opt-in + snapshot (mayoritas ambil-di-tempat) |
| E-19 | Catatan | → `order.notes` → detail order |
| E-20 | Saldo gagal-refresh | Snapshot-lama (tanpa error!) |
| E-21 | Request basi (cari/saldo/quote) | Diabaikan (sequence/penjaga) |
| E-22 | Modal `confirming` mati (`if (false && ...)`) + Termin-tersembunyi + Wide-mati | Kode mati 3 (→ KI baru) |
| E-23 | Cetak-gagal-native | `alert` + tetap `window.print` manual |
| E-24 | Struk tanpa-bayar (tempo) | Metode `Tempo`; tanggal = serah; tanpa-tunai/kembalian |
| E-25 | Kembalian vs sembunyi-diskon | Dihitung dari total-tampil (bruto!) |
| E-26 | Kasir-tanpa-`users.view` | Nama jatuh-ke-sendiri (komentar adapter) |

---

## 4. Katalog Pesan (teks apa adanya)

**Katalog/kartu:** `Cari nama atau kode produk...` · `{n} produk ditemukan` · `Cari varian...` · `Tambah {nama} ke keranjang` / `...sudah di keranjang` / `... — stok habis` · `Stok Habis` / `+ Tambah` · `Habis` / `{jual} ({base})` · `Gagal memuat produk` / `Muat ulang` / `Tidak ada produk yang cocok` / `Semua produk tampil` / `Memuat produk...`.

**Scan:** `Scan Barcode / QR` · `Input kode barcode manual` · `Tambah` · `Kode scan kosong` / `Scanner tidak mengirim kode produk. Coba scan ulang atau ketik kode manual.` · `Produk tidak ditemukan` / `Kode ini belum terdaftar sebagai kode produk aktif.` · `Pilih varian produk` / `Kode yang discan adalah kode produk utama. Pilih varian di kartu produk atau scan barcode varian.` · `Produk tidak aktif` / `...statusnya tidak aktif sehingga tidak bisa dijual di POS.` · `Stok kosong` / `Produk ini terdaftar, tetapi stok cabang aktif kosong.` · `Stok tidak cukup` / `Jumlah di cart sudah mencapai stok tersedia: {n} {uom}.` · `Scan belum bisa diproses` / `Koneksi ke server atau data stok belum siap. Coba scan ulang atau pilih produk secara manual.` · `Mengerti` / `Tutup hasil scan`.

**Cart/checkout:** `Transaksi Aktif` · `{n} items` · `Kosongkan` · `Pilih produk dari katalog` · `Hapus dari cart` · `Kurangi jumlah` / `Tambah jumlah` / `Jumlah {nama}` / `Diskon {nama}` · `Hemat {n}` · `Subtotal` / `Diskon` / `Total` · `Pilih produk terlebih dahulu` / `Harga di bawah minimum` / `Lanjut Pembayaran` · `Pilih produk` / `Harga minimum` / `Bayar` · `Sukses` · `Transaksi Berhasil` / `Barang Diserahkan` + nomor · `Piutang tercatat — lunasi via halaman Order` · `Sembunyikan diskon (harga penuh)` / `Struk/nota tampil di harga penuh tanpa baris diskon.` · `Cetak Struk` / `Cetak Nota` / `Cetak Tagihan` / `Bukti Bayar` / `Lihat Order` / `Transaksi Baru` · `Pelanggan` / `Umum / Walk-in` · `Cari atau pilih pelanggan...` / `Memuat…` / `Tidak ada hasil` · `Tambah Pelanggan Baru` · `Nama pelanggan *` / `No. HP[*]` · `Jenis member (opsional)` / `Non-member` · `Simpan Customer` / `Menyimpan...` · `Nama pelanggan wajib diisi` · `No. HP wajib diisi untuk customer yang berhutang` · `Gagal menambah customer` · `Kirim ke alamat` · `Pembayaran` / `Serah Barang` · `Total transaksi` · `Bayar Sekarang` / `Bayar Nanti` · `Tunai` / `Transfer` · `Uang diterima` / `Kosongkan bila uang pas` / `Uang pas` · `Uang diterima kurang dari total transaksi.` / `Kembalian` · `Jatuh tempo` (+ `Jatuh tempo pembayaran`) / `Stok keluar sekarang. Sisa tagihan menjadi piutang dan bisa dilunasi via halaman Order.` / `Pilih pelanggan dan isi no. HP untuk mencatat piutang.` · `Catatan (opsional)` / `Catatan operasional untuk transaksi ini` / `Catatan transaksi POS` · `Batal` / `Proses Pembayaran` / `Serahkan & Catat Piutang` · `Memproses transaksi...` / `Harap tunggu, jangan tutup halaman.` · `Menghitung harga member...` · `Pilih cabang terlebih dahulu` / `POS membutuhkan cabang aktif.`.

**Arus:** `Pilih pelanggan dan isi no. HP sebelum mencatat piutang.` · `Harga member masih dihitung. Tunggu sebentar sebelum checkout.` · `Order POS sebelumnya sudah dibuat tetapi belum selesai. Selesaikan order tersebut dari halaman Order sebelum membuat transaksi baru.` · `Transaksi gagal. Periksa koneksi dan coba lagi.` · `Harga member belum bisa dihitung. Periksa koneksi atau data harga master produk.` · `Gagal mencetak struk{...}. Coba lagi atau cek printer.`.

**Struk/shell:** `Memuat struk POS...` / `Order tidak ditemukan.` / `Struk POS hanya untuk transaksi penjualan.` · `LUNAS` / `PIUTANG` · `No Order` / `No Bayar` / `Tanggal` / `Kasir` / `Pelanggan` · `Subtotal` / `Diskon` / `Pajak` / `TOTAL` / `Dibayar` / `Tunai` / `Kembalian` / `Sisa` / `Metode` · `REKENING : ...` · `Barang yang sudah dibeli mengikuti kebijakan toko.` / `Terima kasih` · `TES CETAK` / `Kasir Mini ERP` / `Jika ini tercetak, koneksi printer berhasil.` · `Jembatan printer tidak tersedia` / `Printer tidak merespons`.

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Backend POS | Order + bayar + stok + quote + reservasi |
| NF-02 | Edit transaksi | Final + idempoten-tertunda |
| NF-03 | Diskon persen / metode ke-3 / cicilan | Nominal + Tunai/Transfer + penuh |
| NF-04 | Tempo tanpa HP / tanpa pelanggan | Dua lapis (dialog + simpan) |
| NF-05 | Cetak tanpa order / tanpa login | Order + `order.view` |
| NF-06 | Bunyi-gagal-gagalkan / saldo-gagal-blokir | Komentar + catch-diam |
| NF-07 | Kirim-alamat default | Opt-in (mayoritas ambil) |
| NF-08 | Kode mati terpakai | `if (false)` + hidden + Wide (E-22!) |
| NF-09 | Stok-negatif-cart | Jepit semua pintu + maksimum-tombol |
| NF-10 | Kembalian-simpan / tender-non-tunai | Null + hitung-tampil |
