# Data Model Legacy (Konsep) — Modul 18 Dashboard

**Kelompok B — konsep saja**, bukan skema persis.

## 1. Tabel Milik Sendiri

**Tidak ada** — modul 100% baca (tanpa entity, tanpa migrasi!).

## 2. Tabel yang Dibaca (milik modul lain!)

Modul tanpa `TypeOrmModule.forFeature` milik sendiri selain pinjam entity
(`dashboard.module.ts:12–17`: `Order`, `Product`, `KnowledgeDocument`,
`WhatsappChannel`); sisanya via `orderRepo.manager.query` mentah. Semua filter
cabang = `id_branch = sesi`; semua filter perusahaan = `id_company = sesi`.

| Tabel (kolom dipakai) | Dipakai untuk | Filter lingkup |
|---|---|---|
| `orders` (`id_branch, archived_at, order_kind, order_date, goods_delivered_at, due_date, notes, total_amount, id_current_status, id_related_party`) | ringkasan grup, hitung-hari, net-hari/bulan, tren, prioritas, hitung-selesai, top, margin, utang/piutang, transaksi-total | cabang sesi; arsip-keluar; jenis/status sesuai metrik |
| `order_status_definitions` (`status_group`, `label`) | grup ringkasan/tren/prioritas/utang (`INNER JOIN currentStatus`, kecuali hitung-hari & reporting!) | tanpa filter sendiri |
| `order_items` (`id_product, *_snapshot, quantity_in_base_uom, line_total_before_tax, line_total, tax_amount, cogs_amount_snapshot, margin_costed_at, stock_tracked_snapshot`) | top + margin pendapatan & HPP lapis-1/3 | via `orders` cabang |
| `stock_issue_allocations` (`id_order, id_order_item, id_branch, id_product, quantity_in_base_uom, id_inventory_movement`) | HPP lapis-2 | `o.id_branch = sesi` |
| `sales_returns` (`id_branch, archived_at, status, return_date, difference_amount, id_original_order`) | net (UNION kedua), tren, top, margin, utang-penyesuai | cabang sesi; `completed` + arsip-keluar |
| `sales_return_items` (`id_product, *_snapshot, quantity_in_base_uom, line_total*, tax_amount, line_type, id_inventory_movement`) | top + margin sisi retur (`returned`/`replacement` saja!) | via `sales_returns` cabang |
| `sales_return_settlements` (`amount, settlement_type: collect_payment/refund/customer_credit`) | pengurang piutang | via retur completed cabang |
| `payments` (`id_branch, archived_at, id_order, amount`) + `payment_allocations` (`id_branch, archived_at, id_order, allocated_amount ⨝ payments`) | pengurang utang/piutang | cabang sesi |
| `inventory_balances` (`id_branch, id_product, id_stock_location, available_qty, updated_at`) | kritis (SUM per produk) | cabang sesi |
| `stock_locations` (`status, archived_at`) | kritis (hanya `active` + tak-arsip!) | — |
| `inventory_movements` (`movement_type='out'`, `metadata_json.returnUnitCost`) | HPP lapis-2/4 | via alokasi/retur |
| `finance_inventory_cost_movements` (`id_company, id_inventory_movement, total_cost_amount`) + `finance_inventory_cost_states` (`id_company, id_branch, id_product, average_cost`) | HPP hierarki | perusahaan (+cabang untuk state!) |
| `products` (`id_company, stock_tracked, archived_at, status, min_stock_qty, base_uom, product_name, purchase_price, purchase_to_base_factor`) | kritis + lacak + HPP-beli | perusahaan sesi |
| `knowledge_documents` (`id_company, archived_at, status`) | total + siap (`ready`) | perusahaan sesi |
| `whatsapp_channels` (`idCompany → sessionStatus, displayNumber, updatedAt`) | status WA | perusahaan sesi |
| `branches/companies` (via sesi, bukan query!) | `idActiveBranch`/`idCompany` dari `ActiveSessionPayload` (`dashboard.controller.ts:18–20`) | — |

## 3. Catatan Konseptual untuk Rebuild

- Tanpa warisan skema — satu-satunya "model" adalah bentuk respons 20 kunci teratas
  (dokumen lama menyebut 21 — cek! hitung langsung `dashboard.service.ts:106–127` =
  20; kontrak di Kelompok A!). Pertahankan nama field `snake_case` apa adanya.
- Jangan "memiliki" tabel orang lain — dashboard tetap lapisan baca; satu-satunya
  risiko adalah join lintas-perusahaan yang salah (operasional = cabang, master/cost =
  perusahaan — lihat tabel di atas!).
- Bentuk tiap field didokumentasikan di `business-rules.md` §2 (rincian per metrik).
