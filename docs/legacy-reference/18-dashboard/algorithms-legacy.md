# Algorithms Legacy (Konsep) — Modul 18 Dashboard

**Kelompok B — ide algoritma**, bukan kode. Satu service
(`apps/api/src/modules/dashboard/dashboard.service.ts`, 1075 baris, dibaca penuh),
satu controller satu-endpoint, satu halaman. Delapan ide di §1 harus dipertahankan
hasilnya; §3–§12 menguraikan tiap metrik (definisi, sumber tabel/kolom, filter
cabang/status, rumus, zona waktu) sesuai bar kedalaman modul 17.

Kontrak presisi ada di [business-rules.md](business-rules.md); daftar tampil ada di
[reports-list.md](reports-list.md).

## 1. Peta Algoritma → Lokasi

| Algoritma | Lokasi legacy | Invariansi rebuild |
|---|---|---|
| Ringkasan grup-status | `getSummary` baris 44–60 | Semua-jenis, arsip-keluar, grup-asing-abaikan |
| Kritis produk-level | `getCriticalStockCount` 152–179 + `getCriticalStockItems` 181–232 | Agregat-lokasi-aktif vs ambang; 5 by selisih |
| Net operasional | `getNetSalesAmount` 244–273 + `getNetSalesTrend` 275–329 + `getCompletedSalesOrderCount` 541–558 | Serah-menang, retur-completed-ikut, bucket-kosong-0 |
| Skor prioritas | `getPriorityOrders` 331–399 | Tempo-100/80 + pending-60 + umur-cap-30; 5 teratas + alasan |
| Utang/piutang-net | `getOutstandingTotal` 445–509 + `getTotalPayable/Receivable` 437–443 | Kurang 3 sumber, jepit-per-baris, batal-keluar |
| Top terlaris | `getTopSoldProducts` 560–631 | Tanpa-pajak, retur-net, ambang-tampil, qty-turun |
| Estimasi margin | `getEstimatedMarginProducts` 633–1067 + `resolveEstimatedCostBasis` 1050–1067 | Hierarki-5-sumber + snapshot-sekali + status-jujur + urut-margin |
| Pendamping | `getTrackedProductCount` 401–409, `getKnowledge*` 411–426, `getWhatsappStatus` 428–435, `getTotalPurchase/SalesTransactions` 511–539 | Lacak-aktif, SOP-siap, WA-else-putus |
| Orkestrasi + uang + waktu | `getSummary` 62–128 (`Promise.all` 18 paralel), `money()` 31–33, `toNumber()` 26–29, `resolveTodayRange` 130–139, `resolveMonthRange` 141–150, `buildMonthBuckets` 1069–1074 | Paralel-penuh, 2-desimal, hari/bulan-server |

## 2. Keputusan Desain (untuk rebuild)

- **Serah-menang atas tanggal-order**: angka ikut barang-bergerak, bukan kertas —
  jangan ganti ke tanggal-order tanpa keputusan.
- **Estimasi yang mengaku**: `is_estimate + counter + label-basis` membuat angka
  bisa-diajukan-tanpa-menyesatkan — pola kejujuran ini wajib dipertahankan di
  agregat estimasi mana pun.
- **Jepit-per-baris utang**: kelebihan-bayar hilang dari total (bukan negatif!) —
  putuskan di rebuild apakah kelebihan perlu pos sendiri.
- **Fallback-tanpa-tanda**: keputusan kini (KI-132!) — rebuild memilih sadar:
  penanda, samakan-logika, atau hilangkan-fallback.

---

## 3. Orkestrasi: 18 Agregat Paralel dalam Satu Panggilan

**Masalah yang dipecahkan:** halaman butuh ~20 angka dari 10+ tabel tanpa
N+1 dan tanpa banyak round-trip.

**Hasil yang diharapkan:** satu `POST dashboard/summary` mengembalikan 21 field
siap-render; gagal sebagian = gagal seluruhnya (tidak ada hasil parsial).

**Konsep sekarang (`dashboard.service.ts:62–128`):**

- `resolveTodayRange` + `resolveMonthRange` + `trendMonths = clamp(trend_months ?? 6, 1, 12)`
  dihitung dulu, lalu 18 janji dijalankan dalam satu `Promise.all`: hitung kritis,
  item kritis, hitung-hari, net-hari, tren, prioritas, net-bulan, hitung-selesai-bulan,
  top-terlaris, margin (±5 query di dalamnya!), produk-lacak, dokumen-total, dokumen-siap,
  WA, utang, piutang, total-beli, total-jual.
- Respons dirakit apa adanya dengan nama `snake_case` tetap (kontrak di Kelompok A).

**Untuk rebuild:** sifat "satu panggilan = satu snapshot konsisten-lepas" yang mengikat,
bukan jumlah 18-nya. Bebas dipecah/di-cache selama nama field dan semantik filter tetap.

## 4. Uang: `money()` 2-Desimal vs `roundRupiah()` Utuh

**Masalah:** rupiah transaksi tak punya sen, tetapi agregat dashboard memakai
pembulatan berbeda dari modul order/payment.

**Hasil yang diharapkan:** semua nominal dashboard tampil stabil 2 desimal di API,
0 desimal di layar.

**Konsep sekarang:**

- Helper lokal `money(value) = Math.round((value + EPS) * 100) / 100`
  (`dashboard.service.ts:31–33`) dipakai di: `getNetSalesAmount` (272),
  `getNetSalesTrend` per bucket (325), `getOutstandingTotal` (508),
  `net_revenue_amount` top-terlaris (628), dan tiga kali di margin
  (`netRevenue`, `estimatedHpp`, `estimatedMargin` 990–992) plus persen margin
  `money(margin / revenue * 100)` (1002).
- Yang TIDAK dibulatkan `money()`: `today_order_count`, `order_count` tren,
  `net_quantity_in_base_uom` (apa adanya!), `count/amount` transaksi-total
  (`getTotalPurchase/SalesTransactions` 511–539 mengembalikan `Number()` mentah!),
  `priority_score`, `age_days`, `transaction_count`.
- Pembanding: modul order/payment/retur memakai `roundRupiah()`
  (`packages/shared-types/src/money.ts:17–18` = `Math.round(v + EPS)`, rupiah utuh)
  — mis. `order.service.ts:89–90`, `payment.service.ts:809–810`,
  `order-pricing.service.ts:94–95`, `sales-return.service.ts:142–143`.
  Dashboard dengan sengaja (atau terbawa) TIDAK memakainya — selisih ≤ Rp 0,50 per
  angka agregat dimungkinkan. Kandidat temuan — lihat `business-rules.md` seksi temuan baru.
- Layar membuang desimal lagi: `formatCurrency` (`apps/web/src/utils.ts:17–23`)
  memakai `Intl id-ID, maximumFractionDigits: 0`; qty memakai `formatQuantity`
  lokal halaman (`dashboard-page.tsx:180–182`, `id-ID, maximumFractionDigits: 2`);
  persen memakai `formatQuantity(persen) + '%'` (536); sumbu tren memakai
  `compact` (452–454); warning margin memakai `compactNumber` (514).

**Untuk rebuild:** putuskan satu kebijakan (utuh vs 2-desimal) dan samakan
dashboard ↔ finance; selama belum diputuskan, pertahankan `money()` apa adanya
agar snapshot E2E (`today_sales_amount ≥ 20000`) tidak bergeser.

## 5. Waktu: Hari/Bulan-Server vs Rentang-Naive Zona-Perusahaan

**Masalah:** server bisa beda zona dengan perusahaan (`Asia/Jakarta`).

**Hasil yang diharapkan:** default masuk akal tanpa filter; bila halaman mengirim
rentang, itulah yang dipakai mentah.

**Konsep sekarang:**

- Tanpa filter: `resolveTodayRange` (130–139) = tengah-malam–23:59:59.999
  kalender **server** (`new Date(y,m,d,…)`); `resolveMonthRange` (141–150) =
  tanggal-1 00:00:00 s/d akhir-bulan 23:59:59.999 server; tren (277–278) =
  awal-bulan-(N-1) s/d akhir-bulan-berjalan server; bucket `YYYY-MM` lokal server
  (1069–1074).
- Dengan filter: `today_from/to`, `month_from/to` dipakai **apa adanya**
  (string diteruskan ke `>=`/`<=`), tanpa konversi zona.
- Halaman SELALU mengirim eksplisit (`dashboard-page.tsx:213–222`):
  `today = formatDateKey(new Date(), company.timezone)` (`sv-SE` di zona perusahaan,
  155–162) + `today_from/to = '{tgl} 00:00:00/23:59:59'` naive;
  `month_from/to` dari `getCurrentMonthRange` (164–178, naive + label `id-ID` bulan-tahun);
  `trend_months: 6`. Refetch hanya saat `activeBranch.id`/`company.timezone` berubah
  (236); tanpa cabang/perusahaan = `setDashboardSummary(null)` + render `null` (207–211, 355–357).
- E2E justru mengirim ISO-UTC (`10-observability…spec.ts:82–85`
  `2026-05-10T00:00:00.000Z` s/d `23:59:59.999Z`) — tercakup perbandingan leksikografis/
  datetime DB apa adanya (KI-133!).

**Untuk rebuild:** yang mengikat adalah "halaman yang menentukan makna hari/bulan,
server hanya membandingkan". Jangan tambah konversi diam-diam.

## 6. Ringkasan Grup-Status + Hitung Hari Ini

**Hasil yang diharapkan:** empat angka order selalu ada (nol bila kosong, bukan null).

**Konsep sekarang:**

- `order_summary` (45–60): `SELECT cs.status_group, COUNT(o.id_order)` dari `orders o`
  `INNER JOIN currentStatus cs`, filter `o.id_branch = :idBranch` + `archived_at IS NULL`,
  `GROUP BY status_group`; yang tidak dikenal diabaikan (`if (row.statusGroup in …)`).
  Semua `order_kind` ikut (beli + jual!). Order tanpa definisi status HILANG (inner join!).
- `today_order_count` (234–242): `COUNT` tanpa join status — filter cabang + arsip +
  `order_date >= from AND <= to`. Artinya order tanpa status IKUT di sini meski hilang
  dari ringkasan (invariansi yang dijaga!).
- `today_sales_amount` / `month_sales_amount`: lihat §7.

## 7. Net Operasional: Serah-Menanggal + Retur-Completed

**Masalah:** omzet harus ikut barang bergerak, dan retur yang selesai harus
mengkoreksi omzet periode koreksinya (bukan periode jualnya).

**Hasil yang diharapkan:** satu angka net per rentang + tren per bulan + hitung order selesai.

**Konsep sekarang (`getNetSalesAmount` 244–273, dipakai 2× untuk hari & bulan):**

```sql
SELECT COALESCE(SUM(amount),0) FROM (
  SELECT SUM(o.total_amount) FROM orders o
    JOIN order_status_definitions osd ON osd.id = o.id_current_status
   WHERE o.id_branch=? AND o.archived_at IS NULL AND o.order_kind='sales'
     AND osd.status_group='completed'
     AND COALESCE(o.goods_delivered_at, o.order_date) BETWEEN ? AND ?
  UNION ALL
  SELECT SUM(sr.difference_amount) FROM sales_returns sr
   WHERE sr.id_branch=? AND sr.archived_at IS NULL AND sr.status='completed'
     AND sr.return_date BETWEEN ? AND ?
) net_sales  →  money()
```

- Tanggal-operasional = `COALESCE(goods_delivered_at, order_date)` untuk jual;
  `return_date` untuk retur. Retur `difference_amount` boleh positif (tukar tambah!)
  sehingga net bisa NAIK karena retur.
- `getCompletedSalesOrderCount` (541–558): `COUNT` cabang pertama saja (tanpa retur!).
- `getNetSalesTrend` (275–329): rentang N bulan server; UNION per `DATE_FORMAT(…,'%Y-%m')`
  untuk jual vs retur; `order_count` hanya dari sisi jual (retur = 0!);
  `GROUP BY month ORDER BY month`; bucket kosong diisi 0 di JS via `buildMonthBuckets`
  + `Map` (320–328); tiap bucket `money()`.

**Untuk rebuild:** pertahankan "serah-menang, retur-di-periode-retur, bucket-kosong-0,
hitung-tanpa-retur".

## 8. Stok Kritis Produk-Level (Agregat Lokasi Aktif)

**Masalah:** satu produk tersebar di banyak lokasi; kritis harus dinilai per produk,
bukan per baris.

**Hasil yang diharapkan:** satu hitungan + 5 baris teratas terurut selisih terbesar.

**Konsep sekarang:**

- Hitung (152–179, `/* dashboard_critical_stock_count */`):
  `FROM inventory_balances ib JOIN products p JOIN stock_locations sl`,
  filter `ib.id_branch=? AND p.id_company=? AND p.stock_tracked=1`
  `AND p.archived_at IS NULL AND sl.archived_at IS NULL AND sl.status='active'`,
  `GROUP BY ib.id_product HAVING MAX(p.min_stock_qty) IS NOT NULL`
  `AND SUM(ib.available_qty) <= MAX(p.min_stock_qty)` → `COUNT(*)` luar.
- Daftar (181–232, `/* dashboard_critical_stock_items */`): kolom
  `MIN(id_inventory_balance), id_product, product_name, SUM(available_qty),`
  `MAX(min_stock_qty), base_uom, MAX(updated_at)`,
  `GROUP BY id_product, name, uom`, HAVING sama,
  `ORDER BY (min − tersedia) DESC, tersedia ASC, nama ASC LIMIT 5`.
- Tanpa ambang (`min_stock_qty NULL`) = tak-pernah-kritis (HAVING menggugurkan).
  Lokasi arsip/nonaktif tidak ikut SUM. `id_inventory_balance` yang dikembalikan
  adalah MIN per produk (kunci navigasi ke `/stock/{idProduk}`, bukan ke saldo itu!).

**Untuk rebuild:** jangan samakan diam-diam dengan definisi per-baris-lokasi milik
`reporting/stock` (KI-124!) atau fallback halaman per-baris (KI-132!).

## 9. Skor Prioritas: Tempo + Umur

**Masalah:** dari puluhan pending/active, tampilkan 5 yang paling perlu disentuh.

**Hasil yang diharapkan:** urutan deterministik + alasan bahasa Indonesia yang tetap.

**Konsep sekarang (`getPriorityOrders` 331–399):**

- Filter: cabang sesi + arsip-keluar + `cs.status_group IN ('pending','active')`.
- Skor SQL: `CASE WHEN due<today →100 WHEN due≤today+2 →80 WHEN pending →60 ELSE 40 END`
  `+ LEAST(GREATEST(DATEDIFF(today, order_date),0),30)` (umur-berbatas-30, masa-depan = 0).
  Kolom tambahan `age_days = DATEDIFF(today, order_date)` (lalu `MAX(0,…)` di JS),
  `days_until_due = DATEDIFF(due, today)`.
- Urut: `priority_score DESC, due_date ASC (NULL terakhir — cek!), order_date ASC`,
  `LIMIT 5`. Join `relatedParty` untuk `party_name`; `currentStatus.label/group` mentah.
- Alasan JS (391–399): `due<0 → 'Lewat jatuh tempo'`; `≤2 → 'Jatuh tempo dekat'`;
  else `pending → 'Menunggu keputusan'`; else `'Sedang diproses'`.

**Untuk rebuild:** ambang 100/80/60/40 + cap-30 + 5-baris + 4-teks-alasan adalah kontrak.

## 10. Utang/Piutang-Net: Kurang Tiga Sumber, Jepit per Baris

**Masalah:** sisa terbuka = total − yang sudah dibayar/di-resolve, tanpa pernah negatif.

**Hasil yang diharapkan:** dua angka (`total_payable` beli, `total_receivable` jual)
yang cocok dengan strip halaman.

**Konsep sekarang (`getOutstandingTotal` 445–509, dipanggil 437–443):**

- Per baris order: `(total_amount + (sales ? ret.total_adjustment : 0))`
  `− (direct_paid + allocated_paid + (sales ? settlement_adjustment : 0))`,
  lalu `GREATEST(…, 0)` per baris sebelum `SUM` luar + `money()`.
- Empat sub-join per cabang: `payments` langsung (`id_order`, arsip-keluar),
  `payment_allocations ⨝ payments` (keduanya arsip-keluar),
  `sales_returns completed` (`SUM(difference_amount)` per `id_original_order`),
  `sales_return_settlements ⨝ sales_returns completed`
  (`collect_payment + / refund − / customer_credit −`, arsip-keluar).
- Filter luar: cabang + arsip-keluar + `order_kind = side` + `status_group != 'cancelled'`
  (INNER JOIN definisi — tanpa status = hilang!). Retur pembelian TIDAK mengurangi
  utang (hanya sisi sales!). Kelebihan-bayar per baris hilang (`GREATEST`), bukan
  negatif/pos-kredit.

**Untuk rebuild:** putuskan apakah kelebihan perlu pos sendiri; selama belum,
pertahankan jepit-per-baris.

## 11. Top Terlaris: Omzet-Tanpa-Pajak, Retur-Net

**Hasil yang diharapkan:** 5 produk teratas bulan berjalan terurut qty-net.

**Konsep sekarang (`getTopSoldProducts` 560–631):**

- Sisi order: `quantity_in_base_uom`, revenue =
  `NULLIF(line_total_before_tax,0) else line_total − tax else line_total`,
  `doc_key='order:{id}'`, `event_date=COALESCE(goods_delivered_at, order_date)`.
- Sisi retur: qty `returned → −qty else +qty` (pengganti menambah!),
  revenue sama dengan tanda yang sama, `doc_key='return:{id}'`, `event_date=return_date`,
  hanya `line_type IN ('returned','replacement')`, status `completed`, arsip-keluar.
- Luar: `GROUP BY id_product`, identitas (kode/nama/satuan) diambil dari baris
  terbaru via `GROUP_CONCAT(… ORDER BY event_date DESC, doc_key DESC)` +
  `SUBSTRING_INDEX(…,1)`; `HAVING ABS(qty)>0.0001 OR ABS(Rp)>0.009`;
  `ORDER BY qty DESC, revenue DESC, name ASC LIMIT 5`; revenue `money()`,
  qty apa adanya; `transaction_count = COUNT(DISTINCT doc_key)`.

## 12. Estimasi Margin: Hierarki 5 Sumber + Status Jujur

**Masalah:** HPP sebenarnya milik Finance pasca-posting; dashboard butuh angka
"sementara yang jujur" untuk keputusan cepat.

**Hasil yang diharapkan:** 5 produk margin-tertinggi + status yang mengaku estimasi.

**Konsep sekarang (`getEstimatedMarginProducts` 633–1048):**

1. **Pendapatan** (`revenueRows` 685–745): agregat per
   `(id_product, code, name, uom)` — qty-net + revenue-net (aturan sama §11) +
   `COUNT(DISTINCT doc_key)` + `MAX(event_date)`.
2. **HPP lapis-1 — snapshot pasti** (`snapshotCostedRows` 752–781):
   baris `order_items` yang `margin_costed_at IS NOT NULL` memakai
   `SUM(cogs_amount_snapshot)` langsung, SEKALI per baris (bukan per alokasi —
   komentar PLAN.md Fase 4 D3, 747–751!), `costed_movement_count++`.
3. **HPP lapis-2 — alokasi belum-snapshot** (`orderCostRows` 783–831):
   `stock_issue_allocations ⨝ order_items (margin_costed_at IS NULL)`
   `⨝ orders completed ⨝ inventory_movements out`
   `LEFT ficm (cost_movements per company+movement)`
   `LEFT fics (average_cost per company+branch+product)`
   `LEFT products (purchase_price / purchase_to_base_factor)`:
   per alokasi `ficm.total_cost_amount → qty×average_cost → qty×harga-beli-konversi → 0`,
   counter `costed/movement_snapshot(0!)/average/purchase/missing` per kondisi.
4. **HPP lapis-3 — sisa belum-alokasi** (`unallocatedOrderCostRows` 833–895):
   `qty − allocated_qty` (GREATEST(…,0)) × rata-rata/beli, hanya
   `stock_tracked_snapshot=1` + `margin_costed_at IS NULL`,
   `HAVING SUM(sisa)>0.0001`.
5. **HPP lapis-4 — retur** (`returnCostRows` 897–968): `(returned→−1 else +1) ×`
   `(ficm → snapshot JSON `returnUnitCost` via `JSON_EXTRACT(metadata_json,'$.returnUnitCost')``
   `→ rata-rata → beli → 0)`, hanya baris bermovement (`id_inventory_movement NOT NULL`
   untuk counter; tanpa movement = HPP 0 tanpa counter!).
6. **Rakit JS** (970–1018): `Map` per produk (identitas = snapshot terbaru,
   `getProduct` 652–683), filter tampil `|revenue|>0.009 ATAU |hpp|>0.009`,
   `money()` tiap komponen, `margin = money(revenue − hpp)`,
   `persen = revenue>0 ? money(margin/revenue*100) : 0` (KI-135!),
   urut `margin DESC (>0.009) → revenue DESC → nama`, `slice(0,5)`.
7. **Status** (1020–1047): jumlahkan semua counter + `basis:'operational_estimate'`,
   `is_estimate:true`, `cost_basis = resolveEstimatedCostBasis`:
   `missing>0 → 'incomplete'`; else kumpulkan sumber-terpakai
   (`cost_movement/movement_snapshot/average_cost/purchase_price`);
   0 → `'no_stock_cost'`; >1 → `'mixed'`; 1 → nama itu (1050–1067).

**Untuk rebuild:** hierarki + snapshot-sekali + ambang-tampil + urut-margin +
status-jujur adalah satu paket; label halaman yang jujur
(`Estimasi`, `HPP memakai cost movement, average cost, lalu harga beli…`) wajib ikut.
