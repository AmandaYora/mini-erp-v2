# ADR-0011: audit writes inside the caller's transaction (limited exception)

## Status
Accepted (keputusan pemilik K-10, 2026-10-08; implemented F4-9/Fase 2, 2026-10-09)

## Context
Audit writes are best-effort after commit everywhere else: a crash between
the mutation commit and the trail write loses the trail silently. For money
and stock operations (payment create/cancel, finance post/manual/reverse,
stock adjust, delivery/return confirms, user role/permission writes) a lost
trail is a control failure, not a cosmetic gap.

## Decision
`AuditClient.LogTx(ctx, tx, Entry)` borrows the caller's open `*sql.Tx`
(through the `AuditTx` interface) so the trail row co-commits with the
mutation it describes. Scope is LIMITED to money/stock operations; every
other write keeps best-effort `Log`. The audit module never commits or
rolls back the borrowed handle, and it still owns the `audit_logs` /
`audit_log_details` tables — no module touches audit tables directly.

Money paths treat a `LogTx` error as fatal to the transaction (roll back),
never `_ =`-swallowed: an untrailed money mutation must not commit.

## Consequences
The modular-monolith transaction rule gains one documented exception: the
borrowing happens inside the owning module's own transaction, so no
cross-module transaction is created. Full conversion of money-path call
sites lands with F2a (each task) and F3b-8 (before/after details).
