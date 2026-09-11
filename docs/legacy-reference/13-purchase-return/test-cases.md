# Test Cases — Modul 13 Purchase Return (Retur Pembelian)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`purchase-return.service.spec.ts` (396 baris, harness stub per-SQL) + E2E 17
(`17-revision-blackbox`: loco retur-beli — tombol, sebagian, penuh-batal, badge, detail).
E2E 08 nol menyentuh modul ini (klaim peta 00 perlu koreksi: cakupan ada di 17, bukan 08).
FE unit test nol. **GAP** = belum ada test.

---

## TC-P — Rencana & simpan (TC-P-01…TC-P-14, spec)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-P-01 | Bukan-purchase / arsip / belum-terima | `400/404` ×3 | Ya (spec) |
| TC-P-02 | Over (terima − sudah-retur) / duplikat-item | `400` ×2 | Ya (spec) |
| TC-P-03 | Tanggal < terima / > kini+1mnt | `400` ×2 | Ya (spec) |
| TC-P-04 | Lokasi non-daun / available-kurang (onHand cukup!) | `400` ×2 | Ya (spec) |
| TC-P-05 | Saldo kurang + movement `out` satu transaksi | Tepat | Ya (spec) |
| TC-P-06 | Nomor sequence + naik | Tepat | Ya (spec) |
| TC-P-07 | Sebagian (order tetap) vs penuh (cancelled) | Tepat | Ya (spec) |
| TC-P-08 | Penuh tanpa status-cancelled | Tetap + dibiarkan | Ya (spec) |
| TC-P-09 | Refund tanpa/dengan izin | 403 / lolos | Ya (spec ×2) |
| TC-P-10 | Sebagian-UI → preview → simpan → detail + badge | Tepat | GAP (kode saja; tanpa FE test sama sekali!) |
| TC-P-11 | Penuh multi-dokumen (retur-1 6/10 + retur-2 4/10 → batal) | Kumulatif lintas dokumen | GAP |
| TC-P-12 | Refund/kredit-END-TO-END (kas + saldo-supplier) | Settlement + efek | GAP (tanpa E2E!) |
| TC-P-13 | Lokasi-terima-terakhir default + timpa manual | Tepat | GAP |
| TC-P-14 | Metadata tanpa-`idOrderItem` (margin aman) | Absen terverifikasi | GAP (komentar tanpa jaring!) |

## TC-E — E2E 17 loco retur-beli (TC-E-01…TC-E-05)

| ID | Skenario | Kunci | Sudah ada |
|---|---|---|---|
| TC-E-01 | Tombol `Retur ke Supplier` → URL pra-isi `?order_id=` tanpa error | Pintu masuk | Ya (E2E 17) |
| TC-E-02 | Sebagian 4/10 via API (payload = form): `cancels_order` false, stok 10→6, utang turun | Sebagian | Ya (E2E 17) |
| TC-E-03 | Baris order: `Selesai` + `Diretur sebagian` | Badge daftar | Ya (E2E 17 — **bertentangan dengan KI-77 → KI-102!**) |
| TC-E-04 | Sisa 6/10: `cancels_order` true, stok 0, utang 0, baris `Dibatalkan` + `Diretur penuh` | Penuh-batal | Ya (E2E 17) |
| TC-E-05 | Detail merender `Order pembelian dibatalkan` tanpa error | Detail | Ya (E2E 17) |

## GAP (G-01…G-06)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Kontradiksi badge-daftar (E2E vs `summary_only`) + klaim peta 08 | Mengunci KI-102 (jalankan E2E!) |
| G-02 | Tanpa FE test sama sekali | Tiga halaman + modal tanpa jaring |
| G-03 | Metadata-tanpa-id (margin) | Guard kritis tanpa jaring |
| G-04 | Kumulatif-penuh multi-dokumen + lock-klik-ganda | Kontrak batal tanpa jaring integrasi |
| G-05 | Lokasi-terakhir + non-tracked + tanggal-batas | Jalur varian tanpa jaring |
| G-06 | Nomor + izin `cancel` mati + akun-5300 | Kontrak presisi tanpa jaring |
