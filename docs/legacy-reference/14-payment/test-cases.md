# Test Cases — Modul 14 Payment / Pembayaran

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`payment.service.spec.ts` (549 baris: over/desimal/batal/FIFO/arsip/ringkasan/bukti),
`payment-receipt-print-page.test.tsx`, E2E 07 (FIFO dua sisi + bukti + over-tolak + arsip),
E2E 22 (arsip-tertagih), E2E 05/06 (COD/bayar). **GAP** = belum ada test.

---

## TC-P — Buat & alokasi (TC-P-01…TC-P-10)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-P-01 | Over-sisa order | `400 Maksimal` | Ya (spec) |
| TC-P-02 | Desimal | `400 bulat` | Ya (spec) |
| TC-P-03 | Order-batal langsung | `400` | Ya (spec) |
| TC-P-04 | 27rb → 5×10rb | FIFO 10+10+7 (order 1–3!) | Ya (E2E 07 dua sisi) |
| TC-P-05 | Lunasi sisa transfer | Saldo 0 | Ya (E2E 07) |
| TC-P-06 | Arsip tetap-ditagih + dilunasi | Lolos + saldo 0 | Ya (E2E 22 + spec) |
| TC-P-07 | Tanpa order + tanpa pihak | `400 Pilih...` | GAP (kode saja) |
| TC-P-08 | Manual salah (luar/0/over/total) | 4 pesan | GAP (tanpa E2E manual!) |
| TC-P-09 | Sisi-sampah / bentrok-dua-arah | `400` jenis/sisi | GAP |
| TC-P-10 | Tanpa-tunggakan dua sisi | 2 pesan | GAP |

## TC-A — Arsip, bukti, ringkasan, cetak (TC-A-01…TC-A-10)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-A-01 | Prepaid/COD-selesai jadi-berutang | `400` arsip | Ya (spec ×2) |
| TC-A-02 | Target hilang | `404` | Ya (spec) |
| TC-A-03 | Arsip → saldo kembali 50rb | Tepat | Ya (E2E 07) |
| TC-A-04 | Bukti: tipe-sampah / hilang / simpan-kunci / URL / tanpa-path | Tepat | Ya (spec ×5) |
| TC-A-05 | Bukti E2E: path-id + URL-signed | Tepat | Ya (E2E 07) |
| TC-A-06 | Ringkasan: overdue / abaikan-tempo-prepaid / efek-retur | Tepat | Ya (spec ×3) |
| TC-A-07 | Cetak: akumulasi + sisa-sesudah + status + tanpa-bayar | Tepat | Ya (FE test) |
| TC-A-08 | Over-99jt supplier | `/alokasi\|saldo\|tagihan\|hutang/` | Ya (E2E 07) |
| TC-A-09 | Net-terminal arsip-bebas; non-terminal jadi-berutang bebas | Lolos | GAP (sisi longgar!) |
| TC-A-10 | Ganti-bukti-timpa; URL-kedaluwarsa; cetak-arsip-bayar | Timpa/ditolak/? | GAP |

## GAP (G-01…G-06)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | Alokasi manual END-TO-END | Tanpa UI + tanpa E2E (mati-separuh?) |
| G-02 | Sisi longgar arsip (non-terminal/net-terminal) | Batasan jelas tanpa jaring |
| G-03 | Ganti-timpa bukti + yatim-storage | Data hilang tanpa jaring |
| G-04 | Validasi tanggal/metode + ambang-0.009 + walk-in-tak-tertagih | Aturan longgar tanpa jaring |
| G-05 | UI seksi (pecahan-buang, referensi-auto, sembunyi-hapus) | Tanpa FE test khusus bayar |
| G-06 | Generator-kembar-3 + prefix-fosil + darurat | Duplikasi tanpa jaring |
