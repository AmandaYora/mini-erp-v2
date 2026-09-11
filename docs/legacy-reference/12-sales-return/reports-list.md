# Reports List — Modul 12 Sales Return (Retur Penjualan)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diverifikasi dari service,
controller, halaman, dan E2E.

---

## 1. Keputusan: modul ini TIDAK punya laporan berkas

Tidak ada export/rekap retur: daftar + detail + preview adalah operasional. Satu-satunya angka
agregat implisit — efek finansial ke order (total & bayar-efektif) — dihitung modul 08/09 dari
`difference_amount` + settlement (bukan laporan modul ini).

## 2. Angka operasional (kontrak tampil)

4 kartu detail (Nilai Retur/Pengganti/Selisih/Penyelesaian) · kolom daftar (Retur/Pengganti/
Selisih tebal/Penyelesaian) · live bar lengket (proporsional-bulat) · preview server (final) ·
tabel settlement · badge kirim-pengganti.

## 3. Bahan modul lain

Selisih + settlement + tanggal → penyesuaian piutang order + sumber posting finance; stok +
movement + modal → saldo/HPP; SJ pengganti → arsip + cetak (11); audit → riwayat (20).

## 4. Yang eksplisit BUKAN laporan

- Preview (rencana sesaat, tanpa id) · konteks (inisialisasi form) · kartu SJ pengganti
  (status kirim) · `[Cetak]` halaman (= print browser).
