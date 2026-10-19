# ADR-0013: twin modules stay separate (no generic document engine)

## Status
Accepted (F8-10, 2026-10-09)

## Context
Sales/purchasing and salesreturn/purchasereturn look alike (~3.4k and ~3.3k
lines). A generic document engine (generics or interfaces) would remove about
4% of the code while merging money flows whose rules differ: credit approval,
ship-to, member pricing and tax invoices on the sales side; supplier invoice,
credit days and receipt settlement on the purchase side; exchanges and the
settlement of returns.

## Decision
The twins remain separate modules. Their similarity is kept honest by
**parity tests**, not by shared machinery: the same scenario runs against a
sales order and a purchase order (and against both returns) and must give the
same structural outcome. Small mechanical helpers (`totals`, `StatusCounts`,
`OrderParties`, `ListByParty`, `SetStatus`) stay duplicated with cross-reference
comments. The single arithmetic source for money is `shared/money` (ADR-0010).

## Consequences
No risky refactor of money paths; the duplication is documented so it is not
proposed again. Repositories of the twins share the same sqlc pattern (ADR-0012),
so their likeness is visible in the queries rather than in Go.
