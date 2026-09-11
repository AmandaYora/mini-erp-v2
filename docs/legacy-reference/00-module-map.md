# 00 — Peta Modul Legacy (Mini ERP)

**Status:** mapping struktural saja — dibuat murni dari struktur folder, nama file, nama
class/controller, dan nama route. **Belum ada analisis logic.** Semua klaim di dokumen ini bisa
diverifikasi tanpa membaca isi method.

**Tujuan dokumen:** menjadi peta wilayah untuk rebuild (arsitektur/algoritma/teknologi/schema baru,
tapi fitur + UX flow + tampilan identik). Setiap modul di sini akan dianalisis mendalam pada tahap
berikutnya — checklistnya di [PROGRESS.md](PROGRESS.md).

---

## 0. Catatan Penting Sebelum Membaca

**Repo ini bukan monolith legacy dalam arti klasik.** Ia sudah berupa **monorepo npm workspaces**
yang terstruktur modular:

| Workspace | Isi | Tech |
|---|---|---|
| `apps/api` | Backend modular-monolith, 18 NestJS module | NestJS + TypeORM + MySQL |
| `apps/web` | Frontend SPA, 17 module folder | React + React Router + Tailwind + Vite (PWA) |
| `apps/e2e` | 27 spec Playwright berbasis alur bisnis | Playwright |
| `packages/shared-types` | primitive lintas app (money, quantity, text, enums) | TS lib |
| `packages/shared-contracts` | envelope API + union `PermissionCode` | TS lib |
| `android-pos-shell` | Shell Android/Kotlin untuk POS (WebView + printer ESC/POS) | Kotlin/Gradle |

Konsekuensi untuk rebuild: yang "legacy" di sini adalah **skema DB + algoritma + beberapa keputusan
arsitektur**, bukan tata letak kode. Repo juga **sudah punya knowledge base** di
[`knowledge/`](../../knowledge/) (18 modul, 9 berkas domain) yang bisa dipakai sebagai pembanding
saat analisis mendalam — dokumen ini disusun independen dari sana, dari struktur nyata.

Konvensi arsitektur yang terlihat dari struktur (relevan saat rebuild):

- Semua endpoint bisnis adalah `@Post` dengan body `{ data }`. **220 endpoint** terhitung;
  satu-satunya `@Get` adalah `assistant/runs/stats`.
- Schema DB hanya lewat migrasi SQL bernomor: **`001_baseline.sql` … `050_purchase_returns.sql`**
  (`apps/api/src/database/migrations/`). Ini urutan evolusi fitur yang paling jujur di repo.
- State frontend = React Context (`store/app-store.tsx`) + slice, bukan Redux/Zustand.
- Semua HTTP frontend lewat satu pintu `apps/web/src/lib/api.ts`.

---

## 1. Ringkasan Modul yang Ditemukan

21 modul bisnis + 1 lapisan shared. Kolom "API module" merujuk `apps/api/src/modules/`,
"Web module" merujuk `apps/web/src/modules/`.

| # | Modul / Domain | API module | Web module | Route utama (FE) |
|---|---|---|---|---|
| 01 | Auth & Session | `auth` | `auth` | `/login`, `/select-branch` |
| 02 | Users, Roles & Permissions | `user` | `users` (+ `company/roles-settings`) | `/users`, `/settings/roles` |
| 03 | Company & Settings | `company` | `company` | `/settings`, `/settings/order-status` |
| 04 | Branch (Multi-Cabang) | `branch` | `company/pages/branches-pages` | `/branches` |
| 05 | Product / Catalog | `product` | `products` | `/products`, `/product-categories` |
| 06 | Business Party (Customer/Supplier) | `business-party` | `business-party` | `/customers`, `/suppliers` |
| 07 | Member Type & Member Pricing | `business-party` (sub) | `member-types` | `/member-types` |
| 08 | Order — Purchasing | `order` | `orders` | `/orders` (kind pembelian) |
| 09 | Order — Sales | `order` | `orders` | `/orders` (kind penjualan) |
| 10 | Goods Receipt (Penerimaan Barang) | `order` (sub) | `orders` | via `/orders/:orderId` |
| 11 | Delivery / Pengiriman | `order` (sub) | `orders` (sub) | `/delivery-work-queue` |
| 12 | Sales Return (Retur Penjualan) | `order` (sub) | `sales-returns` | `/sales-returns` |
| 13 | Purchase Return (Retur Pembelian) | `order` (sub) | `purchase-returns` | `/purchase-returns` |
| 14 | Payment / Pembayaran | `payment` | `orders` (sub) | via `/orders/:orderId` |
| 15 | POS / Kasir | `order` + `payment` (source POS) | `pos` | `/pos` |
| 16 | Stock / Inventory & Gudang | `stock` | `stock` | `/stock`, `/gudang` |
| 17 | Finance / Accounting & Pajak | `finance` | `finance` | `/finance/*` (16 halaman) |
| 18 | Dashboard | `dashboard` | `dashboard` | `/dashboard` |
| 19 | Reporting | `reporting` | *(tanpa folder khusus)* | — |
| 20 | Audit Log | `audit-log` | `audit-log` | `/audit-logs` |
| 21 | Assistant AI + WhatsApp + Knowledge/RAG | `assistant`, `tools`, `whatsapp`, `knowledge` | `assistant` | `/assistant/*` |
| — | Storage / Media | `storage` | *(dipakai lintas modul)* | — |
| — | **Shared / Cross-Cutting** | `common`, `infrastructure`, `database` | `components`, `store`, `lib`, `layout` | — |

**Anomali yang perlu dicatat sekarang** (jangan diselesaikan di tahap ini):

- Modul `order` adalah yang terbesar dan menampung **6 domain** sekaligus: order, goods receipt,
  delivery, sales return, purchase return, order status workflow. Kandidat pemecahan saat rebuild.
- `reporting` punya API module tapi **tidak punya folder web** — endpoint `reporting/orders` dan
  `reporting/stock` dikonsumsi dari luar folder itu (perlu ditelusuri saat analisis).
- Route `/knowledge`, `/knowledge/upload`, `/knowledge/retrieval`, `/whatsapp`,
  `/assistant/channel|knowledge|tools` semuanya **redirect-only** (`<Navigate>` ke
  `/assistant/setup` atau `/assistant/config`) — alias legacy, bukan halaman nyata.
- `tools` dan `storage` adalah module API tanpa controller (service-only, dipakai module lain).

---

## 2. Peta Modul → File

Notasi: `C` controller · `S` service · `E` entity · `T` test/spec · `P` page · `H` hook ·
`Cmp` component · `Sl` slice.

### 01 — Auth & Session

**API** `apps/api/src/modules/auth/`

- `C` `auth.controller.ts` — `@Controller('auth')`
- `S` `auth.service.ts` + `T` `auth.service.spec.ts`
- Guards: `guards/jwt-auth.guard.ts`, `guards/permission.guard.ts` (+`T`), `guards/branch.guard.ts`
- Strategy: `strategies/jwt.strategy.ts` (+`T`)
- `auth.module.ts`
- Entity pendukung ada di module `user`: `user-session.entity.ts`

**Endpoint** `auth/login`, `auth/refresh`, `auth/logout`, `auth/me`, `auth/switch-role`,
`auth/switch-branch`

**Web** `apps/web/src/modules/auth/`

- `P` `pages/auth-pages.tsx`
- `H` `hooks/use-auth-module.ts` (+`T`)
- `auth-redirect.ts` (+`T`)

**E2E** `apps/e2e/tests/business/00-auth-shell-access.spec.ts`

---

### 02 — Users, Roles & Permissions

**API** `apps/api/src/modules/user/`

- `C` `user.controller.ts` (`@Controller('users')`), `role.controller.ts` (`@Controller('roles')`)
- `S` `user.service.ts` (+`T`), `role.service.ts` (+`T`)
- `reserved-role-policy.ts` — kebijakan role `superadmin`
- `E` `user.entity.ts`, `role.entity.ts`, `permission.entity.ts`, `role-permission.entity.ts`,
  `user-role.entity.ts`, `user-branch-access.entity.ts`, `user-session.entity.ts`
- `user.module.ts`

**Endpoint** `users/{list,create,update,update-status,change-password}`,
`roles/{list,create,update,delete,permissions/update}`

**Web**

- `P` `modules/users/pages/users-pages.tsx` (+`T`)
- `H` `modules/users/hooks/use-users-module.ts` (+`T`)
- `P` `modules/company/pages/roles-settings-page.tsx` (+`T`)
- `modules/company/components/role-access-config.ts` (+`T`)
- `Sl` `store/slices/users.slice.ts` (+`T`), `store/slices/roles.slice.ts` (+`T`)

**Kontrak** `packages/shared-contracts/src/permission-code.ts` — union `PermissionCode`
(prefix terbanyak: `order`, `stock`, `product`, `payment`, `knowledge`, `user`)

**E2E** `10-observability-permission-hardening.spec.ts`, `18-role-access-matrix.spec.ts`

---

### 03 — Company & Settings

**API** `apps/api/src/modules/company/`

- `C` `company.controller.ts` (`@Controller('company')`)
- `S` `company.service.ts` (+`T`)
- `E` `company.entity.ts`, `company-settings.entity.ts`, `company-feature.entity.ts`
- `company.module.ts`

**Endpoint** `company/profile/{get,update}`, `company/settings/{get,update}`,
`company/features/{list,update}`

**Web** `apps/web/src/modules/company/`

- `P` `pages/settings-page.tsx`, `pages/order-status-settings-page.tsx`
- `H` `hooks/use-company-module.ts` (+`T`)
- `Sl` `store/slices/company.slice.ts`, `store/slices/statuses.slice.ts`
- Adapter `store/adapters/org.adapter.ts`, tipe `types/org.ts`

---

### 04 — Branch (Multi-Cabang)

**API** `apps/api/src/modules/branch/`

- `C` `branch.controller.ts` (`@Controller('branches')`)
- `S` `branch.service.ts` (+`T`)
- `E` `branch.entity.ts`, `branch-document-sequence.entity.ts`
- `branch.module.ts`

**Endpoint** `branches/{my-access,active,list,create,update}`

**Web** `P` `modules/company/pages/branches-pages.tsx` · route `/branches`, `/branches/create`,
`/branches/:branchId/edit` · guard backend `auth/guards/branch.guard.ts`

**E2E** `04-master-data-branch-gudang.spec.ts`

---

### 05 — Product / Catalog

**API** `apps/api/src/modules/product/`

- `C` `product.controller.ts`
- `S` `product.service.ts` (+`T`), `product-import.service.ts` (+`T`)
- `E` `product.entity.ts`, `product-category.entity.ts`, `product-variant.entity.ts`
- `product.module.ts`

**Endpoint** `products/{list,search-options,detail,create,update,archive}`,
`products/media/{list,upload,archive,set-primary}`,
`product-categories/{list,create,update,archive}`, `product-imports/{preview,commit}`

**Web** `apps/web/src/modules/products/`

- `P` `pages/products-list-page.tsx`, `pages/product-detail-page.tsx`,
  `pages/product-form-page.tsx` (+`T`), `pages/product-categories-page.tsx`
- `Cmp` `product-media-panel.tsx` (+`T`), `product-bulk-upload-modal.tsx`,
  `bulk-upload-template.tsx`, `product-category-select.tsx` (+`T`),
  `product-inventory-fields.tsx`, `product-transaction-unit-field.tsx`,
  `initial-product-image-field.tsx`, `product-search-select.tsx` (+`T`),
  `field-label-with-hint.tsx`, `product-form-shared.ts`
- `product-type-config.ts` (+`T`)
- `H` `hooks/use-products-module.ts` (+`T`)
- `Sl` `store/slices/products.slice.ts` (+`T`) · adapter `store/adapters/catalog.adapter.ts` ·
  tipe `types/catalog.ts`
- Shared komposit: `components/domain/product-search-select.tsx`

**Migrasi terkait** `005_product_pricing_model`, `008_product_dual_uom`, `036_product_variants`,
`035_keep_legacy_product_uom_writable`

**E2E** `15-product-search-picker.spec.ts`

---

### 06 — Business Party (Customer / Supplier)

**API** `apps/api/src/modules/business-party/`

- `C` `business-party.controller.ts`, `customer-address.controller.ts`
- `S` `business-party.service.ts` (+`T`), `customer-address.service.ts` (+`T`)
- `E` `business-party.entity.ts`, `business-party-delivery-address.entity.ts`
- `business-party.module.ts`

**Endpoint** `customers/{list,detail,create,update,archive,restore}`,
`suppliers/{list,detail,create,update,archive,restore}`,
`customer-addresses/{list,create,update,archive}`

**Web** `apps/web/src/modules/business-party/`

- `P` `pages/customers-pages.tsx`, `pages/suppliers-pages.tsx`
- `Cmp` `components/address-picker.tsx` (+`T`), `components/customer-address-book.tsx`
- `H` `hooks/use-business-party-module.ts` (+`T`), `hooks/use-customer-addresses.ts`
- `Sl` `store/slices/parties.slice.ts`
- Shared komposit: `components/domain/party-search-select.tsx`

**Migrasi terkait** `044_customer_delivery_addresses`

**E2E** `20-customer-crud-consistency.spec.ts`, `21-customer-edit-create-anomaly.spec.ts`,
`22-customer-relational-integrity.spec.ts`

---

### 07 — Member Type & Member Pricing

**API** `apps/api/src/modules/business-party/` (sub-domain)

- `C` `member-type.controller.ts`
- `S` `member-type.service.ts` (+`T`), `member-pricing.service.ts` (+`T`)
- `E` `member-type.entity.ts`

**Endpoint** `member-types/{list,create,update,archive}`, `pricing/quote`

**Web** `apps/web/src/modules/member-types/`

- `P` `pages/member-types-page.tsx`
- `H` `hooks/use-member-types-module.ts`
- `Sl` `store/slices/member-types.slice.ts`

**Route** `/member-types`, `/member-types/create`, `/member-types/:memberTypeId/edit`

**Migrasi terkait** `040_member_pricing`

---

### 08–10 — Order (Purchasing, Sales, Goods Receipt)

**API** `apps/api/src/modules/order/` — modul terbesar

- `C` `order.controller.ts`
- `S` `order.service.ts` (+`T` `order.service.spec.ts`, `order.service.financial.spec.ts`),
  `order-status.service.ts` (+`T`), `order-pricing.service.ts` (+`T`),
  `order-export.service.ts` (+`T`), `goods-receipt.service.ts`
- `order.types.ts`, `order.module.ts`
- `E` `order.entity.ts`, `order-item.entity.ts`, `order-status-definition.entity.ts`,
  `order-status-transition.entity.ts`, `order-status-history.entity.ts`,
  `goods-receipt.entity.ts`, `goods-receipt-item.entity.ts`, `stock-issue-allocation.entity.ts`

**Endpoint**
`orders/{list,export,detail,create,update,update-status,archive,receive-goods,deliver-goods,status-history,approve-credit}`,
`goods-receipts/{list,detail,create}`, `order-status-definitions/{list,create,update}`,
`order-status-transitions/{list,create,update}`

**Web** `apps/web/src/modules/orders/`

- `P` `pages/orders-list-page.tsx`, `pages/order-detail-page.tsx`,
  `pages/order-form-page.tsx` (+`T`)
- `Cmp` `components/order-form-helpers.ts`, `components/order-detail-helpers.ts`
- Cetak: `pages/sales-document-print-page.tsx`, `pages/sales-print-pages.tsx` (+`T`),
  `components/print-shared.tsx` (+`T`), `components/sales-print-shared.tsx` (+`T`)
- `H` `hooks/use-orders-module.ts` (+`T`)
- `Sl` `store/slices/orders.slice.ts` · adapter `store/adapters/commerce.adapter.ts` ·
  tipe `types/commerce.ts`

**Route** `/orders`, `/orders/create`, `/orders/:orderId`, `/orders/:orderId/edit`,
`/orders/:orderId/sales-document/print`

**Migrasi terkait** `006_backfill_transition_labels`, `007_rename_order_kind_values`,
`010_order_credit_approval`, `011_order_financial_terms`, `016_order_financial_foundation`,
`018_goods_receipts`, `041_order_export_permission`, `049_order_item_cost_snapshot`

**E2E** `02-operational-real-flow.spec.ts`, `05-purchase-receipt-payment.spec.ts`,
`11-volume-realistic-ui-50-po-100-sales.spec.ts`, `16-order-export-report.spec.ts`,
`17-revision-blackbox.spec.ts`

---

### 11 — Delivery / Pengiriman

**API** `apps/api/src/modules/order/` (sub-domain)

- `C` `delivery.controller.ts`
- `S` `delivery.service.ts` (+`T`), `delivery-proof.service.ts` (+`T`)
- `E` `delivery-note.entity.ts`, `delivery-note-item.entity.ts`

**Endpoint** `deliveries/{create,confirm,list,work-queue,archive,proof/upload,proof-url}`

**Web**

- `P` `modules/orders/pages/delivery-work-queue-page.tsx`,
  `modules/orders/pages/delivery-note-print-page.tsx` (+`T`)
- `Cmp` `modules/orders/components/delivery-section.tsx`
- `H` `modules/orders/hooks/use-delivery-notes.ts`

**Route** `/delivery-work-queue`, `/orders/:orderId/delivery/:sjId/print`

**Migrasi terkait** `013_sales_delivery_flow`, `014_delivery_notes`, `037_delivery_note_proof`,
`046_replacement_delivery`

**E2E** `06-sales-delivery-construction.spec.ts`, `18-exchange-replacement-delivery.spec.ts`

---

### 12 — Sales Return (Retur Penjualan)

**API** `apps/api/src/modules/order/` (sub-domain)

- `C` `sales-return.controller.ts` (`@Controller('sales-returns')`)
- `S` `sales-return.service.ts` (+`T`)
- `E` `sales-return.entity.ts`, `sales-return-item.entity.ts`, `sales-return-settlement.entity.ts`

**Endpoint** `sales-returns/{list,detail,context,preview,create}`,
`sales-returns/replacement-deliveries/{dispatch,confirm}`

**Web** `apps/web/src/modules/sales-returns/`

- `P` `pages/sales-returns-page.tsx` (+`T`), `pages/sales-returns-list-page.tsx`,
  `pages/sales-return-create-page.tsx`, `pages/sales-return-detail-page.tsx`
- `Cmp` `components/replacement-delivery-section.tsx`, `components/sales-returns-shared.ts`

**Route** `/sales-returns`, `/sales-returns/create`, `/sales-returns/:returnId`

**Migrasi terkait** `038_sales_returns`, `046_replacement_delivery`

---

### 13 — Purchase Return (Retur Pembelian)

**API** `apps/api/src/modules/order/` (sub-domain)

- `C` `purchase-return.controller.ts` (`@Controller('purchase-returns')`)
- `S` `purchase-return.service.ts` (+`T`)
- `E` `purchase-return.entity.ts`, `purchase-return-item.entity.ts`,
  `purchase-return-settlement.entity.ts`

**Endpoint** `purchase-returns/{list,detail,context,preview,create}`

**Web** `apps/web/src/modules/purchase-returns/`

- `P` `pages/purchase-returns-page.tsx`, `pages/purchase-returns-list-page.tsx`,
  `pages/purchase-return-create-page.tsx`, `pages/purchase-return-detail-page.tsx`
- `Cmp` `components/purchase-returns-shared.ts`

**Route** `/purchase-returns`, `/purchase-returns/create`, `/purchase-returns/:returnId`

**Migrasi terkait** `050_purchase_returns` (migrasi terakhir di repo)

**E2E** `08-inventory-return-transfer-adjustment.spec.ts`

---

### 14 — Payment / Pembayaran

**API** `apps/api/src/modules/payment/`

- `C` `payment.controller.ts`
- `S` `payment.service.ts` (+`T`)
- `E` `payment.entity.ts`, `payment-allocation.entity.ts`
- `payment.module.ts`

**Endpoint** `payments/{list,party-balances,party-ledger,create,upload-proof,proof-url,archive}`

**Web**

- `Cmp` `modules/orders/components/payment-section.tsx`
- `H` `modules/orders/hooks/use-payments.ts`
- `P` `modules/orders/pages/payment-receipt-print-page.tsx` (+`T`)

**Route** `/orders/:orderId/payments/:paymentId/print`

**Migrasi terkait** `009_payment_module`, `012_payment_proof_attachment`,
`033_payment_party_allocations`, `047_payment_amount_tendered`

**E2E** `07-pos-payments-ledger.spec.ts`

---

### 15 — POS / Kasir

**API** tidak punya module sendiri — memakai `order` + `payment` dengan penanda source POS
(`015_pos_order_source.sql`).

**Web** `apps/web/src/modules/pos/`

- `P` `pages/pos-page.tsx` (+`T` `pages/pos-product-card.test.tsx`)
- `Cmp` `pos-cart-item-row.tsx`, `pos-product-card.tsx`, `pos-category-drilldown.tsx`,
  `pos-checkout-dialog.tsx`, `pos-customer-panel.tsx` (+`T`), `pos-customer-autocomplete.tsx`,
  `pos-customer-summary.tsx`, `pos-scan-result-dialog.tsx`, `payment-method-button.tsx`,
  `pos-shared.tsx`
- `H` `hooks/use-pos-cart.ts` (+`T`)
- Cetak: `modules/orders/pages/pos-thermal-receipt-print-page.tsx`
- Perangkat: `lib/native-printer.ts` (+`T`), `components/domain/qr-scanner-modal.tsx` (+2 `T`)

**Shell Android** `android-pos-shell/app/src/main/java/id/minierp/kasir/`

- `MainActivity.kt`, `printer/PrinterBridge.kt`, `printer/PrinterService.kt`, `printer/EscPos.kt`,
  `printer/ReceiptRenderer.kt`, `printer/PrinterConfig.kt`, `printer/SettingsActivity.kt`
- Layout `res/layout/activity_main.xml`, `res/layout/activity_settings.xml`

**Route** `/pos`, `/orders/:orderId/pos-receipt/print`

**E2E** `07-pos-payments-ledger.spec.ts`

---

### 16 — Stock / Inventory & Gudang

**API** `apps/api/src/modules/stock/`

- `C` `stock.controller.ts` (`@Controller('stock')`)
- `S` `stock.service.ts` (+`T`), `stock-location.service.ts`, `stock-transfer.service.ts`,
  `stock-opening.service.ts` (+`T`), `stock-damaged.service.ts`
- `E` `inventory-balance.entity.ts`, `inventory-movement.entity.ts`, `stock-location.entity.ts`,
  `stock-reservation.entity.ts`, `stock-transfer.entity.ts`, `stock-transfer-item.entity.ts`
- `stock.module.ts`

**Endpoint** `stock/{balances,scan-product,detail,adjust,movements,transfer}`,
`stock/allocations/suggest`, `stock/reservations/{hold,release}`,
`stock/opening/{context,preview,commit}`, `stock/damaged/{list,move-in,restore,write-off}`,
`stock/locations/{list,create,update,archive}`,
`stock/transfers/{list,create,dispatch,receive,cancel}`

**Web** `apps/web/src/modules/stock/`

- `P` `pages/stock-list-page.tsx`, `pages/stock-detail-page.tsx`, `pages/gudang-page.tsx`,
  `pages/stock-locations-page.tsx`, `pages/stock-movements-page.tsx`,
  `pages/stock-transfer-page.tsx`, `pages/stock-location-transfer-page.tsx`,
  `pages/stock-adjustment-page.tsx`, `pages/stock-damaged-page.tsx`, `pages/stock-opening-page.tsx`
- `Cmp` `components/stock-opening-template.ts`
- `H` `hooks/use-stock-module.ts` (+`T`)
- `Sl` `store/slices/stock.slice.ts` · adapter `store/adapters/inventory.adapter.ts` ·
  tipe `types/inventory.ts`
- Shared komposit: `components/domain/stock-location-select.tsx`,
  `components/forms/hierarchical-select.tsx`

**Route** `/stock`, `/stock/movements`, `/stock/opening`, `/stock/adjustments/create`,
`/stock/:itemId`, `/gudang`, `/gudang/locations`, `/gudang/transfer`, `/gudang/pindah-lokasi`,
`/gudang/damaged`

**Util cost** `common/cost-sanity.util.ts` (+`T`), `common/uom-cost.util.ts` (+`T`)

**Migrasi terkait** `017_stock_adjustment_approval_and_permissions`,
`019_stock_issue_allocations`, `020_stock_location_allocation_flags`, `021_stock_reservations`,
`034_stock_transfer_documents`, `039_expand_quantity_precision`

**E2E** `08-inventory-return-transfer-adjustment.spec.ts`,
`12-stock-opening-real-usage-screenshots.spec.ts`, `04-master-data-branch-gudang.spec.ts`

---

### 17 — Finance / Accounting & Pajak

**API** `apps/api/src/modules/finance/` — **63 endpoint**, modul paling padat

- `C` `finance.controller.ts`, `finance-posting.controller.ts`, `finance-reporting.controller.ts`,
  `finance-close.controller.ts`, `finance-tax-adjustment.controller.ts`,
  `business-expense.controller.ts`
- `S` `finance.service.ts` (+`T`), `finance-posting.service.ts` (+`T`),
  `finance-reporting.service.ts` (+`T`), `finance-close.service.ts` (+`T`),
  `finance-inventory-cost.service.ts` (+`T`), `finance-export.service.ts` (+`T`),
  `finance-tax-adjustment.service.ts` (+`T`), `business-expense.service.ts` (+`T`)
- Util/kebijakan `finance-document-sequence.util.ts` (+`T`), `finance-posting.reasons.ts` (+`T`)
- Spec tambahan `finance-posting-a2-a3.spec.ts`, `finance-posting-batch-equivalence.spec.ts`
- `E` (16) `finance-account.entity.ts`, `finance-account-mapping.entity.ts`,
  `finance-cash-account.entity.ts`, `finance-journal-entry.entity.ts`,
  `finance-journal-line.entity.ts`, `finance-period.entity.ts`,
  `finance-posting-source.entity.ts`, `finance-inventory-cost-movement.entity.ts`,
  `finance-inventory-cost-state.entity.ts`, `finance-opening-balance.entity.ts`,
  `finance-opening-inventory-item.entity.ts`, `finance-document-sequence.entity.ts`,
  `finance-tax-period.entity.ts`, `finance-tax-report-snapshot.entity.ts`,
  `finance-tax-adjustment.entity.ts`, `business-expense.entity.ts`
- `finance.module.ts`

**Endpoint (kelompok)** `finance/accounts/*`, `finance/account-mappings/*`,
`finance/cash-accounts/*`, `finance/periods/*`, `finance/opening/*` (7),
`finance/posting-sources/*` (10), `finance/daily-close/overview`,
`finance/journals/{list,detail,reverse}`, `finance/reports/*` (12: cash-summary, receivables,
payables, inventory-value, margin, gross-profit, tax-summary, general-ledger, trial-balance,
profit-loss, balance-sheet, tax-detail), `finance/tax-periods/{list,close,reopen}`,
`finance/tax-adjustments/*`, `finance/expenses/*` (6), `finance/close/{readiness,safe-close}`,
`finance/export/tax-package`

**Web** `apps/web/src/modules/finance/` — 18 file halaman

- `P` `finance-pages.tsx` (+`T` `finance-pages.resolve.test.ts`), `finance-back-office-page.tsx`,
  `finance-back-office-advanced.tsx`, `finance-cash-page.tsx`, `business-expense-page.tsx`,
  `finance-receivables-payables-page.tsx`, `finance-margin-page.tsx`, `profit-loss-page.tsx`,
  `balance-sheet-page.tsx`, `trial-balance-page.tsx`, `general-ledger-page.tsx`,
  `finance-journals-page.tsx`, `finance-tax-page.tsx`, `tax-detail-page.tsx`,
  `tax-adjustment-page.tsx`, `period-close-page.tsx`, `finance-opening-page.tsx`,
  `finance-settings-page.tsx`
- `Cmp` `components/finance-shared.tsx`

**Route** `/finance/{back-office,cash,expenses,receivables-payables,margin,profit-loss,balance-sheet,tax,tax-detail,tax-adjustments,period-close,journals,general-ledger,trial-balance,opening,settings}`

**Util penting** `common/date-range.util.ts` (+`T`) — rentang tanggal zona Asia/Jakarta

**Database** `database/cost-ledger-rebuild.util.ts` (+`T`), `database/rebuild-cost-ledger.ts`,
`database/backfill-order-item-cost-snapshot.ts`

**Migrasi terkait** `022_finance_foundation`, `023_finance_posting_sources`,
`024_finance_inventory_valuation`, `025_finance_journals`, `026_finance_tax_reports`,
`030_finance_opening_equity`, `031_finance_business_expenses`, `032_finance_tax_adjustments`,
`043_round_money_to_whole_rupiah`, `045_finance_pending_reason_code`,
`048_finance_close_force_permission`

**E2E** `03-finance-real-flow.spec.ts`, `09-finance-spt-posting.spec.ts`,
`13-straight-through-finance-gap.spec.ts`, `15-tutup-buku-daily-close.spec.ts`,
`14-document-print-tax-coverage.spec.ts`, `zz-manual-finance-layer-ui.spec.ts`

---

### 18 — Dashboard

**API** `apps/api/src/modules/dashboard/`

- `C` `dashboard.controller.ts` (`@Controller('dashboard')`) · `S` `dashboard.service.ts` (+`T`) ·
  `dashboard.module.ts`

**Endpoint** `dashboard/summary`

**Web** `apps/web/src/modules/dashboard/`

- `P` `pages/dashboard-page.tsx` · `H` `hooks/use-dashboard-module.ts` (+`T`)
- Shared komposit: `components/structure/summary-card.tsx`

---

### 19 — Reporting

**API** `apps/api/src/modules/reporting/`

- `C` `reporting.controller.ts` (`@Controller('reporting')`)
- `S` `reporting.service.ts` (+`T`), `metrics-job.service.ts` (+`T`) — job terjadwal
  (`ScheduleModule`)
- `E` `daily-operational-metric.entity.ts` · `reporting.module.ts`

**Endpoint** `reporting/orders`, `reporting/stock`

**Web** *tidak ada folder module khusus* — konsumen sebenarnya perlu ditelusuri pada tahap analisis.

---

### 20 — Audit Log

**API** `apps/api/src/modules/audit-log/`

- `C` `audit-log.controller.ts` (`@Controller('audit-logs')`) · `S` `audit-log.service.ts` (+`T`)
- `E` `audit-log.entity.ts` · `audit-log.module.ts`
- Dipakai lintas modul: hampir semua service write memanggil `auditLog.log(...)`

**Endpoint** `audit-logs/list`

**Web** `apps/web/src/modules/audit-log/`

- `P` `pages/audit-log-pages.tsx` · `H` `hooks/use-audit-log-module.ts` (+`T`)
- `Sl` `store/slices/audit-log.slice.ts`

**E2E** `10-observability-permission-hardening.spec.ts`

---

### 21 — Assistant AI + WhatsApp + Knowledge/RAG

Satu domain UX (`/assistant/*`) yang dilayani **4 module API**.

**API `assistant/`**

- `C` `assistant.controller.ts` — `assistant/preview` (`@Post`) + `assistant/runs/stats`
  (**satu-satunya `@Get` di seluruh API**)
- `S` `assistant.service.ts` (+`T`)
- `E` `assistant-run.entity.ts`, `assistant-tool-execution.entity.ts` · `assistant.module.ts`

**API `tools/`** (service-only, tanpa controller)

- `S` `tools.service.ts` (+`T`) · `tools.module.ts`

**API `whatsapp/`**

- `C` `whatsapp.controller.ts`
- `S` `whatsapp.service.ts` (+`T`), `whatsapp-channel.service.ts` (+`T`),
  `whatsapp-gateway.service.ts` (+`T`) — gateway Baileys
- `E` `whatsapp-channel.entity.ts`, `whatsapp-authorization.entity.ts`,
  `conversation-thread.entity.ts`, `conversation-message.entity.ts` · `whatsapp.module.ts`
- **Endpoint** `whatsapp/status`, `whatsapp/channel/{connect,disconnect}`,
  `whatsapp/authorizations/{list,create,update,revoke}`,
  `whatsapp/assistant-config/{get,update}`, `whatsapp/messages/simulate`

**API `knowledge/`**

- `C` `knowledge.controller.ts` · `S` `knowledge.service.ts` (+`T`)
- `E` `knowledge-document.entity.ts`, `knowledge-document-version.entity.ts`,
  `knowledge-chunk.entity.ts` · `knowledge.module.ts`
- **Endpoint** `knowledge/documents/{list,create,detail,archive,update,process}`

**Web** `apps/web/src/modules/assistant/`

- `P` `pages/assistant-setup-page.tsx`, `pages/assistant-config-page.tsx`
- `Cmp` `assistant-setup-panel.tsx` (+`T`), `assistant-answer-mode-section.tsx`,
  `assistant-capabilities-section.tsx`, `assistant-knowledge-section.tsx`
- `H` `hooks/use-assistant-module.ts` (+`T`) · `Sl` `store/slices/assistant.slice.ts` (+`T`)

**Route** `/assistant` → redirect `/assistant/setup`; halaman nyata `/assistant/setup` dan
`/assistant/config`. `/assistant/channel`, `/assistant/knowledge`, `/assistant/tools`,
`/whatsapp`, `/knowledge`, `/knowledge/upload`, `/knowledge/retrieval` = alias redirect.

**Migrasi terkait** `003_phase3_assistant_mvp`, `004_phase4_rag_pipeline`,
`028_assistant_simulate_permission`

**E2E** `19-whatsapp-authorization-management.spec.ts`

---

### Storage / Media (pendukung lintas modul)

**API** `apps/api/src/modules/storage/` — service-only, tanpa controller

- `S` `storage.service.ts` (+`T`), `image-optimizer.service.ts` (+`T`)
- Driver `drivers/local-storage.driver.ts`, `drivers/s3-storage.driver.ts`
- `E` `media-file.entity.ts` · `storage.types.ts` · `storage.module.ts`
- Direktori runtime `apps/api/storage/`

**Konsumen** foto produk (`products/media/*`), bukti pembayaran (`payments/upload-proof`),
bukti pengiriman (`deliveries/proof/upload`)

**Web pendukung** `lib/image-compression.ts` (+`T`)

**Migrasi terkait** `027_media_files`

---

## 3. Kode Lintas Modul (Shared / Helper / Base)

Bagian ini yang paling menentukan biaya rebuild — sekali berubah, semua modul terpengaruh.

### 3.1 Shared Packages (memengaruhi API + Web sekaligus)

| File | Peran |
|---|---|
| `packages/shared-contracts/src/api-envelope.ts` | Bentuk envelope `{code, info, data, errors}` |
| `packages/shared-contracts/src/permission-code.ts` | Union `PermissionCode` — sumber tunggal nama permission |
| `packages/shared-types/src/money.ts` | Primitive uang |
| `packages/shared-types/src/quantity.ts` | Primitive kuantitas / UOM |
| `packages/shared-types/src/text.ts` | Primitive teks |
| `packages/shared-types/src/enums.ts` | Enum lintas domain |

### 3.2 Backend Cross-Cutting — `apps/api/src/common/`

| File | Peran |
|---|---|
| `interceptors/response.interceptor.ts` | Membungkus semua response sukses ke envelope |
| `filters/http-exception.filter.ts` | Envelope error terpusat |
| `decorators/active-session.decorator.ts` | Sumber tunggal `idCompany`/`idBranch`/`idUser`/role |
| `decorators/require-permission.decorator.ts` | Penanda permission per-endpoint |
| `types/session.types.ts` | Tipe `ActiveSessionPayload` |
| `date-range.util.ts` (+`T`) | Rentang tanggal zona waktu — dipakai berat oleh finance |
| `date.util.ts` | Helper tanggal |
| `number.util.ts` | Pembulatan/format angka |
| `pagination.util.ts` | Pola paginasi seragam |
| `search/tokenized-search.ts` (+`T`) | Pencarian token — dipakai list/search-options |
| `cost-sanity.util.ts` (+`T`) | Guard cost basis sebelum movement stok |
| `uom-cost.util.ts` (+`T`) | Konversi cost antar UOM |
| `db-error.util.ts` | Normalisasi error MySQL |

**Guards lintas modul** (`modules/auth/guards/`): `jwt-auth.guard.ts`, `permission.guard.ts`,
`branch.guard.ts` — dipakai hampir semua controller.

### 3.3 Backend Infrastruktur & Database

| File | Peran |
|---|---|
| `apps/api/src/app.module.ts` | Registrasi 18 module + `ConfigModule` + `ScheduleModule` |
| `apps/api/src/main.ts` | Bootstrap, interceptor/filter global |
| `infrastructure/database/database.module.ts` | Koneksi TypeORM (`synchronize:false`) |
| `infrastructure/database/clean-database-logger.ts` | Logger query |
| `database/migration-runner.ts` | Runner migrasi SQL bernomor |
| `database/seed-runner.ts` | Seed data awal (role, permission, akun default) |
| `database/db-reset.ts` | Reset DB dev |
| `database/migrations/001…050*.sql` | **50 migrasi** — jejak evolusi fitur |
| `database/cost-ledger-rebuild.util.ts` (+`T`), `rebuild-cost-ledger.ts` | Rebuild cost ledger |
| `database/backfill-order-item-cost-snapshot.ts` | Backfill snapshot HPP |
| `apps/api/src/types/*.d.ts` | Deklarasi tipe pihak ketiga (bcryptjs, passport-jwt, qrcode) |

### 3.4 Frontend Cross-Cutting

| File | Peran |
|---|---|
| `apps/web/src/lib/api.ts` (+`T`) | **Satu-satunya tempat `fetch`** — `apiPost`/`apiUpload`, buka envelope, `ApiError` |
| `apps/web/src/store/app-store.tsx` | `AppProvider` — root state (Context, bukan Redux) |
| `apps/web/src/store/store-types.ts` | Tipe store |
| `apps/web/src/store/slices/*.slice.ts` | 11 slice: assistant, audit-log, company, member-types, orders, parties, products, roles, statuses, stock, users |
| `apps/web/src/store/adapters/*.adapter.ts` | 5 adapter respons API → tipe FE: catalog, commerce, common, inventory, org |
| `apps/web/src/store/use-toasts.ts` | Notifikasi global |
| `apps/web/src/modules/module-registry.tsx` (+`T`) | **Registry rute + menu + permission** (944 baris) — sumber tunggal navigasi, 79 rute |
| `apps/web/src/route-access.ts` (+`T`) | Aturan akses rute diturunkan dari registry |
| `apps/web/src/app.tsx`, `main.tsx` | Root & bootstrap |
| `apps/web/src/layout/app-layout.tsx`, `layout/avatar-menu.tsx` | Shell aplikasi (sidebar, header) |
| `apps/web/src/types/*.ts` | Tipe FE: catalog, commerce, inventory, org, enums, shared |
| `apps/web/src/utils.ts` (+`T`), `utils/qr-export.ts` (+`T`) | Helper umum |
| `apps/web/src/hooks/use-global-loading.ts`, `lib/loading-bus.ts` | Indikator loading global |
| `apps/web/src/lib/pwa-registration.ts` (+`T`), `components/domain/pwa-update-prompt.tsx` | PWA |
| `apps/web/src/lib/chunk-load-recovery.ts` (+`T`) | Recovery chunk gagal muat |
| `apps/web/src/lib/native-printer.ts` (+`T`) | Bridge printer shell Android |
| `apps/web/src/lib/image-compression.ts` (+`T`) | Kompresi gambar sebelum upload |
| `apps/web/src/modules/shared/attribute-field.ts` (+`T`) | Field atribut lintas modul |
| `apps/web/src/modules/core/pages/error-pages.tsx`, `core/hooks/use-app-shell-module.ts` (+`T`) | Error page & app shell |

**Design system web** — `apps/web/src/components/` (dipakai semua modul; ini yang menjaga
"tampilan SAMA PERSIS"):

- `primitives/` — `button.tsx`, `badge.tsx`
- `forms/` — `form-field.tsx`, `select-field.tsx`, `search-select.tsx`, `async-search-select.tsx`,
  `hierarchical-select.tsx`, `segmented-control.tsx`, `password-input.tsx` (+`T`),
  `field-hint.tsx`, `select-shared.tsx`
- `data/` — `data-table.tsx`, `pagination.tsx`
- `structure/` — `page-header.tsx`, `section-card.tsx`, `filter-bar.tsx`, `action-row.tsx`,
  `summary-card.tsx`
- `overlays/` — `modal.tsx`, `confirm-dialog.tsx`, `toast-viewport.tsx`
- `feedback/` — `empty-state.tsx`, `loading-state.tsx`, `notice.tsx`, `global-loader.tsx`
- `domain/` — `product-search-select.tsx`, `party-search-select.tsx`, `stock-location-select.tsx`,
  `qr-scanner-modal.tsx`, `qr-generator-modal.tsx`, `pwa-update-prompt.tsx`
- `field-classes.ts`, `types.ts`, `index.ts` (barrel)
- Style `apps/web/src/tailwind.css`

### 3.5 Tooling & Test Infrastruktur

| File | Peran |
|---|---|
| `scripts/check-architecture.mjs` | Penegak batas arsitektur (`npm run arch:check` / `lint`) |
| `scripts/check-vanilla-css.sh`, `scripts/vanilla-check.js` | Guard CSS |
| `scripts/deploy-vps.sh` | Deploy VPS |
| `tsconfig.base.json`, `package.json` (workspaces) | Konfigurasi monorepo |
| `apps/e2e/global-setup.ts`, `apps/e2e/helpers/business.ts`, `apps/e2e/playwright.config.ts` | Fondasi E2E |
| `apps/web/src/test/setup.ts`, `apps/web/src/test/mock-store.ts` | Fondasi unit test web |

---

## 4. Aset Rujukan untuk Fase Berikutnya

Sumber terbaik untuk merekonstruksi **fitur + UX flow + tampilan** secara identik:

1. **`apps/web/src/modules/module-registry.tsx`** — 79 rute + menu + permission dalam satu file.
   Ini kontrak navigasi yang harus direplikasi 1:1.
2. **`apps/e2e/tests/business/*.spec.ts`** — 27 spec beralur bisnis nyata (termasuk uji volume
   50 PO / 100 penjualan dan screenshot). Ini spesifikasi perilaku paling eksekutabel di repo.
3. **`apps/api/src/database/migrations/001…050`** — urutan lahirnya setiap fitur; berguna untuk
   memahami "kenapa schema seperti ini" sebelum mendesain schema baru.
4. **`knowledge/`** — knowledge base yang sudah ada (18 modul, 9 berkas domain) sebagai pembanding.

---

## 5. Batasan Dokumen Ini

- Belum menyentuh isi method, aturan bisnis, state machine, atau algoritma apa pun.
- Belum memetakan tabel DB per modul (baru sebatas nama entity dan nama migrasi).
- Belum memverifikasi konsumen `reporting/*` di frontend.
- Pembagian "modul" untuk POS, Delivery, Goods Receipt, Sales/Purchase Return adalah **pembacaan
  domain**, bukan folder terpisah di backend — semuanya berada di dalam `modules/order/`
  (kecuali POS yang murni frontend).
