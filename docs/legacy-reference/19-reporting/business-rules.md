# Business Rules — Modul 19 Reporting

**Kelompok A.** Sumber: baca penuh `reporting.service.ts` (70 baris),
`metrics-job.service.ts` (123 baris), `reporting.controller.ts` (28 baris),
`daily-operational-metric.entity.ts` (37 baris), `reporting.module.ts` (25 baris),
kedua spec (347 baris), migrasi `001_baseline.sql:407–422`, E2E 10
(`10-observability-permission-hardening.spec.ts:24–43, 90–100`), `role-access-config.ts:111–114`
+ test system-only, dan grep web (tanpa UI!).

---

## 1. Aturan Akses

| ID | Aturan |
|---|---|
| BR-01 | Kedua endpoint wajib JWT + `reporting.view` + cabang aktif (`BranchGuard`). |
| BR-02 | `reporting.view` default hanya untuk superadmin/owner/admin; staff tidak punya; custom role tidak bisa diberi (system-only, dijaga test!). |
| BR-03 | Data selalu lingkup cabang sesi (`idActiveBranch`) — tak ada parameter cabang. |
| BR-04 | Modul exempt audit: baca maupun cron tidak menulis `audit_logs`. |

## 2. Laporan Order (`reporting/orders`)

| ID | Aturan |
|---|---|
| BR-05 | Filter semua opsional (`reporting.service.ts:35–38`): `date_from` (`o.order_date >= :df`, inklusif), `date_to` (`o.order_date <= :dt`, inklusif), `status_group` (via `cs.status_group = :sg` dari `LEFT JOIN currentStatus`), `order_kind` (`o.order_kind = :ok`). Nilai tak dikenal (mis. status_group salah ketik) = hasil kosong, bukan error. Sumber tabel/kolom: `orders.order_date/order_kind/archived_at/id_branch` + `order_status_definitions.status_group`. |
| BR-06 | Perbandingan tanggal leksikografis/mentah terhadap `order_date` (kolom datetime, tanpa konversi zona!; E2E memakai ISO penuh `2026-05-01T00:00:00.000Z` s/d `2026-05-12T23:59:59.999Z` — rentang inklusif tercakup; `10-observability…:26–32`). Beda dari dashboard (serah-menang!) dan cron (`DATE()`!) — disengaja per laporan. |
| BR-07 | Order terarsip (`o.archived_at NOT NULL`) selalu dikecualikan (`reporting.service.ts:32`) — dari daftar (`getMany`), hitungan (`getCount`), maupun total (SUM terpisah!). |
| BR-08 | Urutan tetap: `o.orderDate` DESC (terbaru dulu; tak ada opsi urut lain! `reporting.service.ts:33`). |
| BR-09 | `page` default 1; `limit` default 20, **cap 100 diam-diam** (`Math.min(limit ?? 20, 100)`, `reporting.service.ts:26`; kelebihan dipotong tanpa error; `meta.limit` = nilai efektif). Tiga query paralel dari `qb.clone()`: halaman + hitung + SUM (`40–49`). |
| BR-10 | `total_sales` = `COALESCE(SUM(o.total_amount), 0)` atas **seluruh** baris terfilter (query terpisah `46–48`, bukan jumlah halaman!) — `Number()` mentah tanpa `money()/roundRupiah()`! Mencakup semua `order_kind` dan semua status terfilter, termasuk pembelian dan batal bila tak difilter (KI-123!). Kosong = 0 (bukan null!). |
| BR-11 | Daftar memakai LEFT JOIN status & pihak (`leftJoinAndSelect currentStatus + relatedParty`, `28–30`): order tanpa definisi status **tetap tampil** (berlawanan dengan cron BR-18!). Isi baris = order lengkap + relasi (E2E cocokkan `id/id_order/idOrder` + `orderNumber/order_number`). |

## 3. Laporan Stok (`reporting/stock`)

| ID | Aturan |
|---|---|
| BR-12 | Hanya produk terlacak (`p.stock_tracked = 1`) dan tidak diarsip (`p.archived_at IS NULL`); lingkup `ib.id_branch = sesi`; urutan `p.productName` A–Z (`reporting.service.ts:55–61`); tanpa paginasi (selalu semua! `getMany()` langsung). Sumber: `inventory_balances ⨝ products`. |
| BR-13 | `critical_only: true` menambah: `p.min_stock_qty IS NOT NULL AND ib.available_qty <= p.min_stock_qty` (`63–65`) — **per baris saldo lokasi**, bukan total per produk (KI-124!; beda dari dashboard `SUM()<=MAX()` dan beda dari cron yang tanpa filter lokasi!). |
| BR-14 | `critical_only` falsy (absen/false) = tanpa filter kritis (klausa tak ditambah!). |
| BR-15 | Respons = array mentah saldo + relasi `product` (kontrak berbeda dari F-01 — disengaja, bukan bug: dipertahankan!; E2E baca `idProduct/product.id`, `10-observability…:97–100`). |

## 4. Cron Metrik Harian

| ID | Aturan |
|---|---|
| BR-16 | Jadwal `EVERY_DAY_AT_1AM` **waktu server** (`@Cron`, `metrics-job.service.ts:31`); tanggal agregat = kemarin-server diformat UTC: `yesterday.setDate(getDate()-1)` + `toISOString().slice(0,10)` = **tanggal UTC**, bukan WIB (KI-125!; `32–37`). `aggregateForDate(dateStr)` publik untuk catch-up manual via skrip (tanpa endpoint! `39–42`). |
| BR-17 | Hanya cabang `status = 'active'` (`branchRepo.find({where:{status:'active'}})`, `46`); nol cabang = nol query tulis (berhasil diam-diam + log `Done aggregating 0 branches…`). |
| BR-18 | Hitung order per cabang (`61–72`): `DATE(o.order_date) = tanggal` (fungsi DATE = tanggal **DB/server**, konsisten dengan BR-16, bukan WIB!) + `o.archived_at IS NULL` + INNER JOIN `order_status_definitions` (order tanpa status tak-terhitung! berlawanan dengan BR-11!). Tepat 1 query agregat `GROUP BY status_group`. |
| BR-19 | Omzet metrik (`total_sales_amount`) hanya `order_kind = 'sales'` (`SUM(CASE WHEN sales THEN total_amount ELSE 0 END)`); jenis lain menyumbang hitungan tapi Rp 0. Status apa pun ikut (termasuk batal!) selama tanggal cocok. Akumulasi JS `+= Number(total)` (`81–89`); `Number()` mentah tanpa `money()`! Kolom DB `DECIMAL(18,2)`. |
| BR-20 | Kelompok status tak dikenal (di luar pending/active/completed/cancelled) ikut `total_orders` (`+= cnt`) tapi tak masuk empat counter (counter = penimpaan `=`, bukan `+=`, per grup! `85–88`). |
| BR-21 | Upsert unik (cabang, tanggal) (`106–121`): `INSERT … VALUES (?,?,?,?,?,?,?,?,?, NOW(3)) ON DUPLICATE KEY UPDATE <8 kolom> = VALUES(<kolom>)` atas kunci `uq_daily_metrics (id_branch, metric_date)` (`001_baseline.sql:420`); tulis ulang tanggal sama menimpa semua kolom + `generated_at` baru (idempoten! aman catch-up ulang). Tepat 3 query per cabang (order, kritis, upsert — dijaga spec 2 cabang = 6!). |
| BR-22 | Gagal per cabang → `logger.error('Failed to aggregate branch {id} for {date}: {msg}')`, lanjut cabang berikut (`49–54`); agregasi tak pernah melempar ke scheduler (test: cabang-1 lempar → tetap resolve + cabang-2 ≥3 query!). |
| BR-23 | `generated_at` = `NOW(3)` (presisi milidetik; migrasi `DATETIME(3)` vs entity `datetime precision 6` — cek! tak berpengaruh perilaku). |

## 5. Katalog Pesan Lengkap (teks apa adanya)

- `Branch aktif belum dipilih` (403 BranchGuard — satu-satunya pesan khas modul; kedua endpoint `reporting.controller.ts:12,15–27`).
- `Aggregating daily_operational_metrics for {YYYY-MM-DD}` (log info, `metrics-job.service.ts:44`).
- `Done aggregating {n} branches for {YYYY-MM-DD}` (log info, `56`).
- `Failed to aggregate branch {id} for {date}: {pesan-error-asli}` (log error, `52`).
- 401/403/500 lain = envelope global ([PERLU KONFIRMASI] teks persis — belum dibaca di modul ini).
- Tanpa pesan validasi milik sendiri (filter salah ketik = hasil kosong diam-diam!).

## Temuan baru & pertanyaan terbuka (belum ber-ID)

- Tanpa pembulatan uang: `reporting.service.ts:51` (`Number(totalSales)`) dan cron `metrics-job.service.ts:83–84` (`Number(row.total)`) tanpa `money()` 2-desimal (dashboard!) maupun `roundRupiah()` utuh (`packages/shared-types/src/money.ts:17–18`). Agregat reporting bisa menampilkan pecahan sen yang tak bisa ditagih; kolom DB `DECIMAL(18,2)` (`001_baseline.sql:416`) akan membulatkan diam-diam saat cron menulis. Perlu satu kebijakan rupiah? Cek!
- Kritis cron tanpa filter lokasi & perusahaan: `metrics-job.service.ts:92–103` hanya `ib.id_branch + p.stock_tracked + p.min_stock_qty + available + p.archived`, TANPA `stock_locations.status/archived` (dashboard `dashboard.service.ts:168–169,210–211` mensyaratkan!) dan TANPA `p.id_company`. Cabang dengan lokasi arsip/nonaktif ikut terhitung kritis di metrik tetapi tidak di dashboard/laporan-stok. Disengaja? Cek!
- `metricRepo` di-inject tak dipakai: `metrics-job.service.ts:23–24` meng-inject `DailyOperationalMetric` tetapi semua tulis via `dataSource.query` mentah (`61,92,106`); satu-satunya pemakaian di spec adalah mock `find` yang tak pernah dipanggil. Sisa injeksi atau persiapan baca yang batal? Cek!
- `generated_at` presisi-3 vs 6: migrasi `DATETIME(3)` (`001_baseline.sql:418`) vs entity `datetime precision 6` (`daily-operational-metric.entity.ts:35–36`). Mana yang benar di DB berjalan? Cek! Tak berpengaruh perilaku selain presisi stempel.
- `DATE(order_date)` vs `order_date >=/:` rentang: laporan order (`reporting.service.ts:35–36`) memakai perbandingan mentah inklusif (zona = pengirim!), cron memakai `DATE()` server (`metrics-job.service.ts:68`). Order `2026-05-12T00:30:00+07` bisa masuk hari berbeda di kedua laporan. Perlu WIB-aware seperti finance? Cek!
- `total_sales` mencakup pembelian/batal (KI-123!) tetapi E2E selalu memfilter `order_kind:'sales'` (`10-observability…:26–32`) sehingga kaveat tak pernah terlihat di test. Apakah kontrak dipertahankan atau diganti `total_amount`? Cek! (tanpa mengubah KI-123!).
