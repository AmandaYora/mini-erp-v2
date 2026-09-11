# Feature Inventory — Modul 18 Dashboard

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diturunkan dari
`dashboard.controller.ts` (21 baris, penuh), `dashboard.service.ts` (1075 baris,
penuh), `dashboard.service.spec.ts` (337 baris — 9 test dibaca), `dashboard-page.tsx`
(628 baris, penuh), `use-dashboard-module.ts` + test-nya, registry (`/dashboard`,
grup "Utama"), `role-access-config` (`dashboard.view`), `utils.ts`
(formatCurrency/formatDateTime/compactNumber/statusTone), dan E2E 10.

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 1 — `POST dashboard/summary` (`dashboard.controller.ts:15–20`: `@Post('summary')` + `@HttpCode(200)` + `@RequirePermission('dashboard.view')` + body `{ data: DashboardFilters }`; tanpa cabang = `403 Branch aktif belum dipilih` via `BranchGuard`; tanpa token = 401 E2E!) |
| Permission | `dashboard.view` (default superadmin/owner/admin/staff! — label `Dashboard: Lihat`; registry `module-registry.tsx:200–207` + `role-access-config.ts:37–40`) |
| Guard | JWT + permission + **BranchGuard** (cabang aktif wajib!; `idActiveBranch!` + `idCompany` dari sesi — tak ada param cabang!) |
| Filter request | `trend_months` (jepit 1–12, default 6, `getSummary:64`), `today_from/to`, `month_from/to` (semua opsional; string dipakai mentah; tanpa = hari/bulan kalender **server**!) |
| Respons | 20 kunci teratas (dokumen lama menyebut 21 — cek! `dashboard.service.ts:106–127` = 20): `order_summary{pending,active,completed,cancelled}`, `critical_stock_count`, `critical_stock_items[≤5]`, `today_order_count`, `today_sales_amount(money)`, `sales_trend[{month,total_sales:money,order_count}]`, `priority_orders[≤5]`, `month_sales_amount(money)`, `month_sales_order_count`, `top_sold_products_month[≤5]`, `top_estimated_margin_products_month[≤5]`, `estimated_margin_status`, `tracked_product_count`, `knowledge_document_count`, `knowledge_ready_count`, `whatsapp_status{state,phone,updated_at}`, `total_payable(money)`, `total_receivable(money)`, `total_purchase_transactions{count,amount}`, `total_sales_transactions{count,amount}` |
| Halaman | 1 — `/dashboard` "Ringkasan Operasional" (menu "Utama" → "Dashboard"; halaman default setelah login! `auth-redirect.ts:1`; `FALLBACK_PATH=/dashboard`; tanpa perusahaan/cabang → render `null`!) |
| Uang | 2 desimal di API (`money()`; kecuali transaksi-total mentah!); tampil 0 desimal (`formatCurrency id-ID`, `utils.ts:17–23`); qty ≤2 desimal; persen via `formatQuantity`; tren compact; waktu tanpa detik (`formatDateTime`, `utils.ts:25–37`) |
| Audit | Tidak menulis (modul baca + exempt) |
| Penomoran | Tidak ada |
| Aksi yang TIDAK ada | Tulis/ubah/hapus apa pun; filter cabang (selalu sesi!); ekspor; refresh manual (refetch saat cabang/timezone berubah!); realtime |

**Karakter modul.** Satu panggilan agregat + halaman KPI. Halaman punya **fallback
workspace** untuk 6 angka bila API gagal — dengan logika BERBEDA dari server
(tanpa penanda! — lihat KI-132).

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Ringkasan Order (`order_summary`: pending/active/completed/cancelled)

Hitung semua order tak-diarsip per grup status (grup tak dikenal diabaikan!).
Halaman memakai pending+active sebagai `Order Perlu Aksi`; completed/cancelled
diambil tapi **tak ditampilkan** (KI-134!).

### F-02 — Stok Kritis (`critical_stock_count` + `critical_stock_items`, maks 5!)

Definisi produk-level: jumlah stok aktif (lokasi aktif + tak-diarsip) per produk
dibanding `min_stock_qty`; produk tanpa ambang tak-pernah-kritis. Daftar 5 teratas
diurut selisih-terbesar (produk, sisa, minimum, satuan, update-terakhir).
**Sama definisinya dengan filter stok** (dijaga spec!) — tapi BEDA dengan fallback
halaman per-baris-lokasi dan BEDA dengan `reporting/stock` (KI-124, KI-132!).

### F-03 — Hari Ini (`today_order_count` + `today_sales_amount`)

Hitung (`getTodayOrderCount 234–242`) = semua jenis order dalam rentang
(`orders`: cabang + arsip-keluar + `order_date BETWEEN from AND to`; TANPA join status —
order tanpa status ikut!); nominal (`getNetSalesAmount 244–273` + `money()`) =
penjualan-selesai-net (sales completed + selisih-retur-completed; tanggal =
`COALESCE(goods_delivered_at, order_date)` vs `return_date`!).
Default rentang = hari kalender **server** (`resolveTodayRange 130–139`); halaman selalu
kirim eksplisit zona perusahaan (`YYYY-MM-DD HH:mm:ss` naive via `formatDateKey sv-SE`,
`dashboard-page.tsx:155–162, 213–222`!); E2E kirim ISO-UTC (`10-observability…:82–85`,
KI-133!). Fallback halaman bila null: hitung dari `workspaceOrders` tanggal-sama zona
perusahaan + omzet `sales+completed` lokal tanpa retur (238–254!).

### F-04 — Bulan Ini + Transaksi Total (`month_sales_amount`, `month_sales_order_count`, `total_purchase/sales_transactions`)

Nominal & hitung penjualan-selesai-net sebulan (aturan tanggal sama F-03) +
total transaksi pembelian/penjualan completed sepanjang masa (hitung + nominal!).
Dua total terakhir **tak ditampilkan di mana pun** (KI-134!).

### F-05 — Tren Penjualan (`sales_trend`: N bulan, label `YYYY-MM`)

Bucket per bulan: penjualan-selesai + selisih-retur;
`order_count` hanya order (retur = 0!); bulan kosong TETAP muncul bernilai 0!).
Default 6, min 1, maks 12.

### F-06 — Pesanan Prioritas (maks 5!)

Order pending/active tak-diarsip + skor (`lewat-tempo 100 / dekat-2-hari 80 /
pending 60 / else 40` + umur-hari cap-30!) + alasan Indonesia
(`Lewat jatuh tempo`/`Jatuh tempo dekat`/`Menunggu keputusan`/`Sedang diproses`).
Urut skor-turun, tempo-naik, order-naik.

### F-07 — Top-5 Terlaris + Estimasi Margin (`top_sold_products_month`, `top_estimated_margin_products_month`, `estimated_margin_status`)

Terlaris: qty-net-basis + omzet-net-tanpa-pajak (urutan: `line_total_before_tax`
else `line_total − pajak`!) − retur + pengganti, ambang tampil
(qty>0.0001 ATAU Rp>0.009!), urut qty-turun. Margin: HPP-estimasi per hierarki
(cost-movement → snapshot-retur → rata-rata → beli/UOM → 0-hilang!) + status
kejujuran (`is_estimate: true`, `basis: 'operational_estimate'`, counter per
sumber + label `cost_movement/movement_snapshot/average_cost/purchase_price/
missing/mixed/no_stock_cost/incomplete`); persen = margin ÷ omzet (omzet ≤ 0 → **0!**
— KI-135); saring tampil (|omzet|>0.009 ATAU |HPP|>0.009); urut margin-turun;
peringatan `Estimasi cost belum lengkap` bila ada baris tanpa-modal
(`{n} baris stok belum punya average cost atau harga beli.`).
Label halaman jujur: badge `Estimasi` + `HPP memakai cost movement, average cost,
lalu harga beli jika diperlukan.` Final/auditable tetap milik Finance!

### F-08 — Pendamping (`tracked_product_count`, `knowledge_document/ready_count`, `whatsapp_status`, `total_payable`, `total_receivable`)

Produk-lacak aktif perusahaan + dokumen-SOP (total & siap) + WA
(`state` else `disconnected`, `phone` else `''`, `updated_at` else null;
label `Terhubung` bila connected/ready, `Menghubungkan…` bila connecting,
else `Belum terhubung` + `{n} SOP siap dipakai.`) + utang/piutang-terbuka
(total − bayar-langsung − alokasi − penyelesaian-retur-penjualan, per-baris
dijepit ≥ 0!, status batal dikecualikan!).

---

## 3. Edge Case (16)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Tanpa cabang aktif | `403 Branch aktif belum dipilih` |
| E-02 | Staff | Boleh (punya `dashboard.view`!) |
| E-03 | Tanpa token | 401 (E2E membuktikan!) |
| E-04 | Filter kosong | Hari-server + bulan-server + tren-6 |
| E-05 | `trend_months: 99` | 12 (jepit!); `0`/negatif → 1 |
| E-06 | Grup status tak dikenal | Diabaikan dari ringkasan (tetap di total prioritas? TIDAK — prioritas hanya pending/active!) |
| E-07 | Order tanpa definisi status | Hilang dari ringkasan/transaksi (INNER JOIN!) tapi ikut hitung-hari-ini (tanpa join!) |
| E-08 | Produk tanpa `min_stock_qty` | Tak-pernah-kritis |
| E-09 | Lokasi arsip/nonaktif | Stoknya tak dihitung (definisi "aktif"!) |
| E-10 | Retur completed | Menambah nominal-net (positif bila selisih positif!) |
| E-11 | Omzet ≤ 0 di margin | Persen = 0 (bukan negatif! KI-135) |
| E-12 | Tanpa modal sama sekali | `cost_basis: 'incomplete'` + badge Estimasi + warning |
| E-13 | WA tanpa channel | `disconnected` + `''` + null |
| E-14 | Utang-bayar-lebih | Per-baris dijepit 0 (kelebihan hilang dari total!) |
| E-15 | API gagal di halaman | Fallback workspace diam-diam (logika beda! KI-132) |
| E-16 | Tanpa perusahaan/cabang di store | Halaman null (tanpa error!) |

---

## 4. Katalog Pesan (teks apa adanya)

Navigasi/UI: `Ringkasan Operasional` · `Pantauan cabang {nama} dari transaksi, stok,
dan finance yang sudah tercatat.` · `Buat Pesanan Baru` (bila boleh!) · `Order Perlu
Aksi` (`{p} pending, {a} aktif`) · `Penjualan Net Bulan Ini` (`{n} order selesai pada
{bulan}`) · `Piutang Terbuka` (`Sudah memperhitungkan pembayaran, alokasi, dan retur`) ·
`Stok Kritis` (`Total stok aktif cabang dibandingkan minimum produk`) · `Penjualan Net
Hari Ini` (`{n} pesanan tercatat hari ini.`) · `Utang Supplier Terbuka` (`Saldo pembelian
yang belum dilunasi.`) · `Produk Stok Dipantau` (`Produk aktif yang memakai kontrol stok.`) ·
`Kanal WhatsApp` (`{n} SOP siap dipakai.`) · `Terhubung`/`Menghubungkan…`/`Belum terhubung` ·
`Tren Penjualan Net` (`Nilai penjualan selesai setelah koreksi retur dan tukar barang.`) ·
`Penjualan Net` (tooltip) · `5 Barang Paling Laku Bulan Ini` (`Periode {bulan}.`) ·
`{kode|Tanpa kode} | {n} transaksi` · `Belum ada penjualan bulan ini` (`Produk terlaris
muncul setelah ada penjualan selesai atau retur selesai pada bulan berjalan.`) ·
`5 Produk Estimasi Margin Tertinggi Bulan Ini` (`Berdasarkan transaksi selesai bulan
berjalan. HPP memakai cost movement, average cost, lalu harga beli jika diperlukan.`) ·
`Estimasi` · `Estimasi cost belum lengkap` (`{n} baris stok belum punya average cost atau
harga beli.`) · `Omzet {x} | Est. HPP {y}` · `Estimasi margin belum bisa dihitung`
(`Transaksi selesai sudah ada, tetapi cost produk belum cukup untuk membentuk ranking
margin.`) · `Belum ada penjualan selesai bulan ini` (`Estimasi margin muncul setelah ada
penjualan selesai atau retur selesai pada bulan berjalan.`) · `Pesanan Prioritas`
(`Pesanan {cabang} yang paling perlu ditindaklanjuti.` + `Lihat Semua`) ·
`Pihak terkait belum dipilih` · `Jatuh tempo {tgl|-}` · `Status belum diketahui` ·
`Tidak ada pesanan prioritas` (`Tidak ada pesanan yang perlu tindakan cepat saat ini.`) ·
`Stok Kritis` (`Produk yang total stok aktifnya sudah berada di bawah atau sama dengan
batas minimum.` + `Buka Stok`) · `Tersisa {n}` · `Minimum {n} {satuan} | update terakhir
{tgl}` · `Tidak ada stok kritis` (`Semua item tracked masih berada di atas batas minimum.`).
Alasan prioritas: `Lewat jatuh tempo`/`Jatuh tempo dekat`/`Menunggu keputusan`/`Sedang diproses`.
Backend: modul ini **tanpa pesan khas** (angka + label mentah!).

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Tampil completed/cancelled/transaksi-total | Diambil, tak dirender (KI-134) |
| NF-02 | Penanda mode-fallback | `??` diam-diam (KI-132) |
| NF-03 | Refresh manual/realtime/ekspor | Refetch hanya saat cabang/timezone berubah |
| NF-04 | Preload slice oleh hook | Test eksplisit: hook tak reload apa pun! |
| NF-05 | Direktori `apps/e2e/tests/dashboard/` | Tak ada (KI-130 mencakup!) |
| NF-06 | Audit untuk baca | Exempt (observability baca!) |
| NF-07 | Filter/pilih cabang di request | Selalu sesi (arsitektur!) |
| NF-08 | Detik di waktu + desimal di nominal tampil | Format tanpa detik/desimal (disengaja!) |
