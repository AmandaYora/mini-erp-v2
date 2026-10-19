# ADR-0016: atomicity across modules = idempotent steps + CAS, single-instance lock as a courtesy

## Status
Accepted (Fase 2, 2026-10-09)

## Context
A business action often spans modules (confirm a shipment: post stock, settle
payment, flip the status, complete the order). Cross-module transactions are
forbidden (`ADR-0002`), and an outbox would add machinery this system does not
need.

## Decision
- Every step is **idempotent** (unique keys on document + position + direction,
  `client_ref` per document) and the action is ordered so that repeating the
  same request after a failure in the middle completes it: validate → idempotent
  effects → CAS status flip (`UPDATE … WHERE status IN (…)` + `RowsAffected`) →
  idempotent post-final step.
- A lost race is read as "already done" (replay → 200), never as a conflict,
  unless the target state differs.
- Nothing is compensated silently. A document left half-done (a draft whose
  stock already moved, a pending receipt) is **visible** (`GET /ops/integrity`)
  and resolved by repeating or cancelling it.
- `shared/doclock` serialises same-document work inside one process
  (single-container deployment). It is a courtesy that reduces contention, not a
  correctness mechanism: with it bypassed, the unique keys, CAS and locks held
  by the database still keep stock and status exact (the concurrency tests run
  both ways).
- Deadlocks (1213) and lock timeouts (1205) are retried by `dberr.RetryTx`; lock
  ordering is global and sorted.

## Consequences
Running more than one instance stays safe for stock and status; only the
cross-document quota checks would need `SELECT … FOR UPDATE` on the order row
(the places are marked).
