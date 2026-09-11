# User Flows — Modul 10 Goods Receipt

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** `@Post` + 200 + `{ data }` +
`BranchGuard`. Alur PO (buat → proses) = modul 08; di sini dari tahap terima. Bagian ambigu
ditandai **[PERLU KONFIRMASI]**.

---

## Matriks izin per aksi

| Aksi | Izin | Tanpa izin |
|---|---|---|
| Buka dialog + kartu riwayat | `order.update` (tombol) / `order.view` (kartu) | Tombol hilang |
| Kirim penerimaan (+ alias create) | `order.update` | 403 guard |
| Baca `goods-receipts/list|detail` | `order.view` | 403 guard |
| COD bayar-kini di penerimaan | + `payment.create` di sesi | 403 khusus; UI sembunyikan opsi |

## UF-01 — Terima parsial multi-lokasi (E2E 05)

PO tempo (100 btg + 20 pak, tahap terima) → dialog (sisa penuh, lokasi default) → qty 50 + 10 +
pilah sumber/tujuan + SJ `...-MOBIL-1` + catatan mobil → Parsial → stok +50/+10 beda lokasi;
tanggal null; status tetap; riwayat 1 baris (lokasi per baris). Over 60/50 → `400 sisa`.
Sisa → tanggal + selesai + history otomatis + toast selesai.

## UF-02 — Prabayar lunas-dulu (E2E 05)

Tombol disembunyikan + notice → bayar penuh (modul 14) → tombol muncul → terima penuh (SJ
`...-LUNAS`) → `fully_received` + stok +10.

## UF-03 — COD bayar-di-akhir (E2E 05)

Terima 2/4 (tanpa keputusan) → terima 2/4 + `pay_now` tunai → stok + payment `payable` +
selesai; saldo supplier → 0. Varian alih-tempo: UF-05 modul 08.

## UF-04 — Isi-otomatis & alias

Tanpa items → sisa-semua terisi (satu mobil penuh tanpa rinci); `goods-receipts/create` dengan
nama-alias (`order_id`, `stock_location_id`, `quantity_received`, `notes`) → hasil identik (tanpa
pemanggil produksi — kompatibilitas diam).

## UF-05 — Baca riwayat

Detail PO → kartu (seluruh batch; Waktu + pencatat + SJ + lokasi + item + catatan) → cocokkan
fisik vs sistem per batch. Tanpa aksi per baris (koreksi via adjustment modul 16).

## UF-06 — Alur gagal yang dirancang (ringkas)

| Pemicu | Respons |
|---|---|
| Tanpa sisa / tanpa tahap / bukan-PO / terminal | `400` tahap (dialog hanya jaga prepaid) |
| Over-sisa / duplikat / qty-0 / luar-order | `400` baris |
| Prabayar berutang / COD-final tanpa mode / mode di luar COD(-final) | `400` termin |
| Tanpa izin bayar COD | `403` khusus |
| Lokasi induk/nonaktif/hilang | `400` lokasi |
| Bukti gagal | Tetap sah |
| Konkuren ganda | Tanpa lock (→ KI-92 E-18) |

