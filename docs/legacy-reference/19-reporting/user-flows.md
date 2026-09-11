# User Flows — Modul 19 Reporting

**Kelompok A.** Tanpa UI, alur = urutan panggilan API + cron. Aktor: Pemilik/Admin
(berizin), Staff (ditolak), Sistem (cron).

---

## F-01 Melihat Laporan Order per Cabang (satu-satunya pola pakai nyata — dari E2E 10)

**Aktor:** Owner/Admin di cabang BLR (hak `reporting.view` + cabang aktif;
`10-observability…:46–49` `openApp(/dashboard)` + `switchToBranchCode(BLR)`).

1. Login + pilih cabang aktif (tanpa cabang → `403 Branch aktif belum dipilih`).
2. `POST reporting/orders` dengan `{ date_from:'2026-05-01T00:00:00.000Z', date_to:'2026-05-12T23:59:59.999Z', order_kind:'sales', page:N, limit:100 }` (`24–32`).
3. Terima `{ items, total_sales:Number-mentah!, meta:{page, limit, total} }`; cari order di
   `items` (cocokkan `id/id_order/idOrder` + `orderNumber/order_number`, `33–36`).
4. Bila `meta.total > page × limit` → ulangi dengan `page + 1` (maksimal 10 halaman di E2E;
   `poll(...,30s)` hingga ketemu! `39–42, 90–92`).
5. `total_sales` dibaca sebagai angka periode (catatan: mencakup semua jenis terfilter —
   KI-123!; tanpa `money()` — temuan baru!).

## F-02 Mengecek Stok Kritis Cabang

1. Syarat sama (izin + cabang aktif).
2. `POST reporting/stock` dengan `{ critical_only: true }` (atau `false`/kosong = semua;
   E2E: `{critical_only:false}` lalu cocokkan `idProduct/id_product/product.id`, `94–100`).
3. Terima array mentah saldo + produk (tanpa `items/meta`!); tiap baris kritis memenuhi
   `min_stock_qty NOT NULL AND available_qty <= min_stock_qty` per baris lokasi.
4. Catatan: kritis di sini **per baris lokasi**, bukan per produk seperti dashboard (KI-124!);
   cron memakai rumus baris yang sama TETAPI tanpa filter lokasi (temuan baru!).

## F-03 Agregasi Harian Otomatis (tanpa aktor manusia)

1. Tiap 01:00 waktu server (`@Cron(EVERY_DAY_AT_1AM)`, `metrics-job.service.ts:31`), cron
   menjalankan `runDailyAggregation` → kemarin-server → `YYYY-MM-DD` UTC (KI-125!).
2. Sistem per cabang aktif (`status='active'`): hitung order per `status_group`
   (`DATE(order_date)=tanggal`, INNER JOIN, arsip-keluar) + omzet sales-saja
   (`CASE sales`, batal ikut!) + jumlah baris stok kritis (per-baris!).
3. Hasil di-upsert (`INSERT…ON DUPLICATE KEY UPDATE` atas `(cabang,tanggal)`,
   `generated_at=NOW(3)`) ke `daily_operational_metrics` — menimpa bila tanggal sama.
4. Cabang gagal → `logger.error('Failed to aggregate branch…')`, sisanya tetap jalan
   (tak melempar!); agregasi manual via `aggregateForDate('YYYY-MM-DD')` tanpa endpoint.
   Tidak ada notifikasi ke user.
5. Hari ini: **tidak ada yang membaca hasilnya** (tulisan-tanpa-pembaca — KI-126!;
   grep nol SELECT; `metricRepo` tak dipakai!).

## F-04 Catch-up Manual (operator teknis)

1. Insiden (server mati saat 01:00) → jalankan `aggregateForDate('YYYY-MM-DD')`
   via skrip sekali-jalan (tanpa endpoint!).
2. Upsert menimpa tanggal yang sudah ada — aman dijalankan ulang.

## F-05 Akses Ditolak (staff)

1. Staff (tanpa `reporting.view`) memanggil endpoint mana pun → `403`.
2. Tanpa token → `401`. Tanpa cabang aktif → `403 Branch aktif belum dipilih`.
