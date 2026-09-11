# Test Cases — Modul 18 Dashboard

**Kelompok A — 16 kasus.** T-01…T-09 dari `dashboard.service.spec.ts` (337 baris,
9 test dibaca penuh); T-10…T-12 dari hook-test (124 baris, 5 test!); T-13…T-16 dari
halaman (628 baris, tanpa test — verifikasi manual!) + E2E 10
(`10-observability-permission-hardening.spec.ts:82–88, 152–157`).

## Service (T-01–T-09)

- T-01 Grup (3,7,15,2) → ringkasan tepat + 18 query paralel (`getSummary:85–104`
  `Promise.all`; mock `createQueryBuilder.getRawMany`, `dashboard.service.spec.ts:92–110`).
- T-02 Tanpa order (`rawMany: []`) → semua nol (bukan null!): ringkasan 0 + kritis 0
  (`112–120`; raw default `defaultRawQuery 46–60` meniru bucket-kosong-0, GREATEST-0).
- T-03 Kritis (produk 44 `Baut Baja`: tersedia 8 ≤ min 10) → count 2 + item tepat
  (samakan definisi daftar-stok!; mock `dashboard_critical_stock_count/items`,
  `122–151`; menegaskan agregat-produk-level vs ambang!).
- T-04 Metrik pendamping (9 order via `getCount`, 125rb via `SUM(amount)` + `money()`;
  7 produk-lacak; 5/3 dokumen total/siap; WA `connected/628123`; tren-6!) → semua field
  tepat (`153–181`; `sales_trend` panjang 6 meski raw kosong — bucket-kosong-0 terbukti!).
- T-05 Utang/piutang mentah (350rb beli / 120rb jual via `GREATEST` + `params[5]` jenis!)
  → tepat (`183–196`); presisi: mock mengembalikan string, service `money()` 2-desimal —
  bandingkan `roundRupiah()` utuh (kandidat temuan, bukan diblokir test!).
- T-06 Top + margin (`Cat Tembok` 12 qty / 2,4jt omzet; HPP 1,5jt; margin 900rb; persen 37,5;
  `cost_basis:'mixed'` 1-costed + 2-average!) → 5 teratas + status-kejujuran
  (`basis:'operational_estimate', is_estimate:true`, counter lengkap `198–273`).
- T-07 D3: baris `margin_costed_at NOT NULL` pakai `cogs_amount_snapshot` 1jt SEKALI +
  fallback alokasi 500rb (bukan saling menimpa!) → HPP 1,5jt + margin 900rb +
  `cost_basis:'cost_movement'` (`275–323`; mock cabang `oi.margin_costed_at IS NOT NULL`
  vs `FROM stock_issue_allocations sia`!).
- T-08 Transaksi pembelian + penjualan completed (`tx_count:5, total_amount:2500000`
  via `getRawOne`) → hitung + nominal tepat, TANPA `money()` (mentah `Number()`!
  `325–336` — inkonsistensi presisi yang lolos test!).
- T-09 (E2E nyata) pola E2E-10: `today_from/to` ISO-UTC `2026-05-10` + `today_order_count ≥ 1`,
  `today_sales_amount ≥ 20000`, `tracked_product_count > 0` (`10-observability…:82–88`);
  anonim `POST dashboard/summary {data:{}}` → 401 (`152–157`); halaman `/dashboard` +
  3 pantauan → sidebar tampil + tanpa runtime-error (137–141).

## Hook (T-10–T-12, 5 test!)

- T-10 Idle/login → tanpa reload apa pun (4 slice!).
- T-11 Belum-login → tanpa reload.
- T-12 Sebagian ready/loading → tanpa reload; return 4 field tepat.

## Halaman (T-13–T-16, tanpa test — verifikasi manual!)

- T-13 KPI ↔ navigasi (pending/active/selesai/piutang/kritis → 4 rute!).
- T-14 3 keadaan margin (data / ada-transaksi-tanpa-cost / kosong!) → 3 notice tepat.
- T-15 Fallback tanpa-API → 6 angka workspace tampil tanpa penanda (KI-132!).
- T-16 WA 3 label (Terhubung/Menghubungkan…/Belum terhubung) + SOP-siap.

## Celah Test

- G-01 Tanpa test halaman (render/navigasi/fallback/notice!).
- G-02 Tanpa test 403-cabang/403-izin endpoint summary.
- G-03 Tanpa test jepit-tren/waktu-server-vs-zona (KI-133!).
- G-04 Tanpa test field-tak-tampil (KI-134!).
