# Reports List — Modul 09 Order Sales

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Mekanika export = modul 08;
di sini kontrak sisi-sales + dokumen cetak (laporan per-dokumen).

---

## 1. Export (filter Penjualan)

Sama: validasi, batas 366 hari/5000, file `report-order_penjualan_...`, sheet Ringkasan/Daftar/
Detail. Sisi-sales: kolom sopir/plat terisi dari SJ non-arsip (kosong = belum dicatat); kolom
member terisi dari snapshot; ringkasan dashboard (selesai/bruto/retur/neto) + estimasi margin
dihitung dari subset sales (rumus margin milik 17 — M8-Q3).

## 2. Dokumen cetak = laporan per order (kontrak angka)

| Dokumen | Angka mengikat |
|---|---|
| Nota/Tagihan | Jumlah kotor (penuh × qty) + Diskon + Pajak + Total + Sudah Dibayar + Sisa (`max(0, Total − Bayar)`) + terbilang total-tampil; mode sembunyi: bruto + sisa-riil |
| Bukti Bayar | Nominal + sisa-sesudah-bayar-itu |
| Struk POS | Thermal 80mm; `discount_as_price` opsional (milik 15) |

## 3. Angka operasional

`{n} order` (filter) · Dibayar/Total kolom · badge bayar · `Diserahkan {waktu}` ·
`Diterima...` n/a sales · Total/Sisa seksi bayar · placeholder jumlah = sisa ·
`discount_as_price` checkbox POS bila hemat/diskon.

## 4. Bahan modul lain

Snapshot sales (harga/diskon/member/pajak/UOM/ship-to) + bayar + serah-tanggal + retur →
posting Finance, saldo Payment, net-sales Dashboard, rekap SPT. Aturan cetak (penuh, bruto,
sisa-riil) menjaga nota cocok dengan rekap.

## 5. Yang eksplisit BUKAN laporan

- Riwayat Status (timeline operasional) · tabel bayar (riwayat) · Notes cetak (sementara) ·
  `amount_tendered`/kembalian (tampil, tak tersimpan).
