# UI/UX Spec — Modul 14 Payment / Pembayaran

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Tanpa halaman sendiri: seksi
di detail order + kartu finance (milik 17) + cetak bukti. Form/dialog milik 08/09 dirujuk.
Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. Seksi Pembayaran (detail order)

Kartu `Pembayaran` / `Riwayat pembayaran dan posisi finansial order ini.` + badge Termin +
`[Ubah ke Tempo]` (ghost; syarat modul 09) + `[+ Catat Pembayaran]` (secondary; create + tutup
+ sisa > 0). 4 kotak (Total Tagihan/Kewajiban · Sudah Dibayar · Sisa Piutang/Utang
merah/hijau · Status Finansial badge Lunas/Sebagian/Jatuh Tempo/Belum Bayar). Kosong:
`Belum ada transaksi pembayaran` + varian supplier/customer.

**Form** `Catat Pembayaran Baru`: Tanggal Bayar* (kini) · Jumlah (Rp)* (teks-numerik,
pecahan-dibuang, placeholder = sisa) · Metode (Tunai/Transfer/Cek-Giro, cari) · No. Referensi
(auto `REF-...`, bisa-ubah) · Catatan · Bukti (`(JPG/PNG, maks 5MB)` + nama-kompres) →
`[Batal]` + `[Simpan Pembayaran]` (`Menyimpan...`); sukses toast + tutup + refresh; bukti
menyusul (gagal tak membatalkan).

**Tabel**: No. Bayar (tebal) · Tanggal · Metode (badge) · No. Referensi (`-`) · Jumlah (kanan
tebal) · Bukti (`Lihat` kecil-brand / `—`) · Aksi (`Cetak` tab-baru · `Edit` ganti-bukti
(create) · `Hapus` merah (syarat arsip)). Loading `Memuat...`; gagal toast.

**Modal bukti**: `Edit Bukti Pembayaran` / `Perbarui lampiran bukti transfer untuk transaksi
ini.` + ringkasan (No/Tanggal/Jumlah-brand/Metode) + `Lampirkan Bukti Baru (JPG/PNG, maks
5MB)` + notice timpa + file. **Viewer**: overlay gelap + `✕`/`Tutup` + Escape + gambar
(`Bukti Pembayaran`, 85vh). **Hapus**: `Hapus pembayaran?` / `...tidak bisa dibatalkan.` /
`Hapus Pembayaran`.

## 2. Kartu finance (milik 17, kontrak data)

`payments/party-balances` ×2 sisi + `party-ledger` → dialog `Rincian {nama}`. Detail milik 17;
kontrak respons di BR modul ini.

## 3. Cetak bukti (tab baru)

Toolbar kertas + Cetak/Tutup; kop status (`Lunas`/`Ada Sisa`) + tanggal; pihak (arah-beda
sales/purchase; `Pelanggan umum`/`Supplier`); rincian (order/tanggal/metode/referensi); tabel
4 baris akumulatif; catatan + kalimat verifikasi. Tanpa bayar: pesan khusus; loading khusus.

## 4. Format yang mengikat

- Label sisi: Tagihan/Kewajiban · Sudah Dibayar (Customer/Supplier) · Sisa Piutang/Utang ·
  Termin (Prabayar/Tempo/Bayar Saat Terima/Serah) · Finansial 4 status.
- Metode selalu Indonesia (Tunai/Transfer Bank/Cek-Giro) di form/tabel/cetak.
- Jumlah kanan-tebal; tanggal datetime; referensi `-` bila kosong.
- Status cetak = sisa-sesudah (historis-stabil); akumulasi = urut-tanggal.
