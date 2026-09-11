# Numbering & Sequence — Modul 18 Dashboard

**Kelompok A. Tidak ada penomoran dokumen di modul ini** (verifikasi: tanpa
sequence/counter/format nomor di `apps/api/src/modules/dashboard/` maupun halaman;
grep `formatDocumentSequenceNumber`/`sequence` nol di modul ini!).

Satu-satunya nilai "nomor" adalah teknis (bukan nomor dokumen — tanpa format/counter!):

| Nilai | Arti | Aturan (sumber!) |
|---|---|---|
| `trend_months` | Lebar tren | Opsional; jepit 1–12, default 6 (`dashboard.service.ts:64`); halaman selalu 6 (`dashboard-page.tsx:222`) |
| `priority_score` | Skor urut internal | `100/80/60/40 + umur-cap-30` (`dashboard.service.ts:344–351`); bukan nomor tampil (yang tampil = alasan-teks!) |
| `age_days` | Umur order (hari) | `MAX(0, DATEDIFF(today, order_date))` (`dashboard.service.ts:387`) |
| `transaction_count` | `COUNT(DISTINCT doc_key)` top/margin | `order:…` + `return:…` (`dashboard.service.ts:570, 695, 730`) |
| `month:'YYYY-MM'` | Bucket tren | `buildMonthBuckets` kalender server (`1069–1074`) |
| `id_*` | Kunci navigasi | `id_order → /orders/{id}`, `id_product → /products/{id} /stock/{id}`; `id_inventory_balance` kritis = MIN per produk (tak dipakai navigasi!) |
| Counter margin | Kejujuran, bukan urut | `costed/movement_snapshot/average/purchase/missing` + `transaction_count` (`1020–1047`) |
