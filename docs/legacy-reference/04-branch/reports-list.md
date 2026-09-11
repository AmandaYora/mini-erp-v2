# Reports — Modul 04 Branch (Multi-Cabang)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.**

---

## 1. Kesimpulan

**Modul Cabang tidak menghasilkan satu pun laporan.** Tidak ada layar laporan, tidak ada ekspor,
tidak ada cetak, tidak ada agregasi angka di dalam modul ini. Halaman `/branches` adalah daftar
master data biasa.

Namun **cabang adalah sumbu utama hampir seluruh laporan sistem**. Bagian di bawah ini
mendokumentasikan peran itu, karena rebuild harus mempertahankan cara cabang membatasi angka.

---

## 2. Peran Cabang di Laporan Modul Lain

### 2.1 Sumbu pembatas bawaan: cabang aktif

Sebagian besar layar berangka (dashboard, laporan penjualan, stok, pengiriman, pembayaran)
**tidak punya pemilih cabang**. Angkanya terbatas pada **cabang aktif sesi**, yang diambil dari
sesi — bukan dari filter di layar.

Konsekuensi yang harus dipertahankan: untuk melihat angka cabang lain, pengguna **berganti cabang
aktif** lewat chip topbar, bukan mengganti filter. Tidak ada layar yang menjumlahkan seluruh
cabang menjadi satu angka perusahaan.

**[PERLU KONFIRMASI]** Apakah pemilik sistem pernah membutuhkan angka gabungan seluruh cabang
(mis. total penjualan perusahaan) dan selama ini menjumlahkannya manual? Ini kesenjangan produk,
bukan bug.

### 2.2 Layar yang punya pemilih cabang eksplisit

| Layar | Cara cabang dipakai |
|---|---|
| **Mutasi Stok Antar-Cabang** | Dropdown cabang asal dan tujuan, berlabel `Nama - Kota`. Cabang tujuan wajib berstatus aktif |
| **Saldo Awal Persediaan** (Keuangan) | Dropdown cabang untuk tiap baris saldo awal; memanggil daftar cabang langsung |
| **Riwayat Aktivitas** | Kolom cabang per baris; cabang yang tidak dikenal tampil sebagai `Branch #<id>` |

Ketiganya memakai daftar cabang yang sama, sehingga **peran tanpa izin lihat cabang akan melihat
dropdown kosong** — lihat KI-47.

### 2.3 Metrik operasional harian

Satu-satunya agregasi yang mengiterasi cabang secara langsung adalah **job metrik harian**, yang
berjalan otomatis setiap hari pukul 01:00 waktu server.

| Aspek | Nilai |
|---|---|
| Cakupan | Satu baris metrik per **cabang per tanggal** |
| Cabang yang diproses | **Hanya yang berstatus aktif** |
| Tanggal yang dihitung | Hari kemarin |
| Angka yang dihitung | Jumlah order per kelompok status (menunggu, aktif, selesai, batal), total penjualan, dan jumlah barang berstok kritis |
| Nilai penjualan | Hanya order **penjualan** yang dihitung; order pembelian dihitung jumlahnya tapi tidak menambah nilai penjualan |
| Order terarsip | Tidak dihitung |
| Ketahanan | Bila satu cabang gagal dihitung, cabang lain tetap diproses |

**Aturan yang harus dipertahankan:** menonaktifkan cabang **menghentikan pengumpulan metrik
hariannya**, dan riwayat metrik cabang itu berhenti bertambah — tidak dihapus, tapi berlubang mulai
hari penonaktifan. Karena tidak ada UI untuk menonaktifkan cabang, efek ini belum tentu pernah
terjadi.

### 2.4 Nomor dokumen sebagai penanda cabang di laporan

Karena prefix nomor dokumen memuat kode cabang (`ORD-BLR/...`, `PAY-BLR/...`), setiap laporan yang
menampilkan nomor dokumen **secara tidak langsung menampilkan cabang asalnya**. Ini dipakai
pengguna sebagai cara membaca cabang dari lembar cetak.

Konsekuensi penting untuk laporan pajak/SPT: **mengubah kode cabang memutus konsistensi penanda
ini** — dokumen sebelum dan sesudah perubahan punya prefix berbeda meski dari cabang yang sama.
Lihat KI-40 dan [numbering-sequence.md](numbering-sequence.md).

### 2.5 Cabang pada dokumen cetak

Kop nota dan surat jalan menampilkan **nama cabang** dan **alamat cabang** aktif. Bila profil
dokumen di Pengaturan sudah diisi, nilai dari sana yang menang; alamat/kota cabang hanya jadi
cadangan.

Detail formatnya ada di [ui-ux-spec.md](ui-ux-spec.md) §4.3.

---

## 3. Yang Tidak Ada

| Laporan yang mungkin diharapkan | Kenyataan |
|---|---|
| Perbandingan performa antar-cabang | Tidak ada |
| Rekap penjualan seluruh cabang dalam satu layar | Tidak ada |
| Daftar cabang siap cetak / ekspor | Tidak ada |
| Riwayat perubahan data cabang sebagai laporan | Hanya lewat Riwayat Aktivitas umum (`branch.create`, `branch.update`) |
| Laporan pemakaian nomor dokumen per cabang | Tidak ada |
| Laporan cabang tanpa transaksi | Tidak ada |
