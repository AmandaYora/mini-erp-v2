# Rancangan F3a — Paritas Layar OPERASIONAL (Step 5, re-trace untuk reuse)

Status dokumen: rancangan siap-implementasi. Semua bukti `file:baris` hasil bacaan langsung kode revamp
(`H:/dimasprasetio/SAAS/mini-erp-revamp`) dan legacy (`H:/dimasprasetio/SAAS/mini-erp`). Klaim audit yang
terbantah/sebagian ditandai eksplisit. Tidak ada kode yang diubah.

Konvensi di dokumen ini:
- "FE" = `apps/web/src`, "BE" = `apps/api/internal`.
- Nomor migrasi ditulis `0000NN_<nama>`: nomor riil diambil saat merge (ada agen lain yang juga menambah
  migrasi; terakhir di repo = 000031, 000026–000029 kosong). Satu migrasi = satu modul (BACKEND_GUIDE).
- Respons selalu envelope `{success,message,data[,meta{page,limit,total,totalPages}]}`; uang integer rupiah.
- Tanggal filter `from`/`to` = `YYYY-MM-DD` (WIB), diterapkan setengah-terbuka `>= from 00:00:00 AND < to+1hari`
  (menghindari celah 23:59:59.xxxxxx pada DATETIME(6); `order_date` disimpan sebagai tanggal lokal jam 00:00).

## 0. Ringkasan temuan verifikasi (koreksi atas audit)

| # | Klaim audit | Hasil baca kode | Status |
|---|---|---|---|
| 1 | List SO/PO tak punya filter tanggal/termin/pihak/status_group; cari hanya `number` | BENAR: `sales/infrastructure/repository.go:209-224` (hanya status/channel/`number LIKE`), `purchasing/.../repository.go` idem. `status_group` legacy = `status` revamp (status tetap 4 nilai, by-design) → TIDAK perlu field baru. **Temuan tambahan**: `partyName` TIDAK PERNAH terisi di list (`scanOrder` tak mengisi; `service.List` tak dekorasi) → kolom "Customer" `SalesOrdersPage.tsx:179` selalu "Walk-in", ekspor kosong | Terbukti |
| 2 | "PaidForOrders sudah ada" | SEBAGIAN: hanya method repo internal `payment/infrastructure/repository.go:79` (+ dipakai `payment/application/service.go`). TIDAK ada di `payment/contracts/client.go`. Dan sales/purchasing (L4) tak boleh bergantung ke payment (L5) → endpoint gabungan ditaruh di modul payment, FE yang menyusun | Sebagian |
| 3 | komponen async-search-select sudah ada | BENAR, bahkan sudah ada wrapper `PartySearchSelect` (`party/components/PartySearchSelect.tsx`) dan `ProductSearchSelect` — tetapi **tidak dipakai di mana pun** (grep nol impor). Semua pemilih masih `SearchSelect` + list `limit:50` | Terbukti (komponen mati) |
| 4 | Stok wajib pilih produk | BENAR: `stock/presentation/handler.go:143-161` mengembalikan `[]` bila tanpa `productId` | Terbukti |
| 5a | user kehilangan email di pencarian | **TERBANTAH**: `user/infrastructure/repository.go:200,241` sudah `username/full_name/email` | Terbantah |
| 5b | party kehilangan email & alamat | BENAR: `party/.../repository.go:137` hanya `code/name/phone`; legacy `business-party.service.ts:39` + `email`, `address_text` | Terbukti |
| 5c | audit tanpa tanggal/pengguna/cabang | SEBAGIAN: legacy (`audit-log.service.ts:43-55`) juga hanya `branch`, `entity_type`, `action_key LIKE`. Gap paritas nyata = `action` LIKE (revamp `=`). Tanggal/aktor = penambahan murah (nilai tambah), bukan paritas | Sebagian |
| 5d | journals tanpa search | Bukan gap paritas: legacy `finance-journals-page.tsx` hanya filter tanggal. Opsional S | Terbantah sebagai gap |
| 5e | expenses tanpa search/tanggal/status/akun | BENAR: `finance/infrastructure/ledger.go:242` hanya `branch_id`; legacy `business-expense.service.ts:479-488` + ringkasan agregat | Terbukti |
| 5f | produk kehilangan filter tipe/tracked & kolom | BENAR di UI; data sudah ada di `productView` (`type`, `tracked`, `purchasePrice`) — hanya filter BE + kolom FE | Terbukti |
| 6 | print profile butuh `company.view`, error ditelan | BENAR: `usePrintProfile.ts:38-60`, `company/handler.go:116`. NPWP: legacy juga tidak mencetak (grep nol di `orders/`, `pos/`) → bukan gap | Terbukti / NPWP terbantah |
| 7 | UX sesi/404/dashboard/POS perm | BENAR semua (`routes/index.tsx:12`, `AppProvider.tsx:9-14`, `auth.store.ts:57-60`, `http-client.ts` `rotate()` menelan galat jaringan, `dashboard.service.ts` → `/reporting/sales-trend`, `party/routes.go:31`) | Terbukti |
| 10a | `switch-role` | **Temuan penting**: `auth/application/service.go:resolveSession` memakai `GetPermissionCodes(ctx, roleIDs)` = UNION semua role; role aktif hanya mengubah `RoleID/RoleCode`. Legacy `switchRole` mengembalikan izin role AKTIF saja (`auth.service.ts:165-174`). Jadi UI ganti-role di revamp tanpa perubahan izin = tak berguna | Terbukti → butuh keputusan pemilik (§Q1) |
| 10b | PlaceholderPage fallback | Mati: semua item menu punya halaman (dicek skrip) kecuali `/dashboard` yang dikecualikan | Terbukti (kode mati) |
| 11 | 253 panggilan FE cocok router | Dijalankan ulang (`scratchpad/match.js backend_routes.txt`): `total calls 253 ok 253`; rute BE tak dipanggil FE: `auth/refresh` (dipanggil via axios mentah di `http-client.ts`), `auth/switch-role`, `finance/costing/sync`, `finance/costing/uncosted`, `stock/allocations/suggest`, `stock/reservations/hold|release` | Terbukti |

Dependensi lintas agen (kontrak yang kuasumsikan, WAJIB disepakati saat merge):
- F2a: `GET /stock/balances` (daftar) — lihat F3a-4; `clientRef` idempoten pada `POST /sales-orders` + hold/release stok — lihat F3a-8.
- Agen penjualan: `payment_terms` `prepaid` & aturan "COD/prepaid wajib lunas sebelum selesai/kirim", "net wajib customer + HP" di `validateHeader` — F3a-9 hanya mengerjakan UI + pemakaian pesan server.
- Agen finance: peleburan Reporting/Reports (F3a-10) dan UI costing.

---

## Pondasi bersama (dikerjakan dulu; dipakai F3a-1,3,5)

### P1. `internal/shared/listquery` (BE, S)
Masalah: tiap modul menulis `List` dan `Count` dengan blok kondisi yang disalin-tempel (sales/purchasing/delivery/salesreturn/purchasereturn/goodsreceipt/payment/audit/party/user/product). Penambahan filter di dua tempat = sumber bug (List≠Count).
Rancangan (utilitas teknis murni → boleh di `shared/`):
```go
package listquery
type Where struct{ conds []string; args []any }
func New() *Where
func (w *Where) Eq(col string, v any)                 // lewati bila zero value
func (w *Where) In(col string, vals ...any)           // lewati bila kosong; maks 1000 nilai
func (w *Where) StatusCSV(col, csv string)            // "confirmed,completed" -> IN (...)
func (w *Where) DateRange(col, from, to string)       // setengah-terbuka, validasi YYYY-MM-DD, abaikan kosong
func (w *Where) Like(term string, cols ...string)     // per kata AND, antar kolom OR; escape % _ \
func (w *Where) LikeOrIn(term string, likeCols []string, inCol string, ids []int64) // (number LIKE .. OR party_id IN ..)
func (w *Where) SQL() (string, []any)                 // "branch_id = ? AND ..." untuk dipakai List & Count
```
Pemakaian: repo membuat `w := listquery.New(); w.Eq("branch_id", b); …; where, args := w.SQL()` sekali, lalu dipakai oleh `List` dan `Count` → satu sumber kebenaran. Gaya tetap `database/sql`.
Test (Go unit, `shared/listquery/listquery_test.go`): urutan placeholder, escape `%`/`_`, `DateRange` setengah-terbuka & tanggal salah diabaikan (+ error di layer service), `In` >1000 → dipotong dengan error, kosong → tanpa kondisi.

### P2. Kontrak party: `PartyClient.SearchIDs` (BE, S)
`SearchIDs(ctx, partyType, term string, limit int) ([]int64, error)` di `party/contracts/client.go`; implementasi `party/infrastructure/repository.go` memakai `listquery.Like(term, "code","name","phone","email","address_text")` + `party_type` (kosong = keduanya), `ORDER BY id`, `LIMIT limit+1` (cap 1000; bila terlampaui → kembalikan 1000 pertama + `truncated` lewat varian `SearchIDsPage` TIDAK perlu: dokumentasikan batas). Dipakai F3a-1 (order) dan F3a-5 (pembayaran).
Indeks: `parties` sudah punya `(party_type, name)` (`000031`); LIKE `%x%` tak bisa pakai indeks, tabel parties kecil (ribuan) → aman. Tidak perlu indeks baru.

### P3. Kontrak user: `UserClient.NamesByIDs` (BE, S)
Dipakai audit (nama aktor, F3a-5) dan `createdByName` di cetak (F3a-6). `user/contracts/client.go` + repo `SELECT id, full_name FROM users WHERE id IN (...)`. Best-effort seperti `PartyClient.NamesByIDs`.

### P4. Skrip kontrak FE↔router (lihat F3a-12) dan pemilih async (F3a-3).

---

## F3a-1 — Daftar order Jual/Beli: filter lengkap, partyName, ekspor ikut filter  (Ukuran: M)

### Masalah & bukti
- BE `sales/presentation/handler.go:117-120`: hanya `search,status,channel`. Repo `repository.go:209-263` `number LIKE`. Purchasing `handler.go:113-115` hanya `search,status`.
- `partyName` kosong di list (lihat §0). `sales/application/export.go:62-110` & `purchasing/application/export.go:76,122` memanggil `repo.List` → kolom Customer/Supplier kosong; ekspor tak membawa tanggal.
- FE `SalesOrdersPage.tsx:37-50,129-137`: hanya cari/status/channel. Tidak ada tanggal/termin/pihak.
- Legacy `order.service.ts:183,204-220`: `search` ke nomor+`rp.code/name/phone/email`; filter `status_group`, `payment_terms`, `date_from/date_to`, `id_related_party`; baris list membawa ringkasan finansial (`paidAmount/outstanding/financialStatus`) dan `orders-list-page.tsx` menampilkan badge bayar + label termin.

### Backend
1. Endpoint (tetap, param baru): `GET /api/v1/sales-orders` dan `GET /api/v1/purchase-orders`
   - `search` (nomor ATAU pihak: kode/nama/telepon/email/alamat),
   - `status` (CSV: `confirmed,completed`),
   - `channel` (sales),
   - `paymentTerms` (`cod|net|prepaid` — nilai `prepaid` aktif setelah agen penjualan; validasi enum longgar: nilai tak dikenal → 400 `paymentTerms`),
   - `from`, `to` (tanggal order, WIB, setengah-terbuka),
   - `partyId`.
   - `GET /sales-orders/export` & `/purchase-orders/export` menerima param yang SAMA (+`format`).
2. Struktur: `application.ListFilter{Search, Statuses []string, Channel, PaymentTerms string, PartyID int64, From, To string}`; `Service.List(ctx, branchID, f ListFilter, page, limit)`; `Service.Export(ctx, branchID, f, format)`. Handler memakai fungsi `parseListFilter(r)` yang dipakai List dan Export (menghapus duplikasi query-parse).
3. Pencarian pihak lintas modul (tanpa join) — **trade-off & rekomendasi**:
   | Opsi | Kelebihan | Kekurangan |
   |---|---|---|
   | A. `PartyClient.SearchIDs` lalu `(number LIKE ? OR party_id IN (ids))` | Tanpa perubahan skema; cocok semantik legacy (nama TERKINI, telepon, email, alamat); hasil benar bila pihak diganti nama | 1 query ekstra (tabel parties kecil, ms); batas 1000 id → istilah 1 huruf bisa terpotong |
   | B. Kolom snapshot `party_name` di order | Satu tabel; ekspor/list tanpa batch; sejalan filosofi snapshot dokumen | Backfill lintas modul di migrasi, nama lama tidak ikut rename (legacy mencari nama terkini), telepon/email tak tercakup tanpa kolom lagi, `LIKE %x%` tetap scan |
   **Rekomendasi: A**. Tampilan nama di list tetap lewat `PartyClient.NamesByIDs` (satu batch per halaman) — dua langkah itu menutup kolom kosong. B hanya dipertimbangkan bila ada >~500 rb order per cabang dan pencarian nama terukur >1 dtk (lalu tambah `party_name` + FULLTEXT).
   Implementasi: `ids, _ := parties.SearchIDs(ctx, "customer", term, 1000)`; `w.LikeOrIn(term, []string{"number"}, "party_id", ids)`. Bila `ids` kosong → hanya `number LIKE` (tanpa `IN ()` kosong).
4. Dekorasi list: `NamesByIDs(unique party ids)`; `party_id=0` → "Tunai" (konsisten `service.ListOrders`).
5. Ekspor: `exportXLSX/exportPDF` memakai `ListFilter`, tiap halaman 500 baris didekorasi `NamesByIDs`; tambah kolom `Pembayaran` (Dibayar/Sisa) TIDAK dibuat (butuh payment; di luar ekspor MVP) — dicatat sebagai opsi M di F3a-2.
6. Migrasi `0000NN_orders_filter_indexes` (modul sales & purchasing → DUA berkas, satu per modul):
   - `sales_orders`: `(branch_id, order_date)`, `(branch_id, party_id, id)`.
   - `purchase_orders`: idem.
   Alasan: filter tanggal dengan `ORDER BY id DESC` — indeks `(branch_id,id)` tetap dipakai tanpa filter; dengan rentang tanggal sempit MySQL memilih `(branch_id, order_date)` (filesort himpunan kecil). `(branch_id, party_id, id)` melayani filter pihak + urutan. `paymentTerms`/`channel`/`status` berkardinalitas rendah → tanpa indeks (disaring pada rentang yang sudah sempit).
7. Volume: tabel order bertambah linier; query tetap bounded oleh `LIMIT 20-100`; Count memakai kondisi yang sama (tanpa sort). Pencarian `LIKE '%x%'` pada nomor memindai rentang cabang (perilaku lama) — diterima.

### Frontend
- `modules/sales/services/sales.service.ts`: `SalesListParams` + `{paymentTerms, from, to, partyId}`; `cleanParams` dan `downloadExport` memakai SATU fungsi `buildListQuery(p)` (hilangkan duplikasi).
- `modules/sales/schemas/sales-list-filter.schema.ts` (Zod): `from<=to`, enum status/termin; `z.infer` untuk tipe filter; parse dari `URLSearchParams`.
- State filter di URL (`useSearchParams`) seperti legacy (dapat dibagikan/refresh) — hook baru `shared/hooks/use-url-filters.ts` (domain-agnostik: skema Zod + default → `[filters, setFilters]`, reset `page` saat filter berubah). Pola "apply-on-submit" tetap untuk kolom cari; select/tanggal menerapkan langsung.
- `SalesOrdersPage.tsx`: tambah `DateInput` Dari/Sampai, `SelectInput` Termin, pemilih Pelanggan (`PartySearchSelect partyKind="customer" allowClear`, sudah ada), placeholder cari "Nomor, nama, telepon, email…". Kolom baru: Termin (`paymentTermsLabel`), Jatuh Tempo, Bayar (badge dari F3a-2). Ekspor memakai filter yang sama (sudah lewat `buildListQuery`).
- `PurchaseOrdersPage.tsx` idem (pemilih `PartySearchSelect partyKind="supplier"`).
- Reuse: `FilterBar`, `FormField`, `DateInput`, `Pagination`, `useAsyncData`; label `paymentTermsLabel` perlu varian `prepaid` ("Prabayar") — tambah di `sales/types.ts` & `purchasing/types.ts`.

### Test
- Go integrasi (`internal/integration/orders_list_filters_test.go`, pola `freshDB/newShop/sellFixture`): (a) cari nama customer menemukan SO; (b) cari telepon/email; (c) rentang tanggal inklusif hari terakhir (SO jam 00:00 tanggal `to` ikut); (d) `status=confirmed,completed`; (e) `paymentTerms=net`; (f) `partyId`; (g) `partyName` terisi & walk-in "Tunai"; (h) ekspor xlsx berisi kolom Customer terisi + hanya baris terfilter (buka dengan excelize); (i) Count = jumlah baris tanpa limit.
- Go unit: `parseListFilter` (tanggal salah → 400 `from`).
- Vitest: `sales-list-filter.schema.test.ts`; `SalesOrdersPage.test.tsx` (memanggil service dengan param benar saat filter diterapkan, URL tersinkron); `buildListQuery.test.ts`.
- Dependensi: P1, P2. Tidak bergantung agen lain (kecuali nilai `prepaid`).

---

## F3a-2 — Detail order terpadu: Rincian Tagihan, riwayat, aksi dari dalam order  (Ukuran: L)

### Masalah & bukti
- `SalesOrderDetailPage.tsx:179-262` dan `PurchaseOrderDetailPage.tsx`: hanya info/item/total/aksi status. `sales/types.ts:25-52` tanpa paid/outstanding. `PurchaseOrderDetailPage` mengakui "orderView tidak membawa receivedQty".
- `ReceiveModal`/`CreateDeliveryModal` hanya dipanggil dari halaman daftar, tanpa preselect (`ReceiveModal.tsx:46-50` props hanya `open/onClose/onCreated`; opsi PO `limit:50`).
- Legacy `order-detail-page.tsx`: kartu Status Operasional / Status Finansial / Total Tagihan + "Sisa dibayar" (`:230-255`), Rincian Tagihan, riwayat penerimaan (PO) & pengiriman (SO), aksi Terbitkan SJ / Terima Barang / Retur / Bayar, cetak Tagihan vs Nota (`:420-424`), `outstanding` memodulasi tombol.
- Tagihan revamp = `grandTotal - Σ alokasi aktif` (`payment/application/service.go:GetPartyBalance`); retur dikelola terpisah sebagai kredit (settle-credit). Jadi "sisa tagihan" revamp BUKAN dikoreksi retur — beda dari legacy `buildFinancialSummary`; sengaja (ADR returns). Detail menampilkan kredit retur sebagai blok terpisah (di bawah).

### Backend (semua read-only; tanpa tabel baru)
1. **payment** (L5, pemilik data uang): 
   - `GET /api/v1/payments/order-billing?orderType=sales|purchase&ids=1,2,3` (perm `payment.view`, maks 100 id) →
     `[{orderType, orderId, grandTotal, paid, outstanding, paymentStatus: "unpaid|partial|paid", paymentTerms, dueDate, overdue bool}]`.
     Implementasi: `s.repo.PaidForOrders` (sudah satu query GROUP BY) + data order lewat kontrak: sales sudah punya `SummariesByIDs`; **tambah `purchasing.PurchaseOrderClient.SummariesByIDs`** (batch, ditiru dari sales `aggregates.go`) + tambah `PaymentTerms` ke `OrderSummary` kedua modul (kolom sudah ada di tabel). `overdue = outstanding>0 && terms=="net" && dueDate < hari ini WIB`. `paymentStatus` turunan (paid bila `outstanding<=0`).
   - Daftar pembayaran per order: perluas `GET /api/v1/payments` dengan `orderType`+`orderId` (kondisi `id IN (SELECT payment_id FROM payment_allocations WHERE order_type=? AND order_id=?)` — tabel milik payment sendiri, indeks `ix_payment_allocations_order` sudah ada). List sekarang tidak mengisi `allocations`; tambahkan `allocatedToOrder` (jumlah alokasi ke order itu) pada baris saat filter order aktif.
   - Kredit retur terkait order: FE memakai `GET /sales-returns?salesOrderId=` / `/purchase-returns?purchaseOrderId=` (sudah ada) — tanpa BE baru.
2. **goodsreceipt**: `GET /api/v1/goods-receipts/progress?purchaseOrderId=` (perm `goodsreceipt.view`) → `[{productId, variantId, receivedQtyBase}]` via satu `SELECT … GROUP BY product_id, variant_id` pada `goods_receipt_items` JOIN `goods_receipts` (satu modul). Menggantikan loop `ReceivedQty` per posisi.
3. **delivery**: `GET /api/v1/deliveries/progress?salesOrderId=` → `[{productId, variantId, deliveredQtyBase, pendingQtyBase}]` (SJ draf = pending, confirmed = delivered; GROUP BY). Dipakai tombol "Terbitkan SJ" (sisa > 0).
4. Daftar `GET /goods-receipts`/`/deliveries` sudah memfilter `purchaseOrderId`/`salesOrderId` → dipakai apa adanya untuk "Riwayat Penerimaan/Pengiriman". Tambah filter `salesReturnId` pada deliveries (menghapus `findReplacementDelivery` yang memuat 50 lalu menyaring klien, `sales-return.service.ts:125-142`).
5. `createdByName` pada detail SO/PO: `orderView` + `createdById` (kolom `created_by` ada) + nama via P3. Dipakai kolom "Dibuat oleh" & "Staff" di cetak (F3a-6).
6. Migrasi: tidak ada tabel. Indeks sudah memadai.

### Frontend
- Hook `modules/payment/hooks/useOrderBilling.ts` (+ `paymentService.orderBilling(type, ids)`): dipakai detail & list. Gagal 403 (tanpa `payment.view`) → `null` (UI menyembunyikan bagian bayar, tidak error).
- Komponen bersama domain order di `modules/payment/components/OrderBillingCard.tsx` (kartu Status Bayar/Total/Dibayar/Sisa/Jatuh tempo efektif + badge Lunas/Bayar Sebagian/Jatuh Tempo/Belum Bayar) dipakai SO & PO. Pemakaian lintas modul FE diizinkan (FE modul = pengelompokan UX).
- `SalesOrderDetailPage.tsx` & `PurchaseOrderDetailPage.tsx` ditambah bagian (urutan seperti legacy):
  1. Kartu ringkas: Status, Status Finansial, Total + Sisa.
  2. Rincian Tagihan: subtotal/diskon/DPP/pajak/total + Dibayar/Sisa/Jatuh tempo.
  3. Item (PO: kolom "Diterima x/y" dari `progress`; SO: "Terkirim x/y").
  4. Riwayat Penerimaan (PO) / Pengiriman (SO) — tabel ringkas + link ke detail dokumen.
  5. Pembayaran: daftar pembayaran teralokasi + tombol **Bayar** (membuka `PaymentFormModal` dengan `initialPartyId`, `initialAllocation={orderType,orderId,amount=sisa}`); tombol **Setujui Kredit** (sudah ada).
  6. Retur terkait (daftar) + tombol **Retur / Tukar** / **Retur ke Supplier** (aturan legacy: SO terkirim/selesai; PO diterima) → membuka modal retur dengan `initialSourceId`.
- Aksi terpadu (aturan tampil mengikuti legacy BR-14/16, `order-detail-page.tsx:230-290`):
  - **Terima Barang** (PO confirmed, `goodsreceipt.create`; prepaid belum lunas → disembunyikan + Notice) → `ReceiveModal initialPoId`.
  - **Terbitkan Surat Jalan** (SO confirmed non-POS, `delivery.create`, sisa kirim >0; prepaid belum lunas → disembunyikan) → `CreateDeliveryModal initialSoId`.
  - **Cetak Tagihan/Nota**: judul dari status bayar (F3a-6).
- `ReceiveModal`/`CreateDeliveryModal`: prop opsional `initialPoId`/`initialSoId` + tidak lagi memuat opsi `limit:50` (ganti `AsyncSearchSelect`, lihat F3a-3); saat `initial*` ada, pemilih terkunci & baris langsung diisi dari `progress` (qty default = sisa, bukan qty PO penuh — menutup komentar "sisa parsial tak bisa dihitung klien").
- Daftar order: badge bayar per baris dari `order-billing` untuk id halaman (satu panggilan ekstra per halaman); filter "Status bayar" (Belum/Sebagian/Lunas/Jatuh tempo) TIDAK dibuat di BE (butuh join lintas modul) — legacy juga hanya menampilkan badge (`orders-list-page.tsx:59-65`), jadi sesuai paritas.
- Tipe: `modules/payment/types.ts` `OrderBilling`, `modules/goodsreceipt/types.ts` `ReceiptProgress`, `modules/delivery/types.ts` `DeliveryProgress`; Zod untuk payload `PaymentFormModal` yang sudah ada diperluas (`initialAllocation`).

### Test
- Go integrasi (`order_billing_test.go`): order dibayar sebagian → `partial`, lunas → `paid`, pembayaran dibatalkan tak dihitung, net lewat jatuh tempo → `overdue`; cabang lain → tidak muncul; `ids>100` → 400. `progress` goods receipt: dua penerimaan parsial menjumlah; SJ draf vs confirmed pada `delivery progress`. `payments?orderType&orderId` hanya pembayaran terkait.
- Vitest: `OrderBillingCard.test.tsx` (4 status), `SalesOrderDetailPage.test.tsx` (tombol Bayar/Terbitkan SJ/Retur muncul sesuai status+termin+izin; 403 billing menyembunyikan blok), `ReceiveModal.test.tsx` (preselect mengunci pemilih & mengisi sisa).
- Volume: semua query berbasis daftar id ≤100 / satu order; tanpa N+1.
- Dependensi: P3, F3a-3 (pemilih), agen penjualan (`prepaid`, aturan lunas), F2a (tidak). Konflik file: `payment/application/service.go` juga disentuh agen atomisitas → ubah via file baru `payment/application/billing.go`.

---

## F3a-3 — Pemilih async-search server-side (hapus daftar 50 baris)  (Ukuran: M)

### Masalah & bukti (semua `limit: 50` lalu filter klien)
`purchasing/services/purchase-order.service.ts:89-93` (supplier form PO), `payment/services/payment.service.ts:85-96` (customer+supplier), `goodsreceipt/services/goodsreceipt.service.ts:44-49` (PO confirmed), `delivery/services/delivery.service.ts:76-83` (SO confirmed), `salesreturn/services/sales-return.service.ts:126-155` & `purchasereturn/services/purchase-return.service.ts:84-100` (dua status × 50). Daftar order `ORDER BY id DESC` → dokumen lama tak terjangkau.
Pemilih sync `SearchSelect` juga dipakai di `PartyBalanceSection`, `SettleCreditModal`, `PaymentFormModal` (menerima prop `parties[]`).
Komponen siap: `shared/components/ui/async-search-select.tsx` (debounce 250 ms, abort signal, `hasMore`), `PartySearchSelect`, `ProductSearchSelect` (keduanya belum dipakai).

### Backend
- Endpoint daftar sudah mendukung `search`+`status` → cukup tambah **`status` CSV** (P1 `StatusCSV`) pada sales/purchasing list (sudah di F3a-1) agar retur memakai SATU panggilan `status=confirmed,completed`. Cari nomor + nama pihak otomatis (F3a-1).
- Tidak ada endpoint baru. (Opsional: `GET /party-options` ringkas — tidak perlu; `GET /customers`/`/suppliers` dengan `search` + `limit=20` sudah cukup, dan `search` kini mencakup email/alamat lewat F3a-5.)

### Frontend
- Komponen baru domain-order (di modul pemilik, bukan `shared/`):
  - `modules/purchasing/components/PurchaseOrderSearchSelect.tsx` (`status` prop CSV, `loadOptions(q)` → `purchaseOrderService.list({search:q,status,limit:20})`; label `number — partyName`; `description`: tanggal + total).
  - `modules/sales/components/SalesOrderSearchSelect.tsx` idem.
  - Pemilih pihak campuran untuk Pembayaran: `modules/party/components/PartySearchSelect` diperluas `partyKind="any"` (dua panggilan paralel customer+supplier, gabung 10+10, label membawa jenis).
- Ganti pemakaian: `PurchaseOrderFormPage` (supplier), `SalesOrderFormPage` (customer: hapus pola ganda TextInput+SearchSelect baris ±495-530 → `PartySearchSelect`; produk → `ProductSearchSelect`), `PaymentFormModal`/`SettleCreditModal`/`PartyBalanceSection` (prop `parties[]` dihapus; terima `partyId` + `PartySearchSelect any`), `ReceiveModal`, `CreateDeliveryModal`, kedua modal retur. Hapus `purchasingLookupService.supplierOptions`, `paymentLookupService.partyOptions`, `goodsreceiptService.purchaseOrderOptions`, `deliveryService.salesOrderOptions`, `*ReturnLookupService.sourceOrders`.
- `PaymentFormModal`: baris alokasi `orderId` (teks bebas "orderId" angka!) diganti pemilih dokumen (SO/PO menurut jenis pihak, `status=confirmed,completed`, filter `partyId` ditambah ke list order — param `partyId` sudah ada di F3a-1) + tampilan sisa dari `order-billing`.
- Aturan lint/arch: tak ada `limit: 50` untuk pemilih; tambah ke `contract-check` (F3a-12) aturan "no-picker-prefetch": peringatan bila service memanggil `apiPage` dengan `limit: ≥50` di berkas `*lookup*`.

### Test
- Vitest: `PurchaseOrderSearchSelect.test.tsx` (ketik → request dengan `search`, abort request lama, hasMore), regresi: PO nomor lama (id kecil) tetap bisa dicari; `PaymentFormModal.test.tsx` (pilih pihak supplier → pilihan dokumen bertipe purchase).
- Go: sudah tercakup F3a-1 (CSV status).
- Volume: tiap ketikan = satu query berindeks `(branch_id,id)`+LIKE, `limit 20`.

---

## F3a-4 — Daftar saldo stok semua barang (UI)  (Ukuran: M; BE oleh F2a)

### Masalah & bukti
`StockPage.tsx:74-92,281` wajib pilih produk ("Backend mewajibkan productId"). Legacy `stock-list-page.tsx:64,135,168-195`: cari kode/nama + scan, "Stok Kritis Saja", kolom Tersedia/Fisik/Dipesan/Status (Kritis/Aman), per produk (agregat semua lokasi), `gudang-page.tsx:110` per lokasi, opsi `include_damaged`.
Kolom "Dipesan" = `reserved` (sudah ada di `StockBalance.reserved`, FE menamai "Ditahan").

### Kontrak yang diasumsikan dari F2a (harus sama saat merge)
`GET /api/v1/stock/balances?view=product|position&search=&criticalOnly=true|false&locationId=&includeDamaged=true|false&productIds=1,2,3&page=&limit=` (perm `stock.view`), paginasi. Baris `view=product`: `{productId, productCode, productName, baseUom, minStock, onHand, reserved, available, critical}`; `view=position`: sama + `variantId, locationId`. `productIds` (maks 100) dipakai F3a-5 untuk kolom stok di daftar produk. Bila F2a menamai berbeda, FE hanya menyesuaikan `stock.service.ts` + `types.ts`.
Catatan reuse: `ProductAvailability` (`stock/infrastructure/repository.go:259`) sudah ada sebagai agregat.

### Frontend
- `StockPage.tsx` (652 baris) dipecah: bagian Saldo → komponen `modules/stock/components/StockBalanceSection.tsx` (halaman jadi komposisi Saldo/Riwayat/Lokasi). Saldo default = daftar semua barang (view produk, 20/hal), pencarian apply-on-submit, `scan` barcode (reuse `PosScannerModal`? — komponen scan ada di `pos/components`; pindahkan `PosScannerModal` ke `shared/components/ui/barcode-scanner-modal.tsx` bila domain-agnostik (isinya kamera + decode); bila tidak, dua pemakai mengimpor dari pos — keputusan implementor), `Switch` "Stok kritis saja", `SearchSelect` lokasi (opsional → `view=position`), checkbox "Termasuk rusak".
- Klik baris → `/stock/items/:productId` (sudah ada `StockItemPage`).
- Kolom: Produk (kode+nama), Tersedia (+ badge Kritis bila `critical`), Fisik, Dipesan, Satuan dasar, Status.
- Tipe `StockBalanceRow`/`StockBalanceQuery` + Zod `stockBalanceFilterSchema`; filter di URL (`use-url-filters`, F3a-1).

### Test
- Vitest: `StockBalanceSection.test.tsx` (default memanggil tanpa `productId`; toggle kritis mengirim `criticalOnly=true`; pilih lokasi beralih `view=position`; baris kritis berbadge).
- Go: milik F2a (integrasi `stock_balances_list_test.go`).
- Dependensi: F2a (blokir). Boleh dikerjakan dengan mock `msw`-style selama F2a belum merge.

---

## F3a-5 — Filter/pencarian daftar lain  (Ukuran: M)

Tiap butir: BE param baru (selalu via `listquery`) + FE filter + indeks.

| Daftar | Gap terbukti | BE | FE | Indeks (migrasi modul pemilik) |
|---|---|---|---|---|
| Retur jual `GET /sales-returns` | tanpa search & tanggal (`salesreturn/.../repository.go:274-333`) | `search` (nomor + nomor SO via sub-IN pada tabel sendiri? **tidak** — nomor SO ada di modul sales: pakai `sales.GetByNumber`/`SummariesByIDs`… cukup `search` pada nomor retur + `salesOrderId`), `from`,`to` (`return_date`), `status` CSV | `SalesReturnsPage` tambah cari+tanggal | `sales_returns (branch_id, return_date)` |
| Retur beli | idem | idem (`purchase_returns`) | `PurchaseReturnsPage` | `purchase_returns (branch_id, return_date)` |
| Produk `GET /products` | tanpa `type`,`tracked` | `type=barang|jasa`, `tracked=true|false` (`product/.../repository.go:312-370` + CountProducts via P1) | `ProductsPage.tsx:170-190` + kolom Harga Beli (gate `products.update`?—legacy menampilkan; ikuti izin yang sama dengan form), Tipe, **Stok** (dari F3a-4 `productIds`, best-effort bila tanpa `stock.view`) | `products (type, name)`? tidak perlu (tabel kecil) |
| Pihak `GET /customers|suppliers` | cari tanpa `email`,`address_text` (`party/.../repository.go:137,153`) | tambah dua kolom ke `LIKE` (Count ikut — saat ini Count memakai kondisi terpisah → pakai P1) | placeholder "nama, kode, telepon, email, alamat" | tak perlu |
| User | **Terbantah** (email sudah) | — | hanya perbaiki placeholder `UsersPage.tsx:341` bila tak menyebut email | — |
| Pembayaran `GET /payments` | hanya `partyId,status`; `search`/`method` disaring klien pada halaman aktif (`payment.service.ts:2-4`, `PaymentsPage.tsx:96-109`) → hasil salah | `search` (nomor+catatan OR `party_id IN SearchIDs`), `method`, `direction`, `from`,`to` (`paid_at`), `orderType`+`orderId` (F3a-2), `partyId`, `status`; dekorasi `partyName` via `NamesByIDs` (saat ini list TIDAK mengisi nama; halaman mengambilnya dari lookup 50 pihak) | `PaymentsPage` kirim semua filter ke server, hapus filter klien dan `paymentLookupService` | `payments (branch_id, paid_at)`, `payments (branch_id, party_id, id)` |
| Audit `GET /audit-logs` | action `=` (legacy LIKE); tanpa tanggal/aktor | `action` LIKE `%x%`, `entity`, `from`,`to`, `actorId`; Count via P1; dekorasi nama aktor (P3) pada `recordView` (`actorName`) | `AuditPage.tsx` tambah tanggal + pemilih pengguna (`UserSearch`—memakai `GET /users?search=` bila `users.view`, bila tidak, input angka disembunyikan) + kolom Aktor nama | `audit_logs (actor_id, created_at)` (sudah ada `(branch_id, created_at)`) |
| Biaya `GET /finance/expenses` | hanya branch (`ledger.go:242`) | `search` (nomor+catatan), `from`,`to` (`expense_date`), `status`, `expenseAccountId`; respons paginasi + `summary{postedCount, postedAmount}` seperti legacy (`business-expense.service.ts:56-65`) — agregat seluruh hasil filter | `ExpensesPage` (filter + SummaryCard) | `business_expenses (branch_id, expense_date)` |
| Jurnal | bukan gap paritas (§0) | opsional S: `search` nomor+memo (`finance_journal_entries`) | opsional | cek `(branch_id, entry_date)` ada; bila belum, tambah |

Catatan teknis: `listquery.DateRange` dipakai untuk semua; `CountEntries` jurnal memakai `entry_date <= to` (tanpa batas hari) — rapikan ke setengah-terbuka bersamaan.

Test: Go integrasi per daftar (satu berkas `list_filters_test.go`, tabel-driven: seed 3 baris beda tanggal/metode/pihak, verifikasi tiap filter + Count konsisten + `page` 2); khusus pembayaran: cari nama pihak, `method=transfer`, tanggal, `orderId`. Vitest: skema filter per halaman + service param.
Volume: indeks di atas; pencarian `LIKE` pada tabel kecil/terpaginasi.
Dependensi: P1,P2,P3; F3a-2 (filter order di payments).

---

## F3a-6 — Cetak: profil cetak untuk semua peran, judul/isi nota, verifikasi layout  (Ukuran: M)

### Masalah & bukti
- `usePrintProfile.ts:38-60` memanggil `GET /company/settings` (`company.view`, `company/handler.go:116`), `.catch` menelan → kasir/staf tanpa `company.view` mencetak tanpa kop/rekening/kalibrasi kertas.
- `SalesInvoicePrintPage.tsx:193-221`: tak ada "Sudah Dibayar"/"Sisa Tagihan", opsi sembunyikan diskon, catatan cetak (tidak disimpan), judul selalu "Faktur". Legacy `sales-document-print-page.tsx:104,114,220-223`: judul dari status bayar (Lunas/Sebagian/Tagihan), baris "Sudah Dibayar" bila >0, "Sisa Tagihan", `Tgl. Jtp` "-" bila lunas/tanpa tempo, "Staff" = pembuat order, USER = pencetak, opsi `hide discount` (harga penuh, Disc "-", jumlah naik ke bruto), `Notes` hanya-cetak, kolom urut No/Qty/Satuan/Produk/Harga Satuan/Disc/Harga Jual.
- NPWP: legacy tidak mencetak → TIDAK ada gap; opsional (tidak dikerjakan) cetak `taxId` pada faktur pajak bila `taxType≠none`.

### Backend
1. `GET /api/v1/company/print-profile` — hanya `Authenticate` + `RequireBranch` (tanpa perm; pola `authcontracts.Authenticate` seperti `/auth/me`), mengembalikan allow-list: `{name, legalName, address, city, phone, documentProfile}` (`documentProfile` = nilai kunci `document_profile` dari `company_settings`, string JSON). Kunci lain di `company_settings` TIDAK dibocorkan. Konstanta `PrintSettingKeys = []string{"document_profile"}` di `company/contracts`.
2. `orderView` sales: `createdById`, `createdByName` (P3). `paymentView`/`order-billing` untuk nota (F3a-2) — nota memakai `GET /payments/order-billing?orderType=sales&ids=<id>`; bila 403 → nota tanpa baris bayar (judul "Nota"), logika degrade di FE.

### Frontend
- `company.service.ts`: `getPrintProfile()`; `usePrintProfile` memakainya (tetap `parseDocumentProfile` toleran) dan **tidak diam**: bila gagal, `printWarning` ditampilkan di toolbar ("Kop/rekening tidak termuat: <pesan>") — cetak tetap jalan.
- `SalesInvoicePrintPage`: judul `salesDocumentTitle(outstanding, paid)` ("Nota" lunas / "Nota (Sebagian)" / "Tagihan"); baris "Sudah Dibayar"/"Sisa Tagihan"; Tgl. Jtp "-" bila lunas; urutan kolom disamakan legacy (No, Qty, Satuan, Produk, Harga Satuan, Disc [bila ada diskon baris & tak disembunyikan], Harga Jual) — kolom "Kode" tidak dibuang sepenuhnya: ditaruh di bawah nama produk kecil (legacy tak punya kolom kode); toolbar: checkbox "Sembunyikan diskon" (param URL `hideDiscount`, `readHideDiscount` di `print/paper.ts`), textarea "Notes (hanya tercetak)"; "Staff" = `createdByName`, "USER" = pencetak. Logika murni diekstrak ke `print/lib/sales-document.ts` (`documentTitle`, `displayTotals(hideDiscount)`), diuji.
- Judul tombol di detail SO: "Cetak Tagihan" bila sisa>0 else "Cetak Nota" (F3a-2).
- Kwitansi (`PaymentReceiptPrintPage.tsx`): legacy menampilkan Total Tagihan / Total Terbayar Setelah Ini / Sisa Setelah Pembayaran + No. HP/Alamat pihak + judul "Bukti Pembayaran Customer/Supplier"; revamp punya terbilang + tanda tangan (legacy: BR-30 "tanpa tanda tangan, disengaja"). Tambah blok ringkasan (dari `order-billing` + urutan alokasi) tanpa menghapus terbilang; tanda tangan dipertahankan → **Q4** (konfirmasi pemilik).

### Verifikasi layout per dokumen (hasil bandingan kode; "FISIK" = butuh uji kertas pemilik, tahap L3)
| Dokumen | Legacy vs revamp | Status |
|---|---|---|
| Surat Jalan (`DeliveryNotePrintPage`) | kolom No/Qty/Satuan/Produk/Keterangan identik; field Nama/Penerima/No HP/Alamat/Tgl Kirim/No Nota/Tgl Nota/Staff/Kendaraan | Sama di kode; FISIK: ukuran font/margin kontinu |
| Faktur/Nota | berbeda (di atas) → disamakan | FISIK setelah perubahan |
| Kwitansi | berbeda (di atas) | Q4 + FISIK |
| Struk POS thermal | lebar 80/58 mm sama; baris Tunai diterima/Kembalian — lihat F3a-8 | FISIK printer termal asli |
| Label QR | tak ada padanan legacy detail; ukuran label | FISIK |
Kalibrasi kertas (`PaperCalibrationPage`) tetap acuan; default `241.3×139.7 mm` ditandai "hasil tes lama" → wajib uji ulang di mesin produksi.

### Test
- Go integrasi: `print-profile` dapat diakses peran tanpa `company.view`, tidak memuat kunci selain allow-list, tak terautentikasi → 401.
- Vitest: `sales-document.test.ts` (judul 3 kondisi, hideDiscount menaikkan jumlah ke bruto & sisa tetap riil, "-" jatuh tempo), `usePrintProfile.test.tsx` (gagal → warning, bukan diam), snapshot struktur `SalesInvoicePrintPage`.
- Dependensi: F3a-2 (order-billing), P3.

---

## F3a-7 — UX: 404, sesi, izin dashboard/POS  (Ukuran: M)

Butir & rancangan:
1. **404**: `NotFoundPage` (`modules/auth/pages/NotFoundPage.tsx`) — rute `*` di `routes/index.tsx:12` diganti. Bila belum login → `Navigate /login?next=…`; bila login → halaman 404 di dalam `AppLayout` (taruh `{path:"*"}` sebagai anak rute terlindungi `protectedRoutes`) dengan tombol Dashboard. Test Vitest: URL tak dikenal (login & tamu).
2. **Sesi habis tanpa kehilangan state**:
   - `AppProvider.tsx:9-14`: `window.location.href="/login"` → `/login?next=${encodeURIComponent(pathname+search)}` dan pakai `router.navigate` (ekspor `router` dari `routes/index.tsx`) bukan reload penuh bila memungkinkan.
   - Draf yang dilindungi: POS (keranjang+pelanggan+`checkoutRef`, F3a-8) dan form SO/PO disimpan ke `sessionStorage` (`shared/hooks/use-draft.ts`, kunci `draft:<userId>:<route>`, try/catch seperti panduan browser-storage, dibersihkan saat submit sukses/logout); dipulihkan setelah login ulang pada rute `next` (pemulihan memunculkan banner "Draf dipulihkan").
   - `http-client.ts` `rotate()`: bedakan **kedaluwarsa** (respons 400/401 dari `/auth/refresh`) vs **tak terjangkau** (tanpa respons / 5xx / timeout). Hanya kedaluwarsa → `tokenStore.clear()` + `onAuthExpired`. Tak terjangkau → tolak error asli (status 0 "Terjadi kesalahan jaringan"), token tetap.
   - `auth.store.ts:57-60` `loadMe()`: hapus token hanya bila `status===401|403`; galat jaringan/5xx → `ready:true`, `bootError:"network"`, tampilkan layar "Tidak dapat terhubung — Coba lagi" (`RequireAuth` membaca `bootError`) tanpa menendang ke login.
   - Opsional (M tambahan, tidak wajib): dialog "Sesi berakhir — masuk lagi" in-place (login tanpa meninggalkan halaman, lalu ulang request yang gagal) — dicatat sebagai peningkatan; draf tersimpan sudah menutup kebutuhan utama.
3. **Dashboard memanggil `/reporting/sales-trend` (butuh `reporting.view`)**: tambah `GET /api/v1/dashboard/trend?from&to` (perm `dashboard.view`) di modul dashboard (pakai `FinanceClient.DailyTrend` yang sama dengan reporting; satu pembacaan — PERFORMANCE_GUIDE §2) dan ganti `dashboard.service.ts salesTrend`. `reporting/sales-trend` tetap untuk konsumen lain (F3a-10).
4. **POS memanggil `GET /member-types` (`member_types.manage`)**: tambah permission `member_types.view` (`party/contracts` katalog + `party/presentation/routes.go:31` GET → `member_types.view`; POST/PUT/archive tetap `manage`). Migrasi `0000NN_perm_member_types_view` (modul user/party — satu berkas di modul `user` karena memiliki tabel `permissions`): sisip permission + beri ke semua role yang memegang `member_types.manage`, `customers.view` atau `pricing.quote` (agar kasir lama tak kehilangan). Seed (`cmd/seed/main.go`) sudah menyinkronkan katalog. `PartyFormModal` (pemilih tipe member) ikut memakai `view`. FE: `RequirePerm` menu "Tipe Member" tetap `manage`.
5. **POS tanpa pesan izin**: `PosPage.tsx:103,127` menelan error (`catch(() => set([]))`). Tambah `PosPreflight` (hook `usePosReadiness`): cek izin `stock.view`, `products.view`, `customers.view`, `sales.create`, `sales.update`(konfirmasi), `delivery.create`, `payment.create`, `pricing.quote` dari `permissions` store; yang kurang → Notice jelas per fitur ("Tanpa izin Pelanggan: transaksi hanya walk-in") dan checkout diblokir hanya bila izin inti (`sales.create/update`, `delivery.create`) kurang. Galat jaringan pada katalog/kategori → toast dengan pesan server (bukan diam).
Test: Vitest `http-client.refresh.test.ts` (jaringan putus ≠ logout; 401 refresh = logout), `auth.store.test.ts` (loadMe jaringan putus mempertahankan token), `NotFoundPage.test.tsx`, `usePosReadiness.test.ts`; Go integrasi `dashboard_trend_test.go` (peran hanya `dashboard.view` sukses, tanpa → 403) dan `member_types_view_test.go` (GET dengan `view` saja 200, POST 403).
Dependensi: F3a-8 (draf POS).

---

## F3a-8 — POS frontend: idempoten, reservasi stok, UX checkout  (Ukuran: L; BE oleh F2a)

### Masalah & bukti
`PosPage.tsx:397-545`: lima panggilan berurutan (`createOrder → confirm → createDelivery → confirmDelivery → createPayment`, `pos.service.ts:75-104`); gagal di tengah → keranjang tetap terisi, "Bayar" lagi membuat SO BARU (duplikat). Tidak ada hold stok (legacy `pos-page.tsx:339-372`: hold 600 dtk, debounce 350 ms, satu kunci reservasi keranjang, dilepas bila kosong, tak menahan saat checkout/sukses); legacy retry aman via `pendingCheckoutOrderRef` (`:536-570`). Tidak ada alamat kirim, diskon per baris (`use-pos-cart.ts:150-169` legacy), uang tunai diterima & kembalian hanya di klien (`completed.tendered/change`) tidak disimpan.

### Kontrak diasumsikan dari F2a
(a) `POST /sales-orders` dan langkah berikutnya menerima `clientRef` (UUID per transaksi POS) → pengulangan dengan `clientRef` sama mengembalikan dokumen yang sama (200, bukan 409) di cabang+pengguna sama; (b) bila F2a menyediakan satu endpoint orkestrasi `POST /api/v1/sales-orders/pos-checkout` (create+confirm+SJ+bayar atomik+idempoten), FE memakainya satu kali; jika tidak, FE menjalankan lima langkah dengan `clientRef` per langkah (`<ref>:order|confirm|delivery|payment`); (c) hold/release dengan kunci keranjang tunggal `items[]` dan TTL 600 dtk, izin cukup `sales.create` (saat ini `stock.manage` — kasir biasanya tak punya, `stock/presentation/routes.go:28-29`); (d) `amountTendered`, `changeAmount` disimpan di pembayaran tunai; (e) `shipToAddressId` & diskon per baris sudah ada di `orderPayload` (`shipToAddressId`, `discountPct/Nominal`).
Jika F2a hanya menyediakan (a), seluruh rancangan FE di bawah tetap berlaku (sequential idempoten).

### Frontend
- `modules/pos/lib/checkout-session.ts` (murni, diuji): mesin keadaan `CheckoutSession{ref, signature, steps:{order?:{id,number,total}, confirmed?, delivery?:{id}, deliveryConfirmed?, payment?:{id}}}`; `signature` = hash keranjang+pelanggan+termin+lokasi. Fungsi `nextStep(session)` & `canResume(session, currentSignature)`.
- Siklus: `ref = crypto.randomUUID()` dibuat saat keranjang pertama kali tak kosong (disimpan `sessionStorage` bersama keranjang, F3a-7 `use-draft`); **tidak diganti** saat checkout gagal; diganti hanya setelah sukses/"Kosongkan keranjang". Tekan "Bayar" lagi → lanjut dari langkah gagal (atau panggilan ulang idempoten) — tidak pernah membuat SO baru. Bila `signature` berubah setelah order terbentuk → Notice: "Order POS sebelumnya (nomor X) belum selesai — selesaikan/batalkan dulu" (meniru pesan legacy), tombol "Lanjutkan order X" / "Batalkan order X".
- `pos.service.ts`: `checkout(payload, signal)` (satu panggilan bila F2a menyediakan) dengan fallback `runSteps`; tiap langkah menyertakan `clientRef`; galat tetap memunculkan `message` asli server.
- Reservasi: `hooks/useStockHold.ts` — efek debounce 350 ms atas `cart` (hanya baris `tracked`), `PUT`-semantik hold (F2a); lepas saat keranjang kosong/unmount/sukses; tidak menahan saat `processing` atau `completed` (hindari "reservasi hantu" seperti catatan legacy). Gagal hold (stok kurang) → tandai baris "stok tidak cukup (tersedia N)" tanpa memblokir input; checkout memblokir baris itu.
- Diskon per baris: kolom diskon (% atau Rp) di keranjang, dikirim `discountPct/discountNominal`; estimasi memakai `domain` sisi FE yang sama dengan `sales/lib/estimate.ts`; guard harga minimum diperlakukan server (F3a-9) → tampil pesan server.
- Alamat kirim: toggle "Kirim ke alamat" (pelanggan non-walk-in) memakai `GET /customers/:id/addresses` (sudah ada) → `shipToAddressId`.
- Uang diterima & kembalian: kirim `amountTendered` pada pembayaran tunai; struk memakai nilai dari respons (bukan state klien) → cetak ulang konsisten (`PosThermalReceiptPage` membaca dari server).
- Pemecahan file: `PosPage.tsx` (1092 baris) → `components/PosCart.tsx`, `PosCheckoutDialog.tsx`, `PosProductPanel.tsx`, hook `usePosCart` (state+matematika), `useCheckout`. Perilaku tetap; pemecahan agar diuji.
- Skema Zod `pos-checkout.schema.ts` diperluas: `tendered >= total` dicek terhadap total estimasi HANYA sebagai peringatan; keputusan server menang.

### Test
- Vitest: `checkout-session.test.ts` (gagal di langkah 3 → ulang melanjutkan langkah 3; signature berubah → blok; sukses → ref baru), `useStockHold.test.tsx` (debounce, lepas saat kosong, tak hold saat processing), `PosPage.checkout.test.tsx` dengan `service` palsu: "Bayar" dua kali setelah kegagalan memanggil `createOrder` dengan `clientRef` yang SAMA, tidak ada order kedua.
- Go: milik F2a (idempoten, hold). Integrasi tambahan di sisi saya: tidak ada.
Dependensi: F2a (blokir untuk `clientRef`/hold), F3a-7 (draf), F3a-9 (pesan harga minimum).

---

## F3a-9 — Harga jual manual per baris + klem harga minimum + termin (UI)  (Ukuran: L)

### Masalah & bukti
BE `sales/handler.go:67-74` `linePayload` tanpa `unitPrice`; `service.go:216-218` harga = member `Applied` atau `p.SellingPrice`; komentar "klien TIDAK mengirim unitPrice" (`types.ts:4`). Legacy: harga manual per baris (`order-form-page.tsx:686-691`), guard minimum (`order.service.ts:430-436,645-651`): sales + pricingSource≠`member_rule` + `minSellingPrice` ada + harga bersih < minimum → 400 pesan "Harga jual untuk produk 'X' (Rp a) tidak boleh di bawah harga jual minimum (Rp b)"; member menimpa harga penuh (`order-pricing.service.ts:resolveLinePricing`); `priceEdited` mencegah requote menimpa harga manual; edit memakai harga penuh (BR-10/11). Revamp `product.MinSellingPrice` sudah ada (`product/contracts/client.go:47`, `int64`, 0 = tanpa minimum karena kolom NOT NULL? — verifikasi: `MinSellingPrice int64`; anggap `0` = tidak ada minimum, selaras validasi `min <= selling`).

### Backend
1. Payload: `linePayload.UnitPrice *int64 \`json:"unitPrice"\`` (nil = harga otomatis; ≥0). `LineInput.UnitPrice *int64`.
2. `priceLines` (`service.go:148-239`): urutan keputusan per baris:
   - member `quoted.Applied` → harga member, `pricingSource="member_rule"`, `unitPrice` kiriman DIABAIKAN (legacy: member menimpa); guard minimum dilewati.
   - else `UnitPrice != nil` → `pricingSource="manual"` (harga manual; validasi `>=0`).
   - else `p.SellingPrice`, `pricingSource="standard"`.
   - Guard minimum (hanya `manual` dan `standard`): `if p.MinSellingPrice > 0 && netLine < round(p.MinSellingPrice × qty)` → field error baris: `baris N: harga jual bersih Rp a/satuan di bawah harga jual minimum Rp b` (net per satuan = `netLine/qty`, format id-ID). Memakai net setelah diskon (legacy: `amounts.unitPrice` = harga setelah diskon). Integer-safe: bandingkan total baris.
3. Kolom baru `sales_order_items.pricing_source VARCHAR(20) NOT NULL DEFAULT 'standard'` (migrasi `0000NN_sales_items_pricing_source`, modul sales) + `itemView.pricingSource` + `contracts.Item.PricingSource`; dipakai FE edit (kirim ulang `unitPrice` hanya untuk baris `manual`) dan laporan. `Update` (draf) mengikuti aturan sama (tidak ada "re-quote diam-diam" yang menimpa manual karena FE mengirim harga manual).
4. POS (`channel=pos`): tetap tanpa `unitPrice` dari kasir kecuali izin? Legacy POS mengirim `unit_price` dari katalog/member. Rancangan: server menerima `unitPrice` di POS hanya bila sama dengan harga katalog (diabaikan, harga server) — sederhana: **POS tidak mengirim unitPrice** (perilaku sekarang dipertahankan).
5. Net wajib customer+HP, prepaid, COD lunas: **agen penjualan**. Dari sisi saya kontrak error: `validateHeader` mengembalikan `FieldError{partyId|paymentTerms}`; FE hanya menampilkan.
6. Tidak mengubah bentuk laporan margin; `pricing_source='manual'` bisa dipakai audit.

### Frontend
- `sales/types.ts`: `SalesOrderLinePayload.unitPrice?: number`; `SalesOrderItem.pricingSource`; hapus komentar "klien TIDAK mengirim".
- `sales-order.schema.ts`: `unitPrice: z.coerce.number().int().min(0).optional()`; perbaiki header komentar.
- `SalesOrderFormPage.tsx`: kolom "Harga" (input angka rupiah) per baris — nilai awal `catalogPrice`/harga member dari `pricing/quote` (`pricingService.quote`, izin `pricing.quote`; gagal → harga katalog + notice); ubah → `priceEdited=true` dan harga tidak diganti oleh requote (BR-10). Di bawah input: petunjuk "Minimum Rp X" (dari `ProductSearchSelect`/detail produk `minSellingPrice`) dan peringatan inline merah bila harga bersih < minimum (klem klien hanya bantuan; server menolak sah). Baris member: input terkunci + badge "Harga member". Diskon: tampil per-satuan ditentukan klem `(harga penuh − minimum)×qty` sebagai batas atas input diskon Rp (BR-09) — peringatan, bukan pemblokiran.
- Termin: `paymentTerms` pilihan `cod | net | prepaid` (label: Bayar Saat Serah / Tempo / Prabayar — `paymentTermsLabel`); net → wajib customer (bukan walk-in) + jatuh tempo, tampilkan telepon customer & peringatan bila kosong (server menolak: pesan ditampilkan); prepaid/COD: Notice "Wajib lunas sebelum {dikirim|diselesaikan}".
- Estimasi `estimateLine` memakai harga baris (bukan `catalogPrice` saja).
- Detail SO: kolom "Sumber harga" kecil (Manual/Member).

### Test
- Go unit: `priceLines` tabel (member menimpa manual; manual < min → error; = min lolos; diskon membuat net < min → error; `UnitPrice` nil → katalog; negatif → error) — fungsi guard diekstrak ke `sales/domain/pricing.go` (`CheckMinPrice`) dengan test; Go integrasi: create+update dengan harga manual tersimpan, `pricing_source` benar, member tak bisa di-bypass.
- Vitest: `SalesOrderFormPage` (harga manual dikirim; baris member terkunci; peringatan minimum), `estimate.test.ts` diperluas.
Dependensi: agen penjualan (prepaid/COD/net). Konflik: `sales/application/service.go` juga disentuh agen lain → pecah ke berkas baru `pricing.go` bila memungkinkan.

---

## F3a-10 — Finance/Reporting/Auth: UI costing, switch-role, placeholder, peleburan laporan  (Ukuran: M)

1. **HPP "produk keluar tanpa harga modal"**: `GET /finance/costing/uncosted` (sudah mengisi kode+nama, `application/costing.go:175-191`) dan `POST /finance/costing/sync` (`finance.post`) tanpa UI. Legacy `finance-opening-page.tsx:71` ("Lengkapi Stok Awal", produk "Menunggu data").
   - FE: panel `CostingStatusCard` di `finance/pages/OpeningPage.tsx` (judul "Barang Keluar Tanpa Harga Modal"): tabel uncosted + tombol "Sinkronkan biaya" (`finance.post`, hasil `SyncResult` ditampilkan) + tautan ke Saldo Awal Stok/Edit produk (harga beli). Peringatan juga di `PostingPage` bila daftar tak kosong (membaca `uncosted` ringan).
   - Service: `financeService.costingUncosted()`, `costingSync()`; tipe `UncostedItem`.
   - Test Vitest: kartu tampil/kosong/sinkron berhasil; Go sudah punya integrasi costing? tambah `costing_uncosted_test.go` bila belum (verifikasi di `finance_closing_test.go`).
2. **`stock/allocations/suggest`** — dipakai F3a-2 (`CreateDeliveryModal` saran lokasi per baris, "Saran lokasi") alih-alih dibiarkan tanpa UI; tombol "Saran" per baris. Bila implementor menilai tak perlu: hapus route + `Suggest` (keputusan teknis, bukan pemilik).
3. **`auth/switch-role`**: ketergantungan keputusan **Q1**. Rancangan opsi B (paritas, rekomendasi): `Verify/resolveSession` memakai `GetPermissionCodes(ctx, []int64{roleID})` (izin role AKTIF); `Me` mengembalikan `roles:[{id,code,name}]` + `activeRoleId`; `AvatarMenu` (di `AppLayout.tsx`) tambah submenu "Ganti Role" (tampil bila `roles.length>1`) → `authService.switchRole(roleId)` → simpan token baru → `loadMe()`; `sessionCache` sudah di-drop saat switch. Opsi A (hapus): hapus route `switch-role`, `Service.SwitchRole`, `UpdateBranchRole`/`active_role_id` dan field `RoleCode` bila tak dipakai. Test: Go integrasi `user_roles_test.go` diperluas (dua role: izin berubah setelah switch; role dicabut saat sesi berjalan → fallback role pertama), Vitest `AvatarMenu.roles.test.tsx`.
4. **PlaceholderPage**: hapus `app/routes/placeholder.tsx` dan `?? PlaceholderPage` di `protected.routes.tsx`; gantikan dengan pengecekan saat build/test (`routes.registry.test.ts`: tiap `MENU_GROUPS` path ada di `MODULE_PAGES` selain `/dashboard`) — masuk F3a-12.
5. **Reporting vs Reports tumpang tindih**: `ReportingPage` (Tren & Stok: sales-trend + inventory) vs `finance/ReportsPage` (inventory-value) memakai tipe `InventoryPosition` ganda (`reporting/types.ts:12`, `finance/types.ts:290`); legacy tidak punya halaman reporting sama sekali (`19-reporting/feature-inventory.md`: "0 halaman web"). Rancangan peleburan (koordinasi agen finance):
   - Hapus bagian Stok dari `ReportingPage` → tautan ke `/finance/reports/inventory-value` (sumber tunggal `ReportsPage`).
   - Tren penjualan dipindah ke Dashboard (endpoint `dashboard/trend`, F3a-7) dan `ReportingPage` dikecilkan jadi "Tren Penjualan" dengan rentang bebas (`GET /reporting/sales-trend` dipertahankan, izin `reporting.view`).
   - Satu tipe `InventoryPosition` (`finance/types.ts`), `reporting/types.ts` hanya `TrendPoint`. Menu "Tren & Stok" → "Tren Penjualan".
   - Pertimbangan lanjutan (tidak sekarang): hapus modul `reporting` sepenuhnya bila `dashboard/trend` + `finance` menutup semua (Q5 opsional).
   Test: Vitest regresi tipe (`tsc`) + `ReportingPage.test.tsx` ringan.

---

## F3a-11 — Komponen retur kembar → parametris  (Ukuran: M)

Bukti: `PurchaseReturnCreateModal.tsx` (473) vs `SalesReturnCreateModal.tsx` (775): blok state, `useAsyncData` konteks + fallback detail order, reset-on-open, `handleSourceChange`, `patchLine`, `clampQty`, `collectPayload`, preview, submit identik; beda: nama field (`received` vs `delivered`), service/skema, dan Sales punya mode tukar (`returnMode`, `replacements`, `uniqueUoms`, `ReplDraft`). Detail: `PurchaseReturnDetailPage` (416) vs `SalesReturnDetailPage` (766): bagian Informasi/Item/Penyelesaian/Konfirmasi/Batal identik; Sales menambah dua modal pengiriman pengganti.
Rancangan (perilaku tidak berubah; refaktor bertahap):
- Folder FE baru `modules/returns/` (pengelompokan UX, FRONTEND_GUIDE):
  - `lib/clamp-qty.ts` (+test), `lib/return-lines.ts` (`toLineDrafts(ctx, kind)`, `estimateTotal`).
  - `hooks/useReturnSource.ts<TKind>`: parameter `{ kind, loadContext, loadOrderDetail, loadLocations }` → mengembalikan `{ sourceId, setSourceId, lines, patchLine, preview…, contextError, loadingSource }` (memuat logika render-phase reset yang sama).
  - `components/ReturnCreateModal.tsx`: konfigurasi `kind: "sales"|"purchase"` (label: "Order Jual/Beli", "Terkirim/Diterima", service, skema Zod) + slot `extra` (`ReplacementEditor` hanya untuk sales) + `onCreated`.
  - `components/ReturnDetailView.tsx` (Informasi, Item, Penyelesaian, aksi Konfirmasi/Batal via `runAction`) + slot `extraSections/extraModals` (sales: pengganti).
- `SalesReturnCreateModal` & `PurchaseReturnCreateModal` menjadi pembungkus tipis (<80 baris) yang memanggil komponen bersama; nama ekspor/rute tak berubah.
- Pemecahan `SalesReturnDetailPage` (766) menjadi pembungkus + `ReplacementDispatchModal`, `ReplacementConfirmModal`, `SettlementModal` (`settlement` bersama dengan purchase).
- Urutan aman: (1) ekstrak fungsi murni + test; (2) ganti Purchase (lebih sederhana) dan jalankan test; (3) ganti Sales; (4) detail.
- Test Vitest: `return-lines.test.ts`, `useReturnSource.test.tsx` (fallback saat konteks 403, clamp), uji "kontrak keluaran" `ReturnCreateModal` per kind: payload yang dikirim identik dengan versi lama (rekam payload dari test karakterisasi yang ditulis SEBELUM refaktor).
Dependensi: F3a-3 (pemilih sumber `SalesOrderSearchSelect`) — kerjakan setelahnya agar tak mengubah dua kali.

---

## F3a-12 — Cek otomatis kontrak FE↔router dan izin  (Ukuran: S)

Fakta: skrip `scratchpad/match.js` + `revamp_routes.py` sudah membuktikan 253/253. Produksikan jadi `scripts/contract-check.mjs` (Node murni, tanpa Python) dan sambungkan ke `scripts/arch-check.mjs` sebagai aturan **E/F/G** (npm run `arch:check` dan `verify`):
- **E (rute)**: parse `mux.Handle("VERB path", perm("x", …))` semua `internal/modules/**/presentation/*.go` (+ `server`), parse panggilan FE (`apiGet|apiPage|apiPost|apiPut|apiPatch|apiDelete|apiUpload*` dan `httpClient.get/post…` dengan literal/template string, templat `${}` → `{x}`), cocokkan metode+pola. `MISMATCH` → gagal. Allow-list tetap: `/api/v1/auth/refresh` (axios mentah di `http-client.ts`).
- **F (izin)**: setiap `perm:` di `app/routes/registry.tsx`, `RequirePerm perm=`, `EXTRA_ROUTES.perm`, dan `can("…")` di FE harus ada pada `PermissionCatalog()` (`**/contracts/*.go`) atau literal `perm("…")` BE; typo izin → gagal. (Mencegah lagi `member_types.manage` vs `view`.)
- **G (halaman)**: tiap path `MENU_GROUPS` (kecuali `/dashboard`) ada di `MODULE_PAGES`; tiap file `pages/*.tsx` ter-rute (port `orphans.js`).
- Peringatan (tidak gagal): rute BE tak dipanggil FE dan tak ada di `scripts/contract-allowlist.json` (hold/release stok masuk allow-list sampai F3a-8 selesai); pemilih dengan `limit>=50` di berkas `*lookup*` (F3a-3).
- Test: `scripts/contract-check.test.mjs` (node:test) dengan fixture mini (Go + TS) memastikan deteksi mismatch & izin salah; jalankan di `verify`.
- Tidak mengubah BE/FE produksi; murni skrip. Biaya rendah, nilai tinggi karena banyak endpoint berubah di F3a/F2a.

---

## Urutan kerja yang disarankan
1. P1,P2,P3 + F3a-12 (skrip) → 2. F3a-1 → 3. F3a-5 (daftar lain) → 4. F3a-3 (pemilih) → 5. F3a-2 (detail terpadu) → 6. F3a-6 (cetak) → 7. F3a-7 (UX) → 8. F3a-9 (harga manual) → 9. F3a-4 & F3a-8 (setelah F2a) → 10. F3a-10, F3a-11.
Konflik file dengan agen lain: `sales/application/service.go`, `payment/application/service.go`, `stock/*` — pakai berkas baru (`pricing.go`, `billing.go`) dan rebase sering.

## Daftar migrasi tambahan (nomor diisi saat merge; satu per modul)
| Nama | Modul | Isi |
|---|---|---|
| `orders_filter_indexes` (sales) | sales | `sales_orders (branch_id, order_date)`, `(branch_id, party_id, id)` |
| `orders_filter_indexes` (purchasing) | purchasing | idem `purchase_orders` |
| `returns_date_indexes` | salesreturn, purchasereturn (dua berkas) | `(branch_id, return_date)` |
| `payments_filter_indexes` | payment | `(branch_id, paid_at)`, `(branch_id, party_id, id)` |
| `audit_actor_index` | audit | `(actor_id, created_at)` |
| `expenses_date_index` | finance | `business_expenses (branch_id, expense_date)` |
| `sales_items_pricing_source` | sales | kolom `pricing_source` |
| `perm_member_types_view` | user | permission + grant ke pemegang `member_types.manage`/`customers.view`/`pricing.quote` |
Down-migration tiap berkas: `DROP INDEX`/`DROP COLUMN`/hapus permission.

## Risiko
1. Perubahan semantik izin bila opsi B switch-role dipilih: pengguna multi-role kehilangan izin gabungan sampai ganti role (butuh komunikasi + default role aktif = role dengan izin terbanyak).
2. `SearchIDs` terpotong 1000 id untuk istilah sangat umum (1–2 huruf): hasil pencarian pihak tak lengkap; mitigasi: minimal 2 karakter untuk pencarian pihak, tetap cari nomor.
3. Tiga agen menyentuh `sales/application/service.go`/`payment/application/service.go`: risiko konflik merge → berkas baru + urutan merge: F2a/atomisitas dulu.
4. Kontrak F2a (daftar saldo, `clientRef`, hold) hanya diasumsikan; bila bentuknya berbeda, F3a-4/8 perlu penyesuaian service/tipe (bukan arsitektur).
5. Nota cetak: hasil fisik tak bisa diverifikasi di sini (kertas kontinu/termal) → butuh uji pemilik (L3).
6. `order-billing` menghitung sisa tanpa koreksi retur (beda dari angka legacy `outstandingAmount` yang menggabung retur); pengguna yang terbiasa angka legacy mungkin menemukan selisih — retur ditampilkan sebagai blok terpisah.
7. Draf `sessionStorage` memuat data transaksi di perangkat bersama kasir; bersihkan saat logout/sukses dan kunci per user.

## Pertanyaan terbuka (hanya yang butuh keputusan pemilik)
- **Q1** Ganti role aktif: (B) kembalikan paritas legacy — izin = role aktif (rekomendasi, least-privilege) atau (A) pertahankan union izin dan HAPUS fitur ganti-role? Berapa banyak pengguna produksi yang memegang >1 role?
- **Q2** Kolom Harga Beli di daftar produk: tampil untuk semua pemegang `products.view` (seperti legacy) atau hanya yang berhak mengubah produk?
- **Q3** Peleburan Reporting: setuju menghapus bagian "Stok" dari halaman Reporting dan memindahkan tren ke Dashboard (menu "Tren & Stok" → "Tren Penjualan"), atau modul `reporting` dihapus total?
- **Q4** Kwitansi: pertahankan terbilang + tanda tangan (tambahan revamp; legacy sengaja tanpa tanda tangan, BR-30) atau samakan persis dengan legacy?
- **Q5** Pencarian pihak minimal 2 karakter (batas 1000 id) — dapat diterima, atau butuh pencarian tanpa batas (menuju snapshot `party_name`)?
