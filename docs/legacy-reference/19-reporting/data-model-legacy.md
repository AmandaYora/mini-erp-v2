# Data Model Legacy (Konsep) — Modul 19 Reporting

**Kelompok B — konsep saja**, bukan skema persis. Sumber: `001_baseline.sql:407–422`
+ `daily-operational-metric.entity.ts:1–37` + `metrics-job.service.ts:106–121` (upsert).

## 1. Tabel Milik Sendiri (1)

Satu-satunya tabel tulis modul ini; **ditulis cron via upsert, tak dibaca siapa pun**
(grep `apps/api/src` nol SELECT di luar modul; dashboard pun tak membacanya!).

### `daily_operational_metrics` — satu baris per (cabang, tanggal)

| Kolom (migrasi → entity) | Tipe & default | Diisi dari (cron!) |
|---|---|---|
| `id_daily_operational_metric` → `id` (`entity:5–6`) | `INT AUTO_INCREMENT PK` | Otomatis (bukan bagian upsert!) |
| `id_branch` → `idBranch` (`8–9`) | `INT NOT NULL`, FK `fk_daily_metrics_branch → branches(id_branch)` | `branch.id` per iterasi cabang aktif |
| `metric_date` → `metricDate` (`11–12`) | `DATE NOT NULL` (entity `type:'date'`, string `YYYY-MM-DD`!) | `dateStr` kemarin-UTC (`toISOString().slice(0,10)`) |
| `total_orders` → `totalOrders` (`14–15`) | `INT NOT NULL DEFAULT 0` | `Σ count` semua grup (termasuk tak dikenal!) |
| `pending_orders` → `pendingOrders` (`17–18`) | `INT NOT NULL DEFAULT 0` | `count` grup `pending` (penimpaan `=`, bukan `+=`!) |
| `active_orders` → `activeOrders` (`20–21`) | `INT NOT NULL DEFAULT 0` | `count` grup `active` |
| `completed_orders` → `completedOrders` (`23–24`) | `INT NOT NULL DEFAULT 0` | `count` grup `completed` |
| `cancelled_orders` → `cancelledOrders` (`26–27`) | `INT NOT NULL DEFAULT 0` | `count` grup `cancelled` |
| `total_sales_amount` → `totalSalesAmount` (`29–30`) | `DECIMAL(18,2) NOT NULL DEFAULT 0` (entity `precision 18, scale 2`!) | `Σ CASE sales THEN total_amount ELSE 0` (`Number()` mentah, tanpa `money()`!) |
| `critical_stock_item_count` → `criticalStockItemCount` (`32–33`) | `INT NOT NULL DEFAULT 0` | `COUNT(*)` baris kritis per-baris-lokasi (tanpa filter lokasi!) |
| `generated_at` → `generatedAt` (`35–36`) | `DATETIME(3) NOT NULL` migrasi vs `datetime precision 6` entity — cek! | `NOW(3)` setiap upsert (baru meski tanggal sama!) |

Kunci & relasi: `PRIMARY KEY (id_daily_operational_metric)`; `UNIQUE KEY uq_daily_metrics
(id_branch, metric_date)` (target `ON DUPLICATE KEY UPDATE`!); FK cabang saja (tanpa FK
perusahaan/tanggal!). Komentar migrasi §4.8: `Reporting (tabel agregasi, tidak dipakai di MVP)`.

## 2. Tabel yang Dibaca (milik modul lain — tanpa FK dari sini!)

- `orders` (`id_branch, order_date, archived_at, order_kind, total_amount, id_current_status`)
  + join `order_status_definitions` (grup status; LEFT di laporan, INNER di cron!) + pihak terkait.
- `inventory_balances` (`id_branch, id_product, available_qty`) + join `products`
  (`stock_tracked, min_stock_qty, archived_at, productName`) — laporan: `id_branch` sesi;
  cron: tanpa `id_company`/lokasi (kandidat temuan!).
- `branches` (`status`) — daftar `status='active'` untuk cron (`branchRepo.find`).

## 3. Catatan Konseptual untuk Rebuild

- Modul ini tidak memiliki relasi tulis ke tabel operasional — murni baca + satu
  tabel agregat terisolasi (hanya FK cabang).
- Anomali skema: `generated_at` memakai `datetime` presisi-3 di migrasi tetapi
  entity memakai `datetime` presisi-6 ([PERLU KONFIRMASI] mana yang benar di DB
  berjalan — tak berpengaruh perilaku).
- Keputusan rebuild: nasib tabel agregat (hapus / hidupkan / ganti) diputuskan
  di KI-126; selain itu tak ada warisan skema. Bila dipertahankan: pertahankan kunci
  unik (cabang,tanggal) + semantik menimpa + `DECIMAL(18,2)`; putuskan pembulatan
  (`Number()` kini!) dan filter kritis sebelum menghidupkan pembaca.
