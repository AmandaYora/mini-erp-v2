# Reports List — Modul 19 Reporting

**Kelompok A.** Modul ini MENGHASILKAN 2 laporan operasional (+ 1 tabel agregat
tulisan-tanpa-pembaca). Bukan laporan keuangan (itu modul 17) dan bukan ringkasan
dashboard (itu modul 18).

---

## 1. Laporan Order — `POST reporting/orders` (`reporting.service.ts:14–53`)

- **Filter:** `date_from` (`order_date >=`, inklusif, opsional) / `date_to` (`<=`, inklusif,
  opsional; keduanya mentah tanpa zona! E2E: `2026-05-01T00:00:00.000Z` s/d
  `2026-05-12T23:59:59.999Z`), `status_group` (pending/active/completed/cancelled via
  `cs.status_group`; salah ketik = kosong!), `order_kind` (sales/purchase/...),
  `page` (default 1), `limit` (default 20, cap 100 diam-diam!).
- **Kolom:** seluruh field order + relasi status berjalan (`currentStatus`, LEFT JOIN —
  tanpa status tetap tampil!) + pihak terkait (`relatedParty`).
- **Angka utama:** `total_sales` = `COALESCE(SUM(total_amount),0)` **seluruh** baris
  terfilter (query SUM terpisah, semua halaman!; `Number()` mentah tanpa `money()`!),
  `meta = {page, limit:efektif, total:hitung-terfilter}`.
- **Logika perhitungan:** lingkup `id_branch` sesi; arsip dikecualikan (daftar+hitung+total);
  urut `orderDate` DESC (terbaru dulu; tanpa opsi!); filter tanggal leksikografis inklusif;
  status via join definisi (order tanpa status tetap tampil — berlawanan dengan cron!).
- **Konsumsi nyata:** E2E 10 (`findOrderInReport`: `order_kind:'sales', page:1–10, limit:100`,
  cocokkan `id/id_order/idOrder` + `orderNumber/order_number`, berhenti bila `total ≤ page×limit`).
- **Kaveat kontrak:** nama `total_sales` menyesatkan bila filter mencakup
  pembelian/pembatalan (KI-123!); tanpa pembulatan (temuan baru!).

## 2. Laporan Stok — `POST reporting/stock` (`reporting.service.ts:55–69`)

- **Filter:** `critical_only?: boolean` (satu-satunya!; falsy = semua).
- **Kolom:** seluruh field saldo (`inventory_balances`) + relasi `product`.
- **Logika perhitungan:** lingkup `id_branch` sesi; hanya `stock_tracked = 1` +
  produk tak-diarsip, urut nama A–Z (tanpa paginasi — selalu semua!); mode kritis =
  `min_stock_qty IS NOT NULL AND available_qty <= min_stock_qty`
  **per baris lokasi** (bukan agregat per produk seperti dashboard! KI-124!).
- **Konsumsi nyata:** E2E 10 (memastikan produk baru muncul dengan `critical_only: false`;
  cocokkan `idProduct/id_product/product.id`).
- **Kaveat kontrak:** respons array mentah tanpa paginasi/meta (berbeda dari laporan order —
  disengaja, dipertahankan!).

## 3. Tabel Agregat Harian — `daily_operational_metrics` (tulisan-tanpa-pembaca!)

- **Kolom per (cabang, tanggal):** `total/pending/active/completed/cancelled_orders`
  (INT 0), `total_sales_amount` (`DECIMAL(18,2)`, sales saja! `Number()` mentah!),
  `critical_stock_item_count` (INT 0, per-baris tanpa filter lokasi!), `generated_at`
  (`NOW(3)`; migrasi `DATETIME(3)` vs entity `precision 6` — cek!). Skema per kolom di
  [data-model-legacy.md](data-model-legacy.md) §1.
- **Logika perhitungan:** cron `EVERY_DAY_AT_1AM` waktu server atas tanggal-kemarin-UTC
  (`toISOString().slice(0,10)`, KI-125!); lingkup cabang `status='active'` (nol = nol tulis!);
  order via `DATE(order_date)=tanggal` + INNER JOIN status (tanpa-status hilang!) +
  `archived_at IS NULL`; omzet `CASE sales THEN total_amount ELSE 0` (batal ikut!);
  grup-asing ikut total tapi tak masuk counter (`=` bukan `+=`!); stok via `COUNT(*) baris
  kritis`; tulis `INSERT…ON DUPLICATE KEY UPDATE` atas `uq_daily_metrics(cabang,tanggal)`
  (menimpa + stempel baru!); gagal-per-cabang → `logger.error` + lanjut (tak melempar!).
  Tepat 3 query per cabang.
- **Konsumsi nyata:** tidak ada (nol SELECT di seluruh backend maupun frontend;
  `metricRepo` di-inject tak dipakai — temuan baru!).
- **Status:** komentar migrasi §4.8 "tidak dipakai di MVP" — diputuskan nasibnya di KI-126.
