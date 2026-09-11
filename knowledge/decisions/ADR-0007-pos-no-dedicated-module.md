# ADR-0007: POS has no dedicated backend module

## Status
Accepted

## Context
In the legacy system, POS never had its own backend module — it was a frontend assembly of the
`order` and `payment` modules with a source marker
(`docs/legacy-reference/00-module-map.md` §15). This pattern worked well: POS was the lowest-risk
module to change (nothing depends on it) and served as the integration test that all its upstream
contracts were correct.

## Decision
Keep this pattern in the revamp. POS is a frontend page (`apps/web/src/modules/pos/`) that calls
the `sales` module's endpoint with `channel: "pos"`, plus `party`, `product`, `stock`, and
`payment` directly. No `internal/modules/pos/` exists.

## Consequences
One fewer module to maintain; POS automatically benefits from every improvement to `sales` and
`payment`. Trade-off: POS-specific behavior (e.g. an offline-resilient cart — see
`docs/PRD.md` §5, open question on cart persistence) must be solved in the frontend or via
`sales`/`payment` contracts, not by carving out POS-only backend logic.
