# Feature Inventory — Modul 20 Audit Log

**Kelompok A — kontrak yang tidak boleh berubah di sistem baru.** Diturunkan dari
`audit-log.controller.ts` (20 baris, penuh), `audit-log.service.ts` (60 baris, penuh),
`audit-log.entity.ts` (penuh), `audit-log.module.ts`, `audit-log.service.spec.ts`
(201 baris, penuh), `audit-log-pages.tsx` (263 baris, penuh),
`use-audit-log-module.ts` + test-nya, `audit-log.slice.ts`,
`toAuditLog` (`org.adapter.ts`), `formatDateTime` (`utils.ts`), registry
(`/audit-logs`), `role-access-config.ts`, DDL `001_baseline.sql`, E2E 10 + 08 + 01,
`adapters.test.ts` (2 test `toAuditLog`), `api.ts` (amplop `{ data }`), dan
**enumerasi 102 situs `auditLog.log` di 28 berkas service produksi** (daftar lengkap §2;
`.spec.ts` dikecualikan; plus 1 kunci skrip SQL `rebuild-cost-ledger.ts`).

Berkas terkait: [ui-ux-spec.md](ui-ux-spec.md) · [user-flows.md](user-flows.md) ·
[business-rules.md](business-rules.md) · [reports-list.md](reports-list.md) ·
[numbering-sequence.md](numbering-sequence.md) · [test-cases.md](test-cases.md)

---

## 1. Ringkasan Cakupan

| Aspek | Nilai |
|---|---|
| Endpoint | 1 — `audit-logs/list` (`@Post` + `@HttpCode(200)` + `{ data }`; prefix jamak `audit-logs`!) |
| Permission | `audit_log.view` (grup Role & Akses "Riwayat Aktivitas" → "Lihat") |
| Guard | JWT + permission saja — **tanpa BranchGuard** (lingkup perusahaan, bukan cabang!) |
| Halaman | 1 — `/audit-logs` "Riwayat Aktivitas" (menu grup "Pantauan") |
| Produsen | 28 berkas service menulis via `AuditLogService.log` (10 modul, §2); modul exempt (tak menulis): assistant, whatsapp, tools, auth, reporting, dashboard, storage, audit-log sendiri |
| Situs pemanggilan | 102 — semuanya `await` telanjang tanpa try/catch (nol fire-and-forget; verifikasi skrip atas seluruh modul) |
| Baris | Append-only (tanpa endpoint ubah/hapus; service tanpa update/delete!) |
| Respons list | `{ items, meta: { page, limit, total } }` — page default 1, limit default 20, **cap 100** |
| Filter API | `id_branch`, `entity_type` (persis `=`), `action_key` (**LIKE `%...%`** substring!) — semua opsional |
| Urutan | Terbaru dulu (`happenedAt` DESC) |
| Penomoran | Tidak ada |
| Aksi yang TIDAK ada | Ubah/hapus baris; filter tanggal; filter aktor; lihat isi before/after di UI; ekspor; filter apa pun di halaman (API bisa, UI tak memakai!) |

---

## 2. Daftar Fitur & Sub-Fitur

### F-01 — Tulis jejak (`AuditLogService.log`, tanpa endpoint — via kode!)

Setiap write bisnis memanggil `log({ idCompany, idBranch|null, actorType, idActor?,
actionKey, entityType, idEntity?, before?, after?, metadata? })`; opsional kosong → NULL;
`happenedAt` = `new Date()` server. `actorType` = `'user'` di 102/102 situs produksi
(cek! tak ada penulis `system` via service — satu-satunya baris `system` berasal dari
skrip SQL `rebuild-cost-ledger.ts:417-418`, bukan `log()`).
Gagal tulis = error menular ke pemanggil tanpa try/catch di situs mana pun
(akibatnya beda per pola — lihat business-rules BR-18/BR-19!).

**Kunci yang dipakai backend — HASIL ENUMERASI KODE (bukan perkiraan): 102 situs
pemanggilan di 28 berkas `*.service.ts` (`.spec.ts` dikecualikan) = 90 `actionKey`
statis unik + 4 templat dinamis `` `${partyType}.create|update|archive|restore` ``
(`business-party.service.ts:107,166,176,196`; `PartyType = 'customer'|'supplier'|'partner'`
— `packages/shared-types/src/enums.ts:10` — sehingga mengembang jadi 12 kunci runtime)
+ 1 kunci skrip SQL (`finance.cost_ledger.restatement`, `rebuild-cost-ledger.ts:417-418`,
actor `system`, di luar `AuditLogService`) = 103 nilai `action_key` runtime berbeda.**

### F-01a — Tabel pemanggil per modul (statis; baris = lokasi `actionKey:`)

| Modul | Berkas | `actionKey` (baris) | `entityType` |
|---|---|---|---|
| Order | `order/order.service.ts` | `order.create` (:517), `order.update` (:705), `order.status_change` (:787), `order.archive` (:797), `order.approve_credit` (:826) | `order` |
| Order | `order/delivery.service.ts` | `payment.create` (:575, :937), `order.approve_credit` (:593, :944), `order.goods_delivered` (:613), `delivery.create` (:770), `delivery.confirm` (:954), `delivery.archive` (:1394) | `payment` / `order` / `delivery_note` |
| Order | `order/goods-receipt.service.ts` | `payment.create` (:379), `order.approve_credit` (:397), `order.goods_received` (:417) | `payment` / `order` |
| Order | `order/delivery-proof.service.ts` | `delivery.proof_uploaded` (:111) | `media_file` |
| Order | `order/sales-return.service.ts` | `sales_return.create` (:376), `sales_return.dispatch_replacement` (:526), `sales_return.confirm_replacement` (:580) | `sales_return` / `delivery_note` / `delivery_note` |
| Order | `order/purchase-return.service.ts` | `purchase_return.create` (:299) | `purchase_return` |
| Payment | `payment/payment.service.ts` | `payment.create` (:294, :368), `payment.archive` (:426), `payment.proof_uploaded` (:469) | `payment` |
| Product | `product/product.service.ts` | `product_category.create` (:143), `.update` (:157), `.archive` (:167), `product.create` (:378), `product.update` (:427), `product.archive` (:912), `product_media.upload` (:1010), `.archive` (:1059), `.set_primary` (:1102) | `product_category` / `product` / `media_file` |
| Product | `product/product-import.service.ts` | `product.bulk_import` (:256) | `product` |
| Stock | `stock/stock.service.ts` | `stock.adjust` (:1221) | `inventory_balance` |
| Stock | `stock/stock-location.service.ts` | `stock.location.create` (:189 sub-lokasi-dalam-transaksi, :218 jalur biasa), `.update` (:254), `.archive` (:281) | `stock_location` |
| Stock | `stock/stock-damaged.service.ts` | `stock.damaged.move_in` (:185), `.restore` (:260), `.write_off` (:312) | `inventory_balance` |
| Stock | `stock/stock-transfer.service.ts` | `stock.transfer_document.create` (:180), `.dispatch` (:251), `.receive` (:329), `.cancel` (:357), `stock.transfer` (:460) | `stock_transfer` / `inventory_balance` |
| Stock | `stock/stock-opening.service.ts` | `stock.opening_import.commit` (:379) | `stock_opening_import` |
| Finance | `finance/finance.service.ts` | `finance.account.create` (:147), `.update` (:180), `.archive` (:217), `finance.account_mapping.update` (:302), `finance.cash_account.create` (:351), `.update` (:378), `.archive` (:398), `finance.period.create` (:446), `.reopen` (:469), `finance.opening.update` (:566), `.post` (:819), `.supplement` (:1064) | `finance_account` / `finance_account_mapping` / `finance_cash_account` / `finance_period` / `finance_opening_balance` |
| Finance | `finance/finance-posting.service.ts` | `finance.posting_source.sync` (:164), `.ignore` (:644), `.restore` (:682), `.cancel_posting` (:745), `finance.journal.post` (:857), `finance.posting.close_day` (:949), `finance.journal.reverse` (:1152) | `finance_posting_source` / `finance_journal_entry` |
| Finance | `finance/finance-close.service.ts` | `finance.period.safe_close` (:230) | `finance_period` |
| Finance | `finance/finance-reporting.service.ts` | `finance.tax_period.close` (:480), `.reopen` (:502) | `finance_tax_period` |
| Finance | `finance/finance-tax-adjustment.service.ts` | `finance_tax_adjustment.update` (:131), `.create` (:181), `.archive` (:203) | `finance_tax_adjustment` |
| Finance | `finance/business-expense.service.ts` | `business_expense.create` (:340), `.cancel` (:450) | `business_expense` |
| User | `user/user.service.ts` | `user.create` (:118), `.update` (:179), `.update_status` (:215), `.change_password` (:268) | `user` |
| User | `user/role.service.ts` | `role.create` (:85), `.update` (:119), `.delete` (:152), `.permissions.update` (:203) | `role` |
| Business-party | `business-party/business-party.service.ts` | `` `${partyType}.create` `` (:107), `.update` (:166), `.archive` (:176), `.restore` (:196) → ×3 nilai party = 12 kunci | `business_party` |
| Business-party | `business-party/customer-address.service.ts` | `customer_address.create` (:83), `.update` (:117), `.archive` (:152) | `business_party_address` |
| Business-party | `business-party/member-type.service.ts` | `member_type.create` (:80), `.update` (:114), `.archive` (:136) | `member_type` |
| Company | `company/company.service.ts` | `company.profile.update` (:70), `company.settings.update` (:129, bersyarat `if (session)`), `company.feature.update` (:173, bersyarat `if (session)`) | `company` / `company_settings` / `company_feature` |
| Branch | `branch/branch.service.ts` | `branch.create` (:157), `branch.update` (:236) | `branch` |
| Knowledge | `knowledge/knowledge.service.ts` | `knowledge.create` (:121), `knowledge.archive` (:173), `knowledge.update` (:297) | `knowledge_document` |

Kunci ganda (satu kunci, banyak situs): `payment.create` 5 situs (2 payment + 2
deliver-goods + 1 receive-goods), `order.approve_credit` 4 situs (order + deliver ×2 +
receive), `stock.location.create` 2 situs (cabang sub-lokasi vs biasa).

### F-01b — Matriks modul × diaudit/tidak

| Modul API (18) | Menulis audit? | Bukti |
|---|---|---|
| order | ✅ 6 berkas, 21 situs | tabel F-01a |
| payment | ✅ 1 berkas, 4 situs | tabel F-01a |
| product | ✅ 2 berkas, 10 situs | tabel F-01a |
| stock | ✅ 5 berkas, 14 situs | tabel F-01a |
| finance | ✅ 6 berkas, 27 situs | tabel F-01a |
| user | ✅ 2 berkas, 8 situs | tabel F-01a |
| business-party | ✅ 3 berkas, 10 situs (4 templat + 6 statis) | tabel F-01a |
| company | ✅ 1 berkas, 3 situs (2 bersyarat sesi!) | `company.service.ts:124,168` |
| branch | ✅ 1 berkas, 2 situs | tabel F-01a |
| knowledge | ✅ 1 berkas, 3 situs | tabel F-01a |
| auth | ❌ Tak pernah memanggil | grep `auditLog\|AuditLog\|actionKey` di `auth/` → nol berkas |
| assistant | ❌ | grep → nol berkas |
| whatsapp | ❌ | grep → nol berkas |
| tools | ❌ | grep → nol berkas |
| reporting | ❌ | grep → nol berkas |
| dashboard | ❌ | grep → nol berkas |
| storage | ❌ | grep → nol berkas |
| audit-log | ❌ (sengaja — hindari rekursi) | modul hanya berisi controller+service+entity |

### F-01c — Verifikasi KI-149 per baris (status kode saat ini; KI-149 sendiri tak diubah!)

6 dari 7 baris KI-149 ternyata **sudah teraudit** di kode saat ini — hanya unduhan
berkas pajak yang benar-benar tanpa jejak:

| Baris KI-149 | Status kode | Lokasi bukti |
|---|---|---|
| Memposting sumber (satu/massal/tutup hari) | ✅ TERAUDIT (`finance.journal.post` per source + `finance.posting.close_day`; `postBatch` tercakup transitif per item) | `finance-posting.service.ts:852-861, 944-952, 866-888` |
| Membalik jurnal | ✅ TERAUDIT (`finance.journal.reverse`) | `:1147-1160` |
| Membatalkan posting | ✅ TERAUDIT (`finance.posting_source.cancel_posting`, di dalamnya memanggil reversal :720-728) | `:740-759` |
| Mengabaikan/memulihkan sumber | ✅ TERAUDIT (`.ignore` + `.restore`) | `:639-650, :677-688` |
| Menarik transaksi (sinkronisasi) | ✅ TERAUDIT (`finance.posting_source.sync`) | `:159-168` |
| Mengunduh paket berkas pajak (Layer 1 & 2) | ❌ BENAR TAK TERAUDIT — `FinanceExportService` tanpa injeksi `AuditLogService` | `finance-export.service.ts:45-50` (ctor hanya reporting+taxAdjustment); endpoint `finance-close.controller.ts:38-46` |
| Menyimpan/mengarsipkan penyesuaian pajak | ✅ TERAUDIT (`.create/.update/.archive`) | `finance-tax-adjustment.service.ts:126-137,176-186,198-204` |

### F-01d — Pola pemanggilan (fakta kode; tafsir atomisitas lihat BR-19!)

- 102/102 situs = `await this.auditLog.log({...})` telanjang — nol fire-and-forget
  (`void`/tanpa-await) dan nol try/catch per situs (verifikasi skrip atas seluruh modul).
- Pola DALAM-transaksi (audit leksikal di dalam callback `transaction`, contoh terverifikasi:
  `order.service.ts:506-519` (`em.*` + audit `:517` + penutup `:519`); `payment.service.ts:287-301`;
  `stock.service.ts:1171-1238` (audit `:1216-1235`); `delivery.service.ts:570-625` tiga audit berurutan;
  `goods-receipt.service.ts:373-424`; `sales-return.service.ts:371-389`;
  `purchase-return.service.ts:294-309`; `stock-transfer.service.ts:175-187`;
  `stock-damaged.service.ts:180-192`; `stock-location.service.ts:184-196`).
- Pola SETELAH-transaksi (audit sesudah komit, contoh terverifikasi:
  `branch.service.ts:136-162`; `user.service.ts:116-118,177-179,208-221,261-273`;
  `role.service.ts:78-89`; `finance-posting.service.ts:623-650,655-688,703-738+740-759,798-862,897-954,1139-1161`;
  `finance-close.service.ts:211-239`; `business-expense.service.ts:333-349,358-449`;
  `delivery-proof.service.ts:104-122`; tanpa-transaksi: `finance.service.ts:122-154`,
  `payment.service.ts:434-479` upload-bukti).
- `before`/`after`: create = `after` saja (ringkas: nomor/nama/jumlah); update =
  `before` + `after` (snapshot); archive = minimal/tanpa payload
  (mis. `order.archive` :797 tanpa before/after; `payment.archive` :421-430 hanya
  `{ allocationCount }`); finance-posting = status before→after + id jurnal;
  `stock.adjust` = qty before→after + metadata approval (`stock.service.ts:1216-1235`).
- `idBranch`: master lintas-cabang memakai `null` (company/user/role/product/knowledge/
  finance-master); operasional memakai cabang sesi; `branch.create/update` memakai
  `saved.id` (cabang yang baru dibuat/diubah itu sendiri — `branch.service.ts:152-161,231-239`).
- Pengecualian: `company.settings.update`/`company.feature.update` hanya menulis audit
  `if (session)` (`company.service.ts:123-141,167-179`) — panggilan internal/seed tanpa
  sesi berjalan diam-diam tanpa jejak (cek! pemanggil tanpa-sesi di luar spec).

### F-02 — Daftar jejak (`audit-logs/list`)

Filter cabang/entitas/aksi + paginasi (lihat §1); dipakai halaman (tanpa filter,
25/halaman) + slice prefetch (100 pertama) + E2E 10 (3 asersi `action_key`+`entity_type`)
+ E2E 08 (`stock.adjust`+`inventory_balance`).

### F-03 — Halaman Riwayat Aktivitas (`/audit-logs`)

Header (breadcrumb `Dashboard > Riwayat Aktivitas`, judul, deskripsi
`Jejak aktivitas penting yang tercatat dari modul operasional dan konfigurasi.`)
+ kartu `Aktivitas Terbaru` (deskripsi `Memuat…` / `{total} aktivitas tercatat.`)
+ tabel 5 kolom (Waktu/Pengguna/Aktivitas/Data Terkait/Keterangan) + paginasi 25 +
empty-state (`Belum ada aktivitas` + `Aktivitas penting akan muncul setelah ada
perubahan data di modul operasional atau pengaturan.`). Tanpa login-data → render null.
Gagal muat → tabel kosong diam-diam (catch → `[]`, tanpa pesan error!).

### F-04 — Resolusi nama & label (adapter + peta label)

- Aktor: cocokkan `idActor` ke daftar user → `displayTitle`; tak cocok → `User #{id}`;
  tanpa aktor → `System`.
- Cabang: cocokkan ke daftar cabang → nama; tak cocok → `Branch #{id}`.
- Aksi: 90 label Indonesia (`ACTION_LABELS`, `audit-log-pages.tsx:12-103` — dihitung
  programatik, bukan perkiraan); tak dikenal → format fallback
  (`kunci.lain` → `Kunci - Lain`).
- Entitas: 29 label (`ENTITY_LABELS`, `:105-135`); tak dikenal → Kapitalisasi kata.
- Waktu: `formatDateTime` = `id-ID`, `dd MMM yyyy HH:mm` (tanpa detik!); kosong → `-`.
- Keterangan: **selalu** `Aktivitas tercatat otomatis.` (adapter mengisi description =
  actionKey sehingga fallback selalu menang — before/after/metadata tak pernah tampil!
  Lihat KI-128).

---

## 3. Edge Case (14)

| # | Kondisi | Perilaku |
|---|---|---|
| E-01 | Tanpa `audit_log.view` | 403 (pola global) |
| E-02 | Tanpa token | 401 |
| E-03 | Tanpa cabang aktif | **Tetap bisa** (tanpa BranchGuard — lingkup perusahaan!) |
| E-04 | `limit: 500` | Cap 100 diam-diam |
| E-05 | Filter kosong | Semua baris perusahaan, terbaru dulu |
| E-06 | `action_key: 'order'` | Substring LIKE — cocok `order.create`, `order.update`, ... (bukan persis!) |
| E-07 | Kunci tak dikenal di UI | Fallback `Kunci - Lain` (tak pernah blank!) |
| E-08 | Entitas tak dikenal di UI | Fallback kapitalisasi |
| E-09 | Aktor bukan user / user terhapus | `System` / `User #{id}` |
| E-10 | Gagal muat halaman | Tabel kosong diam-diam (tanpa error!) |
| E-11 | Slice gagal (`users/list`/`audit-logs/list`) | Catch per-request → `[]`; nama aktor terdegradasi diam-diam |
| E-12 | >100 user perusahaan | `users/list` limit 100 → aktor di luar itu jadi `User #{id}` (KI-131!) |
| E-13 | before/after/metadata besar | Tersimpan JSON utuh, tak pernah dibaca UI |
| E-14 | Dua tulis bersamaan | Dua baris (tanpa dedupari — append-only!) |

---

## 4. Katalog Pesan (teks apa adanya)

- `Dashboard > Riwayat Aktivitas` (breadcrumb) · `Riwayat Aktivitas` (judul + menu Pantauan).
- `Jejak aktivitas penting yang tercatat dari modul operasional dan konfigurasi.`
- `Aktivitas Terbaru` · `Memuat…` · `{total} aktivitas tercatat.` · `Belum ada aktivitas` ·
  `Aktivitas penting akan muncul setelah ada perubahan data di modul operasional atau pengaturan.`
- Kolom: `Waktu` `Pengguna` `Aktivitas` `Data Terkait` `Keterangan`.
- `Aktivitas tercatat otomatis.` (selalu!) · `System` · `User #{id}` · `Branch #{id}` · `-` (waktu kosong).
- Grup izin: `Riwayat Aktivitas` → `Lihat`.

---

## 5. Yang Sengaja Tidak Ada (verifikasi, bukan asumsi)

| # | Hal yang tidak ada | Bukti |
|---|---|---|
| NF-01 | Ubah/hapus baris | Service tanpa update/delete; controller 1 endpoint |
| NF-02 | BranchGuard (sengaja — lintas cabang!) | Controller hanya JWT + permission |
| NF-03 | Filter di halaman (cabang/entitas/aksi/tanggal!) | Page memanggil tanpa filter; API mendukungnya |
| NF-04 | Tampil before/after/metadata | Adapter tak memetakan; kolom Keterangan selalu default |
| NF-05 | Label untuk 27 nilai runtime nyata (14 statis: `member_type`×3, `customer_address`×3, `purchase_return.create`, `delivery.proof_uploaded`, `finance.opening.supplement`, `finance.posting.close_day`, `product.bulk_import`, `company.profile.update`, `sales_return`×2 dispatch/confirm; 12 dinamis `customer|supplier|partner`×create/update/archive/restore; 1 skrip `finance.cost_ledger.restatement`) | Tak ada di `ACTION_LABELS` (KI-127; daftar pasti hasil diff programatik label↔kunci) |
| NF-06 | Kunci untuk 14 label mati (`company.update`, `delivery_note.create/update`, `category.create/update/archive`, `payment.upload_proof`, `finance.period.close`, `status.create/update`, `transition.create`, `user.archive`, `stock.adjust.approve`) + 7 label entitas mati (`product_media`, `category`, `stock_adjustment`, `goods_receipt`, `order_status_definition`, `order_status_transition`, `finance_journal` — backend memakai `media_file`/`finance_journal_entry`/tanpa padanan) | Tak ada di backend (KI-127; diff programatik) |
| NF-07 | Direktori `apps/e2e/tests/audit-log/` + `dashboard/` | Tak ada (klaim knowledge basi — KI-130) |
| NF-08 | Audit untuk baca/cron/auth/reporting/audit itu sendiri | Daftar exempt knowledge §4 |
| NF-09 | Unduh berkas pajak (`finance/export/tax-package`) tanpa jejak siapa-mengunduh-apa-kapan | `FinanceExportService` tanpa `AuditLogService` (lihat F-01c) |
| NF-10 | Kunci `finance.cost_ledger.restatement` ditulis skrip SQL mentah (actor `system`), di luar `AuditLogService` dan di luar enumerasi `.log()` | `rebuild-cost-ledger.ts:416-430` |
