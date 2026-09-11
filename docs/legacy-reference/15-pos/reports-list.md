# Reports List — Modul 15 POS / Kasir

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diverifikasi dari halaman,
struk, dan E2E.

---

## 1. Keputusan: modul ini TIDAK punya laporan berkas

Tanpa rekap/POS-report/export: penjualan kasir tercatat sebagai order-POS biasa (laporan via
export order + finance, milik 08/17). Satu-satunya "rekap" = ringkasan cart sesaat (Subtotal/
Diskon/Total) + struk per transaksi.

## 2. Dokumen per transaksi (kontrak angka)

Struk thermal (§F-06 feature): 8 baris ringkasan + kembalian-bruto + rekening + kebijakan.
Simetris nota; stabil-cetak-ulang (payload-final!).

## 3. Angka operasional

`{n} produk ditemukan` · `{n} items` · Total-besar · `Sisa` (tempo) · Kembalian · umur? —
tanpa-umur (kasir tanpa-ledger!). Penjualan per-kasir = nama-pembuat (filter milik 08/17).

## 4. Bahan modul lain

Order-POS + bayar + tender + snapshot → export + saldo + finance + SPT + net-sales (tanggal =
terima-bayar ?? serah ?? order!).

## 5. Yang eksplisit BUKAN laporan

- Cart (sesaat, tanpa-id) · saran-cepat (tampil) · antrean? — tanpa-antrean (langsung!) ·
  pindai (aksi, bukan data).
