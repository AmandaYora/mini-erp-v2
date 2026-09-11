#!/usr/bin/env bash
# One-off helper used during initial project setup to lay down the empty module
# skeleton described in knowledge/MODULE_MAP.md. Safe to delete after the real
# modules are implemented — it only creates empty folders + a wiring stub.
set -euo pipefail

cd "$(dirname "$0")/../apps/api/internal/modules"

declare -A MODULES=(
  [audit]="Audit trail — records who did what, when, across all modules. Consumed as an infrastructure dependency by every write-side module."
  [media]="File storage abstraction (local disk / S3 driver) for product photos, payment proofs, and delivery proofs."
  [user]="User accounts, roles, and permissions (RBAC)."
  [company]="Single-company profile and application-wide settings."
  [branch]="Multi-branch directory and per-branch document numbering sequences."
  [product]="Product catalog, categories, variants, unit of measure and stock/sell conversion factor."
  [party]="Customers, suppliers, delivery addresses, member types and member pricing rules."
  [stock]="Stock locations, inventory balances and movements, transfers, adjustments, opening balances."
  [purchasing]="Purchase orders to suppliers."
  [sales]="Sales orders to customers (regular and POS channel)."
  [goodsreceipt]="Receiving of goods against a purchase order."
  [delivery]="Shipment of goods against a sales order: delivery notes and proof of delivery."
  [payment]="Payments and allocations against orders; party balances and ledger."
  [salesreturn]="Customer returns against a sales order and/or delivery."
  [purchasereturn]="Returns to a supplier against a purchase order and/or goods receipt."
  [finance]="Chart of accounts, journal entries, inventory costing (HPP), financial reports, tax period close and tax export."
  [dashboard]="Operational summary widgets for the active branch/company."
  [reporting]="Scheduled operational metrics aggregation (orders, stock)."
)

declare -A LAYERS=(
  [audit]="L0 — Foundation"
  [media]="L0 — Foundation"
  [user]="L1 — Identity & Organization"
  [company]="L1 — Identity & Organization"
  [branch]="L1 — Identity & Organization"
  [product]="L2 — Master Data"
  [party]="L2 — Master Data"
  [stock]="L3 — Inventory"
  [purchasing]="L4 — Core Transactions"
  [sales]="L4 — Core Transactions"
  [goodsreceipt]="L5 — Transaction Derivatives"
  [delivery]="L5 — Transaction Derivatives"
  [payment]="L5 — Transaction Derivatives"
  [salesreturn]="L6 — Returns"
  [purchasereturn]="L6 — Returns"
  [finance]="L7 — Finance & Tax"
  [dashboard]="L8 — Observability"
  [reporting]="L8 — Observability"
)

for name in "${!MODULES[@]}"; do
  base="$name"
  mkdir -p "$base"/{contracts,application,domain/events,infrastructure/queries,infrastructure/sqlc,presentation}
  for d in contracts application domain domain/events infrastructure infrastructure/queries infrastructure/sqlc presentation; do
    touch "$base/$d/.gitkeep"
  done
  cat > "$base/$name.module.go" <<EOF
package $name

// ${LAYERS[$name]}
// ${MODULES[$name]}
//
// Only contracts/ is importable by other modules — see
// knowledge/MODULE_MAP.md and .claude/rules/backend-modular-monolith.md.
EOF
  echo "created $base"
done
