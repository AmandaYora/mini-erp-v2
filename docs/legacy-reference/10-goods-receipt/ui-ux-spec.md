# UI/UX Spec — Modul 10 Goods Receipt

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Modul ini **tanpa screen
sendiri**: seluruh UX hidup di `/orders/:orderId` (dialog + kartu) memakai izin `order.*`.
Spesifikasi widget penuh dialog ada di modul 08 (ui-ux §3); di sini kontrak sisi-dokumen +
keadaan. Bagian ambigu ditandai **[PERLU KONFIRMASI]**.

---

## 1. Dialog Terima Barang (ringkas; penuh di modul 08)

Badge kuning `Terima Barang` + judul + tutup `×` (`Tutup dialog`) → deskripsi dinamis (net vs
COD-keputusan) → `No. Surat Jalan Supplier` (`Opsional`) + `Tanggal Terima` (`datetime-local`)
→ `Lokasi cepat untuk semua item` (required + helper pilah) → `Barang diterima dan lokasi`
(label `{nama} sisa {n} {uom}`; input `max/min/step`, dijepit 0…sisa; `Lokasi simpan` per baris;
kosong → `Semua item pada purchase order ini sudah diterima.`) → kotak biru `Total item
purchase order` (`• {nama} +{qty} {uom} ({base} stok)`, tracked saja) → `Catatan penerimaan` →
keputusan COD kondisional (Bayar: total readonly + Tanggal Bayar + Metode
Tunai/Transfer/Cek-Giro + referensi + catatan + Bukti `(opsional, JPG/PNG, maks 5MB)` +
`{nama} — akan dikompres sebelum disimpan`; Tempo: `Jatuh Tempo Baru` + Total Utang; tanpa
izin bayar: teks penjelasan, hanya Tempo) → tombol 4 varian + 3 syarat kunci.

Tombol pembuka (detail PO): `[Terima Barang]` primer (syarat: `order.update` + purchase +
belum-tanggal + ada-transisi-selesai + bukan-prepaid-berutang) else teks redup
`Diterima {datetime}`. Tanpa tombol = tanpa jalan (tanpa rute khusus terima).

## 2. Kartu Riwayat Penerimaan Barang

Judul + deskripsi `Batch penerimaan supplier dan lokasi barang masuk.` Kosong → notice `Belum ada penerimaan` /
`Purchase order ini belum memiliki batch penerimaan barang.` Tabel tanpa paginasi/aksi/cetak:
Waktu (+`oleh {nama}`) · Dokumen Supplier (`-`) · Lokasi (header → item-pertama → `-`) ·
Item Diterima (`{nama}: {qty} {uom}`, tiap baris penerimaan) · Catatan (`-`).

## 3. Umpan balik (toast slice `receiveGoods`)

Sukses `Barang berhasil diterima` + 1 dari 5 deskripsi (bayar/tempo/selesai-otomatis/parsial/
lunas-dulu/fallback — teks di modul 08 §F-04.5); gagal `Gagal menerima barang` + pesan server.
Daftar + stok me-reload; detail me-reload; bukti menyusul (gagal tak membatalkan).

## 4. Yang terlihat dari dokumen (format mengikat)

- Identitas penerimaan di UI = **tanggal + SJ supplier + lokasi** (tanpa nomor dokumen).
- Qty tampil UOM transaksi; kotak biru menambah padanan base bila faktor ≠ 1.
- `Diterima X / Y {uom}` per baris PO = kumulatif seluruh batch (sumber: `receiptSummary`).
- Tanpa status per penerimaan (final vs parsial hanya di toast + tanggal PO + status PO).

