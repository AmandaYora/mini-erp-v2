# ADR-0014: application layer = transaction scripts; contracts double as data model

## Status
Accepted (F8-14, 2026-10-09)

## Context
`domain/` holds ~200 lines while `application/` holds ~16k: the code is written
as use cases (transaction scripts), which fits a single-tenant ERP maintained
by one team. A full domain-driven rewrite would cost a lot and buy little.

## Decision
- `application/` = use cases. Pure rules that are reused or easy to get wrong
  (pricing, UOM conversion, permissions, pro-rating) live in `domain/` or
  `shared/money` as pure functions with table tests.
- A transaction starts in the layer that orchestrates several steps INSIDE one
  module; a repository opens its own transaction only for a single aggregate
  (header + lines). `application` does not import `database/sql` for new code;
  the stock module exposes `InTx`.
- `contracts` structs are the public API and the row model (21 of 28 files).
  They are split only when the DB shape and the API shape diverge.
- No port interfaces for repositories (one implementation, no second
  consumer); interfaces exist where a test double is needed (e.g. the WhatsApp
  gateway).

## Consequences
Money and stock flows stay readable top-to-bottom. The idempotency and CAS
rules (ADR-0016) are the correctness mechanism, not object lifecycles.
