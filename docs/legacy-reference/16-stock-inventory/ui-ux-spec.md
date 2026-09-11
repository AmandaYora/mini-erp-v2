# UI/UX Spec — Modul 16 Stock / Inventory & Gudang

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** 10 rute (`stock.view` untuk
lihat; `stock.adjust` untuk sesuaikan/buka/rusak-tulis; `stock.transfer` untuk pindah/dokumen;
`stock.update` untuk lokasi-tulis; `order.create` untuk reservasi-kasir di balik layar).
Menu grup **"Stok & Gudang"** 6 entri. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. `/stock` — Stok Barang

Header (`Dashboard > Stok`; `Stok Barang`; `Pantau ketersediaan barang secara menyeluruh.`;
`[Riwayat Mutasi]` + `[Penyesuaian Stok]` bila adjust). Filter: `Cari Barang`
(`stock-search`, `Kode atau nama produk...`, debounce 400ms) + scan (`Scan Barcode / QR`) +
`Tampilan` (`Semua Stok`/`Stok Kritis Saja`). Kartu `Daftar Barang` (`{n} barang ditemukan.`/
`Memuat…`): tabel Produk (nama + kode!) · Tersedia-besar (+ `Kritis`!) · Fisik · Dipesan ·
Status (`Kritis`/`Aman`) · Aksi-`Detail`; kosong `Tidak ada stok` (+ `Tidak ada barang yang
sesuai dengan filter pencarian Anda.`!); paginasi 20; guard `activeWorkspaceData` (null =
kosong!).

## 2. `/stock/:itemId` — Detail

Header (breadcrumb `Dashboard > Stok > {nama}` + deskripsi-kode + Kembali-`navigate(-1)` +
Penyesuaian-prefill `?item_id=&returnTo=/stock`). Metrik 2×2 (`Metrik Stok`: `Tersedia (Siap
Jual)` / `Fisik Aktual` / `Dipesan` / `Minimum Stok`, 2xl! + `Memuat...`). Posisi Gudang
(`Distribusi lokasi fisik barang ini.`; pohon relevan: leluhur + daun-berstok-tebal + badge +
varian-chip; kosong `Tidak ada stok fisik di gudang manapun.`). Riwayat (`Riwayat Mutasi
Terakhir`; `Pergerakan masuk, keluar, dan koreksi stok untuk barang ini.`; kolom Waktu/Tipe-
badge/Qty +/−/Alasan; 10-terakhir-server + `Lihat Semua` (prefill kode!)). Tanpa-produk +
selesai-muat = halaman kosong (null!).

## 3. `/stock/adjustments/create` — Penyesuaian

Header (`Dashboard > {Gudang|Stok} > Penyesuaian` dinamis-returnTo!; `Koreksi jumlah stok.`;
Kembali-`navigate(-1)`). Kartu Tipe (3 tombol-besar: `Terima Barang` + `Menambah stok (contoh:
barang masuk dari supplier atau retur).`-hijau + `Keluarkan Barang` + `Mengurangi stok (contoh:
barang rusak, hilang, atau dipakai).`-merah + `Koreksi Fisik` + `Menyesuaikan angka stok ke
jumlah aktual (contoh: Stock Opname).`-kuning). Detail (`Detail Penyesuaian` + `Lengkapi
informasi barang dan lokasi.`; Item async tracked-saja + Varian-kondisional (`Pilih varian`,
auto-bila-tunggal!) + Lokasi default-daun-pertama + Qty default-1/min-dinamis (koreksi-0!) +
Alasan required-`Contoh: koreksi hasil stock opname pagi...`). `[Batal]` (→returnTo!) +
`[Lanjut Otorisasi]` (disabled tanpa-item/lokasi/varian-wajib). Modal `Otorisasi Penanggung
Jawab` (420px; `Masukkan akun owner, admin, atau superadmin untuk menyimpan penyesuaian stok.`;
`Username atau email` + `Password` + `[Batal]`/`[Otorisasi & Simpan]`/`Memverifikasi...`;
sukses → reset + returnTo + `Penyesuaian stok tersimpan`; gagal → `Gagal menyimpan penyesuaian
stok` + pesan, tetap-buka).

## 4. `/stock/movements` — Riwayat Mutasi

Header (`Dashboard > Stok > Riwayat Mutasi`; `Awasi seluruh riwayat pergerakan stok
keluar-masuk secara menyeluruh.`; tanpa aksi!). Filter: `Cari Produk` (`search-item`,
`Kode atau nama produk...`, debounce-400!) + scan (`Scan Barcode / QR`, mengisi-search!) +
`Tipe Mutasi` (`Semua Tipe`/`Terima Barang (Masuk)`/`Keluarkan Barang (Keluar)`/`Koreksi
Fisik`) + `Rentang Tanggal` (dari–sampai + `T23:59:59`-sampai!) + `Hapus Filter` (merah,
reset-total!). Kartu (`{n} mutasi ditemukan.`/`Memuat…`): tabel Produk (nama + kode + lokasi!)
· Waktu · Tipe (badge `Terima`-hijau/`Keluar`-merah/`Koreksi`-kuning!) · Qty (+/−/polos + UOM!)
· Saldo (`sebelum → sesudah` + UOM!) · Catatan; kosong `Belum ada mutasi stok` + `Mutasi akan
muncul setelah ada penerimaan, pengeluaran, transfer, atau koreksi stok.`; paginasi 25.

## 5. `/gudang` — Gudang (drill-down)

Header (`Dashboard > Gudang`; `Jelajahi lokasi penyimpanan dan lihat isi masing-masing.` +
`[Kelola Lokasi]` + `[Pindah Lokasi]` (transfer!)). Breadcrumb (`Semua Gudang` + rantai
+ `← Kembali ke ...`). Kartu lokasi (📦-daun/🗂️-grup + nama + kode + badge `{n} sub-lokasi`/
`{n} jenis barang`/`Kosong` + ›; klik = masuk/toggle-isi!). Isi-daun (`Isi: {nama}`;
`Memuat…`/`Gudang ini kosong.` + `Belum ada stok yang tercatat di lokasi ini.`/
`{n} jenis barang tersimpan di sini.` + `Pindah dari Sini`): baris-expand (nama + kode +
tersedia + `Kritis` + ▲/▼; buka: 3 metrik Tersedia/Fisik-Aktual/Dipesan + `Pindah Lokasi`/
`Penyesuaian Stok` deep-link!). Cache-per-daun (limit-200!). Kosong `Belum ada lokasi` +
`Tambahkan lokasi melalui halaman Kelola Lokasi.`

## 6. `/gudang/locations` — Kelola Lokasi

Header (`Dashboard > Gudang > Lokasi`; `Kelola hierarki lokasi penyimpanan. Barang hanya bisa
disimpan di Lokasi Akhir (tanpa sub-lokasi di bawahnya).` + `[+ Tambah Lokasi]` (kelola)).
Tree (`Tree Lokasi`; `{n} lokasi aktif pada cabang ini.`; sejajar-24px; `▾/▸/—`; badge
`Lokasi Akhir`-info/`Grup`-netral; handle `⠿` + `Geser untuk mengubah urutan` (kelola!;
baca-saja-tanpa-handle!) + highlight-biru-saat-drag; drag-sesama-level → simpan-otomatis +
`Urutan lokasi diperbarui`!). Baris: `[+ Sub-lokasi]` + `[Edit]` + `[Arsipkan]`
(tanpa-dialog! + toast arsip!). Modal tambah/edit (badge Tambah/Edit; induk-readonly-path
`A > B > C`; Kode* + Nama*; Batal/Simpan/`Menyimpan...`). Modal pindah-otomatis
(`Pindahkan Stok Otomatis?`; notice `Lokasi {induk} masih memiliki stok. Jika dilanjutkan,
semua stok pada lokasi tersebut akan dipindahkan secara otomatis ke sub-lokasi baru {anak}
yang akan dibuat.` + `Aksi ini tidak dapat dibatalkan setelah dikonfirmasi.`;
`[Batal]`/`[Ya, Pindahkan & Buat]`/`Memproses...`). Kosong contoh Toko/Gudang A/B.

## 7. `/gudang/transfer` — Surat Transfer (dokumen!)

Header (`Dashboard > {Gudang|Stok} > Transfer Antar Cabang` dinamis-returnTo!;
`Buat surat transfer pengiriman fisik antar cabang/gudang.`). Kartu `Surat Transfer`
(`Draft belum mengubah stok. Status Sedang Dikirim sudah keluar dari stok asal dan menunggu
penerimaan di tujuan.`; `Memuat surat transfer...` / kosong `Belum ada surat transfer` +
`Surat transfer akan muncul setelah Anda membuat pengiriman barang antar cabang.`): tabel
Tanggal/Nomor (`{TRF-...}`-tebal!)/Arah (`Dalam Cabang`-netral/`Keluar`-warning/`Masuk`-info)/
Status (`Draft`-info/`Sedang Dikirim`-warning/`Diterima`-sukses/`Batal`-merah)/Dari/Tujuan/
Barang (`Nama (qty UOM)` koma!)/Aksi (`Cetak` selalu + `Kirim` bila-draf-asal + `Terima`
bila-transit-tujuan + `Batal` bila-draf-asal!). Form `Form Transfer Antar Cabang`
(`Setelah disimpan, sistem membuat surat draft. Stok asal baru berkurang setelah tombol Kirim
ditekan.`): Dari-Cabang-readonly + Ke-Cabang + Dari-Lokasi + Produk-tracked + Varian
(`Pilih varian`) + Ke-Lokasi (`Memuat lokasi tujuan...`; `Lokasi tujuan belum tersedia` bila
kosong!) + Jumlah (`Maks: X`) + `Nama Sopir`* + `Nomor Kendaraan` + Catatan; notice
(`Belum ada cabang aktif` / `Mengecek stok...` / `Produk tidak tersedia di lokasi ini` +
`Pilih lokasi asal yang berbeda atau produk yang berbeda.` / `Jumlah melebihi stok tersedia`
+ `Stok tersedia: X. Kurangi jumlah...` / `Stok tersedia di lokasi asal` + `siap
ditransfer.`); `[Batal]` + `[Simpan Surat Transfer]` (gate: cabang + lokasi-dua + produk +
varian-wajib + sopir + 0 < qty ≤ tersedia!). Cetak (`Surat Transfer Barang`; Nomor/Tanggal/
Status; Dari/Tujuan/Sopir/Kendaraan; tabel Barang/Jumlah/Lokasi-Asal/Lokasi-Tujuan; Catatan;
tanda-3 `Gudang Asal`/`Sopir`/`Gudang Tujuan` + `Nama & Tanda Tangan`!; cetak = `window.print`
+ CSS-sembunyi-semua-kecuali-surat!). Toast: `Surat transfer antar cabang disimpan` +
`Cetak surat lalu tekan Kirim saat barang benar-benar keluar dari gudang asal.` /
`Barang ditandai sedang dikirim` + `Stok asal sudah berkurang. Stok tujuan baru bertambah
setelah surat diterima.` / `Transfer diterima, stok tujuan bertambah` /
`Surat transfer dibatalkan` + 3-gagal + `Gagal memuat lokasi cabang tujuan`.

## 8. `/gudang/pindah-lokasi` — Pindah Lokasi (seketika!)

Header (`Dashboard > Gudang > Pindah Lokasi`; `Pindahkan stok antar lokasi dalam cabang aktif
tanpa surat jalan.` + `[← Kembali]`). Kartu `Detail Pindah Lokasi` (`Stok langsung keluar
dari lokasi asal dan masuk ke lokasi tujuan.`): Lokasi-asal + Lokasi-tujuan (beda-paksa,
tujuan-kecualikan-asal!) + Produk-tracked + Varian (`Pilih varian`, label `nama (kode)`!) +
Qty (min-1, maks-tersedia + `Maks: X`!) + Catatan (`Contoh: pindah ke rak picking, relokasi
display, atau susun ulang gudang`); notice `Lokasi asal dan tujuan sama` + 4-notice-stok
(`Pilih lokasi akhir` + `Lokasi asal harus berupa lokasi paling bawah, misalnya rak atau bin
yang menyimpan barang.` / `Mengecek stok` + `Memuat stok tersedia di lokasi asal.` /
`Stok tidak tersedia` + `Produk ini tidak memiliki stok siap pindah di lokasi asal.` /
`Qty melebihi stok tersedia` + `Stok tersedia: X` / `Stok tersedia` + `siap dipindahkan
dari lokasi asal.`); `[Batal]` + `[Pindahkan]`/`Memindahkan...` (gate-9-syarat!); sukses =
reset-form + `Stok dipindahkan` + `Lokasi asal dan tujuan sudah diperbarui.` (tetap-di-halaman!).

## 9. `/gudang/damaged` — Barang Rusak

Header (`Dashboard > Gudang > Barang Rusak`; `Pisahkan barang rusak dari stok jual, tetap
lihat qty dan potensi nilainya.`). Kartu (`Jenis Barang Rusak` + `Potensi Nilai`-format-rupiah!).
`Daftar Barang Rusak` (kosong `Belum ada barang rusak` + `Barang yang dipisahkan dari stok
jual akan tampil di daftar ini.`): tabel Produk (+ varian-kode!) · Lokasi (badge-warning!) ·
Qty-kanan · Potensi-Nilai-kanan · Aksi (`Kembalikan` + `Rugi/Buang`-merah, izin!; tanpa-izin
= `—`!) + paginasi-20. Form `Catat Barang Rusak` (`Kosongkan lokasi asal jika barang rusak
kembali dari pengiriman dan sebelumnya sudah keluar dari stok.`): Produk-tracked + Varian
(`Pilih varian`) + Lokasi-asal-opsional (`Tidak dari stok aktif`!) + Qty + Alasan
(`Contoh: rusak di jalan, patah saat bongkar, atau cacat dari supplier`); notice
`Barang rusak tidak ikut stok jual. Jika nanti masih layak, gunakan tombol Kembalikan.`;
`[Simpan Barang Rusak]` (gate!). Modal `Kembalikan ke stok normal` (badge; Lokasi-tujuan* +
Qty (maks-rusak!) + Alasan-bawaan-`Masih layak jual`; `[Batal]`/`[Simpan]`). Buang =
langsung-tanpa-dialog (alasan-tetap `Tidak layak dijual`!). Toast: `Barang rusak dicatat` /
`Barang dikembalikan ke stok normal` / `Barang rusak ditandai rugi/buang` + 3-gagal +
`Gagal memuat barang rusak`.

## 10. `/stock/opening` — Stok Awal (wizard!)

Header (`Stok & Gudang > Stok Awal`; `Migrasi jumlah fisik barang per lokasi sebelum transaksi
berjalan dicatat.` + `[Buka Wizard]`). 4-kartu (Barang-Stok/Lokasi-Aktif/
Belum-Dipilih-Lokasi-kuning-bila-ada/Sudah-Diisi!). `Alur Migrasi Stok` (`Satu wizard untuk
lokasi, template, preview, import stok, dan sinkron draft keuangan.`; notice `Mulai dari
wizard` + `Template akan mengambil barang dan lokasi dari data sistem agar formatnya
konsisten.` / sukses `Stok awal sudah diimport` + `{n} baris stok masuk dan {m} baris saldo
awal keuangan tersinkron.`). Wizard `Migrasi Stok Awal` (badge-Excel; `Lengkapi lokasi,
download template, cek file, lalu simpan stok awal.`; aksi-`[Buka Saldo Awal Keuangan]`
bila-import + `[Tutup]` + `[Simpan Stok Awal]`/`Memproses...`-gate!): metrik-5
(Barang/Belum-Dipilih-kuning/Lokasi/Sudah-Diisi/Error-File-merah!); lokasi-kosong =
`Tambah Lokasi Gudang` (`GUD-UTAMA`/`Gudang Utama` + `[Simpan Lokasi]`!); tetapkan =
`Pilih Lokasi untuk Template` (`{n} barang belum dipilihkan lokasi...`) /
`Ubah Lokasi untuk Import Tambahan` + massal (`Terapkan lokasi yang sama` +
`Terapkan ke yang kosong`/`Terapkan ke semua`!) + tabel Kode/Nama/Satuan/Status
(`Sudah Diisi`-hijau/`Belum Diisi`-kuning)/Lokasi-di-Template; siap = `Template siap
diunduh` + `{n} barang yang belum punya stok awal pada lokasi terpilih.`; file
(`Pilih file Excel stok awal` + `{nama} - {ukuran} - .xlsx / .xls` /
`Gunakan template dari wizard ini agar kolom produk dan lokasi sudah terisi.`;
`[Download Template Belum Diisi]`-gate + `[Pilih File]`/`[Ganti File]` + `[Cek File]`!);
preview (badge `Import selesai`/`Siap diproses`/`Tidak ada baris baru`/`Perlu diperbaiki` +
`{valid} baris siap import, {lewati} dilewati, {error} error, nilai stok {Rp}`; notice
`Sebagian baris dilewati` + `Barang/lokasi yang sudah punya stok awal tidak diimport ulang
agar stok tidak dobel...` / `Stok dan draft keuangan tersimpan` + `{n} movement dibuat.
Draft Saldo Awal Keuangan tersinkron {Rp}. Lanjutkan ke Saldo Awal Keuangan untuk mengisi
tanggal mulai Finance sebelum posting.`; isu-12 + `Masih ada {n} pesan lain...` /
tabel-10-baris Barang/Lokasi/Qty/Harga-Modal/Nilai!); error `Perlu diperiksa`
(`Kode Lokasi dan Nama Lokasi wajib diisi.` / `Pilih lokasi untuk semua barang sebelum
download template.` / `Semua barang pada lokasi yang dipilih sudah punya stok awal. ...` /
`Gagal membuat template Excel.` / `Gagal membaca file stok awal.` /
`Gagal memproses stok awal.` / `Gagal memuat stok awal` / `Gagal membuat lokasi gudang.`).

## 11. Menu, guard & katalog teks

Menu `Stok & Gudang` (6, urut-kode): Stok (`/stock`, view) · Gudang (`/gudang`, view) ·
Transfer Cabang (`/gudang/transfer`, **transfer**!) · Pindah Lokasi (`/gudang/pindah-lokasi`,
**transfer**!) · Barang Rusak (`/gudang/damaged`, view-rute!) · Stok Awal (`/stock/opening`,
**adjust**!). Tanpa-menu: `/stock/movements` (view) · `/gudang/locations` (view-rute!) ·
`/stock/adjustments/create` (adjust!) · `/stock/:itemId` (view, **terakhir**!).
Tanpa-izin = rute-diblokir; tombol-aksi hilang per-izin (`Penyesuaian Stok`, `Pindah Lokasi`,
`Kelola`-handle, `Kembalikan`/`Rugi/Buang`, `Pindah dari Sini`). Barcode-scan: tombol
`Scan Barcode / QR` (aria-label!) + modal-QR → isi-pencarian. Toast-katalog: lihat
[feature-inventory.md](feature-inventory.md) F-12.2 (21-pesan!).
