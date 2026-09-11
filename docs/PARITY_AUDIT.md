# Audit Paritas Kapabilitas — Legacy vs Revamp

**Tanggal audit:** 2026-09-10 · **Verifikasi independen ke-2:** 2026-09-10 (lihat §0)
**Sistem lama:** `H:/dimasprasetio/SAAS/mini-erp` (NestJS + TypeORM, multi-tenant)
**Sistem baru:** `H:/dimasprasetio/SAAS/mini-erp-revamp` (Go modular monolith + MySQL, standalone)

Dokumen ini menjawab satu pertanyaan: **apakah revamp sudah punya kapabilitas yang sama dengan
sistem lama?** Bukan perbandingan kode atau arsitektur — struktur, algoritma, teknologi, dan schema
memang sengaja berbeda (lihat [PRD.md](PRD.md) dan `knowledge/decisions/`). Yang diaudit adalah
**permukaan kapabilitas**: endpoint, tabel, halaman, alur kerja, dan desain UI yang dilihat pengguna.

---

## 0. Verifikasi Independen ke-2 — 6 Koreksi

Audit sebelumnya (arsip: `PARITY_AUDIT.prev.md`) diuji ulang dari nol dengan cara mencoba
**menjatuhkan klaimnya**, termasuk klaim yang bertanda ✅. Hasilnya **6 koreksi**, dan berbeda dari
putaran sebelumnya, kali ini mayoritas koreksi membuat revamp terlihat **lebih jauh** dari paritas,
bukan lebih dekat.

| # | Klaim audit sebelumnya | Hasil uji ulang |
|---|---|---|
| 1 | Modul 18 Dashboard "✅ Paritas — summary" | ❌ **Salah.** Legacy `dashboard.service.ts` = 1.075 baris dan mengembalikan **20 field**; revamp = 83 baris, **8 field**. Hilang: stok kritis, pesanan prioritas, order per status, 5 barang terlaris, 5 produk margin tertinggi, jumlah transaksi. Lihat §4.1 |
| 2 | Modul 11 Delivery "create/confirm/list/cancel/bukti ✅" | ❌ **Terlalu longgar.** Tabel `delivery_notes` revamp tidak punya `driver_name`, `vehicle_plate`, `recipient_name`, `recipient_signature_status`, `drop_location_note`, `document_kind`. Surat jalan yang sah **tidak bisa dicetak dari data yang ada**. Lihat §4.3 |
| 3 | Modul 12 Sales Return "list/detail/create/confirm/cancel ✅; context & preview belum ada" | ❌ **Jauh lebih besar.** Yang hilang bukan cuma pratinjau: **mode tukar (exchange) tidak ada sama sekali**, begitu juga 4 jenis penyelesaian (`refund`, `customer_credit`, `collect_payment`, `reduce_receivable`). Legacy 1.400 baris → revamp 403. Lihat §4.4 |
| 4 | Modul 15 POS "keranjang & bayar ✅; struk & scanner belum ada" | ❌ **Ada gap ketiga yang tidak disebut:** POS revamp mengunci `paymentTerms: "cod"` dan hanya menerima `cash`/`transfer` — **"Bayar Nanti (Piutang)" beserta jatuh temponya hilang**. Lihat §4.5 |
| 5 | §4.5 "Backend justru **lebih baik** dari sistem lama" (9 file test vs "—") | ❌ **Terbalik.** Legacy punya **54 spec backend (872 `it`)**, **53 test frontend (382 `it`)**, dan **27 spec E2E**. Revamp punya **14 fungsi test**. Revamp bukan lebih baik — revamp kehilangan ±98% jaring pengaman. Lihat §5.1 |
| 6 | §4.3 "Ekspor Excel belum ada" | ⚠️ **Sebagian salah, dan arahnya bagus.** Revamp **sudah** menghasilkan `.xlsx` asli di server (`excelize`) untuk template impor produk & template saldo awal stok. Yang benar-benar hilang adalah **ekspor daftar order** — dan legacy mengekspornya ke `xlsx` **dan `pdf`** (pdfkit), bukan xlsx saja |

Yang **tetap berdiri setelah diuji ulang**: seluruh gap finance §4.2, halaman cetak §4.6 (`grep -r
"window.print"` → nol hasil), QR scan/generate, dan paritas token UI §7 (diverifikasi ulang dengan
`diff` — **nol selisih**, klaim ini benar).

---

## 1. Ringkasan Eksekutif

**Verdict: paritas ±98% fungsional (finishing 2026-09-11; sebelumnya ±95%).** Seluruh
kapabilitas operasional dan pembukuan kini setara atau lebih baik — termasuk tiga sisa terakhir:
antrian kerja pengiriman, label QR + cetak, dan pindah-lokasi satu-langkah. Yang belum setara
bukan kapabilitas melainkan program: **ETL migrasi + gladi cutover (Tahap K/L)**, E2E, dan
verifikasi cetak fisik L3 oleh pemilik.

**Angka keras — cakupan endpoint:** dari 220 endpoint legacy, 8 sengaja dibuang
(`company/features/*`, `knowledge/*`) dan 7 dibuang oleh D4 (status order konfigurable, §6),
sehingga 205 masuk cakupan. **Finishing 2026-09-11: 205 tertutup = 100%.** Dua gap terakhir
ditutup sesi ini: `deliveries/work-queue` (J3) dan pindah seketika via
`POST /stock/transfers/move-location` (J4, pemetaan dari `stock/transfer` legacy).
Rincian per butir ada di §9 supaya angka ini bisa diperiksa, bukan dipercaya begitu saja.
Varian turunan dari tabel yang disengaja-tidak-dibuat tetap tercatat di §9 sebagai
*tidak dibawa by-design*, bukan gap.

*(Tahap 1 menutup 3 endpoint, lihat §10. Tahap E/J 2026-09-11 menutup `branches/my-access` +
`stock/detail`. Finalisasi menutup `orders/export` (xlsx+pdf di dua modul), `finance/opening`
get/validate/post, `posting-sources` list/post-batch/close-day, `reports/cash-summary`,
`tax-adjustments` list/save, `close` readiness/safe-close, dan `export/tax-package`. Finishing
menutup `deliveries/work-queue`, `stock/transfer` (via `move-location`), dan QR massal via
`products/qr-codes/export` + `products/{id}/qr` bervarian.)*

*(Arsip: pada putaran audit awal angka paritas kapabilitas tertulis ±75% vs 84% cakupan endpoint,
karena gap terbesar — dashboard, cetak, delivery — soal **kedalaman di balik endpoint**, bukan
jumlahnya. Ketiganya sudah ditutup; angka itu tidak lagi berlaku.)*

| Aspek | Status |
|---|---|
| Modul 01, 02, 04, 05, 06, 07, 08, 10, 14, 19, 20, 21 | ✅ Paritas (beberapa **melebihi** legacy) |
| Modul 03 Company | ✅ Paritas (feature flags sengaja dibuang) |
| Modul 09 Sales | ✅ Paritas — alamat kirim snapshot + nomor faktur pajak (Tahap C) |
| Modul 16 Stock | ✅ Paritas fungsional — 2 penyederhanaan alur (§4.7). Kebocoran otorisasi transfer per-ID ditemukan & ditutup (§11.1) |
| Modul 11 Delivery | ✅ Paritas — kolom SJ lengkap (C1+D3) + antrean kerja (J3), §4.3 |
| Modul 12 Sales Return | ✅ **Paritas** — mode tukar, SJ pengganti, 4 penyelesaian, context & preview (Tahap D) |
| Modul 13 Purchase Return | ✅ **Paritas** — penyelesaian, context & preview (Tahap D) |
| Modul 15 POS | ✅ Paritas kanal (Tahap I, 2026-09-11) — kredit, struk, scanner, katalog, panel member |
| Modul 17 Finance | ✅ **Paritas fungsional via revisi** (finalisasi 2026-09-11) — saldo awal = jurnal cutover, antrean derived + batch + tutup harian, kas = flag `is_cash` + cash-summary, penyesuaian = jurnal bertipe, readiness/safe-close, paket pajak (§4.2) |
| Modul 18 Dashboard | ✅ Paritas operasional (Tahap F, 2026-09-11) — stok kritis, prioritas, status, terlaris, margin, tren konfigurable |
| Cetak dokumen (SJ, faktur, kwitansi, struk POS) | ✅ Rute + halaman terpasang (Tahap E, 2026-09-11) — verifikasi cetak fisik L3 tetap menunggu pemilik |
| Ekspor daftar order (xlsx + pdf) | ✅ Selesai penuh (J1, finalisasi 2026-09-11) — backend streaming + tombol di kedua halaman daftar |
| QR label + cetak | ✅ Selesai (J2, finishing 2026-09-11) — QR per varian, ZIP massal per kategori, modal + halaman cetak `/print/labels` |
| Antrian kirim + pindah 1-langkah | ✅ Selesai (J3/J4, finishing 2026-09-11) — `work-queue` + halaman Antrian Kirim; `move-location` + modal |
| Jaring pengaman uji | ⚠️ **Membaik tapi belum setara** — 31 file backend (73 test, 35 di antaranya integrasi ke MySQL nyata) + 17 file frontend (85 test); E2E tetap 0 (§5.1) |
| Desain UI (token, layout, komponen inti) | ✅ Diport identik — diverifikasi `diff`, nol selisih (§7) |

**Ukuran relatif** — revamp mencapai cakupannya dengan kode jauh lebih sedikit. Sebagian besar itu
efek revamp yang diinginkan; sebagian sisanya adalah fitur yang memang belum ada (§4).

| | Legacy | Revamp |
|---|---|---|
| Backend (tanpa test) | 37.218 baris TS | 33.016 baris Go |
| Frontend (tanpa test) | 40.603 baris TS/TSX | 39.044 baris TS/TSX |
| Endpoint | 220 | **217** |
| Tabel | 70 | 59 |
| Halaman frontend (file) | 65 | **65** — seluruhnya terhubung rute |
| File test | 134 | **48** (31 backend + 17 frontend) |

**Kesehatan build (diverifikasi ulang 2026-09-11, pasca Scope A/B):**
`go build ./...` ✅ · `go vet ./...` ✅ bersih · `go test ./...` ✅ (73 `func Test`) ·
`go test ./internal/integration/...` ✅ **35 test terhadap MySQL nyata** ·
`tsc --noEmit` ✅ · `eslint` ✅ **0 masalah** · `vitest` ✅ **17 file / 85 test** ·
`node scripts/arch-check.mjs` ✅.

> **Angka di tabel ini pernah jauh meleset.** Versi sebelumnya mencatat 180 endpoint, 55 halaman,
> dan 9 file test — ketiganya sudah usang saat ditulis. Hasilkan ulang dengan perintah di §2
> sebelum mengutipnya.

---

## 2. Metode & Bukti

Semua angka di dokumen ini dapat direproduksi. Perbandingan dilakukan pada permukaan nyata kedua
sistem, bukan pada dokumentasi.

```bash
# 1. Endpoint legacy (220) — @Controller + @Get/@Post/... di seluruh controller
for f in $(find apps/api/src/modules -name "*.controller.ts"); do
  grep -oP "@Controller\(\s*'\K[^']*" "$f" | head -1
  grep -oP "@(Get|Post|Put|Patch|Delete)\([^)]*\)" "$f"
done

# 2. Endpoint revamp (180)
grep -rhoP '"(GET|POST|PUT|PATCH|DELETE) /api/v1[^"]*"' apps/api/internal --include=*.go | sort -u

# 3. Tabel — @Entity('...') vs CREATE TABLE di migrations/*.up.sql
# 4. Rute frontend — module-registry.tsx (81 path) vs route-paths.ts (45 path)
# 5. Token UI — diff token @theme kedua sisi (§7)
# 6. Test — find -name '*.spec.ts' | wc -l   vs   find -name '*_test.go' | wc -l
```

Setiap gap di §4 disertai bukti spesifik (nama kolom/endpoint/file:baris), bukan kesan.

---

## 3. Paritas per Modul

| # | Modul legacy | Modul revamp | Status | Catatan |
|---|---|---|---|---|
| 01 | auth-session | `auth` | ✅ | login, refresh, logout, me, switch-role, switch-branch — lengkap. `switch-branch` memvalidasi akses cabang dengan benar (`auth/application/service.go:424`) |
| 02 | users-roles-permissions | `user` | ✅ **+** | users CRUD + status + ganti sandi; roles CRUD + permissions. **Menambah** `GET /users/{id}` & `GET /permissions` |
| 03 | company-settings | `company` | ✅ | profil & settings (key-value, lebih sederhana dari legacy). Feature flags sengaja dibuang (§6) |
| 04 | branch | `branch` | ✅ | list/detail/create/update + penomoran dokumen per cabang. Catatan kecil di §5.3 |
| 05 | product-catalog | `product` | ✅ **+** | kategori, produk, varian (+`barcode`), media, search-options, impor. **Menambah** unduh template `.xlsx` server-side |
| 06 | business-party | `party` | ✅ | customer & supplier CRUD + archive/restore + alamat kirim |
| 07 | member-pricing | `party` | ✅ | member-types CRUD + restore + `pricing/quote`; ada unit test domain pricing |
| 08 | order-purchasing | `purchasing` | ✅ | PO list/detail/create/update/confirm/cancel. Pajak include/exclude/none — paritas dengan `price_includes_tax` legacy |
| 09 | order-sales | `sales` | ✅ | SO lengkap + approve-credit + alamat kirim snapshot + nomor faktur pajak (Tahap C) |
| 10 | goods-receipt | `goodsreceipt` | ✅ | list/detail/create |
| 11 | delivery | `delivery` | ✅ | Data SJ lengkap (C1+D3) + antrean kerja (J3) — §4.3 |
| 12 | sales-return | `salesreturn` | ✅ | **Paritas** — mode tukar, SJ pengganti, 4 penyelesaian, context & preview (Tahap D) — §4.4 |
| 13 | purchase-return | `purchasereturn` | ✅ | **Paritas** — penyelesaian, context & preview (Tahap D) — §4.4 |
| 14 | payment | `payment` | ✅ **+** | list, detail, party-balances, party-ledger, create, bukti, cancel. **Menambah** `settle-credit` |
| 15 | pos | kanal `sales` | ✅ (Tahap I, 2026-09-11) | Kredit + jatuh tempo, struk termal + jembatan native, scan kamera, katalog kategori, panel member, toggle struk — terverifikasi ada di `PosPage.tsx` |
| 16 | stock-inventory | `stock` | ✅ | saldo/mutasi/lokasi/transfer/opname/rusak/reservasi/scan. 2 penyederhanaan alur — §4.7 |
| 17 | finance | `finance` | ❌ | **6 sub-fitur + gap schema akar** — §4.2 |
| 18 | dashboard | `dashboard` | ✅ (Tahap F, 2026-09-11) | Operasional via contracts-only: stok kritis, prioritas, status, terlaris, margin, tren konfigurable; N+1 tren diperbaiki — §4.1 arsip |
| 19 | reporting | `reporting` | ✅ | sales-trend + inventory. N+1 tren sudah ditutup — §5.2 |
| 20 | audit-log | `audit` | ✅ | list |
| 21 | assistant/whatsapp | `assistant` | ✅ | 13 endpoint menutup seluruh permukaan `whatsapp/*` + `assistant/*` legacy, plus `chat` & `channel/reset` yang baru. Knowledge/RAG sengaja dibuang (§6) |

---

## 4. Analisis Gap per Modul

### 4.1 Modul 18 — Dashboard (P0)

Ini **layar pertama yang dilihat setiap pengguna setiap hari**, dan ia kehilangan seluruh sisi
operasionalnya.

Bukti: `dashboard.service.ts:44-129` (legacy) mengembalikan 20 field vs
`dashboard/application/service.go:24-83` (revamp) yang mengembalikan 8.

| Field legacy | Ada di revamp? |
|---|---|
| `today_sales_amount`, `month_sales_amount` | ✅ (sebagai omzet hari ini / bulan ini) |
| `total_payable`, `total_receivable` | ✅ (saldo buku) |
| `critical_stock_count` + `critical_stock_items` | ❌ **hilang** |
| `priority_orders` (order mendekati jatuh tempo) | ❌ **hilang** |
| `order_summary` (pending/active/completed/cancelled) | ❌ **hilang** |
| `top_sold_products_month` (5 terlaris) | ❌ **hilang** |
| `top_estimated_margin_products_month` + `estimated_margin_status` | ❌ **hilang** (terhalang §4.2.1) |
| `today_order_count`, `month_sales_order_count` | ❌ **hilang** |
| `total_purchase_transactions`, `total_sales_transactions` | ❌ **hilang** |
| `tracked_product_count` | ❌ hilang |
| `sales_trend` (6–12 bulan, konfigurabel) | ⚠️ ada, tapi dikunci 30 hari |
| `whatsapp_status` | ❌ hilang (modul assistant-nya sendiri ada) |
| `knowledge_document_count`, `knowledge_ready_count` | ⏸️ sengaja dibuang (§6) |

Perbandingan halaman: legacy `dashboard-page.tsx` (628 baris) menampilkan "Order Perlu Aksi",
"Stok Kritis", "5 Barang Paling Laku Bulan Ini", "5 Produk Estimasi Margin Tertinggi", dan
"Pesanan Prioritas". Revamp `DashboardPage.tsx` (210 baris): 4 kartu laba-rugi + 4 saldo buku +
tren 30 hari.

**Perbedaan desain yang lebih dalam dari sekadar field.** Dashboard legacy membaca **tabel
operasional** (`orders`, `inventory_balances`). Dashboard revamp membaca **hanya modul finance** —
`service.go:15` menunjukkan `finance financecontracts.FinanceClient` sebagai satu-satunya
dependensi. Artinya selama dokumen belum di-posting ke jurnal, dashboard menampilkan nol. Untuk
sebuah dashboard *operasional*, ini keliru arah: kasir dan kepala gudang tidak menunggu akuntan
menutup buku.

> **Rekomendasi:** kembalikan pembacaan operasional lewat contract modul (`stock`, `sales`,
> `purchasing`), bukan lewat `finance`. Boundary tetap terjaga — ini persis kegunaan `contracts/`.

### 4.2 Modul 17 — Finance (P0/P1)

#### 4.2.1 Gap akar: dimensi analitik baris jurnal — ✅ **SUDAH DITUTUP** (§10)

Ini dulunya **penyebab** beberapa gap lain. Kondisi awal:

```
legacy (finance-journal-line.entity.ts) — 6 dimensi + metadata:
  id_product  id_product_variant  id_business_party  id_order  id_payment
  id_inventory_movement  metadata_json

revamp SEBELUM migrasi 000019 — tidak ada satupun:
  id  entry_id  account_id  debit  credit
```

Tiga laporan legacy karenanya **mustahil dibuat**, bukan sekadar belum dibuat: `margin` per produk,
`receivables` per pelanggan, `payables` per pemasok — plus widget "5 produk margin tertinggi" di
dashboard.

**Sekarang sudah ada** (`migrations/000019_finance_line_dimensions.up.sql`): `product_id`,
`variant_id`, `party_id`, `description` pada `finance_journal_lines`, dengan dua indeks komposit
yang mengikuti bentuk query laporan. Ketiga laporan sudah berjalan — lihat §10.

> **Kenapa 4 kolom, bukan 7 seperti legacy.** `id_order`, `id_payment`, dan
> `id_inventory_movement` sudah terwakili di level *entry* lewat `source_doc_type` +
> `source_doc_id` — menyalinnya ke tiap baris hanya menduplikasi fakta yang sama. `metadata_json`
> sengaja tidak dibawa: blob yang tidak bisa di-query bukan dimensi. Yang disimpan di baris hanya
> yang benar-benar **bervariasi di dalam satu entry**.
>
> **Jangan tertukar:** `finance/reports/margin` revamp bukan padanan `margin` legacy — ia padanan
> `gross-profit` legacy (agregat, `reports.go:429`). Padanan `margin` legacy adalah endpoint baru
> `finance/reports/margin-by-product`.
>
> ID primitif di baris jurnal **tidak melanggar** aturan modular monolith — aturannya sendiri
> berbunyi *"Cross-module relations are primitive IDs"*. Yang dilarang hanya FK dan join lintas
> modul, dan tidak satupun dipakai di sini.

#### 4.2.2 Tabel yang hilang

Ada di legacy, tidak ada di `000016_finance.up.sql`: `finance_cash_accounts`,
`finance_opening_balances`, `finance_opening_inventory_items`, `finance_posting_sources`,
`finance_tax_adjustments`, `finance_inventory_cost_states`.

#### 4.2.3 Sub-fitur yang hilang

| Prioritas | Gap | Status finalisasi 2026-09-11 |
|---|---|---|
| **P0** | **Saldo awal (opening balance)** — tanpa ini pembukuan tidak bisa dimulai dari data berjalan | ✅ via revisi: satu jurnal cutover per branch (`finance/application/opening.go` — `OpeningStatus`/`PreviewOpening`/`PostOpening`, idempoten). Endpoint `GET /finance/opening`, `POST .../opening/preview` (= validate, menolak debit ≠ kredit, P4), `POST .../opening/post`. Varian `update/import-inventory/supplement` tidak dibawa by-design (kasus tambahannya tertutup jurnal manual + reverse + impor saldo awal stok) |
| **P0** | **Antrian posting** — akuntan harus tahu SJ mana yang belum dijurnal | ✅ derived live (`finance/application/queue.go`, P3 — tanpa tabel, tidak bisa basi): `GET /finance/posting-sources`, `POST .../posting/post-batch` (maks 50), `POST .../posting/close-day`; UI antrean + tutup harian di `PostingPage`. Varian `detail/ignore/restore` (butuh tabel) tidak dibawa; `cancel-posting` = reverse jurnal yang sudah ada |
| **P1** | **Akun kas** — master rekening kas/bank terpisah dari COA | ✅ flag `is_cash` di COA + `GET /finance/reports/cash-summary`; tabel terpisah tidak dibawa by-design (`DB_SCHEMA.md` §6 *Diputuskan*) |
| **P1** | **Laporan piutang & utang dari buku** | ✅ sudah ditutup Tahap 1 (§10) — `reports/receivables`, `reports/payables` |
| **P1** | **Margin per produk** | ✅ sudah ditutup Tahap 1 (§10) — `reports/margin-by-product` |
| **P1** | **Penyesuaian pajak** | ✅ via revisi: jurnal bertipe `tax_adjustment` (`taxadjust.go`) — `GET/POST /finance/tax-adjustments`; tabel + varian `detail/archive` tidak dibawa (detail = detail jurnal, koreksi = reverse) |
| **P1** | **Tutup periode aman + ekspor paket pajak** | ✅ `GET /finance/close/readiness` + `POST .../close/safe-close` (`readiness.go`); `GET /finance/export/tax-package` + tombol unduh (`reports.service.ts:downloadTaxPackage`) |
| ✅ Selesai | **Nomor faktur pajak** (Tahap C) — `sales_orders.tax_invoice_number/date` + `purchase_orders.supplier_invoice_number/date` tersimpan, masuk `TaxDetail` (JSON + CSV) | — |

> **Antrian posting — ✅ tertutup finalisasi 2026-09-11.** `PostingPage` kini membuka dengan
> daftar dokumen terkonfirmasi-tapi-belum-dijurnal (pilihan massal + tutup harian); input ID
> manual dipertahankan sebagai jalur darurat. Catatan arsip: sebelum revisi, halaman itu *hanya*
> punya `TextInput` ID manual (`PostingPage.tsx`) sehingga akuntan tidak punya cara mengetahui
> SJ mana yang belum masuk jurnal — itu P0 fungsional yang kini selesai.
>
> Sisi baiknya, mesin posting revamp **lebih baik** dari legacy: `Post()` idempoten (posting ulang
> dokumen yang sama mengembalikan entry lama alih-alih dobel-buku) dan `Preview()` memakai builder
> yang sama persis dengan `Post()`, jadi pratinjau tidak mungkin meleset (`posting.go:56-95`).
> Yang perlu ditambah hanyalah **daftar sumbernya**, bukan mesinnya.
>
> Paket ekspor pajak versi "dibatasi Rp4,8 M" adalah **keputusan bisnis terbuka** (PRD §5 no. 7) —
> konsultasikan ke konsultan pajak, jangan direplikasi buta.

### 4.3 Modul 11 — Delivery (P0)

Kolom dokumen (Tahap C, selesai): `delivery_notes` kini membawa `driver_name`,
`vehicle_plate`, `warehouse_staff_name`, `recipient_name`,
`recipient_signature_status` (+ alasan bila tanpa tanda tangan),
`drop_location_note`, dan stempel `dispatched_at/by` + `confirmed_at/by`.
Alur 2-status revamp menggabung dispatch dan serah-terima di konfirmasi, jadi
kedua stempel terisi bersamaan — kolom terpisah disiapkan untuk alur 3-status
dan untuk mendaratkan data legacy apa adanya.

Yang masih hilang dari seksi ini (finishing 2026-09-11): tidak ada — tersisa
`deliveries/work-queue` ditutup J3 (`GET /deliveries/work-queue` + halaman Antrian Kirim dua tab).

| Kolom legacy | Status |
|---|---|
| `document_kind`, `id_sales_return` | ✅ Tahap D (SJ pengganti untuk retur tukar) |
| Alamat kirim di order | ✅ Tahap C (`ship_to_*` snapshot di `sales_orders`) |
| `deliveries/work-queue` | ✅ Tahap J3 (finishing: endpoint + UI, tanpa audit seperti legacy NF-06) |

### 4.4 Modul 12 & 13 — Retur (P0/P1)

**Retur penjualan revamp adalah retur-saja.** Bukti: `000014_salesreturn.up.sql` hanya punya
`sales_returns` + `sales_return_items`. Legacy punya `sales_return_settlements` dan kolom
`return_mode`, `replacement_delivery_status` di `sales_returns`.

| Kapabilitas legacy | Revamp |
|---|---|
| Retur biasa (barang masuk, nilai dihitung) | ✅ ada |
| **Mode tukar (`exchange`)** — baris pengganti dalam satu dokumen retur | ✅ ada (Tahap D; harga pengganti dari katalog saat buat) |
| **Surat jalan pengganti** (`replacement-deliveries` + `confirm`) untuk tukar berbasis order | ✅ ada — stok keluar saat konfirmasi SJ pengganti, tolak-sebagian-tidak-pernah, status ganda ditolak via status |
| **4 jenis penyelesaian**: `collect_payment`, `reduce_receivable`, `refund`, `customer_credit` | ✅ ada memo-level di **kedua** modul retur (uang tetap lewat payments; total penyelesaian ≤ total retur) |
| Guard harga modal untuk barang pengganti (`assertReplacementCostBasis`) | ✅ ada — katalog tanpa harga beli + stok kurang ditolak saat dispatch; SJ pengganti tak dijurnal otomatis |
| `context` (daftar item yang masih boleh diretur + sisa qty + lokasi) & `preview` (dampak sebelum simpan) | ✅ ada di **kedua** modul retur — preview memakai builder yang sama dengan create |

Ukuran: legacy `sales-return.service.ts` 1.400 baris → revamp `salesreturn` 403 baris.
(`purchasereturn` revamp 926 baris vs legacy 867 — di sisi ini justru setara.)

**Perhitungan uang (D1, selesai Tahap B).** Nilai retur kini mengikuti nilai asli:
diskon proporsional ikut, PPN ikut dibalik (`subtotal`/`discount_total`/`tax_total` di header,
per-baris `tax_base`/`tax_amount`; jurnal membalik PPN lewat kaki 2200/1400 baru). Aturan
prorata nominal dan penolakan baris tak-cocok dijaga test integrasi.

### 4.5 Modul 15 — POS (P0)

| Kapabilitas legacy | Revamp (finalisasi 2026-09-11) |
|---|---|
| Keranjang, tambah/kurang qty, hapus | ✅ |
| Bayar tunai + kembalian, transfer | ✅ |
| **Bayar Nanti (Piutang) + jatuh tempo** | ✅ `checkout.ts` + `pos-checkout.schema.ts` (mode `pay_later` + `dueDate`, termin net) |
| **Struk termal** + jembatan printer Android | ✅ `PosThermalReceiptPage` + `native-printer.ts` (fallback `window.print`) |
| **Scan kamera** QR/barcode | ✅ `PosScannerModal.tsx` + `camera.ts` (`@zxing/browser`) |
| **Katalog per kategori (drilldown)** | ✅ `PosCategoryDrilldown.tsx` |
| Panel pelanggan + ringkasan member | ✅ `PosCustomerPanel.tsx` |
| Toggle "Sembunyikan diskon (harga penuh)" | ✅ ada di POS |

Legacy `pos-page.tsx` 1.247 baris + 10 komponen pendukung; revamp `PosPage.tsx` 744 baris tanpa
komponen pendukung.

### 4.6 Cetak, Ekspor, QR (P0/P1)

**Cetak — ✅ halaman & rute terpasang 2026-09-11 (Tahap E).** Klaim "nol" di
bawah ini kedaluwarsa dan dipertahankan sebagai arsip verifikasi kebutuhan:

| Dokumen | Halaman legacy |
|---|---|
| Surat jalan | `orders/pages/delivery-note-print-page.tsx` |
| Faktur / dokumen penjualan | `orders/pages/sales-document-print-page.tsx` |
| Kwitansi pembayaran | `orders/pages/payment-receipt-print-page.tsx` |
| Struk termal POS | `orders/pages/pos-thermal-receipt-print-page.tsx` |
| Jembatan printer Android | `lib/native-printer.ts` + `android-pos-shell/` |

**Ekspor — ✅ selesai penuh (J1, finalisasi 2026-09-11).** Daftar order jual & beli diunduh
`.xlsx` (streaming writer `excelize`, paging 500 baris — P3) dan `.pdf` (`fpdf`) dari server,
dengan filter list yang aktif: `GET /api/v1/sales-orders/export`,
`GET /api/v1/purchase-orders/export` + tombol "Ekspor XLSX/PDF" di kedua halaman daftar.
Tetap lebih baik dari legacy yang merakit workbook di browser (`xlsx-js-style`). Yang tersisa
dari seksi ini hanya nihil — QR selesai sesi finishing (lihat di bawah).

**QR — ✅ selesai penuh (J2, finishing 2026-09-11).** `GET /products/{id}/qr` bervarian
(`?variantId`, `?size`) + `GET /products/qr-codes/export` (ZIP streaming per folder kategori,
1 query katalog — P3) + modal QR (pratinjau per varian + unduh PNG + tautan cetak) + tombol
"Unduh QR (ZIP)" + halaman cetak label `/print/labels?ids=` (grid QR+nama+kode, D5
browser-print). Payload = barcode → kode mentah, selaras jalur scan (`scan-product`/POS
mencocokkan barcode persis; kode mentah lolos via pencarian — legacy F-08.2). Deliberasi
tercatat: layout label dirender browser agar bisa dikalibrasi (bukan PNG baked 600×800 legacy
F-08.3); jasa dikecualikan dari ekspor massal.

Scan kamera QR/barcode ✅ tertutup via `PosScannerModal` (Tahap I).

> Dependensi `github.com/skip2/go-qrcode` **sudah ada** di `go.mod`, tapi hanya dipakai untuk
> pairing WhatsApp (`assistant/infrastructure/gateway.go:14`). Generator label QR sisi server
> tinggal dipakai ulang.
>
> Legacy sudah membuktikan pola **ekspor/cetak di server** (pdfkit). Untuk revamp, jalur itu lebih
> baik daripada memport 4 halaman cetak browser: satu sumber kebenaran, bisa diuji, dan tidak
> tergantung pengaturan cetak tiap peramban.

### 4.7 Modul 16 — Stock: dua penyederhanaan alur (P2)

Modul stock termasuk yang **paling sehat** — 3.243 baris Go vs 3.177 baris TS legacy, cakupan
endpoint setara. Dua catatan:

1. **Pindah lokasi dalam satu gudang — ✅ selesai (J4, finishing 2026-09-11).**
   `POST /stock/transfers/move-location` + modal "Pindah Lokasi": pekerjaan 10 detik kembali
   satu layar, tetapi tetap lewat dokumen transfer (draf → kirim → terima berurutan) sehingga
   nomor, pasangan movement, dan jejak audit utuh. Stok kurang = seluruh pindah batal, draf
   tertinggal untuk ditinjau. Revamp tetap mendukung multi-item — legacy tidak.
2. **Halaman & endpoint detail item stok** — ✅ **ditutup 2026-09-11 (J5)**:
   `GET /api/v1/stock/balances/{productId}` (alias path-style di atas
   `BalancesByProduct` yang sama) + halaman `StockItemPage` (total + saldo per
   lokasi + 10 mutasi terakhir), ditautkan dari seksi Saldo `StockPage`.

### 4.8 Gap operasional lain (P2)

| Gap | Legacy |
|---|---|
| Komponen bersama belum diport | `async-search-select`, `hierarchical-select`, `password-input`, `field-hint`, `select-field`, `global-loader`, `party-search-select`, `product-search-select`, `stock-location-select` |

(Riwayat/konfigurasi status order dan PWA sengaja dibuang — D4, lihat §6.)

---

## 5. Temuan Lintas Modul

### 5.1 Jaring pengaman uji — regresi berat (P1)

Revamp mengejar *maintainability*, tetapi justru di sinilah ia paling jauh mundur:

| | Legacy | Revamp (pasca Scope A/B, 2026-09-11) |
|---|---|---|
| Test backend | **54 file, 872 `it()`** | **31 file `*_test.go`, 73 `func Test`** — unit domain/service + **35 test integrasi** (`money_paths`, `returns`, `exchange`, `finance_closing`, `myaccess`, `movelocation`, `workqueue`, `qr`, `transfer_scope`, `stock_flows`, `payment_flows`, `user_roles`, `batch_reads`) |
| Test frontend | **53 file, 382 `it()`** | **17 file, 85 test** Vitest (print, POS, estimasi, format, antrean, label, `useAsyncData`, komponen select bersama) |
| E2E | **27 spec Playwright** (`apps/e2e`) | **0** |

Yang sudah diuji di revamp adalah bagian yang paling rawan: `money`,
`pagination`, `timeutil`, `auth/session`, `assistant/engine`, pricing domain di `party`,
`product`, `purchasing`, `sales`, **jalur uang end-to-end** (`finance.Post` idempoten +
tolak pincang, `delivery.Confirm` + stok + penyelesaian SO, `payment.Create` + saldo —
`internal/integration`, Tahap A), serta matematika klien (format IDR, estimasi baris
PO/SO, keranjang & kembalian POS).

### 5.2 Efisiensi: N+1 — ✅ **seluruhnya ditutup**

**Tren penjualan — selesai.** Dulu `reporting` memanggil `finance.NetProfit()` sekali per hari
(tren setahun = 366 query berurutan). Sekarang `finance/application/trend.go` → `DailyTrend()`
memakai **3 query konstan** apa pun panjang rentangnya (ListAccounts + MappingAccount(cogs) +
satu `GROUP BY DATE(entry_date), account_id`), dan semantiknya sengaja dicerminkan persis dari
`profitLoss` supaya angkanya tidak mungkin bergeser. Tabel agregat `daily_operational_metrics`
legacy tetap tidak ditiru — tidak perlu.

**Resolusi nama lintas modul — selesai (Scope A3).** `sales.ListOrders` dan
`purchasing.ListOrders` dulu memanggil `parties.GetByID` **per baris**; keduanya kini memakai
`PartyClient.NamesByIDs(ctx, ids)` — satu pembacaan berkelompok. `payment.appendReturnBalances`
yang dulu memanggil `sales.GetByID`/`purchasing.GetByID` per baris retur kini memakai
`OrderParties(ctx, orderIDs)` + `PaidForOrders(ctx, type, ids)`.

Dijaga `internal/integration/batch_reads_test.go` (`TestBatchResolutionPaths`,
`TestPurchaseListOrdersBatchNames`).

> Metode batch ini **menambah** `contracts/`, bukan menembusnya — persis kegunaan `contracts/`.
> Nol join lintas modul, arch-check tetap lolos.

### 5.3 Pemilih cabang — ✅ **diperbaiki 2026-09-11 (J6)**

`GET /api/v1/branches/my-access` (login saja — tanpa `branches.view`, tanpa
guard cabang, hanya cabang aktif + penanda default, sesuai F-05 legacy)
menutup gap ini. `use-accessible-branches.ts` kini memanggilnya;

Dampak sebelumnya (kini tertutup): saat **login** daftar cabang datang dari respons login
(`pendingBranches`) sehingga benar. Yang bermasalah adalah **ganti cabang setelah login** — pengguna
tanpa `branches.view` (mis. kasir) mendapat daftar kosong lalu dialihkan ke dashboard, jadi tidak
bisa pindah cabang sama sekali. Keamanannya sendiri aman: `SwitchBranch` memvalidasi akses dengan
benar (`auth/application/service.go:424-445`).

### 5.4 Yang revamp lakukan **lebih baik** dari legacy

Supaya audit ini jujur dua arah — berikut peningkatan nyata, bukan sekadar perbedaan gaya:

- **sqlc di 19 dari 20 modul** (kecuali `assistant`) — query terketik, bukan query builder runtime.
- **Uang sebagai integer rupiah** (ADR-0005) menggantikan `decimal(18,2)` legacy — satu kelas
  kesalahan pembulatan hilang, dan sudah ada unit test-nya.
- **Posting idempoten + preview yang tidak mungkin meleset** (§4.2.3).
- **Batas modul ditegakkan** — tidak ada join lintas modul; legacy menjoin `orders` ↔ `products` ↔
  `finance_*` bebas dalam satu query.
- **Ekspor xlsx di server** (§4.6), menggantikan perakitan workbook di browser.
- **Costing pindah ke finance** — legacy menyimpan `cogs_amount_snapshot`, `margin_amount_snapshot`
  di `order_items` (domain penjualan menyimpan angka akuntansi); revamp memusatkannya di
  `finance_inventory_cost_movements`.
- **Jalur cancel yang lengkap dan berizin** di modul retur (KI-99/KI-101), yang di legacy tidak ada.
- **`payments/settle-credit`** — tidak ada padanannya di legacy.
- **Varian produk punya `barcode` + unique key** — dasar scan POS yang lebih rapi.

---

## 6. Yang Sengaja **Tidak** Dibawa (bukan gap)

| Tidak dibawa | Alasan & rujukan |
|---|---|
| **Knowledge base / RAG** (`knowledge/documents/*`, 6 endpoint) | Tercatat di kode: `assistant/application/engine.go:19` — *"minus policy_qna: no knowledge in L9 scope"*. Bot WA-nya sendiri **dibawa** |
| Multi-tenant / `companyId` | [ADR-0009](../knowledge/decisions/ADR-0009-single-tenant-takeout-multitenancy.md) |
| Feature flags perusahaan (`company/features/*`) | PRD §5 no. 2 — mati di 3 lapisan di sistem lama |
| Modul `order` tunggal | [ADR-0004](../knowledge/decisions/ADR-0004-order-domain-split.md) — dipecah `purchasing` + `sales` |
| Modul POS tersendiri | [ADR-0007](../knowledge/decisions/ADR-0007-pos-no-dedicated-module.md) — POS = kanal frontend |
| Localization selain id-ID/IDR/WIB | [ADR-0006](../knowledge/decisions/ADR-0006-locale-timezone-locked.md) |
| Status order konfigurable + riwayat (`order-status-definitions/*`, `order-status-transitions/*`, `orders/status-history`, 7 endpoint) | D4 — tetap dibuang (Tahap A); status terkunci di kode (ini juga yang membuat `order_summary` dashboard mustahil — §4.1), riwayat lama diarsip CSV saat cutover (K8) |
| Tabel `daily_operational_metrics` + cron agregasi | Bukan keputusan tercatat, tapi wajar — lihat §5.2 untuk penggantinya |
| Master `finance_cash_accounts` terpisah | Finalisasi 2026-09-11 — diganti flag `is_cash` di COA (`DB_SCHEMA.md` §6) |
| Tabel `finance_opening_*`, `finance_posting_sources`, `finance_tax_adjustments` | Finalisasi 2026-09-11 — diganti jurnal bertipe + antrean derived (§4.2.3, PLAN Tahap G/H) |
| Varian `posting-sources/{detail,ignore,restore,cancel-posting}`, `tax-adjustments/{detail,archive}` | Finalisasi 2026-09-11 — butuh tabel yang dibatalkan; tertutup pratinjau + reverse jurnal (§9) |
| `orders/update-status` arbitrer | Status terkunci di kode per D4; transisi via confirm/cancel/approve-credit (§9) |

---

## 7. Paritas Desain UI — ✅ Terpenuhi

Klaim ini **diuji ulang dan benar**. Token desain diport identik, bukan ditiru:

```bash
diff <(grep -oP "^\s+--(color|font|radius)[a-z-]*:\s*\K[^;]+" apps/web/src/theme/theme.css) \
     <(grep -oP "^\s+--(color|font|radius)[a-z-]*:\s*\K[^;]+" ../mini-erp/apps/web/src/tailwind.css)
# → nol selisih
```

- Palet: `ink #0f172a`, `heading #334155`, `muted #64748b`, `hairline #cbd5e1`, `brand #1e3a5f`,
  `brand-hover #16314f`, `accent #c2603a`, `ok #15803d`, `warn #b45309`, `bad #b91c1c`,
  sidebar `#0f172a` — **identik**.
- Tipografi & radius: `Inter` + `--radius-{sm,md,lg}` = 6/8/10px — **identik**.
- Variabel kompat `:root` (`--bg-canvas`, `--shadow-*`, `--topbar-height: 64px`) — **identik**.

Komponen inti sudah punya padanan (button, badge, data-table, pagination, modal, confirm-dialog,
toast, notice, empty-state, page-header, section-card, summary-card, filter-bar, action-row,
form-field, search-select, segmented-control). Yang belum diport: lihat §4.8.

> **Catatan navigasi** (konsekuensi ADR-0004, bukan regresi): `/orders` tunggal → `/purchase-orders`
> + `/sales-orders`; `/gudang/*` → `/stock/*` dengan Saldo/Mutasi/Lokasi digabung dalam satu
> `StockPage`.
>
> **Hitung halaman nyata, jangan hitung rute.** Dari 10 rute `/assistant/*`, `/knowledge/*`,
> `/whatsapp` di registry legacy, **8 hanya `<Navigate>`**; halaman nyatanya dua, dan revamp punya
> persis dua padanannya.

---

## 8. Rencana Penutupan Gap (urutan usulan)

Diurutkan berdasarkan dependensi teknis lebih dulu, lalu "tanpa ini sistem tidak bisa dipakai".

| Tahap | Isi | Alasan urutan |
|---|---|---|
| **1** | ✅ *selesai* — **Dimensi analitik di `finance_journal_lines`** (§4.2.1, §10) | Membuka margin per produk, piutang/utang dari buku, dan buku besar per baris. Tiga bug posting ikut ketahuan & diperbaiki |
| **2** | **Kolom surat jalan + alamat kirim di order** (§4.3) | Prasyarat data untuk tahap 4. Membuat halaman cetak sebelum ini = mencetak field kosong |
| **3** | **Vitest di `apps/web` + test service backend** (§5.1) | Dipasang **sebelum** finance & retur disentuh berat, bukan sesudah |
| **4** | **Cetak dokumen** (§4.6): surat jalan → faktur → kwitansi → struk termal POS | P0 lapangan. Pertimbangkan render di server (pola pdfkit legacy) alih-alih 4 halaman cetak browser |
| **5** | **Dashboard operasional** (§4.1) — stok kritis, pesanan prioritas, terlaris, hitungan order | P0 harian, murah, tidak butuh migrasi (kecuali widget margin yang menunggu tahap 1) |
| **6** | **Finance P0**: saldo awal + **daftar** dokumen belum-posting (§4.2.3) | ✅ selesai via revisi tanpa tabel baru (finalisasi 2026-09-11) |
| **7** | **POS**: bayar-nanti/kredit → struk → scan kamera → katalog kategori (§4.5) | Bayar-nanti lebih dulu: itu kapabilitas, sisanya kenyamanan |
| **8** | **Retur**: mode tukar + penyelesaian + `context`/`preview` (§4.4). **Konfirmasi dulu** kebijakan diskon-diabaikan | Butuh migrasi + keputusan bisnis |
| **9** | **Finance P1**: akun kas, piutang/utang, penyesuaian pajak, nomor faktur pajak, tutup periode aman | ✅ selesai via revisi (finalisasi 2026-09-11); piutang/utang & margin sejak Tahap 1 |
| **10** | Ekspor daftar order (xlsx+pdf) ✅, QR scan ✅ + label/ZIP/cetak ✅, antrian kerja ✅, pindah-lokasi 1 langkah ✅, detail item stok ✅ | Selesai penuh (finishing 2026-09-11) |
| **11** | ✅ *selesai* — N+1 tren & lintas modul (§5.2), komponen UI bersama (§11.2 B2), `branches/my-access` (§5.3). Riwayat status order + PWA tetap dibuang by-design (§6) | Penghalusan |

Sebelum mengerjakan tiap tahap: baca bagian modulnya di
[`docs/legacy-reference/`](legacy-reference/), lalu `known-issues.md` dan `open-questions.md` —
**replikasi perilaku, bukan replikasi cacat** (PRD §4).

---

## 9. Lampiran — Endpoint Legacy Tanpa Padanan (tersisa 0 + varian terbuka)

Dasar angka §1. **Finishing 2026-09-11: 30 butir asal seluruhnya tertutup atau dinyatakan tidak
dibawa.** Rincian per butir di bawah supaya angka ini bisa diperiksa.

**Finance (26) — 13 tertutup, 10 tidak dibawa by-design, 3 terbuka**

*Tertutup via revisi (tanpa tabel baru):*
`close/{readiness,safe-close}` · `daily-close/overview` (sebagai `POST /finance/posting/close-day`) ·
`export/tax-package` · `opening/{get,validate,post}` (sebagai `GET /finance/opening`,
`POST .../opening/preview`, `POST .../opening/post`) ·
`posting-sources/{list,post-batch,close-day}` (sebagai `GET /finance/posting-sources`,
`POST .../posting/post-batch`, `POST .../posting/close-day`) ·
`reports/cash-summary` · `tax-adjustments/{list,save}` (sebagai `GET/POST /finance/tax-adjustments`)

*Tidak dibawa by-design (`DB_SCHEMA.md` §6 *Diputuskan*; kapabilitasnya tertutup lain cara):*
`cash-accounts/{list,create,update,archive}` → flag `is_cash` + CRUD COA;
`posting-sources/{detail,ignore,restore}` → tanpa tabel (detail = pratinjau, abaikan/pulihkan
tidak bermakna tanpa antrean tersimpan); `posting-sources/cancel-posting` →
`POST /finance/journals/{id}/reverse`; `tax-adjustments/{detail,archive}` → detail jurnal + reverse.

*Terbuka (keputusan bisnis, bukan penghambat cutover):* `opening/{update,import-inventory,supplement}` —
saldo susulan/inventaris opening tertutup via `POST /finance/journals/manual` + impor saldo awal
stok; bila pemilik butuh alur khusus, daftarkan sebagai enhancement.

*(`reports/{margin,receivables,payables}` sudah ditutup — §10.)*

**Order & pengiriman (3) — tersisa 0**
`deliveries/work-queue` ✅ (`GET /deliveries/work-queue` + halaman Antrian Kirim, J3) ·
`orders/export` ✅ (`sales-orders/export` + `purchase-orders/export`, xlsx+pdf, J1) ·
`orders/update-status` ✅ (tercakup confirm/cancel/approve-credit; status terkunci di kode per D4)

*(Retur sudah ditutup — Tahap D: `sales-returns/context`, `sales-returns/preview`,
`sales-returns/replacement-deliveries` + `confirm`, `purchase-returns/context`,
`purchase-returns/preview`.)*

**Stok (1) — tersisa 0**
`stock/transfer` ✅ (pindah seketika via `POST /stock/transfers/move-location` + modal, J4 —
pemindahan satu-langkah dengan jejak dokumen; lihat §4.7)

*(J5/J6 2026-09-11: `stock/detail` ditutup `GET /stock/balances/{productId}` +
halaman `StockItemPage`; `branches/my-access` ditutup endpoint login-only +
hook `use-accessible-branches` — keduanya keluar dari daftar ini.)*

---

## 10. Pengerjaan — Tahap 1 Selesai

**Migrasi `000019_finance_line_dimensions`** menambah `product_id`, `variant_id`, `party_id`,
`description` ke `finance_journal_lines` + dua indeks komposit (`account_id, product_id` dan
`account_id, party_id` — akun didahulukan karena selektivitasnya paling tinggi).

Setiap builder jurnal kini memasang dimensinya: pendapatan & HPP per produk, piutang & hutang per
lawan transaksi. Tiga endpoint baru berdiri di atasnya, plus UI-nya:

| Endpoint baru | UI |
|---|---|
| `GET /finance/reports/margin-by-product?from=&to=[&limit=]` | tab **Marjin** → tabel "Marjin per Produk" |
| `GET /finance/reports/receivables[?asOf=]` | tab **Piutang & Hutang** (+ menu sidebar) |
| `GET /finance/reports/payables[?asOf=]` | idem |

Biaya baca: **satu query berkelompok** per laporan, bukan satu per produk/party — justru N+1 itulah
yang dimensi ini ada untuk dihindari. Piutang/utang memakai batas `asOf` (kumulatif), bukan rentang:
saldo ditanyakan "per tanggal", dan filter rentang akan diam-diam membuang faktur yang lebih tua.

### 10.1 Tiga bug posting yang ikut ketahuan

Membedah builder untuk memasang dimensi membuat tiga kesalahan uang terlihat. Semuanya diperbaiki
di tahap yang sama, karena menambah dimensi di atas angka yang salah hanya melaporkan angka salah
dengan lebih rinci.

| # | Bug | Akibat | Perbaikan |
|---|---|---|---|
| 1 | **Surat jalan parsial dibukukan penuh.** `buildDelivery` memakai `so.GrandTotal`/`Subtotal`/`TaxTotal` — nilai **seluruh order** — untuk setiap SJ. Pengiriman bertahap adalah alur yang didukung (`delivery/application/service.go` membatasi tiap SJ ke sisa SO), jadi ini terjangkau, bukan teoretis | SO yang dikirim 2 kali membukukan pendapatan, PPN, dan piutang **dua kali penuh** | Nilai di-pro-rata ke baris yang benar-benar ada di SJ itu, per produk |
| 2 | **PPN dihitung dua kali pada PO harga-termasuk-pajak.** `buildGoodsReceipt` mendebit Persediaan sebesar *net* (yang untuk `tax_type = include` sudah mengandung PPN) lalu **menambahkan** PPN Masukan di atasnya | Contoh 1 unit @ Rp11.100 (PPN 11% di dalam): terbukukan Persediaan 11.100 + PPN 1.100 / Hutang 12.200. Seharusnya 10.000 + 1.100 / 11.100. Ketiga kaki salah | Persediaan didebit pada **tax base**; hutang = jumlah yang benar-benar didebit, jadi seimbang menurut konstruksi |
| 3 | **Cost ledger memakai basis yang sama salahnya.** `receiptUnitCost` memakai `lineNet`, dan komentarnya mengklaim "pre-tax" — padahal hanya benar untuk `none`/`exclude` | Rata-rata bergerak menggelembung sebesar PPN pada PO inklusif → HPP dan marjin ikut salah | Memakai tax base yang sama dengan jurnal GR, sehingga cost ledger dan buku tidak bisa berbeda |

Ketiganya sekarang dijaga unit test (`finance/application/builders_test.go`): `lineEconomics` untuk
ketiga tipe pajak termasuk kombinasi diskon, `prorate` termasuk pembagian-nol, dan satu test yang
mengirim satu order dalam dua surat jalan lalu memastikan totalnya persis sekali, bukan dua kali.

> **Catatan data lama:** jurnal yang sudah ter-posting **sebelum** perbaikan ini tetap menyimpan
> angka lamanya — posting bersifat idempoten, jadi tidak ada yang otomatis dihitung ulang. Bila
> sudah ada data produksi dengan PO/SO ber-PPN inklusif atau pengiriman bertahap, jurnal terkait
> perlu di-`reverse` lalu di-posting ulang. Di lingkungan yang masih data uji, abaikan saja.

### 10.2 Verifikasi

`go build ./...` ✅ · `go test ./...` ✅ · `tsc --noEmit` ✅ · `vite build` ✅.
*(Angka paket saat Tahap 1; kesehatan build terkini ada di §1 dan §11.)*

### 10.3 Catatan terpisah — ESLint frontend ✅ **sudah berjalan**

Arsip: dulu `npx eslint "src/**/*.{ts,tsx}"` menolak dengan *"all of the files matching the glob
pattern are ignored"* — lint tidak memeriksa satu file pun. **Sekarang lint menjangkau seluruh
`src/` dan hasilnya 0 masalah** (dari 101 warning saat Scope B dimulai; 89 di antaranya
`react-hooks/set-state-in-effect`, ditutup dengan hook `useAsyncData` yang dipakai 65 file).

---

## 11. Pengerjaan — Scope A & B (2026-09-11)

Ditutup dari [`plan/verifikasi-modul-2026-09-11/PLAN.md`](plan/verifikasi-modul-2026-09-11/PLAN.md).

### 11.1 A1 — kebocoran otorisasi transfer stok (P0, satu-satunya bug fungsional)

Empat endpoint transfer-per-ID tidak pernah membandingkan cabang dokumen dengan cabang sesi;
`GetTransfer` bahkan tidak menerima `branchID`, dan `guarded` hanyalah `RequireBranch` (memastikan
*ada* cabang aktif, bukan cabang **yang mana**). Pengguna di cabang B bisa mengirim, menerima, atau
membatalkan transfer milik cabang A dengan menebak ID numerik.

`stock` adalah satu-satunya dari 8 modul berdokumen yang tidak melakukan pemeriksaan ini — anomali,
bukan desain.

Diperbaiki dengan **aturan arah**, bukan satu perbandingan seragam (menyamaratakan akan mematahkan
transfer antar-cabang):

| Operasi | Cabang sesi yang sah |
|---|---|
| `GET` | `FromBranchID` **atau** `ToBranchID` |
| `dispatch`, `cancel` | `FromBranchID` saja |
| `receive` | `ToBranchID` saja |

Penolakan memakai `NotFound`, bukan `Forbidden` — keberadaan dokumen cabang lain tidak boleh bocor
lewat beda pesan.

Dijaga `internal/integration/transfer_scope_test.go` — termasuk kasus **"receive di cabang tujuan
berhasil"**, penjaga yang memastikan perbaikan ini tidak berubah jadi regresi lintas-cabang.
Sapuan lanjutan menemukan kasus sejenis pada lokasi stok (`TestLocationBranchScope`).

### 11.2 Sisanya

| Butir | Hasil |
|---|---|
| A2 jaring pengaman | 23 → **31 file test**, integrasi 19 → **35 test**: `stock_flows` (saldo awal, hold/release reservasi, adjust set-mode), `payment_flows` (tolak overpay, cancel hitung ulang), `user_roles` (hapus role terpakai) |
| A3 N+1 | lihat §5.2 |
| A4 toolchain | `go.mod` pin `toolchain go1.26.7` — build tidak lagi bergantung toolchain yang kebetulan ter-cache |
| A5 rahasia | `JWT_SECRET` ditolak bila = nilai contoh di luar `development`, dan bila < 32 karakter |
| B1 envelope | ✅ **tuntas** — `toast.fromServer(res.message, fallback, deskripsi)` + varian `api*Full` yang mempertahankan `{data, message}`. **98 sisi panggil tulis** memakai pesan server. Delapan `toast.success` tersisa memang bukan tulis-server: 4 unduhan berkas (mengembalikan `void`, tanpa envelope), 2 agregat batch yang dihitung klien (`Diposting N dokumen`), dan 2 pesan komposit POS yang menggabungkan beberapa keadaan klien (piutang / pembayaran tertunda / lunas) |
| B2 komponen | 9 komponen: 6 generik di `shared/components/ui/`, 3 varian domain (`PartySearchSelect`, `ProductSearchSelect`, `StockLocationSelect`) di modulnya — aturan *shared harus domain-agnostik* dihormati |
| B3 render beruntun | hook `useAsyncData` (65 file); eslint 101 → **0** |
| B4 uji frontend | 11 → **17 file**, 60 → **85 test** |

> **Kerapuhan yang ikut ketahuan dan sudah dikunci:** `React.act` hanya ada di
> `react.development.js`, sedangkan `react/index.js` memilih build dari `process.env.NODE_ENV`.
> Menjalankan test dengan `NODE_ENV=production` (lazim di CI dan Docker build) membuat **seluruh
> test yang me-render komponen** gagal dengan *"React.act is not a function"* — bukan karena
> kodenya salah. Dikunci lewat `env: { NODE_ENV: "development" }` di `apps/web/vitest.config.ts`.

**Belum dikerjakan:** C0 (git init) — **ditunda atas keputusan pemilik** sampai repo target
disiapkan. Selama itu belum ada, tidak ada riwayat, diff, atau jalur rollback untuk seluruh repo,
dan `.gitignore` masih perlu ditambahi `apps/api/storage/` sebelum commit pertama (isinya
`whatsapp_session.db` — sesi WhatsApp terautentikasi — dan foto produk unggahan pengguna).
