# Reports List — Modul 11 Delivery / Pengiriman

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diverifikasi dari service,
controller, halaman, dan E2E.

---

## 1. Keputusan: modul ini TIDAK punya laporan berkas

Tidak ada export/rekap SJ: antrean adalah daftar kerja operasional (dengan limit + filter),
bukan laporan; daftar SJ per order ber-relasi penuh tanpa agregat; cetak SJ adalah dokumen per
SJ (lihat §2), bukan rekap.

## 2. Dokumen operasional (kontrak angka)

| Dokumen | Angka mengikat |
|---|---|
| Cetak SJ | Qty per baris (UOM transaksi) + STATUS bayar + Staff/USER + karbon; tanpa harga apa pun |
| Kartu antrean | Progress `{kirim}/{total}` + sisa + umur + arsip-flag |

## 3. Angka operasional

`{n}` tab berhitung · `Progress SJ` · `Sisa {n} item / {q} qty` · `Ada SJ menunggu kembali` ·
Umur (`Hari ini`/`{n} hari`) · `{n} bukti tersimpan` · `meta` n/a (array mentah list).

## 4. Bahan modul lain

Dispatch + tanggal-kirim + alokasi → saldo/HPP/finance; tanggal-serah → net-sales dashboard;
nomor-SJ + sopir/plat → export order (modul 08); snapshot → retur-jual (12); arsip → bukti
audit (20).

## 5. Yang eksplisit BUKAN laporan

- Antrean (tanpa audit, limit 200, bukan berkas).
- `deliveries/list` (array relasional per order).
- Modal Bukti (pratinjau arsip, bukan rekap).
