# Algorithms Legacy (Konsep) — Modul 19 Reporting

**Kelompok B — ide algoritma**, bukan kode. Hanya 3 logika, semuanya kecil, tetapi
tiap rumus didokumentasikan presisi (definisi, sumber tabel/kolom, filter
cabang/status, rumus, zona waktu) sesuai bar modul 17. Sumber: baca penuh
`reporting.service.ts` (70 baris), `metrics-job.service.ts` (123 baris),
`daily-operational-metric.entity.ts` (37 baris), `reporting.controller.ts` (28 baris),
`001_baseline.sql:407–422`, kedua spec (347 baris), E2E 10.

Konfirmasi cakupan: TANPA UI (tanpa folder `apps/web/src/modules/reporting`,
tanpa rute registry — grep `reporting/` di web hanya `role-access-config.test.ts:16`!);
hanya 2 endpoint + 1 cron + 1 tabel tulisan-tanpa-pembaca.

## 1. Peta Algoritma → Lokasi

| Algoritma | Lokasi legacy | Invariansi rebuild |
|---|---|---|
| Daftar order + total periode | `reporting.service.ts:14–53` (`getOrderReport`) | Filter opsional; total = seluruh set terfilter; cap-100 diam-diam; arsip keluar; terbaru dulu |
| Daftar stok + mode kritis | `reporting.service.ts:55–69` (`getStockReport`) | Hanya terlacak tak-diarsip; kritis per baris lokasi; array mentah tanpa paging |
| Agregasi harian + upsert | `metrics-job.service.ts:30–122` (`runDailyAggregation` + `aggregateForDate` + `aggregateBranch`) | Cron 01:00 server; H-1 UTC per cabang aktif; omzet sales-saja; upsert per (cabang,tanggal); gagal-per-cabang lanjut |

## 2. Keputusan Desain (untuk rebuild)

- **Total terpisah dari halaman** (query SUM kedua): angka periode tetap benar
  berapa pun paginasinya — pertahankan polanya bila laporan dipertahankan.
- **Kritis per baris vs per produk**: reporting memakai definisi paling sederhana
  (satu perbandingan per baris); dashboard memakai agregat per produk. Bedanya
  disengaja-atau-tidak diputuskan di KI-124 — rebuild tidak boleh diam-diam
  menyamakan tanpa keputusan.
- **Cron menimpa, bukan menambah**: agregat = snapshot yang bisa dihitung ulang
  kapan pun (sifat yang membuat catch-up manual aman) — pertahankan bila tabel
  dipertahankan.
- **Gagal-per-cabang-lanjut**: satu cabang rusak tak boleh menghentikan cabang
  lain — pertahankan polanya di job terjadwal mana pun.

---

## 3. Daftar Order + Total Periode (`getOrderReport`)

**Masalah:** tampilkan halaman kecil tetapi total periode tetap benar.

**Hasil yang diharapkan:** `{ items, total_sales, meta: { page, limit, total } }`
dengan `total_sales` = seluruh set terfilter, bukan jumlah halaman.

**Konsep sekarang (`reporting.service.ts:14–53`):**

- Basis: `orderRepo.createQueryBuilder('o')`
  `.leftJoinAndSelect('o.currentStatus','cs')`
  `.leftJoinAndSelect('o.relatedParty','rp')`
  `.where('o.id_branch = :idBranch', sesi)` + `.andWhere('o.archived_at IS NULL')`
  + `.orderBy('o.orderDate','DESC')` (terbaru dulu; tanpa opsi urut lain!).
  LEFT JOIN artinya order tanpa definisi status TETAP tampil (berlawanan dengan cron §5!).
- Filter opsional (semua `andWhere` bila terisi): `date_from → o.order_date >= :df`
  inklusif; `date_to → o.order_date <= :dt` inklusif (perbandingan leksikografis/
  datetime mentah, tanpa konversi zona! E2E memakai ISO penuh
  `2026-05-01T00:00:00.000Z` s/d `2026-05-12T23:59:59.999Z`);
  `status_group → cs.status_group = :sg`; `order_kind → o.order_kind = :ok`.
  Nilai tak dikenal (mis. salah ketik) = hasil kosong, bukan error.
- Paginasi: `page ?? 1`, `limit = min(limit ?? 20, 100)` — cap-100 diam-diam
  (tanpa error; `meta.limit` = nilai efektif!). Tiga query dari `qb.clone()` paralel:
  `skip((page-1)*limit).take(limit).getMany()` + `getCount()` + SUM terpisah
  `select('COALESCE(SUM(o.total_amount),0)','totalSales').getRawOne()`.
- `total_sales = Number(totalsRow?.totalSales ?? 0)` — MENTAH tanpa `money()` maupun
  `roundRupiah()` (beda dari dashboard yang `money()`! kandidat temuan — lihat
  `business-rules.md` temuan baru). Mencakup SEMUA `order_kind` + SEMUA status terfilter
  (termasuk beli & batal bila tak difilter — KI-123!). Kosong = `items:[]`, `total:0`,
  `total_sales:0` (nol, bukan null!).

**Untuk rebuild:** pola "halaman + hitung + SUM terpisah dari basis yang sama" yang
mengikat; putuskan penamaan `total_sales` bila mencakup pembelian.

## 4. Daftar Stok + Mode Kritis (`getStockReport`)

**Hasil yang diharapkan:** array mentah saldo + produk, tanpa paginasi.

**Konsep sekarang (`reporting.service.ts:55–69`):**

- Basis: `balanceRepo.createQueryBuilder('ib')`
  `.where('ib.id_branch = :idBranch', sesi)`
  `.leftJoinAndSelect('ib.product','p')`
  `.andWhere('p.archived_at IS NULL')` + `.andWhere('p.stock_tracked = 1')`
  + `.orderBy('p.productName','ASC')` (A–Z; tanpa opsi lain!). Selalu semua (tanpa
  `skip/take`!).
- `critical_only: true` menambah `p.min_stock_qty IS NOT NULL AND
  ib.available_qty <= p.min_stock_qty` — PER BARIS saldo lokasi, bukan agregat per
  produk (beda dari dashboard `SUM() <= MAX()`! KI-124!). Falsy (absen/false) = tanpa
  filter kritis. Respons = array mentah (kontrak beda dari F-01 — disengaja!).
- Baris = saldo + relasi `product` (E2E membaca `idProduct/product.id`,
  `10-observability…:97–100`).

**Untuk rebuild:** jangan samakan definisi kritis tanpa keputusan KI-124.

## 5. Agregasi Harian + Upsert (`MetricsJobService`)

**Masalah:** sediakan snapshot harian per cabang yang bisa dihitung ulang kapan pun.

**Hasil yang diharapkan:** satu baris per (cabang, tanggal-kemarin) setiap 01:00,
menimpa bila dihitung ulang; satu cabang gagal tak menghentikan lain; tanpa pembaca.

**Konsep sekarang (`metrics-job.service.ts:1–123`):**

- Jadwal: `@Cron(CronExpression.EVERY_DAY_AT_1AM)` = tiap hari 01:00 **waktu server**
  (30–31). `runDailyAggregation` (32–37): `yesterday = now − 1 hari` (waktu server!),
  `dateStr = yesterday.toISOString().slice(0,10)` = tanggal **UTC** (bukan WIB!
  KI-125!), lalu `aggregateForDate(dateStr)`.
- Lingkup (`aggregateForDate` 43–57, publik untuk catch-up manual via skrip
  sekali-jalan, tanpa endpoint!): `branchRepo.find({where:{status:'active'}})`;
  nol cabang = nol query tulis (berhasil diam-diam!). Per cabang `try/catch`:
  gagal → `logger.error('Failed to aggregate branch {id} for {date}: {msg}')`,
  lanjut; tak pernah melempar ke scheduler (49–54). Log info buka/tutup:
  `Aggregating daily_operational_metrics for {date}` / `Done aggregating {n} branches…`.
- Hitung order per cabang (`aggregateBranch` 59–89): `SELECT cs.status_group,
  COUNT(o.id_order) AS count,
  COALESCE(SUM(CASE WHEN o.order_kind='sales' THEN o.total_amount ELSE 0 END),0) AS total
  FROM orders o INNER JOIN order_status_definitions cs ON cs.id = o.id_current_status
  WHERE o.id_branch=? AND DATE(o.order_date)=? AND o.archived_at IS NULL GROUP BY cs.status_group`.
  Catatan presisi: `DATE()` = tanggal DB/server (konsisten dengan jadwal server, bukan
  WIB!); INNER JOIN = tanpa-status tak-terhitung (berlawanan dengan §3!);
  omzet hanya `sales` (jenis lain hitung tapi Rp 0); status apa pun ikut termasuk batal!;
  grup tak dikenal ikut `total_orders` tapi tak masuk 4 counter (penimpaan `=`, bukan
  `+=`!); agregat JS: `totalOrders += cnt`, `totalSalesAmount += Number(total)`.
- Hitung kritis (92–103): `SELECT COUNT(*) AS cnt FROM inventory_balances ib
  JOIN products p ON p.id_product = ib.id_product
  WHERE ib.id_branch=? AND p.stock_tracked=1 AND p.min_stock_qty IS NOT NULL
  AND ib.available_qty <= p.min_stock_qty AND p.archived_at IS NULL`.
  Catatan: TANPA filter `stock_locations.status/archived` (beda dari dashboard yang
  mensyaratkan lokasi aktif!) dan TANPA `p.id_company` (beda dari dashboard!) —
  kandidat temuan. Per-baris-lokasi (sama §4), bukan per-produk.
- Tulis (106–121): `INSERT INTO daily_operational_metrics
  (id_branch, metric_date, total_orders, pending_orders, active_orders, completed_orders,
  cancelled_orders, total_sales_amount, critical_stock_item_count, generated_at)
  VALUES (?,?,?,?,?,?,?,?,?, NOW(3)) ON DUPLICATE KEY UPDATE
  <semua kolom> = VALUES(<kolom>), generated_at = VALUES(generated_at)`.
  Kunci unik `(id_branch, metric_date)` (`001_baseline.sql:420`); tulis-ulang menimpa
  semua kolom + stempel baru (idempoten!). `generated_at = NOW(3)` presisi milidetik
  (migrasi `DATETIME(3)` vs entity `datetime precision 6` — cek! tak berpengaruh perilaku).
  Tepat 3 query per cabang (order, kritis, upsert) — dijaga spec (2 cabang = 6 query!).

**Untuk rebuild:** jadwal-server + tanggal-UTC + 3-query + upsert-menimpa +
gagal-lanjut adalah satu paket; nasib tabel diputuskan di KI-126 (komentar migrasi
`4.8 Reporting (tabel agregasi, tidak dipakai di MVP)`!).

## 6. Uang & Waktu Lintas Modul (ringkas perbandingan!)

- Uang: reporting `total_sales/total_sales_amount` = `Number()` mentah tanpa pembulatan
  (`reporting.service.ts:51`; cron `Number(row.total)` 83–84 + `DECIMAL(18,2)` di DB);
  dashboard memakai `money()` 2-desimal; order/payment memakai `roundRupiah()` utuh.
  Tiga kebijakan berbeda untuk "rupiah" — putuskan satu di rebuild (kandidat temuan!).
- Waktu: laporan order memakai `order_date` mentah inklusif (zona = apa yang dikirim
  pemanggil!); cron memakai `DATE(order_date)` server + tanggal-UTC-kemarin; dashboard
  memakai serah-menang + naive-zona-perusahaan. Tidak ada yang memakai `Asia/Jakarta`
  eksplisit kecuali pengirimnya — bandingkan finance yang WIB-aware (BR-106 modul 17!).
