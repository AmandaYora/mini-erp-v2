# Numbering & Sequence — Modul 19 Reporting

**Kelompok A. Tidak ada penomoran dokumen di modul ini** (verifikasi: tidak ada
sequence, counter, format nomor, maupun pemanggil `formatDocumentSequenceNumber`
di `apps/api/src/modules/reporting/`).

Satu-satunya nilai "nomor" yang muncul adalah parameter teknis, bukan nomor dokumen
(tanpa sequence/counter/format nomor dokumen; grep `formatDocumentSequenceNumber` nol!):

| Nilai | Arti | Aturan (sumber!) |
|---|---|---|
| `page` / `limit` | Paginasi `reporting/orders` | Default 1/20, `min(limit,100)` diam-diam (`reporting.service.ts:25–26`); `meta.limit` = efektif |
| `generated_at` | Stempel tulis metrik (`NOW(3)`) | Presisi milidetik; migrasi `DATETIME(3)` vs entity `precision 6` — cek! (`001_baseline.sql:418`, `entity:35–36`) |
| `metric_date` | Tanggal agregat (`YYYY-MM-DD`) | Kemarin-server diformat UTC (`toISOString().slice(0,10)`, `metrics-job.service.ts:35`, KI-125!) |
| Counter metrik | Bukan nomor dokumen | `total/pending/active/completed/cancelled_orders`, `total_sales_amount DECIMAL(18,2)`, `critical_stock_item_count` (INT 0!) |
| `id_branch` | Kunci upsert, bukan urut | Pasangan unik `(id_branch, metric_date)` (`uq_daily_metrics`) |
