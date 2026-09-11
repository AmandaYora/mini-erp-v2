# Test Cases — Modul 10 Goods Receipt

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`order.service.spec.ts` (bagian receive: movement, multi-lokasi, tahap, terminal), E2E 05
(3 skenario ujung-ke-ujung). **Tanpa spec**: `createGoodsReceipt`, `listGoodsReceipts`,
`detailGoodsReceipt`. **GAP** = belum ada test.

---

## TC-G — Dokumen & efek (TC-G-01…TC-G-12)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-G-01 | Terima penuh | Movement + selesai + tanggal + history | Ya (spec) |
| TC-G-02 | Multi-lokasi satu batch | Saldo per lokasi | Ya (spec) |
| TC-G-03 | Bukan-purchase / terminal / tanpa-item / tanpa-status / tanpa-tahap | `404`/`400` ×4 | Ya (spec) |
| TC-G-04 | Parsial pilah-lokasi → null tanggal; over → `400 sisa`; sisa → tanggal + selesai; list ≥2; utang > 0 | E2E penuh | Ya (E2E 05-1) |
| TC-G-05 | Prabayar berutang → `prabayar\|lunas`; lunasi → penuh + stok | Blokir-cair | Ya (E2E 05-2) |
| TC-G-06 | COD 2+2 + `pay_now` → id bayar + saldo 0 | Keputusan akhir | Ya (E2E 05-3) |
| TC-G-07 | Alias (`order_id`, `stock_location_id`, `quantity_received`, `notes`) | Delegasi identik | GAP → KI-88 |
| TC-G-08 | List saring-order + urut + paginasi; detail by-id + relasi | Tepat / `404` | GAP → KI-88 |
| TC-G-09 | Header seragam vs campur; non-stok tanpa movement; saldo-baru-0 | Aturan tepat | GAP (sebagian) |
| TC-G-10 | SJ ganda + catatan-whitespace-null + backdate | Diizinkan / null / masa-lalu | GAP (KI-80) |
| TC-G-11 | Konkuren ganda melebihi sisa | Tanpa lock (beda retur-beli) | GAP → KI-92 E-18 |
| TC-G-12 | Tanpa nomor/edit/batal/cetak/halaman | Absen terverifikasi | GAP → KI-89/91 |

## GAP (G-01…G-05)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Alias + list + detail tanpa spec & (alias/detail) tanpa pemanggil | Mengunci KI-88 (buang atau uji) |
| G-02 | Konkuren sisa | Satu-satunya tulis-stok tanpa lock di commerce |
| G-03 | Identitas tanpa nomor | Mengunci KI-89 (butuh nomor atau tetapkan tanpa) |
| G-04 | Arsip mati + backdate | Mengunci KI-91 + KI-80 |
| G-05 | UI riwayat (lokasi-fallback, tanpa-aksi, toast-5) | Tanpa jaring FE khusus penerimaan |

