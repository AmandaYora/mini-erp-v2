# UI/UX Spec — Modul 11 Delivery / Pengiriman

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Rute: `/delivery-work-queue`
(`order.view`, menu Operasional → Pantauan Surat Jalan), `/orders/:orderId/delivery/:sjId/print`
(`authenticated` — tanpa permission), seksi Surat Jalan di `/orders/:orderId`. Bagian ambigu
ditandai **[PERLU KONFIRMASI]**.

---

## 1. Screen `/delivery-work-queue` — Pantauan Surat Jalan

**Header**: breadcrumb `Dashboard > Pantauan Surat Jalan`; judul sama; deskripsi `Antrian kerja
untuk menerbitkan SJ dan memantau SJ fisik yang belum kembali.` (tanpa aksi).

**Tab berhitung**: `Buat SJ ({n})` / `Menunggu SJ Kembali ({n})` (`tab` di URL; default Buat).

**FilterBar** (semua di URL): `Cari` (`delivery-work-search`, `No order, no SJ, atau customer`,
debounce 350ms + trim) · `Dari tanggal` / `Sampai tanggal` (`date`) · `Umur SJ`
(Semua/Hari ini/> 2 hari/> 7 hari; hanya berefek di tab Tunggu) · `[Reset filter]` (ghost, bila
ada filter; menyisakan tab). Gagal: notice `Pantauan tidak dapat dimuat` +
`Gagal memuat pantauan surat jalan. Coba ulangi atau periksa koneksi.`; loading
`Memuat pantauan surat jalan...`.

**Tabel Buat** (`Order Siap Dibuatkan SJ` / `Urutan diprioritaskan dari order paling lama agar
antrian pengiriman tidak tertahan.`): Order (nomor + customer/`Pelanggan umum`) · Tanggal Order
(`{tgl}` + `Jatuh tempo pembayaran {tgl}` bila ada) · Status (kuning) · Progress SJ
(`{kirim}/{total} item dikirim` tebal + `Sisa {n} item / {q} qty` + `Ada SJ menunggu kembali`
bila ada) · Termin (Prabayar/Tempo/Bayar Saat Serah) · Aksi (`[Buat SJ]` kecil bila
`order.update` → `?action=create-sj`; `[Lihat Order]` secondary). Kosong: `Tidak ada order yang
perlu dibuatkan SJ` / `Semua sales order aktif pada filter ini sudah dibuatkan SJ atau belum siap
dikirim.`

**Tabel Tunggu** (`SJ Menunggu Kembali` / `Urutan diprioritaskan dari tanggal kirim paling lama
agar arsip fisik segera ditutup.`): Surat Jalan (nomor + order) · Customer · Tanggal Kirim
(tanggal + dispatch-datetime) · Umur (badge merah >7 else kuning: `Hari ini`/`{n} hari`) ·
Sopir/Kendaraan (nama + plat bila ada) · Arsip (`Arsip tersimpan` hijau / `Belum ada arsip SJ`
kuning) · Aksi (`[Cetak SJ]` tab-baru + `[Konfirmasi]` secondary bila `order.update` →
`?action=confirm-sj&delivery_note_id=` + `[Lihat Order]` ghost). Kosong: `Tidak ada SJ yang
menunggu kembali` / `Semua surat jalan pada filter ini sudah dikonfirmasi atau belum
diterbitkan.`

## 2. Seksi Surat Jalan (detail order)

Kartu `Surat Jalan` / `Daftar pengiriman dan konfirmasi serah barang ke customer.` + tombol
`Terbitkan Surat Jalan` (secondary; non-terminal + kelola + sisa). Notice kuning bila
habis-tapi-aktif (`Semua barang sedang dalam pengiriman` / `Tunggu sopir membawa kembali surat
jalan fisik, lalu unggah arsip SJ sebelum konfirmasi.`). `Memuat...` / kosong (`Belum ada
pengiriman` / `Terbitkan surat jalan pertama untuk memulai proses pengiriman ke customer.` +
tombol).

**Tabel SJ**: No. SJ (+tanggal-kirim) · Sopir (+plat) · Petugas Gudang · Item Dibawa
(`{nama[-varian]}: {qty} {uom}` + ` (lokasi: qty, ...)` bila alokasi) · Dikirim (datetime) ·
Status (`Aktif` kuning / `Selesai - Tanpa TTD` kuning / `Selesai - TTD Penerima` hijau /
`Selesai` hijau + `{n} bukti tersimpan`) · Aksi (`[Konfirmasi Kembali]` primer + `[Bukti]` +
`[Cetak]` + `[Batalkan]` merah; dua terakhir manapun aktif + kelola; konfirmasi aktif saja).

**Dialog Terbit** (di detail; badge? — tanpa badge, judul dari konteks; field §F-01.2; footer
`[Batal]` + `[Terbitkan & Cetak]`/`Memproses...` disabled-tanpa-item/saat-kirim; sukses buka
cetak otomatis).

**Modal Konfirmasi** (640px, badge kuning `SJ Kembali`, kunci-tutup): notice info `{nomor}` +
`Upload foto atau scan surat jalan fisik yang kembali dari sopir.` · Arsip SJ Fisik
(`(JPG/PNG/WebP, maks 5MB)`; `*` bila belum tersimpan; `{nama} - akan dikompres sebelum
disimpan` / `Arsip SJ sudah tersimpan. Upload file baru hanya jika perlu menambah bukti.`) ·
`Status TTD Penerima *` (radio `Ada TTD penerima` / `Tidak ada TTD penerima`) · Nama Penerima
(`Opsional`) vs Alasan* (placeholder contoh teras) · Foto Lokasi (`(opsional)`) · Catatan
Lokasi (`Opsional`) · `[Batal]` + `[Konfirmasi Selesai]`/`Memproses...`.

**Modal Bukti** (720px, badge hijau `Bukti`, judul `Bukti Surat Jalan`): nomor + notice kuning
alasan-tanpa-TTD + catatan-drop + tombol per bukti (`Arsip SJ`/`Foto Lokasi`) + `Memuat
bukti...` → gambar (`Bukti surat jalan`, 70vh).

**Dialog Batal** (`Batalkan Surat Jalan?` / `...stok ... dikembalikan ke gudang.` /
`Ya, Batalkan SJ`; gagal → tetap terbuka).

## 3. Cetak SJ (tab baru, login saja)

Toolbar: radio `Kertas A4 (biasa)` / `Kontinu 3-Ply (Dot-Matrix)` + checkbox preprinted
(kontinu) + Notes-hanya-cetak + `[Cetak / Print]` + `[Tutup]`. Isi: kop + `SURAT JALAN` + nomor;
pihak (Nama, Penerima bila beda, No HP, Alamat); Tanggal Kirim / No Nota / Tanggal Nota / Staff
(pembuat) / Kendaraan (bila ada); tabel No/Qty/Satuan/Produk/Keterangan-kosong; `STATUS :
{Lunas|Bayar Sebagian|Bayar Saat Serah (COD)|Tempo|Belum Lunas}`; Catatan SJ; kota + 3 kotak
(Petugas Gudang {nama} / Sopir {nama} / Pembeli {nama}); footbar waktu + `PUTIH : SOPIR
MERAH : PELANGGAN` + `USER : {pencetak}`. A4 landscape + ganjal 5; kontinu tanpa. Tanpa SJ:
`Surat jalan tidak ditemukan.`; loading `Memuat...`.

## 4. Format yang mengikat

- Umur SJ dihitung hari-kalender lokal (`Hari ini`/`{n} hari`; merah >7).
- Qty SJ `id-ID`; tanggal kirim `formatLongDate`; dispatch `formatDateTime`.
- `Penerima` hanya bila beda dari nama pihak; plat/kendaraan kondisional.
- Konvensi karbon `PUTIH : SOPIR   MERAH : PELANGGAN` terkunci (keputusan bisnis, bukan bug).
- Shortcut `?action=` sekali-pakai (`replace`), diam bila tak-syarat.
