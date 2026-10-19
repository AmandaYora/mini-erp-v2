# ADR-0010: shared/money as the arithmetic kernel (exception to the shared rule)

## Status
Accepted (keputusan pemilik K-9, 2026-10-08; implemented F8-7, 2026-10-09)

## Context
The backend rule says `shared/` holds only technical utilities, never domain
logic. Five copies of the line-pricing formula lived in sales, purchasing,
finance, salesreturn and purchasereturn, each commented "duplicated
intentionally". Finance re-derives document numbers from PO/SO items, so one
rounding divergence between any two copies books a journal that does not
match its document.

## Decision
`internal/shared/money` exposes the single money-math kernel:
`LineAmounts` (gross/discount/net/base/tax/total per line),
`Round` (half away from zero) and `Prorate` (share of a line amount).
All five modules call it; no module defines its own copy (enforced by
arch-check: `lineEconomics|CalcLine|prorate` outside `shared/money` fails).

The formula is pure numerics over the locked ADR-0005 policy (integer
rupiah, half-away-from-zero per step), not domain policy: tax types, rates
and discounts remain the caller's business inputs.

## Consequences
Finance == documents by construction instead of by five-hand maintenance.
Proven by a 1M-case property test against verbatim copies of the old
functions as oracles, plus the unchanged money-path integration tests.
