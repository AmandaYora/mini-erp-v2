# Reports List — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diverifikasi dari service,
controller, halaman, dan spec.

---

## 1. Keputusan: modul ini TIDAK punya laporan berkas

Tanpa export/rekap: daftar + detail + preview operasional. Efek ke utang (potong + refund +
kredit) dihitung modul 08 dari total + settlement (bukan laporan sini).

## 2. Angka operasional

3 kartu detail (Nilai/Penyelesaian/Status-teks) · kolom Nilai + badge `Order dibatalkan` ·
live bar (`unit_price × qty`) · preview server (pajak-penuh) · badge `Diretur penuh` di daftar
order (modul 08 membaca `cancels_order`).

## 3. Bahan modul lain

Nilai + settlement + tanggal → utang-efektif order + sumber posting (HPP/selisih akun 5300 —
migrasi 050!); stok + movement → saldo/HPP; penuh → cancelled; audit → riwayat.

## 4. Yang eksplisit BUKAN laporan

- Preview/konteks (sesaat/inisialisasi) · notice-batal (status) · live bar (tanpa pajak).
