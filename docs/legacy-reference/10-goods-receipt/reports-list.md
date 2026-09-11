# Reports List — Modul 10 Goods Receipt

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diverifikasi dari service,
controller, FE, dan E2E.

---

## 1. Keputusan: modul ini TIDAK punya laporan

Tidak ada endpoint rekap, ekspor, halaman rekap, maupun agregasi penerimaan: `goods-receipts/list`
adalah daftar operasional per cabang/order (dengan paginasi), bukan rekap; kartu riwayat adalah
tabel per PO; export order (modul 08) tidak memiliki kolom/sheet penerimaan.

## 2. Angka operasional (kontrak tampil)

`Diterima X / Y {uom}` per baris PO (kumulatif `receiptSummary.byOrderItem`) · tabel riwayat
(Waktu/pencatat/SJ/lokasi/item/catatan) · `meta.total` list.

## 3. Bahan modul lain

Batch + movement + tanggal menjadi dasar: saldo Stock, basis biaya & posting Finance (HPP/PPN),
kesiapan retur-beli, badge Diretur (modul 08/13). Kebenaran rekap bergantung snapshot UOM +
`movedAt` = tanggal-terima (backdate ikut — KI-80).

## 4. Yang eksplisit BUKAN laporan

- `goods-receipts/list` (operasional, ber-relasi, tanpa agregat).
- Kotak `Total item purchase order` (ringkasan dialog, sesaat).
- Toast `fully_received` (status respons, bukan dokumen).

