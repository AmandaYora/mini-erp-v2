# Reports List — Modul 08 Order Purchasing

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Laporan milik modul ini +
logika perhitungan. Dari `order-export.service.ts`, modal export, E2E 16 (dilaporkan di peta
modul; perilaku umum), dan angka daftar/detail.

---

## 1. Laporan milik modul: Export Order (`orders/export`, izin `order.export`)

Satu-satunya berkas laporan. Memakai BranchGuard (cabang sesi); input `order_kind` all/sales/
purchase, `date_from` + `date_to` (`YYYY-MM-DD` wajib), `format` xlsx/pdf.

**Validasi (urutan):** jenis invalid → `Jenis order export tidak valid`; format invalid →
`Format export tidak valid`; tanggal kosong/format-salah → `{Tanggal awal|Tanggal akhir} {wajib
diisi dengan format YYYY-MM-DD|tidak valid}`; awal > akhir → `Tanggal awal tidak boleh melebihi
tanggal akhir`; > 366 hari → `Range export maksimal 366 hari`; > 5000 order →
`Data export melebihi 5000 order. Persempit periode export.` (tanpa file parsial — tolak utuh).

**Cakupan:** order non-arsip cabang dalam `[dari 00:00:00, sampai 23:59:59]` (tanggal order),
opsional filter kind. File: `report-order_{semua|penjualan|pembelian}_{dari}_{sampai}.{xlsx|pdf}`.

**Sheet Excel:** `Ringkasan` (3 kolom Nilai+Keterangan; konteks perusahaan/cabang/filter) ·
`Daftar Order` (22 kolom + autofilter + format angka: nomor, jenis, sumber, tanggal, jatuh tempo,
kode/nama pihak, status, termin (+label), status finansial (+label), subtotal, diskon, sebelum
pajak, pajak, total, dibayar, sisa, metode, kasir, no faktur pajak, **no invoice supplier**,
sopir, plat, catatan) · `Detail Item` (18 kolom + autofilter: order, tanggal, pihak, kode/nama/
varian produk, qty, UOM transaksi, UOM base, faktor, qty-base, harga, diskon, pajak, total,
sumber harga, nama member). Nilai dari snapshot (stabil walau master berubah); sopir/plat dari SJ
non-arsip (kosong = belum dicatat).

**PDF:** ringkas siap-print (ringkasan + baris order + kolom sopir/plat padat); detail item penuh
hanya di Excel.

**Ringkasan terhitung (keduanya):** jumlah order selesai + bruto + penyesuaian retur + neto
(`DashboardSalesSummary`) dan estimasi margin (modul Finance memakai biaya; rumus milik 17 —
[PERLU KONFIRMASI] batas tepat modul ini vs Finance untuk margin di export).

## 2. Angka operasional (bukan berkas, tetapi kontrak tampil)

| Angka | Lokasi | Rumus |
|---|---|---|
| `{n} order pada filter aktif` / `{n} order` | Daftar | `meta.total` (cabang + non-arsip + filter) |
| Dibayar / Total | Kolom daftar | dibayar (langsung+alokasi) / total_setelah_retur |
| Badge Lunas/Sebagian/Jatuh Tempo/Belum Bayar | Status | BR-36 |
| `Diretur penuh/sebagian` | Status (detail; daftar mati — E-30) | BR-37 |
| `Diterima X / Y` | Baris detail | kumulatif penerimaan vs qty order |
| Total Utang / Total pembayaran | Dialog terima | sisa order / sisa (readonly) |

## 3. Bahan untuk laporan modul lain

PO + penerimaan + snapshot pajak/UOM + bayar + retur menjadi sumber posting Finance (jurnal,
HPP, PPN, rekap SPT) dan saldo payable Payment. Kontrak snapshot (BR-15/BR-30) menjaga rekap
tetap benar walau master berubah.

## 4. Yang eksplisit BUKAN laporan

- **Riwayat Penerimaan Barang**: tabel operasional per PO (bukan rekap lintas PO).
- **`receiptSummary`/`returnSummary`**: peta bantu UI (`byOrderItem`, badge), tanpa periode/agregat.
- **Nomor SJ supplier**: teks bebas per penerimaan (bukan penomoran — modul ini tak menerbitkan
  nomor penerimaan).
