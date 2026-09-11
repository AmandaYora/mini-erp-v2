# Business Rules — Modul 18 Dashboard

**Kelompok A.** Sumber: baca penuh service 1075 baris + halaman + spec + E2E.

---

## 1. Aturan Akses & Request

| ID | Aturan |
|---|---|
| BR-01 | JWT + `dashboard.view` (semua role bawaan termasuk staff!) + BranchGuard. |
| BR-02 | Filter opsional: `trend_months` (jepit 1–12, default 6), `today_from/to`, `month_from/to` (tanpa = hari/bulan kalender **server**). |
| BR-03 | Selalu lingkup cabang sesi (+ perusahaan untuk produk/knowledge/WA/cost!). |

## 2. Aturan Angka (kontrak presisi!)

| ID | Aturan |
|---|---|
| BR-04 | Uang API = 2 desimal (`money()` di `dashboard.service.ts:31–33` = `round((v+EPS)*100)/100`); persen margin = 2 desimal (`money()` juga!); qty apa-adanya (`toNumber`, tanpa bulat!); hitung = bulat. Pengecualian: `total_purchase/sales_transactions.amount` = `Number()` mentah tanpa `money()` (`dashboard.service.ts:511–539`); `priority_score`/`age_days`/`transaction_count` tanpa `money()`. Tampil selalu 0 desimal (`formatCurrency`, `utils.ts:17–23`). Beda dari `roundRupiah()` utuh milik order/payment — lihat temuan baru di bawah! |
| BR-05 | Tanggal-operasional = `COALESCE(goods_delivered_at, order_date)` (serah-menang!) untuk sales (`dashboard.service.ts:258, 286, 553, 579, 705, 759, 790, 826`); `return_date` untuk retur (`265–266, 303–311, 604–611, 731–739`). Tanpa filter = hari/bulan kalender **server** (`resolveTodayRange 130–139`, `resolveMonthRange 141–150`, tren `277–278`); dengan filter = string dipakai mentah tanpa konversi zona. Halaman selalu kirim naive zona-perusahaan (`dashboard-page.tsx:155–178, 213–222`); E2E kirim ISO-UTC (`10-observability…:82–85`) — keduanya dibandingkan apa adanya (KI-133!). |
| BR-06 | Net = sales-completed (`orders`: `id_branch=?` + `archived_at IS NULL` + `order_kind='sales'` + `status_group='completed'` via `order_status_definitions`) + `SUM(difference_amount)` retur-completed (`sales_returns`: `id_branch=?` + `archived_at IS NULL` + `status='completed'`) — retur boleh positif (tukar-tambah menaikkan net!). Rumus SQL di `dashboard.service.ts:246–270` (hari/bulan) dan `282–318` (tren). Arsip selalu keluar; order tanpa definisi status hilang dari semua query ber-JOIN (`INNER JOIN`, mis. 49, 292, 501) tapi IKUT `today_order_count` yang tanpa join (234–242!). |
| BR-07 | Omzet produk = `COALESCE(NULLIF(line_total_before_tax,0), line_total − tax, line_total)` per baris (`order_items 578, 703`; `sales_return_items 600–601, 726–729`; retur `returned` dinegasi, `replacement` positif!). HPP hierarki per alokasi/baris: `finance_inventory_cost_movements.total_cost_amount` → snapshot JSON `returnUnitCost` (`metadata_json`, khusus retur `returned`!) → `finance_inventory_cost_states.average_cost` → `products.purchase_price / purchase_to_base_factor` → 0; baris `margin_costed_at NOT NULL` memakai `cogs_amount_snapshot` SEKALI per baris (`752–781`, bukan per alokasi!). Persen = `revenue>0 ? money(margin/revenue*100) : 0` (1002; KI-135!). Sumber tabel/kolom persis di `algorithms-legacy.md` §12. |
| BR-08 | Ambang tampil: terlaris `HAVING ABS(qty)>0.0001 OR ABS(Rp)>0.009` + `ORDER BY qty DESC, revenue DESC, nama ASC LIMIT 5` (615–617); margin `filter \|revenue\|>0.009 ATAU \|hpp\|>0.009` + urut `margin DESC (>0.009) → revenue DESC → nama` + `slice(0,5)` (987–1018). Identitas terlaris = snapshot terbaru via `GROUP_CONCAT … ORDER BY event_date DESC, doc_key DESC` (565–567); margin = snapshot terbaru via `getProduct` JS (652–683). |
| BR-09 | Kritis = agregat-produk-lokasi-aktif vs ambang: `inventory_balances ib ⨝ products p ⨝ stock_locations sl`, filter `ib.id_branch=? AND p.id_company=? AND p.stock_tracked=1 AND p.archived_at IS NULL AND sl.archived_at IS NULL AND sl.status='active'`, `GROUP BY id_product HAVING MAX(min_stock_qty) IS NOT NULL AND SUM(available_qty) <= MAX(min_stock_qty)` (152–179, 181–232). Tanpa-ambang = tak-pernah-kritis! Daftar 5: `ORDER BY (min−tersedia) DESC, tersedia ASC, nama ASC LIMIT 5` (215–218); kolom `MIN(id_balance), SUM(tersedia), MAX(min), MAX(updated_at)`. BEDA dengan `reporting/stock` per-baris dan fallback halaman per-baris (KI-124, KI-132!). |
| BR-10 | Prioritas: hanya `status_group IN ('pending','active')`, arsip-keluar, cabang-sesi; maks 5. Skor SQL `CASE due<today→100; due≤today+2→80; pending→60; else→40 END + LEAST(GREATEST(DATEDIFF(today,order_date),0),30)` (344–351); urut `skor DESC, due ASC, order ASC` (358–361). Alasan tetap: `due<0→'Lewat jatuh tempo'; ≤2→'Jatuh tempo dekat'; pending→'Menunggu keputusan'; else→'Sedang diproses'` (391–399). `age_days = MAX(0, DATEDIFF(today,order_date))` (387). `due NULL` → alasan jatuh ke status (cek!). |
| BR-11 | Utang (`purchase`) / piutang (`sales`): per baris `(total_amount + (sales? ret.total_adjustment:0)) − (direct_paid + allocated_paid + (sales? settlement_adjustment:0))`, lalu `GREATEST(…,0)` per baris sebelum `SUM` + `money()` (445–509). Sumber: `payments` langsung + `payment_allocations⨝payments` + `sales_returns completed SUM(difference_amount)` + `sales_return_settlements (collect_payment +/refund −/customer_credit −)⨝sales_returns completed` — semua lingkup `id_branch=?` + arsip-keluar. Filter luar: `order_kind=side` + `status_group != 'cancelled'` (INNER JOIN!). Retur-pembelian tak mengurangi utang! Kelebihan-bayar hilang (jepit 0). |
| BR-12 | Tren: bucket `YYYY-MM` N-bulan (`trend_months` dijepit 1–12 default 6, `buildMonthBuckets 1069–1074`); bulan kosong TETAP muncul 0 via `Map` (320–328); tiap bucket `money()`; `order_count` hanya sisi order (retur = 0 per baris SQL 306!). Label layar `Mon yyyy id-ID` (`dashboard-page.tsx:319–322`); fallback lokal tanpa-retur sama sekali (344–350). |
| BR-13 | Status margin selalu `basis:'operational_estimate' + is_estimate:true` + 6 counter (`transaction/costed_movement/movement_snapshot/average_cost/purchase_price/missing_cost`) + label `cost_basis`: `missing>0→'incomplete'`; else 0 sumber→`'no_stock_cost'`; >1→`'mixed'`; 1→nama sumber (`cost_movement/movement_snapshot/average_cost/purchase_price`) — `resolveEstimatedCostBasis 1050–1067`, dihitung per produk (1004) dan agregat (1045). Tak-pernah-mengklaim-final; badge `Estimasi` + warning bila `missing>0` wajib tampil! |

### Rincian per metrik → tabel/kolom/filter (tambahan presisi, tanpa ID baru)

- `order_summary` ← `orders.id_branch, archived_at` + `order_status_definitions.status_group` (inner join `currentStatus`). Semua jenis ikut.
- `today_order_count` ← `orders.order_date` dalam `[today_from, today_to]` (inklusif `>=`/`<=`); semua jenis + semua status (termasuk tanpa-status!).
- `today/month_sales_amount` + `month_sales_order_count` ← rumus BR-06; hitung-bulan hanya sisi order (`541–558`).
- `sales_trend[]` ← `{month:'YYYY-MM', total_sales:money, order_count:int}` sepanjang N bulan server.
- `critical_stock_count/items[]` ← rumus BR-09; item = `{id_inventory_balance:MIN, id_product, product_name, available_qty:SUM, min_stock_qty:MAX, base_uom, updated_at:MAX}`.
- `priority_orders[]` ← `{id_order, order_number, order_date, due_date, notes, status_label, status_group, party_name, priority_score:number, priority_reason:4-teks, age_days:≥0}`.
- `top_sold_products_month[]` ← `{id_product, product_code:'' bila null (624!), product_name:'Produk #id' bila null (625!), base_uom:'' bila null, net_quantity_in_base_uom:number, net_revenue_amount:money, transaction_count:DISTINCT doc}`.
- `top_estimated_margin_products_month[]` ← top + `{estimated_hpp_amount:money, estimated_margin_amount:money, estimated_margin_percent:money, cost_basis:label, costed/movement_snapshot/average/purchase/missing counts}`; `estimated_margin_status` = agregat + `basis/is_estimate`.
- `tracked_product_count` ← `products: id_company=? + archived NULL + status='active' + stock_tracked=1` (401–409; lingkup perusahaan, bukan cabang!).
- `knowledge_document/ready_count` ← `knowledge_documents: id_company=? + archived NULL (+ status='ready')` (411–426; lingkup perusahaan!).
- `whatsapp_status` ← `whatsapp_channels.findOne({idCompany})` → `{state:??'disconnected', phone:??'', updated_at:??null}` (428–435; lingkup perusahaan!).
- `total_payable/receivable` ← rumus BR-11 (uang `money()`).
- `total_purchase/sales_transactions` ← `{count, amount:Number-mentah!}` order `completed` sepanjang masa per `order_kind` (511–539; tak tampil di UI!).

### 4 KPI + tren + ranking + teks persis di UI (tambahan presisi, tanpa ID baru)

- KPI-1 `Order Perlu Aksi` = `pending+active`; hint `{p} pending, {a} aktif`; nada `warn bila >0 else ok`; klik → `/orders?status_group=pending` (`dashboard-page.tsx:375–381`).
- KPI-2 `Penjualan Net Bulan Ini` = `formatCurrency(month_sales_amount)`; hint `{n} order selesai pada {Bulan Tahun id-ID}`; nada `info`; klik → `/orders?status_group=completed` (383–388).
- KPI-3 `Piutang Terbuka` = `formatCurrency(total_receivable)`; hint `Sudah memperhitungkan pembayaran, alokasi, dan retur`; nada `warn bila >0 else ok`; klik → `/finance/receivables-payables` (390–395).
- KPI-4 `Stok Kritis` = `critical_stock_count`; hint `Total stok aktif cabang dibandingkan minimum produk`; nada `bad bila >0 else ok`; klik → `/stock?critical_only=true` (397–402).
- Strip sekunder 4 sel: `Penjualan Net Hari Ini` (`{n} pesanan tercatat hari ini.`) · `Utang Supplier Terbuka` (merah bila >0; `Saldo pembelian yang belum dilunasi.`) · `Produk Stok Dipantau` (`Produk aktif yang memakai kontrol stok.`) · `Kanal WhatsApp` (titik + `Terhubung`/`Menghubungkan…`/`Belum terhubung` + `{n} SOP siap dipakai.`) — `dashboard-page.tsx:407–431`.
- Tren: kartu `Tren Penjualan Net` + deskripsi `Nilai penjualan selesai setelah koreksi retur dan tukar barang.`; bar 40/56px radius-atas-6; tooltip `Penjualan Net` + nominal penuh; sumbu compact (434–471).
- Ranking terlaris: `5 Barang Paling Laku Bulan Ini` + `Periode {bulan}.`; baris `nama` + `{kode|Tanpa kode} | {n} transaksi` + qty + satuan-else-`unit`; klik → `/products/{id}`; kosong → info `Belum ada penjualan bulan ini` + `Produk terlaris muncul setelah ada penjualan selesai atau retur selesai pada bulan berjalan.` (474–503).
- Ranking margin: `5 Produk Estimasi Margin Tertinggi Bulan Ini` + `Berdasarkan transaksi selesai bulan berjalan. HPP memakai cost movement, average cost, lalu harga beli jika diperlukan.` + badge `Estimasi`; warning `Estimasi cost belum lengkap` + `{n} baris stok belum punya average cost atau harga beli.` (compact!); baris `Omzet {x} | Est. HPP {y}` + margin + `{persen}%`; 3 kosong: info `Belum ada penjualan selesai bulan ini` + `Estimasi margin muncul setelah ada penjualan selesai atau retur selesai pada bulan berjalan.` vs warning `Estimasi margin belum bisa dihitung` + `Transaksi selesai sudah ada, tetapi cost produk belum cukup untuk membentuk ranking margin.` (505–550).
- Prioritas: `Pesanan Prioritas` + `Pesanan {cabang} yang paling perlu ditindaklanjuti.` + `Lihat Semua`→`/orders`; baris `nomor` + badge + `{pihak|Pihak terkait belum dipilih}` + `Jatuh tempo {tgl|-}` + `{alasan|catatan|-}`; klik → `/orders/{id}`; kosong → success `Tidak ada pesanan prioritas` + `Tidak ada pesanan yang perlu tindakan cepat saat ini.` (554–588).
- Kritis: `Stok Kritis` + `Produk yang total stok aktifnya sudah berada di bawah atau sama dengan batas minimum.` + `Buka Stok`→`/stock?critical_only=true`; baris `nama` + `Tersisa {n}` + `Minimum {n} {satuan} | update terakhir {tgl}`; klik → `/stock/{idProduk}`; kosong → success `Tidak ada stok kritis` + `Semua item tracked masih berada di atas batas minimum.` (590–624).

## 3. Aturan Tampil & Fallback

| ID | Aturan |
|---|---|
| BR-14 | Halaman kirim rentang zona-perusahaan naive + tren-6; refetch saat cabang/timezone berubah; gagal → fallback workspace (KI-132!). |
| BR-15 | Tampil: pending+active (completed/cancelled/transaksi-total diambil-tak-tampil! KI-134); knowledge siap-else-total; WA label ramah (connected/ready→Terhubung!). |
| BR-16 | Format: nominal tanpa-desimal, qty ≤2-desimal, waktu tanpa-detik, tren compact, persen `formatQuantity`. |
| BR-17 | Hook tanpa preload (dijaga test!); tanpa-cabang = null; `summary-card.tsx` tak dipakai halaman. |

## 4. Katalog Pesan

Backend tanpa pesan khas. Seluruh string UI di feature-inventory §4 (dipakai apa adanya!).

## Temuan baru & pertanyaan terbuka (belum ber-ID)

- Uang 2-desimal vs rupiah utuh: `dashboard.service.ts:31–33` (`money()` ×100÷100) dipakai semua nominal dashboard, sedangkan order/payment/retur memakai `roundRupiah()` utuh (`packages/shared-types/src/money.ts:17–18`; dipakai `apps/api/src/modules/order/order.service.ts:89–90`, `payment.service.ts:809–810`, `order-pricing.service.ts:94–95`, `sales-return.service.ts:142–143`). Agregat dashboard bisa berselisih ≤ Rp 0,50 per angka dari angka transaksional; `total_purchase/sales_transactions.amount` bahkan tanpa `money()` sama sekali (`dashboard.service.ts:511–539`). Perlu keputusan: samakan ke utuh atau pertahankan 2-desimal?
- `due_date NULL` di prioritas: skor jatuh ke cabang `pending→60 else 40` (`dashboard.service.ts:344–351`) dan alasan jatuh ke status (`391–399`); `days_until_due` null. Apakah order tanpa tempo boleh kalah prioritas dari order bertempo jauh? Kode tidak menjelaskan — cek!
- `id_inventory_balance` kritis = `MIN()` per produk (`dashboard.service.ts:194`), tetapi halaman menavigasi ke `/stock/{idProduk}` (`dashboard-page.tsx:605`) sehingga id saldo itu tak dipakai navigasi. Apakah field ini kontrak atau sisa? Cek!
- `order_count` tren selalu 0 untuk bulan yang hanya berisi retur (`dashboard.service.ts:306, 320–328`), sehingga tooltip tren bisa menampilkan Rp ≠ 0 dengan 0 order. Disengaja? Cek!
- `tracked_product_count` lingkup perusahaan (`dashboard.service.ts:401–409`) sedangkan `critical_stock_count` lingkup cabang+perusahaan; halaman melabeli keduanya seolah satu cabang (`dashboard-page.tsx:419–421` vs `397–402`). Bila perusahaan multi-cabang, angka pantau ≠ angka kritis. Disengaja? Cek!
- `knowledge_ready_count ?? knowledge_document_count` di halaman (`dashboard-page.tsx:295–298`): bila siap = 0 tetapi total > 0, yang tampil total (bukan 0!). Fallback menutupi "ada dokumen tapi belum siap". Perlu penanda? Cek!
- WA `ready` dianggap terhubung di halaman (`dashboard-page.tsx:302`) tetapi backend hanya meneruskan `sessionStatus` mentah (`dashboard.service.ts:428–435`); nilai `ready` tidak pernah ditulis backend yang dibaca (`whatsapp-channel.entity.ts:18` hanya `connected/reconnecting/disconnected` — cek! dari mana `ready` berasal).
