# ADR-0004: Split legacy `order` module into six modules

## Status
Accepted

## Context
The legacy system (`../mini-erp`) put purchasing, sales, goods receipt, delivery, sales return,
and purchase return all inside one backend module (`order/`, ~9,057 lines) — the largest module
in the legacy codebase and its own analysis flagged it as a candidate for splitting
(`docs/legacy-reference/00-module-map.md`).

## Decision
The new backend splits this into six independent modules, each with its own `contracts/`:
`purchasing`, `sales`, `goodsreceipt`, `delivery`, `salesreturn`, `purchasereturn`. `purchasing`
and `sales` share the concept of an "order" but are not merged — cross-references (e.g. a delivery
referencing a sales order) go through `contracts/`, not shared internals.

## Consequences
Clear ownership per document type, easier to test and reason about in isolation. Trade-off: shared
"order" concepts (numbering, status vocabulary) must be deliberately kept consistent across the
two order modules via `branch` (document sequence) and each module's own status handling, not via
a shared base class.
