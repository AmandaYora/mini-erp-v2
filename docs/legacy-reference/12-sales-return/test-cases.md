# Test Cases — Modul 12 Sales Return (Retur Penjualan)

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** **Sudah ada** =
`sales-return.service.spec.ts` (142 baris: settlement 6 + penundaan 4 + modal-guard 2),
`sales-returns-page.test.tsx` (150 baris: create ..., E2E 18 (pengganti penuh). **GAP** = belum ada.

---

## TC-R — Settlement & tunda & modal (TC-R-01…TC-R-12, unit)

| ID | Input | Output | Sudah ada |
|---|---|---|---|
| TC-R-01 | +25000 tanpa tipe, pihak 10 | `collect_payment` 25000 tunai | Ya (spec) |
| TC-R-02 | −15000 tanpa tipe, pihak 10, sisa 0 | `customer_credit` 15000, metode null | Ya (spec) |
| TC-R-03 | −15000, sisa 50000 | `reduce_receivable`, jumlah 0, potong 15000 | Ya (spec) |
| TC-R-04 | −50000, sisa 20000 | `customer_credit` 30000 + potong 20000 | Ya (spec) |
| TC-R-05 | −15000 walk-in tanpa izin refund | 403 | Ya (spec) |
| TC-R-06 | −15000 walk-in + izin | `refund` 15000 tunai | Ya (spec) |
| TC-R-07 | Tunda: exchange + (sales_delivery\|manual) + fisik | true | Ya (spec) |
| TC-R-08 | POS fisik / tanpa-pengganti / jasa-saja | false ×3 | Ya (spec ×3) |
| TC-R-09 | Modal 0 (avg + beli) | `400` | Ya (spec) |
| TC-R-10 | Modal 5000 | lolos | Ya (spec) |
| TC-R-11 | Create: centang + alasan → `Siap dicek` → preview (tanpa create) → simpan → detail; bar lengket di atas `Order Asal` | Tepat | Ya (FE test: 1 skenario, mock context/preview/create) |
| TC-R-12 | E2E 18: pending (stok 5) → dispatch (stok 3, `dispatched`) → confirm (`delivered`, tanggal) + retur +2 | Ujung-ke-ujung | Ya (E2E 18) |

## TC-E — E2E & integrasi (TC-E-01…TC-E-06)

| ID | Skenario | Kunci | Sudah ada |
|---|---|---|---|
| TC-E-01 | E2E 18 penuh (retur + tukar + SJ + confirm) | Lihat TC-R-12 | Ya |
| TC-E-02 | Piutang-efektif order pasca-retur (total & bayar terkoreksi) | Rumus modul 08/09 | Ya (E2E 22 pola + kode) |
| TC-E-03 | Guard arsip-member (modul 06/07 membaca relasi penerima) | n/a langsung | Ya (E2E 22-B) |
| TC-E-04 | Cetak nota pasca-retur (sisa terkoreksi) | Snapshot + efek | GAP (E2E 14 umum) |
| TC-E-05 | Rusak → lokasi RUSAK; tak-kembali → tanpa gerak | Kondisi | GAP |
| TC-E-06 | Refund tunai butuh izin; walk-in kredit→refund | Izin | GAP (unit ada, E2E tidak) |

## GAP (G-01…G-07)

| ID | Celah | Kenapa penting |
|---|---|---|
| G-01 | `buildPlan` utuh (kelayakan, sisa, proporsi, pajak) | Inti tanpa jaring integrasi |
| G-02 | Kondisi (rusak-RUSAK, tak-kembali) + lokasi | Jalur stok khusus tanpa jaring |
| G-03 | Dispatch/confirm utuh (stok, status, audit) | Hanya E2E 18 satu jalur |
| G-04 | Kas-tanpa-baris (collect/refund) | Mengunci KI baru (perilaku kas) |
| G-05 | Batas (±0.009, tolerance sisa, rasio-bulat) | Ambang tanpa jaring |
| G-06 | UI (bar lengket, preview, seksi kirim, toast) | FE test hanya create-sebagian |
| G-07 | Nomor + `cancelled_*` mati | Kontrak penomoran + kolom mati |
