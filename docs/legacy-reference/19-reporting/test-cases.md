# Test Cases — Modul 19 Reporting

**Kelompok A — 18 kasus** (dokumen lama menyebut 16 — cek! daftar kini T-01…T-18;
format: Kondisi → Aksi → Hasil). Kasus T-01…T-11 dicerminkan dari spec
(`reporting.service.spec.ts` 200 baris: 8 order + 3 stok; `metrics-job.service.spec.ts`
147 baris: 1 cron + 6 agregat); T-12…T-18 dari perilaku kode + E2E 10
(`10-observability-permission-hardening.spec.ts:24–43, 90–100, 144–157`) yang belum
tercakup spec. Presisi: spec memakai mock `clone()` (halaman/hitung/SUM terpisah!),
`Number()` mentah tanpa `money()` (bandingkan dashboard `money()` vs `roundRupiah()` utuh —
inkonsistensi yang lolos test, kandidat temuan!).

## Laporan Order (T-01–T-08)

- T-01 Filter kosong → `items` halaman-1 (20 default), `meta.total` = semua order
  cabang tak-diarsip, `total_sales` = `Number(SUM(total_amount))` semuanya (tanpa bulat!;
  mock `rawOne:{totalSales:'500000'}` → 500000, `reporting.service.spec.ts:76–87`).
- T-02 `page: 3, limit: 10` → `meta.page = 3`, `meta.limit = 10` (potongan halaman-3;
  `skip/take` di-clone halaman saja, hitung/SUM tak terpotong! `89–97`).
- T-03 `limit: 999` → `meta.limit = 100` (cap `min(999,100)` diam-diam, tanpa error! `147–154`).
- T-04 `date_from: '2024-01-01'` → klausa `order_date >= :df` dipakai (`andWhere` berisi
  `:df`, `99–109`).
- T-05 `date_to: '2024-01-31'` → klausa `order_date <= :dt` dipakai (`111–121`).
- T-06 `status_group: 'completed'` → filter via `cs.status_group = :sg` (LEFT JOIN tetap
  tampil tanpa-status bila tak difilter! `123–133`).
- T-07 `order_kind: 'sales'` → filter `order_kind = :ok` (`134–145`).
- T-08 Tanpa yang cocok → `items: []`, `total: 0`, `total_sales: 0` (`rawOne:'0'` → 0,
  `156–163`; nol bukan null!).

## Laporan Stok (T-09–T-11)

- T-09 Filter kosong → semua saldo terlacak tak-diarsip, urut nama A–Z.
- T-10 `critical_only: true` → hanya baris `available_qty <= min_stock_qty`
  ber-`min_stock_qty` terisi.
- T-11 `critical_only: false` → tanpa klausa `min_stock_qty` (sama dengan kosong!).

## Cron (T-12–T-17, dari `metrics-job.service.spec.ts`)

- T-12 Cron berjalan → `aggregateForDate` dipanggil dengan kemarin `YYYY-MM-DD`
  (`runDailyAggregation: yesterday−1 + toISOString().slice(0,10)`; spy, `44–56`).
  Presisi zona: kemarin-server diformat UTC (KI-125!) — tak diuji beda WIB vs UTC (celah G-04!).
- T-13 Dua cabang aktif (`[{id:1},{id:2}]`) → tepat 6 query (3 per cabang: order, kritis,
  upsert; mock `status_group→[]`, `min_stock_qty→[{cnt:'0'}]`, else INSERT; `58–70`).
- T-14 Baris status (3,2,10,1 + total 300,500,2000,50; kritis 4) → upsert
  `(cabang:7, tanggal, 16=3+2+10+1, 3, 2, 10, 1, 2850=300+500+2000+50, 4)`
  (param `[idBranch, dateStr, total, pending, active, completed, cancelled, sales, kritis]`,
  `90–124`; membuktikan `=` per grup + `+=` total + sales-saja!).
- T-15 Tanpa baris order (`[]`) → upsert nol (`totalOrders 0`, `totalSalesAmount 0`,
  kritis mock 0; `126–137`).
- T-16 Cabang-1 gagal (query-1 lempar `DB connection lost`) → agregasi tetap resolve;
  cabang-2 tetap diproses (≥3 query; `72–88` — gagal-lanjut terbukti!).
- T-17 Tanpa cabang aktif (`[]`) → nol query `dataSource.query` (berhasil diam-diam; `139–145`).
- T-18 E2E nyata: order penjualan (`findOrderInReport` hingga 10 halaman `limit:100`,
  `24–43, 90–92`) + stok (`critical_only:false` berisi produk baru, `94–100`) muncul di
  kedua laporan; kasir tanpa `reporting.view` → 403 untuk laporan keuangan (pola izin sama,
  `144–150`); anonim `dashboard/summary` → 401 (`152–157`).

## Celah Test (belum tercakup — bukan untuk diperbaiki di sini, dicatat untuk rebuild)

- G-01 Cap-100 dan default-20 tak diuji via HTTP (hanya unit service).
- G-02 `Branch aktif belum dipilih` tak diuji untuk reporting.
- G-03 Kontradiksi kritis-per-baris vs per-produk tak diuji di mana pun.
- G-04 Batas tanggal UTC-vs-WIB cron tak diuji.
- G-05 Nol test untuk "tabel metrik tak dibaca siapa pun" (butuh keputusan KI-126 dulu).
