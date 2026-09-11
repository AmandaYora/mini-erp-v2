# Feature Inventory — Modul 19 Reporting

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diturunkan dari
`reporting.controller.ts` (28 baris, dibaca penuh), `reporting.service.ts` (70 baris,
penuh), `metrics-job.service.ts` (123 baris, penuh), `daily-operational-metric.entity.ts`
(penuh), `reporting.module.ts` (penuh), kedua spec (347 baris, penuh), migrasi
`001_baseline.sql` §4.8, E2E `10-observability-permission-hardening.spec.ts`,
`role-access-config.ts` + test-nya, dan `permission-code.ts`. Modul terkecil di repo:
**2 endpoint, 1 tabel, 0 halaman web.**

Berkas terkait: [user-flows.md](user-flows.md) · [business-rules.md](business-rules.md) ·
[reports-list.md](reports-list.md) · [test-cases.md](test-cases.md) ·
[algorithms-legacy.md](algorithms-legacy.md) · [data-model-legacy.md](data-model-legacy.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 2 — `POST reporting/orders` (`reporting.controller.ts:15–20`) + `POST reporting/stock` (`22–27`); keduanya `@Post` + `@HttpCode(200)` + `@RequirePermission('reporting.view')` + body `{ data }` (`data ?? {}` bila kosong!); `orders` menerima `{date_from, date_to, status_group, order_kind, page, limit}`, `stock` menerima `{critical_only}` |
| Permission | `reporting.view` (`role-access-config.ts:111–114`, grup `Laporan Operasional`, label `Lihat Laporan`) — default untuk superadmin/owner/admin; **tidak** untuk staff; **system-only** di UI Role & Akses (tidak bisa di-assign ke custom role; dijaga `role-access-config.test.ts:16–17` bersama `whatsapp.simulate`!) |
| Guard | `JwtAuthGuard` + `PermissionGuard` + `BranchGuard` (`reporting.controller.ts:12`; cabang aktif wajib — tanpa cabang: `403 Branch aktif belum dipilih`; `idActiveBranch!` dari sesi, tak ada param cabang!) |
| Halaman web | **Tidak ada** — tanpa folder `apps/web/src/modules/reporting` (daftar modul web: assistant/audit-log/auth/business-party/company/core/dashboard/finance/… — tanpa reporting!), tanpa rute `/reporting` di `module-registry.tsx` (hanya `/dashboard` 200–207!), tanpa slice/hook/halaman; satu-satunya konsumen adalah E2E 10 + pemanggil API langsung |
| Tabel milik sendiri | 1 — `daily_operational_metrics` (11 kolom, `001_baseline.sql:407–422`; ditulis cron `EVERY_DAY_AT_1AM` via `INSERT…ON DUPLICATE KEY UPDATE`, **tidak dibaca siapa pun** — grep nol SELECT!) |
| Tabel yang dibaca | `orders` (+ `order_status_definitions`, pihak terkait), `inventory_balances` (+ `products`), `branches` (cabang aktif cron!) |
| Audit | Tidak menulis audit (modul exempt, seperti assistant/whatsapp/tools/auth/audit-log) |
| Penomoran dokumen | Tidak ada |
| Aksi yang TIDAK ada | Baca metrik harian (tanpa endpoint!), tulis/hapus laporan, ekspor file, filter cabang (selalu cabang sesi!), halaman UI |

**Karakter modul.** Ini API operasional mentah (bukan laporan keuangan — itu modul 17):
daftar order + total, dan daftar saldo stok per cabang. Tabel agregat hariannya adalah
tulisan-tanpa-pembaca (migrasi sendiri berkomentar "tidak dipakai di MVP").

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Laporan Order (`reporting/orders`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-01.1 | Filter | `date_from` (inklusif `>=`), `date_to` (inklusif `<=`), `status_group` (pending/active/completed/cancelled — via definisi status), `order_kind` (sales/purchase/...) — semua opsional, boleh kosong (= semua) |
| F-01.2 | Isi baris | Order lengkap + relasi `currentStatus` + `relatedParty` (eager-join; konsumen E2E membaca `id`/`id_order`/`orderNumber`/`order_number`) |
| F-01.3 | Paginasi | `page` default 1, `limit` default 20, **cap 100** (lebih → diam-diam jadi 100, tanpa error!) |
| F-01.4 | Total | `total_sales` = SUM(`total_amount`) **seluruh** baris terfilter (bukan cuma halaman ini!) — lihat KI-123 |
| F-01.5 | Urutan & arsip | Terbaru dulu (`orderDate` DESC); yang terarsip selalu dikecualikan |
| F-01.6 | Respons | `{ items, total_sales, meta: { page, limit, total } }` |

### F-02 — Laporan Stok (`reporting/stock`)

| # | Sub-fitur | Detail |
|---|---|---|
| F-02.1 | Mode semua | Tanpa filter → semua saldo produk terlacak (`stock_tracked = 1`) yang tidak diarsip, urut nama produk A–Z |
| F-02.2 | Mode kritis | `critical_only: true` → hanya baris `available_qty <= min_stock_qty` (dengan `min_stock_qty` terisi) — **per baris lokasi**, bukan per produk! (beda definisi dengan dashboard — lihat KI-124) |
| F-02.3 | Respons | **Array mentah** (tanpa `items`/`meta`/paginasi — tidak seperti F-01!) |
| F-02.4 | Isi baris | Saldo + relasi `product` (E2E membaca `idProduct`/`product.id`) |

### F-03 — Agregasi Metrik Harian (cron, tanpa endpoint!)

| # | Sub-fitur | Detail |
|---|---|---|
| F-03.1 | Jadwal | Tiap hari 01:00 **waktu server**; mengagregat **kemarin** (`toISOString` = tanggal UTC! — lihat KI-125) |
| F-03.2 | Lingkup | Semua cabang `status = 'active'`; tanpa cabang aktif = tidak menulis apa pun (bukan error!) |
| F-03.3 | Metrik order | Hitung per `status_group` (pending/active/completed/cancelled + total) + omzet **khusus sales** (`order_kind = 'sales'`, jenis lain dihitung tapi Rp 0) |
| F-03.4 | Metrik stok | Satu angka: jumlah baris saldo kritis (rumus sama dengan F-02.2) |
| F-03.5 | Tulis | Upsert per (cabang, tanggal) — tanggal sama ditulis ulang menimpa (idempoten!) |
| F-03.6 | Tahan-gagal | Satu cabang gagal → log error, cabang lain tetap jalan; agregasi tak pernah melempar |
| F-03.7 | Catch-up manual | `aggregateForDate('YYYY-MM-DD')` publik untuk skrip sekali-jalan (tanpa endpoint — via kode!) |

---

## 3. Edge Case (12)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Tanpa cabang aktif | `403 Branch aktif belum dipilih` (kedua endpoint) |
| E-02 | Tanpa `reporting.view` (staff/custom) | `403` (E2E membuktikan 403 untuk finance; pola sama) |
| E-03 | Tanpa token | `401` (pola global) |
| E-04 | `limit: 999` | Diam-diam jadi 100 (tanpa error, `meta.limit = 100`) |
| E-05 | Filter tanggal kosong | Semua order cabang (arsip tetap dikecualikan) |
| E-06 | Tidak ada yang cocok | `items: []`, `total: 0`, `total_sales: 0` (nol, bukan null!) |
| E-07 | `critical_only: false`/absen | Mode semua (tanpa filter kritis) |
| E-08 | Produk `min_stock_qty` NULL + `critical_only` | Dikecualikan (tanpa ambang = tak-pernah-kritis!) |
| E-09 | Produk tak-terlacak/arsip | Selalu dikecualikan (kedua mode) |
| E-10 | Cabang gagal di cron | Log `Failed to aggregate branch {id} for {date}: ...`, lanjut cabang berikut |
| E-11 | Cron tanggal-sama dua kali | Upsert menimpa (tanpa duplikat — kunci unik cabang+tanggal!) |
| E-12 | Order tanpa status-definisi (cron) | Tak-terhitung (INNER JOIN!) — bandingkan F-01 yang LEFT JOIN (tetap tampil!) |

---

## 4. Katalog Pesan (teks apa adanya)

- `Branch aktif belum dipilih` (403, tanpa cabang — dari `BranchGuard` bersama).
- `Failed to aggregate branch {id} for {date}: {pesan}` (log server, bukan ke user!).
- `Aggregating daily_operational_metrics for {date}` / `Done aggregating {n} branches for {date}` (log info).
- Pesan 401/403 standar global (belum dibaca di modul ini — [PERLU KONFIRMASI] teks persis envelope error).
- **Tidak ada** pesan validasi milik sendiri (filter salah ketik = diabaikan diam-diam, mis. `status_group` tak dikenal → hasil kosong, bukan error!).

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Endpoint baca `daily_operational_metrics` | Tak ada controller-nya; grep: hanya INSERT di job + definisi tabel |
| NF-02 | Pembaca tabel di mana pun (termasuk dashboard!) | Grep seluruh `apps/api/src`: nol SELECT di luar modul reporting |
| NF-03 | Halaman/rute/slice web | Tanpa folder modul; registry tanpa `reporting`; satu-satunya rujukan web = config izin + test-nya |
| NF-04 | Assign `reporting.view` ke custom role | Test eksplisit: system-only bersama `whatsapp.simulate` |
| NF-05 | Ekspor file (Excel/PDF) | Respons JSON saja (ekspor milik order/finance!) |
| NF-06 | Audit log untuk baca maupun cron | Modul exempt (knowledge §4) |
| NF-07 | Filter/pilih cabang di request | Selalu `idActiveBranch` sesi (aturan arsitektur!) |
| NF-08 | Injeksi `metricRepo` terpakai | Di-inject di job tapi tak-pernah-dipanggil (query mentah via `dataSource`!) |
